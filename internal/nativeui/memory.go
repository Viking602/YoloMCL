package nativeui

import (
	"runtime"
	"runtime/debug"
	"time"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// idleMemory runs in the snapshot worker. Rendering and decoding allocate in
// bursts; once they stop, the heap may stay near its previous GC target for
// minutes. Give MyGo time to release its software frame (2 s), then return
// unused Go heap pages to the OS once per burst, never during active tasks.
// This does not trim the process working set or evict live UI/image caches.
type idleMemory struct {
	frame     uint64
	since     time.Time
	checked   bool
	last      time.Time
	allocated uint64
	released  uint64
}

func (m *idleMemory) ready(now time.Time, frame uint64, busy bool) bool {
	if m.since.IsZero() || m.frame != frame || busy {
		m.frame, m.since, m.checked = frame, now, false
		return false
	}
	if m.checked || now.Sub(m.since) < 3*time.Second || (!m.last.IsZero() && now.Sub(m.last) < 10*time.Second) {
		return false
	}
	m.checked = true
	return true
}

func (m *idleMemory) worthwhile(now time.Time, stats runtime.MemStats) bool {
	// Small clicks on an already large live cache do not justify another GC.
	// A large load burst can settle sooner than the normal 30-second cooldown.
	allocated := stats.TotalAlloc - m.allocated
	burst := m.last.IsZero() || allocated >= 32<<20 || (allocated >= 4<<20 && now.Sub(m.last) >= 30*time.Second)
	unused := stats.HeapIdle-stats.HeapReleased >= 16<<20
	// A cheaper first render can finish below the usual 24 MiB threshold
	// while still leaving several MiB of temporary nodes and text layouts.
	// Reclaim that initial allocation burst once, through the same idle gate;
	// subsequent clicks retain the normal thresholds and cooldown.
	startup := m.last.IsZero() && allocated >= 32<<20 && stats.HeapAlloc >= 16<<20
	return startup || (stats.HeapAlloc >= 24<<20 && burst) || (unused && (m.last.IsZero() || now.Sub(m.last) >= 30*time.Second))
}

func (m *idleMemory) reclaim(frame uint64, jobs []launcher.Job, artworkBusy bool, released uint64) {
	busy := artworkBusy
	for _, j := range jobs {
		if j.State == "running" || j.State == "queued" {
			busy = true
			break
		}
	}
	now := time.Now()
	if released != m.released {
		m.checked = false
	}
	if !m.ready(now, frame, busy) {
		return
	}
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	if released != m.released || m.worthwhile(now, stats) {
		// The runtime decommits free pages, including the reclaimed first
		// frame and temporary scan buffers. Do not lower GOGC or GOMEMLIMIT:
		// active imports and installations still need room to work.
		debug.FreeOSMemory()
		m.last, m.allocated = now, stats.TotalAlloc
		m.released = released
	} else if !m.last.IsZero() && now.Sub(m.last) < 30*time.Second && stats.HeapAlloc >= 24<<20 && stats.TotalAlloc-m.allocated >= 4<<20 {
		// A later small redraw can allocate one software frame. Defer its
		// collection to the normal cooldown instead of keeping it forever.
		m.checked = false
	}
}
