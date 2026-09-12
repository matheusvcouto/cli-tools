package repozip

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Git struct {
	env []string
}

func (g Git) environment() []string {
	if g.env != nil {
		return append([]string(nil), g.env...)
	}
	return sanitizedGitEnv(os.Environ())
}

type commandResult struct {
	stdout []byte
	stderr []byte
	code   int
}

func (g Git) run(repo string, args ...string) (commandResult, error) {
	argv := append([]string{"-C", repo}, args...)
	cmd := exec.Command("git", argv...)
	cmd.Env = g.environment()
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	res := commandResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), code: 0}
	if err == nil {
		return res, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		res.code = exit.ExitCode()
		return res, nil
	}
	return res, err
}

func sanitizedGitEnv(env []string) []string {
	// Prevent caller-provided repository overrides from redirecting Git away
	// from the explicitly selected -C repository.
	blocked := map[string]struct{}{
		"GIT_DIR": {}, "GIT_WORK_TREE": {}, "GIT_INDEX_FILE": {},
		"GIT_OBJECT_DIRECTORY": {}, "GIT_COMMON_DIR": {},
		"GIT_ALTERNATE_OBJECT_DIRECTORIES": {},
	}
	out := make([]string, 0, len(env))
	for _, item := range env {
		key, _, ok := strings.Cut(item, "=")
		if ok {
			if _, drop := blocked[key]; drop {
				continue
			}
		}
		out = append(out, item)
	}
	return out
}

func detail(res commandResult) string {
	if s := strings.TrimSpace(string(res.stderr)); s != "" {
		return s
	}
	return strings.TrimSpace(string(res.stdout))
}

