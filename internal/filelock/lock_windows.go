//go:build windows

package filelock

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	lockfileFailImmediately = 0x00000001
	lockfileExclusiveLock   = 0x00000002
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = kernel32.NewProc("LockFileEx")
	procUnlockFileEx = kernel32.NewProc("UnlockFileEx")
)

type windowsLock struct{ f *os.File }

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

	var overlapped syscall.Overlapped
	if err := callLockFileEx(syscall.Handle(f.Fd()), lockfileExclusiveLock, &overlapped); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("lock: %w", err)
	}
	runtime.KeepAlive(f)
	return &windowsLock{f: f}, nil
}

func (l *windowsLock) Close() error {
	var overlapped syscall.Overlapped
	unlockErr := callUnlockFileEx(syscall.Handle(l.f.Fd()), &overlapped)
	runtime.KeepAlive(l.f)
	closeErr := l.f.Close()
	if unlockErr != nil {
		return fmt.Errorf("unlock: %w", unlockErr)
	}
	return closeErr
}

func callLockFileEx(handle syscall.Handle, flags uint32, overlapped *syscall.Overlapped) error {
	r1, _, callErr := procLockFileEx.Call(
		uintptr(handle),
		uintptr(flags),
		0,
		uintptr(^uint32(0)),
		uintptr(^uint32(0)),
		uintptr(unsafe.Pointer(overlapped)),
	)
	if r1 != 0 {
		return nil
	}
	if callErr != syscall.Errno(0) {
		return callErr
	}
	return syscall.EINVAL
}

func callUnlockFileEx(handle syscall.Handle, overlapped *syscall.Overlapped) error {
	r1, _, callErr := procUnlockFileEx.Call(
		uintptr(handle),
		0,
		uintptr(^uint32(0)),
		uintptr(^uint32(0)),
		uintptr(unsafe.Pointer(overlapped)),
	)
	if r1 != 0 {
		return nil
	}
	if callErr != syscall.Errno(0) {
		return callErr
	}
	return syscall.EINVAL
}
