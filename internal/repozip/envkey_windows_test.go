//go:build windows

package repozip

import (
	"strings"
	"testing"
)

func TestSanitizedGitEnvUsesWindowsCaseInsensitiveKeys(t *testing.T) {
	env := []string{
		"PATH=C:\\Windows\\System32",
		"git_dir=C:\\attacker",
		"Git_Work_Tree=C:\\attacker-worktree",
		"KEEP=yes",
	}
	got := sanitizedGitEnv(env)
	joined := strings.Join(got, "\n")
	if strings.Contains(strings.ToLower(joined), "git_dir=") || strings.Contains(strings.ToLower(joined), "git_work_tree=") {
		t.Fatalf("Git override leaked through Windows case-insensitive environment filtering: %q", got)
	}
	if !strings.Contains(joined, "KEEP=yes") {
		t.Fatalf("unrelated environment entry removed: %q", got)
	}
}
