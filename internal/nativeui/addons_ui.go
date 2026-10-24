package nativeui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// ensureZhNames loads the MC百科 name mapping once, in the background,
// when the UI is in Chinese.
func (a *App) ensureZhNames() {
	if a.isEN() || a.zhAsked || a.service == nil {
		return
	}
	a.zhAsked = true
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		z, _ := a.service.ZhNames(a.ctx)
		if z != nil {
			a.post(func() { a.zhNames = z })
		}
	}()
}

// zhTitle puts the Chinese name first when the UI is Chinese and one is
// known; English shows the original only.
func (a *App) zhTitle(zh, original string) (primary, secondary string) {
	if a.isEN() || zh == "" || strings.EqualFold(strings.TrimSpace(zh), strings.TrimSpace(original)) || strings.Contains(original, zh) {
		return original, ""
	}
	return zh, original
}

func hasCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// addonNames turns "[加速渲染] Accelerated Rendering", "钠 (Sodium)" or a
// raw file name into one primary title and a lighter secondary name: the
// Chinese name leads when known, the original (Latin) name follows.
func addonNames(name, file string) (primary, secondary string) {
	n := strings.TrimSpace(name)
	if n == "" {
		n = strings.TrimSuffix(strings.TrimSuffix(file, ".disabled"), filepath.Ext(strings.TrimSuffix(file, ".disabled")))
	}
	var tags []string
	for {
		n = strings.TrimSpace(n)
		var end int
		switch {
		case strings.HasPrefix(n, "["):
			end = strings.Index(n, "]")
		case strings.HasPrefix(n, "【"):
			end = strings.Index(n, "】")
		default:
			end = -1
		}
		if end <= 0 {
			break
		}
		open := 1
		if strings.HasPrefix(n, "【") {
			open = len("【")
		}
		tags = append(tags, strings.TrimSpace(n[open:end]))
		closeLen := 1
		if strings.HasPrefix(n[end:], "】") {
			closeLen = len("】")
		}
		n = n[end+closeLen:]
	}
	n = strings.TrimSpace(strings.Trim(n, "-_ "))
	// "中文名 (English)" / "English (中文名)"
	for _, pair := range [][2]string{{"(", ")"}, {"（", "）"}} {
		if i := strings.Index(n, pair[0]); i > 0 && strings.HasSuffix(n, pair[1]) {
			a, b := strings.TrimSpace(n[:i]), strings.TrimSpace(n[i+len(pair[0]):len(n)-len(pair[1])])
			if hasCJK(a) != hasCJK(b) {
				if hasCJK(b) {
					a, b = b, a
				}
				return a, b
			}
		}
	}
	if !hasCJK(n) {
		for _, tg := range tags {
			if hasCJK(tg) {
				return tg, n
			}
		}
	}
	if n == "" && len(tags) > 0 {
		n = tags[0]
	}
	return n, ""
}

// letterTile is the placeholder icon: the first letter on a tinted tile.
func letterTile(c *ui.Context, text string, size float32, dim bool) {
	s := uiScale(c)
	letter := "?"
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			letter = strings.ToUpper(string(r))
			break
		}
	}
	tones := []ui.Color{ui.Hex("#0a84ff"), ui.Hex("#5e5ce6"), ui.Hex("#30b0c7"), ui.Hex("#34c759"), ui.Hex("#ff9f0a"), ui.Hex("#ff375f"), ui.Hex("#bf5af2")}
	h := 0
	for _, r := range text {
		h = h*31 + int(r)
	}
	tone := tones[(h%len(tones)+len(tones))%len(tones)]
	if dim {
		tone = muted
	}
	ui.Box(c).Size(size, size).Radius(size * .24).Background(tone.Alpha(.14)).Center().Shrink(0).Children(func() {
		ui.Text(c, letter).FontSize(size * .46).FontWeight(700).TextColor(tone).NoWrap()
	})
	_ = s
}

type addonItem struct {
	key, kind, file, name string
	enabled               bool
	meta                  string
	latest                string // newer version available
	status                string
	size, source, badge   string   // trailing facts; empty ones are hidden
	rank                  int      // in-game resource pack priority (1 = top)
	iconURL               string   // CurseForge / Modrinth project icon (fallback)
	zhName                string   // MC百科 Chinese name, when the UI is Chinese
	version, loader       string   // chips on cards
	summary               string   // platform summary, else the file's own description
	cats                  []string // platform category slugs
	missing               []string // required mod ids no enabled jar provides
}

