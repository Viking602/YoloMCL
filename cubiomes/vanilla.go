package cubiomes

import (
	"bytes"
	"compress/gzip"
	"context"
	"embed"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Bundled registry data and parameter trees are read only when a map opens.
// Provenance, format and licenses are in data/README.md.
//
//go:embed data/*.gz
var seedMapVanillaData embed.FS

func seedMapRelease(version string) (minor, patch int, ok bool) {
	p := strings.Split(version, ".")
	if len(p) < 2 || len(p) > 3 || p[0] != "1" {
		return
	}
	for _, part := range p {
		if part == "" {
			return
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return
			}
		}
	}
	minor, _ = strconv.Atoi(p[1])
	if len(p) == 3 {
		patch, _ = strconv.Atoi(p[2])
	}
	last, found := map[int]int{18: 2, 19: 4, 20: 6, 21: 11}[minor]
	ok = found && patch <= last
	return
}

func readSeedMapBuiltin(name string) ([]byte, error) {
	b, err := seedMapVanillaData.ReadFile("data/" + name + ".gz")
	if err != nil {
		return nil, err
	}
	z, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer z.Close()
	return io.ReadAll(io.LimitReader(z, 4<<20))
}

// BuiltinDocuments returns an owned copy of the bundled climate registry subset.
// Unsupported versions return an empty list, allowing explicit custom registries.
func BuiltinDocuments(version string) ([]Document, error) {
	if docs, ok, err := latestBuiltinDocuments(version); ok {
		return docs, err
	}
	if mc, ok := legacyRelease(version); ok {
		return legacyBuiltinDocuments(version, mc), nil
	}
	minor, patch, ok := seedMapRelease(version)
	if !ok {
		return nil, nil
	}
	name := "registries"
	if minor == 18 {
		name = "registries18"
	}
	profile := strconv.Itoa(minor)
	if minor == 21 && patch >= 4 {
		profile = "214"
		if patch >= 5 {
			profile = "215"
		}
	}
	value, _ := builtinRecords.LoadOrStore(name+"/"+profile, &builtinRecord{})
	record := value.(*builtinRecord)
	record.once.Do(func() { record.docs, record.err = loadBuiltinDocuments(name, profile) })
	if record.err != nil {
		return nil, record.err
	}
	out := make([]Document, len(record.docs))
	for i, doc := range record.docs {
		out[i] = doc
		out[i].Source = "builtin:" + version
		out[i].Data = append(json.RawMessage(nil), doc.Data...)
	}
	return out, nil
}

type builtinRecord struct {
	once sync.Once
	docs []Document
	err  error
}

var builtinRecords sync.Map // finite validated registry/biome profiles

func loadBuiltinDocuments(name, profile string) ([]Document, error) {
	b, err := readSeedMapBuiltin(name + ".json")
	if err != nil {
		return nil, err
	}
	var registries map[string]map[string]json.RawMessage
	if err := json.Unmarshal(b, &registries); err != nil {
		return nil, err
	}
	profilesRaw, err := readSeedMapBuiltin("biomes.json")
	if err != nil {
		return nil, err
	}
	var profiles map[string]map[string]map[string]json.RawMessage
	if err := json.Unmarshal(profilesRaw, &profiles); err != nil {
		return nil, err
	}
	for kind, entries := range profiles[profile] {
		registries[kind] = entries
	}
	var out []Document
	for kind, entries := range registries {
		for id, data := range entries {
			out = append(out, Document{ID: seedMapID(id), Kind: kind, Data: data})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kind+"/"+out[i].ID < out[j].Kind+"/"+out[j].ID })
	return out, nil
}

var vanillaTrees sync.Map // finite version/preset trees

func vanillaClimateTree(ctx context.Context, version, preset string) (*climateTree, []string, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if latestRelease(version) {
		return latestVanillaClimateTree(ctx, version, preset)
	}
	minor, patch, ok := seedMapRelease(version)
	if !ok {
		return nil, nil, fmt.Errorf("unverified biome preset version %s", version)
	}
	if preset == "minecraft:nether" {
		key := "nether" + strconv.Itoa(climateTreeOrder(version))
		if value, ok := vanillaTrees.Load(key); ok {
			tree := value.(*climateTree)
			return tree, tree.biomes, nil
		}
		// MultiNoiseBiomeSourceParameterList.Preset.NETHER, in insertion order.
		var entries []map[string]any
		for _, b := range []struct {
			id           string
			t, h, offset float64
		}{{"nether_wastes", 0, 0, 0}, {"soul_sand_valley", 0, -.5, 0}, {"crimson_forest", .4, 0, 0}, {"warped_forest", 0, .5, .375}, {"basalt_deltas", -.5, 0, .175}} {
			entries = append(entries, map[string]any{"biome": "minecraft:" + b.id, "parameters": map[string]float64{"temperature": b.t, "humidity": b.h, "continentalness": 0, "erosion": 0, "depth": 0, "weirdness": 0, "offset": b.offset}})
		}
		b, err := json.Marshal(entries)
		if err != nil {
			return nil, nil, err
		}
		tree, _, err := parseClimateList(ctx, b, climateTreeOrder(version))
		if err != nil {
			return nil, nil, err
		}
		value, _ := vanillaTrees.LoadOrStore(key, tree)
		tree = value.(*climateTree)
		return tree, tree.biomes, nil
	}
	if preset != "minecraft:overworld" {
		return nil, nil, fmt.Errorf("unknown biome preset %s", preset)
	}
	name := "overworld20"
	switch {
	case minor == 18:
		name = "overworld18"
	case minor == 19 && patch <= 2:
		name = "overworld192"
	case minor == 19:
		name = "overworld19"
	case minor == 21 && patch >= 5:
		name = "overworld215"
	case minor == 21 && patch >= 4:
		name = "overworld21"
	}
	if value, ok := vanillaTrees.Load(name); ok {
		tree := value.(*climateTree)
		return tree, tree.biomes, nil
	}
	b, err := readSeedMapBuiltin(name + ".bin")
	if err != nil {
		return nil, nil, err
	}
	tree, _, err := decodeClimateTree(ctx, b, climateTreeOrder(version))
	if err != nil {
		return nil, nil, err
	}
	value, _ := vanillaTrees.LoadOrStore(name, tree)
	tree = value.(*climateTree)
	return tree, tree.biomes, nil
}

