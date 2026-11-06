package nativeui

import (
	"cmp"
	"image"
	"math"
	"slices"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

const (
	seedMapFadeDuration = 160 * time.Millisecond
	seedMapTextureBytes = 64 << 20
	seedMapTextureCount = 192
)

type seedMapTexture struct {
	tile          *launcher.SeedMapTile
	bitmap        *ui.Bitmap
	factor, bytes int
	ink, paper    ui.Color
	born          time.Time
	used          uint64
}

type seedMapRaster struct {
	textures    map[launcher.SeedMapTileKey]*seedMapTexture
	clock       uint64
	bytes       int
	uploads     uint64
	uploadBytes uint64
}

func (r *seedMapRaster) accept(frame launcher.SeedMapFrame) {
	if r.textures == nil {
		r.textures = make(map[launcher.SeedMapTileKey]*seedMapTexture)
	}
	for _, tile := range frame.Tiles() {
		if r.textures[tile.Key] == nil {
			r.textures[tile.Key] = &seedMapTexture{tile: tile}
		}
	}
}

func (v *seedMapView) mapReady() bool { return len(v.frame.Tiles()) != 0 }

func (r *seedMapRaster) prepare(t *seedMapTexture, factor int, now time.Time) {
	t.used = r.clock
	if t.bitmap != nil && t.factor == factor && (factor == 1 || t.ink == ink && t.paper == panel) {
		return
	}
	img := t.tile.Image
	if factor > 1 {
		img = seedMapReliefBitmap(t.tile, factor, ink, panel)
	}
	if t.bitmap == nil {
		t.born = now
	}
	r.bytes -= t.bytes
	t.bitmap = ui.NewBitmap(img).Pixelated()
	t.factor, t.bytes, t.ink, t.paper = factor, len(img.Pix), ink, panel
	r.bytes += t.bytes
	r.uploads++
	r.uploadBytes += uint64(t.bytes)
}

func (r *seedMapRaster) prune() {
	for len(r.textures) > seedMapTextureCount || r.bytes > seedMapTextureBytes {
		var oldest *seedMapTexture
		for _, t := range r.textures {
			if t.used != r.clock && (oldest == nil || t.used < oldest.used) {
				oldest = t
			}
		}
		if oldest == nil { // The visible raster is bounded separately below.
			return
		}
		delete(r.textures, oldest.tile.Key)
		r.bytes -= oldest.bytes
	}
}

// Bake fine relief into the display tile once, not thousands of vector fills on
// every pointer move. Integer expansion keeps both terrain cells and contour
// strokes sharp. The small chunk grid remains a separate world-aligned overlay.
func seedMapReliefBitmap(tile *launcher.SeedMapTile, factor int, dark, light ui.Color) *image.RGBA {
	src := tile.Image
	w, h := src.Rect.Dx()*factor, src.Rect.Dy()*factor
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		row := dst.Pix[y*dst.Stride : (y+1)*dst.Stride]
		for x := range w {
			i := (y/factor)*src.Stride + (x/factor)*4
			copy(row[x*4:x*4+4], src.Pix[i:i+4])
		}
	}
	for _, edge := range tile.Contours() {
		color := dark.Alpha(.52)
		if edge.Light {
			color = light.Alpha(.65)
		}
		if edge.Boundary {
			color = dark.Alpha(.8)
		}
		x, y := int(edge.X)*factor, int(edge.Z)*factor
		for d := range factor {
			px, py := x, y+d
			if edge.Horizontal {
				px, py = x+d, y
			}
			if px < 0 || py < 0 || px >= w || py >= h {
				continue
			}
			i, alpha := py*dst.Stride+px*4, uint32(color.A)
			for channel, value := range [3]uint8{color.R, color.G, color.B} {
				dst.Pix[i+channel] = uint8((uint32(value)*alpha + uint32(dst.Pix[i+channel])*(255-alpha) + 127) / 255)
			}
		}
	}
	return dst
}

type seedMapDrawTile struct {
	texture *seedMapTexture
	rect    ui.Rect
	back    bool
}

