package worldgen

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// nonOverworld lists the vanilla biomes outside the overworld biome source.
var nonOverworld = map[string]bool{"nether_wastes": true, "soul_sand_valley": true, "crimson_forest": true, "warped_forest": true, "basalt_deltas": true, "the_end": true, "small_end_islands": true, "end_midlands": true, "end_highlands": true, "end_barrens": true, "the_void": true}

type sortKey struct {
	id   string
	step int
}

// sortFeatures mirrors FeatureSorter.buildFeaturesPerStep for one biome order.
func sortFeatures(order []string, lists map[string][][]string) [][]string {
	idx := map[sortKey]int{}
	edges := map[sortKey]map[sortKey]bool{}
	steps := 0
	for _, b := range order {
		var seq []sortKey
		steps = max(steps, len(lists[b]))
		for j, step := range lists[b] {
			for _, f := range step {
				k := sortKey{f, j}
				if _, ok := idx[k]; !ok {
					idx[k] = len(idx)
				}
				seq = append(seq, k)
			}
		}
		for i := 1; i < len(seq); i++ {
			if edges[seq[i-1]] == nil {
				edges[seq[i-1]] = map[sortKey]bool{}
			}
			edges[seq[i-1]][seq[i]] = true
		}
	}
	less := func(a, b sortKey) bool {
		if a.step != b.step {
			return a.step < b.step
		}
		return idx[a] < idx[b]
	}
	sorted := func(m map[sortKey]bool) []sortKey {
		var out []sortKey
		for k := range m {
			out = append(out, k)
		}
		sort.Slice(out, func(i, j int) bool { return less(out[i], out[j]) })
		return out
	}
	done := map[sortKey]bool{}
	var out []sortKey
	var dfs func(sortKey)
	dfs = func(n sortKey) {
		if done[n] {
			return
		}
		for _, c := range sorted(edges[n]) {
			dfs(c)
		}
		done[n] = true
		out = append(out, n)
	}
	keys := map[sortKey]bool{}
	for k := range edges {
		keys[k] = true
	}
	for _, k := range sorted(keys) {
		dfs(k)
	}
	res := make([][]string, steps)
	for i := len(out) - 1; i >= 0; i-- {
		res[out[i].step] = append(res[out[i].step], out[i].id)
	}
	return res
}

// TestGenerateOreData regenerates oredata/<version>.json from an extracted
// client jar: ORE_JAR_DATA=<dir containing data/minecraft> ORE_VERSION=1.21.4.
func TestGenerateOreData(t *testing.T) {
	dir, ver := os.Getenv("ORE_JAR_DATA"), os.Getenv("ORE_VERSION")
	if dir == "" || ver == "" {
		t.Skip("set ORE_JAR_DATA and ORE_VERSION")
	}
	wg := filepath.Join(dir, "data/minecraft/worldgen")
	files, _ := filepath.Glob(filepath.Join(wg, "biome/*.json"))
	lists := map[string][][]string{}
	var order []string
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".json")
		if nonOverworld[name] {
			continue
		}
		var b struct{ Features [][]string }
		raw, _ := os.ReadFile(f)
		if err := json.Unmarshal(raw, &b); err != nil {
			t.Fatal(err)
		}
		lists["minecraft:"+name] = b.Features
		order = append(order, "minecraft:"+name)
	}
	base := sortFeatures(order, lists)[OreStep]
	varies := map[string]bool{}
	rng := rand.New(rand.NewSource(1))
	for range 300 {
		o := append([]string(nil), order...)
		rng.Shuffle(len(o), func(i, j int) { o[i], o[j] = o[j], o[i] })
		s := sortFeatures(o, lists)[OreStep]
		for i := range base {
			if s[i] != base[i] {
				varies[s[i]], varies[base[i]] = true, true
			}
		}
	}
	type feat struct {
		ID          string
		Index       int
		Approximate bool `json:",omitempty"`
		Biomes      []string
		Placement   json.RawMessage
		Config      json.RawMessage
	}
	out := struct {
		Version  string
		Features []feat
	}{Version: ver}
	for i, id := range base {
		name := strings.TrimPrefix(id, "minecraft:")
		var placed struct {
			Feature   string
			Placement json.RawMessage
		}
		raw, err := os.ReadFile(filepath.Join(wg, "placed_feature", name+".json"))
		if err != nil || json.Unmarshal(raw, &placed) != nil {
			continue
		}
		cfg, err := os.ReadFile(filepath.Join(wg, "configured_feature", strings.TrimPrefix(placed.Feature, "minecraft:")+".json"))
		if err != nil || !strings.Contains(string(cfg), `"minecraft:ore"`) || !strings.Contains(string(cfg), "_ore\"") {
			continue
		}
		var biomes []string
		for _, b := range order {
			if len(lists[b]) > OreStep && contains(lists[b][OreStep], id) {
				biomes = append(biomes, b)
			}
		}
		var c any
		json.Unmarshal(cfg, &c)
		compact, _ := json.Marshal(c)
		out.Features = append(out.Features, feat{id, i, varies[id], biomes, placed.Placement, compact})
		t.Logf("%2d %s approx=%v biomes=%d", i, id, varies[id], len(biomes))
	}
	b, _ := json.MarshalIndent(out, "", " ")
	if err := os.WriteFile(filepath.Join("oredata", ver+".json"), append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
