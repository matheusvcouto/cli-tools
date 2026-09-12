package repozip

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/matheusvcouto/cli-tools/internal/testenv"
)

func sandboxEnv(t *testing.T) []string {
	t.Helper()
	env, err := testenv.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func initRepo(t *testing.T) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "init", "-q")
	gitRun(t, repo, "config", "user.name", "Synthetic User")
	gitRun(t, repo, "config", "user.email", "synthetic@example.com")
	return repo
}

func gitRun(t *testing.T, repo string, args ...string) string {
	t.Helper()
	argv := append([]string{"-C", repo}, args...)
	cmd := exec.Command("git", argv...)
	cmd.Env = sandboxEnv(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func service(t *testing.T) Service {
	t.Helper()
	return Service{Git: Git{env: sandboxEnv(t)}, Archiver: Archiver{}}
}

func TestParseArgsFlagsCanAppearAroundSource(t *testing.T) {
	opts, err := ParseArgs([]string{"--git", "-n", "snap", "repo", "-v", "v1"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Source != "repo" || !opts.Git || opts.Name != "snap" || opts.Version != "v1" {
		t.Fatalf("bad options: %+v", opts)
	}
	if _, err := ParseArgs([]string{"-o", "x.zip", "-n", "x"}); err == nil {
		t.Fatal("expected output/name conflict")
	}
	if _, err := ParseArgs([]string{"-o", "x.zip", "-v", "v1"}); err == nil {
		t.Fatal("expected output/suffix conflict")
	}
}

func TestSnapshotUsesGitSelectionAndExcludesIgnoredFiles(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, ".gitignore"), "ignored.txt\n")
	writeFile(t, filepath.Join(repo, "tracked.txt"), "tracked")
	writeFile(t, filepath.Join(repo, "untracked.txt"), "untracked")
	writeFile(t, filepath.Join(repo, "ignored.txt"), "ignored")
	gitRun(t, repo, "add", ".gitignore", "tracked.txt")

	res, err := service(t).Run(context.Background(), Options{Source: repo})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(res.Output) != filepath.Join(repo, ".tmp", "repo-zip") {
		t.Fatalf("unexpected output: %s", res.Output)
	}
	names := zipNames(t, res.Output)
	for _, want := range []string{".gitignore", "tracked.txt", "untracked.txt"} {
		if !names[want] {
			t.Fatalf("missing %s in %#v", want, names)
		}
	}
	if names["ignored.txt"] {
		t.Fatal("ignored file was archived")
	}
	for name := range names {
		if strings.HasPrefix(name, ".git/") {
			t.Fatal(".git included without --git")
		}
	}
}

func TestSnapshotTreatsEquivalentRepositoryPathSpellingsAsTheSameRoot(t *testing.T) {
	realParent := t.TempDir()
	repo := filepath.Join(realParent, "repo")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "init", "-q")
	gitRun(t, repo, "config", "user.name", "Synthetic User")
	gitRun(t, repo, "config", "user.email", "synthetic@example.com")
	writeFile(t, filepath.Join(repo, "tracked.txt"), "tracked")
	gitRun(t, repo, "add", "tracked.txt")

	aliasParent := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(realParent, aliasParent); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	aliasRepo := filepath.Join(aliasParent, "repo")
	output := filepath.Join(aliasRepo, "snapshot.zip")

	res, err := service(t).Run(context.Background(), Options{Source: aliasRepo, Output: output})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != output {
		t.Fatalf("output path spelling changed: got %q want %q", res.Output, output)
	}
	zr, err := zip.OpenReader(output)
	if err != nil {
		t.Fatalf("open snapshot through aliased repository path: %v", err)
	}
	defer zr.Close()
}

func TestEquivalentAncestorSpellingDoesNotReturnSymlinkRoot(t *testing.T) {
	realRoot := t.TempDir()
	alias := filepath.Join(t.TempDir(), "repo-link")
	if err := os.Symlink(realRoot, alias); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}

	got := equivalentAncestorSpelling(alias, realRoot)
	if got != realRoot {
		t.Fatalf("symlink root was preserved: got %q want %q", got, realRoot)
	}
}

