package nativeui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestTooltipShortLabelsStayOnOneLine(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	for _, window := range [][2]int{{1536, 1024}, {1522, 1017}, {1280, 820}, {900, 600}} {
		for _, scale := range []float32{1, 1.25, 1.5, 2} {
			for _, text := range []string{"Zulu", "Temurin", "21.0.10", "打开文件夹", "Copy path"} {
				t.Run(fmt.Sprintf("%s-%d-scale-%g", text, window[0], scale), func(t *testing.T) {
					var textW, textH float32
					tt := ui.NewTester(func(c *ui.Context) {
						s := uiScale(c)
						localTheme := *theme
						localTheme.FontSize *= s
						localTheme.Spacing *= s
						c.SetTheme(&localTheme)
						textW, textH = c.MeasureText(0, ui.Span{Text: text, Font: localTheme.Font, Size: localTheme.FontSize - 1, Weight: 400})
						ui.Column(c).Fill().Padding(80).Font(uiSansFamily()).Background(panel).Children(func() {
							tooltip(c, ui.Button(c, "Runtime").Label("anchor"), text)
						})
					}, window[0], window[1])
					tt.SetScale(scale)
					anchor, ok := tt.Find("anchor")
					if !ok {
						t.Fatal("missing tooltip anchor")
					}
					tt.Move(anchor.X+anchor.W/2, anchor.Y+anchor.H/2)
					tt.Advance(650 * time.Millisecond)
					r, ok := tt.Find(text)
					snapshotUI(t, tt, fmt.Sprintf("tooltip-%s-%d-%g", text, window[0], scale))
					if !ok || r.H > textH+.5 || r.W+.05 < textW {
						t.Fatalf("short tooltip wrapped or clipped: bounds=%+v, natural size=%gx%g", r, textW, textH)
					}
				})
			}
		}
	}
}

func TestTooltipLongTextAndDismissal(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, text string }{
		{"path", `C:\Program Files\` + strings.Repeat(`A-long-runtime-folder\`, 10) + `bin\java.exe`},
		{"chinese", strings.Repeat("自动选择此版本时优先使用这个环境。", 12)},
		{"line-breaks", "Zulu\n21.0.10\n64-bit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := tc.text
			var naturalW, naturalH float32
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme)
				naturalW, naturalH = ui.ShapeText(text, ui.Font{Family: theme.Font, Size: theme.FontSize - 1, Weight: 400}, 0).Size()
				ui.Column(c).Fill().Background(panel).Padding(12).Children(func() {
					tooltip(c, ui.Button(c, "Runtime").Label("anchor").AlignSelf(ui.End), text)
				})
			}, 360, 600)
			anchor, _ := tt.Find("anchor")
			tt.Move(anchor.X+anchor.W/2, anchor.Y+anchor.H/2)
			tt.Advance(500 * time.Millisecond)
			if tt.HasText(text) {
				t.Fatal("tooltip appeared before its hover delay")
			}
			tt.Advance(150 * time.Millisecond)
			r, ok := tt.Find(text)
			snapshotUI(t, tt, "tooltip-"+tc.name)
			if !ok || r.X < 4 || r.X+r.W > 356 || r.Y < 4 || r.Y+r.H > 596 {
				t.Fatalf("tooltip text escaped the viewport: %+v", r)
			}
			if naturalW > 360 && r.H <= naturalH+.5 {
				t.Fatal("long tooltip was clipped instead of wrapping")
			}
			if strings.Contains(text, "\n") && (r.H > naturalH+.5 || r.H < naturalH-.5) {
				t.Fatalf("explicit line breaks changed: bounds=%+v natural=%gx%g", r, naturalW, naturalH)
			}
			tt.Advance(time.Second)
			if tt.FramePending() {
				t.Fatal("a stationary tooltip keeps redrawing")
			}
			tt.Key(0, ui.KeyEscape)
			tt.Advance(time.Second)
			if tt.HasText(text) {
				t.Fatal("Escape did not dismiss the tooltip")
			}
			tt.Move(4, 590)
			tt.Key(0, ui.KeyTab)
			if !tt.HasText(text) {
				t.Fatal("keyboard focus did not show the tooltip")
			}
		})
	}
}
