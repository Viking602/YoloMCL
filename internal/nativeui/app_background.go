package nativeui

import (
	"time"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

const activeSnapshotInterval = 450 * time.Millisecond

func signalWorker(wake chan struct{}) {
	select {
	case wake <- struct{}{}:
	default:
	}
}

func (a *App) wakeSnapshots() { signalWorker(a.pollWake) }

func snapshotInterval(background bool, jobs []launcher.Job) time.Duration {
	if background {
		return 5 * time.Second
	}
	for _, job := range jobs {
		if job.State == "running" || job.State == "queued" {
			return activeSnapshotInterval
		}
	}
	return 2 * time.Second
}

// UI actions and completed workers wake polling early. Coalesce those signals
// so bursts of image/dependency results never turn into a busy snapshot loop.
func (a *App) pollSnapshots(systemLanguages func() []string) {
	timer := time.NewTimer(activeSnapshotInterval)
	defer timer.Stop()
	var observed snapshotObserver
	var memory idleMemory
	var language string
	var last time.Time
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-timer.C:
		case <-a.pollWake:
		}
		if wait := activeSnapshotInterval - time.Since(last); wait > 0 {
			timer.Reset(wait)
			continue
		}
		epoch := a.selectionEpoch.Load()
		snap := a.service.Snapshot()
		last = time.Now()
		timer.Reset(snapshotInterval(a.background.Load(), snap.Jobs))
		changed := observed.changed(snap, epoch)
		if systemLanguages != nil {
			next := launcher.SystemLanguage(systemLanguages())
			changed = changed || next != language
			language = next
		}
		if !changed {
			a.imageMu.Lock()
			artworkBusy := len(a.imagePending) != 0
			a.imageMu.Unlock()
			memory.reclaim(a.frameSerial.Load(), snap.Jobs, artworkBusy, a.cacheRelease.Load())
			continue
		}
		a.post(func() {
			old := jobSignature(a.snapshot.Jobs)
			a.applyJobNotice(a.snapshot.Jobs, snap.Jobs)
			a.applySnapshot(snap, epoch)
			if old != jobSignature(snap.Jobs) && a.page == "mods" {
				a.loadMods()
			}
		})
	}
}

func (a *App) refreshInventories() {
	defer a.bgWG.Done()
	monitor := a.service.NewInventoryMonitor()
	defer monitor.Close()
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-timer.C:
		case <-a.inventoryWake:
		}
		monitor.Refresh(a.ctx)
		delay := 5 * time.Second
		if a.background.Load() {
			delay = 30 * time.Second
		}
		timer.Reset(delay)
	}
}
