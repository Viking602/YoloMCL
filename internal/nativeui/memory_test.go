package nativeui

import (
	"runtime"
	"testing"
	"time"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestIdleMemoryCollectsReleasedCacheOnceAfterWorkSettles(t *testing.T) {
	now := time.Now()
	m := idleMemory{frame: 1, since: now.Add(-time.Minute), checked: true, last: now.Add(-time.Minute)}
	last := m.last
	for _, busy := range []struct {
		jobs   []launcher.Job
		images bool
	}{{jobs: []launcher.Job{{State: "running"}}}, {images: true}} {
		m.reclaim(1, busy.jobs, busy.images, 1)
		if !m.last.Equal(last) || m.released != 0 {
			t.Fatal("cache release forced a collection during active work")
		}
	}
	m.reclaim(2, nil, false, 1)
	if !m.last.Equal(last) {
		t.Fatal("cache release collected during a new animation frame")
	}
	m.since = time.Now().Add(-4 * time.Second)
	m.reclaim(2, nil, false, 1)
	if m.last.Equal(last) || m.released != 1 {
		t.Fatal("idle maintenance missed released caches without a new allocation burst")
	}
	last = m.last
	m.reclaim(2, nil, false, 1)
	if !m.last.Equal(last) {
		t.Fatal("a single release caused repeated collections")
	}
}

func TestIdleMemoryWaitsForQuiescence(t *testing.T) {
	var m idleMemory
	now := time.Now()
	if m.ready(now, 1, false) || m.ready(now.Add(2*time.Second), 1, false) {
		t.Fatal("reclaimed before the renderer could release its frame")
	}
	if m.ready(now.Add(3*time.Second), 2, false) {
		t.Fatal("reclaimed during a new frame")
	}
	if !m.ready(now.Add(7*time.Second), 2, false) {
		t.Fatal("did not reclaim after the UI settled")
	}
	if m.ready(now.Add(time.Hour), 2, false) {
		t.Fatal("an idle window caused repeated collections")
	}
}

func TestIdleMemoryDefersForTasksAndRateLimitsBursts(t *testing.T) {
	var m idleMemory
	now := time.Now()
	m.ready(now, 1, false)
	if !m.ready(now.Add(4*time.Second), 1, false) {
		t.Fatal("first idle period was missed")
	}
	m.last = now.Add(4 * time.Second)
	m.ready(now.Add(5*time.Second), 2, false)
	if m.ready(now.Add(9*time.Second), 2, false) {
		t.Fatal("small interactions caused frequent collections")
	}
	if m.ready(now.Add(35*time.Second), 2, true) || m.ready(now.Add(36*time.Second), 2, false) {
		t.Fatal("active work did not postpone reclamation")
	}
	if !m.ready(now.Add(39*time.Second), 2, false) {
		t.Fatal("task completion did not start a new idle period")
	}
}

func TestIdleMemorySkipsSmallBurstsAndReclaimsLargeLoads(t *testing.T) {
	now := time.Now()
	m := idleMemory{last: now.Add(-12 * time.Second), allocated: 100 << 20}
	stats := runtime.MemStats{HeapAlloc: 40 << 20, TotalAlloc: 101 << 20}
	if m.worthwhile(now, stats) {
		t.Fatal("a small click on a large live cache triggered another collection")
	}
	stats.TotalAlloc += 4 << 20
	if m.worthwhile(now, stats) || !m.worthwhile(now.Add(20*time.Second), stats) {
		t.Fatal("a later software redraw did not use the normal cooldown")
	}
	stats.TotalAlloc += 32 << 20
	if !m.worthwhile(now, stats) {
		t.Fatal("a settled load burst waited for the full 30-second cooldown")
	}
	stats.HeapAlloc = 8 << 20
	if m.worthwhile(now, stats) {
		t.Fatal("already reclaimed transient data triggered another collection")
	}
	stats.HeapIdle = 20 << 20
	if m.worthwhile(now, stats) || !m.worthwhile(now.Add(20*time.Second), stats) {
		t.Fatal("unused pages did not observe the normal cooldown")
	}
}

func TestIdleMemoryReclaimsSmallerStartupBurstOnce(t *testing.T) {
	now := time.Now()
	var m idleMemory
	// The optimized overview allocates less, so startup can leave about
	// 22 MiB on the heap without ever reaching the old 24 MiB idle trigger.
	stats := runtime.MemStats{HeapAlloc: 22 << 20, TotalAlloc: 36 << 20}
	if !m.worthwhile(now, stats) {
		t.Fatal("a smaller startup retained its temporary rendering allocations")
	}
	m.last, m.allocated = now, stats.TotalAlloc
	stats.TotalAlloc += 4 << 20
	if m.worthwhile(now.Add(time.Minute), stats) {
		t.Fatal("startup reclamation lowered the threshold for later small clicks")
	}
	m = idleMemory{}
	stats = runtime.MemStats{HeapAlloc: 8 << 20, TotalAlloc: 36 << 20}
	if m.worthwhile(now, stats) {
		t.Fatal("an already small startup heap triggered another collection")
	}
}
