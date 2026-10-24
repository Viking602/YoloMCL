package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// writeScanDiagnosticReport is the local-only scan diagnostic selected by MCL_DIAG_SCAN.
// Match a version or directory name without changing the saved selection.
func writeScanDiagnosticReport(s *launcher.Service, path, filter string) error {
	filter = strings.ToLower(strings.TrimSpace(filter))
	var rows []map[string]any
	for _, r := range s.Snapshot().Instances {
		for _, v := range r.Versions {
			if !strings.Contains(strings.ToLower(r.Name+"/"+v.ID), filter) {
				continue
			}
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			start := time.Now()
			first := time.Duration(0)
			mods, err := s.ListModsStream(r.ID, v.ID, func([]launcher.Mod) {
				if first == 0 {
					first = time.Since(start)
				}
			})
			total := time.Since(start)
			runtime.ReadMemStats(&after)
			if first == 0 {
				first = total
			}
			row := map[string]any{"version": v.ID, "mods": len(mods), "firstListMs": float64(first.Microseconds()) / 1000,
				"completeMs": float64(total.Microseconds()) / 1000, "allocatedBytes": after.TotalAlloc - before.TotalAlloc, "gcCycles": after.NumGC - before.NumGC}
			if err != nil {
				row["error"] = err.Error()
			}
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		return fmt.Errorf("no installed version matches %q", filter)
	}
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return os.WriteFile(path, b, 0644)
}

// writeDiagnosticReport writes timings and icon coverage for the selected instance of
// every directory to path (developer QA; reads only, renames nothing).
func writeDiagnosticReport(s *launcher.Service, path string) error {
	snapStart := time.Now()
	snap := s.Snapshot()
	report := map[string]any{"snapshotMs": time.Since(snapStart).Milliseconds()}
	var rows []map[string]any
	for _, r := range snap.Instances {
		if r.ModCount == 0 {
			continue
		}
		legacy, _ := s.DiagLegacyScan(r.ID, r.SelectedVersion)
		t0 := time.Now()
		mods, e := s.ListModsAt(r.ID, r.SelectedVersion)
		if e != nil {
			continue
		}
		cold := time.Since(t0)
		t1 := time.Now()
		_, _ = s.ListModsAt(r.ID, r.SelectedVersion)
		warm := time.Since(t1)
		local := 0
		for _, m := range mods {
			if _, e := s.AddonIcon(r.ID, r.SelectedVersion, "mod", m.File); e == nil {
				local++
			}
		}
		t2 := time.Now()
		urls, ue := s.ModIconURLs(context.Background(), r.ID, r.SelectedVersion, mods)
		both := 0
		for _, m := range mods {
			if _, e := s.AddonIcon(r.ID, r.SelectedVersion, "mod", m.File); e == nil || urls[m.SHA1] != "" {
				both++
			}
		}
		row := map[string]any{"name": r.Name, "mods": len(mods), "legacyScanMs": legacy.Milliseconds(), "listColdMs": cold.Milliseconds(), "listWarmMs": warm.Milliseconds(), "jarIcons": local, "withRemote": both, "remoteMs": time.Since(t2).Milliseconds()}
		if ue != nil {
			row["remoteErr"] = ue.Error()
		}
		// Chinese names (MC百科 mapping) by slug or mod id.
		refs := make([]launcher.AddonRef, 0, len(mods))
		for _, m := range mods {
			refs = append(refs, launcher.AddonRef{SHA1: m.SHA1, File: m.File})
		}
		info, _ := s.AddonRemoteInfo(context.Background(), r.ID, r.SelectedVersion, "mod", refs)
		remoteSum, jarSum, withCats := 0, 0, 0
		for _, m := range mods {
			switch {
			case info[m.SHA1].Summary != "":
				remoteSum++
			case m.Description != "":
				jarSum++
			}
			if len(info[m.SHA1].Cats) > 0 {
				withCats++
			}
		}
		row["summaryRemote"], row["summaryJar"], row["withCategories"] = remoteSum, jarSum, withCats
		if z, ze := s.ZhNames(context.Background()); z != nil {
			zh := 0
			for _, m := range mods {
				if z.Mod(info[m.SHA1].CFSlug, info[m.SHA1].MRSlug, m.ModID) != "" {
					zh++
				}
			}
			row["zhNames"] = zh
			row["zhTable"] = z.Size()
		} else if ze != nil {
			row["zhErr"] = ze.Error()
		}
		for _, kind := range []string{"resourcepack", "shader"} {
			packs, e := s.ListAssetPacksAt(r.ID, r.SelectedVersion, kind)
			if e != nil || len(packs) == 0 {
				continue
			}
			own := 0
			for _, p := range packs {
				if _, e := s.AddonIcon(r.ID, r.SelectedVersion, kind, p.File); e == nil {
					own++
				}
			}
			pref := s.PackRefs(r.ID, r.SelectedVersion, kind, packs)
			pinfo, _ := s.AddonRemoteInfo(context.Background(), r.ID, r.SelectedVersion, kind, pref)
			withRemote := 0
			byFile := map[string]bool{}
			for _, ref := range pref {
				if pinfo[ref.SHA1].Icon != "" {
					byFile[ref.File] = true
				}
			}
			for _, p := range packs {
				if _, e := s.AddonIcon(r.ID, r.SelectedVersion, kind, p.File); e == nil || byFile[p.File] {
					withRemote++
				}
			}
			sum, cats := 0, 0
			for _, p := range packs {
				h := ""
				for _, ref := range pref {
					if ref.File == p.File {
						h = ref.SHA1
					}
				}
				if pinfo[h].Summary != "" || p.Description != "" {
					sum++
				}
				if len(pinfo[h].Cats) > 0 {
					cats++
				}
			}
			row[kind] = map[string]int{"count": len(packs), "ownIcons": own, "withRemote": withRemote, "withSummary": sum, "withCategories": cats}
		}
		rows = append(rows, row)
		if len(rows) >= 3 {
			break
		}
	}
	report["instances"] = rows
	b, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(b))
	return os.WriteFile(path, b, 0644)
}
