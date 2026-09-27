//go:build darwin || linux

package repozip

func normalizeEnvironmentKey(key string) string { return key }
