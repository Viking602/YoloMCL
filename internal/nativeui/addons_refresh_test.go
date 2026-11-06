package nativeui

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestCompletedModUpdateRefreshesVersionAndUpdateCount(t *testing.T) {
	for _, prior := range []string{"running", "unobserved"} {
		t.Run(prior, func(t *testing.T) {
			s, err := launcher.New(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			mc := filepath.Join(t.TempDir(), ".minecraft")
			for _, dir := range []string{"mods", ".mcl"} {
				if err := os.MkdirAll(filepath.Join(mc, dir), 0755); err != nil {
					t.Fatal(err)
				}
			}
			var jar bytes.Buffer
			z := zip.NewWriter(&jar)
			w, err := z.Create("fabric.mod.json")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write([]byte(`{"id":"example","name":"Example","version":"2.0"}`)); err != nil {
				t.Fatal(err)
			}
			if err := z.Close(); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(mc, "mods", "example-2.jar"), jar.Bytes(), 0644); err != nil {
				t.Fatal(err)
			}
			// Keep this test offline: the update check is already fresh.
			if err := os.WriteFile(filepath.Join(mc, ".mcl", "mod-checks.json"), []byte(`{}`), 0644); err != nil {
				t.Fatal(err)
			}
			r, err := s.AddRoot(mc, "Updated instance")
			if err != nil {
				t.Fatal(err)
			}
			a, err := New(s)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { a.cancel(); a.bgWG.Wait() })
			a.instTab = "mods"
			a.addonListMode = prior == "unobserved"
			a.modsKey = r.ID + "|" + r.SelectedVersion + "|mods"
			a.mods = []launcher.Mod{{File: "example-1.jar", Name: "Example", Version: "1.0", Enabled: true, Status: "update", Latest: &launcher.ProjectVersion{ID: "v2", Number: "2.0"}}}
			a.cacheAddons(a.modsKey, addonCacheEntry{mods: a.mods, loaded: time.Now()})
			if _, _, updates := a.addonFilterCounts(a.addonEntries()); updates != 1 {
				t.Fatal("fixture must show the old version as updatable")
			}
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme)
				ui.Column(c).Fill().Padding(16).Children(func() { a.addonResults(c, a.filteredAddons(a.addonEntries())) })
			}, 800, 280)
			if !tt.HasText("1.0") {
				t.Fatal("fixture did not display the installed version")
			}
			job := launcher.Job{ID: "update", TitleKey: "mods.installDeps", TargetRootID: r.ID, State: "running"}
			if prior == "running" {
				a.snapshot.Jobs = []launcher.Job{job}
			}
			job.State = "completed"
			next := a.snapshot
			next.Jobs = []launcher.Job{job}
			a.applySnapshot(next, a.selectionEpoch.Load())
			if !a.modsRefreshing || a.modsLoading || a.mods[0].Version != "1.0" {
				t.Fatal("completion did not start a quiet refresh of the instance's mod tab")
			}
			a.bgWG.Wait()
			a.drainEvents()
			if a.modsRefreshing || len(a.mods) != 1 || a.mods[0].Version != "2.0" || a.mods[0].File != "example-2.jar" || a.mods[0].Latest != nil {
				t.Fatalf("completed update retained the old card: %+v", a.mods)
			}
			if _, _, updates := a.addonFilterCounts(a.addonEntries()); updates != 0 {
				t.Fatal("updatable count retained the installed update")
			}
			tt.Frame()
			if !tt.HasText("2.0") || tt.HasText("1.0") {
				t.Fatalf("rendered version did not refresh: %v", tt.Texts())
			}
			if _, ok := tt.Find(a.tr().ModsUpdateBtn); ok {
				t.Fatal("installed update still has an update button")
			}
			if cached := a.addonCache[a.modsKey]; cached.loaded.IsZero() || cached.mods[0].Version != "2.0" {
				t.Fatal("returning to the tab would restore the old version")
			}
			gen := a.modsGen
			a.applySnapshot(next, a.selectionEpoch.Load())
			if a.modsGen != gen {
				t.Fatal("unchanged completed job restarted the scan")
			}
		})
	}
}

func TestAddonJobRefreshScopeAndNavigation(t *testing.T) {
	for _, tc := range []struct {
		name, page, tab, title, root, state string
		refresh, expire                     bool
	}{
		{"progress", "home", "mods", "mods.installDeps", "r", "running", false, false},
		{"other-root", "home", "mods", "mods.installDeps", "other", "completed", false, false},
		{"unrelated-job", "home", "mods", "world.backup", "r", "completed", false, false},
		{"overview", "home", "overview", "mods.installDeps", "r", "completed", false, true},
		{"offscreen", "tasks", "mods", "mods.installDeps", "r", "completed", false, true},
		{"other-kind", "home", "shader", "mods.installDeps", "r", "completed", false, true},
		{"legacy-page", "mods", "mods", "mods.installDeps", "r", "completed", true, true},
		{"check", "home", "mods", "mods.detect", "r", "completed", true, true},
		{"retry", "home", "mods", "mods.installDeps", "r", "completed", true, true},
		{"partial-failure", "home", "mods", "mods.installDeps", "r", "failed", true, true},
		{"cancel", "home", "mods", "mods.installDeps", "r", "cancelled", true, true},
		{"pack-update", "home", "mods", "pack.update", "r", "completed", true, true},
		{"resourcepack", "home", "resourcepack", "addon.resourcepack", "r", "completed", true, false},
		{"shader", "home", "shader", "addon.shader", "r", "completed", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			a := &App{ctx: ctx, service: &launcher.Service{}, events: make(chan func(), 16), firstFrame: make(chan struct{}),
				page: tc.page, instTab: tc.tab, addonTab: tc.tab,
				snapshot: launcher.Snapshot{ActiveRoot: "r", Instances: []launcher.Instance{{Root: launcher.Root{ID: "r", SelectedVersion: "v"}}}},
			}
			t.Cleanup(func() { cancel(); a.bgWG.Wait() })
			for _, key := range []string{"r|v|mods", "r|shared|mods", "r|v|resourcepack", "r|v|shader", "other|v|mods"} {
				a.cacheAddons(key, addonCacheEntry{loaded: time.Now()})
			}
			next := a.snapshot
			next.Jobs = []launcher.Job{{ID: "task", TitleKey: tc.title, TargetRootID: tc.root, State: tc.state}}
			if tc.name == "retry" {
				a.snapshot.Jobs = append([]launcher.Job(nil), next.Jobs...)
				next.Jobs[0].Attempt++ // retry finished entirely between snapshots
			}
			a.applySnapshot(next, 0)
			if a.modsRefreshing != tc.refresh {
				t.Fatalf("refreshing = %v, want %v", a.modsRefreshing, tc.refresh)
			}
			for _, key := range []string{"r|v|mods", "r|shared|mods"} {
				if expired := a.addonCache[key].loaded.IsZero(); expired != tc.expire {
					t.Fatalf("%s expired = %v, want %v", key, expired, tc.expire)
				}
			}
			if tc.root != "other" && a.addonCache["other|v|mods"].loaded.IsZero() {
				t.Fatal("completion invalidated another instance")
			}
			gen := a.modsGen
			a.applySnapshot(next, 0)
			if a.modsGen != gen {
				t.Fatal("unchanged job restarted the scan")
			}
			if tc.expire && !tc.refresh {
				a.page, a.instTab, a.addonTab = "home", "mods", "mods"
				a.ensureAddons()
				if !a.modsRefreshing {
					t.Fatal("returning immediately reused a stale list")
				}
			}
		})
	}
}
