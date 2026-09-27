//go:build darwin || linux

package aiprofile

import "path/filepath"

func profileDirIdentity(path string) string { return filepath.Clean(path) }
