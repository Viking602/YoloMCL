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
	cached, haveMods := a.addonCache[key+"|mods"]
	loader := findShaderLoader(cached.mods)
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
		if tab == "shader" && !haveMods {
			mods, err := a.service.ListModsStreamContext(ctx, id, ver, nil)
			if err == nil {
				loader, haveMods = findShaderLoader(mods), true
			}
		}
		a.post(func() {
			if ctx.Err() != nil {
				return
			}
			if a.gameStates == nil {
				a.gameStates = map[string]launcher.GameOptions{}
			}
			a.gameStates[key] = st
			if tab == "shader" && haveMods {
				if a.shaderLoaders == nil {
					a.shaderLoaders = map[string]shaderLoaderInfo{}
				}
				a.shaderLoaders[key] = loader
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
	if c, hit := a.addonCache[key+"|mods"]; hit {
		info := findShaderLoader(c.mods)
		return info.name, info.version, true
	}
	info, ok := a.shaderLoaders[key]
	return info.name, info.version, ok
}

// Shader status needs two strings, not another retained copy of every mod
// in every instance whose shaders have ever been opened.
type shaderLoaderInfo struct{ name, version string }

func findShaderLoader(mods []launcher.Mod) shaderLoaderInfo {
	for _, m := range mods {
		if !m.Enabled {
			continue
		}
		id, f := strings.ToLower(m.ModID), strings.ToLower(m.File)
		switch {
		case id == "iris" || strings.HasPrefix(f, "iris-") || strings.HasPrefix(f, "iris_"):
			return shaderLoaderInfo{"Iris", m.Version}
		case id == "oculus" || strings.HasPrefix(f, "oculus"):
			return shaderLoaderInfo{"Oculus", m.Version}
		case id == "optifine" || strings.HasPrefix(f, "optifine"):
			return shaderLoaderInfo{"OptiFine", m.Version}
		}
	}
	return shaderLoaderInfo{}
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
