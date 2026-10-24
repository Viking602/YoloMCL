package nativeui

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type formState struct {
	Name, Path, Parent, Minecraft, Loader, LoaderVersion, Target, VersionID, Server, Username, Password, Profile string
	Java, Memory, JVM                                                                                            string
	MemAuto, VerMemAuto                                                                                          bool
	VerName                                                                                                      string
	ConfigVersion, VerJava, VerMemory, VerJVM                                                                    string
}

func New(s *launcher.Service) (*App, error) {
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{
		service:            s,
		ctx:                ctx,
		cancel:             cancel,
		events:             make(chan func(), 256),
		page:               "home",
		addonTab:           "mods",
		provider:           "modrinth",
		resourceType:       "modpack",
		filterLoader:       "all",
		accountTab:         "microsoft",
		model:              "default",
		images:             map[string]ui.ImageSource{},
		imagePending:       map[string]bool{},
		imageRetry:         map[string]time.Time{},
		imageSlots:         make(chan struct{}, 6),
		overviewImageSlots: make(chan struct{}, 2),
	}
	a.snapshot = s.Snapshot()
	a.settings = a.snapshot.Settings
	a.settings.Language = launcher.NormalizeLanguage(a.settings.Language)
	a.normalizeStateValues()
	a.memory = float64(a.settings.MemoryMB)
	a.instLoader = a.settings.InstanceLoaderFilter
	a.memoryText = fmt.Sprint(a.settings.MemoryMB)
	a.refreshMemory()
	a.preview = newSkinPreview()
	if e := loadAssets(); e != nil {
		cancel()
		return nil, e
	}
	return a, nil
}

func (a *App) SetLanguage(lang string) {
	a.setLanguage(lang)
}

// OverrideLanguage applies the -lang flag for this process only; unlike
// SetLanguage it never writes the user's saved settings.
func (a *App) OverrideLanguage(lang string) {
	a.languageOverride = launcher.NormalizeLanguage(lang)
	a.normalizeStateValues()
}

func (a *App) Attach(win *mygo.Window) {
	if a.firstFrame == nil {
		a.firstFrame = make(chan struct{})
	}
	a.winMu.Lock()
	a.win = win
	a.winMu.Unlock()
	win.OnReadyToShow(func() {
		a.firstFrameOnce.Do(func() { close(a.firstFrame) })
		a.startBackgroundJava()
		a.startLauncherUpdates()
		go a.service.Maintain(a.ctx)
		a.bgWG.Add(1)
		go func() {
			defer a.bgWG.Done()
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				a.service.RefreshInventories(a.ctx)
				select {
				case <-a.ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
		go a.service.AutoCheckPackUpdates(a.ctx) // 24h throttle, silent on network failure
	})
	win.OnClosed(func() { a.cancel() })
	go func() {
		ticker := time.NewTicker(450 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				epoch := a.selectionEpoch.Load()
				snap := a.service.Snapshot()
				a.post(func() {
					old := jobSignature(a.snapshot.Jobs)
					a.applyJobNotice(a.snapshot.Jobs, snap.Jobs)
					a.applySnapshot(snap, epoch)
					if old != jobSignature(snap.Jobs) && a.page == "mods" {
						a.loadMods()
					}
				})
			}
		}
	}()
}

func (a *App) post(fn func()) {
	select {
	case a.events <- fn:
		a.winMu.RLock()
		win := a.win
		a.winMu.RUnlock()
		if win != nil {
			win.Update(func() {})
		}
	case <-a.ctx.Done():
	}
}

func decode[T any](b json.RawMessage) T { var v T; _ = json.Unmarshal(b, &v); return v }

func (a *App) notice(s string) { a.message = s; a.errorMessage = false }

func (a *App) goPage(page string) {
	if page == "settings" && a.page != "settings" {
		a.prevPage = a.page
	}
	if d := navIndex(page) - navIndex(a.page); d != 0 {
		a.pageDir = float32(d / max(d, -d))
	}
	if page == "mods" { // 模组 is a per-instance tab now
		page = "home"
		a.instTab = a.addonTab
		if a.instTab == "" {
			a.instTab = "mods"
		}
	}
	if page == "discover" && a.page != page {
		a.discoverMotion.categories = discoverCategoryMotion{}
	}
	a.page = page
	switch page {
	case "home":
		a.loadMods()
	case "discover":
		if len(a.minecraftVersions) == 0 && !a.manifestLoading && a.manifestError == "" {
			a.loadManifest()
		}
		if len(a.searchResult.Projects) == 0 {
			a.search()
		}
	case "wardrobe":
		a.loadAppearance()
	case "settings":
		a.refreshMemory()
		a.ensureJavaScan()
	}
}
