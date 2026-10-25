package nativeui

import (
	"context"
	"image"
	"reflect"
	"runtime"
	"testing"
	"testing/synctest"
	"time"
	"weak"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func discoverRetentionApp(t *testing.T) *App {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{
		ctx: ctx, cancel: cancel, events: make(chan func(), 16),
		page: "discover", provider: "modrinth", resourceType: "modpack",
		manifestLoading: true, // navigation must not fetch the manifest in these tests
	}
	a.acceptDiscoverResults(a.discoverQuery(0), launcher.SearchResult{Projects: discoverMotionProjects(0, 18), Total: 100, Limit: 18})
	a.discoverMotion.cardProjects = a.searchResult.Projects
	a.discoverMotion.scroll.st.Y = 400
	t.Cleanup(func() { cancel(); a.bgWG.Wait() })
	return a
}

func drainDiscoverEvents(a *App) {
	for {
		select {
		case fn := <-a.events:
			fn()
		default:
			return
		}
	}
}

func TestDiscoverRetentionReturnsWithinMinute(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := discoverRetentionApp(t)
		first := &a.searchResult.Projects[0]
		a.goPage("tasks")
		time.Sleep(59 * time.Second)
		synctest.Wait()
		if len(a.events) != 0 || &a.searchResult.Projects[0] != first {
			t.Fatal("the feed was released before one minute")
		}
		a.goPage("discover")
		time.Sleep(2 * time.Minute)
		synctest.Wait()
		drainDiscoverEvents(a)
		if &a.searchResult.Projects[0] != first || a.discoverMotion.scroll.st.Y != 400 || a.discoverFeed.idleCancel != nil || a.cacheRelease.Load() != 0 {
			t.Fatal("returning lost the feed, scroll position, or left an eviction armed")
		}
	})
}

func TestDiscoverRetentionKeepsEmptyResultsUntilExpiry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := discoverRetentionApp(t)
		a.query = "no matching projects"
		a.acceptDiscoverResults(a.discoverQuery(0), launcher.SearchResult{})
		requests := make(chan launcher.SearchQuery, 4)
		a.discoverFeed.fetch = func(ctx context.Context, q launcher.SearchQuery) (launcher.SearchResult, error) {
			requests <- q
			return launcher.SearchResult{}, nil
		}
		a.goPage("tasks")
		time.Sleep(30 * time.Second)
		a.goPage("discover")
		synctest.Wait()
		if len(requests) != 0 || a.searchLoading {
			t.Fatal("returning within one minute resubmitted a completed empty search")
		}
		a.goPage("tasks")
		time.Sleep(time.Minute)
		synctest.Wait()
		drainDiscoverEvents(a)
		a.goPage("discover")
		synctest.Wait()
		if len(requests) != 1 || (<-requests).Query != "no matching projects" {
			t.Fatal("an expired empty search was not reloaded with its saved query")
		}
	})
}

func TestGoDiscoverSubmitsOneSearch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := discoverRetentionApp(t)
		// A real service is present in production; the injected fetch prevents
		// this test from performing any network or filesystem work.
		a.service = &launcher.Service{}
		a.page = "tasks"
		requests := make(chan launcher.SearchQuery, 4)
		a.discoverFeed.fetch = func(ctx context.Context, q launcher.SearchQuery) (launcher.SearchResult, error) {
			requests <- q
			return launcher.SearchResult{}, nil
		}
		a.goDiscover("mods", "sodium")
		synctest.Wait()
		if len(requests) != 1 {
			t.Fatalf("opening Discover submitted %d searches; want one", len(requests))
		}
		q := <-requests
		if q.Type != "mod" || q.Query != "sodium" || q.Offset != 0 {
			t.Fatalf("opening Discover searched the wrong scope: %+v", q)
		}
	})
}

func TestDiscoverRetentionReplacesDifferentPendingSearch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := discoverRetentionApp(t)
		a.query = "empty search"
		a.acceptDiscoverResults(a.discoverQuery(0), launcher.SearchResult{})
		requests := make(chan feedTestRequest, 4)
		a.discoverFeed.fetch = func(ctx context.Context, q launcher.SearchQuery) (launcher.SearchResult, error) {
			requests <- feedTestRequest{ctx: ctx, query: q}
			<-ctx.Done()
			return launcher.SearchResult{}, ctx.Err()
		}
		a.query = "different search"
		a.search()
		synctest.Wait()
		old := <-requests
		a.query = "empty search"
		a.goPage("tasks")
		a.goPage("discover")
		synctest.Wait()
		if len(requests) != 1 || old.ctx.Err() == nil {
			t.Fatal("an old empty result suppressed replacement of a mismatched pending search")
		}
		fresh := <-requests
		if fresh.query.Query != "empty search" || fresh.ctx.Err() != nil {
			t.Fatal("the returning search does not match the current controls")
		}
	})
}

