//go:build windows

package launcher

import (
	"strings"
	"syscall"
	"unsafe"
)

// hostJavaLanguage is the language a JVM started by this launcher takes as its
// default locale: on Windows, the user default locale. Only its language
// subtag is used (case mapping differs for tr/az/lt).
func hostJavaLanguage() string {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")
	if proc.Find() != nil {
		return ""
	}
	buf := make([]uint16, 85)
	n, _, _ := proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	name := syscall.UTF16ToString(buf)
	lang, _, _ := strings.Cut(name, "-")
	return strings.ToLower(lang)
}
