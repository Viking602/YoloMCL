package launcher

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/cubiomes"
)

func TestWorldgenOverlayOrderAndTagReplacement(t *testing.T) {
	for _, directory := range []bool{false, true} {
		t.Run(map[bool]string{false: "zip", true: "directory"}[directory], func(t *testing.T) {
			root := t.TempDir()
			previous := filepath.Join(root, "previous")
			writeTestFile(t, filepath.Join(previous, "data/test/tags/worldgen/biome/example.json"), []byte(`{"values":["test:previous"]}`))
			files := map[string][]byte{
				"pack.mcmeta":                                       []byte(`{"overlays":{"entries":[{"formats":[41,48],"directory":"z_first"},{"formats":{"min_inclusive":48,"max_inclusive":48},"directory":"a_last"},{"formats":49,"directory":"future"}]}}`),
				"data/test/worldgen/biome/example.json":             []byte(`{"chosen":"root"}`),
				"z_first/data/test/worldgen/biome/example.json":     []byte(`{"chosen":"first"}`),
				"a_last/data/test/worldgen/biome/example.json":      []byte(`{"chosen":"last"}`),
				"future/data/test/worldgen/biome/example.json":      []byte(`{"chosen":"future"}`),
				"data/test/tags/worldgen/biome/example.json":        []byte(`{"values":["test:discard"]}`),
				"a_last/data/test/tags/worldgen/biome/example.json": []byte(`{"values":["test:last"]}`),
			}
			pack := filepath.Join(root, "pack")
			if directory {
				for name, data := range files {
					writeTestFile(t, filepath.Join(pack, filepath.FromSlash(name)), data)
				}
			} else {
				writeTestFile(t, pack, zipBytes(t, files))
			}
			snapshot, err := WorldgenIngest(context.Background(), WorldgenInput{Minecraft: "1.21.1", Sources: []WorldgenSource{{previous, "datapack:previous"}, {pack, "datapack:pack"}}})
			if err != nil {
				t.Fatal(err)
			}
			doc := worldgenDoc(t, snapshot, "biome", "test:example")
			if doc.Source != "datapack:pack#a_last" || !bytes.Contains(doc.Data, []byte("last")) {
				t.Fatalf("wrong overlay: %+v", doc)
			}
			tag := worldgenDoc(t, snapshot, "biome_tag", "test:example")
			if !bytes.Contains(tag.Data, []byte("previous")) || !bytes.Contains(tag.Data, []byte("last")) || bytes.Contains(tag.Data, []byte("discard")) {
				t.Fatalf("tag retained shadowed root: %s", tag.Data)
			}
		})
	}
}

func TestWorldgenMinorFormatsAndInvalidMetadata(t *testing.T) {
	metadata := []byte(`{"overlays":{"entries":[{"min_format":[101,1],"max_format":101,"directory":"minor"},{"min_format":121,"max_format":[121,0],"directory":"current"}]}}`)
	for _, test := range []struct {
		format WorldgenPackFormat
		want   string
	}{{WorldgenPackFormat{101, 0}, ""}, {WorldgenPackFormat{101, 1}, "minor"}, {WorldgenPackFormat{101, 99}, "minor"}, {WorldgenPackFormat{121, 0}, "current"}, {WorldgenPackFormat{121, 1}, ""}} {
		overlays, err := worldgenPackOverlays(metadata, &test.format)
		if err != nil || (test.want == "" && len(overlays) != 0) || (test.want != "" && (len(overlays) != 1 || overlays[0] != test.want)) {
			t.Fatalf("%+v: %v %v", test, overlays, err)
		}
	}
	for _, metadata := range []string{`{"overlays":{"entries":[{"formats":48,"directory":"../escape"}]}}`, `{"overlays":{"entries":[{"min_format":121,"directory":"missing_max"}]}}`, `{"overlays":{"entries":[{"formats":[49,48],"directory":"inverted"}]}}`} {
		if _, err := worldgenPackOverlays([]byte(metadata), &WorldgenPackFormat{48, 0}); err == nil {
			t.Fatalf("accepted %s", metadata)
		}
	}
	if _, err := worldgenPackOverlays(metadata, nil); err == nil {
		t.Fatal("guessed snapshot pack version")
	}
}

