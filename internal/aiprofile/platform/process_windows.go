//go:build windows

package platform

import (
	"context"
	"fmt"

	"github.com/matheusvcouto/cli-tools/internal/aiprofile"
)

type Runner struct{}

func (Runner) Replace(context.Context, string, []string, []string, aiprofile.ProcessIO) error {
	return fmt.Errorf("run/acp are not implemented on Windows yet")
}
