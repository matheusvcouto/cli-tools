//go:build windows

package main

import (
	"os"
	"path/filepath"
)

// Require native executables on Windows. os/exec otherwise consults PATHEXT,
// which may resolve .cmd/.bat wrappers with cmd.exe-specific argument parsing.
func goCommandName() string  { return "go.exe" }
func gitCommandName() string { return "git.exe" }

func platformContractEnv(home, config, tmp string) []string {
	env := []string{
		"USERPROFILE=" + home,
		"APPDATA=" + filepath.Join(config, "roaming"),
		"LOCALAPPDATA=" + filepath.Join(config, "local"),
		"TMP=" + tmp,
		"TEMP=" + tmp,
	}
	// Windows process creation and native tools rely on these OS variables. Keep
	// only the execution-critical values; user config/credentials remain rooted
	// in the sandbox above.
	for _, key := range []string{"SystemRoot", "WINDIR", "ComSpec", "PATHEXT"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}
