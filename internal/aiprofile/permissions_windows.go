//go:build windows

package aiprofile

import (
	"fmt"
	"os"
	"os/user"
	"strings"
	"syscall"
	"unsafe"
)

const (
	winReadControl   = 0x00020000
	winWriteDAC      = 0x00040000
	winSEFileObject  = 1
	winDACLInfo      = 0x00000004
	winProtectedDACL = 0x80000000
	winSDDLRevision1 = 1
)

var (
	advapi32ProfilePermissions    = syscall.NewLazyDLL("advapi32.dll")
	procConvertStringSDToSDW      = advapi32ProfilePermissions.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
	procGetSecurityDescriptorDACL = advapi32ProfilePermissions.NewProc("GetSecurityDescriptorDacl")
	procSetSecurityInfo           = advapi32ProfilePermissions.NewProc("SetSecurityInfo")
)

type winSecurityDescriptor struct{}
type winACL struct{}

// secureProfileRoot gives the profile tree an explicit protected DACL instead
// of relying on os.Chmod. On Windows, Chmod only changes the read-only file
// attribute and does not provide Unix-style 0700 privacy.
//
// The DACL grants full control to the current user, Local System, and the
// built-in Administrators group. The ACEs inherit to both files and
// directories. SetSecurityInfo propagates those inherited ACEs to existing
// descendants. Explicit ACEs deliberately set on a descendant are not claimed
// to be rewritten by this root-level operation.
func secureProfileRoot(rootFile *os.File, rootPath string) error {
	expected, err := rootFile.Stat()
	if err != nil {
		return fmt.Errorf("stat profile root handle: %w", err)
	}
	if !expected.IsDir() {
		return fmt.Errorf("profile root handle is not a directory")
	}

	current, err := user.Current()
	if err != nil {
		return fmt.Errorf("resolve current Windows user: %w", err)
	}
	sid := strings.TrimSpace(current.Uid)
	if !validWindowsSIDString(sid) {
		return fmt.Errorf("current Windows user has invalid SID %q", sid)
	}

	path16, err := syscall.UTF16PtrFromString(rootPath)
	if err != nil {
		return fmt.Errorf("encode profile root path: %w", err)
	}

	// Open a second handle with WRITE_DAC. FILE_FLAG_OPEN_REPARSE_POINT avoids
	// following a last-component reparse point. Comparing it with the already
	// confined os.Root handle prevents applying a DACL to a raced replacement.
	h, err := syscall.CreateFile(
		path16,
		winReadControl|winWriteDAC,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_FLAG_BACKUP_SEMANTICS|syscall.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return fmt.Errorf("open profile root for Windows ACL update: %w", err)
	}
	aclFile := os.NewFile(uintptr(h), rootPath)
	if aclFile == nil {
		_ = syscall.CloseHandle(h)
		return fmt.Errorf("wrap profile root ACL handle")
	}
	defer aclFile.Close()

	actual, err := aclFile.Stat()
	if err != nil {
		return fmt.Errorf("stat profile root ACL handle: %w", err)
	}
	if !actual.IsDir() || !os.SameFile(expected, actual) {
		return fmt.Errorf("profile root changed identity before Windows ACL update")
	}

	sddl := "D:P" +
		"(A;OICI;FA;;;" + sid + ")" +
		"(A;OICI;FA;;;SY)" +
		"(A;OICI;FA;;;BA)"

	dacl, free, err := daclFromSDDL(sddl)
	if err != nil {
		return err
	}
	defer free()

	ret, _, _ := procSetSecurityInfo.Call(
		uintptr(h),
		winSEFileObject,
		winDACLInfo|winProtectedDACL,
		0,
		0,
		uintptr(unsafe.Pointer(dacl)),
		0,
	)
	if ret != 0 {
		return fmt.Errorf("set protected Windows profile DACL: %w", syscall.Errno(ret))
	}
	return nil
}

func validWindowsSIDString(s string) bool {
	if strings.TrimSpace(s) != s || s == "" {
		return false
	}
	_, err := syscall.StringToSid(s)
	return err == nil
}

func daclFromSDDL(sddl string) (*winACL, func(), error) {
	text, err := syscall.UTF16PtrFromString(sddl)
	if err != nil {
		return nil, nil, fmt.Errorf("encode Windows profile DACL: %w", err)
	}
	var sd *winSecurityDescriptor
	r1, _, callErr := procConvertStringSDToSDW.Call(
		uintptr(unsafe.Pointer(text)),
		winSDDLRevision1,
		uintptr(unsafe.Pointer(&sd)),
		0,
	)
	if r1 == 0 {
		return nil, nil, fmt.Errorf("build Windows profile DACL: %w", windowsCallError(callErr))
	}
	free := func() {
		_, _ = syscall.LocalFree(syscall.Handle(uintptr(unsafe.Pointer(sd))))
	}

	var present uint32
	var dacl *winACL
	var defaulted uint32
	r1, _, callErr = procGetSecurityDescriptorDACL.Call(
		uintptr(unsafe.Pointer(sd)),
		uintptr(unsafe.Pointer(&present)),
		uintptr(unsafe.Pointer(&dacl)),
		uintptr(unsafe.Pointer(&defaulted)),
	)
	if r1 == 0 {
		free()
		return nil, nil, fmt.Errorf("read Windows profile DACL: %w", windowsCallError(callErr))
	}
	if present == 0 || dacl == nil {
		free()
		return nil, nil, fmt.Errorf("generated Windows profile security descriptor has no DACL")
	}
	return dacl, free, nil
}

func windowsCallError(err error) error {
	if err == nil || err == syscall.Errno(0) {
		return syscall.EINVAL
	}
	return err
}