// addonRow is one mod / resource pack / shader row: icon, Chinese-first
// title with the original name lighter, a short meta line. Clicking the
// row toggles it; disabled rows dim with a quiet 已禁用 tag; secondary
// actions are small ghost icons.
func (a *App) addonRow(c *ui.Context, it addonItem, compact bool, onToggle func(), actions func()) {
	s := uiScale(c)
	height := float32(60)
	if compact {
		height = 54
	}
	ui.Row(c.Key("addon-" + it.key)).Label("资源项 " + it.name).FillWidth().Height(height * s).Gap(6 * s).AlignItems(ui.Center).Children(func() {
		b := ui.ButtonBase(c.Key("toggle-" + it.key)).Label(a.L("切换启用 ", "Toggle ") + it.name).Tooltip(it.file).Grow(1).Basis(0).MinWidth(0).FillHeight().PaddingX(10 * s).Gap(12 * s).Radius(radiusRow * s).AlignItems(ui.Center)
		bg := ui.Transparent
		if b.Hovered() {
			bg = ui.RGBA(120, 130, 160, .08)
		}
		b.Background(b.AnimateColor("bg", bg, fastAnim))
		primary, secondary := a.itemNames(it)
		if b.Children(func() {
			size := 36 * s
			icon := a.addonIcon(a.root().ID, a.root().SelectedVersion, it.kind, it.file, it.iconURL)
			op := float32(1)
			if !it.enabled {
				op = .42
			}
			ui.Box(c).Size(size, size).Shrink(0).Opacity(op).Children(func() {
				if icon != nil {
					ui.Box(c).Size(size, size).Radius(size * .22).Clip().Background(panel).Center().Children(func() {
						ui.Image(c, icon).Size(size, size).Fit(ui.Contain)
					})
				} else {
					letterTile(c, primary+secondary, size, !it.enabled)
				}
			})
			fg, sub := ink, muted
			if !it.enabled {
				fg = muted
			}
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Clip().Gap(3 * s).Children(func() {
				ui.Row(c).FillWidth().MinWidth(0).Gap(8 * s).AlignItems(ui.Center).Children(func() {
					ui.Text(c, primary).Shrink(1).MinWidth(0).FontSize(typeBody * s).FontWeight(600).TextColor(fg).SingleLine()
					if secondary != "" {
						ui.Text(c, secondary).Shrink(2).MinWidth(0).FontSize(typeFootnote * s).FontWeight(400).TextColor(sub).SingleLine()
					}
					if !it.enabled {
						quietTag(c, a.L("已禁用", "Disabled"), muted)
					}
					if it.latest != "" {
						quietTag(c, a.L("可更新 ", "Update ")+it.latest, accent)
					}
				})
				if it.meta != "" {
					ui.Text(c, it.meta).FillWidth().MinWidth(0).FontSize(typeCaption * s).TextColor(sub).SingleLine()
				}
			})
		}).Clicked() {
			onToggle()
		}
		ui.Row(c).Shrink(0).Gap(10 * s).PaddingX(4 * s).AlignItems(ui.Center).Children(func() {
			if it.rank > 0 {
				quietTag(c, fmt.Sprintf("#%d", it.rank), inkSecondary)
			}
			if it.badge != "" {
				quietTag(c, it.badge, ui.Hex("#248a3d"))
			}
			if len(it.missing) > 0 {
				ui.Box(c).Shrink(0).Tooltip(a.L("缺少前置：", "Missing dependencies: ") + strings.Join(it.missing, ", ")).Children(func() { quietTag(c, a.L("缺少前置", "Missing deps"), danger) })
			}
			if logo := sourceLogo(it.source); logo != nil {
				ui.Icon(c, logo).Size(14*s, 14*s).Tooltip(sourceName(it.source)).Label(sourceName(it.source)).Shrink(0)
			}
			if it.size != "" {
				ui.Text(c, it.size).Width(64 * s).TextAlign(ui.End).FontSize(typeCaption * s).TextColor(muted).NoWrap()
			}
		})
		if actions != nil {
			ui.Row(c).Shrink(0).Gap(2 * s).AlignItems(ui.Center).Children(actions)
		}
	})
}

func sourceLogo(src string) *ui.SVG {
	switch strings.ToLower(src) {
	case "curseforge":
		return curseforgeLogo
	case "modrinth":
		return modrinthLogo
	}
	return nil
}

func sourceName(src string) string {
	switch strings.ToLower(src) {
	case "curseforge":
		return "CurseForge"
	case "modrinth":
		return "Modrinth"
	}
	return ""
}

