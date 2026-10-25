package nativeui

import (
	"context"
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func (a *App) search() { a.searchAt(0) }

func (a *App) searchAt(offset int) {
	a.startDiscoverRequest(a.discoverQuery(offset), false)
}

func (a *App) discoverQuery(offset int) launcher.SearchQuery {
	kind := normalizeResourceType(a.resourceType)
	loader := loaderKey(a.filterLoader)
	if loader == "vanilla" || (kind != "mod" && kind != "modpack") {
		loader = ""
	}
	sortKey := normalizeSortKey(a.searchSort)
	sort := ""
	if sortKey == "downloads" || sortKey == "updated" {
		sort = sortKey
	}
	return launcher.SearchQuery{Source: a.provider, Query: a.query, Type: kind, Minecraft: a.filterMC, Loader: loader, Offset: offset, Sort: sort, Categories: append([]string(nil), a.selectedCats()...)}
}

type discoverSearchFunc func(context.Context, launcher.SearchQuery) (launcher.SearchResult, error)

type discoverFeedState struct {
	nextOffset int
	loading    bool
	done       bool
	err        string
	cancel     context.CancelFunc
	fetch      discoverSearchFunc
	idleCancel context.CancelFunc
}

const discoverRetention = time.Minute

// A short visit to another page keeps the feed and its scroll position. One
// cancellable timer expires the cache; navigating among other pages must not
// extend its lifetime. All retained UI state is released on the UI thread.
func (a *App) scheduleDiscoverRelease(page string) {
	f := &a.discoverFeed
	if page == "discover" {
		if f.idleCancel != nil {
			f.idleCancel()
			f.idleCancel = nil
		}
		return
	}
	if a.page != "discover" {
		return
	}
	ctx, cancel := context.WithCancel(a.ctx)
	f.idleCancel = cancel
	timer := time.NewTimer(discoverRetention)
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		a.post(func() {
			// A return can cancel a timeout already queued for the next frame.
			if ctx.Err() != nil || a.page == "discover" {
				return
			}
			a.releaseDiscover()
		})
	}()
}

func (a *App) releaseDiscover() {
	f := &a.discoverFeed
	if f.idleCancel != nil {
		f.idleCancel()
	}
	if f.cancel != nil {
		f.cancel()
	}
	// A cancelled request may already have posted its result.
	a.searchRevision++
	released := len(a.searchResult.Projects) > 0 || len(a.discoverMotion.cardProjects) > 0
	a.searchResult = launcher.SearchResult{}
	a.searchLoading, a.searchError = false, ""
	a.discoverMotion = discoverMotionState{}
	*f = discoverFeedState{fetch: f.fetch}
	a.discoverMCOpen = false
	delete(a.numberRolls, "discover-total")
	a.imageMu.Lock()
	for key, use := range a.imageUses {
		if use.discover {
			if use.data != nil && use.data.refs == 1 {
				if bitmap, ok := use.data.source.(*ui.Bitmap); ok {
					if a.retiredImages == nil {
						a.retiredImages = make(map[*ui.Bitmap]struct{})
					}
					a.retiredImages[bitmap] = struct{}{}
				}
			}
			a.deleteImageLocked(key)
			released = true
		}
	}
	a.releaseRetiredImagesLocked()
	a.imageMu.Unlock()
	if released {
		// Let normal idle maintenance collect detached data after animations
		// and active jobs settle, even when no new allocation burst occurred.
		a.cacheRelease.Add(1)
	}
}

func (a *App) startDiscoverRequest(q launcher.SearchQuery, more bool) {
	f := &a.discoverFeed
	fetch := f.fetch
	if fetch == nil && a.service != nil {
		fetch = a.service.Search
	}
	if fetch == nil {
		return
	}
	if f.cancel != nil {
		f.cancel()
	}
	ctx, cancel := context.WithCancel(a.ctx)
	f.cancel = cancel
	a.searchRevision++
	rev := a.searchRevision
	a.searchLoading, f.loading, f.err = !more, more, ""
	if !more {
		a.searchError = ""
		m := &a.discoverMotion
		if m.hasQuery && (m.query.Source != q.Source || m.query.Type != q.Type) {
			// The sidebar already shows the new scope. Drop the old cards and
			// count now, so they cannot be mistaken for (or opened as) its results.
			a.searchResult = launcher.SearchResult{}
			m.cards, m.cardProjects = nil, nil
			m.scroll.st.ScrollTo(0, 0, ui.Animation{Duration: time.Nanosecond})
		}
	}
	a.discoverMotion.request = q
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		defer cancel()
		result, err := fetch(ctx, q)
		a.post(func() { a.finishDiscoverRequest(rev, q, more, result, err) })
	}()
}

func (a *App) finishDiscoverRequest(rev int, q launcher.SearchQuery, more bool, result launcher.SearchResult, err error) {
	if rev != a.searchRevision {
		return
	}
	f := &a.discoverFeed
	a.searchLoading, f.loading, f.cancel = false, false, nil
	if err != nil {
		if more {
			f.err = a.friendlyErr(err)
		} else {
			a.searchError = a.friendlyErr(err)
		}
		return
	}
	if more {
		if !a.discoverMotion.hasQuery || !sameDiscoverQuery(a.discoverMotion.query, q) || q.Offset != f.nextOffset {
			return
		}
		a.appendDiscoverResults(q, result)
		return
	}
	a.searchError = ""
	a.acceptDiscoverResults(q, result)
}