func TestDiscoverRetentionExpiresAcrossOtherPages(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := discoverRetentionApp(t)
		projects := weak.Make(&a.searchResult.Projects[0])
		a.goPage("tasks")
		time.Sleep(40 * time.Second)
		a.goPage("home")
		time.Sleep(19 * time.Second)
		synctest.Wait()
		if len(a.events) != 0 || len(a.searchResult.Projects) == 0 {
			t.Fatal("other page navigation released the feed early")
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if len(a.events) != 1 || len(a.searchResult.Projects) == 0 {
			t.Fatal("expiry did not queue exactly one UI-thread cleanup at 60 seconds")
		}
		drainDiscoverEvents(a)
		if a.searchResult.Projects != nil || a.discoverMotion.cardProjects != nil || a.discoverMotion.cards != nil || a.discoverMotion.hasQuery || a.discoverMotion.scroll.st.Y != 0 || a.discoverFeed.idleCancel != nil || a.cacheRelease.Load() != 1 {
			t.Fatal("expiry retained the feed or did not request idle reclamation")
		}
		runtime.GC()
		if projects.Value() != nil {
			t.Fatal("expired projects remain reachable")
		}
		time.Sleep(2 * time.Minute)
		synctest.Wait()
		if len(a.events) != 0 || a.cacheRelease.Load() != 1 {
			t.Fatal("cleanup kept waking the UI after expiry")
		}
		runtime.KeepAlive(a)
	})
}

func TestDiscoverRetentionExpiresWithoutPainting(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := discoverRetentionApp(t)
		a.background.Store(true)
		// Model Window.Update's main-thread queue while a minimized window
		// stops painting. No View or tester frame drains the result queue.
		updates := make(chan func(), 8)
		a.updateUI = func(fn func()) { updates <- fn }
		bitmap := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 96, 96)))
		a.cacheImageLocked("hidden-cover", bitmap, 0, 96)
		a.remoteImageSize("hidden-cover", 96)
		a.goPage("tasks")
		time.Sleep(time.Minute)
		synctest.Wait()
		if len(updates) != 1 {
			t.Fatal("expiry did not dispatch exactly one main-thread update")
		}
		(<-updates)()
		if len(a.events) != 0 || a.searchResult.Projects != nil || a.cacheRelease.Load() != 1 || a.frameSerial.Load() != 0 {
			t.Fatal("cleanup depended on a new frame in a hidden window")
		}
		if w, h := bitmap.Size(); w != 1 || h != 1 || len(a.images) != 0 {
			t.Fatal("hidden-window cleanup retained unused cover pixels")
		}
		time.Sleep(2 * time.Minute)
		synctest.Wait()
		if len(updates) != 0 {
			t.Fatal("hidden cleanup kept waking the main thread after expiry")
		}
	})
}

func TestDiscoverRetentionIgnoresQueuedTimeoutAfterReturn(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := discoverRetentionApp(t)
		a.goPage("tasks")
		time.Sleep(time.Minute)
		synctest.Wait() // the timeout is queued, but no UI frame has processed it
		a.goPage("discover")
		a.goPage("tasks")
		drainDiscoverEvents(a)
		time.Sleep(59 * time.Second)
		synctest.Wait()
		drainDiscoverEvents(a)
		if len(a.searchResult.Projects) == 0 || a.discoverFeed.idleCancel == nil {
			t.Fatal("an old timeout cleared the returning feed or its new timer")
		}
		time.Sleep(time.Second)
		synctest.Wait()
		drainDiscoverEvents(a)
		if a.searchResult.Projects != nil || a.discoverFeed.idleCancel != nil {
			t.Fatal("the second departure did not get its own one-minute deadline")
		}
	})
}

