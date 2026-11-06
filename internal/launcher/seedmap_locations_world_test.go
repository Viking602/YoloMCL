package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// TestStructureLocationsAgainstSaves checks seed-map structure positions
// against saved structure starts. MCL_LOCATION_SAVES lists saves/<world>
// directories (";"). For every saved start it reports, per namespace and
// structure, whether the map places a marker at that chunk:
//   - exact: grid + frequency/exclusion + biome filter all pass
//   - biome_reject: the grid predicts it but the biome filter drops it
//   - grid_miss: the placement does not produce that chunk
//   - no_set / unsupported: no structure_set lists it, or the set's
//     placement could not be parsed (with the issue list)
//
// It also counts predicted markers in saved (non-empty) chunks without a
// saved start of that set (predicted_no_start: structure-level biome or
// terrain checks the map does not model).
func TestStructureLocationsAgainstSaves(t *testing.T) {
	list := os.Getenv("MCL_LOCATION_SAVES")
	if list == "" {
		t.Skip("MCL_LOCATION_SAVES lists saves/world directories")
	}
	only := os.Getenv("MCL_LOCATION_ONLY") // namespace substring filter
	dims := map[string]string{"minecraft:overworld": "", "minecraft:the_nether": "DIM-1", "minecraft:the_end": "DIM1"}
	for _, world := range strings.Split(list, ";") {
		instance := filepath.Dir(filepath.Dir(world))
		root := Root{ID: "loc-oracle", Path: instance}
		if filepath.Base(filepath.Dir(instance)) == "versions" {
			root.Path, root.SelectedVersion = filepath.Dir(filepath.Dir(instance)), filepath.Base(instance)
		}
		s := testService(t)
		s.cfg.Roots = []Root{root}
		snapshot, level, err := s.SeedMapWorldgen(context.Background(), root.ID, "", filepath.Base(world))
		if err != nil {
			t.Logf("%s: %v", world, err)
			continue
		}
		sampler, err := NewLocalSampler(context.Background(), snapshot, level.RandomSeed)
		if err != nil {
			t.Logf("%s: %v", world, err)
			continue
		}
		eraMap, _ := s.DetectEras(context.Background(), root.ID, "", filepath.Base(world))
		eraSamplers := []*LocalSampler{sampler}
		if eraMap != nil && len(eraMap.Eras) > 1 {
			eraSamplers = make([]*LocalSampler, len(eraMap.Eras))
			for i := range eraMap.Eras {
				es, err := s.EraSampler(context.Background(), root.ID, "", filepath.Base(world), i)
				if err != nil {
					t.Logf("%s: era %d: %v", world, i, err)
					eraSamplers = []*LocalSampler{sampler}
					eraMap = nil
					break
				}
				eraSamplers[i] = es
			}
		}
		if store, err := s.StructureAssetStoreWorld(root.ID, root.SelectedVersion, filepath.Base(world)); err == nil {
			for _, es := range eraSamplers {
				locBindJigsawStarts(store, es)
			}
			locBindJigsawStarts(store, sampler)
			var unbound []string
			for _, id := range sampler.JigsawStartPools() {
				if len(sampler.jigsawPools[id]) == 0 {
					unbound = append(unbound, id+"("+locPoolWhy(store, id)+")")
				}
			}
			t.Logf("jigsaw start pools %d, unbound %d: %v", len(sampler.JigsawStartPools()), len(unbound), unbound)
			store.Close()
		} else {
			t.Logf("asset store: %v", err)
		}
		seed := level.RandomSeed
		setDocs := 0
		for _, doc := range snapshot.Documents {
			if doc.Kind == "structure_set" {
				setDocs++
			}
		}
		var issues []string
		for _, is := range sampler.coverage.Issues {
			if is.Code != "saved_settings" {
				issues = append(issues, is.Code+":"+is.Source)
			}
		}
		t.Logf("== %s seed %d: %d structure_set docs, %d placements parsed; issues %v", world, seed, setDocs, len(sampler.placements), issues)
		stType := map[string]string{}
		for _, doc := range snapshot.Documents {
			if doc.Kind == "structure" {
				var d struct {
					Type string          `json:"type"`
					H    json.RawMessage `json:"start_height"`
					J    string          `json:"start_jigsaw_name"`
					P    string          `json:"project_start_to_heightmap"`
				}
				_ = json.Unmarshal(doc.Data, &d)
				stType[doc.ID] = fmt.Sprintf("%s h=%s j=%s p=%s", d.Type, strings.Join(strings.Fields(string(d.H)), ""), d.J, d.P)
			}
		}
		bySt := map[string][]seedMapPlacement{}
		for _, p := range sampler.placements {
			for _, st := range p.structures {
				bySt[st] = append(bySt[st], p)
			}
		}
		tally := map[string]map[string]int{} // structure -> outcome -> n
		bump := func(id, out string) {
			if tally[id] == nil {
				tally[id] = map[string]int{}
			}
			tally[id][out]++
		}
		ringCache := map[string]map[[2]int32]bool{}
		for dim, sub := range dims {
			d := sampler.dimensions[dim]
			t.Logf("  dim %s mode=%q opaque=%v engine=%v issues=%v", dim, d.mode, d.opaque, d.engine != nil, d.issues)
			scratch := d.scratch()
			paths, _ := mcworld.RegionPaths(world, sub)
			saved := map[string]map[[2]int]bool{} // structure -> start chunks
			present := map[[2]int]bool{}
			savedBB := map[string][6]int32{}
			for _, path := range paths {
				r, err := mcworld.OpenRegion(path)
				if err != nil {
					continue
				}
				for i := 0; i < 1024; i++ {
					c, _ := r.Chunk(i%32, i/32)
					if c == nil {
						continue
					}
					status := c.Status
					if status == "" || strings.HasSuffix(status, "empty") {
						continue
					}
					present[[2]int{int(c.X), int(c.Z)}] = true
					for _, st := range c.Starts() {
						if len(st.Pieces) == 0 || (only != "" && !strings.Contains(st.ID, only)) {
							continue
						}
						if saved[st.ID] == nil {
							saved[st.ID] = map[[2]int]bool{}
						}
						saved[st.ID][[2]int{st.ChunkX, st.ChunkZ}] = true
						savedBB[fmt.Sprint(st.ID, st.ChunkX, st.ChunkZ)] = st.Pieces[0].BB
					}
				}
			}
			for id, chunks := range saved {
				ps := bySt[id]
				for ch := range chunks {
					if len(ps) == 0 {
						bump(id, "no_set")
						continue
					}
					best := "grid_miss"
					use := sampler
					if eraMap != nil {
						v := eraMap.EraAt(ch[0], ch[1])
						if v.Era >= 0 && v.Era < len(eraSamplers) {
							use = eraSamplers[v.Era]
						}
					}
					d = use.dimensions[dim]
					scratch = d.scratch()
					for _, p := range ps {
						out := "grid_miss"
						if p.rings != nil {
							rc := ringCache[p.id+dim]
							if rc == nil {
								rc = map[[2]int32]bool{}
								cs, _ := sampler.ringChunks(context.Background(), p, dim)
								for _, c := range cs {
									rc[c] = true
								}
								ringCache[p.id+dim] = rc
							}
							if rc[[2]int32{int32(ch[0]), int32(ch[1])}] {
								out = "exact"
							}
						} else if p.landmarkGrid {
							rx, rz := tfRegionOfChunk(ch[0]), tfRegionOfChunk(ch[1])
							cx, cz := p.candidate(seed, rx, rz)
							if cx>>4 == ch[0] && cz>>4 == ch[1] && p.placementChunk(seed, cx, cz) {
								out = "exact"
							}
						} else if p.esLandmark {
							rx, rz := ch[0]>>5, ch[1]>>5
							cx, cz := p.candidate(seed, rx, rz)
							if cx>>4 == ch[0] && cz>>4 == ch[1] && p.placementChunk(seed, cx, cz) {
								out = "exact"
							}
						} else {
							rx, rz := floorDivInt(ch[0], p.spacing), floorDivInt(ch[1], p.spacing)
							cx, cz := p.candidate(seed, rx, rz)
							if cx>>4 == ch[0] && cz>>4 == ch[1] && p.placementChunk(seed, cx, cz) {
								out = "exact"
								if p.biomesKnown && d.mode != "" {
									bx, by, bz := use.biomeProbe(p, dim, cx, 64, cz)
									if len(p.probes) > 0 {
										bx, by, bz = use.probePos(p.probes[0], dim, cx, 64, cz)
									}
									if b, ok, pass := use.biomeCheck(p, d, dim, cx, 64, cz, scratch); ok && !pass {
										out = fmt.Sprintf("biome_reject:%s probe=%d,%d,%d pool=%v %s", b, bx, by, bz, p.jigsaw != nil && len(use.jigsawPools[p.jigsaw.pool]) > 0, locSaved(world, sub, bx, by, bz))
										if os.Getenv("MCL_LOC_PROBES") != "" {
											if bb, ok := savedBB[fmt.Sprint(id, ch[0], ch[1])]; ok {
												sx, sz := int(bb[0]+bb[3])/2, int(bb[2]+bb[5])/2
												sb, _ := d.biomeAt(sx, by, sz, scratch, false)
												out += fmt.Sprintf("\n      saved piece0 bb=%v centre=%d,%d ours@y%d=%s", bb, sx, sz, by, sb)
											}
											for k, pr := range p.probes {
												px, py, pz := use.probePos(pr, dim, cx, 64, cz)
												pb, _ := d.biomeAt(px, py, pz, scratch, false)
												out += fmt.Sprintf("\n      probe[%s] %d,%d,%d %s allowed=%v %s", p.structures[k], px, py, pz, pb, pr.allowed[pb], locSaved(world, sub, px, py, pz))
											}
										}
									}
								} else {
									out = "exact_nobiome"
								}
							}
						}
						if out == "exact" || (best == "grid_miss") {
							best = out
						}
					}
					if strings.HasPrefix(best, "biome_reject") {
						t.Logf("  %s @chunk %v: %s [%s]", id, ch, best, stType[id])
						best = "biome_reject"
					}
					if best == "grid_miss" {
						t.Logf("  %s @chunk %v: grid_miss", id, ch)
					}
					bump(id, best)
				}
			}
			// predicted markers in saved chunks without a saved start
			for _, p := range sampler.placements {
				if p.rings != nil || !p.inDimension(dim) || (only != "" && !strings.Contains(p.id, only)) {
					continue
				}
				seenRegion := map[[2]int]bool{}
				for ch := range present {
					var rx, rz int
					switch {
					case p.landmarkGrid:
						rx, rz = tfRegionOfChunk(ch[0]), tfRegionOfChunk(ch[1])
					case p.esLandmark:
						rx, rz = ch[0]>>5, ch[1]>>5
					default:
						rx, rz = floorDivInt(ch[0], p.spacing), floorDivInt(ch[1], p.spacing)
					}
					if seenRegion[[2]int{rx, rz}] {
						continue
					}
					seenRegion[[2]int{rx, rz}] = true
					cx, cz := p.candidate(seed, rx, rz)
					c := [2]int{cx >> 4, cz >> 4}
					if !present[c] || !p.placementChunk(seed, cx, cz) {
						continue
					}
					if p.biomesKnown && d.mode != "" {
						if _, ok, pass := sampler.biomeCheck(p, d, dim, cx, 64, cz, scratch); ok && !pass {
							continue
						}
					}
					hit := false
					for _, st := range p.structures {
						if saved[st][c] {
							hit = true
						}
					}
					if !hit {
						bump("set "+p.id, "predicted_no_start")
					} else {
						bump("set "+p.id, "predicted_ok")
					}
				}
			}
			d.releaseScratch(scratch)
		}
		ids := make([]string, 0, len(tally))
		for id := range tally {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		ns := map[string][2]int{}
		for _, id := range ids {
			m := tally[id]
			var parts []string
			for k, v := range m {
				parts = append(parts, fmt.Sprintf("%s=%d", k, v))
			}
			sort.Strings(parts)
			t.Logf("  %-55s %s", id, strings.Join(parts, " "))
			if !strings.HasPrefix(id, "set ") {
				n := strings.SplitN(id, ":", 2)[0]
				a := ns[n]
				for k, v := range m {
					a[1] += v
					if k == "exact" {
						a[0] += v
					}
				}
				ns[n] = a
			}
		}
		for n, a := range ns {
			t.Logf("  NS %-20s exact %d/%d", n, a[0], a[1])
		}
	}
}

// locBindJigsawStarts mirrors nativeui.bindJigsawStarts.
func locBindJigsawStarts(store *structure3d.Store, sampler *LocalSampler) {
	pools := map[string][]JigsawStartPiece{}
	for _, id := range sampler.JigsawStartPools() {
		ns, name := structure3d.Namespaced(id)
		data, err := store.Read(structure3d.DataPath(ns, "worldgen/template_pool", name, ".json"))
		if err != nil {
			continue
		}
		var pool struct {
			Elements []struct {
				Weight  int `json:"weight"`
				Element struct {
					Type     string          `json:"element_type"`
					Location json.RawMessage `json:"location"`
				} `json:"element"`
			} `json:"elements"`
		}
		if json.Unmarshal(data, &pool) != nil {
			continue
		}
		var pieces []JigsawStartPiece
		ok := true
		for _, e := range pool.Elements {
			var piece JigsawStartPiece
			switch e.Element.Type {
			case "minecraft:empty_pool_element", "empty_pool_element":
				piece.Empty = true
			case "minecraft:single_pool_element", "single_pool_element", "minecraft:legacy_single_pool_element", "legacy_single_pool_element",
				"yungsapi:yung_single_element", "yung_single_element",
				"yungsapi:max_count_single_element", "max_count_single_element",
				"yungsapi:max_count_legacy_single_element", "max_count_legacy_single_element":
				var loc string
				if json.Unmarshal(e.Element.Location, &loc) != nil {
					ok = false
					break
				}
				t, err := structure3d.LoadTemplate(store, loc)
				if err != nil {
					ok = false
					break
				}
				piece.Size = [2]int{t.Size[0], t.Size[2]}
			default:
				ok = false
			}
			if !ok || e.Weight < 1 || e.Weight > 150 || len(pieces) > 4096 {
				ok = false
				break
			}
			for range e.Weight {
				pieces = append(pieces, piece)
			}
		}
		if ok && len(pieces) > 0 {
			pools[id] = pieces
		}
	}
	sampler.SetJigsawStartPools(pools)
}

// locSaved reports the saved biome at a block and the saved surface there.
func locSaved(world, sub string, x, y, z int) string {
	cx, cz := x>>4, z>>4
	r, err := mcworld.OpenRegion(filepath.Join(world, sub, "region", fmt.Sprintf("r.%d.%d.mca", cx>>5, cz>>5)))
	if err != nil {
		return "saved=?"
	}
	c, _ := r.Chunk(cx&31, cz&31)
	if c == nil {
		return "saved=nochunk"
	}
	out := "saved=" + c.Biome(x, y, z)
	for _, k := range []string{"WORLD_SURFACE_WG", "WORLD_SURFACE", "OCEAN_FLOOR"} {
		if h, ok := c.Heightmap(k, x&15, z&15, -64); ok {
			out += fmt.Sprintf(" %s=%d", k, h)
		}
	}
	return out
}

func locPoolWhy(store *structure3d.Store, id string) string {
	ns, name := structure3d.Namespaced(id)
	data, err := store.Read(structure3d.DataPath(ns, "worldgen/template_pool", name, ".json"))
	if err != nil {
		return "read:" + err.Error()
	}
	var pool struct {
		Elements []struct {
			Weight  int `json:"weight"`
			Element struct {
				Type     string          `json:"element_type"`
				Location json.RawMessage `json:"location"`
			} `json:"element"`
		} `json:"elements"`
	}
	if err := json.Unmarshal(data, &pool); err != nil {
		return "json:" + err.Error()
	}
	for _, e := range pool.Elements {
		var loc string
		_ = json.Unmarshal(e.Element.Location, &loc)
		if e.Weight < 1 || e.Weight > 150 {
			return fmt.Sprintf("weight %d", e.Weight)
		}
		switch e.Element.Type {
		case "minecraft:empty_pool_element", "minecraft:single_pool_element", "minecraft:legacy_single_pool_element":
			if e.Element.Type != "minecraft:empty_pool_element" {
				if _, err := structure3d.LoadTemplate(store, loc); err != nil {
					return "template " + loc + ": " + err.Error()
				}
			}
		default:
			return "type " + e.Element.Type
		}
	}
	return "?"
}
