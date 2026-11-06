package nativeui

import (
	"bytes"
	"context"
	"image"
	"image/draw"
	"image/png"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// Avatar state belongs to the UI thread. Request identity prevents an old
// download from overwriting a newly selected/uploaded skin.
type avatarCache struct {
	heads     map[string]*ui.Bitmap
	loads     map[string]*avatarLoad
	slots     chan struct{}
	headOrder []string
	icons     map[string]*ui.Bitmap
	iconTried map[string]bool
	iconOrder []string
}

type avatarLoad struct {
	cancel   context.CancelFunc
	loading  bool
	complete bool
	failures int
	retryAt  time.Time
}

// skinHead composites the face (8,8) and hat overlay (40,8) of a 64×32/64×64
// skin into an 8×8 image, preserving the proportional region for HD skins.
func skinHead(skin image.Image) image.Image {
	if skin == nil || skin.Bounds().Dx() < 64 || skin.Bounds().Dx() > 1024 || skin.Bounds().Dx()%64 != 0 || skin.Bounds().Dy() != skin.Bounds().Dx() && skin.Bounds().Dy() != skin.Bounds().Dx()/2 {
		return nil
	}
	b := skin.Bounds().Min
	scale := skin.Bounds().Dx() / 64
	out := image.NewNRGBA(image.Rect(0, 0, 8*scale, 8*scale))
	draw.Draw(out, out.Bounds(), skin, b.Add(image.Pt(8*scale, 8*scale)), draw.Src)
	draw.Draw(out, out.Bounds(), skin, b.Add(image.Pt(40*scale, 8*scale)), draw.Over)
	return out
}

// javaUUIDHash mirrors java.util.UUID.hashCode, which vanilla uses to pick
// the default Steve (even) or Alex (odd) model.
func javaUUIDHash(uuid string) (int32, bool) {
	h := strings.ReplaceAll(uuid, "-", "")
	if len(h) != 32 {
		return 0, false
	}
	msb, e1 := strconv.ParseUint(h[:16], 16, 64)
	lsb, e2 := strconv.ParseUint(h[16:], 16, 64)
	if e1 != nil || e2 != nil {
		return 0, false
	}
	x := msb ^ lsb
	return int32(x>>32) ^ int32(x), true
}

func defaultIsAlex(uuid string) bool {
	h, ok := javaUUIDHash(uuid)
	return ok && h&1 == 1
}

func defaultHeadFor(ac launcher.Account) image.Image {
	return skinHead(builtinSkin(defaultIsAlex(ac.UUID)))
}

// accountHead shows a cached head while refreshing its public skin. Transient
// failures have bounded retries instead of hiding the avatar until restart.
func (a *App) accountHead(ac launcher.Account) *ui.Bitmap {
	bm, previous := a.avatars.heads[ac.ID], a.avatars.loads[ac.ID]
	if bm != nil && (previous == nil || previous.complete) {
		return bm
	}
	if a.service == nil || a.ctx.Err() != nil || previous != nil && (previous.loading || time.Now().Before(previous.retryAt)) {
		return bm
	}
	if a.avatars.loads == nil {
		a.avatars.loads = map[string]*avatarLoad{}
	}
	if a.avatars.slots == nil {
		a.avatars.slots = make(chan struct{}, 2)
	}
	ctx, cancel := context.WithCancel(a.ctx)
	load := &avatarLoad{cancel: cancel, loading: true}
	if previous != nil {
		previous.cancel()
		load.failures = previous.failures
	}
	a.avatars.loads[ac.ID] = load
	a.touchHead(ac.ID)
	slots := a.avatars.slots
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		defer cancel()
		if bm == nil && ac.Kind != "offline" {
			if skin, err := a.service.CachedAccountSkin(ctx, ac.ID); err == nil {
				if head := accountSkinHead(ac, skin); head != nil {
					a.post(func() {
						if a.avatars.loads[ac.ID] == load {
							a.rememberHead(ac.ID, ui.NewBitmap(head).Pixelated())
						}
					})
				}
			}
		}
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			return
		}
		requestCtx, stop := context.WithTimeout(ctx, 15*time.Second)
		skin, err := a.service.LoadAccountSkin(requestCtx, ac.ID)
		stop()
		<-slots
		if ctx.Err() != nil {
			return
		}
		var head image.Image
		if err == nil {
			head = accountSkinHead(ac, skin)
		}
		failures := load.failures + 1
		delay := avatarRetryDelay(failures)
		a.post(func() {
			if a.avatars.loads[ac.ID] != load {
				return
			}
			load.loading = false
			if head != nil {
				load.complete = true
				a.rememberHead(ac.ID, ui.NewBitmap(head).Pixelated())
			} else {
				load.failures, load.retryAt = failures, time.Now().Add(delay)
			}
		})
		if head != nil {
			return
		}
		// Wake an idle window once when retrying becomes possible. A hidden
		// account does not issue another request until it is displayed again.
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
			a.post(func() {
				if a.avatars.loads[ac.ID] == load {
					load.retryAt = time.Time{}
				}
			})
		case <-ctx.Done():
		}
	}()
	return bm
}

