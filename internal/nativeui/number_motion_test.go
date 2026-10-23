package nativeui

import (
	"bytes"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestNumberFlipKeepsUnitsAndUnchangedDigitsStill(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	a := &App{}
	value := "123.4 GB"
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		ui.Column(c).Fill().Padding(24).Background(panel).Children(func() {
			a.flipNumber(c, "capacity", value, 32, 700, ink, true).Width(240)
		})
	}, 300, 100)
	initial := append([]byte(nil), tt.Image().Pix...)
	tt.Advance(time.Second)
	if !bytes.Equal(initial, tt.Image().Pix) {
		t.Fatal("unchanged number animated")
	}
	unit, _ := tt.Find("数字单位 capacity1")
	unchanged, _ := tt.Find("数字位 capacity-0-0")
	unitPixels := overviewRegionPixels(tt.Image(), unit)
	unchangedPixels := overviewRegionPixels(tt.Image(), unchanged)
	value = "193.4 GB"
	tt.Frame()
	snapshotUI(t, tt, "number-flip-start")
	tt.Advance(100 * time.Millisecond)
	if bytes.Equal(initial, tt.Image().Pix) {
		t.Fatal("changed digit did not turn")
	}
	if !bytes.Equal(unitPixels, overviewRegionPixels(tt.Image(), unit)) || !bytes.Equal(unchangedPixels, overviewRegionPixels(tt.Image(), unchanged)) {
		t.Fatal("number flip animated its unit or an unchanged digit")
	}
	snapshotUI(t, tt, "number-flip-outgoing")
	tt.Advance(140 * time.Millisecond)
	snapshotUI(t, tt, "number-flip-incoming")
	tt.Advance(160 * time.Millisecond)
	if !tt.HasText(value) {
		t.Fatal("number flip did not settle on its target")
	}
	if now, _ := tt.Find("数字单位 capacity1"); now != unit {
		t.Fatal("number unit moved")
	}
	snapshotUI(t, tt, "number-flip-complete")
	// Shrinking values keep the unit anchored at the same right edge.
	value = "3.4 GB"
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	if now, _ := tt.Find("数字单位 capacity1"); now != unit {
		t.Fatal("shorter value shifted the unit")
	}
}

func TestNumberFlipRapidTargetsAndReducedMotion(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	a := &App{}
	value := "93"
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Background(panel).Children(func() {
			a.flipNumber(c, "count", value, 24, 700, ink).Width(180)
		})
	}, 220, 80)
	first, _ := tt.Find("数字位 count-0-0")
	value = "300"
	tt.Frame()
	if now, _ := tt.Find("数字位 count-0-0"); now != first {
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