func quietTag(c *ui.Context, text string, tone ui.Color) {
	s := uiScale(c)
	ui.Row(c).Shrink(0).Height(18 * s).PaddingX(6 * s).Radius(5 * s).Background(tone.Alpha(.10)).Center().Children(func() {
		ui.Text(c, text).FontSize(typeCaption2 * s).FontWeight(600).TextColor(tone).NoWrap()
	})
}

func joinMeta(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "   ")
}

func (a *App) modItem(m launcher.Mod) addonItem {
	it := addonItem{key: "mod-" + stableFile(m.File), kind: "mod", file: m.File, name: m.Name, enabled: m.Enabled, status: m.Status}
	it.meta = joinMeta(m.Version, a.loaderDisplay(m.Loader), sizeText(m.Size))
	if m.Latest != nil {
		it.latest = m.Latest.Number
	}
	return it
}

func (a *App) packItem(p launcher.AssetPack) addonItem {
	desc := p.Description
	if len([]rune(desc)) > 48 {
		desc = string([]rune(desc)[:47]) + "…"
	}
	return addonItem{key: p.Kind + "-" + stableFile(p.File), kind: p.Kind, file: p.File, name: p.Name, enabled: p.Enabled, meta: joinMeta(desc, sizeText(p.Size))}
}

// toggledName is the file name after enabling / disabling (.disabled).
func toggledName(file string) string {
	if strings.HasSuffix(file, ".disabled") {
		return strings.TrimSuffix(file, ".disabled")
	}
	return file + ".disabled"
}

// stableFile is a file name without the .disabled suffix: row keys and
// icon cache keys use it so a toggle never remounts the row.
func stableFile(file string) string { return strings.TrimSuffix(file, ".disabled") }

// applyToggle flips one entry in the visible list and the cached list
// in place: the row updates at once, nothing is rescanned.
func (a *App) applyToggle(tab, from, to string) bool {
	r := a.root()
	if r == nil {
		return false
	}
	hit := false
	flip := func(mods []launcher.Mod, packs []launcher.AssetPack) {
		for i := range mods {
			if mods[i].File == from {
				mods[i].File, mods[i].Enabled, hit = to, !strings.HasSuffix(to, ".disabled"), true
			}
		}
		for i := range packs {
			if packs[i].File == from {
				packs[i].File, packs[i].Enabled, hit = to, !strings.HasSuffix(to, ".disabled"), true
			}
		}
	}
	if tab == "mods" {
		flip(a.mods, nil)
	} else {
		flip(nil, a.assetPacks)
	}
	if c, ok := a.addonCache[r.ID+"|"+r.SelectedVersion+"|"+tab]; ok {
		flip(c.mods, c.packs) // shares backing arrays with a.mods in the usual case
	}
	return hit
}

// toggleAsync renames in the background, one at a time, after the row has
// already flipped; a failure flips it back and says why.
func (a *App) toggleAsync(tab, file string, run func() error) {
	to := toggledName(file)
	if !a.applyToggle(tab, file, to) {
		return
	}
	start := time.Now()
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		a.toggleMu.Lock()
		e := run()
		a.toggleMu.Unlock()
		a.post(func() {
			a.lastToggle = time.Since(start)
			if e != nil {
				a.applyToggle(tab, to, file)
				a.message, a.errorMessage = a.L("没能切换 ", "Could not toggle ")+stableFile(file)+": "+a.friendlyErr(e), true
			}
		})
	}()
}

func (a *App) togglePackFn(p launcher.AssetPack) func() {
	return func() {
		r := a.root()
		if r == nil {
			return
		}
		id, kind, file := r.ID, p.Kind, p.File
		a.toggleAsync(kind, file, func() error { return a.service.ToggleAssetPack(id, kind, file) })
	}
}

