package nativeui

import (
	"embed"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
)

//go:embed assets/*
var assets embed.FS

var loadOnce sync.Once

var preloadOnce sync.Once

var assetErr error

var modrinthLogo, curseforgeLogo, microsoftLogo, landscape *ui.SVG

var icons = map[string]*ui.SVG{}

// loaderLogos are the projects' own official logos (see THIRD_PARTY_NOTICES.md),
// keyed by loader key: vanilla, fabric, forge, neoforge, quilt.
var loaderLogos = map[string]*ui.Bitmap{}

var brandLogos = map[int]*ui.Bitmap{}

// PreloadAssets overlaps immutable resource preparation with reading the
// launcher's configuration. New joins loadOnce and reports any error before
// the first view can use the fonts, images or theme. No window is touched.
func PreloadAssets() {
	preloadOnce.Do(func() { go func() { _ = loadAssets() }() })
}

// BrandIconPNG is the 256 px app icon for the window and taskbar.
func BrandIconPNG() []byte {
	b, _ := assets.ReadFile("assets/brand/brand-256.png")
	return b
}

// Apple-inspired light palette: aurora canvas, frosted white surfaces, system accents.
var (
	bg            = ui.Hex("#f1f4fb")
	panel         = ui.Hex("#ffffff")
	raised        = ui.RGBA(246, 248, 252, .9)
	line          = ui.RGBA(60, 72, 110, .10)
	lineStrong    = ui.RGBA(60, 72, 110, .18)
	glassEdge     = ui.RGBA(255, 255, 255, .9)
	ink           = ui.Hex("#1d1d1f")
	inkSecondary  = ui.Hex("#3a3a44")
	muted         = ui.Hex("#5f6170") // ≥4.5:1 on frosted white over the aurora
	accent        = ui.Hex("#007aff") // Apple System Blue
	accentIndigo  = ui.Hex("#5e5ce6") // Apple System Indigo
	accentViolet  = ui.Hex("#af52de") // Apple System Purple
	accentHover   = ui.Hex("#0066d6")
	accentSoft    = ui.RGBA(0, 122, 255, .10)
	blue          = ui.Hex("#0a84ff")
	danger        = ui.Hex("#ff3b30") // Apple System Red
	dangerSoft    = ui.RGBA(255, 59, 48, .10)
	fieldColor    = ui.RGBA(118, 128, 160, .10)
	fieldHover    = ui.RGBA(118, 128, 160, .17)
	selectedColor = ui.RGBA(0, 122, 255, .14)
	success       = ui.Hex("#34c759") // Apple System Green
	successInk    = ui.Hex("#15803d")
	successSoft   = ui.RGBA(52, 199, 89, .12)
	warning       = ui.Hex("#ff9500") // Apple System Orange
	warningInk    = ui.Hex("#b45309")
	theme         *ui.Theme
	fastAnim      = motionFast
)

