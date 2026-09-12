//go:build darwin || linux

package integration

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/matheusvcouto/cli-tools/internal/testenv"
)

type fakeReport struct {
	Executable         string   `json:"executable"`
	Args               []string `json:"args"`
	CodexHome          string   `json:"codex_home"`
	ClaudeConfigDir    string   `json:"claude_config_dir"`
	APIKey             string   `json:"api_key"`
	OpenAIBaseURL      string   `json:"openai_base_url"`
	CodexAPIKey        string   `json:"codex_api_key"`
	CodexAccessToken   string   `json:"codex_access_token"`
	CodexSQLiteHome    string   `json:"codex_sqlite_home"`
	AnthropicAPIKey    string   `json:"anthropic_api_key"`
	AnthropicAuthToken string   `json:"anthropic_auth_token"`
	AnthropicProfile   string   `json:"anthropic_profile"`
	ClaudeUseBedrock   string   `json:"claude_use_bedrock"`
	FoundryAuthToken   string   `json:"foundry_auth_token"`
	AnthropicHeaders   string   `json:"anthropic_custom_headers"`
	SecureStorageDir   string   `json:"secure_storage_dir"`
	PluginCacheDir     string   `json:"plugin_cache_dir"`
	ProfileGuidance    string   `json:"profile_guidance"`
	ProjectGuidance    string   `json:"project_guidance"`
	CWD                string   `json:"cwd"`
}

func TestCLIEndToEndWithSyntheticState(t *testing.T) {
	root := moduleRoot(t)
	binDir := t.TempDir()
	aiBin := filepath.Join(binDir, "ai-profile")
	repoZipBin := filepath.Join(binDir, "repo-zip")
	buildGo(t, root, aiBin, "./cmd/ai-profile")
	buildGo(t, root, repoZipBin, "./cmd/repo-zip")
	buildFakeAI(t, binDir)

	sandbox := t.TempDir()
	baseEnv := safeTestEnvAt(t, sandbox)
	home := filepath.Join(sandbox, "home")
	profileRoot := filepath.Join(home, "profiles")
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude", "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(home, ".codex", "AGENTS.md"), "default codex guidance must not leak")
	write(t, filepath.Join(home, ".claude", "CLAUDE.md"), "default claude guidance must not leak")
	write(t, filepath.Join(home, ".claude", "rules", "default.md"), "default rule must not leak")

	project := filepath.Join(sandbox, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(project, "AGENTS.md"), "project codex guidance")
	write(t, filepath.Join(project, "CLAUDE.md"), "project claude guidance")

	baseEnv = setEnv(baseEnv, "AI_PROFILE_ROOT", profileRoot)
	baseEnv = setEnv(baseEnv, "PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for key, value := range map[string]string{
		"OPENAI_API_KEY":                  "must-not-reach-child",
		"OPENAI_BASE_URL":                 "https://wrong.invalid",
		"CODEX_API_KEY":                   "must-not-reach-child",
		"CODEX_ACCESS_TOKEN":              "must-not-reach-child",
		"CODEX_SQLITE_HOME":               "/wrong/sqlite",
		"ANTHROPIC_API_KEY":               "must-not-reach-child",
		"ANTHROPIC_AUTH_TOKEN":            "must-not-reach-child",
		"CLAUDE_CODE_OAUTH_TOKEN":         "must-not-reach-child",
		"ANTHROPIC_PROFILE":               "wrong-profile",
		"CLAUDE_CODE_USE_BEDROCK":         "1",
		"ANTHROPIC_FOUNDRY_AUTH_TOKEN":    "must-not-reach-child",
		"ANTHROPIC_CUSTOM_HEADERS":        "Authorization: Bearer wrong",
		"CLAUDE_SECURESTORAGE_CONFIG_DIR": "/wrong/secure",
		"CLAUDE_CODE_PLUGIN_CACHE_DIR":    "/wrong/plugins",
	} {
		baseEnv = setEnv(baseEnv, key, value)
	}

	codexDir := createAndResolveProfile(t, aiBin, baseEnv, "codex", "work")
	claudeDir := createAndResolveProfile(t, aiBin, baseEnv, "claude", "work")
	write(t, filepath.Join(codexDir, "AGENTS.md"), "profile codex guidance")
	write(t, filepath.Join(claudeDir, "CLAUDE.md"), "profile claude guidance")

	for _, tc := range []struct {
		tool       string
		action     string
		wantExe    string
		wantCode   int
		profileDir string
		args       []string
	}{
		{tool: "codex", action: "run", wantExe: "codex", wantCode: 17, profileDir: codexDir, args: []string{"--alpha", "two words"}},
		{tool: "codex", action: "acp", wantExe: "codex-acp", wantCode: 23, profileDir: codexDir, args: []string{"--beta", "three words"}},
		{tool: "claude", action: "run", wantExe: "claude", wantCode: 17, profileDir: claudeDir, args: []string{"--alpha", "two words"}},
		{tool: "claude", action: "acp", wantExe: "claude-agent-acp", wantCode: 23, profileDir: claudeDir, args: []string{"--beta", "three words"}},
	} {
		t.Run(tc.tool+"-"+tc.action, func(t *testing.T) {
			argv := []string{tc.tool, tc.action, "work"}
			argv = append(argv, tc.args...)
			report, stderr, code := runExpectExitAt(t, project, baseEnv, aiBin, tc.wantCode, argv...)
			if stderr != "" {
				t.Fatalf("wrapper wrote stderr: %q", stderr)
			}
			if code != tc.wantCode || report.Executable != tc.wantExe {
				t.Fatalf("unexpected report/code: %#v code=%d", report, code)
			}
			assertFakeReport(t, tc.tool, report, tc.profileDir, project, tc.args)
		})
	}

	// Claude isolation preparation preserves its own context while excluding the
	// default ~/.claude memory/rules path as defense in depth.
	raw, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		filepath.Join(home, ".claude", "CLAUDE.md"),
		filepath.Join(home, ".claude", "CLAUDE.local.md"),
		filepath.Join(home, ".claude", "rules", "**"),
	} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("Claude settings missing context exclusion %q: %s", want, raw)
		}
	}

	testRepoZipBinary(t, repoZipBin)
	testRepoZipWorktreeBundleBinary(t, repoZipBin)
}

