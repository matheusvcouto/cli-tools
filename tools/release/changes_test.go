package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPlanReleaseUsesIndependentToolVersionsAndHighestSuiteImpact(t *testing.T) {
	root := t.TempDir()
	cmdRoot := filepath.Join(root, "cmd")
	for _, tc := range []struct{ name, version string }{{"ai-profile", "0.1.1"}, {"repo-zip", "0.1.1"}} {
		dir := filepath.Join(cmdRoot, tc.name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		raw := `{"schema_version":1,"name":"` + tc.name + `","version":"` + tc.version + `","stability":"beta"}`
		if err := os.WriteFile(filepath.Join(dir, "tool.json"), []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	changes := filepath.Join(root, "changes")
	if err := os.MkdirAll(changes, 0o755); err != nil {
		t.Fatal(err)
	}
	record := `{"schema_version":1,"changes":[` +
		`{"component":"module","impact":"patch","breaking":false,"summary":"core fix"},` +
		`{"component":"ai-profile","impact":"minor","breaking":false,"summary":"feature"},` +
		`{"component":"repo-zip","impact":"patch","breaking":false,"summary":"fix"}]}`
	if err := os.WriteFile(filepath.Join(changes, "x.json"), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	changelog := filepath.Join(root, "CHANGELOG.md")
	if err := os.WriteFile(changelog, []byte("# Changelog\n\n## [Unreleased]\n\n## [0.1.1] - 2026-09-12\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, _, _, err := planRelease(cmdRoot, changes, changelog, "v0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if plan.NextSuite.String() != "0.2.0" || plan.SuiteImpact != impactMinor {
		t.Fatalf("suite plan=%+v", plan)
	}
	if got := plan.Tools["ai-profile"].Next.String(); got != "0.2.0" {
		t.Fatalf("ai-profile next=%s", got)
	}
	if got := plan.Tools["repo-zip"].Next.String(); got != "0.1.2" {
		t.Fatalf("repo-zip next=%s", got)
	}
	if _, _, _, err := planRelease(cmdRoot, changes, changelog, "v0.1.2"); err == nil {
		t.Fatal("expected incorrect suite bump to fail")
	}
}

func TestStableBreakingToolRequiresMajorImpact(t *testing.T) {
	root := t.TempDir()
	cmdRoot := filepath.Join(root, "cmd")
	dir := filepath.Join(cmdRoot, "tool")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tool.json"), []byte(`{"schema_version":1,"name":"tool","version":"1.2.3","stability":"stable"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	changes := filepath.Join(root, "changes")
	if err := os.MkdirAll(changes, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changes, "x.json"), []byte(`{"schema_version":1,"changes":[{"component":"tool","impact":"minor","breaking":true,"summary":"break syntax"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	changelog := filepath.Join(root, "CHANGELOG.md")
	if err := os.WriteFile(changelog, []byte("# Changelog\n\n## [Unreleased]\n\n## [1.2.3] - 2026-09-12\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := planRelease(cmdRoot, changes, changelog, "v1.3.0"); err == nil || !strings.Contains(err.Error(), "requires major") {
		t.Fatalf("err=%v", err)
	}
}

func TestChangeCoverageMapsCoreAndToolScopes(t *testing.T) {
	tools := map[string]toolManifestAtPath{
		"ai-profile": {},
		"repo-zip":   {},
	}
	paths := []string{"cli/compile.go", "internal/repozip/service.go", "docs/testing.md"}
	required := requiredChangeComponents(paths, tools)
	for _, name := range []string{"module", "ai-profile", "repo-zip"} {
		if _, ok := required[name]; !ok {
			t.Fatalf("missing %s in %v", name, required)
		}
	}
	files := []loadedChangeFile{{Record: changeRecordFile{Changes: []changeRecord{
		{Component: "module", Impact: impactMinor, Summary: "core"},
		{Component: "ai-profile", Impact: impactMinor, Summary: "core consumer"},
		{Component: "repo-zip", Impact: impactMinor, Summary: "core consumer"},
	}}}}
	if err := validateChangeCoverage(paths, files, tools); err != nil {
		t.Fatal(err)
	}
	if err := validateChangeCoverage(paths, files[:0], tools); err == nil {
		t.Fatal("expected missing coverage to fail")
	}
}

func TestImpactNoneRequiresJustification(t *testing.T) {
	_, err := decodeChangeFile([]byte(`{"schema_version":1,"changes":[{"component":"module","impact":"none","breaking":false,"summary":"no contract change"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateChange(changeRecord{Component: "module", Impact: impactNone, Summary: "no contract change"}); err == nil {
		t.Fatal("expected none without justification to fail")
	}
}

func TestWritePreparedReleasePreflightsArchiveBeforeMutation(t *testing.T) {
	root := t.TempDir()
	cmdRoot := filepath.Join(root, "cmd")
	toolDir := filepath.Join(cmdRoot, "tool")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(toolDir, "tool.json")
	manifestRaw := []byte(`{"schema_version":1,"name":"tool","version":"0.1.0","stability":"beta"}`)
	if err := os.WriteFile(manifestPath, manifestRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	changesDir := filepath.Join(root, "changes")
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	changePath := filepath.Join(changesDir, "x.json")
	changeRaw := []byte(`{"schema_version":1,"changes":[{"component":"module","impact":"patch","breaking":false,"summary":"core fix"},{"component":"tool","impact":"patch","breaking":false,"summary":"tool fix"}]}`)
	if err := os.WriteFile(changePath, changeRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	changelogPath := filepath.Join(root, "CHANGELOG.md")
	changelogRaw := []byte("# Changelog\n\n## [Unreleased]\n\n## [0.1.0] - 2026-09-12\n")
	if err := os.WriteFile(changelogPath, changelogRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	plan, files, manifests, err := planRelease(cmdRoot, changesDir, changelogPath, "v0.1.1")
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(changesDir, "archive", "0.1.1")
	if err := os.MkdirAll(archive, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archive, "x.json"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writePreparedRelease(plan, files, manifests, cmdRoot, changelogPath, changesDir, time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)); err == nil || !strings.Contains(err.Error(), "archive already exists") {
		t.Fatalf("err=%v", err)
	}
	if got, err := os.ReadFile(manifestPath); err != nil || string(got) != string(manifestRaw) {
		t.Fatalf("manifest mutated before preflight failure: %q err=%v", got, err)
	}
	if got, err := os.ReadFile(changelogPath); err != nil || string(got) != string(changelogRaw) {
		t.Fatalf("changelog mutated before preflight failure: %q err=%v", got, err)
	}
	if got, err := os.ReadFile(changePath); err != nil || string(got) != string(changeRaw) {
		t.Fatalf("change record moved before preflight failure: %q err=%v", got, err)
	}
}

func TestFirstV1ToolReleaseRequiresExplicitStablePromotion(t *testing.T) {
	root := t.TempDir()
	cmdRoot := filepath.Join(root, "cmd")
	toolDir := filepath.Join(cmdRoot, "tool")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(toolDir, "tool.json")
	if err := os.WriteFile(manifestPath, []byte(`{"schema_version":1,"name":"tool","version":"0.9.0","stability":"beta"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	changesDir := filepath.Join(root, "changes")
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	changePath := filepath.Join(changesDir, "x.json")
	withoutPromotion := `{"schema_version":1,"changes":[{"component":"tool","impact":"major","breaking":true,"summary":"v1 API freeze"}]}`
	if err := os.WriteFile(changePath, []byte(withoutPromotion), 0o644); err != nil {
		t.Fatal(err)
	}
	changelogPath := filepath.Join(root, "CHANGELOG.md")
	if err := os.WriteFile(changelogPath, []byte("# Changelog\n\n## [Unreleased]\n\n## [0.9.0] - 2026-09-12\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := planRelease(cmdRoot, changesDir, changelogPath, "v1.0.0"); err == nil || !strings.Contains(err.Error(), "explicit stability promotion to stable") {
		t.Fatalf("expected explicit stable promotion requirement, err=%v", err)
	}

	withPromotion := `{"schema_version":1,"changes":[{"component":"tool","impact":"major","breaking":true,"summary":"v1 API freeze","stability":"stable"}]}`
	if err := os.WriteFile(changePath, []byte(withPromotion), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, _, _, err := planRelease(cmdRoot, changesDir, changelogPath, "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	tool := plan.Tools["tool"]
	if tool.Next.String() != "1.0.0" || tool.CurrentStability != "beta" || tool.NextStability != "stable" {
		t.Fatalf("unexpected v1 promotion plan: %+v", tool)
	}
	if got := renderPreparedRelease(plan); !strings.Contains(got, "stability beta -> stable") {
		t.Fatalf("preview does not expose promotion: %q", got)
	}
}

func TestToolStabilityCannotMoveBackwards(t *testing.T) {
	_, err := nextToolStability("stable", semVersion{Major: 1}, semVersion{Major: 1, Patch: 1}, []changeRecord{{Stability: "beta"}})
	if err == nil || !strings.Contains(err.Error(), "cannot move backwards") {
		t.Fatalf("expected downgrade rejection, err=%v", err)
	}
}

func TestModuleChangeCannotSetToolStability(t *testing.T) {
	root := t.TempDir()
	cmdRoot := filepath.Join(root, "cmd")
	toolDir := filepath.Join(cmdRoot, "tool")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(toolDir, "tool.json"), []byte(`{"schema_version":1,"name":"tool","version":"0.1.0","stability":"beta"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	changesDir := filepath.Join(root, "changes")
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changesDir, "x.json"), []byte(`{"schema_version":1,"changes":[{"component":"module","impact":"patch","breaking":false,"summary":"metadata","stability":"stable"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	changelogPath := filepath.Join(root, "CHANGELOG.md")
	if err := os.WriteFile(changelogPath, []byte("# Changelog\n\n## [Unreleased]\n\n## [0.1.0] - 2026-09-12\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := planRelease(cmdRoot, changesDir, changelogPath, "v0.1.1"); err == nil || !strings.Contains(err.Error(), "cannot set tool stability") {
		t.Fatalf("expected module stability rejection, err=%v", err)
	}
}

func TestWritePreparedReleaseRollsBackAfterPostMutationFailure(t *testing.T) {
	root := t.TempDir()
	cmdRoot := filepath.Join(root, "cmd")
	toolDir := filepath.Join(cmdRoot, "tool")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(toolDir, "tool.json")
	manifestRaw := []byte(`{"schema_version":1,"name":"tool","version":"0.1.0","stability":"beta"}`)
	if err := os.WriteFile(manifestPath, manifestRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	contractPath := filepath.Join(toolDir, "cli.contract.json")
	contractRaw := []byte("old-contract\n")
	if err := os.WriteFile(contractPath, contractRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	changesDir := filepath.Join(root, "changes")
	if err := os.MkdirAll(filepath.Join(changesDir, "archive"), 0o755); err != nil {
		t.Fatal(err)
	}
	changePath := filepath.Join(changesDir, "x.json")
	changeRaw := []byte(`{"schema_version":1,"changes":[{"component":"module","impact":"patch","breaking":false,"summary":"core fix"},{"component":"tool","impact":"patch","breaking":false,"summary":"tool fix"}]}`)
	if err := os.WriteFile(changePath, changeRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	changelogPath := filepath.Join(root, "CHANGELOG.md")
	changelogRaw := []byte("# Changelog\n\n## [Unreleased]\n\n## [0.1.0] - 2026-09-12\n")
	if err := os.WriteFile(changelogPath, changelogRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	plan, files, manifests, err := planRelease(cmdRoot, changesDir, changelogPath, "v0.1.1")
	if err != nil {
		t.Fatal(err)
	}
	boom := func(string) error { return fmt.Errorf("synthetic contract failure") }
	if err := writePreparedReleaseWith(plan, files, manifests, cmdRoot, changelogPath, changesDir, time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC), boom); err == nil || !strings.Contains(err.Error(), "synthetic contract failure") {
		t.Fatalf("expected synthetic failure, err=%v", err)
	}
	for path, want := range map[string][]byte{
		manifestPath:  manifestRaw,
		contractPath:  contractRaw,
		changelogPath: changelogRaw,
		changePath:    changeRaw,
	} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read restored %s: %v", path, err)
		}
		if string(got) != string(want) {
			t.Fatalf("%s not restored:\n got %q\nwant %q", path, got, want)
		}
	}
	if _, err := os.Lstat(filepath.Join(toolDir, "CHANGELOG.md")); !os.IsNotExist(err) {
		t.Fatalf("tool changelog created despite rollback: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(changesDir, "archive", "0.1.1", "x.json")); !os.IsNotExist(err) {
		t.Fatalf("change record archive survived rollback: %v", err)
	}
	if info, err := os.Stat(filepath.Join(changesDir, "archive")); err != nil || !info.IsDir() {
		t.Fatalf("pre-existing archive directory was not preserved: info=%v err=%v", info, err)
	}
}

func TestWritePreparedReleaseCommitsVersionStabilityAndArchive(t *testing.T) {
	root := t.TempDir()
	cmdRoot := filepath.Join(root, "cmd")
	toolDir := filepath.Join(cmdRoot, "tool")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(toolDir, "tool.json")
	if err := os.WriteFile(manifestPath, []byte(`{"schema_version":1,"name":"tool","version":"0.9.0","stability":"beta"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	contractPath := filepath.Join(toolDir, "cli.contract.json")
	if err := os.WriteFile(contractPath, []byte("old-contract\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changesDir := filepath.Join(root, "changes")
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	changePath := filepath.Join(changesDir, "x.json")
	changeRaw := []byte(`{"schema_version":1,"changes":[{"component":"module","impact":"major","breaking":true,"summary":"module v1"},{"component":"tool","impact":"major","breaking":true,"summary":"tool v1","stability":"stable"}]}`)
	if err := os.WriteFile(changePath, changeRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	changelogPath := filepath.Join(root, "CHANGELOG.md")
	if err := os.WriteFile(changelogPath, []byte("# Changelog\n\n## [Unreleased]\n\n## [0.9.0] - 2026-09-12\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, files, manifests, err := planRelease(cmdRoot, changesDir, changelogPath, "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	regenerate := func(string) error {
		return writeFileAtomic(contractPath, []byte("new-contract\n"), 0o644)
	}
	if err := writePreparedReleaseWith(plan, files, manifests, cmdRoot, changelogPath, changesDir, time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC), regenerate); err != nil {
		t.Fatal(err)
	}
	manifestRaw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifestRaw), `"version": "1.0.0"`) || !strings.Contains(string(manifestRaw), `"stability": "stable"`) {
		t.Fatalf("manifest was not promoted atomically: %s", manifestRaw)
	}
	if got, err := os.ReadFile(contractPath); err != nil || string(got) != "new-contract\n" {
		t.Fatalf("contract not committed: %q err=%v", got, err)
	}
	archivePath := filepath.Join(changesDir, "archive", "1.0.0", "x.json")
	if got, err := os.ReadFile(archivePath); err != nil || string(got) != string(changeRaw) {
		t.Fatalf("change record not archived: %q err=%v", got, err)
	}
	if _, err := os.Lstat(changePath); !os.IsNotExist(err) {
		t.Fatalf("pending change record still exists after commit: %v", err)
	}
}
