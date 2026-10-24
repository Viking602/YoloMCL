package nativeui

import (
	"time"

	"github.com/egoist/mygo/ui"
)

type discoverCategoryMotion struct {
	key          string
	entered      time.Time
	contentReady bool
	started      bool
	progress     float32
}

// Run after the results render, so readiness follows the visible cards' actual
// entrance, not a timeout that might expire while the request is still loading.
func (a *App) settleDiscoverCategories(c *ui.Context) {
	m := &a.discoverMotion
	cats := &m.categories
	if cats.key != a.catKey() || cats.contentReady || a.searchLoading {
		return
	}
	if a.searchError == "" {
		if !m.hasQuery || m.query.Source != a.provider || m.query.Type != normalizeResourceType(a.resourceType) {
			return
		}
		if len(a.searchResult.Projects) > 0 && (m.cardProjects == nil || m.revealing) {
			return
		}
	}
	// Cached results can already be settled when the Discover page itself is
	// still entering. Reduced motion keeps the data gate but skips this wait.
	if !c.Preferences().ReduceMotion {
		if remaining := cats.entered.Add(motionDurEnter).Sub(c.Now()); remaining > 0 {
			c.After(remaining)
			return
		}
	}
	cats.contentReady = true
	c.AnimationFrame()
}

func (a *App) discoverCategoryBody(c *ui.Context) ui.Element {
	s := uiScale(c)
	m := &a.discoverMotion.categories
	body := ui.Column(c.Key("discover-categories")).Label("发现分类").FillWidth().Gap(8 * s)
	if !m.contentReady || !m.started {
		body.Jump("reveal", 0)
	}
	m.progress = 0
	if m.contentReady {
		m.started = true
		m.progress = body.AnimateTo("reveal", 1, ui.Animation{Duration: 260 * time.Millisecond, Ease: easeOutCubic})
		if c.Preferences().ReduceMotion {
			m.progress = body.Jump("reveal", 1)
		}
	}
	// Keep the slot's height stable and glyphs at native resolution. Invisible
	// filters cannot take mouse or keyboard focus while the content is loading.
	y := (1 - m.progress) * 8 * s
	return body.Opacity(m.progress).Margin(y, 0, -y, 0).Disabled(m.progress == 0)
}
