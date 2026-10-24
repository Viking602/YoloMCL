package nativeui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func projectFilterTester(t *testing.T, source, kind string) (*App, *ui.Tester) {
	t.Helper()
	a, _, _ := projectDialogFixture(t, 0)
	a.bgWG.Wait()
	a.service = nil
	a.project = &launcher.Project{ID: "example", Source: source, Type: kind, Title: "Example Project", Description: "A resource for your next adventure"}
	a.snapshot.ActiveRoot = "instance"
	a.snapshot.Instances = []launcher.Instance{{
		Root:     launcher.Root{ID: "instance", Name: "My Minecraft", SelectedVersion: "game"},
		Versions: []launcher.VersionSummary{{ID: "game", Minecraft: "1.21.1", Loader: "neoforge"}},
	}}
	a.imageRetry["version:instance:game"] = time.Now().Add(time.Hour)
	a.projectVersions = []launcher.ProjectVersion{
		{ID: "new", Number: "v3.0", Name: "New version", GameVersions: []string{"26.3"}, Loaders: []string{"fabric"}, Type: "release"},
		{ID: "beta", Number: "v2.1-beta", GameVersions: []string{"1.21.1", "1.21"}, Loaders: []string{"neoforge"}, Type: "beta"},
		{ID: "stable", Number: "v2.0", GameVersions: []string{"1.21.1", "1.21"}, Loaders: []string{"neoforge"}, Type: "release"},
		{ID: "fabric", Number: "v2.0-fabric", GameVersions: []string{"1.21.1"}, Loaders: []string{"fabric"}, Type: "release"},
		{ID: "old", Number: "v1.0", GameVersions: []string{"1.20.1"}, Loaders: []string{"forge"}, Type: "alpha"},
	}
	for i := range a.projectVersions {
		v := &a.projectVersions[i]
		v.Source, v.ProjectID = source, a.project.ID
		v.Files = []launcher.DownloadFile{{Name: v.Number + ".jar", Size: 530000}}
		a.packRemote[v.ID] = &launcher.PackPreview{}
		if kind == "shader" {
			v.Loaders = []string{"iris", "optifine"}
		}
		if kind == "resourcepack" {
			v.Loaders = []string{"minecraft"}
		}
	}
	a.dialog, a.dialogOpen = "project", true
	tt := ui.NewTester(func(c *ui.Context) {
		for len(a.events) > 0 {
			(<-a.events)()
		}
		c.SetTheme(theme)
		a.dialogView(c, ui.Box(c).Fill().Background(bg))
	}, 1536, 1024)
	return a, tt
}

func TestProjectFiltersAcrossProvidersAndResourceTypes(t *testing.T) {
	for _, source := range []string{"modrinth", "curseforge"} {
		for _, kind := range []string{"mod", "modpack", "resourcepack", "shader"} {
			t.Run(source+"-"+kind, func(t *testing.T) {
				a, tt := projectFilterTester(t, source, kind)
				tt.SetPreferences(ui.Preferences{ReduceMotion: true})
				tt.Frame()
				if !a.verFilter.allTypes {
					t.Fatal("releases-only was enabled by default")
				}
				if !slices.ContainsFunc(a.visibleVersions(), func(v launcher.ProjectVersion) bool { return v.ID == "beta" }) {
					t.Fatal("default filters hid compatible beta")
				}
				clickUI(t, tt, "只看正式版")
				for _, v := range a.visibleVersions() {
					if v.Type != "release" {
						t.Fatal("release filter kept non-release", v.ID)
					}
				}
				clickUI(t, tt, "只看正式版")
				// Versions beyond the old four-chip limit are reachable by search.
				clickUI(t, tt, "项目游戏版本筛选")
				tt.Type("1.20")
				tt.Key(0, ui.KeyEnter)
				if a.verFilter.mc != "1.20.1" || !a.projShowAll {
					t.Fatalf("choice was not committed: %+v", a.verFilter)
				}
				// The default NeoForge filter should yield to an older Minecraft
				// version that only has Forge files, instead of producing a dead end.
				if vis := a.visibleVersions(); len(vis) != 1 || vis[0].ID != "old" || a.form.VersionID != "old" {
					t.Fatalf("wrong filtered selection: %v %s", vis, a.form.VersionID)
				}
				clickUI(t, tt, "重置筛选")
				if len(a.visibleVersions()) != len(a.projectVersions) || !a.verFilter.allTypes || !a.projShowAll {
					t.Fatal("reset did not restore the full catalog")
				}
				if kind == "mod" || kind == "modpack" {
					clickUI(t, tt, "版本加载器筛选")
					clickUI(t, tt, "版本加载器筛选 Fabric")
					if a.verFilter.loader != "fabric" || len(a.visibleVersions()) != 2 {
						t.Fatalf("Fabric selection did not filter files: %+v, all=%t, files=%d", a.verFilter, a.projShowAll, len(a.visibleVersions()))
					}
				}
				if kind != "modpack" {
					clickUI(t, tt, "只看兼容")
					for _, v := range a.visibleVersions() {
						if !a.projectVersionFits(v) {
							t.Fatal("incompatible file in compatibility filter")
						}
					}
					clickUI(t, tt, "只看兼容")
					if len(a.visibleVersions()) != len(a.projectVersions) {
						t.Fatal("unchecking compatibility kept the implicit instance filters")
					}
				}
				snapshotUI(t, tt, "project-filters-"+source+"-"+kind)
			})
		}
	}
}

