package launcher

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

func TestListSurfaceStructures(t *testing.T) {
	if os.Getenv("MCL_LIST_SURF") == "" {
		t.Skip()
	}
	s := testService(t)
	root := Root{ID: "se", Path: `D:\MC\.minecraft`, SelectedVersion: "project01"}
	s.cfg.Roots = []Root{root}
	store, err := s.StructureAssetStoreWorld(root.ID, root.SelectedVersion, "Codex_Compat_Regression_REI_20260925")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	files := store.List("data/")
	type def struct {
		Step string `json:"step"`
	}
	var ids []string
	for path := range files {
		// data/<ns>/worldgen/structure/<name>.json
		if !strings.Contains(path, "/worldgen/structure/") || !strings.HasSuffix(path, ".json") {
			continue
		}
		if strings.Contains(path, "/worldgen/structure_set/") || strings.Contains(path, "/worldgen/structure_processor") {
			continue
		}
		b, err := store.Read(path)
		if err != nil {
			continue
		}
		var d def
		if json.Unmarshal(b, &d) != nil {
			continue
		}
		if d.Step != "surface_structures" && d.Step != "" {
			continue
		}
		// empty step might still be surface - skip empty for now; only surface_structures
		if d.Step != "surface_structures" {
			continue
		}
		// path -> id
		rest := strings.TrimPrefix(path, "data/")
		i := strings.Index(rest, "/worldgen/structure/")
		if i < 0 {
			continue
		}
		ns := rest[:i]
		name := strings.TrimSuffix(rest[i+len("/worldgen/structure/"):], ".json")
		if strings.Contains(name, "/") {
			continue // nested templates sometimes
		}
		ids = append(ids, ns+":"+name)
	}
	sort.Strings(ids)
	// unique
	out := ids[:0]
	var prev string
	for _, id := range ids {
		if id != prev {
			out = append(out, id)
			prev = id
		}
	}
	ids = out
	t.Logf("surface_structures count=%d", len(ids))
	for i, id := range ids {
		if strings.Contains(id, "iceandfire") || strings.Contains(id, "desert_pyramid") || strings.Contains(id, "jungle_pyramid") || strings.Contains(id, "stronghold") || strings.Contains(id, "swamp_hut") {
			t.Logf("  [%d] %s", i, id)
		}
	}
	// also dump around fire_dragon
	for i, id := range ids {
		if strings.Contains(id, "fire_dragon_roost") || strings.Contains(id, "siren_island") || strings.Contains(id, "ice_dragon_roost") || strings.Contains(id, "lightning_dragon_roost") {
			t.Logf("TARGET [%d] %s", i, id)
		}
	}
}
