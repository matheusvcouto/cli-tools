//go:build darwin || linux

package platform

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"

	"github.com/matheusvcouto/cli-tools/internal/aiprofile"
)

type Runner struct{}

func (Runner) Replace(_ context.Context, binary string, args []string, env []string, _ aiprofile.ProcessIO) error {
	path, err := exec.LookPath(binary)
	if err != nil {
		return fmt.Errorf("find %s in PATH: %w", binary, err)
	}
	argv := append([]string{binary}, args...)
	if err := syscall.Exec(path, argv, env); err != nil {
		return fmt.Errorf("exec %s: %w", binary, err)
	}
	return nil
}
