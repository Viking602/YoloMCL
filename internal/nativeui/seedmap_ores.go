package nativeui

import (
	"math"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// seedMapOreMaxChunks caps the chunks decorated per frame.
const seedMapOreMaxChunks = 32 * 32

// seedMapOreColors are mineral swatches; they are data colours, not chrome.
var seedMapOreColors = map[string]ui.Color{
	"coal":     ui.Hex("#3b3f45"),
	"iron":     ui.Hex("#d8a27a"),
	"copper":   ui.Hex("#e0703c"),
	"gold":     ui.Hex("#f2c230"),
	"redstone": ui.Hex("#e0302c"),
	"lapis":    ui.Hex("#2f5fd0"),
	"diamond":  ui.Hex("#3fe0d8"),
	"emerald":  ui.Hex("#2fc45a"),
}

var seedMapOreOrder = []string{"diamond", "emerald", "gold", "redstone", "lapis", "iron", "copper", "coal"}

// seedMapOres caches the ore cells each source chunk places for one seed.
type seedMapOres struct {
	seed      int64
	version   string
	data      *worldgen.OreData
	dataExact bool
	err       error
	cells     map[[2]int32][]worldgen.OreCell
}

func (o *seedMapOres) load(seed int64, version string) {
	if o.data != nil && o.version == version && o.seed == seed && len(o.cells) < 20000 {
		return
	}
	if o.version != version || (o.data == nil && o.err == nil) {
		o.data, o.dataExact, o.err = worldgen.LoadOreData(version)
	}
	o.seed, o.version = seed, version
	o.cells = map[[2]int32][]worldgen.OreCell{}
}

// seedMapOreOverlay paints ore blobs crossing the slice Y. Biomes come from
// the rendered frame at the slice height, so biome-limited ores are
// approximate near biome borders.
func (a *App) seedMapOreOverlay(c *ui.Context, v *seedMapView, w, h, units float32) bool {
	v.ores.load(v.seed, v.snapshot.Minecraft)
	if v.ores.data == nil {
		return false
	}
	y := v.viewport.Y
	x0 := v.camera.x - float64(w/2/units)
	z0 := v.camera.z - float64(h/2/units)
	x1 := v.camera.x + float64(w/2/units)
	z1 := v.camera.z + float64(h/2/units)
	cx0, cz0 := int32(math.Floor(x0/16))-1, int32(math.Floor(z0/16))-1
	cx1, cz1 := int32(math.Floor(x1/16))+1, int32(math.Floor(z1/16))+1
	if int(cx1-cx0+1)*int(cz1-cz0+1) > seedMapOreMaxChunks || y < -64 || y > 319 {
		return false
	}
	frame := v.frame
	ox, oz := frame.Viewport.Origin()
	step := float64(max(1, frame.Viewport.BlocksPerPixel))
	world := &worldgen.OreWorld{Biome: func(bx, _, bz int) string {
		if p, ok := frame.PointAt(int(math.Floor((float64(bx)-float64(ox))/step)), int(math.Floor((float64(bz)-float64(oz))/step))); ok {
			return p.Biome.ID
		}
		return "minecraft:plains"
	}}
	type cell struct {
		x, z  int
		ore   string
		exact bool
	}
	var cells []cell
	for cz := cz0; cz <= cz1; cz++ {
		for cx := cx0; cx <= cx1; cx++ {
			k := [2]int32{cx, cz}
			list, ok := v.ores.cells[k]
			if !ok {
				worldgen.ChunkOres(v.seed, cx, cz, v.ores.data.Features, world, func(oc worldgen.OreCell) { list = append(list, oc) })
				v.ores.cells[k] = list
			}
			for _, oc := range list {
				if oc.Y == y && (v.oreFilter == "" || v.oreFilter == oc.Feature.Ore) {
					cells = append(cells, cell{oc.X, oc.Z, oc.Feature.Ore, oc.Feature.Exact()})
				}
			}
		}
	}
	camX, camZ := v.camera.x, v.camera.z
	ui.Box(c.Key("seedmap-ores")).Absolute().Left(0).Top(0).Size(w, h).Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(ui.Rect{X: r.X, Y: r.Y, W: w, H: h}, ink.Alpha(.45), 0)
		for _, ce := range cells {
			col, ok := seedMapOreColors[ce.ore]
			if !ok {
				col = muted
			}
			if !ce.exact {
				col = col.Alpha(.55)
			}
			px := math.Floor(float64(w/2) + (float64(ce.x)-camX)*float64(units))
			pz := math.Floor(float64(h/2) + (float64(ce.z)-camZ)*float64(units))
			sz := float32(max(2, math.Floor(float64(units))))
			p.Fill(ui.Rect{X: r.X + float32(px), Y: r.Y + float32(pz), W: sz, H: sz}, col, 0)
		}
	})
	return true
}

// seedMapOreBadge is the legend: tap a mineral to show only it.
func (a *App) seedMapOreBadge(c *ui.Context, v *seedMapView, s float32, drawn bool) {
	ui.Box(c.Key("seedmap-ores-badge")).Absolute().Top(10*s).Left(10*s).Padding(8*s, 8*s).Radius(radiusChip*s).Background(panel.Alpha(.94)).Border(1, accent.Alpha(.3)).Children(func() {
		ui.Column(c).Gap(4 * s).Children(func() {
			ui.Text(c, a.L("seedmap.ores.title")+"  Y "+strconv.Itoa(v.viewport.Y)).FontSize(typeCaption * s).Bold().TextColor(ink).NoWrap()
			ui.Row(c).Gap(4 * s).AlignItems(ui.Center).Children(func() {
				for _, ore := range seedMapOreOrder {
					label := a.L("seedmap.ores." + ore)
					bg := panel
					if v.oreFilter == ore {
						bg = accent.Alpha(.18)
					}
					b := ui.ButtonBase(c.Key("seedmap-ore-"+ore)).Label(label).Padding(4*s, 4*s).Radius(radiusChip * s).Background(bg)
					b.Children(func() {
						ui.Row(c).Gap(4 * s).AlignItems(ui.Center).Children(func() {
							ui.Box(c).Width(9 * s).Height(9 * s).Radius(radiusMin * s).Background(seedMapOreColors[ore])
							ui.Text(c, label).FontSize(typeCaption * s).TextColor(ink).NoWrap()
						})
					})
					if b.Clicked() {
						if v.oreFilter == ore {
							v.oreFilter = ""
						} else {
							v.oreFilter = ore
						}
					}
				}
			})
			ui.Text(c, a.L("seedmap.ores.note")).FontSize(typeCaption * s).TextColor(muted).NoWrap()
			if v.ores.data != nil && !v.ores.dataExact {
				ui.Text(c, a.L("seedmap.ores.version")+" "+v.ores.data.Version).FontSize(typeCaption * s).TextColor(warningInk).NoWrap()
			}
			if !drawn {
				ui.Text(c, a.L("seedmap.ores.zoom")).FontSize(typeCaption * s).Bold().TextColor(warningInk).NoWrap()
			}
		})
	})
}
