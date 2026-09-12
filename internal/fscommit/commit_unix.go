//go:build darwin || linux

package fscommit

import (
	"fmt"

	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

func replaceRoot(root *safefs.Root, src, dst string) error {
	if err := root.Rename(src, dst); err != nil {
		return fmt.Errorf("replace %q inside safety root: %w", dst, err)
	}
	return nil
}
