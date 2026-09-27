//go:build windows

package repozip

import (
	"os/user"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

const repozipSEDACLProtected = 0x1000

var (
	repozipGetNamedSecurityInfoWTest = repozipAdvapi32.NewProc("GetNamedSecurityInfoW")
	repozipGetSDControlTest          = repozipAdvapi32.NewProc("GetSecurityDescriptorControl")
	repozipConvertSDToStringTest     = repozipAdvapi32.NewProc("ConvertSecurityDescriptorToStringSecurityDescriptorW")
)

func TestSecurePrivateTempFileAppliesProtectedWindowsACL(t *testing.T) {
	f, err := createPrivateTempFile(t.TempDir(), "bundle-*.tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	sddl, control := readRepozipWindowsDACLForTest(t, f.Name())
	if control&repozipSEDACLProtected == 0 {
		t.Fatalf("temporary-file DACL is not protected: control=%#x sddl=%s", control, sddl)
	}
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	if !sddlGrantsSID(sddl, current.Uid) {
		t.Fatalf("temporary-file DACL does not contain current user SID %q: %s", current.Uid, sddl)
	}
	if strings.Contains(sddl, ";;;WD)") || strings.Contains(sddl, ";;;S-1-1-0)") {
		t.Fatalf("temporary-file DACL grants Everyone access: %s", sddl)
	}
}

func readRepozipWindowsDACLForTest(t *testing.T, path string) (string, uint16) {
	t.Helper()
	path16, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	var sd *repozipSecurityDescriptor
	ret, _, _ := repozipGetNamedSecurityInfoWTest.Call(
		uintptr(unsafe.Pointer(path16)),
		repozipSEFileObject,
		repozipDACLInfo,
		0, 0, 0, 0,
		uintptr(unsafe.Pointer(&sd)),
	)
	if ret != 0 {
		t.Fatalf("GetNamedSecurityInfoW(%q): %v", path, syscall.Errno(ret))
	}
	defer func() { _, _ = syscall.LocalFree(syscall.Handle(uintptr(unsafe.Pointer(sd)))) }()

	var control uint16
	var revision uint32
	r1, _, callErr := repozipGetSDControlTest.Call(
		uintptr(unsafe.Pointer(sd)),
		uintptr(unsafe.Pointer(&control)),
		uintptr(unsafe.Pointer(&revision)),
	)
	if r1 == 0 {
		t.Fatalf("GetSecurityDescriptorControl(%q): %v", path, repozipWindowsCallError(callErr))
	}

	var text *uint16
	var textLen uint32
	r1, _, callErr = repozipConvertSDToStringTest.Call(
		uintptr(unsafe.Pointer(sd)),
		repozipSDDLRevision1,
		repozipDACLInfo,
		uintptr(unsafe.Pointer(&text)),
		uintptr(unsafe.Pointer(&textLen)),
	)
	if r1 == 0 {
		t.Fatalf("ConvertSecurityDescriptorToStringSecurityDescriptorW(%q): %v", path, repozipWindowsCallError(callErr))
	}
	defer func() { _, _ = syscall.LocalFree(syscall.Handle(uintptr(unsafe.Pointer(text)))) }()
	if text == nil || textLen == 0 {
		t.Fatalf("empty SDDL returned for %q", path)
	}
	return syscall.UTF16ToString(unsafe.Slice(text, textLen)), control
}

// Windows rewrites some SIDs to SDDL aliases. The built-in Administrator
// account S-1-5-21-…-500 is stored as LA, so a literal SID search misses it.
func sddlGrantsSID(sddl, sid string) bool {
	if strings.Contains(sddl, sid) {
		return true
	}
	trustee, err := canonicalSDDLTrustee(sid)
	return err == nil && trustee != "" && strings.Contains(sddl, ";;;"+trustee+")")
}

func canonicalSDDLTrustee(sid string) (string, error) {
	raw := "D:(A;;FA;;;" + sid + ")"
	text, err := syscall.UTF16PtrFromString(raw)
	if err != nil {
		return "", err
	}
	var sd *repozipSecurityDescriptor
	r1, _, callErr := repozipConvertStringSDToSDW.Call(
		uintptr(unsafe.Pointer(text)),
		repozipSDDLRevision1,
		uintptr(unsafe.Pointer(&sd)),
		0,
	)
	if r1 == 0 {
		return "", repozipWindowsCallError(callErr)
	}
	defer func() { _, _ = syscall.LocalFree(syscall.Handle(uintptr(unsafe.Pointer(sd)))) }()

	var out *uint16
	var outLen uint32
	r1, _, callErr = repozipConvertSDToStringTest.Call(
		uintptr(unsafe.Pointer(sd)),
		repozipSDDLRevision1,
		repozipDACLInfo,
		uintptr(unsafe.Pointer(&out)),
		uintptr(unsafe.Pointer(&outLen)),
	)
	if r1 == 0 {
		return "", repozipWindowsCallError(callErr)
	}
	defer func() { _, _ = syscall.LocalFree(syscall.Handle(uintptr(unsafe.Pointer(out)))) }()
	if out == nil || outLen == 0 {
		return "", syscall.EINVAL
	}
	rendered := syscall.UTF16ToString(unsafe.Slice(out, outLen))
	i := strings.LastIndex(rendered, ";;;")
	j := strings.LastIndex(rendered, ")")
	if i < 0 || j <= i+3 {
		return "", syscall.EINVAL
	}
	return rendered[i+3 : j], nil
}
