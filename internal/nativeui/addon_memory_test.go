package nativeui

import (
	"context"
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestAddonCacheBoundsReleaseMetadataAndKeepRecentLists(t *testing.T) {
	a := &App{addonRemote: map[string]launcher.RemoteInfo{}, modIconURLs: map[string]string{}, modIconAsked: map[string]bool{}}
	add := func(id string) {
		a.addonRemote[id] = launcher.RemoteInfo{Summary: id}
		a.modIconURLs[id] = "icon-" + id
		a.modIconAsked["root|"+id+"|1"] = true
		a.cacheAddons("root|"+id+"|mods", addonCacheEntry{mods: []launcher.Mod{{SHA1: id,
			Latest: &launcher.ProjectVersion{Changelog: strings.Repeat(id, addonCacheBudget/3)},
		}}})
	}
	add("a")
	add("b")
	a.touchAddons("root|a|mods")
	add("c")
	if a.addonCacheBytes > addonCacheBudget || len(a.addonCache) != 2 || len(a.addonCache["root|a|mods"].mods) != 1 {
		t.Fatal("large historical release data escaped the cache budget or evicted the recently used list")
	}
	if _, ok := a.addonRemote["b"]; ok || a.modIconURLs["b"] != "" || a.modIconAsked["root|b|1"] {
		t.Fatal("evicting the old list retained its separate platform metadata")
	}
	if a.addonRemote["a"].Summary != "a" || a.addonRemote["c"].Summary != "c" {
		t.Fatal("eviction discarded metadata for a retained list")
	}
	// A single unusually large pack stays usable instead of continuously
	// evicting itself and decoding it again on every tab switch.
	a.cacheAddons("huge", addonCacheEntry{mods: []launcher.Mod{{Description: strings.Repeat("x", addonCacheBudget+1)}}})
	if len(a.addonCache) != 1 || len(a.addonCache["huge"].mods) != 1 {
		t.Fatal("oversized current pack did not replace older lists")
	}
	a.modsKey, a.mods = "huge", a.addonCache["huge"].mods
	a.cacheAddons("shaders", addonCacheEntry{packs: []launcher.AssetPack{{File: "shader.zip"}}})
	if len(a.addonCache["huge"].mods) != 1 {
		t.Fatal("opening shaders evicted the oversized current mod list that is still in use")
	}
}

func TestAddonNavigationJoinsPendingLoadsAndReusesRecentLists(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{ctx: ctx, service: &launcher.Service{}, firstFrame: make(chan struct{}),
		events: make(chan func(), 8), addonTab: "mods", instTab: "mods", page: "home",
		snapshot: launcher.Snapshot{ActiveRoot: "r", Instances: []launcher.Instance{{Root: launcher.Root{ID: "r", SelectedVersion: "v"}}}},
	}
	defer func() { cancel(); a.bgWG.Wait() }()
	a.loadMods()
	gen := a.modsGen
	for range 5 {
		a.ensureAddons()
	}
	if a.modsGen != gen || !a.modsRefreshing {
		t.Fatal("same navigation started overlapping scans")
	}
	a.mods = []launcher.Mod{{Name: "Unfinished", Pending: true}}
	a.goPage("tasks")
	if a.modsRefreshing || a.modsLoading || a.modsCancel != nil || a.modsGen == gen || len(a.mods) != 0 {
		t.Fatal("leaving the list did not invalidate its unfinished scan")
	}
	a.cacheAddons("r|v|mods", addonCacheEntry{mods: []launcher.Mod{{Name: "Ready"}}, loaded: time.Now()})
	a.ensureAddons()
	if a.modsRefreshing || len(a.mods) != 1 || a.mods[0].Name != "Ready" {
		t.Fatal("returning to a recent list rescanned or lost the ready rows")
	}
	a.modCheckCancel = func() {}
	a.cancelAddonLoad()
	if !a.addonCache["r|v|mods"].loaded.IsZero() || len(a.mods) != 1 {
		t.Fatal("cancelled update check was marked fresh or lost its completed rows")
	}
	a.loadMods()
	if !a.modsRefreshing {
		t.Fatal("explicit refresh incorrectly reused the recent scan")
	}
}

func TestLeavingModPageReleasesPreparedRows(t *testing.T) {
	for _, destination := range []string{"overview", "tasks"} {
		t.Run(destination, func(t *testing.T) {
			a, _ := instanceTabsTester(t, "mods")
			a.filteredAddons(a.addonEntries())
			mods := a.mods
			if len(a.addonListData.shown) == 0 {
				t.Fatal("fixture did not prepare mod rows")
			}
			if destination == "overview" {
				a.setInstTab(destination)
			} else {
				a.goPage(destination)
			}
			if a.addonListData.inputs != nil || a.addonListData.entries != nil || a.addonListData.shown != nil {
				t.Fatal("leaving mods retained prepared rows and their old scan references")
			}
			if len(a.mods) != len(mods) || &a.mods[0] != &mods[0] {
				t.Fatal("leaving the tab discarded the current list needed for a quick return")
			}
		})
	}
}

func TestModScanProgressCoalescesAndDropsCancelledResults(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := &App{ctx: ctx, events: make(chan func(), 4), modsGen: 1}
	var progress modListProgress
	for i := range 1000 {
		progress.publish(a, ctx, 1, []launcher.Mod{{Name: fmt.Sprint(i)}})
	}
	if len(a.events) != 1 {
		t.Fatal("busy UI queued full intermediate scans")
	}
	(<-a.events)()
	if len(a.mods) != 1 || a.mods[0].Name != "999" || progress.latest != nil {
		t.Fatal("coalescing lost the latest snapshot or kept the consumed list")
	}
	progress.publish(a, ctx, 1, []launcher.Mod{{Name: "obsolete"}})
	cancel()
	progress.clear()
	(<-a.events)()
	if a.mods[0].Name != "999" || progress.latest != nil {
		t.Fatal("cancelled scan replaced the current list or retained its last snapshot")
	}
}

func TestIdenticalModArtworkIsSharedAcrossInstances(t *testing.T) {
	a := &App{imagePending: map[string]bool{"addon:sha1:new-content": true}}
	icon := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 64, 64)))
	a.cacheImageLocked("addon:sha1:same-content", icon, 0)
	for _, instance := range []string{"pack-a", "pack-b", "pack-c"} {
		if a.addonIcon("root", instance, "mod", instance+".jar.disabled", "", "same-content") != icon {
			t.Fatal("same mod in another pack did not reuse its bitmap")
		}
	}
	if len(a.images) != 1 {
		t.Fatal("cross-instance artwork was duplicated")
	}
	if a.addonIcon("root", "pack-a", "mod", "pack-a.jar", "", "new-content") != nil {
		t.Fatal("replacing a jar reused the old content's icon")
	}
}

func TestShaderLoaderUsesCompactFallbackAndLiveModChanges(t *testing.T) {
	a := &App{snapshot: launcher.Snapshot{ActiveRoot: "r", Instances: []launcher.Instance{{Root: launcher.Root{ID: "r", SelectedVersion: "v"}}}}}
	mods := []launcher.Mod{{Name: "Other", Enabled: true}, {ModID: "iris", Version: "1.8", Enabled: true}}
	a.shaderLoaders = map[string]shaderLoaderInfo{"r|v": findShaderLoader(mods)}
	if name, version, known := a.shaderLoader(); name != "Iris" || version != "1.8" || !known {
		t.Fatal("shader status lost its compact cached result")
	}
	mods[1].Enabled = false
	a.cacheAddons("r|v|mods", addonCacheEntry{mods: mods})
	if name, _, known := a.shaderLoader(); name != "" || !known {
		t.Fatal("shader status ignored a freshly disabled loader")
	}
}
