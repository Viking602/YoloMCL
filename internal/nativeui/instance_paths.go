package nativeui

import (
	"path/filepath"
	"strings"
)

func (a *App) hasRootPath(p string) bool {
	for _, r := range a.snapshot.Instances {
		if p == "" || strings.EqualFold(filepath.Clean(r.Path), filepath.Clean(p)) {
			return true
		}
	}
	return p == ""
}
