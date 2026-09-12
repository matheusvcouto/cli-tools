package aiprofile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeRunner struct {
	binary string
	args   []string
	env    []string
	calls  int
	err    error
}

func (f *fakeRunner) Replace(_ context.Context, binary string, args []string, env []string, _ ProcessIO) error {
	f.calls++
	f.binary = binary
	f.args = append([]string(nil), args...)
	f.env = append([]string(nil), env...)
	return f.err
}

func testService(t *testing.T) (*Service, *fakeRunner) {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "profiles")
	home := filepath.Join(base, "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	r := &fakeRunner{}
	return &Service{
		Store: Store{Root: root}, Runner: r, HomeDir: home,
		Now: func() time.Time { return time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC) },
		Env: func() []string {
			return []string{"PATH=/synthetic/bin", "KEEP=yes", "OPENAI_API_KEY=secret-placeholder", "ANTHROPIC_API_KEY=secret-placeholder"}
		},
	}, r
}

func TestCreateListRenameAndDelete(t *testing.T) {
	s, _ := testService(t)
	created, err := s.Create("claude", "personal")
	if err != nil {
		t.Fatal(err)
	}
	if created.Alias != "personal" || created.Tool != "claude" {
		t.Fatalf("unexpected profile: %+v", created)
	}
	if filepath.Dir(created.Dir) != s.Store.Root {
		t.Fatalf("profile escaped root: %s", created.Dir)
	}
	if info, err := os.Stat(created.Dir); err != nil || !info.IsDir() {
		t.Fatalf("profile dir missing: %v", err)
	}

	profiles, spec, err := s.List("claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0].Alias != "personal" || spec.ConfigEnv != "CLAUDE_CONFIG_DIR" {
		t.Fatalf("bad list: %+v %+v", profiles, spec)
	}

	renamed, err := s.Rename("claude", "personal", "main")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Dir != created.Dir {
		t.Fatalf("rename moved physical dir: %q != %q", renamed.Dir, created.Dir)
	}

	removed, err := s.DeleteConfirmed("claude", "main")
	if err != nil {
		t.Fatal(err)
	}
	if removed != created.Dir {
		t.Fatalf("unexpected removed path: %s", removed)
	}
	if _, err := os.Lstat(created.Dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("profile dir still exists or unexpected error: %v", err)
	}
	profiles, _, err = s.List("claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 0 {
		t.Fatalf("profile remained in index: %+v", profiles)
	}
}

func TestStoreCorruptionDoesNotBecomeEmpty(t *testing.T) {
	s, _ := testService(t)
	if err := s.Store.EnsureRoot(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Store.Root, "index.json"), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.List("claude"); err == nil {
		t.Fatal("expected corruption error")
	}
	if _, err := s.Create("claude", "new"); err == nil {
		t.Fatal("expected create to refuse corrupt store")
	}
}

