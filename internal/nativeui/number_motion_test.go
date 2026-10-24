package nativeui

import (
	"bytes"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestNumberRollKeepsUnitsAndUnchangedDigitsStill(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	a := &App{}
	value := "123.4 GB"
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		ui.Column(c).Fill().Padding(24).Background(panel).Children(func() {
			a.rollNumber(c, "capacity", value, 32, 700, ink, true).Width(240)
		})
	}, 300, 100)
	initial := append([]byte(nil), tt.Image().Pix...)
	tt.Advance(time.Second)
	if !bytes.Equal(initial, tt.Image().Pix) {
		t.Fatal("unchanged number animated")
	}
	unit, _ := tt.Find("数字单位 capacity1")
	unchanged, _ := tt.Find("数字位 capacity-0-2")
	unitPixels := overviewRegionPixels(tt.Image(), unit)
	unchangedPixels := overviewRegionPixels(tt.Image(), unchanged)
	value = "193.4 GB"
	tt.Frame()
	snapshotUI(t, tt, "number-roll-start")
	tt.Advance(100 * time.Millisecond)
	if bytes.Equal(initial, tt.Image().Pix) {
		t.Fatal("changed digit did not roll")
	}
	if !bytes.Equal(unitPixels, overviewRegionPixels(tt.Image(), unit)) || !bytes.Equal(unchangedPixels, overviewRegionPixels(tt.Image(), unchanged)) {
		t.Fatal("number roll animated its unit or an unchanged digit")
	}
	snapshotUI(t, tt, "number-roll-100ms")
	tt.Advance(140 * time.Millisecond)
	snapshotUI(t, tt, "number-roll-240ms")
	tt.Advance(160 * time.Millisecond)
	if !tt.HasText(value) {
		t.Fatal("number roll did not settle on its target")
	}
	if now, _ := tt.Find("数字单位 capacity1"); !sameNumberBounds(now, unit) {
		t.Fatal("number unit moved")
	}
	snapshotUI(t, tt, "number-roll-complete")
	// Shrinking values keep the unit anchored at the same right edge.
	value = "3.4 GB"
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	if now, _ := tt.Find("数字单位 capacity1"); !sameNumberBounds(now, unit) {
		t.Fatal("shorter value shifted the unit")
	}
}

func TestNumberRollRapidTargetsAndReducedMotion(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	a := &App{}
	value := "93"
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Background(panel).Children(func() {
			a.rollNumber(c, "count", value, 24, 700, ink).Width(180)
		})
	}, 220, 80)
	first, _ := tt.Find("动态数字 count")
	value = "300"
	tt.Frame()
	if now, _ := tt.Find("动态数字 count"); now != first {
		t.Fatal("adding a digit moved the existing number")
	}
	tt.Advance(60 * time.Millisecond)
	value = "237"
	tt.Frame()
	tt.Advance(340 * time.Millisecond)
	if !tt.HasText("237") || tt.HasText("300") {
		t.Fatal("rapid switch retained an obsolete target")
	}
	value = "48"
	tt.Frame()
	tt.Advance(250 * time.Millisecond)
	value = "12"
	tt.Frame()
	tt.Advance(120 * time.Millisecond)
	tt.Advance(400 * time.Millisecond)
	if !tt.HasText("12") {
		t.Fatal("a change during the incoming half was dropped")
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	value = "1024"
	tt.Frame()
	if !tt.HasText(value) {
		t.Fatal("reduced motion kept the previous number")
	}
	still := append([]byte(nil), tt.Image().Pix...)
	tt.Advance(time.Second)
	if !bytes.Equal(still, tt.Image().Pix) {
		t.Fatal("reduced motion still animates digits")
	}
	value = "3"
	tt.Frame()
	if units, _ := tt.Find("数字位 count-0-0"); units.X != first.X {
		t.Fatal("a shorter left-aligned value was left indented")
	}
}

func TestNumberRollStaggersDigitsAndReversesDirection(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	a := &App{}
	value := "128"
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		ui.Column(c).Fill().Background(chromeBg).Padding(24).Children(func() {
			ui.Column(c).Width(260).Padding(20).Gap(8).Radius(16).Background(panel).Children(func() {
				ui.Text(c, "已安装模组").Font(uiSansFamily()).FontSize(14).TextColor(muted)
				a.rollNumber(c, "mods", value, 36, 750, ink).FillWidth()
				a.rollNumber(c, "enabled", value+" 个已启用", 13, 400, muted).FillWidth()
			})
		})
	}, 310, 190)
	snapshotUI(t, tt, "number-count-128")
	value = "371"
	tt.Frame()
	tt.Advance(20 * time.Millisecond)
	digits := a.numberRolls["mods"].digits
	if digits["mods-0-0"].position <= 8 || digits["mods-0-1"].position != 2 || digits["mods-0-2"].position != 1 {
		t.Fatal("number columns started together instead of units first")
	}
	tt.Advance(40 * time.Millisecond)
	if digits["mods-0-1"].position <= 2 || digits["mods-0-2"].position != 1 {
		t.Fatal("tens did not follow units before hundreds")
	}
	snapshotUI(t, tt, "number-count-060ms")
	tt.Advance(50 * time.Millisecond)
	if digits["mods-0-2"].position <= 1 {
		t.Fatal("hundreds did not join the staggered roll")
	}
	snapshotUI(t, tt, "number-count-110ms")
	tt.Advance(100 * time.Millisecond)
	snapshotUI(t, tt, "number-count-210ms")
	tt.Advance(300 * time.Millisecond)
	if !tt.HasText("371") || !tt.HasText("371 个已启用") {
		t.Fatal("main count and enabled count did not settle together")
	}
	snapshotUI(t, tt, "number-count-371")
	value = "128"
	tt.Frame()
	tt.Advance(20 * time.Millisecond)
	if digits["mods-0-0"].position >= 1 || digits["mods-0-1"].position != 7 || digits["mods-0-2"].position != 3 {
		t.Fatal("decreasing counts did not roll backwards from units")
	}
	tt.Advance(500 * time.Millisecond)
	if !tt.HasText("128") {
		t.Fatal("decreasing roll did not settle")
	}
}

