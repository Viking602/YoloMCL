package nativeui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestFriendlyTime(t *testing.T) {
	now := time.Date(2026, 10, 21, 22, 0, 0, 0, time.Local)
	cases := []struct {
		at time.Time
		zh string
		en string
	}{
		{now.Add(-20 * time.Second), "刚刚", "just now"},
		{now.Add(-11 * time.Minute), "11 分钟前", "11 min ago"},
		{now.Add(-3 * time.Hour), "今天 19:00", "Today 19:00"},
		{now.Add(-24 * time.Hour), "昨天 22:00", "Yesterday 22:00"},
		{now.Add(-72 * time.Hour), "3 天前", "3 days ago"},
		{time.Date(2026, 8, 30, 9, 0, 0, 0, time.Local), "08月30日", "Aug 30"},
		{time.Date(2025, 8, 9, 9, 0, 0, 0, time.Local), "2025年08月09日", "Aug 09, 2025"},
	}
	for _, c := range cases {
		if g := friendlyTime(false, c.at, now); g != c.zh {
			t.Errorf("zh %v = %q, want %q", c.at, g, c.zh)
		}
		if g := friendlyTime(true, c.at, now); g != c.en {
			t.Errorf("en %v = %q, want %q", c.at, g, c.en)
		}
	}
}

func TestModWindowVirtualizes(t *testing.T) {
	if f, l := modWindow(30, 60, ui.Rect{}, ui.Rect{}); f != 0 || l != 30 {
		t.Fatalf("small list should render fully, got %d..%d", f, l)
	}
	if f, l := modWindow(237, 60, ui.Rect{}, ui.Rect{}); f != 0 || l != 40 {
		t.Fatalf("first frame should draw a screenful, got %d..%d", f, l)
	}
	view := ui.Rect{Y: 200, H: 600}
	list := ui.Rect{Y: 200 - 6000, H: 237 * 60} // scrolled 100 rows down
	f, l := modWindow(237, 60, list, view)
	if f != 88 || l != 123 || l-f > 40 {
		t.Fatalf("window = %d..%d", f, l)
	}
	list.Y = 200 - 237*60 + 100 // at the very end
	if _, l := modWindow(237, 60, list, view); l != 237 {
		t.Fatalf("end window last = %d", l)
	}
}

func TestJavaDisplayCapitalises(t *testing.T) {
	for in, want := range map[string]string{
		"java-runtime-delta": "Java-runtime-delta",
		"java-21-openjdk":    "Java-21-openjdk",
		"JAVA vendor":        "Java vendor",
		"javaw":              "javaw",
		"jdk-21.0.4+7":       "jdk-21.0.4+7",
		"Oracle java SE":     "Oracle Java SE",
	} {
		if got := javaDisplay(in); got != want {
			t.Errorf("%q -> %q want %q", in, got, want)
		}
	}
}
