package structure3d_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/structure3d"
)

// TestAssembleBlocksAgainstWorld compares Assemble output for mansion,
// end city and monument with fully generated chunks. STRUCTURE_BLOCKS_VERIFY
// lists region folders; optional STRUCTURE_BLOCKS_ONLY filters ids.
func TestAssembleBlocksAgainstWorld(t *testing.T) {
	dirs := os.Getenv("STRUCTURE_BLOCKS_VERIFY")
	if os.Getenv("ASM_NO_YUNG") != "" {
		SetPortYung(false)
		defer SetPortYung(true)
	}
	if dirs == "" {
		t.Skip("set STRUCTURE_BLOCKS_VERIFY")
	}
	jar := os.Getenv("STRUCTURE_BLOCKS_JAR")
	if jar == "" {
		jar = `D:\MC\.minecraft\versions\1.21.1\1.21.1.jar`
	}
	if _, err := os.Stat(jar); err != nil {
		t.Skip("jar required:", jar)
	}
	store := &Store{}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.OpenJar("game", jar); err != nil {
		t.Fatal(err)
	}
	// Optional STRUCTURE_BLOCKS_INSTANCE: an instance root whose mods/ jars
	// layer over the game jar (world datapacks come from the first dir).
	if inst := os.Getenv("STRUCTURE_BLOCKS_INSTANCE"); inst != "" {
		n := OpenModsDir(store, inst+"/mods")
		world := strings.Split(dirs, ";")[0]
		for strings.HasSuffix(strings.ToLower(filepath.Base(world)), "region") || strings.Contains(filepath.Base(world), "DIM") {
			world = filepath.Dir(world)
		}
		t.Logf("%s: %d mod jars, datapacks %v", inst, n, OpenWorldDatapacks(store, "world", world))
	}
	// ASM_MONUMENT_DY: Tectonic oceans.monument_offset of the save's pack.
	if v := os.Getenv("ASM_MONUMENT_DY"); v != "" {
		dy := 0
		fmt.Sscan(v, &dy)
		store.SetMonumentYOffset(dy)
	}
	want := map[string]bool{
		"minecraft:mansion": true, "minecraft:end_city": true, "minecraft:monument": true,
	}
	if only := os.Getenv("STRUCTURE_BLOCKS_ONLY"); only != "" {
		want = map[string]bool{}
		for _, id := range strings.Split(only, ",") {
			if !strings.Contains(id, ":") {
				id = "minecraft:" + id
			}
			want[id] = true
		}
	}
	type tally struct{ solid, solidOK, air, airOK, starts int }
	per := map[string]*tally{}
	miss := map[string]int{}
	for _, dir := range strings.Split(dirs, ";") {
		cc := &regionCache{dir: dir, regions: map[[2]int]*mcworld.Region{}, chunks: map[[2]int]*mcworld.Chunk{}}
		base := dir
		for strings.HasSuffix(strings.ToLower(filepath.Base(base)), "region") ||
			strings.Contains(filepath.Base(base), "DIM") {
			base = filepath.Dir(base)
		}
		seed, seedErr := mcworld.LevelSeed(base)
		if seedErr != nil {
			t.Logf("skip %s: %v", dir, seedErr)
			continue
		}
		minY, maxY := -64, 320
		if strings.Contains(dir, "DIM-1") || strings.Contains(dir, "the_nether") {
			minY, maxY = 0, 256
		}
		if strings.Contains(dir, "DIM1") || strings.Contains(dir, "the_end") {
			minY, maxY = 0, 256
		}
		paths, _ := filepath.Glob(filepath.Join(dir, "*.mca"))
		for _, pth := range paths {
			r, err := mcworld.OpenRegion(pth)
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
						ox, oz := s.ChunkX*16, s.ChunkZ*16
						y0 := int(s.Pieces[0].BB[1])
						for _, pc := range s.Pieces {
							if tp, ok := pc.Raw["TPY"]; ok {
								switch v := tp.(type) {
								case int32:
									y0 = int(v)
								case int64:
									y0 = int(v)
								}
								break
							}
						}
						height := func(x, z int) int { return y0 }
						sc, err := Assemble(store, s.ID, seed, ox+8, oz+8, height)
						if err != nil || sc == nil {
							t.Logf("%s @%d,%d: %v", s.ID, s.ChunkX, s.ChunkZ, err)
							continue
						}
						tl := per[s.ID]
						if tl == nil {
							tl = &tally{}
							per[s.ID] = tl
						}
						tl.starts++
						s0, k0 := tl.solid, tl.solidOK
						for _, b := range sc.Blocks {
							if b.Y < minY || b.Y > maxY {
								continue
							}
							name := b.State.Name
							got, ok := cc.block(b.X, b.Y, b.Z)
							if !ok {
								continue
							}
							if isAirName(name) {
								tl.air++
								if isAirName(got) {
									tl.airOK++
								} else {
									miss[fmt.Sprintf("%s: air -> %s", s.ID, got)]++
								}
								continue
							}
							tl.solid++
							if got == name || strings.TrimPrefix(got, "minecraft:") == strings.TrimPrefix(name, "minecraft:") {
								tl.solidOK++
							} else {
								miss[fmt.Sprintf("%s: %s -> %s", s.ID, name, got)]++
								if os.Getenv("ASM_MISS_Y") != "" {
									miss[fmt.Sprintf("%s: y%d dx%d dz%d %v", s.ID, b.Y, (b.X-ox+64)/16, (b.Z-oz+64)/16, isAirName(got) || got == "minecraft:water")]++
								}
							}
						}
						if os.Getenv("ASM_PER_START") != "" {
							t.Logf("  start %s %d,%d y%d: %d/%d", s.ID, s.ChunkX, s.ChunkZ, y0, tl.solidOK-k0, tl.solid-s0)
						}
					}
				}
			}
		}
	}
	ids := make([]string, 0, len(per))
	for id := range per {
		ids = append(ids, id)
	}
	for _, id := range ids {
		tl := per[id]
		pct := 0.0
		if tl.solid > 0 {
			pct = 100 * float64(tl.solidOK) / float64(tl.solid)
		}
		ap := 0.0
		if tl.air > 0 {
			ap = 100 * float64(tl.airOK) / float64(tl.air)
		}
		t.Logf("%s: %d starts; placed blocks %d/%d (%.2f%%); carved air %d/%d (%.2f%%)",
			id, tl.starts, tl.solidOK, tl.solid, pct, tl.airOK, tl.air, ap)
	}
	type kv struct {
		k string
		n int
	}
	var top []kv
	for k, n := range miss {
		top = append(top, kv{k, n})
	}
	for i := 0; i < len(top); i++ {
		for j := i + 1; j < len(top); j++ {
			if top[j].n > top[i].n {
				top[i], top[j] = top[j], top[i]
			}
		}
	}
	for i, e := range top {
		if i >= 40 {
			break
		}
		t.Logf("  mismatch %s: %d", e.k, e.n)
	}
	if len(per) == 0 {
		t.Fatal("no matching structure starts")
	}
}