func TestProjectGameBadgeTracksSelectedCompatibility(t *testing.T) {
	for _, kind := range []string{"shader", "resourcepack", "mod", "modpack"} {
		a, tt := projectFilterTester(t, "modrinth", kind)
		a.projectVersions = []launcher.ProjectVersion{{ID: "multi", Number: "r5.9.3", GameVersions: []string{"26.3", "26.1.2", "1.21.1"}, Loaders: []string{"neoforge", "iris"}, Type: "release"}}
		a.packRemote["multi"] = &launcher.PackPreview{}
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		clickUI(t, tt, "项目游戏版本筛选")
		tt.Type("26.1.2")
		tt.Key(0, ui.KeyEnter)
		if !tt.HasText("26.1.2 +2") || tt.HasText("26.3 +2") {
			t.Fatalf("%s badge did not follow filter: %v", kind, tt.Texts())
		}
		if a.form.VersionID != "multi" {
			t.Fatal("multi-version file lost its selection")
		}
	}
}

func TestProjectLongVersionChoicesRemainReachable(t *testing.T) {
	for _, lang := range []string{LangZH, LangEN} {
		for _, small := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-small-%t", lang, small), func(t *testing.T) {
				a, tt := projectFilterTester(t, "modrinth", "modpack")
				a.OverrideLanguage(lang)
				if small {
					tt.SetSize(900, 540)
				} else {
					tt.SetScale(1.5)
				}
				long := "26.4-snapshot-10-with-a-long-version-identifier"
				for i := range 80 {
					a.projectVersions = append(a.projectVersions, launcher.ProjectVersion{ID: fmt.Sprintf("snapshot-%d", i), Number: fmt.Sprintf("Snapshot %d", i), GameVersions: []string{fmt.Sprintf("26.4-snapshot-%d", i)}, Loaders: []string{"fabric"}, Type: "alpha"})
				}
				a.projectVersions = append(a.projectVersions, launcher.ProjectVersion{ID: "long", Number: "Long version", GameVersions: []string{long}, Type: "alpha"})
				a.packRemote["long"] = &launcher.PackPreview{}
				tt.Advance(350 * time.Millisecond)
				clickUI(t, tt, a.L("项目游戏版本筛选", "Project game version filter"))
				tt.Type("long-version")
				tt.Advance(300 * time.Millisecond)
				opt, ok := tt.Find("游戏版本选项 " + long)
				popup, popOK := tt.Find("游戏版本选项")
				if !ok || !popOK || opt.W <= 0 || opt.X < popup.X || opt.X+opt.W > popup.X+popup.W+.5 {
					t.Fatalf("long option not visible: %+v %+v", opt, popup)
				}
				snapshotUI(t, tt, fmt.Sprintf("project-long-choice-%s-%t", lang, small))
				tt.Key(0, ui.KeyEnter)
				tt.Advance(350 * time.Millisecond)
				if a.form.VersionID != "long" || a.verFilter.mc != long {
					t.Fatal("long version could not be selected")
				}
				panel, _ := tt.Find("活动弹窗")
				for _, label := range []string{a.L("版本加载器筛选", "Version loader filter"), a.L("只看正式版", "Releases only"), "按钮文字 install-confirm"} {
					b, ok := tt.Find(label)
					if !ok || b.X < panel.X || b.X+b.W > panel.X+panel.W+.5 || b.Y+b.H > panel.Y+panel.H+.5 {
						t.Fatalf("control clipped: %s %+v in %+v", label, b, panel)
					}
				}
				snapshotUI(t, tt, fmt.Sprintf("project-long-selected-%s-%t", lang, small))
			})
		}
	}
}

