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

func (Runner) Replace(ctx context.Context, binary string, args []string, env []string, _ aiprofile.ProcessIO) error {
	// syscall.Exec irreversibly replaces this process. If cancellation is
	// already requested, avoid launching an agent after its caller has stopped
	// waiting. A context cannot cancel an agent after a successful exec;
	// termination then belongs to the caller's normal signal handling.
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := exec.LookPath(binary)
	if err != nil {
		return fmt.Errorf("find %s in PATH: %w", binary, err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	argv := append([]string{binary}, args...)
	if err := syscall.Exec(path, argv, env); err != nil {
		return fmt.Errorf("exec %s: %w", binary, err)
	}
	return nil
}
