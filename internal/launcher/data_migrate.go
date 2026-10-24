package launcher

import (
	"os"
	"path/filepath"
	"strings"
)

// migrateDataRoot moves the pre-rename data folder (%APPDATA%\MCL) to the
// YoloMCL root once: everything except Java runtimes (runtime/, runtimes/ and
// their temp folders), which are dropped so YoloMCL starts with one managed
// runtime folder. Runs only when the new root does not exist yet. The old
// folder is removed afterwards if nothing is left in it.
func migrateDataRoot(oldDir, newDir string) error {
	if _, e := os.Stat(newDir); e == nil {
		return nil
	}
	ents, e := os.ReadDir(oldDir)
	if e != nil {
		return nil
	}
	if e := os.MkdirAll(newDir, 0o700); e != nil {
		return e
	}
	var firstErr error
	for _, en := range ents {
		n := en.Name()
		if isRuntimeEntry(n) {
			continue
		}
		if e := os.Rename(filepath.Join(oldDir, n), filepath.Join(newDir, n)); e != nil && firstErr == nil {
			firstErr = e
		}
	}
	if firstErr == nil {
		left, _ := os.ReadDir(oldDir)
		onlyRuntimes := true
		for _, en := range left {
			if !isRuntimeEntry(en.Name()) {
				onlyRuntimes = false
			}
		}
		if onlyRuntimes {
			_ = os.RemoveAll(oldDir)
		}
	}
	return firstErr
}

func isRuntimeEntry(n string) bool {
	n = strings.ToLower(n)
	return n == "runtime" || n == "runtimes"
}
