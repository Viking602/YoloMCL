package nativeui

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestNativeResourceImagesUseDiskCacheAndClearFailedLoads(t *testing.T) {
	s, e := launcher.New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a, e := New(s)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { a.cancel(); a.bgWG.Wait() }()
	raw := "https://cdn.example.test/real-icon.png"
	sum := sha256.Sum256([]byte(raw))
	file := filepath.Join(a.snapshot.DataDir, "cache", "resource-icons", hex.EncodeToString(sum[:])+".img")
	_ = os.MkdirAll(filepath.Dir(file), 0700)
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetNRGBA(x, y, color.NRGBA{90, 190, 120, 255})
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	if e = os.WriteFile(file, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	tt := ui.NewTester(a.View, 1320, 860)
	a.remoteImage(raw)
	waitUI(t, tt, func() bool { return a.images[raw] != nil })
	if a.imagePending[raw] {
		t.Fatal("successful cached icon is still pending")
	}
	bad := "not-an-https-url"
	a.remoteImage(bad)
	waitUI(t, tt, func() bool { return !a.imagePending[bad] })
	if !a.imageRetry[bad].After(time.Now()) {
		t.Fatal("failed image load did not enter a retryable state")
	}
	if source, e := decodeResourceImage([]byte(`<svg`)); e == nil || source != nil {
		t.Fatal("invalid SVG became a non-nil native image")
	}
	if source, e := decodeResourceImage([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><circle cx="5" cy="5" r="4" fill="#fff"/></svg>`)); e != nil || source == nil {
		t.Fatal("SVG resource icons are not supported", e)
	}
}

// Enable explicitly for the real API -> CDN -> decoder -> native widget check.
func TestLiveModrinthResourceIcons(t *testing.T) {
	if os.Getenv("MCL_LIVE_ICON_QA") != "1" {
		t.Skip("explicit live network check")
	}
	s, e := launcher.New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	result, e := s.Search(ctx, launcher.SearchQuery{Source: "modrinth", Type: "mod", Query: "Sodium"})
	if e != nil {
		t.Fatal(e)
	}
	if len(result.Projects) < 3 {
		t.Fatal("expected live Modrinth results")
	}
	result.Projects = result.Projects[:3]
	a, e := New(s)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { a.cancel(); a.bgWG.Wait() }()
	a.page = "discover"
	a.resourceType = "模组"
	a.query = "Sodium"
	a.searchResult = result
	tt := ui.NewTester(a.View, 1320, 860)
	deadline := time.Now().Add(40 * time.Second)
	for time.Now().Before(deadline) {
		tt.Frame()
		all := true
		for _, p := range result.Projects {
			if a.images[p.Icon] == nil {
				all = false
				break
			}
		}
		if all {
			tt.Advance(650 * time.Millisecond) // include the completed card/icon entrances
			snapshotUI(t, tt, "resources")
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("live resource icons did not reach native widgets")
}
