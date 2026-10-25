package nativeui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// discoverView is the native Discover page: the same split as 实例 (a
// rail on the left, an inset panel on the right) instead of a centered
// web-style column with a page heading.
func (a *App) discoverView(c *ui.Context) {
	s := uiScale(c)
	a.normalizeStateValues()
	a.ensureZhNames()
	ui.Row(c.Key("discover-split")).Label("发现视图").Fill().Gap(0).AlignItems(ui.Stretch).Children(func() {
		a.discoverRail(c)
		contentPanel(ui.Column(c.Key("discover-panel")), s).Grow(1).Basis(0).MinWidth(0).Children(func() {
			a.discoverToolbar(c)
			sc := ui.Scroll(c.Key("page-scroll-discover")).TrackScroll(&a.discoverMotion.scroll.st)
			sc = overlayScrollbar(sc, &a.discoverMotion.scroll)
			a.discoverMotion.viewport = sc.LayoutBounds()
			sc.Grow(1).MinHeight(0).Padding(4*s, pageGutter*s, 28*s).Children(func() {
				ui.Column(c).FillWidth().Gap(16 * s).Children(func() { a.discoverResults(c) })
			})
		})
	})
	a.settleDiscoverCategories(c)
}

func (a *App) discoverRail(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	ui.Column(c.Key("discover-rail")).Label("发现筛选").Width(instListWidth*s).Shrink(0).FillHeight().Padding(20*s, 14*s, 16*s, 16*s).Gap(18 * s).Children(func() {
		// Source: one segmented control with the providers' own logos.
		ui.Row(c.Key("discover-source")).Label("来源").FillWidth().Padding(3 * s).Gap(3 * s).Radius(11 * s).Background(fieldColor).Children(func() {
			for _, src := range []struct {
				key, name string
				logo      *ui.SVG
			}{{"modrinth", "Modrinth", modrinthLogo}, {"curseforge", "CurseForge", curseforgeLogo}} {
				it := src
				sel := a.provider == it.key
				b := ui.ButtonBase(c.Key("brand-" + it.key)).Label(it.name).Grow(1).Basis(0).Height(30 * s).Gap(7 * s).Radius(8 * s).Justify(ui.Center).AlignItems(ui.Center)
				bg := ui.Transparent
				if sel {
					bg = panel
					b.Shadow(0, 1*s, 4*s, 0, ui.RGBA(15, 23, 42, .08))
				} else if b.Hovered() {
					bg = ui.RGBA(255, 255, 255, .55)
				}
				b.Background(b.AnimateColor("bg", bg, fastAnim))
				fg := inkSecondary
				if sel {
					fg = ink
				}
				if b.Children(func() {
					ui.Image(c, it.logo).Size(15*s, 15*s).Fit(ui.Contain).Shrink(0)
					ui.Text(c, it.name).FontSize(typeFootnote * s).FontWeight(600).TextColor(fg).NoWrap()
				}).Clicked() && !sel {
					a.provider = it.key
					a.search()
				}
			}
		})
		// Type: rows like the instance list.
		ui.Column(c).FillWidth().Gap(2 * s).Children(func() {
			railTitle(c, a.L("类型", "Type"))
			for _, ty := range []struct{ label, key, icon string }{
				{t.DiscoverTypeModpack, "modpack", "layers"},
				{t.DiscoverTypeMod, "mod", "mods"},
				{t.DiscoverTypeRP, "resourcepack", "cube"},
				{t.DiscoverTypeShader, "shader", "sun"},
			} {
				it := ty
				sel := normalizeResourceType(a.resourceType) == it.key
				b := ui.ButtonBase(c.Key("discover-type-" + it.key)).Label(it.label).FillWidth().Height(40 * s).PaddingX(10 * s).Gap(11 * s).Radius(radiusRow * s).AlignItems(ui.Center)
				bg, fg := ui.Transparent, ink
				if sel {
					bg, fg = accent, panel
				} else if b.Hovered() {
					bg = ui.RGBA(255, 255, 255, .8)
				}
				b.Background(b.AnimateColor("bg", bg, fastAnim))
				if b.Children(func() {
					ui.Icon(c, icons[it.icon]).Size(15*s, 15*s).TextColor(fg).Shrink(0)
					ui.Text(c, it.label).FontSize(typeBody * s).FontWeight(600).TextColor(fg).SingleLine()
					ui.Spacer(c).Grow(1)
				}).Clicked() && !sel {
					a.resourceType = it.label
					a.search()
				}
			}
		})
		ui.Column(c).FillWidth().Gap(8 * s).Children(func() {
			railTitle(c, a.L("筛选", "Filters"))
			a.discoverVersionFilter(c)
			kind := normalizeResourceType(a.resourceType)
			if kind == "mod" || kind == "modpack" {
				if loaderSelectBox(c, "discover-loader", t.DiscoverFilterLoader, &a.filterLoader, []string{t.LoaderAll, t.LoaderFabric, t.LoaderForge, t.LoaderNeoForge, t.LoaderQuilt}).FillWidth().Changed() {
					a.search()
				}
			}
			if selectBox(c, "discover-sort", t.DiscoverSortRelevance, &a.searchSort, []string{t.DiscoverSortRelevance, t.DiscoverSortDownloads, t.DiscoverSortUpdated}).FillWidth().Changed() {
				a.search()
			}
			if v := a.gameVersion(); v != nil && kind != "modpack" && a.filterMC != v.Minecraft {
				b := ui.ButtonBase(c.Key("discover-use-inst")).Label(a.L("用当前实例的版本", "Use this instance's version")).Height(28 * s).PaddingX(8 * s).Radius(7 * s).Gap(6 * s).AlignItems(ui.Center)
				bg := ui.Transparent
				if b.Hovered() {
					bg = accentSoft
				}
				b.Background(b.AnimateColor("bg", bg, fastAnim))
				if b.Children(func() {
					ui.Icon(c, icons["refresh"]).Size(11*s, 11*s).TextColor(accent)
					ui.Text(c, fmt.Sprintf(a.L("用当前实例的版本 %s", "Use this instance's %s"), v.Minecraft)).FontSize(typeFootnote * s).FontWeight(600).TextColor(accent).NoWrap()
				}).Clicked() {
					a.filterMC = v.Minecraft
					a.discoverMCText = v.Minecraft
					if kind == "mod" {
						a.filterLoader = a.loaderDisplay(v.Loader)
					}
					a.search()
				}
			}
		})
		a.discoverCatsRail(c)
		ui.Spacer(c).Grow(1)
		if r := a.root(); r != nil && normalizeResourceType(a.resourceType) != "modpack" {
			ui.Row(c).FillWidth().Padding(10*s, 12*s).Radius(10 * s).Background(ui.RGBA(120, 130, 160, .07)).Gap(10 * s).AlignItems(ui.Center).Children(func() {
				a.instIcon(c, *r, a.gameVersion(), 26*s)
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(2 * s).Children(func() {
					label(c, a.L("安装到", "Installs to")).FontSize(typeCaption * s)
					name := r.Name
					if v := a.gameVersion(); v != nil {
						name, _ = a.cardText(*r, *v)
					}
					ui.Text(c, name).FontSize(typeFootnote * s).FontWeight(600).TextColor(ink).SingleLine().FillWidth().MinWidth(0)
				})
			})
		}
	})
}

