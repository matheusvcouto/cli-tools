//go:build darwin || linux || windows

package integration

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/aiprofile"
	profilecli "github.com/matheusvcouto/cli-tools/internal/aiprofile/cli"
	"github.com/matheusvcouto/cli-tools/internal/aiprofile/platform"
	"github.com/matheusvcouto/cli-tools/internal/repozip"
	repocli "github.com/matheusvcouto/cli-tools/internal/repozip/cli"
	"github.com/matheusvcouto/cli-tools/internal/testenv"
)

func platformExecutableName(name string) string {
	if os.PathSeparator == '\\' {
		return name + ".exe"
	}
	return name
}

func helperExecutableName(path string) string {
	name := filepath.Base(path)
	if strings.EqualFold(filepath.Ext(name), ".exe") {
		name = strings.TrimSuffix(name, filepath.Ext(name))
	}
	return name
}

func TestMain(m *testing.M) {
	switch helperExecutableName(os.Args[0]) {
	case "ai-profile":
		os.Exit(runAIProfileHelper(os.Args[1:]))
	case "repo-zip":
		os.Exit(runRepoZipHelper(os.Args[1:]))
	case "codex", "codex-acp", "claude", "claude-agent-acp", "grok":
		os.Exit(runFakeAIHelper())
	default:
		os.Exit(m.Run())
	}
}

type fakeReport struct {
	Executable              string   `json:"executable"`
	Args                    []string `json:"args"`
	CodexHome               string   `json:"codex_home"`
	ClaudeConfigDir         string   `json:"claude_config_dir"`
	GrokHome                string   `json:"grok_home"`
	XAIAPIKey               string   `json:"xai_api_key"`
	GrokConfig              string   `json:"grok_config"`
	GrokAuthProviderCommand string   `json:"grok_auth_provider_command"`
	GrokAuthPath            string   `json:"grok_auth_path"`
	GrokAuth                string   `json:"grok_auth"`
	GrokClaudeSkills        string   `json:"grok_claude_skills_enabled"`
	GrokDisableAutoUpdater  string   `json:"grok_disable_autoupdater"`
	GrokSandbox             string   `json:"grok_sandbox"`
	APIKey                  string   `json:"api_key"`
	OpenAIBaseURL           string   `json:"openai_base_url"`
	CodexAPIKey             string   `json:"codex_api_key"`
	CodexAccessToken        string   `json:"codex_access_token"`
	CodexSQLiteHome         string   `json:"codex_sqlite_home"`
	OpenAIFederation        string   `json:"openai_federation_rule_id"`
	OpenAIIdentityFile      string   `json:"openai_identity_token_file"`
	OpenAIIdentityCtx       string   `json:"openai_workload_identity_context"`
	AnthropicAPIKey         string   `json:"anthropic_api_key"`
	AnthropicAuthToken      string   `json:"anthropic_auth_token"`
	AnthropicConfigDir      string   `json:"anthropic_config_dir"`
	AnthropicActiveProfile  string   `json:"anthropic_active_profile"`
	AnthropicProfile        string   `json:"anthropic_profile"`
	AnthropicFederation     string   `json:"anthropic_federation_rule_id"`
	AnthropicIdentity       string   `json:"anthropic_identity_token"`
	AnthropicIdentityFile   string   `json:"anthropic_identity_token_file"`
	AnthropicServiceAccount string   `json:"anthropic_service_account_id"`
	ClaudeUseBedrock        string   `json:"claude_use_bedrock"`
	FoundryAuthToken        string   `json:"foundry_auth_token"`
	AnthropicHeaders        string   `json:"anthropic_custom_headers"`
	SecureStorageDir        string   `json:"secure_storage_dir"`
	PluginCacheDir          string   `json:"plugin_cache_dir"`
	ProfileGuidance         string   `json:"profile_guidance"`
	ProjectGuidance         string   `json:"project_guidance"`
	CWD                     string   `json:"cwd"`
}

