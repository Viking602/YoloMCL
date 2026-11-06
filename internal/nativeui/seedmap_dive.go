package nativeui

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// Dive timing. Reduced motion skips straight to the end.
const (
	diveDuration    = 1000 * time.Millisecond
	diveOutDuration = 700 * time.Millisecond
	previewPersp    = 360.0 // eye distance in blocks once a terrain slab is shown
	previewSlab     = 96    // smallest slab width in blocks around the structure
	previewGhost    = 0.16  // opacity of the cut-away ground once arrived
)

// Reference heights on the gauge (1.18+ overworld).
const (
	gaugeSeaLevel = 63
	gaugeLava     = -54
	gaugeBedrock  = -64
	gaugeWidth    = 100 // minimum width; translated labels can widen the column
)

// camera is the view the preview renders now: the dive camera while diving,
// else the orbit camera, both cut just above the structure once a terrain
// slab is shown.
func (p *seedMapStructurePreview) camera() structure3d.Camera {
	cam := structure3d.Camera{Yaw: p.yaw, Pitch: p.pitch, Zoom: p.zoom, PanX: p.panX, PanY: p.panY,
		Aimed: p.aimed, Aim: p.aim, Boxes: p.boxes, Select: p.sel.Index}
	if !p.terrain {
		return cam
	}
	cam.Persp = previewPersp
	// Solid terrain (地形虚影 off) keeps the whole slab; ghost mode cuts it
	// down to the structure and draws the removed ground faintly.
	cam.Cut, cam.CutY = p.ghost || p.xray, p.finalCutY()
	if !cam.Cut && !p.onSurface() {
		// solid mode, buried structure (fortress, bastion, stronghold): the
		// structure is the focus, so the rock (and the Nether roof) above its
		// top comes off; the ground stays solid around and below it
		cam.Cut, cam.CutY = true, float64(min(p.structMaxY+1, p.terrainTop+2))
	}
	cam.HideTerrain = p.xray
	if p.ghost && !p.xray {
		// the ground the cut takes away fades to a faint ghost as it sinks
		k := math.Max(0, math.Min(1, (p.diveT-0.45)/0.4))
		cam.Ghost = previewGhost * k * k * (3 - 2*k)
	}
	if p.diveT < 1 {
		cam = structure3d.DiveCamera(p.diveT, cam, float64(p.terrainTop), cam.CutY)
	}
	// The renderer's depth order puts a positive pitch below the model
	// (the open cutaway look of the plain preview). The slab must be seen
	// from above, so its views use the mirrored pitch.
	cam.Pitch = -cam.Pitch
	return cam
}

// finalCutY is where the dive ends: the ground is cut down to the
// structure's floor, so nothing of the terrain covers the structure. Only a
// cross-section below it stays as context.
func (p *seedMapStructurePreview) finalCutY() float64 {
	if p.onSurface() && !p.ghost {
		// a surface structure stands on the ground: nothing to cut away
		return float64(p.terrainTop + 2)
	}
	return float64(p.structMinY)
}

// onSurface reports a structure built on the overworld or End ground (villages,
// temples, outposts): its floor sits within a few blocks of the surface.
func (p *seedMapStructurePreview) onSurface() bool {
	return (p.dim == "minecraft:overworld" || p.dim == "minecraft:the_end" && p.surfaceY > 0) && p.structMinY >= p.surfaceY-6
}

// cutY is the cut-plane height now, for the gauge needle.
func (p *seedMapStructurePreview) cutY() float64 {
	return structure3d.DiveCutY(p.diveT, float64(p.terrainTop), p.finalCutY())
}

// slabFitZoom frames a slab of the given width in the full-size render,
// with room for the near edge that perspective enlarges.
func slabFitZoom(slab int) float64 {
	return float64(structurePreviewFullW) / (float64(slab) * 1.6)
}

// previewSlabSize covers the structure footprint plus a margin of ground.
func previewSlabSize(m *structure3d.Mesh) int {
	n := int(math.Ceil(m.Extent()))*2 + 32
	n = max(previewSlab, min(n, 320))
	return n + n%2
}

func (p *seedMapStructurePreview) startDive(out bool) {
	p.diving, p.diveOut, p.diveFrom = true, out, time.Time{}
	if !out {
		p.diveT = 0
	}
}

