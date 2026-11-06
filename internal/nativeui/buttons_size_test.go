package nativeui

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Buttons render at a size token whatever height a caller passes: sm 28,
// md 32, lg 36, icon buttons square, everything on the 4 px grid.
func TestButtonSizeTokens(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	for _, bs := range buttonSizes {
		if int(bs.height)%4 != 0 || int(bs.pad)%4 != 0 || int(bs.gap)%4 != 0 {
			t.Fatalf("off the 4 px grid: %+v", bs)
		}
	}
	cases := []struct{ pass, want float32 }{{26, 28}, {28, 28}, {30, 28}, {32, 32}, {40, 36}, {btnLg, 36}}
	for _, tc := range cases {
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme)
			ui.Row(c).Padding(20).Gap(20).Children(func() {
				fixedButton(c, "text", "Backups", "folder", 0, tc.pass*uiScale(c), false, false)
				iconOnlyButton(c, "icon", "cross", "Close", tc.pass*uiScale(c), false)
			})
		}, 1536, 1024)
		tt.Advance(0)
		text, icon := addonFilterBounds(t, tt, "Backups"), addonFilterBounds(t, tt, "Close")
		if math.Abs(float64(text.H-tc.want)) > .01 || math.Abs(float64(icon.H-tc.want)) > .01 || icon.W != icon.H {
			t.Fatalf("pass %.0f: text %+v icon %+v, want height %.0f", tc.pass, text, icon, tc.want)
		}
	}
}
