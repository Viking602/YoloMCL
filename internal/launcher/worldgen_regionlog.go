package launcher

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Viking602/YoloMCL/cubiomes"
)

// terraBlenderRegionLogKey carries TerraBlender's own "Registered region"
// lines from the instance's last launch. Forge runs FMLCommonSetupEvent
// enqueueWork tasks in parallel-dispatch completion order, so the region
// index order is not derivable from the jars; the log records what the game did.
const terraBlenderRegionLogKey = "@terrablender/region-log"

var terraBlenderRegionLine = regexp.MustCompile(`Registered region (\S+) to index (\d+) for type (OVERWORLD|NETHER)`)

func readTerraBlenderRegionLog(root string) []byte {
	f, err := os.Open(filepath.Join(root, "logs", "latest.log"))
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 64<<10), 1<<20)
	for s.Scan() {
		m := terraBlenderRegionLine.FindStringSubmatch(s.Text())
		if m == nil {
			continue
		}
		if m[1] == "minecraft:overworld" && m[2] == "0" {
			out = out[:0] // a later bootstrap in the same log replaces the earlier one
		}
		out = append(out, m[3]+" "+m[1]+" "+m[2])
		if len(out) > 4096 {
			return nil
		}
	}
	if len(out) == 0 {
		return nil
	}
	return []byte(strings.Join(out, "\n") + "\n")
}

// applyTerraBlenderRegionLog reorders regions by the instance's own
// "Registered region X to index N" lines from logs/latest.log. It only applies
// when that log names every region we built (a log from an older mod set is
// ignored). Otherwise the fallback order stays, which is fully deterministic
// and independent of map/goroutine order:
//  1. minecraft base region (index 0);
//  2. regions registered directly in FMLCommonSetupEvent, by provider in
//     forgeOrderRegionPrograms order (Forge ModSorter: AFTER/BEFORE dependency
//     edges, ties by lower-cased jar file name, then mod id), each provider's
//     regions in its own registration order;
//  3. enqueueWork (deferred) regions in that same provider order;
//  4. Quark's Zeta-bus region last.
//
// Forge runs enqueueWork tasks in parallel, so a real game can differ from
// step 3; only the log can tell, hence it wins when complete.
func applyTerraBlenderRegionLog(snapshot *WorldgenSnapshot, dimension cubiomes.Dimension, regions []cubiomes.TerraBlenderRegion) bool {
	data := snapshot.ModConfigs[terraBlenderRegionLogKey]
	if len(data) == 0 {
		return false
	}
	typ := "OVERWORLD"
	if dimension == cubiomes.Nether {
		typ = "NETHER"
	}
	index := map[string]int{}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) != 3 || f[0] != typ {
			continue
		}
		if n, err := strconv.Atoi(f[2]); err == nil {
			index[f[1]] = n
		}
	}
	for _, r := range regions {
		if _, ok := index[r.ID]; !ok {
			return false
		}
	}
	sort.SliceStable(regions, func(a, b int) bool { return index[regions[a].ID] < index[regions[b].ID] })
	for i := range regions {
		regions[i].Index = i
	}
	return true
}

// quarkTerrablenderKey holds the three quark-common.toml values that decide
// Quark's region (the full file exceeds the config read limit). Quark
// registers it from a Zeta-bus enqueueWork task the interpreter does not
// dispatch; javap TerrablenderUndergroundBiomeHandler: registered only when
// the underground biome list is non-empty (Glimmering Weald enabled) and
// terrablenderAddRegion, weight terrablenderRegionWeight; addBiomes is
// addModifiedVanillaOverworldBiomes(no-op) plus the underground biome.
const quarkTerrablenderKey = "@quark/terrablender"

func readQuarkTerrablender(root string) []byte {
	f, err := os.Open(filepath.Join(root, "config", "quark-common.toml"))
	if err != nil {
		return nil
	}
	defer f.Close()
	want := map[string]string{`"Terrablender Add Region"`: "add", `"Terrablender Region Weight"`: "weight", `"Glimmering Weald"`: "weald"}
	var out []string
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 64<<10), 1<<20)
	for n := 0; s.Scan() && n < 200000; n++ {
		k, v, ok := strings.Cut(strings.TrimSpace(s.Text()), "=")
		if !ok {
			continue
		}
		if name, hit := want[strings.TrimSpace(k)]; hit {
			out = append(out, name+"="+strings.TrimSpace(v))
			delete(want, strings.TrimSpace(k))
		}
	}
	if len(out) == 0 {
		return nil
	}
	return []byte(strings.Join(out, "\n") + "\n")
}

// quarkTerrablenderRegion returns Quark's overworld region when Quark is
// installed and its config enables it. Defaults follow Quark's config spec.
func quarkTerrablenderRegion(snapshot *WorldgenSnapshot) (weight int, ok bool) {
	if !worldgenHasMod(snapshot, "quark") {
		return 0, false
	}
	add, weald, weight := true, true, 1
	for _, line := range strings.Split(string(snapshot.ModConfigs[quarkTerrablenderKey]), "\n") {
		k, v, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		switch k {
		case "add":
			add = v == "true"
		case "weald":
			weald = v == "true"
		case "weight":
			if n, err := strconv.Atoi(v); err == nil {
				weight = n
			}
		}
	}
	return weight, add && weald && weight > 0
}
