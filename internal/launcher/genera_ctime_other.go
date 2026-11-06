//go:build !windows

package launcher

import (
	"io/fs"
	"time"
)

// fileCreated falls back to the modification time where the platform's
// stat has no portable creation time.
func fileCreated(info fs.FileInfo) time.Time { return info.ModTime() }
