//go:build windows

package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

func platformUserConfigDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Clean(dir), nil
}

func platformNushellDataHome(configHome string) (string, error) { return configHome, nil }

func platformUserDataDir(configHome string) (string, error) { return configHome, nil }

func platformPowerShellCompletionPath(name, _ string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, "Documents", "PowerShell", "Completions", name+".ps1"), nil
}
