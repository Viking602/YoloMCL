package nativeui

import (
	"fmt"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type discoverReveal uint8

const (
	discoverLift discoverReveal = iota
	discoverExpand
)

type discoverCardMotion struct {
	mode         discoverReveal
	started      bool
	progress     float32
	delay        time.Duration
	index        int
	afterMove    time.Time
	iconURL      string
	iconReady    bool
	iconProgress float32
}

type discoverMotionState struct {
	query, request launcher.SearchQuery
	hasQuery       bool
	mode           discoverReveal
	cards          map[string]*discoverCardMotion
	cardProjects   []launcher.Project
	scroll         scrollTrack
	viewport       ui.Rect
}

func sameDiscoverQuery(a, b launcher.SearchQuery) bool {
	return a.Source == b.Source && a.Type == b.Type && a.Query == b.Query &&
		a.Minecraft == b.Minecraft && a.Loader == b.Loader && a.Sort == b.Sort &&
		slices.Equal(a.Categories, b.Categories)
}

// Called only for the latest successful request. In-flight and failed requests
// keep the displayed cards, their scroll position and their animation state.
func (a *App) acceptDiscoverResults(q launcher.SearchQuery, result launcher.SearchResult) {
	m := &a.discoverMotion
	m.mode = discoverLift
	if m.hasQuery && (m.query.Source != q.Source || m.query.Type != q.Type ||
		m.query.Minecraft != q.Minecraft || m.query.Loader != q.Loader ||
		m.query.Sort != q.Sort || !slices.Equal(m.query.Categories, q.Categories)) {
		m.mode = discoverExpand
	}
	m.scroll.st.ScrollTo(0, 0, ui.Animation{Duration: time.Nanosecond})
	m.query, m.hasQuery = q, true
	m.cardProjects = nil
	a.resetDiscoverFeed(q, &result)
	a.searchResult = result
}

func discoverProjectKey(p launcher.Project) string { return p.Source + "|" + p.ID }

func (a *App) syncDiscoverCards(c *ui.Context, projects []launcher.Project) {
	m := &a.discoverMotion
	if len(projects) == 0 {
		m.cards, m.cardProjects = nil, nil
		return
	}
	if len(projects) == len(m.cardProjects) && &projects[0] == &m.cardProjects[0] {
		return
	}
	current := make(map[string]*discoverCardMotion, len(projects))
	reordering := false
	for i, p := range projects {
		key := discoverProjectKey(p)
		state := m.cards[key]
		if state == nil {
			state = &discoverCardMotion{mode: m.mode}
		} else if state.index != i && state.started {
			reordering = true
		}
		state.index = i
		current[key] = state
	}
	if reordering {
		for _, state := range current {
			if !state.started {
				// Give surviving cards time to vacate their previous cells;
				// translucent newcomers must not overlap moving text.
				state.afterMove = c.Now().Add(motionDurMove)
			}
		}
	}
	m.cards = current
	m.cardProjects = projects
}

func discoverGridWidth(c *ui.Context) float32 {
	// Discover fills the window. Use the current size instead of last frame's
	// measured grid: a resize can otherwise briefly add columns, shrink the
	// virtual canvas and permanently clamp away the user's scroll position.
	w, _ := c.Size()
	return max(0, w-(instListWidth+2*pageGutter+10)*uiScale(c)-2)
}

func discoverColumns(c *ui.Context) int {
	s := uiScale(c)
	w := discoverGridWidth(c)
	return max(1, int((w+14*s)/(300*s+14*s)))
}

func (a *App) animateDiscoverCard(c *ui.Context, card ui.Element, state *discoverCardMotion, index, columns int) {
	s := uiScale(c)
	// Stable grid identities let surviving cards move to their new cells without
	// being faded out or replaying their entrance when filters or sorting change.
	card.Transition(ui.ElementTransition{Animation: &motionMove, Position: true})
	if c.Preferences().ReduceMotion {
		state.started, state.progress = true, card.Jump("reveal", 1)
		return
	}
	if !state.started {
		bounds := card.LayoutBounds()
		_, y := card.LocalToWindow(0, 0)
		view := a.discoverMotion.viewport
		if bounds.W <= 0 || view.H <= 0 {
			card.Opacity(0)
			c.AnimationFrame()
			return
		}
		if y+bounds.H <= view.Y || y >= view.Y+view.H {
			card.Opacity(0)
			return
		}
		state.started = true
		row := max(0, int((y-view.Y)/(bounds.H+14*s)))
		state.delay = max(0, state.afterMove.Sub(c.Now())) + time.Duration(min(150, row*38+(index%columns)*26))*time.Millisecond
		card.Jump("reveal", 0)
	}
	p := card.AnimateTo("reveal", 1, ui.Animation{Duration: 420 * time.Millisecond, Delay: state.delay, Ease: ui.CubicBezier(.22, 1, .36, 1)})
	state.progress = p
	card.Opacity(min(1, p*1.3))
	switch state.mode {
	case discoverExpand:
		scale := .92 + .08*p
		card.Scale(scale, scale)
	default:
		card.Translate(0, 26*s*(1-p))
	}
}

// Icons can arrive well after text. Reveal just the image from left to right
// over the fallback tile; neither the title nor the card replays its entrance.
func (a *App) discoverIcon(c *ui.Context, p launcher.Project, state *discoverCardMotion) {
	size := 44 * uiScale(c)
	img := a.remoteImage(p.Icon)
	box := ui.Box(c.Key("discover-icon")).Label("项目封面 "+p.Title).Size(size, size).Radius(size * .22).Clip().Shrink(0)
	if state.iconURL != p.Icon {
		state.iconURL, state.iconReady = p.Icon, false
		box.Jump("image-reveal", 0)
	}
	progress := float32(0)
	if img != nil {
		if !state.iconReady {
			state.iconReady = true
			start := float32(0)
			if state.progress < .5 { // cached images enter with their card
				start = 1
			}
			box.Jump("image-reveal", start)
		}
		progress = box.AnimateTo("image-reveal", 1, ui.Animation{Duration: 300 * time.Millisecond, Ease: easeStandard})
		if c.Preferences().ReduceMotion {
			progress = box.Jump("image-reveal", 1)
		}
	}
	state.iconProgress = progress
	box.Children(func() {
		if progress < 1 {
			ui.Box(c.Key("icon-placeholder")).Size(size, size).Opacity(1 - progress).Children(func() { letterTile(c, p.Title, size, false) })
		}
		if img != nil {
			ui.Box(c.Key("icon-reveal")).Absolute().Left(0).Top(0).Width(size * progress).Height(size).Clip().Children(func() {
				// The reveal adds a rectangular clip inside the tile. Round the
				// image itself too: stationary GPU clips only shape the innermost
				// clip, so the outer tile alone loses its corners after motion.
				ui.Image(c, img).Absolute().Left(0).Top(0).Size(size, size).Fit(ui.Contain).Radius(size * .22)
			})
		}
	})
}

func (a *App) discoverSkeleton(c *ui.Context) {
	s := uiScale(c)
	grid := ui.Grid(c.Key("discover-skeleton")).Label(a.tr().DiscoverLoading).FillWidth().Gap(14 * s)
	cols := discoverColumns(c)
	grid.Columns(cols).Children(func() {
		for i := range cols * 3 {
			ui.Column(c.Key(fmt.Sprintf("search-placeholder-%d", i))).Height(194*s).MinWidth(0).Padding(16*s).Gap(16*s).Radius(radiusCard*s).Background(panel).Border(1, line.Alpha(.5)).Children(func() {
				ui.Row(c).FillWidth().Gap(12 * s).Children(func() {
					ui.Box(c).Size(44*s, 44*s).Radius(10 * s).Background(fieldColor.Alpha(.6))
					ui.Column(c).Grow(1).MinWidth(0).Gap(9 * s).PaddingY(5 * s).Children(func() {
						ui.Box(c).FillWidth().Height(12 * s).Radius(4 * s).Background(fieldColor.Alpha(.55))
						ui.Box(c).Width(82 * s).Height(9 * s).Radius(4 * s).Background(fieldColor.Alpha(.35))
					})
				})
				ui.Box(c).FillWidth().Height(9 * s).Radius(4 * s).Background(fieldColor.Alpha(.4))
				ui.Box(c).Width(140 * s).Height(9 * s).Radius(4 * s).Background(fieldColor.Alpha(.35))
				ui.Spacer(c).Grow(1)
				ui.Row(c).FillWidth().Justify(ui.SpaceBetween).Children(func() {
					ui.Box(c).Width(68 * s).Height(12 * s).Radius(4 * s).Background(fieldColor.Alpha(.4))
					ui.Box(c).Width(72 * s).Height(28 * s).Radius(8 * s).Background(accentSoft.Alpha(.7))
				})
			})
		}
	})
}
