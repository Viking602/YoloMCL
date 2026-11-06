package modjar

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// Uses the installed BOP/mappings when present (MODJAR_BOP_JAR,
// MODJAR_MAPPINGS, MODJAR_SRG); skips otherwise.
func TestReadSurfaceRulesBOP(t *testing.T) {
	jar, maps, srg := os.Getenv("MODJAR_BOP_JAR"), os.Getenv("MODJAR_MAPPINGS"), os.Getenv("MODJAR_SRG")
	if jar == "" || maps == "" || srg == "" {
		t.Skip("BOP jar / mappings not configured")
	}
	f, err := os.Open(maps)
	if err != nil {
		t.Skip(err)
	}
	symbols, err := ReadOfficialSymbols(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	sz, err := zip.OpenReader(srg)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := sz.Open("config/joined.tsrg")
	if err != nil {
		t.Fatal(err)
	}
	if err := symbols.AddSRG(stream); err != nil {
		t.Fatal(err)
	}
	stream.Close()
	sz.Close()
	z, err := zip.OpenReader(jar)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	options := Options{Symbols: symbols, ModID: "biomesoplenty", Loader: "forge"}
	if client := os.Getenv("MODJAR_CLIENT_JAR"); client != "" {
		cz, err := zip.OpenReader(client)
		if err != nil {
			t.Fatal(err)
		}
		defer cz.Close()
		options.Class = func(name string) ([]byte, error) {
			f, err := cz.Open(name + ".class")
			if err != nil {
				return nil, nil
			}
			defer f.Close()
			return io.ReadAll(f)
		}
	}
	sets, err := ReadSurfaceRules(context.Background(), &z.Reader, options)
	if err != nil {
		t.Fatal(err)
	}
	cats := map[string]bool{}
	for _, s := range sets {
		if s.Namespace != "biomesoplenty" {
			t.Fatalf("namespace %q", s.Namespace)
		}
		cats[s.Category] = true
	}
	if !cats["OVERWORLD"] || !cats["NETHER"] {
		t.Fatalf("categories %v", cats)
	}
	files := map[string]bool{}
	for _, e := range z.File {
		files[e.Name] = true
	}
	blocks, biomes, noises := map[string]bool{}, map[string]bool{}, map[string]bool{}
	var walk func(any)
	walk = func(x any) {
		switch v := x.(type) {
		case map[string]any:
			if rs, ok := v["result_state"].(map[string]any); ok {
				blocks[rs["Name"].(string)] = true
			}
			if n, ok := v["noise"].(string); ok {
				noises[n] = true
			}
			if ids, ok := v["biome_is"].([]any); ok {
				for _, id := range ids {
					biomes[id.(string)] = true
				}
			}
			for _, c := range v {
				walk(c)
			}
		case []any:
			for _, c := range v {
				walk(c)
			}
		}
	}
	for _, s := range sets {
		var x any
		if err := json.Unmarshal(s.Rule, &x); err != nil {
			t.Fatal(err)
		}
		walk(x)
		if os.Getenv("MODJAR_DUMP") != "" {
			os.WriteFile(os.Getenv("MODJAR_DUMP")+"-"+strings.ToLower(s.Category)+".json", s.Rule, 0o644)
		}
	}
	// Field-name-derived ids must be real registry entries.
	for id := range blocks {
		ns, path, _ := strings.Cut(id, ":")
		if ns == "biomesoplenty" && !files["assets/biomesoplenty/blockstates/"+path+".json"] {
			t.Errorf("block %s has no blockstate", id)
		}
	}
	lang := ""
	for _, e := range z.File {
		if e.Name == "assets/biomesoplenty/lang/en_us.json" {
			r, _ := e.Open()
			b := make([]byte, e.UncompressedSize64)
			_, _ = io.ReadFull(r, b)
			r.Close()
			lang = string(b)
		}
	}
	for id := range biomes {
		ns, path, _ := strings.Cut(id, ":")
		if ns == "biomesoplenty" && !strings.Contains(lang, `"biome.biomesoplenty.`+path+`"`) {
			t.Errorf("biome %s unknown", id)
		}
		_ = path
	}
	if !blocks["biomesoplenty:origin_grass_block"] || !biomes["biomesoplenty:crag"] || !blocks["minecraft:stone"] {
		t.Fatalf("missing expected ids: blocks %v biomes %v", blocks, biomes)
	}
	if options.Class != nil && !noises["minecraft:surface_swamp"] {
		t.Errorf("noise keys not read from Noises.<clinit>: %v", noises)
	}
	t.Logf("%d sets, %d blocks, %d biomes, noises %v", len(sets), len(blocks), len(biomes), noises)
}