func createAndResolveProfile(t *testing.T, aiBin string, env []string, tool, alias string) string {
	t.Helper()
	cmd := exec.Command(aiBin, tool, "new", alias)
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create %s profile: %v\n%s", tool, err, out)
	}
	cmd = exec.Command(aiBin, tool, "list", "--json")
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var listed []struct {
		Profile string `json:"profile"`
		Dir     string `json:"dir"`
	}
	if err := json.Unmarshal(out, &listed); err != nil {
		t.Fatalf("decode %s list: %v\n%s", tool, err, out)
	}
	if len(listed) != 1 || listed[0].Profile != alias || listed[0].Dir == "" {
		t.Fatalf("unexpected %s list result: %#v", tool, listed)
	}
	return listed[0].Dir
}

func assertFakeReport(t *testing.T, tool string, report fakeReport, wantHome, wantCWD string, wantArgs []string) {
	t.Helper()
	if report.CWD != wantCWD {
		t.Fatalf("cwd=%q want %q", report.CWD, wantCWD)
	}
	if strings.Join(report.Args, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("argv mismatch: got %#v want %#v", report.Args, wantArgs)
	}
	switch tool {
	case "codex":
		if report.APIKey != "" || report.OpenAIBaseURL != "" || report.CodexAPIKey != "" || report.CodexAccessToken != "" || report.CodexSQLiteHome != "" {
			t.Fatalf("Codex authentication override leaked: %#v", report)
		}
		if report.CodexHome != wantHome || report.ClaudeConfigDir != "" {
			t.Fatalf("bad Codex profile env: %#v", report)
		}
		if report.ProfileGuidance != "profile codex guidance" || report.ProjectGuidance != "project codex guidance" {
			t.Fatalf("bad Codex context: %#v", report)
		}
	case "claude":
		if report.AnthropicAPIKey != "" || report.AnthropicAuthToken != "" || report.AnthropicProfile != "" || report.ClaudeUseBedrock != "" || report.FoundryAuthToken != "" || report.AnthropicHeaders != "" || report.SecureStorageDir != "" || report.PluginCacheDir != "" {
			t.Fatalf("Claude authentication/config redirect leaked: %#v", report)
		}
		if report.ClaudeConfigDir != wantHome || report.CodexHome != "" {
			t.Fatalf("bad Claude profile env: %#v", report)
		}
		if report.ProfileGuidance != "profile claude guidance" || report.ProjectGuidance != "project claude guidance" {
			t.Fatalf("bad Claude context: %#v", report)
		}
	default:
		t.Fatalf("unknown tool %q", tool)
	}
}

func runExpectExitAt(t *testing.T, dir string, env []string, bin string, wantCode int, args ...string) (fakeReport, string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("expected exit %d, got err=%v stdout=%q stderr=%q", wantCode, err, stdout.String(), stderr.String())
	}
	if exit.ExitCode() != wantCode {
		t.Fatalf("exit=%d want=%d stdout=%q stderr=%q", exit.ExitCode(), wantCode, stdout.String(), stderr.String())
	}
	var report fakeReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode fake output: %v\n%s", err, stdout.String())
	}
	return report, stderr.String(), exit.ExitCode()
}

