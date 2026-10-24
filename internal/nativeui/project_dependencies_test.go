package nativeui

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type planTestReply struct {
	items []launcher.PlanItem
	err   error
}
type planTestRequest struct {
	ctx     context.Context
	version string
	emit    func([]launcher.PlanItem)
	reply   chan planTestReply
}

func fakePlans(a *App) <-chan planTestRequest {
	requests := make(chan planTestRequest, 12)
	a.resolvePlan = func(ctx context.Context, root, source, project, version string, emit func([]launcher.PlanItem)) ([]launcher.PlanItem, error) {
		r := planTestRequest{ctx, version, emit, make(chan planTestReply, 1)}
		requests <- r
		// Deliberately permit late completion after cancellation: generation
		// checks must reject it even when the same version has been reopened.
		select {
		case result := <-r.reply:
			return result.items, result.err
		case <-a.ctx.Done():
			return nil, a.ctx.Err()
		}
	}
	return requests
}

func nextPlanRequest(t *testing.T, tt *ui.Tester, requests <-chan planTestRequest) planTestRequest {
	t.Helper()
	var r planTestRequest
	waitUI(t, tt, func() bool {
		select {
		case r = <-requests:
			return true
		default:
			return false
		}
	})
	return r
}

func planFixture(version string, deps int) []launcher.PlanItem {
	items := []launcher.PlanItem{{Key: "modrinth:example", Source: "modrinth", ProjectID: "example", VersionID: version, Version: version, Title: "Example Project", Role: "target", Status: "install"}}
	for i := range deps {
		id := fmt.Sprintf("dep-%s-%d", version, i)
		items = append(items, launcher.PlanItem{Key: "modrinth:" + id, Source: "modrinth", ProjectID: id, VersionID: id, Version: "1.2.3", Title: "Library " + id, Role: "required", Status: "install", Depth: 1})
	}
	return items
}

func TestProjectDependenciesFollowSelectionAndAnimate(t *testing.T) {
	for _, reduced := range []bool{false, true} {
		t.Run(fmt.Sprint(reduced), func(t *testing.T) {
			a, tt := projectFilterTester(t, "modrinth", "mod")
			requests := fakePlans(a)
			tt.SetPreferences(ui.Preferences{ReduceMotion: reduced})
			tt.Advance(350 * time.Millisecond)
			first := nextPlanRequest(t, tt, requests)
			if first.version != "stable" || !tt.HasText("正在检查前置") {
				t.Fatal("initial plan did not match the recommended selection")
			}
			first.emit(planFixture(first.version, 0))
			waitUI(t, tt, func() bool { return a.plan[0].Status == "install" })
			if _, disabled, _ := a.installAction(); !disabled {
				t.Fatal("install enabled before the full plan completed")
			}
			clickUI(t, tt, "v2.1-beta")
			second := nextPlanRequest(t, tt, requests)
			if first.ctx.Err() == nil || second.version != "beta" || !tt.HasText("前置所选版本 v2.1-beta") {
				t.Fatalf("switch did not cancel and replace the previous plan: canceled=%v, selected=%s, request=%s, texts=%v", first.ctx.Err(), a.form.VersionID, second.version, tt.Texts())
			}
			clickUI(t, tt, "v2.0")
			third := nextPlanRequest(t, tt, requests)
			if second.ctx.Err() == nil || third.version != "stable" {
				t.Fatal("rapid reversal kept the wrong version")
			}
			tt.Advance(300 * time.Millisecond)
			before, ok := tt.Find("前置列表")
			if !ok {
				t.Fatal("missing loading placeholder")
			}
			third.emit(planFixture(third.version, 3))
			waitUI(t, tt, func() bool { return len(a.plan) == 4 })
			tt.Advance(70 * time.Millisecond)
			mid, _ := tt.Find("前置列表")
			snapshotUI(t, tt, fmt.Sprintf("project-dependencies-enter-%t", reduced))
			tt.Advance(350 * time.Millisecond)
			end, _ := tt.Find("前置列表")
			if math.Abs(float64(mid.W-end.W)) > .1 || (reduced && math.Abs(float64(mid.H-end.H)) > .1) || (!reduced && (mid.H <= before.H || mid.H >= end.H)) {
				t.Fatalf("dependency growth should animate height without scaling: before=%+v mid=%+v end=%+v reduced=%t", before, mid, end, reduced)
			}
			first.emit(planFixture("obsolete", 1))
			first.reply <- planTestReply{err: errors.New("obsolete request failed")}
			second.reply <- planTestReply{items: planFixture("beta", 1)}
			third.reply <- planTestReply{items: planFixture("stable", 3)}
			a.bgWG.Wait()
			tt.Frame()
			if a.planLoading || a.planErr != "" || len(a.plan) != 4 || a.plan[0].VersionID != "stable" {
				t.Fatalf("late completion overwrote current dependencies: %+v, %s", a.plan, a.planErr)
			}
			if _, disabled, _ := a.installAction(); disabled {
				t.Fatal("finished compatible plan was not installable")
			}
			items, _, _ := a.planSelection()
			if len(items) != 4 || items[0].VersionID != a.form.VersionID {
				t.Fatal("installation does not use displayed version/dependencies")
			}
			snapshotUI(t, tt, fmt.Sprintf("project-dependencies-ready-%t", reduced))

			clickUI(t, tt, "v2.1-beta")
			fourth := nextPlanRequest(t, tt, requests)
			a.projectVersionFilter = "no-result"
			tt.Frame()
			if a.form.VersionID != "" || a.planKey != "" || len(a.plan) != 0 || fourth.ctx.Err() == nil {
				t.Fatal("empty filter kept an obsolete plan")
			}
			fourth.reply <- planTestReply{items: planFixture("beta", 1)}
			a.bgWG.Wait()
			tt.Frame()
			if len(a.plan) > 0 || a.planLoading {
				t.Fatal("late plan returned to the empty filter")
			}
		})
	}
}

