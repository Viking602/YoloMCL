package launcher

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// savedSubstance classifies a saved block into noise-stage substance. ok is
// false for feature/decoration blocks (trees, plants, snow, structures) that
// the noise stage does not place.
func savedSubstance(name string) (TerrainSubstance, bool) {
	n := strings.TrimPrefix(name, "minecraft:")
	switch {
	case n == "air" || n == "cave_air" || n == "void_air":
		return TerrainAir, true
	case n == "water" || n == "bubble_column":
		return TerrainWater, true
	case n == "lava":
		return TerrainLava, true
	case strings.HasSuffix(n, "_ore") || mcworld.IsOre(name):
		return TerrainSolid, true
	}
	for _, s := range []string{"stone", "deepslate", "granite", "diorite", "andesite", "tuff", "dirt", "grass_block", "podzol", "mycelium", "sand", "sandstone", "gravel", "clay", "terracotta", "netherrack", "basalt", "blackstone", "soul_sand", "soul_soil", "nylium", "end_stone", "bedrock", "calcite", "mud", "snow_block", "powder_snow", "packed_ice", "magma_block", "dripstone_block", "moss_block", "coarse_dirt", "rooted_dirt", "raw_iron_block", "raw_copper_block", "glowstone"} {
		if n == s || strings.HasSuffix(n, "_"+s) || strings.HasPrefix(n, s) && !strings.Contains(n, "slab") && !strings.Contains(n, "stairs") && !strings.Contains(n, "wall") && !strings.Contains(n, "brick") {
			return TerrainSolid, true
		}
	}
	return 0, false
}