func loadAssets() error {
	loadOnce.Do(func() {
		// Only the settings wordmark uses this face. Bundle its exact glyphs
		// instead of reading and registering a 5 MB CJK font on every launch.
		font, e := assets.ReadFile("assets/pixel.otf")
		if e != nil {
			assetErr = e
			return
		}
		if e = ui.RegisterFont(font, "MCL Pixel"); e != nil {
			assetErr = e
			return
		}
		// JetBrains Mono v2.304 (variable wght, SIL OFL 1.1), under its own
		// family name so an installed copy of another version never wins.
		mono, e := assets.ReadFile("assets/JetBrainsMono.ttf")
		if e != nil {
			assetErr = e
			return
		}
		if e = ui.RegisterFont(mono, uiMonoFamily); e != nil {
			assetErr = e
			return
		}
		theme = ui.LightTheme()
		theme.Background = bg
		theme.Surface = fieldColor
		theme.SurfaceHover = fieldHover
		theme.SurfacePressed = selectedColor
		theme.Border = line
		theme.Text = ink
		theme.TextMuted = muted
		theme.Accent = accent
		theme.AccentHover = accentHover
		theme.AccentPressed = ui.Hex("#0051a8")
		theme.AccentText = ui.Hex("#ffffff")
		theme.Danger = danger
		theme.Success = successInk
		theme.Warning = warningInk
		theme.Font = uiFontFamily(titlebarOS())
		theme.FontSize = 14.5
		theme.Radius = 9
		theme.Spacing = 16.0 / 3
		theme.Focus = ui.RGBA(0, 122, 255, .25)
		// Minecraft-style pixel icons: 16×16 blocks, filled with currentColor
		// so they follow the text color of wherever they are drawn.
		const px = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" shape-rendering="crispEdges">`
		for name, body := range map[string]string{
			"home":            `<path d="M2 7h2V5h2V3h4v2h2v2h2v7H2Zm3 7V9h6v5Z"/>`,
			"mods":            `<path d="M2 2h5v5H2Zm7 0h5v5H9ZM2 9h5v5H2Zm7 0h5v5H9Z"/>`,
			"search":          `<path d="M6 2h3v1h2v1h1v2h1v3h-1v2H9v1H6v-1H4v-2H3V6h1V4h2Zm0 2H5v5h1v1h3V9h1V4H9V3H6Zm5 6h1v1h1v1h1v1h-1v1h-1v-1h-1v-1h-1v-1h1Z"/>`,
			"skin":            `<path d="M6 1h4v1h2v1h1v2h1v2h-1v1h-1V6H9V4H7v2H5v2H4V7H3V5h1V3h2Zm-3 8h3v6H3Zm7 0h3v6h-3Z"/>`,
			"tasks":           `<path d="M7 1h2v2h2v2h2v2h-2v2H9v2H7V9H5V7H3V5h2V3h2Zm-5 11h12v2H2Z"/>`,
			"settings":        `<path d="M6 0h1v1h-1ZM7 0h1v1h-1ZM8 0h1v1h-1ZM9 0h1v1h-1ZM1 1h1v1h-1ZM2 1h1v1h-1ZM6 1h1v1h-1ZM7 1h1v1h-1ZM8 1h1v1h-1ZM9 1h1v1h-1ZM13 1h1v1h-1ZM14 1h1v1h-1ZM1 2h1v1h-1ZM2 2h1v1h-1ZM3 2h1v1h-1ZM6 2h1v1h-1ZM7 2h1v1h-1ZM8 2h1v1h-1ZM9 2h1v1h-1ZM12 2h1v1h-1ZM13 2h1v1h-1ZM14 2h1v1h-1ZM2 3h1v1h-1ZM3 3h1v1h-1ZM4 3h1v1h-1ZM5 3h1v1h-1ZM6 3h1v1h-1ZM7 3h1v1h-1ZM8 3h1v1h-1ZM9 3h1v1h-1ZM10 3h1v1h-1ZM11 3h1v1h-1ZM12 3h1v1h-1ZM13 3h1v1h-1ZM3 4h1v1h-1ZM4 4h1v1h-1ZM5 4h1v1h-1ZM6 4h1v1h-1ZM7 4h1v1h-1ZM8 4h1v1h-1ZM9 4h1v1h-1ZM10 4h1v1h-1ZM11 4h1v1h-1ZM12 4h1v1h-1ZM3 5h1v1h-1ZM4 5h1v1h-1ZM5 5h1v1h-1ZM6 5h1v1h-1ZM9 5h1v1h-1ZM10 5h1v1h-1ZM11 5h1v1h-1ZM12 5h1v1h-1ZM0 6h1v1h-1ZM1 6h1v1h-1ZM2 6h1v1h-1ZM3 6h1v1h-1ZM4 6h1v1h-1ZM5 6h1v1h-1ZM10 6h1v1h-1ZM11 6h1v1h-1ZM12 6h1v1h-1ZM13 6h1v1h-1ZM14 6h1v1h-1ZM15 6h1v1h-1ZM0 7h1v1h-1ZM1 7h1v1h-1ZM2 7h1v1h-1ZM3 7h1v1h-1ZM4 7h1v1h-1ZM11 7h1v1h-1ZM12 7h1v1h-1ZM13 7h1v1h-1ZM14 7h1v1h-1ZM15 7h1v1h-1ZM0 8h1v1h-1ZM1 8h1v1h-1ZM2 8h1v1h-1ZM3 8h1v1h-1ZM4 8h1v1h-1ZM11 8h1v1h-1ZM12 8h1v1h-1ZM13 8h1v1h-1ZM14 8h1v1h-1ZM15 8h1v1h-1ZM0 9h1v1h-1ZM1 9h1v1h-1ZM2 9h1v1h-1ZM3 9h1v1h-1ZM4 9h1v1h-1ZM5 9h1v1h-1ZM10 9h1v1h-1ZM11 9h1v1h-1ZM12 9h1v1h-1ZM13 9h1v1h-1ZM14 9h1v1h-1ZM15 9h1v1h-1ZM3 10h1v1h-1ZM4 10h1v1h-1ZM5 10h1v1h-1ZM6 10h1v1h-1ZM9 10h1v1h-1ZM10 10h1v1h-1ZM11 10h1v1h-1ZM12 10h1v1h-1ZM3 11h1v1h-1ZM4 11h1v1h-1ZM5 11h1v1h-1ZM6 11h1v1h-1ZM7 11h1v1h-1ZM8 11h1v1h-1ZM9 11h1v1h-1ZM10 11h1v1h-1ZM11 11h1v1h-1ZM12 11h1v1h-1ZM2 12h1v1h-1ZM3 12h1v1h-1ZM4 12h1v1h-1ZM5 12h1v1h-1ZM6 12h1v1h-1ZM7 12h1v1h-1ZM8 12h1v1h-1ZM9 12h1v1h-1ZM10 12h1v1h-1ZM11 12h1v1h-1ZM12 12h1v1h-1ZM13 12h1v1h-1ZM1 13h1v1h-1ZM2 13h1v1h-1ZM3 13h1v1h-1ZM6 13h1v1h-1ZM7 13h1v1h-1ZM8 13h1v1h-1ZM9 13h1v1h-1ZM12 13h1v1h-1ZM13 13h1v1h-1ZM14 13h1v1h-1ZM1 14h1v1h-1ZM2 14h1v1h-1ZM6 14h1v1h-1ZM7 14h1v1h-1ZM8 14h1v1h-1ZM9 14h1v1h-1ZM13 14h1v1h-1ZM14 14h1v1h-1ZM6 15h1v1h-1ZM7 15h1v1h-1ZM8 15h1v1h-1ZM9 15h1v1h-1Z"/>`,
			"folder":          `<path d="M2 3h5v1h2v1h5v8H2Z"/>`,
			"account":         `<path d="M6 2h4v1h1v4h-1v1H6V7H5V3h1ZM3 11h10v1h1v2H2v-2h1Z"/>`,
			"plus":            `<path d="M7 3h2v4h4v2H9v4H7V9H3V7h4Z"/>`,
			"play":            `<path d="M4 2h2v1h2v1h2v1h2v1h1v4h-1v1h-2v1H8v1H6v1H4Z"/>`,
			"clock":           `<path fill-rule="evenodd" d="M6 1h4v1h2v1h1v2h1v6h-1v2h-1v1h-2v1H6v-1H4v-1H3v-2H2V5h1V3h1V2h2ZM6 3v1H4v2H3v4h1v2h2v1h4v-1h2v-2h1V6h-1V4h-2V3Z"/><path d="M7 4h2v4h3v2H7Z"/>`,
			"check":           `<path d="M2 8h2v2h2v2h2v-2h2V8h2V6h2V4h-2v2h-2v2H8v2H6V8H4Z"/>`,
			"arrow":           `<path d="M2 7h8V5h2v2h2v2h-2v2h-2V9H2Z"/>`,
			"cross":           `<path d="M3 4h2v1h2v2h2V5h2V4h2v2h-2v1h-2v2h2v2h2v1h-2v-2h-2V9H7v2H5v2H3v-2h2V9h2V7H5V5H3Z"/>`,
			"chevron":         `<path d="M5 3h2v2h2v2h2v2h-2v2H7v2H5v-2h2V9H5V7h2V5H5Z"/>`,
			"chevron-left":    `<path d="M11 2H8v2H6v2H4v4h2v2h2v2h3v-2H9v-2H7V6h2V4h2Z"/>`,
			"chevron-right":   `<path d="M5 2h3v2h2v2h2v4h-2v2H8v2H5v-2h2v-2h2V6H7V4H5Z"/>`,
			"minus":           `<path d="M3 7h10v2H3Z"/>`,
			"pause":           `<path d="M4 3h3v10H4Zm5 0h3v10H9Z"/>`,
			"trash":           `<path d="M6 1h4v1h4v2H2V2h4ZM3 5h10v10H3Zm2 2v6h2V7Zm4 0v6h2V7Z"/>`,
			"list":            `<path d="M2 3h2v2H2Zm4 0h8v2H6ZM2 7h2v2H2Zm4 0h8v2H6Zm-4 4h2v2H2Zm4 0h8v2H6Z"/>`,
			"chevron-down":    `<path d="M3 5h2v2h2v2h2V7h2V5h2v2h-2v2h-2v2H7V9H5V7H3Z"/>`,
			"cube":            `<path d="M7 1h2v1h2v1h2v1h2v2h-2v1h-2v1H9v2h5v2H9v2H7v-2H2v-2h5V8H5V7H3V6H1V4h2V3h2V2h2Z"/>`,
			"layers":          `<path d="M7 1h2v1h2v1h2v1h2v2H9v1H7V7H1V5h2V4h2V3h2Zm7 6h2v2h-2v1h-2v1H9v1H7v-1H5v-1H3v-1H1V9h2V8h2v1h2v1h2V9h2V8h2Zm0 4h2v2h-2v1h-2v1H9v1H7v-1H5v-1H3v-1H1v-2h2v1h2v1h2v1h2v-1h2v-1h2Z"/>`,
			"info":            `<path d="M6 1h4v1h2v2h1v2h1v4h-1v2h-2v1H6v-1H4v-2H3V6h1V4h2ZM7 4h2v2H7Zm0 4h2v4H7Z"/>`,
			"gamepad":         `<path d="M4 5h8v1h2v2h1v4h-2v1h-1v1h-2v-2H8v-1H6v1H4v2H2v-1H1v-4h1V8h1V6h1Zm1 2v1H4v2h1v1h2v-1h1V8H7V7Zm6 1h2v2h-2Z"/>`,
			"folder-add":      `<path d="M1 3h5v1h2v1h6v2H9v2H7v2H5V9H3V7h2V5H1Zm6 5h2v2h2v2h-2v2H7v-2H5v-2h2Z"/>`,
			"external":        `<path d="M9 2h5v5h-2V5.4L7.7 9.7 6.3 8.3 10.6 4H9ZM2 4h5v2H4v6h6V9h2v5H2Z"/>`,
			"globe":           `<path d="M6 1h4v1h1v1h2v1h1v2h1v4h-1v2h-1v1h-2v1H9v1H6v-1H4v-1H2v-2H1V6h1V4h1V3h2V2h2ZM3 6v1h2V6Zm8 0h2v1h-2Zm-6 1h6v1H5Zm-2 2h10v1H3Zm2 2h2v1H5Zm4 0h2v1H9Z"/>`,
			"download":        `<path d="M7 1h2v5H7ZM3 6h10v1H3ZM4 7h8v1H4ZM5 8h6v1H5ZM6 9h4v1H6ZM7 10h2v1H7ZM2 13h12v2H2Z"/>`,
			"doc":             `<path d="M3 1h8v2H3ZM3 1h2v14H3ZM3 13h10v2H3ZM11 4h2v11h-2ZM10 2h1v1h1v1h-1ZM7 6h4v1H7ZM7 9h4v1H7Z"/>`,
			"sun":             `<path d="M6 5h4v1h1v4h-1v1H6v-1H5V6h1ZM7 1h2v2H7ZM7 13h2v2H7ZM1 7h2v2H1ZM13 7h2v2h-2ZM3 3h2v2H3ZM11 3h2v2h-2ZM3 11h2v2H3ZM11 11h2v2h-2Z"/>`,
			"terminal":        `<path d="M2 3h12v10H2Zm2 2h2v1h1v1h1v2H7v1H5v1H4V9H3V7h1V6h1Zm5 4h3v2H9Z"/>`,
			"refresh":         `<path d="M6 2h4v1h2v1h1v1h1v2h-2V6h-1V5H9V4H7v1H6v1H5v4h1v1h1v1h2v-1h1V9h2v1h-1v1h-1v1H9v1H6v-1H4v-1H3V9H2V6h1V4h1V3h2Zm6 7h2v1h1v1h1v2h-1v1h-2v-2h1v-1h-1v-1h-1Z"/>`,
			"loader-vanilla":  `<path d="M6 2h4v1h2v1h2v1h1v6h-1v1h-2v1h-2v1H6v-1H4v-1H2v-1H1V5h1V4h2V3h2Zm1 4H5v1H4v3h1v1h2v1h2V9h1V8h1V5H9V4H7Z"/>`,
			"loader-fabric":   `<path d="M3 2h10v12H3Zm2 2h6v2H5Zm0 3h6v2H5Zm0 3h4v2H5Z"/>`,
			"loader-forge":    `<path d="M6 2h4v2H6ZM4 5h8v1h1v2h1v1h-2v1H4V9H3V8H2V6h1V5h1Zm-1 7h10v2H3Z"/>`,
			"loader-neoforge": `<path d="M7 1h2v1h2v1h2v1h1v4h-1v1h-1v1h-1v1H9v2h2v2H9v1H7v-1H5v-2h2v-2H6v-1H5v-1H4V8H3V4h1V3h2V2h1Zm-1 5h4v1h1v2h-2V8H8V7H6Z"/>`,
			"loader-quilt":    `<path d="M7 1h2v2h2v2h2v2h2v2h-2v2h-2v2H9v2H7v-2H5v-2H3v-2H1V7h2V5h2V3h2Zm0 4H6v1H5v2h1v1h2v1h2V8h1V6H9V5H7Z"/>`,
		} {
			icons[name] = ui.MustParseSVG([]byte(px + `<g fill="currentColor">` + body + `</g></svg>`))
		}
		for _, key := range []string{"vanilla", "fabric", "forge", "neoforge", "quilt"} {
			b, e := assets.ReadFile("assets/loaders/" + key + ".png")
			if e != nil {
				assetErr = e
				return
			}
			bmp, e := ui.DecodeBitmap(b)
			if e != nil {
				assetErr = e
				return
			}
			loaderLogos[key] = bmp
		}
		for _, px := range []int{32, 64, 128} {
			if b, e := assets.ReadFile(fmt.Sprintf("assets/brand/brand-%d.png", px)); e == nil {
				if bmp, e := ui.DecodeBitmap(b); e == nil {
					brandLogos[px] = bmp
				}
			}
		}
		// GitHub's Octicons mark (MIT); retain smooth curves for the brand icon.
		icons["github"] = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><path d="M6.766 11.328c-2.063-.25-3.516-1.734-3.516-3.656 0-.781.281-1.625.75-2.188-.203-.515-.172-1.609.063-2.062.625-.078 1.468.25 1.968.703.594-.187 1.219-.281 1.985-.281.765 0 1.39.094 1.953.265.484-.437 1.344-.765 1.969-.687.218.422.25 1.515.046 2.047.5.593.766 1.39.766 2.203 0 1.922-1.453 3.375-3.547 3.64.531.344.89 1.094.89 1.954v1.625c0 .468.391.734.86.547C13.781 14.359 16 11.53 16 8.03 16 3.61 12.406 0 7.984 0 3.563 0 0 3.61 0 8.031a7.88 7.88 0 0 0 5.172 7.422c.422.156.828-.125.828-.547v-1.25c-.219.094-.5.156-.75.156-1.031 0-1.64-.562-2.078-1.609-.172-.422-.36-.672-.719-.719-.187-.015-.25-.093-.25-.187 0-.188.313-.328.625-.328.453 0 .844.281 1.25.86.313.452.64.655 1.031.655s.641-.14 1-.5c.266-.265.47-.5.657-.656"/></svg>`))
		microsoftLogo = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><path fill="#f25022" d="M0 0h23v23H0z"/><path fill="#7fba00" d="M25 0h23v23H25z"/><path fill="#00a4ef" d="M0 25h23v23H0z"/><path fill="#ffb900" d="M25 25h23v23H25z"/></svg>`))
		for file, target := range map[string]**ui.SVG{"modrinth.svg": &modrinthLogo, "curseforge.svg": &curseforgeLogo, "landscape.svg": &landscape} {
			b, e := assets.ReadFile("assets/" + file)
			if e != nil {
				assetErr = e
				return
			}
			*target, e = ui.ParseSVG(b)
			if e != nil {
				assetErr = e
				return
			}
		}
	})
	return assetErr
}

