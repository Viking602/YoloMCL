package launcher

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// KubeJSWorldgen is what an instance's KubeJS scripts do to world
// generation, read statically (no script runs). Static forms map onto data
// a datapack could hold; anything else is listed as Approximate so the map
// labels it ≈ rather than claiming a seed-exact result.
type KubeJSWorldgen struct {
	// DataDir is kubejs/data when present: KubeJS loads it as a datapack,
	// so it layers exactly like one (seed-exact).
	DataDir string
	// DataWorldgen is how many JSON files under kubejs/data mention worldgen,
	// biome_modifier, dimension, or structure paths (already ingested as a pack).
	DataWorldgen int
	// DataKinds counts those JSON files by path kind (structure_set, biome, …).
	DataKinds map[string]int
	// RemovedFeatures are placed-feature ids removed with string literals
	// (removeFeatureById); equivalent to a biome-modifier remove_features.
	RemovedFeatures []string
	// AddedOres counts static addOre/addSpawn calls: ore and spawn changes
	// do not move biomes or structures, but the terrain view lists them.
	AddedOres int
	// Approximate lists "file:line hook" for worldgen hooks this reader
	// cannot turn into data (computed ids, callbacks, unknown events).
	Approximate []string
	Scripts     int
}

var (
	kjsWorldgenHook = regexp.MustCompile(`\b(WorldgenEvents\.\w+|onEvent\(\s*['"]worldgen\.\w+['"]|StartupEvents\.registry\(\s*['"](?:worldgen/|minecraft:worldgen/)[^'"]*['"])`)
	kjsRemoveByID   = regexp.MustCompile(`removeFeatureById\(\s*(?:['"]\w+['"]\s*,\s*)?(\[[^\]]*\]|['"][^'"]+['"])\s*\)`)
	kjsStringLit    = regexp.MustCompile(`['"]([a-z0-9_.-]+(?::[a-z0-9_/.-]+)?)['"]`)
	kjsAddOre       = regexp.MustCompile(`\.(addOre|addSpawn)\s*\(`)
	kjsOtherCall    = regexp.MustCompile(`\.(removeOres|removeSpawns|removeFeature(?:ById)?|addFeature\w*|add\w*Feature|modify\w*|replace\w*)\s*\(`)
)

// ScanKubeJSWorldgen reads gameDir/kubejs. It returns nil when the folder
// is missing or holds no worldgen hooks and no data/ pack.
func ScanKubeJSWorldgen(gameDir string) *KubeJSWorldgen {
	root := filepath.Join(gameDir, "kubejs")
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return nil
	}
	out := &KubeJSWorldgen{}
	if st, err := os.Stat(filepath.Join(root, "data")); err == nil && st.IsDir() {
		out.DataDir = filepath.Join(root, "data")
		out.DataKinds = map[string]int{}
		_ = filepath.WalkDir(out.DataDir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(p), ".json") {
				return nil
			}
			rel, _ := filepath.Rel(out.DataDir, p)
			rel = filepath.ToSlash(rel)
			kind := kubejsDataKind(rel)
			if kind == "" {
				return nil
			}
			out.DataWorldgen++
			out.DataKinds[kind]++
			return nil
		})
	}
	removed := map[string]bool{}
	for _, sub := range []string{"startup_scripts", "server_scripts"} {
		_ = filepath.WalkDir(filepath.Join(root, sub), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(p), ".js") {
				return nil
			}
			info, err := d.Info()
			if err != nil || info.Size() > 1<<20 {
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			out.Scripts++
			src := stripJSComments(string(data))
			if !kjsWorldgenHook.MatchString(src) {
				return nil
			}
			rel, _ := filepath.Rel(gameDir, p)
			rel = filepath.ToSlash(rel)
			// removeFeatureById with literal ids (arrays may span lines).
			for _, m := range kjsRemoveByID.FindAllStringSubmatch(src, -1) {
				for _, id := range kjsStringLit.FindAllStringSubmatch(m[1], -1) {
					v := id[1]
					if !strings.Contains(v, ":") {
						v = "minecraft:" + v
					}
					removed[v] = true
				}
			}
			for i, line := range strings.Split(src, "\n") {
				where := rel + ":" + strconv.Itoa(i+1)
				switch {
				case strings.Contains(line, "removeFeatureById("):
					loc := kjsRemoveByID.FindStringIndex(src[lineStart(src, i):])
					if loc == nil || loc[0] > len(line) {
						out.Approximate = append(out.Approximate, where+" removeFeatureById (computed id)")
					}
				case kjsAddOre.MatchString(line):
					out.AddedOres++
				case kjsOtherCall.MatchString(line):
					out.Approximate = append(out.Approximate, where+" "+kjsOtherCall.FindStringSubmatch(line)[1])
				case kjsWorldgenHook.MatchString(line) && !strings.Contains(line, "WorldgenEvents.add") && !strings.Contains(line, "WorldgenEvents.remove"):
					out.Approximate = append(out.Approximate, where+" "+kjsWorldgenHook.FindString(line))
				}
			}
			return nil
		})
	}
	for id := range removed {
		out.RemovedFeatures = append(out.RemovedFeatures, id)
	}
	sort.Strings(out.RemovedFeatures)
	if out.DataDir == "" && len(out.RemovedFeatures) == 0 && out.AddedOres == 0 && len(out.Approximate) == 0 {
		return nil
	}
	return out
}

