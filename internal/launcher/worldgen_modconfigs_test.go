package launcher

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Viking602/YoloMCL/cubiomes"
)

// Config fixtures come from the installed mod's actual registration and
// resolver behavior, including biome fallbacks and zero-weight End areas.
func TestWorldgenNativeModConfigOracle(t *testing.T) {
	file, err := os.Open("../../cubiomes/testdata/terrablender-bop-26.3-configs.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var fixture struct {
		Variants []struct {
			Name    string
			Toggles map[string]bool
			Profile struct {
				Dimensions struct {
					Overworld, Nether struct {
						RegionSize int `json:"region_size"`
						Regions    []cubiomes.TerraBlenderRegion
					}
					End map[string]struct {
						Size    int
						Entries []cubiomes.TerraBlenderWeightedBiome
					}
				}
			}
			Cases []struct {
				Dimension, Seed string
				Grids           []struct {
					Range   [6]int
					Names   string `json:"fnv1a_names"`
					Samples []struct {
						Position [3]int
						Climate  [6]int64
						Region   int
						Biome    string
					}
				}
			}
		}
	}
	if err := json.NewDecoder(reader).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, v := range fixture.Variants {
		t.Run(v.Name, func(t *testing.T) {
			ow, nether, end := v.Profile.Dimensions.Overworld, v.Profile.Dimensions.Nether, v.Profile.Dimensions.End
			tb := fmt.Sprintf("[general]\noverworld_region_size=%d\nnether_region_size=%d\nvanilla_overworld_region_weight=%d\nvanilla_nether_region_weight=%d\n[end]\nhighlands_biome_size=%d\nmidlands_biome_size=%d\nedge_biome_size=%d\nisland_biome_size=%d\nvanilla_end_highlands_weight=%d\nvanilla_end_midlands_weight=%d\nvanilla_end_barrens_weight=%d\nvanilla_small_end_islands_weight=%d\n",
				ow.RegionSize, nether.RegionSize, ow.Regions[0].Weight, nether.Regions[0].Weight, end["highlands"].Size, end["midlands"].Size, end["edge"].Size, end["islands"].Size, end["highlands"].Entries[0].Weight, end["midlands"].Entries[0].Weight, end["edge"].Entries[0].Weight, end["islands"].Entries[0].Weight)
			bop := fmt.Sprintf("[overworld]\nbop_primary_overworld_region_weight=%d\nbop_secondary_overworld_region_weight=%d\nbop_overworld_rare_region_weight=%d\n[nether]\nbop_nether_region_weight=%d\nbop_nether_rare_region_weight=%d\n", ow.Regions[1].Weight, ow.Regions[2].Weight, ow.Regions[3].Weight, nether.Regions[1].Weight, nether.Regions[2].Weight)
			toggles, err := json.Marshal(v.Toggles)
			if err != nil {
				t.Fatal(err)
			}
			configs := map[string][]byte{"terrablender.toml": []byte(tb), "biomesoplenty/generation.toml": []byte(bop), "biomesoplenty/biome_toggles.json": toggles}
			var snapshot *WorldgenSnapshot
			if dir := os.Getenv("MCL_WORLDGEN_FIXTURES"); dir != "" {
				artifacts, err := cubiomes.BuiltinTerraBlenderArtifacts("26.3", "26.3.0.0.15")
				if err != nil {
					t.Fatal(err)
				}
				input := WorldgenInput{Minecraft: "26.3", HasMods: true, ModConfigs: configs}
				for _, a := range artifacts {
					input.Sources = append(input.Sources, WorldgenSource{filepath.Join(dir, a.Filename), "mod:" + a.Filename})
				}
				snapshot, err = WorldgenIngest(ctx, input)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				snapshot = nativeModSnapshot(t)
				snapshot.ModConfigs = configs
				rehashNativeModSnapshot(t, snapshot)
			}
			for _, c := range v.Cases {
				t.Run(c.Dimension+"/"+c.Seed, func(t *testing.T) {
					seed, err := strconv.ParseInt(c.Seed, 10, 64)
					if err != nil {
						t.Fatal(err)
					}
					sam, err := NewLocalSampler(ctx, snapshot, seed)
					if err != nil {
						t.Fatal(err)
					}
					id := map[string]string{"overworld": "minecraft:overworld", "nether": "minecraft:the_nether", "end": "minecraft:the_end"}[c.Dimension]
					d := sam.dimensions[id]
					if d.engine == nil {
						t.Fatalf("config adapter not bound: %+v", sam.Coverage(id))
					}
					for gi, grid := range c.Grids {
						r := cubiomes.Range{X: grid.Range[0], Y: grid.Range[1], Z: grid.Range[2], StepXZ: grid.Range[3], SizeX: grid.Range[4], SizeY: 1, SizeZ: grid.Range[5], StepY: 1}
						out := make([]cubiomes.BiomeID, r.SizeX*r.SizeZ)
						probe := d.engine.NewSampler()
						if err := probe.GenerateIDs(ctx, r, out); err != nil {
							t.Fatal(err)
						}
						hash := fnv.New64a()
						for _, id := range out {
							fmt.Fprintln(hash, d.palette[id].ID)
						}
						want, err := strconv.ParseUint(grid.Names, 16, 64)
						if err != nil {
							t.Fatal(err)
						}
						if hash.Sum64() != want {
							t.Fatalf("grid %v: %016x want %s", grid.Range, hash.Sum64(), grid.Names)
						}
						for _, point := range grid.Samples {
							x, y, z := point.Position[0], point.Position[1], point.Position[2]
							index := (z-r.Z)/r.StepXZ*r.SizeX + (x-r.X)/r.StepXZ
							if got := d.palette[out[index]].ID; got != point.Biome {
								t.Fatalf("at %v biome %s want %s", point.Position, got, point.Biome)
							}
							if c.Dimension != "end" {
								if got, ok := probe.Climate(x, y, z); !ok || got != point.Climate {
									t.Fatalf("climate differs at %v", point.Position)
								}
								if got, ok := probe.TerraBlenderRegionIndex(x, z); !ok || got != point.Region {
									t.Fatalf("region differs at %v", point.Position)
								}
							}
						}
						// The first row has identical search history in the 64-wide
						// oracle and MCL's 128-wide image; later rows need not.
						if c.Seed == "0" && gi == 0 {
							tile, err := sam.Sample(ctx, SeedMapTileKey{Dimension: id, X: r.X / (128 * r.StepXZ), Z: r.Z / (128 * r.StepXZ), Y: r.Y, BlocksPerPixel: r.StepXZ})
							if err != nil {
								t.Fatal(err)
							}
							for i := range r.SizeX {
								if tile.Image.RGBAAt(i, 0) != d.palette[out[i]].Color {
									t.Fatalf("config render differs at column %d", i)
								}
							}
						}
					}
				})
			}
		})
	}
}