func TestDiscoverRetentionCancelsRequestsAndReloadsFilters(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := discoverRetentionApp(t)
		a.provider, a.resourceType, a.query = "curseforge", "mod", "tools"
		a.filterMC, a.filterLoader, a.searchSort = "1.21.1", "neoforge", "downloads"
		a.catSel = map[string][]string{"curseforge|mod": {"technology"}}
		want := a.discoverQuery(0)
		requests := make(chan feedTestRequest, 4)
		a.discoverFeed.fetch = func(ctx context.Context, q launcher.SearchQuery) (launcher.SearchResult, error) {
			r := feedTestRequest{ctx: ctx, query: q, reply: make(chan feedTestReply, 1)}
			requests <- r
			select {
			case reply := <-r.reply:
				return reply.result, reply.err
			case <-ctx.Done():
				return launcher.SearchResult{}, ctx.Err()
			}
		}
		a.search()
		synctest.Wait()
		old := <-requests
		oldRevision := a.searchRevision
		a.goPage("tasks")
		a.goPage("discover")
		synctest.Wait()
		if old.ctx.Err() != nil || len(requests) != 0 {
			t.Fatal("a quick return restarted the unfinished search")
		}
		a.goPage("tasks")
		time.Sleep(time.Minute)
		synctest.Wait()
		drainDiscoverEvents(a)
		synctest.Wait()
		drainDiscoverEvents(a)
		if old.ctx.Err() == nil || a.searchLoading || a.discoverFeed.loading || a.searchError != "" || a.discoverFeed.err != "" {
			t.Fatal("expiry did not cancel the search cleanly")
		}
		a.goPage("discover")
		synctest.Wait()
		fresh := <-requests
		if !reflect.DeepEqual(fresh.query, want) || fresh.query.Offset != 0 || !a.searchLoading {
			t.Fatalf("return did not reload the saved filters from page one: %+v", fresh.query)
		}
		a.finishDiscoverRequest(oldRevision, old.query, false, launcher.SearchResult{Projects: discoverMotionProjects(30, 2)}, nil)
		if len(a.searchResult.Projects) != 0 || !a.searchLoading {
			t.Fatal("late results repopulated an expired feed")
		}
		fresh.reply <- feedTestReply{result: launcher.SearchResult{Projects: discoverMotionProjects(80, 2), Total: 2}}
		synctest.Wait()
		drainDiscoverEvents(a)
		if a.searchLoading || len(a.searchResult.Projects) != 2 || a.searchResult.Projects[0].ID != "80" {
			t.Fatal("the new search did not replace the expired feed")
		}
	})
}

func TestDiscoverRetentionReleasesOnlyUnusedArtwork(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := discoverRetentionApp(t)
		add := func(key string, content imageContentKey) weak.Pointer[ui.Bitmap] {
			bitmap := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 96, 96)))
			a.cacheImageContentLocked(key, bitmap, 0, content)
			a.remoteImageSize(key, 96)
			return weak.Make(bitmap)
		}
		expired := add("discover-only", imageContentKey{sum: [32]byte{1}, edge: 96})
		add("reused-by-home", imageContentKey{sum: [32]byte{2}, edge: 96})
		add("shared-pixels", imageContentKey{sum: [32]byte{3}, edge: 96})
		a.cacheImageContentLocked("addon:local", a.images["shared-pixels"], 0, imageContentKey{sum: [32]byte{3}, edge: 96})
		a.goPage("home")
		shared := a.remoteImageSize("reused-by-home", 96)
		active := a.imageContextLocked()
		before := a.imageBytes
		time.Sleep(time.Minute)
		synctest.Wait()
		drainDiscoverEvents(a)
		if a.images["discover-only"] != nil || a.images["shared-pixels"] != nil || a.images["reused-by-home"] != shared || a.images["addon:local"] == nil || active.Err() != nil {
			t.Fatal("expiry retained stale artwork or evicted/cancelled another page's images")
		}
		if len(a.images) != 2 || len(a.imageContents) != 2 || a.imageBytes != before-imageCost(shared, 0) {
			t.Fatal("shared content references or cache accounting leaked on expiry")
		}
		runtime.GC()
		if expired.Value() != nil {
			t.Fatal("expired bitmap is still retained")
		}
		runtime.KeepAlive(a)
	})
}

func TestDiscoverRetentionStopsOnShutdown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := discoverRetentionApp(t)
		a.goPage("tasks")
		a.cancel()
		a.bgWG.Wait()
		time.Sleep(2 * time.Minute)
		synctest.Wait()
		if len(a.events) != 0 {
			t.Fatal("the timer posted a cleanup after shutdown")
		}
	})
}

