package toolhome

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func fakeLookup(home string, env map[string]string) Lookup {
	return Lookup{
		LookupEnv: func(key string) (string, bool) {
			value, ok := env[key]
			return value, ok
		},
		UserHomeDir: func() (string, error) { return home, nil },
	}
}

func TestResolveDefaultsUnderHome(t *testing.T) {
	home := t.TempDir()
	h, err := Resolve("skills", fakeLookup(home, nil))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, DirName, "skills")
	if h.Dir() != want {
		t.Fatalf("Dir() = %q, want %q", h.Dir(), want)
	}
	if h.Tool() != "skills" {
		t.Fatalf("Tool() = %q", h.Tool())
	}
	layout := map[string]string{
		h.ConfigFile(): filepath.Join(want, "config.toml"),
		h.StateDir():   filepath.Join(want, "state"),
		h.CacheDir():   filepath.Join(want, "cache"),
		h.BackupsDir(): filepath.Join(want, "backups"),
	}
	for got, want := range layout {
		if got != want {
			t.Errorf("layout path = %q, want %q", got, want)
		}
	}
	if _, err := os.Lstat(filepath.Join(home, DirName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Resolve must not touch the filesystem, stat err = %v", err)
	}
}

func TestResolveEnvOverride(t *testing.T) {
	home := t.TempDir()
	override := t.TempDir()
	h, err := Resolve("skills", fakeLookup(home, map[string]string{EnvRoot: "  " + override + "  "}))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(override, "skills"); h.Dir() != want {
		t.Fatalf("Dir() = %q, want %q", h.Dir(), want)
	}
}

func TestResolveBlankEnvIsUnset(t *testing.T) {
	home := t.TempDir()
	h, err := Resolve("skills", fakeLookup(home, map[string]string{EnvRoot: "   "}))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, DirName, "skills"); h.Dir() != want {
		t.Fatalf("Dir() = %q, want %q", h.Dir(), want)
	}
}

func TestResolveRejectsRelativeEnv(t *testing.T) {
	_, err := Resolve("skills", fakeLookup(t.TempDir(), map[string]string{EnvRoot: "relative/dir"}))
	if err == nil || !strings.Contains(err.Error(), EnvRoot) {
		t.Fatalf("expected %s error, got %v", EnvRoot, err)
	}
}

func TestResolveRejectsRelativeHome(t *testing.T) {
	if _, err := Resolve("skills", fakeLookup("relative-home", nil)); err == nil {
		t.Fatal("expected error for relative home directory")
	}
}

func TestResolveHomeLookupError(t *testing.T) {
	lookup := Lookup{UserHomeDir: func() (string, error) { return "", errors.New("no home") }}
	if _, err := Resolve("skills", lookup); err == nil {
		t.Fatal("expected home lookup error")
	}
}

func TestResolveRejectsInvalidToolNames(t *testing.T) {
	for _, name := range []string{"", "Skills", "-skills", "skills-", "a--b", "a/b", "..", "a b", `a\b`, "skills.v2"} {
		if _, err := Resolve(name, fakeLookup(t.TempDir(), nil)); err == nil {
			t.Errorf("Resolve(%q) succeeded, want error", name)
		}
	}
	for _, name := range []string{"skills", "repo-zip", "ai-profile", "x1"} {
		if _, err := Resolve(name, fakeLookup(t.TempDir(), nil)); err != nil {
			t.Errorf("Resolve(%q) = %v, want success", name, err)
		}
	}
}

func TestEnsureCreatesPrivateDirectories(t *testing.T) {
	h, err := Resolve("skills", fakeLookup(t.TempDir(), nil))
	if err != nil {
		t.Fatal(err)
	}
	path, err := h.Ensure("backups")
	if err != nil {
		t.Fatal(err)
	}
	same, err := sameDir(path, h.BackupsDir())
	if err != nil || !same {
		t.Fatalf("Ensure returned %q, want %q (err %v)", path, h.BackupsDir(), err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(h.Dir())
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != dirPerm {
			t.Fatalf("tool dir perm = %o, want %o", perm, dirPerm)
		}
	}
}

func TestEnsureRejectsNonLocalPaths(t *testing.T) {
	h, err := Resolve("skills", fakeLookup(t.TempDir(), nil))
	if err != nil {
		t.Fatal(err)
	}
	escape := filepath.Join("..", "other")
	for _, sub := range []string{escape, filepath.Join(t.TempDir(), "abs")} {
		if _, err := h.Ensure(sub); err == nil {
			t.Errorf("Ensure(%q) succeeded, want error", sub)
		}
	}
}

func TestEnsureOnZeroHomeFails(t *testing.T) {
	if _, err := (Home{}).Ensure(""); err == nil {
		t.Fatal("expected error for zero Home")
	}
}

func TestEnsureRejectsSymlinkedToolDir(t *testing.T) {
	home := t.TempDir()
	h, err := Resolve("skills", fakeLookup(home, nil))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(h.Dir()), 0o700); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, h.Dir()); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := h.Ensure("state"); err == nil {
		t.Fatal("Ensure followed a symlinked tool directory")
	}
	if _, err := os.Lstat(filepath.Join(outside, "state")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("created through symlink: %v", err)
	}
}

func TestEnsureConfigCreatesOnceAndNeverOverwrites(t *testing.T) {
	h, err := Resolve("skills", fakeLookup(t.TempDir(), nil))
	if err != nil {
		t.Fatal(err)
	}
	created, err := h.EnsureConfig([]byte("first = true\n"))
	if err != nil || !created {
		t.Fatalf("first EnsureConfig = %v, %v; want true, nil", created, err)
	}
	created, err = h.EnsureConfig([]byte("second = true\n"))
	if err != nil || created {
		t.Fatalf("second EnsureConfig = %v, %v; want false, nil", created, err)
	}
	data, err := os.ReadFile(h.ConfigFile())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "first = true\n" {
		t.Fatalf("config overwritten: %q", data)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(h.ConfigFile())
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != filePerm {
			t.Fatalf("config perm = %o, want %o", perm, filePerm)
		}
	}
}

func TestEnsureConfigRejectsNonRegularFile(t *testing.T) {
	h, err := Resolve("skills", fakeLookup(t.TempDir(), nil))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(h.ConfigFile(), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := h.EnsureConfig([]byte("x")); err == nil {
		t.Fatal("expected error when config.toml is a directory")
	}
}

func TestEnsureConfigDoesNotFollowSymlink(t *testing.T) {
	h, err := Resolve("skills", fakeLookup(t.TempDir(), nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Ensure(""); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target.toml")
	if err := os.Symlink(target, h.ConfigFile()); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := h.EnsureConfig([]byte("x")); err == nil {
		t.Fatal("expected error for symlinked config.toml")
	}
	if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("wrote through symlink: %v", err)
	}
}

func sameDir(a, b string) (bool, error) {
	ai, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	return os.SameFile(ai, bi), nil
}