// stripJSComments blanks // and /* */ comments, keeping line numbers.
func stripJSComments(s string) string {
	var b strings.Builder
	inStr := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr != 0 {
			b.WriteByte(c)
			if c == '\\' && i+1 < len(s) {
				i++
				b.WriteByte(s[i])
			} else if c == inStr {
				inStr = 0
			}
			continue
		}
		if c == '"' || c == '\'' || c == '`' {
			inStr = c
			b.WriteByte(c)
			continue
		}
		if c == '/' && i+1 < len(s) && s[i+1] == '/' {
			for i < len(s) && s[i] != '\n' {
				i++
			}
			if i < len(s) {
				b.WriteByte('\n')
			}
			continue
		}
		if c == '/' && i+1 < len(s) && s[i+1] == '*' {
			i += 2
			for i+1 < len(s) && !(s[i] == '*' && s[i+1] == '/') {
				if s[i] == '\n' {
					b.WriteByte('\n')
				}
				i++
			}
			i++
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// lineStart is the byte offset of line n (0-based) in s.
func lineStart(s string, n int) int {
	off := 0
	for ; n > 0; n-- {
		i := strings.IndexByte(s[off:], '\n')
		if i < 0 {
			return len(s)
		}
		off += i + 1
	}
	return off
}

// kubejsDataKind classifies a path under kubejs/data/<ns>/... for reporting.
// Empty means not worldgen-related.
func kubejsDataKind(rel string) string {
	low := strings.ToLower(rel)
	switch {
	case strings.Contains(low, "/worldgen/structure_set/"):
		return "structure_set"
	case strings.Contains(low, "/worldgen/structure/"):
		return "structure"
	case strings.Contains(low, "/worldgen/template_pool/"):
		return "template_pool"
	case strings.Contains(low, "/worldgen/biome/"):
		return "biome"
	case strings.Contains(low, "/worldgen/noise_settings/") || strings.Contains(low, "/worldgen/noise/"):
		return "noise"
	case strings.Contains(low, "/worldgen/density_function/"):
		return "density_function"
	case strings.Contains(low, "/worldgen/placed_feature/") || strings.Contains(low, "/worldgen/configured_feature/"):
		return "feature"
	case strings.Contains(low, "/biome_modifier/") || strings.Contains(low, "/forge/biome_modifier/") || strings.Contains(low, "/neoforge/biome_modifier/"):
		return "biome_modifier"
	case strings.Contains(low, "/dimension/") || strings.Contains(low, "/dimension_type/"):
		return "dimension"
	case strings.Contains(low, "/tags/worldgen/"):
		return "worldgen_tag"
	case strings.Contains(low, "/worldgen/"):
		return "worldgen_other"
	default:
		return ""
	}
}

// appendKubeJSWorldgenIssues records what ScanKubeJSWorldgen detected so the
// seed map can label approximate scripts and confirm datapack ingestion.
// Compatible outputs (kubejs/data JSON) are already sourced via
// seedMapInstancePacks; this only reports. Caps issue fan-out.
func appendKubeJSWorldgenIssues(gameDir string, issues *[]WorldgenIssue) {
	k := ScanKubeJSWorldgen(gameDir)
	if k == nil {
		return
	}
	add := func(code, source string) {
		if issues == nil || len(*issues) >= 128 {
			return
		}
		*issues = append(*issues, WorldgenIssue{code, source})
	}
	if k.DataDir != "" {
		parts := make([]string, 0, len(k.DataKinds))
		for kind, n := range k.DataKinds {
			parts = append(parts, kind+"="+strconv.Itoa(n))
		}
		sort.Strings(parts)
		src := strconv.Itoa(k.DataWorldgen) + " docs"
		if len(parts) > 0 {
			src += " [" + strings.Join(parts, ",") + "]"
		}
		add("kubejs_data", src)
	}
	if k.AddedOres > 0 {
		add("kubejs_add_ore", strconv.Itoa(k.AddedOres))
	}
	for _, id := range k.RemovedFeatures {
		add("kubejs_remove_feature", id)
	}
	const approxCap = 24
	for i, a := range k.Approximate {
		if i >= approxCap {
			add("kubejs_approx", strconv.Itoa(len(k.Approximate)-approxCap)+" more")
			break
		}
		add("kubejs_approx", a)
	}
}
