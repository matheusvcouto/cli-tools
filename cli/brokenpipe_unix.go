//go:build !windows

package cli

import (
	"errors"
	"syscall"
)

func isPlatformBrokenPipe(err error) bool {
	return errors.Is(err, syscall.EPIPE)
}
