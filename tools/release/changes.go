package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	corecli "github.com/matheusvcouto/cli-tools/cli"
	productversion "github.com/matheusvcouto/cli-tools/internal/version"
)

const changeRecordSchemaVersion = 1

type impact string

const (
	impactNone  impact = "none"
	impactPatch impact = "patch"
	impactMinor impact = "minor"
	impactMajor impact = "major"
)

type changeRecordFile struct {
	SchemaVersion int            `json:"schema_version"`
	Changes       []changeRecord `json:"changes"`
}

type changeRecord struct {
	Component     string `json:"component"`
	Impact        impact `json:"impact"`
	Breaking      bool   `json:"breaking"`
	Summary       string `json:"summary"`
	Justification string `json:"justification,omitempty"`
	Stability     string `json:"stability,omitempty"`
}

type loadedChangeFile struct {
	Path   string
	Record changeRecordFile
}

type semVersion struct{ Major, Minor, Patch int }

func (v semVersion) String() string { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) }

func parseReleaseSemver(s string) (semVersion, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return semVersion{}, fmt.Errorf("version %q must be X.Y.Z", s)
	}
	var out semVersion
	vals := []*int{&out.Major, &out.Minor, &out.Patch}
	for i, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return semVersion{}, fmt.Errorf("version %q is not canonical SemVer", s)
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return semVersion{}, fmt.Errorf("version %q is not canonical SemVer", s)
		}
		*vals[i] = n
	}
	return out, nil
}

func bumpVersion(v semVersion, x impact) semVersion {
	switch x {
	case impactPatch:
		v.Patch++
	case impactMinor:
		v.Minor++
		v.Patch = 0
	case impactMajor:
		v.Major++
		v.Minor, v.Patch = 0, 0
	}
	return v
}

func impactRank(x impact) int {
	switch x {
	case impactNone:
		return 0
	case impactPatch:
		return 1
	case impactMinor:
		return 2
	case impactMajor:
		return 3
	default:
		return -1
	}
}

func maxImpact(a, b impact) impact {
	if impactRank(b) > impactRank(a) {
		return b
	}
	return a
}

func loadChangeFiles(dir string, validComponents map[string]struct{}) ([]loadedChangeFile, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("changes directory %q does not exist", dir)
	}
	if err != nil {
		return nil, err
	}
	var out []loadedChangeFile
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		record, err := decodeChangeFile(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		for i, change := range record.Changes {
			if _, ok := validComponents[change.Component]; !ok {
				return nil, fmt.Errorf("%s change %d: unknown component %q", path, i+1, change.Component)
			}
			if err := validateChange(change); err != nil {
				return nil, fmt.Errorf("%s change %d: %w", path, i+1, err)
			}
		}
		out = append(out, loadedChangeFile{Path: path, Record: record})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func decodeChangeFile(raw []byte) (changeRecordFile, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var record changeRecordFile
	if err := dec.Decode(&record); err != nil {
		return changeRecordFile{}, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return changeRecordFile{}, fmt.Errorf("change record must contain exactly one JSON object")
		}
		return changeRecordFile{}, err
	}
	if record.SchemaVersion != changeRecordSchemaVersion {
		return changeRecordFile{}, fmt.Errorf("unsupported schema_version %d", record.SchemaVersion)
	}
	if len(record.Changes) == 0 {
		return changeRecordFile{}, fmt.Errorf("changes must not be empty")
	}
	return record, nil
}

func validateChange(c changeRecord) error {
	if strings.TrimSpace(c.Component) == "" {
		return fmt.Errorf("component is required")
	}
	if impactRank(c.Impact) < 0 {
		return fmt.Errorf("invalid impact %q", c.Impact)
	}
	if strings.TrimSpace(c.Summary) == "" {
		return fmt.Errorf("summary is required")
	}
	if c.Impact == impactNone && strings.TrimSpace(c.Justification) == "" {
		return fmt.Errorf("impact none requires justification")
	}
	if c.Breaking && c.Impact == impactNone {
		return fmt.Errorf("breaking change cannot use impact none")
	}
	if c.Stability != "" && stabilityRank(c.Stability) < 0 {
		return fmt.Errorf("invalid stability %q", c.Stability)
	}
	return nil
}

