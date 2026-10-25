package nativeui

import (
	"bytes"
	"image"
	"image/png"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

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
	t.Setenv("APPDATA", t.TempDir())
	a := &App{}
	w := a.wd()
	cape := launcher.Cape{URL: "https://example.org/cape.png"}
	w.capeAsked[cape.URL] = true
	_, err := decodeCapeImage([]byte("truncated PNG"))
	a.finishCapeImage(w, "", cape, nil, nil, err)
	if w.capeAsked[cape.URL] || !w.capeFailed[cape.URL] || w.capeImgs[cape.URL] != nil {
		t.Fatal("decode failure must release request and allow an explicit retry")
	}
}

func TestFailedCapeStopsAnimatingAndCanRetryWhileSelected(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	s, err := launcher.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.cancel(); a.bgWG.Wait() })
	a.OverrideLanguage(LangZH)
	// An invalid URL fails deterministically without making any network request.
	cape := launcher.Cape{ID: "test", Name: "Vanilla", URL: "http://example.test/cape.png"}
	a.appearance.Capes = []launcher.Cape{cape}
	a.selectedCape = cape.Name
	w := a.wd()
	frames := 0
	tt := ui.NewTester(func(c *ui.Context) {
		frames++
		c.SetTheme(theme)
		var thumb *ui.Bitmap
		if img := a.capeImage(cape); img != nil {
			thumb = ui.NewBitmap(capeFront(img)).Pixelated()
		}
		ui.Box(c).Fill().Background(panel).Center().Children(func() {
			a.capeTile(c, cape, thumb, 72, 100)
		})
	}, 180, 160)
	finishRequest := func() {
		t.Helper()
		a.bgWG.Wait()
		select {
		case fn := <-a.events:
			fn()
		case <-time.After(time.Second):
			t.Fatal("cape request did not complete")
		}
		tt.Frame()
	}
	if _, ok := tt.Find("披风正在加载 Vanilla"); !ok {
		t.Fatal("pending request has no loading indicator")
	}
	finishRequest()
	if !tt.HasText("重试") {
		t.Fatal("failed request left a spinner instead of a retry action")
	}
	tt.Advance(time.Second)
	before := frames
	tt.Advance(30 * time.Second)
	if frames != before || w.capeAsked[cape.URL] {
		t.Fatal("failed cape kept redrawing or retrying in the background")
	}
	snapshotUI(t, tt, "cape-load-failed")
	clickUI(t, tt, cape.Name)
	if w.capeFailed[cape.URL] || !w.capeAsked[cape.URL] {
		t.Fatal("selected tile could not restart its failed request")
	}
	finishRequest()
	if !w.capeFailed[cape.URL] || !tt.HasText("重试") {
		t.Fatal("failed retry did not restore its retry action")
	}
	img := image.NewRGBA(image.Rect(0, 0, 64, 32))
	a.finishCapeImage(w, a.appearanceAccount, cape, []byte("texture"), img, nil)
	tt.Frame()
	if w.capeFailed[cape.URL] || tt.HasText("重试") || a.preview.cape != img {
		t.Fatal("successful completion did not clear the error and update the preview")
	}
}
