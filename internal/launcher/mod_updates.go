package launcher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func instanceCompatibility(r Root) (string, string, error) {
	versions := scanVersions(r.Path)
	for _, v := range versions {
		if v.ID == r.SelectedVersion {
			return v.Minecraft, v.Loader, nil
		}
	}
	if p := readPack(r.Path); p != nil {
		return p.Minecraft, p.Loader, nil
	}
	return "", "", errors.New("请先选择一个已安装的游戏版本")
}

func (s *Service) CheckMods(rootID string) (string, error) {
	r, e := s.getRoot(rootID)
	if e != nil {
		return "", e
	}
	return s.startJob("检测模组版本", rootID, func(ctx context.Context, p progress) error {
		return s.checkModsIn(ctx, r, r.VersionDir(), p, nil)
	})
}

// CheckModsBackground runs the version check off any job or UI thread and
// reports results in batches as they arrive (sha1 -> checked mod). It
// stops when ctx is cancelled (tab or pack changed).
func (s *Service) CheckModsBackground(ctx context.Context, rootID, versionID string, partial func(map[string]Mod)) error {
	r, e := s.rootAt(rootID, versionID)
	if e != nil {
		return e
	}
	return s.checkModsIn(ctx, r, r.VersionDir(), func(string, int, int) {}, partial)
}

// ModChecksAge is how long ago the version check last ran for a version
// (a large value when never).
func (s *Service) ModChecksAge(rootID, versionID string) time.Duration {
	r, e := s.rootAt(rootID, versionID)
	if e != nil {
		return 1 << 62
	}
	st, e := os.Stat(filepath.Join(r.VersionDir(), ".mcl", "mod-checks.json"))
	if e != nil {
		return 1 << 62
	}
	return time.Since(st.ModTime())
}