func TestCLIEndToEndWithSyntheticState(t *testing.T) {
	binDir := t.TempDir()
	aiBin := filepath.Join(binDir, platformExecutableName("ai-profile"))
	repoZipBin := filepath.Join(binDir, platformExecutableName("repo-zip"))
	installSelfAs(t, aiBin)
	installSelfAs(t, repoZipBin)
	for _, name := range []string{"codex", "codex-acp", "claude", "claude-agent-acp", "grok"} {
		installSelfAs(t, filepath.Join(binDir, platformExecutableName(name)))
	}

	sandbox := t.TempDir()
	baseEnv := safeTestEnvAt(t, sandbox)
	if testing.CoverMode() != "" {
		coverDir := filepath.Join(sandbox, "cover")
		if err := os.MkdirAll(coverDir, 0o755); err != nil {
			t.Fatal(err)
		}
		baseEnv = setEnv(baseEnv, "GOCOVERDIR", coverDir)
	}
	home := filepath.Join(sandbox, "home")
	profileRoot := filepath.Join(home, "profiles")
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude", "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".config", "anthropic"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(home, ".codex", "AGENTS.md"), "default codex guidance must not leak")
	write(t, filepath.Join(home, ".claude", "CLAUDE.md"), "default claude guidance must not leak")
	write(t, filepath.Join(home, ".claude", "rules", "default.md"), "default rule must not leak")
	write(t, filepath.Join(home, ".config", "anthropic", "active_config"), "default-anthropic-profile-must-not-leak")

	project := filepath.Join(sandbox, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(project, "AGENTS.md"), "project codex guidance")
	write(t, filepath.Join(project, "CLAUDE.md"), "project claude guidance")

	baseEnv = setEnv(baseEnv, "AI_PROFILE_ROOT", profileRoot)
	baseEnv = setEnv(baseEnv, "PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for key, value := range map[string]string{
		"OPENAI_API_KEY":                   "must-not-reach-child",
		"OPENAI_BASE_URL":                  "https://wrong.invalid",
		"CODEX_API_KEY":                    "must-not-reach-child",
		"CODEX_ACCESS_TOKEN":               "must-not-reach-child",
		"CODEX_SQLITE_HOME":                "/wrong/sqlite",
		"OPENAI_FEDERATION_RULE_ID":        "wrong-federation",
		"OPENAI_IDENTITY_TOKEN_FILE":       "/wrong/openai-token",
		"OPENAI_WORKLOAD_IDENTITY_CONTEXT": `{"source":"wrong"}`,
		"ANTHROPIC_API_KEY":                "must-not-reach-child",
		"ANTHROPIC_AUTH_TOKEN":             "must-not-reach-child",
		"ANTHROPIC_CONFIG_DIR":             "/wrong/anthropic-config",
		"CLAUDE_CODE_OAUTH_TOKEN":          "must-not-reach-child",
		"ANTHROPIC_PROFILE":                "wrong-profile",
		"ANTHROPIC_FEDERATION_RULE_ID":     "wrong-federation",
		"ANTHROPIC_IDENTITY_TOKEN":         "must-not-reach-child",
		"ANTHROPIC_IDENTITY_TOKEN_FILE":    "/wrong/anthropic-token",
		"ANTHROPIC_SERVICE_ACCOUNT_ID":     "wrong-service-account",
		"CLAUDE_CODE_USE_BEDROCK":          "1",
		"ANTHROPIC_FOUNDRY_AUTH_TOKEN":     "must-not-reach-child",
		"ANTHROPIC_CUSTOM_HEADERS":         "Authorization: Bearer wrong",
		"CLAUDE_SECURESTORAGE_CONFIG_DIR":  "/wrong/secure",
		"CLAUDE_CODE_PLUGIN_CACHE_DIR":     "/wrong/plugins",
		"XAI_API_KEY":                      "must-not-reach-child",
		"GROK_CONFIG":                      "/wrong/grok-config.toml",
		"GROK_AUTH_PROVIDER_COMMAND":       "wrong-auth-provider",
		"GROK_AUTH_PATH":                   "/wrong/auth.json",
		"GROK_AUTH":                        "wrong-inline-auth",
		"GROK_CLAUDE_SKILLS_ENABLED":       "true",
		"GROK_SANDBOX":                     "strict",
	} {
		baseEnv = setEnv(baseEnv, key, value)
	}

	codexDir := createAndResolveProfile(t, aiBin, baseEnv, "codex", "work")
	claudeDir := createAndResolveProfile(t, aiBin, baseEnv, "claude", "work")
	grokDir := createAndResolveProfile(t, aiBin, baseEnv, "grok", "work")
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
		{tool: "grok", action: "run", wantExe: "grok", wantCode: 17, profileDir: grokDir, args: []string{"-p", "two words"}},
		{tool: "grok", action: "acp", wantExe: "grok", wantCode: 17, profileDir: grokDir, args: []string{"--model", "grok-4.7"}},
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
			wantArgs := tc.args
			if tc.tool == "grok" && tc.action == "acp" {
				wantArgs = append([]string{"agent"}, tc.args...)
				wantArgs = append(wantArgs, "stdio")
			}
			assertFakeReport(t, tc.tool, report, tc.profileDir, project, wantArgs)
		})
	}

	// Claude isolation preparation preserves its own context while excluding the
	// default ~/.claude memory/rules path as defense in depth.
	raw, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		ClaudeMdExcludes []string `json:"claudeMdExcludes"`
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("decode Claude settings: %v\n%s", err, raw)
	}
	for _, want := range []string{
		filepath.Join(home, ".claude", "CLAUDE.md"),
		filepath.Join(home, ".claude", "CLAUDE.local.md"),
		filepath.Join(home, ".claude", "rules", "**"),
	} {
		found := false
		for _, got := range settings.ClaudeMdExcludes {
			if got == want {
				found = true
				break
			}
		}
		if !found {
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
	if !sameDirectory(report.CWD, wantCWD) {
		t.Fatalf("cwd=%q want %q", report.CWD, wantCWD)
	}
	if strings.Join(report.Args, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("argv mismatch: got %#v want %#v", report.Args, wantArgs)
	}
	switch tool {
	case "codex":
		if report.APIKey != "" || report.OpenAIBaseURL != "" || report.CodexAPIKey != "" || report.CodexAccessToken != "" || report.CodexSQLiteHome != "" || report.OpenAIFederation != "" || report.OpenAIIdentityFile != "" || report.OpenAIIdentityCtx != "" {
			t.Fatalf("Codex authentication override leaked: %#v", report)
		}
		if report.CodexHome != wantHome || report.ClaudeConfigDir != "" {
			t.Fatalf("bad Codex profile env: %#v", report)
		}
		if report.ProfileGuidance != "profile codex guidance" || report.ProjectGuidance != "project codex guidance" {
			t.Fatalf("bad Codex context: %#v", report)
		}
	case "claude":
		if report.AnthropicAPIKey != "" || report.AnthropicAuthToken != "" || report.AnthropicProfile != "" || report.AnthropicFederation != "" || report.AnthropicIdentity != "" || report.AnthropicIdentityFile != "" || report.AnthropicServiceAccount != "" || report.ClaudeUseBedrock != "" || report.FoundryAuthToken != "" || report.AnthropicHeaders != "" || report.SecureStorageDir != "" || report.PluginCacheDir != "" {
			t.Fatalf("Claude authentication/config redirect leaked: %#v", report)
		}
		if report.ClaudeConfigDir != wantHome || report.AnthropicConfigDir != filepath.Join(wantHome, ".anthropic") || report.AnthropicActiveProfile != "" || report.CodexHome != "" {
			t.Fatalf("bad Claude profile env: %#v", report)
		}
		if report.ProfileGuidance != "profile claude guidance" || report.ProjectGuidance != "project claude guidance" {
			t.Fatalf("bad Claude context: %#v", report)
		}
	case "grok":
		if report.XAIAPIKey != "" || report.GrokConfig != "" || report.GrokAuthProviderCommand != "" || report.GrokAuthPath != "" || report.GrokAuth != "" || report.GrokClaudeSkills != "" {
			t.Fatalf("Grok authentication/config redirect leaked: %#v", report)
		}
		if report.GrokHome != wantHome || report.CodexHome != "" || report.ClaudeConfigDir != "" {
			t.Fatalf("bad Grok profile env: %#v", report)
		}
		if report.GrokDisableAutoUpdater != "1" || report.GrokSandbox != "strict" {
			t.Fatalf("Grok launcher controls not preserved/applied: %#v", report)
		}
	default:
		t.Fatalf("unknown tool %q", tool)
	}
}

func sameDirectory(a, b string) bool {
	aInfo, err := os.Stat(a)
	if err != nil {
		return false
	}
	bInfo, err := os.Stat(b)
	return err == nil && os.SameFile(aInfo, bInfo)
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

func installSelfAs(t *testing.T, dst string) {
	t.Helper()
	src, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Link(src, dst); err == nil {
		return
	}
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

func runAIProfileHelper(args []string) int {
	service := core.NewLazy(func() (*aiprofile.Service, error) {
		root, err := aiprofile.DefaultRoot()
		if err != nil {
			return nil, err
		}
		return aiprofile.NewService(aiprofile.Store{Root: root}, platform.Runner{})
	})
	app, err := profilecli.New(service, aiprofile.ProcessIO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}, core.ProductMetadata{Version: "test", SuiteVersion: "test"})
	if err == nil {
		err = app.Run(context.Background(), args, core.IO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr, Terminal: core.TerminalFromFiles(os.Stdin, os.Stdout, os.Stderr)})
	}
	if err != nil {
		if code, ok := platform.ChildExitCode(err); ok {
			return code
		}
		core.RenderDiagnostic(os.Stderr, err)
		return core.ExitCode(err)
	}
	return 0
}

func runRepoZipHelper(args []string) int {
	app, err := repocli.New(repozip.Service{Git: repozip.Git{}, Archiver: repozip.Archiver{}}, core.ProductMetadata{Version: "test", SuiteVersion: "test"})
	if err == nil {
		err = app.Run(context.Background(), args, core.IO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr, Terminal: core.TerminalFromFiles(os.Stdin, os.Stdout, os.Stderr)})
	}
	if err != nil {
		core.RenderDiagnostic(os.Stderr, err)
		return core.ExitCode(err)
	}
	return 0
}

