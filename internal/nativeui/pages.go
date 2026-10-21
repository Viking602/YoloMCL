package nativeui

import (
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	"mcl/internal/launcher"
)

func (a *App) home(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	heading(c, t.HomeKicker, t.HomeTitle, t.HomeDesc, func() {
		if button(c, "import-home", t.HomeImportPackBtn, "tasks", false, a.busy) {
			a.openDialog("import")
		}
		if button(c, "install-home", t.HomeInstallBtn, "plus", true, a.busy) {
			a.openDialog("install")
		}
	})

	if len(a.snapshot.Instances) == 0 {
		emptyState(c, "onboarding-card", "gamepad", t.HomeEmptyTitle, t.HomeEmptyDesc, func() {
			if button(c, "add-root", t.HomeAddFolderBtn, "folder-add", true, a.busy) {
				a.openDialog("add")
			}
		})
		return
	}
	// Directories are one fixed-height row of tabs; the active one's
	// versions follow directly. Path/logs/removal live in the 管理 dialog.
	var activeRoot *launcher.Instance
	for i := range a.snapshot.Instances {
		if a.snapshot.Instances[i].ID == a.snapshot.ActiveRoot {
			activeRoot = &a.snapshot.Instances[i]
		}
	}
	a.rootTabs(c)
	if activeRoot == nil {
		return
	}
	r := *activeRoot
	if len(r.Versions) == 0 {
		emptyState(c, "root-empty", "layers", t.HomeRootEmptyTitle, r.Name, func() {
			if button(c, "install-empty-root", t.HomeInstallBtn, "plus", true, a.busy) {
				a.openDialog("install")
			}
		})
		return
	}
	a.featuredVersion(c, r)
	var loaders, vanilla []launcher.VersionSummary
	for _, v := range r.Versions {
		if v.ID == r.SelectedVersion {
			continue
		}
		if loaderKey(v.Loader) == "vanilla" {
			vanilla = append(vanilla, v)
		} else {
			loaders = append(loaders, v)
		}
	}
	w, _ := c.Size()
	cols := 3
	if w < 1300 {
		cols = 2
	}
	if w < 860 {
		cols = 1
	}
	for _, g := range []struct {
		key, title string
		items      []launcher.VersionSummary
	}{{"loaders", t.HomeGroupLoaders, loaders}, {"vanilla", t.LoaderVanilla, vanilla}} {
		if len(g.items) == 0 {
			continue
		}
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8*s).Padding(space2*s, 4*s, 0, 4*s).Children(func() {
			label(c, g.title).FontSize(13 * s).FontWeight(600).TextColor(inkSecondary).SingleLine().Shrink(0)
			label(c, fmt.Sprint(len(g.items))).FontSize(13 * s).SingleLine().Shrink(0)
		})
		items := g.items
		ui.Column(c.Key("version-grid-" + g.key)).FillWidth().Gap(space2 * s).Children(func() {
			for i := 0; i < len(items); i += cols {
				ui.Row(c).FillWidth().Gap(space2 * s).AlignItems(ui.Stretch).Children(func() {
					for j := i; j < i+cols; j++ {
						if j < len(items) {
							a.versionTile(c, r, items[j])
						} else {
							ui.Box(c).Grow(1).Basis(0).MinWidth(0)
						}
					}
				})
			}
		})
	}
}

// featuredVersion highlights the selected version (details + settings);
// launching stays in the bottom launch bar.
func (a *App) featuredVersion(c *ui.Context, r launcher.Instance) {
	s := uiScale(c)
	t := a.tr()
	var v *launcher.VersionSummary
	for i := range r.Versions {
		if r.Versions[i].ID == r.SelectedVersion {
			v = &r.Versions[i]
		}
	}
	card := glass(ui.Row(c.Key("featured-version")).Label("主版本卡").FillWidth().Clip().Padding(space4*s, space5*s).Gap(space4*s), 22*s, s).AlignItems(ui.Center)
	card.BorderLinearGradient(ui.LinearGradient{From: ui.RGBA(10, 132, 255, .45), To: ui.RGBA(175, 82, 222, .35), Angle: 120, Oklab: true})
	card.Children(func() {
		if v == nil || !loaderLogo(c, v.Loader, 56*s) {
			appleSoftIconBox(c, 56*s, "gamepad", ui.Hex("#eaf3ff"), ui.Hex("#dbeafe"), accent)
		}
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Clip().Gap(6 * s).Children(func() {
			if v == nil {
				ui.Text(c, t.HomeHeroNoVersion).FillWidth().MinWidth(0).FontSize(24 * s).Bold().TextColor(ink).SingleLine()
				label(c, r.Name).FillWidth().MinWidth(0).FontSize(13.5 * s).SingleLine()
				return
			}
			ft, fs := a.cardText(r, *v)
			titleWithTags(c, ft, v.ID, 21*s, 700, a.versionTags(r, *v, true), 4)
			if fs != "" {
				label(c, fs).FillWidth().MinWidth(0).FontSize(14 * s).SingleLine().Tooltip(v.ID)
			}
		})
		if v != nil && button(c, "cfg-featured", t.HomeVersionCfgBtn, "settings", false, a.busy) {
			a.openVersionConfig(r.ID, v.ID)
		}
	})
}

