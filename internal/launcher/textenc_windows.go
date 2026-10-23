package launcher

import "syscall"

var procGetACP = syscall.NewLazyDLL("kernel32.dll").NewProc("GetACP")

func systemCodePage() uint32 {
	cp, _, _ := procGetACP.Call()
	return uint32(cp)
}
