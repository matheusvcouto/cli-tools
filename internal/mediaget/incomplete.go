package mediaget

import (
	"errors"
	"io/fs"
	"math"
	"os"

	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

// Incomplete identifies only the private area created by this download.
// It cannot be constructed by callers to delete arbitrary paths.
type Incomplete struct {
	Path                     string
	parent, name             string
	original, parentIdentity os.FileInfo
}

func (p *Incomplete) open() (*safefs.Root, error) {
	root, err := safefs.Open(p.parent)
	if err != nil {
		return nil, err
	}
	parent, err := root.Lstat(".")
	if err == nil && !os.SameFile(parent, p.parentIdentity) {
		err = errors.New("destino mudou; limpeza bloqueada")
	}
	if err == nil {
		current, e := root.Lstat(p.name)
		err = e
		if err == nil && (!current.IsDir() || !os.SameFile(current, p.original)) {
			err = errors.New("área de download mudou; limpeza bloqueada")
		}
	}
	if err != nil {
		root.Close()
		return nil, err
	}
	return root, nil
}

// Size counts regular files without following symlinks.
func (p *Incomplete) Size() (int64, error) {
	root, err := p.open()
	if err != nil {
		return 0, err
	}
	defer root.Close()
	var bytes int64
	err = root.WalkDir(p.name, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > math.MaxInt64-bytes {
			return errors.New("tamanho excede limite")
		}
		bytes += info.Size()
		return nil
	})
	return bytes, err
}

// Discard revalidates identities and removes only this download's private area.
func (p *Incomplete) Discard() error {
	root, err := p.open()
	if err != nil {
		return err
	}
	defer root.Close()
	return root.RemoveAll(p.name)
}
