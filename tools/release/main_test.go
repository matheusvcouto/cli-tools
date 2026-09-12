package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/matheusvcouto/cli-tools/internal/testenv"
)

func TestParseGoVersion(t *testing.T) {
	cases := map[string][3]int{
		"go1.27.1":        {1, 27, 1},
		"go1.28":          {1, 28, 0},
		"go1.27.2-custom": {1, 27, 2},
	}
	for input, want := range cases {
		got, err := parseGoVersion(input)
		if err != nil {
			t.Fatalf("parseGoVersion(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("parseGoVersion(%q) = %v, want %v", input, got, want)
		}
	}
	if _, err := parseGoVersion("devel custom"); err == nil {
		t.Fatal("expected invalid version to fail")
	}
}

func TestRepositoryCommandsAreDiscovered(t *testing.T) {
	env, err := testenv.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range env {
		key, value, _ := strings.Cut(item, "=")
		t.Setenv(key, value)
	}
	t.Setenv("GOFLAGS", "")
	got, err := discoverCommands("../../cmd")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ai-profile", "repo-zip"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %v, want %v", got, want)
	}
}

func TestReleaseOSNameUsesMiseFriendlyMacOSLabel(t *testing.T) {
	if got := releaseOSName("darwin"); got != "macos" {
		t.Fatalf("releaseOSName(darwin) = %q, want macos", got)
	}
	if got := releaseOSName("linux"); got != "linux" {
		t.Fatalf("releaseOSName(linux) = %q, want linux", got)
	}
}

func TestArchivesAreReproducible(t *testing.T) {
	root := filepath.Join(t.TempDir(), "stage")
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bin", "example")
	if err := os.WriteFile(bin, []byte("synthetic-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		ext  string
		make func(string, string) error
	}{
		{name: "tar.gz", ext: ".tar.gz", make: tarGzDir},
		{name: "zip", ext: ".zip", make: zipDir},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := filepath.Join(t.TempDir(), "a"+tc.ext)
			b := filepath.Join(t.TempDir(), "b"+tc.ext)
			if err := tc.make(a, root); err != nil {
				t.Fatal(err)
			}
			if err := tc.make(b, root); err != nil {
				t.Fatal(err)
			}
			aRaw, err := os.ReadFile(a)
			if err != nil {
				t.Fatal(err)
			}
			bRaw, err := os.ReadFile(b)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(aRaw, bRaw) {
				t.Fatalf("%s output is not reproducible", tc.name)
			}
		})
	}
}