// Apple Oklab gradient helpers.
func appleAccentGradient(hovered bool) ui.LinearGradient {
	if hovered {
		return ui.LinearGradient{From: ui.Hex("#2b95ff"), To: ui.Hex("#7471f0"), Angle: 120}
	}
	return ui.LinearGradient{From: ui.Hex("#0a84ff"), To: accentIndigo, Angle: 120}
}

// tintGradient is the two-tone squircle fill of a colored row or header icon.
func tintGradient(from, to ui.Color) ui.LinearGradient {
	return ui.LinearGradient{From: from, To: to, Angle: 135}
}

// glass styles e as a frosted surface: translucent white over the aurora
// canvas, a bright hairline edge and a soft two-layer ambient shadow.
func glass(e ui.Element, radius, s float32) ui.Element {
	return e.Radius(radius).LinearGradient(ui.LinearGradient{From: ui.RGBA(255, 255, 255, .82), To: ui.RGBA(255, 255, 255, .66), Angle: 180}).
		Border(1, glassEdge).Shadow(0, 12*s, 32*s, -10*s, ui.RGBA(40, 52, 110, .14)).Shadow(0, 1*s, 3*s, 0, ui.RGBA(40, 52, 110, .05))
}

// popover styles floating layers (dialogs, menus) that sit above content
// and must stay legible over anything.
func popover(e ui.Element, radius, s float32) ui.Element {
	return e.Radius(radius).LinearGradient(ui.LinearGradient{From: ui.RGBA(255, 255, 255, 1), To: ui.RGBA(248, 249, 253, 1), Angle: 180}).
		Border(1, glassEdge).Shadow(0, 24*s, 60*s, -12*s, ui.RGBA(30, 40, 90, .28)).Shadow(0, 0, 0, 1, ui.RGBA(30, 40, 90, .06))
}