// rootTabs renders game directories as rounded tabs (icon + name + version
// count) in a horizontally scrolling row of fixed height, ending with
// "+ 添加目录". The full path is the tab's tooltip.
func (a *App) rootTabs(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	ui.ScrollHorizontal(c.Key("root-tabs-scroll")).Label("目录标签栏").FillWidth().Height(44 * s).Shrink(0).Children(func() {
		ui.Row(c).Height(44*s).Gap(space2*s).AlignItems(ui.Center).Padding(0, 2*s).Children(func() {
			for _, root := range a.snapshot.Instances {
				r := root
				active := r.ID == a.snapshot.ActiveRoot
				bg, fg, sub := ui.RGBA(255, 255, 255, .62), ink, muted
				b := ui.ButtonBase(c.Key("root-tab-" + r.ID)).Label("目录 " + r.Name).Tooltip(r.Path).Height(36 * s).MaxWidth(260 * s).Shrink(0).PaddingX(14 * s).Gap(8 * s).Radius(999).AlignItems(ui.Center).Disabled(a.busy || (!r.Available && !active))
				if active {
					bg, fg, sub = accentSoft, accent, accent.Alpha(.75)
					b.Border(1, accent.Alpha(.30))
				} else {
					b.Border(1, glassEdge)
					if b.Hovered() {
						bg = panel
					}
				}
				b.Background(b.AnimateColor("bg", bg, fastAnim))
				b.Children(func() {
					icon := "folder"
					if r.Pack != nil {
						icon = "layers"
					}
					ui.Icon(c, icons[icon]).Size(15*s, 15*s).TextColor(fg).Shrink(0)
					ui.Text(c, midEllipsis(c, r.Name, 190*s, 14*s, 600)).FontSize(14 * s).FontWeight(600).TextColor(fg).Shrink(1).MinWidth(0).SingleLine()
					ui.Text(c, fmt.Sprint(len(r.Versions))).FontSize(12.5 * s).FontWeight(600).TextColor(sub).NoWrap().Shrink(0)
					if !r.Available {
						ui.Icon(c, icons["cross"]).Size(12*s, 12*s).TextColor(danger).Shrink(0)
					}
				})
				if b.RightClicked() {
					if !active {
						_ = a.service.Select(r.ID, "", "")
						a.snapshot = a.service.Snapshot()
					}
					a.openDialog("root")
				} else if b.Clicked() && !active {
					a.selectRoot(r.ID)
				}
				if active && iconOnlyButton(c, "root-settings", "settings", t.HomeDirSettingsBtn, 32*s, a.busy) {
					a.openDialog("root")
				}
			}
			add := ui.ButtonBase(c.Key("add-root")).Label(t.HomeAddFolderBtn).Height(36*s).Shrink(0).PaddingX(14*s).Gap(6*s).Radius(999).AlignItems(ui.Center).Border(1, glassEdge).Disabled(a.busy)
			addBg := ui.Transparent
			if add.Hovered() {
				addBg = panel
			}
			add.Background(add.AnimateColor("bg", addBg, fastAnim))
			if add.Children(func() {
				ui.Icon(c, icons["plus"]).Size(14*s, 14*s).TextColor(inkSecondary)
				ui.Text(c, t.HomeAddFolderBtn).FontSize(14 * s).FontWeight(600).TextColor(inkSecondary).NoWrap()
			}).Clicked() {
				if a.form.Path == "" && !a.hasRootPath(launcher.DefaultGameDir()) {
					a.form.Path = launcher.DefaultGameDir() // the official launcher's folder on this OS
				}
				a.openDialog("add")
			}
		})
	})
}

// loaderVersionOf extracts the loader build from standard installer ids:
// fabric-loader-X-MC, quilt-loader-X-MC, neoforge-X, MC-forge-X.
func loaderVersionOf(v launcher.VersionSummary) string {
	id, mc := v.ID, v.Minecraft
	switch loaderKey(v.Loader) {
	case "fabric", "quilt":
		for _, p := range []string{"fabric-loader-", "quilt-loader-"} {
			if strings.HasPrefix(id, p) && strings.HasSuffix(id, "-"+mc) {
				return strings.TrimSuffix(strings.TrimPrefix(id, p), "-"+mc)
			}
		}
	case "neoforge":
		if strings.HasPrefix(id, "neoforge-") {
			return strings.TrimPrefix(id, "neoforge-")
		}
	case "forge":
		if strings.HasPrefix(id, mc+"-forge-") {
			return strings.TrimPrefix(id, mc+"-forge-")
		}
	}
	return ""
}

// javaFor falls back to the parent vanilla version's requirement, since
// loader JSONs usually inherit javaVersion.
func javaFor(r launcher.Instance, v launcher.VersionSummary) int {
	if v.JavaVersion > 0 {
		return v.JavaVersion
	}
	for _, p := range r.Versions {
		if p.ID == v.Minecraft && p.JavaVersion > 0 {
			return p.JavaVersion
		}
	}
	return 0
}

func (a *App) loadMods() {
	r := a.root()
	if r == nil {
		a.mods = nil
		a.assetPacks = nil
		return
	}
	id := r.ID
	tab := a.addonTab
	if tab == "" {
		tab = "mods"
	}
	a.modsLoading = true
	go func() {
		if tab == "resourcepack" || tab == "shader" {
			packs, e := a.service.ListAssetPacks(id, tab)
			a.post(func() {
				if a.snapshot.ActiveRoot != id {
					return
				}
				a.modsLoading = false
				if e != nil {
					a.message = e.Error()
					a.errorMessage = true
					return
				}
				a.assetPacks = packs
			})
			return
		}
		mods, e := a.service.ListMods(id)
		a.post(func() {
			if a.snapshot.ActiveRoot != id {
				return
			}
			a.modsLoading = false
			if e != nil {
				a.message = e.Error()
				a.errorMessage = true
				return
			}
			a.mods = mods
		})
	}()
}

