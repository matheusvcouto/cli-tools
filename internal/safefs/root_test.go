package safefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootRejectsParentEscape(t *testing.T) {
	base := t.TempDir()
	r, err := Open(base)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, err := r.Open("../outside"); err == nil {
		t.Fatal("expected parent escape rejection")
	}
}

func TestRootBasicOperations(t *testing.T) {
	base := t.TempDir()
	r, err := Open(base)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.MkdirAll("a/b", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "a", "b", "x"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := r.Open("a/b/x")
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if err := r.Rename("a/b/x", "a/b/y"); err != nil {
		t.Fatal(err)
	}
	if err := r.RemoveAll("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(base, "a")); !os.IsNotExist(err) {
		t.Fatalf("remove failed: %v", err)
	}
}

func TestRootRejectsRootSymlink(t *testing.T) {
	base := t.TempDir()
	realRoot := filepath.Join(base, "real")
	if err := os.Mkdir(realRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	linkRoot := filepath.Join(base, "link")
	if err := os.Symlink(realRoot, linkRoot); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := Open(linkRoot); err == nil {
		t.Fatal("expected safe root symlink rejection")
	}
}

func TestEnsureDirCreatesNestedDirectoryInsideRealAncestor(t *testing.T) {
	base := t.TempDir()
	want := filepath.Join(base, "a", "b", "c")
	got, err := EnsureDir(want, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("EnsureDir path=%q want %q", got, want)
	}
	info, err := os.Lstat(want)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("created path is not a real directory: %v", info.Mode())
	}
}

func TestEnsureDirRejectsSymlinkedParentWithoutCreatingOutside(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(base, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	outsideChild := filepath.Join(outside, "must-not-exist")
	if _, err := EnsureDir(filepath.Join(link, "must-not-exist"), 0o755); err == nil {
		t.Fatal("expected symlinked parent rejection")
	}
	if _, err := os.Lstat(outsideChild); !os.IsNotExist(err) {
		t.Fatalf("outside directory was created through symlink: %v", err)
	}
}
