package nativeui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// flyDuration is how long a click glides the view to its target.
const flyDuration = 600 * time.Millisecond

// seedMapFly is a glide of the orbit centre, pan and zoom in progress.
type seedMapFly struct {
	from, to structure3d.Camera
	start    time.Time
}

// yApprox marks heights that are not seed-exact.
func (p *seedMapStructurePreview) yApprox() string {
	if p.seeded {
		return ""
	}
	return "≈"
}

func (p *seedMapStructurePreview) curAim() [3]float64 {
	if p.aimed {
		return p.aim
	}
	return p.mesh.Center
}

// startFly glides the orbit centre to aim and the zoom to zoom, with the pan
// returning to the middle. Reduced motion jumps.
func (a *App) startFly(c *ui.Context, v *seedMapView, p *seedMapStructurePreview, aim [3]float64, zoom float64) {
	p.fly = &seedMapFly{
		from: structure3d.Camera{Aim: p.curAim(), PanX: p.panX, PanY: p.panY, Zoom: p.zoom},
		to:   structure3d.Camera{Aim: aim, Zoom: zoom},
	}
	a.stepFly(c, v, p)
}

func (a *App) stepFly(c *ui.Context, v *seedMapView, p *seedMapStructurePreview) {
	f := p.fly
	now := c.Now()
	if f.start.IsZero() {
		f.start = now
	}
	t := float64(now.Sub(f.start)) / float64(flyDuration)
	if c.Preferences().ReduceMotion || t >= 1 {
		t = 1
	}
	g := structure3d.GlideCamera(t, f.from, f.to)
	p.aim, p.aimed = g.Aim, true
	p.panX, p.panY, p.zoom = g.PanX, g.PanY, g.Zoom
	if t >= 1 {
		p.fly = nil
		a.rerenderStructurePreview(v)
		return
	}
	a.rerenderStructurePreview(v)
	c.AnimationFrame()
}

// fitStructurePreview frames the whole structure: centre, no pan, a zoom
// from its extent.
func (a *App) fitStructurePreview(c *ui.Context, v *seedMapView, p *seedMapStructurePreview) {
	half := math.Max(p.mesh.Extent(), float64(p.mesh.MaxY-p.mesh.MinY+1)/2)
	zoom := math.Max(3, math.Min(64, float64(structurePreviewFullH)*0.4/math.Max(half, 1)))
	a.startFly(c, v, p, p.mesh.Center, zoom)
}

// stageFit is the Contain fit of the full-size frame in the stage: scale
// and top-left offset in stage pixels.
func stageFit(b ui.Rect) (k, ox, oy float32) {
	k = min(b.W/structurePreviewFullW, b.H/structurePreviewFullH)
	return k, (b.W - structurePreviewFullW*k) / 2, (b.H - structurePreviewFullH*k) / 2
}

// pickStructureBlock selects the block under stage point (px, py) and flies
// to it; empty space clears the selection.
func (a *App) pickStructureBlock(c *ui.Context, v *seedMapView, p *seedMapStructurePreview, b ui.Rect, px, py float32) {
	shown := &p.frames[1-p.back]
	if shown.Image == nil {
		return
	}
	k, ox, oy := stageFit(b)
	fw := float32(shown.Image.Rect.Dx())
	scale := fw / structurePreviewFullW
	x, y := int((px-ox)/k*scale), int((py-oy)/k*scale)
	pk, ok := p.mesh.PickAt(shown, x, y)
	if !ok {
		if p.sel.Index != 0 {
			p.sel = structure3d.Picked{}
			a.rerenderStructurePreview(v)
		}
		return
	}
	p.sel = pk
	a.startFly(c, v, p, [3]float64{float64(pk.X) + .5, float64(pk.Y) + .5, float64(pk.Z) + .5}, p.zoom)
}

