package structure3d_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/structure3d"
)

type regionCache struct {
	dir     string
	regions map[[2]int]*mcworld.Region
	chunks  map[[2]int]*mcworld.Chunk
}

func (c *regionCache) chunk(cx, cz int) *mcworld.Chunk {
	ck := [2]int{cx, cz}
	ch, ok := c.chunks[ck]
	if ok {
		return ch
	}
	rk := [2]int{cx >> 5, cz >> 5}
	r, ok := c.regions[rk]
	if !ok {
		r, _ = mcworld.OpenRegion(filepath.Join(c.dir, fmt.Sprintf("r.%d.%d.mca", rk[0], rk[1])))
		c.regions[rk] = r
	}
	if r != nil {
		ch, _ = r.Chunk(cx&31, cz&31)
		if ch != nil && ch.Status != "minecraft:full" && ch.Status != "full" {
			ch = nil
		}
	}
	c.chunks[ck] = ch
	return ch
}

func (c *regionCache) block(x, y, z int) (string, bool) {
	ch := c.chunk(x>>4, z>>4)
	if ch == nil {
		return "", false
	}
	return ch.Block(x&15, y, z&15), true
}

func isAirName(n string) bool {
	return n == "minecraft:air" || n == "minecraft:cave_air" || n == "minecraft:void_air"
}

