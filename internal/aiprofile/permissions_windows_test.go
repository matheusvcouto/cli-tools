//go:build windows

package aiprofile

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

const winSEDACLProtected = 0x1000

var (
	procGetNamedSecurityInfoWTest = advapi32ProfilePermissions.NewProc("GetNamedSecurityInfoW")
	procGetSDControlTest          = advapi32ProfilePermissions.NewProc("GetSecurityDescriptorControl")
	procConvertSDToStringTest     = advapi32ProfilePermissions.NewProc("ConvertSecurityDescriptorToStringSecurityDescriptorW")
)

func TestEnsureRootAppliesProtectedWindowsACL(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "profiles")
	if err := os.Mkdir(rootPath, 0o700); err != nil {
		t.Fatal(err)
	}
	childPath := filepath.Join(rootPath, "existing.json")
	if err := os.WriteFile(childPath, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := (Store{Root: rootPath}).EnsureRoot(); err != nil {
		t.Fatalf("EnsureRoot: %v", err)
	}

	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{rootPath, childPath} {
		sddl, control := readWindowsDACLForTest(t, path)
		if !strings.Contains(sddl, current.Uid) {
			t.Fatalf("DACL for %q does not contain current user SID %q: %s", path, current.Uid, sddl)
		}
		if strings.Contains(sddl, ";;;WD)") || strings.Contains(sddl, ";;;S-1-1-0)") {
			t.Fatalf("DACL for %q grants Everyone access: %s", path, sddl)
		}
		if path == rootPath && control&winSEDACLProtected == 0 {
			t.Fatalf("root DACL is not protected: control=%#x sddl=%s", control, sddl)
		}
	}
}

func TestLoadExistingRootHardensWindowsACLBeforeReading(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "legacy-profiles")
	if err := os.Mkdir(rootPath, 0o777); err != nil {
		t.Fatal(err)
	}
	if _, err := (Store{Root: rootPath}).Load(); err != nil {
		t.Fatalf("Load legacy root: %v", err)
	}
	sddl, control := readWindowsDACLForTest(t, rootPath)
	if control&winSEDACLProtected == 0 {
		t.Fatalf("Load did not protect legacy root DACL: control=%#x sddl=%s", control, sddl)
	}
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sddl, current.Uid) {
		t.Fatalf("legacy root DACL does not contain current user SID %q: %s", current.Uid, sddl)
	}
	if strings.Contains(sddl, ";;;WD)") || strings.Contains(sddl, ";;;S-1-1-0)") {
		t.Fatalf("legacy root DACL grants Everyone access after Load: %s", sddl)
	}
}

func TestValidWindowsSIDString(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"S-1-5-21-1-2-3-1001", true},
		{"s-1-5-18", true},
		{"S-1-5-21-X", false},
		{"S-1-5-21);(A;;FA;;;WD", false},
		{"S--", false},
		{"S-1-5-", false},
		{" S-1-5-18", false},
		{"", false},
	} {
		if got := validWindowsSIDString(tc.in); got != tc.want {
			t.Fatalf("validWindowsSIDString(%q)=%v want %v", tc.in, got, tc.want)
		}
	}
}

func readWindowsDACLForTest(t *testing.T, path string) (string, uint16) {
	t.Helper()
	path16, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	var sd *winSecurityDescriptor
	ret, _, _ := procGetNamedSecurityInfoWTest.Call(
		uintptr(unsafe.Pointer(path16)),
		winSEFileObject,
		winDACLInfo,
		0, 0, 0, 0,
		uintptr(unsafe.Pointer(&sd)),
	)
	if ret != 0 {
		t.Fatalf("GetNamedSecurityInfoW(%q): %v", path, syscall.Errno(ret))
	}
	defer func() { _, _ = syscall.LocalFree(syscall.Handle(uintptr(unsafe.Pointer(sd)))) }()

	var control uint16
	var revision uint32
	r1, _, callErr := procGetSDControlTest.Call(
		uintptr(unsafe.Pointer(sd)),
		uintptr(unsafe.Pointer(&control)),
		uintptr(unsafe.Pointer(&revision)),
	)
	if r1 == 0 {
		t.Fatalf("GetSecurityDescriptorControl(%q): %v", path, windowsCallError(callErr))
	}

	var text *uint16
	var textLen uint32
	r1, _, callErr = procConvertSDToStringTest.Call(
		uintptr(unsafe.Pointer(sd)),
		winSDDLRevision1,
		winDACLInfo,
		uintptr(unsafe.Pointer(&text)),
		uintptr(unsafe.Pointer(&textLen)),
	)
	if r1 == 0 {
		t.Fatalf("ConvertSecurityDescriptorToStringSecurityDescriptorW(%q): %v", path, windowsCallError(callErr))
	}
	defer func() { _, _ = syscall.LocalFree(syscall.Handle(uintptr(unsafe.Pointer(text)))) }()
	if text == nil || textLen == 0 {
		t.Fatalf("empty SDDL returned for %q", path)
	}
	return syscall.UTF16ToString(unsafe.Slice(text, textLen)), control
}
