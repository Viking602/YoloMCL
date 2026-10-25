package nativeui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestModCacheSettingsClearsDiskAndQueuedLists(t *testing.T) {
	a, tt := settingsTester(t)
	cacheFile := filepath.Join(a.snapshot.DataDir, "cache", "mod-metadata.bin")
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cacheFile, []byte("disposable"), 0600); err != nil {
		t.Fatal(err)
	}
	a.cacheAddons("root|pack|mods", addonCacheEntry{mods: []launcher.Mod{{File: "cached.jar"}}})
	a.mods = []launcher.Mod{{File: "in-flight.jar"}}
	a.addonTab = "mods"
	gen := a.modsGen
	a.refreshModCacheUsage()
	waitUI(t, tt, func() bool { return !a.modCacheLoading })
	if a.modCacheInfo.Bytes != 10 {
		t.Fatal(a.modCacheInfo)
	}
	tt.Scroll(700, 400, 0, 10000)
	snapshotUI(t, tt, "mod-cache-settings")
	clickUI(t, tt, "清理缓存")
	waitUI(t, tt, func() bool { return !a.modCacheClearing })
	if _, err := os.Stat(cacheFile); !os.IsNotExist(err) {
		t.Fatal("cache not removed", err)
	}
	if a.modsGen <= gen || len(a.mods) != 0 || len(a.addonCache) != 0 || a.addonCacheBytes != 0 || a.modCacheInfo.Bytes != 0 {
		t.Fatal("stale mod state retained")
	}
	if a.errorMessage {
		t.Fatal(a.message)
	}
}
