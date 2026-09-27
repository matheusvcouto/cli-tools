//go:build windows

package platform

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	processTerminate    = 0x0001
	processSetQuota     = 0x0100
	threadSuspendResume = 0x0002

	createSuspended  = 0x00000004
	th32csSnapThread = 0x00000004

	jobObjectExtendedLimitInformationClass = 9
	jobObjectLimitKillOnJobClose           = 0x00002000
	// WinError.h ERROR_NO_MORE_FILES (18); not exposed by Go syscall.
	windowsErrorNoMoreFiles syscall.Errno = 18
)

var (
	jobKernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObjectW         = jobKernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObject  = jobKernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJobObject = jobKernel32.NewProc("AssignProcessToJobObject")
	procTerminateJobObject       = jobKernel32.NewProc("TerminateJobObject")
	procCreateToolhelp32Snapshot = jobKernel32.NewProc("CreateToolhelp32Snapshot")
	procThread32First            = jobKernel32.NewProc("Thread32First")
	procThread32Next             = jobKernel32.NewProc("Thread32Next")
	procOpenThread               = jobKernel32.NewProc("OpenThread")
	procResumeThread             = jobKernel32.NewProc("ResumeThread")
)

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobObjectExtendedLimitInformation struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

type threadEntry32 struct {
	Size           uint32
	Usage          uint32
	ThreadID       uint32
	OwnerProcessID uint32
	BasePri        int32
	DeltaPri       int32
	Flags          uint32
}

type windowsJob struct {
	handle syscall.Handle
}

func newWindowsJob() (*windowsJob, error) {
	r1, _, callErr := procCreateJobObjectW.Call(0, 0)
	if r1 == 0 {
		return nil, fmt.Errorf("CreateJobObjectW: %w", windowsCallError(callErr))
	}
	job := &windowsJob{handle: syscall.Handle(r1)}

	// KILL_ON_JOB_CLOSE is a final containment guarantee: if this wrapper exits
	// unexpectedly after assigning the child, Windows terminates every process
	// still associated with this job instead of leaving an agent subtree behind.
	info := jobObjectExtendedLimitInformation{}
	info.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
	r1, _, callErr = procSetInformationJobObject.Call(
		uintptr(job.handle),
		uintptr(jobObjectExtendedLimitInformationClass),
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
	)
	runtime.KeepAlive(&info)
	if r1 == 0 {
		_ = job.Close()
		return nil, fmt.Errorf("SetInformationJobObject(KILL_ON_JOB_CLOSE): %w", windowsCallError(callErr))
	}
	return job, nil
}

func (j *windowsJob) Assign(pid int) error {
	process, err := syscall.OpenProcess(processSetQuota|processTerminate, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("OpenProcess(%d): %w", pid, err)
	}
	defer syscall.CloseHandle(process)

	r1, _, callErr := procAssignProcessToJobObject.Call(uintptr(j.handle), uintptr(process))
	if r1 == 0 {
		return fmt.Errorf("AssignProcessToJobObject(%d): %w", pid, windowsCallError(callErr))
	}
	return nil
}

func (j *windowsJob) Terminate(exitCode uint32) error {
	r1, _, callErr := procTerminateJobObject.Call(uintptr(j.handle), uintptr(exitCode))
	if r1 == 0 {
		return fmt.Errorf("TerminateJobObject: %w", windowsCallError(callErr))
	}
	return nil
}

func (j *windowsJob) Close() error {
	if j == nil || j.handle == 0 {
		return nil
	}
	handle := j.handle
	j.handle = 0
	if err := syscall.CloseHandle(handle); err != nil {
		return fmt.Errorf("CloseHandle(job): %w", err)
	}
	return nil
}

func windowsCallError(callErr error) error {
	if callErr != nil && callErr != syscall.Errno(0) {
		return callErr
	}
	return syscall.EINVAL
}

// resumeSuspendedProcess resumes the primary thread of a process created with
// CREATE_SUSPENDED. The process is assigned to its job before this function is
// called, removing the post-Start window in which an agent could create an
// uncontained descendant. CREATE_SUSPENDED guarantees that the primary thread
// cannot execute until ResumeThread is called.
func resumeSuspendedProcess(pid int) error {
	snapshotRaw, _, callErr := procCreateToolhelp32Snapshot.Call(uintptr(th32csSnapThread), 0)
	snapshot := syscall.Handle(snapshotRaw)
	if snapshot == syscall.InvalidHandle {
		return fmt.Errorf("CreateToolhelp32Snapshot: %w", windowsCallError(callErr))
	}
	defer syscall.CloseHandle(snapshot)

	entry := threadEntry32{Size: uint32(unsafe.Sizeof(threadEntry32{}))}
	r1, _, callErr := procThread32First.Call(uintptr(snapshot), uintptr(unsafe.Pointer(&entry)))
	for r1 != 0 {
		if entry.OwnerProcessID == uint32(pid) {
			threadRaw, _, openErr := procOpenThread.Call(uintptr(threadSuspendResume), 0, uintptr(entry.ThreadID))
			if threadRaw == 0 {
				return fmt.Errorf("OpenThread(%d): %w", entry.ThreadID, windowsCallError(openErr))
			}
			thread := syscall.Handle(threadRaw)
			resumeCount, _, resumeErr := procResumeThread.Call(uintptr(thread))
			closeErr := syscall.CloseHandle(thread)
			if resumeCount == uintptr(^uint32(0)) {
				return fmt.Errorf("ResumeThread(%d): %w", entry.ThreadID, windowsCallError(resumeErr))
			}
			if resumeCount != 1 {
				return fmt.Errorf("ResumeThread(%d): unexpected previous suspend count %d", entry.ThreadID, resumeCount)
			}
			if closeErr != nil {
				return fmt.Errorf("CloseHandle(thread): %w", closeErr)
			}
			return nil
		}

		entry.Size = uint32(unsafe.Sizeof(threadEntry32{}))
		r1, _, callErr = procThread32Next.Call(uintptr(snapshot), uintptr(unsafe.Pointer(&entry)))
	}
	if callErr != nil && callErr != syscall.Errno(0) && callErr != windowsErrorNoMoreFiles {
		return fmt.Errorf("Thread32Next: %w", callErr)
	}
	return fmt.Errorf("primary thread for process %d not found", pid)
}
