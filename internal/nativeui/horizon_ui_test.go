package nativeui

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestHorizonNamingAndJobTitle(t *testing.T) {
	a, _ := acctTester(t, "zh-CN", 1280, 800)
	v := launcher.VersionSummary{
		ID:            "horizon-nightly-2026-11-20-0a875da",
		Minecraft:     "26.3",
		Loader:        "horizon",
		LoaderVersion: "nightly-2026-11-20-0a875da",
		JavaVersion:   25,
	}
	n := a.nameOf(launcher.Instance{Versions: []launcher.VersionSummary{v}}, v)
	if n.Title != "Horizon nightly-2026-11-20-0a875da" || n.Sub != "" || !n.MCTag {
		t.Fatalf("unexpected Horizon naming: %+v", n)
	}
	if loaderKey(a.tr().LoaderHorizon) != "horizon" {
		t.Fatalf("loaderKey(LoaderHorizon) = %q, want horizon", loaderKey(a.tr().LoaderHorizon))
	}
	if loaderIconName("horizon") != "loader-horizon" || icons["loader-horizon"] == nil {
		t.Fatal("Horizon pixel icon not registered")
	}
	if loaderLogos["horizon"] == nil || loaderLogoKey("horizon") != "horizon" {
		t.Fatal("Horizon official PNG logo not registered")
	}

	j := launcher.Job{Title: "安装 Horizon nightly-2026-11-20-0a875da", State: "running"}
	j.TitleKey, j.TitleArg, _ = launcher.JobTitleKey(j.Title)
	a.settings.Language = LangZH
	if got := a.jobTitle(j); got != "安装 Horizon nightly-2026-11-20-0a875da" {
		t.Fatalf("zh job title = %q", got)
	}
	a.settings.Language = LangEN
	if got := a.jobTitle(j); got != "Install Horizon nightly-2026-11-20-0a875da" {
		t.Fatalf("en job title = %q", got)
	}
}

func TestInstallRequiredJava(t *testing.T) {
	cases := []struct {
		mc, loader string
		want       int
	}{
		{"26.3", "horizon", 25},
		{"1.21.4", "horizon", 25},
		{"26.3", "vanilla", 25},
		{"26.1.2", "fabric", 25},
		{"26w02a", "vanilla", 25},
		{"1.21.4", "fabric", 21},
		{"1.20.5", "vanilla", 21},
		{"24w14a", "vanilla", 21},
		{"25w08a", "vanilla", 21},
		{"1.20.4", "forge", 17},
		{"1.18.2", "forge", 17},
		{"24w13a", "vanilla", 17},
		{"21w37a", "vanilla", 17},
		{"1.17.1", "vanilla", 16},
		{"21w19a", "vanilla", 16},
		{"1.16.5", "forge", 8},
		{"1.12.2", "forge", 8},
		{"b1.7.3", "vanilla", 8},
		{"rd-132211", "vanilla", 8},
		{"", "vanilla", 0},
	}
	for _, tc := range cases {
		if got := installRequiredJava(tc.mc, tc.loader); got != tc.want {
			t.Errorf("installRequiredJava(%q, %q) = %d, want %d", tc.mc, tc.loader, got, tc.want)
		}
	}
}

