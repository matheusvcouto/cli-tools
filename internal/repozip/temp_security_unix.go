//go:build !windows

package repozip

import "os"

func securePrivateTempFile(f *os.File) error {
	return f.Chmod(0o600)
}
