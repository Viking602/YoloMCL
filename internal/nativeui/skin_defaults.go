package nativeui

import (
	"bytes"
	"image"
	"image/png"
	"sync"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

var builtinSkins struct {
	once   sync.Once
	images [2]image.Image
}

// builtinSkin decodes each embedded skin once. Returned images are shared
// and must be treated as read-only.
func builtinSkin(slim bool) image.Image {
	builtinSkins.once.Do(func() {
		for i, model := range []string{"default", "slim"} {
			img, err := png.Decode(bytes.NewReader(launcher.VanillaSkinPNG(model)))
			if err != nil {
				panic("invalid embedded Minecraft skin: " + err.Error())
			}
			builtinSkins.images[i] = img
		}
	})
	if slim {
		return builtinSkins.images[1]
	}
	return builtinSkins.images[0]
}

// offlineDefault is true when an offline account has no skin of its own.
func (a *App) offlineDefault() bool {
	ac := a.account()
	return ac != nil && ac.Kind == "offline" && a.appearance.Skin == "" && len(a.wd().pendingSkin) == 0
}

// Both official textures are bundled; changing the model never waits on I/O.
func (a *App) showVanilla(model string) {
	a.preview.slim = model == "slim"
	a.preview.skin = builtinSkin(a.preview.slim)
	a.preview.dirty = true
}