func TestHorizonInstallDialogCardAndSwitch(t *testing.T) {
	a, tt := acctTester(t, "zh-CN", 1440, 900)
	a.snapshot.ActiveAccount = "ms-1"
	a.snapshot.Instances = []launcher.Instance{{
		Root: launcher.Root{ID: "r1", Name: "MC", Path: `D:\MC\.minecraft`},
		Versions: []launcher.VersionSummary{
			{ID: "1.21.11-fabric", Minecraft: "1.21.11", Loader: "fabric", LoaderVersion: "0.16.14"},
			{ID: "26.3", Minecraft: "26.3", Loader: "vanilla"},
		},
	}}
	a.snapshot.ActiveRoot = "r1"
	a.java = []launcher.JavaInfo{
		{Path: `C:\Program Files\Java\jdk-25\bin\javaw.exe`, Version: "25.0.1", Major: 25, Vendor: "Eclipse Adoptium"},
	}
	a.minecraftVersions = []launcher.MinecraftVersion{
		{ID: "26.3", Type: "release", ReleaseTime: "2026-09-15T00:00:00Z"},
		{ID: "26.2", Type: "release", ReleaseTime: "2026-06-16T00:00:00Z"},
		{ID: "26.1.2", Type: "release", ReleaseTime: "2026-04-21T00:00:00Z"},
		{ID: "26.1.1", Type: "release", ReleaseTime: "2026-04-02T00:00:00Z"},
		{ID: "26.1", Type: "release", ReleaseTime: "2026-03-24T00:00:00Z"},
		{ID: "1.21.11", Type: "release", ReleaseTime: "2025-12-09T00:00:00Z"},
		{ID: "1.21.4", Type: "release", ReleaseTime: "2024-12-03T00:00:00Z"},
		{ID: "1.21.1", Type: "release", ReleaseTime: "2024-08-08T00:00:00Z"},
		{ID: "1.20.4", Type: "release", ReleaseTime: "2023-12-07T00:00:00Z"},
	}
	a.openDialog("install")
	a.form.Target = a.tr().InstallTargetCurrent
	a.form.Minecraft = "26.3"
	a.form.Loader = a.tr().LoaderHorizon
	a.loaders = []string{
		"nightly-2026-11-20-0a875da",
		"nightly-2026-11-19-8c2f41b",
		"nightly-2026-11-18-3e9d12a",
	}
	a.form.LoaderVersion = "nightly-2026-11-20-0a875da"
	a.loaderLoading = false
	a.loaderError = ""
	a.horizonTargetMC = "26.3"
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	snapshotUI(t, tt, "install-horizon-zh")

	if _, ok := tt.Find(a.tr().LoaderHorizon); !ok {
		t.Fatal("Horizon loader card missing in install dialog")
	}
	if !tt.HasText("Minecraft 26.3") || !tt.HasText("Horizon nightly-2026-11-20-0a875da") {
		t.Fatalf("expected live install footer summary; visible texts: %v", tt.Texts())
	}
	if !tt.HasText("安装前环境检查") || !tt.HasText("安装流程预览") || !tt.HasText("已满足 Horizon 正版启动要求") {
		t.Fatalf("expected preflight and steps cards; visible texts: %v", tt.Texts())
	}
	if btn, ok := tt.Find("按钮文字 install-game-now"); !ok || btn.Y+btn.H > 900 {
		t.Fatalf("expected install button pinned in viewport, got ok=%v rect=%+v", ok, btn)
	}

	// Verify clicking a quick-pick recent build updates LoaderVersion.
	if err := tt.Click("nightly-2026-11-19-8c2f41b"); err != nil {
		t.Fatalf("could not click quick-pick build row: %v", err)
	}
	tt.Frame()
	if a.form.LoaderVersion != "nightly-2026-11-19-8c2f41b" {
		t.Fatalf("a.form.LoaderVersion = %q, want nightly-2026-11-19-8c2f41b", a.form.LoaderVersion)
	}

	// Inspect Fabric with missing Java 21 to verify warning & download CTA.
	a.form.Minecraft = "1.21.4"
	a.form.Loader = a.tr().LoaderFabric
	a.loaders = []string{"0.16.14", "0.16.13", "0.16.12", "0.16.11"}
	a.form.LoaderVersion = "0.16.14"
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	snapshotUI(t, tt, "install-fabric-missing-java-zh")
	if !tt.HasText("下载 Java 21") {
		t.Fatalf("expected missing Java 21 CTA; visible texts: %v", tt.Texts())
	}

	// Inspect Vanilla ready layout.
	a.form.Minecraft = "26.3"
	a.form.Loader = a.tr().LoaderVanilla
	a.loaders = nil
	a.form.LoaderVersion = ""
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	snapshotUI(t, tt, "install-vanilla-zh")

	// Switch to an incompatible MC version with an offline account and verify the Horizon switch CTA + MS warning.
	a.snapshot.ActiveAccount = "off-1"
	a.form.Loader = a.tr().LoaderHorizon
	a.form.Minecraft = "1.21.4"
	a.loaders = nil
	a.form.LoaderVersion = ""
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	snapshotUI(t, tt, "install-horizon-mismatch-zh")

	if _, ok := tt.Find("切换到 Minecraft 26.3"); !ok {
		t.Fatalf("expected Horizon switch-to-26.3 button; visible texts: %v", tt.Texts())
	}
	if !tt.HasText("Horizon 启动需要 Microsoft 正版账号") {
		t.Fatalf("expected Horizon Microsoft warning for offline account; visible texts: %v", tt.Texts())
	}

	// Verify clicking the switch button also updates versionKind when the target is a snapshot.
	a.minecraftVersions = []launcher.MinecraftVersion{
		{ID: "26.4-snapshot-1", Type: "snapshot", ReleaseTime: "2026-11-15T00:00:00Z"},
		{ID: "1.21.4", Type: "release", ReleaseTime: "2024-12-03T00:00:00Z"},
	}
	a.versionKind = a.versionKindLabel("release")
	a.horizonTargetMC = "26.4-snapshot-1"
	tt.Frame()
	if err := tt.Click("切换到 Minecraft 26.4-snapshot-1"); err != nil {
		t.Fatalf("could not click Horizon switch button: %v; visible texts: %v", err, tt.Texts())
	}
	tt.Frame()
	if a.form.Minecraft != "26.4-snapshot-1" {
		t.Fatalf("a.form.Minecraft = %q, want 26.4-snapshot-1", a.form.Minecraft)
	}
	if normalizeVersionKindKey(a.versionKind) != "snapshot" {
		t.Fatalf("versionKind = %q, want snapshot", a.versionKind)
	}

	// Verify narrow 960x640 layout with New Separate Directory selected.
	a2, tt2 := acctTester(t, "zh-CN", 960, 640)
	a2.snapshot.ActiveAccount = "ms-1"
	a2.snapshot.Instances = []launcher.Instance{{Root: launcher.Root{ID: "r1", Name: "MC", Path: `D:\MC\.minecraft`}}}
	a2.snapshot.ActiveRoot = "r1"
	a2.minecraftVersions = []launcher.MinecraftVersion{{ID: "26.3", Type: "release", ReleaseTime: "2026-09-15T00:00:00Z"}}
	a2.openDialog("install")
	a2.form.Target = a2.tr().InstallTargetNew
	a2.form.Minecraft = "26.3"
	a2.form.Loader = a2.tr().LoaderHorizon
	a2.loaders = []string{"nightly-2026-11-20-0a875da", "nightly-2026-11-19-8c2f41b"}
	a2.form.LoaderVersion = "nightly-2026-11-20-0a875da"
	tt2.Frame()
	tt2.Advance(400 * time.Millisecond)
	snapshotUI(t, tt2, "install-horizon-newdir-960")
}