func (a *App) loadMoreDiscover() {
	m, f := &a.discoverMotion, &a.discoverFeed
	if !m.hasQuery || a.searchLoading ||
		f.loading || f.done || a.searchError != "" || a.provider != m.query.Source ||
		normalizeResourceType(a.resourceType) != m.query.Type {
		return
	}
	// Continue the submitted query, not text the user is still editing.
	q := m.query
	q.Offset, q.Limit = f.nextOffset, a.searchResult.Limit
	a.startDiscoverRequest(q, true)
}

// The cursor counts raw API records, including duplicates. Deduplicating the
// visible list must never send the next request to an already-consumed offset.
func (a *App) resetDiscoverFeed(q launcher.SearchQuery, result *launcher.SearchResult) {
	f := &a.discoverFeed
	n := len(result.Projects)
	result.Projects = uniqueDiscoverProjects(nil, result.Projects)
	f.nextOffset = q.Offset + n
	f.loading, f.err = false, ""
	f.done = discoverFeedEnd(q, *result, n) || len(result.Projects) == 0
}

func (a *App) appendDiscoverResults(q launcher.SearchQuery, result launcher.SearchResult) {
	f := &a.discoverFeed
	before, n := len(a.searchResult.Projects), len(result.Projects)
	a.searchResult.Projects = uniqueDiscoverProjects(a.searchResult.Projects, result.Projects)
	a.searchResult.Total = max(result.Total, len(a.searchResult.Projects))
	f.nextOffset, f.err = q.Offset+n, ""
	f.done = discoverFeedEnd(q, result, n) || len(a.searchResult.Projects) == before
	a.discoverMotion.mode = discoverLift
	a.discoverMotion.cardProjects = nil
}

func discoverFeedEnd(q launcher.SearchQuery, result launcher.SearchResult, received int) bool {
	limit := result.Limit
	if limit <= 0 {
		limit = q.Limit
	}
	if limit <= 0 {
		limit = 18
	}
	return received == 0 || (result.Total > 0 && q.Offset+received >= result.Total) ||
		(result.Total <= 0 && received < limit)
}

func uniqueDiscoverProjects(existing, page []launcher.Project) []launcher.Project {
	seen := make(map[string]bool, len(existing)+len(page))
	for _, p := range existing {
		seen[discoverProjectKey(p)] = true
	}
	for _, p := range page {
		key := discoverProjectKey(p)
		if p.ID != "" && !seen[key] {
			existing = append(existing, p)
			seen[key] = true
		}
	}
	return existing
}

// A fixed-height canvas builds only nearby cards. Its full height preserves the
// scrollbar and scroll anchor while more results are appended below it.
func (a *App) discoverFeedGrid(c *ui.Context, grid ui.Element, cols int, ps []launcher.Project) {
	s := uiScale(c)
	gap, cardH := 14*s, 194*s+2 // the card's fixed slots plus its 1 px border
	step := cardH + gap
	rows := (len(ps) + cols - 1) / cols
	bounds := grid.LayoutBounds()
	w := discoverGridWidth(c)
	_, bounds.Y = grid.LocalToWindow(0, 0)
	_, windowH := c.Size()
	first, last := addonGridWindow(rows, step, bounds, a.discoverMotion.viewport, windowH)
	cardW := max(0, (w-float32(cols-1)*gap)/float32(cols))
	grid.Height(max(0, float32(rows)*step-gap)).Children(func() {
		for i := first * cols; i < min(len(ps), last*cols); i++ {
			a.discoverCard(c, ps[i], i, cols).Absolute().
				Left(float32(i%cols)*(cardW+gap)).Top(float32(i/cols)*step).Size(cardW, cardH)
		}
	})
}

func (a *App) discoverFeedFooter(c *ui.Context) {
	s := uiScale(c)
	f, m := &a.discoverFeed, &a.discoverMotion
	// A new cursor needs one layout before checking the end. Otherwise the old
	// footer position could trigger multiple requests before appended rows lay out.
	footer := ui.Row(c.Key(fmt.Sprintf("feed-footer-%d-%d", a.searchRevision, f.nextOffset))).
		Label(a.L("连续加载状态", "Continuous loading status")).FillWidth().Height(44 * s).Gap(8 * s).Center()
	_, y := footer.LocalToWindow(0, 0)
	nearEnd := footer.LayoutBounds().H > 0 && m.viewport.H > 0 && y <= m.viewport.Y+m.viewport.H+220*s
	if nearEnd && f.err == "" {
		a.loadMoreDiscover()
	} else if footer.LayoutBounds().H <= 0 {
		c.AnimationFrame()
	}
	footer.Children(func() {
		switch {
		case a.searchLoading:
			label(c, a.L("正在更新结果…", "Updating results…"))
		case f.loading:
			ui.Spinner(c).Size(14*s, 14*s)
			label(c, a.L("正在加载更多…", "Loading more…"))
		case f.err != "":
			label(c, a.L("后续结果加载失败", "Couldn't load more results"))
			if fixedButton(c, "discover-retry-more", a.L("重试", "Retry"), "refresh", 0, btnSm*s, false, false) {
				a.loadMoreDiscover()
			}
		case f.done:
			label(c, a.L("没有更多结果", "No more results"))
		default:
			label(c, a.L("向下滚动加载更多", "Scroll down for more"))
		}
	})
}
