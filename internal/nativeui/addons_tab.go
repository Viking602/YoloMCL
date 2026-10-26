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
	if a.launcherUpdateRequired() {
		return
	}
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
	a.search()
	a.goPage("discover")
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
			textField(c, &a.modFilter).Placeholder(a.L("addons_tab.search_name_or_file")).Label(t.ModsSearchAria).Grow(1).Basis(0).MinWidth(160 * s)
			sortOpts := []string{a.L("addons_tab.by_name"), a.L("addons_tab.by_size")}
			cur := sortOpts[0]
			if a.addonSortKey == "size" {
				cur = sortOpts[1]
			}
			if selectBox(c, "addon-sort", a.L("addons_tab.sort"), &cur, sortOpts).Width(116 * s).Shrink(0).Changed() {
				a.addonSortKey = ""
				if cur == sortOpts[1] {
					a.addonSortKey = "size"
				}
			}
			ui.Row(c.Key("addon-view")).Label(a.L("addons_tab.view")).Padding(2 * s).Gap(2 * s).Radius(9 * s).Background(fieldColor).Shrink(0).Children(func() {
				for _, v := range []struct {
					list       bool
					icon, text string
				}{{false, "mods", a.L("addons_tab.cards")}, {true, "list", a.L("addons_tab.list")}} {
					sel := a.addonListMode == v.list
					b := tooltip(c, ui.ButtonBase(c.Key("addon-view-"+v.icon)).Label(v.text), v.text).Size(btnSm*s, btnSm*s-4*s).Radius(7 * s).Center()
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
			if iconOnlyButton(c, "addon-open-folder", "folder", a.L("addons_tab.open_folder"), btnSm*s+4*s, false) {
				a.openSub(addonFolder(kind))
			}
			if kind == "mods" {
				if a.modChecking {
					ui.Row(c).Height(btnSm*s + 4*s).Gap(6 * s).AlignItems(ui.Center).Shrink(0).Children(func() {
						ui.Spinner(c).Size(12*s, 12*s)
						label(c, a.L("addons_tab.checking_updates")).FontSize(typeFootnote * s).NoWrap()
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
			if fixedButton(c, "addon-import", a.L("addons_tab.import"), "plus", 0, btnSm*s+4*s, false, a.instancePending("addLocalMods", "addLocalAssetPacks") || a.root() == nil) {
				a.importAddons(kind)
			}
			if fixedButton(c, "find-mods", a.L("addons_tab.discover"), "search", 0, btnSm*s+4*s, true, false) {
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
			hint := a.L("addons_tab.click_to_enable_or_disable")
			if a.addonTab == "mods" {
				hint = a.L("addons_tab.click_to_toggle_disabling_asks_for_confirmation")
			}
			label(c, hint).FontSize(typeCaption * s).NoWrap().Shrink(1).MinWidth(0)
		})
	})
}

func (a *App) addonEmpty(c *ui.Context) {
	s := uiScale(c)
	kind := a.addonTab
	icon, title, desc, cta := "mods", a.L("addons_tab.no_mods_in_this_instance_yet"), a.L("addons_tab.install_from_discover_or_drop_jar_files_onto"), a.L("addons_tab.find_mods")
	switch kind {
	case "resourcepack":
		icon, title, desc, cta = "layers", a.L("addons_tab.no_resource_packs_yet"), a.L("addons_tab.pick_one_in_discover_or_drop_a_zip"), a.L("addons_tab.find_resource_packs")
	case "shader":
		icon, title, desc, cta = "sun", a.L("addons_tab.no_shaders_yet"), a.L("addons_tab.pick_one_in_discover_or_drop_a_zip_2"), a.L("addons_tab.find_shaders")
	}
	emptyState(c, "empty-"+kind, icon, title, desc, func() {
		if fixedButton(c, "empty-discover", cta, "search", 0, btnMd*s, true, false) {
			a.goDiscover(kind, "")
		}
		if fixedButton(c, "empty-import", a.L("addons_tab.import"), "plus", 0, btnMd*s, false, a.instancePending("addLocalMods", "addLocalAssetPacks")) {
			a.importAddons(kind)
		}
		if fixedButton(c, "empty-folder", a.L("addons_tab.open_folder"), "folder", 0, btnMd*s, false, false) {
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
				label(c, a.L("addons_tab.in_use_in_game")).FontSize(typeCaption * s)
				cur := a.L("addons_tab.no_shader_chosen_in_game_yet")
				fg := muted
				switch {
				case !stOK:
					cur = a.L("addons_tab.reading")
				case st.ShadersOn:
					cur, fg = strings.TrimSuffix(st.ShaderPack, ".zip"), ink
				case st.ShaderConfig != "" && st.ShaderPack != "":
					cur = a.L("addons_tab.shaders_are_off")
				}
				ui.Text(c, cur).FontSize(typeHeadline * s).FontWeight(650).TextColor(fg).SingleLine().FillWidth().MinWidth(0)
			})
		})
		ui.Box(c).Width(1).Height(34 * s).Background(line).Shrink(0)
		ui.Row(c).Grow(1).Basis(260 * s).MinWidth(0).Gap(12 * s).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(3 * s).Children(func() {
				label(c, a.L("addons_tab.shader_loader")).FontSize(typeCaption * s)
				switch {
				case !known:
					ui.Text(c, a.L("addons_tab.checking")).FontSize(typeBody * s).FontWeight(600).TextColor(muted).SingleLine()
				case name != "":
					ui.Row(c).Gap(6 * s).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons["check"]).Size(13*s, 13*s).TextColor(ui.Hex("#34c759"))
						ui.Text(c, strings.TrimSpace(name+" "+ver)).FontSize(typeBody * s).FontWeight(600).TextColor(ink).SingleLine().MinWidth(0)
					})
				default:
					ui.Text(c, fmt.Sprintf(a.L("addons_tab.is_needed_to_load_shaders"), need)).FontSize(typeBody * s).FontWeight(600).TextColor(ui.Hex("#c26a00")).SingleLine().MinWidth(0)
				}
			})
			if known && name == "" && query != "" {
				if fixedButton(c, "shader-get-loader", fmt.Sprintf(a.L("addons_tab.get"), need), "download", 0, btnSm*s+4*s, true, false) {
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
			label(c, fmt.Sprintf(a.L("addons_tab.no_matching"), a.addonKindWord()))
			if fixedButton(c, "addon-clear-filter", a.L("addons_tab.clear_filters"), "cross", 0, btnSm*s, false, false) {
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
				shown[i].it.badge = a.L("addons_tab.in_use")
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
				sectionTitle(c, a.L("addons_tab.on_in_game_by_priority"), len(active))
				list("rp-active", active)
			})
		}
		if len(rest) > 0 {
			ui.Column(c).FillWidth().Gap(8 * s).Children(func() {
				if len(active) > 0 {
					sectionTitle(c, a.L("addons_tab.not_on_in_game"), len(rest))
				}
				list("rp-rest", rest)
			})
		}
		return
	}
	list("pack-list", shown)
}
