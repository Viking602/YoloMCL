package nativeui

import (
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// Keep known runtimes visible during a partial/cancelled refresh. A completed
// scan replaces the list so removed installations are still dropped.
func mergeJavaScanRows(known, found []launcher.JavaInfo) []launcher.JavaInfo {
	key := func(p string) string {
		p = filepath.Clean(p)
		if runtime.GOOS == "windows" {
			p = strings.ToLower(p)
		}
		return p
	}
	out := append([]launcher.JavaInfo(nil), known...)
	index := make(map[string]int, len(out))
	for i, j := range out {
		index[key(j.Path)] = i
	}
	for _, j := range found {
		if i, ok := index[key(j.Path)]; ok {
			out[i] = j
		} else {
			index[key(j.Path)] = len(out)
			out = append(out, j)
		}
	}
	return out
}
