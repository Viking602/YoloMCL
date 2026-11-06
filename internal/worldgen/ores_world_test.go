package worldgen_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/worldgen"
)

type saveChunks struct {
	regions map[[2]int]*mcworld.Region
	chunks  map[[2]int]*mcworld.Chunk
}

func openSave(dir string) *saveChunks {
	s := &saveChunks{regions: map[[2]int]*mcworld.Region{}, chunks: map[[2]int]*mcworld.Chunk{}}
	paths, _ := mcworld.RegionPaths(dir, "")
	for _, p := range paths {
		var rx, rz int
		if _, err := fmt.Sscanf(filepath.Base(p), "r.%d.%d.mca", &rx, &rz); err != nil {
			continue
		}
		if r, err := mcworld.OpenRegion(p); err == nil {
			s.regions[[2]int{rx, rz}] = r
		}
	}
	return s
}

func (s *saveChunks) chunk(cx, cz int) *mcworld.Chunk {
	k := [2]int{cx, cz}
	if c, ok := s.chunks[k]; ok {
		return c
	}
	var c *mcworld.Chunk
	if r := s.regions[[2]int{cx >> 5, cz >> 5}]; r != nil {
		c, _ = r.Chunk(cx&31, cz&31)
		if c != nil && !strings.HasSuffix(c.Status, "full") {
			c = nil
		}
	}
	s.chunks[k] = c
	return c
}

func (s *saveChunks) block(x, y, z int) string {
	c := s.chunk(x>>4, z>>4)
	if c == nil {
		return ""
	}
	return c.Block(x&15, y, z&15)
}

var stoneFamily = map[string]int{"minecraft:stone": 1, "minecraft:granite": 1, "minecraft:diorite": 1, "minecraft:andesite": 1, "minecraft:deepslate": 2, "minecraft:tuff": 2}

func isAir(b string) bool {
	return b == "minecraft:air" || b == "minecraft:cave_air" || b == "minecraft:void_air"
}

// TestOresAgainstWorld predicts every ore feature block for interior chunks
// of a saved 1.21 world and checks the saved block at each predicted cell.
func TestOresAgainstWorld(t *testing.T) {
	dir, ver := os.Getenv("WORLDGEN_ORES"), os.Getenv("WORLDGEN_ORES_VERSION")
	if dir == "" || ver == "" {
		t.Skip("set WORLDGEN_ORES and WORLDGEN_ORES_VERSION")
	}
	data, exact, err := LoadOreData(ver)
	if err != nil || !exact {
		t.Fatalf("ore data %s: %v exact=%v", ver, err, exact)
	}
	for _, o := range strings.Split(os.Getenv("WORLDGEN_ORES_INDEX"), ",") {
		var id string
		var idx int
		if n, _ := fmt.Sscanf(strings.Replace(o, "=", " ", 1), "%s %d", &id, &idx); n == 2 {
			for i := range data.Features {
				if data.Features[i].ID == id {
					data.Features[i].Index = idx
				}
			}
		}
	}
	seed, err := mcworld.LevelSeed(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := openSave(dir)
	w := &OreWorld{
		Biome: func(x, y, z int) string {
			if c := s.chunk(x>>4, z>>4); c != nil {
				return c.Biome(x&15, y, z&15)
			}
			return ""
		},
		Base: func(x, y, z int) int {
			b := s.block(x, y, z)
			if f, ok := stoneFamily[b]; ok {
				return f
			}
			if strings.HasSuffix(b, "_ore") {
				if strings.Contains(b, "deepslate_") {
					return 2
				}
				return 1
			}
			return 0
		},
		NearAir: func(x, y, z int) bool {
			return isAir(s.block(x+1, y, z)) || isAir(s.block(x-1, y, z)) || isAir(s.block(x, y+1, z)) || isAir(s.block(x, y-1, z)) || isAir(s.block(x, y, z+1)) || isAir(s.block(x, y, z-1))
		},
	}
	type tally struct{ pred, hit int }
	byFeature := map[string]*tally{}
	byOre := map[string]*tally{}
	saved := map[string]int{}
	covered := map[string]int{}
	var keys [][2]int
	for k := range s.regions {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i][0]*1000+keys[i][1] < keys[j][0]*1000+keys[j][1] })
	chunks := 0
	for _, rk := range keys {
		for i := 0; i < 1024 && chunks < 300; i++ {
			cx, cz := rk[0]*32+i%32, rk[1]*32+i/32
			ok := true
			for dx := -2; dx <= 2 && ok; dx++ {
				for dz := -2; dz <= 2; dz++ {
					if s.chunk(cx+dx, cz+dz) == nil {
						ok = false
						break
					}
				}
			}
			if !ok {
				continue
			}
			chunks++
			pred := map[[3]int]OreCell{}
			for dx := -1; dx <= 1; dx++ {
				for dz := -1; dz <= 1; dz++ {
					ChunkOres(seed, int32(cx+dx), int32(cz+dz), data.Features, w, func(c OreCell) {
						if c.X>>4 == cx && c.Z>>4 == cz {
							pred[[3]int{c.X, c.Y, c.Z}] = c
						}
					})
				}
			}
			for p, c := range pred {
				ft := byFeature[c.Feature.ID]
				if ft == nil {
					ft = &tally{}
					byFeature[c.Feature.ID] = ft
				}
				ot := byOre[c.Feature.Ore]
				if ot == nil {
					ot = &tally{}
					byOre[c.Feature.Ore] = ot
				}
				ft.pred++
				ot.pred++
				if s.block(p[0], p[1], p[2]) == c.Block {
					ft.hit++
					ot.hit++
				}
			}
			ch := s.chunk(cx, cz)
			for y := -64; y < 320; y++ {
				for z := 0; z < 16; z++ {
					for x := 0; x < 16; x++ {
						b := ch.Block(x, y, z)
						if !strings.HasSuffix(b, "_ore") {
							continue
						}
						ore := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(b, "minecraft:"), "deepslate_"), "_ore")
						saved[ore]++
						if c, ok := pred[[3]int{cx*16 + x, y, cz*16 + z}]; ok && c.Block == b {
							covered[ore]++
						}
					}
				}
			}
		}
	}
	if chunks == 0 {
		t.Fatal("no interior chunks")
	}
	var ids []string
	for id := range byFeature {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		f := byFeature[id]
		t.Logf("SAVE feature %-30s predicted %6d matched %6d (%.1f%%)", id, f.pred, f.hit, 100*float64(f.hit)/float64(max(1, f.pred)))
	}
	for ore, n := range saved {
		o := byOre[ore]
		if o == nil {
			o = &tally{}
		}
		t.Logf("SAVE ore %-9s saved %6d recall %.1f%%  predicted %6d precision %.1f%%", ore, n, 100*float64(covered[ore])/float64(n), o.pred, 100*float64(o.hit)/float64(max(1, o.pred)))
	}
	t.Logf("SAVE %s: %d interior chunks", dir, chunks)
}
