package launcher

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// Terrain tells generation eras apart only where the terrain changed. Mods
// that only add pool entries (新的世界 (2) @3,-16: plains village made before
// Tsuki was installed on 09-24, after Tectonic) leave the same terrain, so a
// saved jigsaw start picks its mod set from its own pieces: the era store as
// is, then without the code pool edits of the jars added after each install
// batch, newest batch first, until the saved layout is reproduced.

// StartStore is the structure store a start at chunk (cx, cz) was generated
// with, plus whether the layout is known exact: the saved start (if any) is
// reproduced, or there is no save and the chunk's era is certain. The caller
// closes the store.
func (s *Service) StartStore(ctx context.Context, rootID, versionID, world, structureID string, cx, cz int) (*structure3d.Store, bool, error) {
	store, exact, _, err := s.StartStoreEra(ctx, rootID, versionID, world, structureID, cx, cz)
	return store, exact, err
}

// StartStoreEra is StartStore plus the era whose terrain places the start.
// It differs from the chunk's terrain era when the start was saved at the
// structure_starts step before a terrain mod was installed and the chunk's
// terrain was finished after (REI tower_sand @0,26: desert start at Y70 in
// what is now ocean); the saved layout picks the era that reproduces it.
func (s *Service) StartStoreEra(ctx context.Context, rootID, versionID, world, structureID string, cx, cz int) (*structure3d.Store, bool, int, error) {
	m, err := s.DetectEras(ctx, rootID, versionID, world)
	if err != nil {
		return nil, false, 0, err
	}
	ev := m.EraAt(cx, cz)
	key := fmt.Sprint(rootID, "|", versionID, "|", world, "|", structureID, "|", cx, ",", cz)
	startEditMu.Lock()
	if e, ok := startEraCache[key]; ok {
		ev = EraVerdict{Era: e}
	}
	startEditMu.Unlock()
	store, err := s.StructureAssetStoreEra(rootID, versionID, world, m.Eras[ev.Era].Without)
	if err != nil {
		return nil, false, 0, err
	}
	st := savedStart(m.worldDir, structureID, cx, cz)
	if st == nil {
		exact := !ev.Approx || len(m.Eras) == 1
		if exact && savedChunkExists(m.worldDir, cx, cz) {
			// Generated without this start, perhaps before a pool mod that
			// was installed later: unknown which mod set applied.
			exact = !s.lateEditsChangeLayout(ctx, m, store, rootID, versionID, world, ev.Era, structureID, cx, cz)
		}
		return store, exact, ev.Era, nil
	}
	startEditMu.Lock()
	drop, known := startEditCache[key]
	startEditMu.Unlock()
	all := store.PoolEdits()
	if known {
		if drop != nil {
			store.SetPoolEdits(withoutEditSources(all, drop))
		}
		return store, drop != nil, ev.Era, nil
	}
	smp, err := s.EraSampler(ctx, rootID, versionID, world, ev.Era)
	if err != nil {
		return store, false, ev.Era, nil
	}
	level, err := mcworld.LevelSeed(filepath.Dir(filepath.Join(m.worldDir, "level.dat")))
	if err != nil {
		return store, false, ev.Era, nil
	}
	match := func() (bool, error) {
		const dim = "minecraft:overworld"
		info, _ := smp.TerrainInfo(dim)
		floor := func(x, z int) int {
			if h, ok := smp.TerrainSurfaceAt(dim, x, z); ok {
				return h + 1
			}
			return 64
		}
		free := func(x, z int) int { return max(floor(x, z), info.SeaLevel) }
		got, err := structure3d.PlaceJigsawFloor(store, structureID, level, cx, cz, free, floor, structure3d.OverworldHeight)
		if err != nil {
			return false, err
		}
		return len(got.Pieces) == len(st.Pieces) && savedPiecesMatched(*st, got) == len(st.Pieces), nil
	}
	remember := func(d []string) {
		startEditMu.Lock()
		startEditCache[key] = d
		startEditMu.Unlock()
	}
	ok, err := match()
	if err != nil { // not a jigsaw structure: compare the code-built pieces
		e, ok, rebuilt := s.sceneStartEra(ctx, m, rootID, versionID, world, *st, level, ev.Era, store)
		if rebuilt && !ok {
			return store, false, ev.Era, nil // rebuilt, but no era reproduces the saved boxes
		}
		if ok {
			if e != ev.Era {
				store.Close()
				if store, err = s.StructureAssetStoreEra(rootID, versionID, world, m.Eras[e].Without); err != nil {
					return nil, false, 0, err
				}
			}
			return store, true, e, nil
		}
		return store, !ev.Approx || len(m.Eras) == 1, ev.Era, nil
	}
	if ok {
		remember([]string{})
		return store, true, ev.Era, nil
	}
	for _, d := range laterEditSources(m.jars, all) {
		store.SetPoolEdits(withoutEditSources(all, d))
		if ok, _ := match(); ok {
			remember(d)
			return store, true, ev.Era, nil
		}
	}
	store.SetPoolEdits(all)
	for e := range m.Eras {
		if e == ev.Era {
			continue
		}
		alt, err := s.StructureAssetStoreEra(rootID, versionID, world, m.Eras[e].Without)
		if err != nil {
			continue
		}
		asmp, err := s.EraSampler(ctx, rootID, versionID, world, e)
		if err != nil {
			alt.Close()
			continue
		}
		prev, prevSmp := store, smp
		store, smp = alt, asmp
		if ok, _ := match(); ok {
			prev.Close()
			remember([]string{})
			startEditMu.Lock()
			startEraCache[key] = e
			startEditMu.Unlock()
			return store, true, e, nil
		}
		store, smp = prev, prevSmp
		alt.Close()
	}
	remember(nil)
	return store, false, ev.Era, nil
}

