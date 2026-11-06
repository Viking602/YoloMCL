package cubiomes

// TerraBlender's publicly documented region pipeline is a weighted integer
// field followed by one fuzzy and 3+regionSize majority zooms. This independent
// implementation reuses the library's historical LCG and represents each zoom
// as a memoized pure function. Source references and versions are recorded in
// data/terrablender.provenance.json; no mod code executes here.
type terraBlenderLayer struct {
	seed  uint64
	cache []terraBlenderCell
}

type terraBlenderCell struct {
	key   uint64
	value uint16
	valid bool
}

type terraBlenderRandom struct{ value, seed uint64 }

func terraBlenderRandomAt(seed uint64, x, z int32) terraBlenderRandom {
	v := seed
	for range 2 {
		v = voronoiStep(v, uint64(int64(x)))
		v = voronoiStep(v, uint64(int64(z)))
	}
	return terraBlenderRandom{v, seed}
}

func (r *terraBlenderRandom) next(bound int) int {
	v := (int64(r.value) >> 24) % int64(bound)
	if v < 0 {
		v += int64(bound)
	}
	r.value = voronoiStep(r.value, r.seed)
	return int(v)
}

func (s *terraBlenderScratch) region(tb *terraBlender, level int, x, z int32) int {
	if tb.constant {
		return tb.constantValue
	}
	layer := &s.layers[level]
	key := uint64(uint32(x)) | uint64(uint32(z))<<32
	hash := (key ^ key>>32) * 0x9e3779b97f4a7c15
	hash ^= hash >> 32
	entry := &layer.cache[hash&uint64(len(layer.cache)-1)]
	if entry.valid && entry.key == key {
		return int(entry.value)
	}
	result := tb.emptyValue
	if level == 0 {
		if tb.total > 0 {
			r := terraBlenderRandomAt(layer.seed, x, z)
			pick := r.next(tb.total)
			for i, weight := range tb.weights {
				if pick < weight {
					result = i
					if tb.values != nil {
						result = int(tb.values[i])
					}
					break
				}
			}
		}
	} else {
		px, pz := x>>1, z>>1
		a := s.region(tb, level-1, px, pz)
		result = a
		if x&1 != 0 || z&1 != 0 {
			r := terraBlenderRandomAt(layer.seed, px<<1, pz<<1)
			c := s.region(tb, level-1, px, (z+1)>>1)
			vertical := r.next(2)
			if x&1 == 0 {
				if vertical == 1 {
					result = c
				}
			} else {
				b := s.region(tb, level-1, (x+1)>>1, pz)
				horizontal := r.next(2)
				if z&1 == 0 {
					if horizontal == 1 {
						result = b
					}
				} else {
					d := s.region(tb, level-1, (x+1)>>1, (z+1)>>1)
					corners := [4]int{a, b, c, d}
					chosen := -1
					if level != 1 {
						// A unique mode wins. Two equally frequent pairs have
						// no unique mode, so they use the same random tie break
						// as four distinct corners. No random draw on a winner.
						best, tied := 1, false
						for i, candidate := range corners {
							count, earlier := 0, false
							for j, value := range corners {
								if value == candidate {
									count++
									earlier = earlier || j < i
								}
							}
							if earlier {
								continue
							}
							if count > best {
								chosen, best, tied = candidate, count, false
							} else if count == best {
								tied = true
							}
						}
						if tied {
							chosen = -1
						}
					}
					result = chosen
					if chosen < 0 {
						result = corners[r.next(4)]
					}
				}
			}
		}
	}
	*entry = terraBlenderCell{key: key, value: uint16(result), valid: true}
	return result
}
