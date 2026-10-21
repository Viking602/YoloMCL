package nativeui

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"mcl/internal/launcher"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type App struct {
	advancedOpen                                          bool
	service                                               *launcher.Service
	win                                                   *mygo.Window
	winMu                                                 sync.RWMutex
	ctx                                                   context.Context
	cancel                                                context.CancelFunc
	events                                                chan func()
	page                                                  string
	snapshot                                              launcher.Snapshot
	settings                                              launcher.Settings
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
	busy                                                  bool
	message                                               string
	noticeKind                                            string // "", "error", "warning", "info"
	noticeRetry                                           func()
	noticeFor                                             string // message the retry belongs to
	noticeShown                                           string
	noticeShownKind                                       string
	noticeCopied                                          bool
	appearanceErr                                         string
	confirmRemove                                         string
	toastHeld                                             bool
	toastGen                                              int
	toastLifeKey                                          string
	errorMessage                                          bool
	dialog                                                string
	dialogOpen                                            bool
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
	mods                                                  []launcher.Mod
	assetPacks                                            []launcher.AssetPack
	modFilter                                             string
	modsLoading                                           bool
	provider, resourceType, query, filterMC, filterLoader string
	searchResult                                          launcher.SearchResult
	searchLoading                                         bool
	searchError                                           string
	searchRevision                                        int
	searchSort                                            string
	project                                               *launcher.Project
	projectVersions                                       []launcher.ProjectVersion
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
	if e := loadAssets(); e != nil {
		return nil, e
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{
		service:      s,
		ctx:          ctx,
		cancel:       cancel,
		events:       make(chan func(), 256),
		page:         "home",
		addonTab:     "mods",
		provider:     "modrinth",
		resourceType: "modpack",
		filterLoader: "all",
		accountTab:   "microsoft",
		model:        "default",
		images:       map[string]ui.ImageSource{},
		imagePending: map[string]bool{},
		imageRetry:   map[string]time.Time{},
		imageSlots:   make(chan struct{}, 6),
	}
	a.snapshot = s.Snapshot()
	a.settings = a.snapshot.Settings
	a.settings.Language = launcher.NormalizeLanguage(a.settings.Language)
	a.normalizeStateValues()
	a.memory = float64(a.settings.MemoryMB)
	a.memoryText = fmt.Sprint(a.settings.MemoryMB)
	a.refreshMemory()
	a.java = s.CachedJavaList()
	a.preview = newSkinPreview()
	return a, nil
}

func (a *App) SetLanguage(lang string) {
	a.setLanguage(lang)
}

func (a *App) Attach(win *mygo.Window) {
	a.winMu.Lock()
	a.win = win
	a.winMu.Unlock()
	a.startJavaScan(false)
	go a.service.AutoCheckPackUpdates(a.ctx) // 24h throttle, silent on network failure
	win.OnClosed(func() { a.cancel() })
	go func() {
		ticker := time.NewTicker(450 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				snap := a.service.Snapshot()
				a.post(func() {
					old := jobSignature(a.snapshot.Jobs)
					a.applyJobNotice(a.snapshot.Jobs, snap.Jobs)
					a.snapshot = snap
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

func (a *App) execute(method string, payload any, done func(json.RawMessage)) {
	if a.busy {
		return
	}
	a.busy = true
	a.message = ""
	go func() {
		b, e := json.Marshal(payload)
		var raw string
		if e == nil {
			raw, e = a.service.Dispatch(a.ctx, method, string(b))
		}
		snap := a.service.Snapshot()
		a.post(func() {
			a.busy = false
			a.snapshot = snap
			if e != nil {
				a.message = e.Error()
				a.errorMessage = true
				a.noticeFor, a.noticeRetry = a.message, func() { a.execute(method, payload, done) }
				return
			}
			if done != nil {
				done(json.RawMessage(raw))
			}
		})
	}()
}

func decode[T any](b json.RawMessage) T { var v T; _ = json.Unmarshal(b, &v); return v }
func (a *App) notice(s string)          { a.message = s; a.errorMessage = false }

func jobSignature(jobs []launcher.Job) string {
	var b strings.Builder
	for _, j := range jobs {
		b.WriteString(fmt.Sprintf("%s:%s:%d:%s|", j.ID, j.State, j.Done, j.Message))
	}
	return b.String()
}

func (a *App) root() *launcher.Instance {
	for i := range a.snapshot.Instances {
		if a.snapshot.Instances[i].ID == a.snapshot.ActiveRoot {
			return &a.snapshot.Instances[i]
		}
	}
	return nil
}

func (a *App) account() *launcher.Account {
	for i := range a.snapshot.Accounts {
		if a.snapshot.Accounts[i].ID == a.snapshot.ActiveAccount {
			return &a.snapshot.Accounts[i]
		}
	}
	return nil
}

func (a *App) gameVersion() *launcher.VersionSummary {
	if r := a.root(); r != nil {
		for i := range r.Versions {
			if r.Versions[i].ID == r.SelectedVersion {
				return &r.Versions[i]
			}
		}
	}
	return nil
}

func (a *App) goPage(page string) {
	if page == "settings" && a.page != "settings" {
		a.prevPage = a.page
	}
	if d := navIndex(page) - navIndex(a.page); d != 0 {
		a.pageDir = float32(d / max(d, -d))
	}
	a.page = page
	switch page {
	case "mods":
		a.loadMods()
	case "discover":
		if len(a.searchResult.Projects) == 0 {
			a.search()
		}
	case "wardrobe":
		a.loadAppearance()
	case "settings":
		a.refreshMemory()
		if !a.javaScannedOnce {
			a.startJavaScan(false)
		}
	}
}

func (a *App) selectRoot(id string) {
	a.execute("select", map[string]string{"rootId": id}, func(json.RawMessage) { a.loadMods() })
}

func (a *App) selectVersion(rootID, versionID string) {
	a.execute("select", map[string]string{"rootId": rootID, "version": versionID}, func(json.RawMessage) { a.loadMods() })
}

func (a *App) loadVersionConfigForm(versionID string) {
	a.form.ConfigVersion = versionID
	a.form.VerJava = ""
	a.form.VerMemory = "0"
	a.form.VerJVM = ""
	a.form.VerName = ""
	a.form.VerMemAuto = true
	if r := a.root(); r != nil {
		a.form.VerMemAuto, _ = r.MemoryPlan(versionID, a.settings)
		if !a.form.VerMemAuto {
			_, mb := r.MemoryPlan(versionID, a.settings)
			a.form.VerMemory = fmt.Sprint(mb)
		}
	}
	if r := a.root(); r != nil && r.VersionConfigs != nil {
		if vc, ok := r.VersionConfigs[versionID]; ok {
			a.form.VerJava = vc.JavaPath
			if vc.MemoryMB > 0 {
				a.form.VerMemory = fmt.Sprint(vc.MemoryMB)
			}
			a.form.VerJVM = vc.JVMArgs
			a.form.VerName = vc.DisplayName
		}
	}
}

func (a *App) openVersionConfig(rootID, versionID string) {
	if rootID != "" && rootID != a.snapshot.ActiveRoot {
		_ = a.service.Select(rootID, "", versionID)
		a.snapshot = a.service.Snapshot()
	}
	a.openDialog("version")
	if versionID != "" {
		a.loadVersionConfigForm(versionID)
	}
}

func (a *App) inspectPackFile(path string) {
	a.packPreview = nil
	a.packErr = ""
	a.form.Path = path
	if _, err := a.service.InspectPack(path); err != nil {
		a.packErr = err.Error()
	}
	if preview, err := a.service.InspectPack(path); err == nil {
		a.packPreview = &preview
		if a.form.Name == "" {
			name := preview.Name
			if preview.Version != "" && !strings.Contains(name, preview.Version) {
				name += "-" + preview.Version
			}
			a.form.Name = cleanName(name)
		}
	}
	if a.form.Name == "" {
		a.form.Name = cleanName(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	}
	if a.form.Parent == "" {
		if r := a.root(); r != nil {
			p := r.Path
			if strings.EqualFold(filepath.Base(p), ".minecraft") {
				p = filepath.Dir(p)
			}
			a.form.Parent = filepath.Dir(p)
		} else {
			a.form.Parent = filepath.Dir(path)
		}
	}
}

func (a *App) handleDroppedFiles(paths []string) {
	if len(paths) == 0 || a.busy {
		return
	}
	first := paths[0]
	if st, err := os.Stat(first); err == nil && st.IsDir() {
		a.openDialog("add")
		a.form.Path = first
		return
	}
	ext := strings.ToLower(filepath.Ext(first))
	switch ext {
	case ".mrpack":
		if a.offerPackUpdate(first) {
			return
		}
		a.openDialog("import")
		a.inspectPackFile(first)
	case ".zip":
		kind, err := a.service.ArchiveKind(first)
		if err != nil {
			a.message, a.errorMessage = err.Error(), true // never guess
			return
		}
		if kind == "modpack" {
			if a.offerPackUpdate(first) {
				return
			}
			a.openDialog("import")
			a.inspectPackFile(first)
			return
		}
		if r := a.root(); r == nil {
			a.message, a.errorMessage = a.tr().DropNeedRoot, true
			return
		} else {
			a.execute("addLocalAssetPacks", map[string]any{"rootId": r.ID, "kind": kind, "paths": paths}, func(json.RawMessage) {
				a.addonTab = kind
				a.goPage("mods")
				a.loadMods()
			})
			return
		}
	case ".jar":
		if r := a.root(); r != nil {
			var jars []string
			for _, p := range paths {
				if strings.EqualFold(filepath.Ext(p), ".jar") {
					jars = append(jars, p)
				}
			}
			if len(jars) > 0 {
				a.execute("addLocalMods", map[string]any{"rootId": r.ID, "paths": jars}, func(json.RawMessage) {
					a.addonTab = "mods"
					a.goPage("mods")
					a.loadMods()
				})
			}
		} else {
			a.openDialog("add")
		}
	}
}

func (a *App) openDialog(name string) {
	t := a.tr()
	a.dialog = name
	a.dialogOpen = true
	a.packPreview = nil
	a.form = formState{Loader: t.LoaderVanilla, Target: t.InstallTargetNew, Memory: "0", VerMemory: "0"}
	a.message = ""
	switch name {
	case "install":
		if a.root() != nil {
			a.form.Target = t.InstallTargetCurrent
		}
		a.versionQuery = ""
		a.versionKind = t.InstallKindRelease
		if a.settings.ShowSnapshots {
			a.versionKind = t.InstallKindAll
		}
		a.loadManifest()
	case "import":
		if r := a.root(); r != nil {
			p := r.Path
			if strings.EqualFold(filepath.Base(p), ".minecraft") {
				p = filepath.Dir(p)
			}
			a.form.Parent = filepath.Dir(p)
		}
	case "root":
		if !a.javaScannedOnce && !a.javaScanning {
			a.startJavaScan(false)
		}
		if r := a.root(); r != nil {
			a.form.Name = r.Name
			a.form.Java = r.JavaPath
			a.form.Memory = fmt.Sprint(r.MemoryMB)
			a.form.MemAuto = r.MemoryMB <= 0
			if a.form.MemAuto {
				a.form.Memory = "4096"
			}
			a.form.JVM = r.JVMArgs
			ver := r.SelectedVersion
			if ver == "" && len(r.Versions) > 0 {
				ver = r.Versions[0].ID
			}
			a.loadVersionConfigForm(ver)
		}
	case "account":
		a.accountAdding, a.serverEdit, a.serverCustom = false, nil, false
	case "version":
		if !a.javaScannedOnce && !a.javaScanning {
			a.startJavaScan(false)
		}
		if r := a.root(); r != nil {
			ver := r.SelectedVersion
			if ver == "" && len(r.Versions) > 0 {
				ver = r.Versions[0].ID
			}
			a.loadVersionConfigForm(ver)
		}
	case "logs":
		a.loadLogs()
	}
}

func (a *App) closeDialog() {
	if a.device != nil {
		a.service.CancelMicrosoft(a.device.ID)
	}
	a.device = nil
	a.profiles = nil
	a.dialog = ""
	a.dialogOpen = false
	a.form.Password = ""
}

func (a *App) executeDialog(fn func() error) {
	if a.win == nil {
		return
	}
	go func() {
		if e := fn(); e != nil {
			a.post(func() { a.message = e.Error(); a.errorMessage = true })
		}
	}()
}

func (a *App) chooseFolder(target *string) {
	t := a.tr()
	a.executeDialog(func() error {
		paths, e := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: t.DialogChooseFolder, Directory: true, ShowHiddenFiles: true})
		a.post(func() {
			if e == nil && len(paths) > 0 {
				*target = paths[0]
			}
		})
		return e
	})
}

func (a *App) chooseFiles(kind string, done func([]string)) {
	t := a.tr()
	filters := []mygo.FileFilter{{Name: t.DiscoverTypeModpack, Extensions: []string{"mrpack", "zip"}}}
	multiple := false
	switch kind {
	case "mods":
		filters = []mygo.FileFilter{{Name: t.DiscoverTypeMod, Extensions: []string{"jar"}}}
		multiple = true
	case "zip":
		filters = []mygo.FileFilter{{Name: "ZIP Archive", Extensions: []string{"zip"}}}
		multiple = true
	case "png":
		filters = []mygo.FileFilter{{Name: "PNG", Extensions: []string{"png"}}}
	case "java":
		filters = []mygo.FileFilter{{Name: "Java", Extensions: []string{"exe"}}}
	}
	a.executeDialog(func() error {
		paths, e := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Filters: filters, Multiple: multiple, Title: t.DialogBrowseBtn})
		if e == nil && len(paths) > 0 {
			a.post(func() { done(paths) })
		}
		return e
	})
}

func (a *App) openFolder(id string) {
	p, e := a.service.RootPath(id)
	if e == nil {
		e = mygo.Shell.OpenPath(p)
	}
	if e != nil {
		a.message = e.Error()
		a.errorMessage = true
	}
}

func (a *App) launch() {
	r := a.root()
	if r == nil {
		a.openDialog("add")
		return
	}
	if a.account() == nil {
		a.openDialog("account")
		return
	}
	if r.SelectedVersion == "" {
		a.openDialog("install")
		return
	}
	a.execute("launch", map[string]string{"rootId": r.ID, "accountId": a.snapshot.ActiveAccount}, func(json.RawMessage) { a.notice(a.tr().InstallLaunchPreparing) })
}

func heading(c *ui.Context, _, text, description string, actions func()) {
	pageHeader(c, nil, text, description, actions)
}

// pageHeader is the one page header: optional breadcrumb, large title,
// muted subtitle, actions on the right — aligned to the content grid.
func pageHeader(c *ui.Context, crumb func(), text, description string, actions func()) {
	s := uiScale(c)
	ui.Row(c).Label("页面标题栏").FillWidth().Gap(space5 * s).AlignItems(ui.End).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Shrink(1).Clip().Gap(space1 * s).Children(func() {
			if crumb != nil {
				crumb()
			}
			ui.Text(c, text).Label("页面标题 " + text).FontSize(typeDisplay * s).FontWeight(750).TextColor(ink).SingleLine()
			if description != "" {
				ui.Text(c, description).FontSize(typeCallout * s).TextColor(muted).MinWidth(0) // wraps: descriptive text never ellipsizes
			}
		})
		if actions != nil {
			ui.Row(c).Shrink(0).Gap(space2 * s).AlignItems(ui.Center).Children(actions)
		}
	})
}

func empty(c *ui.Context, titleText, description string) {
	emptyState(c, "empty", "folder", titleText, description, nil)
}

// emptyState is the one placeholder used by every list: a frosted card with
// a tinted squircle, a headline, a muted explanation and optional actions.
func emptyState(c *ui.Context, key, icon, titleText, description string, actions func()) {
	s := uiScale(c)
	glass(ui.Column(c.Key(key)).FillWidth().Gap(space2*s).Padding(space8*s+space4*s, space8*s), radiusCard*s, s).AlignItems(ui.Center).Children(func() {
		appleSoftIconBox(c, 52*s, icon, accent, accentIndigo, panel)
		ui.Box(c).Height(space1 * s)
		ui.Text(c, titleText).FontSize(typeTitle * s).Bold().TextColor(ink).SingleLine()
		label(c, description).MaxWidth(560 * s).TextAlign(ui.Center)
		if actions != nil {
			ui.Row(c).Gap(space2 * s).PaddingY(space2 * s).AlignItems(ui.Center).Children(actions)
		}
	})
}

func accountKind(s string) string {
	switch s {
	case "microsoft":
		return "Microsoft 官方"
	case "yggdrasil":
		return "第三方账号"
	}
	return "离线账号"
}

func loaderName(s string) string {
	switch s {
	case "fabric":
		return "Fabric"
	case "quilt":
		return "Quilt"
	case "forge":
		return "Forge"
	case "neoforge":
		return "NeoForge"
	}
	return "原版"
}

func loaderKey(s string) string {
	v := strings.ToLower(strings.TrimSpace(s))
	switch v {
	case "", "vanilla", "原版", "all", "all loaders", "全部加载器":
		return "vanilla"
	case "fabric":
		return "fabric"
	case "quilt":
		return "quilt"
	case "forge":
		return "forge"
	case "neoforge":
		return "neoforge"
	}
	return v
}

func compact(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprint(n)
}

func sizeText(n int64) string { return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024)) }

