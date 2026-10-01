//go:build !darwin && !linux

package ytdlp

import (
	"context"
	"errors"
	"os/exec"
)

func platformAvailable() error {
	return errors.New("download de mídia ainda não está implementado nesta plataforma; use macOS ou Linux")
}
func installHint(name string) string { return "instale " + name + " no PATH" }
func command(ctx context.Context, path string, args []string) *exec.Cmd {
	return exec.CommandContext(ctx, path, args...)
}
