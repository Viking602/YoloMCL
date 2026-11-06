package cubiomes

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

var referenceCases = []struct {
	name string
	cfg  Config
}{
	{"b1.7.3/Overworld", Config{Version: "b1.7.3"}},
	{"1.6.4/Overworld", Config{Version: "1.6.4"}},
	{"1.7.10/Overworld", Config{Version: "1.7.10"}},
	{"1.12.2/Overworld", Config{Version: "1.12.2"}},
	{"1.16.5/Overworld", Config{Version: "1.16.5"}},
	{"1.17.1/Overworld", Config{Version: "1.17.1"}},
	{"1.20.1/Overworld", Config{Version: "1.20.1"}},
	{"1.20.1/Large", Config{Version: "1.20.1", LargeBiomes: true}},
	{"1.17.1/Nether", Config{Version: "1.17.1", Dimension: Nether}},
	{"1.17.1/End", Config{Version: "1.17.1", Dimension: End}},
	{"1.20.1/End", Config{Version: "1.20.1", Dimension: End}},
}
var referenceRange = Range{X: 8192, Y: 64, Z: -8192, SizeX: 128, SizeY: 1, SizeZ: 128, StepXZ: 4, StepY: 4}

func TestReferenceBenchmarkHashes(t *testing.T) {
	b, err := os.ReadFile("testdata/benchmark-hashes.json")
	if err != nil {
		t.Fatal(err)
	}
	var hashes map[string]string
	if err = json.Unmarshal(b, &hashes); err != nil {
		t.Fatal(err)
	}
	for _, tc := range referenceCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.cfg
			cfg.Seed = 42
			g, err := New(context.Background(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			out := make([]BiomeID, 128*128)
			if err = g.NewSampler().GenerateIDs(context.Background(), referenceRange, out); err != nil {
				t.Fatal(err)
			}
			h := uint64(14695981039346656037)
			for _, id := range out {
				h = (h ^ uint64(id)) * 1099511628211
			}
			if got := fmt.Sprintf("%016x", h); got != hashes[tc.name] {
				t.Fatalf("Go hash %s != C %s", got, hashes[tc.name])
			}
		})
	}
}

// Warm sampler and caller-owned output, exactly matching benchmark.c.
func BenchmarkReference(b *testing.B) {
	for _, tc := range referenceCases {
		b.Run(tc.name, func(b *testing.B) {
			cfg := tc.cfg
			cfg.Seed = 42
			g, err := New(context.Background(), cfg)
			if err != nil {
				b.Fatal(err)
			}
			s := g.NewSampler()
			out := make([]BiomeID, 128*128)
			if err = s.GenerateIDs(context.Background(), referenceRange, out); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				if err = s.GenerateIDs(context.Background(), referenceRange, out); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkReferenceNew(b *testing.B) {
	for _, tc := range referenceCases {
		b.Run(tc.name, func(b *testing.B) {
			cfg := tc.cfg
			i := 0
			b.ReportAllocs()
			for b.Loop() {
				cfg.Seed = int64(42 + (i & 1023))
				i++
				if _, err := New(context.Background(), cfg); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
