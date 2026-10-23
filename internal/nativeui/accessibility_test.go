package nativeui

import (
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func ratio(a, b ui.Color) float64 {
	l1, l2 := lum(a), lum(b)
	return (math.Max(l1, l2) + .05) / (math.Min(l1, l2) + .05)
}

func TestAccessibilityTagContrast(t *testing.T) {
	// tag text over its 10% tint on a white card
	if r := ratio(tagNeutral, ui.Hex("#eef0f3")); r < 4.5 {
		t.Errorf("neutral tag contrast %.2f", r)
	}
}

func p2Fixture(t *testing.T, w, h int, sc float32) (*App, *ui.Tester) {
	a, tt := acctTester(t, "zh-CN", w, h)
	tt.SetScale(sc)
	a.closeDialog()
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	return a, tt
}

func settle(tt *ui.Tester) { tt.Frame(); tt.Advance(600 * time.Millisecond) }

func TestAccessibilityInfoToastAutoDismiss(t *testing.T) {
	a, tt := p2Fixture(t, 1280, 800, 1)
	a.notice("已保存")
	settle(tt)
	if _, ok := tt.Find("提示通知"); !ok {
		t.Fatal("info toast missing")
	}
	r, _ := tt.Find("页面通知")
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(5 * time.Second)
	if a.message == "" {
		t.Error("hover should hold the toast")
	}
	tt.Move(5, 400)
	tt.Advance(5 * time.Second)
	if a.message != "" {
		t.Error("info toast should auto-dismiss ~4s after the pointer leaves")
	}
	a.message, a.errorMessage = "失败", true
	settle(tt)
	tt.Advance(6 * time.Second)
	if a.message == "" {
		t.Error("errors must stay until dismissed")
	}
}

func TestAccessibilityAccountRemoveIsTwoStep(t *testing.T) {
	a, tt := p2Fixture(t, 1280, 800, 1)
	a.openDialog("account")
	settle(tt)
	n := len(a.snapshot.Accounts)
	clickUI(t, tt, a.tr().AccRemoveBtn)
	settle(tt)
	if (len(a.pendingRequests) > 0) || len(a.snapshot.Accounts) != n {
		t.Fatal("first click must only arm the removal")
	}
	if _, ok := tt.Find(a.tr().AccRemoveConfirm); !ok {
		t.Fatal("confirm step missing")
	}
	snapIf(t, tt, "p2_13_account_after_1x")
}

func snapIf(t *testing.T, tt *ui.Tester, name string) {
	if shotsOn() {
		snapshotUI(t, tt, name)
	}
}

func TestAccessibilityShots(t *testing.T) {
	if !shotsOn() {
		t.Skip()
	}
	a, tt := p2Fixture(t, 1280, 800, 1)
	snapIf(t, tt, "p2_06_tags_after_1x")
	a2, tt2 := p2Fixture(t, 1280, 800, 2)
	_ = a2
	snapIf(t, tt2, "p2_06_tags_after_2x")
	a.openDialog("install")
	settle(tt)
	snapIf(t, tt, "p2_07_paths_after_1x")
	a.closeDialog()
	settle(tt)
	a.openVersionConfig("r1", "fabric-loader-0.20.1-1.21.8")
	settle(tt)
	snapIf(t, tt, "p2_08_title_after_1x")
	snapIf(t, tt, "p2_12_jvm_after_1x")
	a3, tt3 := p2Fixture(t, 960, 640, 1)
	a3.openVersionConfig("r1", "fabric-loader-0.20.1-1.21.8")
	settle(tt3)
	snapIf(t, tt3, "p2_12_jvm_after_min")
	a.closeDialog()
	settle(tt)
	a.goPage("tasks")
	settle(tt)
	snapIf(t, tt, "p2_09_empty_tasks_after_1x")
	a.goPage("mods")
	a.mods = nil
	settle(tt)
	snapIf(t, tt, "p2_09_empty_mods_after_1x")
	var ps []launcher.Project
	for i := 0; i < 8; i++ {
		ps = append(ps, launcher.Project{ID: fmt.Sprint(i), Source: "modrinth", Title: fmt.Sprintf("Example Modpack %d", i), Author: "author", Downloads: int64(1000000 * (9 - i)), Categories: []string{"optimization", "multiplayer"}, Description: "A calm example description that is long enough to need ellipsis at narrow widths."})
	}
	a.searchResult = launcher.SearchResult{Projects: ps, Total: 80, Limit: 8}
	a.goPage("discover")
	a.searchResult = launcher.SearchResult{Projects: ps, Total: 80, Limit: 8}
	settle(tt)
	snapIf(t, tt, "p2_10_discover_after_1x")
	a4, tt4 := p2Fixture(t, 960, 640, 1)
	a4.searchResult = launcher.SearchResult{Projects: ps, Total: 80, Limit: 8}
	a4.page = "discover"
	settle(tt4)
	snapIf(t, tt4, "p2_10_discover_after_min")
	a.goPage("settings")
	settle(tt)
	snapIf(t, tt, "p2_11_settings_after_1x")
	// Tab walk through main dialogs: focus must stay inside the dialog.
	for _, d := range []string{"add", "account", "install", "root", "version"} {
		b, tb := p2Fixture(t, 1280, 800, 1)
		if d == "version" {
			b.openVersionConfig("r1", "fabric-loader-0.20.1-1.21.8")
		} else {
			b.openDialog(d)
		}
		settle(tb)
		for i := 0; i < 3; i++ {
			tb.Key(0, ui.KeyTab)
			tb.Advance(60 * time.Millisecond)
		}
		tb.Advance(300 * time.Millisecond)
		snapIf(t, tb, "p2_tab_"+d+"_1x")
	}
}

func shotsOn() bool { return os.Getenv("MCL_NATIVE_QA_OUTPUT") != "" }
