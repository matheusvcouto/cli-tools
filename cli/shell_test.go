package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func shellFixture(t *testing.T) *CompiledApp {
	t.Helper()
	app, err := Compile(App{
		ID: "shell-fixture", Name: "fixture", Builtins: Builtins{Completion: true},
		Root: Command{ID: "root", Name: "fixture", Commands: []Command{{
			ID: "serve", Name: "serve", Aliases: []string{"s"}, Summary: "serve things",
			Args:  []Arg{{ID: "target", Name: "target", Summary: "target name", Value: EnumValue(Choice{Value: "one", Description: "first"}, Choice{Value: "two words", Description: "spaced"})}},
			Flags: []Flag{{ID: "format", Long: "format", Short: 'f', Summary: "output format", Value: EnumValue(Choice{Value: "json"}, Choice{Value: "text"})}},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func TestShellAdaptersConformanceSurface(t *testing.T) {
	app := shellFixture(t)
	for _, shell := range SupportedShells() {
		t.Run(string(shell), func(t *testing.T) {
			script, err := app.CompletionScript(shell)
			if err != nil {
				t.Fatal(err)
			}
			if script == "" || !strings.Contains(script, "fixture") || !strings.Contains(script, "completion protocol 1") {
				t.Fatalf("incomplete %s adapter:\n%s", shell, script)
			}
			caps, err := ShellCapabilities(shell)
			if err != nil {
				t.Fatal(err)
			}
			if !caps.DynamicProtocol {
				t.Fatalf("%s does not declare dynamic protocol", shell)
			}
		})
	}
}

func TestPowerShellUsesCursorPositionForMidLineCompletion(t *testing.T) {
	app := shellFixture(t)
	script, err := app.CompletionScript(ShellPowerShell)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"$cursorPosition", "$elements = @($commandAst.CommandElements)", "$extent.StartOffset", "$extent.EndOffset", "$cursorArg = $argv.Count"} {
		if !strings.Contains(script, want) {
			t.Fatalf("PowerShell adapter does not derive cursor argv from AST extents; missing %q:\n%s", want, script)
		}
	}
}

func TestCompletionScriptsDoNotEmbedSpecMetadata(t *testing.T) {
	base := shellFixture(t)
	changed, err := Compile(App{
		ID: "different", Name: "fixture", Builtins: Builtins{Completion: true},
		Root: Command{ID: "other-root", Name: "fixture", Commands: []Command{{
			ID: "different.command", Name: "totally-different", Aliases: []string{"td"},
			Flags: []Flag{{ID: "different.flag", Long: "different", Value: EnumValue(Choice{Value: "never-embed-me"})}},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, shell := range SupportedShells() {
		a, err := base.CompletionScript(shell)
		if err != nil {
			t.Fatal(err)
		}
		b, err := changed.CompletionScript(shell)
		if err != nil {
			t.Fatal(err)
		}
		if a != b {
			t.Fatalf("%s completion script depends on CLI spec metadata", shell)
		}
		if strings.Contains(a, "never-embed-me") || strings.Contains(a, "totally-different") || strings.Contains(a, "output format") || strings.Contains(a, "two words") {
			t.Fatalf("%s completion script embeds runtime spec metadata:\n%s", shell, a)
		}
		caps, err := ShellCapabilities(shell)
		if err != nil {
			t.Fatal(err)
		}
		if caps.StaticMetadata || caps.TypedSignatures {
			t.Fatalf("%s advertises stale-prone static metadata capabilities: %+v", shell, caps)
		}
	}
}

func TestBashAdapterSyntaxAndNULTransport(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not installed")
	}
	app := shellFixture(t)
	script, err := app.CompletionScript(ShellBash)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	completion := filepath.Join(dir, "fixture.bash")
	if err := os.WriteFile(completion, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(bash, "-n", completion).CombinedOutput(); err != nil {
		t.Fatalf("bash syntax: %v\n%s\n%s", err, out, script)
	}
	fake := filepath.Join(dir, "fixture")
	fakeScript := `#!/usr/bin/env bash
if [[ "$1" == __cli && "$2" == complete-shell ]]; then
  printf '%s\0' 'cli-completion' '1'
  printf '%s\0' 'candidate' 'two words' '' 'spaced value' 'value' '' 'id'
  printf '%s\0' 'directive' '0' '0' '0' '0' '1' 'end'
fi
`
	if err := os.WriteFile(fake, []byte(fakeScript), 0o700); err != nil {
		t.Fatal(err)
	}
	probe := `source "$1"
COMP_WORDS=(fixture "t")
COMP_CWORD=1
COMP_LINE='fixture t'
COMP_POINT=${#COMP_LINE}
_fixture_completion fixture t fixture
printf '<%s>\n' "${COMPREPLY[@]}"
`
	cmd := exec.Command(bash, "-c", probe, "bash", completion)
	cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash completion probe: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "<two words>" {
		t.Fatalf("unexpected completion: %q", out)
	}
}

func TestBashAndZshUseNativeCursorPrefixes(t *testing.T) {
	app := shellFixture(t)
	bash, err := app.CompletionScript(ShellBash)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bash, `local cur="$2" base="$2"`) {
		t.Fatalf("Bash adapter must use the word-being-completed argument for cursor prefix:\n%s", bash)
	}
	zsh, err := app.CompletionScript(ShellZsh)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(zsh, `local cur="$PREFIX"`) {
		t.Fatalf("Zsh adapter must use PREFIX for cursor-aware completion:\n%s", zsh)
	}
}

func TestFishAndZshPreserveAttachedFlagPrefixForNativeFiles(t *testing.T) {
	app := shellFixture(t)
	fish, err := app.CompletionScript(ShellFish)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"native_attach", "native_prefix", `string match -q -- '--*=*'`} {
		if !strings.Contains(fish, want) {
			t.Fatalf("Fish adapter missing attached-value handling %q:\n%s", want, fish)
		}
	}
	zsh, err := app.CompletionScript(ShellZsh)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(zsh, `compset -P '*='`) {
		t.Fatalf("Zsh adapter missing attached-value prefix handling:\n%s", zsh)
	}
}

func TestGeneratedCompletionCommandsUseSameGraph(t *testing.T) {
	app := shellFixture(t)
	var out strings.Builder
	if err := app.Run(context.Background(), []string{"completion", "list"}, IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	for _, shell := range SupportedShells() {
		if !strings.Contains(out.String(), string(shell)) {
			t.Fatalf("missing %s in %q", shell, out.String())
		}
	}
	out.Reset()
	if err := app.Run(context.Background(), []string{"completion", "generate", "bash"}, IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "complete -F _fixture_completion") {
		t.Fatalf("unexpected bash output:\n%s", out.String())
	}
	contract := app.Contract()
	found := false
	for _, cmd := range contract.Root.Commands {
		if cmd.ID == builtinCompletionCommandID {
			found = true
		}
	}
	if !found {
		t.Fatal("generated completion command missing from contract")
	}
}

func TestCompletionLifecycleUsesSyntheticUserDirs(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(home, "config")
	data := filepath.Join(home, "data")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("SHELL", "/bin/fish")

	app := shellFixture(t)
	var out strings.Builder
	if err := app.Run(context.Background(), []string{"completion", "install", "fish"}, IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(config, "fish", "completions", "fixture.fish")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "generated by fixture; completion protocol 1") {
		t.Fatalf("unexpected installed file: %q", raw)
	}
	for _, forbidden := range []string{".bashrc", ".zshrc", "config.nu", "Microsoft.PowerShell_profile.ps1"} {
		if _, err := os.Stat(filepath.Join(home, forbidden)); !os.IsNotExist(err) {
			t.Fatalf("completion install unexpectedly touched %s", forbidden)
		}
	}

	out.Reset()
	if err := app.Run(context.Background(), []string{"completion", "status", "fish"}, IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "fish\tcurrent\t"+path) {
		t.Fatalf("status=%q", out.String())
	}

	// A prior generated file may be upgraded, but a foreign file is never
	// overwritten or removed by lifecycle commands.
	if err := os.WriteFile(path, []byte("# generated by fixture; completion protocol 0\nold\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := app.Run(context.Background(), []string{"completion", "install", "fish"}, IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	if raw, err = os.ReadFile(path); err != nil || !strings.Contains(string(raw), "completion protocol 1") {
		t.Fatalf("upgrade raw=%q err=%v", raw, err)
	}
	if err := os.WriteFile(path, []byte("user owned\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background(), []string{"completion", "uninstall", "fish"}, IO{Out: &out}); err == nil {
		t.Fatal("expected foreign completion removal to be refused")
	}
	if raw, err = os.ReadFile(path); err != nil || string(raw) != "user owned\n" {
		t.Fatalf("foreign file changed raw=%q err=%v", raw, err)
	}
}

func TestCompletionInstallAndUninstallWithoutShellOperateOnAllAdapters(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("SHELL", "/bin/does-not-matter")
	app := shellFixture(t)

	// Preflight must fail before touching any other target when one target is foreign.
	zshTarget, err := app.completionTarget(ShellZsh)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(zshTarget.Path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(zshTarget.Path, []byte("foreign\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background(), []string{"completion", "install"}, IO{Out: &strings.Builder{}}); err == nil {
		t.Fatal("install-all should reject a foreign target during preflight")
	}
	for _, shell := range SupportedShells() {
		target, err := app.completionTarget(shell)
		if err != nil {
			t.Fatal(err)
		}
		if shell == ShellZsh {
			continue
		}
		if _, err := os.Stat(target.Path); !os.IsNotExist(err) {
			t.Fatalf("%s was mutated before preflight completed: %v", shell, err)
		}
	}
	if err := os.Remove(zshTarget.Path); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	if err := app.Run(context.Background(), []string{"completion", "install"}, IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	for _, shell := range SupportedShells() {
		target, err := app.completionTarget(shell)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(target.Path); err != nil {
			t.Fatalf("%s completion not installed at %s: %v", shell, target.Path, err)
		}
	}
	out.Reset()
	if err := app.Run(context.Background(), []string{"completion", "uninstall"}, IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	for _, shell := range SupportedShells() {
		target, _ := app.completionTarget(shell)
		if _, err := os.Stat(target.Path); !os.IsNotExist(err) {
			t.Fatalf("%s completion still exists after uninstall-all: %v", shell, err)
		}
	}
}

func TestCompletionStatusWithoutShellIsDeterministic(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	app := shellFixture(t)
	var out strings.Builder
	if err := app.Run(context.Background(), []string{"completion", "status"}, IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != len(SupportedShells()) {
		t.Fatalf("lines=%q", lines)
	}
	got := make([]string, 0, len(lines))
	for _, line := range lines {
		got = append(got, strings.SplitN(line, "\t", 2)[0])
	}
	want := sortedShellNames()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got=%v want=%v", got, want)
	}
}

func TestNativeShellSyntaxWhereAvailable(t *testing.T) {
	app := shellFixture(t)
	cases := []struct {
		name   string
		shell  Shell
		binary string
		args   func(string) []string
	}{
		{name: "fish", shell: ShellFish, binary: "fish", args: func(path string) []string { return []string{"-N", "-n", path} }},
		{name: "nushell", shell: ShellNushell, binary: "nu", args: func(path string) []string { return []string{"-n", path} }},
		{name: "zsh", shell: ShellZsh, binary: "zsh", args: func(path string) []string { return []string{"-f", "-n", path} }},
		{name: "powershell", shell: ShellPowerShell, binary: "pwsh", args: func(path string) []string {
			probe := `$errors=$null;$tokens=$null;[System.Management.Automation.Language.Parser]::ParseFile(` + psSingleQuote(path) + `,[ref]$tokens,[ref]$errors)>$null;if($errors.Count -gt 0){$errors | ForEach-Object { Write-Error $_ }; exit 1}`
			return []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-Command", probe}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bin, err := exec.LookPath(tc.binary)
			if err != nil {
				t.Skip(tc.binary + " not installed on this runner")
			}
			script, err := app.CompletionScript(tc.shell)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "completion.txt")
			if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(bin, tc.args(path)...)
			cmd.Env = nativeShellEnv(t, "")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%s rejected generated completion: %v\n%s", tc.name, err, out)
			}
		})
	}
}

func TestNushellCompletionVersionFloor(t *testing.T) {
	for _, tc := range []struct {
		version string
		want    bool
	}{
		{version: "0.110.0", want: false},
		{version: "0.113.1", want: false},
		{version: "0.114.0", want: true},
		{version: "0.114.1", want: true},
		{version: "0.115.1", want: true},
		{version: "1.0.0", want: true},
	} {
		t.Run(tc.version, func(t *testing.T) {
			got, err := nushellCompletionVersionSupported(tc.version)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("supported=%v want=%v", got, tc.want)
			}
		})
	}
	if _, err := nushellCompletionVersionSupported("not-a-version"); err == nil {
		t.Fatal("expected malformed version to fail")
	}
}

func TestNushellCompletionRuntimeDiagnosticDetectsLegacyVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("synthetic nu executable uses a POSIX script")
	}
	dir := t.TempDir()
	nu := filepath.Join(dir, "nu")
	if err := os.WriteFile(nu, []byte("#!/bin/sh\nprintf '0.110.0\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	status, note := completionRuntimeDiagnostic(context.Background(), ShellNushell, completionCurrent)
	if status != "error" || !strings.Contains(note, "below the completion requirement 0.114+") {
		t.Fatalf("status=%q note=%q", status, note)
	}

	if err := os.WriteFile(nu, []byte("#!/bin/sh\nprintf '0.114.1\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	status, note = completionRuntimeDiagnostic(context.Background(), ShellNushell, completionCurrent)
	if status != "" || note != "" {
		t.Fatalf("supported Nushell diagnosed unexpectedly: status=%q note=%q", status, note)
	}
}
