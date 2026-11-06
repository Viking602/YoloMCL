package structure3d

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// TestSeededPreviews renders the seed-exact igloo and mineshaft previews.
// Set STRUCTURE3D_PNG_DIR to keep the frames.
func TestSeededPreviews(t *testing.T) {
	s := liveStore(t)
	for _, tc := range []struct {
		id     string
		seed   int64
		x, z   int
		seeded bool
		cam    Camera
		name   string
	}{{"minecraft:igloo", 1, 0, 0, true, Camera{}, "igloo"}, {"minecraft:mineshaft", 1, 100, 200, false, Camera{}, "mineshaft"}, {"minecraft:mineshaft", 1, 100, 200, false, Camera{Yaw: 0.6, Pitch: 1.1, Zoom: 40}, "mineshaft_close"}, {"minecraft:shipwreck", 1, 300, -500, true, Camera{}, "shipwreck"}, {"minecraft:shipwreck_beached", 1, 300, -500, true, Camera{}, "shipwreck_beached"}, {"minecraft:ruined_portal", 1, 64, 64, true, Camera{}, "ruined_portal"}, {"minecraft:ruined_portal_ocean", 1, 64, 64, true, Camera{}, "ruined_portal_ocean"}, {"minecraft:swamp_hut", 1, 0, 0, false, Camera{}, "swamp_hut"}, {"minecraft:desert_pyramid", 1, 0, 0, false, Camera{}, "desert_pyramid"}, {"minecraft:jungle_pyramid", 1, 0, 0, false, Camera{}, "jungle_pyramid"}, {"minecraft:buried_treasure", 1, 0, 0, true, Camera{Yaw: 0.6, Pitch: 0.55, Zoom: 40}, "buried_treasure"}, {"minecraft:ocean_ruin_warm", 1, -576, -3024, true, Camera{}, "ocean_ruin_warm"}, {"minecraft:ocean_ruin_cold", 1, 0, 0, true, Camera{}, "ocean_ruin_cold"}, {"minecraft:stronghold", 1, 0, 0, false, Camera{Yaw: 0.6, Pitch: 0.7, Zoom: 3}, "stronghold"}, {"minecraft:monument", 1, 0, 0, true, Camera{}, "monument"}, {"minecraft:mansion", 1, 0, 0, true, Camera{}, "mansion"}, {"minecraft:end_city", 1, 0, 0, true, Camera{}, "end_city"}} {
		res, err := Preview(s, PreviewRequest{StructureID: tc.id, Seed: tc.seed, X: tc.x, Z: tc.z, Width: 640, Height: 480, Camera: tc.cam, Surface: func(x, z int) int { return 70 }})
		if err != nil {
			t.Fatal(err)
		}
		if res.Unsupported != "" || res.Seeded != tc.seeded || res.Seeded == res.LayoutSeeded || res.BlockCount == 0 {
			t.Fatalf("%s: %+v", tc.id, res)
		}
		t.Logf("%s: %d blocks seeded=%v layoutSeeded=%v", tc.id, res.BlockCount, res.Seeded, res.LayoutSeeded)
		if dir := os.Getenv("STRUCTURE3D_PNG_DIR"); dir != "" {
			f, err := os.Create(filepath.Join(dir, tc.name+".png"))
			if err == nil {
				png.Encode(f, res.Image)
				f.Close()
			}
		}
	}
}