func appleSoftIconBox(c *ui.Context, size float32, iconName string, from, to, fg ui.Color) {
	ui.Box(c).Size(size, size).Radius(size*.3).Shrink(0).LinearGradient(tintGradient(from, to)).InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, .35)).Shadow(0, size*.08, size*.25, -size*.06, from.Alpha(.35)).Center().Children(func() {
		if icons[iconName] != nil {
			ui.Icon(c, icons[iconName]).Size(size*.52, size*.52).TextColor(fg)
		}
	})
}

func loaderIconName(loader string) string {
	v := strings.ToLower(strings.TrimSpace(loader))
	switch v {
	case "fabric":
		return "loader-fabric"
	case "forge":
		return "loader-forge"
	case "neoforge":
		return "loader-neoforge"
	case "quilt":
		return "loader-quilt"
	case "all", "all loaders", "全部加载器":
		return "layers"
	default:
		return "loader-vanilla"
	}
}

func loaderTone(loader string) ui.Color {
	switch strings.ToLower(strings.TrimSpace(loader)) {
	case "fabric":
		return ui.Hex("#0284c7")
	case "forge":
		return ui.Hex("#d97706")
	case "neoforge":
		return ui.Hex("#ea580c")
	case "quilt":
		return ui.Hex("#7c3aed")
	default:
		return successInk
	}
}

