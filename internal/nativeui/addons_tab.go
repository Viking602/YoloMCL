package nativeui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// instGameState is what the game itself remembers for an instance: the
// shader pack picked in Iris / Oculus / OptiFine and the resource pack
// order from options.txt. Only files that exist are read; unknown stays "".
type instGameState struct {
	shaderCfg  string // iris | oculus | optifine | ""
	shaderPack string
	shadersOn  bool
	hasOptions bool
	rpOrder    []string // pack files, highest priority first
}

func parseProps(b []byte) map[string]string {
	out := map[string]string{}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(strings.TrimRight(lines[i], "\r"))
		for strings.HasSuffix(line, `\`) && !strings.HasSuffix(line, `\\`) && i+1 < len(lines) { // continuation
			i++
			line = strings.TrimSuffix(line, `\`) + strings.TrimSpace(lines[i])
		}
		if line == "" || line[0] == '#' || line[0] == '!' {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			k, v, ok = strings.Cut(line, ":")
		}
		if !ok {
			continue
		}
		out[propUnescape(strings.TrimSpace(k))] = propUnescape(strings.TrimSpace(v))
	}
	return out
}

// propUnescape decodes Java .properties escapes: \uXXXX (Iris/Oculus/
// OptiFine write non-ASCII names this way), \t \n \r \f, and \x -> x for
// any other char (\! \: \= \# \space \\).
func propUnescape(v string) string {
	if !strings.Contains(v, `\`) {
		return v
	}
	var b strings.Builder
	r := []rune(v)
	for i := 0; i < len(r); i++ {
		if r[i] != '\\' || i+1 >= len(r) {
			b.WriteRune(r[i])
			continue
		}
		i++
		switch r[i] {
		case 'u':
			if i+4 < len(r) {
				if n, e := strconv.ParseUint(string(r[i+1:i+5]), 16, 32); e == nil {
					b.WriteRune(rune(n))
					i += 4
					continue
				}
			}
			b.WriteRune('u')
		case 't':
			b.WriteRune('\t')
		case 'n':
			b.WriteRune('\n')
		case 'r':
			b.WriteRune('\r')
		case 'f':
			b.WriteRune('\f')
		default:
			b.WriteRune(r[i])
		}
	}
	return b.String()
}

func readGameState(dir string) instGameState {
	var st instGameState
	for _, c := range []struct{ file, kind string }{{"config/iris.properties", "iris"}, {"config/oculus.properties", "oculus"}} {
		b, e := os.ReadFile(filepath.Join(dir, c.file))
		if e != nil {
			continue
		}
		p := parseProps(b)
		st.shaderCfg, st.shaderPack = c.kind, p["shaderPack"]
		st.shadersOn = st.shaderPack != "" && !strings.EqualFold(p["enableShaders"], "false")
		break
	}
	if st.shaderCfg == "" {
		if b, e := os.ReadFile(filepath.Join(dir, "optionsshaders.txt")); e == nil {
			st.shaderCfg = "optifine"
			if v := parseProps(b)["shaderPack"]; v != "" && v != "OFF" && v != "(internal)" {
				st.shaderPack, st.shadersOn = v, true
			}
		}
	}
	if b, e := os.ReadFile(filepath.Join(dir, "options.txt")); e == nil {
		st.hasOptions = true
		for _, line := range strings.Split(string(b), "\n") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(line), "resourcePacks:"); ok {
				var list []string
				if json.Unmarshal([]byte(v), &list) == nil {
					for i := len(list) - 1; i >= 0; i-- { // the last entry wins in game
						if f, ok := strings.CutPrefix(list[i], "file/"); ok {
							st.rpOrder = append(st.rpOrder, f)
						}
					}
				}
			}
		}
	}
	return st
}

// refreshGameState rereads the game's own files in the background; the
// previous value stays on screen until the new one lands (no flash).
func (a *App) refreshGameState() {
	a.refreshGameStateContext(a.ctx)
}

func (a *App) refreshGameStateContext(ctx context.Context) {
	r := a.root()
	if r == nil || a.service == nil {
		return
	}
	key := r.ID + "|" + r.SelectedVersion
	id, ver, tab := r.ID, r.SelectedVersion, a.addonTab
	ready := a.firstFrame
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		if !waitForFirstFrame(ctx, ready) {
			return
		}
		dir, e := a.service.InstanceFolder(id, ver, "")
		if e != nil {
			return
		}
		st := readGameState(dir)
		var mods []launcher.Mod
		if tab == "shader" {
			mods, _ = a.service.ListModsStreamContext(ctx, id, ver, nil)
		}
		a.post(func() {
			if ctx.Err() != nil {
				return
			}
			if a.gameStates == nil {
				a.gameStates = map[string]instGameState{}
				a.shaderMods = map[string][]launcher.Mod{}
			}
			a.gameStates[key] = st
			if mods != nil {
				a.shaderMods[key] = mods
			}
		})
	}()
}

func (a *App) gameState() (instGameState, bool) {
	r := a.root()
	if r == nil {
		return instGameState{}, false
	}
	st, ok := a.gameStates[r.ID+"|"+r.SelectedVersion]
	return st, ok
}

// shaderLoader finds Iris / Oculus / OptiFine among the instance's mods.
func (a *App) shaderLoader() (name, version string, known bool) {
	r := a.root()
	if r == nil {
		return "", "", false
	}
	key := r.ID + "|" + r.SelectedVersion
	mods, ok := a.shaderMods[key]
	if c, hit := a.addonCache[key+"|mods"]; hit {
		mods, ok = c.mods, true
	}
	if !ok {
		return "", "", false
	}
	for _, m := range mods {
		if !m.Enabled {
			continue
		}
		id, f := strings.ToLower(m.ModID), strings.ToLower(m.File)
		switch {
		case id == "iris" || strings.HasPrefix(f, "iris-") || strings.HasPrefix(f, "iris_"):
			return "Iris", m.Version, true
		case id == "oculus" || strings.HasPrefix(f, "oculus"):
			return "Oculus", m.Version, true
		case id == "optifine" || strings.HasPrefix(f, "optifine"):
			return "OptiFine", m.Version, true
		}
	}
	return "", "", true
}

// shaderNeed is the shader loader that fits the instance's mod loader.
func shaderNeed(loader string) (label, query string) {
	switch strings.ToLower(loader) {
	case "fabric", "quilt", "neoforge":
		return "Iris", "iris"
	case "forge":
		return "Oculus", "oculus"
	}
	return "OptiFine", ""
}

func (a *App) goDiscover(kind, query string) {
	t := a.tr()
	switch kind {
	case "resourcepack":
		a.resourceType = t.DiscoverTypeRP
	case "shader":
		a.resourceType = t.DiscoverTypeShader
	default:
		a.resourceType = t.DiscoverTypeMod
	}
	a.query = query
	a.searchResult = launcher.SearchResult{}
	a.goPage("discover")
	if query != "" && a.service != nil {
		a.search()
	}
}

func (a *App) importAddons(kind string) {
	r := a.root()
	if r == nil {
		return
	}
	id := r.ID
	if kind == "mods" {
		a.chooseFiles("mods", func(paths []string) {
			a.execute("addLocalMods", map[string]any{"rootId": id, "paths": paths}, func(json.RawMessage) { a.loadMods() })
		})
		return
	}
	a.chooseFiles("zip", func(paths []string) {
		a.execute("addLocalAssetPacks", map[string]any{"rootId": id, "kind": kind, "paths": paths}, func(json.RawMessage) { a.loadMods() })
	})
}

func addonFolder(kind string) string {
	switch kind {
	case "resourcepack":
		return "resourcepacks"
	case "shader":
		return "shaderpacks"
	}
	return "mods"
}

type addonEntry struct {
	it     addonItem
	mod    *launcher.Mod
	pack   *launcher.AssetPack
	name   string
	size   int64
	update bool
}

func (a *App) addonEntries() []addonEntry {
	var out []addonEntry
	if a.addonTab == "mods" {
		inst := ""
		if v := a.gameVersion(); v != nil {
			inst = strings.ToLower(v.Loader)
		}
		have := map[string]bool{}
		for _, m := range a.mods {
			if m.Enabled {
				have[strings.ToLower(m.ModID)] = true
				for _, p := range m.Provides {
					have[strings.ToLower(p)] = true
				}
			}
		}
		for i := range a.mods {
			m := &a.mods[i]
			it := a.modItem(*m)
			if m.Enabled {
				for _, r := range m.Requires {
					if !have[strings.ToLower(r)] {
						it.missing = append(it.missing, r)
					}
				}
			}
			loader := a.loaderDisplay(m.Loader)
			if strings.EqualFold(m.Loader, inst) || m.Loader == "" {
				loader = "" // same as the instance: no need to repeat it on every row
			}
			it.meta = joinMeta(m.Version, loader)
			it.version, it.loader = m.Version, loader
			it.size, it.source = sizeText(m.Size), m.Source
			it.iconURL = a.modIconURLs[m.SHA1]
			p, sec := addonNames(m.Name, m.File)
			rem := a.addonRemote[m.SHA1]
			it.summary, it.cats = rem.Summary, rem.Cats
			if it.summary == "" {
				it.summary = m.Description
			}
			if !a.isEN() {
				if zh := a.zhNames.Mod(rem.CFSlug, rem.MRSlug, m.ModID); zh != "" && !hasCJK(p) {
					it.zhName = zh
				}
			}
			out = append(out, addonEntry{it: it, mod: m, name: strings.ToLower(p + " " + sec + " " + m.File + " " + m.ModID), size: m.Size, update: m.Latest != nil})
		}
		return out
	}
	for i := range a.assetPacks {
		p := &a.assetPacks[i]
		it := a.packItem(*p)
		it.meta = strings.Join(strings.Fields(p.Description), " ")
		it.size = sizeText(p.Size)
		rem := a.addonRemote[p.Kind+"|"+stableFile(p.File)]
		it.iconURL, it.cats = rem.Icon, rem.Cats
		it.summary = strings.Join(strings.Fields(p.Description), " ")
		if rem.Summary != "" && (it.summary == "" || len([]rune(it.summary)) < 12) {
			it.summary = rem.Summary
		}
		out = append(out, addonEntry{it: it, pack: p, name: strings.ToLower(p.Name + " " + p.File + " " + p.Description), size: p.Size})
	}
	return out
}

func (a *App) addonFilterCounts(all []addonEntry) (on, off, upd int) {
	for _, e := range all {
		if e.it.enabled {
			on++
		} else {
			off++
		}
		if e.update {
			upd++
		}
	}
	return
}

func (a *App) filteredAddons(all []addonEntry) []addonEntry {
	q := strings.ToLower(strings.TrimSpace(a.modFilter))
	var out []addonEntry
	for _, e := range all {
		if q != "" && !strings.Contains(e.name, q) {
			continue
		}
		switch a.addonFilterKind {
		case "on":
			if !e.it.enabled {
				continue
			}
		case "off":
			if e.it.enabled {
				continue
			}
		case "update":
			if !e.update {
				continue
			}
		}
		out = append(out, e)
	}
	if a.addonSortKey == "size" {
		sort.SliceStable(out, func(i, j int) bool { return out[i].size > out[j].size })
	} else {
		sort.SliceStable(out, func(i, j int) bool { return out[i].name < out[j].name })
	}
	return out
}

func (a *App) addonKindWord() string {
	switch a.addonTab {
	case "resourcepack":
		return a.L("资源包", "resource packs")
	case "shader":
		return a.L("光影", "shaders")
	}
	return a.L("模组", "mods")
}

// addonToolbar: search, sort, the folder, import and Discover on one line;
// filter pills with live counts on the next.
func (a *App) addonToolbar(c *ui.Context, all []addonEntry) {
	s := uiScale(c)
	t := a.tr()
	kind := a.addonTab
	ui.Column(c.Key("addon-toolbar")).FillWidth().Gap(12 * s).Children(func() {
		ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
			textField(c, &a.modFilter).Placeholder(a.L("搜索名称或文件名", "Search name or file")).Label(t.ModsSearchAria).Grow(1).Basis(0).MinWidth(160 * s)
			sortOpts := []string{a.L("按名称", "By name"), a.L("按大小", "By size")}
			cur := sortOpts[0]
			if a.addonSortKey == "size" {
				cur = sortOpts[1]
			}
			if selectBox(c, "addon-sort", a.L("排序", "Sort"), &cur, sortOpts).Width(116 * s).Shrink(0).Changed() {
				a.addonSortKey = ""
				if cur == sortOpts[1] {
					a.addonSortKey = "size"
				}
			}
			ui.Row(c.Key("addon-view")).Label(a.L("视图", "View")).Padding(2 * s).Gap(2 * s).Radius(9 * s).Background(fieldColor).Shrink(0).Children(func() {
				for _, v := range []struct {
					list       bool
					icon, text string
				}{{false, "mods", a.L("卡片", "Cards")}, {true, "list", a.L("列表", "List")}} {
					sel := a.addonListMode == v.list
					b := ui.ButtonBase(c.Key("addon-view-"+v.icon)).Label(v.text).Tooltip(v.text).Size(btnSm*s, btnSm*s-4*s).Radius(7 * s).Center()
					bg, fg := ui.Transparent, muted
					if sel {
						bg, fg = panel, ink
					} else if b.Hovered() {
						fg = inkSecondary
					}
					b.Background(b.AnimateColor("bg", bg, fastAnim))
					list := v.list
					if b.Children(func() { ui.Icon(c, icons[v.icon]).Size(12*s, 12*s).TextColor(fg) }).Clicked() {
						a.addonListMode = list
					}
				}
			})
			if iconOnlyButton(c, "addon-open-folder", "folder", a.L("打开文件夹", "Open folder"), btnSm*s+4*s, false) {
				a.openSub(addonFolder(kind))
			}
			if kind == "mods" {
				if a.modChecking {
					ui.Row(c).Height(btnSm*s + 4*s).Gap(6 * s).AlignItems(ui.Center).Shrink(0).Children(func() {
						ui.Spinner(c).Size(12*s, 12*s)
						label(c, a.L("正在检查更新", "Checking updates")).FontSize(typeFootnote * s).NoWrap()
					})
				}
				if iconOnlyButton(c, "check-mods", "refresh", t.ModsCheckUpdatesBtn, btnSm*s+4*s, a.modChecking || a.pendingOp("checkMods") || a.root() == nil) {
					id := a.root().ID
					a.execute("checkMods", map[string]string{"rootId": id}, func(b json.RawMessage) {
						a.mods = decode[[]launcher.Mod](b)
						a.notice(a.tr().ModsCheckedNotice)
					})
				}
			}
			if fixedButton(c, "addon-import", a.L("导入文件", "Import"), "plus", 0, btnSm*s+4*s, false, a.instancePending("addLocalMods", "addLocalAssetPacks") || a.root() == nil) {
				a.importAddons(kind)
			}
			if fixedButton(c, "find-mods", a.L("去发现", "Discover"), "search", 0, btnSm*s+4*s, true, false) {
				a.goDiscover(kind, "")
			}
		})
		on, off, upd := a.addonFilterCounts(all)
		if len(all) == 0 || (off == 0 && upd == 0 && a.addonFilterKind == "") {
			return // nothing to tell apart: every item is on and current
		}
		ui.Row(c).FillWidth().Gap(6 * s).AlignItems(ui.Center).Children(func() {
			pill := func(key, text string, n int) {
				sel := a.addonFilterKind == key
				b := ui.ButtonBase(c.Key("addon-filter-" + key)).Label(text).Height(28 * s).PaddingX(11 * s).Gap(6 * s).Radius(999).AlignItems(ui.Center).Shrink(0)
				bg, fg := ui.RGBA(120, 130, 160, .08), inkSecondary
				if sel {
					bg, fg = accentSoft, accent
				} else if b.Hovered() {
					bg = ui.RGBA(120, 130, 160, .14)
				}
				b.Background(b.AnimateColor("bg", bg, fastAnim))
				if b.Children(func() {
					ui.Text(c, text).FontSize(typeFootnote * s).FontWeight(600).TextColor(fg).NoWrap()
					ui.Text(c, fmt.Sprint(n)).FontSize(typeFootnote * s).FontWeight(500).TextColor(fg.Alpha(.7)).NoWrap()
				}).Clicked() {
					a.addonFilterKind = key
				}
			}
			pill("", a.L("全部", "All"), len(all))
			pill("on", a.L("已启用", "Enabled"), on)
			if off > 0 || a.addonFilterKind == "off" {
				pill("off", a.L("已禁用", "Disabled"), off)
			}
			if upd > 0 || a.addonFilterKind == "update" {
				pill("update", a.L("可更新", "Updates"), upd)
			}
			ui.Spacer(c).Grow(1)
			label(c, a.L("点按一行即可启用或禁用", "Click a row to enable or disable")).FontSize(typeCaption * s).NoWrap().Shrink(1).MinWidth(0)
		})
	})
}

func (a *App) addonEmpty(c *ui.Context) {
	s := uiScale(c)
	kind := a.addonTab
	icon, title, desc, cta := "mods", a.L("这个实例还没有模组", "No mods in this instance yet"), a.L("在发现页搜索安装，或把 .jar 文件直接拖进窗口。", "Install from Discover, or drop .jar files onto the window."), a.L("发现模组", "Find mods")
	switch kind {
	case "resourcepack":
		icon, title, desc, cta = "layers", a.L("还没有资源包", "No resource packs yet"), a.L("在发现页挑选资源包，或把 .zip 拖进窗口。装好后在游戏的资源包菜单里启用。", "Pick one in Discover, or drop a .zip onto the window, then turn it on in the game's resource pack menu."), a.L("发现资源包", "Find resource packs")
	case "shader":
		icon, title, desc, cta = "sun", a.L("还没有光影", "No shaders yet"), a.L("在发现页挑选光影，或把 .zip 拖进窗口。装好后在游戏的光影菜单里选择。", "Pick one in Discover, or drop a .zip onto the window, then choose it in the game's shader menu."), a.L("发现光影", "Find shaders")
	}
	emptyState(c, "empty-"+kind, icon, title, desc, func() {
		if fixedButton(c, "empty-discover", cta, "search", 0, btnMd*s, true, false) {
			a.goDiscover(kind, "")
		}
		if fixedButton(c, "empty-import", a.L("导入文件", "Import"), "plus", 0, btnMd*s, false, a.instancePending("addLocalMods", "addLocalAssetPacks")) {
			a.importAddons(kind)
		}
		if fixedButton(c, "empty-folder", a.L("打开文件夹", "Open folder"), "folder", 0, btnMd*s, false, false) {
			a.openSub(addonFolder(kind))
		}
	})
}

// shaderStatusCard: the shader the game will use and whether the loader
// it needs is installed. Only facts read from disk are shown.
func (a *App) shaderStatusCard(c *ui.Context) {
	s := uiScale(c)
	st, stOK := a.gameState()
	name, ver, known := a.shaderLoader()
	inst := ""
	if v := a.gameVersion(); v != nil {
		inst = v.Loader
	}
	need, query := shaderNeed(inst)
	glass(ui.Row(c.Key("shader-status")).Label("光影状态").FillWidth().Wrap().Gap(18*s).Padding(16*s, 20*s), radiusCard*s, s).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Grow(1).Basis(260 * s).MinWidth(0).Gap(14 * s).AlignItems(ui.Center).Children(func() {
			appleSoftIconBox(c, 40*s, "sun", ui.Hex("#ff9f0a"), ui.Hex("#ff6b3d"), panel)
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(3 * s).Children(func() {
				label(c, a.L("游戏中正在使用", "In use in game")).FontSize(typeCaption * s)
				cur := a.L("还没有在游戏里选择光影", "No shader chosen in game yet")
				fg := muted
				switch {
				case !stOK:
					cur = a.L("读取中", "Reading")
				case st.shadersOn:
					cur, fg = strings.TrimSuffix(st.shaderPack, ".zip"), ink
				case st.shaderCfg != "" && st.shaderPack != "":
					cur = a.L("光影已关闭", "Shaders are off")
				}
				ui.Text(c, cur).FontSize(typeHeadline * s).FontWeight(650).TextColor(fg).SingleLine().FillWidth().MinWidth(0)
			})
		})
		ui.Box(c).Width(1).Height(34 * s).Background(line).Shrink(0)
		ui.Row(c).Grow(1).Basis(260 * s).MinWidth(0).Gap(12 * s).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(3 * s).Children(func() {
				label(c, a.L("光影加载器", "Shader loader")).FontSize(typeCaption * s)
				switch {
				case !known:
					ui.Text(c, a.L("检查中", "Checking")).FontSize(typeBody * s).FontWeight(600).TextColor(muted).SingleLine()
				case name != "":
					ui.Row(c).Gap(6 * s).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons["check"]).Size(13*s, 13*s).TextColor(ui.Hex("#34c759"))
						ui.Text(c, strings.TrimSpace(name+" "+ver)).FontSize(typeBody * s).FontWeight(600).TextColor(ink).SingleLine().MinWidth(0)
					})
				default:
					ui.Text(c, fmt.Sprintf(a.L("需要 %s 才能加载光影", "%s is needed to load shaders"), need)).FontSize(typeBody * s).FontWeight(600).TextColor(ui.Hex("#c26a00")).SingleLine().MinWidth(0)
				}
			})
			if known && name == "" && query != "" {
				if fixedButton(c, "shader-get-loader", fmt.Sprintf(a.L("安装 %s", "Get %s"), need), "download", 0, btnSm*s+4*s, true, false) {
					a.goDiscover("mods", query)
				}
			}
		})
	})
}

func sectionTitle(c *ui.Context, text string, n int) {
	s := uiScale(c)
	ui.Row(c).FillWidth().PaddingX(4 * s).Gap(8 * s).AlignItems(ui.Center).Children(func() {
		ui.Text(c, text).FontSize(typeCallout * s).FontWeight(650).TextColor(inkSecondary).NoWrap()
		ui.Text(c, fmt.Sprint(n)).FontSize(typeCallout * s).TextColor(muted).NoWrap()
	})
}

// addonTabBody is the embedded 模组 / 资源包 / 光影 tab.
func (a *App) addonTabBody(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	if _, ok := a.gameState(); !ok && !a.gameStateAsked[a.addonTab+a.root().ID+"|"+a.root().SelectedVersion] {
		if a.gameStateAsked == nil {
			a.gameStateAsked = map[string]bool{}
		}
		a.gameStateAsked[a.addonTab+a.root().ID+"|"+a.root().SelectedVersion] = true
		a.refreshGameState()
	}
	if a.addonTab == "shader" {
		a.shaderStatusCard(c)
	}
	if a.addonTab == "mods" && !a.modsLoading {
		a.fetchModIcons()
		a.ensureZhNames()
	}
	if a.addonTab != "mods" && !a.modsLoading {
		a.fetchPackIcons(a.addonTab)
	}
	all := a.addonEntries()
	if a.modsLoading && len(all) == 0 {
		a.addonToolbar(c, all)
		loadingState(c, "mods-loading", t.ModsLoading)
		return
	}
	if len(all) == 0 {
		a.addonEmpty(c)
		return
	}
	a.addonToolbar(c, all)
	shown := a.filteredAddons(all)
	if len(shown) == 0 {
		glass(ui.Column(c.Key("addon-nomatch")).FillWidth().Padding(28*s, 24*s).Gap(10*s), radiusCard*s, s).AlignItems(ui.Center).Children(func() {
			label(c, fmt.Sprintf(a.L("没有符合条件的%s", "No matching %s"), a.addonKindWord()))
			if fixedButton(c, "addon-clear-filter", a.L("清除筛选", "Clear filters"), "cross", 0, btnSm*s, false, false) {
				a.modFilter, a.addonFilterKind = "", ""
			}
		})
		return
	}
	grid := !a.addonListMode
	draw := func(it addonItem, toggle func(), actions func()) {
		if grid {
			a.addonCard(c, it, toggle, actions)
		} else {
			a.addonRow(c, it, false, toggle, actions)
		}
	}
	row := func(e addonEntry) {
		if e.mod != nil {
			m := *e.mod
			draw(e.it, a.toggleModFn(m), func() {
				if m.Latest != nil && (grid && iconOnlyButton(c, "update-"+m.File, "download", t.ModsUpdateBtn, btnSm*s, false) || !grid && fixedButton(c, "update-"+m.File, t.ModsUpdateBtn, "download", 0, btnSm*s, false, false)) {
					a.execute("installMod", map[string]string{"rootId": a.root().ID, "source": m.Source, "projectId": m.ProjectID, "versionId": m.Latest.ID, "replace": m.File}, func(json.RawMessage) {
						a.notice(a.tr().ModsUpdateQueued)
					})
				}
				if iconOnlyButton(c, "remove-"+m.File, "trash", t.ModsRemoveBtn, btnSm*s, a.pendingAt("removeMod", "rootId", a.root().ID, "name", m.File)) {
					a.form.Name = m.File
					a.dialog = "removeMod"
					a.dialogOpen = true
				}
			})
			return
		}
		pk := *e.pack
		draw(e.it, a.togglePackFn(pk), func() {
			if iconOnlyButton(c, "remove-pack-"+pk.File, "trash", t.ModsRemoveBtn, btnSm*s, a.pendingAt("removeAssetPack", "rootId", a.root().ID, "name", pk.File)) {
				a.execute("removeAssetPack", map[string]string{"rootId": a.root().ID, "kind": pk.Kind, "name": pk.File}, func(json.RawMessage) { a.loadMods() })
			}
		})
	}
	// list renders a group as the card grid or the classic list.
	list := func(key string, entries []addonEntry) {
		if grid {
			a.addonGrid(c, key, entries, row)
			return
		}
		glass(ui.Column(c.Key(key)).FillWidth().Padding(6*s), radiusCard*s, s).Children(func() {
			for _, e := range entries {
				row(e)
			}
		})
	}
	if a.addonTab == "mods" && grid {
		list("mod-grid", shown)
		return
	}
	if a.addonTab == "mods" {
		first, last := modWindow(len(shown), 60*s, a.modListBounds, a.tabViewBounds)
		list := glass(ui.Column(c.Key("mod-list")).FillWidth().Padding(6*s), radiusCard*s, s)
		a.modListBounds = list.LayoutBounds()
		list.Children(func() {
			if first > 0 {
				ui.Box(c.Key("mod-pad-top")).FillWidth().Height(float32(first) * 60 * s)
			}
			for _, e := range shown[first:last] {
				row(e)
			}
			if last < len(shown) {
				ui.Box(c.Key("mod-pad-end")).FillWidth().Height(float32(len(shown)-last) * 60 * s)
			}
		})
		return
	}
	st, _ := a.gameState()
	if a.addonTab == "shader" {
		for i := range shown {
			if st.shadersOn && shown[i].pack.File == st.shaderPack {
				shown[i].it.badge = a.L("使用中", "In use")
			}
		}
	}
	if a.addonTab == "resourcepack" && st.hasOptions {
		rank := map[string]int{}
		for i, f := range st.rpOrder {
			rank[f] = i + 1
		}
		var active, rest []addonEntry
		for _, e := range shown {
			if n := rank[e.pack.File]; n > 0 {
				e.it.rank = n
				active = append(active, e)
			} else {
				rest = append(rest, e)
			}
		}
		sort.SliceStable(active, func(i, j int) bool { return active[i].it.rank < active[j].it.rank })
		for i := range active {
			active[i].it.rank = i + 1 // packs missing from the folder do not take a place
		}
		if len(active) > 0 {
			ui.Column(c).FillWidth().Gap(8 * s).Children(func() {
				sectionTitle(c, a.L("游戏中已启用，按优先级", "On in game, by priority"), len(active))
				list("rp-active", active)
			})
		}
		if len(rest) > 0 {
			ui.Column(c).FillWidth().Gap(8 * s).Children(func() {
				if len(active) > 0 {
					sectionTitle(c, a.L("未在游戏中启用", "Not on in game"), len(rest))
				}
				list("rp-rest", rest)
			})
		}
		return
	}
	list("pack-list", shown)
}