func railTitle(c *ui.Context, text string) {
	s := uiScale(c)
	ui.Text(c, text).FontSize(typeFootnote * s).FontWeight(650).TextColor(muted).PaddingX(10 * s).PaddingY(4 * s).SingleLine()
}

func (a *App) discoverToolbar(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	ui.Row(c.Key("discover-toolbar")).Label("搜索栏").FillWidth().Padding(18*s, pageGutter*s, 12*s).Gap(10 * s).AlignItems(ui.Center).Shrink(0).Children(func() {
		in := textField(c.Key("discover-query"), &a.query).Placeholder(t.DiscoverSearchHint).Label(t.DiscoverSearchAria).Grow(1).Basis(0).MinWidth(160 * s)
		submit := in.Submitted()
		if in.Shortcut(0, ui.KeyEnter) {
			submit = true
		}
		if strings.TrimSpace(a.query) != "" {
			cb := ui.ButtonBase(c.Key("discover-query-clear")).Label(a.L("清空搜索", "Clear search")).Size(24*s, 24*s).Radius(999).Center().Shrink(0)
			if cb.Hovered() {
				cb.Background(ui.RGBA(120, 130, 160, .14))
			}
			if cb.Children(func() {
				ui.Icon(c, icons["cross"]).Size(10*s, 10*s).TextColor(muted)
			}).Clicked() {
				a.query = ""
				submit = true
			}
		}
		if fixedButton(c, "search-now", t.DiscoverSearchBtn, "search", 0, btnSm*s+4*s, true, false) {
			submit = true
		}
		if submit {
			a.search()
		}
		ui.Box(c.Key("search-activity")).Label("搜索加载指示").Size(14*s, 14*s).Shrink(0).Children(func() {
			if a.searchLoading {
				ui.Spinner(c).Size(14*s, 14*s)
			}
		})
		count := "—"
		if len(a.searchResult.Projects) > 0 || (!a.searchLoading && a.searchError == "") {
			count = a.formatResultCount(int64(a.searchResult.Total))
		}
		a.rollNumber(c, "discover-total", count, typeFootnote*s, 400, muted, true).Width(98 * s).Shrink(0)
	})
}

