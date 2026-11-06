package nativeui

import (
	"fmt"
	"image"
	"math"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// Exercise the actual panel, including wrapping, scrolling and escaping the
// map's clip when expanded. This fixture does not need a game installation.
func TestSeedMapPreviewInformationLayout(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{LangZH, LangEN} {
		for _, dpi := range []float32{1, 2} {
			for _, size := range [][2]float32{{520, 440}, {640, 560}, {340, 320}} {
				t.Run(fmt.Sprintf("%s/%.0fx/%.0f", lang, dpi, size[0]), func(t *testing.T) {
					p := &seedMapStructurePreview{title: "minecraft:bastion_remnant", seeded: true, terrain: true, noiseTerrain: true,
						dim: "minecraft:the_nether", surfaceY: 31, terrainTop: 127, structMinY: 32, structMaxY: 87, blocks: 29863,
						diveT: 1, yaw: .6, pitch: .55, zoom: 14, w: size[0], h: size[1],
						mesh: &structure3d.Mesh{MinY: 32, MaxY: 87}, bitmap: ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 720, 540)))}
					if size[0] == 340 {
						p.title = "example:very_long_modded_structure_identifier_for_wrapping"
					}
					v := &seedMapView{structurePreview: p}
					a := &App{languageOverride: lang, seedMap: v}
					var measure func(string, float32, bool) float32
					tt := ui.NewTester(func(c *ui.Context) {
						c.SetTheme(theme)
						measure = func(text string, w float32, mono bool) float32 {
							font := uiSansFamily()
							if mono {
								font = theme.Font
							}
							_, h := c.MeasureText(w, ui.Span{Text: text, Font: font, Size: typeCaption})
							return h
						}
						ui.Column(c).Fill().Background(bg).Children(func() {
							ui.Box(c).Absolute().Left(24).Top(80).Size(800, 650).Clip().Children(func() {
								a.seedMapStructurePreviewFloat(c, v, 800, 650)
							})
						})
					}, 1536, 1024)
					tt.SetScale(dpi)
					tt.SetPreferences(ui.Preferences{ReduceMotion: true})
					tt.Frame()
					snapshotUI(t, tt, fmt.Sprintf("preview-initial-%s-%.0f-%.0fx", lang, size[0], dpi))
					find := func(text string) ui.Rect {
						t.Helper()
						r, ok := tt.Find(text)
						if !ok || r.W <= 0 || r.H <= 0 {
							t.Fatalf("missing %q: %+v", text, r)
						}
						return r
					}
					checkText := func(text string, mono bool) {
						t.Helper()
						r := find(text)
						panel := find(a.L("seedmap.structure_preview.window"))
						if r.X < panel.X || r.X+r.W > panel.X+panel.W || r.H+.5 < measure(text, r.W, mono) {
							t.Fatalf("text truncated or outside panel: %q, rect=%+v, measured height=%.1f", text, r, measure(text, r.W, mono))
						}
					}
					checkText(p.title, true)
					model := find(a.L("seedmap.structure_preview.model"))
					if size[0] == 340 {
						viewport := find(a.L("seedmap.structure_preview.content"))
						// Scroll the gutter, leaving wheel-over-model available for zoom.
						tt.Scroll(viewport.X+viewport.W-4, viewport.Y+viewport.H/2, 0, 10000)
						tt.Advance(300 * time.Millisecond)
						info := find(a.L("seedmap.structure_preview.information"))
						if p.scroll.st.Y <= 0 || info.Y < viewport.Y-.5 || info.Y+info.H > viewport.Y+viewport.H+.5 {
							t.Fatalf("compact panel information is unreachable: info=%+v viewport=%+v scroll=%+v", info, viewport, p.scroll.st)
						}
					}
					for _, key := range []string{"hint_click", "hint_rotate", "hint_zoom", "hint_pan", "seeded"} {
						checkText(a.L("seedmap.structure_preview."+key), false)
					}
					checkText("≈ "+a.L("seedmap.terrain.noise_nether"), false)
					checkText(a.structureYText(p), false)
					checkText(fmt.Sprintf(a.L("seedmap.structure_preview.blocks"), p.blocks), false)
					hints := find(a.L("seedmap.structure_preview.controls"))
					info := find(a.L("seedmap.structure_preview.information"))
					if hints.Y < model.Y+model.H-p.scroll.st.Y-.5 || info.Y < hints.Y+hints.H {
						t.Fatalf("information overlaps image or hints: model=%+v hints=%+v info=%+v", model, hints, info)
					}
					if size[0] >= 520 && p.scroll.st.MaxY > .5 {
						t.Fatalf("normal panel should show all summaries without scrolling: %.1f overflow", p.scroll.st.MaxY)
					}
					snapshotUI(t, tt, fmt.Sprintf("preview-info-%s-%.0f-%.0fx", lang, size[0], dpi))
					original := find(a.L("seedmap.structure_preview.window"))
					clickUI(t, tt, a.L("seedmap.structure_preview.expand"))
					tt.Frame()
					expanded := find(a.L("seedmap.structure_preview.window"))
					if !p.expanded || expanded.W <= original.W || expanded.H <= original.H || expanded.X < 0 || expanded.Y < 42 || expanded.X+expanded.W > 1536 || expanded.Y+expanded.H > 1024 || p.scroll.st.MaxY > .5 {
						t.Fatalf("expanded preview did not escape the map or fit the window: %+v scroll=%+v", expanded, p.scroll.st)
					}
					clickUI(t, tt, a.L("seedmap.structure_preview.seeded"))
					tt.Frame()
					if !p.detailsOpen {
						t.Fatal("provenance explanation did not open")
					}
					checkText(a.L("seedmap.structure_preview.seeded_detail"), false)
					snapshotUI(t, tt, fmt.Sprintf("preview-info-expanded-%s-%.0fx", lang, dpi))
					clickUI(t, tt, a.L("seedmap.structure_preview.seeded"))
					tt.Key(0, ui.KeyEscape)
					tt.Frame()
					if p.expanded || v.structurePreview != p || find(a.L("seedmap.structure_preview.window")) != original {
						t.Fatal("Escape must restore the floating size without closing the preview")
					}
					if tt.FramePending() {
						tt.Advance(time.Second)
						if tt.FramePending() {
							t.Fatal("settled preview keeps requesting frames")
						}
					}
					clickUI(t, tt, a.L("seedmap.structure_preview.expand"))
					clickUI(t, tt, a.L("seedmap.structure_preview.close"))
					tt.Frame()
					if _, ok := tt.Find(a.L("seedmap.structure_preview.window")); v.structurePreview != nil || ok {
						t.Fatal("closing expanded preview left the overlay open")
					}
				})
			}
		}
	}
}

