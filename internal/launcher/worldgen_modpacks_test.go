package launcher

import (
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/cubiomes"
)

// Real, checksum-pinned archives remain outside the Go project. This exercises
// ingestion, mod identity/config dispatch, density compilation, biome palette
// and rendering together, rather than only calling the region algorithm.
func TestWorldgenNativeModPacksOracle(t *testing.T) {
	dir := os.Getenv("MCL_WORLDGEN_FIXTURES")
	if dir == "" {
		t.Skip("MCL_WORLDGEN_FIXTURES points to external pinned mod/datapack artifacts")
	}
	fixturePath := os.Getenv("MCL_TERRABLENDER_PACKS_DENSE")
	if fixturePath == "" {
		fixturePath = "../../cubiomes/testdata/terrablender-bop-26.3-packs.json.gz"
	}
	file, err := os.Open(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var reader io.Reader = file
	if strings.HasSuffix(fixturePath, ".gz") {
		compressed, err := gzip.NewReader(file)
		if err != nil {
			t.Fatal(err)
		}
		defer compressed.Close()
		reader = compressed
	}
	var fixture struct {
		Cases []struct {
			Dimension, Seed string
			Packs           []string
			Grids           []struct {
				Range   [6]int
				Names   string `json:"fnv1a_names"`
				Regions string `json:"fnv1a_regions"`
				Samples []struct {
					Position [3]int
					Climate  [6]int64
					Region   int
					Biome    string
				}
			}
		}
	}
	if err := json.NewDecoder(reader).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	var lock map[string]struct{ Filename, SHA512 string }
	data, err := os.ReadFile("testdata/worldgen/fixtures.lock.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		t.Fatal(err)
	}
	verified := map[string]bool{}
	for _, name := range []string{"terralith", "tectonic"} {
		entry := lock[name]
		data, err := os.ReadFile(filepath.Join(dir, entry.Filename))
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha512.Sum512(data)); got != entry.SHA512 {
			t.Fatalf("changed %s archive", name)
		}
		verified["file/"+entry.Filename] = true
	}
	artifacts, err := cubiomes.BuiltinTerraBlenderArtifacts("26.3", "26.3.0.0.15")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	snapshots := map[string]*WorldgenSnapshot{}
	for _, c := range fixture.Cases {
		key := strings.Join(c.Packs, "+")
		t.Run(key+"/"+c.Seed, func(t *testing.T) {
			snapshot := snapshots[key]
			if snapshot == nil {
				input := WorldgenInput{Minecraft: "26.3", HasMods: true}
				for _, a := range artifacts {
					input.Sources = append(input.Sources, WorldgenSource{filepath.Join(dir, a.Filename), "mod:" + a.Filename})
				}
				for _, pack := range c.Packs {
					if !verified[pack] {
						t.Fatalf("unknown pack fixture %s", pack)
					}
					input.Sources = append(input.Sources, WorldgenSource{filepath.Join(dir, strings.TrimPrefix(pack, "file/")), "datapack:" + pack})
				}
				var err error
				snapshot, err = WorldgenIngest(ctx, input)
				if err != nil {
					t.Fatal(err)
				}
				snapshots[key] = snapshot
			}
			seed, err := strconv.ParseInt(c.Seed, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			sam, err := NewLocalSampler(ctx, snapshot, seed)
			if err != nil {
				t.Fatal(err)
			}
			id := "minecraft:overworld"
			d, exists := sam.dimensions[id]
			if !exists || d.engine == nil {
				t.Fatalf("mod/pack adapter not bound: %+v", sam.Coverage(id))
			}
			for _, grid := range c.Grids {
				r := cubiomes.Range{X: grid.Range[0], Y: grid.Range[1], Z: grid.Range[2], StepXZ: grid.Range[3], SizeX: grid.Range[4], SizeY: 1, SizeZ: grid.Range[5], StepY: 1}
				ids := make([]cubiomes.BiomeID, r.SizeX*r.SizeZ)
				probe := d.engine.NewSampler()
				if err := probe.GenerateIDs(ctx, r, ids); err != nil {
					t.Fatal(err)
				}
				names, regions := fnv.New64a(), fnv.New64a()
				for i, id := range ids {
					fmt.Fprintln(names, d.palette[id].ID)
					region, ok := probe.TerraBlenderRegionIndex(r.X+i%r.SizeX*r.StepXZ, r.Z+i/r.SizeX*r.StepXZ)
					if !ok {
						t.Fatal("missing region sampler")
					}
					fmt.Fprintln(regions, region)
				}
				for _, point := range grid.Samples {
					x, y, z := point.Position[0], point.Position[1], point.Position[2]
					i := (z-r.Z)/r.StepXZ*r.SizeX + (x-r.X)/r.StepXZ
					if got := d.palette[ids[i]].ID; got != point.Biome {
						t.Fatalf("at %v: biome %s want %s", point.Position, got, point.Biome)
					}
					if got, ok := probe.Climate(x, y, z); !ok || got != point.Climate {
						t.Fatalf("at %v: climate %v/%v want %v", point.Position, got, ok, point.Climate)
					}
					if got, ok := probe.TerraBlenderRegionIndex(x, z); !ok || got != point.Region {
						t.Fatalf("at %v: region %d/%v want %d", point.Position, got, ok, point.Region)
					}
				}
				wantNames, err := strconv.ParseUint(grid.Names, 16, 64)
				if err != nil {
					t.Fatal(err)
				}
				if got := names.Sum64(); got != wantNames {
					t.Fatalf("grid %v biome hash %016x want %s", grid.Range, got, grid.Names)
				}
				wantRegions, err := strconv.ParseUint(grid.Regions, 16, 64)
				if err != nil {
					t.Fatal(err)
				}
				if got := regions.Sum64(); got != wantRegions {
					t.Fatalf("grid %v region hash %016x want %s", grid.Range, got, grid.Regions)
				}
				if r.SizeX == 128 && r.SizeZ == 128 {
					tile, err := sam.Sample(ctx, SeedMapTileKey{Dimension: id, X: r.X / (128 * r.StepXZ), Z: r.Z / (128 * r.StepXZ), Y: r.Y, BlocksPerPixel: r.StepXZ})
					if err != nil {
						t.Fatal(err)
					}
					if len(tile.Biomes) == 0 {
						t.Fatal("empty modded tile")
					}
					for i, id := range ids {
						if tile.Image.RGBAAt(i%128, i/128) != d.palette[id].Color {
							t.Fatalf("render differs at cell%d", i)
						}
					}
				}
			}
		})
	}
}
