//go:build windows

package filelock

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestWindowsFileLockTarget(t *testing.T) {
	if os.Getenv("GO_WINDOWS_FILELOCK_TARGET") != "1" {
		return
	}
	path := os.Getenv("GO_WINDOWS_FILELOCK_PATH")
	marker := os.Getenv("GO_WINDOWS_FILELOCK_MARKER")
	ready := os.Getenv("GO_WINDOWS_FILELOCK_READY")
	if ready != "" {
		if err := os.WriteFile(ready, []byte("ready"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := AcquireFile(f)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := os.WriteFile(marker, []byte("acquired"), 0o600); err != nil {
		t.Fatal(err)
	}
}

const windowsErrorLockViolation syscall.Errno = 33 // ERROR_LOCK_VIOLATION (WinError.h)

func TestWindowsFileLockContentionTarget(t *testing.T) {
	if os.Getenv("GO_WINDOWS_FILELOCK_CONTENTION_TARGET") != "1" {
		return
	}
	path := os.Getenv("GO_WINDOWS_FILELOCK_PATH")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var overlapped syscall.Overlapped
	err = callLockFileEx(syscall.Handle(f.Fd()), lockfileExclusiveLock|lockfileFailImmediately, &overlapped)
	if err == nil {
		_ = callUnlockFileEx(syscall.Handle(f.Fd()), &overlapped)
		t.Fatal("competing process unexpectedly acquired an already-held exclusive lock")
	}
	if !errors.Is(err, windowsErrorLockViolation) {
		t.Fatalf("competing process error=%v; want ERROR_LOCK_VIOLATION", err)
	}
}

func TestAcquireFileBlocksCompetingProcessUntilRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.lock")
	marker := filepath.Join(t.TempDir(), "second-acquired")
	firstFile, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	first, err := AcquireFile(firstFile)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}

	exe, err := os.Executable()
	if err != nil {
		_ = first.Close()
		t.Fatal(err)
	}

	// First prove cross-process exclusion without timing: a second process using
	// the same Win32 range with FAIL_IMMEDIATELY must observe LOCK_VIOLATION.
	probe := exec.Command(exe, "-test.run=^TestWindowsFileLockContentionTarget$")
	probe.Env = append(os.Environ(),
		"GO_WINDOWS_FILELOCK_CONTENTION_TARGET=1",
		"GO_WINDOWS_FILELOCK_PATH="+path,
	)
	if output, err := probe.CombinedOutput(); err != nil {
		_ = first.Close()
		t.Fatalf("cross-process contention probe failed: %v\n%s", err, output)
	}

	ready := filepath.Join(t.TempDir(), "second-ready")
	cmd := exec.Command(exe, "-test.run=^TestWindowsFileLockTarget$")
	cmd.Env = append(os.Environ(),
		"GO_WINDOWS_FILELOCK_TARGET=1",
		"GO_WINDOWS_FILELOCK_PATH="+path,
		"GO_WINDOWS_FILELOCK_MARKER="+marker,
		"GO_WINDOWS_FILELOCK_READY="+ready,
	)
	if err := cmd.Start(); err != nil {
		_ = first.Close()
		t.Fatalf("start competing process: %v", err)
	}

	// Wait until the child has reached the lock attempt, rather than assuming
	// process startup completed within a fixed sleep (important on ARM runners).
	readyDeadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		} else if !os.IsNotExist(err) {
			_ = first.Close()
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			t.Fatalf("inspect contention readiness marker: %v", err)
		}
		if time.Now().After(readyDeadline) {
			_ = first.Close()
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			t.Fatal("competing process did not reach the lock attempt")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// LockFileEx without LOCKFILE_FAIL_IMMEDIATELY is blocking. Combined with
	// the non-blocking contention probe above, this checks both exclusion and the
	// production blocking behavior.
	time.Sleep(100 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		_ = first.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal("competing process acquired lock before release")
	} else if !os.IsNotExist(err) {
		_ = first.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("inspect competing marker: %v", err)
	}

	if err := first.Close(); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("release first lock: %v", err)
	}

	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	select {
	case err := <-wait:
		if err != nil {
			t.Fatalf("competing process after release: %v", err)
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("competing process did not acquire after first lock was released")
	}
	if raw, err := os.ReadFile(marker); err != nil || string(raw) != "acquired" {
		t.Fatalf("competing process did not confirm lock acquisition: content=%q err=%v", raw, err)
	}
}

func TestAcquireFileRejectsNonRegularHandle(t *testing.T) {
	dir, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireFile(dir); err == nil {
		t.Fatal("expected directory lock handle to be rejected")
	}
}