func TestTabSwitchFromInstallAndHome(t *testing.T) {
	a, tt := acctTester(t, "zh-CN", 1440, 900)
	a.closeDialog()
	a.snapshot.Instances = []launcher.Instance{{
		Root: launcher.Root{ID: "r1", Name: "MC", Path: `D:\MC\.minecraft`, SelectedVersion: "neoforge-21.1.250"},
		Versions: []launcher.VersionSummary{
			{ID: "neoforge-21.1.250", Minecraft: "1.21.1", Loader: "neoforge", LoaderVersion: "21.1.250"},
			{ID: "horizon-nightly-2026-11-20-0a875da", Minecraft: "26.3", Loader: "horizon", LoaderVersion: "nightly-2026-11-20-0a875da"},
		},
	}}
	a.snapshot.ActiveRoot = "r1"
	a.minecraftVersions = []launcher.MinecraftVersion{
		{ID: "26.3", Type: "release", ReleaseTime: "2026-09-15T00:00:00Z"},
		{ID: "26.2", Type: "release", ReleaseTime: "2026-06-16T00:00:00Z"},
		{ID: "26.1.2", Type: "release", ReleaseTime: "2026-04-21T00:00:00Z"},
		{ID: "26.1.1", Type: "release", ReleaseTime: "2026-04-02T00:00:00Z"},
		{ID: "26.1", Type: "release", ReleaseTime: "2026-03-24T00:00:00Z"},
		{ID: "1.21.11", Type: "release", ReleaseTime: "2025-12-09T00:00:00Z"},
		{ID: "1.21.4", Type: "release", ReleaseTime: "2024-12-03T00:00:00Z"},
		{ID: "1.21.1", Type: "release", ReleaseTime: "2024-08-08T00:00:00Z"},
		{ID: "1.20.4", Type: "release", ReleaseTime: "2023-12-07T00:00:00Z"},
	}
	tt.Frame()
	tt.Advance(400 * time.Millisecond)

	// 1. Switch tabs directly from Home.
	for _, tab := range []string{"发现", "衣橱", "任务", "实例"} {
		if err := tt.Click(tab); err != nil {
			t.Fatalf("click %s from home failed: %v", tab, err)
		}
		for range 15 {
			tt.Advance(20 * time.Millisecond)
		}
	}

	// 2. Open Install dialog, stay clean, switch tabs.
	a.openDialog("install")
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	if err := tt.Click("发现"); err != nil {
		t.Fatalf("click 发现 from clean install failed: %v", err)
	}
	for range 15 {
		tt.Advance(20 * time.Millisecond)
	}
	if a.page != "discover" || a.dialogOpen {
		t.Fatalf("expected discover with dialog closed, got page=%q dialogOpen=%v", a.page, a.dialogOpen)
	}

	// 3. Open Install dialog, pick Horizon (dirty), click 任务 -> discard -> switch.
	a.goPage("home")
	tt.Frame()
	a.openDialog("install")
	tt.Frame()
	if err := tt.Click("Horizon"); err != nil {
		t.Fatalf("click Horizon card failed: %v", err)
	}
	tt.Frame()
	if err := tt.Click("任务"); err != nil {
		t.Fatalf("click 任务 from dirty install failed: %v", err)
	}
	tt.Frame()
	if a.confirmation == nil {
		t.Fatalf("expected discard confirmation when leaving dirty install dialog")
	}
	if err := tt.Click("按钮文字 confirmation-accept"); err != nil {
		t.Fatalf("click confirmation-accept failed: %v", err)
	}
	for range 15 {
		tt.Advance(20 * time.Millisecond)
	}
	if a.page != "tasks" || a.dialogOpen {
		t.Fatalf("expected tasks with dialog closed, got page=%q dialogOpen=%v", a.page, a.dialogOpen)
	}
}

