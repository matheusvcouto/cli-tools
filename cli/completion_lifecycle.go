package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type completionInstallState string

const (
	completionMissing  completionInstallState = "missing"
	completionCurrent  completionInstallState = "current"
	completionOutdated completionInstallState = "outdated"
	completionForeign  completionInstallState = "foreign"
	completionUnsafe   completionInstallState = "unsafe"
)

type completionTarget struct {
	Shell      Shell
	Path       string
	Activation string
}

type completionInspection struct {
	Target completionTarget
	State  completionInstallState
}

func optionalCompletionShell(inv *Invocation, id string) *Shell {
	name, ok := ValueAs[string](inv, id)
	if !ok {
		return nil
	}
	shell := Shell(name)
	return &shell
}

func (c *CompiledApp) completionTarget(shell Shell) (completionTarget, error) {
	name := c.graph.Name
	if name == "" || filepath.Base(name) != name || name == "." || name == ".." {
		return completionTarget{}, fmt.Errorf("cli: app name %q is unsafe for completion installation", name)
	}
	configHome, err := userConfigDir()
	if err != nil {
		return completionTarget{}, err
	}

	target := completionTarget{Shell: shell}
	switch shell {
	case ShellFish:
		target.Path = filepath.Join(configHome, "fish", "completions", name+".fish")
		target.Activation = "Fish discovers files in its user completions directory automatically."
	case ShellNushell:
		dataHome, err := nushellDataHome(configHome)
		if err != nil {
			return completionTarget{}, err
		}
		target.Path = filepath.Join(dataHome, "nushell", "vendor", "autoload", name+".nu")
		target.Activation = "Nushell 0.114+ discovers vendor/autoload scripts automatically."
	case ShellBash:
		dataHome, err := userDataDir(configHome)
		if err != nil {
			return completionTarget{}, err
		}
		target.Path = filepath.Join(dataHome, "bash-completion", "completions", name)
		target.Activation = "Bash loads this directory when bash-completion user completions are enabled."
	case ShellZsh:
		dataHome, err := userDataDir(configHome)
		if err != nil {
			return completionTarget{}, err
		}
		dir := filepath.Join(dataHome, "zsh", "site-functions")
		target.Path = filepath.Join(dir, "_"+name)
		target.Activation = "Zsh requires this site-functions directory in fpath and compinit to be enabled."
	case ShellPowerShell:
		if runtime.GOOS == "windows" {
			home, err := os.UserHomeDir()
			if err != nil {
				return completionTarget{}, fmt.Errorf("resolve user home directory: %w", err)
			}
			target.Path = filepath.Join(home, "Documents", "PowerShell", "Completions", name+".ps1")
		} else {
			target.Path = filepath.Join(configHome, "powershell", "completions", name+".ps1")
		}
		target.Activation = "PowerShell requires this file to be dot-sourced from a profile or session."
	default:
		return completionTarget{}, fmt.Errorf("cli: unsupported shell %q", shell)
	}
	return target, nil
}

func userConfigDir() (string, error) {
	if raw := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); raw != "" {
		if !filepath.IsAbs(raw) {
			return "", fmt.Errorf("XDG_CONFIG_HOME must be absolute")
		}
		return filepath.Clean(raw), nil
	}
	if runtime.GOOS == "windows" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolve user config directory: %w", err)
		}
		return filepath.Clean(dir), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, ".config"), nil
}

func nushellDataHome(configHome string) (string, error) {
	if raw := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); raw != "" {
		if !filepath.IsAbs(raw) {
			return "", fmt.Errorf("XDG_DATA_HOME must be absolute")
		}
		return filepath.Clean(raw), nil
	}
	if runtime.GOOS == "windows" {
		return configHome, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support"), nil
	}
	return filepath.Join(home, ".local", "share"), nil
}

func userDataDir(configHome string) (string, error) {
	if raw := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); raw != "" {
		if !filepath.IsAbs(raw) {
			return "", fmt.Errorf("XDG_DATA_HOME must be absolute")
		}
		return filepath.Clean(raw), nil
	}
	if runtime.GOOS == "windows" {
		return configHome, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share"), nil
}

