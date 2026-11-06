package worldgen

import "math"

// Carver shapes reproduce the overworld cave, cave_extra_underground and
// canyon carvers (vanilla 1.18+ configured_carver data). Every overworld
// biome uses that same list, so placement needs no biome lookup. Only the
// tunnel geometry is seed-exact: whether a block really turns to air also
// depends on the replaceable tag, aquifers and later features.

var mthSin = func() (t [65536]float32) {
	for i := range t {
		t[i] = float32(math.Sin(float64(i) * math.Pi * 2 / 65536))
	}
	return
}()

func mSin(d float64) float32 { return mthSin[int64(d*10430.378350470453)&65535] }
func mCos(d float64) float32 { return mthSin[int64(d*10430.378350470453+16384)&65535] }

func mFloor(d float64) int {
	i := int(d)
	if d < float64(i) {
		return i - 1
	}
	return i
}

func newRandom(seed int64) *JavaRandom {
	r := &JavaRandom{}
	r.SetSeed(seed)
	return r
}

// CarveShape is one carveEllipsoid call.
type CarveShape struct {
	X, Y, Z, HR, VR float64
	Floor           float64    // cave: skip when local dy <= Floor
	Widths          *[]float32 // canyon: per-height width factors (index y+64-1)
	// Nether: carveEllipsoid's Y clamp is minGenY+1 .. minGenY+genDepth-1-7
	// (0 / 128 there); zero keeps the overworld -64 / 384.
	Nether bool
}

func (s *CarveShape) yRange() (lo, hi int) {
	if s.Nether {
		return 1, 120
	}
	return -63, 312
}

// Carved reports whether block (x, y, z) lies inside the shape, using the
// same cell tests as WorldCarver.carveEllipsoid and the carvers' skip checks.
func (s *CarveShape) Carved(x, y, z int) bool {
	ylo, yhi := s.yRange()
	lo := max(mFloor(s.Y-s.VR)-1, ylo)
	hi := min(mFloor(s.Y+s.VR)+1, yhi)
	if y <= lo || y > hi {
		return false
	}
	dx := (float64(x) + 0.5 - s.X) / s.HR
	dz := (float64(z) + 0.5 - s.Z) / s.HR
	if dx*dx+dz*dz >= 1 {
		return false
	}
	dy := (float64(y) - 0.5 - s.Y) / s.VR
	if s.Widths != nil {
		return (dx*dx+dz*dz)*float64((*s.Widths)[y+64-1])+dy*dy/6 < 1
	}
	return dy > s.Floor && dx*dx+dy*dy+dz*dz < 1
}

// Bounds is the inclusive block box the shape can touch.
func (s *CarveShape) Bounds() Box {
	ylo, yhi := s.yRange()
	return Box{mFloor(s.X - s.HR), max(mFloor(s.Y-s.VR), ylo), mFloor(s.Z - s.HR), mFloor(s.X + s.HR), min(mFloor(s.Y+s.VR)+1, yhi), mFloor(s.Z + s.HR)}
}

func uniformF(r *JavaRandom, lo, hi float32) float32 { return r.NextFloat()*(hi-lo) + lo }

type caveCfg struct {
	prob float32
	yMin int
	yMax int
}

var overworldCaves = [2]caveCfg{{0.15, -56, 180}, {0.07, -56, 47}}

// OverworldCarves lists the carver shapes started in chunk (cx, cz). The
// shapes may reach up to 8 chunks away.
func OverworldCarves(seed int64, cx, cz int32) []CarveShape {
	var out []CarveShape
	for i, c := range overworldCaves {
		r := LargeFeatureRandom(seed+int64(i), cx, cz)
		if r.NextFloat() <= c.prob {
			out = caveCarve(r, c, cx, cz, out)
		}
	}
	r := LargeFeatureRandom(seed+2, cx, cz)
	if r.NextFloat() <= 0.01 {
		out = canyonCarve(r, cx, cz, out)
	}
	return out
}

