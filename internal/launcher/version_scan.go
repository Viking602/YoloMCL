package launcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// findVersionJSON locates and parses a version's metadata in one read.
func findVersionJSON(root, dirName string) (string, gameVersion, error) {
	parse := func(p string) (gameVersion, error) {
		b, err := os.ReadFile(p)
		if err != nil {
			return gameVersion{}, err
		}
		var gv gameVersion
		err = json.Unmarshal(b, &gv)
		return gv, err
	}
	p, e := safeJoin(root, "versions/"+dirName+"/"+dirName+".json")
	if e == nil {
		if _, err := os.Stat(p); err == nil {
			gv, err := parse(p)
			return p, gv, err
		}
	}
	vDir, e := safeJoin(root, "versions/"+dirName)
	if e != nil {
		return "", gameVersion{}, e
	}
	files, e := os.ReadDir(vDir)
	if e != nil {
		return "", gameVersion{}, e
	}
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(strings.ToLower(f.Name()), ".json") {
			continue
		}
		cand, err := safeJoin(root, "versions/"+dirName+"/"+f.Name())
		if err != nil {
			continue
		}
		if gv, err := parse(cand); err == nil && (gv.MainClass != "" || gv.InheritsFrom != "" || len(gv.Libraries) > 0) {
			return cand, gv, nil
		}
	}
	return "", gameVersion{}, os.ErrNotExist
}
func scanVersions(root string) []VersionSummary {
	versions, _ := scanVersionsWithFiles(root)
	return versions
}

// Return the exact metadata paths with the result so the inventory can watch
// them without enumerating the same version directories a second time.
func scanVersionsWithFiles(root string) ([]VersionSummary, []string) {
	var files []string
	out := []VersionSummary{}
	metadata := map[string]gameVersion{}
	versionsDir, err := safeJoin(root, "versions")
	if err != nil {
		return out, files
	}
	entries, _ := os.ReadDir(versionsDir)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		files = append(files, filepath.Join("versions", entry.Name()))
		p, v, e := findVersionJSON(root, entry.Name())
		if p == "" {
			continue
		}
		if rel, err := filepath.Rel(root, p); err == nil {
			files = append(files, rel)
		}
		if e != nil {
			continue
		}
		metadata[entry.Name()] = v
	}
	// Resolve only local parents: scanning a folder must never fetch or change
	// its version files. Bounds and a per-chain set handle missing/cyclic parents.
	var resolve func(string, map[string]bool) gameVersion
	resolve = func(id string, seen map[string]bool) gameVersion {
		v := metadata[id]
		seen[id] = true
		if parent := v.InheritsFrom; parent != "" && !seen[parent] && len(seen) < 12 {
			if _, present := metadata[parent]; present {
				v = mergeVersion(resolve(parent, seen), v)
			}
		}
		v.ClientVersion = minecraftVersionOf(v)
		return v
	}
	for id := range metadata {
		v := resolve(id, map[string]bool{})
		loader, mc := detectLoader(v), minecraftVersionOf(v)
		out = append(out, VersionSummary{ID: id, Minecraft: mc, Loader: loader, LoaderVersion: loaderVersion(v, loader, mc), Snapshot: isSnapshotID(mc), JavaVersion: v.JavaVersion.MajorVersion})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, files
}

// detectLoader classifies a version JSON. Libraries are checked by
// priority rather than order: NeoForge versions also ship helper libraries
// under net.minecraftforge (e.g. srgutils), so NeoForge must win over Forge.
func detectLoader(v gameVersion) string {
	found := map[string]bool{}
	for _, lib := range v.Libraries {
		switch {
		case strings.HasPrefix(lib.Name, "net.fabricmc:fabric-loader:"):
			found["fabric"] = true
		case strings.HasPrefix(lib.Name, "org.quiltmc:quilt-loader:"):
			found["quilt"] = true
		case strings.HasPrefix(lib.Name, "net.neoforged"):
			found["neoforge"] = true
		case strings.HasPrefix(lib.Name, "net.minecraftforge:"):
			found["forge"] = true
		}
	}
	for _, l := range []string{"neoforge", "quilt", "fabric", "forge"} {
		if found[l] {
			return l
		}
	}
	if strings.HasPrefix(v.MainClass, "net.neoforged.") || versionArgument(v, "--fml.neoForgeVersion", "-Dfml.neoForgeVersion") != "" {
		return "neoforge"
	}
	low := strings.ToLower(v.ID + " " + v.InheritsFrom)
	for _, l := range []string{"neoforge", "forge", "fabric", "quilt"} {
		if strings.Contains(low, l) {
			return l
		}
	}
	return "vanilla"
}