func TestHorizonSeedMapTabRendersBiomesAndStructures(t *testing.T) {
	t.Setenv("MCL_DEVELOPMENT", "")
	s, err := launcher.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mc := t.TempDir()
	buildID := "nightly-2026-11-20-0a875da"
	verID := "horizon-" + buildID
	verDir := filepath.Join(mc, "versions", verID)
	if err := os.MkdirAll(filepath.Join(verDir, "horizon-mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	meta := `{"id":"` + verID + `","clientVersion":"26.3","type":"release","javaVersion":{"majorVersion":25},"horizon":{"buildId":"` + buildID + `","releaseTag":"nightly","minecraft":"26.3","javaMajor":25}}`
	if err := os.WriteFile(filepath.Join(verDir, verID+".json"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := s.AddRoot(mc, "Horizon")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Select(r.ID, "", verID); err != nil {
		t.Fatal(err)
	}
	a, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.cancel(); a.bgWG.Wait() })
	a.OverrideLanguage(LangZH)
	tt := ui.NewTester(a.View, 1440, 900)
	tt.Frame()

	clickUI(t, tt, a.L("instance.tab_prefix")+" "+a.L("seedmap.open"))
	waitUI(t, tt, func() bool {
		return a.seedMap != nil && (a.seedMap.err != "" || (a.seedMap.mapReady() && !a.seedMap.pending))
	})
	if a.seedMap.err != "" {
		t.Fatalf("Horizon seedmap errored: %s", a.seedMap.err)
	}
	v := a.seedMap
	v.viewport.BlocksPerPixel = 4
	tt.Frame()
	waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport == v.viewport })
	tt.Advance(400 * time.Millisecond)
	snapshotUI(t, tt, "seedmap-horizon-zh")

	if v.sampler == nil || len(v.frame.Biomes) == 0 || len(v.frame.Hits) == 0 {
		t.Fatalf("expected Horizon 26.3 seedmap biomes and structure hits, got sampler=%v biomes=%d hits=%d", v.sampler != nil, len(v.frame.Biomes), len(v.frame.Hits))
	}
	if !tt.HasText(a.L("seedmap.surface_active")) {
		t.Fatalf("expected Horizon 26.3 seedmap surface mode active; visible texts: %v", tt.Texts())
	}
}

