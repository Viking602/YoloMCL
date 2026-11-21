package nativeui

import (
	"embed"
	"fmt"
	"sync"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
)

//go:embed assets/*
var assets embed.FS

var loadOnce sync.Once

var preloadOnce sync.Once

var assetErr error

var modrinthLogo, curseforgeLogo, microsoftLogo, landscape *ui.SVG

var icons = map[string]*ui.SVG{}

// loaderLogos are the projects' own official logos (see docs/THIRD_PARTY_NOTICES.md),
// keyed by loader key: vanilla, fabric, forge, neoforge, quilt, horizon.
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
		// Crisp Lucide vector icons from lobe-mygo for unified desktop UI chrome.
		for name, svg := range map[string]*ui.SVG{
			"home":          lucide.House(),
			"mods":          lucide.LayoutGrid(),
			"search":        lucide.Search(),
			"skin":          lucide.Shirt(),
			"tasks":         lucide.Download(),
			"settings":      lucide.Settings(),
			"folder":        lucide.Folder(),
			"folder-add":    lucide.FolderPlus(),
			"account":       lucide.User(),
			"plus":          lucide.Plus(),
			"play":          lucide.Play(),
			"pause":         lucide.Pause(),
			"clock":         lucide.Clock(),
			"check":         lucide.Check(),
			"arrow":         lucide.ArrowRight(),
			"cross":         lucide.X(),
			"chevron":       lucide.ChevronRight(),
			"chevron-left":  lucide.ChevronLeft(),
			"chevron-right": lucide.ChevronRight(),
			"chevron-down":  lucide.ChevronDown(),
			"minus":         lucide.Minus(),
			"expand":        lucide.Maximize2(),
			"restore":       lucide.Minimize2(),
			"trash":         lucide.Trash2(),
			"list":          lucide.List(),
			"cube":          lucide.Box(),
			"layers":        lucide.Layers(),
			"info":          lucide.Info(),
			"gamepad":       lucide.Gamepad2(),
			"external":      lucide.ExternalLink(),
			"globe":         lucide.Globe(),
			"download":      lucide.Download(),
			"doc":           lucide.FileText(),
			"sun":           lucide.Sun(),
			"terminal":      lucide.Terminal(),
			"refresh":       lucide.RefreshCw(),
		} {
			icons[name] = svg
		}
		// Minecraft-specific pixel icons kept for seedmap structure markers
		// and fallback loader emblems.
		const px = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" shape-rendering="crispEdges">`
		for name, body := range map[string]string{
			"seedmap-ship":     `<path d="M7 1h2v7h5v4h-2v2H4v-2H2V8h5Zm-4 13h10v1H3Zm2-7V3h1v4Zm5 0V3h1v4Z"/>`,
			"seedmap-fortress": `<path d="M1 2h2v2h2V2h2v4h2V2h2v2h2V2h2v13H1Zm5 13h4v-5H6ZM3 7v2h2V7Zm8 0v2h2V7Z"/>`,
			"seedmap-city":     `<path d="M1 15V7h3V4h3V1h2v3h3v3h3v8h-4v-5H5v5ZM4 6v2h2V6Zm6 0v2h2V6Z"/>`,
			"seedmap-spawn":    `<path d="M3 1h2v14H3Zm2 1h9v6H5Zm0 6h5v2H5Z"/>`,
			"seedmap-temple":   `<path d="M1 14h14v1H1Zm1-3h12v2H2Zm2-3h8v2H4Zm2-3h4v2H6Zm1-3h2v2H7Z"/>`,
			"loader-vanilla":   `<path d="M6 2h4v1h2v1h2v1h1v6h-1v1h-2v1h-2v1H6v-1H4v-1H2v-1H1V5h1V4h2V3h2Zm1 4H5v1H4v3h1v1h2v1h2V9h1V8h1V5H9V4H7Z"/>`,
			"loader-fabric":    `<path d="M3 2h10v12H3Zm2 2h6v2H5Zm0 3h6v2H5Zm0 3h4v2H5Z"/>`,
			"loader-forge":     `<path d="M6 2h4v2H6ZM4 5h8v1h1v2h1v1h-2v1H4V9H3V8H2V6h1V5h1Zm-1 7h10v2H3Z"/>`,
			"loader-neoforge":  `<path d="M7 1h2v1h2v1h2v1h1v4h-1v1h-1v1h-1v1H9v2h2v2H9v1H7v-1H5v-2h2v-2H6v-1H5v-1H4V8H3V4h1V3h2V2h1Zm-1 5h4v1h1v2h-2V8H8V7H6Z"/>`,
			"loader-quilt":     `<path d="M7 1h2v2h2v2h2v2h2v2h-2v2h-2v2H9v2H7v-2H5v-2H3v-2H1V7h2V5h2V3h2Zm0 4H6v1H5v2h1v1h2v1h2V8h1V6H9V5H7Z"/>`,
			"loader-horizon":   `<path d="M7 2h2v2H7ZM3 3h2v2H3ZM11 3h2v2h-2ZM6 5h4v1h1v1h1v2H4V7h1V6h1ZM1 7h2v2H1ZM13 7h2v2h-2ZM1 10h14v2H1ZM3 13h10v1H3Z"/>`,
		} {
			icons[name] = ui.MustParseSVG([]byte(px + `<g fill="currentColor">` + body + `</g></svg>`))
		}
		for _, key := range []string{"vanilla", "fabric", "forge", "neoforge", "quilt", "horizon"} {
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
