package structure3d

import (
	"image"
	"image/color"
	"math"
	"slices"
)

// Camera is an orbit camera around the structure centroid.
type Camera struct {
	Yaw, Pitch, Zoom float64 // radians; Zoom is pixels per block
	PanX, PanY       float64 // screen offset of the orbit centre, in pixels

	// Persp is the eye distance in blocks for a perspective view; 0 keeps
	// the orthographic view.
	Persp float64
	// Cut hides every face lying entirely above world Y CutY, like a
	// cross-section through the ground.
	Cut  bool
	CutY float64
	// Aim is the orbit centre in world blocks when Aimed; otherwise the
	// mesh centre.
	Aimed bool
	Aim   [3]float64
	// HideTerrain drops the terrain slab (X-ray), keeping the structure.
	HideTerrain bool
	// Boxes draws the piece bounding boxes instead of the blocks
	// (结构框 mode); the terrain slab stays as it is.
	Boxes bool
	// Select highlights one block: its 1-based index in the scene, 0 none.
	Select int32
	// Ghost draws the terrain the cut took away as a faint see-through
	// layer with this opacity (0 hides it), so it never covers the
	// structure.
	Ghost float64
}

// DefaultCamera looks from the south-east, slightly above.
func DefaultCamera() Camera {
	return Camera{Yaw: 0.6, Pitch: 0.55, Zoom: 18}
}

// Scene is world-space placed blocks ready to render.
type Scene struct {
	Blocks       []WorldBlock
	Min, Max     [3]int
	LayoutSeeded bool // piece layout seed-exact, blocks illustrative
	Seeded       bool // placement reproduced from the world seed, verified against saves
	// Pieces are the placed pieces' boxes; empty means one box around
	// every block.
	Pieces []PieceBox
	// Carved are positions the structure set to air: vanilla cuts these out
	// of the terrain it builds into.
	Carved [][3]int
	// StartPieces, when set, are the boxes as first saved in STRUCTURE_STARTS,
	// before postProcess reset them (chunks not yet decorated keep these).
	StartPieces []PieceBox
}

// WorldBlock is one block in assembled world coordinates.
type WorldBlock struct {
	X, Y, Z int
	State   BlockState
}

// Renderer draws a Scene to RGBA with a per-pixel depth buffer.
type Renderer struct {
	Store  *Store
	cache  *modelCache
	Width  int
	Height int
}

func NewRenderer(store *Store, w, h int) *Renderer {
	return &Renderer{Store: store, cache: newModelCache(), Width: w, Height: h}
}

