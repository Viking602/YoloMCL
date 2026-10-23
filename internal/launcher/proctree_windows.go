//go:build windows

package launcher

import (
	"errors"
	"os/exec"
	"syscall"
	"unsafe"
)

var (
	k32                 = syscall.NewLazyDLL("kernel32.dll")
	pCreateJobObject    = k32.NewProc("CreateJobObjectW")
	pSetInfoJobObject   = k32.NewProc("SetInformationJobObject")
	pQueryInfoJobObject = k32.NewProc("QueryInformationJobObject")
	pAssignProcessToJob = k32.NewProc("AssignProcessToJobObject")
	pTerminateJobObject = k32.NewProc("TerminateJobObject")
)

type jobBasicLimit struct {
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

type jobExtLimit struct {
	Basic                 jobBasicLimit
	IoInfo                [6]uint64
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

type jobAccounting struct {
	TotalUserTime, TotalKernelTime, ThisPeriodUserTime, ThisPeriodKernelTime int64
	TotalPageFaults, TotalProcesses, ActiveProcesses, TotalTerminated        uint32
}

// procTree holds a game's whole process tree in a Job Object: every child
// the game spawns (e.g. Crash Assistant's watcher JVM) joins the job, kill
// ends them all, and KILL_ON_JOB_CLOSE ends them if the launcher dies.
type procTree struct {
	job syscall.Handle
	cmd *exec.Cmd
}

func prepareTree(c *exec.Cmd) {}

func attachTree(c *exec.Cmd) (*procTree, error) {
	t := &procTree{cmd: c}
	h, _, e := pCreateJobObject.Call(0, 0)
	if h == 0 {
		return t, e
	}
	t.job = syscall.Handle(h)
	var info jobExtLimit
	info.Basic.LimitFlags = 0x2000 // JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if r, _, e := pSetInfoJobObject.Call(h, 9, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info)); r == 0 {
		return t, e
	}
	ph, e := syscall.OpenProcess(0x0001|0x0100|0x0400, false, uint32(c.Process.Pid)) // TERMINATE|SET_QUOTA|QUERY_INFORMATION
	if e != nil {
		return t, e
	}
	defer syscall.CloseHandle(ph)
	if r, _, e := pAssignProcessToJob.Call(h, uintptr(ph)); r == 0 {
		return t, e
	}
	return t, nil
}

// kill ends every process in the tree.
func (t *procTree) kill() error {
	if t.job != 0 {
		if r, _, e := pTerminateJobObject.Call(uintptr(t.job), 1); r != 0 {
			return nil
		} else if t.cmd.Process == nil {
			return e
		}
	}
	if t.cmd.Process == nil {
		return errors.New("no process")
	}
	return t.cmd.Process.Kill()
}

// alive reports whether any process of the tree still runs.
func (t *procTree) alive() bool {
	if t.job == 0 {
		return false
	}
	var a jobAccounting
	if r, _, _ := pQueryInfoJobObject.Call(uintptr(t.job), 1, uintptr(unsafe.Pointer(&a)), unsafe.Sizeof(a), 0); r == 0 {
		return false
	}
	return a.ActiveProcesses > 0
}

func (t *procTree) close() {
	if t.job != 0 {
		_ = syscall.CloseHandle(t.job)
		t.job = 0
	}
}