func (g Git) RepoRoot(source string) (string, error) {
	res, err := g.run(source, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	if res.code != 0 {
		return "", fmt.Errorf("source is not inside a Git repository: %s", detail(res))
	}
	root := strings.TrimSpace(string(res.stdout))
	return filepath.Abs(root)
}

func (g Git) HeadLabel(repo string) (string, error) {
	res, err := g.run(repo, "rev-parse", "--verify", "--short=12", "HEAD")
	if err != nil {
		return "", err
	}
	if res.code == 0 {
		return strings.TrimSpace(string(res.stdout)), nil
	}
	sym, err := g.run(repo, "symbolic-ref", "-q", "HEAD")
	if err != nil {
		return "", err
	}
	if sym.code == 0 {
		return "no-commit", nil
	}
	return "", fmt.Errorf("cannot resolve HEAD: %s", detail(res))
}

func (g Git) AssertNoSubmodules(repo string) error {
	res, err := g.run(repo, "ls-files", "--stage", "-z")
	if err != nil {
		return err
	}
	if res.code != 0 {
		return fmt.Errorf("inspect Git index: %s", detail(res))
	}
	for _, item := range bytes.Split(res.stdout, []byte{0}) {
		if bytes.HasPrefix(item, []byte("160000 ")) {
			return fmt.Errorf("submodules are not supported because a self-contained snapshot cannot be guaranteed")
		}
	}
	return nil
}

func (g Git) AssertNoSparse(repo string) error {
	res, err := g.run(repo, "ls-files", "-t", "-z")
	if err != nil {
		return err
	}
	if res.code != 0 {
		return fmt.Errorf("inspect sparse checkout state: %s", detail(res))
	}
	for _, item := range bytes.Split(res.stdout, []byte{0}) {
		if bytes.HasPrefix(item, []byte("S ")) {
			return fmt.Errorf("sparse checkout/skip-worktree detected; refusing an incomplete snapshot")
		}
	}
	return nil
}

func (g Git) StatusToken(repo string, excludes []string) ([]byte, error) {
	head, err := g.HeadLabel(repo)
	if err != nil {
		return nil, err
	}
	refs, err := g.run(repo, "show-ref", "--head", "-d")
	if err != nil {
		return nil, err
	}
	if refs.code != 0 && refs.code != 1 {
		return nil, fmt.Errorf("inspect Git refs: %s", detail(refs))
	}
	allObjects, err := g.run(repo, "rev-parse", "--all")
	if err != nil {
		return nil, err
	}
	if allObjects.code != 0 {
		return nil, fmt.Errorf("inspect Git reachable refs: %s", detail(allObjects))
	}
	args := []string{"status", "--porcelain=v1", "-z", "--untracked-files=all", "--", ".", ":(exclude)" + DefaultOutputDir + "/**"}
	for _, rel := range excludes {
		if rel != "" {
			args = append(args, ":(exclude,literal)"+filepath.ToSlash(rel))
		}
	}
	res, err := g.run(repo, args...)
	if err != nil {
		return nil, err
	}
	if res.code != 0 {
		return nil, fmt.Errorf("inspect Git status: %s", detail(res))
	}
	token := make([]byte, 0, len(head)+len(refs.stdout)+len(allObjects.stdout)+len(res.stdout)+4)
	token = append(token, head...)
	token = append(token, 0)
	token = append(token, refs.stdout...)
	token = append(token, 0)
	token = append(token, allObjects.stdout...)
	token = append(token, 0)
	token = append(token, res.stdout...)
	return token, nil
}

func (g Git) IsDirty(repo string, excludes []string) (bool, error) {
	args := []string{"status", "--porcelain=v1", "-z", "--untracked-files=all", "--", ".", ":(exclude)" + DefaultOutputDir + "/**"}
	for _, rel := range excludes {
		if rel != "" {
			args = append(args, ":(exclude,literal)"+filepath.ToSlash(rel))
		}
	}
	res, err := g.run(repo, args...)
	if err != nil {
		return false, err
	}
	if res.code != 0 {
		return false, fmt.Errorf("inspect Git status: %s", detail(res))
	}
	return len(res.stdout) != 0, nil
}

func (g Git) TrackedGuard(repo, rel string) error {
	if rel == "" {
		return nil
	}
	slash := filepath.ToSlash(rel)
	if slash == ".git" || strings.HasPrefix(slash, ".git/") {
		return fmt.Errorf("output cannot be inside .git")
	}
	cmd := exec.Command("git", "--literal-pathspecs", "-C", repo, "ls-files", "--error-unmatch", "--", slash)
	cmd.Env = g.environment()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return fmt.Errorf("output %q is tracked by Git; refusing to overwrite repository content", rel)
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check tracked output: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func (g Git) ListFiles(repo string, excludes ...string) ([]string, error) {
	res, err := g.run(repo, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	if res.code != 0 {
		return nil, fmt.Errorf("list repository files: %s", detail(res))
	}
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, raw := range bytes.Split(res.stdout, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		rel := string(raw)
		if strings.Contains(rel, "\\") {
			return nil, fmt.Errorf("repository path %q contains a backslash and cannot be represented safely in a portable ZIP", rel)
		}
		slash := filepath.ToSlash(rel)
		if slash == DefaultOutputDir || strings.HasPrefix(slash, DefaultOutputDir+"/") {
			continue
		}
		excluded := false
		for _, rel := range excludes {
			if rel != "" && slash == filepath.ToSlash(rel) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}
		if _, ok := seen[rel]; ok {
			continue
		}
		full := filepath.Join(repo, filepath.FromSlash(rel))
		info, err := os.Lstat(full)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("inspect %q: %w", rel, err)
		}
		if !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
			return nil, fmt.Errorf("unsupported repository entry type: %s", rel)
		}
		seen[rel] = struct{}{}
		out = append(out, rel)
	}
	sort.Strings(out)
	return out, nil
}

type GitSnapshotMetadata struct {
	SchemaVersion int    `json:"schema_version"`
	Head          string `json:"head"`
	Branch        string `json:"branch"`
	Dirty         bool   `json:"dirty"`
}

func (g Git) SnapshotMetadata(repo string, excludes []string) (GitSnapshotMetadata, error) {
	headRes, err := g.run(repo, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return GitSnapshotMetadata{}, err
	}
	if headRes.code != 0 {
		return GitSnapshotMetadata{}, fmt.Errorf("--git requires at least one commit to create a restorable Git bundle: %s", detail(headRes))
	}
	branchRes, err := g.run(repo, "symbolic-ref", "-q", "HEAD")
	if err != nil {
		return GitSnapshotMetadata{}, err
	}
	branch := ""
	if branchRes.code == 0 {
		branch = strings.TrimSpace(string(branchRes.stdout))
	} else if branchRes.code != 1 {
		return GitSnapshotMetadata{}, fmt.Errorf("resolve Git branch: %s", detail(branchRes))
	}
	dirty, err := g.IsDirty(repo, excludes)
	if err != nil {
		return GitSnapshotMetadata{}, err
	}
	return GitSnapshotMetadata{
		SchemaVersion: 1,
		Head:          strings.TrimSpace(string(headRes.stdout)),
		Branch:        branch,
		Dirty:         dirty,
	}, nil
}

func (g Git) WriteBundle(repo string, dst io.Writer) error {
	argv := []string{"-C", repo, "bundle", "create", "-", "--all"}
	cmd := exec.Command("git", argv...)
	cmd.Env = g.environment()
	cmd.Stdout = dst
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("create Git bundle: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func (g Git) VerifyBundle(repo, bundlePath string) error {
	res, err := g.run(repo, "bundle", "verify", bundlePath)
	if err != nil {
		return err
	}
	if res.code != 0 {
		return fmt.Errorf("verify Git bundle: %s", detail(res))
	}
	return nil
}
