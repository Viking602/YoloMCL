//go:build windows

package memory

import (
	"fmt"
	"syscall"
	"unsafe"
)

// Snapshot polling queries memory every few seconds; resolve the export once.
var procGlobalMemoryStatusEx = syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")

func physicalMemory() (uint64, uint64, error) {
	var state struct {
		Length, Load                                                                                                  uint32
		TotalPhys, AvailablePhys, TotalPageFile, AvailablePageFile, TotalVirtual, AvailableVirtual, AvailableExtended uint64
	}
	state.Length = uint32(unsafe.Sizeof(state))
	ok, _, e := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&state)))
	if ok == 0 {
		return 0, 0, fmt.Errorf("无法检测系统内存: %w", e)
	}
	return state.TotalPhys, state.AvailablePhys, nil
}
