package nativeui

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// avatars caches the 8×8 skin head (face + hat layer) per account id.
// A nil entry means "loading"; absence means "not requested yet".
type avatarCache struct {
	heads     map[string]*ui.Bitmap
	tried     map[string]bool
	headOrder []string
	icons     map[string]*ui.Bitmap
	iconTried map[string]bool
	iconOrder []string
}

// skinHead composites the face (8,8) and hat overlay (40,8) of a 64×32/64×64
// skin into an 8×8 image.
func skinHead(skin image.Image) image.Image {
	if skin == nil || skin.Bounds().Dx() < 64 || skin.Bounds().Dy() < 32 {
		return nil
	}
	b := skin.Bounds().Min
	out := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	draw.Draw(out, out.Bounds(), skin, b.Add(image.Pt(8, 8)), draw.Src)
	draw.Draw(out, out.Bounds(), skin, b.Add(image.Pt(40, 8)), draw.Over)
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

// accountHead returns the cached head bitmap, starting an async load on
// first use. Returns nil (caller shows the generic icon) while loading or
// when no skin is available.
func (a *App) accountHead(ac launcher.Account) *ui.Bitmap {
	if a.avatars.heads == nil {
		a.avatars.heads, a.avatars.tried = map[string]*ui.Bitmap{}, map[string]bool{}
	}
	if bm := a.avatars.heads[ac.ID]; bm != nil {
		return bm
	}
	if a.avatars.tried[ac.ID] || a.service == nil {
		return nil
	}
	a.avatars.tried[ac.ID] = true
	go func() {
		var head image.Image
		if v, e := a.service.Appearance(a.ctx, ac.ID); e == nil {
			head = skinHead(imageFromData(v.Skin))
			if head == nil && ac.Kind == "offline" {
				head = skinHead(builtinSkin(v.Model == "slim"))
			}
		}
		if head == nil && ac.Kind == "offline" {
			head = defaultHeadFor(ac)
		}
		if head == nil {
			return
		}
		a.post(func() { a.rememberHead(ac.ID, ui.NewBitmap(head).Pixelated()) })
	}()
	return nil
}

// invalidateHead drops the cached head (e.g. after a skin upload).
func (a *App) invalidateHead(id string) {
	delete(a.avatars.heads, id)
	delete(a.avatars.tried, id)
}

// headAvatar draws the account head in a rounded square, or the generic
// icon fallback.
func (a *App) headAvatar(c *ui.Context, ac *launcher.Account, size float32, fallback func()) {
	if ac != nil {
		if bm := a.accountHead(*ac); bm != nil {
			ui.Image(c, bm).Label("头像 "+ac.Name).Size(size, size).Radius(size * .22).Fit(ui.Contain).Shrink(0)
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
	if a.avatars.iconTried[key] || a.service == nil {
		return nil
	}
	a.avatars.iconTried[key] = true
	go func() {
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
	a.avatars.headOrder = append(slices.DeleteFunc(a.avatars.headOrder, func(k string) bool { return k == id }), id)
	for len(a.avatars.headOrder) > avatarCacheLimit {
		old := a.avatars.headOrder[0]
		a.avatars.headOrder = a.avatars.headOrder[1:]
		delete(a.avatars.heads, old)
		delete(a.avatars.tried, old)
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
