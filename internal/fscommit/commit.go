package fscommit

import (
	"errors"

	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

var ErrUnsupported = errors.New("safe file commit is not supported on this platform")

// ReplaceRoot atomically replaces dst with src when the platform can provide
// that guarantee. src and dst are relative to the same already-open safety
// root and must reside on the same filesystem.
func ReplaceRoot(root *safefs.Root, src, dst string) error { return replaceRoot(root, src, dst) }
