package nativeui

import (
	"image"
	"image/color"
	"testing"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestDockHasNoManageButton(t *testing.T) {
	for _, lang := range []string{"zh-CN", "en-US"} {
		a, tt := homeTester(t, lang)
		dock, _ := tt.Find("启动栏")
		if r, ok := tt.Find(a.tr().HomeInstanceCfgBtn); ok && r.Y >= dock.Y {
			t.Errorf("%s: manage button still in bottom bar", lang)
		}
		if _, ok := tt.Find(a.tr().HomeLaunchBtn); !ok {
			t.Errorf("%s: launch missing", lang)
		}
	}
}

func TestVersionDialogHasNoDirectoryFields(t *testing.T) {
	a, tt := homeTester(t, "zh-CN")
	tr := a.tr()
	a.openVersionConfig("r1", "fabric-loader-0.20.1-1.21.8")
	tt.Frame()
	tt.Frame()
	if a.dialog != "version" {
		t.Fatalf("gear opened %q", a.dialog)
	}
	if a.snapshot.Instances[0].SelectedVersion != "quilt-loader-0.20.0-beta.9-1.21.8" {
		t.Error("opening version settings must not change the selected version")
	}
	for _, s := range []string{tr.AddRootNameLabel, tr.RootCfgJavaLabel, tr.HomeOpenFolderBtn, tr.HomeLogsBtn, tr.RootCfgUnregisterBtn} {
		if _, ok := tt.Find(s); ok {
			t.Errorf("version dialog shows directory item %q", s)
		}
	}
	if !tt.HasText("版本设置标题") || !tt.HasText("Minecraft 1.21.8") || !tt.HasText(tr.RootCfgVerJavaLabel) {
		t.Error("version dialog missing its version header or Java field")
	}
}

func TestDirectoryDialogFromTab(t *testing.T) {
	a, tt := homeTester(t, "zh-CN")
	tr := a.tr()
	clickUI(t, tt, tr.HomeDirSettingsBtn)
	tt.Frame()
	if a.dialog != "root" || !tt.HasText(tr.DialogTitleDir) {
		t.Fatalf("directory dialog not opened from tab: %q", a.dialog)
	}
	for _, s := range []string{tr.HomeOpenFolderBtn, tr.HomeLogsBtn, tr.AddRootNameLabel, tr.RootCfgUnregisterBtn} {
		if !tt.HasText(s) {
			t.Errorf("directory dialog missing %q", s)
		}
	}
	if tt.HasText(tr.RootCfgVerJVMLabel) {
		t.Error("directory dialog should not contain per-version JVM args")
	}
}

func TestSkinHeadAndDefaults(t *testing.T) {
	skin := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	skin.SetNRGBA(8, 8, color.NRGBA{255, 0, 0, 255})
	skin.SetNRGBA(9, 8, color.NRGBA{0, 0, 255, 255})
	skin.SetNRGBA(41, 8, color.NRGBA{0, 255, 0, 255}) // hat covers face pixel (1,0)
	h := skinHead(skin)
	if h == nil || h.Bounds().Dx() != 8 {
		t.Fatal("head not extracted")
	}
	if r, _, _, _ := h.At(0, 0).RGBA(); r>>8 != 255 {
		t.Error("face pixel lost")
	}
	if _, g, _, _ := h.At(1, 0).RGBA(); g>>8 != 255 {
		t.Error("hat layer not composited")
	}
	if skinHead(image.NewNRGBA(image.Rect(0, 0, 16, 16))) != nil {
		t.Error("tiny image should not produce a head")
	}
	// java.util.UUID.hashCode parity: known values
	if defaultIsAlex("00000000-0000-0000-0000-000000000000") {
		t.Error("zero uuid should be Steve")
	}
	if !defaultIsAlex("00000000-0000-0000-0000-000000000001") {
		t.Error("uuid ...01 should be Alex")
	}
	if defaultIsAlex("not-a-uuid") {
		t.Error("invalid uuid falls back to Steve")
	}
}

func TestAccountAvatarFallbackThenHead(t *testing.T) {
	a, tt := homeTester(t, "zh-CN")
	ac := launcher.Account{ID: "missing", Name: "Ghost", Kind: "microsoft"}
	if a.accountHead(ac) != nil {
		t.Error("unknown account must fall back to the generic icon")
	}
	off, e := a.service.AddOffline("Steve")
	if e != nil {
		t.Fatal(e)
	}
	a.snapshot = a.service.Snapshot()
	a.snapshot.Instances, a.snapshot.ActiveRoot = homeFixture, "r1"
	a.accountHead(off)
	for i := 0; i < 50 && a.avatars.heads[off.ID] == nil; i++ {
		tt.Frame()
	}
	if a.avatars.heads[off.ID] == nil {
		t.Fatal("offline account should get a default head")
	}
	tt.Frame()
	if _, ok := tt.Find("头像 Steve"); !ok {
		t.Error("account pill should show the skin head")
	}
}
