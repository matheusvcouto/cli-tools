//go:build windows

package filelock

import "os"

func acquireFile(f *os.File) (Lock, error) {
	_ = f.Close()
	return nil, ErrUnsupported
}
