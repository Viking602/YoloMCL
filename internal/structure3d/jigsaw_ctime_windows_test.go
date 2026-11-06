//go:build windows

package structure3d_test

import (
	"os"
	"syscall"
	"time"
)

// fileCreated is the file's creation time (NTFS keeps one).
func fileCreated(p string) (time.Time, bool) {
	st, err := os.Stat(p)
	if err != nil {
		return time.Time{}, false
	}
	d, ok := st.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return st.ModTime(), true
	}
	return time.Unix(0, d.CreationTime.Nanoseconds()), true
}