// TestTerrainAgainstSaves compares the compiled final_density + aquifer
// column with real generated chunks. Carvers, features and structures alter
// saved chunks, so rates are reported, not required to be 100%.
// MCL_TERRAIN_SAVE=<saves/world dir>, optional MCL_TERRAIN_CHUNKS (default 96).
func TestTerrainAgainstSaves(t *testing.T) {
	world := os.Getenv("MCL_TERRAIN_SAVE")
	if world == "" {
		t.Skip("MCL_TERRAIN_SAVE names a saves/world directory")
	}
	limit := 96
	if v := os.Getenv("MCL_TERRAIN_CHUNKS"); v != "" {
		var n int
		for _, c := range v {
			n = n*10 + int(c-'0')
		}
		limit = n
	}
	instance := filepath.Dir(filepath.Dir(world))
	root := Root{ID: "terrain-oracle", Path: instance}
	if filepath.Base(filepath.Dir(instance)) == "versions" {
		root.Path, root.SelectedVersion = filepath.Dir(filepath.Dir(instance)), filepath.Base(instance)
	}
	s := testService(t)
	s.cfg.Roots = []Root{root}
	snapshot, level, err := s.SeedMapWorldgen(context.Background(), root.ID, "", filepath.Base(world))
	if err != nil {
		t.Fatal(err)
	}
	// MCL_TERRAIN_REGIONS reads chunks (and the seed) from another world of the
	// same Minecraft version, e.g. a vanilla-terrain server world.
	seed, regions := level.RandomSeed, world
	if other := os.Getenv("MCL_TERRAIN_REGIONS"); other != "" {
		if seed, err = mcworld.LevelSeed(other); err != nil {
			t.Fatal(err)
		}
		regions = other
	}
	sampler, err := NewLocalSampler(context.Background(), snapshot, seed)
	if err != nil {
		t.Fatal(err)
	}
	for _, dim := range []struct{ id, folder string }{{"minecraft:overworld", ""}, {"minecraft:the_nether", "DIM-1"}, {"minecraft:the_end", "DIM1"}} {
		info, ok := sampler.TerrainInfo(dim.id)
		if !ok {
			for _, is := range sampler.dimensions[dim.id].issues {
				t.Logf("%s issue %+v", dim.id, is)
			}
			t.Errorf("%s: no terrain program (mode %q generator %q settings %.80s)", dim.id, sampler.dimensions[dim.id].mode, sampler.dimensions[dim.id].generator, sampler.dimensions[dim.id].settings)
			continue
		}
		paths, _ := mcworld.RegionPaths(regions, dim.folder)
		t.Logf("%s: %d region files", dim.id, len(paths))
		var cells, cellHit, fluid, fluidHit, cols, topExact, top1, top3, chunks int
		var floorCols, floorExact, floor1, floor3, blockCols, blockHit, blockVanilla, land, landExact, land1, land3 int
		blockMiss, blockNetherrack, floorKind := map[string]int{}, 0, map[string]int{}
		netherShapes := map[[2]int32][]worldgen.CarveShape{}
		noiseCol := make([]TerrainSubstance, info.Height)
		var caveCells, caveHit, caveFalse int
		falseBlocks, falseY, lowBand := map[string]int{}, map[int]int{}, map[string]int{}
		rules := make([]string, info.Height)
		owCols, owHit, owUnruled, owMiss := 0, 0, 0, map[string]int{}
		preRules := make([]string, info.Height)
		started := time.Now()
		column := make([]TerrainSubstance, info.Height)
	regions:
		for _, path := range paths {
			r, err := mcworld.OpenRegion(path)
			if err != nil {
				continue
			}
			for i := 0; i < 1024; i += 37 {
				c, _ := r.Chunk(i%32, i/32)
				if c == nil || !strings.HasSuffix(c.Status, "full") {
					if c != nil && chunks == 0 {
						t.Logf("skip status %q", c.Status)
					}
					continue
				}
				if chunks++; chunks > limit {
					break regions
				}
				for lx := 1; lx < 16; lx += 4 {
					for lz := 2; lz < 16; lz += 4 {
						x, z := c.X*16+lx, c.Z*16+lz
						solidTop, colTop, _ := sampler.TerrainColumn(dim.id, x, z, column)
						copy(noiseCol, column)
						if dim.id == "minecraft:the_nether" && os.Getenv("MCL_NO_CARVE") == "" {
							pre := preRules[:]
							if !sampler.SurfaceColumn(dim.id, x, z, noiseCol, colTop, pre) {
								pre = nil
							}
							carveNether(seed, x, z, column, info.MinY, netherShapes, sampler.NetherCarverFluid(dim.id), pre)
							for y := 2; y < 32; y++ {
								if noiseCol[y] == TerrainSolid && column[y] != TerrainSolid {
									lowBand[c.Block(lx, y, lz)]++
								}
							}
							for y := 32; y < 121; y++ {
								carved := noiseCol[y] == TerrainSolid && column[y] == TerrainAir
								if c.Block(lx, y, lz) == "minecraft:cave_air" {
									caveCells++
									if carved {
										caveHit++
									}
								} else if carved {
									caveFalse++
									falseBlocks[c.Block(lx, y, lz)]++
									falseY[y/10]++
								}
							}
						}
						savedTop := info.MinY - 1
						for y := info.MinY + info.Height - 1; y >= info.MinY; y-- {
							want, known := savedSubstance(c.Block(lx, y, lz))
							if !known {
								continue
							}
							if want == TerrainSolid && savedTop < info.MinY {
								savedTop = y
							}
							got := column[y-info.MinY]
							cells++
							if got == want || want == TerrainSolid && got == TerrainSolid {
								cellHit++
							}
							if want == TerrainWater || want == TerrainLava {
								fluid++
								if got == want {
									fluidHit++
								}
							}
						}
						cols++
						if dim.id == "minecraft:overworld" && solidTop == savedTop && savedTop > info.MinY && sampler.SurfaceColumn(dim.id, x, z, column, colTop, rules) {
							owCols++
							want, got := c.Block(lx, savedTop, lz), rules[savedTop-info.MinY]
							switch {
							case got == "":
								owUnruled++
							case got == want:
								owHit++
							default:
								owMiss[got+" -> "+want]++
							}
						}
						if dim.id == "minecraft:the_nether" {
							// floor top: highest solid at or below Y 100 with a non-solid block above
							savedSolid := func(y int) bool { w, _ := savedSubstance(c.Block(lx, y, lz)); return w == TerrainSolid }
							floor := func(solid func(int) bool) int {
								for y := 100; y > info.MinY; y-- {
									if solid(y) && !solid(y+1) {
										return y
									}
								}
								return info.MinY - 1
							}
							sf := floor(savedSolid)
							pf := floor(func(y int) bool { return column[y-info.MinY] == TerrainSolid })
							if sf > info.MinY {
								floorCols++
								d := abs(sf - pf)
								if d > 0 {
									// classify: carvers write cave_air (and lava below Y 11); features
									// place wart blocks, stems, shroomlight, glowstone, basalt pillars
									lo, hi := min(sf, pf), max(sf, pf)+1
									kind := "terrain"
									for y := max(lo, info.MinY); y <= hi && y < info.MinY+info.Height; y++ {
										b := c.Block(lx, y, lz)
										// carved caves below the lava sea fill with lava; above it, lava is a spring/delta feature
										if b == "minecraft:cave_air" || y <= 31 && b == "minecraft:lava" {
											kind = "carver"
											break
										}
										if strings.Contains(b, "wart_block") || strings.Contains(b, "_stem") || strings.Contains(b, "hyphae") || b == "minecraft:shroomlight" || b == "minecraft:glowstone" || b == "minecraft:basalt" || b == "minecraft:blackstone" || b == "minecraft:magma_block" || b == "minecraft:lava" || strings.Contains(b, "fungus") || strings.Contains(b, "roots") || !strings.HasPrefix(b, "minecraft:") && !strings.Contains(b, "ore") {
											kind = "feature/structure"
										}
									}
									floorKind[kind]++
									if kind == "terrain" && os.Getenv("MCL_FLOOR_MISS") != "" {
										t.Logf("terrain floor miss %d,%d saved %d %s/%s pred %d %s/%s", x, z, sf, c.Block(lx, sf, lz), c.Block(lx, sf+1, lz), pf, c.Block(lx, pf, lz), c.Block(lx, pf+1, lz))
									}
								}
								if d > 0 && os.Getenv("MCL_FLOOR_MISS") == "all" {
									t.Logf("floor miss %d,%d saved %d %s (above %s) predicted %d saved-at-pred %s/%s", x, z, sf, c.Block(lx, sf, lz), c.Block(lx, sf+1, lz), pf, c.Block(lx, pf, lz), c.Block(lx, pf+1, lz))
								}
								if d == 0 {
									floorExact++
								}
								if d <= 1 {
									floor1++
								}
								if d <= 3 {
									floor3++
								}
								if d == 0 && sampler.SurfaceColumn(dim.id, x, z, noiseCol, colTop, rules) {
									blockCols++
									want := c.Block(lx, sf, lz)
									if want == "minecraft:netherrack" {
										blockNetherrack++
									}
									got := rules[sf-info.MinY]
									switch {
									case got == "":
										blockVanilla++
									case got == want:
										blockHit++
									default:
										blockMiss[got+" -> "+want]++
									}
								}
							}
						}
						if dim.id == "minecraft:the_end" && savedTop > info.MinY {
							land++
							d := abs(solidTop - savedTop)
							if d == 0 {
								landExact++
							}
							if d <= 1 {
								land1++
							}
							if d <= 3 {
								land3++
							}
						}
						d := solidTop - savedTop
						if d < 0 {
							d = -d
						}
						if d == 0 {
							topExact++
						}
						if d <= 1 {
							top1++
						}
						if d <= 3 {
							top3++
						}
						if d > 8 && cols%7 == 0 {
							t.Logf("%s %d,%d predicted top %d saved %d", dim.id, x, z, solidTop, savedTop)
						}
					}
				}
			}
		}
		pct := func(a, b int) float64 {
			if b == 0 {
				return 0
			}
			return 100 * float64(a) / float64(b)
		}
		t.Logf("%s: %d chunks %d columns in %v: top exact %.1f%% ±1 %.1f%% ±3 %.1f%%; 3D cells %.1f%% of %d; fluid %.1f%% of %d",
			dim.id, chunks, cols, time.Since(started).Round(time.Millisecond), pct(topExact, cols), pct(top1, cols), pct(top3, cols), pct(cellHit, cells), cells, pct(fluidHit, fluid), fluid)
		if owCols > 0 {
			t.Logf("%s top block on exact tops %d: match %.1f%% unruled %.1f%%", dim.id, owCols, pct(owHit, owCols), pct(owUnruled, owCols))
			type kv struct {
				k string
				n int
			}
			var ms []kv
			for k, n := range owMiss {
				ms = append(ms, kv{k, n})
			}
			sort.Slice(ms, func(i, j int) bool { return ms[i].n > ms[j].n })
			for i, m := range ms {
				if i < 15 {
					t.Logf("  top miss %s: %d", m.k, m.n)
				}
			}
		}
		if floorCols > 0 {
			t.Logf("%s floor-top: %d columns exact %.1f%% +-1 %.1f%% +-3 %.1f%%; top block on exact floors %d: match %.1f%% unruled %.1f%% (netherrack-only baseline %.1f%%)", dim.id, floorCols, pct(floorExact, floorCols), pct(floor1, floorCols), pct(floor3, floorCols), blockCols, pct(blockHit, blockCols), pct(blockVanilla, blockCols), pct(blockNetherrack, blockCols))
			t.Logf("%s floor-top misses by cause %v; terrain-only exact %.1f%%", dim.id, floorKind, pct(floorExact, floorExact+floorKind["terrain"]))
			for k, n := range blockMiss {
				if n*50 >= blockCols {
					t.Logf("  top block miss %4d %s", n, k)
				}
			}
		}
		if caveCells+caveFalse > 0 {
			t.Logf("%s carver: saved cave_air cells %d, predicted carved %.1f%%; predicted carved where save is not cave_air %d %v", dim.id, caveCells, pct(caveHit, caveCells), caveFalse, falseBlocks)
			t.Logf("carver fluid from data %+v", sampler.NetherCarverFluid(dim.id))
			t.Logf("false by y/10 %v; carved cells y2..31 in save %v", falseY, lowBand)
		}
		if dim.id == "minecraft:the_end" {
			t.Logf("%s land-only: %d of %d columns have land, top exact %.1f%% +-1 %.1f%% +-3 %.1f%%", dim.id, land, cols, pct(landExact, land), pct(land1, land), pct(land3, land))
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// carveNether applies the seed's nether_cave carver to a noise column:
// solid blocks it reaches become fluid.Substance(y).
func carveNether(seed int64, x, z int, column []TerrainSubstance, minY int, cache map[[2]int32][]worldgen.CarveShape, fluid CarverFluid, rules []string) {
	cx, cz := int32(x>>4), int32(z>>4)
	for dx := int32(-8); dx <= 8; dx++ {
		for dz := int32(-8); dz <= 8; dz++ {
			k := [2]int32{cx + dx, cz + dz}
			ss, ok := cache[k]
			if !ok {
				ss = worldgen.NetherCarves(seed, k[0], k[1])
				cache[k] = ss
			}
			for i := range ss {
				b := ss[i].Bounds()
				if x < b[0] || x > b[3] || z < b[2] || z > b[5] {
					continue
				}
				for y := max(b[1], minY); y <= b[4] && y-minY < len(column); y++ {
					if column[y-minY] == TerrainSolid && (rules == nil || rules[y-minY] != "minecraft:bedrock") && ss[i].Carved(x, y, z) {
						column[y-minY] = fluid.Substance(y)
					}
				}
			}
		}
	}
}
