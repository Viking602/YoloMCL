package nativeui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every icon name the UI asks for must exist, or the button renders blank.
func TestIconNamesExist(t *testing.T) {
	if e := loadAssets(); e != nil {
		t.Fatal(e)
	}
	files, _ := filepath.Glob("*.go")
	re := regexp.MustCompile(`(?:icons\["|(?:iconOnlyButton\(c, "[^"]*", |iconOnlyButton\(c, [a-zA-Z.+"-]+, ))"?([a-z][a-z0-9-]*)"`)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, _ := os.ReadFile(f)
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			if icons[m[1]] == nil {
				t.Errorf("%s: icon %q is not defined", f, m[1])
			}
		}
	}
}
