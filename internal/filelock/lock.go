package filelock

import "os"

type Lock interface {
	Close() error
}

// AcquireFile locks an already-open regular file. On success the returned
// Lock owns f and closes it from Lock.Close. On failure the platform
// implementation also closes f.
func AcquireFile(f *os.File) (Lock, error) { return acquireFile(f) }