func testRepoZipBinary(t *testing.T, repoZipBin string) {
	t.Helper()
	env := safeTestEnv(t)
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, env, repo, "init", "-q")
	git(t, env, repo, "config", "user.name", "Synthetic User")
	git(t, env, repo, "config", "user.email", "synthetic@example.invalid")
	write(t, filepath.Join(repo, ".gitignore"), "ignored.log\n")
	write(t, filepath.Join(repo, "tracked.txt"), "tracked")
	write(t, filepath.Join(repo, "untracked.txt"), "untracked")
	write(t, filepath.Join(repo, "ignored.log"), "ignored")
	outside := filepath.Join(t.TempDir(), "outside.txt")
	write(t, outside, "outside secret")
	if err := os.Symlink(outside, filepath.Join(repo, "external-link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	git(t, env, repo, "add", ".gitignore", "tracked.txt", "external-link")

	outZip := filepath.Join(repo, "snapshot.zip")
	cmd := exec.Command(repoZipBin, repo, "--output", outZip)
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("repo-zip: %v\n%s", err, out)
	}
	zr, err := zip.OpenReader(outZip)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	names := map[string]*zip.File{}
	for _, f := range zr.File {
		names[f.Name] = f
	}
	for _, want := range []string{".gitignore", "tracked.txt", "untracked.txt", "external-link"} {
		if names[want] == nil {
			t.Fatalf("missing %q in archive", want)
		}
	}
	if names["ignored.log"] != nil {
		t.Fatal("ignored file was archived")
	}
	if names["external-link"].Mode()&os.ModeSymlink == 0 {
		t.Fatal("symlink was not preserved as a symlink")
	}
	r, err := names["external-link"].Open()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != outside {
		t.Fatalf("symlink payload=%q want target path %q", raw, outside)
	}
	for name := range names {
		if strings.HasPrefix(name, ".git/") {
			t.Fatalf(".git unexpectedly included: %s", name)
		}
	}
}

func testRepoZipWorktreeBundleBinary(t *testing.T, repoZipBin string) {
	t.Helper()
	env := safeTestEnv(t)
	root := t.TempDir()
	mainRepo := filepath.Join(root, "main")
	if err := os.Mkdir(mainRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, env, mainRepo, "init", "-q")
	git(t, env, mainRepo, "config", "user.name", "Synthetic User")
	git(t, env, mainRepo, "config", "user.email", "synthetic@example.invalid")
	write(t, filepath.Join(mainRepo, ".gitignore"), "ignored.log\n")
	write(t, filepath.Join(mainRepo, "tracked.txt"), "main")
	git(t, env, mainRepo, "add", ".gitignore", "tracked.txt")
	git(t, env, mainRepo, "commit", "-qm", "initial")

	worktree := filepath.Join(root, "worktree")
	git(t, env, mainRepo, "worktree", "add", "-q", "-b", "feature", worktree)
	write(t, filepath.Join(worktree, "tracked.txt"), "dirty")
	write(t, filepath.Join(worktree, "untracked.txt"), "untracked")
	write(t, filepath.Join(worktree, "ignored.log"), "ignored")

	outZip := filepath.Join(t.TempDir(), "worktree.zip")
	cmd := exec.Command(repoZipBin, worktree, "--git", "--output", outZip)
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("repo-zip --git linked worktree: %v\n%s", err, out)
	}
	zr, err := zip.OpenReader(outZip)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var bundle *zip.File
	for _, f := range zr.File {
		switch {
		case f.Name == ".repo-zip/repository.bundle":
			bundle = f
		case f.Name == "ignored.log":
			t.Fatal("ignored worktree file was archived")
		case strings.HasPrefix(f.Name, ".git/") || f.Name == ".git":
			t.Fatalf("raw Git metadata was archived: %s", f.Name)
		}
	}
	if bundle == nil {
		t.Fatal("Git bundle missing from linked worktree archive")
	}
	r, err := bundle.Open()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatal(err)
	}
	bundlePath := filepath.Join(t.TempDir(), "repository.bundle")
	if err := os.WriteFile(bundlePath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, env, worktree, "bundle", "verify", bundlePath)
}