func runFakeAIHelper() int {
	exe := helperExecutableName(os.Args[0])
	profileRoot := os.Getenv("CODEX_HOME")
	profileFile := "AGENTS.md"
	projectFile := "AGENTS.md"
	if strings.HasPrefix(exe, "claude") {
		profileRoot = os.Getenv("CLAUDE_CONFIG_DIR")
		profileFile = "CLAUDE.md"
		projectFile = "CLAUDE.md"
	} else if exe == "grok" {
		profileRoot = os.Getenv("GROK_HOME")
		profileFile = "config.toml"
		projectFile = "AGENTS.md"
	}
	profileGuidance, _ := os.ReadFile(filepath.Join(profileRoot, profileFile))
	activeAnthropicProfile, _ := os.ReadFile(filepath.Join(os.Getenv("ANTHROPIC_CONFIG_DIR"), "active_config"))
	cwd, _ := os.Getwd()
	projectGuidance, _ := os.ReadFile(filepath.Join(cwd, projectFile))
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"executable":                       exe,
		"args":                             os.Args[1:],
		"codex_home":                       os.Getenv("CODEX_HOME"),
		"claude_config_dir":                os.Getenv("CLAUDE_CONFIG_DIR"),
		"grok_home":                        os.Getenv("GROK_HOME"),
		"xai_api_key":                      os.Getenv("XAI_API_KEY"),
		"grok_config":                      os.Getenv("GROK_CONFIG"),
		"grok_auth_provider_command":       os.Getenv("GROK_AUTH_PROVIDER_COMMAND"),
		"grok_auth_path":                   os.Getenv("GROK_AUTH_PATH"),
		"grok_auth":                        os.Getenv("GROK_AUTH"),
		"grok_claude_skills_enabled":       os.Getenv("GROK_CLAUDE_SKILLS_ENABLED"),
		"grok_disable_autoupdater":         os.Getenv("GROK_DISABLE_AUTOUPDATER"),
		"grok_sandbox":                     os.Getenv("GROK_SANDBOX"),
		"api_key":                          os.Getenv("OPENAI_API_KEY"),
		"openai_base_url":                  os.Getenv("OPENAI_BASE_URL"),
		"codex_api_key":                    os.Getenv("CODEX_API_KEY"),
		"codex_access_token":               os.Getenv("CODEX_ACCESS_TOKEN"),
		"codex_sqlite_home":                os.Getenv("CODEX_SQLITE_HOME"),
		"openai_federation_rule_id":        os.Getenv("OPENAI_FEDERATION_RULE_ID"),
		"openai_identity_token_file":       os.Getenv("OPENAI_IDENTITY_TOKEN_FILE"),
		"openai_workload_identity_context": os.Getenv("OPENAI_WORKLOAD_IDENTITY_CONTEXT"),
		"anthropic_api_key":                os.Getenv("ANTHROPIC_API_KEY"),
		"anthropic_auth_token":             os.Getenv("ANTHROPIC_AUTH_TOKEN"),
		"anthropic_config_dir":             os.Getenv("ANTHROPIC_CONFIG_DIR"),
		"anthropic_active_profile":         string(activeAnthropicProfile),
		"anthropic_profile":                os.Getenv("ANTHROPIC_PROFILE"),
		"anthropic_federation_rule_id":     os.Getenv("ANTHROPIC_FEDERATION_RULE_ID"),
		"anthropic_identity_token":         os.Getenv("ANTHROPIC_IDENTITY_TOKEN"),
		"anthropic_identity_token_file":    os.Getenv("ANTHROPIC_IDENTITY_TOKEN_FILE"),
		"anthropic_service_account_id":     os.Getenv("ANTHROPIC_SERVICE_ACCOUNT_ID"),
		"claude_use_bedrock":               os.Getenv("CLAUDE_CODE_USE_BEDROCK"),
		"foundry_auth_token":               os.Getenv("ANTHROPIC_FOUNDRY_AUTH_TOKEN"),
		"anthropic_custom_headers":         os.Getenv("ANTHROPIC_CUSTOM_HEADERS"),
		"secure_storage_dir":               os.Getenv("CLAUDE_SECURESTORAGE_CONFIG_DIR"),
		"plugin_cache_dir":                 os.Getenv("CLAUDE_CODE_PLUGIN_CACHE_DIR"),
		"profile_guidance":                 string(profileGuidance),
		"project_guidance":                 string(projectGuidance),
		"cwd":                              cwd,
	})
	if strings.Contains(exe, "acp") {
		return 23
	}
	return 17
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
	bin := filepath.Join(t.TempDir(), platformExecutableName("ai-profile"))
	installSelfAs(t, bin)

	env := unsetEnv(safeTestEnv(t), "HOME", "USERPROFILE", "AI_PROFILE_ROOT")
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "version", args: []string{"--version"}},
		{name: "help", args: []string{"--help"}},
		{name: "completion", args: []string{"completion", "generate", "bash"}},
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