func (a *App) modPage(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	if a.addonTab == "" {
		a.addonTab = "mods"
	}
	heading(c, t.ModsKicker, t.ModsTitle, t.ModsDesc, func() {
		if a.addonTab == "mods" {
			if button(c, "check-mods", t.ModsCheckUpdatesBtn, "refresh", false, a.busy || a.root() == nil) {
				id := a.root().ID
				a.execute("checkMods", map[string]string{"rootId": id}, func(b json.RawMessage) {
					a.mods = decode[[]launcher.Mod](b)
					a.notice(a.tr().ModsCheckedNotice)
				})
			}
			if button(c, "add-mod", t.ModsImportJarBtn, "plus", true, a.busy || a.root() == nil) {
				id := a.root().ID
				a.chooseFiles("mods", func(paths []string) {
					a.execute("addLocalMods", map[string]any{"rootId": id, "paths": paths}, func(json.RawMessage) { a.loadMods() })
				})
			}
		} else {
			kind := a.addonTab
			if button(c, "add-assetpack", t.ModsImportPackBtn, "plus", true, a.busy || a.root() == nil) {
				id := a.root().ID
				a.chooseFiles("zip", func(paths []string) {
					a.execute("addLocalAssetPacks", map[string]any{"rootId": id, "kind": kind, "paths": paths}, func(json.RawMessage) { a.loadMods() })
				})
			}
		}
	})
	if a.root() == nil {
		empty(c, t.ModsEmptyRootTitle, t.ModsEmptyRootDesc)
		return
	}

	// Instance & Version Selector Bar + Segmented Addon Kind Tabs (Mods / Resource Packs / Shaders)
	glass(ui.Row(c).FillWidth().Wrap().Gap(12*s).Padding(12*s, 18*s), 20*s, s).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Padding(4 * s).Gap(4 * s).Radius(12 * s).Background(fieldColor).Children(func() {
			for _, item := range []struct{ key, label, icon string }{
				{"mods", t.ModsTabMods, "mods"},
				{"resourcepack", t.ModsTabResourcePack, "layers"},
				{"shader", t.ModsTabShaders, "globe"},
			} {
				tab := item
				sel := a.addonTab == tab.key
				targetBg := ui.Transparent
				fg := inkSecondary
				if sel {
					targetBg = panel
					fg = accent
				}
				b := ui.ButtonBase(c.Key("addon-tab-" + tab.key)).Label(tab.label).Height(34 * s).PaddingX(15 * s).Gap(7 * s).Radius(9 * s).AlignItems(ui.Center)
				if sel {
					b.Shadow(0, 2*s, 6*s, 0, ui.RGBA(15, 23, 42, .08))
				} else if b.Hovered() {
					targetBg = ui.Hex("#dfe6f2")
				}
				b.Background(b.AnimateColor("bg", targetBg, fastAnim))
				if b.Children(func() {
					ui.Icon(c, icons[tab.icon]).Size(15*s, 15*s).TextColor(fg)
					ui.Text(c, tab.label).FontSize(14 * s).FontWeight(600).TextColor(fg).NoWrap()
				}).Clicked() && !sel {
					a.addonTab = tab.key
					a.loadMods()
				}
			}
		})
		ui.Spacer(c).Grow(1)
		if r := a.root(); r != nil && len(r.Versions) > 0 {
			label(c, t.ModsContextVersion).FontSize(13.5 * s).FontWeight(600).NoWrap().Shrink(0)
			ver := r.SelectedVersion
			opts := make([]string, len(r.Versions))
			for i, v := range r.Versions {
				opts[i] = v.ID
			}
			rr := *r
			if selectBoxFull(c, "mods-version-select", t.ModsContextVersion, &ver, opts, nil, func(id string) string { return a.versionLine(rr, id) }).Width(220 * s).Shrink(0).Changed() {
				a.selectVersion(r.ID, ver)
			}
		}
	})

	if r := a.root(); r != nil && r.Pack != nil && a.addonTab == "mods" {
		card(c, "pack-audit", func() {
			ui.Row(c).FillWidth().Gap(14 * s).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(42*s, 42*s).Radius(10 * s).Background(accentSoft).Center().Shrink(0).Children(func() {
					ui.Icon(c, icons["layers"]).Size(22*s, 22*s).TextColor(accent)
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(4 * s).Children(func() {
					ui.Text(c, r.Pack.Name+" · "+r.Pack.Version).FontSize(17.5 * s).Bold().TextColor(ink)
					label(c, t.ModsPackAuditDesc)
				})
				if button(c, "check-pack", t.ModsPackVerifyBtn, "check", false, a.busy) {
					a.execute("checkPack", map[string]string{"rootId": r.ID}, func(b json.RawMessage) {
						report := decode[launcher.PackReport](b)
						a.packReport = &report
						a.dialog = "packReport"
						a.dialogOpen = true
					})
				}
			})
		})
	}

	totalCount := len(a.mods)
	if a.addonTab != "mods" {
		totalCount = len(a.assetPacks)
	}
	ui.Row(c).FillWidth().Gap(12 * s).AlignItems(ui.Center).Children(func() {
		ui.TextInput(c, &a.modFilter).Placeholder(t.ModsSearchHint).Label(t.ModsSearchAria).Grow(1).MinWidth(0)
		label(c, a.formatModCount(totalCount)).NoWrap().Shrink(0)
		if button(c, "find-mods", t.ModsExploreMoreBtn, "search", false, false) {
			switch a.addonTab {
			case "resourcepack":
				a.resourceType = t.DiscoverTypeRP
			case "shader":
				a.resourceType = t.DiscoverTypeShader
			default:
				a.resourceType = t.DiscoverTypeMod
			}
			a.searchResult = launcher.SearchResult{}
			a.goPage("discover")
		}
	})

	if a.modsLoading {
		loadingState(c, "mods-loading", t.ModsLoading)
	}

	if a.addonTab != "mods" {
		if len(a.assetPacks) == 0 && !a.modsLoading {
			if a.addonTab == "shader" {
				emptyState(c, "empty-shaders", "cube", t.ModsEmptyShaderTitle, t.ModsEmptyShaderDesc, a.exploreAction(c))
			} else {
				emptyState(c, "empty-rp", "layers", t.ModsEmptyRPTitle, t.ModsEmptyRPDesc, a.exploreAction(c))
			}
		}
		for _, pack := range a.assetPacks {
			p := pack
			if a.modFilter != "" && !strings.Contains(strings.ToLower(p.Name+" "+p.File+" "+p.Description), strings.ToLower(a.modFilter)) {
				continue
			}
			glass(ui.Row(c.Key("pack-"+p.Kind+"-"+p.File)).FillWidth().Padding(16*s, 20*s).Gap(14*s), 18*s, s).AlignItems(ui.Center).Children(func() {
				enabled := p.Enabled
				if ui.Switch(c, &enabled).Label(t.ModsTogglePrefix + p.Name).Changed() {
					a.execute("toggleAssetPack", map[string]string{"rootId": a.root().ID, "kind": p.Kind, "name": p.File}, func(json.RawMessage) { a.loadMods() })
				}
				ui.Column(c).Grow(1).MinWidth(0).Clip().Gap(5 * s).Children(func() {
					ui.Row(c).Gap(8 * s).AlignItems(ui.Center).Children(func() {
						ui.Text(c, p.Name).FontSize(17 * s).Bold().TextColor(ink).SingleLine()
						if p.Kind == "shader" {
							badge(c, t.DiscoverTypeShader, blue)
						} else {
							badge(c, t.DiscoverTypeRP, accent)
						}
					})
					sub := sizeText(p.Size) + "  ·  " + p.File
					if p.Description != "" {
						sub = p.Description + "  ·  " + sub
					}
					label(c, sub).FillWidth().FontSize(13.5 * s).SingleLine()
				})
				if button(c, "remove-pack-"+p.File, t.ModsRemoveBtn, "cross", false, a.busy) {
					a.execute("removeAssetPack", map[string]string{"rootId": a.root().ID, "kind": p.Kind, "name": p.File}, func(json.RawMessage) { a.loadMods() })
				}
			})
		}
		return
	}

	if len(a.mods) == 0 && !a.modsLoading {
		emptyState(c, "empty-mods", "mods", t.ModsEmptyListTitle, t.ModsEmptyListDesc, a.exploreAction(c))
	}
	for _, mod := range a.mods {
		m := mod
		if a.modFilter != "" && !strings.Contains(strings.ToLower(m.Name+" "+m.File+" "+m.ModID), strings.ToLower(a.modFilter)) {
			continue
		}
		glass(ui.Row(c.Key(m.File)).FillWidth().Padding(16*s, 20*s).Gap(14*s), 18*s, s).AlignItems(ui.Center).Children(func() {
			enabled := m.Enabled
			if ui.Switch(c, &enabled).Label(t.ModsTogglePrefix + m.Name).Changed() {
				a.execute("toggleMod", map[string]string{"rootId": a.root().ID, "name": m.File}, func(json.RawMessage) { a.loadMods() })
			}
			ui.Column(c).Grow(1).MinWidth(0).Clip().Gap(5 * s).Children(func() {
				ui.Row(c).Gap(8 * s).AlignItems(ui.Center).Children(func() {
					ui.Text(c, m.Name).FontSize(17 * s).Bold().TextColor(ink).SingleLine()
					badge(c, a.loaderDisplay(m.Loader), blue)
				})
				label(c, m.Version+"  ·  "+sizeText(m.Size)+"  ·  "+m.File).FillWidth().FontSize(13.5 * s).SingleLine()
			})
			if m.Latest != nil {
				badge(c, m.Latest.Number, accent)
				if button(c, "update-"+m.File, t.ModsUpdateBtn, "tasks", true, a.busy) {
					a.execute("installMod", map[string]string{"rootId": a.root().ID, "source": m.Source, "projectId": m.ProjectID, "versionId": m.Latest.ID, "replace": m.File}, func(json.RawMessage) {
						a.notice(a.tr().ModsUpdateQueued)
					})
				}
			} else if m.Status != "" {
				badge(c, a.modStatusText(m.Status), muted)
			}
			if button(c, "remove-"+m.File, t.ModsRemoveBtn, "cross", false, a.busy) {
				a.form.Name = m.File
				a.dialog = "removeMod"
				a.dialogOpen = true
			}
		})
	}
}

