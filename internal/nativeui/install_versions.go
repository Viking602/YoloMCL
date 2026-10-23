package nativeui

import (
	"context"
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
			}
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
	if loader == "vanilla" || mc == "" {
		return
	}
	a.loaderLoading = true
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		defer cancel()
		items, e := a.service.LoaderVersions(ctx, loader, mc)
		a.post(func() {
			if revision != a.loaderRevision || a.dialog != "install" {
				return
			}
			a.loaderLoading = false
			if e != nil {
				a.loaderError = a.friendlyErr(e)
				return
			}
			a.loaders = items
			if len(items) > 0 {
				a.form.LoaderVersion = items[0]
			}
		})
	}()
}

func (a *App) minecraftVersionPicker(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	ui.Row(c).FillWidth().Gap(12 * s).AlignItems(ui.Center).Children(func() {
		ui.Text(c, t.InstallMCLabel).FontWeight(600).TextColor(ink).NoWrap().Shrink(0)
		label(c, a.formatVersionTotal(len(a.minecraftVersions))).FontSize(typeCallout * s).Grow(1).NoWrap()
		if button(c, "refresh-minecraft", t.InstallRefreshBtn, "refresh", false, a.manifestLoading) {
			a.loadManifest()
		}
	})
	ui.Row(c).FillWidth().Gap(12 * s).Children(func() {
		if selectBox(c, "minecraft-kind", t.InstallKindAria, &a.versionKind, a.versionKindOptions()).Width(170 * s).Shrink(0).Changed() {
			a.applyMinecraftFilter()
		}
		if textField(c.Key("minecraft-query"), &a.versionQuery).Label(t.InstallQueryAria).Placeholder(t.InstallQueryHint).Grow(1).MinWidth(0).Changed() {
			a.applyMinecraftFilter()
		}
	})
	items := filterMinecraftVersions(a.minecraftVersions, a.versionKind, a.versionQuery)
	switch {
	case a.manifestLoading && len(a.minecraftVersions) == 0:
		statusLine(c, t.InstallFetchingManifest, muted, true)
	case a.manifestError != "":
		if a.loadFailed(c, "manifest-failed", failInline, a.manifestLoading) {
			a.loadManifest()
		}
	case len(items) == 0:
		statusLine(c, t.InstallNoMatch, muted, false)
	default:
		ids := make([]string, len(items))
		for i, v := range items {
			ids[i] = v.ID
		}
		if selectBox(c, "minecraft-version", t.InstallVersionAria, &a.form.Minecraft, ids).FillWidth().Changed() {
			a.loadLoaderVersions()
		}
		for _, v := range items {
			if v.ID == a.form.Minecraft {
				detail := a.versionKindLabel(minecraftKindKey(v.Type))
				if len(v.ReleaseTime) >= 10 {
					detail += "   " + v.ReleaseTime[:10]
				}
				label(c, a.formatFilteredVersionSummary(detail, len(items))).FontSize(typeCallout * s).NoWrap()
				break
			}
		}
	}
}
