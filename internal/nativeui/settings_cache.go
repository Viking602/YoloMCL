package nativeui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
)

func (a *App) refreshModCacheUsage() {
	if a.modCacheClearing || a.modCacheLoading {
		return
	}
	a.modCacheRevision++
	revision := a.modCacheRevision
	a.modCacheLoading = true
	go func() {
		info, err := a.service.ModCacheUsage(a.ctx)
		a.post(func() {
			if revision != a.modCacheRevision {
				return
			}
			a.modCacheLoading = false
			if err == nil {
				a.modCacheInfo = info
			}
		})
	}()
}

func (a *App) clearModCache() {
	if a.modCacheClearing {
		return
	}
	a.modCacheRevision++
	a.modCacheClearing, a.modCacheLoading = true, false
	a.discardModCache()
	go func() {
		err := a.service.ClearModCache(a.ctx)
		info, _ := a.service.ModCacheUsage(a.ctx)
		a.post(func() {
			a.modCacheClearing = false
			a.modCacheInfo = info
			// Navigation may have started another scan while cleanup ran.
			// Drop its queued results, then reload only if mods are visible.
			a.discardModCache()
			if err != nil {
				a.notice(a.friendlyErr(err))
				a.errorMessage = true
			} else {
				a.notice(a.L("模组信息缓存已清理，下次打开时自动重建", "Mod metadata cache cleared; it will rebuild when needed"))
			}
			if a.page == "home" && a.instTab == "mods" {
				a.ensureAddons()
			}
		})
	}()
}

func (a *App) discardModCache() {
	a.cancelAddonLoad()
	a.mods = nil
	for key, entry := range a.addonCache {
		if strings.HasSuffix(key, "|mods") {
			a.addonCacheBytes -= entry.bytes
			delete(a.addonCache, key)
		}
	}
	a.addonOrder = slices.DeleteFunc(a.addonOrder, func(key string) bool { return strings.HasSuffix(key, "|mods") })
	if a.addonTab == "mods" {
		a.modsKey, a.modsLoadKey = "", ""
	}
	a.modIconAsked = nil
	a.pruneAddonMetadata()
}

func (a *App) modCacheSettings(c *ui.Context) {
	s := uiScale(c)
	settingsGroup(c, "mod-cache-settings", a.L("缓存", "Cache"), "", func() {
		desc := a.L("共享缓存最多 8 MiB，每个实例最多 4 MiB；清理后按需重建", "Up to 8 MiB shared and 4 MiB per instance; rebuilt when needed")
		settingRow(c, nil, a.L("模组信息缓存", "Mod metadata cache"), desc, func() {
			ui.Row(c).Gap(12 * s).AlignItems(ui.Center).Children(func() {
				usage := fmt.Sprintf("%.1f MiB", float64(a.modCacheInfo.Bytes)/(1<<20))
				if a.modCacheLoading {
					usage = a.L("统计中…", "Checking…")
				}
				ui.Text(c, usage).FontSize(typeFootnote * s).TextColor(muted).NoWrap()
				label := a.L("清理缓存", "Clear cache")
				if a.modCacheClearing {
					label = a.L("清理中…", "Clearing…")
				}
				if fixedButton(c, "clear-mod-cache", label, "trash", 0, btnSm*s, false, a.modCacheClearing) {
					a.clearModCache()
				}
			})
		}, nil)
	})
}