func TestProjectDependencyFailureAndInstanceChange(t *testing.T) {
	a, tt := projectFilterTester(t, "modrinth", "mod")
	requests := fakePlans(a)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	first := nextPlanRequest(t, tt, requests)
	first.reply <- planTestReply{err: context.DeadlineExceeded}
	waitUI(t, tt, func() bool { return !a.planLoading })
	if a.planErr == "" {
		t.Fatal("timeout left checking state without a retry")
	}
	clickUI(t, tt, "重试")
	second := nextPlanRequest(t, tt, requests)
	second.reply <- planTestReply{items: planFixture(second.version, 1)}
	waitUI(t, tt, func() bool { return !a.planLoading })
	if a.planErr != "" {
		t.Fatal("retry kept previous error")
	}
	// Two selected game versions in one root still require separate plans.
	a.root().Versions = append(a.root().Versions, launcher.VersionSummary{ID: "other", Minecraft: "1.21.1", Loader: "neoforge"})
	a.root().SelectedVersion = "other"
	a.imageRetry["version:instance:other"] = time.Now().Add(time.Hour)
	third := nextPlanRequest(t, tt, requests)
	if !a.planLoading {
		t.Fatal("changing target version reused the old plan")
	}
	a.closeDialog()
	if third.ctx.Err() == nil {
		t.Fatal("closing did not cancel dependency lookup")
	}
	third.reply <- planTestReply{items: planFixture(third.version, 2)}
	a.bgWG.Wait()
	tt.Frame()
	if len(a.plan) > 1 {
		t.Fatal("closed dialog accepted dependency data")
	}
}

func TestProjectInstallBlocksIncompatibleAndUnresolvedDependencies(t *testing.T) {
	a, tt := projectFilterTester(t, "modrinth", "mod")
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	a.planKey = a.currentPlanKey()
	a.plan = planFixture(a.form.VersionID, 1)
	for _, status := range []string{"none", "resolving", "conflict"} {
		a.plan[1].Status = status
		if _, disabled, _ := a.installAction(); !disabled {
			t.Fatal("install enabled for required dependency", status)
		}
	}
	clickUI(t, tt, "重置筛选")
	clickUI(t, tt, "v3.0")
	if !tt.HasText("此版本不兼容当前实例，请选择匹配的游戏版本和加载器。") {
		t.Fatal("incompatible version has no explanation")
	}
	a.planKey = a.currentPlanKey()
	a.plan = planFixture(a.form.VersionID, 0)
	if _, disabled, _ := a.installAction(); !disabled {
		t.Fatal("different Minecraft/loader became installable")
	}
}