func TestCreateRollsBackDirectoryWhenStoreCommitFails(t *testing.T) {
	s, _ := testService(t)
	first, err := s.Create("claude", "one")
	if err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "external-backup")
	if err := os.WriteFile(external, []byte("sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(s.Store.Root, "index.json.bak")
	_ = os.Remove(backup)
	if err := os.Symlink(external, backup); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Create("claude", "two"); err == nil {
		t.Fatal("expected create commit failure")
	}

	profiles, _, err := s.List("claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0].Alias != "one" || profiles[0].Dir != first.Dir {
		t.Fatalf("store changed despite failed create: %+v", profiles)
	}
	entries, err := os.ReadDir(s.Store.Root)
	if err != nil {
		t.Fatal(err)
	}
	dirs := 0
	for _, entry := range entries {
		if entry.IsDir() {
			dirs++
		}
	}
	if dirs != 1 {
		t.Fatalf("failed create leaked a profile directory: got %d directories", dirs)
	}
	raw, err := os.ReadFile(external)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "sentinel" {
		t.Fatalf("external backup target changed: %q", raw)
	}
}

func TestStoreBackupAfterUpdate(t *testing.T) {
	s, _ := testService(t)
	if _, err := s.Create("claude", "one"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Rename("claude", "one", "two"); err != nil {
		t.Fatal(err)
	}
	backup, err := s.Store.loadPath(filepath.Join(s.Store.Root, "index.json.bak"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := FindProfile(backup, "claude", "one"); !ok {
		t.Fatalf("backup did not preserve previous state: %+v", backup)
	}
}

func TestConcurrentCreatesAreSerialized(t *testing.T) {
	s, _ := testService(t)
	var wg sync.WaitGroup
	errCh := make(chan error, 12)
	for i := 0; i < 12; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Create("codex", "p"+string(rune('a'+i)))
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	profiles, _, err := s.List("codex")
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 12 {
		t.Fatalf("lost update: got %d profiles", len(profiles))
	}
}

func TestConcurrentSameAliasCreatesOneProfileDirectory(t *testing.T) {
	s, _ := testService(t)
	const workers = 8
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Create("codex", "same")
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)

	successes := 0
	for err := range errCh {
		if err == nil {
			successes++
			continue
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("unexpected create error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one successful create, got %d", successes)
	}

	profiles, _, err := s.List("codex")
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 {
		t.Fatalf("expected one profile, got %+v", profiles)
	}

	entries, err := os.ReadDir(s.Store.Root)
	if err != nil {
		t.Fatal(err)
	}
	dirs := 0
	for _, entry := range entries {
		if entry.IsDir() {
			dirs++
		}
	}
	if dirs != 1 {
		t.Fatalf("expected exactly one physical profile directory, got %d", dirs)
	}
}

func TestRunUsesProfileNativeContextWithoutDefaultCodexGuidance(t *testing.T) {
	s, r := testService(t)
	defaultDir := filepath.Join(s.HomeDir, ".codex")
	if err := os.MkdirAll(defaultDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(defaultDir, "AGENTS.md"), []byte("must not leak"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := s.Create("codex", "work")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Dir, "AGENTS.md"), []byte("profile guidance"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.Env = func() []string {
		env := []string{"PATH=/synthetic/bin", "KEEP=yes"}
		spec, _ := LookupTool("codex")
		for _, key := range spec.ClearEnv {
			env = append(env, key+"=synthetic-override")
		}
		return env
	}
	if err := s.Run(context.Background(), "codex", "work", []string{"--example", "value"}, ProcessIO{}); err != nil {
		t.Fatal(err)
	}
	if r.binary != "codex" || strings.Join(r.args, "|") != "--example|value" {
		t.Fatalf("bad process call: %s %#v", r.binary, r.args)
	}
	env := envMap(r.env)
	if env["CODEX_HOME"] != p.Dir {
		t.Fatalf("CODEX_HOME mismatch: %q", env["CODEX_HOME"])
	}
	spec, _ := LookupTool("codex")
	for _, key := range spec.ClearEnv {
		if _, ok := env[key]; ok {
			t.Fatalf("%s leaked", key)
		}
	}
	if env["KEEP"] != "yes" {
		t.Fatal("unrelated env was lost")
	}
	raw, err := os.ReadFile(filepath.Join(p.Dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "profile guidance" {
		t.Fatalf("profile guidance changed: %q", raw)
	}
	info, err := os.Lstat(filepath.Join(p.Dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("profile guidance was replaced by a symlink")
	}
}

func TestClaudeRunClearsAuthOverridesAndAddsDefaultContextExcludes(t *testing.T) {
	s, r := testService(t)
	p, err := s.Create("claude", "work")
	if err != nil {
		t.Fatal(err)
	}
	settings := []byte(`{"model":"example","claudeMdExcludes":["/already/excluded"]}`)
	if err := os.WriteFile(filepath.Join(p.Dir, "settings.json"), settings, 0o600); err != nil {
		t.Fatal(err)
	}
	s.Env = func() []string {
		env := []string{"PATH=/synthetic/bin", "KEEP=yes", "ANTHROPIC_CONFIG_DIR=/wrong/anthropic"}
		spec, _ := LookupTool("claude")
		for _, key := range spec.ClearEnv {
			env = append(env, key+"=synthetic-override")
		}
		// Generic cloud credentials are deliberately not Claude profile overrides.
		env = append(env, "AWS_PROFILE=project-profile", "GOOGLE_APPLICATION_CREDENTIALS=/project/gcp.json")
		return env
	}
	if err := s.Run(context.Background(), "claude", "work", nil, ProcessIO{}); err != nil {
		t.Fatal(err)
	}
	env := envMap(r.env)
	if env["CLAUDE_CONFIG_DIR"] != p.Dir {
		t.Fatalf("CLAUDE_CONFIG_DIR mismatch: %q", env["CLAUDE_CONFIG_DIR"])
	}
	wantAnthropicConfig := filepath.Join(p.Dir, anthropicConfigDirName)
	if env["ANTHROPIC_CONFIG_DIR"] != wantAnthropicConfig {
		t.Fatalf("ANTHROPIC_CONFIG_DIR=%q want %q", env["ANTHROPIC_CONFIG_DIR"], wantAnthropicConfig)
	}
	if info, err := os.Lstat(wantAnthropicConfig); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("profile-local Anthropic config directory is unsafe: info=%v err=%v", info, err)
	}
	spec, _ := LookupTool("claude")
	for _, key := range spec.ClearEnv {
		if _, ok := env[key]; ok {
			t.Fatalf("%s leaked", key)
		}
	}
	if env["AWS_PROFILE"] != "project-profile" || env["GOOGLE_APPLICATION_CREDENTIALS"] != "/project/gcp.json" {
		t.Fatalf("generic project cloud environment was unexpectedly removed: %#v", env)
	}
	if env["KEEP"] != "yes" {
		t.Fatal("unrelated env was lost")
	}

	raw, err := os.ReadFile(filepath.Join(p.Dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Model            string   `json:"model"`
		ClaudeMdExcludes []string `json:"claudeMdExcludes"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Model != "example" {
		t.Fatalf("existing setting lost: %#v", got)
	}
	want := []string{
		"/already/excluded",
		filepath.Join(s.HomeDir, ".claude", "CLAUDE.md"),
		filepath.Join(s.HomeDir, ".claude", "CLAUDE.local.md"),
		filepath.Join(s.HomeDir, ".claude", "rules", "**"),
	}
	for _, item := range want {
		found := false
		for _, existing := range got.ClaudeMdExcludes {
			if existing == item {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing claudeMdExcludes entry %q in %#v", item, got.ClaudeMdExcludes)
		}
	}

	// Preparation must be idempotent.
	before := string(raw)
	if err := s.Run(context.Background(), "claude", "work", nil, ProcessIO{}); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(p.Dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != before {
		t.Fatalf("second preparation changed settings:\nfirst=%s\nsecond=%s", before, raw)
	}
}

func TestClaudeRunRejectsSymlinkedAnthropicConfigDirectory(t *testing.T) {
	s, r := testService(t)
	p, err := s.Create("claude", "work")
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	marker := filepath.Join(outside, "marker")
	if err := os.WriteFile(marker, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(p.Dir, anthropicConfigDirName)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if err := s.Run(context.Background(), "claude", "work", nil, ProcessIO{}); err == nil {
		t.Fatal("expected symlinked Anthropic config directory to be rejected")
	}
	if r.calls != 0 {
		t.Fatal("Claude process ran despite unsafe Anthropic config directory")
	}
	raw, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "unchanged" {
		t.Fatalf("outside marker changed: %q", raw)
	}
}

func TestACPUsesSameIsolationAsRun(t *testing.T) {
	s, r := testService(t)
	p, err := s.Create("codex", "p")
	if err != nil {
		t.Fatal(err)
	}
	s.Env = func() []string {
		env := []string{"PATH=/synthetic/bin"}
		spec, _ := LookupTool("codex")
		for _, key := range spec.ClearEnv {
			env = append(env, key+"=synthetic-override")
		}
		return env
	}
	if err := s.ACP(context.Background(), "codex", "p", nil, ProcessIO{}); err != nil {
		t.Fatal(err)
	}
	env := envMap(r.env)
	if env["CODEX_HOME"] != p.Dir {
		t.Fatalf("CODEX_HOME mismatch: %q", env["CODEX_HOME"])
	}
	spec, _ := LookupTool("codex")
	for _, key := range spec.ClearEnv {
		if _, ok := env[key]; ok {
			t.Fatalf("ACP leaked %s", key)
		}
	}
}

func TestACPUsesAdapterAndNoWrapperOutput(t *testing.T) {
	s, r := testService(t)
	if _, err := s.Create("claude", "p"); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	app := App{Service: s, Version: "test"}
	err := app.Run(context.Background(), []string{"claude", "acp", "p", "--flag"}, AppIO{In: strings.NewReader(""), Out: &out, Err: &errOut})
	if err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("wrapper contaminated ACP stdout: %q", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("unexpected ACP stderr: %q", errOut.String())
	}
	if r.binary != "claude-agent-acp" || strings.Join(r.args, "|") != "--flag" {
		t.Fatalf("bad ACP call: %s %#v", r.binary, r.args)
	}
}

func TestApplyStatuslinePreservesOtherSettings(t *testing.T) {
	s, _ := testService(t)
	p, err := s.Create("claude", "p")
	if err != nil {
		t.Fatal(err)
	}
	settings := []byte(`{"model":"example","theme":"dark","statusLine":{"old":true}}`)
	if err := os.WriteFile(filepath.Join(p.Dir, "settings.json"), settings, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyStatusline("claude", "p", "default"); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	raw, err := os.ReadFile(filepath.Join(p.Dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := jsonUnmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["model"] != "example" || got["theme"] != "dark" {
		t.Fatalf("settings lost: %#v", got)
	}
	status, ok := got["statusLine"].(map[string]any)
	if !ok || status["type"] != "command" {
		t.Fatalf("bad statusLine: %#v", got["statusLine"])
	}
}

func TestDeleteRequiresBothConfirmations(t *testing.T) {
	s, _ := testService(t)
	p, err := s.Create("claude", "profile")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ConfirmDelete(strings.NewReader("wrong\ny\n"), ioDiscard{}, p); err == nil {
		t.Fatal("expected first confirmation failure")
	}
	if err := s.ConfirmDelete(strings.NewReader("profile\nn\n"), ioDiscard{}, p); err == nil {
		t.Fatal("expected final confirmation cancellation")
	}
	if err := s.ConfirmDelete(strings.NewReader("profile\nyes\n"), ioDiscard{}, p); err != nil {
		t.Fatal(err)
	}
}

func TestListJSONStableAndSorted(t *testing.T) {
	s, _ := testService(t)
	if _, err := s.Create("claude", "z"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create("claude", "a"); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	app := App{Service: s, Version: "test"}
	if err := app.Run(context.Background(), []string{"claude", "list", "--json"}, AppIO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"profile": "a"`) {
		t.Fatalf("missing JSON: %s", out.String())
	}
	if strings.Index(out.String(), `"profile": "a"`) > strings.Index(out.String(), `"profile": "z"`) {
		t.Fatalf("not sorted: %s", out.String())
	}
}

func envMap(items []string) map[string]string {
	m := map[string]string{}
	for _, item := range items {
		if k, v, ok := strings.Cut(item, "="); ok {
			m[k] = v
		}
	}
	return m
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

func jsonUnmarshal(raw []byte, v any) error { return json.Unmarshal(raw, v) }

func TestStoreRejectsSymlinkIndex(t *testing.T) {
	s, _ := testService(t)
	if err := s.Store.EnsureRoot(); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "external.json")
	valid := []byte(`{"schema_version":1,"profiles":[]}`)
	if err := os.WriteFile(external, valid, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(s.Store.Root, "index.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Store.Load(); err == nil || !strings.Contains(err.Error(), "symbolic-link") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
}

func TestStoreRejectsSymlinkLockWithoutTouchingTarget(t *testing.T) {
	s, _ := testService(t)
	if err := s.Store.EnsureRoot(); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "external-lock")
	if err := os.WriteFile(external, []byte("sentinel"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(s.Store.Root, ".index.lock")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create("claude", "p"); err == nil {
		t.Fatal("expected symlink lock rejection")
	}
	raw, err := os.ReadFile(external)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "sentinel" {
		t.Fatalf("external lock target changed: %q", raw)
	}
}

func TestStoreRejectsSymlinkBackup(t *testing.T) {
	s, _ := testService(t)
	if _, err := s.Create("claude", "one"); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "external-backup")
	if err := os.WriteFile(external, []byte("sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(s.Store.Root, "index.json.bak")
	_ = os.Remove(backup)
	if err := os.Symlink(external, backup); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Rename("claude", "one", "two"); err == nil {
		t.Fatal("expected symlink backup rejection")
	}
	raw, _ := os.ReadFile(external)
	if string(raw) != "sentinel" {
		t.Fatalf("external backup target changed: %q", raw)
	}
}

func TestApplyStatuslineRejectsSymlinkSettings(t *testing.T) {
	s, _ := testService(t)
	p, err := s.Create("claude", "p")
	if err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "external-settings.json")
	original := []byte(`{"sentinel":true}`)
	if err := os.WriteFile(external, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(p.Dir, "settings.json")); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyStatusline("claude", "p", "default"); err == nil {
		t.Fatal("expected symlink settings rejection")
	}
	raw, _ := os.ReadFile(external)
	if !bytes.Equal(raw, original) {
		t.Fatalf("external settings changed: %s", raw)
	}
}

func TestRunRejectsProfileDirectoryReplacedBySymlink(t *testing.T) {
	s, r := testService(t)
	p, err := s.Create("codex", "p")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(p.Dir); err != nil {
		t.Fatal(err)
	}
	external := t.TempDir()
	if err := os.Symlink(external, p.Dir); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(context.Background(), "codex", "p", nil, ProcessIO{}); err == nil {
		t.Fatal("expected replaced profile directory rejection")
	}
	if r.calls != 0 {
		t.Fatalf("process executed despite unsafe profile directory: %d calls", r.calls)
	}
}

func TestDeleteRollsBackDirectoryWhenStoreCommitFails(t *testing.T) {
	s, _ := testService(t)
	p, err := s.Create("claude", "p")
	if err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "external-backup")
	if err := os.WriteFile(external, []byte("sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(s.Store.Root, "index.json.bak")
	_ = os.Remove(backup)
	if err := os.Symlink(external, backup); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteConfirmed("claude", "p"); err == nil {
		t.Fatal("expected delete commit failure")
	}
	info, err := os.Lstat(p.Dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("profile directory was not rolled back safely: %v %v", info, err)
	}
	profiles, _, err := s.List("claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0].Alias != "p" {
		t.Fatalf("profile index changed despite failed delete: %+v", profiles)
	}
}

func TestCompletionScriptsExposeExpectedDynamicSources(t *testing.T) {
	for _, shell := range []string{"bash", "fish", "zsh"} {
		script, err := completionScript(shell)
		if err != nil {
			t.Fatalf("completion %s: %v", shell, err)
		}
		for _, want := range []string{"__complete tools", "__complete actions", "__complete profiles", "__complete templates"} {
			if !strings.Contains(script, want) {
				t.Fatalf("%s completion missing %q", shell, want)
			}
		}
	}
	if _, err := completionScript("unsupported"); err == nil {
		t.Fatal("unsupported shell should fail")
	}
}

func TestApplyStatuslineRejectsCodex(t *testing.T) {
	s, _ := testService(t)
	if _, err := s.Create("codex", "p"); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyStatusline("codex", "p", "default"); err == nil || !strings.Contains(err.Error(), "Claude") {
		t.Fatalf("expected Claude-only error, got %v", err)
	}
}
