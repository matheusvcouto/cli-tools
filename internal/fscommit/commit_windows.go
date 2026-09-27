//go:build windows

package fscommit

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/matheusvcouto/cli-tools/v2/internal/safefs"
)

const (
	movefileReplaceExisting = 0x00000001
	movefileWriteThrough    = 0x00000008
)

var (
	fsKernel32      = syscall.NewLazyDLL("kernel32.dll")
	procMoveFileExW = fsKernel32.NewProc("MoveFileExW")
)

func replaceRoot(root *safefs.Root, src, dst string) error {
	// Official builds use Go 1.27.1. On Windows, os.Root.Rename uses a
	// handle-relative native rename under the already-open root and requests
	// replacement of an existing destination. Keep the operation confined and
	// never pre-delete dst: failure must remain fail-closed.
	if err := root.Rename(src, dst); err != nil {
		return fmt.Errorf("replace %q inside safety root: %w", dst, err)
	}
	return nil
}

func replacePath(src, dst string) error {
	from, err := syscall.UTF16PtrFromString(src)
	if err != nil {
		return fmt.Errorf("encode replacement source: %w", err)
	}
	to, err := syscall.UTF16PtrFromString(dst)
	if err != nil {
		return fmt.Errorf("encode replacement destination: %w", err)
	}
	r1, _, callErr := procMoveFileExW.Call(
		uintptr(unsafe.Pointer(from)),
		uintptr(unsafe.Pointer(to)),
		uintptr(movefileReplaceExisting|movefileWriteThrough),
	)
	if r1 == 0 {
		if callErr != syscall.Errno(0) {
			return fmt.Errorf("replace %q: %w", dst, callErr)
		}
		return fmt.Errorf("replace %q: %w", dst, syscall.EINVAL)
	}
	return nil
}
