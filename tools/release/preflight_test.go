package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A prepared release has no top-level change records. Archived JSON records
// must not prevent a tag, but an invalid/linked pending record must not vanish
// from the preflight just because it isn't a regular file.
func TestRejectPendingReleaseChanges(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "archive", "v2.0.0")
	if err := os.MkdirAll(archive, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archive, "old.json"), []byte(`{"archived":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := rejectPendingReleaseChanges(dir); err != nil {
		t.Fatalf("archived change records must be permitted: %v", err)
	}
	pending := filepath.Join(dir, "pending.json")
	if err := os.WriteFile(pending, []byte(`{"schema_version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := rejectPendingReleaseChanges(dir); err == nil || !strings.Contains(err.Error(), "pending.json") {
		t.Fatalf("unprepared release must be rejected: %v", err)
	}
	if err := os.Remove(pending); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(archive, "old.json"), pending); err == nil {
		if err := rejectPendingReleaseChanges(dir); err == nil || !strings.Contains(err.Error(), "pending.json") {
			t.Fatalf("symlinked pending record must be rejected: %v", err)
		}
	} else {
		t.Logf("symlink creation unavailable on this filesystem: %v", err)
	}
}

func TestRejectPendingReleaseChangesRequiresDirectory(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-changes")
	if err := rejectPendingReleaseChanges(missing); err == nil {
		t.Fatal("missing changes/ must not count as a prepared release")
	}
}
