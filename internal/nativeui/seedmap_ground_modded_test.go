package nativeui

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// TestModdedGroundShot renders a ≈ strip of mod biomes (a synthetic layout:
// the AFoP overworld sampler cannot place BOP biomes) through the real
// previewGround Top/Section with the instance's textures. Set
// MODDED_GROUND_JAR (game jar) and MODDED_GROUND_INSTANCE (instance root);
// the PNG goes to STRUCTURE3D_PNG_DIR when set.
func TestModdedGroundShot(t *testing.T) {
	jar, inst := os.Getenv("MODDED_GROUND_JAR"), os.Getenv("MODDED_GROUND_INSTANCE")
	if jar == "" || inst == "" {
		t.Skip("MODDED_GROUND_JAR and MODDED_GROUND_INSTANCE required")
	}
	store := &structure3d.Store{}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.OpenJar("game", jar); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d mod jars", structure3d.OpenModsDir(store, filepath.Join(inst, "mods")))

	var ids []string
	for id := range moddedGrounds {
		if strings.HasPrefix(id, "biomesoplenty:") {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	ids = append([]string{"minecraft:plains"}, ids...)
	const band, depth = 10, 24
	g := &previewGround{dim: "minecraft:overworld", step: 1, seed: 1,
		heights: map[[2]int]int{}, biomes: map[[2]int]string{}, ores: map[[3]int]string{},
		surface: func(x, z int) int {
			return 72 + int(3*math.Sin(float64(x)/5)+2*math.Cos(float64(z)/4))
		}}
	g.tex = structure3d.LoadTerrainTextures(store)
	g.registerModdedGrounds()
	for _, id := range ids {
		m := moddedGrounds[id]
		t.Logf("%-34s top %-36s %v  under %-36s %v", id, m.top, g.modded[m.top], m.under, g.modded[m.under])
		if m.top != "" && (!g.modded[m.top] || !g.modded[m.under]) {
			t.Errorf("%s: surface block not textured", id)
		}
	}
	ae := moddedGrounds["aether:*"]
	t.Logf("aether:* top %v under %v stone %v", g.modded[ae.top], g.modded[ae.under], g.modded[ae.stone])
	w := band * len(ids)
	for x := 0; x < w; x++ {
		for z := 0; z < depth; z++ {
			g.biomes[[2]int{x, z}] = ids[x/band]
		}
	}
	m := &structure3d.Mesh{Center: [3]float64{float64(w) / 2, 72, depth / 2}, MinY: 60, MaxY: 80}
	m.AddTerrain(structure3d.Terrain{X0: 0, Z0: 0, W: w, D: depth, Step: 1, Bottom: 64,
		Height: g.surface, Color: g.Top, Section: g.Section, Textures: g.textures()})
	var f structure3d.Frame
	img := m.Render(&f, 1600, 700, structure3d.Camera{Yaw: 0.35, Pitch: -0.6, Zoom: 9})
	if dir := os.Getenv("STRUCTURE3D_PNG_DIR"); dir != "" {
		out, err := os.Create(filepath.Join(dir, "bop-terrain.png"))
		if err != nil {
			t.Fatal(err)
		}
		defer out.Close()
		if err := png.Encode(out, img); err != nil {
			t.Fatal(err)
		}
		t.Logf("bands left→right: %v", ids)
	}
}
