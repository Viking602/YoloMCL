package launcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// The open cache maps a fingerprint of every ingest input to the
// content-addressed snapshot WorldgenIngest already writes. The fingerprint
// covers input fields, the size+mtime of every source/library file (whole tree
// for directories) and every file under the config locations a registration
// may read, so any edit to mods, packs or configs misses and re-ingests.
const openCacheVersion = "open-v24" // v24: yung_jigsaw + land_jigsaw + bettermineshafts Y; v23: graveyard/wilden/tlc/lm + heightmap

func worldgenOpenFingerprint(ctx context.Context, input WorldgenInput, statRoots []string) (string, error) {
	h := sha256.New()
	head := struct {
		V, Minecraft, Loader, LoaderVersion, WorldName string
		JavaMajor                                      int
		HasMods, SavedWorld                            bool
		Generators                                     map[string]string
		SavedDimensions                                map[string]json.RawMessage
		EnabledPacks                                   []string
		Issues                                         []WorldgenIssue
		ModConfigs                                     map[string][]byte
		PackFormat                                     *WorldgenPackFormat
		Symbols                                        bool
		ClassPathErr                                   string
	}{openCacheVersion, input.Minecraft, input.Loader, input.LoaderVersion, input.WorldName, input.JavaMajor, input.HasMods, input.SavedWorld, input.Generators, input.SavedDimensions, input.EnabledPacks, input.Issues, input.ModConfigs, input.PackFormat, input.Symbols != nil, fmt.Sprint(input.classPathErr)}
	if b, err := json.Marshal(head); err == nil {
		h.Write(b)
	} else {
		return "", err
	}
	for _, group := range [][]WorldgenSource{input.Sources, input.Libraries} {
		fmt.Fprintf(h, "\ngroup %d\n", len(group))
		for _, s := range group {
			fmt.Fprintf(h, "%s=", s.Name)
			if err := statTree(ctx, h, s.Path); err != nil {
				return "", err
			}
		}
	}
	for _, root := range statRoots {
		fmt.Fprintf(h, "\nroot %s\n", root)
		if err := statTree(ctx, h, root); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func statTree(ctx context.Context, h hash.Hash, root string) error {
	n := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == root && os.IsNotExist(err) {
				fmt.Fprintf(h, "missing\n")
				return nil
			}
			return err
		}
		if n++; n%512 == 0 {
			if e := ctx.Err(); e != nil {
				return e
			}
		}
		if n > 200000 {
			return fmt.Errorf("open cache: tree too large: %s", root)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		fmt.Fprintf(h, "%s|%t|%d|%d\n", strings.ToLower(rel), d.IsDir(), info.Size(), info.ModTime().UnixNano())
		return nil
	})
	return err
}

func openCacheIndex(cacheDir, key string) string {
	return filepath.Join(cacheDir, "open-"+key[:32]+".ref")
}

// lookupOpenCache returns the cached snapshot for key, or nil.
func lookupOpenCache(ctx context.Context, cacheDir, key string) *WorldgenSnapshot {
	if cacheDir == "" || key == "" {
		return nil
	}
	ref, err := os.ReadFile(openCacheIndex(cacheDir, key))
	if err != nil {
		return nil
	}
	gotKey, snapHash, ok := strings.Cut(strings.TrimSpace(string(ref)), " ")
	if !ok || gotKey != key || len(snapHash) != 64 {
		return nil
	}
	p, err := safeJoin(cacheDir, snapHash+".json")
	if err != nil {
		return nil
	}
	snap, err := readWorldgenCache(ctx, p, snapHash)
	if err != nil {
		return nil
	}
	return snap
}

func storeOpenCache(cacheDir, key string, snap *WorldgenSnapshot) {
	if cacheDir == "" || key == "" || snap == nil || len(snap.Hash) != 64 {
		return
	}
	for _, issue := range snap.Issues {
		if issue.Code == "cache_unavailable" {
			return
		}
	}
	_ = atomicBytes(openCacheIndex(cacheDir, key), []byte(key+" "+snap.Hash), 0600)
}