func aggregateChanges(files []loadedChangeFile) (map[string]impact, map[string][]changeRecord) {
	impacts := map[string]impact{}
	byComponent := map[string][]changeRecord{}
	for _, file := range files {
		for _, change := range file.Record.Changes {
			impacts[change.Component] = maxImpact(impacts[change.Component], change.Impact)
			byComponent[change.Component] = append(byComponent[change.Component], change)
		}
	}
	return impacts, byComponent
}

type toolManifestAtPath struct {
	Path     string
	Manifest productversion.ToolManifest
}

func discoverToolManifests(cmdRoot string) (map[string]toolManifestAtPath, error) {
	entries, err := os.ReadDir(cmdRoot)
	if err != nil {
		return nil, err
	}
	out := map[string]toolManifestAtPath{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(cmdRoot, entry.Name(), "tool.json")
		raw, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		manifest, err := productversion.ParseToolManifest(raw, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		out[manifest.Name] = toolManifestAtPath{Path: path, Manifest: manifest}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no tool manifests found under %s", cmdRoot)
	}
	return out, nil
}

func validReleaseComponents(manifests map[string]toolManifestAtPath) map[string]struct{} {
	out := map[string]struct{}{"module": {}}
	for name := range manifests {
		out[name] = struct{}{}
	}
	return out
}

func latestChangelogVersion(raw string) (semVersion, error) {
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "## [") || strings.HasPrefix(line, "## [Unreleased]") {
			continue
		}
		end := strings.Index(line, "]")
		if end < len("## [") {
			continue
		}
		return parseReleaseSemver(line[len("## ["):end])
	}
	return semVersion{}, fmt.Errorf("no released SemVer section found in changelog")
}

type preparedRelease struct {
	CurrentSuite semVersion
	NextSuite    semVersion
	SuiteImpact  impact
	Tools        map[string]preparedTool
	Changes      map[string][]changeRecord
}

type preparedTool struct {
	Current          semVersion
	Next             semVersion
	Impact           impact
	CurrentStability string
	NextStability    string
}

func stabilityRank(value string) int {
	switch value {
	case "experimental":
		return 0
	case "alpha":
		return 1
	case "beta":
		return 2
	case "stable":
		return 3
	default:
		return -1
	}
}

func requestedStability(changes []changeRecord) (string, error) {
	var requested string
	for _, change := range changes {
		if change.Stability == "" {
			continue
		}
		if requested != "" && requested != change.Stability {
			return "", fmt.Errorf("conflicting stability promotions %q and %q", requested, change.Stability)
		}
		requested = change.Stability
	}
	return requested, nil
}

func nextToolStability(current string, currentVersion, nextVersion semVersion, changes []changeRecord) (string, error) {
	requested, err := requestedStability(changes)
	if err != nil {
		return "", err
	}
	next := current
	if requested != "" {
		if stabilityRank(requested) < stabilityRank(current) {
			return "", fmt.Errorf("stability cannot move backwards from %s to %s", current, requested)
		}
		next = requested
	}
	if nextVersion.Major >= 1 && next != "stable" {
		if currentVersion.Major < 1 {
			return "", fmt.Errorf("first 1.x release requires an explicit stability promotion to stable")
		}
		return "", fmt.Errorf("1.x product versions require stable stability")
	}
	return next, nil
}

