package launcher

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"math"
	"os"
	"strings"
)

// Tectonic (dev.worldgen.tectonic, 3.x NeoForge) adds its jar's
// resourcepacks/tectonic as a required BUILT_IN data pack at Position.TOP when
// config/tectonic.json general.mod_enabled is true. Its tectonic:config_*
// density functions read that config once; mapAll lowers them to vanilla
// functions, which is what the rewrite below reproduces.
const tectonicEmbeddedSource = "embedded:tectonic:"

// tectonicPackDisabled lets tests compare against the pre-Tectonic graph
// (MCL_TERRAIN_NOTECTONIC).
var tectonicPackDisabled bool

const tectonicPackRoot = "resourcepacks/tectonic/"

// seedMapTectonicPack appends the embedded pack after every other source.
func seedMapTectonicPack(gameDir string, sources []WorldgenSource) []WorldgenSource {
	cfg := readTectonicConfig(gameDir)
	if tectonicPackDisabled || !cfg.bool("general.mod_enabled", true) {
		return sources
	}
	for _, s := range sources {
		name, ok := strings.CutPrefix(s.Name, "mod:")
		if !ok || !strings.Contains(strings.ToLower(name), "tectonic") {
			continue
		}
		z, err := zip.OpenReader(s.Path)
		if err != nil {
			continue
		}
		found := false
		for _, f := range z.File {
			if f.Name == tectonicPackRoot+"pack.mcmeta" {
				found = true
				break
			}
		}
		z.Close()
		if found {
			return append(sources, WorldgenSource{s.Path, tectonicEmbeddedSource + name})
		}
	}
	return sources
}

// embeddedPackFiles re-roots the entries below root so the pack reads like
// a standalone archive.
func embeddedPackFiles(files []*zip.File, root string) []*zip.File {
	var out []*zip.File
	for _, f := range files {
		if rest, ok := strings.CutPrefix(f.Name, root); ok && rest != "" {
			c := *f
			c.Name = rest
			out = append(out, &c)
		}
	}
	return out
}

// tectonicNeoforgeOverlays evaluates the neoforge:overlays conditions that
// Tectonic's pack.mcmeta uses: tectonic:config keys and neoforge:mod_loaded.
func tectonicNeoforgeOverlays(meta []byte, cfg tectonicConfig, hasMod func(string) bool) []string {
	var m struct {
		Overlays struct {
			Entries []struct {
				Directory  string `json:"directory"`
				Conditions []struct {
					Type  string `json:"type"`
					Key   string `json:"key"`
					ModID string `json:"modid"`
				} `json:"neoforge:conditions"`
			} `json:"entries"`
		} `json:"neoforge:overlays"`
	}
	if json.Unmarshal(meta, &m) != nil {
		return nil
	}
	var out []string
	for _, e := range m.Overlays.Entries {
		if !worldgenOverlayDirectory.MatchString(e.Directory) || e.Directory == "." || e.Directory == ".." {
			continue
		}
		ok := true
		for _, c := range e.Conditions {
			switch c.Type {
			case "tectonic:config":
				ok = ok && cfg.test(c.Key)
			case "neoforge:mod_loaded":
				ok = ok && hasMod(c.ModID)
			default:
				ok = false
			}
		}
		if ok {
			out = append(out, e.Directory)
		}
	}
	return out
}

type tectonicConfig map[string]any

func readTectonicConfig(gameDir string) tectonicConfig {
	p, err := safeJoin(gameDir, "config/tectonic.json")
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(p)
	if err != nil || len(data) > 1<<20 {
		return nil
	}
	return parseTectonicConfig(data)
}

