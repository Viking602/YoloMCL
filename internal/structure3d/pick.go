package structure3d

import "math"

// PieceBox is one placed piece's box in world blocks (inclusive corners).
type PieceBox struct {
	Min, Max [3]int
	Name     string // template location; "" for a whole-structure box
	Depth    int
}

// Picked is the block under a pixel of the last frame.
type Picked struct {
	Index   int32 // 1-based, the value Camera.Select takes
	X, Y, Z int
	Name    string
}

// PickAt reports the structure block drawn at pixel (x, y) of f, the frame
// last rendered from m. Terrain and background report false.
func (m *Mesh) PickAt(f *Frame, x, y int) (Picked, bool) {
	if f == nil || f.Image == nil {
		return Picked{}, false
	}
	w, h := f.Image.Rect.Dx(), f.Image.Rect.Dy()
	if x < 0 || y < 0 || x >= w || y >= h {
		return Picked{}, false
	}
	id := f.ids[y*w+x]
	if id <= 0 || int(id) > len(m.blocks) {
		return Picked{}, false
	}
	b := m.blocks[id-1]
	return Picked{Index: id, X: b.X, Y: b.Y, Z: b.Z, Name: b.State.Name}, true
}

// Block returns the scene block with a 1-based index.
func (m *Mesh) Block(index int32) (Picked, bool) {
	if index <= 0 || int(index) > len(m.blocks) {
		return Picked{}, false
	}
	b := m.blocks[index-1]
	return Picked{Index: index, X: b.X, Y: b.Y, Z: b.Z, Name: b.State.Name}, true
}

// Project maps a world point to frame pixels of a w×h render with cam,
// the same transform Render uses. ok is false behind the eye.
func (m *Mesh) Project(cam Camera, w, h int, wx, wy, wz float64) (sx, sy float64, ok bool) {
	zoom := cam.Zoom
	if zoom <= 0 {
		zoom = 16
	}
	ox, oy := float64(w)/2+cam.PanX, float64(h)*0.55+cam.PanY
	x, y, z := wx-m.Center[0], wy-m.Center[1], wz-m.Center[2]
	if cam.Aimed {
		x, y, z = wx-cam.Aim[0], wy-cam.Aim[1], wz-cam.Aim[2]
	}
	sy0, cy0 := math.Sin(cam.Yaw), math.Cos(cam.Yaw)
	sp, cp := math.Sin(cam.Pitch), math.Cos(cam.Pitch)
	xr := x*cy0 + z*sy0
	zr := -x*sy0 + z*cy0
	yr := y*cp - zr*sp
	zr = y*sp + zr*cp
	if cam.Persp > 0 {
		d := zr + cam.Persp
		if d < 1 {
			return 0, 0, false
		}
		f := cam.Persp / d
		return ox + xr*zoom*f, oy - yr*zoom*f, true
	}
	return ox + xr*zoom, oy - yr*zoom - zr*zoom*0.15, true
}

// AxisDirs are the on-screen directions (unit-ish, y down) of world +X,
// +Y and +Z for cam, for an axis gizmo.
func AxisDirs(cam Camera) [3][2]float64 {
	m := &Mesh{}
	c := cam
	c.Aimed, c.Aim, c.PanX, c.PanY, c.Persp, c.Zoom = false, [3]float64{}, 0, 0, 0, 1
	ox, oy, _ := m.Project(c, 0, 0, 0, 0, 0)
	var out [3][2]float64
	for i, v := range [3][3]float64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}} {
		x, y, _ := m.Project(c, 0, 0, v[0], v[1], v[2])
		out[i] = [2]float64{x - ox, y - oy}
	}
	return out
}

// drawBoxes draws each piece box's twelve edges over the frame (结构框
// mode). Lines are not depth-tested so every piece stays readable.
func (m *Mesh) drawBoxes(f *Frame, w, h int, cam Camera) {
	pix := f.Image.Pix
	for i, b := range m.Pieces {
		x0, y0, z0 := float64(b.Min[0]), float64(b.Min[1]), float64(b.Min[2])
		x1, y1, z1 := float64(b.Max[0]+1), float64(b.Max[1]+1), float64(b.Max[2]+1)
		c := [8][3]float64{{x0, y0, z0}, {x1, y0, z0}, {x1, y0, z1}, {x0, y0, z1}, {x0, y1, z0}, {x1, y1, z0}, {x1, y1, z1}, {x0, y1, z1}}
		var s [8][2]float64
		vis := true
		for k := range c {
			x, y, ok := m.Project(cam, w, h, c[k][0], c[k][1], c[k][2])
			if !ok {
				vis = false
				break
			}
			s[k] = [2]float64{x, y}
		}
		if !vis {
			continue
		}
		col := boxColor(i)
		for _, e := range [12][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 0}, {4, 5}, {5, 6}, {6, 7}, {7, 4}, {0, 4}, {1, 5}, {2, 6}, {3, 7}} {
			line(pix, w, h, s[e[0]], s[e[1]], col)
		}
	}
}

func boxColor(i int) [3]uint8 {
	pal := [...][3]uint8{{0x2f, 0x6f, 0xeb}, {0xe0, 0x6c, 0x2f}, {0x2f, 0xa8, 0x6a}, {0xb0, 0x4a, 0xd6}, {0xd6, 0x3f, 0x5d}, {0x1f, 0x9a, 0xb0}}
	return pal[i%len(pal)]
}

func line(pix []uint8, w, h int, a, b [2]float64, col [3]uint8) {
	dx, dy := b[0]-a[0], b[1]-a[1]
	n := int(math.Max(math.Abs(dx), math.Abs(dy)))
	if n > 4*(w+h) {
		return // a degenerate projection; skip rather than stall
	}
	for i := 0; i <= n; i++ {
		t := 0.0
		if n > 0 {
			t = float64(i) / float64(n)
		}
		x, y := int(a[0]+dx*t), int(a[1]+dy*t)
		if x < 0 || y < 0 || x >= w || y >= h {
			continue
		}
		o := (y*w + x) * 4
		pix[o], pix[o+1], pix[o+2], pix[o+3] = col[0], col[1], col[2], 255
	}
}
