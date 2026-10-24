package nativeui

import (
	"strconv"
	"strings"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// persistSettings writes the current draft immediately. A blank notice keeps
// the header hint; a validation failure shows the error and saves nothing.
func (a *App) persistSettings(notice string) {
	value, e := strconv.Atoi(strings.TrimSpace(a.memoryText))
	if e != nil || value < 512 || value > int(a.memoryMaximum()) {
		a.message = a.formatMemoryRangeErr(int(a.memoryMaximum()))
		a.errorMessage = true
		return
	}
	a.memory = float64(value)
	a.settings.MemoryMB = value
	a.saveSettingsDraft(notice)
}

// All settings edits, including language switches, share one ordered writer.
// Store the selection mode, never the language resolved from the current OS.
func (a *App) saveSettingsDraft(notice string) {
	a.settings.Language = launcher.NormalizeLanguage(a.settings.Language)
	if a.service == nil {
		return
	}
	draft := a.settings
	epoch := a.selectionEpoch.Load()
	revision := a.settingsRevision.Add(1)
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		// Slider edits and switches may submit several drafts before the
		// disk catches up. Persist only in order, never an older draft last.
		a.settingsMu.Lock()
		if revision != a.settingsRevision.Load() {
			a.settingsMu.Unlock()
			return
		}
		e := a.service.SaveSettings(draft)
		a.settingsMu.Unlock()
		snap := a.service.Snapshot()
		a.post(func() {
			if revision != a.settingsRevision.Load() {
				return
			}
			a.applySnapshot(snap, epoch)
			if e != nil {
				a.message = a.friendlyErr(e)
				a.errorMessage = true
				return
			}
			a.settings = snap.Settings
			a.notice(notice)
		})
	}()
}