func caveCarve(r *JavaRandom, c caveCfg, cx, cz int32, out []CarveShape) []CarveShape {
	const reach = 112
	n := r.NextInt(r.NextInt(r.NextInt(15)+1) + 1)
	for k := int32(0); k < n; k++ {
		x := float64(int(cx)*16 + int(r.NextInt(16)))
		y := float64(int(r.NextInt(int32(c.yMax-c.yMin+1))) + c.yMin)
		z := float64(int(cz)*16 + int(r.NextInt(16)))
		hm := float64(uniformF(r, 0.7, 1.4))
		vm := float64(uniformF(r, 0.8, 1.3))
		floor := float64(uniformF(r, -1, -0.4))
		tunnels := 1
		if r.NextInt(4) == 0 {
			ys := float64(uniformF(r, 0.1, 0.9))
			rad := 1 + r.NextFloat()*6
			d0 := 1.5 + float64(mSin(math.Pi/2)*rad)
			out = append(out, CarveShape{X: x + 1, Y: y, Z: z, HR: d0, VR: d0 * ys, Floor: floor})
			tunnels += int(r.NextInt(4))
		}
		for t := 0; t < tunnels; t++ {
			yaw := r.NextFloat() * (math.Pi * 2)
			pitch := (r.NextFloat() - 0.5) / 4
			th := r.NextFloat()*2 + r.NextFloat()
			if r.NextInt(10) == 0 {
				th *= r.NextFloat()*r.NextFloat()*3 + 1
			}
			count := reach - int(r.NextInt(reach/4))
			out = caveTunnel(r.NextLong(), x, y, z, hm, vm, th, yaw, pitch, 0, count, 1, floor, out)
		}
	}
	return out
}

func caveTunnel(seed int64, x, y, z, hm, vm float64, th, yaw, pitch float32, from, count int, ys, floor float64, out []CarveShape) []CarveShape {
	r := newRandom(seed)
	split := int(r.NextInt(int32(count/2))) + count/4
	steep := r.NextInt(6) == 0
	var f, f1 float32
	for j := from; j < count; j++ {
		d0 := 1.5 + float64(mSin(float64(float32(j)*float32(math.Pi)/float32(count)))*th)
		d1 := d0 * ys
		f2 := mCos(float64(pitch))
		x += float64(mCos(float64(yaw)) * f2)
		y += float64(mSin(float64(pitch)))
		z += float64(mSin(float64(yaw)) * f2)
		if steep {
			pitch *= 0.92
		} else {
			pitch *= 0.7
		}
		pitch += float32(f1 * 0.1)
		yaw += float32(f * 0.1)
		f1 *= 0.9
		f *= 0.75
		f1 += float32(float32(r.NextFloat()-r.NextFloat())*r.NextFloat()) * 2
		f += float32(float32(r.NextFloat()-r.NextFloat())*r.NextFloat()) * 4
		if j == split && th > 1 {
			out = caveTunnel(r.NextLong(), x, y, z, hm, vm, r.NextFloat()*0.5+0.5, yaw-math.Pi/2, pitch/3, j, count, 1, floor, out)  // vanilla branches use yScale 1.0
			return caveTunnel(r.NextLong(), x, y, z, hm, vm, r.NextFloat()*0.5+0.5, yaw+math.Pi/2, pitch/3, j, count, 1, floor, out) // vanilla branches use yScale 1.0
		}
		if r.NextInt(4) != 0 {
			out = append(out, CarveShape{X: x, Y: y, Z: z, HR: d0 * hm, VR: d1 * vm, Floor: floor})
		}
	}
	return out
}

func canyonCarve(r *JavaRandom, cx, cz int32, out []CarveShape) []CarveShape {
	const reach = 112
	x := float64(int(cx)*16 + int(r.NextInt(16)))
	y := float64(int(r.NextInt(67-10+1)) + 10)
	z := float64(int(cz)*16 + int(r.NextInt(16)))
	yaw := r.NextFloat() * (math.Pi * 2)
	pitch := uniformF(r, -0.125, 0.125)
	const ys = 3.0
	// trapezoid(0, 6, plateau 2)
	th := 0 + r.NextFloat()*4 + r.NextFloat()*2
	count := int(float32(reach) * uniformF(r, 0.75, 1))
	rr := newRandom(r.NextLong())
	widths := make([]float32, 384)
	w := float32(1)
	for j := range widths {
		if j == 0 || rr.NextInt(3) == 0 {
			w = 1 + rr.NextFloat()*rr.NextFloat()
		}
		widths[j] = w * w
	}
	var f, f1 float32
	for i := 0; i < count; i++ {
		d0 := 1.5 + float64(mSin(float64(float32(i)*float32(math.Pi)/float32(count)))*th)
		d1 := d0 * ys
		d0 *= float64(uniformF(rr, 0.75, 1))
		d1 *= float64(uniformF(rr, 0.75, 1)) // default factor 1, center factor 0
		f2 := mCos(float64(pitch))
		f3 := mSin(float64(pitch))
		x += float64(mCos(float64(yaw)) * f2)
		y += float64(f3)
		z += float64(mSin(float64(yaw)) * f2)
		pitch *= 0.7
		pitch += float32(f1 * 0.05)
		yaw += float32(f * 0.05)
		f1 *= 0.8
		f *= 0.5
		f1 += float32(float32(rr.NextFloat()-rr.NextFloat())*rr.NextFloat()) * 2
		f += float32(float32(rr.NextFloat()-rr.NextFloat())*rr.NextFloat()) * 4
		if rr.NextInt(4) != 0 {
			out = append(out, CarveShape{X: x, Y: y, Z: z, HR: d0, VR: d1, Widths: &widths})
		}
	}
	return out
}

