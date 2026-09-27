//go:build !windows

package aiprofile

import "os"

func secureProfileRoot(f *os.File, _ string) error {
	return f.Chmod(0o700)
}