func TestDiscoverRetiredArtworkWaitsForDecoders(t *testing.T) {
	for _, reused := range []bool{false, true} {
		name := "cancelled"
		if reused {
			name = "shared-with-next-page"
		}
		t.Run(name, func(t *testing.T) {
			a := discoverRetentionApp(t)
			bitmap := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 96, 96)))
			content := imageContentKey{edge: 96}
			a.cacheImageContentLocked("old-cover", bitmap, 0, content)
			a.remoteImageSize("old-cover", 96)
			a.goPage("tasks")
			a.imagePending = make(map[string]bool)
			ctx := a.imageContextLocked()
			// A decoder on the new page has borrowed the content but has not
			// published its alias yet. Expiry must not mutate its source.
			a.reserveImageLocked("new-cover", ctx)
			a.releaseDiscover()
			if w, _ := bitmap.Size(); w != 96 || len(a.retiredImages) != 1 {
				t.Fatal("expiry invalidated an in-flight shared decode")
			}
			if reused {
				a.cacheImageContentLocked("new-cover", bitmap, 0, content)
			}
			a.clearImagePending("new-cover", ctx)
			if w, _ := bitmap.Size(); w != 96 {
				t.Fatal("decoder completion mutated a bitmap outside the UI callback")
			}
			// More work can start before that callback reaches the UI.
			a.reserveImageLocked("another-cover", ctx)
			drainDiscoverEvents(a)
			if w, _ := bitmap.Size(); w != 96 || a.imageReleasePosted {
				t.Fatal("the queued cleanup ignored newly started image work")
			}
			a.clearImagePending("another-cover", ctx)
			drainDiscoverEvents(a)
			want := 1
			if reused {
				want = 96
			}
			if w, h := bitmap.Size(); w != want || h != want || len(a.retiredImages) != 0 || a.imageReleasePosted {
				t.Fatal("settled cleanup retained unused pixels or invalidated shared artwork")
			}
		})
	}
}

func TestDiscoverRetentionRestoresScrollAndReleasesRenderedResources(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	a := discoverRetentionApp(t)
	a.acceptDiscoverResults(a.discoverQuery(0), launcher.SearchResult{Projects: discoverMotionProjects(0, 90), Total: 90})
	projects := weak.Make(&a.searchResult.Projects[0])
	artwork := func() weak.Pointer[byte] {
		pixels := image.NewRGBA(image.Rect(0, 0, 96, 96))
		bitmap := ui.NewBitmap(pixels)
		a.cacheImageLocked("rendered-cover", bitmap, 0, 96)
		return weak.Make(&pixels.Pix[0])
	}()
	for i := range a.searchResult.Projects {
		a.searchResult.Projects[i].Icon = "rendered-cover"
	}
	tt := ui.NewTester(func(c *ui.Context) {
		drainDiscoverEvents(a)
		c.SetTheme(theme)
		ui.Column(c).Fill().Children(func() {
			if a.page == "discover" {
				a.discoverView(c)
			} else {
				ui.Text(c, "Another page")
			}
		})
	}, 1536, 1024)
	tt.Advance(time.Second)
	tt.Scroll(900, 500, 0, 600)
	tt.Advance(time.Second)
	y := a.discoverMotion.scroll.st.Y
	if y == 0 {
		t.Fatal("fixture did not scroll")
	}
	a.goPage("tasks")
	tt.Frame()
	tt.Advance(time.Second)
	a.goPage("discover")
	tt.Frame()
	tt.Advance(time.Second)
	if a.discoverMotion.scroll.st.Y != y || len(a.searchResult.Projects) != 90 || a.searchLoading {
		t.Fatal("returning rebuilt the feed or reset the rendered scroll position")
	}
	a.goPage("tasks")
	tt.Frame()
	tt.Advance(time.Second)
	a.releaseDiscover()
	tt.Frame()
	tt.Advance(time.Second)
	runtime.GC()
	if projects.Value() != nil || artwork.Value() != nil {
		t.Fatalf("the renderer retained expired Discover resources: projects=%t pixels=%t", projects.Value() != nil, artwork.Value() != nil)
	}
	runtime.KeepAlive(tt)
	runtime.KeepAlive(a)
}
