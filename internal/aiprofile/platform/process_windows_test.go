//go:build windows

package platform

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/matheusvcouto/cli-tools/internal/aiprofile"
	"github.com/matheusvcouto/cli-tools/internal/testenv"
)

func TestWindowsRunnerDoesNotLaunchAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := (Runner{}).Replace(ctx, "not-a-real-agent-executable", nil, nil, aiprofile.ProcessIO{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Windows launch returned %v", err)
	}
}

func TestWindowsRunnerTarget(t *testing.T) {
	if os.Getenv("GO_WINDOWS_RUNNER_TARGET") != "1" {
		return
	}
	if os.Getenv("GO_WINDOWS_RUNNER_GRANDCHILD") == "1" {
		time.Sleep(30 * time.Second)
		return
	}
	if os.Getenv("GO_WINDOWS_RUNNER_SPAWN_CHILD") == "1" {
		child := exec.Command(os.Args[0], "-test.run=^TestWindowsRunnerTarget$")
		child.Env = append(withoutEnvKeys(os.Environ(),
			"GO_WINDOWS_RUNNER_TARGET",
			"GO_WINDOWS_RUNNER_SPAWN_CHILD",
			"GO_WINDOWS_RUNNER_GRANDCHILD",
			"GO_WINDOWS_RUNNER_WAIT",
		), "GO_WINDOWS_RUNNER_TARGET=1", "GO_WINDOWS_RUNNER_GRANDCHILD=1")
		if err := child.Start(); err != nil {
			fmt.Fprintln(os.Stderr, "spawn grandchild:", err)
			os.Exit(92)
		}
		if path := os.Getenv("GO_WINDOWS_RUNNER_PID_FILE"); path != "" {
			// Publish only a complete PID. WriteFile makes an empty file visible
			// before writing, which races with the cancellation test's reader.
			pending := path + ".pending"
			err := os.WriteFile(pending, []byte(strconv.Itoa(child.Process.Pid)), 0o600)
			if err == nil {
				err = os.Rename(pending, path)
			}
			if err != nil {
				_ = child.Process.Kill()
				fmt.Fprintln(os.Stderr, "write grandchild pid:", err)
				os.Exit(93)
			}
		}
		if err := child.Wait(); err != nil {
			os.Exit(94)
		}
		return
	}
	if os.Getenv("GO_WINDOWS_RUNNER_WAIT") == "1" {
		time.Sleep(30 * time.Second)
		return
	}
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read stdin:", err)
		os.Exit(91)
	}
	fmt.Fprintf(os.Stdout, "stdin=%s\nenv=%s\nargs=%s\n", strings.TrimSpace(string(input)), os.Getenv("SYNTHETIC_VALUE"), strings.Join(os.Args[2:], "|"))
	fmt.Fprintln(os.Stderr, "target-stderr")
	if os.Getenv("GO_WINDOWS_RUNNER_EXIT") == "23" {
		os.Exit(23)
	}
}