// Opt-in test reads checksum-pinned public releases stored outside the project.
// These archives are not vendored into MCL or required for ordinary Go tests.
func TestWorldgenRealModPacks(t *testing.T) {
	dir := os.Getenv("MCL_WORLDGEN_FIXTURES")
	if dir == "" {
		t.Skip("set MCL_WORLDGEN_FIXTURES to the verified fixtures directory")
	}
	data, err := os.ReadFile("testdata/worldgen/fixtures.lock.json")
	if err != nil {
		t.Fatal(err)
	}
	var lock map[string]struct {
		Filename string `json:"filename"`
		SHA512   string `json:"sha512"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"terralith", "tectonic"} {
		data, err := os.ReadFile(filepath.Join(dir, lock[name].Filename))
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha512.Sum512(data)) != lock[name].SHA512 {
			t.Fatalf("changed %s fixture", name)
		}
	}
	data, err = os.ReadFile("testdata/worldgen/26.3-packs.json")
	if err != nil {
		t.Fatal(err)
	}
	var oracle struct {
		Cases []struct {
			Packs                                  []string
			Seed                                   string
			TileX, TileZ, Y, BlocksPerPixel, Cells int
			FNV1ANames                             string `json:"fnv1aNames"`
		}
	}
	if err := json.Unmarshal(data, &oracle); err != nil {
		t.Fatal(err)
	}
	var dense struct {
		Cases []struct {
			Packs        []string
			Seed         string
			TileX, TileZ int
			Samples      []struct {
				Position [3]int
				Climate  [6]int64
				Biome    string
			}
		}
	}
	if os.Getenv("MCL_WORLDGEN_DENSE") != "" {
		data, err := os.ReadFile(filepath.Join(dir, "pack-probe-dense.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &dense); err != nil {
			t.Fatal(err)
		}
	}
	for _, names := range [][]string{{"terralith"}, {"tectonic"}, {"terralith", "tectonic"}} {
		t.Run(strings.Join(names, "+"), func(t *testing.T) {
			var sources []WorldgenSource
			var packNames []string
			for _, name := range names {
				sources = append(sources, WorldgenSource{filepath.Join(dir, lock[name].Filename), "datapack:" + name})
				packNames = append(packNames, "file/"+lock[name].Filename)
			}
			snapshot, err := WorldgenIngest(context.Background(), WorldgenInput{Minecraft: "26.3", Sources: sources})
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, doc := range snapshot.Documents {
				if strings.HasPrefix(doc.Source, "datapack:") {
					count++
				}
			}
			if count < 10 || hasWorldgenIssue(snapshot, "source_unreadable") || hasWorldgenIssue(snapshot, "invalid_json") || hasWorldgenIssue(snapshot, "pack_metadata") {
				t.Fatalf("resources=%d issues=%v", count, snapshot.Issues)
			}
			t.Logf("%d effective pack resources, snapshot %s", count, snapshot.Hash)
			samplers := map[int64]*LocalSampler{}
			checked := 0
			for _, test := range oracle.Cases {
				if slices.Equal(test.Packs, packNames) {
					seed, err := strconv.ParseInt(test.Seed, 10, 64)
					if err != nil {
						t.Fatal(err)
					}
					sampler := samplers[seed]
					if sampler == nil {
						sampler, err = NewLocalSampler(context.Background(), snapshot, seed)
						if err != nil {
							t.Fatal(err)
						}
						samplers[seed] = sampler
					}
					coverage := sampler.Coverage("minecraft:overworld")
					if coverage.Terrain != "multi_noise" {
						t.Fatalf("no actual terrain sampling: %+v", coverage)
					}
					tile, err := sampler.Sample(context.Background(), SeedMapTileKey{Dimension: "minecraft:overworld", X: test.TileX, Z: test.TileZ, Y: test.Y, BlocksPerPixel: test.BlocksPerPixel})
					if err != nil {
						t.Fatal(err)
					}
					if len(tile.Biomes) == 0 {
						t.Fatal("real pack produced no biome map")
					}
					dimension := sampler.dimensions["minecraft:overworld"]
					ids := make([]cubiomes.BiomeID, test.Cells)
					grid := cubiomes.Range{X: test.TileX * 512, Y: test.Y, Z: test.TileZ * 512, SizeX: 128, SizeY: 1, SizeZ: 128, StepXZ: 4, StepY: 1}
					if err := dimension.engine.NewSampler().GenerateIDs(context.Background(), grid, ids); err != nil {
						t.Fatal(err)
					}
					hash := fnv.New64a()
					for i, id := range ids {
						io.WriteString(hash, dimension.engine.BiomeName(id))
						hash.Write([]byte{'\n'})
						x, z := i%128, i/128
						if !(test.TileX == 0 && x == 0 || test.TileZ == 0 && z == 0) && tile.Image.RGBAAt(x, z) != dimension.palette[id].Color {
							t.Fatalf("rendered biome differs at %d,%d", x, z)
						}
					}
					if got := fmt.Sprintf("%016x", hash.Sum64()); got != test.FNV1ANames {
						t.Errorf("seed%s tile(%d,%d) got%s want%s", test.Seed, test.TileX, test.TileZ, got, test.FNV1ANames)
					}
					for _, detail := range dense.Cases {
						if slices.Equal(detail.Packs, packNames) && detail.Seed == test.Seed && detail.TileX == test.TileX && detail.TileZ == test.TileZ {
							climateMismatches, biomeMismatches := 0, 0
							probe := dimension.engine.NewSampler()
							for i, point := range detail.Samples {
								climate, ok := probe.Climate(point.Position[0], point.Position[1], point.Position[2])
								if !ok || climate != point.Climate {
									climateMismatches++
									if climateMismatches <= 3 {
										t.Logf("seed%s at%v climate got%v want%v", test.Seed, point.Position, climate, point.Climate)
									}
								}
								if name := dimension.engine.BiomeName(ids[i]); name != point.Biome {
									biomeMismatches++
									if biomeMismatches <= 3 {
										t.Logf("seed%s at%v biome got%s want%s", test.Seed, point.Position, name, point.Biome)
									}
								}
							}
							if climateMismatches != 0 || biomeMismatches != 0 {
								t.Errorf("seed%s tile(%d,%d): %d climate and %d biome mismatches", test.Seed, test.TileX, test.TileZ, climateMismatches, biomeMismatches)
							}
						}
					}
					checked++
				}
			}
			if checked != 9 {
				t.Fatalf("checked%d cases; want9", checked)
			}
		})
	}
}
