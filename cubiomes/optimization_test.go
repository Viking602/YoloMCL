package cubiomes

import (
	"context"
	"encoding/json"
	"testing"
)

// Moving tiles prevent a repeated-output cache from hiding island generation.
func BenchmarkEndMoving(b *testing.B) {
	for _, version := range []string{"1.17.1", "1.20.1"} {
		b.Run(version, func(b *testing.B) {
			g, err := New(context.Background(), Config{Version: version, Seed: 42, Dimension: End})
			if err != nil {
				b.Fatal(err)
			}
			s := g.NewSampler()
			out := make([]BiomeID, 128*128)
			r := referenceRange
			i := 0
			b.ReportAllocs()
			for b.Loop() {
				r.X = 8192 + (i&31)*512
				r.Z = -8192 + ((i>>5)&31)*512
				i++
				if err := s.GenerateIDs(context.Background(), r, out); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkBuiltinDocuments(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := BuiltinDocuments("1.20.1"); err != nil {
			b.Fatal(err)
		}
	}
}

// mcl loads effective documents once, then compiles a different seed on edits.
func BenchmarkCompileSeed(b *testing.B) {
	documents, err := BuiltinDocuments("1.20.1")
	if err != nil {
		b.Fatal(err)
	}
	docs := make(map[string]Document, len(documents))
	for _, doc := range documents {
		docs[doc.Kind+"/"+doc.ID] = doc
	}
	def := Definition{Version: "1.20.1", Settings: json.RawMessage(`"minecraft:overworld"`), BiomeSource: json.RawMessage(`{"type":"minecraft:multi_noise","preset":"minecraft:overworld"}`)}
	b.ReportAllocs()
	for b.Loop() {
		def.Seed++
		if _, err := Compile(context.Background(), docs, def); err != nil {
			b.Fatal(err)
		}
	}
}

// Latest uses the same range as the pinned C comparison. C has no verified
// 26.3 implementation, so these timings are reported separately.
func BenchmarkLatest(b *testing.B) {
	for _, profile := range []struct {
		name  string
		dim   Dimension
		large bool
	}{{"Overworld", Overworld, false}, {"Large", Overworld, true}, {"Nether", Nether, false}, {"End", End, false}} {
		b.Run(profile.name, func(b *testing.B) {
			g, err := New(context.Background(), Config{Version: "26.3", Seed: 42, Dimension: profile.dim, LargeBiomes: profile.large})
			if err != nil {
				b.Fatal(err)
			}
			sampler := g.NewSampler()
			out := make([]BiomeID, 128*128)
			b.ReportAllocs()
			for b.Loop() {
				if err := sampler.GenerateIDs(context.Background(), referenceRange, out); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
