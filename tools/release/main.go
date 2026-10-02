package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/matheusvcouto/cli-tools/internal/fscommit"
	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

type target struct{ GOOS, GOARCH string }

var defaultTargets = []target{{"darwin", "amd64"}, {"darwin", "arm64"}, {"linux", "amd64"}, {"linux", "arm64"}, {"windows", "amd64"}, {"windows", "arm64"}}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "prepare":
			runPrepareCommand(os.Args[2:])
			return
		case "preflight":
			runPreflightCommand(os.Args[2:])
			return
		case "changes":
			runChangesCommand(os.Args[2:])
			return
		case "contracts":
			runContractsCommand(os.Args[2:])
			return
		case "api":
			runAPICommand(os.Args[2:])
			return
		case "build":
			os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
		}
	}
	var version, outDir, changelogPath, notesOut string
	flag.StringVar(&version, "version", "", "release version in vX.Y.Z format")
	flag.StringVar(&outDir, "out", "dist", "output directory")
	flag.StringVar(&changelogPath, "changelog", "CHANGELOG.md", "versioned changelog")
	flag.StringVar(&notesOut, "notes-out", "", "optional path for the extracted release notes")
	flag.Parse()
	if flag.NArg() != 0 {
		fatalf("unexpected arguments: %v", flag.Args())
	}
	notes, err := releaseNotesFromFile(changelogPath, version)
	if err != nil {
		fatalf("release notes: %v", err)
	}
	if err := requireReleaseToolchain(); err != nil {
		fatalf("toolchain: %v", err)
	}
	module, err := modulePath()
	if err != nil {
		fatalf("module path: %v", err)
	}
	parsedVersion, err := parseReleaseSemver(version)
	if err != nil {
		fatalf("release version: %v", err)
	}
	if err := validateReleaseModulePath(module, parsedVersion); err != nil {
		fatalf("release module version: %v", err)
	}
	bins, err := discoverCommands("cmd")
	if err != nil {
		fatalf("discover commands: %v", err)
	}
	if len(bins) == 0 {
		fatalf("no commands found under cmd/")
	}
	if err := prepareOutputDir(outDir); err != nil {
		fatalf("prepare output: %v", err)
	}
	if err := validateNotesOut(outDir, notesOut); err != nil {
		fatalf("release notes output: %v", err)
	}
	for _, t := range defaultTargets {
		if err := buildTarget(version, outDir, module, bins, t); err != nil {
			fatalf("build %s/%s: %v", t.GOOS, t.GOARCH, err)
		}
	}
	if err := verifyReleaseArchiveSet(outDir, version); err != nil {
		fatalf("release archive set: %v", err)
	}
	if err := writeChecksums(outDir); err != nil {
		fatalf("checksums: %v", err)
	}
	if notesOut != "" {
		if err := writeReleaseNotes(notesOut, []byte(notes)); err != nil {
			fatalf("write release notes: %v", err)
		}
	}
	// Re-verify the immutable publication bundle after writing notes. This is a
	// deliberate second boundary: even if an alias/race made notesOut resolve
	// into outDir after validation, release preparation fails rather than
	// silently publishing modified bytes.
	if err := verifyReleaseBundle(outDir, version); err != nil {
		fatalf("final release bundle: %v", err)
	}
}

var releaseVersionRE = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// goVersionRE parses `go env GOVERSION`: go1.27.1, go1.28, or go1.27.2-custom.
var goVersionRE = regexp.MustCompile(`^go(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:\.(0|[1-9][0-9]*))?(?:[-+].*)?$`)

var releaseNoteCategories = map[string]struct{}{
	"Adicionado":    {},
	"Alterado":      {},
	"Corrigido":     {},
	"Segurança":     {},
	"Descontinuado": {},
	"Removido":      {},
}

func releaseNotesFromFile(path, version string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return extractReleaseNotes(string(raw), version)
}

