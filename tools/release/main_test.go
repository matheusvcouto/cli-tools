package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
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

func TestReleaseModulePathMustMatchSemverMajor(t *testing.T) {
	const base = "github.com/matheusvcouto/cli-tools"
	for _, tc := range []struct {
		module string
		major  int
		valid  bool
	}{
		{base, 0, true}, {base, 1, true}, {base, 2, false},
		{base + "/v2", 1, false}, {base + "/v2", 2, true},
		{base + "/v2", 3, false}, {base + "/v3", 3, true},
		{base + "/v3", 2, false}, {"github.com/elsewhere/cli-tools/v2", 2, false},
	} {
		err := validateReleaseModulePath(tc.module, semVersion{Major: tc.major})
		if (err == nil) != tc.valid {
			t.Fatalf("module=%q major=%d valid=%t, got %v", tc.module, tc.major, tc.valid, err)
		}
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
	want := []string{"ai-profile", "media-get", "repo-zip"}
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
	if got := releaseOSName("windows"); got != "windows" {
		t.Fatalf("releaseOSName(windows) = %q, want windows", got)
	}
}

func TestDefaultTargetsIncludeWindowsAMD64AndARM64(t *testing.T) {
	want := map[target]bool{{GOOS: "windows", GOARCH: "amd64"}: false, {GOOS: "windows", GOARCH: "arm64"}: false}
	for _, item := range defaultTargets {
		if _, ok := want[item]; ok {
			want[item] = true
		}
	}
	for item, found := range want {
		if !found {
			t.Fatalf("missing release target %s/%s", item.GOOS, item.GOARCH)
		}
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

func TestReleaseWorkflowPublishesTheExactSmokeTestedBundle(t *testing.T) {
	raw, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(raw)
	if got := strings.Count(workflow, "go run ./tools/release build"); got != 1 {
		t.Fatalf("release artifacts must be built exactly once; build count=%d", got)
	}
	for _, required := range []string{
		"build-release:",
		"uses: actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a",
		"name: release-dist",
		"unix-release-smoke:\n    needs: build-release",
		"windows-release-smoke:\n    needs: build-release",
		"needs: [build-release, unix-release-smoke, windows-release-smoke]",
		"uses: actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c",
		"Get-FileHash",
		"Windows release command set mismatch",
		`test "$actual_names" = "$expected_names"`,
		"cli-tools_${version}_windows_arm64.zip",
		"Download the exact release archives selected for publication",
		"Download the exact verified release archives",
		"pending change records remain under changes/; run release prepare --write before tagging",
		"./scripts/check-workflows.sh",
		"./scripts/install-test-shells.sh",
		"git merge-base --is-ancestor",
		"./scripts/verify-remote-release-tag.sh",
		"./scripts/publish-release.sh",
		"Attest the release archives from the original builder",
		"subject-checksums: dist/SHA256SUMS",
		"overwrite: true",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("release workflow is missing exact-artifact invariant %q", required)
		}
	}

	// Structural contract, not evidence of a successful GitHub run. The
	// attestation is created by the builder, before artifact upload, and
	// publication is delayed until after both families of native smokes.
	buildStart := strings.Index(workflow, "\n  build-release:\n")
	smokeStart := strings.Index(workflow, "\n  unix-release-smoke:\n")
	if buildStart < 0 || smokeStart <= buildStart {
		t.Fatal("release build and native smoke job boundaries not found")
	}
	buildJob := workflow[buildStart:smokeStart]
	if strings.Count(workflow, "uses: actions/attest@") != 1 || !strings.Contains(buildJob, "uses: actions/attest@") ||
		strings.Index(buildJob, "uses: actions/attest@") > strings.Index(buildJob, "name: release-dist") {
		t.Fatal("the original build job must attest the release archives before upload")
	}
	if !strings.Contains(buildJob, "retention-days: 7") || strings.Count(buildJob, "overwrite: true") != 2 {
		t.Fatal("release artifacts must allow exact-run retries with sufficient retention")
	}
	for _, name := range []string{"scripts/publish-release.sh", "scripts/verify-remote-release-tag.sh"} {
		if _, err := os.Stat(filepath.Join("..", "..", name)); err != nil {
			t.Fatalf("release security script %q is missing: %v", name, err)
		}
	}

	windowsStart := strings.Index(workflow, "\n  windows-release-smoke:\n")
	publishStart := strings.Index(workflow, "\n  publish:\n")
	if windowsStart < 0 || publishStart < 0 || publishStart <= windowsStart {
		t.Fatal("release workflow job boundaries not found")
	}
	windowsJob := workflow[windowsStart:publishStart]
	if strings.Contains(windowsJob, "go run ./tools/release") || strings.Contains(windowsJob, "actions/setup-go") {
		t.Fatal("Windows smoke job must execute downloaded release assets, not rebuild them")
	}
	publishJob := workflow[publishStart:]
	if strings.Contains(publishJob, "id-token: write") || strings.Contains(publishJob, "uses: actions/attest@") {
		t.Fatal("publish must not claim build provenance from the downstream download runner")
	}
	if strings.Contains(publishJob, "go run ./tools/release") || strings.Contains(publishJob, "actions/setup-go") {
		t.Fatal("publish job must upload the already verified release bundle, not rebuild it")
	}
}

func TestCIWorkflowCoversMergeQueueAndPublishedArchitectures(t *testing.T) {
	raw, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(raw)
	for _, required := range []string{
		"merge_group:\n    types: [checks_requested]",
		"workflow-lint:",
		"change-records:",
		"./scripts/check-workflows.sh",
		"./scripts/install-test-shells.sh",
		"github.event.pull_request.head.sha",
		"os: ubuntu-24.04\n            arch: amd64",
		"os: ubuntu-24.04-arm\n            arch: arm64",
		"os: macos-15-intel\n            arch: amd64",
		"os: macos-15\n            arch: arm64",
		"os: windows-2025\n            arch: amd64",
		"os: windows-11-vs2026-arm\n            arch: arm64",
		"$listed = ($unformatted | ForEach-Object { \"unformatted: $_\" }) -join [Environment]::NewLine",
		"go test -race ./...",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("CI workflow is missing required coverage %q", required)
		}
	}
}

func TestActionlintConfigDeclaresHostedWindowsARMLabel(t *testing.T) {
	config, err := os.ReadFile("../../.github/actionlint.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "\n    - windows-11-vs2026-arm\n") {
		t.Fatal("actionlint config must declare the GitHub-hosted windows-11-vs2026-arm label")
	}
	script, err := os.ReadFile("../../scripts/check-workflows.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), "-config-file .github/actionlint.yaml") {
		t.Fatal("workflow lint must pass the actionlint config explicitly")
	}
}

func TestGitHubActionsArePinnedToImmutableCommits(t *testing.T) {
	workflowDir := filepath.Join("..", "..", ".github", "workflows")
	entries, err := os.ReadDir(workflowDir)
	if err != nil {
		t.Fatal(err)
	}
	usesLine := regexp.MustCompile(`(?m)^\s*-?\s*uses:\s+([^\s#]+)`) // local actions have no @ ref
	fullSHA := regexp.MustCompile(`^[^@]+@[0-9a-f]{40}$`)
	for _, entry := range entries {
		if entry.IsDir() || (filepath.Ext(entry.Name()) != ".yml" && filepath.Ext(entry.Name()) != ".yaml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(workflowDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		for _, match := range usesLine.FindAllStringSubmatch(text, -1) {
			ref := match[1]
			if strings.HasPrefix(ref, "./") {
				continue
			}
			if !fullSHA.MatchString(ref) {
				t.Fatalf("%s uses mutable or non-SHA action reference %q", entry.Name(), ref)
			}
		}

		lines := strings.Split(text, "\n")
		for i, line := range lines {
			if !strings.Contains(line, "uses: actions/checkout@") {
				continue
			}
			end := min(i+6, len(lines))
			block := strings.Join(lines[i:end], "\n")
			if !strings.Contains(block, "persist-credentials: false") {
				t.Fatalf("%s checkout at line %d must disable persisted credentials", entry.Name(), i+1)
			}
		}
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

func TestVerifyReleaseArchiveSetRequiresExactlyEveryPublishedTarget(t *testing.T) {
	dir := t.TempDir()
	version := "v1.2.3"
	for _, target := range defaultTargets {
		name := releaseArchiveName(version, target)
		if err := os.WriteFile(filepath.Join(dir, name), []byte("archive"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := verifyReleaseArchiveSet(dir, version); err != nil {
		t.Fatalf("complete archive set rejected: %v", err)
	}

	missing := releaseArchiveName(version, defaultTargets[0])
	if err := os.Remove(filepath.Join(dir, missing)); err != nil {
		t.Fatal(err)
	}
	if err := verifyReleaseArchiveSet(dir, version); err == nil || !strings.Contains(err.Error(), "missing release archive") {
		t.Fatalf("missing archive not rejected: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, missing), []byte("archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unexpected.txt"), []byte("unexpected"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyReleaseArchiveSet(dir, version); err == nil || !strings.Contains(err.Error(), "unexpected release output") {
		t.Fatalf("extra release output not rejected: %v", err)
	}
}

func TestValidateNotesOutRefusesReleaseDirectory(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "dist")
	for _, notes := range []string{
		out,
		filepath.Join(out, "release-notes.md"),
		filepath.Join(out, "cli-tools_1.2.3_windows_amd64.zip"),
	} {
		if err := validateNotesOut(out, notes); err == nil {
			t.Fatalf("validateNotesOut(%q, %q) unexpectedly succeeded", out, notes)
		}
	}
	if err := validateNotesOut(out, filepath.Join(root, "release-notes.md")); err != nil {
		t.Fatalf("outside notes path rejected: %v", err)
	}
	if err := validateNotesOut(out, ""); err != nil {
		t.Fatalf("empty notes path rejected: %v", err)
	}
}

func TestValidateNotesOutRefusesSymlinkedParentIntoReleaseDirectory(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "dist")
	if err := os.MkdirAll(filepath.Join(out, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "release-alias")
	if err := os.Symlink(filepath.Join(out, "nested"), alias); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if err := validateNotesOut(out, filepath.Join(alias, "release-notes.md")); err == nil {
		t.Fatal("notes path through symlinked parent into release directory unexpectedly accepted")
	}
}

func TestValidateNotesOutAllowsSymlinkedParentOutsideReleaseDirectory(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "dist")
	external := filepath.Join(root, "external")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(external, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "external-alias")
	if err := os.Symlink(external, alias); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if err := validateNotesOut(out, filepath.Join(alias, "release-notes.md")); err != nil {
		t.Fatalf("outside aliased notes path rejected: %v", err)
	}
}

func TestVerifyReleaseBundleDetectsPostChecksumMutationAndExtraFiles(t *testing.T) {
	const version = "v1.2.3"
	dir := t.TempDir()
	for _, target := range defaultTargets {
		name := releaseArchiveName(version, target)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeChecksums(dir); err != nil {
		t.Fatal(err)
	}
	if err := verifyReleaseBundle(dir, version); err != nil {
		t.Fatalf("valid release bundle rejected: %v", err)
	}

	victim := releaseArchiveName(version, defaultTargets[0])
	if err := os.WriteFile(filepath.Join(dir, victim), []byte("mutated"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyReleaseBundle(dir, version); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("post-checksum mutation was not rejected: %v", err)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, target := range defaultTargets {
		name := releaseArchiveName(version, target)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeChecksums(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "release-notes.md"), []byte("notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyReleaseBundle(dir, version); err == nil || !strings.Contains(err.Error(), "unexpected final release output") {
		t.Fatalf("extra final release file was not rejected: %v", err)
	}
}

func TestWriteReleaseNotesReplacesSymlinkInsteadOfFollowingIt(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "artifact.zip")
	if err := os.WriteFile(target, []byte("artifact"), 0o644); err != nil {
		t.Fatal(err)
	}
	notes := filepath.Join(dir, "release-notes.md")
	if err := os.Symlink(target, notes); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if err := writeReleaseNotes(notes, []byte("notes")); err != nil {
		t.Fatalf("writeReleaseNotes: %v", err)
	}
	gotTarget, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotTarget) != "artifact" {
		t.Fatalf("symlink target was modified: %q", gotTarget)
	}
	info, err := os.Lstat(notes)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("notes output is not a regular replacement: mode=%v", info.Mode())
	}
	gotNotes, err := os.ReadFile(notes)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotNotes) != "notes" {
		t.Fatalf("notes=%q want notes", gotNotes)
	}
}

func TestReleaseIncludesThirdPartyNotices(t *testing.T) {
	t.Chdir("../..")
	stage := t.TempDir()
	if err := copyThirdPartyNotices(stage); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(stage, "THIRD_PARTY_NOTICES.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"golang.org/x/term", "golang.org/x/sys", "Redistribution", "DISCLAIMED"} {
		if !strings.Contains(string(got), text) {
			t.Fatalf("missing %q", text)
		}
	}
}
