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
