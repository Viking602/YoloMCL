package nativeui

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// Render exports the native widget tree through MyGo's software renderer.
func (a *App) Render(path string) error {
	return a.RenderPage(path, a.page, 1536, 1024)
}

func (a *App) ShowPage(page string) {
	if tab, ok := strings.CutPrefix(page, "home/"); ok { // e.g. -page home/shader
		a.goPage("home")
		a.setInstTab(tab)
		return
	}
	// QA: -page account/microsoft opens the sign-in dialog on its Microsoft
	// tab; account/microsoft/start also requests the device code.
	if strings.HasPrefix(page, "account/microsoft") {
		a.openDialog("accountAdd")
		a.accountTab = a.tr().AccTabMicrosoft
		if strings.HasSuffix(page, "/start") && a.snapshot.Services.Microsoft {
			a.execute("startMicrosoft", nil, func(b json.RawMessage) {
				v := decode[launcher.DeviceInfo](b)
				a.device = &v
				a.deviceUntil = time.Now().Add(time.Duration(v.ExpiresIn) * time.Second)
				a.nextDevicePoll = time.Now().Add(time.Duration(max(5, v.Interval)) * time.Second)
			})
		}
		return
	}
	switch page {
	case "home", "mods", "discover", "wardrobe", "tasks", "settings":
		a.goPage(page)
	}
}

func (a *App) RenderPage(path, page string, width, height int) error {
	// "home/<tab>" (or "mods") renders an instance tab; mods load
	// synchronously so the single software frame shows real rows.
	tab := ""
	if strings.HasPrefix(page, "home/") {
		page, tab = "home", strings.TrimPrefix(page, "home/")
	} else if page == "mods" {
		page, tab = "home", "mods"
	}
	switch page {
	case "home", "discover", "wardrobe", "tasks", "settings":
		a.page = page
	}
	if page == "home" {
		if tab != "" {
			a.instTab = tab
		}
		if r := a.root(); r != nil {
			a.instLoaded = r.ID
			switch a.instTab {
			case "resourcepack", "shader":
				a.addonTab = a.instTab
				a.assetPacks, _ = a.service.ListAssetPacks(r.ID, a.instTab)
			default:
				a.addonTab = "mods"
				a.mods, _ = a.service.ListMods(r.ID)
			}
			if a.instTab == "logs" {
				if ch, e := a.service.LogTail(r.ID, r.SelectedVersion, logPageLines, -1); e == nil {
					a.logView = logViewState{key: "file:" + ch.Path, file: ch.Path, lines: parseLogLines(ch.Lines, ""), start: ch.Start, path: ch.Path, follow: true, lastPoll: time.Now().Add(time.Hour)}
				}
			}
			if st, e := a.service.InstanceStatus(r.ID, r.SelectedVersion); e == nil {
				a.instStatus, a.instStatusKey = st, r.ID+"|"+r.SelectedVersion
				// Static exports must wait for this one thumbnail; live views load
				// it asynchronously through localIcon.
				if shot := st.LatestShot; shot != nil && (a.instTab == "" || a.instTab == "overview") {
					if b, e := a.service.ScreenshotPreview(r.ID, r.SelectedVersion, shot.File); e == nil {
						if img, e := decodeResourceImage(b); e == nil {
							a.imageMu.Lock()
							a.images[overviewShotKey(*r, *shot)] = img
							a.imageMu.Unlock()
						}
					}
				}
			}
			if a.instTab == "settings" {
				a.loadVersionConfigForm(r.SelectedVersion)
			}
		}
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
