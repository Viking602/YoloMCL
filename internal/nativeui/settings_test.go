package nativeui

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func settingsTester(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	// Each fixture represents a new window, not the previous test's saved
	// settings scroll position. A taller About footer makes that distinction visible.
	delete(scrollTracks, "settings-scroll")
	s, e := launcher.New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a, e := New(s)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(a.cancel)
	a.systemLanguage = LangZH // deterministic system preference for this fixture
	a.normalizeStateValues()
	a.page = "settings"
	a.settings.MemoryMode = "manual" // these tests exercise the manual slider
	return a, ui.NewTester(a.View, 1536, 1024)
}

func TestSettingsApprovedLayoutAndSmallWindow(t *testing.T) {
	_, tt := settingsTester(t)
	for _, label := range []string{"主导航", "启动环境面板", "游戏版本面板", "设置操作栏", "跟随系统", "简体中文", "English"} {
		got, ok := tt.Find(label)
		minY := float32(42)
		if label == "主导航" {
			minY = 0 // the nav is the title bar of the frameless window
		}
		if !ok || got.W <= 0 || got.H <= 0 || got.X < 0 || got.Y < minY || got.X+got.W > 1536 || got.Y+got.H > 1024 {
			t.Errorf("%s out of bounds or missing: %+v", label, got)
		}
	}
	if _, ok := tt.Find("内置服务面板"); ok {
		t.Error("built-in service status should not be shown in settings")
	}
	snapshotUI(t, tt, "settings-approved")
	for _, size := range [][2]int{{1320, 860}, {1060, 720}} {
		tt.SetSize(size[0], size[1])
		for _, label := range []string{"最大内存滑块", "最大内存输入", "跟随目录设置开关", "显示快照版本开关", "恢复默认", "跟随系统", "简体中文", "English"} {
			r, ok := tt.Find(label)
			if !ok || r.W <= 0 || r.H <= 0 || r.X < 0 || r.Y < 42 || r.X+r.W > float32(size[0]) || r.Y+r.H > float32(size[1]) {
				t.Errorf("%s not fully visible at %v: %+v", label, size, r)
			}
		}
	}
	snapshotUI(t, tt, "settings-small")
	// The About row follows the variable-height installed Java list in a
	// scrollable body; it must be reachable, not necessarily in the first view.
	tt.Scroll(700, 400, 0, 10000)
	if r, ok := tt.Find("关于 YoloMCL 面板"); !ok || r.W <= 0 || r.H <= 0 || r.Y < 42 || r.Y+r.H > 720 {
		t.Fatalf("About footer is not reachable: %+v", r)
	}
}

func TestNativeMemorySliderAndSettingsPersistence(t *testing.T) {
	a, tt := settingsTester(t)
	maxMB := min(16384, a.memoryInfo.MaxMB)
	if maxMB < 2048 {
		t.Skip("insufficient physical memory for slider interaction fixture")
	}
	a.memoryInfo.MaxMB = maxMB
	tt.Frame()
	r, ok := tt.Find("最大内存滑块")
	if !ok {
		t.Fatal("missing native memory slider")
	}
	tt.ClickAt(r.X+r.W-1, r.Y+r.H/2)
	if a.memory != float64(maxMB) || a.memoryText != strconv.Itoa(maxMB) {
		t.Fatalf("slider maximum did not update the numeric input: %v / %s", a.memory, a.memoryText)
	}
	tt.ClickAt(r.X+1, r.Y+r.H/2)
	if a.memory != 1024 || a.memoryText != "1024" {
		t.Fatalf("slider minimum: %v / %s", a.memory, a.memoryText)
	}
	wantMB := min(8192, maxMB)
	clickUI(t, tt, "最大内存输入")
	tt.Key(ui.Ctrl, ui.KeyA)
	tt.Type(strconv.Itoa(wantMB))
	if a.memory != float64(wantMB) {
		t.Fatalf("numeric input did not update the slider: %v", a.memory)
	}
	clickUI(t, tt, "跟随目录设置开关")
	clickUI(t, tt, "显示快照版本开关")
	waitUI(t, tt, func() bool {
		restored, err := launcher.New(a.snapshot.DataDir)
		return err == nil && restored.Snapshot().Settings.MemoryMB == wantMB && restored.Snapshot().Settings.UseRootSettings && restored.Snapshot().Settings.ShowSnapshots
	})
	restored, e := launcher.New(a.snapshot.DataDir)
	if e != nil {
		t.Fatal(e)
	}
	settings := restored.Snapshot().Settings
	if settings.MemoryMB != wantMB || !settings.UseRootSettings || !settings.ShowSnapshots {
		t.Fatalf("native controls did not persist: %+v", settings)
	}
	clickUI(t, tt, "恢复默认")
	waitUI(t, tt, func() bool {
		restored, err := launcher.New(a.snapshot.DataDir)
		return err == nil && !restored.Snapshot().Settings.UseRootSettings && !restored.Snapshot().Settings.ShowSnapshots && restored.Snapshot().Settings.MemoryMB == a.memoryInfo.RecommendedMB
	})
	if a.settings.UseRootSettings || a.settings.ShowSnapshots || a.memory != float64(a.memoryInfo.RecommendedMB) {
		t.Fatalf("restore defaults did not reset the draft: %+v", a.settings)
	}
}