// TestCodeStructureBlocksAgainstWorld post-processes saved fortress pieces
// and compares every written position with the saved block in fully
// generated chunks. STRUCTURE_BLOCKS_VERIFY lists region folders
// ("<world>/DIM-1/region;...").
func TestCodeStructureBlocksAgainstWorld(t *testing.T) {
	dirs := os.Getenv("STRUCTURE_BLOCKS_VERIFY")
	if dirs == "" {
		t.Skip("set STRUCTURE_BLOCKS_VERIFY")
	}
	want := map[string]bool{"minecraft:fortress": true, "minecraft:mineshaft": true, "minecraft:mineshaft_mesa": true, "minecraft:stronghold": true,
		"minecraft:desert_pyramid": true, "minecraft:jungle_pyramid": true, "minecraft:swamp_hut": true}
	// own lists each structure's materials, read as air in the
	// pre-structure ground.
	own := func(id, b string) bool {
		if id == "minecraft:fortress" {
			return strings.Contains(b, "nether_brick")
		}
		if id == "minecraft:stronghold" {
			return strings.Contains(b, "stone_brick") || strings.Contains(b, "iron_") || strings.Contains(b, "oak_") ||
				strings.Contains(b, "bookshelf") || strings.Contains(b, "end_portal") || strings.Contains(b, "smooth_stone") ||
				strings.Contains(b, "cobblestone") || strings.Contains(b, "torch") || strings.Contains(b, "ladder") ||
				b == "minecraft:cobweb" || b == "minecraft:chest" || b == "minecraft:spawner" || b == "minecraft:stone_button"
		}
		switch id {
		case "minecraft:desert_pyramid":
			return strings.Contains(b, "sandstone") || strings.Contains(b, "terracotta") || b == "minecraft:tnt" ||
				b == "minecraft:stone_pressure_plate" || b == "minecraft:chest"
		case "minecraft:jungle_pyramid":
			return strings.Contains(b, "cobblestone") || strings.Contains(b, "stone_bricks") || strings.Contains(b, "redstone") ||
				strings.Contains(b, "trip") || strings.Contains(b, "piston") || b == "minecraft:dispenser" || b == "minecraft:lever" ||
				b == "minecraft:repeater" || b == "minecraft:vine" || b == "minecraft:chest"
		case "minecraft:swamp_hut":
			return strings.Contains(b, "spruce") || b == "minecraft:oak_log" || b == "minecraft:oak_fence" || b == "minecraft:cauldron" ||
				b == "minecraft:crafting_table" || b == "minecraft:potted_red_mushroom"
		}
		switch strings.TrimPrefix(b, "minecraft:") {
		case "oak_planks", "oak_log", "oak_fence", "dark_oak_planks", "dark_oak_log", "dark_oak_fence",
			"rail", "cobweb", "chain", "wall_torch", "spawner":
			return true
		}
		return false
	}
	if only := os.Getenv("STRUCTURE_BLOCKS_ONLY"); only != "" {
		want = map[string]bool{}
		for _, id := range strings.Split(only, ",") {
			want["minecraft:"+strings.TrimPrefix(id, "minecraft:")] = true
		}
	}
	type tally struct{ solid, solidOK, air, airOK, starts int }
	per := map[string]*tally{}
	perPiece := map[string]*tally{}
	miss := map[string]int{}
	var brickBoth, brickEq int
	for _, dir := range strings.Split(dirs, ";") {
		cc := &regionCache{dir: dir, regions: map[[2]int]*mcworld.Region{}, chunks: map[[2]int]*mcworld.Chunk{}}
		seed, seedErr := mcworld.LevelSeed(filepath.Dir(dir))
		var sg *seedGround
		if seedErr == nil {
			sg = newSeedGround(t, seed)
		}
		index := map[string]int{}
		minY, maxY := 0, 256
		if !strings.Contains(dir, "DIM-1") && !strings.Contains(dir, "the_nether") {
			minY, maxY = -64, 320
		}
		floor := func(x, z int) int {
			ch := cc.chunk(x>>4, z>>4)
			if ch == nil {
				return maxY
			}
			if h, ok := ch.Heightmap("OCEAN_FLOOR", x&15, z&15, minY); ok {
				return h
			}
			return maxY
		}
		paths, _ := filepath.Glob(filepath.Join(dir, "*.mca"))
		for _, p := range paths {
			r, err := mcworld.OpenRegion(p)
			if err != nil {
				continue
			}
			for lz := 0; lz < 32; lz++ {
				for lx := 0; lx < 32; lx++ {
					c, _ := r.Chunk(lx, lz)
					if c == nil {
						continue
					}
					for _, s := range c.Starts() {
						if !want[s.ID] || len(s.Pieces) == 0 {
							continue
						}
						var sp []SavedPiece
						for _, p := range s.Pieces {
							o := -1
							if v, ok := p.Raw["O"].(int32); ok {
								o = int(v)
							}
							sp = append(sp, SavedPiece{ID: p.ID, BB: [6]int{int(p.BB[0]), int(p.BB[1]), int(p.BB[2]), int(p.BB[3]), int(p.BB[4]), int(p.BB[5])}, O: o, Tag: p.Raw})
						}
						// Pre-structure ground: saved blocks with the
						// structure's own materials read as air.
						ground := func(x, y, z int) GroundKind {
							b, ok := cc.block(x, y, z)
							if s.ID == "minecraft:stronghold" && ok && sg != nil && (b == "minecraft:air" || b == "minecraft:water") {
								// what the stronghold overwrote: ask the seed
								return sg.kind(x, y, z)
							}
							if s.ID == "minecraft:stronghold" && ok {
								// Strongholds build into stone: their own
								// blocks and the air in piece boxes were
								// solid before.
								if own(s.ID, b) {
									return GroundSolid
								}
								if isAirName(b) {
									// Doorways and corridors open the shells
									// after the walls are drawn, so all air in
									// a piece box was stone (caves through a
									// stronghold are rare).
									for _, p := range sp {
										if x >= p.BB[0] && x <= p.BB[3] && y >= p.BB[1] && y <= p.BB[4] && z >= p.BB[2] && z <= p.BB[5] {
											return GroundSolid
										}
									}
									return GroundAir
								}
							}
							if !ok || isAirName(b) || own(s.ID, b) {
								return GroundAir
							}
							if b == "minecraft:lava" || b == "minecraft:water" {
								return GroundLiquid
							}
							return GroundSolid
						}
						sw := SavedWorld{Ground: ground, MinY: minY, MaxY: maxY}
						if s.ID != "minecraft:fortress" {
							if seedErr != nil {
								continue
							}
							sw.Floor, sw.Seed = floor, seed
							fam := strings.TrimSuffix(s.ID, "_mesa")
							ix, ok := index[fam]
							if !ok {
								// Only a start with a clear winner fixes the
								// index; otherwise use the vanilla one.
								var sure bool
								if ix, sure = bestIndex(t, cc, s.ID, sp, sw); sure {
									index[fam] = ix
									t.Logf("%s: %s decoration feature index %d", dir, fam, ix)
								} else {
									ix = vanillaIndex[fam]
								}
							}
							sw.FeatureIndex = ix
						}
						got, err := PostProcessSavedIn(s.ID, sp, sw)
						if err != nil {
							t.Log(err)
							continue
						}
						tl := per[s.ID]
						if tl == nil {
							tl = &tally{}
							per[s.ID] = tl
						}
						tl.starts++
						owner := func(pos [3]int) string {
							id := ""
							for _, p := range sp {
								if pos[0] >= p.BB[0] && pos[0] <= p.BB[3] && pos[1] >= p.BB[1] && pos[1] <= p.BB[4] && pos[2] >= p.BB[2] && pos[2] <= p.BB[5] {
									id = p.ID
								}
							}
							if id == "" {
								id = "(column)"
							}
							return id
						}
						sb, se := 0, 0
						for pos, st := range got {
							if b0, ok0 := cc.block(pos[0], pos[1], pos[2]); ok0 && strings.HasSuffix(st.Name, "stone_bricks") && strings.HasSuffix(b0, "stone_bricks") {
								sb++
								if b0 == st.Name {
									se++
								}
							}
						}
						t.Logf("start %s %v: brick variants %d/%d", s.ID, sp[0].BB, se, sb)
						if os.Getenv("SH_CHUNKS") != "" && sb > se {
							type cs struct{ b, e, air, caveAir, water int }
							per := map[[2]int]*cs{}
							for pos, st := range got {
								k := [2]int{pos[0] >> 4, pos[2] >> 4}
								c := per[k]
								if c == nil {
									c = &cs{}
									per[k] = c
								}
								b0, _ := cc.block(pos[0], pos[1], pos[2])
								switch b0 {
								case "minecraft:air":
									c.air++
								case "minecraft:cave_air":
									c.caveAir++
								case "minecraft:water":
									c.water++
								}
								if strings.HasSuffix(st.Name, "stone_bricks") && strings.HasSuffix(b0, "stone_bricks") {
									c.b++
									if b0 == st.Name {
										c.e++
									}
								}
							}
							for k, c := range per {
								if c.b > 0 {
									t.Logf("   chunk %v: variants %d/%d air %d cave_air %d water %d", k, c.e, c.b, c.air, c.caveAir, c.water)
								}
							}
						}
						for pos, st := range got {
							b, ok := cc.block(pos[0], pos[1], pos[2])
							if !ok {
								continue
							}
							pt := perPiece[owner(pos)]
							if pt == nil {
								pt = &tally{}
								perPiece[owner(pos)] = pt
							}
							if isAirName(st.Name) {
								tl.air++
								pt.air++
								if isAirName(b) {
									tl.airOK++
									pt.airOK++
								}
								continue
							}
							tl.solid++
							pt.solid++
							if strings.HasSuffix(st.Name, "stone_bricks") && strings.HasSuffix(b, "stone_bricks") {
								brickBoth++
								if st.Name == b {
									brickEq++
								}
							}
							if b == st.Name {
								tl.solidOK++
								pt.solidOK++
							} else {
								miss[strings.TrimPrefix(s.ID, "minecraft:")+": "+st.Name+" -> "+b]++
							}
						}
					}
				}
			}
		}
	}
	pct := func(a, b int) float64 { return 100 * float64(a) / float64(max(b, 1)) }
	for id, tl := range per {
		t.Logf("%s: %d starts; placed blocks %d/%d (%.2f%%); carved air %d/%d (%.2f%%)", id, tl.starts, tl.solidOK, tl.solid, pct(tl.solidOK, tl.solid), tl.airOK, tl.air, pct(tl.airOK, tl.air))
	}
	t.Logf("brick variant agreement where both are stone-brick family: %d/%d (%.2f%%)", brickEq, brickBoth, pct(brickEq, brickBoth))
	var ks []string
	for k := range perPiece {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		tl := perPiece[k]
		t.Logf("  %s: blocks %d/%d (%.2f%%), air %d/%d (%.2f%%)", k, tl.solidOK, tl.solid, pct(tl.solidOK, tl.solid), tl.airOK, tl.air, pct(tl.airOK, tl.air))
	}
	type kv struct {
		k string
		v int
	}
	var ms []kv
	for k, v := range miss {
		ms = append(ms, kv{k, v})
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].v > ms[j].v })
	for i := 0; i < len(ms) && i < 40; i++ {
		t.Logf("  mismatch %s: %d", ms[i].k, ms[i].v)
	}
}

