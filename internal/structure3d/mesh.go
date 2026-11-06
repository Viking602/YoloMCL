package structure3d

import (
	"image"
	"math"
	"sync"
)

// texel is a decoded, pre-tinted texture with direct pixel access.
type texel struct {
	w, h   int
	pix    []uint8 // RGBA, straight alpha
	opaque bool    // every pixel alpha >= 128
}

type texKey struct {
	img  image.Image
	tint int
}

type meshQuad struct {
	p       [4][3]float32 // world space, relative to mesh centre
	uv      [4][2]float32 // 0..1
	n       [3]float32    // face normal
	tex     *texel
	shade   float32
	flat    bool     // face of a closed cuboid; plane elements stay double-sided
	top     float32  // highest vertex Y, relative to the mesh centre
	terrain bool     // part of the terrain slab, not the structure
	block   int32    // 1-based scene block index; 0 for terrain
	col     [2]int32 // terrain top: world X, Z of the column it covers
	// floating terrain tops (End islands, Nether ceiling) know their
	// column's floor, so a cut below the floor drops the column instead
	// of pressing it down onto the cut.
	floating bool
	bot      float32 // column floor Y, relative to the mesh centre
	extra    bool    // a neighbouring structure merged in; not the subject
}

// Mesh is a scene baked once: visible faces only, textures decoded and
// tinted, so a camera change only rasterizes.
type Mesh struct {
	quads  []meshQuad
	Blocks int
	Culled int // faces dropped because a full opaque neighbour hides them

	Center     [3]float64 // world position the quads are relative to
	MinY, MaxY int        // world Y range of the structure blocks
	HasTerrain bool

	// section colours the ground at a world block for cut faces; nil keeps
	// plain stone.
	section    func(x, y, z int) [3]uint8
	texGround  TerrainTextures
	solidMu    sync.Mutex
	solidTex   map[[3]uint8]*texel
	TerrainTop int // highest terrain column, world Y

	Pieces []PieceBox   // piece boxes, world blocks (inclusive)
	blocks []WorldBlock // the scene's blocks, for picking

	// carved is the structure's air per column (Scene.Carved): terrain
	// columns whose top it opens are lowered to the carved floor.
	carved map[[2]int]map[int]bool
	// CarveThrough opens a terrain column down to its lowest carved block,
	// not only through carved air that reaches the top: a structure buried
	// in rock (Nether fortress) shows in its own carved volume.
	CarveThrough bool
	footprint    map[[2]int]int // lowest structure block or carved air per column
}

// Fork shares baked geometry for reading, while append-only additions and
// terrain caches belong to the new mesh. This lets the structure be rendered
// while its surroundings are built. Neither mesh may edit existing quads.
func (m *Mesh) Fork() *Mesh {
	return &Mesh{
		quads: m.quads[:len(m.quads):len(m.quads)], Blocks: m.Blocks, Culled: m.Culled,
		Center: m.Center, MinY: m.MinY, MaxY: m.MaxY, HasTerrain: m.HasTerrain,
		section: m.section, texGround: m.texGround, TerrainTop: m.TerrainTop,
		Pieces: m.Pieces[:len(m.Pieces):len(m.Pieces)], blocks: m.blocks[:len(m.blocks):len(m.blocks)],
		carved: m.carved, CarveThrough: m.CarveThrough, footprint: m.footprint,
	}
}

// carveTop lowers a terrain column top h (world Y of the top block) through
// the structure's carved air that reaches it, as vanilla pieces cut the
// ground they build into (mineshaft shafts, temple pits, stronghold rooms
// crossing the surface).
func (m *Mesh) carveTop(x, z, h int) int {
	col := m.carved[[2]int{x, z}]
	if h == TerrainVoid {
		return h
	}
	if m.CarveThrough {
		if low, ok := m.footprintLow(x, z); ok && low <= h {
			return low - 1
		}
	}
	if col == nil {
		return h
	}
	if m.CarveThrough {
		if low, ok := m.footprintLow(x, z); ok && low <= h {
			return low - 1
		}
	}
	for col[h] {
		h--
	}
	return h
}

// footprintLow is the lowest structure block or carved air in column x, z.
func (m *Mesh) footprintLow(x, z int) (int, bool) {
	if m.footprint == nil {
		m.footprint = map[[2]int]int{}
		note := func(x, y, z int) {
			k := [2]int{x, z}
			if v, ok := m.footprint[k]; !ok || y < v {
				m.footprint[k] = y
			}
		}
		for _, b := range m.blocks {
			note(b.X, b.Y, b.Z)
		}
		for k, col := range m.carved {
			for y := range col {
				note(k[0], y, k[1])
			}
		}
	}
	v, ok := m.footprint[[2]int{x, z}]
	return v, ok
}