// seedMapStructureSelLabel pins the selected block's name and Y beside it.
func (a *App) seedMapStructureSelLabel(c *ui.Context, p *seedMapStructurePreview, b ui.Rect, s float32) {
	if p.sel.Index == 0 || b.W <= 0 {
		return
	}
	sx, sy, ok := p.mesh.Project(p.camera(), structurePreviewFullW, structurePreviewFullH, float64(p.sel.X)+.5, float64(p.sel.Y)+.5, float64(p.sel.Z)+.5)
	if !ok {
		return
	}
	k, ox, oy := stageFit(b)
	x, y := ox+float32(sx)*k, oy+float32(sy)*k
	if x < 0 || y < 0 || x > b.W || y > b.H {
		return
	}
	name := p.sel.Name
	if i := strings.IndexByte(name, ':'); i >= 0 && name[:i] == "minecraft" {
		name = name[i+1:]
	}
	ui.Box(c.Key("seedmap-structure3d-sel-dot")).Absolute().Left(x-3*s).Top(y-3*s).Size(6*s, 6*s).Radius(radiusPill).Background(accent).Border(1, panel)
	ui.Box(c.Key("seedmap-structure3d-sel")).Absolute().Left(x+8*s).Top(y-26*s).Padding(4*s, 8*s).Radius(radiusMin*s).Background(panel).Border(1, accent.Alpha(.5)).Children(func() {
		ui.Text(c, fmt.Sprintf(a.L("seedmap.structure_preview.selected"), name, p.yApprox(), p.sel.Y)).FontSize(typeCaption * s).Bold().TextColor(ink).NoWrap()
	})
}

// seedMapAxisGizmo shows world X (red), Y (green) and Z (blue) for the
// current view in the stage's top-left corner.
func seedMapAxisGizmo(c *ui.Context, p *seedMapStructurePreview, s float32) {
	dirs := structure3d.AxisDirs(p.camera())
	size := 52 * s
	cols := [3]ui.Color{ui.Hex("#d64545"), ui.Hex("#3a9a4a"), ui.Hex("#2f6feb")}
	names := [3]string{"X", "Y", "Z"}
	ui.Box(c.Key("seedmap-structure3d-axes")).Absolute().Left(8*s).Top(8*s).Size(size, size).Draw(func(pt *ui.Painter, r ui.Rect) {
		cx, cy, l := r.X+r.W/2, r.Y+r.H/2, r.W*0.32
		for i, d := range dirs {
			n := math.Hypot(d[0], d[1])
			if n < 1e-6 {
				continue
			}
			ex, ey := cx+float32(d[0]/math.Max(n, 1))*l, cy+float32(d[1]/math.Max(n, 1))*l
			pt.Line(cx, cy, ex, ey, 2*s, cols[i])
			pt.Text(ex+float32(d[0]/n)*5*s-3*s, ey+float32(d[1]/n)*5*s-6*s, names[i], typeCaption*s, cols[i])
		}
	})
}

// seedMapStructureFootprint outlines the open preview's pieces on the 2D
// map, dashed unless the piece layout is seed-exact.
func seedMapStructureFootprint(c *ui.Context, v *seedMapView, w, h, s float32, point func(x, z int) (float32, float32)) {
	p := v.structurePreview
	if p == nil || p.mesh == nil || len(p.mesh.Pieces) == 0 {
		return
	}
	exact := p.seeded || p.layoutSeeded
	pieces := p.mesh.Pieces
	ui.Box(c.Key("seedmap-structure-footprint")).Absolute().Left(0).Top(0).Size(w, h).Draw(func(pt *ui.Painter, r ui.Rect) {
		for _, b := range pieces {
			x0, z0 := point(b.Min[0], b.Min[2])
			x1, z1 := point(b.Max[0]+1, b.Max[2]+1)
			if x1 < 0 || z1 < 0 || x0 > w || z0 > h {
				continue
			}
			rr := ui.Rect{X: r.X + x0, Y: r.Y + z0, W: max(x1-x0, 1), H: max(z1-z0, 1)}
			pt.Fill(rr, accent.Alpha(.10), 0)
			if exact {
				pt.Stroke(rr, accent.Alpha(.75), 0, 1*s)
			} else {
				pt.StrokeDashed(rr, accent.Alpha(.75), 0, 1*s)
			}
		}
	})
}
