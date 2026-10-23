package nativeui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// User-facing strings must say YoloMCL, never a bare "MCL".
func TestNoBareMCLInStrings(t *testing.T) {
	lit := regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)
	bare := regexp.MustCompile(`(^|[^A-Za-z.\-_/\\])MCL([^A-Za-z_\-]|$)`)
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, _ := os.ReadFile(f)
		for i, line := range strings.Split(string(b), "\n") {
			for _, s := range lit.FindAllString(line, -1) {
				if bare.MatchString(s) && !strings.Contains(s, "MCL Pixel") && !strings.Contains(s, "MCL JetBrains") && !strings.Contains(s, "MCL_") {
					t.Errorf("%s:%d bare MCL in %s", f, i+1, s)
				}
			}
		}
	}
}
