package nativeui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type feedTestReply struct {
	result launcher.SearchResult
	err    error
}

type feedTestRequest struct {
	query launcher.SearchQuery
	ctx   context.Context
	reply chan feedTestReply
}

func discoverFeedTester(t *testing.T, source, kind string) (*App, *ui.Tester, <-chan feedTestRequest) {
	t.Helper()
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{ctx: ctx, cancel: cancel, events: make(chan func(), 64), provider: source, resourceType: kind}
	a.settings.Language = LangZH
	requests := make(chan feedTestRequest, 16)
	a.discoverFeed.fetch = func(ctx context.Context, q launcher.SearchQuery) (launcher.SearchResult, error) {
		r := feedTestRequest{query: q, ctx: ctx, reply: make(chan feedTestReply, 1)}
		select {
		case requests <- r:
		case <-ctx.Done():
			return launcher.SearchResult{}, ctx.Err()
		}
		select {
		case reply := <-r.reply:
			return reply.result, reply.err
		case <-ctx.Done():
			return launcher.SearchResult{}, ctx.Err()
		}
	}
	t.Cleanup(func() { cancel(); a.bgWG.Wait() })
	tt := ui.NewTester(func(c *ui.Context) {
		for len(a.events) > 0 {
			(<-a.events)()
		}
		c.SetTheme(theme)
		ui.Column(c).Fill().Background(chromeBg).Children(func() { a.discoverView(c) })
	}, 1536, 1024)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	a.search()
	tt.Frame()
	return a, tt, requests
}

func nextFeedTestRequest(t *testing.T, tt *ui.Tester, requests <-chan feedTestRequest) feedTestRequest {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		tt.Frame()
		select {
		case r := <-requests:
			return r
		default:
			time.Sleep(time.Millisecond)
		}
	}
	t.Fatal("expected a search request")
	return feedTestRequest{}
}

func replyFeedTest(t *testing.T, a *App, tt *ui.Tester, request feedTestRequest, start, n, total int) {
	t.Helper()
	ps := discoverMotionProjects(start, n)
	for i := range ps {
		ps[i].Source, ps[i].Type = request.query.Source, request.query.Type
	}
	request.reply <- feedTestReply{result: launcher.SearchResult{Projects: ps, Total: total, Offset: request.query.Offset, Limit: 18}}
	waitUI(t, tt, func() bool { return !a.searchLoading && !a.discoverFeed.loading })
	tt.Frame()
}

func assertDiscoverSwitchLoading(t *testing.T, a *App, tt *ui.Tester) {
	t.Helper()
	if !a.searchLoading || !tt.HasText("资源加载提示") || !tt.HasText(a.tr().DiscoverLoading) || tt.HasText("搜索结果列表") || len(a.searchResult.Projects) != 0 {
		t.Fatal("switch did not immediately replace the old results with a visible loading state")
	}
	if a.discoverMotion.scroll.st.Y != 0 || a.numberRolls["discover-total"].target != "—" {
		t.Fatal("loading kept the previous result count or scroll position")
	}
}

func TestDiscoverTypeSwitchShowsLoadingImmediately(t *testing.T) {
	for _, source := range []string{"modrinth", "curseforge"} {
		for _, lang := range []string{LangZH, LangEN} {
			for _, kind := range []string{"modpack", "mod", "resourcepack", "shader"} {
				t.Run(source+"-"+lang+"-"+kind, func(t *testing.T) {
					previousKind := "mod"
					if kind == previousKind {
						previousKind = "resourcepack"
					}
					a, tt, requests := discoverFeedTester(t, source, previousKind)
					a.OverrideLanguage(lang)
					if lang == LangEN {
						tt.SetSize(900, 540)
					} else {
						tt.SetPreferences(ui.Preferences{})
					}
					first := nextFeedTestRequest(t, tt, requests)
					replyFeedTest(t, a, tt, first, 0, 18, 18)
					tt.Advance(time.Second)
					tt.Scroll(800, 450, 0, 500)
					tt.Advance(400 * time.Millisecond)
					a.query, a.filterMC = "tw", "1.20.1"
					tr := a.tr()
					labels := map[string]string{"modpack": tr.DiscoverTypeModpack, "mod": tr.DiscoverTypeMod, "resourcepack": tr.DiscoverTypeRP, "shader": tr.DiscoverTypeShader}
					clickUI(t, tt, labels[kind])
					assertDiscoverSwitchLoading(t, a, tt)
					next := nextFeedTestRequest(t, tt, requests)
					if next.query.Source != source || next.query.Type != kind || next.query.Query != "tw" || next.query.Minecraft != "1.20.1" || next.query.Offset != 0 {
						t.Fatalf("switch lost its selected scope or filters: %+v", next.query)
					}
					tt.Advance(150 * time.Millisecond)
					snapshotUI(t, tt, "discover-switch-loading-"+source+"-"+lang+"-"+kind)
					replyFeedTest(t, a, tt, next, 100, 6, 6)
					if tt.HasText("资源加载提示") || !tt.HasText("搜索结果 探索整合包 100") {
						t.Fatal("latest response did not replace the loading state")
					}
					tt.Advance(90 * time.Millisecond)
					motion := a.discoverMotion.cards[source+"|100"]
					if lang == LangZH && (motion.progress <= 0 || motion.progress >= 1) {
						t.Fatal("new results skipped their entrance animation")
					}
					tt.Advance(600 * time.Millisecond)
					if motion.progress != 1 {
						t.Fatal("new results did not finish appearing")
					}
				})
			}
		}
	}
}

