package nativeui

import (
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func waitUI(t *testing.T, tt *ui.Tester, predicate func() bool) {
	t.Helper()
	until := time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		tt.Frame()
		if predicate() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("native UI did not finish: %v", tt.Texts())
}
func clickUI(t *testing.T, tt *ui.Tester, text string) {
	t.Helper()
	if e := tt.Click(text); e != nil {
		t.Fatal(e)
	}
}
func typeUI(t *testing.T, tt *ui.Tester, label, text string) {
	t.Helper()
	clickUI(t, tt, label)
	tt.Type(text)
}
func snapshotUI(t *testing.T, tt *ui.Tester, name string) {
	t.Helper()
	dir := os.Getenv("MCL_NATIVE_QA_OUTPUT")
	if dir == "" {
		return
	}
	if e := os.MkdirAll(dir, 0755); e != nil {
		t.Fatal(e)
	}
	f, e := os.Create(filepath.Join(dir, name+".png"))
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if e = png.Encode(f, tt.Image()); e != nil {
		t.Fatal(e)
	}
}

// This drives native controls, persisted service data and local mod files in one
// flow. It catches broken bindings that compiling the UI alone cannot catch.
func TestNativeOfflineDirectoryAndModFlow(t *testing.T) {
	t.Setenv("MCL_DEVELOPMENT", "")
	s, e := launcher.New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a, e := New(s)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { a.cancel(); a.bgWG.Wait() }()
	a.systemLanguage = LangZH
	a.normalizeStateValues()
	tt := ui.NewTester(a.View, 1320, 860)
	if !tt.HasText("游戏版本") {
		t.Fatal(tt.Texts())
	}
	for y := float32(20); y < 860; y += 80 {
		for x := float32(20); x < 1320; x += 80 {
			if tt.WebViewAt(x, y) != nil {
				t.Fatal("native launcher unexpectedly contains a WebView")
			}
		}
	}
	snapshotUI(t, tt, "home")
	clickUI(t, tt, "管理账号")
	clickUI(t, tt, "离线登录")
	typeUI(t, tt, "角色名输入", "Native_Player")
	if a.form.Username != "Native_Player" {
		t.Fatalf("native text input did not update: %q", a.form.Username)
	}
	snapshotUI(t, tt, "account")
	clickUI(t, tt, "添加离线账号")
	waitUI(t, tt, func() bool { return !(len(a.pendingRequests) > 0) && !a.dialogOpen && len(a.snapshot.Accounts) == 1 })
	if a.snapshot.Accounts[0].Kind != "offline" {
		t.Fatal(a.snapshot.Accounts)
	}
	clickUI(t, tt, "衣橱")
	waitUI(t, tt, func() bool { return !a.appearanceLoading && a.appearanceAccount != "" })
	if !tt.HasText("本机") {
		t.Fatal("offline appearance scope missing")
	}
	snapshotUI(t, tt, "wardrobe")
	game := filepath.Join(t.TempDir(), "A world with spaces")
	mc := filepath.Join(game, ".minecraft")
	if e = os.MkdirAll(filepath.Join(mc, "mods"), 0755); e != nil {
		t.Fatal(e)
	}
	var jar bytes.Buffer
	z := zip.NewWriter(&jar)
	entry, _ := z.Create("fabric.mod.json")
	_, _ = entry.Write([]byte(`{"schemaVersion":1,"id":"native_test","version":"1.2.3","name":"测试模组"}`))
	_ = z.Close()
	if e = os.WriteFile(filepath.Join(mc, "mods", "native-test.jar"), jar.Bytes(), 0644); e != nil {
		t.Fatal(e)
	}
	clickUI(t, tt, "实例")
	clickUI(t, tt, "添加目录")
	typeUI(t, tt, "游戏文件夹输入", game)
	typeUI(t, tt, "目录名称输入", "生存世界")
	clickUI(t, tt, "确认添加目录")
	waitUI(t, tt, func() bool { return !(len(a.pendingRequests) > 0) && !a.dialogOpen && len(a.snapshot.Instances) == 1 })
	if a.snapshot.Instances[0].Path != mc {
		t.Fatalf("directory must resolve its .minecraft: %q", a.snapshot.Instances[0].Path)
	}
	a.goPage("mods") // 模组 is an instance tab now
	tt.Frame()
	waitUI(t, tt, func() bool { return !a.modsLoading && len(a.mods) == 1 })
	if !tt.HasText("测试模组") || !tt.HasText("1.2.3") {
		t.Fatal(tt.Texts())
	}
	clickUI(t, tt, "切换启用 测试模组")
	waitUI(t, tt, func() bool {
		return !(len(a.pendingRequests) > 0) && !a.modsLoading && len(a.mods) == 1 && !a.mods[0].Enabled
	})
	if _, e = os.Stat(filepath.Join(mc, "mods", "native-test.jar.disabled")); e != nil {
		t.Fatal(e)
	}
	snapshotUI(t, tt, "mods")
	clickUI(t, tt, "管理账号")
	clickUI(t, tt, "设置")
	for _, text := range tt.Texts() {
		if strings.Contains(text, "Client ID") || strings.Contains(text, "API Key") {
			t.Fatalf("player UI asks for publisher credentials: %s", text)
		}
	}
	snapshotUI(t, tt, "settings")
	// The window minimum is supported without losing the global navigation.
	tt.SetSize(1060, 720)
	if !tt.HasText("设置") || !tt.HasText("管理账号") {
		t.Fatal("navigation lost at minimum window size")
	}
	restored, e := launcher.New(a.snapshot.DataDir)
	if e != nil {
		t.Fatal(e)
	}
	snap := restored.Snapshot()
	if len(snap.Accounts) != 1 || len(snap.Instances) != 1 {
		t.Fatal("UI changes did not persist")
	}
}

func TestNativeSkinTextureAndOrbit(t *testing.T) {
	p := newSkinPreview()
	skin := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			skin.SetNRGBA(x, y, color.NRGBA{220, 30, 20, 255})
		}
	}
	p.set(skin, nil, false)
	front := p.render(160, 180)
	red := 0
	for y := 0; y < 180; y++ {
		for x := 0; x < 160; x++ {
			v := front.RGBAAt(x, y)
			if v.R > 100 && v.G < 50 && v.A > 0 {
				red++
			}
		}
	}
	if red < 500 {
		t.Fatalf("skin texture not mapped onto native cuboids: %d red pixels", red)
	}
	p.yaw = 1.4
	side := p.render(160, 180)
	if bytes.Equal(front.Pix, side.Pix) {
		t.Fatal("orbit does not change preview")
	}
	cape := image.NewNRGBA(image.Rect(0, 0, 64, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			cape.SetNRGBA(x, y, color.NRGBA{10, 220, 20, 255})
		}
	}
	p.set(skin, cape, false)
	p.yaw = 3.14
	back := p.render(160, 180)
	green := 0
	for i := 0; i < len(back.Pix); i += 4 {
		if back.Pix[i+1] > 100 && back.Pix[i] < 50 {
			green++
		}
	}
	if green < 100 {
		t.Fatalf("cape not visible on back: %d green pixels", green)
	}
}

