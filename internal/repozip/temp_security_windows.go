//go:build windows

package repozip

import (
	"fmt"
	"os"
	"os/user"
	"strings"
	"syscall"
	"unsafe"
)

const (
	repozipSEFileObject  = 1
	repozipDACLInfo      = 0x00000004
	repozipProtectedDACL = 0x80000000
	repozipSDDLRevision1 = 1
	repozipReadControl   = 0x00020000
	repozipWriteDAC      = 0x00040000
)

var (
	repozipAdvapi32                  = syscall.NewLazyDLL("advapi32.dll")
	repozipConvertStringSDToSDW      = repozipAdvapi32.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
	repozipGetSecurityDescriptorDACL = repozipAdvapi32.NewProc("GetSecurityDescriptorDacl")
	repozipSetSecurityInfo           = repozipAdvapi32.NewProc("SetSecurityInfo")
)

type repozipSecurityDescriptor struct{}
type repozipACL struct{}

// createPrivateTempFile returns a regular file opened for read/write whose
// handle also has WRITE_DAC. os.CreateTemp does not request that right, and
// SetSecurityInfo then fails with ACCESS_DENIED. The unique name comes from
// CreateTemp; the handle used afterward is a new open of that same file.
func createPrivateTempFile(dir, pattern string) (*os.File, error) {
	provisional, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, fmt.Errorf("create temporary file: %w", err)
	}
	name := provisional.Name()
	before, statErr := provisional.Stat()
	closeErr := provisional.Close()
	if statErr != nil || closeErr != nil {
		_ = os.Remove(name)
		if statErr != nil {
			return nil, fmt.Errorf("stat temporary file: %w", statErr)
		}
		return nil, fmt.Errorf("close temporary file: %w", closeErr)
	}
	if !before.Mode().IsRegular() {
		_ = os.Remove(name)
		return nil, fmt.Errorf("temporary file is not regular")
	}

	path16, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		_ = os.Remove(name)
		return nil, fmt.Errorf("encode temporary file path: %w", err)
	}
	h, err := syscall.CreateFile(
		path16,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE|repozipReadControl|repozipWriteDAC,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL|syscall.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		_ = os.Remove(name)
		return nil, fmt.Errorf("reopen temporary file for Windows ACL: %w", err)
	}
	f := os.NewFile(uintptr(h), name)
	if f == nil {
		_ = syscall.CloseHandle(h)
		_ = os.Remove(name)
		return nil, fmt.Errorf("wrap temporary file")
	}
	after, err := f.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		_ = f.Close()
		_ = os.Remove(name)
		if err != nil {
			return nil, fmt.Errorf("stat reopened temporary file: %w", err)
		}
		return nil, fmt.Errorf("temporary file changed identity before Windows ACL update")
	}
	if err := securePrivateTempFile(f); err != nil {
		_ = f.Close()
		_ = os.Remove(name)
		return nil, err
	}
	return f, nil
}

// securePrivateTempFile applies a protected DACL directly to the already-open
// temporary file handle. The handle must include WRITE_DAC. os.Chmod does not
// provide Unix-style 0600 privacy on Windows, and applying the ACL by handle
// avoids a pathname race.
func securePrivateTempFile(f *os.File) error {
	if f == nil {
		return fmt.Errorf("secure temporary file: nil handle")
	}
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat temporary file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("temporary file is not regular")
	}

	current, err := user.Current()
	if err != nil {
		return fmt.Errorf("resolve current Windows user: %w", err)
	}
	sid := strings.TrimSpace(current.Uid)
	if sid == "" || sid != current.Uid {
		return fmt.Errorf("current Windows user has invalid SID %q", current.Uid)
	}
	if _, err := syscall.StringToSid(sid); err != nil {
		return fmt.Errorf("current Windows user has invalid SID %q: %w", sid, err)
	}

	sddl := "D:P" +
		"(A;;FA;;;" + sid + ")" +
		"(A;;FA;;;SY)" +
		"(A;;FA;;;BA)"
	dacl, free, err := repozipDACLFromSDDL(sddl)
	if err != nil {
		return err
	}
	defer free()

	ret, _, _ := repozipSetSecurityInfo.Call(
		f.Fd(),
		repozipSEFileObject,
		repozipDACLInfo|repozipProtectedDACL,
		0,
		0,
		uintptr(unsafe.Pointer(dacl)),
		0,
	)
	if ret != 0 {
		return fmt.Errorf("set protected Windows temporary-file DACL: %w", syscall.Errno(ret))
	}
	return nil
}

func repozipDACLFromSDDL(sddl string) (*repozipACL, func(), error) {
	text, err := syscall.UTF16PtrFromString(sddl)
	if err != nil {
		return nil, nil, fmt.Errorf("encode Windows temporary-file DACL: %w", err)
	}
	var sd *repozipSecurityDescriptor
	r1, _, callErr := repozipConvertStringSDToSDW.Call(
		uintptr(unsafe.Pointer(text)),
		repozipSDDLRevision1,
		uintptr(unsafe.Pointer(&sd)),
		0,
	)
	if r1 == 0 {
		return nil, nil, fmt.Errorf("build Windows temporary-file DACL: %w", repozipWindowsCallError(callErr))
	}
	free := func() {
		_, _ = syscall.LocalFree(syscall.Handle(uintptr(unsafe.Pointer(sd))))
	}

	var present uint32
	var dacl *repozipACL
	var defaulted uint32
	r1, _, callErr = repozipGetSecurityDescriptorDACL.Call(
		uintptr(unsafe.Pointer(sd)),
		uintptr(unsafe.Pointer(&present)),
		uintptr(unsafe.Pointer(&dacl)),
		uintptr(unsafe.Pointer(&defaulted)),
	)
	if r1 == 0 {
		free()
		return nil, nil, fmt.Errorf("read Windows temporary-file DACL: %w", repozipWindowsCallError(callErr))
	}
	if present == 0 || dacl == nil {
		free()
		return nil, nil, fmt.Errorf("generated Windows temporary-file security descriptor has no DACL")
	}
	return dacl, free, nil
}

func repozipWindowsCallError(err error) error {
	if err == nil || err == syscall.Errno(0) {
		return syscall.EINVAL
	}
	return err
}