// parseTectonicConfig reads Apollib's commented JSON.
func parseTectonicConfig(data []byte) tectonicConfig {
	var b bytes.Buffer
	inStr, esc := false, false
	for i := 0; i < len(data); i++ {
		ch := data[i]
		if inStr {
			b.WriteByte(ch)
			if esc {
				esc = false
			} else if ch == '\\' {
				esc = true
			} else if ch == '"' {
				inStr = false
			}
			continue
		}
		if ch == '"' {
			inStr = true
		} else if ch == '/' && i+1 < len(data) && data[i+1] == '/' {
			for i < len(data) && data[i] != '\n' {
				i++
			}
			b.WriteByte('\n')
			continue
		} else if ch == '/' && i+1 < len(data) && data[i+1] == '*' {
			end := bytes.Index(data[i+2:], []byte("*/"))
			if end < 0 {
				break
			}
			i += end + 3
			continue
		}
		b.WriteByte(ch)
	}
	// Trailing commas.
	clean := b.Bytes()
	var out []byte
	for i := 0; i < len(clean); i++ {
		if clean[i] == ',' {
			j := i + 1
			for j < len(clean) && (clean[j] == ' ' || clean[j] == '\t' || clean[j] == '\r' || clean[j] == '\n') {
				j++
			}
			if j < len(clean) && (clean[j] == '}' || clean[j] == ']') {
				continue
			}
		}
		out = append(out, clean[i])
	}
	var m map[string]any
	if json.Unmarshal(out, &m) != nil {
		return nil
	}
	return m
}

func (c tectonicConfig) get(path string) (any, bool) {
	var cur any = map[string]any(c)
	for _, part := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = m[part]; !ok {
			return nil, false
		}
	}
	return cur, true
}

func (c tectonicConfig) num(path string, def float64) float64 {
	if v, ok := c.get(path); ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return def
}

