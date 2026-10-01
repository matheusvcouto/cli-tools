//go:build darwin || linux

package ytdlp

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"
)

func platformAvailable() error { return nil }
func installHint(name string) string {
	if name == "ffprobe" {
		name = "ffmpeg"
	}
	if runtime.GOOS == "darwin" {
		return "brew install " + name
	}
	return "sudo apt install " + name + " (Debian/Ubuntu)"
}
func command(ctx context.Context, path string, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = processEnv()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = 2 * time.Second
	return cmd
}