func avatarRetryDelay(failures int) time.Duration {
	return min(5*time.Second*time.Duration(1<<min(max(failures-1, 0), 5)), 2*time.Minute)
}

func accountSkinHead(account launcher.Account, skin launcher.AccountSkin) image.Image {
	if skin.Skin != "" {
		return skinHead(imageFromData(skin.Skin))
	}
	if account.Kind == "offline" {
		return skinHead(builtinSkin(skin.Model == "slim"))
	}
	return defaultHeadFor(account)
}

// invalidateHead drops the cached head (e.g. after a skin upload).
func (a *App) invalidateHead(id string) {
	if load := a.avatars.loads[id]; load != nil {
		load.cancel()
	}
	delete(a.avatars.heads, id)
	delete(a.avatars.loads, id)
	a.avatars.headOrder = slices.DeleteFunc(a.avatars.headOrder, func(key string) bool { return key == id })
}

// headAvatar draws the account head in a rounded square, or the generic
// icon fallback.
func (a *App) headAvatar(c *ui.Context, ac *launcher.Account, size float32, fallback func()) {
	if ac != nil {
		if bm := a.accountHead(*ac); bm != nil {
			ui.Image(c, bm).Label("头像 "+ac.Name).Size(size, size).Radius(iconRadius(size, uiScale(c))).Fit(ui.Contain).Shrink(0)
			return
		}
	}
	fallback()
}

// authIcon returns the cached favicon bitmap of an auth server, loading it
// in the background on first use; nil means "use the globe icon".
func (a *App) authIcon(server string) *ui.Bitmap {
	key := strings.ToLower(strings.TrimRight(server, "/"))
	if a.avatars.icons == nil {
		a.avatars.icons, a.avatars.iconTried = map[string]*ui.Bitmap{}, map[string]bool{}
	}
	if bm := a.avatars.icons[key]; bm != nil {
		return bm
	}
	if b, ok := launcher.BuiltinAuthServer(server); ok {
		if bm := builtinAuthIcon(b.Name); bm != nil {
			return bm // bundled, works offline
		}
	}
	if a.avatars.iconTried[key] || a.service == nil || a.ctx.Err() != nil {
		return nil
	}
	a.avatars.iconTried[key] = true
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		b, e := a.service.AuthServerIcon(a.ctx, server)
		if e != nil {
			return
		}
		img, _, e := image.Decode(bytes.NewReader(b))
		if e != nil {
			return
		}
		a.post(func() {
			bm := ui.NewBitmap(img)
			if img.Bounds().Dx() <= 16 {
				bm = bm.Pixelated() // keep tiny pixel favicons crisp
			}
			a.rememberIcon(key, bm)
		})
	}()
	return nil
}

var builtinIcons sync.Map

// builtinAuthIcon decodes the bundled site icon of a built-in server
// (assets/authicons, sources in docs/THIRD_PARTY_NOTICES.md).
func builtinAuthIcon(name string) *ui.Bitmap {
	if v, ok := builtinIcons.Load(name); ok {
		return v.(*ui.Bitmap)
	}
	file := map[string]string{"LittleSkin": "littleskin.png", "Ely.by": "ely.png"}[name]
	b, e := assets.ReadFile("assets/authicons/" + file)
	if file == "" || e != nil {
		return nil
	}
	img, e := png.Decode(bytes.NewReader(b))
	if e != nil {
		return nil
	}
	bm := ui.NewBitmap(img)
	if img.Bounds().Dx() <= 16 {
		bm = bm.Pixelated()
	}
	builtinIcons.Store(name, bm)
	return bm
}

const avatarCacheLimit = 64

func (a *App) rememberHead(id string, bm *ui.Bitmap) {
	if a.avatars.heads == nil {
		a.avatars.heads = map[string]*ui.Bitmap{}
	}
	a.avatars.heads[id] = bm
	a.touchHead(id)
}

func (a *App) touchHead(id string) {
	a.avatars.headOrder = append(slices.DeleteFunc(a.avatars.headOrder, func(k string) bool { return k == id }), id)
	for len(a.avatars.headOrder) > avatarCacheLimit {
		old := a.avatars.headOrder[0]
		a.avatars.headOrder = a.avatars.headOrder[1:]
		delete(a.avatars.heads, old)
		if load := a.avatars.loads[old]; load != nil {
			load.cancel()
		}
		delete(a.avatars.loads, old)
	}
}

func (a *App) rememberIcon(key string, bm *ui.Bitmap) {
	if a.avatars.icons == nil {
		a.avatars.icons = map[string]*ui.Bitmap{}
	}
	a.avatars.icons[key] = bm
	a.avatars.iconOrder = append(slices.DeleteFunc(a.avatars.iconOrder, func(k string) bool { return k == key }), key)
	for len(a.avatars.iconOrder) > avatarCacheLimit {
		old := a.avatars.iconOrder[0]
		a.avatars.iconOrder = a.avatars.iconOrder[1:]
		delete(a.avatars.icons, old)
		delete(a.avatars.iconTried, old)
	}
}
