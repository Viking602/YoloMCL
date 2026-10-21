package nativeui

import (
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"mcl/internal/launcher"
)

func fix5Shot(t *testing.T, tt *ui.Tester, name string) {
	if os.Getenv("MCL_NATIVE_QA_OUTPUT") != "" && os.Getenv("MCL_SHOT_PREFIX") != "" {
		snapshotUI(t, tt, os.Getenv("MCL_SHOT_PREFIX")+name)
	}
}

// drain runs posted UI work for up to d.
func drain(a *App, tt *ui.Tester, d time.Duration) {
	end := time.Now().Add(d)
	for time.Now().Before(end) {
		select {
		case fn := <-a.events:
			fn()
			tt.Frame()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// Real service state, no fixture: an offline account that the service
// knows is current must show its wardrobe without any error banner.
func TestWardrobeFollowsRealAccount(t *testing.T) {
	t.Setenv("MCL_DEVELOPMENT", "")
	t.Setenv("CURSEFORGE_API_KEY", "")
	s, e := launcher.New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if _, e := s.AddOffline("RealSteve"); e != nil {
		t.Fatal(e)
	}
	a, e := New(s)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(a.cancel)
	a.SetLanguage("zh-CN")
	tt := ui.NewTester(a.View, 1280, 800)
	tt.Frame()
	drain(a, tt, 300*time.Millisecond)
	a.goPage("wardrobe")
	tt.Frame()
	drain(a, tt, 800*time.Millisecond)
	tt.Advance(500 * time.Millisecond)
	if a.message != "" || a.appearanceErr != "" || tt.HasText("请先添加并选择账号") {
		t.Fatalf("stale/contradictory wardrobe error: msg=%q appearanceErr=%q", a.message, a.appearanceErr)
	}
	if !tt.HasText("RealSteve") {
		t.Fatal("current account not shown")
	}
	fix5Shot(t, tt, "1_wardrobe_after_1x")
}

func lum(c ui.Color) float64 {
	f := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= .03928 {
			return x / 12.92
		}
		return math.Pow((x+.055)/1.055, 2.4)
	}
	return .2126*f(c.R) + .7152*f(c.G) + .0722*f(c.B)
}

func TestDisabledButtonsReadableWithReason(t *testing.T) {
	l1, l2 := lum(disabledFill), lum(disabledInk)
	if r := (math.Max(l1, l2) + .05) / (math.Min(l1, l2) + .05); r < 4.5 {
		t.Errorf("disabled label contrast %.2f < 4.5", r)
	}
	a, tt := acctTester(t, "zh-CN", 1280, 800)
	a.openDialog("add")
	tt.Frame()
	tt.Advance(500 * time.Millisecond)
	if _, ok := tt.Find("禁用原因 " + a.tr().WhyNoFolder); !ok {
		t.Error("add-directory confirm should say why it is disabled")
	}
	fix5Shot(t, tt, "2_disabled_adddir_after_1x")
	// clicking it does nothing
	if e := tt.Click(a.tr().AddRootConfirmBtn); e == nil && a.busy {
		t.Error("disabled confirm ran")
	}
	a.closeDialog()
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	a.snapshot.ActiveRoot = "r2" // empty directory: launch disabled
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	fix5Shot(t, tt, "2_disabled_launch_after_1x")
}

func TestToastDoesNotPushLayoutAndRetries(t *testing.T) {
	a, tt := acctTester(t, "zh-CN", 1280, 800)
	a.closeDialog()
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	before, _ := tt.Find("主版本卡")
	retried := false
	a.message, a.errorMessage = "下载失败：连接超时 (resources.download.minecraft.net)", true
	a.noticeFor, a.noticeRetry = a.message, func() { retried = true }
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	after, _ := tt.Find("主版本卡")
	if before.Y != after.Y {
		t.Errorf("notice moved the page: %v -> %v", before.Y, after.Y)
	}
	if _, ok := tt.Find("错误通知"); !ok {
		t.Fatal("typed error notice missing")
	}
	for _, s := range []string{a.tr().NoticeLogs, a.tr().NoticeCopy, a.tr().DismissNotice} {
		if _, ok := tt.Find(s); !ok {
			t.Errorf("notice action %q missing", s)
		}
	}
	fix5Shot(t, tt, "3_notice_after_1x")
	clickUI(t, tt, a.tr().NoticeCopy)
	if tt.Clipboard() != "下载失败：连接超时 (resources.download.minecraft.net)" {
		t.Errorf("copy put %q", tt.Clipboard())
	}
	clickUI(t, tt, a.tr().NoticeRetry)
	if !retried || a.message != "" {
		t.Error("retry should run the failed action and clear the notice")
	}
	tt.Advance(400 * time.Millisecond)
	if _, ok := tt.Find("错误通知"); ok {
		t.Error("notice should ease out")
	}
}

func TestOneNamingEverywhere(t *testing.T) {
	a, tt := acctTester(t, "zh-CN", 1280, 800)
	fo := launcher.Instance{Root: launcher.Root{ID: "fo", Name: "Fabulously Optimized", Path: "/games/fo/.minecraft", SelectedVersion: "fabric-loader-0.19.3-1.21.1"}, Available: true,
		Pack:       &launcher.PackLock{Name: "Fabulously Optimized", Version: "6.5.0", Loader: "fabric", LoaderVersion: "0.19.3", Minecraft: "1.21.1", GameVersion: "fabric-loader-0.19.3-1.21.1"},
		PackUpdate: &launcher.PackUpdateInfo{HasUpdate: true, Latest: &launcher.ProjectVersion{}},
		Versions: []launcher.VersionSummary{
			{ID: "1.21.8", Minecraft: "1.21.8", Loader: "vanilla", JavaVersion: 21},
			{ID: "fabric-loader-0.19.3-1.21.1", Minecraft: "1.21.1", Loader: "fabric"},
		}}
	a.snapshot.Instances = append(a.snapshot.Instances, fo)
	a.snapshot.ActiveRoot = "fo"
	a.closeDialog()
	tt.Frame()
	tt.Advance(500 * time.Millisecond)
	for _, s := range tt.Texts() {
		if strings.Count(s, "原版") > 1 || strings.Count(s, "Fabulously Optimized") > 1 {
			t.Errorf("duplicated naming: %q", s)
		}
	}
	if !tt.HasText("v6.5.0 · Fabric 0.19.3") {
		t.Error("modpack subtitle missing")
	}
	fix5Shot(t, tt, "4_naming_pack_after_1x")
	if got := a.versionLine(fo, "fabric-loader-0.19.3-1.21.1"); got != "Fabulously Optimized 1.21.1" {
		t.Errorf("versionLine = %q", got)
	}
	a.goPage("mods")
	tt.Frame()
	tt.Advance(500 * time.Millisecond)
	if !tt.HasText("Fabulously Optimized 1.21.1") {
		t.Error("mods target picker should use the display name")
	}
	fix5Shot(t, tt, "4_naming_mods_after_1x")
}

func TestKeyboardFocusEscEnter(t *testing.T) {
	for _, sc := range []float32{1, 2} {
		a, tt := acctTester(t, "zh-CN", 1280, 800)
		tt.SetScale(sc)
		a.closeDialog()
		tt.Frame()
		for i := 0; i < 4; i++ {
			tt.Key(0, ui.KeyTab)
			tt.Advance(50 * time.Millisecond)
		}
		tt.Advance(300 * time.Millisecond)
		fix5Shot(t, tt, map[float32]string{1: "5_focus_after_1x", 2: "5_focus_after_2x"}[sc])
	}
	a, tt := acctTester(t, "zh-CN", 1280, 800)
	// Esc closes a dialog
	a.openDialog("root")
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.dialogOpen {
		t.Error("Escape should close the dialog")
	}
	tt.Advance(400 * time.Millisecond)
	// Enter runs the dialog's primary action
	a.openDialog("add")
	a.form.Path = t.TempDir()
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	if !a.busy && a.message == "" {
		t.Errorf("Enter should run 确认添加目录: busy=%v open=%v msg=%q", a.busy, a.dialogOpen, a.message)
	}
	// login method tabs (missed by the audit)
	a2, tt2 := acctTester(t, "zh-CN", 1280, 800)
	clickUI(t, tt2, a2.tr().AccAddAccountBtn)
	tt2.Frame()
	tt2.Advance(400 * time.Millisecond)
	fix5Shot(t, tt2, "5_login_tabs_offline_1x")
	clickUI(t, tt2, a2.tr().AccTabThirdParty)
	tt2.Frame()
	tt2.Advance(400 * time.Millisecond)
	fix5Shot(t, tt2, "5_login_tabs_thirdparty_1x")
}
