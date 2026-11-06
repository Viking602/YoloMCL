package launcher

import (
	"context"
	"os"
	"sort"
	"strings"
)

func worldgenHasMod(snapshot *WorldgenSnapshot, id string) bool {
	for _, mod := range snapshot.Mods {
		if mod.ID == id {
			return true
		}
	}
	return false
}

// KubeJS injects file resources at reload time; they are not listed in
// level.dat's DataPacks.Enabled. Only an installed KubeJS mod activates these
// sources (checked during ingestion). Leftover instance directories do not.
func seedMapInstancePacks(ctx context.Context, gameDir string, sources []WorldgenSource, issues *[]WorldgenIssue) []WorldgenSource {
	root, err := safeJoin(gameDir, "kubejs")
	if err != nil {
		*issues = append(*issues, WorldgenIssue{"source_unreadable", "kubejs"})
		return sources
	}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return sources
	} else if err != nil {
		*issues = append(*issues, WorldgenIssue{"source_unreadable", "kubejs"})
		return sources
	}
	entries, err := seedMapPackEntries(ctx, gameDir, "kubejs/data")
	if err != nil {
		*issues = append(*issues, WorldgenIssue{"source_unreadable", "kubejs/data"})
	}
	// KubeFileResourcePack.scanAndLoad / ServerScriptManager: ZIP files in
	// kubejs/data are ordered case-insensitively; loose files override those ZIPs.
	sort.SliceStable(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})
	var packs []WorldgenSource
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".zip") {
			continue
		}
		path, err := safeJoin(gameDir, "kubejs/data/"+entry.Name())
		if err != nil {
			*issues = append(*issues, WorldgenIssue{"source_unreadable", "kubejs/data/" + entry.Name()})
			continue
		}
		packs = append(packs, WorldgenSource{path, "kubejs:zip:" + entry.Name()})
	}
	packs = append(packs, WorldgenSource{root, "kubejs:data"})
	// 2101.7.2 inserts its BEFORE_MODS pack before inserting at the saved
	// findAfterModsIndex. Consequently file packs precede the last enabled
	// FilePackResources ZIP. Mirror that ordering, not filesystem name order.
	at := len(sources)
	for i, source := range sources {
		if strings.HasPrefix(source.Name, "datapack:") && strings.HasSuffix(strings.ToLower(source.Path), ".zip") {
			at = i
		}
	}
	out := make([]WorldgenSource, 0, len(sources)+len(packs))
	out = append(out, sources[:at]...)
	out = append(out, packs...)
	return append(out, sources[at:]...)
}
