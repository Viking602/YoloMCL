package structure3d

import "math"

// Terrain is a slab of ground around a structure, drawn as solid-colour
// columns. Heights and colours come from the caller (the seed map's surface
// sampler), so the slab is illustrative, not block-exact.
type Terrain struct {
	X0, Z0, W, D int // world block bounds of the slab
	Step         int // column width in blocks (>= 1)
	Bottom       int // world Y of the slab floor
	Height       func(x, z int) int
	Color        func(x, z, h int) [3]uint8
	// Section colours the ground inside the slab at a world block (rock
	// layers, caves, ores). It paints the outer walls in YStep-high cells
	// and the faces the cut opens. Nil keeps plain bands.
	Section func(x, y, z int) [3]uint8
	YStep   int
	// Textures, when loaded from the asset store, tile across the faces
	// instead of the solid colour swatches.
	Textures TerrainTextures
	// Floor, when set, is the world Y a column's ground starts at (floating
	// End islands, the Nether ceiling); the slab then gets an underside.
	// Height returning TerrainVoid leaves the column empty.
	Floor func(x, z int) int
}

// TerrainVoid marks a column with no ground (the End's void).
const TerrainVoid = math.MinInt32

// Cross-section colours of the slab's outer walls.
var (
	terrainDirt      = [3]uint8{0x86, 0x60, 0x43}
	terrainStone     = [3]uint8{0x7d, 0x7d, 0x7d}
	terrainDeepslate = [3]uint8{0x4d, 0x4d, 0x52}
)

