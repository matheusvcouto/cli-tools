//go:build windows

package fscommit

import "github.com/matheusvcouto/cli-tools/internal/safefs"

func replaceRoot(_ *safefs.Root, _, _ string) error { return ErrUnsupported }
