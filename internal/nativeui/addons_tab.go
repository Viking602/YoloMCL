package nativeui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

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
			a.addonFilterPills(c, len(all), on, off, upd)
			ui.Spacer(c).Grow(1)
			hint := a.L("点按切换启用状态", "Click to enable or disable")
			if a.addonTab == "mods" {
				hint = a.L("点按启用或禁用，禁用前需确认", "Click to toggle; disabling asks for confirmation")
			}
			label(c, hint).FontSize(typeCaption * s).NoWrap().Shrink(1).MinWidth(0)
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
				case st.ShadersOn:
					cur, fg = strings.TrimSuffix(st.ShaderPack, ".zip"), ink
				case st.ShaderConfig != "" && st.ShaderPack != "":
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
	a.addonResultBody(c).Children(func() { a.addonResults(c, shown) })
}

func (a *App) addonResults(c *ui.Context, shown []addonEntry) {
	s := uiScale(c)
	t := a.tr()
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
			if st.ShadersOn && shown[i].pack.File == st.ShaderPack {
				shown[i].it.badge = a.L("使用中", "In use")
			}
		}
	}
	if a.addonTab == "resourcepack" && st.HasOptions {
		rank := map[string]int{}
		for i, f := range st.ResourcePackOrder {
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
