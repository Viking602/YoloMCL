package nativeui

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func discoverMotionTester(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	a := &App{provider: "modrinth", resourceType: "modpack", searchLoading: true,
		images: map[string]ui.ImageSource{}, imagePending: map[string]bool{}}
	a.settings.Language = LangZH
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		ui.Column(c).Fill().Background(chromeBg).Children(func() { a.discoverView(c) })
	}, 1536, 1024)
	tt.Advance(500 * time.Millisecond)
	return a, tt
}

func discoverMotionProjects(start, n int) []launcher.Project {
	ps := make([]launcher.Project, n)
	for i := range ps {
		ps[i] = launcher.Project{ID: fmt.Sprint(start + i), Source: "modrinth", Type: "modpack",
			Title: fmt.Sprintf("探索整合包 %02d", start+i), Author: "Minecraft community", Downloads: 123456,
			Description: "Explore a new world with friends. Build, discover and create your next adventure.",
			Categories:  []string{"adventure", "multiplayer"}}
	}
	return ps
}

func showDiscoverResults(a *App, tt *ui.Tester, q launcher.SearchQuery, ps []launcher.Project) {
	a.searchLoading = false
	a.acceptDiscoverResults(q, launcher.SearchResult{Projects: ps, Total: 120, Limit: 18, Offset: q.Offset})
	tt.Frame()
	tt.Frame() // place new cards before beginning their visible entrances
}

func TestDiscoverLoadStaggersOnlyVisibleCards(t *testing.T) {
	a, tt := discoverMotionTester(t)
	snapshotUI(t, tt, "discover-loading")
	rail, _ := tt.Find("发现筛选")
	railPixels := overviewRegionPixels(tt.Image(), rail)
	ps := discoverMotionProjects(0, 18)
	showDiscoverResults(a, tt, launcher.SearchQuery{Source: "modrinth", Type: "modpack"}, ps)
	tt.Advance(65 * time.Millisecond)
	first := a.discoverMotion.cards[discoverProjectKey(ps[0])]
	second := a.discoverMotion.cards[discoverProjectKey(ps[1])]
	if first.progress <= second.progress || first.progress <= 0 || first.progress >= 1 {
		t.Fatalf("cards did not enter in sequence: %.3f %.3f", first.progress, second.progress)
	}
	mid, _ := tt.Find("搜索结果 " + ps[0].Title)
	snapshotUI(t, tt, "discover-enter-065ms")
	tt.Advance(150 * time.Millisecond)
	snapshotUI(t, tt, "discover-enter-215ms")
	tt.Advance(450 * time.Millisecond)
	end, _ := tt.Find("搜索结果 " + ps[0].Title)
	if mid.Y <= end.Y || mid.X != end.X || mid.W != end.W {
		t.Fatalf("initial entrance should lift without changing width: mid %+v end %+v", mid, end)
	}
	if !bytes.Equal(railPixels, overviewRegionPixels(tt.Image(), rail)) {
		t.Fatal("results loading repainted the filter rail")
	}
	last := a.discoverMotion.cards[discoverProjectKey(ps[17])]
	if last.started {
		t.Fatal("offscreen cards spent their entrance before being seen")
	}
	snapshotUI(t, tt, "discover-loaded")
	tt.Scroll(1200, 800, 0, 700)
	tt.Advance(650 * time.Millisecond)
	if !last.started || last.progress != 1 {
		t.Fatal("scrolling into a new card did not reveal it")
	}
	tt.Scroll(1200, 800, 0, -700)
	tt.Advance(650 * time.Millisecond)
	if first.progress != 1 {
		t.Fatal("scrolling back replayed a card entrance")
	}
}

func TestDiscoverRefreshRetainsCardsAndFilteringExpandsNewOnes(t *testing.T) {
	a, tt := discoverMotionTester(t)
	q := launcher.SearchQuery{Source: "modrinth", Type: "modpack"}
	ps := discoverMotionProjects(0, 6)
	showDiscoverResults(a, tt, q, ps)
	tt.Advance(650 * time.Millisecond)
	box, _ := tt.Find("搜索结果 " + ps[0].Title)
	pixels := overviewRegionPixels(tt.Image(), box)
	oldState := a.discoverMotion.cards[discoverProjectKey(ps[0])]
	a.searchLoading = true
	tt.Frame()
	tt.Advance(120 * time.Millisecond)
	if !bytes.Equal(pixels, overviewRegionPixels(tt.Image(), box)) {
		t.Fatal("fetching a new result dimmed or moved an existing card")
	}
	showDiscoverResults(a, tt, q, ps)
	tt.Advance(100 * time.Millisecond)
	if !bytes.Equal(pixels, overviewRegionPixels(tt.Image(), box)) || oldState.progress != 1 {
		t.Fatal("an unchanged response replayed its animation")
	}
	q.Categories = []string{"adventure"}
	newProject := discoverMotionProjects(100, 1)[0]
	showDiscoverResults(a, tt, q, []launcher.Project{ps[0], ps[2], newProject})
	tt.Advance(90 * time.Millisecond)
	mid, _ := tt.Find("搜索结果 " + newProject.Title)
	if oldState != a.discoverMotion.cards[discoverProjectKey(ps[0])] || oldState.progress != 1 {
		t.Fatal("filtering replayed a surviving project")
	}
	if a.discoverMotion.cards[discoverProjectKey(newProject)].progress != 0 {
		t.Fatal("new card appeared on top of a card still moving out of its cell")
	}
	tt.Advance(250 * time.Millisecond)
	snapshotUI(t, tt, "discover-filter-expand")
	tt.Advance(550 * time.Millisecond)
	end, _ := tt.Find("搜索结果 " + newProject.Title)
	if mid.W >= end.W || mid.H >= end.H {
		t.Fatalf("filter results did not expand: mid %+v end %+v", mid, end)
	}
	if len(a.discoverMotion.cards) != 3 {
		t.Fatal("removed projects retained animation state")
	}
}