func label(c *ui.Context, s string) ui.Element {
	return ui.Text(c, s).TextColor(muted).FontSize(typeCallout * uiScale(c))
}

// Apple Pill Badge with soft gradient fill
func badge(c *ui.Context, s string, color ui.Color) {
	scale := uiScale(c)
	from, to := ui.Hex("#f1f5f9"), ui.Hex("#e8eef6")
	fg := color
	switch color {
	case accent, blue:
		from, to = ui.Hex("#eaf3ff"), ui.Hex("#dcebff")
		fg = accent
	case success, successInk:
		from, to = ui.Hex("#e8faed"), ui.Hex("#d3f5dd")
		fg = successInk
	case danger:
		from, to = ui.Hex("#ffeceb"), ui.Hex("#ffdfdd")
		fg = danger
	case warning, warningInk:
		from, to = ui.Hex("#fff6e5"), ui.Hex("#ffebcc")
		fg = warningInk
	}
	ui.Row(c).Shrink(0).Padding(3*scale, 9*scale).Radius(999).LinearGradient(tintGradient(from, to)).Border(1, ui.RGBA(255, 255, 255, .7)).Children(func() {
		ui.Text(c, s).FontSize(typeCaption * scale).FontWeight(600).TextColor(fg).NoWrap().Shrink(0)
	})
}

func tagChip(c *ui.Context, s string) {
	scale := uiScale(c)
	ui.Row(c).Shrink(0).Padding(3*scale, 9*scale).Radius(999).Background(fieldColor).Children(func() {
		ui.Text(c, s).FontSize(typeCaption2 * scale).FontWeight(500).TextColor(inkSecondary).NoWrap().Shrink(0)
	})
}

func buttonWidth(c *ui.Context, text, icon string, fontSize float32) float32 {
	w, _ := c.MeasureText(0, ui.Span{Text: text, Font: theme.Font, Size: fontSize, Weight: 600})
	w += 28 * uiScale(c)
	if icon != "" {
		w += fontSize + 8*uiScale(c)
	}
	return float32(math.Ceil(float64(max(w, btnMin*uiScale(c)))))
}

// styleButton paints a capsule-like squircle: primary CTAs get the blue →
// indigo gradient with a colored glow and a top highlight, secondary ones a
// frosted white face with a hairline edge.
func styleButton(b ui.Element, s float32, primary, disabled bool) {
	hovered := b.Hovered() && !disabled
	pressed := b.Pressed() && !disabled
	if disabled {
		// One disabled look for every button: neutral fill, hairline edge,
		// readable label — never opacity alone (see disabledInk).
		b.Background(disabledFill).Border(1, disabledEdge)
		return
	}
	if primary {
		if pressed {
			b.LinearGradient(ui.LinearGradient{From: ui.Hex("#0a6fe0"), To: ui.Hex("#4f4dd0"), Angle: 120}).InsetShadow(0, 1*s, 3*s, 0, ui.RGBA(0, 0, 60, .18)).Shadow(0, 2*s, 6*s, -2*s, ui.RGBA(10, 100, 255, .35))
			return
		}
		glow := ui.RGBA(10, 100, 255, .30)
		if hovered {
			glow = ui.RGBA(10, 100, 255, .42)
		}
		b.LinearGradient(appleAccentGradient(hovered)).InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, .28)).Shadow(0, 4*s, 12*s, -3*s, glow)
		return
	}
	targetBg := ui.RGBA(255, 255, 255, .78)
	edge := lineStrong.Alpha(.12)
	switch {
	case disabled:
		targetBg, edge = ui.RGBA(255, 255, 255, .4), line.Alpha(.06)
	case pressed:
		targetBg = ui.RGBA(236, 240, 250, 1)
	case hovered:
		targetBg, edge = ui.RGBA(255, 255, 255, 1), lineStrong.Alpha(.2)
	}
	b.Background(b.AnimateColor("bg", targetBg, fastAnim)).Border(1, edge)
	if !disabled && !pressed {
		b.Shadow(0, 1*s, 3*s, 0, ui.RGBA(40, 52, 110, .08))
	}
}