// Render draws the scene. Empty scenes yield a blank image.
func (r *Renderer) Render(scene *Scene, cam Camera) *image.RGBA {
	w, h := r.Width, r.Height
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 0xe8, 0xec, 0xef, 0xff
	}
	if scene == nil || len(scene.Blocks) == 0 {
		return img
	}
	depth := make([]float32, w*h)
	for i := range depth {
		depth[i] = float32(math.Inf(1))
	}
	cx := float64(scene.Min[0]+scene.Max[0]) / 2
	cy := float64(scene.Min[1]+scene.Max[1]) / 2
	cz := float64(scene.Min[2]+scene.Max[2]) / 2
	sy, cyaw := math.Sin(cam.Yaw), math.Cos(cam.Yaw)
	sp, cp := math.Sin(cam.Pitch), math.Cos(cam.Pitch)
	zoom := cam.Zoom
	if zoom <= 0 {
		zoom = 16
	}
	ox, oy := float64(w)/2, float64(h)*0.55

	project := func(x, y, z float64) (sx, sy2, zdepth float64) {
		x -= cx
		y -= cy
		z -= cz
		// yaw around Y, then pitch around X
		xr := x*cyaw + z*sy
		zr := -x*sy + z*cyaw
		yr := y*cp - zr*sp
		zr = y*sp + zr*cp
		return ox + xr*zoom, oy - yr*zoom - zr*zoom*0.15, zr
	}

	shade := map[string]float64{"up": 1, "down": 0.55, "north": 0.8, "south": 0.75, "west": 0.65, "east": 0.7}
	type transparentFace struct {
		quad                  [4][3]float64
		tex                   image.Image
		face                  Face
		shade, x, y, z, depth float64
	}
	var transparent []transparentFace
	fluids := map[[3]int]string{}
	for _, b := range scene.Blocks {
		if kind := fluidKind(b.State.Name); kind != "" {
			fluids[[3]int{b.X, b.Y, b.Z}] = kind
		}
	}
	for _, b := range scene.Blocks {
		model := r.cache.resolve(r.Store, b.State)
		if model == nil {
			continue
		}
		for _, el := range model.Elements {
			for _, face := range el.Faces {
				if model.Fluid != "" {
					o := dirOffset[face.Dir]
					if fluids[[3]int{b.X + o[0], b.Y + o[1], b.Z + o[2]}] == model.Fluid {
						continue
					}
				}
				if face.Blend {
					n := dirNormal[face.Dir]
					if float64(n[1])*sp+(-float64(n[0])*sy+float64(n[2])*cyaw)*cp > 0 {
						continue
					}
				}
				quad := faceQuad(el, face.Dir)
				tex := face.Texture
				if face.TintIndex >= 0 {
					tex = tinted(tex, BiomeTint(face.TintIndex))
				}
				sh := shade[face.Dir]
				if sh == 0 {
					sh = 0.8
				}
				if face.Emissive {
					sh = 1
				}
				if face.Blend {
					var d float64
					for _, p := range quad {
						_, _, z := project(float64(b.X)+p[0], float64(b.Y)+p[1], float64(b.Z)+p[2])
						d += z / 4
					}
					transparent = append(transparent, transparentFace{quad, tex, face, sh, float64(b.X), float64(b.Y), float64(b.Z), d})
					continue
				}
				drawTexturedQuad(img, depth, project, quad, tex, face, sh, float64(b.X), float64(b.Y), float64(b.Z))
			}
		}
	}
	slices.SortStableFunc(transparent, func(a, b transparentFace) int {
		if a.depth > b.depth {
			return -1
		}
		if a.depth < b.depth {
			return 1
		}
		return 0
	})
	for _, f := range transparent {
		drawTexturedQuad(img, depth, project, f.quad, f.tex, f.face, f.shade, f.x, f.y, f.z)
	}
	return img
}

func faceQuad(el Element, dir string) [4][3]float64 {
	x0, y0, z0 := el.From[0]/16, el.From[1]/16, el.From[2]/16
	x1, y1, z1 := el.To[0]/16, el.To[1]/16, el.To[2]/16
	switch dir {
	case "up":
		return [4][3]float64{{x0, y1, z0}, {x1, y1, z0}, {x1, y1, z1}, {x0, y1, z1}}
	case "down":
		return [4][3]float64{{x0, y0, z1}, {x1, y0, z1}, {x1, y0, z0}, {x0, y0, z0}}
	case "north":
		return [4][3]float64{{x1, y0, z0}, {x0, y0, z0}, {x0, y1, z0}, {x1, y1, z0}}
	case "south":
		return [4][3]float64{{x0, y0, z1}, {x1, y0, z1}, {x1, y1, z1}, {x0, y1, z1}}
	case "west":
		return [4][3]float64{{x0, y0, z0}, {x0, y0, z1}, {x0, y1, z1}, {x0, y1, z0}}
	default: // east
		return [4][3]float64{{x1, y0, z1}, {x1, y0, z0}, {x1, y1, z0}, {x1, y1, z1}}
	}
}

