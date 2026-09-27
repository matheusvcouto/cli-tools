package aiprofile

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/matheusvcouto/cli-tools/v2/internal/safefs"
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

// Regression: a second ai-profile process may change the alias while a user
// is reading the interactive deletion confirmation. It must never delete the
// replacement profile (or the renamed original).
func TestDeleteConfirmedProfileRejectsReusedAlias(t *testing.T) {
	s, _ := testService(t)
	selected, err := s.Create("grok", "victim")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Rename("grok", "victim", "preserved"); err != nil {
		t.Fatal(err)
	}
	replacement, err := s.Create("grok", "victim")
	if err != nil {
		t.Fatal(err)
	}
	if selected.Dir == replacement.Dir {
		t.Fatal("profile allocator reused directory")
	}
	if _, err := s.DeleteConfirmedProfile(selected); err == nil || !strings.Contains(err.Error(), "changed after deletion confirmation") {
		t.Fatalf("reused alias was not rejected: %v", err)
	}
	for _, p := range []Profile{selected, replacement} {
		if _, err := os.Stat(filepath.Join(p.Dir, "config.toml")); err != nil {
			t.Fatalf("deletion touched profile %q: %v", p.Dir, err)
		}
	}
	profiles, _, err := s.List("grok")
	if err != nil || len(profiles) != 2 {
		t.Fatalf("expected both profiles to survive: profiles=%+v err=%v", profiles, err)
	}
}

