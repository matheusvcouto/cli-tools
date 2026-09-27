//go:build windows

package repozip

import (
	"fmt"

	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

func ensurePublicationSupported() error { return nil }

func publishArchive(root *safefs.Root, tempName, finalName string, force bool) error {
	if force {
		// Go 1.27's Windows os.Root backend performs a handle-relative native
		// replace. Never delete finalName first: replacement is one operation or
		// an error.
		if err := root.Rename(tempName, finalName); err != nil {
			return fmt.Errorf("replace destination: %w", err)
		}
		return nil
	}

	// Link creation is the no-clobber primitive: Windows fails the operation
	// when finalName already exists. A filesystem without hard-link support is
	// intentionally rejected instead of degrading to a TOCTOU check+rename.
	if err := root.Link(tempName, finalName); err != nil {
		return fmt.Errorf("publish destination without overwrite: %w", err)
	}
	if err := root.Remove(tempName); err != nil {
		return fmt.Errorf("destination published but temporary link cleanup failed: %w", err)
	}
	return nil
}