func (a *App) search() { a.searchAt(0) }

func (a *App) searchAt(offset int) {
	a.searchRevision++
	rev := a.searchRevision
	a.searchLoading = true
	a.searchError = ""
	kind := normalizeResourceType(a.resourceType)
	loader := loaderKey(a.filterLoader)
	if loader == "vanilla" || (kind != "mod" && kind != "modpack") {
		loader = ""
	}
	sortKey := normalizeSortKey(a.searchSort)
	sort := ""
	if sortKey == "downloads" || sortKey == "updated" {
		sort = sortKey
	}
	q := launcher.SearchQuery{Source: a.provider, Query: a.query, Type: kind, Minecraft: a.filterMC, Loader: loader, Offset: offset, Sort: sort}
	go func() {
		result, e := a.service.Search(a.ctx, q)
		a.post(func() {
			if rev != a.searchRevision {
				return
			}
			a.searchLoading = false
			if e != nil {
				a.searchResult = launcher.SearchResult{}
				a.searchError = e.Error()
				return
			}
			a.searchResult = result
		})
	}()
}

func (a *App) providerSwitch(c *ui.Context, source, name string, logo *ui.SVG) {
	s := uiScale(c)
	active := a.provider == source
	targetBg, fgCol := fieldColor, inkSecondary
	if active {
		targetBg, fgCol = accentSoft, accent
	}
	b := ui.ButtonBase(c.Key("brand-"+source)).Label(name).Height(40*s).PaddingX(15*s).Gap(8*s).Radius(12*s).Border(1, line).AlignItems(ui.Center).Shrink(0)
	if b.Hovered() && !active {
		targetBg = ui.Hex("#e2e8f0")
	}
	b.Background(b.AnimateColor("bg", targetBg, fastAnim))
	if b.Children(func() {
		ui.Image(c, logo).Size(20*s, 20*s).Shrink(0).Fit(ui.Contain)
		ui.Text(c, name).FontSize(14.5 * s).FontWeight(600).TextColor(fgCol).NoWrap()
	}).Clicked() && !active {
		a.provider = source
		a.searchResult = launcher.SearchResult{}
		a.search()
	}
}

