package nativeui

import (
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestFloatingUIScreenshots(t *testing.T) {
	if os.Getenv("MCL_NATIVE_QA_OUTPUT") == "" {
		t.Skip("optional product captures")
	}
	a, _, _ := projectDialogFixture(t, 39)
	a.dialog, a.dialogOpen = "project", true
	tt := ui.NewTester(a.View, 1536, 1024)
	tt.Advance(400 * time.Millisecond)
	snapshotUI(t, tt, "installation-page")
	for _, label := range []string{"活动弹窗", "install-dialog", "按钮文字 install-confirm"} {
		r, _ := tt.Find(label)
		t.Log(label, r)
	}
	a.closeDialogNow()
	tt.Frame()
	a.openKeyBindings("", "")
	waitUI(t, tt, func() bool { return a.keyBindings.view != nil && !a.dialogPending() })
	tt.Advance(400 * time.Millisecond)
	snapshotUI(t, tt, "game-settings-drawer")
	a.closeDialogNow()
	a.openDialog("account")
	tt.Advance(400 * time.Millisecond)
	snapshotUI(t, tt, "accounts-drawer")
	a.closeDialogNow()
	a.openDialog("add")
	a.form.Name = "我的新实例 Minecraft"
	a.requestCloseDialog()
	tt.Frame()
	snapshotUI(t, tt, "unsaved-dialog")
}

func TestPopconfirmDismissalFocusAndSingleExecution(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	a, executed := &App{languageOverride: LangZH}, 0
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		ui.Column(c).Fill().Padding(40).Background(bg).Children(func() {
			a.confirmIcon(c, "delete-item", "trash", "删除备份", "world.zip", "删除后无法恢复", btnMd, false, func() { executed++ })
		})
	}, 900, 540)
	for _, dismiss := range []string{"escape", "outside", "cancel"} {
		clickUI(t, tt, "删除备份")
		tt.Advance(250 * time.Millisecond)
		if a.popconfirmKey == "" || executed != 0 {
			t.Fatal("first click executed the action")
		}
		if !tt.HasText("world.zip") {
			t.Fatal("confirmation omitted its target")
		}
		switch dismiss {
		case "escape":
			tt.Key(0, ui.KeyEscape)
		case "outside":
			tt.ClickAt(800, 480)
		case "cancel":
			clickUI(t, tt, "按钮文字 popconfirm-cancel-delete-item")
		}
		tt.Frame()
		if a.popconfirmKey != "" || executed != 0 {
			t.Fatal("dismissal changed data")
		}
		if dismiss != "outside" && !tt.Focused("删除备份") {
			t.Fatal("focus did not return to the opener")
		}
	}
	clickUI(t, tt, "删除备份")
	tt.Advance(250 * time.Millisecond)
	button, ok := tt.Find("按钮文字 popconfirm-accept-delete-item")
	if !ok {
		t.Fatal("missing named action")
	}
	tt.ClickAt(button.X+button.W/2, button.Y+button.H/2)
	tt.ClickAt(button.X+button.W/2, button.Y+button.H/2)
	if executed != 1 || a.popconfirmKey != "" {
		t.Fatal("confirmation executed more than once")
	}
}