var (
	startEditMu    sync.Mutex
	startEditCache = map[string][]string{} // nil: no mod set reproduces the save
	startEraCache  = map[string]int{}      // saved start placed by another era's terrain
)

// savedStart reads the start of structureID saved in chunk (cx, cz).
func savedStart(worldDir, structureID string, cx, cz int) *mcworld.Start {
	if worldDir == "" {
		return nil
	}
	p := filepath.Join(worldDir, "region", fmt.Sprintf("r.%d.%d.mca", cx>>5, cz>>5))
	r, err := mcworld.OpenRegion(p)
	if err != nil {
		return nil
	}
	c, _ := r.Chunk(cx&31, cz&31)
	if c == nil {
		return nil
	}
	for _, st := range c.Starts() {
		if st.ID == structureID && len(st.Pieces) > 0 {
			return &st
		}
	}
	return nil
}

// modKey normalises a jar name or edit source to its letters before the
// version: "touhou_little_maid_spell-1.21.1-1.8.4.jar" and the source
// "touhou_little_maid_spell" both give "touhoulittlemaidspell".
func modKey(name string) string {
	if i := strings.LastIndex(name, "]"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c == '-' || c == '+' || (c >= '0' && c <= '9') {
			break
		}
		if c >= 'a' && c <= 'z' {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// laterEditSources lists, newest install batch first, the edit sources of
// jars added at or after each batch time (jar updates excluded, see
// eraCandidates). Only batches that change the set of sources are kept.
func laterEditSources(jars []eraJar, edits []structure3d.PoolEdit) [][]string {
	sources := map[string]bool{}
	for _, e := range edits {
		sources[strings.ToLower(e.Source)] = true
	}
	var times []time.Time
	seen := map[time.Time]bool{}
	for _, j := range jars {
		t := j.Created.Truncate(time.Minute)
		if !seen[t] {
			seen[t] = true
			times = append(times, t)
		}
	}
	sort.Slice(times, func(i, k int) bool { return times[i].After(times[k]) })
	var out [][]string
	last := ""
	for _, t := range times {
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
		if sig := strings.Join(drop, ","); len(drop) > 0 && sig != last {
			last = sig
			out = append(out, drop)
		}
	}
	return out
}

func withoutEditSources(all []structure3d.PoolEdit, drop []string) []structure3d.PoolEdit {
	var keep []structure3d.PoolEdit
	for _, e := range all {
		skip := false
		for _, d := range drop {
			if strings.EqualFold(e.Source, d) {
				skip = true
			}
		}
		if !skip {
			keep = append(keep, e)
		}
	}
	return keep
}

// savedPiecesMatched counts saved pieces a placed layout reproduces
// (template, rotation and either box).
func savedPiecesMatched(st mcworld.Start, got *structure3d.JigsawLayout) int {
	want := map[string]int{}
	for _, p := range st.Pieces {
		want[fmt.Sprint(p.Template, p.Rotation, p.BB)]++
	}
	ok := 0
	for _, p := range got.Pieces {
		loc := p.Location
		if p.Kind != "single" {
			loc = ""
		}
		for _, bb := range [][6]int{p.BB, p.RawBB} {
			k := fmt.Sprint(loc, structure3d.RotationName(p.Rotation), [6]int32{int32(bb[0]), int32(bb[1]), int32(bb[2]), int32(bb[3]), int32(bb[4]), int32(bb[5])})
			if want[k] > 0 {
				want[k]--
				ok++
				break
			}
		}
	}
	return ok
}
