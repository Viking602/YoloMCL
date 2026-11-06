package nativeui

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/launcher"
	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// MCL_DIM_PNG="instDir|world|outDir|dim!set;dim!set@x,z" builds each hit with
// the app's own buildStructurePreview (any dimension) and writes the arrived
// dive view with the default, ghost and solid terrain. Without @x,z the set's
// nearest placement to 0,0 is used. Headless, reads the save only.
func TestDimPreviewPNG(t *testing.T) {
	spec := os.Getenv("MCL_DIM_PNG")
	if spec == "" {
		t.Skip("set MCL_DIM_PNG")
	}
	parts := strings.Split(spec, "|")
	inst, world, out := parts[0], parts[1], parts[2]
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := launcher.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root, err := s.AddRoot(inst, "png")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	snap, level, err := s.SeedMapWorldgen(ctx, root.ID, root.SelectedVersion, world)
	if err != nil {
		t.Fatal(err)
	}
	seed := level.RandomSeed
	sampler, err := s.EraSampler(ctx, root.ID, root.SelectedVersion, world, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range strings.Split(parts[3], ";") {
		dim, rest, _ := strings.Cut(item, "!")
		set, at, hasAt := strings.Cut(rest, "@")
		var hit launcher.FeatureHit
		var sx, sz int
		if hasAt {
			fmt.Sscanf(at, "~%d,%d", &sx, &sz)
		}
		if hasAt && !strings.HasPrefix(at, "~") {
			var x, z int
			fmt.Sscanf(at, "%d,%d", &x, &z)
			hit = launcher.FeatureHit{ID: set, Kind: "structure_set", Dimension: dim, X: x, Z: z}
		} else {
			r, err := sampler.FindStructures(ctx, launcher.SeedMapSearch{Dimension: dim, Structure: set, X: sx, Z: sz, Y: 64, Radius: 4096, Limit: 1})
			if err != nil || len(r.Hits) == 0 {
				t.Errorf("%s: no placement (%v)", item, err)
				continue
			}
			hit = r.Hits[0]
		}
		b := buildStructurePreview(s, root.ID, root.SelectedVersion, world, seed, dim, sampler, true, snap.Minecraft, nil, hit)
		if b.err != nil || b.res == nil || b.res.Mesh == nil {
			t.Errorf("%s @%d,%d: %v", item, hit.X, hit.Z, b.err)
			continue
		}
		m := b.res.Mesh
		for _, mode := range []string{"default", "ghost", "solid"} {
			p := &seedMapStructurePreview{yaw: 0.6, pitch: 0.55, zoom: structurePreviewZoom(hit.ID), diveT: 1, dim: dim, surfaceY: b.surfY,
				structMinY: m.MinY, structMaxY: m.MaxY, terrain: m.HasTerrain, terrainTop: m.TerrainTop, noiseTerrain: b.noiseOK}
			if m.HasTerrain {
				p.zoom = min(p.zoom, slabFitZoom(previewSlabSize(m)))
			}
			p.ghost = !p.onSurface()
			switch mode {
			case "ghost":
				p.ghost = true
			case "solid":
				p.ghost = false
			}
			var f structure3d.Frame
			view := p.camera()
			img := m.Render(&f, 1280, 960, view)
			name := strings.NewReplacer(":", "_", "/", "_").Replace(hit.ID) + fmt.Sprintf("_%d_%d_%s.png", hit.X, hit.Z, mode)
			fh, err := os.Create(filepath.Join(out, name))
			if err != nil {
				t.Fatal(err)
			}
			_ = png.Encode(fh, img)
			_ = fh.Close()
			t.Logf("PNG %s dim=%s target=%s blocks=%d terrain=%v top=%d structY=%d..%d surfY=%d noise=%v onSurface=%v ghost=%v cut=%v cutY=%.0f faces=%d",
				filepath.Join(out, name), dim, b.res.Title, b.res.BlockCount, m.HasTerrain, m.TerrainTop, m.MinY, m.MaxY, b.surfY, b.noiseOK, p.onSurface(), p.ghost, view.Cut, view.CutY, m.Faces())
		}
	}
}
