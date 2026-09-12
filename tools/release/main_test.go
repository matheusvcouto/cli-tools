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

func TestExtractReleaseNotesRequiresVersionDateAndList(t *testing.T) {
	changelog := `# Changelog

## [Unreleased]

- Future change.

## [0.2.0] - 2026-09-12

### Adicionado

- New command.

## [0.1.0] - 2026-09-01

### Corrigido

- Portable paths.
`
	got, err := extractReleaseNotes(changelog, "v0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	want := "## v0.2.0 — 2026-09-12\n\n### Adicionado\n\n- New command.\n"
	if got != want {
		t.Fatalf("release notes = %q, want %q", got, want)
	}

	for _, tc := range []struct {
		name      string
		changelog string
		version   string
	}{
		{name: "invalid version", changelog: changelog, version: "0.2.0"},
		{name: "missing section", changelog: changelog, version: "v0.3.0"},
		{name: "invalid date", changelog: "## [0.2.0] - 12/09/2026\n\n### Corrigido\n\n- Change.\n", version: "v0.2.0"},
		{name: "missing list", changelog: "## [0.2.0] - 2026-09-12\n\nNo list.\n", version: "v0.2.0"},
		{name: "list without category", changelog: "## [0.2.0] - 2026-09-12\n\n- Change.\n", version: "v0.2.0"},
		{name: "empty category", changelog: "## [0.2.0] - 2026-09-12\n\n### Corrigido\n", version: "v0.2.0"},
		{name: "unsupported category", changelog: "## [0.2.0] - 2026-09-12\n\n### Misc\n\n- Change.\n", version: "v0.2.0"},
		{name: "leading zero", changelog: changelog, version: "v00.2.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := extractReleaseNotes(tc.changelog, tc.version); err == nil {
				t.Fatal("expected invalid release notes to fail")
			}
		})
	}
}

func TestPrepareOutputDirNeverRemovesExistingContent(t *testing.T) {
	base := t.TempDir()
	missing := filepath.Join(base, "missing")
	if err := prepareOutputDir(missing); err != nil {
		t.Fatalf("prepare absent output: %v", err)
	}
	if info, err := os.Lstat(missing); err != nil || !info.IsDir() {
		t.Fatalf("output directory was not created: info=%v err=%v", info, err)
	}

	existing := filepath.Join(base, "existing")
	if err := os.Mkdir(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(existing, "keep.txt")
	if err := os.WriteFile(keep, []byte("must remain"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := prepareOutputDir(existing); err == nil {
		t.Fatal("expected non-empty output directory to be refused")
	}
	if got, err := os.ReadFile(keep); err != nil || string(got) != "must remain" {
		t.Fatalf("existing content changed: %q, %v", got, err)
	}

	link := filepath.Join(base, "link")
	if err := os.Symlink(existing, link); err == nil {
		if err := prepareOutputDir(link); err == nil {
			t.Fatal("expected symlinked output directory to be refused")
		}
	}

	outside := t.TempDir()
	parentLink := filepath.Join(base, "parent-link")
	if err := os.Symlink(outside, parentLink); err == nil {
		outsideChild := filepath.Join(outside, "must-not-exist")
		if err := prepareOutputDir(filepath.Join(parentLink, "must-not-exist")); err == nil {
			t.Fatal("expected symlinked output parent to be refused")
		}
		if _, err := os.Lstat(outsideChild); !os.IsNotExist(err) {
			t.Fatalf("output was created through a parent symlink: %v", err)
		}
	}
}

func TestWorkflowChecksChecksumsFromManifestDirectory(t *testing.T) {
	raw, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(raw)
	if !strings.Contains(workflow, "(cd dist && sha256sum -c SHA256SUMS)") {
		t.Fatal("release workflow must verify relative checksum entries from the manifest directory")
	}
	if strings.Contains(workflow, "sha256sum -c dist/SHA256SUMS") {
		t.Fatal("release workflow resolves checksum entries from the repository root")
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