func TestDirtyDialogCancelEscapeAndDiscard(t *testing.T) {
	a, tt := homeTester(t, LangZH)
	a.openDialog("add")
	tt.Advance(300 * time.Millisecond)
	a.form.Name = "Unsaved name"
	a.requestCloseDialog()
	tt.Frame()
	if a.confirmation == nil || !a.dialogOpen || a.form.Name != "Unsaved name" {
		t.Fatal("close discarded the draft")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.confirmation != nil || !a.dialogOpen || a.form.Name != "Unsaved name" {
		t.Fatal("Escape must cancel only the discard decision")
	}
	a.requestCloseDialog()
	tt.Frame()
	clickUI(t, tt, "按钮文字 confirmation-accept")
	if a.dialogOpen || a.confirmation != nil {
		t.Fatal("discard did not close the original form")
	}
}

func TestDrawerGeometryAndPageWithoutModal(t *testing.T) {
	for _, size := range [][2]int{{900, 540}, {1536, 1024}} {
		a, tt := homeTester(t, LangZH)
		tt.SetSize(size[0], size[1])
		a.openDialog("account")
		tt.Advance(400 * time.Millisecond)
		r, ok := tt.Find("活动弹窗")
		if !ok || r.X < 100 || r.X+r.W > float32(size[0]) || r.Y < 42 || r.Y+r.H > float32(size[1]) {
			t.Fatalf("drawer escaped its edge: %+v", r)
		}
		tt.Key(0, ui.KeyEscape)
		tt.Advance(300 * time.Millisecond)
		if a.dialogOpen {
			t.Fatal("Escape did not close the drawer")
		}
		a.dialog, a.dialogOpen = "install", true
		a.minecraftVersions = []launcher.MinecraftVersion{{ID: "1.21.1", Type: "release"}}
		tt.Frame()
		if !tt.HasText(a.tr().DialogTitleInstall) {
			t.Fatal("installation task page missing")
		}
		// The title-bar navigation stays keyboard reachable on a task page.
		r, ok = tt.Find(a.L("navbar.discover"))
		if !ok || r.Y > 42 {
			t.Fatal("main task hid global navigation")
		}
		if a.surfaceKind(a.dialog) != surfacePage {
			t.Fatal("installation is still modal")
		}
	}
}

func TestInstanceDraftSurvivesSecondaryDrawer(t *testing.T) {
	a, tt, _ := worldsTester(t)
	a.setInstTab("settings")
	tt.Frame()
	a.form.VerJVM = "-Dkeep.instance.draft=true"
	before := a.form
	a.openKeyBindings("", "")
	waitUI(t, tt, func() bool { return a.keyBindings.view != nil && !a.dialogPending() })
	tt.Advance(300 * time.Millisecond)
	a.closeDialog()
	tt.Frame()
	if a.form != before || !a.versionDraftDirty() {
		t.Fatal("secondary editor lost the page draft or its baseline")
	}
	a.goPage("tasks")
	if a.page != "home" || a.confirmation == nil {
		t.Fatal("navigation silently discarded instance settings")
	}
}

func TestRootRemovalUndoThroughNotification(t *testing.T) {
	a, tt, _ := worldsTester(t)
	id := a.root().ID
	a.removeRootUndo(id)
	waitUI(t, tt, func() bool { return len(a.snapshot.Instances) == 0 && a.noticeUndo != nil })
	tt.Advance(350 * time.Millisecond)
	clickUI(t, tt, "按钮文字 notice-undo")
	waitUI(t, tt, func() bool { return a.rootByID(id) != nil && a.noticeUndo == nil })
	if a.snapshot.ActiveRoot != id {
		t.Fatal("undo did not restore the active instance")
	}
}

func TestStopAndRollbackWaitForExplicitDecision(t *testing.T) {
	a, tt, _ := worldsTester(t)
	a.requestStop(a.root().ID, "", "world-one")
	tt.Frame()
	if a.confirmation == nil || a.pendingOp("stop") || !tt.HasText("world-one") {
		t.Fatal("stop was not guarded")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.confirmation != nil || a.pendingOp("stop") {
		t.Fatal("Escape performed a stop")
	}
	a.requestPackRollback(a.root().ID, "backup-id", "1.0")
	tt.Frame()
	if a.confirmation == nil || a.pendingOp("rollbackPack") || !tt.HasText(a.L("floating.rollback_hint")) {
		t.Fatal("rollback was not guarded")
	}
}

func TestPassiveTextTapAndActionableRow(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	activated := 0
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		ui.Column(c).Fill().Padding(40).Gap(40).Background(bg).Children(func() {
			b := ui.ButtonBase(c).Label("Open resource").Height(32)
			if b.Children(func() {
				e := ui.Text(c, "A very long resource title").Label("resource-title").Width(90).SingleLine()
				textTooltip(c, e, "A very long resource title", "", ui.Font{Family: theme.Font, Size: 14.5}, 1)
			}).Clicked() {
				activated++
			}
			e := ui.Text(c, "C:/Games/very/long/folder").Label("passive-path").Width(100).SingleLine()
			tapTooltip(c, e, "C:/Games/very/long/folder")
		})
	}, 900, 540)
	clickUI(t, tt, "resource-title")
	if activated != 1 {
		t.Fatal("text help intercepted the resource action")
	}
	clickUI(t, tt, "passive-path")
	tt.Frame()
	r, ok := tt.Find("文本提示")
	if !ok || r.W <= 0 || r.X < 0 || r.X+r.W > 900 || r.Y < 0 || r.Y+r.H > 540 {
		t.Fatal("tap tooltip missing or outside viewport", r)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if _, ok := tt.Find("文本提示"); ok {
		t.Fatal("Escape reopened the tooltip")
	}
}

func TestUndoIsKeyboardReachableInsideModal(t *testing.T) {
	a, tt, _ := worldsTester(t)
	waitUI(t, tt, func() bool { return len(a.pendingRequests) == 0 })
	server, err := a.service.SaveAuthServer("", "QA Server", "https://auth.example.test")
	if err != nil {
		t.Fatal(err)
	}
	a.snapshot = a.service.Snapshot()
	a.openDialog("accountAdd")
	a.accountTab = "third"
	a.form.Server = server.URL
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	if _, ok := tt.Find(a.tr().AccDeleteServerBtn); !ok {
		t.Fatalf("server picker not ready: tab=%s url=%s servers=%+v", a.accountTab, a.form.Server, a.snapshot.AuthServers)
	}
	clickUI(t, tt, a.tr().AccDeleteServerBtn)
	waitUI(t, tt, func() bool { return a.noticeUndo != nil })
	if _, ok := tt.Find("按钮文字 inline-notice-undo"); !ok {
		t.Fatal("undo escaped the modal scope")
	}
	found := false
	for i := 0; i < 10; i++ {
		tt.Key(0, ui.KeyTab)
		if tt.Focused(a.L("floating.undo")) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("keyboard cannot reach undo inside modal")
	}
	tt.Key(0, ui.KeyEnter)
	waitUI(t, tt, func() bool { return a.noticeUndo == nil })
	restored := false
	for _, s := range a.snapshot.AuthServers {
		if s.URL == server.URL && s.Name == server.Name {
			restored = true
		}
	}
	if !restored {
		t.Fatal("keyboard undo did not restore the server")
	}
}