func planRelease(cmdRoot, changesDir, changelogPath, requestedSuite string) (preparedRelease, []loadedChangeFile, map[string]toolManifestAtPath, error) {
	manifests, err := discoverToolManifests(cmdRoot)
	if err != nil {
		return preparedRelease{}, nil, nil, err
	}
	files, err := loadChangeFiles(changesDir, validReleaseComponents(manifests))
	if err != nil {
		return preparedRelease{}, nil, nil, err
	}
	if len(files) == 0 {
		return preparedRelease{}, nil, nil, fmt.Errorf("no pending change records in %s", changesDir)
	}
	impacts, byComponent := aggregateChanges(files)
	for _, change := range byComponent["module"] {
		if change.Stability != "" {
			return preparedRelease{}, nil, nil, fmt.Errorf("module change records cannot set tool stability")
		}
	}
	changelog, err := os.ReadFile(changelogPath)
	if err != nil {
		return preparedRelease{}, nil, nil, err
	}
	currentSuite, err := latestChangelogVersion(string(changelog))
	if err != nil {
		return preparedRelease{}, nil, nil, err
	}
	suiteImpact := impactNone
	for _, x := range impacts {
		suiteImpact = maxImpact(suiteImpact, x)
	}
	if suiteImpact == impactNone {
		return preparedRelease{}, nil, nil, fmt.Errorf("pending changes do not request a suite version bump")
	}
	nextSuite := bumpVersion(currentSuite, suiteImpact)
	requested, err := parseReleaseSemver(requestedSuite)
	if err != nil {
		return preparedRelease{}, nil, nil, err
	}
	if requested != nextSuite {
		return preparedRelease{}, nil, nil, fmt.Errorf("suite version %s does not match computed %s bump from %s to %s", requested, suiteImpact, currentSuite, nextSuite)
	}

	plan := preparedRelease{CurrentSuite: currentSuite, NextSuite: nextSuite, SuiteImpact: suiteImpact, Tools: map[string]preparedTool{}, Changes: byComponent}
	for name, item := range manifests {
		current, err := parseReleaseSemver(item.Manifest.Version)
		if err != nil {
			return preparedRelease{}, nil, nil, fmt.Errorf("%s: %w", item.Path, err)
		}
		x := impacts[name]
		for _, change := range byComponent[name] {
			if current.Major >= 1 && change.Breaking && change.Impact != impactMajor {
				return preparedRelease{}, nil, nil, fmt.Errorf("%s breaking change requires major impact at %s", name, current)
			}
		}
		next := bumpVersion(current, x)
		nextStability, err := nextToolStability(item.Manifest.Stability, current, next, byComponent[name])
		if err != nil {
			return preparedRelease{}, nil, nil, fmt.Errorf("%s: %w", name, err)
		}
		plan.Tools[name] = preparedTool{Current: current, Next: next, Impact: x, CurrentStability: item.Manifest.Stability, NextStability: nextStability}
	}
	return plan, files, manifests, nil
}

func renderPreparedRelease(plan preparedRelease) string {
	var b strings.Builder
	fmt.Fprintf(&b, "suite %s -> %s (%s)\n", plan.CurrentSuite, plan.NextSuite, plan.SuiteImpact)
	names := make([]string, 0, len(plan.Tools))
	for name := range plan.Tools {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t := plan.Tools[name]
		if t.CurrentStability != t.NextStability {
			fmt.Fprintf(&b, "%s %s -> %s (%s; stability %s -> %s)\n", name, t.Current, t.Next, t.Impact, t.CurrentStability, t.NextStability)
			continue
		}
		fmt.Fprintf(&b, "%s %s -> %s (%s; stability %s)\n", name, t.Current, t.Next, t.Impact, t.CurrentStability)
	}
	return b.String()
}

func writePreparedRelease(plan preparedRelease, files []loadedChangeFile, manifests map[string]toolManifestAtPath, cmdRoot, changelogPath, changesDir string, date time.Time) error {
	return writePreparedReleaseWith(plan, files, manifests, cmdRoot, changelogPath, changesDir, date, regenerateContracts)
}

type releaseFileSnapshot struct {
	Path   string
	Exists bool
	Mode   os.FileMode
	Data   []byte
}

func snapshotReleaseFiles(paths []string) ([]releaseFileSnapshot, error) {
	seen := map[string]struct{}{}
	out := make([]releaseFileSnapshot, 0, len(paths))
	for _, path := range paths {
		path = filepath.Clean(path)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			out = append(out, releaseFileSnapshot{Path: path})
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("release transaction target is not a regular file: %s", path)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		out = append(out, releaseFileSnapshot{Path: path, Exists: true, Mode: info.Mode().Perm(), Data: raw})
	}
	return out, nil
}

func restoreReleaseFiles(snapshots []releaseFileSnapshot) error {
	var errs []error
	// Remove files that did not exist before first. This clears partially
	// written contract/archive destinations before restoring original sources.
	for _, snapshot := range snapshots {
		if snapshot.Exists {
			continue
		}
		if err := os.Remove(snapshot.Path); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("remove %s: %w", snapshot.Path, err))
		}
	}
	for _, snapshot := range snapshots {
		if !snapshot.Exists {
			continue
		}
		if err := writeFileAtomic(snapshot.Path, snapshot.Data, snapshot.Mode); err != nil {
			errs = append(errs, fmt.Errorf("restore %s: %w", snapshot.Path, err))
		}
	}
	return errors.Join(errs...)
}

