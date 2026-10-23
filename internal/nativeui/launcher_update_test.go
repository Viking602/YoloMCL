package nativeui

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestLauncherUpdateWaitsForFirstFrameAndChecksOnce(t *testing.T) {
	a, tt := settingsTester(t)
	a.firstFrame = make(chan struct{})
	var calls atomic.Int32
	a.checkLauncherRelease = func(ctx context.Context) (*launcherRelease, error) {
		calls.Add(1)
		return &launcherRelease{version: "1.1.0"}, nil
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
