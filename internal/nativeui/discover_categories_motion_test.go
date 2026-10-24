package nativeui

import (
	"bytes"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func seedDiscoverCategories(a *App) {
	if a.catOpts == nil {
		a.catOpts = map[string][]launcher.Category{}
	}
	var cats []launcher.Category
	for _, name := range []string{"adventure", "technology", "combat", "kitchen-sink", "optimization", "magic", "multiplayer", "quests"} {
		cats = append(cats, launcher.Category{ID: name, Name: name, Slug: name})
	}
	a.catOpts[a.catKey()] = cats
}

func TestDiscoverCategoriesWaitForVisibleContent(t *testing.T) {
	a, tt := discoverMotionTester(t)
	seedDiscoverCategories(a)
	tt.Frame()
	tt.Advance(2 * time.Second)
	m := &a.discoverMotion.categories
	if m.contentReady || m.progress != 0 {
		t.Fatal("cached categories appeared before the request completed")
	}
	chip, ok := tt.Find(a.catLabel("optimization"))
	if !ok {
		t.Fatal("hidden categories did not reserve their layout")
	}
	tt.ClickAt(chip.X+chip.W/2, chip.Y+chip.H/2)
	if len(a.selectedCats()) != 0 {
		t.Fatal("invisible category accepted a click")
	}
	filters, _ := tt.Find("来源")
	filterPixels := overviewRegionPixels(tt.Image(), filters)
	snapshotUI(t, tt, "categories-waiting-for-content")
	ps := discoverMotionProjects(0, 18)
	showDiscoverResults(a, tt, launcher.SearchQuery{Source: "modrinth", Type: "modpack"}, ps)
	tt.Advance(400 * time.Millisecond)
	if m.contentReady || m.progress != 0 || !a.discoverMotion.revealing {
		t.Fatal("categories overtook the visible cards' entrance")
	}
	tt.Advance(240 * time.Millisecond)
	if !m.contentReady || m.progress <= 0 || m.progress >= 1 {
		t.Fatalf("categories did not ease in after the cards: %+v", m)
	}
	mid, _ := tt.Find("发现分类")
	midPixels := overviewRegionPixels(tt.Image(), mid)
	card, _ := tt.Find("搜索结果 " + ps[0].Title)
	cardPixels := overviewRegionPixels(tt.Image(), card)
	snapshotUI(t, tt, "categories-entering")
	tt.Advance(350 * time.Millisecond)
	end, _ := tt.Find("发现分类")
	if m.progress != 1 || mid.X != end.X || math.Abs(float64(mid.W-end.W)) > .1 || math.Abs(float64(mid.H-end.H)) > .1 || mid.Y <= end.Y || mid.Y-end.Y > 8 {
		t.Fatalf("categories did not lift into place at native size: mid=%+v end=%+v progress=%v", mid, end, m.progress)
	}
	if bytes.Equal(midPixels, overviewRegionPixels(tt.Image(), end)) {
		t.Fatal("category appearance did not animate")
	}
	if !bytes.Equal(filterPixels, overviewRegionPixels(tt.Image(), filters)) || !bytes.Equal(cardPixels, overviewRegionPixels(tt.Image(), card)) {
		t.Fatal("category entrance moved or repainted surrounding content")
	}
	if a.discoverMotion.cards[discoverProjectKey(ps[17])].started {
		t.Fatal("category readiness forced offscreen cards to animate")
	}
	snapshotUI(t, tt, "categories-visible")
	clickUI(t, tt, a.catLabel("optimization"))
	if !a.catSelected("optimization") {
		t.Fatal("revealed category is not interactive")
	}
	a.searchLoading = true
	tt.Frame()
	tt.Advance(150 * time.Millisecond)
	if m.progress != 1 || !m.contentReady {
		t.Fatal("filtering within the same resource type hid the categories again")
	}
}

func TestDiscoverCategoriesSwitchScopesAndIgnoreStaleResults(t *testing.T) {
	a, tt := discoverMotionTester(t)
	for _, source := range []string{"modrinth", "curseforge"} {
		for _, kind := range []string{"modpack", "mod", "resourcepack", "shader"} {
			a.provider, a.resourceType, a.searchLoading = source, kind, true
			seedDiscoverCategories(a)
			tt.Frame()
			tt.Advance(700 * time.Millisecond)
			m := &a.discoverMotion.categories
			if m.key != source+"|"+kind || m.contentReady || m.progress != 0 {
				t.Fatalf("cached categories leaked during %s/%s switch: %+v", source, kind, m)
			}
			// A stale/retained result must not unlock the new scope.
			a.searchLoading = false
			tt.Frame()
			if a.discoverMotion.hasQuery && m.contentReady {
				t.Fatal("the previous result unlocked categories for the new resource type")
			}
			ps := discoverMotionProjects(30, 3)
			for i := range ps {
				ps[i].Source, ps[i].Type = source, kind
			}
			showDiscoverResults(a, tt, launcher.SearchQuery{Source: source, Type: kind}, ps)
			tt.Advance(time.Second)
			if !m.contentReady || m.progress != 1 {
				t.Fatalf("new scope did not reveal its categories: %s/%s %+v", source, kind, m)
			}
		}
	}
	// Several fast switches must leave only the final scope ready to enter.
	for _, kind := range []string{"mod", "shader", "modpack"} {
		a.resourceType, a.searchLoading = kind, true
		tt.Frame()
		tt.Advance(35 * time.Millisecond)
	}
	if m := a.discoverMotion.categories; m.key != "curseforge|modpack" || m.contentReady || m.progress != 0 {
		t.Fatalf("rapid switching retained a previous category animation: %+v", m)
	}
}

func TestDiscoverCategoriesLateEmptyAndReducedMotion(t *testing.T) {
	for _, outcome := range []string{"late-categories", "empty-results", "failed-results", "reduced-motion"} {
		t.Run(outcome, func(t *testing.T) {
			a, tt := discoverMotionTester(t)
			if outcome == "reduced-motion" {
				tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			}
			if outcome != "late-categories" {
				seedDiscoverCategories(a)
			}
			tt.Frame()
			if a.discoverMotion.categories.progress != 0 {
				t.Fatal("categories appeared while the data was still loading")
			}
			ps := discoverMotionProjects(0, 3)
			if outcome == "empty-results" || outcome == "failed-results" {
				ps = nil
			}
			showDiscoverResults(a, tt, launcher.SearchQuery{Source: "modrinth", Type: "modpack"}, ps)
			if outcome == "failed-results" {
				a.discoverMotion.hasQuery = false
				a.discoverMotion.categories.contentReady = false
				a.searchError = "offline"
				tt.Frame()
			}
			if outcome == "reduced-motion" {
				tt.Frame()
				if a.discoverMotion.categories.progress != 1 {
					t.Fatal("reduced motion delayed categories after content became ready")
				}
			}
			tt.Advance(time.Second)
			if outcome == "late-categories" {
				seedDiscoverCategories(a)
				tt.Frame()
				tt.Advance(70 * time.Millisecond)
				if p := a.discoverMotion.categories.progress; p <= 0 || p >= 1 {
					t.Fatal("late category data popped in without an entrance")
				}
				tt.Advance(300 * time.Millisecond)
			}
			if !a.discoverMotion.categories.contentReady || a.discoverMotion.categories.progress != 1 {
				t.Fatal("settled empty/error results or late categories stayed hidden")
			}
		})
	}
}

func TestDiscoverCategoriesWaitOnCachedPageReturn(t *testing.T) {
	a, tt := homeTester(t, LangZH)
	a.bgWG.Wait()
	tt.Frame()
	a.service = nil
	a.provider, a.resourceType = "modrinth", "modpack"
	seedDiscoverCategories(a)
	a.acceptDiscoverResults(launcher.SearchQuery{Source: "modrinth", Type: "modpack"}, launcher.SearchResult{Projects: discoverMotionProjects(0, 3), Total: 3})
	a.goPage("discover")
	tt.Frame()
	tt.Advance(time.Second)
	if a.discoverMotion.categories.progress != 1 {
		t.Fatal("initial discovery never finished revealing")
	}
	a.goPage("tasks")
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	a.goPage("discover")
	tt.Frame()
	tt.Advance(150 * time.Millisecond)
	if a.discoverMotion.categories.contentReady || a.discoverMotion.categories.progress != 0 {
		t.Fatal("cached categories overtook the page transition")
	}
	tt.Advance(150 * time.Millisecond)
	if p := a.discoverMotion.categories.progress; p <= 0 || p >= 1 {
		t.Fatal("cached categories did not animate after the page transition")
	}
	snapshotUI(t, tt, "categories-cached-page-return")
	tt.Advance(300 * time.Millisecond)
	if a.discoverMotion.categories.progress != 1 {
		t.Fatal("cached page entrance never finished")
	}
}
