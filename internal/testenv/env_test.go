package testenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewDoesNotInheritUserSecretsOrToolOverrides(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "synthetic-secret-sentinel")
	t.Setenv("ANTHROPIC_API_KEY", "synthetic-secret-sentinel")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "synthetic-secret-sentinel")
	t.Setenv("SSH_AUTH_SOCK", "/synthetic/agent.sock")
	t.Setenv("GIT_EXEC_PATH", "/malicious/git-core")
	t.Setenv("GIT_TEMPLATE_DIR", "/malicious/templates")
	t.Setenv("GIT_DIR", "/synthetic/repository/.git")
	t.Setenv("GOFLAGS", "-toolexec=/malicious/wrapper")
	t.Setenv("GOPROXY", "https://example.invalid")

	root := t.TempDir()
	env, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	got := envMap(env)
	for _, key := range []string{
		"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "AWS_SECRET_ACCESS_KEY", "SSH_AUTH_SOCK",
		"GIT_EXEC_PATH", "GIT_TEMPLATE_DIR", "GIT_DIR", "GOFLAGS",
	} {
		if _, ok := got[key]; ok {
			t.Fatalf("sensitive/override variable %s leaked into test environment", key)
		}
	}
	if got["GOPROXY"] != "off" || got["GOTOOLCHAIN"] != "local" || got["GOENV"] != "off" || got["GOWORK"] != "off" {
		t.Fatalf("Go sandbox controls missing: %#v", got)
	}
	if got["GIT_CONFIG_NOSYSTEM"] != "1" || got["GIT_TERMINAL_PROMPT"] != "0" {
		t.Fatalf("Git sandbox controls missing: %#v", got)
	}
	for _, key := range []string{"HOME", "TMPDIR", "GOCACHE", "GOMODCACHE", "GOPATH"} {
		value := got[key]
		if value == "" {
			t.Fatalf("%s is empty", key)
		}
		rel, err := filepath.Rel(root, value)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			t.Fatalf("%s escaped sandbox: %q", key, value)
		}
	}
}

func envMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, item := range env {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			out[key] = value
		}
	}
	return out
}
