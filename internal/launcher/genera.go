package launcher

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Viking602/YoloMCL/internal/mcworld"
)

// A save can hold chunks generated under different mod sets: REI's spawn
// chunks predate Tectonic and Lithostitched (installed together), the rest
// were generated after. Region timestamps are last-save times, so the era of
// a region is read from its terrain instead: the saved OCEAN_FLOOR height
// (feature blocks stripped) is compared with each candidate era's noise
// terrain, and the era that reproduces it wins. See
// .cache/seedmap-fix-20261028/p116-era-detector.md.

// eraWorldgenMods are mods that change generation for every chunk (terrain
// graph, jigsaw engine, vanilla pool files). A pre-era drops all of them.
var eraWorldgenMods = []string{"tectonic", "lithostitched"}

// GenEra is one candidate generation era: the installed mod set minus jars
// whose lower-case file name contains one of Without.
type GenEra struct {
	Name    string   `json:"name"`
	Without []string `json:"without,omitempty"`
}

// EraVerdict is the era a region or chunk was generated in. Approx is set
// when neither candidate reproduces its terrain clearly (mark results ≈).
type EraVerdict struct {
	Era     int   `json:"era"` // index into EraMap.Eras; 0 is the current install
	Approx  bool  `json:"approx,omitempty"`
	Samples int   `json:"n"`
	Exact   []int `json:"exact"` // exact samples per era
}

// EraMap is the per-region / per-chunk era of one world.
type EraMap struct {
	Eras    []GenEra
	Regions map[[2]int]EraVerdict
	Chunks  map[[2]int]EraVerdict // only chunks of mixed regions

	jars     []eraJar // installed jars, for StartStore
	worldDir string
}

// EraAt is the era of chunk (cx, cz). Unsaved chunks use the current install.
func (m *EraMap) EraAt(cx, cz int) EraVerdict {
	if m == nil {
		return EraVerdict{}
	}
	v, ok := m.Chunks[[2]int{cx, cz}]
	if ok && !v.Approx {
		return v
	}
	if r, rok := m.Regions[[2]int{cx >> 5, cz >> 5}]; rok && !ok {
		if !r.Approx {
			return r
		}
		v = r
	} else if !ok {
		return EraVerdict{}
	}
	// Undecided chunk of a mixed region (structure terrain adaptation and
	// player edits hide its noise terrain): eras are generated in connected
	// patches, so take the decided chunks around it.
	for rad := 1; rad <= eraVoteRadius; rad++ {
		votes := make([]int, len(m.Eras))
		total := 0
		for dz := -rad; dz <= rad; dz++ {
			for dx := -rad; dx <= rad; dx++ {
				if n, ok := m.Chunks[[2]int{cx + dx, cz + dz}]; ok && !n.Approx && n.Era < len(votes) {
					votes[n.Era]++
					total++
				}
			}
		}
		if total < 3 {
			continue
		}
		best := 0
		for i := range votes {
			if votes[i] > votes[best] {
				best = i
			}
		}
		out := v
		out.Era = best
		out.Approx = votes[best]*10 < total*9 // under 90% agreement stays ≈
		return out
	}
	return v
}

// eraVoteRadius bounds the neighbourhood an undecided chunk borrows from.
const eraVoteRadius = 4

// Mixed reports whether any region or chunk is not the current era or is ≈.
func (m *EraMap) Mixed() bool {
	if m == nil {
		return false
	}
	for _, v := range m.Regions {
		if v.Era != 0 || v.Approx {
			return true
		}
	}
	return false
}

const (
	eraRegionMinSamples = 64
	eraRegionMinPct     = 85
	eraRegionMinLead    = 30
)

// classifyEra picks the era whose terrain matched; region rules need
// eraRegionMinPct exact and a eraRegionMinLead point lead, chunk rules (4
// samples) need 3 exact against at most 1. ok=false means undecided.
func classifyEra(exact []int, n int, region bool) (int, bool) {
	if n == 0 || len(exact) == 0 {
		return 0, false
	}
	best, second := -1, -1
	for i := range exact {
		if best < 0 || exact[i] > exact[best] {
			best, second = i, best
		} else if second < 0 || exact[i] > exact[second] {
			second = i
		}
	}
	other := 0
	if second >= 0 {
		other = exact[second]
	}
	if region {
		if n < eraRegionMinSamples {
			return 0, false
		}
		pb, po := exact[best]*100/n, other*100/n
		return best, pb >= eraRegionMinPct && pb-po >= eraRegionMinLead
	}
	return best, exact[best]*4 >= 3*n && other*4 <= n
}