func imageFromData(data string) image.Image {
	b, e := base64.StdEncoding.DecodeString(strings.TrimPrefix(data, "data:image/png;base64,"))
	if e != nil {
		return nil
	}
	img, _, _ := image.Decode(strings.NewReader(string(b)))
	return img
}

func (a *App) upload(kind string) {
	ac := a.account()
	if ac == nil {
		return
	}
	id := ac.ID
	a.chooseFiles("png", func(paths []string) {
		b, e := os.ReadFile(paths[0])
		if e != nil {
			a.message = e.Error()
			return
		}
		model := normalizeSkinModel(a.model)
		a.execute("uploadTexture", map[string]string{"accountId": id, "kind": kind, "model": model, "data": base64.StdEncoding.EncodeToString(b)}, func(json.RawMessage) {
			a.loadAppearance()
			a.notice(a.tr().WardrobeTextureSaved)
		})
	})
}

func cleanName(s string) string {
	for _, r := range `<>:"/\|?*` {
		s = strings.ReplaceAll(s, string(r), "-")
	}
	return strings.TrimSpace(s)
}

// loadingState is the shared in-progress placeholder: a frosted card whose
// icon and message are centered on both axes, so loading text never hugs a
// corner of its container.
func loadingState(c *ui.Context, key, text string) {
	s := uiScale(c)
	glass(ui.Column(c.Key(key)).Label(text).FillWidth().MinHeight(132*s).Padding(space6*s), radiusCard*s, s).AlignItems(ui.Center).Justify(ui.Center).Gap(space3 * s).Children(func() {
		statusSpinner(c, 22*s)
		ui.Text(c, text).FontSize(typeCallout * s).FontWeight(500).TextColor(muted).TextAlign(ui.Center).SingleLine()
	})
}

