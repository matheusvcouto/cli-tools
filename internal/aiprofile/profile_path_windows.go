//go:build windows

package aiprofile

import (
	"path/filepath"
	"strings"
)

// The generated profile directory names are ASCII; folding the full path also
// catches ordinary differences in Windows root drive/path capitalization.
func profileDirIdentity(path string) string { return strings.ToUpper(filepath.Clean(path)) }
