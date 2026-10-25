package nativeui

import (
	"embed"
	"fmt"
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

// loaderLogos are the projects' own official logos (see docs/THIRD_PARTY_NOTICES.md),
// keyed by loader key: vanilla, fabric, forge, neoforge, quilt.
var loaderLogos = map[string]*ui.Bitmap{}

// Java vendor marks are small, shared offline assets, never network thumbnails.
var javaLogos = map[string]*ui.Bitmap{}

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
		theme = newTheme()
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
		for _, key := range []string{"zulu", "temurin", "corretto", "liberica", "graalvm", "jbr", "oracle"} {
			b, e := assets.ReadFile("assets/java/" + key + ".png")
			if e != nil {
				assetErr = e
				return
			}
			bmp, e := ui.DecodeBitmap(b)
			if e != nil {
				assetErr = e
				return
			}
			javaLogos[key] = bmp
		}
		loadEmptyArt()
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
