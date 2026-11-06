package worldgen_test

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/worldgen"
)

const vanillaJar = `D:\MC\.minecraft\versions\1.21.4-Fabric 0.17.2\1.21.4-Fabric 0.17.2.jar`

func portalSetups(t *testing.T) map[string][]RuinedPortalSetup {
	z, err := zip.OpenReader(vanillaJar)
	if err != nil {
		t.Skip(err)
	}
	defer z.Close()
	out := map[string][]RuinedPortalSetup{}
	for _, f := range z.File {
		if !strings.HasPrefix(f.Name, "data/minecraft/worldgen/structure/ruined_portal") {
			continue
		}
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		var def struct {
			Setups []RuinedPortalSetup `json:"setups"`
		}
		json.Unmarshal(b, &def)
		out["minecraft:"+strings.TrimSuffix(strings.TrimPrefix(f.Name, "data/minecraft/worldgen/structure/"), ".json")] = def.Setups
	}
	return out
}

var orientByData = map[string]string{"0": "south", "1": "west", "2": "north", "3": "east"}

// TestScatteredAgainstWorld checks seeded template/rotation/mirror/orientation
// of shipwrecks, ruined portals and scattered features against saved starts.
func TestScatteredAgainstWorld(t *testing.T) {
	dirs := os.Getenv("WORLDGEN_VERIFY")
	if dirs == "" {
		t.Skip("set WORLDGEN_VERIFY")
	}
	setups := portalSetups(t)
	total, ok := map[string]int{}, map[string]int{}
	shown := 0
	for _, dir := range splitList(dirs) {
		seed, err := mcworld.LevelSeed(dir)
		if err != nil {
			t.Log(err)
			continue
		}
		paths, _ := mcworld.RegionPaths(dir, "")
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
						if len(s.Pieces) == 0 {
							continue
						}
						cx, cz := int32(s.ChunkX), int32(s.ChunkZ)
						pc := s.Pieces[0]
						var kind, got, want string
						switch {
						case s.ID == "minecraft:shipwreck" || s.ID == "minecraft:shipwreck_beached":
							kind = "shipwreck"
							tp, rot := Shipwreck(seed, cx, cz, s.ID == "minecraft:shipwreck_beached")
							x, _ := pieceInt(pc.Raw, "TPX")
							z, _ := pieceInt(pc.Raw, "TPZ")
							want = fmt.Sprint(tp, rot, int(cx)*16, int(cz)*16)
							got = fmt.Sprint(pc.Template, pc.Rotation, x, z)
						case strings.HasPrefix(s.ID, "minecraft:ruined_portal"):
							su, known := setups[s.ID]
							if !known {
								continue
							}
							kind = "ruined_portal"
							tp, rot, mir, _ := RuinedPortal(seed, cx, cz, su)
							want = fmt.Sprint(tp, rot, mir)
							rot2 := pc.Rotation
							if rot2 == "" {
								rot2 = fmt.Sprint(pc.Raw["Rotation"])
							}
							got = fmt.Sprint(pc.Template, rot2, pc.Raw["Mirror"])
							if shown == 0 && rot2 != rot {
								t.Logf("raw keys %v", pc.Raw)
							}
						case s.ID == "minecraft:desert_pyramid" || s.ID == "minecraft:jungle_pyramid" || s.ID == "minecraft:swamp_hut":
							kind = "scattered"
							want = ScatteredOrientation(seed, cx, cz)
							got = orientByData[fmt.Sprint(pc.Raw["O"])]
						default:
							continue
						}
						total[kind]++
						if got == want {
							ok[kind]++
						} else if shown < 8 {
							shown++
							t.Logf("%s %s chunk %d,%d saved %q predicted %q", dir, s.ID, cx, cz, got, want)
						}
					}
				}
			}
		}
	}
	for k, n := range total {
		t.Logf("%s: %d/%d exact", k, ok[k], n)
	}
}
