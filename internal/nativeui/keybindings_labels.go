package nativeui

import (
	"context"
	"maps"
	"slices"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// Labels never replace editor values: a scan may finish after an edit or save.
func mergeBindingLabels(view *launcher.KeyBindingsView, labels launcher.KeyBindingsView) {
	byID := make(map[string]launcher.KeyBindingRow, len(labels.Rows))
	for _, row := range labels.Rows {
		byID[row.ID] = row
	}
	for i := range view.Rows {
		row := &view.Rows[i]
		label := byID[row.ID]
		row.Names, row.Namespace, row.ModName = maps.Clone(label.Names), label.Namespace, label.ModName
	}
	view.LabelsIncomplete = labels.LabelsIncomplete
}

func (a *App) cancelKeyBindingLabels() {
	if g := a.keyBindings; g != nil {
		g.labelsGen++
		if g.labelsCancel != nil {
			g.labelsCancel()
			g.labelsCancel = nil
		}
		g.labelsLoading = false
		g.pendingLabels = nil
	}
}

func (a *App) loadKeyBindingLabels(g *keyBindingsState) {
	if g.rootID == "" || !slices.ContainsFunc(g.view.Rows, func(row launcher.KeyBindingRow) bool { return !row.Vanilla }) {
		return
	}
	ctx, cancel := context.WithCancel(a.ctx)
	g.labelsCancel, g.labelsLoading = cancel, true
	gen, rootID, version := g.labelsGen, g.rootID, g.version
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		defer cancel()
		view, err := a.service.ReadKeyBindings(ctx, rootID, version)
		if ctx.Err() != nil {
			return
		}
		a.post(func() {
			if a.keyBindings != g || g.labelsGen != gen || a.dialog != "keyBindings" {
				return
			}
			g.labelsCancel, g.labelsLoading = nil, false
			if err != nil {
				g.view.LabelsIncomplete = true
				return
			}
			g.pendingLabels = &view
			a.applyKeyBindingLabels(g)
		})
	}()
}

func (a *App) applyKeyBindingLabels(g *keyBindingsState) {
	// Regrouping reparents controls. Keep the focused recorder/manual editor
	// in place until input is finished so its keystrokes cannot be lost.
	if g.pendingLabels == nil || g.capture != "" || g.manual != "" {
		return
	}
	mergeBindingLabels(g.view, *g.pendingLabels)
	g.pendingLabels = nil
	ids, _ := a.bindingGroups(g)
	if !slices.Contains(ids, g.mod) {
		g.mod = "all"
	}
}
