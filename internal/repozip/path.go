package repozip

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

func relativeIfInside(path, root string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return ""
	}
	if rel == "." {
		return rel
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return ""
	}
	return rel
}

func ensureDefaultOutputParent(repo string) (string, error) {
	root, err := safefs.Open(repo)
	if err != nil {
		return "", fmt.Errorf("open repository safety root: %w", err)
	}
	defer root.Close()

	current := ""
	for _, part := range []string{".tmp", "repo-zip"} {
		if current == "" {
			current = part
		} else {
			current = filepath.ToSlash(filepath.Join(current, part))
		}
		info, err := root.Lstat(current)
		switch {
		case errors.Is(err, os.ErrNotExist):
			if err := root.Mkdir(current, 0o755); err != nil {
				return "", fmt.Errorf("create %s: %w", current, err)
			}
			info, err = root.Lstat(current)
			if err != nil {
				return "", err
			}
		case err != nil:
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", fmt.Errorf("refuse unsafe default output component: %s is not a real directory", current)
		}
	}
	return filepath.Join(repo, filepath.FromSlash(current)), nil
}

func ensureExplicitParent(path, repo string) (string, error) {
	parent, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return "", err
	}

	if rel := relativeIfInside(parent, repo); rel != "" {
		root, err := safefs.Open(repo)
		if err != nil {
			return "", fmt.Errorf("open repository safety root: %w", err)
		}
		defer root.Close()
		if err := root.MkdirAll(filepath.ToSlash(rel), 0o755); err != nil {
			return "", fmt.Errorf("output parent escapes repository or is unsafe: %w", err)
		}
		// Defense in depth for bootstrap builds using the legacy safefs backend.
		realRepo, err := filepath.EvalSymlinks(repo)
		if err != nil {
			return "", err
		}
		realParent, err := filepath.EvalSymlinks(parent)
		if err != nil {
			return "", err
		}
		if relativeIfInside(realParent, realRepo) == "" && realParent != realRepo {
			return "", fmt.Errorf("output parent escapes repository through a symbolic link")
		}
		return parent, nil
	}

	// An explicitly requested destination outside the repository is allowed.
	// It is still pinned with safefs.Open by Service before temp creation and
	// publication, so later operations do not repeatedly resolve this path.
	created, err := safefs.EnsureDir(parent, 0o755)
	if err != nil {
		return "", err
	}
	return created, nil
}

func validateDestination(path string, force bool) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("destination exists and is not a regular file: %s", path)
	}
	if !force {
		return fmt.Errorf("destination already exists: %s (use --force to replace)", path)
	}
	return nil
}