func (c *CompiledApp) inspectCompletion(shell Shell) (completionInspection, error) {
	target, err := c.completionTarget(shell)
	if err != nil {
		return completionInspection{}, err
	}
	info, err := os.Lstat(target.Path)
	if os.IsNotExist(err) {
		return completionInspection{Target: target, State: completionMissing}, nil
	}
	if err != nil {
		return completionInspection{}, err
	}
	if !info.Mode().IsRegular() {
		return completionInspection{Target: target, State: completionUnsafe}, nil
	}
	raw, err := os.ReadFile(target.Path)
	if err != nil {
		return completionInspection{}, err
	}
	want, err := c.CompletionScript(shell)
	if err != nil {
		return completionInspection{}, err
	}
	if string(raw) == want {
		return completionInspection{Target: target, State: completionCurrent}, nil
	}
	if generatedCompletion(raw, c.graph.Name) {
		return completionInspection{Target: target, State: completionOutdated}, nil
	}
	return completionInspection{Target: target, State: completionForeign}, nil
}

func generatedCompletion(raw []byte, appName string) bool {
	prefix := raw
	if len(prefix) > 1024 {
		prefix = prefix[:1024]
	}
	text := string(prefix)
	return strings.Contains(text, "generated by "+appName+";") && strings.Contains(text, "completion protocol")
}

