package fscommit

import "github.com/matheusvcouto/cli-tools/internal/safefs"

// ReplaceRoot atomically replaces dst with src when the platform can provide
// that guarantee. src and dst are relative to the same already-open safety
// root and must reside on the same filesystem.
func ReplaceRoot(root *safefs.Root, src, dst string) error { return replaceRoot(root, src, dst) }

// ReplacePath replaces dst with src as one platform-native move/replace
// operation. Callers are responsible for path containment; src and dst must be
// on the same filesystem and dst must never be pre-deleted.
func ReplacePath(src, dst string) error { return replacePath(src, dst) }
