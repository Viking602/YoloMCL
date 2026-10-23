package nativeui

import (
	"bytes"
	"encoding/base64"
	"image"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestDefaultSkinPreviewAndSave(t *testing.T) {
	a := &App{preview: newSkinPreview()}
	for _, model := range []string{"default", "slim", "default"} {
		a.showVanilla(model)
		slim := model == "slim"
		if a.preview.slim != slim || a.preview.skin != builtinSkin(slim) {
			t.Fatal("default model switch did not immediately replace its texture")
		}
		if !bytes.Equal(a.currentSkinBytes(), launcher.VanillaSkinPNG(model)) {
			t.Fatal("saving a default skin changed the original texture")
		}
		a.preview.set(nil, nil, slim)
		if a.preview.skin != builtinSkin(slim) {
			t.Fatal("empty account appearance lost the matching default skin")
		}
	}
	custom := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	a.preview.set(custom, nil, true)
	if a.preview.skin != custom {
		t.Fatal("a custom texture was replaced by the default")
	}
	a.appearance.Skin = base64.StdEncoding.EncodeToString([]byte("account skin"))
	if !bytes.Equal(a.currentSkinBytes(), []byte("account skin")) {
		t.Fatal("saving must prefer the account texture")
	}
	a.wd().pendingSkin = []byte("chosen skin")
	if !bytes.Equal(a.currentSkinBytes(), []byte("chosen skin")) {
		t.Fatal("saving must prefer the chosen texture")
	}
}

func TestVanillaWardrobePreview(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	a := &App{preview: newSkinPreview()}
	a.settings.Language = LangZH
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		ui.Row(c).Fill().Background(chromeBg).Padding(20).Children(func() {
			a.wardrobeStage(c, 550)
		})
	}, 480, 590)
	snapshotUI(t, tt, "wardrobe-steve")
	steve := append([]byte(nil), tt.Image().Pix...)
	a.showVanilla("slim")
	tt.Frame()
	if bytes.Equal(steve, tt.Image().Pix) {
		t.Fatal("Steve and Alex rendered identically")
	}
	snapshotUI(t, tt, "wardrobe-alex")
	if tt.HasText("正在读取外观") {
		t.Fatal("the bundled default skin is waiting for I/O")
	}
}
