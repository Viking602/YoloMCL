package nativeui

import (
	"context"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
	"weak"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestPostedUpdatesRunOnceInOrderWithoutPainting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updates := make(chan func(), 8)
	a := &App{ctx: ctx, events: make(chan func(), 8), updateUI: func(fn func()) { updates <- fn }}
	var applied []int
	for i := range 3 {
		a.post(func() { applied = append(applied, i) })
	}
	if len(applied) != 0 {
		t.Fatal("posting changed UI state before its main-thread callback")
	}
	if len(updates) != 1 {
		t.Fatalf("a burst scheduled %d redraw callbacks instead of one", len(updates))
	}
	// One callback drains the FIFO queue. Later wakeups and frames must not
	// apply a result twice, even when several workers posted before a paint.
	for len(updates) > 0 {
		(<-updates)()
	}
	a.drainEvents()
	if !slices.Equal(applied, []int{0, 1, 2}) || len(a.events) != 0 {
		t.Fatalf("results were reordered, duplicated or held until painting: %v", applied)
	}
	a.post(func() { applied = append(applied, 3) })
	cancel()
	(<-updates)()
	if len(applied) != 3 {
		t.Fatal("a queued result was applied after shutdown")
	}
}

func TestPostedUpdateDuringDrainDoesNotScheduleRedundantRedraw(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updates := make(chan func(), 8)
	a := &App{ctx: ctx, events: make(chan func(), 8), updateUI: func(fn func()) { updates <- fn }}
	var applied []int
	a.post(func() {
		applied = append(applied, 1)
		a.post(func() { applied = append(applied, 2) })
	})
	(<-updates)()
	if !slices.Equal(applied, []int{1, 2}) || len(updates) != 0 || a.uiUpdatePending.Load() {
		t.Fatalf("result posted during a drain was lost or scheduled an extra redraw: %v", applied)
	}
	a.post(func() { applied = append(applied, 3) })
	(<-updates)()
	if !slices.Equal(applied, []int{1, 2, 3}) {
		t.Fatal("the next burst was not applied")
	}
}