func TestHorizonNativeRuntimeOverviewAndSettings(t *testing.T) {
	a, tt := acctTester(t, "zh-CN", 1440, 900)
	a.closeDialog()
	a.snapshot.ActiveAccount = "ms-1"
	verID := "horizon-nightly-2026-11-20-0a875da"
	a.snapshot.Instances = []launcher.Instance{{
		Root: launcher.Root{ID: "r1", Name: "MC", Path: `D:\MC\.minecraft`, SelectedVersion: verID},
		Versions: []launcher.VersionSummary{
			{ID: verID, Minecraft: "26.3", Loader: "horizon", LoaderVersion: "nightly-2026-11-20-0a875da", JavaVersion: 25},
		},
	}}
	a.snapshot.ActiveRoot = "r1"
	a.java = []launcher.JavaInfo{
		{Path: `C:\Program Files\Zulu\zulu-25\bin\javaw.exe`, Version: "25.0.2", Major: 25, Vendor: "Azul Systems, Inc."},
	}
	a.memoryInfo = launcher.MemoryInfo{TotalMB: 96 * 1024, AvailableMB: 64 * 1024, MaxMB: 64 * 1024}
	a.instStatusKey = "r1|" + verID
	a.instStatus = launcher.InstanceStatus{
		Dir: `D:\MC\.minecraft\versions\` + verID,
		Play: launcher.PlayStats{
			Launches:  2,
			TotalSec:  180,
			LastSec:   60,
			LastStart: time.Now().Add(-7 * time.Minute),
		},
	}

	// 1. Overview tab
	a.page, a.instTab = "home", "overview"
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	snapshotUI(t, tt, "horizon-overview-zh")

	for _, want := range []string{"Rust 原生", "运行引擎", "Horizon 原生客户端", "无 JVM", "系统原生管理", "无 JVM 堆上限", "资源准备", "仅提取资源"} {
		if !tt.HasText(want) {
			t.Errorf("overview missing Horizon native text %q; visible texts: %v", want, tt.Texts())
		}
	}
	if tt.HasText("JVM 参数") {
		t.Errorf("Horizon overview should not display JVM 参数")
	}

	// 2. Instance Settings tab
	a.instTab = "settings"
	a.loadVersionConfigForm(verID)
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	snapshotUI(t, tt, "horizon-settings-zh")

	for _, want := range []string{"名称与资源准备", "资源准备 Java", "原生运行时", "Horizon (Rust 原生)"} {
		if !tt.HasText(want) {
			t.Errorf("settings missing Horizon native text %q; visible texts: %v", want, tt.Texts())
		}
	}
	if _, ok := tt.Find(a.tr().RootCfgVerJVMLabel); ok || tt.HasText("内存大小") {
		t.Errorf("Horizon settings should not display JVM input or memory slider")
	}
}

func TestHorizonDiscoverAndProjectDialogVisualQA(t *testing.T) {
	a, tt := acctTester(t, "zh-CN", 1440, 900)
	a.closeDialog()
	verID := "horizon-nightly-2026-11-20-0a875da"
	a.snapshot.Instances = []launcher.Instance{{
		Root: launcher.Root{ID: "r1", Name: "Horizon", Path: `D:\MC\.minecraft`, SelectedVersion: verID},
		Versions: []launcher.VersionSummary{
			{ID: verID, Minecraft: "26.3", Loader: "horizon", LoaderVersion: "nightly-2026-11-20-0a875da", JavaVersion: 25},
		},
	}}
	a.snapshot.ActiveRoot = "r1"
	a.mods = []launcher.Mod{{
		File:      "zoom_helper.hmod",
		Name:      "Zoom Helper",
		ModID:     "zoom_helper",
		Version:   "0.5.0",
		Loader:    "horizon",
		Enabled:   true,
		Source:    "horizon",
		ProjectID: "zoom_helper",
	}}
	a.addonCache = map[string]addonCacheEntry{
		"r1|" + verID + "|mods": {mods: a.mods, loaded: time.Now()},
	}

	horizonProjects := []launcher.Project{
		{
			ID:          "flesh_to_leather",
			Source:      "horizon",
			Type:        "mod",
			Title:       "Flesh to Leather",
			Description: "Smelt rotten flesh in a furnace or smoker to craft usable leather.",
			Downloads:   18420,
			Author:      "HorizonTeam",
			Categories:  []string{"utility", "food"},
			URL:         "https://playhorizon.dev/mods/flesh_to_leather",
			Updated:     "2026-11-20T12:00:00Z",
		},
		{
			ID:          "zoom_helper",
			Source:      "horizon",
			Type:        "mod",
			Title:       "Zoom Helper",
			Description: "Smooth spyglass-style camera zoom keybind built for Horizon WASM API.",
			Downloads:   34200,
			Author:      "Skyline",
			Categories:  []string{"utility"},
			URL:         "https://playhorizon.dev/mods/zoom_helper",
			Updated:     "2026-11-19T10:00:00Z",
		},
		{
			ID:          "waystones_h",
			Source:      "horizon",
			Type:        "mod",
			Title:       "Horizon Waystones",
			Description: "Teleportation obelisks and shared world beacons for Minecraft 26.3.",
			Downloads:   9150,
			Author:      "Atlas",
			Categories:  []string{"adventure", "transportation"},
			URL:         "https://playhorizon.dev/mods/waystones_h",
			Updated:     "2026-11-18T08:00:00Z",
		},
		{
			ID:          "biome_compass",
			Source:      "horizon",
			Type:        "mod",
			Title:       "Biome Compass",
			Description: "Locate rare biomes quickly with a lightweight HUD compass overlay.",
			Downloads:   6480,
			Author:      "Nova",
			Categories:  []string{"worldgen", "utility"},
			URL:         "https://playhorizon.dev/mods/biome_compass",
			Updated:     "2026-11-16T15:30:00Z",
		},
	}

	var lastQuery launcher.SearchQuery
	a.discoverFeed.fetch = func(_ context.Context, q launcher.SearchQuery) (launcher.SearchResult, error) {
		lastQuery = q
		if q.Source == "horizon" {
			return launcher.SearchResult{Projects: horizonProjects, Total: len(horizonProjects), Offset: 0, Limit: 18}, nil
		}
		return launcher.SearchResult{Projects: nil, Total: 0, Offset: 0, Limit: 18}, nil
	}
	a.catOpts = map[string][]launcher.Category{
		"horizon|mod": {
			{ID: "adventure", Slug: "adventure", Name: "adventure"},
			{ID: "food", Slug: "food", Name: "food"},
			{ID: "transportation", Slug: "transportation", Name: "transportation"},
			{ID: "utility", Slug: "utility", Name: "utility"},
			{ID: "worldgen", Slug: "worldgen", Name: "worldgen"},
		},
	}
	a.catAsked = map[string]bool{"horizon|mod": true, "modrinth|modpack": true, "modrinth|mod": true}
	a.projectFetch.fetch = func(_ context.Context, source, project, _, _ string) ([]launcher.ProjectVersion, error) {
		return []launcher.ProjectVersion{{
			ID:           "1.1.0",
			ProjectID:    project,
			Source:       source,
			Name:         "Flesh to Leather 1.1.0",
			Number:       "1.1.0",
			Published:    "2026-11-20T12:00:00Z",
			GameVersions: []string{"26.3"},
			Loaders:      []string{"horizon"},
			Type:         "release",
			Files:        []launcher.DownloadFile{{Name: "flesh_to_leather-1.1.0.hmod", Size: 24576, Primary: true}},
		}}, nil
	}
	a.resolvePlan = func(_ context.Context, _, source, project, verID string, emit func([]launcher.PlanItem)) ([]launcher.PlanItem, error) {
		items := []launcher.PlanItem{{
			Key:       source + ":" + project,
			Source:    source,
			ProjectID: project,
			VersionID: verID,
			Title:     "Flesh to Leather",
			Version:   verID,
			Role:      "target",
			Status:    "install",
		}}
		emit(items)
		return items, nil
	}

	// Open Discover from Modrinth modpack, then click Horizon source pill.
	a.page, a.provider, a.resourceType = "discover", "modrinth", a.tr().DiscoverTypeModpack
	tt.Frame()
	if err := tt.Click("Horizon"); err != nil {
		t.Fatalf("click Horizon source button failed: %v", err)
	}
	waitUI(t, tt, func() bool { return !a.searchLoading && len(a.searchResult.Projects) == len(horizonProjects) })
	tt.Advance(450 * time.Millisecond)
	snapshotUI(t, tt, "discover-horizon-zh")

	if a.provider != "horizon" || normalizeResourceType(a.resourceType) != "mod" {
		t.Fatalf("expected provider=horizon type=mod after switching from modpack, got provider=%q type=%q", a.provider, a.resourceType)
	}
	if lastQuery.Source != "horizon" || lastQuery.Loader != "" {
		t.Fatalf("unexpected Horizon discover query: %+v", lastQuery)
	}
	if _, ok := tt.Find(a.tr().DiscoverTypeModpack); ok {
		t.Fatal("Discover should hide 整合包 when Horizon source is active")
	}
	if _, ok := tt.Find(a.tr().DiscoverFilterLoader); ok {
		t.Fatal("Discover should hide Java loader filter when Horizon source is active")
	}
	// Verify all three source buttons sit cleanly inside the 340px rail without overlapping.
	rMR, ok1 := tt.Find("Modrinth")
	rCF, ok2 := tt.Find("CurseForge")
	rHZ, ok3 := tt.Find("Horizon")
	if !ok1 || !ok2 || !ok3 {
		t.Fatalf("expected all 3 source buttons visible, got mr=%v cf=%v hz=%v", ok1, ok2, ok3)
	}
	if rMR.X+rMR.W > rCF.X+1 || rCF.X+rCF.W > rHZ.X+1 || rHZ.X+rHZ.W > 340 {
		t.Fatalf("source buttons overlap or exceed rail: mr=%+v cf=%+v hz=%+v", rMR, rCF, rHZ)
	}
	if !tt.HasText("Flesh to Leather") || !tt.HasText(a.L("discover.installed")) {
		t.Fatalf("expected Horizon cards and installed badge; texts: %v", tt.Texts())
	}

	// Open Horizon project detail dialog and verify Horizon source badge + 26.3 / Horizon version tags.
	a.openProject(horizonProjects[0])
	waitUI(t, tt, func() bool {
		return !a.projLoading && len(a.projectVersions) == 1 && !a.planLoading && len(a.plan) == 1
	})
	tt.Advance(450 * time.Millisecond)
	snapshotUI(t, tt, "discover-horizon-project-zh")
	if !tt.HasText("Flesh to Leather") || !tt.HasText("1.1.0") || !tt.HasText("26.3") {
		t.Fatalf("expected Horizon project dialog version details; texts: %v", tt.Texts())
	}
	if _, disabled, _ := a.installAction(); disabled {
		t.Fatalf("expected Horizon mod install action to be enabled for Horizon 26.3 instance, plan=%+v", a.plan)
	}

	// Verify narrow 900x540 Discover layout with 3-way source switch.
	a.closeDialogNow()
	ttNarrow := ui.NewTester(a.View, 900, 540)
	ttNarrow.Frame()
	ttNarrow.Advance(450 * time.Millisecond)
	snapshotUI(t, ttNarrow, "discover-horizon-900")

	// Verify shaderLoader reports Horizon built-in shader runtime and in-place toggle keeps .hmod filename.
	if name, ver, known := a.shaderLoader(); !known || name != "Horizon" || ver != "nightly-2026-11-20-0a875da" {
		t.Fatalf("shaderLoader() = (%q, %q, %v), want (Horizon, nightly-2026-11-20-0a875da, true)", name, ver, known)
	}
	if !a.applyToggle("mods", "zoom_helper.hmod", "zoom_helper.hmod") || a.mods[0].Enabled || a.mods[0].File != "zoom_helper.hmod" {
		t.Fatalf("expected in-place Horizon mod toggle without renaming, got %+v", a.mods[0])
	}
}

func TestHorizonInPlaceUpgradeUIAndJobTitles(t *testing.T) {
	a, tt := acctTester(t, "zh-CN", 1440, 900)
	a.closeDialog()
	verID := "horizon-nightly-2026-11-20-0a875da"
	oldBuild := "nightly-2026-11-20-0a875da"
	newBuild := "nightly-2026-11-21-9b4c2de"
	a.snapshot.ActiveAccount = "ms-1"
	a.snapshot.Instances = []launcher.Instance{{
		Root: launcher.Root{ID: "r1", Name: "Horizon", Path: `D:\MC\.minecraft`, SelectedVersion: verID},
		Versions: []launcher.VersionSummary{
			{ID: verID, Minecraft: "26.3", Loader: "horizon", LoaderVersion: oldBuild, JavaVersion: 25},
		},
	}}
	a.snapshot.ActiveRoot = "r1"
	a.instStatusKey = "r1|" + verID
	a.instStatus = launcher.InstanceStatus{Dir: `D:\MC\.minecraft\versions\` + verID}
	a.storeHorizonUpdate("r1", verID, launcher.HorizonUpdateInfo{
		CurrentBuild: oldBuild,
		Latest:       &launcher.HorizonRelease{BuildID: newBuild, ReleaseTag: "nightly", Minecraft: "26.3"},
		HasUpdate:    true,
		Releases: []launcher.HorizonRelease{
			{BuildID: newBuild, ReleaseTag: "nightly", Minecraft: "26.3"},
			{BuildID: oldBuild, ReleaseTag: "nightly-2026-11-20", Minecraft: "26.3"},
		},
		LocalBuilds: []string{oldBuild, "nightly-2026-11-15-1111111"},
	})

	// 1. Hero update chip & Overview runtime action buttons appear.
	a.page, a.instTab = "home", "overview"
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	for _, want := range []string{a.L("horizon.update_to") + newBuild, a.L("horizon.upgrade_latest"), a.L("horizon.check_update"), a.L("horizon.switch_build")} {
		if !tt.HasText(want) {
			t.Errorf("overview/hero missing %q; visible texts: %v", want, tt.Texts())
		}
	}

	// 2. Settings tab shows the in-place build upgrade card and selector.
	a.instTab = "settings"
	a.loadVersionConfigForm(verID)
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	for _, want := range []string{a.L("horizon.build_section"), a.L("horizon.upgrade_latest"), a.L("horizon.upgrade_keep_data")} {
		if !tt.HasText(want) {
			t.Errorf("settings missing %q; visible texts: %v", want, tt.Texts())
		}
	}

	// 3. Job title keys translate cleanly in both zh-CN and en-US.
	j := launcher.Job{Title: "更新 Horizon " + newBuild, State: "running"}
	j.TitleKey, j.TitleArg, _ = launcher.JobTitleKey(j.Title)
	a.settings.Language = LangZH
	if got := a.jobTitle(j); got != "更新 Horizon "+newBuild {
		t.Fatalf("zh upgrade job title = %q", got)
	}
	a.settings.Language = LangEN
	if got := a.jobTitle(j); got != "Update Horizon "+newBuild {
		t.Fatalf("en upgrade job title = %q", got)
	}

	jLatest := launcher.Job{Title: "更新 Horizon", State: "running"}
	jLatest.TitleKey, jLatest.TitleArg, _ = launcher.JobTitleKey(jLatest.Title)
	if got := a.jobTitle(jLatest); got != "Update Horizon" {
		t.Fatalf("en latest upgrade job title = %q", got)
	}
}