func (a *App) loadInstanceStatus() {
	a.statusGen++
	gen := a.statusGen
	if a.statusCancel != nil {
		a.statusCancel()
		a.statusCancel = nil
	}
	r := a.root()
	if r == nil {
		return
	}
	id, ver, key := r.ID, r.SelectedVersion, r.ID+"|"+r.SelectedVersion
	changed := a.instStatusKey != key
	if changed {
		a.cancelServerPing()
	}
	a.instStatusKey = key
	a.instStatusErr = ""
	if c, ok := a.statusCache[key]; ok {
		a.instStatus = c
	} else if changed {
		a.instStatus = launcher.InstanceStatus{}
	}
	if a.service == nil {
		return
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.statusCancel = cancel
	a.statusLoading = true
	ready := a.firstFrame
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		defer cancel()
		if !waitForFirstFrame(ctx, ready) {
			return
		}
		st, e := a.service.InstanceOverviewContext(ctx, id, ver)
		if e != nil {
			if ctx.Err() == nil {
				a.post(func() {
					if gen == a.statusGen {
						a.statusLoading, a.instStatusErr = false, a.friendlyErr(e)
					}
				})
			}
			return
		}
		// Give local thumbnails a short head start, without allowing image
		// workers (or a large folder scan) to hold the summary hostage.
		imagesReady := make(chan struct{})
		a.bgWG.Add(1)
		go func() {
			defer a.bgWG.Done()
			defer close(imagesReady)
			a.prepareOverviewImages(ctx, id, ver, st)
			if ctx.Err() == nil {
				a.post(func() {})
			}
		}()
		timer := time.NewTimer(80 * time.Millisecond)
		select {
		case <-imagesReady:
		case <-timer.C:
		case <-ctx.Done():
		}
		timer.Stop()
		if ctx.Err() != nil {
			return
		}
		a.post(func() {
			if gen != a.statusGen {
				return
			}
			if a.statusCache == nil {
				a.statusCache = map[string]launcher.InstanceStatus{}
			}
			visible := st
			if cached, ok := a.statusCache[key]; ok && !cached.StoragePending && cached.Dir == st.Dir {
				visible.Sizes, visible.Total, visible.StoragePending = cached.Sizes, cached.Total, false
				if visible.RecentWorld != nil && cached.RecentWorld != nil && visible.RecentWorld.Dir == cached.RecentWorld.Dir {
					world := *visible.RecentWorld
					world.Size = cached.RecentWorld.Size
					visible.RecentWorld = &world
				}
			}
			a.statusCache[key] = visible
			if a.instStatusKey == key {
				a.instStatus, a.statusLoading = visible, false
			}
		})
		full, e := a.service.CompleteInstanceStorage(ctx, st)
		if e == nil && ctx.Err() == nil {
			a.post(func() {
				if gen != a.statusGen {
					return
				}
				a.statusCache[key] = full
				if a.instStatusKey == key {
					a.instStatus = full
				}
			})
		}
		select {
		case <-imagesReady:
		case <-ctx.Done():
		}
	}()
}

func (a *App) openSub(sub string) {
	r := a.root()
	if r == nil {
		return
	}
	p, e := a.service.InstanceFolder(r.ID, r.SelectedVersion, sub)
	if e == nil {
		e = mygo.Shell.OpenPath(p)
	}
	if e != nil {
		a.message, a.errorMessage = a.friendlyErr(e), true
	}
}

func agoText(a *App, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return friendlyTime(a.isEN(), t, time.Now())
}

// friendlyTime: 刚刚 / N 分钟前 / 今天 15:04 / 昨天 / N 天前 / 08月30日 / 2025年08月30日.
func friendlyTime(en bool, t, now time.Time) string {
	t, now = t.Local(), now.Local()
	L := func(zh, e string) string {
		if en {
			return e
		}
		return zh
	}
	d := now.Sub(t)
	day := func(x time.Time) time.Time { return time.Date(x.Year(), x.Month(), x.Day(), 0, 0, 0, 0, x.Location()) }
	days := int(day(now).Sub(day(t)).Hours() / 24)
	switch {
	case d < time.Minute && d > -time.Minute:
		return L("刚刚", "just now")
	case d < time.Hour && d > 0:
		return fmt.Sprintf(L("%d 分钟前", "%d min ago"), int(d.Minutes()))
	case days == 0:
		return L("今天 ", "Today ") + t.Format("15:04")
	case days == 1:
		return L("昨天 ", "Yesterday ") + t.Format("15:04")
	case days > 1 && days < 7:
		return fmt.Sprintf(L("%d 天前", "%d days ago"), days)
	case t.Year() == now.Year():
		return t.Format(L("01月02日", "Jan 02"))
	}
	return t.Format(L("2006年01月02日", "Jan 02, 2006"))
}

// itemNames is the title pair for a row or card: the Chinese name first
// when known, then the original.
func (a *App) itemNames(it addonItem) (primary, secondary string) {
	primary, secondary = addonNames(it.name, it.file)
	if it.zhName != "" {
		return it.zhName, primary
	}
	return primary, secondary
}
