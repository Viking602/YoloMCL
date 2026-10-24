package nativeui

import (
	"bytes"
	"image"
	"image/png"
	"testing"
	"time"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestCapeTextureChecksDimensionsBeforeDecoding(t *testing.T) {
	for _, width := range []int{64, 4097} {
		var data bytes.Buffer
		if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, width, 32))); err != nil {
			t.Fatal(err)
		}
		_, err := decodeCapeImage(data.Bytes())
		if (err != nil) != (width > 4096) {
			t.Fatalf("width %d: %v", width, err)
		}
	}
}

func TestCapeResponseCannotOverwriteAnotherAccountOrURL(t *testing.T) {
	cape := launcher.Cape{Name: "Shared name", URL: "https://example.org/cape.png"}
	for _, change := range []string{"account", "url", "wardrobe", "selection", "none"} {
		t.Run(change, func(t *testing.T) {
			a := &App{appearanceAccount: "first", selectedCape: cape.Name, preview: &skinPreview{}}
			a.appearance.Capes = []launcher.Cape{cape}
			w := a.wd()
			w.capeAsked[cape.URL] = true
			switch change {
			case "account":
				a.appearanceAccount = "second"
			case "url":
				a.appearance.Capes[0].URL = "https://example.org/changed.png"
			case "wardrobe":
				a.wardrobeSt = nil
			case "selection":
				a.selectedCape = "Another cape"
			}
			img := image.NewRGBA(image.Rect(0, 0, 64, 32))
			a.finishCapeImage(w, "first", cape, []byte("cached texture"), img, nil)
			if (a.preview.cape != nil) != (change == "none") {
				t.Fatal("stale response changed the preview")
			}
			if w.capeImgs[cape.URL] != img || w.capeAsked[cape.URL] {
				t.Fatal("completed request was not cached/released")
			}
		})
	}
}

func TestMalformedCapeCanRetry(t *testing.T) {
	a := &App{}
	w := a.wd()
	cape := launcher.Cape{URL: "https://example.org/cape.png"}
	w.capeAsked[cape.URL] = true
	_, err := decodeCapeImage([]byte("truncated PNG"))
	a.finishCapeImage(w, "", cape, nil, nil, err)
	if w.capeAsked[cape.URL] || !w.capeRetry[cape.URL].After(time.Now()) || w.capeImgs[cape.URL] != nil {
		t.Fatal("decode failure must release request and schedule a bounded retry")
	}
}
