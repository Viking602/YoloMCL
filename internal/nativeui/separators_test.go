package nativeui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Ice's rule: no "·" / "•" separators anywhere in the UI, source or render.
func TestNoDotSeparators(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, _ := os.ReadFile(f)
		for i, line := range strings.Split(string(b), "\n") {
			if strings.ContainsAny(line, "·•") {
				t.Errorf("%s:%d uses a dot separator: %s", f, i+1, strings.TrimSpace(line))
			}
		}
	}
	for _, lang := range []string{"zh-CN", "en-US"} {
		a, tt := homeTester(t, lang)
		for _, page := range []string{"home", "discover", "wardrobe", "tasks", "settings"} {
			a.goPage(page)
			tt.Frame()
			tt.Advance(300 * time.Millisecond)
			for _, txt := range tt.Texts() {
				if strings.ContainsAny(txt, "·•") {
					t.Errorf("%s %s renders %q", lang, page, txt)
				}
				if txt == "MC" || strings.HasPrefix(txt, "MC ") && !strings.HasPrefix(txt, "MC 标签") {
					t.Errorf("%s %s shows a redundant MC label: %q", lang, page, txt)
				}
			}
		}
	}
}

func TestAddonNames(t *testing.T) {
	for _, c := range []struct{ name, file, p, s string }{
		{"[加速渲染] Accelerated Rendering", "", "加速渲染", "Accelerated Rendering"},
		{"钠 (Sodium)", "", "钠", "Sodium"},
		{"Sodium (钠)", "", "钠", "Sodium"},
		{"Lithium", "", "Lithium", ""},
		{"", "[AR]accelerated-1.0.jar", "accelerated-1.0", ""},
	} {
		p, s := addonNames(c.name, c.file)
		if p != c.p || s != c.s {
			t.Errorf("addonNames(%q,%q) = %q,%q want %q,%q", c.name, c.file, p, s, c.p, c.s)
		}
	}
}
