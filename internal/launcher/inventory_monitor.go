package launcher

import (
	"context"
	"slices"
	"time"
)

// A notification is only a hint: metadata is still validated by the existing
// inventory scanner. Periodic validation also bounds staleness if a filesystem
// drops a notification. Unsupported volumes retain the normal polling path.
const inventoryVerifyInterval = 5 * time.Minute

type inventoryChangeWatch interface {
	changed() (bool, error)
	close()
}

type inventoryWatchState struct {
	watch   inventoryChangeWatch
	pending bool
}

// InventoryMonitor avoids probing every version's metadata while a directory
// is unchanged. Its caller owns it, calls Refresh serially, and closes it when
// the maintenance worker exits. It retains no additional inventory cache.
type InventoryMonitor struct {
	service *Service
	watches map[string]*inventoryWatchState
	open    func(string) inventoryChangeWatch
}

func (s *Service) NewInventoryMonitor() *InventoryMonitor {
	return &InventoryMonitor{service: s, watches: make(map[string]*inventoryWatchState), open: watchInventoryChanges}
}

func (m *InventoryMonitor) Close() {
	for path, state := range m.watches {
		if state.watch != nil {
			state.watch.close()
		}
		delete(m.watches, path)
	}
}

func (m *InventoryMonitor) Refresh(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	s := m.service
	s.mu.Lock()
	roots := slices.Clone(s.cfg.Roots)
	s.mu.Unlock()
	for path, state := range m.watches {
		if !slices.ContainsFunc(roots, func(r Root) bool { return r.Path == path }) {
			if state.watch != nil {
				state.watch.close()
			}
			delete(m.watches, path)
		}
	}
	for _, r := range roots {
		if ctx.Err() != nil {
			return
		}
		state := m.watches[r.Path]
		if state == nil {
			state = &inventoryWatchState{pending: true}
			m.watches[r.Path] = state
		}
		if state.watch != nil {
			changed, err := state.watch.changed()
			state.pending = state.pending || changed || err != nil
			if err != nil {
				state.watch.close()
				state.watch = nil
			}
		}
		if state.watch == nil {
			// Arm before validation, so changes during the scan are not lost.
			state.watch, state.pending = m.open(r.Path), true
		}
		entry := s.inventoryEntry(r.Path)
		entry.mu.Lock()
		checked := entry.checked
		validate := state.pending || entry.dirty || entry.data == nil || time.Since(checked) >= inventoryVerifyInterval
		entry.mu.Unlock()
		if validate {
			_, validated := s.checkInventory(r, entry)
			// An early foreground wake can hit the scan's five-second
			// coalescing window, including a concurrent scan that began
			// before this event. Only our own validation consumes the event.
			state.pending = !validated
		}
	}
}
