package modjar

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"math/rand/v2"
	"os"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/cubiomes"
)

type seededBiomeOracle struct {
	Seed    int64 `json:",string"`
	X, Y, Z int32
	Erosion uint64 `json:",string"`
	Biome   string
}

// The optional preparation writes inputs only. Expected values are produced by
// the independent Java reference outside this repository, never by this engine.
func TestInstalledSeededBiomeProgram(t *testing.T) {
	path := os.Getenv("MCL_BIOME_SOURCE_JAR")
	if path == "" {
		t.Skip("MCL_BIOME_SOURCE_JAR names the exact reference mod")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != "4771b487ac3138fccf6f897dbcb39041d0c4817500a0229fc0efcdf3ccd45560" {
		t.Fatal("reference JAR hash changed")
	}
	if output := os.Getenv("MCL_BIOME_SEEDED_INPUT"); output != "" {
		var rows []seededBiomeOracle
		r := rand.New(rand.NewPCG(1, 42))
		for _, seed := range []int64{0, 1, -1, 2904241419426706349, math.MaxInt64} {
			for _, p := range [][2]int32{{0, 0}, {255, 0}, {256, 0}, {257, 0}, {-257, -1}, {1023, 1024}, {1048575, -1048576}, {4194303, -4194304}} {
				rows = append(rows, seededBiomeOracle{Seed: seed, X: p[0], Y: 16, Z: p[1]})
			}
			for i := 0; i < 128; i++ {
				scale := int32(8192)
				if i >= 64 {
					scale = 4194304
				}
				rows = append(rows, seededBiomeOracle{Seed: seed, X: r.Int32N(scale*2) - scale, Y: r.Int32N(64), Z: r.Int32N(scale*2) - scale})
			}
		}
		raw, _ := json.Marshal(rows)
		if err := os.WriteFile(output, raw, 0600); err != nil {
			t.Fatal(err)
		}
		t.Logf("prepared %d independent reference queries", len(rows))
		return
	}
	raw, err := os.ReadFile("testdata/biome-source-c6c-1.2.1-seeded.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []seededBiomeOracle
	if err = json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	env, err := BootstrapBiomeSources(context.Background(), []Archive{{Name: path, Reader: &z.Reader}}, Options{Minecraft: "1.21.1", Loader: "neoforge", Side: "DEDICATED_SERVER", JavaMajor: 21})
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()
	program, err := env.BiomeProgram("c6c:new_end_biome_source")
	if err != nil {
		t.Fatal(err)
	}
	documents, err := cubiomes.BuiltinDocuments("1.21.1")
	if err != nil {
		t.Fatal(err)
	}
	docs := map[string]cubiomes.Document{}
	for _, d := range documents {
		docs[d.Kind+"/"+d.ID] = d
	}
	for _, entry := range z.File {
		parts := strings.SplitN(entry.Name, "/", 5)
		if len(parts) != 5 || parts[0] != "data" || parts[2] != "worldgen" || parts[3] != "biome" || !strings.HasSuffix(parts[4], ".json") {
			continue
		}
		b, e := archiveBytes(context.Background(), entry, 1<<20)
		if e != nil {
			t.Fatal(e)
		}
		id := parts[1] + ":" + strings.TrimSuffix(parts[4], ".json")
		docs["biome/"+id] = cubiomes.Document{Kind: "biome", ID: id, Data: b}
	}
	type workers struct {
		router *cubiomes.NoiseRouterSampler
		biome  *cubiomes.Sampler
	}
	cache := map[int64]workers{}
	biomes := map[string]int{}
	for i, q := range rows {
		w, ok := cache[q.Seed]
		if !ok {
			def := cubiomes.Definition{Version: "1.21.1", Seed: q.Seed, Settings: json.RawMessage(`"minecraft:end"`)}
			r, e := cubiomes.CompileNoiseRouter(context.Background(), docs, def)
			if e != nil {
				t.Fatal(e)
			}
			w.router = r.NewSampler()
			if _, e = program.NewBiomeSampler(context.Background(), w.router); e != nil {
				t.Fatal(e)
			}
			g, e := cubiomes.CompileCustomBiomeSource(context.Background(), docs, def, program)
			if e != nil {
				t.Fatal(e)
			}
			w.biome = g.NewSampler()
			cache[q.Seed] = w
		}
		// Independent raw-value calls have their own short-lived interpreter
		// worker; budget exhaustion must not accumulate over unrelated requests.
		if _, e := program.NewBiomeSampler(context.Background(), w.router); e != nil {
			t.Fatal(e)
		}
		erosion, ok := w.router.Compute(3, q.X*4, q.Y*4, q.Z*4)
		if !ok || math.Float64bits(erosion) != q.Erosion {
			t.Fatalf("row %d seed=%d quart=%d,%d: erosion %.17g/%016x want %016x", i, q.Seed, q.X, q.Z, erosion, math.Float64bits(erosion), q.Erosion)
		}
		id, ok := w.biome.NextNoiseBiome(int(q.X)*4, int(q.Y)*4, int(q.Z)*4)
		if !ok || id != q.Biome {
			t.Fatalf("row %d: biome %s/%v want %s", i, id, ok, q.Biome)
		}
		biomes[id]++
	}
	t.Logf("%d seeded coordinates: raw erosion bit-for-bit and biome IDs match the independent Java reference; %d biomes", len(rows), len(biomes))
}
