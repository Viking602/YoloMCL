package launcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	"github.com/Viking602/YoloMCL/internal/structure3d"
)

var (
	lateLayoutMu    sync.Mutex
	lateLayoutCache = map[string]bool{}
)

// savedChunkExists reports whether the save holds chunk (cx, cz).
func savedChunkExists(worldDir string, cx, cz int) bool {
	if worldDir == "" {
		return false
	}
	r, err := mcworld.OpenRegion(filepath.Join(worldDir, "region", fmt.Sprintf("r.%d.%d.mca", cx>>5, cz>>5)))
	if err != nil {
		return false
	}
	c, _ := r.Chunk(cx&31, cz&31)
	return c != nil
}

// lateEditsChangeLayout reports whether the pool edits of jars installed
// after the world was created change the layout at (cx, cz), so a chunk
// already generated without a saved start cannot say which applied. The
// store's edits are restored before returning.
func (s *Service) lateEditsChangeLayout(ctx context.Context, m *EraMap, store *structure3d.Store, rootID, versionID, world string, era int, structureID string, cx, cz int) bool {
	info, err := os.Stat(m.worldDir)
	if err != nil {
		return false
	}
	all := store.PoolEdits()
	drop := editSourcesSince(m.jars, all, fileCreated(info))
	if len(drop) == 0 {
		return false
	}
	key := fmt.Sprint(m.worldDir, "|", structureID, "|", cx, ",", cz)
	lateLayoutMu.Lock()
	v, ok := lateLayoutCache[key]
	lateLayoutMu.Unlock()
	if ok {
		return v
	}
	smp, err := s.EraSampler(ctx, rootID, versionID, world, era)
	if err != nil {
		return true
	}
	level, err := mcworld.LevelSeed(m.worldDir)
	if err != nil {
		return true
	}
	const dim = "minecraft:overworld"
	ti, _ := smp.TerrainInfo(dim)
	floor := func(x, z int) int {
		if h, ok := smp.TerrainSurfaceAt(dim, x, z); ok {
			return h + 1
		}
		return 64
	}
	free := func(x, z int) int { return max(floor(x, z), ti.SeaLevel) }
	sig := func() string {
		got, err := structure3d.PlaceJigsawFloor(store, structureID, level, cx, cz, free, floor, structure3d.OverworldHeight)
		if err != nil {
			return "err"
		}
		var b strings.Builder
		for _, p := range got.Pieces {
			fmt.Fprint(&b, p.Location, p.Rotation, p.BB, ";")
		}
		return b.String()
	}
	a := sig()
	store.SetPoolEdits(withoutEditSources(all, drop))
	v = a != sig()
	store.SetPoolEdits(all)
	lateLayoutMu.Lock()
	lateLayoutCache[key] = v
	lateLayoutMu.Unlock()
	return v
}

// editSourcesSince is the pool edit sources of jars created at or after t,
// leaving out updates of a mod family that already had an older jar.
func editSourcesSince(jars []eraJar, edits []structure3d.PoolEdit, t time.Time) []string {
	sources := map[string]bool{}
	for _, e := range edits {
		sources[strings.ToLower(e.Source)] = true
	}
	var older []string
	for _, j := range jars {
		if j.Created.Before(t) {
			older = append(older, eraJarFamily(j.Name))
		}
	}
	var drop []string
jars:
	for _, j := range jars {
		if j.Created.Before(t) {
			continue
		}
		fam := eraJarFamily(j.Name)
		for _, o := range older {
			if len(fam) >= 5 && len(o) >= 5 && (strings.HasPrefix(fam, o) || strings.HasPrefix(o, fam)) {
				continue jars
			}
		}
		k := modKey(j.Name)
		for src := range sources {
			if sk := modKey(src); sk != "" && (sk == k || (len(sk) >= 5 && strings.HasPrefix(k, sk))) {
				drop = append(drop, src)
			}
		}
	}
	sort.Strings(drop)
	return drop
}
