package nativeui

import (
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

func BenchmarkSkinPreview(b *testing.B) {
	p := newSkinPreview()
	p.render(440, 520)
	p.render(440, 520)
	b.ReportAllocs()
	for b.Loop() {
		p.yaw += .01
		p.render(440, 520)
	}
}

func TestLeavingWardrobeReleasesScratchAndKeepsPreview(t *testing.T) {
	p := newSkinPreview()
	p.yaw, p.zoom, p.zoomGoal = 1.4, 1.2, 1.2
	p.render(440, 520)
	displayed := p.render(440, 520)
	p.bitmap = ui.NewBitmap(displayed).Pixelated()
	p.dirty = false
	before, bitmap := sha256.Sum256(displayed.Pix), p.bitmap
	a := &App{page: "wardrobe", preview: p}
	a.goPage("tasks")
	if p.depth != nil || p.frames[0] != nil || p.frames[1] != nil {
		t.Fatal("leaving wardrobe retained drawing scratch buffers")
	}
	if p.bitmap != bitmap || p.dirty || p.yaw != 1.4 || p.zoom != 1.2 || p.zoomGoal != 1.2 || sha256.Sum256(displayed.Pix) != before {
		t.Fatal("releasing scratch changed the displayed preview or pose")
	}
	if got := sha256.Sum256(p.render(440, 520).Pix); got != before {
		t.Fatal("resuming rendering changed the pixels")
	}
	p.yaw += .4
	p.render(440, 520)
	if sha256.Sum256(displayed.Pix) != before {
		t.Fatal("resumed drawing overwrote the still-displayed frame")
	}
	if allocations := testing.AllocsPerRun(10, func() { p.yaw += .03; p.render(440, 520) }); allocations != 0 {
		t.Fatalf("resumed rotation allocates %g objects per frame", allocations)
	}
}

func TestSkinPreviewRasterOutput(t *testing.T) {
	// Captured before the raster optimization: shading, UV mapping, legacy
	// transparency and both arm widths must keep the same output.
	want := [2][3]string{
		{"2f8b4c08d6f3bb8dde83386fdd4e89cfcf2bfcab87c7d92f3ec4a4d357b71168", "b3308363095684440902b3c65d951933fd262756543e4b6bd1d7abe20dd4e218", "7a00223c9a1f0a018db04f5d85ce1deb73fe2427592d55437cad3a104daafee7"},
		{"31b6b920cfd6cc7167615e5cc77c7a471e9c7a84dd6586fb4d7d1e654f3b494c", "fc8052b426660e335b55df22194271417e3c76cfecb786400ad0b30938bd293c", "206a6f46bf759a5e668f668932d7ccd81e09d9332fb04202b6fa0b9951e1df6d"},
	}
	for model, slim := range []bool{false, true} {
		p := newSkinPreview()
		p.set(builtinSkin(slim), nil, slim)
		for angle, yaw := range []float64{-.35, 1.4, 3.14} {
			p.yaw = yaw
			img := p.render(160, 180)
			if got := fmt.Sprintf("%x", sha256.Sum256(img.Pix)); got != want[model][angle] {
				t.Fatalf("slim=%v yaw=%g: raster output changed: %s", slim, yaw, got)
			}
		}
	}
}

func TestSkinPreviewTextureOrigins(t *testing.T) {
	// Include transparent overlay pixels and a cape cropped from an image
	// with a non-zero origin. Rebasing identical pixels must not change output.
	p := newSkinPreview()
	skin := image.NewNRGBA(image.Rect(3, 5, 67, 69))
	for y := 5; y < 69; y++ {
		for x := 3; x < 67; x++ {
			skin.SetNRGBA(x, y, color.NRGBA{uint8(x * 3), uint8(y * 3), 70, uint8((x + y) % 256)})
		}
	}
	p.set(skin, skin.SubImage(image.Rect(3, 5, 67, 37)), false)
	p.yaw = 2.5
	rebased := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	draw.Draw(rebased, rebased.Bounds(), skin, skin.Bounds().Min, draw.Src)
	q := newSkinPreview()
	q.set(rebased, rebased.SubImage(image.Rect(0, 0, 64, 32)), false)
	q.yaw = p.yaw
	if sha256.Sum256(p.render(160, 180).Pix) != sha256.Sum256(q.render(160, 180).Pix) {
		t.Fatal("non-zero texture origins changed the rendered skin/cape")
	}
}

func TestSkinRasterTransparency(t *testing.T) {
	base := color.RGBA{50, 100, 150, 255}
	for _, test := range []struct {
		alpha uint8
		want  color.RGBA
	}{
		{0, base}, {15, base},
		{16, color.RGBA{56, 95, 145, 255}},
		{128, color.RGBA{105, 65, 114, 255}},
		{255, color.RGBA{160, 32, 80, 255}},
	} {
		texture := image.NewNRGBA(image.Rect(3, 5, 67, 69))
		draw.Draw(texture, texture.Bounds(), image.NewUniform(color.NRGBA{200, 40, 100, test.alpha}), image.Point{}, draw.Src)
		out := image.NewRGBA(image.Rect(0, 0, 4, 4))
		draw.Draw(out, out.Bounds(), image.NewUniform(base), image.Point{}, draw.Src)
		depth := make([]float64, 16)
		for i := range depth {
			depth[i] = math.Inf(-1)
		}
		rasterTriangle(out, depth, vertex{0, 0, 1, 0, 0}, vertex{4, 0, 1, 0, 0}, vertex{0, 4, 1, 0, 0}, skinFace{texture: texture, shade: 1})
		if got := out.RGBAAt(1, 1); got != test.want {
			t.Fatalf("alpha=%d: got %v, want %v", test.alpha, got, test.want)
		}
		if out.RGBAAt(3, 3) != base || math.IsInf(depth[5], -1) != (test.alpha < 16) {
			t.Fatalf("alpha=%d: incorrect coverage or depth", test.alpha)
		}
	}
}

func TestSkinPreviewReusesBuffersAndRefreshesTextures(t *testing.T) {
	p := newSkinPreview()
	p.render(160, 180)
	p.render(160, 180)
	if allocations := testing.AllocsPerRun(10, func() { p.yaw += .03; p.render(160, 180) }); allocations != 0 {
		t.Fatalf("steady rotation allocates %g objects per frame", allocations)
	}
	old := p.render(160, 180)
	before := sha256.Sum256(old.Pix)
	p.yaw += .4
	p.render(160, 180)
	if sha256.Sum256(old.Pix) != before {
		t.Fatal("render overwrote the frame still displayed by the UI")
	}
	// Rebuild cached geometry when texture, cape or model changes.
	skin := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	cape := image.NewNRGBA(image.Rect(0, 0, 64, 32))
	for offset := 0; offset < len(skin.Pix); offset += 4 {
		copy(skin.Pix[offset:], []byte{200, 30, 60, 255})
	}
	for offset := 0; offset < len(cape.Pix); offset += 4 {
		copy(cape.Pix[offset:], []byte{10, 220, 40, 255})
	}
	p.set(skin, cape, true)
	p.yaw = 3.14
	for _, withCape := range []bool{true, false} {
		if !withCape {
			p.cape = nil
		}
		got := sha256.Sum256(p.render(160, 180).Pix)
		fresh := newSkinPreview()
		fresh.set(p.skin, p.cape, p.slim)
		fresh.yaw, fresh.pitch = p.yaw, p.pitch
		if got != sha256.Sum256(fresh.render(160, 180).Pix) {
			t.Fatal("cached geometry kept a previous skin/cape/model")
		}
	}
}

// Each texel has a distinct color, including the edges and lining. A symmetric
// or single-color cape would hide swapped faces, mirroring and lost HD detail.
func capeUVFixture(resolution int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(3, 5, 3+64*resolution, 5+32*resolution))
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			img.SetNRGBA(x+3, y+5, color.NRGBA{uint8(x + 31), uint8(y*3 + 17), uint8((x*11 + y*7) % 211), 255})
		}
	}
	return img
}

