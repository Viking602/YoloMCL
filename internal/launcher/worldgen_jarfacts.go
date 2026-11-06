package launcher

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/Viking602/YoloMCL/cubiomes/modjar"
)

// jarFacts memoizes pure per-archive scans (TerraBlender discovery, worldgen
// relevance, declared dependencies). Each fact is a function of the archive
// bytes only, so the key is path+size+mtime; a changed jar misses and is
// rescanned. Failed scans are never stored, so errors behave as before.
type jarFact struct {
	Discovery *modjar.Discovery   `json:",omitempty"`
	Relevant  *bool               `json:",omitempty"`
	Deps      map[string][]string `json:",omitempty"`
}

var jarFacts = struct {
	sync.Mutex
	dir   string
	m     map[string]*jarFact
	dirty bool
}{m: map[string]*jarFact{}}

const jarFactsFile = "jar-facts-v1.json"

// useJarFacts loads the persisted memo for cacheDir once per directory.
func useJarFacts(cacheDir string) {
	jarFacts.Lock()
	defer jarFacts.Unlock()
	if cacheDir == "" || jarFacts.dir == cacheDir {
		return
	}
	jarFacts.dir, jarFacts.m, jarFacts.dirty = cacheDir, map[string]*jarFact{}, false
	if b, err := os.ReadFile(filepath.Join(cacheDir, jarFactsFile)); err == nil && len(b) < 64<<20 {
		var m map[string]*jarFact
		if json.Unmarshal(b, &m) == nil && m != nil {
			jarFacts.m = m
		}
	}
}

// saveJarFacts writes the memo when new facts were learned.
func saveJarFacts() {
	jarFacts.Lock()
	defer jarFacts.Unlock()
	if !jarFacts.dirty || jarFacts.dir == "" {
		return
	}
	if b, err := json.Marshal(jarFacts.m); err == nil && os.MkdirAll(jarFacts.dir, 0o700) == nil {
		if atomicBytes(filepath.Join(jarFacts.dir, jarFactsFile), b, 0600) == nil {
			jarFacts.dirty = false
		}
	}
}

func jarFactKey(path string) string {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return ""
	}
	abs, _ := filepath.Abs(path)
	return abs + "|" + strconv.FormatInt(info.Size(), 10) + "|" + strconv.FormatInt(info.ModTime().UnixNano(), 10)
}

func jarFactUpdate(key string, f func(*jarFact)) {
	if key == "" {
		return
	}
	jarFacts.Lock()
	e := jarFacts.m[key]
	if e == nil {
		e = &jarFact{}
		jarFacts.m[key] = e
	}
	f(e)
	jarFacts.dirty = true
	jarFacts.Unlock()
}

func jarFactGet(key string) (jarFact, bool) {
	if key == "" {
		return jarFact{}, false
	}
	jarFacts.Lock()
	defer jarFacts.Unlock()
	e := jarFacts.m[key]
	if e == nil {
		return jarFact{}, false
	}
	return *e, true
}

// discoverTerraBlenderCached is DiscoverTerraBlender with the memo.
func discoverTerraBlenderCached(ctx context.Context, path string) (modjar.Discovery, error) {
	key := jarFactKey(path)
	if f, ok := jarFactGet(key); ok && f.Discovery != nil {
		return *f.Discovery, nil
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		return modjar.Discovery{}, err
	}
	defer z.Close()
	return discoverTerraBlenderZip(ctx, key, &z.Reader)
}

// discoverTerraBlenderZip runs discovery on an already opened archive.
func discoverTerraBlenderZip(ctx context.Context, key string, z *zip.Reader) (modjar.Discovery, error) {
	if f, ok := jarFactGet(key); ok && f.Discovery != nil {
		return *f.Discovery, nil
	}
	d, err := modjar.DiscoverTerraBlender(ctx, z)
	if err == nil && ctx.Err() == nil {
		jarFactUpdate(key, func(e *jarFact) { e.Discovery = &d })
	}
	return d, err
}
