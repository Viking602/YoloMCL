//go:build !windows

package launcher

// systemCodePage: non-Windows systems are UTF-8; GBK logs copied from
// Windows still decode through the GB18030 fallback.
func systemCodePage() uint32 { return 65001 }
