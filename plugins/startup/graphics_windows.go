//go:build windows

package startup

import (
	"context"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

type startupSwapChain struct {
	Width, Height                        uint32
	RefreshNumerator, RefreshDenominator uint32
	Format, Scanline, Scaling            uint32
	SampleCount, SampleQuality           uint32
	Usage, BufferCount                   uint32
	Window                               uintptr
	Windowed, SwapEffect, Flags          uint32
}

func prepareGraphics(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// Load system DLLs by absolute path, independent of CWD and PATH.
	kernel := syscall.NewLazyDLL("kernel32.dll")
	var buf [32768]uint16
	n, _, _ := kernel.NewProc("GetSystemDirectoryW").Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 || n >= uintptr(len(buf)) {
		return
	}
	system := syscall.UTF16ToString(buf[:n])
	user := syscall.NewLazyDLL(filepath.Join(system, "user32.dll"))
	create := syscall.NewLazyDLL(filepath.Join(system, "d3d11.dll")).NewProc("D3D11CreateDeviceAndSwapChain")
	if create.Find() != nil {
		return
	}
	class, _ := syscall.UTF16PtrFromString("STATIC")
	hwnd, _, _ := user.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(class)), 0, 0x80000000, 0, 0, 16, 16, 0, 0, 0, 0)
	if hwnd == 0 {
		return
	}
	defer user.NewProc("DestroyWindow").Call(hwnd)
	desc := startupSwapChain{Width: 16, Height: 16, Format: 87, SampleCount: 1, Usage: 0x20, BufferCount: 2, Window: hwnd, Windowed: 1, SwapEffect: 4}
	levels := [...]uint32{0xb000, 0xa100, 0xa000}
	var swap, device, context unsafe.Pointer
	var level uint32
	hr, _, _ := create.Call(0, 1, 0, 0x20, uintptr(unsafe.Pointer(&levels[0])), uintptr(len(levels)), 7,
		uintptr(unsafe.Pointer(&desc)), uintptr(unsafe.Pointer(&swap)), uintptr(unsafe.Pointer(&device)), uintptr(unsafe.Pointer(&level)), uintptr(unsafe.Pointer(&context)))
	defer func() {
		startupRelease(context)
		startupRelease(swap)
		startupRelease(device)
	}()
	if int32(uint32(hr)) < 0 || swap == nil || device == nil || context == nil {
		return
	}
	if ctx.Err() != nil {
		return
	}
	// A bare Present does not warm the first shader submission. Draw one
	// invisible 16px quad with the pinned toolkit bytecode as well. Failure is
	// harmless: the real window retains MyGo's normal GPU/software fallback.
	_ = warmPipeline(device, context, swap)
}

//go:uintptrescapes
func startupCOM(object unsafe.Pointer, method int, args ...uintptr) uintptr {
	vtable := *(*unsafe.Pointer)(object)
	fn := *(*uintptr)(unsafe.Add(vtable, uintptr(method)*unsafe.Sizeof(uintptr(0))))
	result, _, _ := syscall.SyscallN(fn, append([]uintptr{uintptr(object)}, args...)...)
	return result
}

func startupRelease(object unsafe.Pointer) {
	if object != nil {
		startupCOM(object, 2)
	}
}
