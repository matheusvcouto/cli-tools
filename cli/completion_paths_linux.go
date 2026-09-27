//go:build linux

package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

func platformUserConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, ".config"), nil
}

func platformNushellDataHome(_ string) (string, error) {
	return platformUserDataDir("")
}

func platformUserDataDir(_ string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share"), nil
}

func platformPowerShellCompletionPath(name, configHome string) (string, error) {
	return filepath.Join(configHome, "powershell", "completions", name+".ps1"), nil
}