// stepDive advances the dive by the clock; slow frames are skipped, not
// slowed down.
func (a *App) stepDive(c *ui.Context, v *seedMapView, p *seedMapStructurePreview) {
	now := c.Now()
	if p.diveFrom.IsZero() {
		p.diveFrom = now
	}
	if c.Preferences().ReduceMotion {
		p.diving = false
		if p.diveOut {
			v.structurePreview = nil
			v.canvasFocus.Focus(c)
			return
		}
		p.diveT = 1
		a.rerenderStructurePreview(v)
		return
	}
	el := now.Sub(p.diveFrom)
	if p.diveOut {
		p.diveT = 1 - float64(el)/float64(diveOutDuration)
		if p.diveT <= 0 {
			v.structurePreview = nil
			v.canvasFocus.Focus(c)
			return
		}
	} else {
		p.diveT = float64(el) / float64(diveDuration)
		if p.diveT >= 1 {
			p.diveT, p.diving = 1, false
		}
	}
	if p.diveT != p.drawnT { // the clock moved: draw the new step
		p.drawnT = p.diveT
		a.rerenderStructurePreview(v)
	}
	if p.diving {
		c.AnimationFrame()
	}
}

// closeStructurePreview plays the dive backwards when a slab is shown and
// reports whether the panel is still open.
func (a *App) closeStructurePreview(c *ui.Context, v *seedMapView) bool {
	p := v.structurePreview
	if p != nil && p.cancel != nil {
		p.cancel()
	}
	if p == nil || !p.terrain || p.mesh == nil || c.Preferences().ReduceMotion || p.diveOut {
		v.structurePreview = nil
		v.canvasFocus.Focus(c)
		return false
	}
	p.startDive(true)
	p.diveOut = true
	c.AnimationFrame()
	return true
}

// addPreviewGround puts the seed's ground around the structure: surface
// heights and biome colours as on the map, and rock, carver caves and ores
// in every cross-section.
func addPreviewGround(m *structure3d.Mesh, surface structure3d.HeightFunc, seed int64, dim string, sampler *launcher.LocalSampler, caves bool, version string, store *structure3d.Store) {
	addPreviewGroundContext(context.Background(), m, surface, seed, dim, sampler, caves, version, store)
}

func addPreviewGroundContext(ctx context.Context, m *structure3d.Mesh, surface structure3d.HeightFunc, seed int64, dim string, sampler *launcher.LocalSampler, caves bool, version string, store *structure3d.Store) {
	if ctx.Err() != nil {
		return
	}
	if sampler == nil {
		addPreviewTerrain(m, surface)
		return
	}
	cx, cz := int(math.Round(m.Center[0])), int(math.Round(m.Center[2]))
	bottom := max(min(m.MinY-8, surface(cx, cz)-40), gaugeBedrock)
	slab := previewSlabSize(m)
	// step 1 keeps the surface relief as close to the 2D map as the face
	// budget allows; bigger slabs step by 2 (never 4 — that flattened peaks).
	step := 1
	if slab > 160 {
		step = 2
	}
	x0, z0 := cx-slab/2, cz-slab/2
	g := newPreviewGroundContext(ctx, seed, dim, sampler, surface, step, caves, version, x0, z0, slab, slab, store)
	if ctx.Err() != nil {
		return
	}
	switch dim {
	case "minecraft:the_end":
		m.AddTerrain(structure3d.Terrain{
			X0: x0, Z0: z0, W: slab, D: slab, Step: step, Bottom: 0,
			Height: g.EndHeight, Floor: g.EndFloor,
			Color:   func(_, _, _ int) [3]uint8 { return groundEndStone },
			Section: g.Section, YStep: max(step, 2), Textures: g.textures(),
		})
		return
	case "minecraft:the_nether":
		m.CarveThrough = true // fortresses sit inside the rock
		if g.noise {
			// final_density relief: the floor (lava sea included) and the
			// ceiling as a floating layer from its first solid block.
			m.AddTerrain(structure3d.Terrain{
				X0: x0, Z0: z0, W: slab, D: slab, Step: step, Bottom: 0,
				Height:  g.NetherFloor,
				Color:   g.NetherNoiseTop,
				Section: g.Section, YStep: max(step, 2), Textures: g.textures(),
			})
			m.AddTerrain(structure3d.Terrain{
				X0: x0, Z0: z0, W: slab, D: slab, Step: step, Bottom: netherCeiling,
				Height:  g.NetherCeilingTop,
				Floor:   g.NetherCeiling,
				Color:   g.NetherNoiseTop,
				Section: g.Section, YStep: max(step, 2), Textures: g.textures(),
			})
			return
		}
		// floor up to the lava sea, then the ceiling as a floating layer
		m.AddTerrain(structure3d.Terrain{
			X0: x0, Z0: z0, W: slab, D: slab, Step: step, Bottom: 0,
			Height:  func(_, _ int) int { return netherLava },
			Color:   g.NetherTop,
			Section: g.Section, YStep: max(step, 2), Textures: g.textures(),
		})
		m.AddTerrain(structure3d.Terrain{
			X0: x0, Z0: z0, W: slab, D: slab, Step: step, Bottom: netherCeiling,
			Height:  func(_, _ int) int { return netherTop },
			Floor:   func(_, _ int) int { return netherCeiling },
			Color:   g.NetherTop,
			Section: g.Section, YStep: max(step, 2), Textures: g.textures(),
		})
		return
	}
	height := structure3d.HeightFunc(func(x, z int) int { return surface(x, z) })
	if g.noise {
		height = g.WaterTop // seas, lakes and aquifers from the noise stage
	}
	m.AddTerrain(structure3d.Terrain{
		X0: x0, Z0: z0, W: slab, D: slab,
		Step: step, Bottom: bottom,
		Height:   height,
		Color:    g.Top,
		Section:  g.Section,
		YStep:    max(step, 2),
		Textures: g.textures(),
	})
}

