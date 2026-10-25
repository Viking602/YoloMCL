package launcher

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/png"
	"strings"
	"testing"
	"time"
)

func TestAppearanceLoadsActiveAndSelectableLegacyCapes(t *testing.T) {
	s := testService(t)
	account := Account{ID: "cape-fixture", Kind: "microsoft"}
	if err := s.saveAccount(account, tokenSecret{Access: "fixture-token", Expires: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	var texture bytes.Buffer
	if err := png.Encode(&texture, image.NewRGBA(image.Rect(0, 0, 64, 32))); err != nil {
		t.Fatal(err)
	}
	fakeHTTP(t, s, map[string][]byte{
		"https://api.minecraftservices.com/minecraft/profile": []byte(`{
			"skins": [{"url":"http://textures.minecraft.net/texture/skin","variant":"SLIM","state":"ACTIVE"}],
			"capes": [
				{"id":"vanilla","alias":"Vanilla","url":"http://textures.minecraft.net/texture/vanilla","state":"INACTIVE"},
				{"id":"pan","alias":"Pan","url":"https://textures.minecraft.net/texture/pan","state":"INACTIVE"},
				{"id":"migrator","alias":"Migrator","url":"http://textures.minecraft.net/texture/migrator","state":"ACTIVE"},
				{"id":"common","alias":"Common","url":"http://textures.minecraft.net/texture/common","state":"INACTIVE"}
			]
		}`),
		"https://textures.minecraft.net/texture/skin":     texture.Bytes(),
		"https://textures.minecraft.net/texture/vanilla":  texture.Bytes(),
		"https://textures.minecraft.net/texture/pan":      texture.Bytes(),
		"https://textures.minecraft.net/texture/migrator": texture.Bytes(),
		"https://textures.minecraft.net/texture/common":   texture.Bytes(),
	})
	appearance, err := s.Appearance(context.Background(), account.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantData := "data:image/png;base64," + base64.StdEncoding.EncodeToString(texture.Bytes())
	if appearance.Skin != wantData || appearance.Cape != wantData || appearance.Model != "slim" || len(appearance.Capes) != 4 {
		t.Fatal("active textures or selectable capes were lost")
	}
	for _, cape := range appearance.Capes {
		if !strings.HasPrefix(cape.URL, "https://") || cape.Active != (cape.Name == "Migrator") {
			t.Fatalf("cape URL or active state was not preserved: %+v", cape)
		}
		data, err := s.ResourceImage(context.Background(), cape.URL)
		if err != nil || !bytes.Equal(data, texture.Bytes()) {
			t.Fatalf("%s preview could not load: %v", cape.Name, err)
		}
	}
}