func TestDeleteConfirmedProfileRejectsChangedMetadata(t *testing.T) {
	s, _ := testService(t)
	selected, err := s.Create("claude", "victim")
	if err != nil {
		t.Fatal(err)
	}
	selected.CreatedAt = "1980-01-01T00:00:00Z"
	if _, err := s.DeleteConfirmedProfile(selected); err == nil || !strings.Contains(err.Error(), "changed after deletion confirmation") {
		t.Fatalf("stale profile was not rejected: %v", err)
	}
	if _, err := os.Stat(selected.Dir); err != nil {
		t.Fatalf("stale deletion touched live profile: %v", err)
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

func TestOversizedProfileIndexFailsClosed(t *testing.T) {
	s, _ := testService(t)
	if err := s.Store.EnsureRoot(); err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(s.Store.Root, "index.json")
	f, err := os.OpenFile(index, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	// Sparse file: the test does not allocate or write a large fixture.
	truncateErr := f.Truncate(maxProfileMetadataBytes + 1)
	closeErr := f.Close()
	if truncateErr != nil {
		t.Fatal(truncateErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if _, _, err := s.List("grok"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized index was accepted by List: %v", err)
	}
	if _, err := s.Create("grok", "must-not-be-created"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized index was accepted by Create: %v", err)
	}
	if _, err := os.Stat(index); err != nil {
		t.Fatalf("corrupt index was overwritten or removed: %v", err)
	}
}

// Regressions: a malformed index must never let deleting one alias remove
// another profile's directory, even when the aliases name different tools.
func TestStoreRejectsAliasedProfileDirectory(t *testing.T) {
	s, _ := testService(t)
	original, err := s.Create("grok", "primary")
	if err != nil {
		t.Fatal(err)
	}
	err = s.Store.Update(func(data *StoreData) error {
		data.Profiles = append(data.Profiles, Profile{
			Tool: "codex", Alias: "same-dir", Dir: original.Dir,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		})
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "reuses directory") {
		t.Fatalf("expected duplicate physical directory to fail closed: %v", err)
	}
	profiles, _, err := s.List("grok")
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0].Alias != "primary" {
		t.Fatalf("store was overwritten after invalid mutation: %+v", profiles)
	}
	if _, err := os.Stat(filepath.Join(original.Dir, "config.toml")); err != nil {
		t.Fatalf("original profile contents lost: %v", err)
	}
}

// The same check applies when the index was edited outside Store.Update:
// loading and destructive operations must refuse an ambiguous directory.
func TestDuplicateDirectoryInIndexBlocksDelete(t *testing.T) {
	s, _ := testService(t)
	original, err := s.Create("grok", "keep")
	if err != nil {
		t.Fatal(err)
	}
	data, err := s.Store.Load()
	if err != nil {
		t.Fatal(err)
	}
	data.Profiles = append(data.Profiles, Profile{
		Tool: "claude", Alias: "collision", Dir: original.Dir,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Store.Root, "index.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.List("grok"); err == nil || !strings.Contains(err.Error(), "reuses directory") {
		t.Fatalf("corrupt duplicate directory index was accepted: %v", err)
	}
	if _, err := s.DeleteConfirmed("grok", "keep"); err == nil {
		t.Fatal("delete accepted an index with overlapping directory ownership")
	}
	if _, err := os.Stat(filepath.Join(original.Dir, "config.toml")); err != nil {
		t.Fatalf("delete damaged original profile despite invalid index: %v", err)
	}
}

func TestBoundedMetadataReadRejectsGrowthBeyondLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.json")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	// Exercise the read-time limit independently of the initial Lstat check.
	if err := f.Truncate(maxProfileMetadataBytes + 1); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := readBoundedProfileMetadata(f, path); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("unbounded metadata read: %v", err)
	}
}

// If writes are not bounded as well, the app can generate a valid JSON index
// that its own bounded read rejects on the very next operation.
func TestOversizedGeneratedIndexIsNotCommitted(t *testing.T) {
	s, _ := testService(t)
	created, err := s.Create("grok", "primary")
	if err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(s.Store.Root, "index.json")
	before, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Store.Update(func(data *StoreData) error {
		data.Profiles[0].CreatedAt = strings.Repeat("z", int(maxProfileMetadataBytes))
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("accepted index that cannot be read later: %v", err)
	}
	after, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("oversized update modified the existing index")
	}
	profiles, _, err := s.List("grok")
	if err != nil || len(profiles) != 1 || profiles[0].Dir != created.Dir {
		t.Fatalf("original profile not preserved: %v / %+v", err, profiles)
	}
}

func TestOversizedMetadataWritePreservesExistingFile(t *testing.T) {
	s, _ := testService(t)
	if err := s.Store.EnsureRoot(); err != nil {
		t.Fatal(err)
	}
	root, err := safefs.Open(s.Store.Root)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := writeBoundedProfileMetadataRoot(root, "settings.json", []byte("original"), 0o600, ".settings-"); err != nil {
		t.Fatal(err)
	}
	if err := writeBoundedProfileMetadataRoot(root, "settings.json", make([]byte, maxProfileMetadataBytes+1), 0o600, ".settings-"); err == nil {
		t.Fatal("oversized metadata write was allowed")
	}
	got, err := os.ReadFile(filepath.Join(s.Store.Root, "settings.json"))
	if err != nil || string(got) != "original" {
		t.Fatalf("existing metadata was overwritten: %v / %q", err, got)
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

type caseInsensitiveRunner struct {
	*fakeRunner
}

func (r *caseInsensitiveRunner) NormalizeEnvKey(key string) string { return strings.ToUpper(key) }

func TestRunUsesPlatformEnvironmentKeySemantics(t *testing.T) {
	s, baseRunner := testService(t)
	runner := &caseInsensitiveRunner{fakeRunner: baseRunner}
	s.Runner = runner
	p, err := s.Create("codex", "windows-env")
	if err != nil {
		t.Fatal(err)
	}
	s.Env = func() []string {
		return []string{
			"Path=C:\\Windows\\System32",
			"openai_api_key=must-not-leak",
			"CoDeX_HoMe=must-be-replaced",
			"KEEP=yes",
		}
	}
	if err := s.Run(context.Background(), "codex", "windows-env", nil, ProcessIO{}); err != nil {
		t.Fatal(err)
	}
	for _, item := range baseRunner.env {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		switch strings.ToUpper(key) {
		case "OPENAI_API_KEY":
			t.Fatalf("case-variant credential leaked: %q", item)
		case "CODEX_HOME":
			if key != "CODEX_HOME" || value != p.Dir {
				t.Fatalf("profile root was not canonical replacement: %q", item)
			}
		}
	}
}

func TestGrokCreateRunAndACPUseIsolatedProfile(t *testing.T) {
	s, r := testService(t)
	p, err := s.Create("grok", "work")
	if err != nil {
		t.Fatal(err)
	}
	if p.Tool != "grok" || p.Alias != "work" {
		t.Fatalf("unexpected Grok profile: %+v", p)
	}
	configPath := filepath.Join(p.Dir, "config.toml")
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[compat.claude]", "[compat.cursor]", "[compat.codex]", "auto_update = false", "sessions = false", "skills = false", "hooks = false"} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("initial Grok config missing %q:\n%s", want, raw)
		}
	}

	s.Env = func() []string {
		env := []string{
			"PATH=/synthetic/bin",
			"KEEP=yes",
			"GROK_SANDBOX=strict",
			"GROK_REQUIRED_MINIMUM_VERSION=1.0.0",
			"GROK_DISABLE_API_KEY_AUTH=true",
			"GROK_FORCE_LOGIN_TEAM_ID=team-from-admin",
			"GROK_TELEMETRY_ENABLED=0",
		}
		for _, key := range grokClearEnv {
			env = append(env, key+"=must-not-leak")
		}
		return env
	}
	args := []string{"-p", "hello", "--output-format", "json"}
	if err := s.Run(context.Background(), "grok", "work", args, ProcessIO{}); err != nil {
		t.Fatal(err)
	}
	if r.binary != "grok" || strings.Join(r.args, "|") != strings.Join(args, "|") {
		t.Fatalf("bad Grok run: binary=%q args=%#v", r.binary, r.args)
	}
	env := envMap(r.env)
	if env["GROK_HOME"] != p.Dir {
		t.Fatalf("GROK_HOME=%q want %q", env["GROK_HOME"], p.Dir)
	}
	for _, key := range grokClearEnv {
		if _, ok := env[key]; ok {
			t.Fatalf("Grok isolation leaked %s", key)
		}
	}
	if env["GROK_SANDBOX"] != "strict" || env["GROK_REQUIRED_MINIMUM_VERSION"] != "1.0.0" || env["GROK_TELEMETRY_ENABLED"] != "0" || env["GROK_DISABLE_API_KEY_AUTH"] != "true" || env["GROK_FORCE_LOGIN_TEAM_ID"] != "team-from-admin" {
		t.Fatalf("security/policy variables were unexpectedly removed: %#v", env)
	}
	if env["GROK_DISABLE_AUTOUPDATER"] != "1" {
		t.Fatalf("auto updater was not disabled: %#v", env)
	}
	for _, family := range []string{"CLAUDE", "CURSOR"} {
		for _, feature := range []string{"SKILLS", "RULES", "AGENTS", "MCPS", "HOOKS", "SESSIONS"} {
			key := "GROK_" + family + "_" + feature + "_ENABLED"
			if _, inherited := env[key]; inherited {
				t.Fatalf("inherited Grok compatibility override leaked: %s", key)
			}
		}
	}

	if err := s.ACP(context.Background(), "grok", "work", []string{"--model", "grok-4.7"}, ProcessIO{}); err != nil {
		t.Fatal(err)
	}
	if r.binary != "grok" || strings.Join(r.args, "|") != "agent|--model|grok-4.7|stdio" {
		t.Fatalf("bad Grok ACP launch: binary=%q args=%#v", r.binary, r.args)
	}
}

// Regression: inherited shell overrides must not make an isolated profile use
// a different account, but a deliberate edit to its own config.toml must be
// respected (no wrapper-injected compatibility variables win over the file).
func TestGrokProfileCanOptIntoCompatibilityLocally(t *testing.T) {
	s, r := testService(t)
	p, err := s.Create("grok", "work")
	if err != nil {
		t.Fatal(err)
	}
	custom := []byte("[cli]\nauto_update = false\n[compat.claude]\nskills = true\n")
	if err := os.WriteFile(filepath.Join(p.Dir, "config.toml"), custom, 0o600); err != nil {
		t.Fatal(err)
	}
	s.Env = func() []string {
		return []string{
			"GROK_HOME=/unselected/account", "XAI_API_KEY=other-account",
			"GROK_AUTH_PATH=/other/auth.json", "GROK_AUTH=inline-other-account",
			"GROK_AUTH_JSON=inline-json", "GROK_CODE_XAI_API_KEY=old-key",
			"GROK_FEEDBACK_BASE_URL=https://other.example/collector",
			"GROK_TRACE_UPLOAD_REGION=other-region",
			"GROK_CLAUDE_SKILLS_ENABLED=false", "GROK_CURSOR_HOOKS_ENABLED=true",
			"GROK_SANDBOX=strict", "GROK_REQUIRED_MINIMUM_VERSION=1.0.0",
			"GROK_DISABLE_API_KEY_AUTH=true", "GROK_FORCE_LOGIN_TEAM_ID=admin-team",
		}
	}
	if err := s.Run(context.Background(), "grok", "work", nil, ProcessIO{}); err != nil {
		t.Fatal(err)
	}
	child := envMap(r.env)
	if child["GROK_HOME"] != p.Dir {
		t.Fatalf("GROK_HOME=%q", child["GROK_HOME"])
	}
	for _, key := range []string{
		"XAI_API_KEY", "GROK_AUTH_PATH", "GROK_AUTH", "GROK_AUTH_JSON",
		"GROK_CODE_XAI_API_KEY", "GROK_CLAUDE_SKILLS_ENABLED", "GROK_CURSOR_HOOKS_ENABLED",
		"GROK_FEEDBACK_BASE_URL", "GROK_TRACE_UPLOAD_REGION",
	} {
		if _, ok := child[key]; ok {
			t.Fatalf("inherited override leaked %s", key)
		}
	}
	if child["GROK_SANDBOX"] != "strict" || child["GROK_REQUIRED_MINIMUM_VERSION"] != "1.0.0" || child["GROK_DISABLE_API_KEY_AUTH"] != "true" || child["GROK_FORCE_LOGIN_TEAM_ID"] != "admin-team" {
		t.Fatalf("security controls altered: %#v", child)
	}
	actual, err := os.ReadFile(filepath.Join(p.Dir, "config.toml"))
	if err != nil || string(actual) != string(custom) {
		t.Fatalf("user-owned config changed: %q (%v)", actual, err)
	}
}

func TestGrokWindowsEnvironmentNamesAreCaseInsensitive(t *testing.T) {
	// This tests the shared isolation transform with the Windows normalizer;
	// the native Windows runner and its integration gate are separate.
	spec, ok := LookupTool("grok")
	if !ok {
		t.Fatal("Grok tool missing")
	}
	base := []string{
		"grok_auth_path=C:\\other\\auth.json", "xai_api_key=other",
		"grok_claude_skills_enabled=true", "gRoK_hOmE=C:\\other",
		"GROK_SANDBOX=strict", "KEEP=yes",
	}
	child := isolatedEnv(base, profileEnvironment(spec, `C:\profiles\grok`), spec.ClearEnv, strings.ToUpper)
	seen := make(map[string]string)
	for _, item := range child {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		upper := strings.ToUpper(key)
		if _, duplicate := seen[upper]; duplicate {
			t.Fatalf("duplicate environment key: %s", upper)
		}
		seen[upper] = value
	}
	if seen["GROK_HOME"] != `C:\profiles\grok` || seen["GROK_SANDBOX"] != "strict" || seen["KEEP"] != "yes" {
		t.Fatalf("bad preserved/replaced env: %#v", seen)
	}
	for _, key := range []string{"GROK_AUTH_PATH", "XAI_API_KEY", "GROK_CLAUDE_SKILLS_ENABLED"} {
		if _, ok := seen[key]; ok {
			t.Fatalf("case-variant leaked %s", key)
		}
	}
}

func TestGrokProfileConfigIsUserOwnedAndRequired(t *testing.T) {
	s, r := testService(t)
	p, err := s.Create("grok", "work")
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(p.Dir, "config.toml")
	custom := []byte("[cli]\nauto_update = true\n# user-owned\n")
	if err := os.WriteFile(configPath, custom, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(context.Background(), "grok", "work", nil, ProcessIO{}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(custom) {
		t.Fatalf("Grok preparation rewrote user config:\n%s", after)
	}
	if r.calls != 1 {
		t.Fatalf("Grok process calls=%d want 1", r.calls)
	}

	if err := os.Remove(configPath); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(context.Background(), "grok", "work", nil, ProcessIO{}); err == nil || !strings.Contains(err.Error(), "no config.toml") {
		t.Fatalf("expected missing config to fail closed, got %v", err)
	}
	if r.calls != 1 {
		t.Fatalf("process ran despite missing config: calls=%d", r.calls)
	}
}

func TestGrokRunRejectsSymlinkedConfig(t *testing.T) {
	s, r := testService(t)
	p, err := s.Create("grok", "work")
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(p.Dir, "config.toml")
	if err := os.Remove(configPath); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "outside.toml")
	if err := os.WriteFile(external, []byte("[cli]\nauto_update = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, configPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := s.Run(context.Background(), "grok", "work", nil, ProcessIO{}); err == nil || !strings.Contains(err.Error(), "real regular file") {
		t.Fatalf("expected symlinked config rejection, got %v", err)
	}
	if r.calls != 0 {
		t.Fatalf("process ran despite unsafe Grok config: %d calls", r.calls)
	}
}

func TestProviderInitializationFailureRollsBackNewProfile(t *testing.T) {
	s, _ := testService(t)
	_, err := s.Store.createProfile("grok", "broken", s.Now(), func(_ *safefs.Root, _, _ string) error {
		return errors.New("synthetic initialization failure")
	})
	if err == nil || !strings.Contains(err.Error(), "synthetic initialization failure") {
		t.Fatalf("expected initialization failure, got %v", err)
	}
	entries, readErr := os.ReadDir(s.Store.Root)
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "grok-") {
			t.Fatalf("failed initialization leaked profile directory %q", entry.Name())
		}
	}
	data, loadErr := s.Store.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if _, ok := FindProfile(data, "grok", "broken"); ok {
		t.Fatal("failed initialization was committed to index")
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

// A legal JSON null is not a legal Claude settings document. It used to set
// the settings map to nil, leading to a panic when adding claudeMdExcludes.
func TestClaudeRunRejectsNullSettingsWithoutPanic(t *testing.T) {
	s, runner := testService(t)
	p, err := s.Create("claude", "null-settings")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(p.Dir, "settings.json")
	if err := os.WriteFile(path, []byte("null\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(context.Background(), "claude", p.Alias, nil, ProcessIO{}); err == nil || !strings.Contains(err.Error(), "JSON object") {
		t.Fatalf("expected malformed-settings error (not panic): %v", err)
	}
	if runner.calls != 0 {
		t.Fatal("launched Claude with invalid settings")
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "null\n" {
		t.Fatalf("existing invalid settings were overwritten: %q / %v", raw, err)
	}
}

func TestMissingIndexWithBackupFailsClosed(t *testing.T) {
	s, _ := testService(t)
	first, err := s.Create("grok", "first")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create("codex", "second"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(s.Store.Root, "index.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Store.Load(); err == nil || !strings.Contains(err.Error(), "index.json.bak") {
		t.Fatalf("store discarded backup after losing index: %v", err)
	}
	if _, err := s.Create("grok", "third"); err == nil {
		t.Fatal("create must not start a new index while backup exists")
	}
	if _, err := os.Stat(filepath.Join(first.Dir, "config.toml")); err != nil {
		t.Fatalf("unindexed Grok credentials were damaged: %v", err)
	}
}

func TestMissingIndexWithManagedProfileDirFailsClosed(t *testing.T) {
	s, _ := testService(t)
	first, err := s.Create("grok", "first")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.Store.Root, "index.json.bak")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected first commit to have no backup, got %v", err)
	}
	if err := os.Remove(filepath.Join(s.Store.Root, "index.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Store.Load(); err == nil || !strings.Contains(err.Error(), "managed profile state") {
		t.Fatalf("store silently reset despite orphaned directory: %v", err)
	}
	if _, err := s.Create("grok", "first"); err == nil {
		t.Fatal("new profile reused an alias while orphaned credentials remain")
	}
	if _, err := os.Stat(filepath.Join(first.Dir, "config.toml")); err != nil {
		t.Fatalf("orphaned credentials unexpectedly deleted: %v", err)
	}
}

func TestStoreWithoutIndexOrManagedStateIsEmpty(t *testing.T) {
	s, _ := testService(t)
	if err := s.Store.EnsureRoot(); err != nil {
		t.Fatal(err)
	}
	profiles, _, err := s.List("grok")
	if err != nil || len(profiles) != 0 {
		t.Fatalf("empty store was not accepted: %v / %+v", err, profiles)
	}
	if _, err := s.Create("grok", "first"); err != nil {
		t.Fatalf("fresh store could not create profile: %v", err)
	}
}

// The old NUON migration accepts existing directories with arbitrary leaf
// names (e.g. claude-id). If the first JSON index disappears before a second
// commit, there is no backup: recognizing only generated directory names would
// silently interpret these credentials as an entirely new empty store.
func TestMissingIndexWithLegacyOrResidualEntryFailsClosed(t *testing.T) {
	for _, name := range []string{"claude-id", "custom-profile", ".deleted-aabbcc", ".index-deadbeef.tmp"} {
		t.Run(name, func(t *testing.T) {
			s, _ := testService(t)
			if err := s.Store.EnsureRoot(); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(s.Store.Root, name), 0o700); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Store.Load(); err == nil || !strings.Contains(err.Error(), "residual entry") {
				t.Fatalf("missing index with %q residual entry was accepted: %v", name, err)
			}
			if _, err := s.Create("claude", "new"); err == nil {
				t.Fatal("created a fresh index over unreferenced credentials")
			}
			if _, err := os.Stat(filepath.Join(s.Store.Root, name)); err != nil {
				t.Fatalf("residual entry was removed: %v", err)
			}
		})
	}
}

func TestMissingIndexWithOnlyLockIsEmpty(t *testing.T) {
	s, _ := testService(t)
	if err := s.Store.EnsureRoot(); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(s.Store.Root, ".index.lock")
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if data, err := s.Store.Load(); err != nil || len(data.Profiles) != 0 {
		t.Fatalf("legitimate initial lock was refused: %+v / %v", data, err)
	}
	if _, err := s.Create("claude", "first"); err != nil {
		t.Fatalf("fresh index after failed first transaction: %v", err)
	}
}

func TestMissingIndexRejectsNonRegularLock(t *testing.T) {
	s, _ := testService(t)
	if err := s.Store.EnsureRoot(); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(s.Store.Root, ".index.lock"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Store.Load(); err == nil || !strings.Contains(err.Error(), "lock is not a regular file") {
		t.Fatalf("unsafe lock path was mistaken for empty store: %v", err)
	}
}

func TestLegacyImportRequiresExactRealDirectories(t *testing.T) {
	for _, test := range []struct {
		name     string
		prepare  func(string) error
		contains string
	}{
		{"absent-directory", func(string) error { return nil }, "inspect legacy profile"},
		{"file-instead-of-directory", func(p string) error { return os.WriteFile(p, []byte("not a directory"), 0o600) }, "not a real directory"},
		{"unreferenced-entry", func(p string) error {
			if err := os.Mkdir(p, 0o700); err != nil {
				return err
			}
			return os.Mkdir(filepath.Join(filepath.Dir(p), "unindexed-credentials"), 0o700)
		}, "unreferenced root entry"},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, _ := testService(t)
			if err := s.Store.EnsureRoot(); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(s.Store.Root, "claude-id")
			if err := test.prepare(dir); err != nil {
				t.Fatal(err)
			}
			err := s.Store.ImportLegacy([]Profile{{Tool: "claude", Alias: "old", Dir: dir, CreatedAt: "2026-01-01T00:00:00Z"}}, "", nil)
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("legacy import accepted unsafe root: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(s.Store.Root, "index.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("legacy import incorrectly committed index: %v", err)
			}
		})
	}
}

func TestLegacyImportRejectsExistingBackupWithoutIndex(t *testing.T) {
	s, _ := testService(t)
	if err := s.Store.EnsureRoot(); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(s.Store.Root, "claude-id")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(s.Store.Root, "index.json.bak")
	if err := os.WriteFile(backup, []byte("previous state"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := s.Store.ImportLegacy([]Profile{{Tool: "claude", Alias: "old", Dir: dir, CreatedAt: "2026-01-01T00:00:00Z"}}, "", nil)
	if err == nil || !strings.Contains(err.Error(), "index.json.bak already exists") {
		t.Fatalf("legacy import overwrote backup: %v", err)
	}
	got, err := os.ReadFile(backup)
	if err != nil || string(got) != "previous state" {
		t.Fatalf("backup not preserved: %q / %v", got, err)
	}
}

func TestManagedProfileDirectoryNameRecognition(t *testing.T) {
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"grok-20260926120159-0123abcd", true},
		{"claude-20260926120159-0123abcd", true},
		{"codex-20260926120159-0123abcd", true},
		{"grok-not-a-generated-directory", false},
		{"grok-20260926120159-0123abcd-extra", false},
		{"grok-20260926120159-0123abcg", false},
		{".deleted-0123abcd", false},
	} {
		if got := looksLikeManagedProfileDir(tc.name); got != tc.want {
			t.Errorf("looksLikeManagedProfileDir(%q)=%v want %v", tc.name, got, tc.want)
		}
	}
}

func TestLegacyImportRefusesChangedSource(t *testing.T) {
	s, _ := testService(t)
	if err := s.Store.EnsureRoot(); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(s.Store.Root, "claude-id")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(s.Store.Root, "index.nuon")
	if err := os.WriteFile(source, []byte("new source contents"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := Profile{Tool: "claude", Alias: "old", Dir: dir, CreatedAt: "2026-01-01T00:00:00Z"}
	err := s.Store.ImportLegacy([]Profile{profile}, source, []byte("outdated source"))
	if err == nil || !strings.Contains(err.Error(), "changed since parsing") {
		t.Fatalf("stale legacy import committed: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(s.Store.Root, "index.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("created index from stale source: %v", err)
	}
}
