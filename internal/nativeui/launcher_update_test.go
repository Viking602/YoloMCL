package nativeui

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/buildinfo"
	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestLauncherUpdateWaitsForFirstFrameAndChecksOnce(t *testing.T) {
	a, tt := settingsTester(t)
	a.firstFrame = make(chan struct{})
	var calls atomic.Int32
	a.checkLauncherRelease = func(ctx context.Context) (launcherUpdateResult, error) {
		calls.Add(1)
		return launcherUpdateResult{release: &launcherRelease{version: "1.1.0"}}, nil
	}
	a.startLauncherUpdates()
	a.startLauncherUpdates()
	time.Sleep(20 * time.Millisecond)
	if calls.Load() != 0 {
		t.Fatal("update check competed with the first frame")
	}
	close(a.firstFrame)
	waitUI(t, tt, func() bool { return a.launcherUpdate.phase == "available" })
	a.startLauncherUpdates()
	if calls.Load() != 1 {
		t.Fatal("automatic update check ran more than once")
	}
	a.bgWG.Wait()
}

func TestLauncherUpdateRetryAndRestartSafety(t *testing.T) {
	a, tt := settingsTester(t)
	attempt := 0
	a.launcherUpdate = launcherUpdateState{phase: "available", release: &launcherRelease{
		version: "1.1.0", install: func(_ context.Context, progress func(int64, int64)) error {
			attempt++
			progress(20, 100)
			if attempt == 1 {
				return errors.New("signature verification failed")
			}
			progress(100, 100)
			return nil
		},
	}}
	a.installLauncherUpdate()
	a.installLauncherUpdate() // double click must not start another installer
	waitUI(t, tt, func() bool { return a.launcherUpdate.err != nil })
	if attempt != 1 || a.launcherUpdate.phase != "available" {
		t.Fatal("failed update could not be retried")
	}
	a.installLauncherUpdate()
	waitUI(t, tt, func() bool { return a.launcherUpdate.phase == "ready" })
	a.snapshot.Instances = []launcher.Instance{{Running: true}}
	if !a.launcherRestartBlocked() {
		t.Fatal("restart allowed while a game is running")
	}
	a.snapshot.Instances = nil
	a.snapshot.Jobs = []launcher.Job{{State: "running"}}
	if !a.launcherRestartBlocked() {
		t.Fatal("restart allowed during an installation")
	}
	a.snapshot.Jobs = nil
	if a.launcherRestartBlocked() || attempt != 2 {
		t.Fatal("idle launcher cannot restart after a verified update")
	}
	a.bgWG.Wait()
}

func TestOfficialUpdateExecutableLayout(t *testing.T) {
	for _, sample := range []struct {
		path, goos string
		valid      bool
	}{
		{"YoloMCL.exe", "windows", true}, {"renamed.exe", "windows", false},
		{"YoloMCL", "linux", true}, {"other", "linux", false},
		{filepath.Join("YoloMCL.app", "Contents", "MacOS", "YoloMCL"), "darwin", true},
		{"YoloMCL", "darwin", false},
	} {
		if officialExecutable(sample.path, sample.goos) != sample.valid {
			t.Errorf("unexpected update support: %+v", sample)
		}
	}
}

func TestAboutUpdateStatesFitBothLanguages(t *testing.T) {
	a, _ := settingsTester(t)
	for _, lang := range []string{LangZH, LangEN} {
		a.OverrideLanguage(lang)
		for _, phase := range []string{"checking", "current", "available", "installing", "ready", "disabled", "failed"} {
			a.launcherUpdate = launcherUpdateState{phase: phase, release: &launcherRelease{version: "1.1.0"}, total: 100, downloaded: 42}
			tt := ui.NewTester(func(c *ui.Context) { a.aboutSettings(c) }, 600, 300)
			for _, name := range []string{"关于 YoloMCL 面板", "launcher-version", "launcher-update-status"} {
				r, ok := tt.Find(name)
				if !ok || r.W <= 0 || r.X < 0 || r.X+r.W > 600 || r.Y+r.H > 300 {
					t.Errorf("%s/%s: %s overflowed: %+v", lang, phase, name, r)
				}
			}
			if phase == "available" {
				snapshotUI(t, tt, "launcher-about-"+lang)
			}
		}
	}
}

