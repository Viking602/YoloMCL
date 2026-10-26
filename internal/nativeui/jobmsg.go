package nativeui

import "github.com/Viking602/YoloMCL/internal/i18n"

// jobMsg renders a job message or item line in the UI language.
func (a *App) jobMsg(message string) string {
	return i18n.JobMessage(a.isEN(), message)
}
