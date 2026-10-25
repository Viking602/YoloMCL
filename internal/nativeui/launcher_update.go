package nativeui

import (
	"context"
	"errors"
	"time"

	"github.com/egoist/mygo"

	"github.com/Viking602/YoloMCL/internal/buildinfo"
)

type launcherRelease struct {
	version string
	install func(context.Context, func(int64, int64)) error
}

type launcherUpdateResult struct {
	release *launcherRelease
	policy  *buildinfo.UpdatePolicy
}

type launcherUpdateState struct {
	phase             string // idle, checking, current, available, installing, ready, disabled, failed
	release           *launcherRelease
	downloaded, total int64
	err               error
	currentVersion    string
	policy            *buildinfo.UpdatePolicy
	prompt, locked    bool
}

func (a *App) launcherUpdateRequired() bool {
	return a.launcherUpdate.policy.RequiresUpdate(a.launcherUpdate.currentVersion)
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
		version, cached, dir := state.currentVersion, state.policy, a.snapshot.DataDir
		check = func(ctx context.Context) (launcherUpdateResult, error) {
			return checkOfficialRelease(ctx, version, dir, cached)
		}
	}
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		if !waitForFirstFrame(a.ctx, a.firstFrame) {
			return
		}
		ctx, cancel := context.WithTimeout(a.ctx, 25*time.Second)
		defer cancel()
		result, err := check(ctx)
		a.post(func() {
			state.policy = newerLauncherPolicy(state.policy, result.policy)
			state.err = err
			switch {
			case result.release != nil:
				state.release = result.release
				state.phase, state.prompt = "available", true
			case errors.Is(err, mygo.ErrUpdatesDisabled):
				state.phase = "disabled"
			case err != nil:
				state.phase = "failed"
			default:
				state.phase, state.release = "current", nil
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
	return (a.dialogOpen && a.dialog == "accountAdd") || a.javaScanning
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