func releaseMutationPaths(plan preparedRelease, files []loadedChangeFile, manifests map[string]toolManifestAtPath, changelogPath, changesDir string) []string {
	paths := []string{changelogPath}
	for name, tool := range plan.Tools {
		if tool.Impact == impactNone {
			continue
		}
		item := manifests[name]
		paths = append(paths,
			item.Path,
			filepath.Join(filepath.Dir(item.Path), "CHANGELOG.md"),
		)
	}
	for name, item := range manifests {
		_ = name
		paths = append(paths, filepath.Join(filepath.Dir(item.Path), "cli.contract.json"))
	}
	archive := filepath.Join(changesDir, "archive", plan.NextSuite.String())
	for _, file := range files {
		paths = append(paths, file.Path, filepath.Join(archive, filepath.Base(file.Path)))
	}
	return paths
}

func removeDirIfEmpty(path string) error {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return nil
	}
	return os.Remove(path)
}

func directoryExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("expected directory at %s", path)
	}
	return true, nil
}

func writePreparedReleaseWith(plan preparedRelease, files []loadedChangeFile, manifests map[string]toolManifestAtPath, cmdRoot, changelogPath, changesDir string, date time.Time, regenerate func(string) error) (resultErr error) {
	// Validate every archive destination before mutating manifests/changelogs.
	// A stale or already-prepared archive must fail without leaving the working
	// tree in a half-prepared release state.
	archiveParent := filepath.Join(changesDir, "archive")
	archive := filepath.Join(archiveParent, plan.NextSuite.String())
	archiveParentExisted, err := directoryExists(archiveParent)
	if err != nil {
		return err
	}
	archiveExisted, err := directoryExists(archive)
	if err != nil {
		return err
	}
	for _, file := range files {
		dst := filepath.Join(archive, filepath.Base(file.Path))
		if _, err := os.Lstat(dst); err == nil {
			return fmt.Errorf("change archive already exists: %s", dst)
		} else if !os.IsNotExist(err) {
			return err
		}
		if _, err := os.Lstat(file.Path); err != nil {
			return fmt.Errorf("change record %s is unavailable: %w", file.Path, err)
		}
	}
	snapshots, err := snapshotReleaseFiles(releaseMutationPaths(plan, files, manifests, changelogPath, changesDir))
	if err != nil {
		return err
	}
	defer func() {
		if resultErr == nil {
			return
		}
		rollbackErr := restoreReleaseFiles(snapshots)
		if !archiveExisted {
			if err := removeDirIfEmpty(archive); err != nil && !os.IsNotExist(err) {
				rollbackErr = errors.Join(rollbackErr, fmt.Errorf("remove empty archive dir: %w", err))
			}
		}
		if !archiveParentExisted {
			if err := removeDirIfEmpty(archiveParent); err != nil && !os.IsNotExist(err) {
				rollbackErr = errors.Join(rollbackErr, fmt.Errorf("remove empty archive parent: %w", err))
			}
		}
		if rollbackErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("release rollback failed: %w", rollbackErr))
		}
	}()

	for name, tool := range plan.Tools {
		if tool.Impact == impactNone {
			continue
		}
		item := manifests[name]
		manifest := item.Manifest
		manifest.Version = tool.Next.String()
		manifest.Stability = tool.NextStability
		raw, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return err
		}
		if err := writeFileAtomic(item.Path, append(raw, '\n'), 0o644); err != nil {
			return err
		}
		if err := updateToolChangelog(filepath.Join(filepath.Dir(item.Path), "CHANGELOG.md"), name, tool.Next, date, plan.Changes[name]); err != nil {
			return err
		}
	}
	if err := updateSuiteChangelog(changelogPath, plan.NextSuite, date, flattenChanges(files)); err != nil {
		return err
	}
	if err := regenerate(cmdRoot); err != nil {
		return err
	}
	if err := os.MkdirAll(archive, 0o755); err != nil {
		return err
	}
	for _, file := range files {
		dst := filepath.Join(archive, filepath.Base(file.Path))
		if err := os.Rename(file.Path, dst); err != nil {
			return err
		}
	}
	return nil
}

