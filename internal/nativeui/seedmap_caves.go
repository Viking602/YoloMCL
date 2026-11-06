package nativeui

import (
	"math"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// seedMapCaveMaxChunks caps the chunks rasterised per frame; past it the
// overlay asks for a closer zoom instead of stalling the UI.
const seedMapCaveMaxChunks = 48 * 48

type seedMapCaveKey struct {
	cx, cz int32
	y      int
}

type seedMapCaveMask struct{ cave, canyon [16]uint16 }

// seedMapCaves caches carver shapes per source chunk and rasterised masks per
// (chunk, y) for one seed.
type seedMapCaves struct {
	seed   int64
	shapes map[[2]int32][]worldgen.CarveShape
	masks  map[seedMapCaveKey]seedMapCaveMask
}

func (c *seedMapCaves) reset(seed int64) {
	c.seed = seed
	c.shapes = map[[2]int32][]worldgen.CarveShape{}
	c.masks = map[seedMapCaveKey]seedMapCaveMask{}
}

func (c *seedMapCaves) mask(seed int64, cx, cz int32, y int) seedMapCaveMask {
	if c.shapes == nil || c.seed != seed || len(c.shapes) > 200000 || len(c.masks) > 60000 {
		c.reset(seed)
	}
	k := seedMapCaveKey{cx, cz, y}
	if m, ok := c.masks[k]; ok {
		return m
	}
	var m seedMapCaveMask
	m.cave, m.canyon = worldgen.CarvedChunk(func(x, z int32) []worldgen.CarveShape {
		sk := [2]int32{x, z}
		s, ok := c.shapes[sk]
		if !ok {
			s = worldgen.OverworldCarves(seed, x, z)
			c.shapes[sk] = s
		}
		return s
	}, cx, cz, y)
	c.masks[k] = m
	return m
}

// seedMapCavesSupported reports whether vanilla 1.18+ overworld carvers apply.
// The overlay itself only draws on the height slice, not the surface view.
func (v *seedMapView) seedMapCavesSupported() bool {
	if v.snapshot == nil || v.viewport.Dimension != "minecraft:overworld" {
		return false
	}
	parts := strings.Split(v.snapshot.Minecraft, ".")
	if len(parts) < 2 || parts[0] != "1" {
		return false
	}
	minor, err := strconv.Atoi(parts[1])
	return err == nil && minor >= 18
}

// seedMapCaveOverlay paints carver-carved blocks at the slice height. It
// returns false when the view spans too many chunks to draw.
func (a *App) seedMapCaveOverlay(c *ui.Context, v *seedMapView, w, h, units float32) bool {
	y := v.viewport.Y
	x0 := v.camera.x - float64(w/2/units)
	z0 := v.camera.z - float64(h/2/units)
	x1 := v.camera.x + float64(w/2/units)
	z1 := v.camera.z + float64(h/2/units)
	cx0, cz0 := int32(math.Floor(x0/16)), int32(math.Floor(z0/16))
	cx1, cz1 := int32(math.Floor(x1/16)), int32(math.Floor(z1/16))
	if int(cx1-cx0+1)*int(cz1-cz0+1) > seedMapCaveMaxChunks || y < -63 || y > 312 {
		return false
	}
	type run struct {
		x, z, n int
		kind    int
	}
	var runs []run
	for cz := cz0; cz <= cz1; cz++ {
		for cx := cx0; cx <= cx1; cx++ {
			m := v.caves.mask(v.seed, cx, cz, y)
			for lz := 0; lz < 16; lz++ {
				for kind, row := range [2]uint16{m.canyon[lz], m.cave[lz] &^ m.canyon[lz]} {
					for lx := 0; lx < 16; {
						if row&(1<<lx) == 0 {
							lx++
							continue
						}
						start := lx
						for lx < 16 && row&(1<<lx) != 0 {
							lx++
						}
						runs = append(runs, run{int(cx)*16 + start, int(cz)*16 + lz, lx - start, kind})
					}
				}
			}
		}
	}
	caveColor, canyonColor := accent.Alpha(.78), accentViolet.Alpha(.86)
	if y <= worldgen.CarverLavaY {
		caveColor, canyonColor = warning.Alpha(.82), warningInk.Alpha(.86)
	}
	camX, camZ := v.camera.x, v.camera.z
	ui.Box(c.Key("seedmap-caves")).Absolute().Left(0).Top(0).Size(w, h).Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(ui.Rect{X: r.X, Y: r.Y, W: w, H: h}, panel.Alpha(.35), 0)
		for _, rn := range runs {
			col := caveColor
			if rn.kind == 0 {
				col = canyonColor
			}
			// Snap to whole pixels so adjacent rows meet without seams.
			x0 := math.Floor(float64(w/2) + (float64(rn.x)-camX)*float64(units))
			z0 := math.Floor(float64(h/2) + (float64(rn.z)-camZ)*float64(units))
			x1 := math.Floor(float64(w/2) + (float64(rn.x+rn.n)-camX)*float64(units))
			z1 := math.Floor(float64(h/2) + (float64(rn.z+1)-camZ)*float64(units))
			p.Fill(ui.Rect{X: r.X + float32(x0), Y: r.Y + float32(z0), W: float32(max(1, x1-x0)), H: float32(max(1, z1-z0))}, col, 0)
		}
	})
	return true
}

// seedMapCaveBadge explains the overlay: carvers are seed-exact, noise caves
// (cheese, spaghetti, noodle) and aquifer flooding are not drawn.
func (a *App) seedMapCaveBadge(c *ui.Context, v *seedMapView, s float32, drawn bool) {
	ui.Box(c.Key("seedmap-caves-badge")).Absolute().Top(10*s).Left(10*s).Padding(8*s, 8*s).Radius(radiusChip*s).Background(panel.Alpha(.94)).Border(1, accent.Alpha(.3)).Children(func() {
		ui.Column(c).Gap(4 * s).Children(func() {
			ui.Row(c).Gap(8 * s).AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.L("seedmap.caves.title")+"  Y "+strconv.Itoa(v.viewport.Y)).FontSize(typeCaption * s).Bold().TextColor(ink).NoWrap()
				ui.Box(c).Width(10 * s).Height(10 * s).Radius(radiusMin * s).Background(accent)
				ui.Text(c, a.L("seedmap.caves.cave")).FontSize(typeCaption * s).TextColor(ink).NoWrap()
				ui.Box(c).Width(10 * s).Height(10 * s).Radius(radiusMin * s).Background(accentViolet)
				ui.Text(c, a.L("seedmap.caves.canyon")).FontSize(typeCaption * s).TextColor(ink).NoWrap()
				if v.viewport.Y <= worldgen.CarverLavaY {
					ui.Box(c).Width(10 * s).Height(10 * s).Radius(radiusMin * s).Background(warning)
					ui.Text(c, a.L("seedmap.caves.lava")).FontSize(typeCaption * s).TextColor(warningInk).NoWrap()
				}
			})
			ui.Text(c, a.L("seedmap.caves.note")).FontSize(typeCaption * s).TextColor(muted).NoWrap()
			if !drawn {
				ui.Text(c, a.L("seedmap.caves.zoom")).FontSize(typeCaption * s).Bold().TextColor(warningInk).NoWrap()
			}
		})
	})
}
