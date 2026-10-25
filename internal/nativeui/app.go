package nativeui

import (
	"bytes"
	"context"
	"crypto/sha256"
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
		pollWake:           make(chan struct{}, 1),
		inventoryWake:      make(chan struct{}, 1),
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
		imageSlots:         make(chan struct{}, 2),
		overviewImageSlots: make(chan struct{}, 1),
	}
	a.snapshot = s.Snapshot()
	a.restoreLauncherUpdatePolicy()
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

// HardwareAcceleration reports the saved Windows renderer preference. A change
// takes effect on the next launch, before either renderer acquires resources.
func (a *App) HardwareAcceleration() bool { return a.settings.HardwareAcceleration }

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
	a.updateUI = win.Update
	a.winMu.Unlock()
	visibilityChanged := func() {
		background := !win.IsVisible() || win.IsMinimized()
		a.background.Store(background)
		a.wakeSnapshots()
		if !background {
			signalWorker(a.inventoryWake)
		}
	}
	win.OnShow(visibilityChanged)
	win.OnHide(visibilityChanged)
	win.OnMinimize(visibilityChanged)
	win.OnRestore(visibilityChanged)
	visibilityChanged()
	win.OnReadyToShow(func() {
		a.firstFrameOnce.Do(func() { close(a.firstFrame) })
		a.startBackgroundJava()
		a.startLauncherUpdates()
		go a.service.Maintain(a.ctx)
		a.bgWG.Add(1)
		go a.refreshInventories()
		go a.service.AutoCheckPackUpdates(a.ctx) // 24h throttle, silent on network failure
	})
	win.OnClosed(func() { a.cancel() })
	systemLanguages := a.systemLanguages
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		a.pollSnapshots(systemLanguages)
	}()
}

// Keep only a digest in the polling worker: the UI owns the delivered snapshot
// and can mutate its selection. Comparing a shared previous snapshot would race
// with clicks. JSON also gives nested maps a stable ordering.
type snapshotObserver struct {
	digest    [sha256.Size]byte
	buffer    bytes.Buffer
	encoder   *json.Encoder
	instances []launcher.Instance
	packs     []*launcher.PackLock
	updates   []*launcher.PackUpdateInfo
	epoch     uint64
	seen      bool
}

func (o *snapshotObserver) changed(snap launcher.Snapshot, epoch uint64) bool {
	o.buffer.Reset()
	if o.encoder == nil {
		o.encoder = json.NewEncoder(&o.buffer)
	}
	// Pack locks and update results are immutable inventory publications that
	// can carry thousands of file records and long changelogs. A rescan
	// publishes new pointers, so compare their identity instead of encoding
	// the same payload on every idle poll. Holding the previous pointers keeps
	// a freed address from being mistaken for the old publication.
	packsChanged := len(o.packs) != len(snap.Instances)
	if !packsChanged {
		for i, in := range snap.Instances {
			if o.packs[i] != in.Pack || o.updates[i] != in.PackUpdate {
				packsChanged = true
				break
			}
		}
	}
	o.instances = append(o.instances[:0], snap.Instances...)
	// Clearing before reslicing also drops references in the unused tail
	// when roots are removed from the next snapshot.
	clear(o.packs)
	clear(o.updates)
	o.packs, o.updates = o.packs[:0], o.updates[:0]
	for i := range o.instances {
		in := &o.instances[i]
		o.packs, o.updates = append(o.packs, in.Pack), append(o.updates, in.PackUpdate)
		in.Pack, in.PackUpdate = nil, nil
	}
	snap.Instances = o.instances
	err := o.encoder.Encode(snap)
	clear(o.instances) // do not retain the previous snapshot between polls
	if err != nil {
		// A future non-JSON field must not suppress a UI update.
		return true
	}
	digest := sha256.Sum256(o.buffer.Bytes())
	changed := !o.seen || packsChanged || digest != o.digest || epoch != o.epoch
	o.digest, o.epoch, o.seen = digest, epoch, true
	return changed
}

func (a *App) post(fn func()) {
	select {
	case a.events <- fn:
		a.wakeSnapshots()
		a.requestUIUpdate()
	case <-a.ctx.Done():
	}
}

func (a *App) requestUIUpdate() {
	if a.ctx.Err() != nil {
		return
	}
	a.winMu.RLock()
	update := a.updateUI
	a.winMu.RUnlock()
	if update != nil && a.uiUpdatePending.CompareAndSwap(false, true) {
		// Startup images and scan results can arrive in a burst. One callback
		// applies the whole queue, avoiding redundant native redraw requests.
		// Keep using Update so hidden windows also process cleanup promptly.
		update(a.flushUIUpdates)
	}
}

func (a *App) flushUIUpdates() {
	a.drainEvents()
	a.uiUpdatePending.Store(false)
	// A worker may have posted after the drain, while the flag was still set.
	// Recheck after releasing it; concurrent posters share the same CAS.
	if len(a.events) > 0 {
		a.requestUIUpdate()
	}
}

func (a *App) drainEvents() {
	for {
		if a.ctx != nil && a.ctx.Err() != nil {
			return
		}
		select {
		case fn := <-a.events:
			fn()
		default:
			return
		}
	}
}

func decode[T any](b json.RawMessage) T { var v T; _ = json.Unmarshal(b, &v); return v }

func (a *App) notice(s string) {
	a.message, a.errorMessage, a.toastUntil = s, false, time.Time{}
}

func (a *App) goPage(page string) {
	if a.launcherUpdateRequired() {
		return
	}
	a.wakeSnapshots()
	if page != a.page {
		a.addonListData = addonListData{}
		a.resetImageRequests()
		if page != "home" && page != "mods" {
			a.cancelAddonLoad()
			a.cancelInstanceStatus()
		}
	}
	if a.page == "wardrobe" && page != a.page && a.preview != nil {
		a.preview.releaseScratch()
	}
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
	a.scheduleDiscoverRelease(page)
	a.page = page
	switch page {
	case "home":
		if a.instTab == "" || a.instTab == "overview" {
			a.instTab = "overview"
			a.loadInstanceStatus()
		} else {
			a.setInstTab(a.instTab)
		}
	case "discover":
		if len(a.minecraftVersions) == 0 && !a.manifestLoading && a.manifestError == "" {
			a.loadManifest()
		}
		if len(a.searchResult.Projects) == 0 {
			q := a.discoverQuery(0)
			cached := !a.searchLoading && a.discoverMotion.hasQuery && a.searchError == "" && sameDiscoverQuery(q, a.discoverMotion.query)
			pending := a.searchLoading && sameDiscoverQuery(q, a.discoverMotion.request)
			if !cached && !pending {
				a.search()
			}
		}
	case "wardrobe":
		a.loadAppearance()
	case "settings":
		a.refreshMemory()
		a.ensureJavaScan()
		a.refreshModCacheUsage()
	}
}