// addPreviewTerrain puts an illustrative ground slab around the structure:
// heights from the map's surface sampler, colours by height band.
func addPreviewTerrain(m *structure3d.Mesh, surface structure3d.HeightFunc) {
	cx, cz := int(math.Round(m.Center[0])), int(math.Round(m.Center[2]))
	bottom := min(m.MinY-8, surface(cx, cz)-40)
	bottom = max(bottom, gaugeBedrock)
	slab := previewSlabSize(m)
	step := 2
	if slab > 192 {
		step = 4
	}
	m.AddTerrain(structure3d.Terrain{
		X0: cx - slab/2, Z0: cz - slab/2, W: slab, D: slab,
		Step: step, Bottom: bottom,
		Height: func(x, z int) int { return surface(x, z) },
		Color: func(_, _, h int) [3]uint8 {
			switch {
			case h < gaugeSeaLevel:
				return [3]uint8{0x3f, 0x76, 0xe4}
			case h <= gaugeSeaLevel+2:
				return [3]uint8{0xdb, 0xd3, 0xa0}
			case h > 150:
				return [3]uint8{0xf2, 0xf4, 0xf6}
			case h > 110:
				return [3]uint8{0x8a, 0x8a, 0x8a}
			}
			return [3]uint8{0x6d, 0x9a, 0x4a}
		},
	})
}

// structureYText is the structure's Y range; ≈ unless the blocks themselves
// are seed-exact.
func (a *App) structureYText(p *seedMapStructurePreview) string {
	approx := "≈"
	if p.seeded {
		approx = ""
	}
	if p.structMinY == p.structMaxY {
		return fmt.Sprintf("Y %s%d", approx, p.structMinY)
	}
	return fmt.Sprintf("Y %s%d…%d", approx, p.structMinY, p.structMaxY)
}

