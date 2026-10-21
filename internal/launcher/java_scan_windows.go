//go:build windows

package launcher

import (
	"os"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

func registryString(h syscall.Handle, name string) string {
	p, e := syscall.UTF16PtrFromString(name)
	if e != nil {
		return ""
	}
	var kind, size uint32
	if syscall.RegQueryValueEx(h, p, nil, &kind, nil, &size) != nil || size < 2 || size > 65536 || (kind != syscall.REG_SZ && kind != syscall.REG_EXPAND_SZ) {
		return ""
	}
	b := make([]uint16, (size+1)/2)
	if syscall.RegQueryValueEx(h, p, nil, &kind, (*byte)(unsafe.Pointer(&b[0])), &size) != nil {
		return ""
	}
	value := syscall.UTF16ToString(b)
	if kind == syscall.REG_EXPAND_SZ {
		for _, part := range strings.Split(value, "%") {
			if env := os.Getenv(part); env != "" {
				value = strings.ReplaceAll(value, "%"+part+"%", env)
			}
		}
	}
	return value
}

func registryChildren(h syscall.Handle) []string {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	out := []string{}
	for i := uint32(0); i < 2048; i++ {
		var buf [512]uint16
		n := uint32(len(buf))
		e := syscall.RegEnumKeyEx(h, i, &buf[0], &n, nil, nil, nil, nil)
		if e != nil {
			break
		}
		out = append(out, syscall.UTF16ToString(buf[:n]))
	}
	return out
}

func platformJavaHomes() []string {
	out := []string{}
	var walk func(syscall.Handle, string, uint32, int)
	walk = func(root syscall.Handle, path string, flags uint32, depth int) {
		if depth < 0 {
			return
		}
		p, e := syscall.UTF16PtrFromString(path)
		if e != nil {
			return
		}
		var h syscall.Handle
		if syscall.RegOpenKeyEx(root, p, 0, syscall.KEY_READ|flags, &h) != nil {
			return
		}
		defer syscall.RegCloseKey(h)
		for _, key := range []string{"JavaHome", "InstallationPath", "InstallPath", "InstallLocation", "Path"} {
			if v := registryString(h, key); v != "" {
				out = append(out, v)
			}
		}
		for _, child := range registryChildren(h) {
			walk(root, path+`\`+child, flags, depth-1)
		}
	}
	for _, root := range []syscall.Handle{syscall.HKEY_LOCAL_MACHINE, syscall.HKEY_CURRENT_USER} {
		for _, view := range []uint32{syscall.KEY_WOW64_64KEY, syscall.KEY_WOW64_32KEY} {
			for _, key := range []string{`SOFTWARE\JavaSoft`, `SOFTWARE\Eclipse Adoptium`, `SOFTWARE\AdoptOpenJDK`, `SOFTWARE\Azul Systems\Zulu`, `SOFTWARE\Microsoft\JDK`, `SOFTWARE\BellSoft`, `SOFTWARE\Amazon Corretto`} {
				walk(root, key, view, 5)
			}
		}
	}
	return out
}
func platformJavaDisks() []string {
	dll := syscall.NewLazyDLL("kernel32.dll")
	bits, _, _ := dll.NewProc("GetLogicalDrives").Call()
	out := []string{}
	for i := 0; i < 26; i++ {
		if bits&(1<<i) == 0 {
			continue
		}
		path := string(rune('A'+i)) + `:\`
		p, _ := syscall.UTF16PtrFromString(path)
		kind, _, _ := dll.NewProc("GetDriveTypeW").Call(uintptr(unsafe.Pointer(p)))
		if kind == 3 || kind == 2 {
			out = append(out, path)
		}
	}
	return out
}
