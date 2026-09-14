package cli

import (
	"fmt"
	"io"
	"syscall"
	"testing"
)

func TestIsBrokenPipe(t *testing.T) {
	for _, err := range []error{io.ErrClosedPipe, syscall.EPIPE, fmt.Errorf("write stdout: %w", syscall.EPIPE)} {
		if !IsBrokenPipe(err) {
			t.Fatalf("expected broken pipe for %v", err)
		}
	}
	if IsBrokenPipe(fmt.Errorf("other")) {
		t.Fatal("unrelated error classified as broken pipe")
	}
}