func buttonContent(c *ui.Context, key, text, icon string, fontSize float32, fg ui.Color) {
	if icon != "" && icons[icon] != nil {
		ui.Icon(c, icons[icon]).Size(fontSize, fontSize).Shrink(0)
	}
	ui.Text(c, text).Label("按钮文字 " + key).FontSize(fontSize).FontWeight(600).TextColor(fg).NoWrap().Shrink(0)
}

func buttonForeground(primary, disabled bool) ui.Color {
	switch {
	case disabled:
		return disabledInk
	case primary:
		return panel
	}
	return ink
}

// button is the compact standard action button; its width follows the text.
func button(c *ui.Context, key, text, icon string, primary, disabled bool) bool {
	s := uiScale(c)
	fg := buttonForeground(primary, disabled)
	b := ui.ButtonBase(c.Key(key)).Label(text).Size(buttonWidth(c, text, icon, typeCallout*s), btnMd*s).Shrink(0).Gap(7*s).Padding(0, 14*s).AlignItems(ui.Center).Justify(ui.Center).Radius(radiusControl * s).TextColor(fg)
	return finishButton(c, b, key, text, icon, typeCallout*s, fg, primary, disabled)
}

// fixedButton is a button of at least width × height, so aligned controls
// in a row share one size.
func fixedButton(c *ui.Context, key, text, icon string, width, height float32, primary, disabled bool) bool {
	s := uiScale(c)
	fg := buttonForeground(primary, disabled)
	font := min(typeCallout*s, max(typeCaption*s, height*.43))
	b := ui.ButtonBase(c.Key(key)).Label(text).Size(max(width, buttonWidth(c, text, icon, font)), height).Shrink(0).PaddingX(14 * s).Radius(min(radiusControl*s, height*.30)).TextColor(fg).AlignItems(ui.Center).Justify(ui.Center).Gap(7 * s)
	return finishButton(c, b, key, text, icon, font, fg, primary, disabled)
}

// selectBox wraps ui.SelectBase with an Apple-styled rounded trigger and scrollable frosted popup.
func selectBox(c *ui.Context, key, labelText string, selected *string, options []string) ui.Element {
	return selectBoxWithIcons(c, key, labelText, selected, options, nil)
}

func selectBoxWithIcons(c *ui.Context, key, labelText string, selected *string, options []string, iconFn func(string) (string, ui.Color)) ui.Element {
	return selectBoxFull(c, key, labelText, selected, options, iconFn, nil)
}

// selectBoxFull renders option values through display (when set) so long
// internal ids can be shown as short human titles. Text always ellipsizes.
func selectBoxFull(c *ui.Context, key, labelText string, selected *string, options []string, iconFn func(string) (string, ui.Color), display func(string) string) ui.Element {
	return selectBoxImpl(c, key, labelText, selected, options, iconFn, display, nil, nil)
}

// selectBoxImpl optionally draws a custom leading icon per option.
func selectBoxImpl(c *ui.Context, key, labelText string, selected *string, options []string, iconFn func(string) (string, ui.Color), display func(string) string, drawIcon func(string), trailing func(opt string, highlighted bool)) ui.Element {
	s := uiScale(c)
	show := func(v string) string {
		if display != nil {
			return display(v)
		}
		return v
	}
	ctx := c
	if key != "" {
		ctx = c.Key(key)
	}
	sel := ui.SelectBase(ctx, selected)
	triggerBg := fieldColor
	if sel.Trigger.Hovered() || sel.Open() {
		triggerBg = fieldHover
	}
	if sel.Open() {
		sel.Trigger.Border(1, accent.Alpha(.35))
	}
	if labelText != "" {
		sel.Trigger.Label(labelText)
	}
	sel.Trigger.Height(btnMd*s).Padding(6*s, 12*s).Radius(radiusControl * s).Clip().Background(sel.Trigger.AnimateColor("bg", triggerBg, fastAnim)).AlignItems(ui.Center).Gap(8 * s).Children(func() {
		if drawIcon != nil {
			drawIcon(*selected)
		}
		if iconFn != nil {
			ic, col := iconFn(*selected)
			if loaderLogos[strings.TrimPrefix(ic, "loader-")] != nil {
				loaderLogo(c, *selected, 20*s)
			} else if icons[ic] != nil {
				ui.Icon(c, icons[ic]).Size(16*s, 16*s).TextColor(col).Shrink(0)
			}
		}
		ui.Text(c, show(*selected)).FontSize(typeBody * s).TextColor(ink).Grow(1).Shrink(1).Basis(0).MinWidth(0).SingleLine().Tooltip(*selected)
		if trailing != nil {
			trailing(*selected, false)
		}
		ui.Icon(c, icons["chevron-down"]).Size(15*s, 15*s).TextColor(muted).Shrink(0)
	})
	sel.Popup(func(popupPanel ui.Element) {
		popover(popupPanel.Padding(6*s), 14*s, s)
		popupPanel.TransformOrigin(.5, 0).Transition(ui.ElementTransition{Animation: &motionEnter, Colors: true, Enter: &ui.Motion{Opacity: 0, Y: -6 * s, Scale: .98}})
		scrollArea(c, "select-scroll-"+key+"-"+labelText).MaxHeight(250*s).FillWidth().Padding(0, scrollGutter*s, 0, 0).Children(func() {
			ui.Column(c).FillWidth().Gap(2 * s).Children(func() {
				for _, opt := range options {
					option := opt
					item := sel.Item(option).FillWidth().Padding(8*s, 12*s).Radius(9 * s).AlignItems(ui.Center).Gap(8 * s)
					isSelected := option == *selected
					fgCol := ink
					if item.Highlighted() {
						item.LinearGradient(appleAccentGradient(false))
						fgCol = panel
					} else if isSelected {
						item.Background(accentSoft)
						fgCol = accent
					}
					item.Children(func() {
						if drawIcon != nil {
							drawIcon(option)
						}
						if iconFn != nil {
							ic, col := iconFn(option)
							if item.Highlighted() {
								col = panel
							}
							if loaderLogos[strings.TrimPrefix(ic, "loader-")] != nil {
								loaderLogo(c, option, 20*s)
							} else if icons[ic] != nil {
								ui.Icon(c, icons[ic]).Size(16*s, 16*s).TextColor(col).Shrink(0)
							}
						}
						ui.Text(c, show(option)).FontSize(typeCallout * s).TextColor(fgCol).Grow(1).Basis(0).MinWidth(0).SingleLine()
						if trailing != nil {
							trailing(option, item.Highlighted())
						}
						if isSelected {
							ui.Icon(c, icons["check"]).Size(14*s, 14*s).TextColor(fgCol).Shrink(0)
						}
					})
				}
			})
		})
	})
	return sel.Trigger
}