func (c tectonicConfig) bool(path string, def bool) bool {
	if v, ok := c.get(path); ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// value is ConfigState.getValue; unknown keys are 0 there too.
func (c tectonicConfig) value(key string) float64 {
	switch key {
	case "vertical_scale":
		return c.num("global_terrain.vertical_scale", 1.125)
	case "elevation_boost":
		return c.num("global_terrain.elevation_boost", 0)
	case "min_y":
		return float64(int(c.num("global_terrain.min_y", -64)))
	case "max_y":
		return float64(int(c.num("global_terrain.max_y", 320)))
	case "lava_tunnels":
		return b2f(c.bool("global_terrain.lava_tunnels", true))
	case "ocean_offset":
		return c.num("continents.ocean_offset", -0.8)
	case "alternate_erosion_scale":
		return c.num("continents.erosion_scale", 0.25) * 4
	case "underground_rivers":
		if c.bool("continents.underground_rivers", true) {
			return -1
		}
		return 0
	case "flat_terrain_skew":
		return c.num("continents.flat_terrain_skew", 0.1)
	case "rolling_hills":
		return b2f(c.bool("continents.rolling_hills", true))
	case "jungle_pillars":
		return b2f(c.bool("continents.jungle_pillars", true))
	case "ocean_depth":
		return c.num("oceans.ocean_depth", -0.22)
	case "deep_ocean_depth":
		return c.num("oceans.deep_ocean_depth", -0.45)
	case "depth_cutoff_start":
		return c.num("caves.depth_cutoff_start", 0.1)
	case "depth_cutoff_size":
		return c.num("caves.depth_cutoff_size", 0.1)
	case "cheese_enabled":
		return b2f(c.bool("caves.cheese_enabled", true))
	case "cheese_additive":
		return c.num("caves.cheese_additive", 0.27)
	case "noodle_enabled":
		return b2f(c.bool("caves.noodle_enabled", true))
	case "noodle_additive":
		return c.num("caves.noodle_additive", -0.075)
	}
	return 0
}

// test is ConfigState.test for pack and modifier conditions.
func (c tectonicConfig) test(key string) bool {
	switch key {
	case "disable_islands":
		return !c.bool("islands.enabled", true)
	case "increased_height":
		return c.num("global_terrain.max_y", 320) > 320
	case "ultrasmooth":
		return c.bool("global_terrain.ultrasmooth", false)
	case "remove_frozen_ocean_ice":
		return c.bool("oceans.remove_frozen_ocean_ice", false)
	case "river_lanterns":
		return c.bool("continents.river_lanterns", true)
	case "river_ice":
		return c.bool("continents.river_ice", false)
	case "ore_fix":
		return c.bool("caves.ore_fix", false)
	case "no_carvers":
		return !c.bool("caves.carvers_enabled", true)
	case "improved_jungle_pillars":
		return c.bool("experimental.improved_jungle_pillars", false)
	}
	return false
}

type tectonicNoiseState struct {
	scale, mul, off float64
	smoother        bool
}

// noise is ConfigState.getNoiseState.
func (c tectonicConfig) noise(key string) (tectonicNoiseState, bool) {
	alt := c.bool("experimental.alternate_erosion_scaling", false)
	switch key {
	case "continents":
		return tectonicNoiseState{c.num("continents.continents_scale", 0.13), 1, 0, alt}, true
	case "erosion":
		return tectonicNoiseState{c.num("continents.erosion_scale", 0.25), 1, 0, alt}, true
	case "ridge":
		return tectonicNoiseState{c.num("continents.ridge_scale", 0.25), 1, 0, false}, true
	case "island":
		return tectonicNoiseState{c.num("islands.noise_scale", 0.11), c.num("islands.noise_multiplier", 1), c.num("islands.noise_offset", 0), false}, true
	case "temperature", "vegetation":
		p := "biomes." + key + "_"
		return tectonicNoiseState{c.num(p+"scale", 0.25), c.num(p+"multiplier", 1), c.num(p+"offset", 0), false}, true
	}
	return tectonicNoiseState{}, false
}

// rewriteTectonicDocuments lowers tectonic:* density functions in place.
func rewriteTectonicDocuments(docs map[string]WorldgenDocument, cfg tectonicConfig) (changed int) {
	for key, doc := range docs {
		if !bytes.Contains(doc.Data, []byte(`"tectonic:config_`)) && !bytes.Contains(doc.Data, []byte(`"tectonic:invert"`)) {
			continue
		}
		dec := json.NewDecoder(bytes.NewReader(doc.Data))
		dec.UseNumber()
		var v any
		if dec.Decode(&v) != nil {
			continue
		}
		v = lowerTectonic(v, cfg)
		data, err := json.Marshal(v)
		if err != nil {
			continue
		}
		doc.Data = data
		docs[key] = doc
		changed++
	}
	return changed
}

func lowerTectonic(v any, cfg tectonicConfig) any {
	switch t := v.(type) {
	case []any:
		for i := range t {
			t[i] = lowerTectonic(t[i], cfg)
		}
		return t
	case map[string]any:
		for k, x := range t {
			t[k] = lowerTectonic(x, cfg)
		}
		typ, _ := t["type"].(string)
		switch typ {
		case "tectonic:config_constant":
			key, _ := t["key"].(string)
			return cfg.value(key)
		case "tectonic:config_clamp":
			return map[string]any{"type": "minecraft:min",
				"argument1": map[string]any{"type": "minecraft:max", "argument1": t["input"], "argument2": t["min"]},
				"argument2": t["max"]}
		case "tectonic:config_noise":
			key, _ := t["key"].(string)
			ns, ok := cfg.noise(key)
			if !ok {
				return t
			}
			sx, sz := t["shift_x"], t["shift_z"]
			if ns.smoother {
				// noise((x+sx)*s) == noise(x*s + sx*s) up to rounding (≈).
				sx = map[string]any{"type": "minecraft:mul", "argument1": sx, "argument2": ns.scale}
				sz = map[string]any{"type": "minecraft:mul", "argument1": sz, "argument2": ns.scale}
			}
			var out any = map[string]any{"type": "minecraft:shifted_noise", "noise": t["noise"],
				"xz_scale": ns.scale, "y_scale": 0, "shift_x": sx, "shift_y": 0, "shift_z": sz}
			if ns.mul != 1 {
				out = map[string]any{"type": "minecraft:mul", "argument1": out, "argument2": ns.mul}
			}
			if ns.off != 0 {
				out = map[string]any{"type": "minecraft:add", "argument1": out, "argument2": ns.off}
			}
			return out
		case "tectonic:invert":
			if x, ok := tectonicConst(t["argument"]); ok {
				return 1 / x
			}
		}
		return t
	}
	return v
}

// tectonicConst folds the constant shapes Tectonic passes to invert.
func tectonicConst(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case map[string]any:
		if t["type"] == "minecraft:clamp" {
			x, ok1 := tectonicConst(t["input"])
			lo, ok2 := tectonicConst(t["min"])
			hi, ok3 := tectonicConst(t["max"])
			if ok1 && ok2 && ok3 {
				return math.Min(math.Max(x, lo), hi), true
			}
		}
	}
	return 0, false
}
