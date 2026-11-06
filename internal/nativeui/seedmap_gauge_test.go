package nativeui

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// Check the rendered text bounds, including crowded underground levels and
// shrinking an already visible panel. No game installation is needed.
func TestSeedMapHeightGaugeLabelBounds(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{LangZH, LangEN} {
		for _, scale := range []float32{.65, 1, 1.5} {
			for _, fixture := range []struct {
				name, dim                      string
				surface, bottom, top, selected int
			}{
				{"ancient-city", "minecraft:overworld", 113, -52, -22, -53},
				{"mountain", "minecraft:overworld", 900, 850, 880, 0},
				{"nether", "minecraft:the_nether", 31, 30, 70, 32},
				{"end", "minecraft:the_end", 64, 64, 90, 1},
			} {
				t.Run(fmt.Sprintf("%s/%s/%.2f", lang, fixture.name, scale), func(t *testing.T) {
					a := &App{languageOverride: lang}
					p := &seedMapStructurePreview{terrain: true, ghost: true, diveT: 1,
						dim: fixture.dim, surfaceY: fixture.surface, terrainTop: fixture.surface,
						structMinY: fixture.bottom, structMaxY: fixture.top,
						sel: structure3d.Picked{Index: 1, Y: fixture.selected}}
					height := float32(320)
					tt := ui.NewTester(func(c *ui.Context) {
						c.SetTheme(theme)
						ui.Column(c).Fill().Background(panel).Children(func() {
							ui.Row(c).Absolute().Left(16 * scale).Top(16 * scale).Width(200 * scale).Height(height * scale).
								AlignItems(ui.Stretch).Children(func() { a.seedMapHeightGauge(c, p, scale) })
						})
					}, int(232*scale), int(352*scale))
					tt.SetScale(2)
					selected := fmt.Sprintf(a.L("seedmap.gauge.selected"), p.yApprox(), fixture.selected)
					cut := fmt.Sprintf(a.L("seedmap.gauge.cut"), int(math.Round(p.cutY())))
					for _, size := range []float32{320, 180, 100, 78, 320} {
						height = size
						tt.Frame()
						if !tt.HasText(cut) || !tt.HasText(selected) {
							t.Fatalf("height %.0f: current or selected height missing: %v", size, tt.Texts())
						}
						heading, _ := tt.Find(cut)
						right := heading.X + heading.W + 8*scale
						var rows []ui.Rect
						for _, label := range tt.Texts() {
							r, ok := tt.Find(label)
							if !ok || r.X < 16*scale-.1 || r.X+r.W > right+.1 || r.Y < 16*scale-.1 || r.Y+r.H > (16+height)*scale+.1 {
								t.Fatalf("height %.0f: %q escaped the gauge: %+v", size, label, r)
							}
							rows = append(rows, r)
						}
						sort.Slice(rows, func(i, j int) bool { return rows[i].Y < rows[j].Y })
						for i := 1; i < len(rows); i++ {
							if rows[i].Y < rows[i-1].Y+rows[i-1].H+2*scale-.1 {
								t.Fatalf("height %.0f: label rows overlap: %+v, %+v", size, rows[i-1], rows[i])
							}
						}
						if size == 320 && fixture.dim == "minecraft:overworld" {
							for _, key := range []string{"sea", "deepslate", "lava", "bedrock"} {
								if !tt.HasText(a.L("seedmap.gauge." + key)) {
									t.Fatalf("reference %s missing at full height", key)
								}
							}
						}
						if scale == 1 {
							snapshotUI(t, tt, fmt.Sprintf("height-gauge-%s-%s-%.0f", fixture.name, lang, size))
						}
						if size == 100 && fixture.name == "ancient-city" {
							tt.Move(heading.X+heading.W/2, heading.Y+heading.H/2)
							tt.Advance(time.Second)
							text := strings.Join(tt.Texts(), "\n")
							for _, key := range []string{"sea", "deepslate", "lava", "bedrock"} {
								if !strings.Contains(text, a.L("seedmap.gauge."+key)) {
									t.Fatalf("reference %s missing from the compact gauge tooltip", key)
								}
							}
							if scale == 1 {
								snapshotUI(t, tt, fmt.Sprintf("height-gauge-tooltip-%s", lang))
							}
							tt.Move(0, 0)
							tt.Advance(time.Second)
						}
					}
				})
			}
		}
	}
}
