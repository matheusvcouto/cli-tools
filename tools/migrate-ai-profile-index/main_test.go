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