func flattenChanges(files []loadedChangeFile) []changeRecord {
	var out []changeRecord
	for _, file := range files {
		out = append(out, file.Record.Changes...)
	}
	return out
}

func updateToolChangelog(path, name string, version semVersion, date time.Time, changes []changeRecord) error {
	body := renderChangeBody(changes)
	if body == "" {
		return nil
	}
	header := "# " + name + " changelog\n\n"
	old, err := os.ReadFile(path)
	if err == nil {
		header = string(old)
		if !strings.HasSuffix(header, "\n") {
			header += "\n"
		}
		header += "\n"
	} else if !os.IsNotExist(err) {
		return err
	}
	section := fmt.Sprintf("## [%s] - %s\n\n%s\n", version, date.Format("2006-01-02"), body)
	return writeFileAtomic(path, []byte(header+section), 0o644)
}

func updateSuiteChangelog(path string, version semVersion, date time.Time, changes []changeRecord) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	marker := "## [Unreleased]"
	start := strings.Index(text, marker)
	if start < 0 {
		return fmt.Errorf("changelog has no %s section", marker)
	}
	bodyStart := start + len(marker)
	next := strings.Index(text[bodyStart:], "\n## [")
	if next < 0 {
		return fmt.Errorf("changelog has no released section after Unreleased")
	}
	next += bodyStart
	generated := renderChangeBody(changes)
	section := fmt.Sprintf("\n\n## [%s] - %s\n\n%s\n", version, date.Format("2006-01-02"), generated)
	updated := text[:bodyStart] + section + text[next:]
	return writeFileAtomic(path, []byte(updated), 0o644)
}

func renderChangeBody(changes []changeRecord) string {
	cats := map[string][]string{}
	for _, c := range changes {
		category := "Alterado"
		if !c.Breaking && c.Impact == impactMinor {
			category = "Adicionado"
		} else if !c.Breaking && c.Impact == impactPatch {
			category = "Corrigido"
		}
		cats[category] = append(cats[category], fmt.Sprintf("- **%s:** %s", c.Component, strings.TrimSpace(c.Summary)))
	}
	order := []string{"Adicionado", "Alterado", "Corrigido", "Segurança", "Descontinuado", "Removido"}
	var b strings.Builder
	for _, cat := range order {
		items := cats[cat]
		if len(items) == 0 {
			continue
		}
		sort.Strings(items)
		fmt.Fprintf(&b, "### %s\n\n%s\n\n", cat, strings.Join(items, "\n"))
	}
	return strings.TrimSpace(b.String())
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".release-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func contractToolNames(cmdRoot string) ([]string, error) {
	manifests, err := discoverToolManifests(cmdRoot)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(manifests))
	for name := range manifests {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func generateContract(cmdRoot, name string) ([]byte, error) {
	sandbox, err := os.MkdirTemp("", "cli-tools-contract-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(sandbox)

	home := filepath.Join(sandbox, "home")
	config := filepath.Join(sandbox, "config")
	data := filepath.Join(sandbox, "data")
	cache := filepath.Join(sandbox, "cache")
	tmp := filepath.Join(sandbox, "tmp")
	for _, dir := range []string{home, config, data, cache, tmp} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
	}
	gitConfig := filepath.Join(sandbox, "gitconfig")
	if err := os.WriteFile(gitConfig, nil, 0o600); err != nil {
		return nil, err
	}

	cmd := exec.Command("go", "run", "./"+filepath.ToSlash(filepath.Join(cmdRoot, name)), "__cli", "contract")
	// Contract generation is static. Preserve only the build-tool paths/caches
	// needed to invoke Go, while isolating user HOME/config/credentials and
	// disabling network/module VCS discovery.
	env := []string{
		"HOME=" + home,
		"XDG_CONFIG_HOME=" + config,
		"XDG_DATA_HOME=" + data,
		"XDG_CACHE_HOME=" + cache,
		"GH_CONFIG_DIR=" + filepath.Join(config, "gh"),
		"TMPDIR=" + tmp,
		"GIT_CONFIG_GLOBAL=" + gitConfig,
		"GIT_CONFIG_NOSYSTEM=1",
		"GOENV=off",
		"GOPROXY=off",
		"GOVCS=*:off",
	}
	for _, key := range []string{"PATH", "GOROOT", "GOCACHE", "GOMODCACHE"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("generate %s contract: %w", name, err)
	}
	return out, nil
}

func regenerateContracts(cmdRoot string) error {
	names, err := contractToolNames(cmdRoot)
	if err != nil {
		return err
	}
	generated := make(map[string][]byte, len(names))
	for _, name := range names {
		out, err := generateContract(cmdRoot, name)
		if err != nil {
			return err
		}
		generated[name] = out
	}
	for _, name := range names {
		if err := writeFileAtomic(filepath.Join(cmdRoot, name, "cli.contract.json"), generated[name], 0o644); err != nil {
			return err
		}
	}
	return nil
}

func checkContracts(cmdRoot string) error {
	names, err := contractToolNames(cmdRoot)
	if err != nil {
		return err
	}
	var drift []string
	for _, name := range names {
		generated, err := generateContract(cmdRoot, name)
		if err != nil {
			return err
		}
		path := filepath.Join(cmdRoot, name, "cli.contract.json")
		committed, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s contract lock: %w", name, err)
		}
		if bytes.Equal(committed, generated) {
			continue
		}
		detail := name
		var oldContract, newContract corecli.Contract
		if json.Unmarshal(committed, &oldContract) == nil && json.Unmarshal(generated, &newContract) == nil {
			changes := corecli.DiffContracts(oldContract, newContract)
			if len(changes) != 0 {
				parts := make([]string, 0, len(changes))
				for _, change := range changes {
					parts = append(parts, fmt.Sprintf("%s %s: %s", change.Severity, change.ID, change.Message))
				}
				detail += " [" + strings.Join(parts, "; ") + "]"
			}
		}
		drift = append(drift, detail)
	}
	if len(drift) != 0 {
		return fmt.Errorf("contract lock drift: %s; regenerate through release prepare --write or the CLI contract endpoint", strings.Join(drift, ", "))
	}
	return nil
}