func (a *App) discover(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	a.normalizeStateValues()
	heading(c, t.DiscoverKicker, t.DiscoverTitle, t.DiscoverDesc, nil)
	card(c, "search-controls", func() {
		ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
			a.providerSwitch(c, "modrinth", "Modrinth", modrinthLogo)
			a.providerSwitch(c, "curseforge", "CurseForge", curseforgeLogo)
			if selectBox(c, "discover-type", t.DiscoverTypeModpack, &a.resourceType, []string{t.DiscoverTypeModpack, t.DiscoverTypeMod, t.DiscoverTypeRP, t.DiscoverTypeShader}).Width(135 * s).Shrink(0).Changed() {
				a.search()
			}
			input := ui.TextInput(c, &a.query).Placeholder(t.DiscoverSearchHint).Label(t.DiscoverSearchAria).Grow(1).MinWidth(0)
			if input.Submitted() {
				a.search()
			}
			if button(c, "search-now", t.DiscoverSearchBtn, "search", true, a.searchLoading) {
				a.search()
			}
		})
		ui.Row(c).FillWidth().Wrap().Gap(12 * s).AlignItems(ui.Center).Children(func() {
			mcInput := ui.TextInput(c, &a.filterMC).Placeholder(t.DiscoverFilterMCHint).Label(t.DiscoverFilterMCAria).Width(220 * s).Shrink(0)
			if mcInput.Submitted() {
				a.search()
			}
			kind := normalizeResourceType(a.resourceType)
			if kind == "mod" || kind == "modpack" {
				if loaderSelectBox(c, "discover-loader", t.DiscoverFilterLoader, &a.filterLoader, []string{t.LoaderAll, t.LoaderFabric, t.LoaderForge, t.LoaderNeoForge, t.LoaderQuilt}).Width(185 * s).Shrink(0).Changed() {
					a.search()
				}
			}
			if selectBox(c, "discover-sort", t.DiscoverSortRelevance, &a.searchSort, []string{t.DiscoverSortRelevance, t.DiscoverSortDownloads, t.DiscoverSortUpdated}).Width(155 * s).Shrink(0).Changed() {
				a.search()
			}
			ui.Spacer(c).Grow(1)
			label(c, a.formatResultCount(int64(a.searchResult.Total))).NoWrap().Shrink(0)
		})
	})
	if a.searchLoading {
		loadingState(c, "search-loading", t.DiscoverLoading)
	} else if a.searchError == "" && len(a.searchResult.Projects) == 0 {
		if a.query == "" && a.searchResult.Total == 0 {
			emptyState(c, "search-idle", "search", t.DiscoverIdleTitle, t.DiscoverIdleDesc, nil)
		} else {
			emptyState(c, "search-empty", "search", t.DiscoverEmptyTitle, t.DiscoverEmptyDesc, nil)
		}
	}
	if a.searchError != "" {
		card(c, "search-error", func() {
			ui.Row(c).FillWidth().Gap(space2 * s).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons["info"]).Size(16*s, 16*s).TextColor(danger).Shrink(0)
				ui.Text(c, a.searchError).TextColor(danger).Grow(1).MinWidth(0)
			})
			if button(c, "retry-search", t.DiscoverRetryBtn, "refresh", false, false) {
				a.search()
			}
		})
	}
	// One grouped list, rows split by hairlines (no card per result).
	if len(a.searchResult.Projects) > 0 {
		glass(ui.Column(c.Key("discover-results")).Label("搜索结果列表").FillWidth().Padding(space1*s, 0), radiusCard*s, s).Children(func() {
			for i, project := range a.searchResult.Projects {
				if i > 0 {
					ui.Box(c).FillWidth().Padding(0, space4*s, 0, 76*s).Children(func() {
						ui.Box(c).Height(1).FillWidth().Background(line)
					})
				}
				a.discoverRow(c, project)
			}
		})
	}
	if len(a.searchResult.Projects) > 0 {
		pageSize := a.searchResult.Limit
		if pageSize <= 0 {
			pageSize = 18
		}
		currentPage := a.searchResult.Offset/pageSize + 1
		totalPages := max(1, (a.searchResult.Total+pageSize-1)/pageSize)
		ui.Row(c).Gap(14 * s).AlignItems(ui.Center).Children(func() {
			if button(c, "prev-page", t.DiscoverPrevPage, "", false, a.searchLoading || a.searchResult.Offset == 0) {
				a.searchAt(max(0, a.searchResult.Offset-pageSize))
			}
			badge(c, a.formatPagination(currentPage, totalPages), inkSecondary)
			if button(c, "next-page", t.DiscoverNextPage, "", false, a.searchLoading || a.searchResult.Offset+len(a.searchResult.Projects) >= a.searchResult.Total) {
				a.searchAt(a.searchResult.Offset + len(a.searchResult.Projects))
			}
		})
	}
}

