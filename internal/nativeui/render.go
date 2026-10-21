package nativeui

import (
	"image/png"
	"os"
	"path/filepath"

	"github.com/egoist/mygo/ui"
)

// Render exports the native widget tree through MyGo's software renderer.
func (a *App) Render(path string) error {
	return a.RenderPage(path, a.page, 1536, 1024)
}

func (a *App) ShowPage(page string) {
	switch page {
	case "home", "mods", "discover", "wardrobe", "tasks", "settings":
		a.goPage(page)
	}
}

func (a *App) RenderPage(path, page string, width, height int) error {
	switch page {
	case "home", "mods", "discover", "wardrobe", "tasks", "settings":
		a.page = page
	}
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	f, e := os.Create(path)
	if e != nil {
		return e
	}
	defer f.Close()
	return png.Encode(f, ui.Render(a.View, width, height, 1))
}
