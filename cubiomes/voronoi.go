package cubiomes

// Integer form of cubiomes' post-1.15 Voronoi access. A unit is 1/40960
// of a quart cell; the jitter is (bits/1024 - .5) * .9. Keep C's stable
// corner order and unsigned wrap at every seed step. See data/LICENSE-cubiomes.txt.
func voronoiStep(seed, salt uint64) uint64 {
	return seed*(seed*6364136223846793005+1442695040888963407) + salt
}

func voronoiCell(sha uint64, x, y, z int) (int64, int64, int64) {
	s := sha
	for range 2 {
		s = voronoiStep(s, uint64(int64(x)))
		s = voronoiStep(s, uint64(int64(y)))
		s = voronoiStep(s, uint64(int64(z)))
	}
	rx := (int64((s>>24)&1023) - 512) * 36
	s = voronoiStep(s, sha)
	ry := (int64((s>>24)&1023) - 512) * 36
	s = voronoiStep(s, sha)
	rz := (int64((s>>24)&1023) - 512) * 36
	return rx, ry, rz
}

func voronoiAccess(sha uint64, x, y, z int) (int, int, int) {
	x, y, z = int(int32(x)-2), int(int32(y)-2), int(int32(z)-2)
	qx, qy, qz := x>>2, y>>2, z>>2
	dx, dy, dz := int64(x&3)*10240, int64(y&3)*10240, int64(z&3)*10240
	best := int64(1<<63 - 1)
	ax, ay, az := 0, 0, 0
	for i := range 8 {
		bx, by, bz := (i>>2)&1, (i>>1)&1, i&1
		cx, cy, cz := qx+bx, qy+by, qz+bz
		rx, ry, rz := voronoiCell(sha, cx, cy, cz)
		rx += dx - 40960*int64(bx)
		ry += dy - 40960*int64(by)
		rz += dz - 40960*int64(bz)
		d := rx*rx + ry*ry + rz*rz
		if d < best {
			best, ax, ay, az = d, cx, cy, cz
		}
	}
	return ax, ay, az
}
