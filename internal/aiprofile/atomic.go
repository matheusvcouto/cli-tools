package aiprofile

import (
	"errors"
	"fmt"
	"os"

	"github.com/matheusvcouto/cli-tools/internal/fscommit"
	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

// writeAtomicRoot replaces dst with content while keeping every operation
// relative to the same already-open safety root. Existing destinations must be
// regular files; symlinks and special files are always refused.
func writeAtomicRoot(root *safefs.Root, dst string, content []byte, perm os.FileMode, tempPrefix string) error {
	if info, err := root.Lstat(dst); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("refuse to replace non-regular or symbolic-link file: %s", dst)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	var f *os.File
	var tmp string
	for i := 0; i < 8; i++ {
		suffix, err := randomHex(8)
		if err != nil {
			return err
		}
		tmp = tempPrefix + suffix + ".tmp"
		f, err = root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_RDWR, perm)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
	}
	if f == nil {
		return fmt.Errorf("could not allocate temporary file for %s", dst)
	}

	committed := false
	defer func() {
		_ = f.Close()
		if !committed {
			_ = root.Remove(tmp)
		}
	}()
	if err := f.Chmod(perm); err != nil {
		return err
	}
	if _, err := f.Write(content); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := fscommit.ReplaceRoot(root, tmp, dst); err != nil {
		return err
	}
	committed = true

	if d, err := root.Open("."); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
