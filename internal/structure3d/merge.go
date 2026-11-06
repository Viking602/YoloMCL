package structure3d

// Merge adds another baked structure's faces to m at that structure's own
// world position, so neighbouring structures stand on the same terrain
// slab. Their blocks pick and highlight like the subject's; the subject's
// extent, Y range and fit are unchanged.
func (m *Mesh) Merge(o *Mesh) {
	if o == nil || o == m {
		return
	}
	dx := float32(o.Center[0] - m.Center[0])
	dy := float32(o.Center[1] - m.Center[1])
	dz := float32(o.Center[2] - m.Center[2])
	base := int32(len(m.blocks))
	for _, q := range o.quads {
		if q.terrain {
			continue
		}
		for k := range q.p {
			q.p[k][0] += dx
			q.p[k][1] += dy
			q.p[k][2] += dz
		}
		q.top += dy
		if q.block > 0 {
			q.block += base
		}
		q.extra = true
		m.quads = append(m.quads, q)
	}
	m.blocks = append(m.blocks, o.blocks...)
	m.Blocks += o.Blocks
	m.Pieces = append(m.Pieces, o.Pieces...)
}
