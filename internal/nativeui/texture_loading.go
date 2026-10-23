package nativeui

import (
	"encoding/base64"
	"encoding/json"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"
)

func imageFromData(data string) image.Image {
	b, e := base64.StdEncoding.DecodeString(strings.TrimPrefix(data, "data:image/png;base64,"))
	if e != nil {
		return nil
	}
	img, _, _ := image.Decode(strings.NewReader(string(b)))
	return img
}

func (a *App) upload(kind string) {
	ac := a.account()
	if ac == nil {
		return
	}
	id := ac.ID
	a.chooseFiles("png", func(paths []string) {
		b, e := os.ReadFile(paths[0])
		if e != nil {
			a.message = a.friendlyErr(e)
			return
		}
		model := normalizeSkinModel(a.model)
		a.execute("uploadTexture", map[string]string{"accountId": id, "kind": kind, "model": model, "data": base64.StdEncoding.EncodeToString(b)}, func(json.RawMessage) {
			a.loadAppearance()
			a.notice(a.tr().WardrobeTextureSaved)
		})
	})
}