// AddTerrain appends the slab's top faces, the steps between columns and the
// four outer walls (dirt, stone and deepslate bands) to the mesh.
func (m *Mesh) AddTerrain(t Terrain) {
	if t.Step < 1 {
		t.Step = 1
	}
	if t.Height == nil || t.W < t.Step || t.D < t.Step {
		return
	}
	nx, nz := t.W/t.Step, t.D/t.Step
	h := make([]int, nx*nz)
	for i := 0; i < nx; i++ {
		for j := 0; j < nz; j++ {
			x, z := t.X0+i*t.Step+t.Step/2, t.Z0+j*t.Step+t.Step/2
			h[i*nz+j] = t.Height(x, z)
			if t.Floor == nil {
				h[i*nz+j] = m.carveTop(x, z, h[i*nz+j])
			} else if m.CarveThrough && m.carvedIn(x, z, t.Floor(x, z), h[i*nz+j]) {
				h[i*nz+j] = TerrainVoid // a floating layer (Nether ceiling) the structure cuts through
			}
		}
	}
	tex := m.solid
	m.section = t.Section
	m.texGround = t.Textures
	ys := max(t.YStep, 1)
	cx, cy, cz := float32(m.Center[0]), float32(m.Center[1]), float32(m.Center[2])
	add := func(p [4][3]float32, n [3]float32, c [3]uint8, shade float32, top bool) {
		var q meshQuad
		for k := range p {
			q.p[k] = [3]float32{p[k][0] - cx, p[k][1] - cy, p[k][2] - cz}
		}
		q.n, q.shade, q.terrain = n, shade, true
		if t := pickTerrainTex(t.Textures, c, top); t != nil {
			q.tex = t
		} else {
			q.tex = tex(c)
		}
		// A cut can reveal textured rock/ore beneath an untextured surface
		// (water, for example). Keep world-space UVs even for a solid top.
		for k := range p {
			if n[1] != 0 { // top / bottom: XZ
				q.uv[k] = [2]float32{p[k][0], p[k][2]}
			} else if n[0] != 0 { // east / west: ZY
				q.uv[k] = [2]float32{p[k][2], p[k][1]}
			} else { // north / south: XY
				q.uv[k] = [2]float32{p[k][0], p[k][1]}
			}
		}
		q.top = max(q.p[0][1], q.p[1][1], q.p[2][1], q.p[3][1])
		m.quads = append(m.quads, q)
	}
	wall := func(x0, z0, x1, z1 float32, y0, y1 int, n [3]float32, top [3]uint8, banded bool, shade float32) {
		if t.Section != nil {
			sx, sz := int(math.Floor(float64(x0+x1)/2)), int(math.Floor(float64(z0+z1)/2))
			if n[0] > 0 {
				sx-- // the column inside the wall, not the one beyond
			}
			if n[2] > 0 {
				sz--
			}
			if !banded {
				add([4][3]float32{{x0, float32(y0), z0}, {x1, float32(y0), z1}, {x1, float32(y1), z1}, {x0, float32(y1), z0}}, n, t.Section(sx, (y0+y1)/2, sz), shade, false)
				return
			}
			add([4][3]float32{{x0, float32(y1 - 1), z0}, {x1, float32(y1 - 1), z1}, {x1, float32(y1), z1}, {x0, float32(y1), z0}}, n, top, shade, false)
			for a := y1 - 1; a > y0; {
				b := a - (((a-1)%ys+ys)%ys + 1) // down to the cell floor
				b = max(b, y0)
				add([4][3]float32{{x0, float32(b), z0}, {x1, float32(b), z1}, {x1, float32(a), z1}, {x0, float32(a), z0}}, n, t.Section(sx, b, sz), shade, false)
				a = b
			}
			return
		}
		band := func(a, b int, c [3]uint8) {
			a, b = max(a, y0), min(b, y1)
			if a >= b {
				return
			}
			add([4][3]float32{{x0, float32(a), z0}, {x1, float32(a), z1}, {x1, float32(b), z1}, {x0, float32(b), z0}}, n, c, shade, false)
		}
		if !banded {
			band(y0, y1, top)
			return
		}
		band(y1-1, y1, top)
		band(y1-4, y1-1, terrainDirt)
		band(0, y1-4, terrainStone)
		band(y0, 0, terrainDeepslate)
	}
	color := func(x, z, hh int) [3]uint8 {
		if t.Color != nil {
			return t.Color(x, z, hh)
		}
		return [3]uint8{0x6d, 0x9a, 0x4a}
	}
	s := float32(t.Step)
	top := math.MinInt
	fls := make([]int, nx*nz)
	for i := range fls {
		fls[i] = t.Bottom
		if t.Floor != nil {
			fls[i] = t.Floor(t.X0+(i/nz)*t.Step+t.Step/2, t.Z0+(i%nz)*t.Step+t.Step/2)
		}
	}
	floor := func(i, j int) int { return fls[i*nz+j] }
	// A column with no thickness is void too, including at the slab floor.
	for i := 0; i < nx; i++ {
		for j := 0; j < nz; j++ {
			if h[i*nz+j] != TerrainVoid && floor(i, j) >= h[i*nz+j] {
				h[i*nz+j] = TerrainVoid
			}
		}
	}
	for i := 0; i < nx; i++ {
		for j := 0; j < nz; j++ {
			hh := h[i*nz+j]
			if hh == TerrainVoid {
				continue
			}
			fl := floor(i, j)
			top = max(top, hh)
			x0, z0 := float32(t.X0+i*t.Step), float32(t.Z0+j*t.Step)
			x1, z1 := x0+s, z0+s
			y := float32(hh)
			c := color(t.X0+i*t.Step+t.Step/2, t.Z0+j*t.Step+t.Step/2, hh) // the column h came from
			add([4][3]float32{{x0, y, z0}, {x1, y, z0}, {x1, y, z1}, {x0, y, z1}}, [3]float32{0, 1, 0}, c, 1, true)
			m.quads[len(m.quads)-1].col = [2]int32{int32(t.X0 + i*t.Step + t.Step/2), int32(t.Z0 + j*t.Step + t.Step/2)}
			if t.Floor != nil {
				m.quads[len(m.quads)-1].floating, m.quads[len(m.quads)-1].bot = true, float32(fl)-cy
			}
			if i+1 < nx && h[(i+1)*nz+j] < hh {
				wall(x1, z0, x1, z1, max(h[(i+1)*nz+j], fl), hh, [3]float32{1, 0, 0}, c, h[(i+1)*nz+j] == TerrainVoid, 0.7)
			}
			if i > 0 && h[(i-1)*nz+j] < hh {
				wall(x0, z0, x0, z1, max(h[(i-1)*nz+j], fl), hh, [3]float32{-1, 0, 0}, c, h[(i-1)*nz+j] == TerrainVoid, 0.65)
			}
			if j+1 < nz && h[i*nz+j+1] < hh {
				wall(x0, z1, x1, z1, max(h[i*nz+j+1], fl), hh, [3]float32{0, 0, 1}, c, h[i*nz+j+1] == TerrainVoid, 0.75)
			}
			if j > 0 && h[i*nz+j-1] < hh {
				wall(x0, z0, x1, z0, max(h[i*nz+j-1], fl), hh, [3]float32{0, 0, -1}, c, h[i*nz+j-1] == TerrainVoid, 0.8)
			}
			if i == 0 {
				wall(x0, z0, x0, z1, fl, hh, [3]float32{-1, 0, 0}, c, true, 0.65)
			}
			if i == nx-1 {
				wall(x1, z0, x1, z1, fl, hh, [3]float32{1, 0, 0}, c, true, 0.7)
			}
			if j == 0 {
				wall(x0, z0, x1, z0, fl, hh, [3]float32{0, 0, -1}, c, true, 0.8)
			}
			if j == nz-1 {
				wall(x0, z1, x1, z1, fl, hh, [3]float32{0, 0, 1}, c, true, 0.75)
			}
			// Close the volume so orbiting underneath cannot see the grass
			// through an open bottom. Cull these faces in views from above.
			f, bc := float32(fl), c
			if t.Floor == nil {
				switch {
				case fl < 0:
					bc = terrainDeepslate
				case fl >= hh-4:
					bc = terrainDirt
				default:
					bc = terrainStone
				}
			}
			if t.Section != nil {
				bc = t.Section(t.X0+i*t.Step+t.Step/2, fl, t.Z0+j*t.Step+t.Step/2)
			}
			add([4][3]float32{{x0, f, z1}, {x1, f, z1}, {x1, f, z0}, {x0, f, z0}}, [3]float32{0, -1, 0}, bc, 0.5, false)
			m.quads[len(m.quads)-1].flat = true
		}
	}
	if top == math.MinInt {
		top = t.Bottom
	}
	if m.HasTerrain {
		top = max(top, m.TerrainTop)
	}
	m.TerrainTop, m.HasTerrain = top, true
}