// bestIndex picks the decoration feature index (the structure's place in
// its step's alphabetical registry list) whose random best reproduces one
// start's placed blocks; vanilla 1.21.1 has mineshaft 1 (underground) and
// stronghold 19 (surface), mods can shift them.
func bestIndex(t *testing.T, cc *regionCache, id string, sp []SavedPiece, sw SavedWorld) (int, bool) {
	best, bestN, second, lo := 0, -1, -1, 1<<30
	for i := 0; i < 256; i++ { // modded installs push the index far past vanilla's
		sw.FeatureIndex = i
		got, err := PostProcessSavedIn(id, sp, sw)
		if err != nil {
			return 0, false
		}
		n := 0
		for pos, st := range got {
			if isAirName(st.Name) {
				continue
			}
			if b, ok := cc.block(pos[0], pos[1], pos[2]); ok && b == st.Name {
				n++
			}
		}
		lo = min(lo, n)
		if n > bestN {
			best, bestN, second = i, n, bestN
		} else if n > second {
			second = n
		}
	}
	// The deterministic blocks match for every index; only the spread
	// above the worst index measures the random.
	return best, bestN-lo >= 20 && bestN-second > (bestN-lo)/4
}

// vanillaIndex is each structure's 1.21.1 decoration feature index.
var vanillaIndex = map[string]int{"minecraft:mineshaft": 1, "minecraft:stronghold": 19,
	"minecraft:desert_pyramid": 1, "minecraft:jungle_pyramid": 4, "minecraft:swamp_hut": 20}
