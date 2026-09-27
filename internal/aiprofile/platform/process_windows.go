//go:build windows

package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/matheusvcouto/cli-tools/internal/aiprofile"
)

type Runner struct{}

type childExitError struct {
	code int
}

type launchSpec struct {
	path       string
	prefixArgs []string
}

type npmEntrypoint struct {
	packageName string
	entrypoint  string
}

// These commands are officially distributed as npm command shims on Windows.
// Running a .cmd shim would require cmd.exe and would turn otherwise opaque
// child arguments into shell syntax. Resolve the package entrypoint instead and
// invoke it with node.exe directly so argv remains a real argument vector.
var windowsNPMEntrypoints = map[string]npmEntrypoint{
	"claude": {
		packageName: "@anthropic-ai/claude-code",
		entrypoint:  "cli.js",
	},
	"codex": {
		packageName: "@openai/codex",
		entrypoint:  "bin/codex.js",
	},
	"grok": {
		// Official @xai-official/grok npm shim points to the Node bootstrap;
		// bootstrap delegates to the platform-specific native binary.
		// Never invoke a .cmd/.ps1 shim with opaque user/ACP arguments.
		packageName: "@xai-official/grok",
		entrypoint:  "bin/grok",
	},
	"claude-agent-acp": {
		packageName: "@agentclientprotocol/claude-agent-acp",
		entrypoint:  "dist/index.js",
	},
	"codex-acp": {
		packageName: "@agentclientprotocol/codex-acp",
		entrypoint:  "dist/index.js",
	},
}

func (e *childExitError) Error() string {
	return fmt.Sprintf("child process exited with status %d", e.code)
}

func (e *childExitError) childExitCode() int { return e.code }

// NormalizeEnvKey exposes Windows' case-insensitive environment-key semantics
// to the platform-independent profile isolation logic.
func (Runner) NormalizeEnvKey(key string) string { return strings.ToUpper(key) }

func (Runner) Replace(ctx context.Context, binary string, args []string, env []string, io aiprofile.ProcessIO) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	launch, err := resolveLaunch(binary)
	if err != nil {
		return err
	}

	argv := make([]string, 0, len(launch.prefixArgs)+len(args))
	argv = append(argv, launch.prefixArgs...)
	argv = append(argv, args...)
	cmd := exec.CommandContext(ctx, launch.path, argv...)
	cmd.Env = env
	cmd.Stdin = io.In
	cmd.Stdout = io.Out
	cmd.Stderr = io.Err
	// Start suspended so the child cannot create descendants before it has been
	// placed in the Job Object. Resume only after containment succeeds.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createSuspended}

	job, err := newWindowsJob()
	if err != nil {
		return fmt.Errorf("prepare Windows process containment: %w", err)
	}
	defer job.Close()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", binary, err)
	}
	if err := job.Assign(cmd.Process.Pid); err != nil {
		// Fail closed: the process is still suspended, so it has not executed any
		// agent code or created descendants.
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("contain %s process tree: %w", binary, err)
	}
	if err := resumeSuspendedProcess(cmd.Process.Pid); err != nil {
		_ = job.Terminate(1)
		_ = cmd.Wait()
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("resume contained %s process: %w", binary, err)
	}

	runErr := cmd.Wait()
	// The command may intentionally or accidentally leave descendants alive.
	// Terminate the job after the direct child exits so run/acp never leaks a
	// background agent tree. KILL_ON_JOB_CLOSE provides the same cleanup if this
	// wrapper exits unexpectedly after assignment.
	cleanupErr := job.Terminate(1)
	if cleanupErr != nil {
		// Do not report an ordinary child status when descendant containment may
		// have failed; surface the stronger runtime failure instead.
		return fmt.Errorf("cleanup %s process tree: %w", binary, cleanupErr)
	}
	if runErr != nil {
		// CommandContext terminates the direct child when the context is canceled;
		// the job cleanup above handles its descendants. Attribute that termination
		// to the context before interpreting the OS status as a child exit code.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		var exit *exec.ExitError
		if errors.As(runErr, &exit) {
			if code := exit.ExitCode(); code >= 0 {
				return &childExitError{code: code}
			}
		}
		return fmt.Errorf("run %s: %w", binary, runErr)
	}
	return nil
}

func resolveLaunch(binary string) (launchSpec, error) {
	// Let LookPath honor PATH directory order and PATHEXT as Windows defines
	// them. Searching for binary+.exe first across *all* PATH directories could
	// select a stale executable from a later installation ahead of the intended
	// npm .cmd shim in an earlier directory, silently changing the provider.
	// A selected .cmd is resolved to its validated Node entrypoint below;
	// opaque user arguments are never passed through cmd.exe.
	path, err := exec.LookPath(binary)
	if err != nil {
		return launchSpec{}, fmt.Errorf("find %s in PATH: %w", binary, err)
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".cmd", ".bat":
		entry, ok := windowsNPMEntrypoints[strings.ToLower(filepath.Base(strings.TrimSuffix(binary, filepath.Ext(binary))))]
		if !ok || ext != ".cmd" {
			return launchSpec{}, fmt.Errorf("%s resolves to a Windows shell wrapper %q; refusing to pass opaque arguments through cmd.exe", binary, path)
		}
		return resolveNPMShim(path, binary, entry)
	case ".ps1":
		return launchSpec{}, fmt.Errorf("%s resolves to a PowerShell wrapper %q; refusing to pass opaque arguments through a shell", binary, path)
	default:
		return launchSpec{path: path}, nil
	}
}