func TestWindowsRunnerForwardsArgvEnvironmentAndStdio(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	inPath := filepath.Join(t.TempDir(), "stdin.txt")
	if err := os.WriteFile(inPath, []byte("from-stdin\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	in, err := os.Open(inPath)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.CreateTemp(t.TempDir(), "stdout-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	errOut, err := os.CreateTemp(t.TempDir(), "stderr-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer errOut.Close()

	env, err := testenv.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	env = append(env, "GO_WINDOWS_RUNNER_TARGET=1", "GO_WINDOWS_RUNNER_EXIT=23", "SYNTHETIC_VALUE=ok")
	args := []string{"-test.run=TestWindowsRunnerTarget", "two words", "amp&ersand", "caret^value", "percent%value"}
	runErr := (Runner{}).Replace(context.Background(), exe, args, env, aiprofile.ProcessIO{In: in, Out: out, Err: errOut})
	if code, ok := ChildExitCode(runErr); !ok || code != 23 {
		t.Fatalf("child exit = (%d,%v), err=%v; want (23,true)", code, ok, runErr)
	}
	if err := out.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := errOut.Sync(); err != nil {
		t.Fatal(err)
	}
	stdout, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.ReadFile(errOut.Name())
	if err != nil {
		t.Fatal(err)
	}
	got := string(stdout)
	for _, want := range []string{"stdin=from-stdin", "env=ok", "two words", "amp&ersand", "caret^value", "percent%value"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q: %q", want, got)
		}
	}
	if !strings.Contains(string(stderr), "target-stderr") {
		t.Fatalf("stderr not forwarded: %q", stderr)
	}
}

func TestWindowsRunnerReturnsNilForSuccessfulChild(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	devNullIn, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNullIn.Close()
	devNullOut, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devNullOut.Close()
	env, err := testenv.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	env = append(env, "GO_WINDOWS_RUNNER_TARGET=1")
	if err := (Runner{}).Replace(context.Background(), exe, []string{"-test.run=TestWindowsRunnerTarget"}, env, aiprofile.ProcessIO{In: devNullIn, Out: devNullOut, Err: devNullOut}); err != nil {
		t.Fatalf("successful child: %v", err)
	}
}

func TestWindowsRunnerCancellationReturnsContextError(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	devNullIn, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNullIn.Close()
	devNullOut, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devNullOut.Close()
	env, err := testenv.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	env = append(env, "GO_WINDOWS_RUNNER_TARGET=1", "GO_WINDOWS_RUNNER_WAIT=1")
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	err = (Runner{}).Replace(ctx, exe, []string{"-test.run=TestWindowsRunnerTarget"}, env, aiprofile.ProcessIO{In: devNullIn, Out: devNullOut, Err: devNullOut})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("canceled child error=%v; want context deadline exceeded", err)
	}
}

func TestWindowsRunnerCancellationTerminatesDescendantTree(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	devNullIn, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNullIn.Close()
	devNullOut, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devNullOut.Close()

	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")
	env, err := testenv.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	env = append(env,
		"GO_WINDOWS_RUNNER_TARGET=1",
		"GO_WINDOWS_RUNNER_SPAWN_CHILD=1",
		"GO_WINDOWS_RUNNER_PID_FILE="+pidFile,
	)

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- (Runner{}).Replace(ctx, exe, []string{"-test.run=^TestWindowsRunnerTarget$"}, env, aiprofile.ProcessIO{In: devNullIn, Out: devNullOut, Err: devNullOut})
	}()

	var grandchildPID int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		raw, readErr := os.ReadFile(pidFile)
		if readErr == nil {
			grandchildPID, err = strconv.Atoi(strings.TrimSpace(string(raw)))
			if err != nil {
				cancel()
				t.Fatalf("decode grandchild pid: %v", err)
			}
			break
		}
		if !errors.Is(readErr, os.ErrNotExist) {
			cancel()
			t.Fatalf("read grandchild pid: %v", readErr)
		}
		time.Sleep(25 * time.Millisecond)
	}
	if grandchildPID == 0 {
		cancel()
		<-result
		t.Fatal("grandchild was not started before deadline")
	}

	const synchronize = 0x00100000
	grandchild, err := syscall.OpenProcess(synchronize, false, uint32(grandchildPID))
	if err != nil {
		cancel()
		<-result
		t.Fatalf("open grandchild %d for wait: %v", grandchildPID, err)
	}
	defer syscall.CloseHandle(grandchild)

	cancel()
	select {
	case runErr := <-result:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("canceled runner error=%v; want context canceled", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runner did not return after cancellation")
	}

	const waitObject0 = 0
	status, err := syscall.WaitForSingleObject(grandchild, 5_000)
	if err != nil {
		t.Fatalf("wait for grandchild termination: %v", err)
	}
	if status != waitObject0 {
		t.Fatalf("grandchild process %d still alive after job cancellation; wait status=%d", grandchildPID, status)
	}
}

func withoutEnvKeys(env []string, keys ...string) []string {
	blocked := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		blocked[strings.ToUpper(key)] = struct{}{}
	}
	out := make([]string, 0, len(env))
	for _, item := range env {
		key, _, _ := strings.Cut(item, "=")
		if _, ok := blocked[strings.ToUpper(key)]; ok {
			continue
		}
		out = append(out, item)
	}
	return out
}

