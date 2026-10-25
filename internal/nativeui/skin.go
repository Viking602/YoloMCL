package nativeui

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// The wardrobe uses the same native bitmap primitive as the rest of the UI.
// Cuboid faces are textured on the CPU; no browser, canvas or JavaScript runs.
type skinPreview struct {
	skin, cape         image.Image
	slim, dirty, spin  bool
	yaw, pitch, zoom   float64
	zoomGoal           float64
	last               time.Time
	spinFrame          time.Time
	target             *float64 // eased yaw goal (front / back / cape turn)
	bitmap             *ui.Bitmap
	frames             [2]*image.RGBA // keep the displayed frame intact until Bitmap.Update
	frameIndex         int
	depth              []float64
	mesh               []skinFace
	meshSkin, meshCape image.Image
	meshSlim           bool
}
type point3 struct{ x, y, z float64 }
type vertex struct{ x, y, z, u, v float64 }
type skinFace struct {
	p       [4]point3
	uv      [4][2]float64
	shade   int
	texture *image.NRGBA
}

// Shade each channel once, rather than converting colors and multiplying three
// channels for every covered pixel on every animation frame.
var skinShades = func() [6][256]uint8 {
	var table [6][256]uint8
	for face, shade := range [...]float64{1, .8, .72, .86, 1.05, .6} {
		for channel := range table[face] {
			table[face][channel] = uint8(math.Min(255, float64(channel)*shade))
		}
	}
	return table
}()

func skinPixels(img image.Image) *image.NRGBA {
	if img == nil {
		return nil
	}
	if pixels, ok := img.(*image.NRGBA); ok {
		return pixels
	}
	pixels := image.NewNRGBA(img.Bounds())
	draw.Draw(pixels, pixels.Bounds(), img, img.Bounds().Min, draw.Src)
	return pixels
}

func newSkinPreview() *skinPreview {
	return &skinPreview{skin: builtinSkin(false), yaw: -.35, pitch: -.1, zoom: 1, zoomGoal: 1, dirty: true}
}

// Keep the last displayed bitmap and the user's pose, but release scratch
// pixels and the depth buffer while the wardrobe is offscreen. The bitmap
// owns its current pixels, so an outgoing transition remains intact. A still
// preview can be shown again without rasterizing; motion rebuilds scratch
// buffers once and then resumes the same allocation-free render loop.
func (p *skinPreview) releaseScratch() {
	p.frames = [2]*image.RGBA{}
	p.depth = nil
	p.last, p.spinFrame = time.Time{}, time.Time{}
}