// seedMapHeightGauge is the altimeter on the right edge of the 3D view: the
// cut plane's Y (needle) against surface, sea level, Y 0 (deepslate),
// lava level, bedrock and the structure's Y range.
func (a *App) seedMapHeightGauge(c *ui.Context, p *seedMapStructurePreview, s float32) {
	top := float64(max(p.terrainTop, p.structMaxY, p.surfaceY) + 12)
	bot := float64(gaugeBedrock)
	switch p.dim {
	case "minecraft:the_nether":
		top, bot = netherTop+4, 0
	case "minecraft:the_end":
		top, bot = float64(max(p.terrainTop, p.structMaxY)+12), 0
	}
	cy := p.cutY()
	type mark struct {
		key, label string
		y          float64
		col        ui.Color
		at         float32 // label row top, after collision avoidance
		shown      bool
	}
	var marks []mark
	switch p.dim {
	case "minecraft:the_nether":
		// vanilla nether: lava sea Y 31, bedrock floor Y 0..4, roof Y 123..127
		marks = []mark{
			{key: "roof", y: netherTop, label: a.L("seedmap.gauge.nether_roof"), col: muted},
			{key: "lava", y: netherLava, label: a.L("seedmap.gauge.nether_lava"), col: warningInk},
			{key: "bedrock", y: 0, label: a.L("seedmap.gauge.nether_floor"), col: muted},
		}
	case "minecraft:the_end":
		marks = []mark{{key: "void", y: 0, label: a.L("seedmap.gauge.end_void"), col: muted}}
		if p.surfaceY > 0 {
			marks = append(marks, mark{key: "surface", y: float64(p.surfaceY), label: fmt.Sprintf(a.L("seedmap.gauge.end_island"), p.surfaceY), col: inkSecondary})
		}
	default:
		marks = []mark{{key: "surface", y: float64(p.surfaceY), label: fmt.Sprintf(a.L("seedmap.gauge.surface"), p.surfaceY), col: inkSecondary}}
		if math.Abs(float64(p.surfaceY-gaugeSeaLevel)) > 3 {
			marks = append(marks, mark{key: "sea", y: gaugeSeaLevel, label: a.L("seedmap.gauge.sea"), col: muted})
		}
		marks = append(marks,
			mark{key: "zero", y: 0, label: a.L("seedmap.gauge.deepslate"), col: muted},
			mark{key: "lava", y: gaugeLava, label: a.L("seedmap.gauge.lava"), col: warningInk},
			mark{key: "bedrock", y: gaugeBedrock, label: a.L("seedmap.gauge.bedrock"), col: muted})
	}
	if p.sel.Index != 0 {
		marks = append(marks, mark{key: "selected", y: float64(p.sel.Y), label: fmt.Sprintf(a.L("seedmap.gauge.selected"), p.yApprox(), p.sel.Y), col: accent})
	}
	sort.SliceStable(marks, func(i, j int) bool { return marks[i].y > marks[j].y })
	cutLabel := fmt.Sprintf(a.L("seedmap.gauge.cut"), int(math.Round(cy)))
	cutW, _ := c.MeasureText(0, ui.Span{Text: cutLabel, Font: uiSansFamily(), Size: typeCaption * s, Weight: 700})
	w, rowH := max(gaugeWidth*s, cutW+24*s), 18*s
	for _, m := range marks {
		mw, mh := c.MeasureText(0, ui.Span{Text: m.label, Font: uiSansFamily(), Size: typeCaption * s})
		w, rowH = max(w, mw+36*s), max(rowH, mh)
		top, bot = max(top, m.y), min(bot, m.y)
	}
	top, bot = max(top, float64(p.structMaxY)), min(bot, float64(p.structMinY))
	box := ui.Box(c.Key("seedmap-gauge")).Width(w).Shrink(0).MinHeight(0).Clip().
		Radius(nestedRadius(radiusChrome, previewPad) * s).Background(ui.Hex("#f4f6f8"))
	measuredH := box.LayoutBounds().H
	box.Draw(func(_ *ui.Painter, r ui.Rect) {
		// LayoutBounds is from the previous frame. Settle a new or resized
		// gauge once more even when the camera and pointer have stopped.
		if math.Abs(float64(r.H-measuredH)) > .1 {
			c.AnimationFrame()
		}
	})
	// Keep both endpoint labels inside the card. A small panel uses fewer
	// labels, never a fictitious rail height that spills into the footer.
	head, pad := 40*s, space3*s
	// In a short viewport, reduce bottom breathing room in grid steps before
	// sacrificing the selected-height row. Both labels must remain reachable.
	for pad > space1*s && measuredH-head-pad < rowH {
		pad -= space1 * s
	}
	gh := max(0, measuredH-head-pad)
	railH := max(0, gh-rowH)
	yOf := func(y float64) float32 {
		return rowH/2 + railH*float32(math.Max(0, math.Min(1, (top-y)/(top-bot))))
	}
	// When resized very short, keep the selected height and the major
	// boundaries first. All reference ticks remain at their real heights.
	order := make([]int, len(marks))
	priority := func(key string) int {
		switch key {
		case "selected":
			return 3
		case "surface", "roof", "bedrock", "void":
			return 2
		case "lava":
			return 1
		}
		return 0
	}
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		left, right := marks[order[i]], marks[order[j]]
		if lp, rp := priority(left.key), priority(right.key); lp != rp {
			return lp > rp
		}
		return math.Abs(left.y-cy) < math.Abs(right.y-cy)
	})
	capacity := min(len(marks), int((gh+4*s)/(rowH+4*s)))
	for _, i := range order[:capacity] {
		marks[i].shown = true
	}
	visible := make([]int, 0, capacity)
	for i, m := range marks {
		if m.shown {
			visible = append(visible, i)
		}
	}
	placeGaugeLabels(len(visible), func(i int) float32 { return yOf(marks[visible[i]].y) - rowH/2 },
		func(i int, at float32) { marks[visible[i]].at = at }, rowH+4*s, 0, gh-rowH)
	if capacity < len(marks) {
		labels := make([]string, 0, len(marks))
		for _, m := range marks {
			labels = append(labels, m.label)
		}
		tooltip(c, box, strings.Join(labels, "\n"))
	}
	box.Children(func() {
		ui.Text(c, cutLabel).Absolute().Left(8 * s).Top(8 * s).Width(w - 16*s).Height(24 * s).
			Font(uiSansFamily()).FontFeatures("tnum").FontSize(typeCaption * s).Bold().FixedLineHeight(24 * s).TextAlign(ui.Center).TextColor(accent).
			Radius(radiusChip * s).Background(accent.Alpha(.09)).NoWrap()
		if gh < rowH {
			return
		}
		ui.Box(c.Key("seedmap-gauge-body")).Absolute().Left(0).Top(head).Width(w).Height(gh).Draw(func(pen *ui.Painter, r ui.Rect) {
			for _, m := range marks {
				y := r.Y + yOf(m.y)
				pen.Line(r.X+w-17*s, y, r.X+w-10*s, y, s, m.col.Alpha(.65))
				if m.shown {
					// A displaced label connects back to its true tick, so close
					// layers (lava and bedrock) remain readable without distorting Y.
					pen.Line(r.X+w-25*s, r.Y+m.at+rowH/2, r.X+w-17*s, y, s, m.col.Alpha(.35))
				}
			}
		}).Children(func() {
			gw := 4 * s
			ui.Box(c.Key("seedmap-gauge-rail")).Absolute().Right(10 * s).Top(rowH / 2).Width(gw).Height(railH).Radius(radiusPill).Background(line).Children(func() {
				lo, hi := yOf(float64(p.structMaxY)), yOf(float64(p.structMinY))
				rangeH := min(railH, max(hi-lo, 3*s))
				rangeY := min(lo-rowH/2, railH-rangeH)
				ui.Box(c.Key("seedmap-gauge-struct")).Absolute().Left(0).Top(rangeY).Width(gw).Height(rangeH).Radius(radiusPill).Background(accent.Alpha(.65))
			})
			for _, m := range marks {
				if m.shown {
					ui.Text(c.Key("seedmap-gauge-label-"+m.key), m.label).Absolute().Left(8 * s).Top(m.at).Width(w - 36*s).Height(rowH).
						Font(uiSansFamily()).FontFeatures("tnum").FontSize(typeCaption * s).FixedLineHeight(rowH).TextColor(m.col).TextAlign(ui.End).NoWrap()
				}
			}
			needle := yOf(cy)
			ui.Box(c.Key("seedmap-gauge-needle")).Absolute().Right(5*s).Top(needle-3*s).Size(14*s, 6*s).Radius(radiusPill).Background(accent)
		})
	})
}

// placeGaugeLabels spreads label rows (in order, top to bottom) at least
// gap apart, as near their wanted positions as the room allows.
func placeGaugeLabels(n int, want func(int) float32, set func(int, float32), gap, lo, hi float32) {
	at := make([]float32, n)
	for i := range at {
		at[i] = max(want(i), lo)
		if i > 0 {
			at[i] = max(at[i], at[i-1]+gap)
		}
	}
	for i := n - 1; i >= 0; i-- {
		limit := hi
		if i < n-1 {
			limit = at[i+1] - gap
		}
		at[i] = min(at[i], limit)
	}
	for i := range at {
		set(i, at[i])
	}
}