func climateTreeOrder(version string) int {
	if latestFloatRelease(version) {
		return 19
	}
	minor, patch, ok := seedMapRelease(version)
	if ok && (minor == 18 || minor == 19 && patch <= 2) {
		return 10
	}
	return 6
}

func decodeClimateTree(ctx context.Context, b []byte, order int) (*climateTree, []string, error) {
	if len(b) < 8 {
		return nil, nil, errors.New("truncated builtin climate tree")
	}
	np, nn := int(binary.LittleEndian.Uint32(b)), int(binary.LittleEndian.Uint32(b[4:]))
	if np > 256 || nn > 20000 || len(b) != 8+np*8+nn*8 {
		return nil, nil, errors.New("invalid builtin climate tree")
	}
	parameters := make([]climateRange, np)
	for i := range parameters {
		parameters[i] = climateRange{int64(int32(binary.LittleEndian.Uint32(b[8+i*8:]))), int64(int32(binary.LittleEndian.Uint32(b[12+i*8:])))}
	}
	b = b[8+np*8:]
	ids, seen := []string{}, map[string]bool{}
	steps := []int{1555, 259, 43, 7, 1, 0}
	if order == 10 {
		steps = []int{1111, 111, 11, 1, 0}
	}
	var decode func(int, int) (*climateNode, error)
	decode = func(index, depth int) (*climateNode, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if index < 0 || index >= nn || depth >= len(steps) {
			return nil, errors.New("invalid builtin tree index")
		}
		word := binary.LittleEndian.Uint64(b[index*8:])
		n := &climateNode{}
		for i := range 6 {
			p := int(word>>(i*8)) & 255
			if p >= len(parameters) {
				return nil, errors.New("invalid builtin parameter")
			}
			n.Space[i] = parameters[p]
		}
		if word>>56 == 255 {
			n.Biome = BiomeName(int(word>>48) & 255)
			if n.Biome == "" {
				return nil, errors.New("unknown builtin biome")
			}
			if !seen[n.Biome] {
				ids = append(ids, n.Biome)
				seen[n.Biome] = true
			}
			return n, nil
		}
		for index+steps[depth] >= nn {
			depth++
		}
		step := steps[depth]
		if step == 0 {
			return nil, errors.New("invalid builtin branch")
		}
		first := int(word >> 48)
		if first <= index {
			return nil, errors.New("cyclic builtin tree")
		}
		for i := 0; i < order && first+i*step < nn; i++ {
			child, err := decode(first+i*step, depth+1)
			if err != nil {
				return nil, err
			}
			n.Children = append(n.Children, child)
		}
		return n, nil
	}
	root, err := decode(0, 0)
	if err != nil {
		return nil, nil, err
	}
	return compactClimateTree(root, ids), ids, nil
}

var vanillaBiomeNames = func() map[int]string {
	names := map[int]string{8: "nether_wastes", 9: "the_end", 40: "small_end_islands", 41: "end_midlands", 42: "end_highlands", 43: "end_barrens", 170: "soul_sand_valley", 171: "crimson_forest", 172: "warped_forest", 173: "basalt_deltas", 0: "ocean", 1: "plains", 2: "desert", 3: "windswept_hills", 4: "forest", 5: "taiga", 6: "swamp", 7: "river", 10: "frozen_ocean", 11: "frozen_river", 12: "snowy_plains", 14: "mushroom_fields", 16: "beach", 21: "jungle", 23: "sparse_jungle", 24: "deep_ocean", 25: "stony_shore", 26: "snowy_beach", 27: "birch_forest", 29: "dark_forest", 30: "snowy_taiga", 32: "old_growth_pine_taiga", 34: "windswept_forest", 35: "savanna", 36: "savanna_plateau", 37: "badlands", 38: "wooded_badlands", 44: "warm_ocean", 45: "lukewarm_ocean", 46: "cold_ocean", 48: "deep_lukewarm_ocean", 49: "deep_cold_ocean", 50: "deep_frozen_ocean", 129: "sunflower_plains", 131: "windswept_gravelly_hills", 132: "flower_forest", 140: "ice_spikes", 155: "old_growth_birch_forest", 160: "old_growth_spruce_taiga", 163: "windswept_savanna", 165: "eroded_badlands", 168: "bamboo_jungle", 174: "dripstone_caves", 175: "lush_caves", 177: "meadow", 178: "grove", 179: "snowy_slopes", 180: "jagged_peaks", 181: "frozen_peaks", 182: "stony_peaks", 183: "deep_dark", 184: "mangrove_swamp", 185: "cherry_grove", 186: "pale_garden"}
	for id, name := range names {
		names[id] = "minecraft:" + name
	}
	return names
}()

// BiomeName maps a cubiomes numeric biome ID to its modern namespaced name.
// Unknown or historical-only IDs return an empty string.
func BiomeName(id int) string { return vanillaBiomeNames[id] }
