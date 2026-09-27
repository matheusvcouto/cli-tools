//go:build windows

package repozip

import "strings"

func normalizeEnvironmentKey(key string) string { return strings.ToUpper(key) }