func extractReleaseNotes(changelog, version string) (string, error) {
	if !releaseVersionRE.MatchString(version) {
		return "", fmt.Errorf("version %q must match vX.Y.Z without leading zeroes", version)
	}

	wantPrefix := "## [" + strings.TrimPrefix(version, "v") + "] - "
	lines := strings.Split(strings.ReplaceAll(changelog, "\r\n", "\n"), "\n")
	start := -1
	date := ""
	for i, line := range lines {
		if !strings.HasPrefix(line, wantPrefix) {
			continue
		}
		if start >= 0 {
			return "", fmt.Errorf("duplicate changelog section for %s", version)
		}
		date = strings.TrimSpace(strings.TrimPrefix(line, wantPrefix))
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return "", fmt.Errorf("changelog date for %s must use YYYY-MM-DD: %w", version, err)
		}
		start = i
	}
	if start < 0 {
		return "", fmt.Errorf("CHANGELOG.md has no section %q", wantPrefix+"YYYY-MM-DD")
	}

	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			end = i
			break
		}
	}
	bodyLines := lines[start+1 : end]
	category := ""
	categoryHasItem := false
	categoryCount := 0
	for _, line := range bodyLines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "### ") {
			if category != "" && !categoryHasItem {
				return "", fmt.Errorf("changelog category %q for %s has no Markdown list item", category, version)
			}
			category = strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
			if _, ok := releaseNoteCategories[category]; !ok {
				return "", fmt.Errorf("unsupported changelog category %q for %s", category, version)
			}
			categoryHasItem = false
			categoryCount++
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			if category == "" {
				return "", fmt.Errorf("changelog list item for %s must be under a supported category", version)
			}
			categoryHasItem = true
		}
	}
	if categoryCount == 0 {
		return "", fmt.Errorf("changelog section for %s must contain at least one supported category", version)
	}
	if !categoryHasItem {
		return "", fmt.Errorf("changelog category %q for %s has no Markdown list item", category, version)
	}
	body := strings.TrimSpace(strings.Join(bodyLines, "\n"))
	return fmt.Sprintf("## %s — %s\n\n%s\n", version, date, body), nil
}

func validateNotesOut(outDir, notesOut string) error {
	if strings.TrimSpace(notesOut) == "" {
		return nil
	}
	outAbs, err := filepath.Abs(outDir)
	if err != nil {
		return err
	}
	notesAbs, err := filepath.Abs(notesOut)
	if err != nil {
		return err
	}
	if pathWithin(outAbs, notesAbs) {
		return fmt.Errorf("notes output must be outside release artifact directory %q", outDir)
	}

	// The lexical check above is not enough: a parent symlink can make a path
	// that appears outside dist resolve physically inside it. Resolve only the
	// existing prefix of each path, then append the still-missing suffix. This
	// targeted canonicalization is appropriate for this containment boundary;
	// it is intentionally not used as a generic filesystem safety primitive.
	outPhysical, err := canonicalFuturePath(outAbs)
	if err != nil {
		return fmt.Errorf("resolve release artifact directory: %w", err)
	}
	notesPhysical, err := canonicalFuturePath(notesAbs)
	if err != nil {
		return fmt.Errorf("resolve release notes output: %w", err)
	}
	if pathWithin(outPhysical, notesPhysical) {
		return fmt.Errorf("notes output resolves inside release artifact directory %q", outDir)
	}
	return nil
}

func pathWithin(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

// canonicalFuturePath resolves symlinks/reparse aliases in the nearest
// existing prefix while preserving any not-yet-created suffix. The caller can
// then reason about where a future MkdirAll/create operation will land.
func canonicalFuturePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	current := filepath.Clean(abs)
	var suffix []string
	for {
		_, statErr := os.Stat(current)
		if statErr == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", statErr
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

func writeReleaseNotes(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".release-notes-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o644); err != nil {
		return err
	}
	if _, err := tmp.Write(content); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := fscommit.ReplacePath(tmpName, path); err != nil {
		return err
	}
	committed = true
	return nil
}

func prepareOutputDir(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("output directory is required")
	}
	clean := filepath.Clean(path)
	if clean == "." || filepath.Dir(clean) == clean {
		return fmt.Errorf("refusing unsafe output directory %q", path)
	}
	resolved, err := safefs.EnsureDir(clean, 0o755)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(resolved)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("output directory must be empty; refusing to remove existing content: %s", clean)
	}
	return nil
}

