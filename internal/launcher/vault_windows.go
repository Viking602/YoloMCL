//go:build windows

package launcher

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

type dataBlob struct {
	Size uint32
	Data *byte
}

var crypt32 = syscall.NewLazyDLL("crypt32.dll")
var protect = crypt32.NewProc("CryptProtectData")
var unprotect = crypt32.NewProc("CryptUnprotectData")
var localFree = syscall.NewLazyDLL("kernel32.dll").NewProc("LocalFree")

func cryptBlob(b []byte, decrypt bool) ([]byte, error) {
	if len(b) == 0 {
		return nil, nil
	}
	in := dataBlob{uint32(len(b)), &b[0]}
	var out dataBlob
	proc := protect
	if decrypt {
		proc = unprotect
	}
	r, _, err := proc.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(b)
	if r == 0 {
		return nil, fmt.Errorf("Windows DPAPI 凭据操作失败: %w", err)
	}
	defer localFree.Call(uintptr(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, int(out.Size))...), nil
}
func seal(b []byte) ([]byte, error)   { return cryptBlob(b, false) }
func unseal(b []byte) ([]byte, error) { return cryptBlob(b, true) }
