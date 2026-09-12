package safefs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type backend interface {
	Close() error
	Open(name string) (*os.File, error)
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
	Lstat(name string) (os.FileInfo, error)
	Readlink(name string) (string, error)
	Mkdir(name string, perm os.FileMode) error
	MkdirAll(name string, perm os.FileMode) error
	Link(oldname, newname string) error
	Rename(oldname, newname string) error
	Remove(name string) error
	RemoveAll(name string) error
	Symlink(oldname, newname string) error
	WalkDir(root string, fn fs.WalkDirFunc) error
}

type Root struct{ b backend }

// EnsureDir creates path without following symbolic links in the unresolved
// suffix. It anchors creation at the nearest existing real directory and then
// creates the remaining components through an already-open safety root.
func EnsureDir(path string, perm os.FileMode) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	ancestor := abs
	for {
		info, statErr := os.Lstat(ancestor)
		switch {
		case statErr == nil:
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return "", fmt.Errorf("existing path component is not a real directory: %s", ancestor)
			}
			goto found
		case errors.Is(statErr, os.ErrNotExist):
			parent := filepath.Dir(ancestor)
			if parent == ancestor {
				return "", fmt.Errorf("cannot find an existing directory ancestor for %s", abs)
			}
			ancestor = parent
		default:
			return "", statErr
		}
	}

found:
	root, err := Open(ancestor)
	if err != nil {
		return "", fmt.Errorf("open existing directory ancestor safely: %w", err)
	}
	defer root.Close()

	rel, err := filepath.Rel(ancestor, abs)
	if err != nil {
		return "", err
	}
	if rel != "." {
		if err := root.MkdirAll(filepath.ToSlash(rel), perm); err != nil {
			return "", fmt.Errorf("create directory safely: %w", err)
		}
	}
	finalName := filepath.ToSlash(rel)
	if finalName == "." {
		finalName = "."
	}
	info, err := root.Lstat(finalName)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("created path is not a real directory: %s", abs)
	}
	return abs, nil
}

func Open(path string) (*Root, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	before, err := os.Lstat(abs)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		return nil, fmt.Errorf("safe root must be a real directory: %s", abs)
	}

	b, err := openBackend(abs)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*Root, error) {
		_ = b.Close()
		return nil, err
	}

	opened, err := b.Lstat(".")
	if err != nil {
		return fail(fmt.Errorf("inspect opened safe root: %w", err))
	}
	after, err := os.Lstat(abs)
	if err != nil {
		return fail(fmt.Errorf("revalidate safe root: %w", err))
	}
	if after.Mode()&os.ModeSymlink != 0 || !after.IsDir() {
		return fail(fmt.Errorf("safe root changed while opening: %s", abs))
	}
	if !os.SameFile(before, opened) || !os.SameFile(opened, after) {
		return fail(fmt.Errorf("safe root changed while opening: %s", abs))
	}

	return &Root{b: b}, nil
}

func (r *Root) Close() error                       { return r.b.Close() }
func (r *Root) Open(name string) (*os.File, error) { return r.b.Open(name) }
func (r *Root) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return r.b.OpenFile(name, flag, perm)
}
func (r *Root) Lstat(name string) (os.FileInfo, error)       { return r.b.Lstat(name) }
func (r *Root) Readlink(name string) (string, error)         { return r.b.Readlink(name) }
func (r *Root) Mkdir(name string, perm os.FileMode) error    { return r.b.Mkdir(name, perm) }
func (r *Root) MkdirAll(name string, perm os.FileMode) error { return r.b.MkdirAll(name, perm) }
func (r *Root) Link(oldname, newname string) error           { return r.b.Link(oldname, newname) }
func (r *Root) Rename(oldname, newname string) error         { return r.b.Rename(oldname, newname) }
func (r *Root) Remove(name string) error                     { return r.b.Remove(name) }
func (r *Root) RemoveAll(name string) error                  { return r.b.RemoveAll(name) }
func (r *Root) Symlink(oldname, newname string) error        { return r.b.Symlink(oldname, newname) }
func (r *Root) WalkDir(root string, fn fs.WalkDirFunc) error { return r.b.WalkDir(root, fn) }