// installedProject reports whether the current instance already has the
// project (mods matched by source and project ID).
func (a *App) installedProject(p launcher.Project) bool {
	if p.Type != "mod" {
		return false
	}
	r := a.root()
	if r == nil {
		return false
	}
	mods := a.mods
	if e, ok := a.addonCache[r.ID+"|"+r.SelectedVersion+"|mods"]; ok {
		mods = e.mods
	}
	for _, m := range mods {
		if m.ProjectID == p.ID && strings.EqualFold(m.Source, p.Source) {
			return true
		}
	}
	return false
}

func (a *App) discoverResults(c *ui.Context) {
	t := a.tr()
	a.discoverMotion.revealing = false
	a.syncDiscoverCards(c, a.searchResult.Projects)
	if a.searchError != "" {
		style := failPanel
		if len(a.searchResult.Projects) > 0 {
			style = failInline
		}
		if a.loadFailed(c, "search-error", style, a.searchLoading) {
			a.searchAt(a.discoverMotion.request.Offset)
		}
		if len(a.searchResult.Projects) == 0 {
			return
		}
	}
	if len(a.searchResult.Projects) == 0 {
		if a.searchLoading {
			a.discoverSkeleton(c)
		} else {
			emptyState(c, "search-empty", "search", t.DiscoverEmptyTitle, t.DiscoverEmptyDesc, nil)
		}
		return
	}
	// One grid parent preserves card identity across rows and column counts.
	// Same-scope refreshes keep their cards; provider/type switches show loading.
	grid := ui.Box(c.Key("discover-grid")).Label("搜索结果列表").FillWidth()
	cols := discoverColumns(c)
	ps := a.searchResult.Projects
	a.discoverFeedGrid(c, grid, cols, ps)
	a.discoverFeedFooter(c)
}

