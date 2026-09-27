//go:build windows

package fscommit

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

func TestReplaceRootReplacesExistingFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "next.json"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := safefs.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	if err := ReplaceRoot(root, "next.json", "index.json"); err != nil {
		t.Fatalf("replace: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "new" {
		t.Fatalf("destination = %q, want new", raw)
	}
	if _, err := os.Lstat(filepath.Join(dir, "next.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source still exists after replace: %v", err)
	}
}

func TestReplaceRootFailureDoesNotDeleteDestination(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := safefs.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	if err := ReplaceRoot(root, "missing.json", "index.json"); err == nil {
		t.Fatal("expected missing source to fail")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil {
		t.Fatalf("destination disappeared after failed replace: %v", err)
	}
	if string(raw) != "keep" {
		t.Fatalf("destination changed after failed replace: %q", raw)
	}
}