// checkModsIn identifies mods by hash on Modrinth (both requests at once)
// and by fingerprint on CurseForge, then looks up CurseForge versions with
// a bounded pool. A failing step is recorded and the others still run; an
// error is returned only when nothing could be checked.
func (s *Service) checkModsIn(ctx context.Context, r Root, vDir string, p progress, partial func(map[string]Mod)) error {
	mods, e := scanModsStreamContext(ctx, vDir, nil)
	if e != nil {
		return e
	}
	mc, loader, e := instanceCompatibility(r)
	if e != nil {
		return e
	}
	checks := map[string]Mod{}
	if len(mods) == 0 {
		return atomicJSON(filepath.Join(vDir, ".mcl", "mod-checks.json"), checks)
	}
	hashes := make([]string, 0, len(mods))
	for _, m := range mods {
		hashes = append(hashes, m.SHA1)
	}
	var stepErrs []string
	var mu sync.Mutex
	cfOK := false
	report := func(batch map[string]Mod) {
		if partial != nil && len(batch) > 0 {
			partial(batch)
		}
	}
	known := map[string]mrVersion{}
	updates := map[string]mrVersion{}
	p("正在通过文件哈希识别 Modrinth 模组", 0, len(mods))
	var wg sync.WaitGroup
	var eKnown, eUpd error
	wg.Add(2)
	go func() {
		defer wg.Done()
		eKnown = s.jsonRequest(ctx, "POST", modrinthAPI+"/version_files", map[string]any{"hashes": hashes, "algorithm": "sha1"}, nil, &known)
	}()
	go func() {
		defer wg.Done()
		eUpd = s.jsonRequest(ctx, "POST", modrinthAPI+"/version_files/update", map[string]any{"hashes": hashes, "algorithm": "sha1", "game_versions": []string{mc}, "loaders": []string{loader}}, nil, &updates)
	}()
	wg.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if eKnown != nil {
		stepErrs = append(stepErrs, "Modrinth: "+eKnown.Error())
	}
	if eUpd != nil {
		stepErrs = append(stepErrs, "Modrinth 更新: "+eUpd.Error())
	}
	batch := map[string]Mod{}
	for i, m := range mods {
		m.Latest = nil
		m.Status = "unknown"
		if v, ok := known[m.SHA1]; ok {
			m.Source, m.ProjectID, m.VersionID, m.Status = "modrinth", v.ProjectID, v.ID, "current"
			if next, ok := updates[m.SHA1]; ok && next.ID != v.ID {
				nv := next.version()
				m.Latest = &nv
				m.Status = "update"
			}
			batch[m.SHA1] = m
		}
		checks[m.SHA1] = m
		p("正在识别 "+m.Name, i+1, len(mods))
	}
	report(batch)
	if s.publisher.HasCurseForge() {
		fps := []uint32{}
		byFP := map[uint32]string{}
		for _, m := range mods {
			if checks[m.SHA1].Status != "unknown" && m.Source != "curseforge" {
				continue
			}
			fp := m.Fingerprint
			if fp == 0 { // very large jar: not fingerprinted during the scan
				path, e := safeJoin(vDir, "mods/"+m.File)
				if e != nil {
					continue
				}
				if st, e := os.Stat(path); e != nil || st.Size() > 256<<20 {
					continue
				}
				b, e := os.ReadFile(path)
				if e != nil {
					continue
				}
				fp = curseFingerprint(b)
			}
			fps = append(fps, fp)
			byFP[fp] = m.SHA1
		}
		if len(fps) > 0 {
			p("正在查询 CurseForge 文件指纹", 0, len(fps))
			var matches struct {
				Data struct {
					ExactMatches []struct {
						File cfFile `json:"file"`
					} `json:"exactMatches"`
				} `json:"data"`
			}
			if e := s.cfJSON(ctx, "POST", "/fingerprints", map[string]any{"fingerprints": fps}, &matches); e != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				stepErrs = append(stepErrs, "CurseForge: "+e.Error())
			} else {
				cfOK = true
				ch := make(chan int)
				var vwg sync.WaitGroup
				done := 0
				for w := 0; w < modCheckWorkers; w++ {
					vwg.Add(1)
					go func() {
						defer vwg.Done()
						for i := range ch {
							match := matches.Data.ExactMatches[i]
							mu.Lock()
							hash, ok := byFP[match.File.Fingerprint]
							m := checks[hash]
							mu.Unlock()
							if !ok {
								continue
							}
							m.Source, m.ProjectID, m.VersionID, m.Status = "curseforge", fmt.Sprint(match.File.ModID), fmt.Sprint(match.File.ID), "current"
							vs, e := s.Versions(ctx, "curseforge", m.ProjectID, mc, loader)
							mu.Lock()
							if e != nil {
								if ctx.Err() == nil && len(stepErrs) < 8 {
									stepErrs = append(stepErrs, "CurseForge "+m.Name+": "+e.Error())
								}
							} else if len(vs) > 0 && vs[0].ID != m.VersionID && vs[0].Published > match.File.FileDate {
								m.Latest = &vs[0]
								m.Status = "update"
							}
							checks[hash] = m
							done++
							n := done
							mu.Unlock()
							report(map[string]Mod{hash: m})
							p("检测 CurseForge："+m.Name, n, len(matches.Data.ExactMatches))
						}
					}()
				}
			feed:
				for i := range matches.Data.ExactMatches {
					select {
					case ch <- i:
					case <-ctx.Done():
						break feed
					}
				}
				close(ch)
				vwg.Wait()
				if ctx.Err() != nil {
					return ctx.Err()
				}
			}
		}
	}
	if eKnown != nil && !cfOK { // nothing could be identified
		return errors.New("检查失败：" + strings.Join(stepErrs, "；"))
	}
	if e := atomicJSON(filepath.Join(vDir, ".mcl", "mod-checks.json"), checks); e != nil {
		return e
	}
	if len(stepErrs) > 0 {
		p("部分检查失败："+strings.Join(stepErrs, "；"), len(mods), len(mods))
	}
	return nil
}