func loaderSelectBox(c *ui.Context, key, labelText string, selected *string, options []string) ui.Element {
	return selectBoxWithIcons(c, key, labelText, selected, options, func(opt string) (string, ui.Color) {
		return loaderIconName(opt), loaderTone(opt)
	})
}

func card(c *ui.Context, key string, fn func()) {
	s := uiScale(c)
	glass(ui.Column(c.Key(key)).FillWidth().Clip().Gap(14*s).Padding(18*s, 22*s), radiusCard*s, s).Children(fn)
}

// loaderLogoKey maps a loader name or icon name to its official logo key,
// or "" when there is none (e.g. "all loaders").
func loaderLogoKey(loader string) string {
	k := strings.TrimPrefix(loaderIconName(loader), "loader-")
	if loaderLogos[k] != nil {
		return k
	}
	return ""
}

// loaderLogo draws the loader's official logo unaltered inside a neutral
// rounded container (no tinted tile), or nothing when it has none.
func loaderLogo(c *ui.Context, loader string, size float32) bool {
	k := loaderLogoKey(loader)
	if k == "" {
		return false
	}
	ui.Box(c).Size(size, size).Radius(size*.26).Shrink(0).Background(panel).Border(1, line).Clip().Center().Children(func() {
		inner := size * .8
		if k == "forge" || k == "vanilla" {
			inner = size // full-bleed app-icon style logos
		}
		ui.Image(c, loaderLogos[k]).Size(inner, inner).Fit(ui.Contain).Label(k + " logo")
	})
	return true
}

// iconOnlyButton is a square secondary button; label stays for a11y/tooltip.
func iconOnlyButton(c *ui.Context, key, icon, label string, size float32, disabled bool) bool {
	s := uiScale(c)
	fg := buttonForeground(false, disabled)
	b := ui.ButtonBase(c.Key(key)).Label(label).Tooltip(label).Size(size, size).Shrink(0).Radius(min(radiusControl*s, size*.30)).AlignItems(ui.Center).Justify(ui.Center)
	styleButton(b, s, false, disabled)
	return b.Children(func() { ui.Icon(c, icons[icon]).Size(0.45*size, 0.45*size).TextColor(fg) }).Clicked() && !disabled
}

// javaTag is the small tinted pill for a recommended/required Java major.
func javaTag(c *ui.Context, major int) {
	if major <= 0 {
		return
	}
	scale := uiScale(c)
	ui.Row(c).Label(fmt.Sprintf("Java 标签 %d", major)).Shrink(0).Padding(2*scale, 8*scale).Radius(999).Background(accent.Alpha(.10)).Border(1, accent.Alpha(.18)).Children(func() {
		ui.Text(c, fmt.Sprintf("Java %d", major)).FontSize(typeCaption2 * scale).FontWeight(600).TextColor(accent).NoWrap().Shrink(0)
	})
}

// Disabled controls keep a readable label (about 5:1 on their fill)
// instead of the toolkit's 50% fade.
var (
	disabledFill = ui.Hex("#e8ebf1")
	disabledEdge = ui.RGBA(30, 40, 70, .10)
	disabledInk  = ui.Hex("#596275")
)

// buttonWhy[key] is the reason a button is disabled, set by buttonWhy for
// the frame; shown as its tooltip.
var (
	dialogPrimaryKey string // first enabled primary button in the open dialog
	enterFired       string // primary button Enter activated this frame
	inDialog         bool
)

