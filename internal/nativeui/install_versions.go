package nativeui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func minecraftKindKey(kind string) string {
	switch kind {
	case "release":
		return "release"
	case "snapshot":
		return "snapshot"
	case "old_beta":
		return "old_beta"
	case "old_alpha":
		return "old_alpha"
	}
	return "other"
}

func minecraftKind(kind string) string {
	switch minecraftKindKey(kind) {
	case "release":
		return "正式版"
	case "snapshot":
		return "快照 / 预发布"
	case "old_beta":
		return "历史 Beta"
	case "old_alpha":
		return "历史 Alpha"
	}
	return "其他版本"
}

func filterMinecraftVersions(items []launcher.MinecraftVersion, kind, query string) []launcher.MinecraftVersion {
	out := []launcher.MinecraftVersion{}
	query = strings.ToLower(strings.TrimSpace(query))
	targetKey := normalizeVersionKindKey(kind)
	for _, item := range items {
		itemKey := minecraftKindKey(item.Type)
		if targetKey != "all" && itemKey != targetKey {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(item.ID+" "+item.Type+" "+minecraftKind(item.Type)+" "+itemKey), query) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (a *App) applyMinecraftFilter() {
	items := filterMinecraftVersions(a.minecraftVersions, a.versionKind, a.versionQuery)
	previous := a.form.Minecraft
	if !slices.ContainsFunc(items, func(v launcher.MinecraftVersion) bool { return v.ID == previous }) {
		a.form.Minecraft = ""
		if len(items) > 0 {
			a.form.Minecraft = items[0].ID
		}
	}
	if previous != a.form.Minecraft {
		a.installVerScrollPending = true
		a.loadLoaderVersions()
	}
}

func (a *App) loadManifest() {
	if a.manifestLoading || a.service == nil {
		return
	}
	a.manifestRevision++
	revision := a.manifestRevision
	a.manifestLoading = true
	a.manifestError = ""
	ready := a.firstFrame
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		if !waitForFirstFrame(a.ctx, ready) {
			return
		}
		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		defer cancel()
		manifest, e := a.service.Manifest(ctx)
		a.post(func() {
			if revision != a.manifestRevision {
				return
			}
			a.manifestLoading = false
			if e != nil {
				a.manifestError = a.friendlyErr(e)
				return
			}
			a.minecraftVersions = manifest.Versions
			if a.dialog != "install" || !a.dialogOpen {
				return
			}
			if a.form.Minecraft == "" {
				a.form.Minecraft = manifest.Latest.Release
				a.dialogInitial.Minecraft = a.form.Minecraft
			}
			a.installVerScrollPending = true
			a.applyMinecraftFilter()
		})
	}()
}

func (a *App) loadLoaderVersions() {
	a.loaderRevision++
	revision := a.loaderRevision
	a.loaders = nil
	a.loaderError = ""
	a.loaderLoading = false
	a.form.LoaderVersion = ""
	loader, mc := loaderKey(a.form.Loader), a.form.Minecraft
	if loader == "vanilla" || mc == "" || a.service == nil {
		return
	}
	a.loaderLoading = true
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		defer cancel()
		items, e := a.service.LoaderVersions(ctx, loader, mc)
		var horizonMC string
		if loader == "horizon" && e == nil {
			horizonMC = a.service.HorizonMinecraftVersion(ctx)
		}
		a.post(func() {
			if revision != a.loaderRevision || a.dialog != "install" {
				return
			}
			a.loaderLoading = false
			if horizonMC != "" {
				a.horizonTargetMC = horizonMC
			}
			if e != nil {
				a.loaderError = a.friendlyErr(e)
				return
			}
			a.loaders = items
			if len(items) > 0 {
				a.form.LoaderVersion = items[0]
				a.dialogInitial.LoaderVersion = a.form.LoaderVersion
			}
		})
	}()
}

func (a *App) selectedMinecraftVersion() *launcher.MinecraftVersion {
	for i := range a.minecraftVersions {
		if a.minecraftVersions[i].ID == a.form.Minecraft {
			return &a.minecraftVersions[i]
		}
	}
	return nil
}

func (a *App) latestReleaseVersionID() string {
	for _, v := range a.minecraftVersions {
		if v.Type == "release" {
			return v.ID
		}
	}
	return ""
}

func (a *App) versionKindShort(kind string) string {
	switch minecraftKindKey(kind) {
	case "release":
		return a.L("install.kind_short.release")
	case "snapshot":
		return a.L("install.kind_short.snapshot")
	case "old_beta":
		return a.L("install.kind_short.old_beta")
	case "old_alpha":
		return a.L("install.kind_short.old_alpha")
	default:
		return a.L("install.kind_short.other")
	}
}

