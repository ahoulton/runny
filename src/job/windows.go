//go:build windows

package job

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// jobHandle holds the process-wide job object that contains every spawned
// Python script. It is never closed explicitly: when Runny exits, process
// teardown closes the last handle of the unnamed job, and Windows responds
// by terminating everything still in the job.
var _ windows.Handle

// Init assigns Runny to an unnamed job object with
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE so no spawned script can outlive
// Runny, even after a crash or taskkill. Children created with plain
// CreateProcess (what os/exec uses) join the parent's job automatically,
// so assigning the current process is enough to cover every script and
// its whole subtree.
//
// Assignment is best-effort: if it fails (for example Runny is already
// inside an incompatible job spawned by an IDE or terminal), print one
// warning and continue without containment.
func Init() {
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		_, err := fmt.Fprintf(os.Stderr, "Failed to enable child process containment: %v\n", err)
		if err != nil {
			return
		}
		return
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		handle,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(handle)
		_, err := fmt.Fprintf(os.Stderr, "Failed to enable child process containment: %v\n", err)
		if err != nil {
			return
		}
		return
	}

	if err := windows.AssignProcessToJobObject(handle, windows.CurrentProcess()); err != nil {
		_ = windows.CloseHandle(handle)
		_, err := fmt.Fprintf(os.Stderr, "Failed to enable child process containment: %v\n", err)
		if err != nil {
			return
		}
		return
	}

	_ = handle
}
