package nativeui

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strconv"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
	"mcl/internal/launcher"
)

// avatars caches the 8×8 skin head (face + hat layer) per account id.
// A nil entry means "loading"; absence means "not requested yet".
type avatarCache struct {
	heads     map[string]*ui.Bitmap
	tried     map[string]bool
	icons     map[string]*ui.Bitmap
	iconTried map[string]bool
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

// defaultAlexSkin recolours the built-in default as an Alex-style head
// (ginger hair, lighter skin, green eyes).
func defaultAlexSkin() image.Image {
	src := defaultSkin().(*image.NRGBA)
	out := image.NewNRGBA(src.Bounds())
	copy(out.Pix, src.Pix)
	swap := map[color.NRGBA]color.NRGBA{
		{69, 53, 41, 255}:    {222, 118, 43, 255},
		{191, 150, 114, 255}: {243, 206, 165, 255},
		{77, 93, 110, 255}:   {74, 140, 75, 255},
	}
	for i := 0; i < len(out.Pix); i += 4 {
		c := color.NRGBA{out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3]}
		if n, ok := swap[c]; ok {
			out.Pix[i], out.Pix[i+1], out.Pix[i+2] = n.R, n.G, n.B
		}
	}
	return out
}

func defaultHeadFor(ac launcher.Account) image.Image {
	if defaultIsAlex(ac.UUID) {
		return skinHead(defaultAlexSkin())
	}
	return skinHead(defaultSkin())
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
		}
		if head == nil && ac.Kind == "offline" {
			head = defaultHeadFor(ac)
		}
		if head == nil {
			return
		}
		a.post(func() { a.avatars.heads[ac.ID] = ui.NewBitmap(head).Pixelated() })
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
			a.avatars.icons[key] = bm
		})
	}()
	return nil
}

var builtinIcons sync.Map

// builtinAuthIcon decodes the bundled site icon of a built-in server
// (assets/authicons, sources in THIRD_PARTY_NOTICES.md).
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