// carvedIn reports a structure block or carved air in column x, z at or
// below hi whose column reaches lo (the layer the structure cuts through).
func (m *Mesh) carvedIn(x, z, lo, hi int) bool {
	low, ok := m.footprintLow(x, z)
	return ok && low <= hi
}

// Extent is the horizontal half-size of the structure quads around Center,
// in blocks (terrain excluded).
func (m *Mesh) Extent() float64 {
	var e float32
	for i := range m.quads {
		q := &m.quads[i]
		if q.terrain || q.extra {
			continue
		}
		for _, v := range q.p {
			e = max(e, abs32(v[0]), abs32(v[2]))
		}
	}
	return float64(e)
}

var sectionTexel = &texel{w: 1, h: 1, pix: []uint8{terrainStone[0], terrainStone[1], terrainStone[2], 255}, opaque: true}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

var dirNormal = map[string][3]float32{"up": {0, 1, 0}, "down": {0, -1, 0}, "north": {0, 0, -1}, "south": {0, 0, 1}, "west": {-1, 0, 0}, "east": {1, 0, 0}}
var dirOffset = map[string][3]int{"up": {0, 1, 0}, "down": {0, -1, 0}, "north": {0, 0, -1}, "south": {0, 0, 1}, "west": {-1, 0, 0}, "east": {1, 0, 0}}
var dirShade = map[string]float32{"up": 1, "down": 0.55, "north": 0.8, "south": 0.75, "west": 0.65, "east": 0.7}

func toTexel(img image.Image) *texel {
	b := img.Bounds()
	t := &texel{w: b.Dx(), h: b.Dy(), pix: make([]uint8, b.Dx()*b.Dy()*4), opaque: true}
	i := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bb, a := img.At(x, y).RGBA()
			if a > 0 && a < 0xffff { // un-premultiply
				r, g, bb = r*0xffff/a, g*0xffff/a, bb*0xffff/a
			}
			t.pix[i], t.pix[i+1], t.pix[i+2], t.pix[i+3] = uint8(r>>8), uint8(g>>8), uint8(bb>>8), uint8(a>>8)
			if a < 0x8000 {
				t.opaque = false
			}
			i += 4
		}
	}
	return t
}

// BuildMesh resolves models and drops faces hidden by full opaque cubes.
func BuildMesh(store *Store, scene *Scene) *Mesh {
	return newMeshBuilder(store).build(scene)
}

type meshBuilder struct {
	store *Store
	cache *modelCache
	tex   map[texKey]*texel
}

func newMeshBuilder(store *Store) *meshBuilder {
	return &meshBuilder{store: store, cache: newModelCache(), tex: map[texKey]*texel{}}
}

func (mb *meshBuilder) texel(img image.Image, tint int) *texel {
	if img == nil {
		return nil
	}
	k := texKey{img, tint}
	if t, ok := mb.tex[k]; ok {
		return t
	}
	src := img
	if tint >= 0 {
		src = tinted(img, BiomeTint(tint))
	}
	t := toTexel(src)
	if t.w < 1 || t.h < 1 {
		t = nil
	}
	mb.tex[k] = t
	return t
}

