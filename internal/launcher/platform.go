package launcher

import (
	"os"
	"path/filepath"
	"runtime"
)

// DefaultGameDir is the official launcher's game directory on this OS:
// %APPDATA%\.minecraft, ~/Library/Application Support/minecraft or
// ~/.minecraft. It returns "" when the directory does not exist.
func DefaultGameDir() string { return defaultGameDir(runtime.GOOS, os.Getenv("APPDATA"), homeDir()) }

func homeDir() string { h, _ := os.UserHomeDir(); return h }

func defaultGameDir(goos, appData, home string) string {
	var p string
	switch goos {
	case "windows":
		if appData == "" {
			return ""
		}
		p = filepath.Join(appData, ".minecraft")
	case "darwin":
		p = filepath.Join(home, "Library", "Application Support", "minecraft")
	default:
		p = filepath.Join(home, ".minecraft")
	}
	if st, e := os.Stat(p); e == nil && st.IsDir() {
		return p
	}
	return ""
}