func buildFakeAI(t *testing.T, binDir string) {
	t.Helper()
	srcDir := t.TempDir()
	src := `package main
import (
  "encoding/json"
  "os"
  "path/filepath"
  "strings"
)
func main() {
  exe := filepath.Base(os.Args[0])
  profileRoot := os.Getenv("CODEX_HOME")
  profileFile := "AGENTS.md"
  projectFile := "AGENTS.md"
  if strings.HasPrefix(exe, "claude") {
    profileRoot = os.Getenv("CLAUDE_CONFIG_DIR")
    profileFile = "CLAUDE.md"
    projectFile = "CLAUDE.md"
  }
  profileGuidance, _ := os.ReadFile(filepath.Join(profileRoot, profileFile))
  cwd, _ := os.Getwd()
  projectGuidance, _ := os.ReadFile(filepath.Join(cwd, projectFile))
  _ = json.NewEncoder(os.Stdout).Encode(map[string]any{
    "executable": exe,
    "args": os.Args[1:],
    "codex_home": os.Getenv("CODEX_HOME"),
    "claude_config_dir": os.Getenv("CLAUDE_CONFIG_DIR"),
    "api_key": os.Getenv("OPENAI_API_KEY"),
    "openai_base_url": os.Getenv("OPENAI_BASE_URL"),
    "codex_api_key": os.Getenv("CODEX_API_KEY"),
    "codex_access_token": os.Getenv("CODEX_ACCESS_TOKEN"),
    "codex_sqlite_home": os.Getenv("CODEX_SQLITE_HOME"),
    "anthropic_api_key": os.Getenv("ANTHROPIC_API_KEY"),
    "anthropic_auth_token": os.Getenv("ANTHROPIC_AUTH_TOKEN"),
    "anthropic_profile": os.Getenv("ANTHROPIC_PROFILE"),
    "claude_use_bedrock": os.Getenv("CLAUDE_CODE_USE_BEDROCK"),
    "foundry_auth_token": os.Getenv("ANTHROPIC_FOUNDRY_AUTH_TOKEN"),
    "anthropic_custom_headers": os.Getenv("ANTHROPIC_CUSTOM_HEADERS"),
    "secure_storage_dir": os.Getenv("CLAUDE_SECURESTORAGE_CONFIG_DIR"),
    "plugin_cache_dir": os.Getenv("CLAUDE_CODE_PLUGIN_CACHE_DIR"),
    "profile_guidance": string(profileGuidance),
    "project_guidance": string(projectGuidance),
    "cwd": cwd,
  })
  if strings.Contains(exe, "acp") { os.Exit(23) }
  os.Exit(17)
}`
	if err := os.WriteFile(filepath.Join(srcDir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(binDir, "fake-ai")
	cmd := exec.Command("go", "build", "-o", base, "main.go")
	cmd.Dir = srcDir
	cmd.Env = safeTestEnv(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake AI: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"codex", "codex-acp", "claude", "claude-agent-acp"} {
		if err := os.WriteFile(filepath.Join(binDir, name), raw, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func buildGo(t *testing.T, root, out, pkg string) {
	t.Helper()
	cmd := exec.Command("go", "build", "-trimpath", "-o", out, pkg)
	cmd.Dir = root
	cmd.Env = safeTestEnv(t)
	if raw, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", pkg, err, raw)
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(filepath.Dir(file))
}

func safeTestEnv(t *testing.T) []string {
	t.Helper()
	return safeTestEnvAt(t, t.TempDir())
}

func safeTestEnvAt(t *testing.T, root string) []string {
	t.Helper()
	env, err := testenv.New(root)
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func unsetEnv(env []string, keys ...string) []string {
	blocked := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		blocked[key] = struct{}{}
	}
	out := make([]string, 0, len(env))
	for _, item := range env {
		key, _, ok := strings.Cut(item, "=")
		if ok {
			if _, drop := blocked[key]; drop {
				continue
			}
		}
		out = append(out, item)
	}
	return out
}

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			continue
		}
		out = append(out, item)
	}
	return append(out, prefix+value)
}

func git(t *testing.T, env []string, repo string, args ...string) {
	t.Helper()
	argv := append([]string{"-C", repo}, args...)
	cmd := exec.Command("git", argv...)
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAIProfileStaticCommandsDoNotRequireHomeOrStore(t *testing.T) {
	root := moduleRoot(t)
	bin := filepath.Join(t.TempDir(), "ai-profile")
	buildGo(t, root, bin, "./cmd/ai-profile")

	env := unsetEnv(safeTestEnv(t), "HOME", "USERPROFILE", "AI_PROFILE_ROOT")
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "version", args: []string{"--version"}},
		{name: "help", args: []string{"--help"}},
		{name: "completion", args: []string{"completion", "bash"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(bin, tc.args...)
			cmd.Env = env
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%v failed without HOME: %v\n%s", tc.args, err, out)
			}
		})
	}
}
