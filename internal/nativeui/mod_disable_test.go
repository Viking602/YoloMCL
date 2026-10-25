package nativeui

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func disableModIconFixture() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := range 16 {
		for x := range 16 {
			c := color.RGBA{220, 65, 95, 255}
			if x >= 8 {
				c = color.RGBA{35, 180, 120, 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func assertDisableModIcon(t *testing.T, tt *ui.Tester) {
	t.Helper()
	r, ok := tt.Find("禁用模组图标")
	if !ok || r.W != r.H || r.W <= 0 {
		t.Fatalf("missing or distorted mod artwork: %+v", r)
	}
	fixture := disableModIconFixture()
	for _, x := range []int{4, 12} {
		got := tt.Image().RGBAAt(int(r.X+r.W*float32(x)/16), int(r.Y+r.H/2))
		if want := fixture.RGBAAt(x, 8); got != want {
			t.Fatalf("dialog replaced or tinted mod artwork: got %v, want %v", got, want)
		}
	}
}

func TestModDisableArtworkUsesOriginalInstanceAndSharedCache(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"local", "remote", "late-local", "late-remote"} {
		t.Run(source, func(t *testing.T) {
			const localKey, remoteURL = "addon:sha1:sha", "https://cdn.example.test/mod.png"
			m := launcher.Mod{Name: "Example", File: "example.jar", SHA1: "sha", Enabled: true}
			a := &App{
				snapshot: launcher.Snapshot{ActiveRoot: "r", Instances: []launcher.Instance{{Root: launcher.Root{ID: "r", SelectedVersion: "pack"}}}},
				addonTab: "mods", mods: []launcher.Mod{m}, modIconURLs: map[string]string{},
				images: map[string]ui.ImageSource{}, imagePending: map[string]bool{}, imageRetry: map[string]time.Time{},
			}
			a.settings.Language = LangZH
			icon := ui.NewBitmap(disableModIconFixture())
			switch source {
			case "local":
				a.images[localKey] = icon
				// A cached platform image must not override embedded artwork.
				a.images[remoteURL] = ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 16, 16)))
				a.modIconURLs[m.SHA1] = remoteURL
			case "remote", "late-remote":
				a.imageRetry[localKey] = time.Now().Add(time.Hour)
				a.images[remoteURL] = icon
				if source == "remote" {
					a.modIconURLs[m.SHA1] = remoteURL
				}
			case "late-local":
				a.imagePending[localKey] = true
			}
			a.toggleModFn(m)()
			a.root().SelectedVersion = "other"
			a.images["addon:r:other:mod:example.jar"] = ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 16, 16)))
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme)
				ui.Column(c).Fill().Background(panel).Padding(24).Children(func() { a.disableModDialog(c) })
			}, 560, 210)
			if source == "late-local" || source == "late-remote" {
				if _, ok := tt.Find("禁用模组图标"); ok {
					t.Fatal("pending artwork showed the other instance's image")
				}
				a.images[localKey], a.imagePending[localKey] = icon, false
				if source == "late-remote" {
					delete(a.images, localKey)
					a.modIconURLs[m.SHA1] = remoteURL
				}
				tt.Frame()
			}
			assertDisableModIcon(t, tt)
		})
	}
}