func TestSnapshotRejectsBackslashPathForPortableZip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("backslash is a path separator on Windows")
	}
	repo := initRepo(t)
	name := `weird\name.txt`
	writeFile(t, filepath.Join(repo, name), "content")
	gitRun(t, repo, "add", "--", name)

	_, err := service(t).Run(context.Background(), Options{Source: repo})
	if err == nil || !strings.Contains(err.Error(), "backslash") {
		t.Fatalf("expected portable ZIP backslash rejection, got %v", err)
	}
}

func TestSymlinkIsPreservedAndExternalTargetNotRead(t *testing.T) {
	repo := initRepo(t)
	outside := filepath.Join(t.TempDir(), "outside-secret.txt")
	writeFile(t, outside, "must-not-be-archived")
	link := filepath.Join(repo, "external-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	gitRun(t, repo, "add", "external-link")
	res, err := service(t).Run(context.Background(), Options{Source: repo})
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(res.Output)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var found bool
	for _, f := range zr.File {
		if f.Name != "external-link" {
			continue
		}
		found = true
		if f.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("entry not marked symlink: %v", f.Mode())
		}
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			t.Fatal(err)
		}
		if string(raw) != outside {
			t.Fatalf("symlink payload should be target path, got %q", raw)
		}
		if strings.Contains(string(raw), "must-not-be-archived") {
			t.Fatal("external target content leaked")
		}
	}
	if !found {
		t.Fatal("symlink entry missing")
	}
}

func TestGitSnapshotIncludesRestorableBundleAndMetadata(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "file.txt"), "one")
	gitRun(t, repo, "add", "file.txt")
	gitRun(t, repo, "commit", "-qm", "initial")
	headShort := gitRun(t, repo, "rev-parse", "--short=12", "HEAD")
	headFull := gitRun(t, repo, "rev-parse", "HEAD")
	branch := gitRun(t, repo, "symbolic-ref", "-q", "HEAD")
	writeFile(t, filepath.Join(repo, "file.txt"), "two")

	res, err := service(t).Run(context.Background(), Options{Source: repo, Git: true})
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Base(res.Output)
	if !strings.Contains(base, headShort) || !strings.Contains(base, "dirty") {
		t.Fatalf("unexpected name: %s", base)
	}
	names := zipNames(t, res.Output)
	if !names[".repo-zip/repository.bundle"] || !names[".repo-zip/metadata.json"] {
		t.Fatalf("Git bundle metadata missing: %#v", names)
	}
	if names["ignored.log"] {
		t.Fatal("ignored worktree file was archived")
	}
	for name := range names {
		if strings.HasPrefix(name, ".git/") || name == ".git" {
			t.Fatalf("raw .git metadata must not be archived: %s", name)
		}
	}

	bundle := extractZipEntry(t, res.Output, ".repo-zip/repository.bundle")
	bundlePath := filepath.Join(t.TempDir(), "repository.bundle")
	if err := os.WriteFile(bundlePath, bundle, 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "bundle", "verify", bundlePath)

	var meta struct {
		SchemaVersion int    `json:"schema_version"`
		Head          string `json:"head"`
		Branch        string `json:"branch"`
		Dirty         bool   `json:"dirty"`
	}
	if err := json.Unmarshal(extractZipEntry(t, res.Output, ".repo-zip/metadata.json"), &meta); err != nil {
		t.Fatal(err)
	}
	if meta.SchemaVersion != 1 || meta.Head != headFull || meta.Branch != branch || !meta.Dirty {
		t.Fatalf("unexpected Git metadata: %+v", meta)
	}
}

