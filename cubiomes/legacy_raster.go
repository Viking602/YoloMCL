package cubiomes

// The final-resolution filters dominate legacy maps. Row slices avoid copying
// nine neighbors into the general layer dispatcher for every output cell.
func (l *legacyLayer) smoothArea(s *legacyScratch, out, p []int32, x, z, w, h int) error {
	pw := w + 2
	for j := range h {
		if j&31 == 0 {
			if err := s.ctx.Err(); err != nil {
				return err
			}
		}
		north := p[j*pw+1 : j*pw+1+w]
		row := p[(j+1)*pw : (j+2)*pw]
		south := p[(j+2)*pw+1 : (j+2)*pw+1+w]
		dst := out[j*w : (j+1)*w]
		for i := range dst {
			v, left, up := row[i+1], row[i], north[i]
			if v != left || v != up {
				right, down := row[i+2], south[i]
				if left == right && up == down {
					v = left
					if legacyChunkSeed(l.start, x+i, z+j)&(1<<24) != 0 {
						v = up
					}
				} else {
					if left == right {
						v = left
					}
					if up == down {
						v = up
					}
				}
			}
			dst[i] = v
		}
	}
	return nil
}

func (l *legacyLayer) riverArea(s *legacyScratch, out, p []int32, w, h int) error {
	// This scratch rectangle is exclusively owned; normalize each source cell
	// once instead of recomputing its parity in five overlapping neighborhoods.
	if l.mc >= mc1_7 {
		for i, v := range p {
			if i&4095 == 0 {
				if err := s.ctx.Err(); err != nil {
					return err
				}
			}
			p[i] = legacyRiverID(v)
		}
	}
	pw := w + 2
	for j := range h {
		if j&31 == 0 {
			if err := s.ctx.Err(); err != nil {
				return err
			}
		}
		north := p[j*pw+1 : j*pw+1+w]
		row := p[(j+1)*pw : (j+2)*pw]
		south := p[(j+2)*pw+1 : (j+2)*pw+1+w]
		dst := out[j*w : (j+1)*w]
		for i := range dst {
			v := row[i+1]
			if (l.mc >= mc1_7 || v != 0) && v == row[i] && v == row[i+2] && v == north[i] && v == south[i] {
				dst[i] = -1
			} else {
				dst[i] = biomeRiver
			}
		}
	}
	return nil
}
