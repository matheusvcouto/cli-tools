//go:build windows

package cli

import (
	"errors"
	"syscall"
)

const windowsErrorNoData syscall.Errno = 232 // ERROR_NO_DATA (WinError.h)

func isPlatformBrokenPipe(err error) bool {
	// Windows pipe writes can surface either ERROR_BROKEN_PIPE or
	// ERROR_NO_DATA depending on the pipe path. Keep syscall.EPIPE too because
	// Go defines it on Windows and higher-level wrappers may normalize to it.
	return errors.Is(err, syscall.ERROR_BROKEN_PIPE) ||
		errors.Is(err, windowsErrorNoData) ||
		errors.Is(err, syscall.EPIPE)
}
