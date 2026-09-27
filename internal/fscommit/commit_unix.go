//go:build darwin || linux

package fscommit

import (
	"fmt"
	"os"

	"github.com/matheusvcouto/cli-tools/v2/internal/safefs"
)

func replaceRoot(root *safefs.Root, src, dst string) error {
	if err := root.Rename(src, dst); err != nil {
		return fmt.Errorf("replace %q inside safety root: %w", dst, err)
	}
	return nil
}

func replacePath(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("replace %q: %w", dst, err)
	}
	return nil
}