func (v *seedMapView) drawTiles(now time.Time, w, h, units float32) []seedMapDrawTile {
	r, view := &v.raster, v.viewport
	r.clock++
	factor := 1
	if view.ContourOverlay {
		factor = max(2, min(8, int(math.Round(float64(units*float32(view.BlocksPerPixel))))))
		// Reserve at most half the cache for one visible detailed view,
		// including its partially visible edge tiles.
		area := (view.Width + 256) * (view.Height + 256) * 4
		for factor > 2 && area*factor*factor > seedMapTextureBytes/2 {
			factor--
		}
	}
	var front, back []seedMapDrawTile
	var backLayer launcher.SeedMapTileKey
	best := math.Inf(1)
	for key, t := range r.textures {
		if key.Dimension != view.Dimension || key.Y != view.Y || key.Surface != view.Surface {
			continue
		}
		span := launcher.SeedMapTilePixels * key.BlocksPerPixel
		x := w/2 + float32(float64(key.X*span)-v.camera.x)*units
		y := h/2 + float32(float64(key.Z*span)-v.camera.z)*units
		size := float32(span) * units
		if x+size <= 0 || y+size <= 0 || x >= w || y >= h {
			continue
		}
		item := seedMapDrawTile{texture: t, rect: ui.Rect{X: x, Y: y, W: size, H: size}}
		if key.BlocksPerPixel == view.BlocksPerPixel && key.ContourOverlay == view.ContourOverlay && key.Terrain == (view.Terrain && view.Surface) && key.Structure == view.Structure {
			r.prepare(t, factor, now)
			front = append(front, item)
		} else if t.bitmap != nil {
			layer := key
			layer.X, layer.Z = 0, 0
			distance := math.Abs(math.Log2(float64(key.BlocksPerPixel) / float64(view.BlocksPerPixel)))
			// Prefer the current structure filter when multiple cached layers
			// share a resolution; deterministic ordering avoids flicker.
			if key.Structure != view.Structure {
				distance += .1
			}
			if key.ContourOverlay != view.ContourOverlay {
				distance += .01
			}
			if distance < best || distance == best && layer.Structure < backLayer.Structure {
				best, backLayer = distance, layer
			}
			item.back = true
			back = append(back, item)
		}
	}
	fading := v.pending || v.requested != view
	for _, item := range front {
		fading = fading || now.Sub(item.texture.born) < seedMapFadeDuration
	}
	back = slices.DeleteFunc(back, func(item seedMapDrawTile) bool {
		key := item.texture.tile.Key
		key.X, key.Z = 0, 0
		return !fading || key != backLayer
	})
	for _, item := range back {
		item.texture.used = r.clock
	}
	// A previous zoom level is only a temporary background. Never pin enough
	// background textures to crowd the current view out of the memory budget.
	visibleBytes := 0
	for _, item := range front {
		visibleBytes += item.texture.bytes
	}
	back = slices.DeleteFunc(back, func(item seedMapDrawTile) bool {
		if visibleBytes+item.texture.bytes > seedMapTextureBytes {
			item.texture.used = 0
			return true
		}
		visibleBytes += item.texture.bytes
		return false
	})
	result := append(back, front...)
	slices.SortFunc(result, func(a, b seedMapDrawTile) int {
		if a.back != b.back {
			if a.back {
				return -1
			}
			return 1
		}
		return cmp.Or(cmp.Compare(a.rect.Y, b.rect.Y), cmp.Compare(a.rect.X, b.rect.X))
	})
	r.prune()
	return result
}

func seedMapPaintTiles(p *ui.Painter, r ui.Rect, tiles []seedMapDrawTile, lessMotion bool) {
	for _, tile := range tiles {
		alpha := float32(1)
		if !tile.back && !lessMotion {
			alpha = easeOutCubic(min(1, max(0, float32(p.Now().Sub(tile.texture.born))/float32(seedMapFadeDuration))))
			if alpha < 1 {
				p.After(min(time.Second/30, seedMapFadeDuration-p.Now().Sub(tile.texture.born)))
			}
		}
		dst := tile.rect
		dst.X, dst.Y = dst.X+r.X, dst.Y+r.Y
		p.ImageRegion(tile.texture.bitmap, ui.Rect{}, dst, ui.ImageOptions{Tint: ui.RGB(255, 255, 255).Alpha(alpha)})
	}
}

func seedMapLoading(c *ui.Context, s float32, label string, extra func()) {
	lessMotion := c.Preferences().ReduceMotion
	ui.Row(c.Key("seedmap-wait")).Absolute().Left(12*s).Top(12*s).Padding(8*s, 12*s).Gap(8 * s).AlignItems(ui.Center).Background(panel.Alpha(.94)).Radius(radiusChip * s).Children(func() {
		ui.Box(c).Size(29*s, 12*s).Draw(func(p *ui.Painter, r ui.Rect) {
			phase := int(p.Now().UnixMilli()/160) % 3
			for i := range 3 {
				alpha := float32(.35)
				if lessMotion || i == phase {
					alpha = 1
				}
				p.Fill(ui.Rect{X: r.X + float32(i)*10*s, Y: r.Y + 3*s, W: 5 * s, H: 5 * s}, accent.Alpha(alpha), radiusPill)
			}
			if !lessMotion {
				p.After(time.Second / 12)
			}
		})
		ui.Text(c, label).FontSize(typeCaption * s).TextColor(inkSecondary)
		if extra != nil {
			extra()
		}
	})
}

// Collapse worker updates before posting to the UI. A busy UI must not replay a
// queue of obsolete partial frames and upload each of their textures.
type seedMapDelivery struct {
	mu     sync.Mutex
	frame  launcher.SeedMapFrame
	posted bool
}

func (d *seedMapDelivery) put(frame launcher.SeedMapFrame) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.frame = frame
	if d.posted {
		return false
	}
	d.posted = true
	return true
}

func (d *seedMapDelivery) take() launcher.SeedMapFrame {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.posted = false
	frame := d.frame
	d.frame = launcher.SeedMapFrame{}
	return frame
}
