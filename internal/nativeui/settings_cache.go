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
				a.notice(a.L("settings_cache.mod_metadata_cache_cleared_it_will_rebuild_when"))
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
	settingsGroup(c, "mod-cache-settings", a.L("settings_cache.cache"), "", func() {
		desc := a.L("settings_cache.up_to_8_mib_shared_and_4_mib")
		settingRow(c, nil, a.L("settings_cache.mod_metadata_cache"), desc, func() {
			ui.Row(c).Gap(12 * s).AlignItems(ui.Center).Children(func() {
				usage := fmt.Sprintf("%.1f MiB", float64(a.modCacheInfo.Bytes)/(1<<20))
				if a.modCacheLoading {
					usage = a.L("settings_cache.checking")
				}
				ui.Text(c, usage).FontSize(typeFootnote * s).TextColor(muted).NoWrap()
				label := a.L("settings_cache.clear_cache")
				if a.modCacheClearing {
					label = a.L("settings_cache.clearing")
				}
				if fixedButton(c, "clear-mod-cache", label, "trash", 0, btnSm*s, false, a.modCacheClearing) {
					a.clearModCache()
				}
			})
		}, nil)
	})
}
