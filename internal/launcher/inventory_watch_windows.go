package launcher

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	inventoryKernel32   = syscall.NewLazyDLL("kernel32.dll")
	inventoryWatchFirst = inventoryKernel32.NewProc("FindFirstChangeNotificationW")
	inventoryWatchNext  = inventoryKernel32.NewProc("FindNextChangeNotification")
	inventoryWatchClose = inventoryKernel32.NewProc("FindCloseChangeNotification")
	inventoryDriveType  = inventoryKernel32.NewProc("GetDriveTypeW")
)

type windowsInventoryWatch struct {
	path   string
	root   os.FileInfo
	handle syscall.Handle
}

func watchInventoryChanges(path string) inventoryChangeWatch {
	if !filepath.IsAbs(path) {
		return nil
	}
	volume, err := syscall.UTF16PtrFromString(filepath.VolumeName(path) + `\`)
	if err != nil {
		return nil
	}
	// Remote filesystems need polling: Windows does not guarantee their
	// notifications, including mapped network drives that have a drive letter.
	if kind, _, _ := inventoryDriveType.Call(uintptr(unsafe.Pointer(volume))); kind != 3 { // DRIVE_FIXED
		return nil
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	// Windows FileInfo resolves its file ID lazily. Freeze that identity now,
	// before the path can be replaced, rather than on the first idle poll.
	if !os.SameFile(info, info) {
		return nil
	}
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil
	}
	// One subtree notification handle per root, with no file-event buffer or
	// goroutine per version. File/dir names, attributes, size and last write.
	handle, _, _ := inventoryWatchFirst.Call(uintptr(unsafe.Pointer(name)), 1, 0x1f)
	if handle == ^uintptr(0) {
		return nil
	}
	return &windowsInventoryWatch{path: path, root: info, handle: syscall.Handle(handle)}
}

func (w *windowsInventoryWatch) changed() (bool, error) {
	// Subtree notifications exclude the root itself. Detect deletion,
	// replacement and junction substitution before trusting an idle handle.
	info, err := os.Lstat(w.path)
	if err != nil {
		return true, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, w.root) {
		return true, os.ErrInvalid
	}
	result, err := syscall.WaitForSingleObject(w.handle, 0)
	if err != nil {
		return true, err
	}
	if result == syscall.WAIT_TIMEOUT {
		return false, nil
	}
	if result != syscall.WAIT_OBJECT_0 {
		return true, os.ErrInvalid
	}
	// Only rearm a signalled handle. Rearming after a timeout can lose events
	// and resources (FindNextChangeNotification's wait contract).
	if ok, _, err := inventoryWatchNext.Call(uintptr(w.handle)); ok == 0 {
		return true, err
	}
	return true, nil
}

func (w *windowsInventoryWatch) close() {
	if w.handle != 0 {
		inventoryWatchClose.Call(uintptr(w.handle))
		w.handle = 0
	}
}