func (p *skinPreview) zoomBy(delta float64) {
	p.zoomGoal = max(.65, min(1.4, p.zoomGoal+delta))
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
	if p.mesh != nil && p.meshSkin == p.skin && p.meshCape == p.cape && p.meshSlim == p.slim {
		return p.mesh
	}
	skin, cape := skinPixels(p.skin), skinPixels(p.cape)
	faces := p.mesh[:0]
	add := func(x, y, z, w, h, d float64, u, v int, overlay bool, tex *image.NRGBA) {
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
			skinFace{[4]point3{{x1, y1, z1}, {x0, y1, z1}, {x0, y0, z1}, {x1, y0, z1}}, rect(uu+d, vv+d, w, h), 0, tex},
			skinFace{[4]point3{{x0, y1, z0}, {x1, y1, z0}, {x1, y0, z0}, {x0, y0, z0}}, rect(uu+2*d+w, vv+d, w, h), 1, tex},
			skinFace{[4]point3{{x0, y1, z1}, {x0, y1, z0}, {x0, y0, z0}, {x0, y0, z1}}, rect(uu, vv+d, d, h), 2, tex},
			skinFace{[4]point3{{x1, y1, z0}, {x1, y1, z1}, {x1, y0, z1}, {x1, y0, z0}}, rect(uu+d+w, vv+d, d, h), 3, tex},
			skinFace{[4]point3{{x0, y1, z0}, {x0, y1, z1}, {x1, y1, z1}, {x1, y1, z0}}, rect(uu+d, vv, w, d), 4, tex},
			skinFace{[4]point3{{x1, y0, z0}, {x1, y0, z1}, {x0, y0, z1}, {x0, y0, z0}}, rect(uu+d+w, vv, w, d), 5, tex})
	}
	arm := 4.
	if p.slim {
		arm = 3
	}
	add(-4, 24, -4, 8, 8, 8, 0, 0, false, skin)
	add(-4, 12, -2, 8, 12, 4, 16, 16, false, skin)
	add(-4-arm, 12, -2, arm, 12, 4, 40, 16, false, skin)
	add(-4, 0, -2, 4, 12, 4, 0, 16, false, skin)
	if p.skin.Bounds().Dy() >= 64 {
		add(4, 12, -2, arm, 12, 4, 32, 48, false, skin)
		add(0, 0, -2, 4, 12, 4, 16, 48, false, skin)
	} else {
		add(4, 12, -2, arm, 12, 4, 40, 16, false, skin)
		add(0, 0, -2, 4, 12, 4, 0, 16, false, skin)
	}
	add(-4, 24, -4, 8, 8, 8, 32, 0, true, skin)
	if p.skin.Bounds().Dy() >= 64 {
		add(-4, 12, -2, 8, 12, 4, 16, 32, true, skin)
		add(-4-arm, 12, -2, arm, 12, 4, 40, 32, true, skin)
		add(4, 12, -2, arm, 12, 4, 48, 48, true, skin)
		add(-4, 0, -2, 4, 12, 4, 0, 32, true, skin)
		add(0, 0, -2, 4, 12, 4, 0, 48, true, skin)
	}
	if p.cape != nil {
		start := len(faces)
		add(-5, 7, -3.5, 10, 16, 1, 0, 0, false, cape)
		// The cape's decorated face points away from the player's back (-Z),
		// unlike the front of a skin cuboid. Orient all six atlas regions here
		// once, keeping the outer artwork consistent with capeFront's thumbnail.
		for i, uv := range [6][4][2]float64{
			{{22, 1}, {12, 1}, {12, 17}, {22, 17}}, // lining (+Z)
			{{11, 1}, {1, 1}, {1, 17}, {11, 17}},   // outside (-Z)
			{{12, 1}, {11, 1}, {11, 17}, {12, 17}}, // right edge (-X)
			{{1, 1}, {0, 1}, {0, 17}, {1, 17}},     // left edge (+X)
			{{11, 1}, {11, 0}, {1, 0}, {1, 1}},     // top (+Y)
			{{11, 1}, {11, 0}, {21, 0}, {21, 1}},   // bottom (-Y)
		} {
			faces[start+i].uv = uv
		}
	}
	if len(p.mesh) > len(faces) {
		clear(p.mesh[len(faces):])
	}
	p.mesh, p.meshSkin, p.meshCape, p.meshSlim = faces, p.skin, p.cape, p.slim
	return faces
}

// render alternates two buffers: its result stays intact through the next call
// so a displayed bitmap is never overwritten before its replacement is ready.
func (p *skinPreview) render(w, h int) *image.RGBA {
	p.frameIndex ^= 1
	out := p.frames[p.frameIndex]
	if out == nil || out.Bounds().Dx() != w || out.Bounds().Dy() != h {
		out = image.NewRGBA(image.Rect(0, 0, w, h))
		p.frames[p.frameIndex] = out
	} else {
		clear(out.Pix)
	}
	if len(p.depth) != w*h {
		p.depth = make([]float64, w*h)
	}
	depth := p.depth
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
		// Scale before interpolation and integer sampling: scaling an already
		// rounded atlas coordinate discards texels from high-resolution capes.
		textureScale := float64(f.texture.Bounds().Dx()) / 64
		for i, pt := range f.p {
			x := pt.x*cy + pt.z*sy
			z := -pt.x*sy + pt.z*cy
			yy := pt.y - 16
			y := yy*cp - z*sp
			z = yy*sp + z*cp
			v[i] = vertex{float64(w)/2 + x*scale, float64(h)/2 - y*scale - 4, z, f.uv[i][0] * textureScale, f.uv[i][1] * textureScale}
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
	texture, shade := f.texture, &skinShades[f.shade]
	tw, th := texture.Bounds().Dx(), texture.Bounds().Dy()
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
			if tx < 0 || ty < 0 || tx >= tw || ty >= th {
				continue
			}
			src := texture.Pix[ty*texture.Stride+tx*4:][:4]
			if src[3] < 16 {
				continue
			}
			dst := out.Pix[idx*4:][:4]
			if src[3] == 255 {
				dst[0], dst[1], dst[2] = shade[src[0]], shade[src[1]], shade[src[2]]
			} else {
				alpha := float64(src[3]) / 255
				for channel := range 3 {
					dst[channel] = uint8(float64(shade[src[channel]])*alpha + float64(dst[channel])*(1-alpha))
				}
			}
			dst[3] = 255
			depth[idx] = z
		}
	}
}
