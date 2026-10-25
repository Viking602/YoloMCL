package launcher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSharedModCacheReusesContentAcrossInstances(t *testing.T) {
	s := testService(t)
	first, second := newTestRoot(t, s, "first"), newTestRoot(t, s, "second")
	testJar(t, filepath.Join(first.Path, "mods", "original.jar"), "example", "Example", map[string][]byte{"inner.jar": nestedJar("inner")})
	one, err := s.ListMods(first.ID)
	if err != nil || len(one) != 1 {
		t.Fatal(one, err)
	}
	data, err := os.ReadFile(filepath.Join(first.Path, "mods", "original.jar"))
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(second.Path, "mods", "renamed.jar.disabled"), data)
	// Occupy both parsers: a cross-instance cache hit must not enter ZIP
	// parsing, including nested libraries. Hashing the new file is still required.
	w := beginModScan()
	w.metadata <- struct{}{}
	w.metadata <- struct{}{}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	two, err := s.ListModsStreamContext(ctx, second.ID, "", nil)
	cancel()
	<-w.metadata
	<-w.metadata
	endModScan()
	if err != nil || len(two) != 1 {
		t.Fatal(two, err)
	}
	if two[0].SHA1 != one[0].SHA1 || two[0].Fingerprint != one[0].Fingerprint || two[0].ModID != "example" || two[0].Name != "Example" || two[0].Enabled || len(two[0].Provides) != 1 {
		t.Fatalf("wrong shared result: %+v", two[0])
	}
	c := s.modMetadataCache()
	if c.file != nil || c.entries != nil || c.users != 0 {
		t.Fatal("scan retained shared directory or file handle")
	}
	if !loadModCacheData(second.Path).Shared {
		t.Fatal("shared cache marker missing")
	}
	// Content changes invalidate the SHA lookup even with an identical name.
	testJar(t, filepath.Join(second.Path, "mods", "renamed.jar.disabled"), "changed", "Changed", nil)
	two, err = s.ListMods(second.ID)
	if err != nil || two[0].ModID != "changed" {
		t.Fatal(two, err)
	}
}

func cacheFixture(i int, description string) modCacheEntry {
	return modCacheEntry{SHA1: fmt.Sprintf("%040x", i+1), Size: int64(i + 100), HasFP: true, FP: uint32(i + 1), Meta: Mod{ModID: fmt.Sprint(i), Name: "Cached", Description: description}}
}

