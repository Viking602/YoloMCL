package structure3d_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/structure3d"
	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// TestTemplateStructuresAgainstWorld checks the seeded shipwreck and ruined
// portal placement (template, rotation, mirror and the resulting X/Z bounding
// box) against saved structure starts.
func TestTemplateStructuresAgainstWorld(t *testing.T) {
	dirs := os.Getenv("WORLDGEN_VERIFY")
	jar := `D:\MC\.minecraft\versions\1.21.1\1.21.1.jar`
	if dirs == "" {
		t.Skip("set WORLDGEN_VERIFY")
	}
	s := &Store{}
	if err := s.OpenJar("game", jar); err != nil {
		t.Skip(err)
	}
	setups := map[string][]worldgen.RuinedPortalSetup{}
	total, ok := map[string]int{}, map[string]int{}
	shown := 0
	for _, dir := range strings.Split(dirs, ";") {
		seed, err := mcworld.LevelSeed(dir)
		if err != nil {
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
					for _, st := range c.Starts() {
						if len(st.Pieces) == 0 {
							continue
						}
						cx, cz := int32(st.ChunkX), int32(st.ChunkZ)
						var kind, tp, rot, mir string
						var pivot [3]int
						switch {
						case st.ID == "minecraft:shipwreck" || st.ID == "minecraft:shipwreck_beached":
							kind = "shipwreck"
							tp, rot = worldgen.Shipwreck(seed, cx, cz, st.ID == "minecraft:shipwreck_beached")
							mir, pivot = "NONE", [3]int{4, 0, 15}
						case strings.HasPrefix(st.ID, "minecraft:ruined_portal"):
							su, seen := setups[st.ID]
							if !seen {
								data, err := s.Read(DataPath("minecraft", "worldgen/structure", st.ID[10:], ".json"))
								if err == nil {
									var def struct {
										Setups []worldgen.RuinedPortalSetup `json:"setups"`
									}
									json.Unmarshal(data, &def)
									su = def.Setups
								}
								setups[st.ID] = su
							}
							if len(su) == 0 {
								continue
							}
							kind = "ruined_portal"
							tp, rot, mir, _ = worldgen.RuinedPortal(seed, cx, cz, su)
						default:
							continue
						}
						tm, err := LoadTemplate(s, tp)
						if err != nil {
							t.Fatal(err)
						}
						if kind == "ruined_portal" {
							pivot = [3]int{tm.Size[0] / 2, 0, tm.Size[2] / 2}
						}
						a := worldgen.TransformMirror([3]int{0, 0, 0}, mir, rot, pivot)
						b := worldgen.TransformMirror([3]int{tm.Size[0] - 1, 0, tm.Size[2] - 1}, mir, rot, pivot)
						x0, z0 := int(cx)*16, int(cz)*16
						want := fmt.Sprint(tp, " ", min(a[0], b[0])+x0, min(a[2], b[2])+z0, max(a[0], b[0])+x0, max(a[2], b[2])+z0)
						bb := st.Pieces[0].BB
						got := fmt.Sprint(st.Pieces[0].Template, " ", bb[0], bb[2], bb[3], bb[5])
						total[kind]++
						if got == want {
							ok[kind]++
						} else if shown < 10 {
							shown++
							t.Logf("%s %s chunk %d,%d saved %q predicted %q", dir, st.ID, cx, cz, got, want)
						}
					}
				}
			}
		}
	}
	for k, n := range total {
		t.Logf("%s: template + X/Z bounding box %d/%d exact", k, ok[k], n)
	}
}