func TestWindowsResolveLaunchRejectsUnknownShellShim(t *testing.T) {
	dir := t.TempDir()
	shim := filepath.Join(dir, "synthetic.cmd")
	if err := os.WriteFile(shim, []byte("@exit /b 0\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("PATHEXT", ".COM;.EXE;.BAT;.CMD")
	if _, err := resolveLaunch("synthetic"); err == nil || !strings.Contains(err.Error(), "shell wrapper") {
		t.Fatalf("expected shell wrapper rejection, got %v", err)
	}
}

func TestWindowsResolveLaunchUsesVerifiedNPMEntrypointWithoutShell(t *testing.T) {
	dir := t.TempDir()
	command := "claude-agent-acp"
	entry := windowsNPMEntrypoints[command]
	shim := filepath.Join(dir, command+".cmd")
	if err := os.WriteFile(shim, []byte("@echo off\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	packageRoot := filepath.Join(dir, "node_modules", filepath.FromSlash(entry.packageName))
	entryPath := filepath.Join(packageRoot, filepath.FromSlash(entry.entrypoint))
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf(`{"name":%q,"bin":{%q:%q}}`, entry.packageName, command, entry.entrypoint)
	if err := os.WriteFile(filepath.Join(packageRoot, "package.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entryPath, []byte("#!/usr/bin/env node\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Use a real PE file as the node.exe fixture so command discovery sees a
	// native executable. This test only verifies resolution; it does not claim
	// to execute Node or the ACP package.
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	nodePath := filepath.Join(dir, "node.exe")
	if err := os.WriteFile(nodePath, raw, 0o700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir)
	t.Setenv("PATHEXT", ".COM;.EXE;.BAT;.CMD")
	launch, err := resolveLaunch(command)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(filepath.Clean(launch.path), filepath.Clean(nodePath)) {
		t.Fatalf("launch path=%q want node=%q", launch.path, nodePath)
	}
	if len(launch.prefixArgs) != 1 || filepath.Clean(launch.prefixArgs[0]) != filepath.Clean(entryPath) {
		t.Fatalf("prefix args=%#v want [%q]", launch.prefixArgs, entryPath)
	}
}

func TestWindowsResolveLaunchUsesVerifiedGrokNPMBootstrapWithoutShell(t *testing.T) {
	dir := t.TempDir()
	command := "grok"
	entry := windowsNPMEntrypoints[command]
	if entry.packageName != "@xai-official/grok" || filepath.ToSlash(entry.entrypoint) != "bin/grok" {
		t.Fatalf("unexpected Grok npm contract: %+v", entry)
	}
	shim := filepath.Join(dir, command+".cmd")
	if err := os.WriteFile(shim, []byte("@echo off\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	packageRoot := filepath.Join(dir, "node_modules", filepath.FromSlash(entry.packageName))
	entryPath := filepath.Join(packageRoot, filepath.FromSlash(entry.entrypoint))
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf(`{"name":%q,"bin":{%q:%q}}`, entry.packageName, command, entry.entrypoint)
	if err := os.WriteFile(filepath.Join(packageRoot, "package.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entryPath, []byte("#!/usr/bin/env node\nrequire('./grok-bootstrap.js');\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	nodePath := filepath.Join(dir, "node.exe")
	if err := os.WriteFile(nodePath, raw, 0o700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir)
	t.Setenv("PATHEXT", ".COM;.EXE;.BAT;.CMD")
	launch, err := resolveLaunch(command)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(filepath.Clean(launch.path), filepath.Clean(nodePath)) {
		t.Fatalf("launch path=%q want node=%q", launch.path, nodePath)
	}
	if len(launch.prefixArgs) != 1 || filepath.Clean(launch.prefixArgs[0]) != filepath.Clean(entryPath) {
		t.Fatalf("prefix args=%#v want [%q]", launch.prefixArgs, entryPath)
	}
}

func TestWindowsResolveLaunchUsesVerifiedLocalNPMEntrypointWithoutShell(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "node_modules", ".bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	command := "codex"
	entry := windowsNPMEntrypoints[command]
	shim := filepath.Join(binDir, command+".cmd")
	if err := os.WriteFile(shim, []byte("@echo off\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	packageRoot := filepath.Join(root, "node_modules", filepath.FromSlash(entry.packageName))
	entryPath := filepath.Join(packageRoot, filepath.FromSlash(entry.entrypoint))
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf(`{"name":%q,"bin":{%q:%q}}`, entry.packageName, command, entry.entrypoint)
	if err := os.WriteFile(filepath.Join(packageRoot, "package.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entryPath, []byte("#!/usr/bin/env node\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	nodePath := filepath.Join(binDir, "node.exe")
	if err := os.WriteFile(nodePath, raw, 0o700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", binDir)
	t.Setenv("PATHEXT", ".COM;.EXE;.BAT;.CMD")
	launch, err := resolveLaunch(command)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(filepath.Clean(launch.path), filepath.Clean(nodePath)) {
		t.Fatalf("launch path=%q want node=%q", launch.path, nodePath)
	}
	if len(launch.prefixArgs) != 1 || filepath.Clean(launch.prefixArgs[0]) != filepath.Clean(entryPath) {
		t.Fatalf("prefix args=%#v want [%q]", launch.prefixArgs, entryPath)
	}
}

func TestWindowsResolveLaunchUsesVerifiedPNPMGlobalEntrypointWithoutShell(t *testing.T) {
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	command := "codex"
	entry := windowsNPMEntrypoints[command]
	shim := filepath.Join(binDir, command+".cmd")
	if err := os.WriteFile(shim, []byte("@echo off\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	packageRoot := filepath.Join(home, "global", "v11", "node_modules", filepath.FromSlash(entry.packageName))
	entryPath := filepath.Join(packageRoot, filepath.FromSlash(entry.entrypoint))
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf(`{"name":%q,"bin":{%q:%q}}`, entry.packageName, command, entry.entrypoint)
	if err := os.WriteFile(filepath.Join(packageRoot, "package.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entryPath, []byte("#!/usr/bin/env node\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	nodePath := filepath.Join(binDir, "node.exe")
	if err := os.WriteFile(nodePath, raw, 0o700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PNPM_HOME", home)
	t.Setenv("PATH", binDir)
	t.Setenv("PATHEXT", ".COM;.EXE;.BAT;.CMD")
	launch, err := resolveLaunch(command)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(filepath.Clean(launch.path), filepath.Clean(nodePath)) {
		t.Fatalf("launch path=%q want node=%q", launch.path, nodePath)
	}
	if len(launch.prefixArgs) != 1 || filepath.Clean(launch.prefixArgs[0]) != filepath.Clean(entryPath) {
		t.Fatalf("prefix args=%#v want [%q]", launch.prefixArgs, entryPath)
	}
}

// A .cmd in an earlier PATH directory must not be bypassed by a same-named
// .exe elsewhere. The launcher selects the first installation according to
// the actual Windows LookPath result, then validates the npm package before
// invoking Node without any shell argument expansion.
func TestWindowsResolveLaunchHonorsPATHBeforeLaterExecutable(t *testing.T) {
	first := filepath.Join(t.TempDir(), "first")
	second := filepath.Join(t.TempDir(), "second")
	if err := os.MkdirAll(first, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(second, 0o700); err != nil {
		t.Fatal(err)
	}
	command := "grok"
	entry := windowsNPMEntrypoints[command]
	if err := os.WriteFile(filepath.Join(first, command+".cmd"), []byte("@echo off\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	packageRoot := filepath.Join(first, "node_modules", filepath.FromSlash(entry.packageName))
	entryPath := filepath.Join(packageRoot, filepath.FromSlash(entry.entrypoint))
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf(`{"name":%q,"bin":{%q:%q}}`, entry.packageName, command, entry.entrypoint)
	if err := os.WriteFile(filepath.Join(packageRoot, "package.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entryPath, []byte("#!/usr/bin/env node\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Only resolution is tested; the actual official provider is not mocked.
	// A copy of this native test executable has the correct PE extension.
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	nodePath := filepath.Join(first, "node.exe")
	if err := os.WriteFile(nodePath, content, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(second, "grok.exe"), content, 0o700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", first+string(os.PathListSeparator)+second)
	t.Setenv("PATHEXT", ".COM;.EXE;.BAT;.CMD")
	path, err := exec.LookPath(command)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(filepath.Clean(path), filepath.Join(first, command+".cmd")) {
		t.Fatalf("test prerequisite: LookPath=%q does not select earlier .cmd", path)
	}
	launch, err := resolveLaunch(command)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(filepath.Clean(launch.path), filepath.Clean(nodePath)) {
		t.Fatalf("launch=%q want Node from earlier installation %q", launch.path, nodePath)
	}
	if len(launch.prefixArgs) != 1 || !strings.EqualFold(filepath.Clean(launch.prefixArgs[0]), filepath.Clean(entryPath)) {
		t.Fatalf("wrong verified npm entrypoint: %#v; want %q", launch.prefixArgs, entryPath)
	}
}

func TestWindowsResolveLaunchRejectsMismatchedNPMMetadata(t *testing.T) {
	dir := t.TempDir()
	command := "codex-acp"
	entry := windowsNPMEntrypoints[command]
	if err := os.WriteFile(filepath.Join(dir, command+".cmd"), []byte("@echo off\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	packageRoot := filepath.Join(dir, "node_modules", filepath.FromSlash(entry.packageName))
	entryPath := filepath.Join(packageRoot, filepath.FromSlash(entry.entrypoint))
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entryPath, []byte("#!/usr/bin/env node\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf(`{"name":%q,"bin":{%q:%q}}`, entry.packageName, command, "other.js")
	if err := os.WriteFile(filepath.Join(packageRoot, "package.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("PATHEXT", ".COM;.EXE;.BAT;.CMD")
	if _, err := resolveLaunch(command); err == nil || !strings.Contains(err.Error(), "does not match expected") {
		t.Fatalf("expected metadata mismatch, got %v", err)
	}
}

// An installed package manifest must not be able to exhaust ai-profile's RAM
// during ordinary command resolution, even if it is a sparse or growing file.
func TestWindowsVerifiedNPMEntrypointRejectsOversizedManifest(t *testing.T) {
	root := filepath.Join(t.TempDir(), "node_modules", "@xai-official", "grok")
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	manifest, err := os.OpenFile(filepath.Join(root, "package.json"), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	truncateErr := manifest.Truncate(maxNPMManifestBytes + 1)
	closeErr := manifest.Close()
	if truncateErr != nil {
		t.Fatal(truncateErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "grok"), []byte("#!/usr/bin/env node\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, found, err := verifiedNPMEntrypoint(root, "grok", windowsNPMEntrypoints["grok"])
	if err == nil || found || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized manifest unexpectedly accepted: found=%v err=%v", found, err)
	}
}