func (a *App) openProject(p launcher.Project) {
	a.project = &p
	a.projectVersions = nil
	a.projectVersionFilter = ""
	a.dialog = "project"
	a.dialogOpen = true
	a.form = formState{Name: cleanName(p.Title)}
	if r := a.root(); r != nil {
		dir := r.Path
		if strings.EqualFold(strings.ToLower(dir[max(0, len(dir)-10):]), ".minecraft") {
			// Default parent directory for modpack downloads
		}
	}
	mc, loader := "", ""
	if p.Type == "mod" {
		if v := a.gameVersion(); v != nil {
			mc = v.Minecraft
			loader = v.Loader
		}
	} else if p.Type == "resourcepack" || p.Type == "shader" {
		if v := a.gameVersion(); v != nil {
			mc = v.Minecraft
		}
	}
	a.execute("versions", map[string]string{"source": p.Source, "projectId": p.ID, "minecraft": mc, "loader": loader}, func(b json.RawMessage) {
		a.projectVersions = decode[[]launcher.ProjectVersion](b)
		if len(a.projectVersions) > 0 {
			a.form.VersionID = a.projectVersions[0].ID
		}
	})
}

func (a *App) loadAppearance() {
	ac := a.account()
	a.appearanceErr = ""
	if ac == nil {
		a.appearance = launcher.Appearance{}
		a.appearanceAccount = ""
		return
	}
	id := ac.ID
	a.appearanceLoading = true
	go func() {
		v, e := a.service.Appearance(a.ctx, id)
		a.post(func() {
			if id != a.snapshot.ActiveAccount {
				return
			}
			a.appearanceLoading = false
			if e != nil {
				// Shown inside the wardrobe, next to what failed; a page-wide
				// banner would outlive the account it was about.
				a.appearanceErr = e.Error()
				a.appearance = launcher.Appearance{}
				return
			}
			a.appearanceErr = ""
			t := a.tr()
			a.appearance = v
			a.appearanceAccount = id
			a.model = t.WardrobeModelSteve
			if v.Model == "slim" {
				a.model = t.WardrobeModelAlex
			}
			a.preview.set(imageFromData(v.Skin), imageFromData(v.Cape), v.Model == "slim")
			if h := skinHead(imageFromData(v.Skin)); h != nil {
				a.accountHead(launcher.Account{ID: id}) // ensure cache maps exist
				a.avatars.heads[id] = ui.NewBitmap(h).Pixelated()
			}
			a.selectedCape = t.WardrobeCapeNone
			for _, cape := range v.Capes {
				if cape.Active {
					a.selectedCape = cape.Name
				}
			}
		})
	}()
}

