//go:build !windows

package structure3d_test

import (
	"os"
	"time"
)

// fileCreated falls back to the modification time off Windows.
func fileCreated(p string) (time.Time, bool) {
	st, err := os.Stat(p)
	if err != nil {
		return time.Time{}, false
	}
	return st.ModTime(), true
}