func runContractsCommand(args []string) {
	if len(args) == 0 || args[0] != "check" {
		fatalf("contracts: expected subcommand check")
	}
	fs := flag.NewFlagSet("release contracts check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var cmdRoot string
	fs.StringVar(&cmdRoot, "cmd-root", "cmd", "command root containing tool.json manifests")
	if err := fs.Parse(args[1:]); err != nil {
		fatalf("contracts check flags: %v", err)
	}
	if fs.NArg() != 0 {
		fatalf("contracts check: unexpected arguments: %v", fs.Args())
	}
	if err := checkContracts(cmdRoot); err != nil {
		fatalf("contracts check: %v", err)
	}
	fmt.Println("contract locks match generated CLI contracts")
}

func changedPaths(base, head string) ([]string, error) {
	if head == "" {
		head = "HEAD"
	}
	cmd := exec.Command("git", "diff", "--name-only", base+"..."+head)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			paths = append(paths, filepath.ToSlash(line))
		}
	}
	return paths, nil
}

func requiredChangeComponents(paths []string, tools map[string]toolManifestAtPath) map[string]struct{} {
	required := map[string]struct{}{}
	for _, p := range paths {
		p = filepath.ToSlash(p)
		if strings.HasPrefix(p, "changes/") || strings.HasPrefix(p, "plans/") || strings.HasPrefix(p, "docs/") || p == "README.md" || p == "CHANGELOG.md" {
			continue
		}
		if strings.HasPrefix(p, "cli/") {
			required["module"] = struct{}{}
			for name := range tools {
				required[name] = struct{}{}
			}
			continue
		}
		for name := range tools {
			if strings.HasPrefix(p, "cmd/"+name+"/") {
				required[name] = struct{}{}
			}
			internalName := strings.ReplaceAll(name, "-", "")
			if strings.HasPrefix(p, "internal/"+internalName+"/") {
				required[name] = struct{}{}
			}
		}
		if strings.HasPrefix(p, "tools/release/") || strings.HasPrefix(p, "internal/version/") || p == ".github/workflows/release.yml" {
			required["module"] = struct{}{}
		}
	}
	return required
}