func TestGitSnapshotMetadataIgnoresItsOwnExplicitOutput(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "file.txt"), "clean")
	gitRun(t, repo, "add", "file.txt")
	gitRun(t, repo, "commit", "-qm", "initial")
	output := filepath.Join(repo, "snapshot.zip")

	res, err := service(t).Run(context.Background(), Options{Source: repo, Git: true, Output: output})
	if err != nil {
		t.Fatal(err)
	}
	var meta struct {
		Dirty bool `json:"dirty"`
	}
	if err := json.Unmarshal(extractZipEntry(t, res.Output, ".repo-zip/metadata.json"), &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Dirty {
		t.Fatal("Git metadata was dirtied by repo-zip's own temporary/output files")
	}
}

func TestGitSnapshotRejectsReservedArchiveNamespaceConflict(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "file.txt"), "data")
	writeFile(t, filepath.Join(repo, ".repo-zip", "metadata.json"), "user-data")
	gitRun(t, repo, "add", "file.txt", ".repo-zip/metadata.json")
	gitRun(t, repo, "commit", "-qm", "initial")

	_, err := service(t).Run(context.Background(), Options{Source: repo, Git: true})
	if err == nil || !strings.Contains(err.Error(), "reserves .repo-zip/") {
		t.Fatalf("expected reserved namespace conflict, got %v", err)
	}
}

func TestGitSnapshotWorksFromLinkedWorktree(t *testing.T) {
	root := t.TempDir()
	mainRepo := filepath.Join(root, "main")
	if err := os.Mkdir(mainRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, mainRepo, "init", "-q")
	gitRun(t, mainRepo, "config", "user.name", "Synthetic User")
	gitRun(t, mainRepo, "config", "user.email", "synthetic@example.com")
	writeFile(t, filepath.Join(mainRepo, ".gitignore"), "ignored.log\n")
	writeFile(t, filepath.Join(mainRepo, "tracked.txt"), "main")
	gitRun(t, mainRepo, "add", ".gitignore", "tracked.txt")
	gitRun(t, mainRepo, "commit", "-qm", "initial")

	worktree := filepath.Join(root, "worktree")
	gitRun(t, mainRepo, "worktree", "add", "-q", "-b", "feature", worktree)
	writeFile(t, filepath.Join(worktree, "tracked.txt"), "worktree-dirty")
	writeFile(t, filepath.Join(worktree, "untracked.txt"), "untracked")
	writeFile(t, filepath.Join(worktree, "ignored.log"), "ignored")
	head := gitRun(t, worktree, "rev-parse", "HEAD")

	output := filepath.Join(t.TempDir(), "snapshot.zip")
	res, err := service(t).Run(context.Background(), Options{Source: worktree, Git: true, Output: output})
	if err != nil {
		t.Fatalf("linked worktree --git snapshot failed: %v", err)
	}
	if res.Output != output {
		t.Fatalf("unexpected output: %s", res.Output)
	}
	names := zipNames(t, output)
	for _, want := range []string{".gitignore", "tracked.txt", "untracked.txt", ".repo-zip/repository.bundle", ".repo-zip/metadata.json"} {
		if !names[want] {
			t.Fatalf("missing %s from worktree snapshot: %#v", want, names)
		}
	}
	if names["ignored.log"] {
		t.Fatal("ignored worktree file was archived")
	}
	for name := range names {
		if strings.HasPrefix(name, ".git/") || name == ".git" {
			t.Fatalf("raw worktree gitfile/.git metadata must not be archived: %s", name)
		}
	}

	bundlePath := filepath.Join(t.TempDir(), "repository.bundle")
	if err := os.WriteFile(bundlePath, extractZipEntry(t, output, ".repo-zip/repository.bundle"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, worktree, "bundle", "verify", bundlePath)

	restored := filepath.Join(t.TempDir(), "restored")
	cmd := exec.Command("git", "clone", "-q", bundlePath, restored)
	cmd.Env = sandboxEnv(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone from bundle: %v\n%s", err, out)
	}
	if got := gitRun(t, restored, "rev-parse", "HEAD"); got != head {
		t.Fatalf("restored bundle HEAD = %s, want %s", got, head)
	}

	var meta struct {
		SchemaVersion int    `json:"schema_version"`
		Head          string `json:"head"`
		Branch        string `json:"branch"`
		Dirty         bool   `json:"dirty"`
	}
	if err := json.Unmarshal(extractZipEntry(t, output, ".repo-zip/metadata.json"), &meta); err != nil {
		t.Fatal(err)
	}
	if meta.SchemaVersion != 1 || meta.Head != head || meta.Branch != "refs/heads/feature" || !meta.Dirty {
		t.Fatalf("unexpected worktree metadata: %+v", meta)
	}
}

type callbackArchiver struct {
	base        Archiver
	afterCreate func()
}

func (a callbackArchiver) Create(repo string, dst io.Writer, files []string, gitMeta *GitSnapshotMetadata) error {
	if err := a.base.Create(repo, dst, files, gitMeta); err != nil {
		return err
	}
	if a.afterCreate != nil {
		a.afterCreate()
	}
	return nil
}
func (a callbackArchiver) Verify(src io.ReaderAt, size int64) error { return a.base.Verify(src, size) }

func TestGitSnapshotAbortsIfRepositoryChanges(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "file.txt"), "one")
	gitRun(t, repo, "add", "file.txt")
	gitRun(t, repo, "commit", "-qm", "initial")
	output := filepath.Join(t.TempDir(), "snapshot.zip")
	s := Service{Git: Git{env: sandboxEnv(t)}, Archiver: callbackArchiver{base: Archiver{}, afterCreate: func() {
		writeFile(t, filepath.Join(repo, "changed-during-snapshot.txt"), "change")
	}}}
	_, err := s.Run(context.Background(), Options{Source: repo, Git: true, Output: output})
	if err == nil || !strings.Contains(err.Error(), "state changed") {
		t.Fatalf("expected state-change error, got %v", err)
	}
	if _, statErr := os.Stat(output); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("output was published: %v", statErr)
	}
}