type npmPackageManifest struct {
	Name string            `json:"name"`
	Bin  map[string]string `json:"bin"`
}

// package.json comes from the user's local tool installation. Treat it as
// untrusted metadata; resolving a command must not require reading a
// potentially multi-gigabyte or concurrently growing file into memory.
const maxNPMManifestBytes int64 = 2 << 20

func npmDirectPackageRootForShim(absShim, packageName string) string {
	shimDir := filepath.Dir(absShim)
	packagePath := filepath.FromSlash(packageName)
	if strings.EqualFold(filepath.Base(shimDir), ".bin") {
		nodeModules := filepath.Dir(shimDir)
		if strings.EqualFold(filepath.Base(nodeModules), "node_modules") {
			return filepath.Join(nodeModules, packagePath)
		}
	}
	return filepath.Join(shimDir, "node_modules", packagePath)
}

func pnpmPackageRootsForShim(absShim, packageName string) ([]string, error) {
	shimDir := filepath.Dir(absShim)
	packagePath := filepath.FromSlash(packageName)
	seen := make(map[string]struct{})
	var roots []string

	// pnpm 11+ places global command shims in <PNPM_HOME>\bin while global
	// packages live below <PNPM_HOME>\global\vN\node_modules. pnpm <=10
	// used <PNPM_HOME> itself as the global bin directory. Derive both forms
	// from the shim location. A configured PNPM_HOME is accepted only when it
	// actually corresponds to the directory that supplied the active shim.
	var homes []string
	if strings.EqualFold(filepath.Base(shimDir), "bin") {
		homes = append(homes, filepath.Dir(shimDir))
	} else {
		homes = append(homes, shimDir)
	}
	pnpmHomeMatches := false
	if home := strings.TrimSpace(os.Getenv("PNPM_HOME")); home != "" {
		absHome, err := filepath.Abs(home)
		if err != nil {
			return nil, fmt.Errorf("resolve PNPM_HOME: %w", err)
		}
		if strings.EqualFold(filepath.Clean(shimDir), filepath.Clean(absHome)) ||
			strings.EqualFold(filepath.Clean(shimDir), filepath.Join(filepath.Clean(absHome), "bin")) {
			homes = append(homes, absHome)
			pnpmHomeMatches = true
		}
	}
	for _, home := range homes {
		if err := appendPNPMGlobalPackageRoots(&roots, seen, filepath.Join(home, "global"), packagePath); err != nil {
			return nil, err
		}
	}
	// A custom global package directory is meaningful only when the shim is
	// demonstrably from this pnpm installation (PNPM_HOME match) or when its
	// configured global bin directory is exactly the shim directory.
	globalBinMatches := false
	if globalBin := strings.TrimSpace(os.Getenv("PNPM_CONFIG_GLOBAL_BIN_DIR")); globalBin != "" {
		absBin, err := filepath.Abs(globalBin)
		if err != nil {
			return nil, fmt.Errorf("resolve PNPM_CONFIG_GLOBAL_BIN_DIR: %w", err)
		}
		globalBinMatches = strings.EqualFold(filepath.Clean(absBin), filepath.Clean(shimDir))
	}
	if globalDir := strings.TrimSpace(os.Getenv("PNPM_CONFIG_GLOBAL_DIR")); globalDir != "" && (pnpmHomeMatches || globalBinMatches) {
		absGlobal, err := filepath.Abs(globalDir)
		if err != nil {
			return nil, fmt.Errorf("resolve PNPM_CONFIG_GLOBAL_DIR: %w", err)
		}
		if err := appendPNPMGlobalPackageRoots(&roots, seen, absGlobal, packagePath); err != nil {
			return nil, err
		}
	}
	return roots, nil
}

func appendPNPMGlobalPackageRoots(roots *[]string, seen map[string]struct{}, globalDir, packagePath string) error {
	add := func(path string) {
		clean := filepath.Clean(path)
		key := strings.ToUpper(clean)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		*roots = append(*roots, clean)
	}
	// Accept a directly materialized global node_modules layout as well as the
	// versioned vN layout used by current pnpm releases.
	add(filepath.Join(globalDir, "node_modules", packagePath))
	entries, err := os.ReadDir(globalDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect pnpm global package directory %q: %w", globalDir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		add(filepath.Join(globalDir, entry.Name(), "node_modules", packagePath))
	}
	return nil
}