func TestSettingsSaveKeepsLatestDraft(t *testing.T) {
	a, tt := settingsTester(t)
	defer func() { a.cancel(); a.bgWG.Wait() }()
	// Queue several drafts behind a slow write, then let the workers compete.
	// Neither persistence order nor a queued completion may undo a later edit.
	a.settingsMu.Lock()
	for _, mb := range []int{2048, 4096, 6144} {
		a.memoryText = strconv.Itoa(mb)
		a.settings.ShowSnapshots = mb == 6144
		a.persistSettings("")
	}
	a.settingsMu.Unlock()
	a.bgWG.Wait()
	if got := a.service.Snapshot().Settings; got.MemoryMB != 6144 || !got.ShowSnapshots {
		t.Fatalf("older draft overwrote newest settings: %+v", got)
	}
	// A completion already in the UI queue belongs to the preceding draft.
	a.memoryText = "4096"
	a.settings.UseRootSettings = true
	a.persistSettings("")
	tt.Frame()
	if a.settings.MemoryMB != 4096 || !a.settings.UseRootSettings || !a.settings.ShowSnapshots {
		t.Fatal("queued settings completion replaced the current draft")
	}
	a.bgWG.Wait()
	tt.Frame()
	if got := a.service.Snapshot().Settings; got.MemoryMB != 4096 || !got.UseRootSettings || !got.ShowSnapshots {
		t.Fatalf("latest settings did not persist: %+v", got)
	}
}

func TestNativeJavaFilterAndSelection(t *testing.T) {
	a, tt := settingsTester(t)
	path := filepath.Join(t.TempDir(), "portable-jdk21", "bin", "java.exe")
	a.java = []launcher.JavaInfo{{Path: path, Version: "21.0.10", Major: 21, Vendor: "Eclipse Adoptium", Architecture: "x86_64"}, {Path: filepath.Join(t.TempDir(), "jdk8", "bin", "java.exe"), Version: "1.8.0_452", Major: 8, Vendor: "Zulu"}}
	a.javaPhase = "找到 2 个 Java"
	a.openDialog("java")
	tt.Frame()
	if !tt.HasText("深入扫描") || !tt.HasText("Eclipse Adoptium") || !tt.HasText("x86_64") {
		t.Fatal("Java details or full-scan control missing")
	}
	typeUI(t, tt, "筛选 Java", "portable-jdk21")
	if tt.HasText("1.8.0_452") {
		t.Fatal("Java filter did not narrow the choices")
	}
	snapshotUI(t, tt, "java-detection")
	clickUI(t, tt, "使用")
	if a.dialogOpen || a.settings.JavaPath != path {
		t.Fatalf("Java choice was not applied: %q", a.settings.JavaPath)
	}
	waitUI(t, tt, func() bool { return a.snapshot.Settings.JavaPath == path })
}

func TestGlobalMemoryAutoDefault(t *testing.T) {
	a, tt := settingsTester(t)
	a.settings.MemoryMode = ""
	tt.Frame()
	if _, ok := tt.Find("最大内存滑块"); ok {
		t.Error("auto mode hides the manual slider")
	}
	clickUI(t, tt, a.tr().MemModeManual)
	tt.Frame()
	if a.settings.MemoryMode != "manual" {
		t.Error("switching to manual")
	}
	if _, ok := tt.Find("最大内存滑块"); !ok {
		t.Error("manual mode shows the slider")
	}
}
