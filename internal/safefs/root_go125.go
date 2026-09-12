//go:build go1.25

package safefs

import (
	"io/fs"
	"os"
)

type nativeBackend struct{ root *os.Root }

func openBackend(path string) (backend, error) {
	r, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	return &nativeBackend{root: r}, nil
}

func (b *nativeBackend) Close() error                       { return b.root.Close() }
func (b *nativeBackend) Open(name string) (*os.File, error) { return b.root.Open(name) }
func (b *nativeBackend) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return b.root.OpenFile(name, flag, perm)
}
func (b *nativeBackend) Lstat(name string) (os.FileInfo, error)    { return b.root.Lstat(name) }
func (b *nativeBackend) Readlink(name string) (string, error)      { return b.root.Readlink(name) }
func (b *nativeBackend) Mkdir(name string, perm os.FileMode) error { return b.root.Mkdir(name, perm) }
func (b *nativeBackend) MkdirAll(name string, perm os.FileMode) error {
	return b.root.MkdirAll(name, perm)
}
func (b *nativeBackend) Link(oldname, newname string) error   { return b.root.Link(oldname, newname) }
func (b *nativeBackend) Rename(oldname, newname string) error { return b.root.Rename(oldname, newname) }
func (b *nativeBackend) Remove(name string) error             { return b.root.Remove(name) }
func (b *nativeBackend) RemoveAll(name string) error          { return b.root.RemoveAll(name) }
func (b *nativeBackend) Symlink(oldname, newname string) error {
	return b.root.Symlink(oldname, newname)
}
func (b *nativeBackend) WalkDir(root string, fn fs.WalkDirFunc) error {
	return fs.WalkDir(b.root.FS(), root, fn)
}
