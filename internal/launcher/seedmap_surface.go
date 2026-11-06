package launcher

import (
	"context"
	"encoding/json"
	"image/color"
	"math"

	"github.com/Viking602/YoloMCL/cubiomes"
)

const seedMapSurfaceWidth = SeedMapTilePixels + 2

func seedMapSeaLevel(raw json.RawMessage, docs map[string]WorldgenDocument) int {
	var id string
	if json.Unmarshal(raw, &id) == nil {
		raw = docs["noise_settings/"+seedMapID(id)].Data
	}
	var settings struct {
		Sea *int `json:"sea_level"`
	}
	if json.Unmarshal(raw, &settings) == nil && settings.Sea != nil && *settings.Sea >= -2048 && *settings.Sea <= 2047 {
		return *settings.Sea
	}
	return 63
}

// SurfaceHeightAt estimates terrain surface Y at block (x,z) using the native
// depth graph. ok is false when the dimension has no surface support.
func (s *LocalSampler) SurfaceHeightAt(dimension string, x, z int) (int, bool) {
	d := s.dimensions[dimension]
	if d.noiseSurface() {
		return d.noiseSurfaceAt(x, z)
	}
	if d.engine == nil || !d.engine.SupportsSurface() {
		return 0, false
	}
	scratch := d.scratch()
	if scratch == nil {
		return 0, false
	}
	defer d.releaseScratch(scratch)
	h, ok := scratch.EstimateSurfaceHeight(x, z)
	if !ok {
		return 0, false
	}
	return int(math.Round(float64(h))), true
}

// SupportsSurface enables relief only for a recognized native depth graph.
// Unsupported dimensions continue to show the existing Y-slice biome map.
func (s *LocalSampler) SupportsSurface(dimension string) bool {
	d := s.dimensions[dimension]
	return (d.engine != nil && d.engine.SupportsSurface()) || d.noiseSurface()
}

func seedMapSurface(ctx context.Context, scratch *cubiomes.Sampler, key SeedMapTileKey, x, z, sea int) ([]cubiomes.SurfaceSample, error) {
	// One pixel of world-space halo makes derivatives and contours continuous
	// across tile edges. It is retained with the immutable height/biome raster.
	out := make([]cubiomes.SurfaceSample, seedMapSurfaceWidth*seedMapSurfaceWidth)
	err := scratch.GenerateSurface(ctx, cubiomes.SurfaceRange{X: x - key.BlocksPerPixel, Z: z - key.BlocksPerPixel, SizeX: seedMapSurfaceWidth, SizeZ: seedMapSurfaceWidth, Step: key.BlocksPerPixel, SeaLevel: sea, Voronoi: key.BlocksPerPixel < 4}, out)
	return out, err
}

func seedMapRelief(base color.RGBA, cells []cubiomes.SurfaceSample, x, z, step int, contours bool) color.RGBA {
	i := z*seedMapSurfaceWidth + x
	h := float64(cells[i].Height)
	// Void neighbours (NaN, final_density relief) read as level ground.
	at := func(j int) float64 {
		if v := float64(cells[j].Height); !math.IsNaN(v) {
			return v
		}
		return h
	}
	w, e, n, s := at(i-1), at(i+1), at(i-seedMapSurfaceWidth), at(i+seedMapSurfaceWidth)
	dx, dz := (e-w)/float64(2*step), (s-n)/float64(2*step)
	// Northwest light; ambient light keeps steep shadowed slopes readable.
	light := (.5*dx + .5*dz + .70710678) / math.Sqrt(1+dx*dx+dz*dz)
	shade := max(.48, min(1.24, .55+.66*light))
	if contours {
		interval := float64(max(1, step))
		level := math.Floor(h / interval)
		if level != math.Floor(w/interval) || level != math.Floor(n/interval) {
			shade *= .72
		} else if level != math.Floor(e/interval) || level != math.Floor(s/interval) {
			shade *= 1.06
		}
		if cells[i].Biome != cells[i-1].Biome || cells[i].Biome != cells[i-seedMapSurfaceWidth].Biome {
			shade *= .75
		}
	}
	channel := func(v byte) byte { return byte(max(0, min(255, math.Round(float64(v)*shade)))) }
	return color.RGBA{channel(base.R), channel(base.G), channel(base.B), 255}
}