func TestDiscoverProviderSwitchRejectsStaleResultsAndRetriesCurrentType(t *testing.T) {
	a, tt, requests := discoverFeedTester(t, "modrinth", "mod")
	first := nextFeedTestRequest(t, tt, requests)
	replyFeedTest(t, a, tt, first, 0, 18, 18)
	clickUI(t, tt, "CurseForge")
	assertDiscoverSwitchLoading(t, a, tt)
	old := nextFeedTestRequest(t, tt, requests)
	oldRevision := a.searchRevision
	clickUI(t, tt, a.tr().DiscoverTypeRP)
	assertDiscoverSwitchLoading(t, a, tt)
	latest := nextFeedTestRequest(t, tt, requests)
	if old.ctx.Err() == nil || latest.query.Source != "curseforge" || latest.query.Type != "resourcepack" {
		t.Fatal("rapid switching did not cancel the previous provider/type request")
	}
	a.finishDiscoverRequest(oldRevision, old.query, false, launcher.SearchResult{Projects: discoverMotionProjects(50, 3), Total: 3}, nil)
	tt.Frame()
	assertDiscoverSwitchLoading(t, a, tt)
	latest.reply <- feedTestReply{err: errors.New("connection reset")}
	waitUI(t, tt, func() bool { return !a.searchLoading })
	if !tt.HasText("加载失败") || tt.HasText("搜索结果列表") || tt.HasText("资源加载提示") || len(a.searchResult.Projects) != 0 {
		t.Fatal("failed switch restored results from the previous type")
	}
	clickUI(t, tt, "重试")
	assertDiscoverSwitchLoading(t, a, tt)
	retry := nextFeedTestRequest(t, tt, requests)
	if !sameDiscoverQuery(retry.query, latest.query) || retry.query.Offset != 0 {
		t.Fatal("retry did not use the currently selected provider and type")
	}
	replyFeedTest(t, a, tt, retry, 100, 3, 3)
	a.finishDiscoverRequest(oldRevision, old.query, false, launcher.SearchResult{}, errors.New("stale failure"))
	tt.Frame()
	if a.searchError != "" || !tt.HasText("搜索结果 探索整合包 100") || tt.HasText("搜索结果 探索整合包 50") {
		t.Fatal("a stale failure replaced the current successful response")
	}
}