func finishButton(c *ui.Context, b ui.Element, key, text, icon string, font float32, fg ui.Color, primary, disabled bool) bool {
	s := uiScale(c)
	styleButton(b, s, primary, disabled)
	if disabled {
		// Not ui.Disabled: that halves the opacity. The button still
		// announces itself and keeps focus, but does nothing.
		b.Cursor(ui.CursorDefault)
		if why := pendingWhy; why != "" {
			b.Tooltip(why)
		}
	}
	pendingWhy = ""
	if primary && !disabled && inDialog && dialogPrimaryKey == "" {
		dialogPrimaryKey = key
	}
	clicked := b.Children(func() { buttonContent(c, key, text, icon, font, fg) }).Clicked()
	if !disabled && enterFired == key {
		enterFired = ""
		return true
	}
	return clicked && !disabled
}

var pendingWhy string

// buttonWhy is button with the reason it is disabled: why == "" means
// enabled. The reason shows as a tooltip and, with inline, as a muted hint
// beside the button, so it never depends on hover alone.
func buttonWhy(c *ui.Context, key, text, icon string, primary bool, why string, inline bool) bool {
	pendingWhy = why
	clicked := button(c, key, text, icon, primary, why != "")
	if why != "" && inline {
		whyHint(c, why)
	}
	return clicked
}

func whyHint(c *ui.Context, why string) {
	s := uiScale(c)
	ui.Row(c).Label("禁用原因 " + why).Gap(5 * s).AlignItems(ui.Center).Shrink(1).MinWidth(0).Children(func() {
		ui.Icon(c, icons["info"]).Size(13*s, 13*s).TextColor(disabledInk).Shrink(0)
		ui.Text(c, why).FontSize(typeFootnote * s).TextColor(disabledInk).SingleLine().Shrink(1).MinWidth(0)
	})
}

// pathLabel shows a path whose end matters most: the folder part shrinks
// (ellipsized) and the last element stays whole; the full path is the
// tooltip.
func pathLabel(c *ui.Context, path string, size float32) ui.Element {
	s := uiScale(c)
	dir, base := filepath.Split(path)
	if base == "" {
		dir, base = filepath.Split(strings.TrimRight(path, `/\`))
		base += string(filepath.Separator)
	}
	return ui.Row(c).Label("路径 " + path).Grow(1).Basis(0).MinWidth(0).Gap(0).AlignItems(ui.Center).Tooltip(path).Children(func() {
		if dir != "" {
			ui.Text(c, dir).FontSize(size).TextColor(muted).SingleLine().Shrink(1).MinWidth(0)
		}
		ui.Text(c, base).FontSize(size).FontWeight(600).TextColor(inkSecondary).SingleLine().Shrink(0).MaxWidthPercent(70)
		_ = s
	})
}

// dangerButton is the armed second step of a destructive action.
func dangerButton(c *ui.Context, key, text string, height float32, disabled bool) bool {
	s := uiScale(c)
	b := ui.ButtonBase(c.Key(key)).Label(text).Height(height).PaddingX(12 * s).Shrink(0).Radius(min(12*s, height*.32)).AlignItems(ui.Center).Background(danger).AutoFocus()
	return b.Children(func() {
		ui.Text(c, text).FontSize(typeFootnote * s).FontWeight(600).TextColor(panel).NoWrap()
	}).Clicked() && !disabled
}

// Button size tokens: heights by role, and one horizontal padding.
const (
	btnSm  float32 = 30 // row actions (选择, 设为当前)
	btnMd  float32 = 35 // standard action button
	btnLg  float32 = 40 // primary launch CTA
	btnPad float32 = 12
	btnMin float32 = 54 // minimum width of a text button
)

// midEllipsis shortens text to fit maxW by cutting its middle, keeping the
// start and the end ("一个非常…目录名称", "C:\Users\…\.minecraft").
func midEllipsis(c *ui.Context, text string, maxW, size float32, weight int) string {
	measure := func(t string) float32 {
		w, _ := c.MeasureText(0, ui.Span{Text: t, Font: theme.Font, Size: size, Weight: weight})
		return w
	}
	if measure(text) <= maxW {
		return text
	}
	r := []rune(text)
	lo, hi := 0, len(r)
	for lo < hi { // largest keep count that fits
		mid := (lo + hi + 1) / 2
		head := (mid + 1) / 2
		cand := string(r[:head]) + "…" + string(r[len(r)-(mid-head):])
		if measure(cand) <= maxW {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	head := (lo + 1) / 2
	return string(r[:head]) + "…" + string(r[len(r)-(lo-head):])
}

// uiMonoFamily is the bundled JetBrains Mono, the primary face for Latin
// text, digits, versions and paths.
const uiMonoFamily = "MCL JetBrains Mono"

// uiFontFamily is the UI family list: the bundled JetBrains Mono first
// (it has no CJK glyphs), then the OS's own CJK face, then the system UI.
func uiFontFamily(goos string) string {
	cjk := "Noto Sans CJK SC, Noto Sans SC, WenQuanYi Micro Hei"
	switch goos {
	case "darwin":
		cjk = "PingFang SC, Hiragino Sans GB"
	case "windows":
		cjk = "Microsoft YaHei UI, Microsoft YaHei"
	}
	return uiMonoFamily + ", " + cjk + ", system-ui"
}

// textField is the one single-line input: a fixed control height with no
// vertical padding, so the toolkit centers the line box and mono/CJK text
// sit on the same middle line in every field; 13-DIP side insets.
func textField(c *ui.Context, value *string) ui.Element {
	s := uiScale(c)
	return ui.TextInput(c, value).Height(btnMd*s).Padding(0, 13*s).FontSize(typeCallout * s)
}
