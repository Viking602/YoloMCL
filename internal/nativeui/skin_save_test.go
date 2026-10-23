package nativeui

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestSkinSaveHelpers(t *testing.T) {
	if got := safePNGName(`Snow/ice:Lemon`, "_skin"); got != "Snow_ice_Lemon_skin.png" {
		t.Fatalf("name %q", got)
	}
	if got := safePNGName("my skin.png", ""); got != "my skin.png" {
		t.Fatalf("name %q", got)
	}
	var buf bytes.Buffer
	png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 64, 64)))
	raw := buf.Bytes()
	b := dataURLBytes("data:image/png;base64," + base64.StdEncoding.EncodeToString(raw))
	if !bytes.Equal(b, raw) || !validTexture(b, false) {
		t.Fatal("data url round trip")
	}
	buf.Reset()
	png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 32, 32)))
	if validTexture(buf.Bytes(), false) {
		t.Fatal("32x32 is not a skin")
	}
	p := filepath.Join(t.TempDir(), "x.png")
	if e := writeFileAtomic(p, raw); e != nil {
		t.Fatal(e)
	}
	got, _ := os.ReadFile(p)
	if !bytes.Equal(got, raw) {
		t.Fatal("bytes changed on save")
	}
}
