//go:build !(aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris)

package cli

import (
	"context"
	"os"
	"os/signal"
)

// SignalContext returns a context cancelled by the platform's interrupt
// signal. Platforms without a portable SIGTERM equivalent use os.Interrupt.
func SignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return signal.NotifyContext(parent, os.Interrupt)
}
