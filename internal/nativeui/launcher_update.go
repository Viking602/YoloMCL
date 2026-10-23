package nativeui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"github.com/Viking602/YoloMCL/internal/buildinfo"
)

type launcherRelease struct {
	version string
	install func(context.Context, func(int64, int64)) error
}

type launcherUpdateState struct {
	phase             string // idle, checking, current, available, installing, ready, disabled, failed
	release           *launcherRelease
	downloaded, total int64
	err               error
}

// Only official archive layouts are compatible with the updater's relaunch
// path. A renamed binary can still use the GitHub download link.
func officialExecutable(path, goos string) bool {
	if goos == "windows" {
		return strings.EqualFold(filepath.Base(path), "YoloMCL.exe")
	}
	if filepath.Base(path) != "YoloMCL" {
		return false
	}
	if goos == "darwin" {
		return filepath.Base(filepath.Dir(path)) == "MacOS" &&
			filepath.Base(filepath.Dir(filepath.Dir(path))) == "Contents" &&
			strings.HasSuffix(filepath.Dir(filepath.Dir(filepath.Dir(path))), ".app")
	}
	return goos == "linux"
}

func checkOfficialRelease(ctx context.Context) (*launcherRelease, error) {
	path, err := os.Executable()
	if err != nil {
		return nil, err
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	if !buildinfo.ValidTag(buildinfo.Current().Version) || !officialExecutable(path, runtime.GOOS) {
		return nil, mygo.ErrUpdatesDisabled
	}
	update, err := mygo.Updater.Check(ctx)
	if err != nil || update == nil {
		return nil, err
	}
	if !buildinfo.ValidTag("v" + strings.TrimPrefix(update.Version, "v")) {
		return nil, errors.New("invalid release version")
	}
	return &launcherRelease{version: update.Version, install: update.Install}, nil
}

func (a *App) startLauncherUpdates() {
	a.launcherUpdateOnce.Do(a.checkLauncherUpdates)
}

func (a *App) checkLauncherUpdates() {
	state := &a.launcherUpdate
	if state.phase == "checking" || state.phase == "installing" || state.phase == "ready" {
		return
	}
	state.phase, state.err = "checking", nil
	check := a.checkLauncherRelease
	if check == nil {
		check = checkOfficialRelease
	}
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		if !waitForFirstFrame(a.ctx, a.firstFrame) {
			return
		}
		ctx, cancel := context.WithTimeout(a.ctx, 25*time.Second)
		defer cancel()
		release, err := check(ctx)
		a.post(func() {
			state.release, state.err = release, err
			switch {
			case errors.Is(err, mygo.ErrUpdatesDisabled):
				state.phase = "disabled"
			case err != nil:
				state.phase = "failed"
			case release != nil:
				state.phase = "available"
			default:
				state.phase = "current"
			}
		})
	}()
}

func (a *App) launcherRestartBlocked() bool {
	for _, instance := range a.snapshot.Instances {
		if instance.Running {
			return true
		}
	}
	for _, job := range a.snapshot.Jobs {
		if job.State == "running" || job.State == "queued" {
			return true
		}
	}
	for _, count := range a.pending {
		if count > 0 {
			return true
		}
	}
	return a.accountAdding || a.javaScanning
}

func (a *App) installLauncherUpdate() {
	state := &a.launcherUpdate
	if state.phase != "available" || state.release == nil || state.release.install == nil {
		return
	}
	state.phase, state.err = "installing", nil
	state.downloaded, state.total = 0, 0
	install := state.release.install
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		ctx, cancel := context.WithTimeout(a.ctx, 15*time.Minute)
		defer cancel()
		var last time.Time
		err := install(ctx, func(done, total int64) {
			if done != total && time.Since(last) < 100*time.Millisecond {
				return
			}
			last = time.Now()
			a.post(func() { state.downloaded, state.total = done, total })
		})
		a.post(func() {
			state.err = err
			if err != nil {
				state.phase = "available" // keep the verified candidate available for retry
			} else {
				state.phase = "ready"
			}
		})
	}()
}
