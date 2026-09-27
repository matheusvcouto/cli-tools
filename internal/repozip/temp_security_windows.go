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
)

var (
	repozipAdvapi32                  = syscall.NewLazyDLL("advapi32.dll")
	repozipConvertStringSDToSDW      = repozipAdvapi32.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
	repozipGetSecurityDescriptorDACL = repozipAdvapi32.NewProc("GetSecurityDescriptorDacl")
	repozipSetSecurityInfo           = repozipAdvapi32.NewProc("SetSecurityInfo")
)

type repozipSecurityDescriptor struct{}
type repozipACL struct{}

// securePrivateTempFile applies a protected DACL directly to the already-open
// temporary file handle. os.Chmod does not provide Unix-style 0600 privacy on
// Windows, and applying the ACL by handle avoids a pathname race.
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