func TestGitSnapshotAbortsIfAnyGitRefChanges(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "file.txt"), "one")
	gitRun(t, repo, "add", "file.txt")
	gitRun(t, repo, "commit", "-qm", "first")
	first := gitRun(t, repo, "rev-parse", "HEAD")
	gitRun(t, repo, "branch", "other", first)
	writeFile(t, filepath.Join(repo, "file.txt"), "two")
	gitRun(t, repo, "commit", "-qam", "second")
	second := gitRun(t, repo, "rev-parse", "HEAD")

	output := filepath.Join(t.TempDir(), "snapshot.zip")
	s := Service{Git: Git{env: sandboxEnv(t)}, Archiver: callbackArchiver{base: Archiver{}, afterCreate: func() {
		gitRun(t, repo, "update-ref", "refs/heads/other", second)
	}}}
	_, err := s.Run(context.Background(), Options{Source: repo, Git: true, Output: output})
	if err == nil || !strings.Contains(err.Error(), "state changed") {
		t.Fatalf("expected Git ref change to abort snapshot, got %v", err)
	}
	if _, statErr := os.Stat(output); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("output was published after ref change: %v", statErr)
	}
}

func TestTrackedOutputIsNeverOverwritten(t *testing.T) {
	repo := initRepo(t)
	output := filepath.Join(repo, "archive.zip")
	writeFile(t, output, "tracked-original")
	gitRun(t, repo, "add", "archive.zip")
	gitRun(t, repo, "commit", "-qm", "track output")
	_, err := service(t).Run(context.Background(), Options{Source: repo, Output: output, Force: true})
	if err == nil || !strings.Contains(err.Error(), "tracked") {
		t.Fatalf("expected tracked guard, got %v", err)
	}
	raw, _ := os.ReadFile(output)
	if string(raw) != "tracked-original" {
		t.Fatalf("tracked output changed: %q", raw)
	}
}