func (a *App) wardrobe(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	a.normalizeStateValues()
	heading(c, t.WardrobeKicker, t.WardrobeTitle, t.WardrobeDesc, func() {
		if button(c, "refresh-skin", t.WardrobeRefreshBtn, "refresh", false, a.appearanceLoading) {
			a.loadAppearance()
		}
	})
	if a.account() == nil {
		emptyState(c, "empty-wardrobe", "account", t.WardrobeNoAccountTitle, t.WardrobeNoAccountDesc, func() {
			if button(c, "wardrobe-add-account", t.AccAddAccountBtn, "plus", true, a.busy) {
				a.openDialog("account")
			}
		})
		return
	}
	if a.appearanceErr != "" {
		a.inlineNotice(c, "warning", t.WardrobeLoadFailed, a.appearanceErr)
	}
	ui.Row(c).FillWidth().Gap(20 * s).AlignItems(ui.Start).Children(func() {
		glass(ui.Column(c).Width(390*s).Shrink(0).Gap(14*s).Padding(22*s), 22*s, s).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.account().Name).FontSize(18 * s).Bold().TextColor(ink)
				ui.Spacer(c).Grow(1)
				badge(c, a.accountKindText(a.account().Kind), blue)
			})
			a.preview.view(c, t)
			label(c, t.WardrobeOrbitHint).FontSize(13 * s)
			ui.Row(c).Gap(10 * s).AlignItems(ui.Center).Children(func() {
				if button(c, "front-skin", t.WardrobeFrontBtn, "", false, false) {
					a.preview.yaw = -.35
					a.preview.dirty = true
				}
				if button(c, "back-skin", t.WardrobeBackBtn, "", false, false) {
					a.preview.yaw = math.Pi - .35
					a.preview.dirty = true
				}
				spin := a.preview.spin
				if ui.Checkbox(c, &spin, t.WardrobeAutoSpin).Changed() {
					a.preview.spin = spin
				}
			})
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(18 * s).Children(func() {
			card(c, "skin-options", func() {
				ui.Text(c, t.WardrobeSkinTitle).FontSize(18 * s).Bold().TextColor(ink)
				label(c, t.WardrobeSkinDesc)
				if selectBox(c, "skin-model", t.WardrobeModelLabel, &a.model, []string{t.WardrobeModelSteve, t.WardrobeModelAlex}).FillWidth().Changed() {
					a.preview.slim = normalizeSkinModel(a.model) == "slim"
					a.preview.dirty = true
				}
				ui.Row(c).Gap(10 * s).Children(func() {
					if buttonWhy(c, "upload-skin", t.WardrobeUploadSkinBtn, "plus", true, whyOf(a.busy, t.WhyBusy, !a.appearance.CanSkin, a.skinWhy()), false) {
						a.upload("skin")
					}
					if button(c, "reset-skin", t.WardrobeResetSkinBtn, "", false, a.busy) {
						a.execute("resetTexture", map[string]string{"accountId": a.account().ID, "kind": "skin"}, func(json.RawMessage) { a.loadAppearance() })
					}
				})
			})
			card(c, "cape-options", func() {
				ui.Text(c, t.WardrobeCapeTitle).FontSize(18 * s).Bold().TextColor(ink)
				if a.account().Kind == "microsoft" {
					label(c, t.WardrobeCapeMSDesc)
					names := []string{t.WardrobeCapeNone}
					for _, cape := range a.appearance.Capes {
						names = append(names, cape.Name)
					}
					selectBox(c, "cape-select", t.WardrobeCapeTitle, &a.selectedCape, names).FillWidth()
					if buttonWhy(c, "apply-cape", t.WardrobeApplyCapeBtn, "check", true, whyOf(a.busy, t.WhyBusy, !a.appearance.CanCape, a.skinWhy()), false) {
						id := ""
						for _, cape := range a.appearance.Capes {
							if cape.Name == a.selectedCape {
								id = cape.ID
							}
						}
						a.execute("setCape", map[string]string{"accountId": a.account().ID, "capeId": id}, func(json.RawMessage) { a.loadAppearance() })
					}
				} else {
					label(c, t.WardrobeCapeLocalDesc)
					ui.Row(c).Gap(10 * s).Children(func() {
						if buttonWhy(c, "upload-cape", t.WardrobeUploadCapeBtn, "plus", true, whyOf(a.busy, t.WhyBusy, !a.appearance.CanCape, a.skinWhy()), false) {
							a.upload("cape")
						}
						if button(c, "reset-cape", t.WardrobeRemoveCapeBtn, "", false, a.busy) {
							a.execute("resetTexture", map[string]string{"accountId": a.account().ID, "kind": "cape"}, func(json.RawMessage) { a.loadAppearance() })
						}
					})
				}
			})
			if a.account().Kind == "offline" {
				card(c, "offline-skin-note", func() {
					ui.Text(c, t.WardrobeOfflineNoteTitle).FontSize(16 * s).Bold().TextColor(ink)
					label(c, t.WardrobeOfflineNote1)
					label(c, t.WardrobeOfflineNote2)
				})
			}
			if a.account().Kind == "yggdrasil" && !a.appearance.CanSkin {
				label(c, t.WardrobeYggNoUpload)
			}
		})
	})
}

func (a *App) tasks(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	heading(c, t.TasksKicker, t.TasksTitle, t.TasksDesc, func() {
		if a.root() != nil && button(c, "tasks-open-logs", t.TasksLogsBtn, "terminal", false, a.busy) {
			a.openDialog("logs")
		}
	})
	if len(a.snapshot.Jobs) == 0 {
		emptyState(c, "empty-tasks", "tasks", t.TasksEmptyTitle, t.TasksEmptyDesc, func() {
			if button(c, "tasks-install", t.HomeInstallBtn, "plus", true, a.busy) {
				a.openDialog("install")
			}
			if button(c, "tasks-discover", t.NavDiscover, "search", false, false) {
				a.goPage("discover")
			}
		})
	}
	for _, job := range a.snapshot.Jobs {
		j := job
		card(c, "job-"+j.ID, func() {
			ui.Row(c).FillWidth().Gap(12 * s).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons["tasks"]).Size(20*s, 20*s).TextColor(accent)
				ui.Text(c, j.Title).Grow(1).FontSize(18 * s).Bold().TextColor(ink)
				color := muted
				if j.State == "failed" {
					color = danger
				} else if j.State == "running" {
					color = accent
				} else if j.State == "done" || j.State == "completed" {
					color = success
				}
				badge(c, a.jobStateText(j.State), color)
				if j.State == "running" && button(c, "cancel-"+j.ID, t.TasksCancelBtn, "cross", false, a.busy) {
					a.execute("cancelJob", map[string]string{"id": j.ID}, nil)
				}
				if j.State == "completed" && j.ResultRef != "" && a.rootByID(j.ResultRoot) != nil {
					a.packJobButtons(c, j)
				} else if j.State == "completed" && j.ResultRoot != "" && a.rootByID(j.ResultRoot) != nil {
					if button(c, "job-open-"+j.ID, t.TaskOpenDirBtn, "folder", false, false) {
						a.openFolder(j.ResultRoot)
					}
					if button(c, "job-launch-"+j.ID, t.TaskLaunchBtn, "play", true, a.busy) {
						a.launchRoot(j.ResultRoot)
					}
				}
			})
			ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
				label(c, j.Message).Grow(1).Ellipsis("…")
				if j.Total > 0 {
					pct := int(math.Round(float64(j.Done) * 100 / float64(j.Total)))
					badge(c, fmt.Sprintf("%d%%  (%d / %d)", pct, j.Done, j.Total), accent)
				}
			})
			if j.Total > 0 {
				ui.Progress(c, float64(j.Done)/float64(j.Total))
			}
			if len(j.Items) > 0 {
				ui.Column(c).FillWidth().Gap(6*s).Padding(12*s).Radius(10*s).Background(raised).Border(1, line).Children(func() {
					label(c, t.TasksItemsHeader).FontSize(12.5 * s).FontWeight(600)
					scrollArea(c, "job-items-"+j.ID).MaxHeight(160*s).FillWidth().Padding(0, scrollGutter*s, 0, 0).Children(func() {
						ui.Column(c).FillWidth().Gap(4 * s).Children(func() {
							for i := len(j.Items) - 1; i >= 0; i-- {
								itemText := j.Items[i]
								ui.Row(c).Gap(8 * s).AlignItems(ui.Center).Children(func() {
									iconKey := "check"
									iconCol := success
									if i == len(j.Items)-1 && j.State == "running" {
										iconKey = "tasks"
										iconCol = accent
									}
									ui.Icon(c, icons[iconKey]).Size(13*s, 13*s).TextColor(iconCol).Shrink(0)
									label(c, itemText).FontSize(13 * s).SingleLine()
								})
							}
						})
					})
				})
			}
			label(c, j.Started.Local().Format("2006-01-02 15:04:05")).FontSize(12.5 * s)
		})
	}
}

