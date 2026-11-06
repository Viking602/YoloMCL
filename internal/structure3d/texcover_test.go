package structure3d

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestTexCoverage measures textured-model coverage on real jars.
// STRUCTURE_TEX_JAR = client jar (vanilla, verified first);
// STRUCTURE_TEX_MODS = optional mods dir layered above it;
// STRUCTURE_TEX_PACKS = optional resourcepacks dir layered on top.
func TestTexCoverage(t *testing.T) {
	jar := os.Getenv("STRUCTURE_TEX_JAR")
	if jar == "" {
		t.Skip("STRUCTURE_TEX_JAR not set")
	}
	s := &Store{}
	defer s.Close()
	if err := s.OpenJar("client", jar); err != nil {
		t.Fatal(err)
	}
	van := MeasureTexCoverage(s, "minecraft")
	t.Logf("vanilla: %d/%d textured (%.2f%%)", van.Textured, van.Blocks, pct(van.Textured, van.Blocks))
	if len(van.Missing) > 0 {
		t.Logf("vanilla untextured: %v", van.Missing)
	}
	if van.Blocks == 0 || van.Textured*100 < van.Blocks*90 {
		t.Errorf("vanilla coverage too low")
	}
	tt := LoadTerrainTextures(s)
	if !tt.RegisterBlock([3]uint8{1, 2, 3}, "minecraft:podzol") {
		t.Errorf("podzol top texture not resolved")
	}
	if d := os.Getenv("STRUCTURE_TEX_MODS"); d != "" {
		n := OpenModsDir(s, d)
		if pd := os.Getenv("STRUCTURE_TEX_PACKS"); pd != "" {
			es, _ := os.ReadDir(pd)
			for _, e := range es {
				fp := pd + string(os.PathSeparator) + e.Name()
				if e.IsDir() {
					_ = s.OpenDir("pack:"+e.Name(), fp)
				} else {
					_ = s.OpenJar("pack:"+e.Name(), fp)
				}
			}
		}
		all := MeasureTexCoverage(s, "")
		t.Logf("with %d mod jars: %d/%d textured (%.2f%%)", n, all.Textured, all.Blocks, pct(all.Textured, all.Blocks))
		var ns []string
		for k := range all.ByNS {
			ns = append(ns, k)
		}
		sort.Strings(ns)
		for _, k := range ns {
			v := all.ByNS[k]
			t.Logf("  %-24s %5d/%-5d %.1f%%", k, v[1], v[0], pct(v[1], v[0]))
		}
		if len(all.Missing) > 60 {
			all.Missing = all.Missing[:60]
		}
		t.Logf("untextured sample: %v", all.Missing)
		if dir := os.Getenv("STRUCTURE3D_PNG_DIR"); dir != "" {
			writeTexSheet(t, s, dir, os.Getenv("STRUCTURE_TEX_SHEET_NS"))
		}
	}
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return 100 * float64(a) / float64(b)
}

// writeTexSheet draws the "up" face of the first 128 blocks of one
// namespace (default: the first non-minecraft one) at 4x, as QA evidence
// that modded blocks resolve through the layered store.
func writeTexSheet(t *testing.T, s *Store, dir, ns string) {
	c := newModelCache()
	var ids []string
	for p := range s.List("assets/") {
		rest, ok := strings.CutPrefix(p, "assets/")
		if !ok || !strings.Contains(rest, "/blockstates/") {
			continue
		}
		pns, name, _ := strings.Cut(rest, "/blockstates/")
		if (ns == "" && pns != "minecraft") || pns == ns {
			ids = append(ids, pns+":"+strings.TrimSuffix(name, ".json"))
		}
	}
	sort.Strings(ids)
	const cell, cols = 64, 16
	sheet := image.NewRGBA(image.Rect(0, 0, cell*cols, cell*8))
	n := 0
	for _, id := range ids {
		if n >= cols*8 {
			break
		}
		m := c.resolve(s, BlockState{Name: id})
		if m == nil {
			continue
		}
		var img image.Image
		for _, e := range m.Elements {
			for _, f := range e.Faces {
				if f.Dir == "up" || img == nil {
					img = f.Texture
				}
			}
		}
		if img == nil {
			continue
		}
		b := img.Bounds()
		x0, y0 := (n%cols)*cell, (n/cols)*cell
		for y := 0; y < cell; y++ {
			for x := 0; x < cell; x++ {
				sheet.Set(x0+x, y0+y, img.At(b.Min.X+x*b.Dx()/cell, b.Min.Y+(y*b.Dx()/cell)%b.Dy()))
			}
		}
		n++
	}
	name := filepath.Join(dir, "texsheet-"+strings.ReplaceAll(ns, ":", "_")+".png")
	f, err := os.Create(name)
	if err != nil {
		t.Log(err)
		return
	}
	defer f.Close()
	_ = png.Encode(f, sheet)
	t.Logf("wrote %s (%d blocks)", name, n)
}