func (a *App) installVersionRail(c *ui.Context, railW float32) {
	s := uiScale(c)
	t := a.tr()
	items := filterMinecraftVersions(a.minecraftVersions, a.versionKind, a.versionQuery)
	latestRelease := a.latestReleaseVersionID()
	showKindTag := normalizeVersionKindKey(a.versionKind) == "all"

	ui.Column(c.Key("install-version-rail")).Width(railW).Shrink(0).FillHeight().Gap(0).Children(func() {
		// Rail header, search box and category filter.
		ui.Column(c).FillWidth().Shrink(0).Padding(16*s, 14*s, 12*s, 16*s).Gap(10 * s).Children(func() {
			ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
				ui.Text(c, t.InstallMCLabel).FontSize(typeSubhead * s).FontWeight(700).TextColor(ink).SingleLine().Grow(1).MinWidth(0)
				if len(a.minecraftVersions) > 0 {
					countText := fmt.Sprintf("%d", len(items))
					if len(items) != len(a.minecraftVersions) {
						countText = fmt.Sprintf("%d / %d", len(items), len(a.minecraftVersions))
					}
					tagChip(c, countText)
				}
				if iconOnlyButton(c, "refresh-minecraft", "refresh", t.InstallRefreshBtn, btnSm*s, a.manifestLoading) {
					a.loadManifest()
				}
			})
			if textField(c.Key("minecraft-query"), &a.versionQuery).Label(t.InstallQueryAria).Placeholder(t.InstallQueryHint).FillWidth().Changed() {
				a.applyMinecraftFilter()
				a.installVerScrollPending = true
			}
			if selectBox(c, "minecraft-kind", t.InstallKindAria, &a.versionKind, a.versionKindOptions()).FillWidth().Changed() {
				a.applyMinecraftFilter()
				a.installVerScrollPending = true
			}
		})

		ui.Box(c).FillWidth().Height(1).Margin(0, 14*s, 0, 16*s).Background(line).Shrink(0)

		// Virtualized version list.
		switch {
		case a.manifestLoading && len(a.minecraftVersions) == 0:
			ui.Column(c).FillWidth().Grow(1).MinHeight(0).Padding(24 * s).AlignItems(ui.Center).Justify(ui.Center).Gap(10 * s).Children(func() {
				statusSpinner(c, 20*s)
				ui.Text(c, t.InstallFetchingManifest).FontSize(typeFootnote * s).TextColor(muted).TextAlign(ui.Center)
			})
		case a.manifestError != "":
			ui.Column(c).FillWidth().Grow(1).MinHeight(0).Padding(16 * s).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				if a.loadFailed(c, "manifest-failed", failInline, a.manifestLoading) {
					a.loadManifest()
				}
			})
		case len(items) == 0:
			ui.Column(c).FillWidth().Grow(1).MinHeight(0).Padding(24*s, 18*s).AlignItems(ui.Center).Justify(ui.Center).Gap(8 * s).Children(func() {
				ui.Icon(c, icons["search"]).Size(22*s, 22*s).TextColor(muted.Alpha(.6))
				bodyText(c, t.InstallNoMatch).FontSize(typeFootnote * s).TextColor(muted).TextAlign(ui.Center)
			})
		default:
			if a.installVerScrollPending {
				for i, mv := range items {
					if mv.ID == a.form.Minecraft {
						a.installVerList.ScrollTo(i, ui.Center)
						break
					}
				}
				a.installVerScrollPending = false
			}
			track := scrollTracks["install-mc-list"]
			if track == nil {
				track = &scrollTrack{}
				scrollTracks["install-mc-list"] = track
			}
			list := ui.List(c.Key("install-mc-list"), &a.installVerList, len(items)).
				ItemKey(func(i int) any { return items[i].ID }).TrackScroll(&track.st).
				Grow(1).MinHeight(0).FillWidth().Padding(8*s, 12*s, 16*s, 12*s).Gap(4 * s)
			overlayScrollbar(list, track).Rows(func(row ui.ListRow) {
				rc, v := row.Context, items[row.Index]
				sel := v.ID == a.form.Minecraft
				b := ui.ButtonBase(rc.Key("mc-ver-"+v.ID)).Label(v.ID).FillWidth().Height(48*s).Padding(0, 12*s, 0, 14*s).Radius(radiusRow * s).AlignItems(ui.Center)
				bg, edge, fg, sub := ui.Transparent, ui.Transparent, ink, muted
				if sel {
					bg, edge, fg, sub = panel, lineStrong, ink, inkSecondary
					b.Shadow(0, s, 2*s, 0, ui.RGBA(0, 0, 0, .04))
					b.Draw(func(p *ui.Painter, rect ui.Rect) {
						barH := rect.H * .44
						p.Fill(ui.Rect{X: rect.X + 3*s, Y: rect.Y + (rect.H-barH)/2, W: 2.5 * s, H: barH}, lobePrimary, 1.25*s)
					})
				} else if b.Hovered() {
					bg = lobeLight.FillTertiary
				}
				b.Background(b.AnimateColor("bg", bg, fastAnim)).Border(1, b.AnimateColor("edge", edge, fastAnim))
				if b.Children(func() {
					ui.Column(rc).Grow(1).Basis(0).MinWidth(0).Gap(2 * s).Children(func() {
						ui.Row(rc).FillWidth().MinWidth(0).Gap(6 * s).AlignItems(ui.Center).Children(func() {
							weight := 550
							if sel {
								weight = 650
							}
							ui.Text(rc, v.ID).FontSize(typeCallout * s).FontWeight(weight).TextColor(fg).SingleLine().Shrink(1).MinWidth(0)
							if v.ID == latestRelease {
								ui.Row(rc).Shrink(0).Padding(2*s, 6*s).Radius(radiusChip*s).Background(lobeLight.InfoBg).Border(1, lobeLight.InfoBorder).Children(func() {
									ui.Text(rc, a.L("install.latest_badge")).FontSize(typeCaption2 * s).FontWeight(600).TextColor(lobeLight.InfoText).NoWrap()
								})
							}
							if showKindTag || v.Type != "release" {
								ui.Spacer(rc).Grow(1)
								ui.Row(rc).Shrink(0).Padding(2*s, 6*s).Radius(radiusChip*s).Background(raised).Border(1, line).Children(func() {
									ui.Text(rc, a.versionKindShort(v.Type)).FontSize(typeCaption2 * s).FontWeight(500).TextColor(inkSecondary).NoWrap()
								})
							}
						})
						if len(v.ReleaseTime) >= 10 {
							ui.Text(rc, v.ReleaseTime[:10]).FontSize(typeCaption * s).TextColor(sub).SingleLine()
						}
					})
				}).Clicked() && !sel {
					a.form.Minecraft = v.ID
					a.loadLoaderVersions()
				}
			})
		}
	})
}
