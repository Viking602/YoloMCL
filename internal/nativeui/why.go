package nativeui

// whyOf takes (condition, reason) pairs and returns the first reason whose
// condition holds, or "" when none does: the reason a control is disabled.
func whyOf(pairs ...any) string {
	for i := 0; i+1 < len(pairs); i += 2 {
		if on, _ := pairs[i].(bool); on {
			if s, _ := pairs[i+1].(string); s != "" {
				return s
			}
		}
	}
	return ""
}

func (a *App) packPreviewUnavailable() string {
	if a.packPreview == nil {
		return ""
	}
	return a.packPreview.Unavailable
}

// skinWhy explains a disabled skin/cape action on the wardrobe page.
func (a *App) skinWhy() string {
	t := a.tr()
	if a.appearanceErr != "" {
		return t.WhySkinLoadFailed
	}
	return t.WhySkinUnsupported
}