func TestCapePreviewTextureFaces(t *testing.T) {
	for _, resolution := range []int{1, 2, 4} {
		cape := capeUVFixture(resolution)
		// Screen positions use a scale of 8 pixels per model unit. Compare
		// texel centers so floating-point rounding at triangle edges is irrelevant.
		for _, view := range []struct {
			name             string
			yaw, pitch       float64
			x, y, u, v, w, h int
			flipU, flipV     bool
			shade            float64
		}{
			{"outside", math.Pi, 0, 80, 98, 1, 1, 10, 16, false, false, .8},
			{"lining", 0, 0, 80, 98, 12, 1, 10, 16, false, false, 1},
			{"right-edge", math.Pi / 2, 0, 92, 98, 11, 1, 1, 16, false, false, .72},
			{"left-edge", -math.Pi / 2, 0, 140, 98, 0, 1, 1, 16, false, false, .86},
			{"top", 0, math.Pi / 2, 80, 126, 1, 0, 10, 1, true, true, 1.05},
			{"bottom", 0, -math.Pi / 2, 80, 174, 11, 0, 10, 1, true, false, .6},
		} {
			t.Run(fmt.Sprintf("%dx/%s", resolution, view.name), func(t *testing.T) {
				p := newSkinPreview()
				p.set(image.NewNRGBA(image.Rect(0, 0, 64, 64)), cape, false)
				p.yaw, p.pitch = view.yaw, view.pitch
				img := p.render(240, 316)
				step := 8 / resolution
				for y := range view.h * resolution {
					for x := range view.w * resolution {
						u, v := x, y
						if view.flipU {
							u = view.w*resolution - 1 - x
						}
						if view.flipV {
							v = view.h*resolution - 1 - y
						}
						src := cape.NRGBAAt(3+view.u*resolution+u, 5+view.v*resolution+v)
						shade := func(channel uint8) uint8 { return uint8(min(255, float64(channel)*view.shade)) }
						want := color.RGBA{shade(src.R), shade(src.G), shade(src.B), 255}
						got := img.RGBAAt(view.x+x*step+step/2, view.y+y*step+step/2)
						if got != want {
							t.Fatalf("texel (%d,%d): got %v, want %v", x, y, got, want)
						}
					}
				}
				p.render(240, 316)
				if allocations := testing.AllocsPerRun(10, func() { p.yaw += .01; p.render(240, 316) }); allocations != 0 {
					t.Fatalf("cape rotation allocates %g objects per frame", allocations)
				}
			})
		}
	}
}

func BenchmarkCapePreview(b *testing.B) {
	for _, resolution := range []int{1, 2} {
		b.Run(fmt.Sprintf("%dx", resolution), func(b *testing.B) {
			p := newSkinPreview()
			p.set(builtinSkin(false), capeUVFixture(resolution), false)
			p.render(440, 520)
			p.render(440, 520)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				p.yaw += .01
				p.render(440, 520)
			}
		})
	}
}