func TestDiscoverFeedAppendsDeduplicatesAndPreservesScroll(t *testing.T) {
	for _, tc := range []struct{ source, kind string }{
		{"modrinth", "mod"}, {"curseforge", "mod"},
		{"modrinth", "modpack"}, {"curseforge", "modpack"},
	} {
		t.Run(tc.source+"/"+tc.kind, func(t *testing.T) {
			source, kind := tc.source, tc.kind
			a, tt, requests := discoverFeedTester(t, source, kind)
			first := nextFeedTestRequest(t, tt, requests)
			replyFeedTest(t, a, tt, first, 0, 18, 42)
			if tt.HasText(a.tr().DiscoverNextPage) || tt.HasText(a.tr().DiscoverPrevPage) {
				t.Fatal("continuous results still expose page navigation")
			}
			// Editing the search field must not leak into an automatic continuation.
			a.query = "not submitted"
			tt.Scroll(1200, 800, 0, 1000)
			second := nextFeedTestRequest(t, tt, requests)
			if second.query.Offset != 18 || second.query.Query != first.query.Query || second.query.Source != source || second.query.Type != kind {
				t.Fatalf("continuation lost its submitted query: %+v", second.query)
			}
			for range 8 {
				tt.Frame()
				a.loadMoreDiscover()
			}
			if len(requests) != 0 {
				t.Fatal("one near-bottom event started concurrent requests")
			}
			position := a.discoverMotion.scroll.st.Y
			card, _ := tt.Find("搜索结果 探索整合包 15")
			pixels := overviewRegionPixels(tt.Image(), card)
			oldState := a.discoverMotion.cards[source+"|15"]
			replyFeedTest(t, a, tt, second, 17, 18, 42) // one duplicate in the API page
			if len(a.searchResult.Projects) != 35 || a.discoverFeed.nextOffset != 36 {
				t.Fatalf("deduplication corrupted the API cursor: %d records, offset %d", len(a.searchResult.Projects), a.discoverFeed.nextOffset)
			}
			if math.Abs(float64(a.discoverMotion.scroll.st.Y-position)) > 1 || oldState != a.discoverMotion.cards[source+"|15"] {
				t.Fatal("appending replaced cards or reset the scroll position")
			}
			if !bytes.Equal(pixels, overviewRegionPixels(tt.Image(), card)) {
				t.Fatal("appending repainted an existing card")
			}
			if len(requests) != 0 {
				t.Fatal("old footer coordinates immediately fetched another page")
			}
			snapshotUI(t, tt, "discover-feed-append-"+source+"-"+kind)
			tt.Scroll(1200, 800, 0, 10000)
			third := nextFeedTestRequest(t, tt, requests)
			if third.query.Offset != 36 {
				t.Fatalf("used deduplicated count instead of consumed API offset: %d", third.query.Offset)
			}
			replyFeedTest(t, a, tt, third, 35, 7, 42)
			if len(a.searchResult.Projects) != 42 || !a.discoverFeed.done {
				t.Fatal("last batch did not finish the feed")
			}
			tt.Scroll(1200, 800, 0, 10000)
			tt.Advance(300 * time.Millisecond)
			if len(requests) != 0 || !tt.HasText("没有更多结果") {
				t.Fatal("exhausted feed kept requesting pages")
			}
			snapshotUI(t, tt, "discover-feed-end-"+source+"-"+kind)
		})
	}
}

func TestDiscoverFeedFailureRetriesInPlaceAndRejectsStalePages(t *testing.T) {
	a, tt, requests := discoverFeedTester(t, "modrinth", "shader")
	first := nextFeedTestRequest(t, tt, requests)
	replyFeedTest(t, a, tt, first, 0, 18, 100)
	tt.Scroll(1200, 800, 0, 1000)
	failed := nextFeedTestRequest(t, tt, requests)
	failed.reply <- feedTestReply{err: errors.New("connection reset")}
	waitUI(t, tt, func() bool { return a.discoverFeed.err != "" })
	if a.searchError != "" || len(a.searchResult.Projects) != 18 || a.discoverMotion.scroll.st.Y == 0 {
		t.Fatal("continuation failure replaced or scrolled the result list")
	}
	tt.Advance(300 * time.Millisecond)
	if len(requests) != 0 {
		t.Fatal("failed continuation entered an automatic retry loop")
	}
	clickUI(t, tt, "重试")
	retry := nextFeedTestRequest(t, tt, requests)
	if retry.query.Offset != failed.query.Offset {
		t.Fatal("retry skipped the failed page")
	}
	oldRev := a.searchRevision
	a.filterMC = "1.21.1"
	a.search()
	replacement := nextFeedTestRequest(t, tt, requests)
	if retry.ctx.Err() == nil || replacement.query.Offset != 0 || replacement.query.Minecraft != "1.21.1" {
		t.Fatal("filtering did not cancel the old continuation and start from the beginning")
	}
	replyFeedTest(t, a, tt, replacement, 100, 6, 6)
	a.finishDiscoverRequest(oldRev, retry.query, true, launcher.SearchResult{Projects: discoverMotionProjects(18, 18), Total: 100, Limit: 18}, nil)
	tt.Frame()
	if len(a.searchResult.Projects) != 6 || a.searchResult.Projects[0].ID != "100" || a.discoverMotion.scroll.st.Y != 0 {
		t.Fatal("late response contaminated the replacement query")
	}
	if a.discoverFeed.err != "" || !a.discoverFeed.done {
		t.Fatal("replacement inherited the previous feed's failure state")
	}
}