func (a *App) discoverCard(c *ui.Context, p launcher.Project, index, columns int) ui.Element {
	s := uiScale(c)
	b := ui.ButtonBase(c.Key("card-"+discoverProjectKey(p))).Label("搜索结果 "+p.Title).MinWidth(0).Padding(16*s, 16*s, 14*s).Gap(12 * s).Radius(radiusCard * s).Column().Font(uiSansFamily())
	state := a.discoverMotion.cards[discoverProjectKey(p)]
	a.animateDiscoverCard(c, b, state, index, columns)
	ready := state.progress > .15
	bg := panel
	if b.Hovered() {
		b.Shadow(0, 6*s, 18*s, 0, ui.RGBA(30, 40, 80, .09))
	} else {
		b.Shadow(0, 1*s, 3*s, 0, ui.RGBA(30, 40, 80, .05))
	}
	b.Background(bg).Border(1, ui.RGBA(60, 70, 110, .08))
	installed := a.installedProject(p)
	if b.Children(func() {
		ui.Row(c).FillWidth().Gap(12 * s).AlignItems(ui.Center).Children(func() {
			a.discoverIcon(c, p, state)
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(3 * s).Children(func() {
				title, original := a.zhTitle(a.zhNames.Project(p), p.Title)
				tooltip(c, ui.Text(c, title).FontSize(typeBody*s).FontWeight(650).TextColor(ink).SingleLine().FillWidth().MinWidth(0), p.Title)
				if original != "" || p.Author != "" {
					ui.Row(c).FillWidth().MinWidth(0).Gap(8 * s).AlignItems(ui.Center).Children(func() {
						if original != "" {
							ui.Text(c, original).FontSize(typeCaption * s).FontWeight(550).TextColor(inkSecondary).SingleLine().Shrink(1).MinWidth(0)
						}
						if p.Author != "" {
							ui.Row(c).Gap(3 * s).AlignItems(ui.Center).Shrink(2).MinWidth(0).Children(func() {
								ui.Icon(c, icons["account"]).Size(9*s, 9*s).TextColor(muted).Shrink(0)
								tooltip(c, ui.Text(c, p.Author).FontSize(typeCaption*s).TextColor(muted).SingleLine().Shrink(1).MinWidth(0), a.L("作者 ", "By ")+p.Author)
							})
						}
					})
				}
			})
		})
		desc := strings.Join(strings.Fields(p.Description), " ")
		ui.Box(c).FillWidth().MinWidth(0).Height(38 * s).Children(func() {
			ui.Text(c, desc).FontSize(typeFootnote * s).TextColor(inkSecondary).FillWidth().MinWidth(0).MaxLines(2).Ellipsis("…")
		})
		// Whole chips only, on their own row: one that would not fit is left out.
		tagRow := ui.Row(c.Key("dtags-" + p.Source + p.ID)).FillWidth().Height(18 * s).Clip().Gap(6 * s).AlignItems(ui.Center)
		room := tagRow.LayoutBounds().W
		tagRow.Children(func() {
			for _, ch := range a.cardChips(p.Categories, room, s) {
				cat, l := ch[0], ch[1]
				id := a.catIDFor(cat)
				if id == "" {
					catTag(c, cat, l, false)
					continue
				}
				tb := tooltip(c, ui.ButtonBase(c.Key("ctag-"+p.ID+"-"+id)).Label(l), a.L("按此分类筛选", "Filter by this tag")).Radius(5 * s)
				if tb.Children(func() { catTag(c, cat, l, a.catSelected(id)) }).Clicked() && ready && !a.searchLoading && !a.catSelected(id) {
					a.toggleCat(id)
				}
			}
		})
		ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Gap(4 * s).AlignItems(ui.Center).Shrink(0).Children(func() {
				ui.Icon(c, icons["download"]).Size(11*s, 11*s).TextColor(muted)
				numText(c, compactCount(p.Downloads), typeCaption*s, muted)
			})
			if relative, exact := a.resourceTime(c, p.Updated); relative != "" {
				tooltip(c, numText(c, relative, typeCaption*s, muted).Shrink(0), fmt.Sprintf(a.L("更新时间：%s", "Updated: %s"), exact))
			}
			ui.Spacer(c).Grow(1)
			if installed {
				ui.Row(c).Height(28 * s).PaddingX(10 * s).Gap(5 * s).Radius(8 * s).Background(ui.RGBA(52, 199, 89, .12)).AlignItems(ui.Center).Shrink(0).Children(func() {
					ui.Icon(c, icons["check"]).Size(10*s, 10*s).TextColor(successInk)
					ui.Text(c, a.L("已安装", "Installed")).FontSize(typeFootnote * s).FontWeight(600).TextColor(successInk).NoWrap()
				})
			} else {
				text := a.L("下载", "Download")
				if fixedButton(c, "view-"+p.ID, text, "download", 0, 28*s, true, false) && ready {
					a.openProject(p)
				}
			}
		})
	}).Clicked() && ready {
		a.openProject(p)
	}
	return b
}

// compactCount is 19.1M / 840K / 512 without a unit word.
func compactCount(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.0fK", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}

// cardChips picks category labels that fit whole in room (0 = unknown,
// allow two). In Chinese, untranslated platform names are skipped.
func (a *App) cardChips(cats []string, room, s float32) [][2]string {
	var out [][2]string
	used := float32(0)
	for _, cat := range cats {
		if len(out) >= 2 || isLoaderCategory(cat) {
			continue
		}
		l := a.translateCategory(cat)
		if a.isEN() {
			l = categoryTitle(cat)
		} else if !hasCJK(l) {
			continue
		}
		w := chipWidth(l, s)
		if room > 0 && used+w > room {
			continue
		}
		used += w + 6*s
		out = append(out, [2]string{cat, l})
	}
	return out
}

func chipWidth(l string, s float32) float32 {
	w := float32(0)
	for _, r := range l {
		if r > 0x2e80 {
			w += typeCaption2 * s
		} else {
			w += typeCaption2 * s * .58
		}
	}
	return w + 14*s
}
