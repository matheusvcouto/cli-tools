//go:build windows

package cli

import (
	"fmt"
	"syscall"
	"testing"
)

func TestIsBrokenPipeWindowsNativeErrors(t *testing.T) {
	for _, err := range []error{
		syscall.ERROR_BROKEN_PIPE,
		windowsErrorNoData,
		fmt.Errorf("write stdout: %w", windowsErrorNoData),
	} {
		if !IsBrokenPipe(err) {
			t.Fatalf("expected Windows broken pipe for %v", err)
		}
	}
}