func TestDiscoverRapidReplacement(t *testing.T) {
	a, tt := discoverMotionTester(t)
	q := launcher.SearchQuery{Source: "modrinth", Type: "modpack"}
	showDiscoverResults(a, tt, q, discoverMotionProjects(0, 6))
	tt.Advance(650 * time.Millisecond)
	q.Query = "first"
	showDiscoverResults(a, tt, q, discoverMotionProjects(30, 3))
	tt.Advance(30 * time.Millisecond)
	q.Query = "latest"
	showDiscoverResults(a, tt, q, discoverMotionProjects(60, 3))
	tt.Advance(650 * time.Millisecond)
	if tt.HasText("探索整合包 30") || !tt.HasText("探索整合包 60") {
		t.Fatal("rapid changes left an outgoing card over the latest result")
	}
}

func TestDiscoverLateIconRevealsWithoutReplayingText(t *testing.T) {
	a, tt := discoverMotionTester(t)
	ps := discoverMotionProjects(0, 3)
	raw := "https://icons.example.test/delayed.png"
	ps[0].Icon = raw
	a.imagePending[raw] = true
	showDiscoverResults(a, tt, launcher.SearchQuery{Source: "modrinth", Type: "modpack"}, ps)
	tt.Advance(650 * time.Millisecond)
	card, _ := tt.Find("搜索结果 " + ps[0].Title)
	icon, _ := tt.Find("项目封面 " + ps[0].Title)
	textBox := ui.Rect{X: icon.X + icon.W + 5, Y: card.Y, W: card.X + card.W - icon.X - icon.W - 10, H: card.H}
	textPixels := overviewRegionPixels(tt.Image(), textBox)
	img, err := ui.ParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 44 44"><rect width="44" height="44" rx="8" fill="#1677ff"/><path d="M10 22L19 31L34 13" fill="none" stroke="white" stroke-width="5"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	a.images[raw] = img
	tt.Frame()
	tt.Advance(65 * time.Millisecond)
	state := a.discoverMotion.cards[discoverProjectKey(ps[0])]
	if state.iconProgress <= 0 || state.iconProgress >= 1 || state.progress != 1 {
		t.Fatal("late image snapped in or replayed the whole card")
	}
	if !bytes.Equal(textPixels, overviewRegionPixels(tt.Image(), textBox)) {
		t.Fatal("icon arrival animated neighbouring text")
	}
	snapshotUI(t, tt, "discover-icon-reveal")
	tt.Advance(400 * time.Millisecond)
	if state.iconProgress != 1 {
		t.Fatal("image reveal did not finish")
	}
	iconPixels := overviewRegionPixels(tt.Image(), icon)
	tt.Advance(time.Second)
	if !bytes.Equal(iconPixels, overviewRegionPixels(tt.Image(), icon)) {
		t.Fatal("loaded image kept animating")
	}
}

func TestDiscoverReducedMotionAndResponsiveGrid(t *testing.T) {
	for _, size := range [][2]int{{1536, 1024}, {1100, 760}, {900, 540}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			a, tt := discoverMotionTester(t)
			tt.SetSize(size[0], size[1])
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			ps := discoverMotionProjects(0, 7)
			showDiscoverResults(a, tt, launcher.SearchQuery{Source: "modrinth", Type: "modpack"}, ps)
			tt.Frame()
			first, ok := tt.Find("搜索结果 " + ps[0].Title)
			last, _ := tt.Find("搜索结果 " + ps[6].Title)
			second, _ := tt.Find("搜索结果 " + ps[1].Title)
			if !ok || first.W <= 0 || first.X+first.W > float32(size[0]) || first.W != last.W {
				t.Fatalf("grid columns overflowed or the last card stretched: %+v %+v", first, last)
			}
			if first.Y == second.Y && first.X+first.W > second.X {
				t.Fatalf("grid cards overlap: %+v %+v", first, second)
			}
			download, ok := tt.Find("按钮文字 view-0")
			if !ok || download.X < first.X || download.X+download.W > first.X+first.W {
				t.Fatalf("card action escaped its column: %+v in %+v", download, first)
			}
			for _, state := range a.discoverMotion.cards {
				if state.progress != 1 {
					t.Fatal("reduced motion delayed revealing a card")
				}
			}
			still := append([]byte(nil), tt.Image().Pix...)
			tt.Advance(time.Second)
			if !bytes.Equal(still, tt.Image().Pix) {
				t.Fatal("reduced motion still animates search results")
			}
			snapshotUI(t, tt, fmt.Sprintf("discover-responsive-%d", size[0]))
		})
	}
}
