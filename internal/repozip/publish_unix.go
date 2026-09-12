//go:build darwin || linux

package repozip

import (
	"fmt"

	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

func ensurePublicationSupported() error { return nil }

func publishArchive(root *safefs.Root, tempName, finalName string, force bool) error {
	if force {
		if err := root.Rename(tempName, finalName); err != nil {
			return fmt.Errorf("replace destination: %w", err)
		}
		return nil
	}
	if err := root.Link(tempName, finalName); err != nil {
		return fmt.Errorf("publish destination without overwrite: %w", err)
	}
	if err := root.Remove(tempName); err != nil {
		return fmt.Errorf("destination published but temporary link cleanup failed: %w", err)
	}
	return nil
}