func resolveNPMShim(shimPath, command string, expected npmEntrypoint) (launchSpec, error) {
	absShim, err := filepath.Abs(shimPath)
	if err != nil {
		return launchSpec{}, fmt.Errorf("resolve npm shim path for %s: %w", command, err)
	}
	directRoot := npmDirectPackageRootForShim(absShim, expected.packageName)
	entryPath, found, err := verifiedNPMEntrypoint(directRoot, command, expected)
	if err != nil {
		return launchSpec{}, fmt.Errorf("verify package for shim %q: %w", absShim, err)
	}
	if !found {
		packageRoots, err := pnpmPackageRootsForShim(absShim, expected.packageName)
		if err != nil {
			return launchSpec{}, err
		}
		for _, packageRoot := range packageRoots {
			candidate, found, err := verifiedNPMEntrypoint(packageRoot, command, expected)
			if err != nil {
				return launchSpec{}, fmt.Errorf("verify package for shim %q: %w", absShim, err)
			}
			if !found {
				continue
			}
			if entryPath != "" && !strings.EqualFold(filepath.Clean(entryPath), filepath.Clean(candidate)) {
				return launchSpec{}, fmt.Errorf("%s shim %q resolves to multiple verified package roots; refusing ambiguous launch", command, absShim)
			}
			entryPath = candidate
		}
	}
	if entryPath == "" {
		return launchSpec{}, fmt.Errorf("%s resolves to npm/pnpm shim %q but the expected package manifest is unavailable", command, absShim)
	}

	nodePath, err := exec.LookPath("node.exe")
	if err != nil {
		return launchSpec{}, fmt.Errorf("%s is installed through an npm-compatible package manager but node.exe is not available in PATH: %w", command, err)
	}
	if !strings.EqualFold(filepath.Ext(nodePath), ".exe") {
		return launchSpec{}, fmt.Errorf("node resolves to non-native executable %q", nodePath)
	}
	return launchSpec{path: nodePath, prefixArgs: []string{entryPath}}, nil
}

func verifiedNPMEntrypoint(packageRoot, command string, expected npmEntrypoint) (string, bool, error) {
	manifestPath := filepath.Join(packageRoot, "package.json")
	entryPath := filepath.Join(packageRoot, filepath.FromSlash(expected.entrypoint))
	manifestInfo, err := os.Lstat(manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("inspect package manifest for %s: %w", command, err)
	}
	if manifestInfo.Mode()&os.ModeSymlink != 0 || !manifestInfo.Mode().IsRegular() {
		return "", false, fmt.Errorf("refuse non-regular package manifest for %s: %s", command, manifestPath)
	}
	if manifestInfo.Size() > maxNPMManifestBytes {
		return "", false, fmt.Errorf("package manifest for %s exceeds %d bytes", command, maxNPMManifestBytes)
	}
	f, err := os.Open(manifestPath)
	if err != nil {
		return "", false, fmt.Errorf("open package manifest for %s: %w", command, err)
	}
	opened, statErr := f.Stat()
	if statErr != nil {
		_ = f.Close()
		return "", false, fmt.Errorf("inspect opened package manifest for %s: %w", command, statErr)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(manifestInfo, opened) {
		_ = f.Close()
		return "", false, fmt.Errorf("package manifest for %s changed identity while opening", command)
	}
	raw, readErr := io.ReadAll(io.LimitReader(f, maxNPMManifestBytes+1))
	closeErr := f.Close()
	if readErr != nil {
		return "", false, fmt.Errorf("read package manifest for %s: %w", command, readErr)
	}
	if closeErr != nil {
		return "", false, fmt.Errorf("close package manifest for %s: %w", command, closeErr)
	}
	if int64(len(raw)) > maxNPMManifestBytes {
		return "", false, fmt.Errorf("package manifest for %s exceeds %d bytes", command, maxNPMManifestBytes)
	}
	current, err := os.Lstat(manifestPath)
	if err != nil {
		return "", false, fmt.Errorf("reinspect package manifest for %s: %w", command, err)
	}
	if !current.Mode().IsRegular() || !os.SameFile(opened, current) {
		return "", false, fmt.Errorf("package manifest for %s changed identity during verification", command)
	}
	var manifest npmPackageManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return "", false, fmt.Errorf("decode package manifest for %s: %w", command, err)
	}
	commandName := strings.ToLower(filepath.Base(strings.TrimSuffix(command, filepath.Ext(command))))
	declared, ok := manifest.Bin[commandName]
	if manifest.Name != expected.packageName || !ok || filepath.Clean(filepath.FromSlash(declared)) != filepath.Clean(filepath.FromSlash(expected.entrypoint)) {
		return "", false, fmt.Errorf("package metadata does not match expected %s command", command)
	}

	entryInfo, err := os.Lstat(entryPath)
	if err != nil {
		return "", false, fmt.Errorf("inspect package entrypoint for %s: %w", command, err)
	}
	if entryInfo.Mode()&os.ModeSymlink != 0 || !entryInfo.Mode().IsRegular() {
		return "", false, fmt.Errorf("refuse non-regular package entrypoint for %s: %s", command, entryPath)
	}
	return entryPath, true, nil
}
