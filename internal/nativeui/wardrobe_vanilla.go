package nativeui

// offlineDefault is true when an offline account has no skin of its own.
func (a *App) offlineDefault() bool {
	ac := a.account()
	return ac != nil && ac.Kind == "offline" && a.appearance.Skin == "" && len(a.wd().pendingSkin) == 0
}

// Both official textures are bundled; changing the model never waits on I/O.
func (a *App) showVanilla(model string) {
	a.preview.slim = model == "slim"
	a.preview.skin = builtinSkin(a.preview.slim)
	a.preview.dirty = true
}