func TestDiscoverFeedStopsOnEmptyOrRepeatedBatches(t *testing.T) {
	for _, n := range []int{0, 18} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			a, tt, requests := discoverFeedTester(t, "modrinth", "resourcepack")
			first := nextFeedTestRequest(t, tt, requests)
			replyFeedTest(t, a, tt, first, 0, 18, 1000)
			tt.Scroll(1200, 800, 0, 1000)
			second := nextFeedTestRequest(t, tt, requests)
			replyFeedTest(t, a, tt, second, 0, n, 1000)
			tt.Advance(300 * time.Millisecond)
			if !a.discoverFeed.done || len(a.searchResult.Projects) != 18 || len(requests) != 0 {
				t.Fatal("an empty or repeated batch created a request loop")
			}
		})
	}
}

func TestDiscoverFeedAnimatesNewCardsWithoutReplayingVisitedCards(t *testing.T) {
	a, tt, requests := discoverFeedTester(t, "modrinth", "modpack")
	tt.SetPreferences(ui.Preferences{})
	first := nextFeedTestRequest(t, tt, requests)
	replyFeedTest(t, a, tt, first, 0, 18, 36)
	tt.Advance(650 * time.Millisecond)
	tt.Scroll(1200, 800, 0, 1000)
	second := nextFeedTestRequest(t, tt, requests)
	tt.Advance(650 * time.Millisecond)
	old := a.discoverMotion.cards["modrinth|15"]
	replyFeedTest(t, a, tt, second, 18, 18, 36)
	tt.Advance(215 * time.Millisecond)
	added := a.discoverMotion.cards["modrinth|18"]
	if !added.started || added.progress <= 0 || added.progress >= 1 || old.progress != 1 {
		t.Fatalf("append did not animate only new cards: new %.3f, existing %.3f", added.progress, old.progress)
	}
	snapshotUI(t, tt, "discover-feed-new-cards-215ms")
	tt.Advance(650 * time.Millisecond)
	tt.Scroll(1200, 800, 0, -10000)
	tt.Advance(65 * time.Millisecond)
	if a.discoverMotion.cards["modrinth|0"].progress != 1 {
		t.Fatal("returning to a virtualized card replayed its entrance")
	}
}

func TestDiscoverFeedVirtualizesAllResourceTypes(t *testing.T) {
	for _, kind := range []string{"modpack", "mod", "resourcepack", "shader"} {
		t.Run(kind, func(t *testing.T) {
			a, tt := discoverMotionTester(t)
			a.resourceType = kind
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			ps := discoverMotionProjects(0, 1000)
			for i := range ps {
				ps[i].Type = kind
			}
			showDiscoverResults(a, tt, launcher.SearchQuery{Source: "modrinth", Type: kind}, ps)
			count := func() int {
				n := 0
				for _, text := range tt.Texts() {
					if strings.HasPrefix(text, "搜索结果 ") {
						n++
					}
				}
				return n
			}
			if n := count(); n == 0 || n > 36 {
				t.Fatalf("built %d cards for a 1000-result feed", n)
			}
			if tt.HasText(a.tr().DiscoverNextPage) || tt.HasText(a.tr().DiscoverPrevPage) {
				t.Fatal("a continuous resource type still has pagination")
			}
			a.discoverMotion.scroll.st.ScrollTo(0, math.MaxFloat32, ui.Animation{Duration: time.Nanosecond})
			tt.Frame()
			tt.Advance(100 * time.Millisecond)
			if n := count(); n == 0 || n > 36 || !tt.HasText("搜索结果 "+ps[999].Title) || tt.HasText("搜索结果 "+ps[0].Title) {
				snapshotUI(t, tt, "discover-feed-last-row-failed-"+kind)
				t.Fatalf("virtualized feed lost its last row: %d cards, scroll %.1f/%.1f, first present %t, last present %t", n,
					a.discoverMotion.scroll.st.Y, a.discoverMotion.scroll.st.MaxY,
					tt.HasText("搜索结果 "+ps[0].Title), tt.HasText("搜索结果 "+ps[999].Title))
			}
			tt.SetSize(900, 540)
			tt.Advance(100 * time.Millisecond)
			card, _ := tt.Find("搜索结果 " + ps[999].Title)
			if card.W <= 0 || card.X+card.W > 900 {
				t.Fatalf("feed card escaped the small viewport: %+v", card)
			}
			snapshotUI(t, tt, "discover-feed-small-"+kind)
		})
	}
}
