package nativeui

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// App owns the native window's state and marshals background results onto the
// UI thread. Use New to initialize its services, queues and cancellation scope.
type App struct {
	advancedOpen                                          bool
	service                                               *launcher.Service
	win                                                   *mygo.Window
	winMu                                                 sync.RWMutex
	ctx                                                   context.Context
	cancel                                                context.CancelFunc
	bgWG                                                  sync.WaitGroup // background icon lookups
	firstFrame                                            chan struct{}
	firstFrameOnce                                        sync.Once
	launcherUpdateOnce                                    sync.Once
	launcherUpdate                                        launcherUpdateState
	checkLauncherRelease                                  func(context.Context) (*launcherRelease, error)
	events                                                chan func()
	page                                                  string
	snapshot                                              launcher.Snapshot
	selectionEpoch                                        atomic.Uint64
	selectionMu                                           sync.Mutex
	selectionPending                                      bool
	settings                                              launcher.Settings
	systemLanguage                                        string
	systemLanguages                                       func() []string
	languageOverride                                      string
	settingsRevision                                      atomic.Uint64
	settingsMu                                            sync.Mutex
	memory                                                float64
	memoryText                                            string
	memoryInfo                                            launcher.MemoryInfo
	memoryError                                           string
	javaScanning                                          bool
	javaDeep                                              bool
	javaScanned                                           int
	javaElapsed                                           int64
	javaPhase                                             string
	javaScanCancel                                        context.CancelFunc
	javaScanRevision                                      int
	javaFilter                                            string
	javaScannedOnce                                       bool
	javaReadyToScan                                       bool
	message                                               string
	noticeKind                                            string // "", "error", "warning", "info"
	noticeRetry                                           func()
	noticeFor                                             string // message the retry belongs to
	noticeShown                                           string
	noticeShownKind                                       string
	noticeCopied                                          bool
	appearanceErr                                         string
	confirmRemove                                         string
	wv                                                    worldView
	worldListGen, worldBackupsGen                         uint64
	packChecked                                           map[string]bool
	toastHeld                                             bool
	toastGen                                              int
	toastLifeKey                                          string
	errorMessage                                          bool
	dialog                                                string
	dialogOpen                                            bool
	dialogGen                                             uint64
	pageDir, accountTabDir                                float32
	dialogShown                                           string
	packErr                                               string
	packPlan                                              *launcher.PackUpdatePlan
	packPlanReport                                        bool
	packExpand                                            map[string]bool
	packDropPath                                          string
	prevPage                                              string
	form                                                  formState
	addonTab                                              string
	instTab, instFilter, instLoaded, instLoader           string
	instScrollPending                                     bool
	instRows                                              ui.ListState
	pending                                               map[string]int
	pendingRequests                                       map[string]pendingRequest
	modsGen                                               int
	modsCancel                                            context.CancelFunc
	modCheckCancel                                        func()
	modChecking                                           bool
	addonCache                                            map[string]addonCacheEntry
	addonOrder                                            []string
	instStatus                                            launcher.InstanceStatus
	instStatusKey                                         string
	statusCancel                                          context.CancelFunc
	statusGen                                             int
	statusCache                                           map[string]launcher.InstanceStatus
	statusLoading                                         bool
	instStatusErr                                         string
	overviewDisplay                                       overviewPresentation
	overviewHolding                                       bool
	contentSwaps                                          map[string]*contentSwapState
	numberFlips                                           map[string]*numberFlipState
	overviewImageSlots                                    chan struct{}
	recentPing                                            recentServerPing
	logView                                               logViewState
	logSess                                               logSessions
	launchAgain                                           launchAgainState
	logFilters                                            map[string]logFilter
	accountMenuOpen                                       bool
	javaGet                                               javaGetState
	lastScale                                             float32
	instListBounds                                        ui.Rect
	tabViewBounds, modListBounds                          ui.Rect
	embedMods                                             bool
	mods                                                  []launcher.Mod
	assetPacks                                            []launcher.AssetPack
	modFilter                                             string
	addonFilterKind, addonSortKey                         string
	verMemSlider                                          float64
	toggleMu                                              sync.Mutex
	lastToggle                                            time.Duration
	modIconAsked                                          map[string]bool
	modIconURLs                                           map[string]string
	addonRemote                                           map[string]launcher.RemoteInfo // sha1 (mods) or kind|file (packs)
	packRemoteAsked                                       map[string]bool
	zhNames                                               *launcher.ZhNames
	zhAsked                                               bool
	addonListMode                                         bool
	projShowAll, projLoading                              bool
	plan                                                  []launcher.PlanItem
	planKey, planErr, planJob, planReplace                string
	planLoading, planFinished                             bool
	planErrUI                                             uiError
	catFailed                                             map[string]bool
	catSel                                                map[string][]string // source|type -> selected category ids
	catOpts                                               map[string][]launcher.Category
	catAsked                                              map[string]bool
	planOff                                               map[string]bool // explicit check state, true = unchecked
	planCancel                                            context.CancelFunc
	addonGridW                                            map[string]float32
	gameStates                                            map[string]instGameState
	shaderMods                                            map[string][]launcher.Mod
	gameStateAsked                                        map[string]bool
	modsLoading                                           bool
	provider, resourceType, query, filterMC, filterLoader string
	searchResult                                          launcher.SearchResult
	searchLoading                                         bool
	searchError                                           string
	searchRevision                                        int
	searchSort                                            string
	discoverMotion                                        discoverMotionState
	discoverFeed                                          discoverFeedState
	project                                               *launcher.Project
	projectVersions                                       []launcher.ProjectVersion
	verFilter                                             verFilter
	packRemote                                            map[string]*launcher.PackPreview
	projectVersionFilter                                  string
	packPreview                                           *launcher.PackPreview
	minecraftVersions                                     []launcher.MinecraftVersion
	versionKind, versionQuery                             string
	manifestLoading                                       bool
	manifestError                                         string
	manifestRevision                                      int
	loaders                                               []string
	loaderLoading                                         bool
	loaderError                                           string
	loaderRevision                                        int
	accountTab                                            string
	accountAdding                                         bool
	serverCustom                                          bool
	serverEdit                                            *authServerEditor
	device                                                *launcher.DeviceInfo
	deviceUntil, nextDevicePoll                           time.Time
	polling                                               bool
	profiles                                              []launcher.ThirdProfile
	appearance                                            launcher.Appearance
	avatars                                               avatarCache
	appearanceAccount                                     string
	appearanceLoading                                     bool
	preview                                               *skinPreview
	model, selectedCape                                   string
	wardrobeSt                                            *wardrobeState
	packReport                                            *launcher.PackReport
	logText                                               string
	java                                                  []launcher.JavaInfo
	imageMu                                               sync.Mutex
	images                                                map[string]ui.ImageSource
	imagePending                                          map[string]bool
	imageRetry                                            map[string]time.Time
	imageSlots                                            chan struct{}
}

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
