package nativeui

import (
	"context"
	"strings"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// refreshGameState rereads the game's own files in the background; the
// previous value stays on screen until the new one lands (no flash).
func (a *App) refreshGameState() {
	a.refreshGameStateContext(a.ctx)
}

func (a *App) refreshGameStateContext(ctx context.Context) {
	r := a.root()
	if r == nil || a.service == nil {
		return
	}
	key := r.ID + "|" + r.SelectedVersion
	id, ver, tab := r.ID, r.SelectedVersion, a.addonTab
	ready := a.firstFrame
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		if !waitForFirstFrame(ctx, ready) {
			return
		}
		st, e := a.service.ReadGameOptions(ctx, id, ver)
		if e != nil {
			if ctx.Err() == nil {
				logUIError("game options: " + e.Error())
			}
			return
		}
		var mods []launcher.Mod
		if tab == "shader" {
			mods, _ = a.service.ListModsStreamContext(ctx, id, ver, nil)
		}
		a.post(func() {
			if ctx.Err() != nil {
				return
			}
			if a.gameStates == nil {
				a.gameStates = map[string]launcher.GameOptions{}
				a.shaderMods = map[string][]launcher.Mod{}
			}
			a.gameStates[key] = st
			if mods != nil {
				a.shaderMods[key] = mods
			}
		})
	}()
}

func (a *App) gameState() (launcher.GameOptions, bool) {
	r := a.root()
	if r == nil {
		return launcher.GameOptions{}, false
	}
	st, ok := a.gameStates[r.ID+"|"+r.SelectedVersion]
	return st, ok
}

// shaderLoader finds Iris / Oculus / OptiFine among the instance's mods.
func (a *App) shaderLoader() (name, version string, known bool) {
	r := a.root()
	if r == nil {
		return "", "", false
	}
	key := r.ID + "|" + r.SelectedVersion
	mods, ok := a.shaderMods[key]
	if c, hit := a.addonCache[key+"|mods"]; hit {
		mods, ok = c.mods, true
	}
	if !ok {
		return "", "", false
	}
	for _, m := range mods {
		if !m.Enabled {
			continue
		}
		id, f := strings.ToLower(m.ModID), strings.ToLower(m.File)
		switch {
		case id == "iris" || strings.HasPrefix(f, "iris-") || strings.HasPrefix(f, "iris_"):
			return "Iris", m.Version, true
		case id == "oculus" || strings.HasPrefix(f, "oculus"):
			return "Oculus", m.Version, true
		case id == "optifine" || strings.HasPrefix(f, "optifine"):
			return "OptiFine", m.Version, true
		}
	}
	return "", "", true
}

// shaderNeed is the shader loader that fits the instance's mod loader.
func shaderNeed(loader string) (label, query string) {
	switch strings.ToLower(loader) {
	case "fabric", "quilt", "neoforge":
		return "Iris", "iris"
	case "forge":
		return "Oculus", "oculus"
	}
	return "OptiFine", ""
}