func TestForceReplacesExistingRegularFile(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "file.txt"), "data")
	output := filepath.Join(t.TempDir(), "snapshot.zip")
	writeFile(t, output, "old")
	res, err := service(t).Run(context.Background(), Options{Source: repo, Output: output, Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != output {
		t.Fatalf("unexpected output %s", res.Output)
	}
	if _, err := zip.OpenReader(output); err != nil {
		t.Fatalf("replacement is not a zip: %v", err)
	}
}

func TestNoForceRefusesExistingDestination(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "file.txt"), "data")
	output := filepath.Join(t.TempDir(), "snapshot.zip")
	writeFile(t, output, "old")
	_, err := service(t).Run(context.Background(), Options{Source: repo, Output: output})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected existing destination error, got %v", err)
	}
}

func TestDefaultOutputRejectsSymlinkedInternalDirectory(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "file.txt"), "data")
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(repo, ".tmp")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	_, err := service(t).Run(context.Background(), Options{Source: repo})
	if err == nil || !strings.Contains(err.Error(), "unsafe default output") {
		t.Fatalf("expected symlink guard, got %v", err)
	}
	entries, _ := os.ReadDir(outside)
	if len(entries) != 0 {
		t.Fatalf("wrote outside repo through symlink: %v", entries)
	}
}

func TestSkipWorktreeIsRejected(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "file.txt"), "data")
	gitRun(t, repo, "add", "file.txt")
	gitRun(t, repo, "commit", "-qm", "initial")
	gitRun(t, repo, "update-index", "--skip-worktree", "file.txt")
	_, err := service(t).Run(context.Background(), Options{Source: repo})
	if err == nil || !strings.Contains(err.Error(), "skip-worktree") {
		t.Fatalf("expected sparse guard, got %v", err)
	}
}

func TestGitSnapshotDoesNotDependOnRawGitLockFiles(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "file.txt"), "data")
	gitRun(t, repo, "add", "file.txt")
	gitRun(t, repo, "commit", "-qm", "initial")
	writeFile(t, filepath.Join(repo, ".git", "index.lock"), "stale-lock")

	res, err := service(t).Run(context.Background(), Options{Source: repo, Git: true})
	if err != nil {
		t.Fatalf("bundle snapshot should not inspect raw Git lock files: %v", err)
	}
	for name := range zipNames(t, res.Output) {
		if strings.HasPrefix(name, ".git/") || name == ".git" {
			t.Fatalf("raw Git metadata leaked into archive: %s", name)
		}
	}
}

func TestGitSnapshotRequiresCommitForRestorableBundle(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "uncommitted.txt"), "data")
	output := filepath.Join(t.TempDir(), "snapshot.zip")

	_, err := service(t).Run(context.Background(), Options{Source: repo, Git: true, Output: output})
	if err == nil || !strings.Contains(err.Error(), "at least one commit") {
		t.Fatalf("expected clear bundle requirement error, got %v", err)
	}
	if _, statErr := os.Stat(output); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("output should not be published on bundle failure: %v", statErr)
	}
}

func TestGitlinkSubmoduleIsRejected(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "file.txt"), "data")
	gitRun(t, repo, "add", "file.txt")
	gitRun(t, repo, "commit", "-qm", "initial")
	head := gitRun(t, repo, "rev-parse", "HEAD")
	gitRun(t, repo, "update-index", "--add", "--cacheinfo", "160000,"+head+",submodule")
	_, err := service(t).Run(context.Background(), Options{Source: repo})
	if err == nil || !strings.Contains(err.Error(), "submodules") {
		t.Fatalf("expected submodule guard, got %v", err)
	}
}

func TestVerifyRejectsCorruptZip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.zip")
	writeFile(t, path, "not a zip")
	if err := (Archiver{}).VerifyPath(path); err == nil {
		t.Fatal("expected corrupt zip error")
	}
}

func zipNames(t *testing.T, path string) map[string]bool {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	m := map[string]bool{}
	for _, f := range zr.File {
		m[f.Name] = true
	}
	return m
}