// Dive phases as fractions of the whole transition.
const (
	diveTiltEnd = 0.45 // the top-down map view has tilted into perspective
	diveCutEnd  = 0.85 // the cut plane has reached the structure
)

// DiveCamera is the camera at progress t (0..1) of the dive from a top-down
// view of the slab into the final orbit view. The cut plane sinks from above
// the terrain to cutY, cutting the surface away like a cross-section.
func DiveCamera(t float64, final Camera, terrainTop, cutY float64) Camera {
	t = clamp01(t)
	tilt := smooth(math.Min(1, t/diveTiltEnd))
	c := final
	c.Pitch = lerp(1.5, final.Pitch, tilt)
	c.Yaw = lerp(final.Yaw-0.5, final.Yaw, smooth(t))
	c.Zoom = lerp(final.Zoom*0.35, final.Zoom, smooth(t))
	c.Persp = lerp(900, final.Persp, tilt)
	c.Cut = true
	c.CutY = lerp(terrainTop+2, cutY, smooth(clamp01((t-diveTiltEnd)/(diveCutEnd-diveTiltEnd))))
	return c
}

// DiveCutY is the cut-plane height at progress t, for the height gauge.
func DiveCutY(t, terrainTop, cutY float64) float64 {
	return lerp(terrainTop+2, cutY, smooth(clamp01((clamp01(t)-diveTiltEnd)/(diveCutEnd-diveTiltEnd))))
}

// GlideCamera eases from one camera to another (flying to a selection).
func GlideCamera(t float64, from, to Camera) Camera {
	k := smooth(clamp01(t))
	c := to
	c.Yaw, c.Pitch, c.Zoom = lerp(from.Yaw, to.Yaw, k), lerp(from.Pitch, to.Pitch, k), lerp(from.Zoom, to.Zoom, k)
	c.PanX, c.PanY = lerp(from.PanX, to.PanX, k), lerp(from.PanY, to.PanY, k)
	c.Persp, c.CutY = lerp(from.Persp, to.Persp, k), lerp(from.CutY, to.CutY, k)
	for i := range c.Aim {
		c.Aim[i] = lerp(from.Aim[i], to.Aim[i], k)
	}
	return c
}

func lerp(a, b, k float64) float64 { return a + (b-a)*k }
func clamp01(v float64) float64    { return math.Max(0, math.Min(1, v)) }
func smooth(k float64) float64     { return k * k * (3 - 2*k) }
