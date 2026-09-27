//go:build windows

package repozip

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/matheusvcouto/cli-tools/v2/internal/safefs"
)

func TestWindowsPublishNoClobberPublishesAndRemovesTempName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "archive.tmp"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := safefs.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	if err := publishArchive(root, "archive.tmp", "archive.zip", false); err != nil {
		t.Fatalf("publish no-clobber: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "archive.zip"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "new" {
		t.Fatalf("published content = %q", raw)
	}
	if _, err := os.Lstat(filepath.Join(dir, "archive.tmp")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary name remains: %v", err)
	}
}

func TestWindowsPublishNoClobberNeverOverwritesExistingDestination(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "archive.tmp"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "archive.zip"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := safefs.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	if err := publishArchive(root, "archive.tmp", "archive.zip", false); err == nil {
		t.Fatal("expected no-clobber publication to refuse existing destination")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "archive.zip"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "old" {
		t.Fatalf("existing destination was modified: %q", raw)
	}
	if _, err := os.Stat(filepath.Join(dir, "archive.tmp")); err != nil {
		t.Fatalf("temporary source should remain after failed publication: %v", err)
	}
}

func TestWindowsPublishForceReplacesExistingDestination(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "archive.tmp"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "archive.zip"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := safefs.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	if err := publishArchive(root, "archive.tmp", "archive.zip", true); err != nil {
		t.Fatalf("force publish: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "archive.zip"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "new" {
		t.Fatalf("replacement content = %q", raw)
	}
}