func discoverCommands(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var names []string
	seen := map[string]string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		fold := strings.ToLower(name)
		if prev, ok := seen[fold]; ok {
			return nil, fmt.Errorf("case-folding collision: %q and %q", prev, name)
		}
		seen[fold] = name
		cmd := exec.Command(goCommandName(), "list", "-f", "{{.Name}}", "./"+filepath.ToSlash(filepath.Join(root, name)))
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("%s is not a valid Go package: %w", name, err)
		}
		if strings.TrimSpace(string(out)) != "main" {
			return nil, fmt.Errorf("%s is not package main", name)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func modulePath() (string, error) {
	cmd := exec.Command(goCommandName(), "list", "-m", "-f", "{{.Path}}")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", fmt.Errorf("empty module path")
	}
	return path, nil
}

// A v2+ Go module must have the same major-version suffix as the Git tag.
// Reject the mismatch before changing release output or preparing changelogs.
func validateReleaseModulePath(module string, version semVersion) error {
	const base = "github.com/matheusvcouto/cli-tools"
	want := base
	if version.Major >= 2 {
		want = fmt.Sprintf("%s/v%d", base, version.Major)
	}
	if module != want {
		return fmt.Errorf("release v%s requires module %q; go.mod declares %q", version, want, module)
	}
	return nil
}

func requireReleaseToolchain() error {
	cmd := exec.Command(goCommandName(), "env", "GOVERSION")
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	version := strings.TrimSpace(string(out))
	parts, err := parseGoVersion(version)
	if err != nil {
		return err
	}
	min := [3]int{1, 27, 1}
	for i := range parts {
		if parts[i] > min[i] {
			return nil
		}
		if parts[i] < min[i] {
			return fmt.Errorf("Go %d.%d.%d or newer is required for release builds; found %s", min[0], min[1], min[2], version)
		}
	}
	return nil
}

func parseGoVersion(version string) ([3]int, error) {
	m := goVersionRE.FindStringSubmatch(version)
	if m == nil {
		return [3]int{}, fmt.Errorf("cannot parse GOVERSION %q", version)
	}
	parts := [3]int{}
	for i := range parts {
		if i+1 >= len(m) || m[i+1] == "" {
			continue
		}
		v, err := strconv.Atoi(m[i+1])
		if err != nil {
			return [3]int{}, err
		}
		parts[i] = v
	}
	return parts, nil
}

func buildTarget(version, outDir, module string, bins []string, t target) error {
	stage, err := os.MkdirTemp("", "cli-tools-release-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	binDir := filepath.Join(stage, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	for _, name := range bins {
		exe := name
		if t.GOOS == "windows" {
			exe += ".exe"
		}
		out := filepath.Join(binDir, exe)
		args := []string{"build", "-trimpath", "-ldflags", fmt.Sprintf("-s -w -X %s/internal/version.SuiteVersion=%s", module, version), "-o", out, "./cmd/" + name}
		cmd := exec.Command(goCommandName(), args...)
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+t.GOOS, "GOARCH="+t.GOARCH)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	if err := copyThirdPartyNotices(stage); err != nil {
		return err
	}
	archive := releaseArchiveName(version, t)
	if t.GOOS == "windows" {
		return zipDir(filepath.Join(outDir, archive), stage)
	}
	return tarGzDir(filepath.Join(outDir, archive), stage)
}

// Notices must accompany binaries containing the vendored BSD Go libraries.
func copyThirdPartyNotices(stage string) error {
	notices, err := os.ReadFile("THIRD_PARTY_NOTICES.txt")
	if err != nil {
		return fmt.Errorf("third-party notices: %w", err)
	}
	return os.WriteFile(filepath.Join(stage, "THIRD_PARTY_NOTICES.txt"), notices, 0644)
}

func releaseArchiveName(version string, t target) string {
	ext := ".tar.gz"
	if t.GOOS == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("cli-tools_%s_%s_%s%s", strings.TrimPrefix(version, "v"), releaseOSName(t.GOOS), t.GOARCH, ext)
}

func verifyReleaseArchiveSet(outDir, version string) error {
	expected := make(map[string]struct{}, len(defaultTargets))
	for _, t := range defaultTargets {
		name := releaseArchiveName(version, t)
		if _, exists := expected[name]; exists {
			return fmt.Errorf("duplicate release archive name %q", name)
		}
		expected[name] = struct{}{}
	}

	entries, err := os.ReadDir(outDir)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			return fmt.Errorf("unexpected directory in release output: %s", entry.Name())
		}
		if _, ok := expected[entry.Name()]; !ok {
			return fmt.Errorf("unexpected release output %q", entry.Name())
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("release output is not a regular file: %s", entry.Name())
		}
		seen[entry.Name()] = struct{}{}
	}
	for name := range expected {
		if _, ok := seen[name]; !ok {
			return fmt.Errorf("missing release archive %q", name)
		}
	}
	return nil
}