// versionTile is a compact row for a non-selected version: small logo,
// title + loader build, and icon-sized select/settings actions.
func (a *App) versionTile(c *ui.Context, r launcher.Instance, v launcher.VersionSummary) {
	s := uiScale(c)
	t := a.tr()
	ui.Row(c.Key("ver-row-"+v.ID)).Label("版本卡片 "+v.ID).Grow(1).Basis(0).MinWidth(0).Clip().Padding(space2*s+2*s, space3*s).Radius(radiusRow*s+2*s).Background(ui.RGBA(255, 255, 255, .58)).Border(1, glassEdge).Gap(space3 * s).AlignItems(ui.Center).Children(func() {
		if !loaderLogo(c, v.Loader, 30*s) {
			ui.Box(c).Size(30*s, 30*s).Radius(8 * s).Background(fieldColor).Center().Shrink(0).Children(func() {
				ui.Icon(c, icons[loaderIconName(v.Loader)]).Size(16*s, 16*s).TextColor(loaderTone(v.Loader))
			})
		}
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Clip().Gap(2 * s).Children(func() {
			title, sub := a.cardText(r, v)
			titleWithTags(c, title, v.ID, 14.5*s, 600, a.versionTags(r, v, false), 4)
			if sub != "" {
				label(c, sub).FillWidth().MinWidth(0).FontSize(12 * s).SingleLine().Tooltip(v.ID)
			}
		})
		if fixedButton(c, "pick-ver-"+v.ID, t.HomeUseInstanceBtn, "", 0, 32*s, false, a.busy) {
			a.selectVersion(r.ID, v.ID)
		}
		if iconOnlyButton(c, "cfg-ver-"+v.ID, "settings", t.HomeVersionCfgBtn, 32*s, a.busy) {
			a.openVersionConfig(r.ID, v.ID)
		}
	})
}

// exploreAction is the empty mods/resources list's way forward.
func (a *App) exploreAction(c *ui.Context) func() {
	return func() {
		if button(c, "empty-explore", a.tr().ModsExploreMoreBtn, "search", false, false) {
			a.goPage("discover")
		}
	}
}

// discoverRow is one search result in the grouped list.
func (a *App) discoverRow(c *ui.Context, p launcher.Project) {
	s := uiScale(c)
	t := a.tr()
	ui.Row(c.Key(p.Source+p.ID)).Label("搜索结果 "+p.Title).FillWidth().Padding(space3*s, space4*s).Gap(space4 * s).AlignItems(ui.Center).Children(func() {
		if img := a.remoteImage(p.Icon); img != nil {
			ui.Box(c).Size(44*s, 44*s).Radius(11*s).Clip().Center().Shrink(0).Shadow(0, 2*s, 8*s, 0, ui.RGBA(15, 23, 42, .06)).Children(func() {
				ui.Image(c, img).Size(44*s, 44*s).Fit(ui.Contain)
			})
		} else {
			appleSoftIconBox(c, 44*s, "mods", ui.Hex("#eaf3ff"), ui.Hex("#dbeafe"), accent)
		}
		ui.Column(c).Grow(1).MinWidth(0).Clip().Gap(4 * s).Children(func() {
			ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
				ui.Text(c, p.Title).FontSize(15.5 * s).Bold().TextColor(ink).SingleLine()
				if p.Author != "" {
					label(c, "· "+p.Author).FontSize(13 * s).NoWrap().Shrink(0)
				}
				badge(c, a.formatDownloads(p.Downloads), blue)
			})
			if len(p.Categories) > 0 {
				ui.Row(c).FillWidth().Gap(6 * s).AlignItems(ui.Center).Children(func() {
					shown := 0
					for _, cat := range p.Categories {
						if shown >= 6 {
							break
						}
						trCat := a.translateCategory(cat)
						if trCat == "" {
							continue
						}
						tagChip(c, trCat)
						shown++
					}
				})
			}
			label(c, p.Description).FillWidth().FontSize(13 * s).SingleLine()
		})
		if button(c, "view-"+p.ID, t.DiscoverSelectVerBtn, "arrow", false, a.busy) {
			a.openProject(p)
		}
	})
}

func (a *App) hasRootPath(p string) bool {
	for _, r := range a.snapshot.Instances {
		if p == "" || strings.EqualFold(filepath.Clean(r.Path), filepath.Clean(p)) {
			return true
		}
	}
	return p == ""
}