func TestOutputInsideGitIsRejected(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "file.txt"), "data")
	_, err := service(t).Run(context.Background(), Options{Source: repo, Output: filepath.Join(repo, ".git", "x.zip")})
	if err == nil || !strings.Contains(err.Error(), ".git") {
		t.Fatalf("expected .git output guard, got %v", err)
	}
}

func TestExplicitOutputSymlinkEscapeInsideRepoIsRejected(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "file.txt"), "data")
	outside := t.TempDir()
	link := filepath.Join(repo, "out")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	outsideChild := filepath.Join(outside, "must-not-be-created")
	_, err := service(t).Run(context.Background(), Options{Source: repo, Output: filepath.Join(link, "must-not-be-created", "x.zip")})
	if err == nil || !strings.Contains(err.Error(), "escapes repository") {
		t.Fatalf("expected escape guard, got %v", err)
	}
	if _, statErr := os.Lstat(outsideChild); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("unsafe parent creation escaped repository: %v", statErr)
	}
}

func extractZipEntry(t *testing.T, path, name string) []byte {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(r)
		closeErr := r.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		return raw
	}
	t.Fatalf("ZIP entry not found: %s", name)
	return nil
}

func ExampleParseArgs() {
	opts, _ := ParseArgs([]string{".", "--git", "--version", "v2"})
	fmt.Println(opts.Source, opts.Git, opts.Version)
	// Output: . true v2
}

func TestSnapshotAbortsIfEligibleFileSetChangesWithoutGitMetadata(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "file.txt"), "one")
	gitRun(t, repo, "add", "file.txt")
	output := filepath.Join(t.TempDir(), "snapshot.zip")
	s := Service{Git: Git{env: sandboxEnv(t)}, Archiver: callbackArchiver{base: Archiver{}, afterCreate: func() {
		writeFile(t, filepath.Join(repo, "appeared-during-snapshot.txt"), "change")
	}}}
	_, err := s.Run(context.Background(), Options{Source: repo, Output: output})
	if err == nil || !strings.Contains(err.Error(), "file set changed") {
		t.Fatalf("expected file-set change error, got %v", err)
	}
	if _, statErr := os.Stat(output); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("output was published: %v", statErr)
	}
}

func TestSanitizedGitEnvRemovesRepositoryOverrides(t *testing.T) {
	in := []string{
		"PATH=/usr/bin",
		"GIT_DIR=/tmp/evil-git-dir",
		"GIT_WORK_TREE=/tmp/evil-work-tree",
		"GIT_INDEX_FILE=/tmp/evil-index",
		"GIT_OBJECT_DIRECTORY=/tmp/evil-objects",
		"GIT_COMMON_DIR=/tmp/evil-common",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES=/tmp/evil-alternates",
		"KEEP_ME=yes",
	}
	got := sanitizedGitEnv(in)
	joined := strings.Join(got, "\n")
	for _, forbidden := range []string{
		"GIT_DIR=", "GIT_WORK_TREE=", "GIT_INDEX_FILE=", "GIT_OBJECT_DIRECTORY=", "GIT_COMMON_DIR=", "GIT_ALTERNATE_OBJECT_DIRECTORIES=",
	} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("sanitized environment still contains %s: %q", forbidden, got)
		}
	}
	if !strings.Contains(joined, "PATH=/usr/bin") || !strings.Contains(joined, "KEEP_ME=yes") {
		t.Fatalf("unrelated environment variables were removed: %q", got)
	}
}

func TestDefaultSourceWorksFromRepositoryWorkingDirectory(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "tracked.txt"), "tracked")
	gitRun(t, repo, "add", "tracked.txt")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	}()

	res, err := service(t).Run(context.Background(), Options{Source: "."})
	if err != nil {
		t.Fatalf("repo-zip . from repository root failed: %v", err)
	}
	if filepath.Dir(res.Output) != filepath.Join(repo, ".tmp", "repo-zip") {
		t.Fatalf("unexpected output: %s", res.Output)
	}
	if _, err := os.Stat(res.Output); err != nil {
		t.Fatalf("output missing: %v", err)
	}
}