func TestLauncherUpdateCancelOnlyDefersThisLaunch(t *testing.T) {
	for range 2 { // a fresh window must offer the same available release again
		a, tt := settingsTester(t)
		var checks, installs atomic.Int32
		a.launcherUpdate.currentVersion = "v1.0.0"
		a.checkLauncherRelease = func(context.Context) (launcherUpdateResult, error) {
			checks.Add(1)
			return launcherUpdateResult{release: &launcherRelease{version: "1.1.0", install: func(context.Context, func(int64, int64)) error {
				installs.Add(1)
				return nil
			}}}, nil
		}
		a.startLauncherUpdates()
		waitUI(t, tt, func() bool { return a.dialogOpen && a.dialog == "launcherUpdate" })
		tt.Advance(300 * time.Millisecond)
		if installs.Load() != 0 {
			t.Fatal("startup installed without confirmation")
		}
		snapshotUI(t, tt, "launcher-optional-update")
		clickUI(t, tt, a.L("取消", "Cancel"))
		tt.Advance(500 * time.Millisecond)
		a.startLauncherUpdates()
		tt.Frame()
		if a.dialogOpen || a.launcherUpdate.prompt || checks.Load() != 1 || installs.Load() != 0 {
			t.Fatal("cancel did not defer the update for the rest of this launch")
		}
		a.openDialog("launcherUpdate")
		tt.Advance(300 * time.Millisecond)
		clickUI(t, tt, a.L("确认更新", "Update now"))
		waitUI(t, tt, func() bool { return a.launcherUpdate.phase == "ready" })
		if installs.Load() != 1 {
			t.Fatal("confirmation did not install exactly once")
		}
		a.bgWG.Wait()
	}
}

func TestLauncherUpdatePromptWaitsForActiveDialog(t *testing.T) {
	a, tt := settingsTester(t)
	a.openDialog("add")
	a.form.Name = "unsaved name"
	a.checkLauncherRelease = func(context.Context) (launcherUpdateResult, error) {
		return launcherUpdateResult{release: &launcherRelease{version: "1.1.0"}}, nil
	}
	a.startLauncherUpdates()
	waitUI(t, tt, func() bool { return a.launcherUpdate.phase == "available" })
	if a.dialog != "add" || a.form.Name != "unsaved name" {
		t.Fatal("optional update interrupted an active form")
	}
	a.closeDialog()
	tt.Advance(500 * time.Millisecond)
	tt.Frame()
	if !a.dialogOpen || a.dialog != "launcherUpdate" {
		t.Fatal("queued update was not shown after the form closed")
	}
	a.bgWG.Wait()
}

func TestRequiredLauncherUpdateBlocksDismissalAndSurvivesFailures(t *testing.T) {
	for _, lang := range []string{LangZH, LangEN} {
		t.Run(lang, func(t *testing.T) {
			a, tt := settingsTester(t)
			a.OverrideLanguage(lang)
			a.launcherUpdate = launcherUpdateState{
				currentVersion: "v1.0.0", phase: "available",
				policy: &buildinfo.UpdatePolicy{Version: "1.1.0", MinimumVersion: "1.1.0"},
				release: &launcherRelease{version: "1.1.0", install: func(context.Context, func(int64, int64)) error {
					return errors.New("simulated installation failure")
				}},
			}
			tt.SetSize(900, 540)
			tt.Advance(300 * time.Millisecond)
			assertLocked := func() {
				t.Helper()
				if !a.launcherUpdateRequired() || a.dialog != "launcherUpdate" || !a.dialogOpen || !tt.HasText("启动器更新限制") || tt.HasText("主导航") || tt.HasText(a.L("取消", "Cancel")) {
					t.Fatal("retired launcher exposed its normal interface or a dismissal")
				}
				for _, name := range []string{a.L("退出启动器", "Exit launcher"), "活动弹窗"} {
					r, ok := tt.Find(name)
					if !ok || r.W <= 0 || r.H <= 0 || r.X < 0 || r.Y < 0 || r.X+r.W > 900 || r.Y+r.H > 540 {
						t.Fatalf("mandatory update control is inaccessible: %s %+v", name, r)
					}
				}
			}
			assertLocked()
			snapshotUI(t, tt, "launcher-required-update-"+lang)
			tt.Key(0, ui.KeyEscape)
			tt.ClickAt(5, 100)
			a.closeDialog()
			a.openDialog("install")
			a.goPage("home")
			tab := a.instTab
			a.ShowPage("home/mods")
			if a.instTab != tab {
				t.Fatal("startup page override bypassed the update gate")
			}
			a.execute("launch", nil, nil)
			tt.Frame()
			assertLocked()
			if len(a.pending) != 0 {
				t.Fatal("retired launcher dispatched a new game operation")
			}
			// A failed re-check cannot erase an authenticated cached retirement.
			a.checkLauncherRelease = func(context.Context) (launcherUpdateResult, error) {
				return launcherUpdateResult{}, errors.New("offline")
			}
			a.checkLauncherUpdates()
			waitUI(t, tt, func() bool { return a.launcherUpdate.phase == "failed" })
			assertLocked()
			a.launcherUpdate.phase, a.launcherUpdate.err = "available", nil
			tt.Frame()
			clickUI(t, tt, a.L("确认更新", "Update now"))
			waitUI(t, tt, func() bool { return a.launcherUpdate.phase == "available" && a.launcherUpdate.err != nil })
			assertLocked()
			a.launcherUpdate.release.install = func(context.Context, func(int64, int64)) error { return nil }
			clickUI(t, tt, a.L("确认更新", "Update now"))
			waitUI(t, tt, func() bool { return a.launcherUpdate.phase == "ready" })
			assertLocked() // replacing the file does not change the running old code
			a.bgWG.Wait()
		})
	}
}