func (c *CompiledApp) installCompletions(inv *Invocation, selected *Shell) error {
	shells := completionShellSet(selected)
	inspections := make([]completionInspection, 0, len(shells))
	for _, shell := range shells {
		inspection, err := c.inspectCompletion(shell)
		if err != nil {
			return err
		}
		switch inspection.State {
		case completionUnsafe:
			return completionSafetyError(shell, inspection.Target.Path, "target is not a regular file")
		case completionForeign:
			return completionSafetyError(shell, inspection.Target.Path, "refusing to overwrite a file not generated by this CLI")
		case completionOutdated:
			if runtime.GOOS == "windows" {
				return &Diagnostic{Code: CodeUnavailable, Kind: "completion", Message: fmt.Sprintf("cannot safely replace existing %s completion on Windows: %s", shell, inspection.Target.Path), Hint: "run completion uninstall explicitly, then completion install again", Class: ExitUnavailable}
			}
		}
		inspections = append(inspections, inspection)
	}

	type backup struct {
		inspection completionInspection
		raw        []byte
	}
	var changed []backup
	rollback := func() {
		for i := len(changed) - 1; i >= 0; i-- {
			b := changed[i]
			if b.inspection.State == completionMissing {
				_ = os.Remove(b.inspection.Target.Path)
			} else {
				_ = writeCompletionFile(b.inspection.Target.Path, b.raw)
			}
		}
	}
	for _, inspection := range inspections {
		if inspection.State == completionCurrent {
			continue
		}
		b := backup{inspection: inspection}
		if inspection.State == completionOutdated {
			raw, err := os.ReadFile(inspection.Target.Path)
			if err != nil {
				rollback()
				return err
			}
			b.raw = raw
		}
		script, err := c.CompletionScript(inspection.Target.Shell)
		if err != nil {
			rollback()
			return err
		}
		if err := writeCompletionFile(inspection.Target.Path, []byte(script)); err != nil {
			rollback()
			return err
		}
		changed = append(changed, b)
	}
	for _, inspection := range inspections {
		verb := "installed"
		if inspection.State == completionCurrent {
			verb = "already current"
		}
		if _, err := fmt.Fprintf(inv.IO.Out, "%s completion %s: %s\n", inspection.Target.Shell, verb, inspection.Target.Path); err != nil {
			return err
		}
		if inspection.Target.Activation != "" {
			if _, err := fmt.Fprintln(inv.IO.Out, inspection.Target.Activation); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeCompletionFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".completion-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

func (c *CompiledApp) uninstallCompletions(inv *Invocation, selected *Shell) error {
	shells := completionShellSet(selected)
	inspections := make([]completionInspection, 0, len(shells))
	for _, shell := range shells {
		inspection, err := c.inspectCompletion(shell)
		if err != nil {
			return err
		}
		switch inspection.State {
		case completionUnsafe:
			return completionSafetyError(shell, inspection.Target.Path, "target is not a regular file")
		case completionForeign:
			return completionSafetyError(shell, inspection.Target.Path, "refusing to remove a file not generated by this CLI")
		}
		inspections = append(inspections, inspection)
	}
	type removed struct {
		inspection completionInspection
		raw        []byte
	}
	var removals []removed
	rollback := func() {
		for i := len(removals) - 1; i >= 0; i-- {
			_ = writeCompletionFile(removals[i].inspection.Target.Path, removals[i].raw)
		}
	}
	for _, inspection := range inspections {
		if inspection.State == completionMissing {
			continue
		}
		raw, err := os.ReadFile(inspection.Target.Path)
		if err != nil {
			rollback()
			return err
		}
		if err := os.Remove(inspection.Target.Path); err != nil {
			rollback()
			return err
		}
		removals = append(removals, removed{inspection: inspection, raw: raw})
	}
	for _, inspection := range inspections {
		if inspection.State == completionMissing {
			if _, err := fmt.Fprintf(inv.IO.Out, "%s completion is not installed: %s\n", inspection.Target.Shell, inspection.Target.Path); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(inv.IO.Out, "uninstalled %s completion: %s\n", inspection.Target.Shell, inspection.Target.Path); err != nil {
			return err
		}
	}
	return nil
}

func (c *CompiledApp) completionStatus(inv *Invocation, selected *Shell) error {
	shells := completionShellSet(selected)
	for _, shell := range shells {
		inspection, err := c.inspectCompletion(shell)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(inv.IO.Out, "%s\t%s\t%s\n", shell, inspection.State, inspection.Target.Path); err != nil {
			return err
		}
	}
	return nil
}

func (c *CompiledApp) completionDoctor(inv *Invocation, selected *Shell) error {
	shells := completionShellSet(selected)
	serious := false
	for _, shell := range shells {
		inspection, err := c.inspectCompletion(shell)
		if err != nil {
			return err
		}
		status := "warning"
		switch inspection.State {
		case completionCurrent:
			status = "healthy"
		case completionForeign, completionUnsafe:
			status = "error"
			serious = true
		}

		runtimeStatus, runtimeNote := completionRuntimeDiagnostic(inv.Context, shell, inspection.State)
		if runtimeStatus == "error" {
			status = "error"
			serious = true
		} else if runtimeStatus == "warning" && status == "healthy" {
			status = "warning"
		}

		if _, err := fmt.Fprintf(inv.IO.Out, "%s\t%s\t%s\t%s\n", shell, status, inspection.State, inspection.Target.Path); err != nil {
			return err
		}
		if inspection.Target.Activation != "" {
			if _, err := fmt.Fprintf(inv.IO.Out, "  activation: %s\n", inspection.Target.Activation); err != nil {
				return err
			}
		}
		if runtimeNote != "" {
			if _, err := fmt.Fprintf(inv.IO.Out, "  runtime: %s\n", runtimeNote); err != nil {
				return err
			}
		}
	}
	if serious {
		return &Diagnostic{Code: CodeUnavailable, Kind: "completion", Message: "one or more completion installations are unhealthy", Class: ExitUnavailable}
	}
	return nil
}

const nushellCompletionMinimumMinor = 114

func completionRuntimeDiagnostic(ctx context.Context, shell Shell, state completionInstallState) (status, note string) {
	if shell != ShellNushell || state != completionCurrent {
		return "", ""
	}
	path, err := exec.LookPath("nu")
	if err != nil {
		return "warning", "Nushell executable not found; installed script was not runtime-verified"
	}
	out, err := exec.CommandContext(ctx, path, "--version").CombinedOutput()
	if err != nil {
		return "warning", fmt.Sprintf("could not read Nushell version: %v", err)
	}
	version := strings.TrimSpace(string(out))
	supported, err := nushellCompletionVersionSupported(version)
	if err != nil {
		return "warning", fmt.Sprintf("could not parse Nushell version %q", version)
	}
	if !supported {
		return "error", fmt.Sprintf("Nushell %s is below the completion requirement 0.%d+; the CLI runtime itself remains shell-independent", version, nushellCompletionMinimumMinor)
	}
	return "", ""
}

func nushellCompletionVersionSupported(raw string) (bool, error) {
	var major, minor int
	if _, err := fmt.Sscanf(strings.TrimSpace(raw), "%d.%d", &major, &minor); err != nil {
		return false, err
	}
	if major > 0 {
		return true, nil
	}
	return major == 0 && minor >= nushellCompletionMinimumMinor, nil
}

func completionShellSet(selected *Shell) []Shell {
	if selected != nil {
		return []Shell{*selected}
	}
	names := sortedShellNames()
	out := make([]Shell, len(names))
	for i, name := range names {
		out[i] = Shell(name)
	}
	return out
}

func completionSafetyError(shell Shell, path, message string) error {
	return &Diagnostic{Code: CodeUnavailable, Kind: "completion", Message: fmt.Sprintf("%s completion at %s: %s", shell, path, message), Class: ExitUnavailable}
}
