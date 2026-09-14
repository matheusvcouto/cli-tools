package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/matheusvcouto/cli-tools/internal/testenv"
)

const nativeShellFakeName = "fixture"

func TestMain(m *testing.M) {
	base := strings.TrimSuffix(strings.ToLower(filepath.Base(os.Args[0])), ".exe")
	if base == nativeShellFakeName {
		os.Exit(runNativeShellFake(os.Args[1:]))
	}
	os.Exit(m.Run())
}

func runNativeShellFake(args []string) int {
	if len(args) < 2 || args[0] != ReservedNamespace {
		return 2
	}
	switch args[1] {
	case "complete":
		var request CompletionRequest
		if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "decode completion request: %v\\n", err)
			return 1
		}
		if request.Protocol != CompletionProtocol || request.CursorArg < 0 || request.CursorOffset < 0 {
			return 2
		}
		if request.Shell == "nushell" && (len(request.Argv) != 3 || request.Argv[0] != "fixture" || request.Argv[1] != "serve" || request.Argv[2] != "t" || request.CursorArg != 2 || request.CursorOffset != 1) {
			return 2
		}
		_ = json.NewEncoder(os.Stdout).Encode(CompletionResult{
			Protocol: CompletionProtocol,
			Candidates: []CompletionCandidate{{
				Value: "two words", Label: "two words", Description: "native probe", Kind: CandidateValue, ID: "probe.value",
			}},
		})
		return 0
	case "complete-shell":
		write := func(s string) { _, _ = io.WriteString(os.Stdout, s+"\x00") }
		write("cli-completion")
		write(fmt.Sprint(CompletionProtocol))
		write("candidate")
		write("two words")
		write("two words")
		write("native probe")
		write(string(CandidateValue))
		write("")
		write("probe.value")
		write("directive")
		for i := 0; i < 5; i++ {
			write("0")
		}
		write("end")
		return 0
	default:
		return 2
	}
}

func installNativeShellFake(t *testing.T, dir string) string {
	t.Helper()
	src, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	name := nativeShellFakeName
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	dst := filepath.Join(dir, name)
	if err := os.Link(src, dst); err == nil {
		return dst
	}
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
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
	return dst
}

func writeNativeShellScript(t *testing.T, app *CompiledApp, shell Shell, dir string) string {
	t.Helper()
	script, err := app.CompletionScript(shell)
	if err != nil {
		t.Fatal(err)
	}
	extension := ".txt"
	if shell == ShellPowerShell {
		extension = ".ps1"
	}
	path := filepath.Join(dir, "completion-"+string(shell)+extension)
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func nativeShellEnv(t *testing.T, dir string) []string {
	t.Helper()
	sandbox := t.TempDir()
	env, err := testenv.New(sandbox)
	if err != nil {
		t.Fatal(err)
	}
	env = testenv.Set(env, "ZDOTDIR", filepath.Join(sandbox, "home"))
	if dir == "" {
		return env
	}
	return testenv.Set(env, "PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestFishNativeCompletionBehaviorWhereAvailable(t *testing.T) {
	fish, err := exec.LookPath("fish")
	if err != nil {
		t.Skip("fish not installed on this runner")
	}
	dir := t.TempDir()
	installNativeShellFake(t, dir)
	path := writeNativeShellScript(t, shellFixture(t), ShellFish, dir)
	cmd := exec.Command(fish, "--no-config", "-c", `source $argv[1]; complete -C "fixture serve t"`, path)
	cmd.Env = nativeShellEnv(t, dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fish native completion: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "two") {
		t.Fatalf("fish native completion missing spaced candidate: %q", out)
	}
}

func TestNushellNativeCompletionBehaviorWhereAvailable(t *testing.T) {
	nu, err := exec.LookPath("nu")
	if err != nil {
		t.Skip("nu not installed on this runner")
	}
	version := exec.Command(nu, "-n", "--version")
	version.Env = nativeShellEnv(t, "")
	versionOut, err := version.CombinedOutput()
	if err != nil {
		t.Fatalf("nushell version probe: %v\n%s", err, versionOut)
	}
	parts := strings.Split(strings.TrimSpace(string(versionOut)), ".")
	if len(parts) < 2 || parts[0] != "0" {
		t.Fatalf("unexpected nushell version %q", strings.TrimSpace(string(versionOut)))
	}
	var minor int
	if _, err := fmt.Sscanf(parts[1], "%d", &minor); err != nil {
		t.Fatalf("parse nushell version %q: %v", strings.TrimSpace(string(versionOut)), err)
	}
	if minor < 114 {
		t.Skipf("nushell %s is below the supported 0.114+ completion floor", strings.TrimSpace(string(versionOut)))
	}
	dir := t.TempDir()
	installNativeShellFake(t, dir)
	path := writeNativeShellScript(t, shellFixture(t), ShellNushell, dir)
	probe := filepath.Join(dir, "probe.nu")
	probeBody := fmt.Sprintf("source %s\n\"fixture serve t\" | commandline complete --detailed | to json -r\n", nuExternal(path))
	if err := os.WriteFile(probe, []byte(probeBody), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(nu, "-n", probe)
	cmd.Env = nativeShellEnv(t, dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("nushell native completion: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "two words") {
		t.Fatalf("nushell native completion missing spaced candidate: %q", out)
	}
}

func TestZshNativeCompletionBehaviorWhereAvailable(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh not installed on this runner")
	}
	dir := t.TempDir()
	installNativeShellFake(t, dir)
	path := writeNativeShellScript(t, shellFixture(t), ShellZsh, dir)
	probe := `
compdef() { :; }
compadd() { print -r -- "$@"; }
_directories() { :; }
_files() { :; }
_command_names() { :; }
source "$1"
words=(fixture serve t)
CURRENT=3
_fixture
`
	cmd := exec.Command(zsh, "-f", "-c", probe, "zsh", path)
	cmd.Env = nativeShellEnv(t, dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("zsh native completion: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "two words") {
		t.Fatalf("zsh native completion missing spaced candidate: %q", out)
	}
}

func TestPowerShellNativeCompletionBehaviorWhereAvailable(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh not installed on this runner")
	}
	dir := t.TempDir()
	installNativeShellFake(t, dir)
	path := writeNativeShellScript(t, shellFixture(t), ShellPowerShell, dir)
	probe := `. ` + psSingleQuote(path) + `; $r = TabExpansion2 -inputScript 'fixture serve t' -cursorColumn 15; $r.CompletionMatches | ForEach-Object { $_.CompletionText }`
	cmd := exec.Command(pwsh, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", probe)
	cmd.Env = nativeShellEnv(t, dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("powershell native completion: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "two words") {
		t.Fatalf("powershell native completion missing spaced candidate: %q", out)
	}
}