// SeedMapContour is one raster-cell edge. The UI draws it one display pixel
// wide, so magnifying terrain does not magnify contour stroke thickness.
type SeedMapContour struct {
	X, Z                        int16
	Horizontal, Light, Boundary bool
}

// Contours returns tile-local contour edges. The source includes a one-cell halo
// so independently cached display tiles have identical edges at their seams.
func (t *SeedMapTile) Contours() []SeedMapContour {
	span := SeedMapTilePixels * t.Key.BlocksPerPixel
	return (SeedMapFrame{Viewport: SeedMapViewport{
		X: t.Key.X*span + span/2, Z: t.Key.Z*span + span/2,
		Width: SeedMapTilePixels, Height: SeedMapTilePixels,
		BlocksPerPixel: t.Key.BlocksPerPixel, ContourOverlay: t.Key.ContourOverlay,
	}, tiles: []*SeedMapTile{t}}).Contours()
}

// Contours returns visible fine contour segments, reading no mutable sampler.
func (f SeedMapFrame) Contours() []SeedMapContour {
	if !f.Viewport.ContourOverlay {
		return nil
	}
	v := f.Viewport
	span := SeedMapTilePixels * v.BlocksPerPixel
	x, z := v.Origin()
	var out []SeedMapContour
	for _, tile := range f.tiles {
		if tile.surface == nil {
			continue
		}
		left, top := seedMapFloor(tile.Key.X*span-x, v.BlocksPerPixel), seedMapFloor(tile.Key.Z*span-z, v.BlocksPerPixel)
		for tz := max(0, -top); tz < min(128, v.Height-top); tz++ {
			for tx := max(0, -left); tx < min(128, v.Width-left); tx++ {
				i := (tz+1)*seedMapSurfaceWidth + tx + 1
				cell := tile.surface[i]
				for _, delta := range []int{1, seedMapSurfaceWidth} {
					other := tile.surface[i-delta]
					boundary := cell.Biome != other.Biome
					if boundary || math.Floor(float64(cell.Height)) != math.Floor(float64(other.Height)) {
						out = append(out, SeedMapContour{X: int16(left + tx), Z: int16(top + tz), Horizontal: delta == seedMapSurfaceWidth, Light: !boundary && cell.Height < other.Height, Boundary: boundary})
					}
				}
			}
		}
	}
	return out
}

// SeedMapPoint describes the sample actually displayed, with an optional height
// estimate. It is read from immutable tiles without synchronous generation.
type SeedMapPoint struct {
	X, Y, Z int
	Biome   SeedMapBiome
	Height  float32
	Surface bool
	Void    bool // final_density relief: no ground in this column
}

// PointAt returns the displayed pixel at frame-local raster coordinates.
// Missing progressive tiles have no point; no placeholder biome is returned.
func (f SeedMapFrame) PointAt(px, pz int) (SeedMapPoint, bool) {
	v := f.Viewport
	if px < 0 || pz < 0 || px >= v.Width || pz >= v.Height || v.BlocksPerPixel < 1 {
		return SeedMapPoint{}, false
	}
	span := SeedMapTilePixels * v.BlocksPerPixel
	x, z := v.Origin()
	for _, tile := range f.tiles {
		left, top := seedMapFloor(tile.Key.X*span-x, v.BlocksPerPixel), seedMapFloor(tile.Key.Z*span-z, v.BlocksPerPixel)
		tx, tz := px-left, pz-top
		if tx < 0 || tz < 0 || tx >= SeedMapTilePixels || tz >= SeedMapTilePixels {
			continue
		}
		p := SeedMapPoint{X: tile.Key.X*span + tx*v.BlocksPerPixel, Z: tile.Key.Z*span + tz*v.BlocksPerPixel, Y: v.Y}
		var id cubiomes.BiomeID
		if len(tile.surface) > 0 {
			cell := tile.surface[(tz+1)*seedMapSurfaceWidth+tx+1]
			p.Height, p.Surface, id = cell.Height, true, cell.Biome
			if math.IsNaN(float64(cell.Height)) {
				p.Height, p.Surface, p.Void = 0, false, true
			}
		} else if len(tile.ids) > 0 {
			id = tile.ids[tz*SeedMapTilePixels+tx]
		} else {
			return SeedMapPoint{}, false
		}
		if int(id) >= len(tile.palette) || tile.palette[id].ID == "" {
			return SeedMapPoint{}, false
		}
		p.Biome = tile.palette[id]
		return p, true
	}
	return SeedMapPoint{}, false
}