func TestSharedModCacheBoundsAndRecovery(t *testing.T) {
	ctx := context.Background()
	s := testService(t)
	c := s.modMetadataCache()
	session := c.begin(ctx)
	for i := 0; i < sharedModCacheEntries+1; i++ {
		session.put(ctx, cacheFixture(i, ""))
	}
	if len(c.entries) > sharedModCacheEntries {
		t.Fatal("entry limit exceeded")
	}
	if _, ok := session.lookup(ctx, cacheFixture(0, "").SHA1, 100); ok {
		t.Fatal("oldest entry not evicted")
	}
	latest := cacheFixture(sharedModCacheEntries, "")
	if _, ok := session.lookup(ctx, latest.SHA1, latest.Size); !ok {
		t.Fatal("newest entry missing after compaction")
	}
	if _, ok := session.lookup(ctx, latest.SHA1, latest.Size+1); ok {
		t.Fatal("size mismatch accepted")
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	if !c.compact(ctx) {
		t.Fatal("compaction failed")
	}
	runtime.ReadMemStats(&after)
	if after.TotalAlloc-before.TotalAlloc > 4<<20 {
		t.Fatal("compaction allocated a transfer buffer per record")
	}
	// Reach the byte budget independently of the entry budget.
	for i := 0; i < 160; i++ {
		session.put(ctx, cacheFixture(i+10000, strings.Repeat("x", 60<<10)))
	}
	st, err := c.file.Stat()
	if err != nil || st.Size() > sharedModCacheBytes {
		t.Fatal("byte limit exceeded", st, err)
	}
	last := cacheFixture(10159, "")
	if _, ok := session.lookup(ctx, last.SHA1, last.Size); !ok {
		t.Fatal("newest large entry missing")
	}
	session.close()
	// Simulate interruption halfway through an appended header.
	f, err := os.OpenFile(c.path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Write([]byte("torn"))
	_ = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	session = c.begin(ctx)
	if _, ok := session.lookup(ctx, last.SHA1, last.Size); !ok {
		t.Fatal("valid prefix lost after a torn append")
	}
	st, _ = c.file.Stat()
	if st.Size() != c.end {
		t.Fatal("torn tail was not removed")
	}
	key, _ := modMetadataKey(last.SHA1)
	loc := c.entries[key]
	if _, err = c.file.WriteAt([]byte("!"), loc.offset+sharedModHeaderBytes); err != nil {
		t.Fatal(err)
	}
	if _, ok := session.lookup(ctx, last.SHA1, last.Size); ok {
		t.Fatal("corrupt payload accepted")
	}
	session.put(ctx, last)
	if _, ok := session.lookup(ctx, last.SHA1, last.Size); !ok {
		t.Fatal("corrupt entry did not rebuild")
	}
	session.close()
}

func TestClearModCachePreventsStalePublication(t *testing.T) {
	ctx := context.Background()
	s := testService(t)
	r := newTestRoot(t, s, "clear")
	c := s.modMetadataCache()
	old := c.begin(ctx)
	defer old.close()
	entry := cacheFixture(0, "")
	old.put(ctx, entry)
	for _, root := range []string{r.Path, filepath.Join(r.Path, "versions", "pack")} {
		old.saveLocal(ctx, root, map[string]modCacheEntry{"a.jar": entry})
		writeTestFile(t, filepath.Join(root, "mods", "a.jar"), []byte("keep mod"))
		writeTestFile(t, filepath.Join(root, ".mcl", "mods.json"), []byte("keep records"))
	}
	info, err := s.ModCacheUsage(ctx)
	if err != nil || info.Files != 3 || info.Bytes <= 0 {
		t.Fatal(info, err)
	}
	if err = s.ClearModCache(ctx); err != nil {
		t.Fatal(err)
	}
	old.put(ctx, entry)
	old.saveLocal(ctx, r.Path, map[string]modCacheEntry{"a.jar": entry})
	info, err = s.ModCacheUsage(ctx)
	if err != nil || info.Files != 0 || info.Bytes != 0 {
		t.Fatal("stale session republished cache", info, err)
	}
	for _, root := range []string{r.Path, filepath.Join(r.Path, "versions", "pack")} {
		for _, file := range []string{"mods/a.jar", ".mcl/mods.json"} {
			if _, err = os.Stat(filepath.Join(root, filepath.FromSlash(file))); err != nil {
				t.Fatal("cleanup removed user data", err)
			}
		}
	}
	fresh := c.begin(ctx)
	fresh.put(ctx, entry)
	if _, ok := fresh.lookup(ctx, entry.SHA1, entry.Size); !ok {
		t.Fatal("new sessions cannot rebuild")
	}
	fresh.close()
}

func TestSharedModCacheCancellationReleasesBuffers(t *testing.T) {
	s := testService(t)
	r := newTestRoot(t, s, "cancel")
	testJar(t, filepath.Join(r.Path, "mods", "cancel.jar"), "cancel", "Cancel", nil)
	ctx, cancel := context.WithCancel(context.Background())
	_, err := s.ListModsStreamContext(ctx, r.ID, "", func([]Mod) { cancel() })
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	c := s.modMetadataCache()
	if c.users != 0 || c.file != nil || c.entries != nil {
		t.Fatal("cancel retained shared cache")
	}
	modScanMemory.Lock()
	defer modScanMemory.Unlock()
	if modScanMemory.users != 0 || modScanMemory.work != nil {
		t.Fatal("cancel retained scanner buffers")
	}
}

func TestCancelledModScanKeepsOnlyCompletedFiles(t *testing.T) {
	for _, clear := range []bool{false, true} {
		t.Run(fmt.Sprint("clear=", clear), func(t *testing.T) {
			s := testService(t)
			r := newTestRoot(t, s, "partial")
			for i := range modWorkers*2 + 1 {
				name := fmt.Sprint("mod", i)
				testJar(t, filepath.Join(r.Path, "mods", name+".jar"), name, name, nil)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			completed := map[string]string{}
			_, err := s.ListModsStreamContext(ctx, r.ID, "", func(mods []Mod) {
				for _, mod := range mods {
					if !mod.Pending {
						completed[mod.File] = mod.SHA1
					}
				}
				if len(completed) == 0 {
					return
				}
				if clear {
					if err := s.ClearModCache(context.Background()); err != nil {
						t.Error(err)
					}
				}
				cancel()
			})
			if !errors.Is(err, context.Canceled) || len(completed) == 0 {
				t.Fatalf("scan did not stop after the first ready rows: %v", err)
			}
			cache := loadModCache(r.Path)
			if clear {
				if len(cache) != 0 {
					t.Fatal("cancelled scan resurrected cleared cache entries")
				}
				return
			}
			if len(cache) == 0 || len(cache) > modWorkers {
				t.Fatalf("expected only completed worker results, got %d", len(cache))
			}
			for name, sha := range completed {
				if cache[name].SHA1 != sha || !cache[name].HasFP {
					t.Fatal("completed identity lost on navigation", name)
				}
			}
			// The very first streamed result on return must already contain the
			// ready metadata. No JAR parsing/hashing has started at that point.
			first := true
			mods, err := s.ListModsStreamContext(context.Background(), r.ID, "", func(mods []Mod) {
				if !first {
					return
				}
				first = false
				for _, mod := range mods {
					if c, ok := cache[mod.File]; ok && (mod.Pending || mod.SHA1 != c.SHA1 || mod.ModID != c.Meta.ModID) {
						t.Error("returning to a partial scan reread a completed JAR", mod.File)
					}
				}
			})
			if err != nil || len(mods) != modWorkers*2+1 || first {
				t.Fatalf("resumed scan did not complete: mods=%d err=%v", len(mods), err)
			}
		})
	}
}

func TestCancelledModScanDoesNotReplaceNewerCompletedCache(t *testing.T) {
	s := testService(t)
	r := newTestRoot(t, s, "overlap")
	count := modWorkers*2 + 1
	for i := range count {
		name := fmt.Sprint("mod", i)
		testJar(t, filepath.Join(r.Path, "mods", name+".jar"), name, name, nil)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := s.ListModsStreamContext(ctx, r.ID, "", func(mods []Mod) {
		for _, mod := range mods {
			if !mod.Pending {
				// Complete a newer request while the old one is still delivering
				// its first result, then leave the old view.
				if _, err := s.ListMods(r.ID); err != nil {
					t.Error(err)
				}
				cancel()
				return
			}
		}
	})
	if !errors.Is(err, context.Canceled) || len(loadModCache(r.Path)) != count {
		t.Fatalf("old partial cache replaced a completed newer scan: %v", err)
	}
}

func TestCancelledModScanUpdatesChangedFileStamps(t *testing.T) {
	s := testService(t)
	r := newTestRoot(t, s, "changed")
	for i := range modWorkers*2 + 1 {
		name := fmt.Sprint("mod", i)
		testJar(t, filepath.Join(r.Path, "mods", name+".jar"), name, "Old", nil)
	}
	if _, err := s.ListMods(r.ID); err != nil {
		t.Fatal(err)
	}
	for i := range modWorkers*2 + 1 {
		name := fmt.Sprint("mod", i)
		testJar(t, filepath.Join(r.Path, "mods", name+".jar"), name, "Changed metadata", nil)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var ready Mod
	_, err := s.ListModsStreamContext(ctx, r.ID, "", func(mods []Mod) {
		for _, mod := range mods {
			if !mod.Pending {
				ready = mod
				cancel()
				return
			}
		}
	})
	cache := loadModCache(r.Path)
	if !errors.Is(err, context.Canceled) || ready.SHA1 == "" || cache[ready.File].SHA1 != ready.SHA1 || cache[ready.File].Meta.Name != "Changed metadata" {
		t.Fatalf("checkpoint kept the obsolete stamp instead of a completed changed file: %+v %v", ready, err)
	}
}

func TestLocalModCacheBoundedAndSeededOnce(t *testing.T) {
	ctx := context.Background()
	s := testService(t)
	r := newTestRoot(t, s, "legacy")
	testJar(t, filepath.Join(r.Path, "mods", "legacy.jar"), "legacy", "Legacy", nil)
	mods, err := scanMods(r.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ListMods(r.ID); err != nil {
		t.Fatal(err)
	}
	if !loadModCacheData(r.Path).Shared {
		t.Fatal("legacy cache was not migrated")
	}
	c := s.modMetadataCache()
	session := c.begin(ctx)
	if _, ok := session.lookup(ctx, mods[0].SHA1, mods[0].Size); !ok {
		t.Fatal("legacy metadata not seeded")
	}
	session.close()
	if err = os.Remove(c.path); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ListMods(r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(c.path); !os.IsNotExist(err) {
		t.Fatal("warm local scan reopened shared index")
	}
	entries := map[string]modCacheEntry{}
	for i := 0; i < 100; i++ {
		entries[fmt.Sprintf("%d.jar", i)] = cacheFixture(i, strings.Repeat("x", 60<<10))
	}
	saveModCache(r.Path, entries)
	st, err := os.Stat(filepath.Join(r.Path, ".mcl", modCacheFile))
	if err != nil || st.Size() > maxLocalModCacheBytes {
		t.Fatal(st, err)
	}
	if len(loadModCache(r.Path)) == 0 {
		t.Fatal("oversized pack lost all usable cached entries")
	}
}

func TestSharedModCacheOverlappingSessionsAndClear(t *testing.T) {
	s := testService(t)
	ctx := context.Background()
	c := s.modMetadataCache()
	var wg sync.WaitGroup
	ready, finish := make(chan struct{}, modWorkers), make(chan struct{})
	for i := 0; i < modWorkers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			session := c.begin(ctx)
			defer session.close()
			entry := cacheFixture(i, "")
			session.put(ctx, entry)
			ready <- struct{}{}
			<-finish
			// These old-generation writes must be discarded after clear.
			session.put(ctx, entry)
			if _, ok := session.lookup(ctx, entry.SHA1, entry.Size); ok {
				t.Error("old session returned a post-clear hit")
			}
		}(i)
	}
	for range modWorkers {
		<-ready
	}
	if err := s.ClearModCache(ctx); err != nil {
		t.Fatal(err)
	}
	close(finish)
	wg.Wait()
	if c.users != 0 || c.file != nil || c.entries != nil {
		t.Fatal("overlapping sessions retained state")
	}
	if _, err := os.Stat(c.path); !os.IsNotExist(err) {
		t.Fatal("post-clear file recreated", err)
	}
}