func (mb *meshBuilder) fullCube(m *ResolvedModel) bool {
	for _, el := range m.Elements {
		if el.From != [3]float64{0, 0, 0} || el.To != [3]float64{16, 16, 16} || len(el.Faces) < 6 {
			continue
		}
		ok := true
		for _, f := range el.Faces {
			t := mb.texel(f.Texture, f.TintIndex)
			if t == nil || !t.opaque {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func onBoundary(el Element, dir string) bool {
	switch dir {
	case "up":
		return el.To[1] >= 16
	case "down":
		return el.From[1] <= 0
	case "north":
		return el.From[2] <= 0
	case "south":
		return el.To[2] >= 16
	case "west":
		return el.From[0] <= 0
	default:
		return el.To[0] >= 16
	}
}

func (mb *meshBuilder) build(scene *Scene) *Mesh {
	m := &Mesh{}
	if scene == nil || len(scene.Blocks) == 0 {
		return m
	}
	m.Blocks = len(scene.Blocks)
	m.MinY, m.MaxY = scene.Min[1], scene.Max[1]
	m.blocks = scene.Blocks
	m.Pieces = scene.Pieces
	if len(scene.Carved) > 0 {
		m.carved = map[[2]int]map[int]bool{}
		for _, p := range scene.Carved {
			k := [2]int{p[0], p[2]}
			if m.carved[k] == nil {
				m.carved[k] = map[int]bool{}
			}
			m.carved[k][p[1]] = true
		}
	}
	if len(m.Pieces) == 0 {
		m.Pieces = []PieceBox{{Min: scene.Min, Max: scene.Max}}
	}
	models := make([]*ResolvedModel, len(scene.Blocks))
	full := make(map[[3]int]bool, len(scene.Blocks))
	for i, b := range scene.Blocks {
		models[i] = mb.cache.resolve(mb.store, b.State)
		if models[i] != nil && mb.fullCube(models[i]) {
			full[[3]int{b.X, b.Y, b.Z}] = true
		}
	}
	cx := float64(scene.Min[0]+scene.Max[0]) / 2
	cy := float64(scene.Min[1]+scene.Max[1]) / 2
	cz := float64(scene.Min[2]+scene.Max[2]) / 2
	m.Center = [3]float64{cx, cy, cz}
	for i, b := range scene.Blocks {
		model := models[i]
		if model == nil {
			continue
		}
		for _, el := range model.Elements {
			for _, face := range el.Faces {
				if onBoundary(el, face.Dir) {
					o := dirOffset[face.Dir]
					if full[[3]int{b.X + o[0], b.Y + o[1], b.Z + o[2]}] {
						m.Culled++
						continue
					}
				}
				t := mb.texel(face.Texture, face.TintIndex)
				if t == nil {
					continue
				}
				q := faceQuad(el, face.Dir)
				uvs := [4][2]float64{{face.U0, face.V1}, {face.U1, face.V1}, {face.U1, face.V0}, {face.U0, face.V0}}
				var mq meshQuad
				for k := 0; k < 4; k++ {
					mq.p[k] = [3]float32{float32(float64(b.X) + q[k][0] - cx), float32(float64(b.Y) + q[k][1] - cy), float32(float64(b.Z) + q[k][2] - cz)}
					mq.uv[k] = [2]float32{float32(uvs[k][0] / 16), float32(uvs[k][1] / 16)}
				}
				mq.n = dirNormal[face.Dir]
				mq.flat = el.From[0] != el.To[0] && el.From[1] != el.To[1] && el.From[2] != el.To[2]
				mq.tex = t
				mq.block = int32(i + 1)
				mq.top = max(mq.p[0][1], mq.p[1][1], mq.p[2][1], mq.p[3][1])
				mq.shade = dirShade[face.Dir]
				if mq.shade == 0 {
					mq.shade = 0.8
				}
				m.quads = append(m.quads, mq)
			}
		}
	}
	return m
}

// Faces is the number of quads the mesh rasterizes per frame (before backface culling).
func (m *Mesh) Faces() int { return len(m.quads) }

// Frame owns reusable buffers so steady-state rendering does not allocate.
type Frame struct {
	Image *image.RGBA
	depth []float32
	ids   []int32 // block under each pixel, for picking
	ghost []int   // scratch reused across orbit frames
	// Cross-section material depends on the cut height, not the camera.
	sectionMesh *Mesh
	sectionY    int
	sectionTex  []*texel
}

func (f *Frame) ensure(w, h int) {
	if f.Image == nil || f.Image.Rect.Dx() != w || f.Image.Rect.Dy() != h {
		f.Image = image.NewRGBA(image.Rect(0, 0, w, h))
		f.depth = make([]float32, w*h)
		f.ids = make([]int32, w*h)
	}
}

var bgPix = [4]uint8{0xe8, 0xec, 0xef, 0xff}

// Render rasterizes the mesh into f at w×h. zoom is in full-size pixels per
// block; scale shrinks everything for a low-resolution interactive frame.
func (m *Mesh) Render(f *Frame, w, h int, cam Camera) *image.RGBA {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	f.ensure(w, h)
	pix := f.Image.Pix
	for i := 0; i < len(pix); i += 4 {
		pix[i], pix[i+1], pix[i+2], pix[i+3] = bgPix[0], bgPix[1], bgPix[2], bgPix[3]
	}
	inf := float32(math.Inf(1))
	for i := range f.depth {
		f.depth[i] = inf
		f.ids[i] = 0
	}
	sy, cyaw := float32(math.Sin(cam.Yaw)), float32(math.Cos(cam.Yaw))
	sp, cp := float32(math.Sin(cam.Pitch)), float32(math.Cos(cam.Pitch))
	zoom := float32(cam.Zoom)
	if zoom <= 0 {
		zoom = 16
	}
	ox, oy := float32(w)/2+float32(cam.PanX), float32(h)*0.55+float32(cam.PanY)
	var ax, ay, az float32
	if cam.Aimed {
		ax, ay, az = float32(cam.Aim[0]-m.Center[0]), float32(cam.Aim[1]-m.Center[1]), float32(cam.Aim[2]-m.Center[2])
	}
	persp := float32(cam.Persp)
	cutY := float32(cam.CutY - m.Center[1])
	type pt struct{ x, y, z, u, v float32 }
	var p [4]pt
	f.ghost = f.ghost[:0] // terrain the cut took away, drawn faintly last
	sectionY := int(math.Floor(cam.CutY)) - 1
	if cam.Cut && m.section != nil {
		if f.sectionMesh != m || f.sectionY != sectionY || len(f.sectionTex) != len(m.quads) {
			if cap(f.sectionTex) < len(m.quads) {
				f.sectionTex = make([]*texel, len(m.quads))
			} else {
				f.sectionTex = f.sectionTex[:len(m.quads)]
				clear(f.sectionTex)
			}
			f.sectionMesh, f.sectionY = m, sectionY
		}
	}
	project := func(q *meshQuad, clip bool) bool {
		for k := 0; k < 4; k++ {
			x, y, z := q.p[k][0]-ax, q.p[k][1]-ay, q.p[k][2]-az
			if clip && y+ay > cutY {
				y = cutY - ay
			}
			xr := x*cyaw + z*sy
			zr := -x*sy + z*cyaw
			yr := y*cp - zr*sp
			zr = y*sp + zr*cp
			if persp > 0 {
				d := zr + persp
				if d < 1 {
					return false
				}
				f := persp / d
				p[k] = pt{ox + xr*zoom*f, oy - yr*zoom*f, zr, q.uv[k][0], q.uv[k][1]}
				continue
			}
			p[k] = pt{ox + xr*zoom, oy - yr*zoom - zr*zoom*0.15, zr, q.uv[k][0], q.uv[k][1]}
		}
		return max(p[0].x, p[1].x, p[2].x, p[3].x) >= 0 && min(p[0].x, p[1].x, p[2].x, p[3].x) < float32(w) &&
			max(p[0].y, p[1].y, p[2].y, p[3].y) >= 0 && min(p[0].y, p[1].y, p[2].y, p[3].y) < float32(h)
	}
quads:
	for qi := range m.quads {
		q := &m.quads[qi]
		if q.terrain && cam.HideTerrain || !q.terrain && cam.Boxes {
			continue
		}
		if cam.Ghost > 0 && cam.Cut && q.terrain && q.top > cutY+0.001 && q.n[1] >= 0 {
			f.ghost = append(f.ghost, qi)
		}
		// The cut only takes terrain away, never the structure. Above it
		// terrain walls end at the cut
		// and terrain tops are pressed down onto it, so the slab reads as
		// a solid cross-section instead of showing its far wall through
		// a hole.
		section := false
		if cam.Cut && q.terrain && q.top > cutY+0.001 {
			if q.n[1] < 0 || (q.n[1] == 0 && q.p[0][1] >= cutY) {
				continue
			}
			section = q.n[1] > 0
			if q.floating && q.bot >= cutY {
				continue
			}
		}
		// In perspective, test against the eye rather than parallel rays.
		// Plane elements stay double-sided, as with the orthographic view.
		if q.flat {
			nz := q.n[1]*sp + (-q.n[0]*sy+q.n[2]*cyaw)*cp
			if persp > 0 {
				nz = nz*persp + q.n[0]*(q.p[0][0]-ax) + q.n[1]*(q.p[0][1]-ay) + q.n[2]*(q.p[0][2]-az)
			}
			if nz*backfaceSign > 0 {
				continue
			}
		}
		if !project(q, cam.Cut && q.terrain) {
			continue quads // behind or too close to the eye
		}

		t := q.tex
		shade := q.shade
		if section {
			t, shade = sectionTexel, 0.8 // cut face: stone, not the grass or water on top
			if m.section != nil {
				t = f.sectionTex[qi]
				if t == nil {
					c := m.section(int(q.col[0]), sectionY, int(q.col[1]))
					t = pickTerrainTex(m.texGround, c, true)
					if t == nil {
						t = m.solid(c)
					}
					f.sectionTex[qi] = t
				}
				shade = 0.85
			}
		}
		hi := cam.Select != 0 && q.block == cam.Select
		rasterTri(f, w, h, p[0], p[1], p[2], t, shade, q.block, hi, 1)
		rasterTri(f, w, h, p[0], p[2], p[3], t, shade, q.block, hi, 1)
	}
	for _, qi := range f.ghost {
		q := &m.quads[qi]
		if !project(q, false) {
			continue
		}
		a := float32(cam.Ghost)
		rasterTri(f, w, h, p[0], p[1], p[2], q.tex, q.shade, 0, false, a)
		rasterTri(f, w, h, p[0], p[2], p[3], q.tex, q.shade, 0, false, a)
	}
	if cam.Boxes {
		m.drawBoxes(f, w, h, cam)
	}
	return f.Image
}

// backfaceSign is the winding sign of a back-facing quad on screen
// (verified by TestMeshMatchesReference: culled and unculled frames agree).
var backfaceSign float32 = 1

func rasterTri(f *Frame, w, h int, a, b, c struct{ x, y, z, u, v float32 }, t *texel, shade float32, id int32, hi bool, alpha float32) {
	minX := int(floor32(min(a.x, b.x, c.x)))
	maxX := int(ceil32(max(a.x, b.x, c.x)))
	minY := int(floor32(min(a.y, b.y, c.y)))
	maxY := int(ceil32(max(a.y, b.y, c.y)))
	if minX < 0 {
		minX = 0
	}
	if minY < 0 {
		minY = 0
	}
	if maxX >= w {
		maxX = w - 1
	}
	if maxY >= h {
		maxY = h - 1
	}
	if minX > maxX || minY > maxY {
		return
	}
	area := (b.x-a.x)*(c.y-a.y) - (c.x-a.x)*(b.y-a.y)
	if area > -1e-6 && area < 1e-6 {
		return
	}
	inv := 1 / area
	tw, th := float32(t.w), float32(t.h)
	pix, depth := f.Image.Pix, f.depth
	for y := minY; y <= maxY; y++ {
		py := float32(y) + 0.5
		row := y * w
		for x := minX; x <= maxX; x++ {
			px := float32(x) + 0.5
			w0 := ((b.x-px)*(c.y-py) - (c.x-px)*(b.y-py)) * inv
			if w0 < 0 {
				continue
			}
			w1 := ((c.x-px)*(a.y-py) - (a.x-px)*(c.y-py)) * inv
			if w1 < 0 {
				continue
			}
			w2 := 1 - w0 - w1
			if w2 < 0 {
				continue
			}
			z := w0*a.z + w1*b.z + w2*c.z
			di := row + x
			if z >= depth[di] {
				continue
			}
			u := w0*a.u + w1*b.u + w2*c.u
			v := w0*a.v + w1*b.v + w2*c.v
			u -= floor32(u)
			v -= floor32(v)
			tx, ty := int(u*tw), int(v*th)
			if tx >= t.w {
				tx = t.w - 1
			}
			if ty >= t.h {
				ty = t.h - 1
			}
			ti := (ty*t.w + tx) * 4
			if t.pix[ti+3] < 128 {
				continue
			}
			o := di * 4
			r, g, b := float32(t.pix[ti])*shade, float32(t.pix[ti+1])*shade, float32(t.pix[ti+2])*shade
			if alpha < 1 { // ghost: blend over what is drawn, keep its depth
				pix[o] = uint8(float32(pix[o])*(1-alpha) + r*alpha)
				pix[o+1] = uint8(float32(pix[o+1])*(1-alpha) + g*alpha)
				pix[o+2] = uint8(float32(pix[o+2])*(1-alpha) + b*alpha)
				continue
			}
			depth[di] = z
			f.ids[di] = id
			if hi { // selected block: pulled toward a warm highlight
				r, g, b = r*0.45+255*0.55, g*0.45+196*0.55, b*0.45+40*0.55
			}
			pix[o], pix[o+1], pix[o+2] = uint8(r), uint8(g), uint8(b)
			pix[o+3] = 255
		}
	}
}

func floor32(v float32) float32 {
	i := float32(int32(v))
	if i > v {
		i--
	}
	return i
}

func ceil32(v float32) float32 {
	i := float32(int32(v))
	if i < v {
		i++
	}
	return i
}

// solid is a one-colour texel, shared per colour.
func (m *Mesh) solid(c [3]uint8) *texel {
	m.solidMu.Lock()
	defer m.solidMu.Unlock()
	if m.solidTex == nil {
		m.solidTex = map[[3]uint8]*texel{}
	}
	if t, ok := m.solidTex[c]; ok {
		return t
	}
	t := &texel{w: 1, h: 1, pix: []uint8{c[0], c[1], c[2], 255}, opaque: true}
	m.solidTex[c] = t
	return t
}
