package nativeui

import (
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
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
	// Include transparent overlay pixels, a cape and a non-zero texture origin.
	p := newSkinPreview()
	skin := image.NewNRGBA(image.Rect(3, 5, 67, 69))
	for y := 5; y < 69; y++ {
		for x := 3; x < 67; x++ {
			skin.SetNRGBA(x, y, color.NRGBA{uint8(x * 3), uint8(y * 3), 70, uint8((x + y) % 256)})
		}
	}
	p.set(skin, skin.SubImage(image.Rect(3, 5, 67, 37)), false)
	p.yaw = 2.5
	if got := fmt.Sprintf("%x", sha256.Sum256(p.render(160, 180).Pix)); got != "6a15a4392b5518910298a12241419dec5d8c82e26bbd788dae25514196d7de5a" {
		t.Fatalf("transparent skin/cape output changed: %s", got)
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
