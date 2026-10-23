package nativeui

import (
	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/locales/zh-Hans"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// BindSystemLanguage must run in MyGo's WhenReady callback, before the first
// window is created. Reading the native locale earlier can block waiting for
// the event loop, especially in offscreen renders and UI tests.
func (a *App) BindSystemLanguage() {
	// MyGo reads Windows display languages, macOS preferred languages and
	// Linux LANGUAGE / LC_ALL / LC_MESSAGES / LANG through its OS backends.
	a.systemLanguages = mygo.App.PreferredLanguages
	a.refreshSystemLanguage()
}

func (a *App) refreshSystemLanguage() {
	if a.systemLanguages == nil {
		return
	}
	previous := a.lang()
	// PreferredLanguages returns a cached snapshot updated by native locale
	// notifications. This performs no OS calls or directory scans per frame,
	// and notices changes even when MyGo's own fallback locale is unchanged.
	a.systemLanguage = launcher.SystemLanguage(a.systemLanguages())
	if previous != a.lang() {
		a.normalizeStateValues()
	}
}
