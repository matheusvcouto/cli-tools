// Package testenv builds minimal subprocess environments for tests.
// It intentionally omits user credentials, shell startup state, Git overrides,
// Go configuration, editors, hooks, SSH agents, and cloud/provider variables.
package testenv

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// New creates a hermetic-ish local environment rooted at root.
// It keeps only PATH plus a few OS variables needed to execute local tools.
// Go is forced offline/local and Git global/system config + hooks are disabled.
func New(root string) ([]string, error) {
	dirs := []string{
		filepath.Join(root, "home"),
		filepath.Join(root, "tmp"),
		filepath.Join(root, "xdg", "config"),
		filepath.Join(root, "xdg", "cache"),
		filepath.Join(root, "xdg", "state"),
		filepath.Join(root, "go", "cache"),
		filepath.Join(root, "go", "modcache"),
		filepath.Join(root, "go", "path"),
		filepath.Join(root, "git", "hooks"),
		filepath.Join(root, "git", "template"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create test sandbox %q: %w", dir, err)
		}
	}

	home := filepath.Join(root, "home")
	tmp := filepath.Join(root, "tmp")
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"USERPROFILE=" + home,
		"TMPDIR=" + tmp,
		"TMP=" + tmp,
		"TEMP=" + tmp,
		"XDG_CONFIG_HOME=" + filepath.Join(root, "xdg", "config"),
		"XDG_CACHE_HOME=" + filepath.Join(root, "xdg", "cache"),
		"XDG_STATE_HOME=" + filepath.Join(root, "xdg", "state"),
		"LANG=C",
		"LC_ALL=C",
		"TZ=UTC",
		"TERM=dumb",

		// Go: no user GOENV/workspace/toolchain downloads/network/caches.
		"GOTOOLCHAIN=local",
		"GOENV=off",
		"GOWORK=off",
		"GOPROXY=off",
		"GOSUMDB=off",
		"GOVCS=*:off",
		"GOTELEMETRY=off",
		"GOCACHE=" + filepath.Join(root, "go", "cache"),
		"GOMODCACHE=" + filepath.Join(root, "go", "modcache"),
		"GOPATH=" + filepath.Join(root, "go", "path"),

		// Git: no system/global config, templates, user hooks or prompts.
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=" + filepath.Join(root, "git", "no-global-config"),
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=core.hooksPath",
		"GIT_CONFIG_VALUE_0=" + filepath.Join(root, "git", "hooks"),
		"GIT_CONFIG_KEY_1=init.templateDir",
		"GIT_CONFIG_VALUE_1=" + filepath.Join(root, "git", "template"),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_PAGER=cat",
		"PAGER=cat",
	}

	// exec.Cmd on Windows needs these to find/launch executables reliably.
	if runtime.GOOS == "windows" {
		for _, key := range []string{"SystemRoot", "WINDIR", "ComSpec", "PATHEXT"} {
			if value := os.Getenv(key); value != "" {
				env = append(env, key+"="+value)
			}
		}
	}
	return env, nil
}

func Set(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, item := range env {
		if len(item) >= len(prefix) && item[:len(prefix)] == prefix {
			continue
		}
		out = append(out, item)
	}
	return append(out, prefix+value)
}
