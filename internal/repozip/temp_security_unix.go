//go:build !windows

package repozip

import (
	"fmt"
	"os"
)

func createPrivateTempFile(dir, pattern string) (*os.File, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, fmt.Errorf("create temporary file: %w", err)
	}
	if err := securePrivateTempFile(f); err != nil {
		name := f.Name()
		_ = f.Close()
		_ = os.Remove(name)
		return nil, err
	}
	return f, nil
}

func securePrivateTempFile(f *os.File) error {
	return f.Chmod(0o600)
}