func TestProjectVersionOrderingIncludesSnapshotsAndHistoricalVersions(t *testing.T) {
	input := []string{"26.4-snapshot-5", "26.4-snapshot-10", "26.4-rc1", "26.4", "1.21.9", "1.21.10", "24w14a", "b1.7.3", "a1.2.6"}
	vs := mcVersionsOf([]launcher.ProjectVersion{{GameVersions: input}})
	for _, pair := range [][2]string{{"26.4", "26.4-rc1"}, {"26.4-rc1", "26.4-snapshot-10"}, {"26.4-snapshot-10", "26.4-snapshot-5"}, {"1.21.10", "1.21.9"}} {
		if slices.Index(vs, pair[0]) >= slices.Index(vs, pair[1]) {
			t.Fatalf("incorrect order: %v", vs)
		}
	}
	if len(vs) != len(input) {
		t.Fatal("lost historical Minecraft versions", vs)
	}
}

func TestProjectVersionRequestsIgnoreReopenAndLateErrors(t *testing.T) {
	a, tt := projectFilterTester(t, "modrinth", "shader")
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	type request struct {
		ctx   context.Context
		reply chan error
	}
	requests := make(chan request, 4)
	a.projectFetch.fetch = func(ctx context.Context, source, id, mc, loader string) ([]launcher.ProjectVersion, error) {
		if mc != "" || loader != "" {
			t.Error("catalog was filtered at the API")
		}
		r := request{ctx, make(chan error, 1)}
		requests <- r
		select {
		case err := <-r.reply:
			return []launcher.ProjectVersion{{ID: "loaded", Number: "Loaded", GameVersions: []string{"1.21.1"}}}, err
		case <-a.ctx.Done():
			return nil, a.ctx.Err()
		}
	}
	p := *a.project
	a.openProject(p)
	first := <-requests
	a.closeDialog()
	a.openProject(p)
	second := <-requests
	if first.ctx.Err() == nil {
		t.Fatal("closed request was not canceled")
	}
	second.reply <- nil
	waitUI(t, tt, func() bool { return !a.projLoading })
	first.reply <- errors.New("obsolete failure")
	a.bgWG.Wait()
	tt.Frame()
	if a.projectFetch.err != "" || a.form.VersionID != "loaded" {
		t.Fatal("old request replaced reopened project", a.projectFetch.err)
	}
	a.reloadProjectVersions()
	failed := <-requests
	failed.reply <- errors.New("network unavailable")
	waitUI(t, tt, func() bool { return !a.projLoading })
	if a.projectFetch.err == "" || a.form.VersionID != "" || !tt.HasText("加载失败") {
		t.Fatal("failed request left a spinner or stale selection")
	}
	clickUI(t, tt, "重试")
	retry := <-requests
	retry.reply <- nil
	waitUI(t, tt, func() bool { return !a.projLoading })
	if a.form.VersionID != "loaded" || a.projectFetch.err != "" || strings.Contains(strings.Join(tt.Texts(), " "), "obsolete failure") {
		t.Fatal("retry did not recover")
	}
}
