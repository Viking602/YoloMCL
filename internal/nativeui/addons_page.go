package nativeui

import (
	"encoding/json"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func (a *App) modPage(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	if a.addonTab == "" {
		a.addonTab = "mods"
	}
	actions := func() {
		if a.addonTab == "mods" {
			if button(c, "check-mods", t.ModsCheckUpdatesBtn, "refresh", false, a.pendingOp("checkMods") || a.root() == nil) {
				id := a.root().ID
				a.execute("checkMods", map[string]string{"rootId": id}, func(b json.RawMessage) {
					a.mods = decode[[]launcher.Mod](b)
					a.notice(a.tr().ModsCheckedNotice)
				})
			}
			if button(c, "add-mod", t.ModsImportJarBtn, "plus", true, a.instancePending("addLocalMods") || a.root() == nil) {
				id := a.root().ID
				a.chooseFiles("mods", func(paths []string) {
					a.execute("addLocalMods", map[string]any{"rootId": id, "paths": paths}, func(json.RawMessage) { a.loadMods() })
				})
			}
		} else {
			kind := a.addonTab
			if button(c, "add-assetpack", t.ModsImportPackBtn, "plus", true, a.instancePending("addLocalAssetPacks") || a.root() == nil) {
				id := a.root().ID
				a.chooseFiles("zip", func(paths []string) {
					a.execute("addLocalAssetPacks", map[string]any{"rootId": id, "kind": kind, "paths": paths}, func(json.RawMessage) { a.loadMods() })
				})
			}
		}
	}
	if !a.embedMods {
		heading(c, t.ModsKicker, t.ModsTitle, t.ModsDesc, actions)
	}
	if a.root() == nil {
		empty(c, t.ModsEmptyRootTitle, t.ModsEmptyRootDesc)
		return
	}

	// Instance & Version Selector Bar + Segmented Addon Kind Tabs (Mods / Resource Packs / Shaders)
	if !a.embedMods {
		glass(ui.Row(c).FillWidth().Wrap().Gap(14*s).Padding(13*s, 18*s), radiusCard*s, s).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Padding(3 * s).Gap(3 * s).Radius(10 * s).Background(fieldColor).Children(func() {
				for _, item := range []struct{ key, label, icon string }{
					{"mods", t.ModsTabMods, "mods"},
					{"resourcepack", t.ModsTabResourcePack, "layers"},
					{"shader", t.ModsTabShaders, "sun"},
				} {
					tab := item
					sel := a.addonTab == tab.key
					targetBg := ui.Transparent
					fg := inkSecondary
					if sel {
						targetBg = panel
						fg = accent
					}
					b := ui.ButtonBase(c.Key("addon-tab-" + tab.key)).Label(tab.label).Height(btnSm * s).PaddingX(14 * s).Gap(6 * s).Radius(7.5 * s).AlignItems(ui.Center)
					if sel {
						b.Shadow(0, 2*s, 6*s, 0, ui.RGBA(15, 23, 42, .08))
					} else if b.Hovered() {
						targetBg = ui.Hex("#dfe6f2")
					}
					b.Background(b.AnimateColor("bg", targetBg, fastAnim))
					if b.Children(func() {
						ui.Icon(c, icons[tab.icon]).Size(13*s, 13*s).TextColor(fg)
						ui.Text(c, tab.label).FontSize(typeCallout * s).FontWeight(600).TextColor(fg).NoWrap()
					}).Clicked() && !sel {
						a.addonTab = tab.key
						a.loadMods()
					}
				}
			})
			ui.Spacer(c).Grow(1)
			if r := a.root(); r != nil && len(r.Versions) > 0 {
				label(c, t.ModsContextVersion).FontSize(typeCallout * s).FontWeight(600).NoWrap().Shrink(0)
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
	}

	if r := a.root(); r != nil && r.Pack != nil && a.addonTab == "mods" {
		card(c, "pack-audit", func() {
			ui.Row(c).FillWidth().Gap(14 * s).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(42*s, 42*s).Radius(10 * s).Background(accentSoft).Center().Shrink(0).Children(func() {
					ui.Icon(c, icons["layers"]).Size(22*s, 22*s).TextColor(accent)
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(4 * s).Children(func() {
					ui.Text(c, r.Pack.Name+"   "+r.Pack.Version).FontSize(typeHeadline * s).Bold().TextColor(ink)
					label(c, t.ModsPackAuditDesc)
				})
				if button(c, "check-pack", t.ModsPackVerifyBtn, "check", false, a.instancePending("checkPack")) {
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

	if a.embedMods {
		a.addonTabBody(c)
		return
	}
	totalCount := len(a.mods)
	if a.addonTab != "mods" {
		totalCount = len(a.assetPacks)
	}
	ui.Row(c).FillWidth().Gap(12 * s).AlignItems(ui.Center).Children(func() {
		textField(c, &a.modFilter).Placeholder(t.ModsSearchHint).Label(t.ModsSearchAria).Grow(1).MinWidth(0)
		label(c, a.formatModCount(totalCount)).NoWrap().Shrink(0)
		if a.embedMods {
			actions()
		}
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
				emptyState(c, "empty-shaders", "sun", t.ModsEmptyShaderTitle, t.ModsEmptyShaderDesc, a.exploreAction(c))
			} else {
				emptyState(c, "empty-rp", "layers", t.ModsEmptyRPTitle, t.ModsEmptyRPDesc, a.exploreAction(c))
			}
		}
		if len(a.assetPacks) > 0 {
			glass(ui.Column(c.Key("pack-list")).FillWidth().Padding(6*s), radiusCard*s, s).Children(func() {
				for _, pack := range a.assetPacks {
					p := pack
					if a.modFilter != "" && !strings.Contains(strings.ToLower(p.Name+" "+p.File+" "+p.Description), strings.ToLower(a.modFilter)) {
						continue
					}
					pk := p
					a.addonRow(c, a.packItem(pk), false, a.togglePackFn(pk), func() {
						if iconOnlyButton(c, "remove-pack-"+pk.File, "cross", t.ModsRemoveBtn, btnSm*s, a.pendingAt("removeAssetPack", "rootId", a.root().ID, "name", pk.File)) {
							a.execute("removeAssetPack", map[string]string{"rootId": a.root().ID, "kind": pk.Kind, "name": pk.File}, func(json.RawMessage) { a.loadMods() })
						}
					})
				}
			})
		}
		return
	}

	if len(a.mods) == 0 && !a.modsLoading {
		emptyState(c, "empty-mods", "mods", t.ModsEmptyListTitle, t.ModsEmptyListDesc, a.exploreAction(c))
	}
	if len(a.mods) == 0 {
		return
	}
	shown := a.mods
	if a.modFilter != "" {
		shown = nil
		q := strings.ToLower(a.modFilter)
		for _, m := range a.mods {
			if strings.Contains(strings.ToLower(m.Name+" "+m.File+" "+m.ModID), q) {
				shown = append(shown, m)
			}
		}
	}
	first, last := modWindow(len(shown), 60*s, a.modListBounds, a.tabViewBounds)
	list := glass(ui.Column(c.Key("mod-list")).FillWidth().Padding(6*s), radiusCard*s, s)
	a.modListBounds = list.LayoutBounds()
	list.Children(func() {
		if first > 0 {
			ui.Box(c.Key("mod-pad-top")).FillWidth().Height(float32(first) * 60 * s)
		}
		defer func() {
			if last < len(shown) {
				ui.Box(c.Key("mod-pad-end")).FillWidth().Height(float32(len(shown)-last) * 60 * s)
			}
		}()
		for _, mod := range shown[first:last] {
			m := mod
			a.addonRow(c, a.modItem(m), false, a.toggleModFn(m), func() {
				if m.Latest != nil && fixedButton(c, "update-"+m.File, t.ModsUpdateBtn, "download", 0, btnSm*s, false, false) {
					a.execute("installMod", map[string]string{"rootId": a.root().ID, "source": m.Source, "projectId": m.ProjectID, "versionId": m.Latest.ID, "replace": m.File}, func(json.RawMessage) {
						a.notice(a.tr().ModsUpdateQueued)
					})
				}
				if iconOnlyButton(c, "remove-"+m.File, "cross", t.ModsRemoveBtn, btnSm*s, a.pendingAt("removeMod", "rootId", a.root().ID, "name", m.File)) {
					a.form.Name = m.File
					a.dialog = "removeMod"
					a.dialogOpen = true
				}
			})
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

// modWindow is the slice of fixed-height mod rows near the viewport; rows
// outside it are replaced by spacers so 200+ mods stay cheap to lay out.
// Before the first layout it draws a screenful from the top.
func modWindow(n int, rowH float32, list, view ui.Rect) (int, int) {
	const minVirtual, overscan = 60, 12
	if n <= minVirtual || rowH <= 0 {
		return 0, n
	}
	if list.H <= 0 || view.H <= 0 {
		return 0, min(n, 40)
	}
	top := view.Y - list.Y
	first := max(0, int(top/rowH)-overscan)
	last := min(n, int((top+view.H)/rowH)+1+overscan)
	if first > last {
		first = last
	}
	return first, last
}
