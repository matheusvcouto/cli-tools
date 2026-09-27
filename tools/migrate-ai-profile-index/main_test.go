package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/matheusvcouto/cli-tools/internal/aiprofile"
)

func TestParseLegacyTable(t *testing.T) {
	input := `[[tool, alias, dir, created_at]; [claude, personal, "/tmp/profiles/claude-id", "2026-01-02T03:04:05"], [codex, work, /tmp/profiles/codex-id, 2026-02-03T04:05:06]]`
	got, err := parseLegacyNUON(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Alias != "personal" || got[1].CreatedAt != "2026-02-03T04:05:06" {
		t.Fatalf("unexpected parse: %+v", got)
	}
}

func TestMigratePreservesFieldsAndSource(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "profiles")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	d1 := filepath.Join(root, "claude-id")
	d2 := filepath.Join(root, "codex-id")
	if err := os.Mkdir(d1, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(d2, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(base, "index.nuon")
	input := `[[tool, alias, dir, created_at]; [claude, personal, "` + d1 + `", "2026-01-02T03:04:05"], [codex, work, "` + d2 + `", "2026-02-03T04:05:06"]]`
	if err := os.WriteFile(source, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := migrate(source, root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("source was removed: %v", err)
	}
	data, err := (aiprofile.Store{Root: root}).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Profiles) != 2 || data.Profiles[0].Dir != d1 || data.Profiles[0].CreatedAt != "2026-01-02T03:04:05" {
		t.Fatalf("migration lost data: %+v", data)
	}
	if err := migrate(source, root); err == nil {
		t.Fatal("expected overwrite refusal")
	}
}

// An explicit migration may adopt arbitrary legacy leaf directory names, but
// it must refuse root entries absent from the source instead of orphaning data.
func TestMigrateRefusesUnreferencedLegacyState(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "profiles")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "claude-id")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(root, "unlisted")
	if err := os.Mkdir(orphan, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(base, "index.nuon")
	input := `[[tool, alias, dir, created_at]; [claude, personal, "` + dir + `", "2026-01-02T03:04:05"]]`
	if err := os.WriteFile(source, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := migrate(source, root); err == nil {
		t.Fatal("migration adopted an incomplete source index")
	}
	if _, err := os.Lstat(filepath.Join(root, "index.json")); !os.IsNotExist(err) {
		t.Fatalf("migration created index despite leftover data: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(orphan); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal(err)
	}
}

// The original Nushell index is ~/.ai-profiles/index.nuon: a correct import
// must preserve that exact file inside the destination without treating it as
// an unreferenced artifact, while refusing every other unexpected entry.
func TestMigratePreservesLegacyIndexInsideDestinationRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "profiles")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	legacyDir := filepath.Join(root, "claude-id")
	if err := os.Mkdir(legacyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "index.nuon")
	content := `[[tool, alias, dir, created_at]; [claude, personal, "` + legacyDir + `", "2026-01-02T03:04:05"]]`
	if err := os.WriteFile(source, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := migrate(source, root); err != nil {
		t.Fatalf("same-root migration: %v", err)
	}
	got, err := os.ReadFile(source)
	if err != nil || string(got) != content {
		t.Fatalf("legacy source was modified: %q / %v", got, err)
	}
	data, err := (aiprofile.Store{Root: root}).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Profiles) != 1 || data.Profiles[0].Dir != legacyDir {
		t.Fatalf("migration did not adopt legacy directory: %+v", data)
	}
}

func TestReadLegacyIndexRejectsOversizedInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.nuon")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxLegacyIndexBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := readLegacyIndex(path); err == nil {
		t.Fatal("unbounded legacy index accepted")
	}
}
