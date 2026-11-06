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

// MCL_PREVIEW_PNG="instDir|world|outDir|id@cx,cz;id@cx,cz" builds each saved
// start the way loadStructurePreview does (start era store, era layout
// terrain, terrain slab) and writes <outDir>/<name>.png. Headless: no
// window; reads the save only.
func TestStructurePreviewPNG(t *testing.T) {
	spec := os.Getenv("MCL_PREVIEW_PNG")
	if spec == "" {
		t.Skip("set MCL_PREVIEW_PNG")
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
	rootID, versionID := root.ID, root.SelectedVersion
	ctx := context.Background()
	_, level, err := s.SeedMapWorldgen(ctx, rootID, versionID, world)
	if err != nil {
		t.Fatal(err)
	}
	seed := level.RandomSeed
	m, err := s.DetectEras(ctx, rootID, versionID, world)
	if err != nil {
		t.Fatal(err)
	}
	const dim = "minecraft:overworld"
	for _, item := range strings.Split(parts[3], ";") {
		at := strings.LastIndex(item, "@")
		id := item[:at]
		var cx, cz int
		fmt.Sscanf(item[at+1:], "%d,%d", &cx, &cz)
		hit := launcher.FeatureHit{ID: id, Kind: "structure_set", Dimension: dim, X: cx * 16, Z: cz * 16}
		chunkEra := m.EraAt(cx, cz).Era
		sampler, err := s.EraSampler(ctx, rootID, versionID, world, chunkEra)
		if err != nil {
			t.Fatal(err)
		}
		store, exact, startEra, err := s.StartStoreEra(ctx, rootID, versionID, world, id, cx, cz)
		if err != nil {
			t.Errorf("%s: %v", item, err)
			continue
		}
		layout := sampler
		if startEra != chunkEra {
			layout, _ = s.EraSampler(ctx, rootID, versionID, world, startEra)
		}
		target, err := resolveHitTarget(store, hit, seed, dim, sampler)
		if err != nil {
			store.Close()
			t.Errorf("%s: %v", item, err)
			continue
		}
		surface := structure3d.HeightFunc(func(x, z int) int {
			if top, ok := sampler.TerrainSurfaceAt(dim, x, z); ok {
				return top
			}
			return 64
		})
		layoutSurface := structure3d.HeightFunc(func(x, z int) int {
			if top, ok := layout.TerrainSurfaceAt(dim, x, z); ok {
				return top
			}
			return surface(x, z)
		})
		var ground structure3d.Ground
		if structureUsesGround(target) {
			ground = newPreviewGround(seed, dim, sampler, surface, 1, true, "", hit.X-64, hit.Z-64, 128, 128, nil).StructureGround()
		}
		info, _ := sampler.TerrainInfo(dim)
		{
			top, _ := sampler.TerrainSurfaceAt(dim, hit.X, hit.Z)
			t.Logf("%s sea=%d surface@hit=%d layout=%d", item, info.SeaLevel, top, layoutSurface(hit.X, hit.Z))
		}
		cam := structure3d.Camera{Yaw: 0.6, Pitch: 0.55, Zoom: structurePreviewZoom(id)}
		res, err := launcher.PreviewStructureSea(store, target, dim, seed, hit.X, hit.Z, layoutSurface, ground, info.SeaLevel, structurePreviewFullW, structurePreviewFullH, cam)
		if err != nil || res == nil || res.Mesh == nil {
			store.Close()
			t.Errorf("%s: preview %v", item, err)
			continue
		}
		addPreviewGround(res.Mesh, surface, seed, dim, sampler, true, "", store)
		var f structure3d.Frame
		view := cam
		view.Pitch = -0.6                      // looking down onto the slab
		mode := os.Getenv("MCL_PREVIEW_GHOST") // "", "default", "ghost", "solid"
		suffix := ""
		if mode != "" {
			// the preview's own camera once the dive has arrived
			m := res.Mesh
			surfY, _ := sampler.TerrainSurfaceAt(dim, hit.X, hit.Z)
			p := &seedMapStructurePreview{yaw: 0.6, pitch: 0.55, zoom: cam.Zoom, diveT: 1, dim: dim, surfaceY: surfY,
				structMinY: m.MinY, structMaxY: m.MaxY, terrain: m.HasTerrain, terrainTop: m.TerrainTop}
			p.zoom = min(p.zoom, slabFitZoom(previewSlabSize(m)))
			p.ghost = !p.onSurface()
			switch mode {
			case "ghost":
				p.ghost = true
			case "solid":
				p.ghost = false
			}
			view = p.camera()
			suffix = fmt.Sprintf("_%s-ghost%v", mode, p.ghost)
			t.Logf("MODE %s onSurface=%v ghost=%v cut=%v cutY=%.0f ghostAlpha=%.2f", mode, p.onSurface(), p.ghost, view.Cut, view.CutY, view.Ghost)
		}
		img := res.Mesh.Render(&f, 1280, 960, view)
		name := strings.NewReplacer(":", "_", "/", "_").Replace(id) + fmt.Sprintf("_%d_%d%s.png", cx, cz, suffix)
		fh, err := os.Create(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		_ = png.Encode(fh, img)
		_ = fh.Close()
		store.Close()
		t.Logf("PNG %s exact=%v startEra=%d chunkEra=%d blocks=%d seeded=%v unsupported=%q faces=%d", filepath.Join(out, name), exact, startEra, chunkEra, res.BlockCount, res.Seeded, res.Unsupported, res.Mesh.Faces())
	}
}