// statusLine centers a short status message inside a form or dialog body.
func statusLine(c *ui.Context, text string, color ui.Color, busy bool) {
	s := uiScale(c)
	ui.Row(c).FillWidth().MinHeight(44*s).Padding(space2*s, space4*s).Radius(radiusControl * s).Background(fieldColor.Alpha(.6)).AlignItems(ui.Center).Justify(ui.Center).Gap(space2 * s).Children(func() {
		if busy {
			statusSpinner(c, 16*s)
		}
		ui.Text(c, text).FontSize(typeCallout * s).FontWeight(500).TextColor(color).TextAlign(ui.Center).MinWidth(0).Shrink(1).SingleLine().Tooltip(text)
	})
}

// statusSpinner draws a ring with an accent arc that advances with the clock.
func statusSpinner(c *ui.Context, size float32) {
	phase := float32(time.Now().UnixMilli()%1200) / 1200
	ui.Box(c).Size(size, size).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		rad := min(r.W, r.H) / 2
		dots := 8
		for i := 0; i < dots; i++ {
			ang := (float64(i)/float64(dots) + float64(phase)) * 2 * math.Pi
			x := cx + float32(math.Cos(ang))*rad*.72
			y := cy + float32(math.Sin(ang))*rad*.72
			alpha := .15 + .85*float32(i)/float32(dots-1)
			d := rad * .32
			p.Fill(ui.Rect{X: x - d/2, Y: y - d/2, W: d, H: d}, accent.Alpha(alpha), d/2)
		}
	})
}

