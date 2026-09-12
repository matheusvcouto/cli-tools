//go:build darwin || linux

package filelock

import (
	"fmt"
	"os"
	"syscall"
)

type unixLock struct{ f *os.File }

func acquireFile(f *os.File) (Lock, error) {
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("inspect lock file: %w", err)
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, fmt.Errorf("lock file is not regular")
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("lock: %w", err)
	}
	return &unixLock{f: f}, nil
}

func (l *unixLock) Close() error {
	unlockErr := syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	closeErr := l.f.Close()
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}