func TestSeedMapBastionPreviewLayoutShots(t *testing.T) {
	const jar = `D:\MC\.minecraft\versions\1.21.1\1.21.1.jar`
	if _, err := os.Stat(jar); err != nil {
		t.Skip("live 1.21.1 game jar required")
	}
	a, tt, saves := worldsTester(t)
	copyLiveJar(t, jar, saves)
	a.OverrideLanguage(LangZH)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tab := a.L("instance.tab_prefix") + " " + a.L("seedmap.open")
	waitUI(t, tt, func() bool { _, ok := tt.Find(tab); return ok })
	clickUI(t, tt, tab)
	waitUI(t, tt, func() bool { return a.seedMap != nil && a.seedMap.mapReady() && !a.seedMap.pending })
	v := a.seedMap
	tt.SetSize(1440, 900)
	a.seedMapDimension(v, "minecraft:the_nether")
	waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport.Dimension == "minecraft:the_nether" })
	checkNetherCarverBounds(t, v.sampler, v.seed)
	v.selected = &launcher.FeatureHit{ID: "minecraft:bastion_remnant", X: 0, Z: 0, Confidence: "biome_candidate"}
	a.openStructurePreview(v)
	p := v.structurePreview
	waitUI(t, tt, func() bool { return p.bitmap != nil && p.mesh != nil })
	initial := p.mesh
	if !p.loading || initial.HasTerrain {
		t.Fatal("structure was not shown before its terrain")
	}
	snapshotUI(t, tt, "preview-bastion-loading")
	p.yaw, p.panX = .7, 5
	// Building the Nether noise slab can exceed the usual 5-second UI wait.
	deadline := time.Now().Add(30 * time.Second)
	for p.loading && time.Now().Before(deadline) {
		tt.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	if p.mesh == nil || p.err != "" || p.unsupported != "" {
		t.Fatalf("bastion preview failed: err=%q unsupported=%q", p.err, p.unsupported)
	}
	if p.mesh == initial || initial.HasTerrain || !p.mesh.HasTerrain || p.yaw != .7 || p.panX != 5 {
		t.Fatal("terrain completion mutated the first mesh or reset navigation")
	}
	waitUI(t, tt, func() bool { return !p.rendering && !p.dirty && !p.diving })
	tt.Advance(300 * time.Millisecond)
	snapshotUI(t, tt, "preview-bastion")
	clickUI(t, tt, a.L("seedmap.structure_preview.expand"))
	tt.Advance(300 * time.Millisecond)
	if !p.expanded || p.scroll.st.MaxY > .5 {
		t.Fatal("expanded bastion still clips information", p.scroll.st)
	}
	snapshotUI(t, tt, "preview-bastion-expanded")
	clickUI(t, tt, a.L("seedmap.structure_preview.restore"))
	tt.Advance(300 * time.Millisecond)
	if p.expanded || math.Abs(float64(p.scroll.st.Y)) > .5 {
		t.Fatal("restore did not return to the floating panel")
	}
	// A scrollable panel must leave wheel-over-model to the orbit controls.
	p.h = 280
	tt.Frame()
	stage, ok := tt.Find(a.L("seedmap.structure_preview.model"))
	if !ok || p.scroll.st.MaxY <= 0 {
		t.Fatal("compact fixture did not make the panel scrollable")
	}
	zoom, y := p.zoom, p.scroll.st.Y
	tt.Scroll(stage.X+stage.W/2, stage.Y+stage.H/2, 0, 40)
	tt.Frame()
	if p.zoom >= zoom || p.scroll.st.Y != y {
		t.Fatalf("model wheel did not zoom independently: zoom=%v scroll=%v", p.zoom, p.scroll.st)
	}
	waitUI(t, tt, func() bool { return !p.rendering && !p.dirty })
	if w := p.frames[1-p.back].Image.Rect.Dx(); w > 360 {
		t.Fatal("wheel did not use an interactive frame", w)
	}
	tt.Advance(200 * time.Millisecond)
	waitUI(t, tt, func() bool { return !p.rendering && !p.dirty })
	if w := p.frames[1-p.back].Image.Rect.Dx(); w != structurePreviewFullW {
		t.Fatal("wheel stop did not restore full detail", w)
	}
}