// applyJobNotice replaces the "任务已启动" notice once tasks finish: 导入完成
// for a successful import, the failure reason otherwise.
func (a *App) applyJobNotice(old, cur []launcher.Job) {
	t := a.tr()
	was := map[string]string{}
	for _, j := range old {
		was[j.ID] = j.State
	}
	for _, j := range cur {
		if was[j.ID] != "running" || j.State == "running" {
			continue
		}
		switch {
		case j.State == "completed" && strings.HasPrefix(j.Title, "更新整合包"):
			a.message, a.errorMessage = t.PackUpdateDone, false
		case j.State == "completed" && strings.HasPrefix(j.Title, "回滚整合包"):
			a.message, a.errorMessage = t.PackRollbackDone, false
		case j.State == "failed" && (strings.HasPrefix(j.Title, "更新整合包") || strings.HasPrefix(j.Title, "回滚整合包")):
			a.message, a.errorMessage = j.Message, true
		case j.State == "completed" && strings.HasPrefix(j.Title, "导入整合包"):
			a.message, a.errorMessage = t.ImportDoneNotice, false
		case j.State == "failed" && a.message == t.InstallTaskCreated:
			a.message, a.errorMessage = j.Message, true
		case a.message == t.InstallTaskCreated:
			a.message = ""
		}
	}
}

// closeDialogNow closes the dialog without its exit animation (used when
// the page underneath changes at the same moment).
func (a *App) closeDialogNow() {
	a.closeDialog()
	a.dialogShown = ""
}

func (a *App) rootByID(id string) *launcher.Instance {
	for i := range a.snapshot.Instances {
		if a.snapshot.Instances[i].ID == id {
			return &a.snapshot.Instances[i]
		}
	}
	return nil
}

// launchRoot makes a directory current (keeping its selected version) and
// launches it, as 「启动」 on a finished import task does.
func (a *App) launchRoot(id string) {
	if a.snapshot.ActiveRoot == id {
		a.launch()
		return
	}
	a.execute("select", map[string]string{"rootId": id}, func(json.RawMessage) {
		a.snapshot = a.service.Snapshot()
		a.launch()
	})
}
