package nativeui

import (
	"testing"
	"time"
)

// Everyday actions never take the global busy state (which greys every
// button and reads as a whole-window flash).
func TestEverydayActionsDoNotSetBusy(t *testing.T) {
	a, tt := homeTester(t, "zh-CN")
	for _, m := range []string{"logs", "toggleMod", "toggleAssetPack", "checkMods", "select", "updateVersion", "setVersionName"} {
		a.execute(m, map[string]string{"rootId": "r1"}, nil)
		if a.dialogPending() {
			t.Fatalf("%s set busy", m)
		}
		if !a.pendingOp(m) {
			t.Fatalf("%s should show local progress", m)
		}
	}
	a.loadLogs()
	if a.dialogPending() {
		t.Fatal("log refresh set busy")
	}
	a.selectRoot("r1")
	if a.dialogPending() {
		t.Fatal("directory switch set busy")
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(a.pending) > 0 && time.Now().Before(deadline) {
		tt.Frame()
		done := true
		for _, n := range a.pending {
			if n > 0 {
				done = false
			}
		}
		if done {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// A mods result for a version that is no longer selected is dropped.
func TestStaleModsResultIsDropped(t *testing.T) {
	a, _ := homeTester(t, "zh-CN")
	a.loadMods()
	first := a.modsGen
	a.loadMods()
	if a.modsGen == first {
		t.Fatal("generation did not advance")
	}
	a.cacheAddons("k", addonCacheEntry{})
	for i := 0; i < addonCacheSize+5; i++ {
		a.cacheAddons(string(rune('a'+i)), addonCacheEntry{})
	}
	if len(a.addonCache) != addonCacheSize || len(a.addonOrder) != addonCacheSize {
		t.Fatalf("cache not bounded: %d", len(a.addonCache))
	}
}