func TestNumberRollCarryPrecisionAndInterruption(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	a := &App{}
	value := "99"
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(20).Background(panel).Children(func() {
			a.rollNumber(c, "count", value, 30, 700, ink, true).Width(250)
		})
	}, 300, 100)
	units, _ := tt.Find("数字位 count-0-0")
	value = "100"
	tt.Frame()
	if now, _ := tt.Find("数字位 count-0-0"); !sameNumberBounds(now, units) {
		t.Fatalf("carry moved the anchored units column: before=%+v after=%+v", units, now)
	}
	tt.Advance(110 * time.Millisecond)
	tt.Frame() // sample at the exact time of the incoming request
	d := a.numberRolls["count"].digits["count-0-0"]
	current := d.position
	value = "237"
	tt.Frame()
	if d.position != current || math.Abs(float64(d.target-current)) > 5 || d.anim.Delay != 0 {
		t.Fatal("interrupting a roll jumped, paused, or queued the previous target")
	}
	tt.Advance(500 * time.Millisecond)
	if !tt.HasText(value) {
		t.Fatal("interrupted carry retained an old count")
	}
	value = "3.40 GB"
	tt.Frame()
	tt.Advance(500 * time.Millisecond)
	point, _ := tt.Find("小数点 count-0")
	integer := a.numberRolls["count"].digits["count-0-0"]
	value = "3.45 GB"
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	if now, _ := tt.Find("小数点 count-0"); !sameNumberBounds(now, point) || integer.position != 3 {
		t.Fatal("fractional update moved the point or rolled the unchanged integer")
	}
	tt.Advance(500 * time.Millisecond)
	value = "3.4 GB"
	tt.Frame()
	tt.Advance(500 * time.Millisecond)
	if now, _ := tt.Find("小数点 count-0"); !sameNumberBounds(now, point) || integer.position != 3 {
		t.Fatal("shrinking precision moved the point or rolled the unchanged integer")
	}
	for _, next := range []string{"—", "371", "999999999999999", "0"} {
		value = next
		tt.Frame()
		tt.Advance(motionNumber.Duration + 3*numberStagger + 50*time.Millisecond)
		if !tt.HasText(value) {
			t.Fatalf("roll exceeded its bounded duration for %s", next)
		}
		for id, digit := range a.numberRolls["count"].digits {
			if digit.position < 0 || digit.position >= 10 {
				t.Fatalf("settled wheel %s was not normalized: %v", id, digit.position)
			}
		}
	}
	// Stop a live stagger immediately when reduced motion is enabled.
	value = "123456789"
	tt.Frame()
	tt.Advance(50 * time.Millisecond)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	if !tt.HasText(value) {
		t.Fatal("reduced motion did not stop an active stagger")
	}
}

func TestNumberRollKeepsRightAnchorAcrossFontSizes(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	for _, size := range []float32{13.7, 24, 30, 30.25, 36} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			a := &App{}
			value := "99"
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).Fill().Padding(20).Children(func() {
					a.rollNumber(c, "count", value, size, 700, ink, true).Width(250)
				})
			}, 300, 100)
			units, found := tt.Find("数字位 count-0-0")
			if !found {
				t.Fatal("units column is missing")
			}
			for _, next := range []string{"100", "999", "1000", "9"} {
				value = next
				tt.Frame()
				if now, found := tt.Find("数字位 count-0-0"); !found || !sameNumberBounds(now, units) {
					t.Fatalf("%s moved units: before=%+v after=%+v", next, units, now)
				}
				tt.Advance(600 * time.Millisecond)
			}
		})
	}
}

func sameNumberBounds(a, b ui.Rect) bool {
	// Fractional platform font metrics can accumulate float32 rounding error.
	// Keep the tolerance far below a visible pixel or a column's width.
	const epsilon = .001
	return math.Abs(float64(a.X-b.X)) <= epsilon && math.Abs(float64(a.Y-b.Y)) <= epsilon &&
		math.Abs(float64(a.W-b.W)) <= epsilon && math.Abs(float64(a.H-b.H)) <= epsilon
}

func TestNumberDirectionKeepsDecimalPrecision(t *testing.T) {
	for _, tc := range []struct {
		from, to string
		want     float32
	}{{"99", "100", 1}, {"100", "99", -1}, {"1.2", "1.19", -1}, {"1.05", "1.1", 1},
		{"999999999999999999", "999999999999999998", -1}} {
		t.Run(fmt.Sprintf("%s-to-%s", tc.from, tc.to), func(t *testing.T) {
			if got := numberDirection(tc.from, tc.to); got != tc.want {
				t.Fatalf("direction = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCapacityUnits(t *testing.T) {
	for _, tc := range []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"}, {512, "512 B"}, {1 << 10, "1 KB"},
		{1 << 20, "1.0 MB"}, {1023 << 20, "1023.0 MB"},
		{1 << 30, "1.0 GB"}, {3615385190, "3.4 GB"}, {1 << 40, "1.0 TB"},
	} {
		if got := fmtSize(tc.bytes); got != tc.want {
			t.Errorf("%d bytes = %q, want %q", tc.bytes, got, tc.want)
		}
		if tc.bytes >= 1<<30 && sizeText(tc.bytes) != tc.want {
			t.Errorf("compact capacity has not switched to GB/TB: %s", sizeText(tc.bytes))
		}
	}
}
