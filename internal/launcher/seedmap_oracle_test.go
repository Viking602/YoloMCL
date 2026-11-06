package launcher

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cubiomesOracle is JSON written by a cubiomes-linked program kept under
// .cache; it is test-only reference data and never part of the build.
type cubiomesOracle struct {
	Cases []struct {
		Version     string        `json:"version"`
		Seed        int64         `json:"seed"`
		Strongholds [][2]int      `json:"strongholds"`
		Fortresses  []oracleStart `json:"fortresses"`
		EndCities   []oracleStart `json:"endcities"`
	} `json:"cases"`
}

type oracleStart struct {
	Chunk  [2]int        `json:"chunk"`
	Pieces []oraclePiece `json:"pieces"`
}

type oraclePiece struct {
	Name     string `json:"name"`
	Type     int    `json:"type"`
	Rot      int    `json:"rot"`
	Depth    int    `json:"depth"`
	Pos      [3]int `json:"pos"`
	BB0, BB1 [3]int
}

func loadCubiomesOracle(t *testing.T) *cubiomesOracle {
	p := os.Getenv("MCL_CUBIOMES_ORACLE")
	if p == "" {
		t.Skip("MCL_CUBIOMES_ORACLE points at oracle JSON")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var o cubiomesOracle
	if err := json.Unmarshal(data, &o); err != nil {
		t.Fatal(err)
	}
	return &o
}

// TestStrongholdRingsAgainstCubiomes: MCL_ORACLE_INSTANCES lists
// version=versionDir pairs (';'-separated) of instances to sample from.
func TestStrongholdRingsAgainstCubiomes(t *testing.T) {
	o := loadCubiomesOracle(t)
	instances := map[string]string{}
	for _, pair := range strings.Split(os.Getenv("MCL_ORACLE_INSTANCES"), ";") {
		if v, dir, ok := strings.Cut(pair, "="); ok {
			instances[v] = dir
		}
	}
	for version, dir := range instances {
		root := Root{ID: "oracle", Path: filepath.Dir(filepath.Dir(dir)), SelectedVersion: filepath.Base(dir)}
		s := testService(t)
		s.cfg.Roots = []Root{root}
		snapshot, _, err := s.SeedMapWorldgen(context.Background(), root.ID, "", "")
		if err != nil {
			t.Fatalf("%s: %v", version, err)
		}
		total, hit, seeds, seedsAll := 0, 0, 0, 0
		for _, c := range o.Cases {
			if c.Version != version {
				continue
			}
			sampler, err := NewLocalSampler(context.Background(), snapshot, c.Seed)
			if err != nil {
				t.Fatal(err)
			}
			var chunks [][2]int32
			for _, p := range sampler.placements {
				if p.rings != nil && p.inDimension("minecraft:overworld") {
					if chunks, err = sampler.ringChunks(context.Background(), p, "minecraft:overworld"); err != nil {
						t.Fatal(err)
					}
				}
			}
			got := map[[2]int32]bool{}
			for _, ch := range chunks {
				got[ch] = true
			}
			miss := 0
			for _, sh := range c.Strongholds {
				total++
				if got[[2]int32{int32(sh[0] >> 4), int32(sh[1] >> 4)}] {
					hit++
				} else {
					miss++
					if miss <= 3 {
						t.Logf("%s seed %d: missing chunk %d,%d", version, c.Seed, sh[0]>>4, sh[1]>>4)
					}
				}
			}
			seeds++
			if miss == 0 && len(chunks) == len(c.Strongholds) {
				seedsAll++
			}
		}
		t.Logf("ORACLE strongholds %s: %d/%d positions, %d/%d seeds exact", version, hit, total, seedsAll, seeds)
	}
}

// TestBiomesAgainstCubiomes diffs quart biomes at y=0 (MCL_CUBIOMES_BIOMES).
func TestBiomesAgainstCubiomes(t *testing.T) {
	p := os.Getenv("MCL_CUBIOMES_BIOMES")
	if p == "" {
		t.Skip("MCL_CUBIOMES_BIOMES points at biome oracle JSON")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var o struct {
		Cases []struct {
			Version string   `json:"version"`
			Seed    int64    `json:"seed"`
			Points  [][3]any `json:"points"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &o); err != nil {
		t.Fatal(err)
	}
	instances := map[string]string{}
	for _, pair := range strings.Split(os.Getenv("MCL_ORACLE_INSTANCES"), ";") {
		if v, dir, ok := strings.Cut(pair, "="); ok {
			instances[v] = dir
		}
	}
	snaps := map[string]*WorldgenSnapshot{}
	for _, c := range o.Cases {
		dir := instances[c.Version]
		if dir == "" {
			continue
		}
		if snaps[c.Version] == nil {
			root := Root{ID: "oracle", Path: filepath.Dir(filepath.Dir(dir)), SelectedVersion: filepath.Base(dir)}
			s := testService(t)
			s.cfg.Roots = []Root{root}
			snap, _, err := s.SeedMapWorldgen(context.Background(), root.ID, "", "")
			if err != nil {
				t.Fatal(err)
			}
			snaps[c.Version] = snap
		}
		sampler, err := NewLocalSampler(context.Background(), snaps[c.Version], c.Seed)
		if err != nil {
			t.Fatal(err)
		}
		hit, shown := 0, 0
		for _, pt := range c.Points {
			qx, qz, want := int(pt[0].(float64)), int(pt[1].(float64)), "minecraft:"+pt[2].(string)
			got, _ := sampler.BiomeAt("minecraft:overworld", qx*4, 0, qz*4)
			if got == want {
				hit++
			} else if shown++; shown <= 4 {
				t.Logf("%s seed %d quart %d,%d: cubiomes %s ours %s", c.Version, c.Seed, qx, qz, want, got)
			}
		}
		t.Logf("ORACLE biomes %s seed %d: %d/%d", c.Version, c.Seed, hit, len(c.Points))
	}
}