func releaseOSName(goos string) string {
	if goos == "darwin" {
		return "macos"
	}
	return goos
}

func tarGzDir(dst, root string) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			_ = f.Close()
		}
	}()

	gz := gzip.NewWriter(f)
	gz.Header.ModTime = time.Unix(0, 0).UTC()
	gz.Header.OS = 255
	tw := tar.NewWriter(gz)
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		h, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		h.Name = filepath.ToSlash(rel)
		h.ModTime = time.Unix(0, 0).UTC()
		h.AccessTime = time.Time{}
		h.ChangeTime = time.Time{}
		h.Uid = 0
		h.Gid = 0
		h.Uname = ""
		h.Gname = ""
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, in)
		closeErr := in.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	closed = true
	return nil
}

func zipDir(dst, root string) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			_ = f.Close()
		}
	}()

	zw := zip.NewWriter(f)
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		h, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		h.Name = filepath.ToSlash(rel)
		h.Method = zip.Deflate
		h.Modified = time.Unix(0, 0).UTC()
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(w, in)
		closeErr := in.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	closed = true
	return nil
}

func verifyReleaseBundle(outDir, version string) error {
	expected := make(map[string]struct{}, len(defaultTargets)+1)
	expected["SHA256SUMS"] = struct{}{}
	for _, t := range defaultTargets {
		expected[releaseArchiveName(version, t)] = struct{}{}
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if _, ok := expected[entry.Name()]; !ok {
			return fmt.Errorf("unexpected final release output %q", entry.Name())
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("final release output is not a regular file: %s", entry.Name())
		}
		seen[entry.Name()] = struct{}{}
	}
	for name := range expected {
		if _, ok := seen[name]; !ok {
			return fmt.Errorf("missing final release output %q", name)
		}
	}
	return verifyChecksums(outDir, version)
}

func verifyChecksums(outDir, version string) error {
	raw, err := os.ReadFile(filepath.Join(outDir, "SHA256SUMS"))
	if err != nil {
		return err
	}
	expected := make(map[string]struct{}, len(defaultTargets))
	for _, t := range defaultTargets {
		expected[releaseArchiveName(version, t)] = struct{}{}
	}
	seen := make(map[string]struct{}, len(expected))
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	for _, line := range lines {
		hashText, name, ok := strings.Cut(strings.TrimSpace(line), "  ")
		if !ok || len(hashText) != sha256.Size*2 || name == "" {
			return fmt.Errorf("invalid checksum entry %q", line)
		}
		if _, err := hex.DecodeString(hashText); err != nil {
			return fmt.Errorf("invalid checksum for %q: %w", name, err)
		}
		if _, ok := expected[name]; !ok {
			return fmt.Errorf("checksum references unexpected release archive %q", name)
		}
		if _, dup := seen[name]; dup {
			return fmt.Errorf("duplicate checksum entry for %q", name)
		}
		seen[name] = struct{}{}
		f, err := os.Open(filepath.Join(outDir, name))
		if err != nil {
			return err
		}
		h := sha256.New()
		_, copyErr := io.Copy(h, f)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		actual := hex.EncodeToString(h.Sum(nil))
		if !strings.EqualFold(actual, hashText) {
			return fmt.Errorf("checksum mismatch for %q", name)
		}
	}
	for name := range expected {
		if _, ok := seen[name]; !ok {
			return fmt.Errorf("missing checksum entry for %q", name)
		}
	}
	return nil
}

func writeChecksums(outDir string) error {
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && e.Name() != "SHA256SUMS" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	f, err := os.Create(filepath.Join(outDir, "SHA256SUMS"))
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			_ = f.Close()
		}
	}()

	w := bufio.NewWriter(f)
	for _, name := range names {
		in, err := os.Open(filepath.Join(outDir, name))
		if err != nil {
			return err
		}
		h := sha256.New()
		_, copyErr := io.Copy(h, in)
		closeErr := in.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if _, err := fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(h.Sum(nil)), name); err != nil {
			return err
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	closed = true
	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