func drawTexturedQuad(img *image.RGBA, depth []float32, project func(x, y, z float64) (float64, float64, float64),
	quad [4][3]float64, tex image.Image, face Face, shade, bx, by, bz float64) {
	if tex == nil {
		return
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	tb := tex.Bounds()
	tw, th := tb.Dx(), tb.Dy()
	if tw < 1 || th < 1 {
		return
	}
	type pt struct{ x, y, z, u, v float64 }
	uvs := [4][2]float64{{face.U0, face.V1}, {face.U1, face.V1}, {face.U1, face.V0}, {face.U0, face.V0}}
	var p [4]pt
	for i := 0; i < 4; i++ {
		sx, sy, zd := project(bx+quad[i][0], by+quad[i][1], bz+quad[i][2])
		p[i] = pt{sx, sy, zd, uvs[i][0] / 16, uvs[i][1] / 16}
	}
	// two triangles
	tris := [2][3]int{{0, 1, 2}, {0, 2, 3}}
	for _, t := range tris {
		a, b, c := p[t[0]], p[t[1]], p[t[2]]
		minX := int(math.Floor(min3(a.x, b.x, c.x)))
		maxX := int(math.Ceil(max3(a.x, b.x, c.x)))
		minY := int(math.Floor(min3(a.y, b.y, c.y)))
		maxY := int(math.Ceil(max3(a.y, b.y, c.y)))
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
		area := (b.x-a.x)*(c.y-a.y) - (c.x-a.x)*(b.y-a.y)
		if math.Abs(area) < 1e-6 {
			continue
		}
		for y := minY; y <= maxY; y++ {
			for x := minX; x <= maxX; x++ {
				px, py := float64(x)+0.5, float64(y)+0.5
				w0 := ((b.x-px)*(c.y-py) - (c.x-px)*(b.y-py)) / area
				w1 := ((c.x-px)*(a.y-py) - (a.x-px)*(c.y-py)) / area
				w2 := 1 - w0 - w1
				if w0 < 0 || w1 < 0 || w2 < 0 {
					continue
				}
				z := w0*a.z + w1*b.z + w2*c.z
				di := y*w + x
				if float32(z) >= depth[di] {
					continue
				}
				u := w0*a.u + w1*b.u + w2*c.u
				v := w0*a.v + w1*b.v + w2*c.v
				tx := tb.Min.X + int(math.Floor(fract(u)*float64(tw)))
				ty := tb.Min.Y + int(math.Floor(fract(v)*float64(th)))
				cr, cg, cb, ca := tex.At(tx, ty).RGBA()
				if ca == 0 || !face.Blend && ca < 0x8000 {
					continue
				}
				depth[di] = float32(z)
				if face.Blend {
					alpha := float64(ca) / 65535
					cr, cg, cb = cr*65535/ca, cg*65535/ca, cb*65535/ca
					old := img.RGBAAt(x, y)
					img.SetRGBA(x, y, color.RGBA{
						uint8(float64(old.R)*(1-alpha) + float64(cr>>8)*shade*alpha),
						uint8(float64(old.G)*(1-alpha) + float64(cg>>8)*shade*alpha),
						uint8(float64(old.B)*(1-alpha) + float64(cb>>8)*shade*alpha), 255})
					continue
				}
				img.SetRGBA(x, y, color.RGBA{
					uint8(float64(cr>>8) * shade),
					uint8(float64(cg>>8) * shade),
					uint8(float64(cb>>8) * shade),
					255,
				})
			}
		}
	}
}

func fract(v float64) float64 {
	v = v - math.Floor(v)
	if v < 0 {
		v += 1
	}
	return v
}

func min3(a, b, c float64) float64 { return math.Min(a, math.Min(b, c)) }
func max3(a, b, c float64) float64 { return math.Max(a, math.Max(b, c)) }

// BuildScene packs placed blocks into a bounded scene.
func BuildScene(blocks []WorldBlock) *Scene {
	if len(blocks) == 0 {
		return &Scene{}
	}
	s := &Scene{Blocks: blocks, Min: [3]int{blocks[0].X, blocks[0].Y, blocks[0].Z}, Max: sMax(blocks[0])}
	for _, b := range blocks[1:] {
		if b.X < s.Min[0] {
			s.Min[0] = b.X
		}
		if b.Y < s.Min[1] {
			s.Min[1] = b.Y
		}
		if b.Z < s.Min[2] {
			s.Min[2] = b.Z
		}
		if b.X > s.Max[0] {
			s.Max[0] = b.X
		}
		if b.Y > s.Max[1] {
			s.Max[1] = b.Y
		}
		if b.Z > s.Max[2] {
			s.Max[2] = b.Z
		}
	}
	return s
}

func sMax(b WorldBlock) [3]int { return [3]int{b.X, b.Y, b.Z} }