func TestBilingualSwitchingAndCoreWorkflows(t *testing.T) {
	t.Setenv("MCL_DEVELOPMENT", "")
	s, e := launcher.New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a, e := New(s)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { a.cancel(); a.bgWG.Wait() }()
	a.systemLanguage = LangZH
	a.normalizeStateValues()
	tt := ui.NewTester(a.View, 1440, 920)
	if !tt.HasText("游戏版本") {
		t.Fatalf("expected system zh-CN home title, got: %v", tt.Texts())
	}
	// Verify zero duplicate action texts on the Home screen
	for _, uniqueText := range []string{"添加账号", "安装游戏", "添加目录", "导入整合包"} {
		count := 0
		for _, txt := range tt.Texts() {
			if txt == uniqueText {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("expected %q to appear exactly once on Home, got %d in %v", uniqueText, count, tt.Texts())
		}
	}
	// Toggle to English from the titlebar language switcher
	clickUI(t, tt, "EN")
	waitUI(t, tt, func() bool { return tt.HasText("Game Instances") })

	// Verify English dialogs for Modpack Import and Game/Modloader Installation
	clickUI(t, tt, "Import Modpack")
	if !tt.HasText("Modpack Archive") || !tt.HasText("Location") {
		t.Fatalf("English import modpack dialog incomplete: %v", tt.Texts())
	}
	clickUI(t, tt, "Close")

	clickUI(t, tt, "Install Game")
	if !tt.HasText("Install Minecraft & Mod Loader Wizard") || !tt.HasText("Step 3: Choose Mod Loader & Version") {
		t.Fatalf("English install dialog incomplete: %v", tt.Texts())
	}
	clickUI(t, tt, "Close")

	// Verify English navigation across pages
	a.goPage("mods") // no directory yet: mods redirect to the Instances view
	tt.Frame()
	if a.page != "home" || a.instTab != "mods" || !tt.HasText("Instances") {
		t.Fatalf("mods should open the Instances view: %v", tt.Texts())
	}
	clickUI(t, tt, "Wardrobe")
	if !tt.HasText("Character Wardrobe") {
		t.Fatalf("English Wardrobe page title missing: %v", tt.Texts())
	}
	clickUI(t, tt, "Tasks & Logs")
	if !tt.HasText("Tasks & Activity") {
		t.Fatalf("English Tasks page title missing: %v", tt.Texts())
	}
	// Settings now sit next to the launch button in the bottom bar.
	a.goPage("home")
	tt.Advance(400 * time.Millisecond)
	if _, ok := tt.Find("按钮文字 dock-settings"); ok {
		t.Fatalf("the launch dock should be gone: %v", tt.Texts())
	}
	clickUI(t, tt, "Settings")
	if !tt.HasText("Settings") || !tt.HasText("Runtime Environment") {
		t.Fatalf("English Settings page missing: %v", tt.Texts())
	}

	// Verify language persistence across service reload
	waitUI(t, tt, func() bool {
		restored, err := launcher.New(a.snapshot.DataDir)
		return err == nil && restored.Snapshot().Settings.Language == LangEN
	})

	// Switch back to Simplified Chinese inside Settings
	clickUI(t, tt, "简体中文")
	waitUI(t, tt, func() bool { return tt.HasText("设置") && tt.HasText("启动环境") })
}

func TestPerVersionConfigAndResourceShaderPacksFlow(t *testing.T) {
	t.Setenv("MCL_DEVELOPMENT", "")
	s, e := launcher.New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	rootDir := filepath.Join(t.TempDir(), "IsolatedPackRoot")
	verDir := filepath.Join(rootDir, "versions", "恶咒")
	if e = os.MkdirAll(filepath.Join(verDir, "mods"), 0755); e != nil {
		t.Fatal(e)
	}
	if e = os.MkdirAll(filepath.Join(verDir, "resourcepacks"), 0755); e != nil {
		t.Fatal(e)
	}
	if e = os.MkdirAll(filepath.Join(verDir, "shaderpacks"), 0755); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(verDir, "pack-profile.json"), []byte(`{"id":"1.20.1-forge","inheritsFrom":"1.20.1","mainClass":"cpw.mods.bootstraplauncher.BootstrapLauncher","javaVersion":{"majorVersion":17}}`), 0644); e != nil {
		t.Fatal(e)
	}
	// Create a valid resource pack zip with pack.mcmeta
	var rpBuf bytes.Buffer
	zw := zip.NewWriter(&rpBuf)
	f, _ := zw.Create("pack.mcmeta")
	_, _ = f.Write([]byte(`{"pack":{"pack_format":15,"description":"§aHigh-Res Faith Pack"}}`))
	_ = zw.Close()
	if e = os.WriteFile(filepath.Join(verDir, "resourcepacks", "faith-hd.zip"), rpBuf.Bytes(), 0644); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(verDir, "shaderpacks", "ComplementaryReimagined.zip"), rpBuf.Bytes(), 0644); e != nil {
		t.Fatal(e)
	}

	if _, e = s.AddRoot(verDir, "恶咒整合包"); e != nil {
		t.Fatal(e)
	}
	a, e := New(s)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { a.cancel(); a.bgWG.Wait() }()
	a.systemLanguage = LangZH
	a.normalizeStateValues()
	tt := ui.NewTester(a.View, 1440, 920)
	if !tt.HasText("恶咒") {
		t.Fatalf("expected version '恶咒' from versions/ to be visible on Home: %v", tt.Texts())
	}
	// Configure per-version Memory & G1GC JVM arguments
	a.setInstTab("settings") // 设置 is the instance tab in the split view
	tt.Frame()
	if !a.form.VerMemAuto || !tt.HasText("自动内存值") && !strings.Contains(strings.Join(tt.Texts(), " "), "自动（约") {
		t.Fatalf("new versions should default to automatic memory: %v", tt.Texts())
	}
	clickUI(t, tt, "手动")
	clickUI(t, tt, "4 GB")
	clickUI(t, tt, "G1GC 均衡 (推荐)")
	clickUI(t, tt, "保存")
	waitUI(t, tt, func() bool {
		return !(len(a.pendingRequests) > 0) && !a.dialogOpen && a.root().VersionConfigs["恶咒"].MemoryMB == 4096
	})
	vc := a.root().VersionConfigs["恶咒"]
	if vc.MemoryMB != 4096 || !strings.Contains(vc.JVMArgs, "-XX:+UseG1GC") {
		t.Fatalf("per-version config was not saved: %+v", vc)
	}

	// Switch to Mods & Addons page and test Resource Packs & Shaders tabs
	a.goPage("mods") // 模组 is an instance tab now
	tt.Frame()
	clickUI(t, tt, "资源包")
	waitUI(t, tt, func() bool { return !a.modsLoading && len(a.assetPacks) == 1 })
	if !tt.HasText("faith-hd") || !tt.HasText("High-Res Faith Pack") {
		t.Fatalf("resource pack metadata not rendered: %v", tt.Texts())
	}
	clickUI(t, tt, "光影")
	waitUI(t, tt, func() bool { return !a.modsLoading && len(a.assetPacks) == 1 })
	if !tt.HasText("ComplementaryReimagined") {
		t.Fatalf("shader pack not rendered: %v", tt.Texts())
	}
}
