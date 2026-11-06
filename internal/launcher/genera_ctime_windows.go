//go:build windows

package launcher

import (
	"io/fs"
	"syscall"
	"time"
)

// fileCreated is a file's creation time (NTFS keeps it when a file is
// copied in), falling back to the modification time.
func fileCreated(info fs.FileInfo) time.Time {
	if d, ok := info.Sys().(*syscall.Win32FileAttributeData); ok && d != nil {
		return time.Unix(0, d.CreationTime.Nanoseconds())
	}
	return info.ModTime()
}