func validateChangeCoverage(paths []string, files []loadedChangeFile, manifests map[string]toolManifestAtPath) error {
	required := requiredChangeComponents(paths, manifests)
	declared := map[string]struct{}{}
	for _, file := range files {
		for _, change := range file.Record.Changes {
			declared[change.Component] = struct{}{}
		}
	}
	var missing []string
	for component := range required {
		if _, ok := declared[component]; !ok {
			missing = append(missing, component)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return fmt.Errorf("changed contract scopes require change records for: %s", strings.Join(missing, ", "))
	}
	return nil
}

func runPrepareCommand(args []string) {
	fs := flag.NewFlagSet("release prepare", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var suiteVersion, cmdRoot, changesDir, changelogPath, dateText, apiDir, apiLock string
	var write bool
	fs.StringVar(&suiteVersion, "suite-version", "", "target suite version in vX.Y.Z or X.Y.Z form")
	fs.StringVar(&cmdRoot, "cmd-root", "cmd", "command root containing tool.json manifests")
	fs.StringVar(&changesDir, "changes", "changes", "pending change-record directory")
	fs.StringVar(&changelogPath, "changelog", "CHANGELOG.md", "suite changelog")
	fs.StringVar(&apiDir, "api-dir", "cli", "public Go API package directory")
	fs.StringVar(&apiLock, "api-lock", "cli/api.contract.json", "public Go API compatibility lock")
	fs.StringVar(&dateText, "date", time.Now().UTC().Format("2006-01-02"), "release date in YYYY-MM-DD")
	fs.BoolVar(&write, "write", false, "apply the prepared versions/changelogs/contracts and archive records")
	if err := fs.Parse(args); err != nil {
		fatalf("prepare flags: %v", err)
	}
	if fs.NArg() != 0 {
		fatalf("prepare: unexpected arguments: %v", fs.Args())
	}
	if suiteVersion == "" {
		fatalf("prepare: --suite-version is required")
	}
	date, err := time.Parse("2006-01-02", dateText)
	if err != nil {
		fatalf("prepare date: %v", err)
	}
	plan, files, manifests, err := planRelease(cmdRoot, changesDir, changelogPath, suiteVersion)
	if err != nil {
		fatalf("prepare: %v", err)
	}
	if _, err := verifyPublicAPILock(apiDir, apiLock); err != nil {
		fatalf("prepare public API: %v", err)
	}
	fmt.Print(renderPreparedRelease(plan))
	if !write {
		fmt.Println("preview only; pass --write to apply")
		return
	}
	if err := writePreparedRelease(plan, files, manifests, cmdRoot, changelogPath, changesDir, date); err != nil {
		fatalf("prepare write: %v", err)
	}
	fmt.Printf("prepared suite v%s\n", plan.NextSuite)
}

func runChangesCommand(args []string) {
	if len(args) == 0 || args[0] != "validate" {
		fatalf("changes: expected subcommand validate")
	}
	fs := flag.NewFlagSet("release changes validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var base, head, cmdRoot, changesDir string
	fs.StringVar(&base, "base", "", "optional git base revision for path coverage")
	fs.StringVar(&head, "head", "HEAD", "git head revision for path coverage")
	fs.StringVar(&cmdRoot, "cmd-root", "cmd", "command root containing tool.json manifests")
	fs.StringVar(&changesDir, "changes", "changes", "pending change-record directory")
	if err := fs.Parse(args[1:]); err != nil {
		fatalf("changes validate flags: %v", err)
	}
	if fs.NArg() != 0 {
		fatalf("changes validate: unexpected arguments: %v", fs.Args())
	}
	manifests, err := discoverToolManifests(cmdRoot)
	if err != nil {
		fatalf("changes validate manifests: %v", err)
	}
	files, err := loadChangeFiles(changesDir, validReleaseComponents(manifests))
	if err != nil {
		fatalf("changes validate records: %v", err)
	}
	if base != "" {
		paths, err := changedPaths(base, head)
		if err != nil {
			fatalf("changes validate diff: %v", err)
		}
		if err := validateChangeCoverage(paths, files, manifests); err != nil {
			fatalf("changes validate coverage: %v", err)
		}
	}
	fmt.Printf("validated %d change record file(s)\n", len(files))
}
