package launcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Exercise navigation through three packs, including both summary validation
// and storage totals. Files only need directory metadata: opening JARs or
// decoding screenshots would not be part of the overview scan.
func BenchmarkInstanceOverviewSwitch(b *testing.B) {
	s, err := New(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	root := Root{ID: "bench", Path: b.TempDir()}
	s.cfg.Roots = []Root{root}
	versions := []string{"one", "two", "three"}
	for _, version := range versions {
		dir := filepath.Join(root.Path, "versions", version)
		for _, group := range []struct {
			dir, ext string
			count    int
		}{{"mods", ".jar", 600}, {"screenshots", ".png", 200}, {"logs", ".log", 80}, {"config", ".toml", 300}, {"saves/world/region", ".mca", 100}} {
			path := filepath.Join(dir, filepath.FromSlash(group.dir))
			if err := os.MkdirAll(path, 0755); err != nil {
				b.Fatal(err)
			}
			for i := 0; i < group.count; i++ {
				if err := os.WriteFile(filepath.Join(path, fmt.Sprintf("file-%04d%s", i, group.ext)), []byte("metadata fixture"), 0644); err != nil {
					b.Fatal(err)
				}
			}
		}
	}
	ctx := context.Background()
	for _, cold := range []bool{true, false} {
		name := "warm"
		if cold {
			name = "cold"
		}
		b.Run(name, func(b *testing.B) {
			load := func(version string) {
				st, err := s.InstanceOverviewContext(ctx, root.ID, version)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := s.CachedInstanceStorage(ctx, st); err != nil {
					b.Fatal(err)
				}
			}
			for _, version := range versions {
				load(version)
			}
			b.ReportAllocs()
			i := 0
			for b.Loop() {
				version := versions[i%len(versions)]
				if cold {
					s.invalidateStatus(filepath.Join(root.Path, "versions", version))
				}
				load(version)
				i++
			}
		})
	}
}