// CarvedChunk rasterises chunk (cx, cz) at height y: bit x of row z is set
// when a cave or canyon carver removes that block. shapes returns the shapes
// started in a chunk (callers usually cache OverworldCarves).
func CarvedChunk(shapes func(cx, cz int32) []CarveShape, cx, cz int32, y int) (cave, canyon [16]uint16) {
	col := Box{int(cx) * 16, y, int(cz) * 16, int(cx)*16 + 15, y, int(cz)*16 + 15}
	for dx := int32(-8); dx <= 8; dx++ {
		for dz := int32(-8); dz <= 8; dz++ {
			ss := shapes(cx+dx, cz+dz)
			for i := range ss {
				s := &ss[i]
				if !s.Bounds().Intersects(col) {
					continue
				}
				dst := &cave
				if s.Widths != nil {
					dst = &canyon
				}
				for z := 0; z < 16; z++ {
					for x := 0; x < 16; x++ {
						if dst[z]&(1<<x) == 0 && s.Carved(col[0]+x, y, col[2]+z) {
							dst[z] |= 1 << x
						}
					}
				}
			}
		}
	}
	return cave, canyon
}

// CarverLavaY is the highest overworld y that carvers fill with lava
// (lava_level above_bottom 8).
const CarverLavaY = -56

// NetherCarves lists the nether_cave carver shapes started in chunk (cx, cz)
// (NetherWorldCarver over CaveWorldCarver: probability 0.2, y uniform
// 0..126, constant radius multipliers 1 / 1 and floor -0.7, room y_scale
// 0.5, cave bound 10, thickness (2f+f)*2 with no wide roll, tunnel y scale
// 5). Every vanilla Nether biome lists only this carver, so its index is 0.
func NetherCarves(seed int64, cx, cz int32) []CarveShape {
	r := LargeFeatureRandom(seed, cx, cz)
	if r.NextFloat() > 0.2 {
		return nil
	}
	const reach = 112
	var out []CarveShape
	n := r.NextInt(r.NextInt(r.NextInt(10)+1) + 1)
	for k := int32(0); k < n; k++ {
		x := float64(int(cx)*16 + int(r.NextInt(16)))
		y := float64(r.NextInt(127))
		z := float64(int(cz)*16 + int(r.NextInt(16)))
		const floor = -0.7
		tunnels := 1
		if r.NextInt(4) == 0 {
			rad := 1 + r.NextFloat()*6
			d0 := 1.5 + float64(mSin(math.Pi/2)*rad)
			out = append(out, CarveShape{X: x + 1, Y: y, Z: z, HR: d0, VR: d0 * 0.5, Floor: floor, Nether: true})
			tunnels += int(r.NextInt(4))
		}
		for t := 0; t < tunnels; t++ {
			yaw := r.NextFloat() * (math.Pi * 2)
			pitch := (r.NextFloat() - 0.5) / 4
			th := (r.NextFloat()*2 + r.NextFloat()) * 2
			count := reach - int(r.NextInt(reach/4))
			from := len(out)
			out = caveTunnel(r.NextLong(), x, y, z, 1, 1, th, yaw, pitch, 0, count, 5, floor, out)
			for i := from; i < len(out); i++ {
				out[i].Nether = true
			}
		}
	}
	return out
}
