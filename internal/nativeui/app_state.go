package nativeui

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// App owns the native window state and marshals background results onto the
// UI thread. Use New to initialize services, queues and cancellation.
type App struct {
	// Window lifecycle. App is never copied; locks guard the documented worker boundaries.
	service        *launcher.Service
	win            *mygo.Window
	winMu          sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	bgWG           sync.WaitGroup // background work joined during shutdown
	firstFrame     chan struct{}
	firstFrameOnce sync.Once
	events         chan func()

	// Navigation and dialogs. Unless stated otherwise, state belongs to the UI thread.
	page                   string
	prevPage               string
	pageDir, accountTabDir float32
	dialog                 string
	dialogOpen             bool
	dialogGen              uint64
	dialogShown            string
	form                   formState
	confirmRemove          string
	lastScale              float32
	accountMenuOpen        bool

	// Configuration snapshots and ordered background persistence.
	snapshot         launcher.Snapshot
	selectionEpoch   atomic.Uint64
	selectionMu      sync.Mutex
	selectionPending bool
	settings         launcher.Settings
	systemLanguage   string
	systemLanguages  func() []string
	languageOverride string
	settingsRevision atomic.Uint64
	settingsMu       sync.Mutex
	advancedOpen     bool
	memory           float64
	memoryText       string
	memoryInfo       launcher.MemoryInfo
	memoryError      string
	verMemSlider     float64

	// Java discovery and installation.
	java             []launcher.JavaInfo
	javaScanning     bool
	javaDeep         bool
	javaScanned      int
	javaElapsed      int64
	javaPhase        string
	javaScanCancel   context.CancelFunc
	javaScanRevision int
	javaFilter       string
	javaScannedOnce  bool
	javaReadyToScan  bool
	javaGet          javaGetState

	// Instances, add-on lists and their asynchronous caches.
	addonTab                                    string
	instTab, instFilter, instLoaded, instLoader string
	instTabMotion                               instanceTabMotion
	instScrollPending                           bool
	instRows                                    ui.ListState
	pending                                     map[string]int
	pendingRequests                             map[string]pendingRequest
	modsGen                                     int
	modsCancel                                  context.CancelFunc
	modCheckCancel                              func()
	modChecking                                 bool
	addonCache                                  map[string]addonCacheEntry
	addonOrder                                  []string
	instListBounds                              ui.Rect
	tabViewBounds, modListBounds                ui.Rect
	embedMods                                   bool
	mods                                        []launcher.Mod
	assetPacks                                  []launcher.AssetPack
	modFilter                                   string
	addonFilterKind, addonSortKey               string
	addonFilterMotion                           addonFilterMotion
	toggleMu                                    sync.Mutex
	modDisable                                  modDisableState
	lastToggle                                  time.Duration
	modIconAsked                                map[string]bool
	modIconURLs                                 map[string]string
	addonRemote                                 map[string]launcher.RemoteInfo // sha1 (mods) or kind|file (packs)
	packRemoteAsked                             map[string]bool
	zhNames                                     *launcher.ZhNames
	zhAsked                                     bool
	addonListMode                               bool
	addonGridW                                  map[string]float32
	gameStates                                  map[string]launcher.GameOptions
	shaderMods                                  map[string][]launcher.Mod
	gameStateAsked                              map[string]bool
	modsLoading                                 bool

	// Overview, worlds and live logs.
	instStatus                    launcher.InstanceStatus
	instStatusKey                 string
	statusCancel                  context.CancelFunc
	statusGen                     int
	statusCache                   map[string]launcher.InstanceStatus
	statusLoading                 bool
	instStatusErr                 string
	overviewDisplay               overviewPresentation
	overviewHolding               bool
	overviewImageSlots            chan struct{}
	recentPing                    recentServerPing
	wv                            worldView
	worldListGen, worldBackupsGen uint64
	logView                       logViewState
	logSess                       logSessions
	launchAgain                   launchAgainState
	logFilters                    map[string]logFilter
	logText                       string

	// Resource discovery, version selection and dependency resolution.
	provider, resourceType, query, filterMC, filterLoader string
	discoverMCText                                        string
	discoverMCOpen                                        bool
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
	versionList                                           projectVersionListState
	projectVersionFilter                                  string
	projShowAll, projLoading                              bool
	projectFetch                                          projectVersionRequest
	projectMCChoice                                       projectChoiceState
	plan                                                  []launcher.PlanItem
	planKey, planErr, planJob, planReplace                string
	planLoading, planFinished                             bool
	planErrUI                                             uiError
	planOff                                               map[string]bool // explicit check state, true = unchecked
	planCancel                                            context.CancelFunc
	planRevision                                          uint64
	resolvePlan                                           resolveInstallFunc
	catFailed                                             map[string]bool
	catSel                                                map[string][]string // source|type -> selected category ids
	catOpts                                               map[string][]launcher.Category
	catAsked                                              map[string]bool
	packRemote                                            map[string]*launcher.PackPreview
	packPreview                                           *launcher.PackPreview

	// Game installation and modpack updates.
	minecraftVersions         []launcher.MinecraftVersion
	versionKind, versionQuery string
	manifestLoading           bool
	manifestError             string
	manifestRevision          int
	loaders                   []string
	loaderLoading             bool
	loaderError               string
	loaderRevision            int
	packChecked               map[string]bool
	packErr                   string
	packPlan                  *launcher.PackUpdatePlan
	packPlanReport            bool
	packExpand                map[string]bool
	packDropPath              string
	packReport                *launcher.PackReport

	// Account login, appearance and wardrobe previews.
	accountTab                  string
	serverCustom                bool
	serverEdit                  *authServerEditor
	device                      *launcher.DeviceInfo
	deviceUntil, nextDevicePoll time.Time
	polling                     bool
	profiles                    []launcher.ThirdProfile
	appearance                  launcher.Appearance
	avatars                     avatarCache
	appearanceAccount           string
	appearanceLoading           bool
	appearanceErr               string
	preview                     *skinPreview
	model, selectedCape         string
	wardrobeSt                  *wardrobeState

	// Shared feedback, motion and release checks.
	message              string
	noticeKind           string // "", "error", "warning", "info"
	noticeRetry          func()
	noticeFor            string // message the retry belongs to
	noticeShown          string
	noticeShownKind      string
	noticeCopied         bool
	toastHeld            bool
	toastGen             int
	toastLifeKey         string
	errorMessage         bool
	contentSwaps         map[string]*contentSwapState
	numberRolls          map[string]*numberRollState
	launcherUpdateOnce   sync.Once
	launcherUpdate       launcherUpdateState
	checkLauncherRelease func(context.Context) (*launcherRelease, error)

	// Remote image cache. imageMu protects lookup state shared with workers.
	imageMu      sync.Mutex
	images       map[string]ui.ImageSource
	imagePending map[string]bool
	imageRetry   map[string]time.Time
	imageSlots   chan struct{}
}
