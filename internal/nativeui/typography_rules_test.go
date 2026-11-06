package nativeui

import (
	"math"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/i18n"
)

func TestBodyTextKeepsUnitOnSameLine(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	font := ui.Font{Family: uiSansFamily(), Size: 16}
	text := "Download size: 512 MB"
	width, _ := ui.ShapeText("Download size: 512 ", font, 0).Size()
	lastLine := func(text string) string {
		lines := ui.ShapeText(text, font, width).Lines()
		last := lines[len(lines)-1]
		units := utf16.Encode([]rune(text))
		return strings.TrimSpace(string(utf16.Decode(units[last.Start:last.End])))
	}
	if got := lastLine(text); got != "MB" {
		t.Fatalf("fixture must originally leave a lone unit, got %q", got)
	}
	if got := lastLine(i18n.BindTextPairs(text)); got != "512\u00a0MB" {
		t.Fatalf("number and unit separated: %q", got)
	}
}

func TestTextTooltipOnlyForActualOverflow(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	text := "A Minecraft resource pack with a complete title"
	width := float32(140)
	font := ui.Font{Family: uiSansFamily(), Size: 16, Weight: 600}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		ui.Column(c).Fill().Padding(40).Background(panel).Children(func() {
			e := ui.Text(c, text).Label("overflow-label").Width(width).MinWidth(0).SingleLine().Font(font.Family).FontSize(font.Size).FontWeight(font.Weight)
			textTooltip(c, e, text, "", font, 1)
		})
	}, 900, 160)
	tt.Advance(time.Second)
	anchor, _ := tt.Find("overflow-label")
	tt.Move(anchor.X+20, anchor.Y+5)
	tt.Advance(650 * time.Millisecond)
	if !tt.HasText(text) {
		t.Fatal("clipped label did not expose its full text")
	}
	width = 700
	tt.Frame()
	tt.Frame()
	tt.Advance(time.Second)
	if tt.HasText(text) {
		t.Fatal("a fitting label still has a redundant tooltip")
	}
	if tt.FramePending() {
		t.Fatal("overflow measurements kept the idle page repainting")
	}
	// A new, longer value must invalidate the cached measurement at the same
	// width, then shrinking a box must re-enable its hover tooltip.
	text = strings.Repeat(text, 3)
	tt.Frame()
	tt.Frame()
	tt.Move(0, 0)
	tt.Move(anchor.X+20, anchor.Y+5)
	tt.Advance(650 * time.Millisecond)
	if !tt.HasText(text) {
		t.Fatal("text change did not invalidate overflow measurement")
	}
}

func TestChangingDigitsKeepNativeWidth(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	value := "1111"
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		ui.Column(c).Fill().AlignItems(ui.Start).Children(func() { numText(c, value, 20, ink).Label("value") })
	}, 400, 100)
	first, _ := tt.Find("value")
	if first.W <= 0 || first.W >= 100 {
		t.Fatalf("expected intrinsic digit width, got %g", first.W)
	}
	for _, valueToShow := range []string{"8888", "0000", "9999", "2468"} {
		value = valueToShow
		tt.Frame()
		r, _ := tt.Find("value")
		if math.Abs(float64(first.W-r.W)) > .02 {
			t.Fatalf("tabular figure width changed: 1111=%g, %s=%g", first.W, value, r.W)
		}
		t.Logf("%s: %.3f DIP", value, r.W)
	}
}