func TestPostedUpdatesCoalesceConcurrentWorkersWithoutLosingResults(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updates := make(chan func(), 256)
	a := &App{ctx: ctx, events: make(chan func(), 256), updateUI: func(fn func()) { updates <- fn }}
	const workers, results = 8, 64
	var done sync.WaitGroup
	var next [workers]int
	count := 0
	for worker := range workers {
		done.Go(func() {
			for result := range results {
				a.post(func() {
					if next[worker] != result {
						t.Fatalf("worker %d: result %d, expected %d", worker, result, next[worker])
					}
					next[worker]++
					count++
				})
				runtime.Gosched()
			}
		})
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for count < workers*results {
		select {
		case update := <-updates:
			update()
		case <-timer.C:
			cancel()
			done.Wait()
			t.Fatalf("main-thread wakeup lost: applied %d results", count)
		}
	}
	done.Wait()
	for len(updates) > 0 {
		(<-updates)()
	}
	if a.uiUpdatePending.Load() || len(a.events) != 0 {
		t.Fatal("queue did not return to idle after all results were applied")
	}
}

func TestSnapshotPollingSleepsAndWakesForWork(t *testing.T) {
	s, err := launcher.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	synctest.Test(t, func(t *testing.T) {
		a, err := New(s)
		if err != nil {
			t.Fatal(err)
		}
		defer a.cancel()
		var polls atomic.Int32
		go a.pollSnapshots(func() []string { polls.Add(1); return []string{LangZH} })
		time.Sleep(5 * time.Second)
		if polls.Load() > 4 {
			t.Fatal("idle window kept building snapshots at the active-task rate")
		}
		a.background.Store(true)
		a.wakeSnapshots()
		time.Sleep(3 * time.Second) // drain the change's one coalesced wake
		before := polls.Load()
		time.Sleep(10 * time.Second)
		if polls.Load()-before > 2 {
			t.Fatal("hidden window did not reduce snapshot work")
		}
		account, err := s.AddOffline("CPU_Test")
		if err != nil {
			t.Fatal(err)
		}
		a.post(func() {}) // completed UI action wakes a sleeping poller
		time.Sleep(activeSnapshotInterval)
		for len(a.events) > 0 {
			(<-a.events)()
		}
		if len(a.snapshot.Accounts) != 1 || a.snapshot.Accounts[0].ID != account.ID {
			t.Fatal("background action waited for the idle polling deadline")
		}
		a.cancel()
		synctest.Wait()
	})
}

func TestSnapshotObserver(t *testing.T) {
	var observer snapshotObserver
	snap := launcher.Snapshot{
		ActiveRoot: "root",
		Instances:  []launcher.Instance{{Root: launcher.Root{ID: "root", SelectedVersion: "one"}}},
		Jobs:       []launcher.Job{{ID: "download", State: "running", Steps: map[string]string{"a": "running", "b": "done"}}},
	}
	if !observer.changed(snap, 0) || observer.changed(snap, 0) {
		t.Fatal("unchanged snapshots should be delivered only once")
	}
	// Map insertion order must not cause an idle redraw.
	snap.Jobs[0].Steps = map[string]string{"b": "done", "a": "running"}
	if observer.changed(snap, 0) {
		t.Fatal("map order changed the snapshot")
	}
	for _, mutate := range []func(){
		func() { snap.Jobs[0].BytesDone++ },
		func() { snap.Jobs[0].Steps["a"] = "done" },
		func() { snap.Jobs[0].State = "done" },
		func() { snap.Instances[0].RunningVersions = []string{"one"} },
		func() { snap.Instances[0].SelectedVersion = "two" },
		func() { snap.Settings.MemoryMB = 8192 },
		func() { snap.Instances[0].Pack = &launcher.PackLock{Name: "pack"} },
		func() { snap.Instances[0].PackUpdate = &launcher.PackUpdateInfo{HasUpdate: true} },
		func() { snap.Accounts = []launcher.Account{{ID: "player"}} },
	} {
		mutate()
		if !observer.changed(snap, 0) || observer.changed(snap, 0) {
			t.Fatal("a mutable snapshot change was lost or repeated")
		}
	}
	// A snapshot skipped while a selection was pending must be redelivered
	// after persistence, even if its service data is identical.
	if !observer.changed(snap, 1) || observer.changed(snap, 1) {
		t.Fatal("selection epoch did not invalidate the observer")
	}
}

func TestSnapshotObserverReleasesRemovedPacks(t *testing.T) {
	var observer snapshotObserver
	keep := launcher.Instance{Pack: &launcher.PackLock{Name: "keep"}, PackUpdate: &launcher.PackUpdateInfo{HasUpdate: true}}
	removedPack, removedUpdate := func() (weak.Pointer[launcher.PackLock], weak.Pointer[launcher.PackUpdateInfo]) {
		pack := &launcher.PackLock{Name: "removed"}
		update := &launcher.PackUpdateInfo{HasUpdate: true}
		observer.changed(launcher.Snapshot{Instances: []launcher.Instance{keep, {Pack: pack, PackUpdate: update}}}, 0)
		return weak.Make(pack), weak.Make(update)
	}()
	snap := launcher.Snapshot{Instances: []launcher.Instance{keep}}
	if !observer.changed(snap, 0) || observer.changed(snap, 0) {
		t.Fatal("removing a root must produce exactly one snapshot update")
	}
	runtime.GC()
	if removedPack.Value() != nil || removedUpdate.Value() != nil {
		t.Fatal("observer retained the removed root's pack or update payload")
	}
	observer.changed(launcher.Snapshot{}, 0)
	for _, pack := range observer.packs[:cap(observer.packs)] {
		if pack != nil {
			t.Fatal("empty snapshot retained a pack in the backing array")
		}
	}
	for _, update := range observer.updates[:cap(observer.updates)] {
		if update != nil {
			t.Fatal("empty snapshot retained an update in the backing array")
		}
	}
	runtime.KeepAlive(&observer)
}

func TestIdleViewStopsSchedulingFrames(t *testing.T) {
	s, err := launcher.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.cancel(); a.bgWG.Wait() })
	a.page = "tasks"
	frames := 0
	tt := ui.NewTester(func(c *ui.Context) {
		frames++
		a.View(c)
	}, 1536, 1024)
	tt.Advance(2 * time.Second)
	before := frames
	tt.Advance(5 * time.Second)
	if frames != before {
		t.Fatalf("idle view built %d unnecessary frames", frames-before)
	}
	// Event-driven work still wakes the view and completes its animation.
	a.post(func() { a.notice("Finished") })
	tt.Frame()
	tt.Advance(time.Second)
	if !tt.HasText("Finished") || frames <= before+1 {
		t.Fatal("a worker result did not display and animate")
	}
	before = frames
	tt.Advance(time.Second)
	if frames != before {
		t.Fatal("stationary toast rebuilt the view during its countdown")
	}
	tt.Advance(5 * time.Second)
	before = frames
	tt.Advance(5 * time.Second)
	if a.message != "" || frames != before {
		t.Fatal("toast did not expire and return to idle")
	}
}

func TestStatusSpinnerSchedulesOnlyWhileActive(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	frames, busy := 0, true
	tt := ui.NewTester(func(c *ui.Context) {
		frames++
		if busy {
			statusSpinner(c, 20)
		}
	}, 200, 100)
	before := frames
	tt.Advance(200 * time.Millisecond)
	if frames <= before+1 {
		t.Fatal("busy indicator stopped animating without the global timer")
	}
	busy = false
	tt.Frame()
	tt.Advance(time.Second)
	before = frames
	tt.Advance(time.Second)
	if frames != before {
		t.Fatal("hidden spinner continued redrawing")
	}
	busy = true
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Advance(time.Second)
	before = frames
	tt.Advance(time.Second)
	if frames != before {
		t.Fatal("reduced motion spinner continued animating")
	}
}

func TestDeviceExpiryDoesNotWaitForNextPoll(t *testing.T) {
	a := &App{
		device:         &launcher.DeviceInfo{ID: "expired"},
		deviceUntil:    time.Now().Add(-time.Second),
		nextDevicePoll: time.Now().Add(time.Minute),
	}
	a.pollDevice()
	if a.device != nil || !a.errorMessage || a.message == "" {
		t.Fatal("expired device sign-in would keep scheduling immediate frames")
	}
}