// eraExcluded reports whether a mod jar is left out of an era.
func eraExcluded(file string, without []string) bool {
	low := strings.ToLower(file)
	for _, w := range without {
		if strings.Contains(low, w) {
			return true
		}
	}
	return false
}

func withoutEraMods(sources []WorldgenSource, without []string) []WorldgenSource {
	if len(without) == 0 {
		return sources
	}
	out := sources[:0:0]
	for _, s := range sources {
		if name, ok := strings.CutPrefix(s.Name, "mod:"); ok && eraExcluded(name, without) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// eraFeatureBlock is a block above the noise terrain that OCEAN_FLOOR still
// counts (trees, ice, snow layers, plants).
func eraFeatureBlock(name string) bool {
	for _, s := range []string{"_log", "_leaves", "_wood", "_stem", "mushroom_block", "bamboo", "cactus", "vine", "bee_nest", "pumpkin", "melon", "cocoa", "minecraft:air", "cave_air", "snow", "packed_ice", "blue_ice", "minecraft:ice", "minecraft:water", "snow_block", "_hyphae", "mangrove_roots", "pointed_dripstone", "sugar_cane"} {
		if strings.HasSuffix(name, s) || strings.Contains(name, s+"[") {
			return true
		}
	}
	return false
}

// eraSamples are the saved noise-terrain tops of a chunk (4 columns).
func eraSamples(c *mcworld.Chunk) [][3]int {
	if !strings.HasSuffix(c.Status, "full") {
		if st, _ := c.Root["Status"].(string); !strings.HasSuffix(st, "full") {
			return nil
		}
	}
	var out [][3]int
	for _, lz := range []int{3, 11} {
		for _, lx := range []int{3, 11} {
			h, ok := c.Heightmap("OCEAN_FLOOR", lx, lz, -64)
			if !ok {
				continue
			}
			for h > -64 && eraFeatureBlock(c.Block(lx, h-1, lz)) {
				h--
			}
			out = append(out, [3]int{c.X*16 + lx, h, c.Z*16 + lz})
		}
	}
	return out
}

// eraSurface is the noise terrain first-free height of one era.
type eraSurface func(x, z int) (int, bool)

type eraCacheFile struct {
	Eras    []GenEra                  `json:"eras"`
	Regions map[string]eraCacheRegion `json:"regions"`
}

type eraCacheRegion struct {
	Size    int64                 `json:"size"`
	ModTime int64                 `json:"mtime"`
	Verdict EraVerdict            `json:"verdict"`
	Chunks  map[string]EraVerdict `json:"chunks,omitempty"`
}

// detectEras scores every region file of worldDir against the surfaces
// (one per era). cache (may be nil) holds earlier verdicts keyed by region
// size and mtime; the returned file is the updated cache.
func detectEras(ctx context.Context, worldDir string, eras []GenEra, surfaces []eraSurface, cache *eraCacheFile) (*EraMap, *eraCacheFile, error) {
	m := &EraMap{Eras: eras, Regions: map[[2]int]EraVerdict{}, Chunks: map[[2]int]EraVerdict{}}
	next := &eraCacheFile{Eras: eras, Regions: map[string]eraCacheRegion{}}
	if cache != nil && fmt.Sprint(cache.Eras) != fmt.Sprint(eras) {
		cache = nil
	}
	paths, err := mcworld.RegionPaths(worldDir, "")
	if err != nil {
		return nil, nil, err
	}
	score := func(samples [][3]int) ([]int, int) {
		exact := make([]int, len(surfaces))
		n := 0
		for _, s := range samples {
			okAll := true
			hs := make([]int, len(surfaces))
			for i, f := range surfaces {
				h, ok := f(s[0], s[2])
				if !ok {
					okAll = false
					break
				}
				hs[i] = h
			}
			if !okAll {
				continue
			}
			n++
			for i, h := range hs {
				if h == s[1] {
					exact[i]++
				}
			}
		}
		return exact, n
	}
	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		st, err := os.Stat(p)
		if err != nil || st.Size() == 0 {
			continue
		}
		base := filepath.Base(p)
		var rx, rz int
		if _, err := fmt.Sscanf(base, "r.%d.%d.mca", &rx, &rz); err != nil {
			continue
		}
		key := [2]int{rx, rz}
		if cache != nil {
			if c, ok := cache.Regions[base]; ok && c.Size == st.Size() && c.ModTime == st.ModTime().UnixNano() {
				m.Regions[key] = c.Verdict
				for k, v := range c.Chunks {
					var cx, cz int
					if _, err := fmt.Sscanf(k, "%d,%d", &cx, &cz); err == nil {
						m.Chunks[[2]int{cx, cz}] = v
					}
				}
				next.Regions[base] = c
				continue
			}
		}
		r, err := mcworld.OpenRegion(p)
		if err != nil {
			continue
		}
		var all [][3]int
		perChunk := map[[2]int][][3]int{}
		for i := 0; i < 1024; i++ {
			c, _ := r.Chunk(i%32, i/32)
			if c == nil {
				continue
			}
			s := eraSamples(c)
			if len(s) == 0 {
				continue
			}
			perChunk[[2]int{c.X, c.Z}] = s
			if i%4 == 0 { // region score from every 4th chunk
				all = append(all, s...)
			}
		}
		exact, n := score(all)
		era, ok := classifyEra(exact, n, true)
		v := EraVerdict{Era: era, Approx: !ok, Samples: n, Exact: exact}
		entry := eraCacheRegion{Size: st.Size(), ModTime: st.ModTime().UnixNano(), Verdict: v}
		if !ok && len(perChunk) > 0 {
			// Mixed region: decide chunk by chunk.
			entry.Chunks = map[string]EraVerdict{}
			// Region fallback for chunks without a verdict: its plurality
			// era when that leads clearly, else the current install; ≈.
			v.Era, v.Approx = 0, true
			if n > 0 && len(exact) > 1 {
				lo := exact[0]
				for _, e := range exact {
					lo = min(lo, e)
				}
				if (exact[era]-lo)*100 >= eraRegionMinLead*n {
					v.Era = era
				}
			}
			for k, s := range perChunk {
				ce, cn := score(s)
				e, cok := classifyEra(ce, cn, false)
				cv := EraVerdict{Era: e, Approx: !cok, Samples: cn, Exact: ce}
				if !cok {
					cv.Era = 0
				}
				m.Chunks[k] = cv
				entry.Chunks[fmt.Sprintf("%d,%d", k[0], k[1])] = cv
			}
		}
		if n == 0 {
			v = EraVerdict{} // nothing saved to judge: current era
		}
		entry.Verdict = v
		m.Regions[key] = v
		next.Regions[base] = entry
	}
	return m, next, nil
}

// eraJar is an installed mod jar and when it landed in the mods folder.
type eraJar struct {
	Name    string
	Created time.Time
}

// eraCandidates is the current install plus, when a whole-world worldgen mod
// is installed, the era before those mods. The pre-era also leaves out every
// jar added at or after the first worldgen mod (REI: Tsuki, added 09-24,
// appends plains/taiga houses; dropping it makes the pre-era villages
// exact), except a jar whose name family already has an older jar: an
// update of an existing mod (kaleidoscopecookery next to kaleidoscope_*)
// rewrites the file and resets its creation time.
func eraCandidates(jars []eraJar) []GenEra {
	eras := []GenEra{{Name: "current"}}
	var without []string
	var cutoff time.Time
	for _, w := range eraWorldgenMods {
		for _, j := range jars {
			if eraExcluded(j.Name, []string{w}) {
				without = append(without, w)
				if cutoff.IsZero() || j.Created.Before(cutoff) {
					cutoff = j.Created
				}
				break
			}
		}
	}
	if len(without) == 0 {
		return eras
	}
	name := "pre-" + strings.Join(without, "+")
	if !cutoff.IsZero() {
		var older []string
		for _, j := range jars {
			if j.Created.Before(cutoff) {
				older = append(older, eraJarFamily(j.Name))
			}
		}
	jars:
		for _, j := range jars {
			if j.Created.Before(cutoff) || eraExcluded(j.Name, without) {
				continue
			}
			fam := eraJarFamily(j.Name)
			for _, o := range older {
				if len(fam) >= 5 && len(o) >= 5 && (strings.HasPrefix(fam, o) || strings.HasPrefix(o, fam)) {
					continue jars
				}
			}
			without = append(without, strings.ToLower(j.Name))
		}
	}
	return append(eras, GenEra{Name: name, Without: without})
}

// eraJarFamily is the leading letters of a jar name after any "[...]" label:
// "[森罗物语·厨房] kaleidoscopecookery-1.6.0.jar" -> "kaleidoscopecookery".
func eraJarFamily(name string) string {
	if i := strings.LastIndex(name, "]"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.ToLower(strings.TrimSpace(name))
	end := 0
	for end < len(name) && name[end] >= 'a' && name[end] <= 'z' {
		end++
	}
	return name[:end]
}

type eraWorld struct {
	once     sync.Once
	m        *EraMap
	samplers []*LocalSampler
	err      error
}

var (
	eraMu     sync.Mutex
	eraWorlds = map[string]*eraWorld{}
)

// DetectEras returns the era map of a saved world, cached per process and on
// disk under the launcher cache (keyed by region size and mtime). A world
// with one candidate era (no whole-world worldgen mod) maps to era 0.
func (s *Service) DetectEras(ctx context.Context, rootID, versionID, world string) (*EraMap, error) {
	w, err := s.eraWorld(ctx, rootID, versionID, world)
	if err != nil {
		return nil, err
	}
	return w.m, nil
}

// EraSampler is the noise sampler of era i of a world (DetectEras first).
func (s *Service) EraSampler(ctx context.Context, rootID, versionID, world string, era int) (*LocalSampler, error) {
	w, err := s.eraWorld(ctx, rootID, versionID, world)
	if err != nil {
		return nil, err
	}
	if era < 0 || era >= len(w.samplers) {
		return nil, fmt.Errorf("era %d out of range", era)
	}
	return w.samplers[era], nil
}

func (s *Service) eraWorld(ctx context.Context, rootID, versionID, world string) (*eraWorld, error) {
	key := rootID + "|" + versionID + "|" + world
	eraMu.Lock()
	w := eraWorlds[key]
	if w == nil {
		w = &eraWorld{}
		eraWorlds[key] = w
	}
	eraMu.Unlock()
	w.once.Do(func() { w.m, w.samplers, w.err = s.detectWorldEras(ctx, rootID, versionID, world) })
	if w.err != nil {
		eraMu.Lock()
		delete(eraWorlds, key) // retry next time
		eraMu.Unlock()
	}
	return w, w.err
}

func (s *Service) detectWorldEras(ctx context.Context, rootID, versionID, world string) (*EraMap, []*LocalSampler, error) {
	r, err := s.rootAt(rootID, versionID)
	if err != nil {
		return nil, nil, err
	}
	if err := validWorldDir(world); err != nil {
		return nil, nil, err
	}
	gameDir := r.VersionDir()
	worldDir, err := safeJoin(gameDir, "saves/"+world)
	if err != nil {
		return nil, nil, err
	}
	var jars []eraJar
	if ents, err := os.ReadDir(filepath.Join(gameDir, "mods")); err == nil {
		for _, e := range ents {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".jar") {
				if info, err := e.Info(); err == nil {
					jars = append(jars, eraJar{e.Name(), fileCreated(info)})
				}
			}
		}
	}
	eras := eraCandidates(jars)
	var samplers []*LocalSampler
	var surfaces []eraSurface
	const dim = "minecraft:overworld"
	for _, era := range eras {
		snap, level, err := s.seedMapWorldgenEra(ctx, rootID, versionID, world, era.Without)
		if err != nil {
			return nil, nil, err
		}
		smp, err := NewLocalSampler(ctx, snap, level.RandomSeed)
		if err != nil {
			return nil, nil, err
		}
		samplers = append(samplers, smp)
		surfaces = append(surfaces, func(x, z int) (int, bool) {
			h, ok := smp.TerrainSurfaceAt(dim, x, z)
			return h + 1, ok
		})
	}
	if len(eras) == 1 {
		return &EraMap{Eras: eras, Regions: map[[2]int]EraVerdict{}, Chunks: map[[2]int]EraVerdict{}, jars: jars, worldDir: worldDir}, samplers, nil
	}
	sum := sha1.Sum([]byte(gameDir + "|" + world))
	cachePath := filepath.Join(s.dir, "cache", "seedmap", "eras", hex.EncodeToString(sum[:8])+".json")
	var cache *eraCacheFile
	if b, err := os.ReadFile(cachePath); err == nil {
		var c eraCacheFile
		if json.Unmarshal(b, &c) == nil {
			cache = &c
		}
	}
	m, next, err := detectEras(ctx, worldDir, eras, surfaces, cache)
	if err != nil {
		return nil, nil, err
	}
	if b, err := json.Marshal(next); err == nil && s.dir != "" {
		_ = os.MkdirAll(filepath.Dir(cachePath), 0o755)
		_ = os.WriteFile(cachePath, b, 0o644)
	}
	m.jars, m.worldDir = jars, worldDir
	return m, samplers, nil
}
