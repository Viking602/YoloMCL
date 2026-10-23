package nativeui

import (
	"image"
	"image/color"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// The wardrobe uses the same native bitmap primitive as the rest of the UI.
// Cuboid faces are textured on the CPU; no browser, canvas or JavaScript runs.
type skinPreview struct {
	skin, cape        image.Image
	slim, dirty, spin bool
	yaw, pitch, zoom  float64
	last              time.Time
	target            *float64 // eased yaw goal (front / back / cape turn)
	bitmap            *ui.Bitmap
}
type point3 struct{ x, y, z float64 }
type vertex struct{ x, y, z, u, v float64 }
type skinFace struct {
	p       [4]point3
	uv      [4][2]float64
	shade   float64
	texture image.Image
}

func newSkinPreview() *skinPreview {
	return &skinPreview{skin: builtinSkin(false), yaw: -.35, pitch: -.1, zoom: 1, dirty: true}
}
func (p *skinPreview) set(skin, cape image.Image, slim bool) {
	if skin == nil {
		skin = builtinSkin(slim)
	}
	p.skin = skin
	p.cape = cape
	p.slim = slim
	p.dirty = true
}
func (p *skinPreview) faces() []skinFace {
	var faces []skinFace
	add := func(x, y, z, w, h, d float64, u, v int, overlay bool, tex image.Image) {
		if tex == nil {
			return
		}
		inflate := 0.
		if overlay {
			inflate = .18
		}
		x0, x1, y0, y1, z0, z1 := x-inflate, x+w+inflate, y-inflate, y+h+inflate, z-inflate, z+d+inflate
		rect := func(x, y, w, h float64) [4][2]float64 {
			return [4][2]float64{{x, y}, {x + w, y}, {x + w, y + h}, {x, y + h}}
		}
		uu, vv := float64(u), float64(v)
		faces = append(faces,
			skinFace{[4]point3{{x1, y1, z1}, {x0, y1, z1}, {x0, y0, z1}, {x1, y0, z1}}, rect(uu+d, vv+d, w, h), 1, tex},
			skinFace{[4]point3{{x0, y1, z0}, {x1, y1, z0}, {x1, y0, z0}, {x0, y0, z0}}, rect(uu+2*d+w, vv+d, w, h), .8, tex},
			skinFace{[4]point3{{x0, y1, z1}, {x0, y1, z0}, {x0, y0, z0}, {x0, y0, z1}}, rect(uu, vv+d, d, h), .72, tex},
			skinFace{[4]point3{{x1, y1, z0}, {x1, y1, z1}, {x1, y0, z1}, {x1, y0, z0}}, rect(uu+d+w, vv+d, d, h), .86, tex},
			skinFace{[4]point3{{x0, y1, z0}, {x0, y1, z1}, {x1, y1, z1}, {x1, y1, z0}}, rect(uu+d, vv, w, d), 1.05, tex},
			skinFace{[4]point3{{x1, y0, z0}, {x1, y0, z1}, {x0, y0, z1}, {x0, y0, z0}}, rect(uu+d+w, vv, w, d), .6, tex})
	}
	arm := 4.
	if p.slim {
		arm = 3
	}
	add(-4, 24, -4, 8, 8, 8, 0, 0, false, p.skin)
	add(-4, 12, -2, 8, 12, 4, 16, 16, false, p.skin)
	add(-4-arm, 12, -2, arm, 12, 4, 40, 16, false, p.skin)
	add(-4, 0, -2, 4, 12, 4, 0, 16, false, p.skin)
	if p.skin.Bounds().Dy() >= 64 {
		add(4, 12, -2, arm, 12, 4, 32, 48, false, p.skin)
		add(0, 0, -2, 4, 12, 4, 16, 48, false, p.skin)
	} else {
		add(4, 12, -2, arm, 12, 4, 40, 16, false, p.skin)
		add(0, 0, -2, 4, 12, 4, 0, 16, false, p.skin)
	}
	add(-4, 24, -4, 8, 8, 8, 32, 0, true, p.skin)
	if p.skin.Bounds().Dy() >= 64 {
		add(-4, 12, -2, 8, 12, 4, 16, 32, true, p.skin)
		add(-4-arm, 12, -2, arm, 12, 4, 40, 32, true, p.skin)
		add(4, 12, -2, arm, 12, 4, 48, 48, true, p.skin)
		add(-4, 0, -2, 4, 12, 4, 0, 32, true, p.skin)
		add(0, 0, -2, 4, 12, 4, 0, 48, true, p.skin)
	}
	if p.cape != nil {
		add(-5, 7, -3.5, 10, 16, 1, 0, 0, false, p.cape)
	}
	return faces
}
func (p *skinPreview) render(w, h int) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	depth := make([]float64, w*h)
	for i := range depth {
		depth[i] = math.Inf(-1)
	}
	for y := h - 40; y < h-16; y++ {
		for x := w/2 - 65; x < w/2+65; x++ {
			dx, dy := float64(x-w/2)/65, float64(y-h+28)/12
			if dx*dx+dy*dy < 1 {
				out.SetRGBA(x, y, color.RGBA{9, 12, 13, 150})
			}
		}
	}
	cy, sy, cp, sp := math.Cos(p.yaw), math.Sin(p.yaw), math.Cos(p.pitch), math.Sin(p.pitch)
	scale := float64(h-60) / 32 * p.zoom
	for _, f := range p.faces() {
		var v [4]vertex
		for i, pt := range f.p {
			x := pt.x*cy + pt.z*sy
			z := -pt.x*sy + pt.z*cy
			yy := pt.y - 16
			y := yy*cp - z*sp
			z = yy*sp + z*cp
			v[i] = vertex{float64(w)/2 + x*scale, float64(h)/2 - y*scale - 4, z, f.uv[i][0], f.uv[i][1]}
		}
		rasterTriangle(out, depth, v[0], v[1], v[2], f)
		rasterTriangle(out, depth, v[0], v[2], v[3], f)
	}
	return out
}
func rasterTriangle(out *image.RGBA, depth []float64, a, b, c vertex, f skinFace) {
	den := (b.y-c.y)*(a.x-c.x) + (c.x-b.x)*(a.y-c.y)
	if math.Abs(den) < .001 {
		return
	}
	w, h := out.Bounds().Dx(), out.Bounds().Dy()
	minX, maxX := max(0, int(math.Floor(min(a.x, b.x, c.x)))), min(w-1, int(math.Ceil(max(a.x, b.x, c.x))))
	minY, maxY := max(0, int(math.Floor(min(a.y, b.y, c.y)))), min(h-1, int(math.Ceil(max(a.y, b.y, c.y))))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			px, py := float64(x)+.5, float64(y)+.5
			u := ((b.y-c.y)*(px-c.x) + (c.x-b.x)*(py-c.y)) / den
			v := ((c.y-a.y)*(px-c.x) + (a.x-c.x)*(py-c.y)) / den
			t := 1 - u - v
			if u < -0.0001 || v < -0.0001 || t < -0.0001 {
				continue
			}
			z := a.z*u + b.z*v + c.z*t
			idx := y*w + x
			if z < depth[idx] {
				continue
			}
			tx, ty := int(a.u*u+b.u*v+c.u*t), int(a.v*u+b.v*v+c.v*t)
			bounds := f.texture.Bounds()
			tx = tx * bounds.Dx() / 64
			ty = ty * bounds.Dx() / 64
			if tx < 0 || ty < 0 || tx >= bounds.Dx() || ty >= bounds.Dy() {
				continue
			}
			col := color.NRGBAModel.Convert(f.texture.At(tx+bounds.Min.X, ty+bounds.Min.Y)).(color.NRGBA)
			if col.A < 16 {
				continue
			}
			shade := func(n uint8) uint8 { return uint8(math.Min(255, float64(n)*f.shade)) }
			alpha := float64(col.A) / 255
			old := out.RGBAAt(x, y)
			mix := func(n, o uint8) uint8 { return uint8(float64(n)*alpha + float64(o)*(1-alpha)) }
			out.SetRGBA(x, y, color.RGBA{mix(shade(col.R), old.R), mix(shade(col.G), old.G), mix(shade(col.B), old.B), 255})
			depth[idx] = z
		}
	}
}
