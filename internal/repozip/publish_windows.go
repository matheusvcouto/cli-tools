//go:build windows

package repozip

import (
	"fmt"

	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

func ensurePublicationSupported() error {
	return fmt.Errorf("repo-zip publication is not supported on Windows yet: safe replace/no-clobber semantics are not implemented")
}

func publishArchive(_ *safefs.Root, _, _ string, _ bool) error {
	return fmt.Errorf("repo-zip publication is not supported on Windows yet: safe replace/no-clobber semantics are not implemented")
}
