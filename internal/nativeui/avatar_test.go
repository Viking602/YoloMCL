package nativeui

import (
	"bytes"
	"context"
	"encoding/json"
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

func avatarFixture(t *testing.T) (*App, launcher.Account, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := launcher.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	account, err := s.AddOffline("AvatarTest")
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.cancel(); a.bgWG.Wait() })
	path := filepath.Join(dir, "cosmetics", account.ID, "skin.png")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	return a, account, path
}

func avatarPNG(t *testing.T, shade color.NRGBA, scale int) []byte {
	t.Helper()
	skin := image.NewNRGBA(image.Rect(0, 0, 64*scale, 64*scale))
	for y := 8 * scale; y < 16*scale; y++ {
		for x := 8 * scale; x < 16*scale; x++ {
			skin.SetNRGBA(x, y, shade)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, skin); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestAccountAvatarRecoversAfterFailure(t *testing.T) {
	a, account, path := avatarFixture(t)
	if err := os.WriteFile(path, []byte("broken image"), 0600); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(a.View, 900, 600)
	waitUI(t, tt, func() bool { load := a.avatars.loads[account.ID]; return load != nil && !load.loading })
	failed := a.avatars.loads[account.ID]
	if failed.failures != 1 || !time.Now().Before(failed.retryAt) {
		t.Fatal("failure was not scheduled for retry")
	}
	if err := os.WriteFile(path, avatarPNG(t, color.NRGBA{R: 241, G: 33, B: 77, A: 255}, 1), 0600); err != nil {
		t.Fatal(err)
	}
	for range 10 {
		tt.Frame()
	}
	if a.avatars.loads[account.ID] != failed || a.avatars.heads[account.ID] != nil {
		t.Fatal("frames bypassed retry backoff")
	}
	failed.retryAt = time.Time{}
	waitUI(t, tt, func() bool { return a.avatars.heads[account.ID] != nil })
	if !a.avatars.loads[account.ID].complete {
		t.Fatal("retry did not complete")
	}
	box, ok := tt.Find("头像 " + account.Name)
	if !ok {
		t.Fatal("head never replaced generic account icon")
	}
	r, g, b, _ := tt.Image().At(int(box.X+box.W/2), int(box.Y+box.H/2)).RGBA()
	if r>>8 != 241 || g>>8 != 33 || b>>8 != 77 {
		t.Fatalf("wrong skin displayed: %d %d %d", r>>8, g>>8, b>>8)
	}
}

func TestAccountAvatarInvalidationDiscardsQueuedResult(t *testing.T) {
	a, account, path := avatarFixture(t)
	if err := os.WriteFile(path, avatarPNG(t, color.NRGBA{B: 255, A: 255}, 1), 0600); err != nil {
		t.Fatal(err)
	}
	a.accountHead(account)
	a.bgWG.Wait() // The old download is complete, but its UI event is queued.
	old := a.avatars.loads[account.ID]
	a.invalidateHead(account.ID)
	if err := os.WriteFile(path, avatarPNG(t, color.NRGBA{R: 255, A: 255}, 1), 0600); err != nil {
		t.Fatal(err)
	}
	a.accountHead(account)
	a.bgWG.Wait()
	a.drainEvents()
	if a.avatars.loads[account.ID] == old || a.avatars.heads[account.ID] == nil {
		t.Fatal("new skin was not retained")
	}
	tt := ui.NewTester(a.View, 900, 600)
	box, ok := tt.Find("头像 " + account.Name)
	if !ok {
		t.Fatal("avatar missing")
	}
	r, _, b, _ := tt.Image().At(int(box.X+box.W/2), int(box.Y+box.H/2)).RGBA()
	if r>>8 != 255 || b != 0 {
		t.Fatal("stale completion replaced new skin", r, b)
	}
}

func TestAccountAvatarPreservesHeadOnRefreshFailure(t *testing.T) {
	a, account, _ := avatarFixture(t)
	// This public provider returns a deterministic URL validation error,
	// so the test never needs external network or credentials.
	account.Kind, account.Server = "yggdrasil", "invalid-server"
	cfg := map[string]any{"schema": 1, "accounts": []launcher.Account{account}, "activeAccount": account.ID, "roots": []any{}}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	dir := a.snapshot.DataDir
	if err := os.WriteFile(filepath.Join(dir, "config.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := launcher.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	a.cancel()
	a.bgWG.Wait()
	a, err = New(s)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.cancel(); a.bgWG.Wait() })
	// A displayed valid skin must survive a failed refresh and remain usable.
	a.rememberHead(account.ID, ui.NewBitmap(defaultHeadFor(account)))
	_, cancel := context.WithCancel(a.ctx)
	defer cancel()
	a.avatars.loads = map[string]*avatarLoad{account.ID: {cancel: cancel}}
	tt := ui.NewTester(a.View, 900, 600)
	waitUI(t, tt, func() bool { return !a.avatars.loads[account.ID].loading })
	if a.avatars.heads[account.ID] == nil || a.avatars.loads[account.ID].complete {
		t.Fatal("failed refresh discarded the existing head")
	}
}

func TestSkinHeadHDAndAvatarRetryLimits(t *testing.T) {
	b := avatarPNG(t, color.NRGBA{G: 255, A: 255}, 2)
	skin, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	head := skinHead(skin)
	if head == nil || head.Bounds().Dx() != 16 {
		t.Fatal("HD skin head was not scaled")
	}
	if _, g, _, _ := head.At(0, 0).RGBA(); g != 65535 {
		t.Fatal("HD head sampled the wrong texture region")
	}
	for count, want := range map[int]time.Duration{1: 5 * time.Second, 2: 10 * time.Second, 3: 20 * time.Second, 8: 2 * time.Minute, 100: 2 * time.Minute} {
		if avatarRetryDelay(count) != want {
			t.Fatal("unbounded avatar retry", count)
		}
	}
}
