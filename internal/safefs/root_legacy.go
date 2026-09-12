//go:build !go1.25

package safefs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type legacyBackend struct{ base string }

func openBackend(path string) (backend, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("safe root must be a real directory: %s", abs)
	}
	return &legacyBackend{base: abs}, nil
}

func (b *legacyBackend) Close() error { return nil }

func (b *legacyBackend) clean(name string) (string, []string, error) {
	if filepath.IsAbs(name) {
		return "", nil, fmt.Errorf("absolute path is not allowed inside safe root")
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", nil, fmt.Errorf("path escapes safe root: %s", name)
	}
	if clean == "." {
		return clean, nil, nil
	}
	return clean, strings.Split(clean, string(filepath.Separator)), nil
}

// resolve validates every existing parent as a real directory. When
// allowFinalSymlink is false, an existing final symlink is rejected too.
// This fallback is intentionally stricter than os.Root: it refuses all
// traversed symlinks rather than trying to prove an in-root target.
func (b *legacyBackend) resolve(name string, allowFinalSymlink bool) (string, error) {
	clean, parts, err := b.clean(name)
	if err != nil {
		return "", err
	}
	if clean == "." {
		return b.base, nil
	}
	current := b.base
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, lerr := os.Lstat(current)
		if errors.Is(lerr, os.ErrNotExist) {
			// Missing final paths are valid for create/rename destinations. A
			// missing parent will be handled by the operation itself.
			continue
		}
		if lerr != nil {
			return "", lerr
		}
		last := i == len(parts)-1
		if info.Mode()&os.ModeSymlink != 0 {
			if last && allowFinalSymlink {
				continue
			}
			return "", fmt.Errorf("symbolic link traversal is not allowed by legacy safe root: %s", name)
		}
		if !last && !info.IsDir() {
			return "", fmt.Errorf("path parent is not a directory: %s", current)
		}
	}
	return filepath.Join(b.base, clean), nil
}

func (b *legacyBackend) Open(name string) (*os.File, error) {
	p, err := b.resolve(name, false)
	if err != nil {
		return nil, err
	}
	return os.Open(p)
}
func (b *legacyBackend) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	p, err := b.resolve(name, false)
	if err != nil {
		return nil, err
	}
	return os.OpenFile(p, flag, perm)
}
func (b *legacyBackend) Lstat(name string) (os.FileInfo, error) {
	p, err := b.resolve(name, true)
	if err != nil {
		return nil, err
	}
	return os.Lstat(p)
}
func (b *legacyBackend) Readlink(name string) (string, error) {
	p, err := b.resolve(name, true)
	if err != nil {
		return "", err
	}
	return os.Readlink(p)
}
func (b *legacyBackend) Mkdir(name string, perm os.FileMode) error {
	clean, parts, err := b.clean(name)
	if err != nil {
		return err
	}
	if clean == "." || len(parts) == 0 {
		return os.ErrExist
	}
	parent := filepath.Dir(clean)
	if _, err := b.resolve(parent, false); err != nil {
		return err
	}
	return os.Mkdir(filepath.Join(b.base, clean), perm)
}
func (b *legacyBackend) MkdirAll(name string, perm os.FileMode) error {
	clean, parts, err := b.clean(name)
	if err != nil {
		return err
	}
	if clean == "." {
		return nil
	}
	current := b.base
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, lerr := os.Lstat(current)
		switch {
		case errors.Is(lerr, os.ErrNotExist):
			if err := os.Mkdir(current, perm); err != nil && !errors.Is(err, os.ErrExist) {
				return err
			}
			info, lerr = os.Lstat(current)
			if lerr != nil {
				return lerr
			}
		case lerr != nil:
			return lerr
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("unsafe directory component: %s", current)
		}
	}
	return nil
}
func (b *legacyBackend) Link(oldname, newname string) error {
	o, err := b.resolve(oldname, false)
	if err != nil {
		return err
	}
	n, err := b.resolve(newname, true)
	if err != nil {
		return err
	}
	return os.Link(o, n)
}
func (b *legacyBackend) Rename(oldname, newname string) error {
	o, err := b.resolve(oldname, true)
	if err != nil {
		return err
	}
	n, err := b.resolve(newname, true)
	if err != nil {
		return err
	}
	return os.Rename(o, n)
}
func (b *legacyBackend) Remove(name string) error {
	p, err := b.resolve(name, true)
	if err != nil {
		return err
	}
	return os.Remove(p)
}
func (b *legacyBackend) RemoveAll(name string) error {
	p, err := b.resolve(name, true)
	if err != nil {
		return err
	}
	return os.RemoveAll(p)
}
func (b *legacyBackend) Symlink(oldname, newname string) error {
	p, err := b.resolve(newname, true)
	if err != nil {
		return err
	}
	return os.Symlink(oldname, p)
}
func (b *legacyBackend) WalkDir(root string, fn fs.WalkDirFunc) error {
	start, err := b.resolve(root, false)
	if err != nil {
		return err
	}
	return filepath.WalkDir(start, func(path string, d fs.DirEntry, walkErr error) error {
		rel, relErr := filepath.Rel(b.base, path)
		if relErr != nil {
			return relErr
		}
		return fn(filepath.ToSlash(rel), d, walkErr)
	})
}
