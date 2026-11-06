package launcher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Viking602/YoloMCL/internal/i18n"
)

func (s *Service) InstallMod(rootID, source, project, id, replace string) (string, error) {
	r, e := s.getRoot(rootID)
	if e != nil {
		return "", e
	}
	return s.startJob("安装模组及依赖", rootID, func(ctx context.Context, p progress) error {
		return s.installModIn(ctx, p, r, source, project, id, replace, nil)
	})
}

// installModIn installs one mod version with its required dependencies;
// skip lists source:project keys already satisfied (present by mod id or
// on another platform) that must not be downloaded again.
func (s *Service) installModIn(ctx context.Context, p progress, r Root, source, project, id, replace string, skip map[string]bool) error {
	{
		mc, loader, e := instanceCompatibility(r)
		if e != nil {
			return e
		}
		if loader == "vanilla" {
			return errors.New("原版不加载模组，请先安装并选择 Fabric / Quilt / Forge / NeoForge")
		}
		version, e := s.projectVersion(ctx, source, project, id)
		if e != nil {
			return e
		}
		if version.ProjectID != project {
			return errors.New("模组版本与项目不匹配")
		}
		plan := []ProjectVersion{}
		seen := map[string]string{}
		vDir := r.VersionDir()
		cache := s.modMetadataCache().begin(ctx)
		defer cache.close()
		installed, e := scanModsCached(ctx, vDir, nil, cache)
		if e != nil {
			return e
		}
		var resolve func(ProjectVersion, bool) error
		resolve = func(v ProjectVersion, install bool) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			key := v.Source + ":" + v.ProjectID
			if previous, ok := seen[key]; ok {
				if previous != v.ID {
					return fmt.Errorf(i18n.Text(false, "dependencies.version_conflict"), v.Name)
				}
				return nil
			}
			seen[key] = v.ID
			if len(seen) > maxDependencyProjects {
				return errDependencyLimit
			}
			if err := checkDependencyVersion(v, source, v.ProjectID, mc, loader); err != nil {
				return err
			}
			for _, d := range v.Dependencies {
				if d.Kind != "required" {
					continue
				}
				if d.ProjectID != "" && skip[v.Source+":"+d.ProjectID] {
					continue // explicit preview choices, including unchecked rows
				}
				var dep ProjectVersion
				var err error
				present := false
				for _, m := range installed {
					if m.Enabled && m.Source == v.Source && m.ProjectID == d.ProjectID && (d.VersionID == "" || m.VersionID == d.VersionID) {
						present = true
						dep, err = s.installedDependencyVersion(ctx, r, v.Source, d.ProjectID, m, mc, loader)
						break
					}
				}
				if !present {
					dep, err = s.dependencyVersion(ctx, v.Source, d, mc, loader)
				}
				if err != nil {
					return err
				}
				if skip[dep.Source+":"+dep.ProjectID] {
					continue // a version-only edge now has its canonical project key
				}
				if d.ProjectID == "" {
					for _, m := range installed {
						if m.Enabled && m.Source == dep.Source && m.ProjectID == dep.ProjectID && m.VersionID == dep.ID {
							present = true
							break
						}
					}
				}
				if e = resolve(dep, !present); e != nil {
					return e
				}
			}
			if install {
				plan = append(plan, v)
			}
			return nil
		}
		if e = resolve(version, true); e != nil {
			return e
		}
		stage, e := safeJoin(r.Path, ".mcl/staging/"+newID())
		if e != nil {
			return e
		}
		if e = os.MkdirAll(stage, 0755); e != nil {
			return e
		}
		defer os.RemoveAll(stage)
		files := []DownloadFile{}
		identities := []downloadIdentity{}
		names := map[string]bool{}
		for i, v := range plan {
			f, e := primaryFile(v)
			if e != nil {
				return e
			}
			if f.URL == "" {
				return fmt.Errorf("%s 禁止第三方下载，请在 CurseForge 网站下载后手动导入", v.Name)
			}
			if !strings.HasSuffix(f.Name, ".jar") || filepath.Base(f.Name) != f.Name {
				return errors.New("平台返回了无效模组文件名")
			}
			if names[strings.ToLower(f.Name)] {
				return errors.New("依赖文件名冲突")
			}
			names[strings.ToLower(f.Name)] = true
			p("下载 "+v.Name, i, len(plan))
			staged, e := safeJoin(stage, f.Name)
			if e != nil {
				return e
			}
			var identity downloadIdentity
			if e = s.downloadWithIdentity(ctx, f, staged, &identity); e != nil {
				return e
			}
			files = append(files, f)
			identities = append(identities, identity)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		records := readRecords(vDir)
		backup, e := safeJoin(r.Path, ".mcl/backups/"+time.Now().Format("20060102-150405")+"-"+newID()[:6])
		if e != nil {
			return e
		}
		if e = os.MkdirAll(backup, 0755); e != nil {
			return e
		}
		moved := map[string]string{}
		added := []string{}
		fresh := loadModCache(vDir)
		workspace := beginModScan()
		defer endModScan()
		rollback := func() {
			for _, path := range added {
				_ = os.Remove(path)
			}
			for orig, bak := range moved {
				_ = os.Rename(bak, orig)
			}
		}
		for i, v := range plan {
			f := files[i]
			stagedMod := Mod{}
			identity := &identities[i]
			hash, fp, hasFP, err := workspace.readJarCached(ctx, filepath.Join(stage, f.Name), identity.Size, &stagedMod, cache, identity, nil)
			if err != nil {
				rollback()
				return err
			}
			disabled := false
			for _, m := range installed {
				if (m.Source == v.Source && m.ProjectID == v.ProjectID) || (stagedMod.ModID != "" && stagedMod.ModID == m.ModID) || m.File == replace && v.ProjectID == project || strings.TrimSuffix(m.File, ".disabled") == f.Name {
					orig, e := safeJoin(vDir, "mods/"+m.File)
					if e != nil {
						rollback()
						return e
					}
					bak := filepath.Join(backup, m.File)
					if _, ok := moved[orig]; !ok {
						if e = os.Rename(orig, bak); e != nil {
							rollback()
							return e
						}
						moved[orig] = bak
						delete(fresh, m.File)
					}
					if !m.Enabled && v.ProjectID == project {
						disabled = true
					}
				}
			}
			name := f.Name
			if disabled {
				name += ".disabled"
			}
			dest, e := safeJoin(vDir, "mods/"+name)
			if e != nil {
				rollback()
				return e
			}
			if e = os.MkdirAll(filepath.Dir(dest), 0755); e != nil {
				rollback()
				return e
			}
			if _, e = os.Stat(dest); !os.IsNotExist(e) {
				rollback()
				return errors.New("目标模组文件已经存在")
			}
			if e = os.Rename(filepath.Join(stage, f.Name), dest); e != nil {
				rollback()
				return e
			}
			added = append(added, dest)
			st, e := os.Stat(dest)
			if e != nil {
				rollback()
				return e
			}
			records[f.Name] = modRecord{SHA1: hash, Source: v.Source, ProjectID: v.ProjectID, VersionID: v.ID}
			fresh[name] = modCacheEntry{Size: st.Size(), MTime: st.ModTime().UnixNano(), SHA1: hash, FP: fp, HasFP: hasFP, Meta: metaOnly(stagedMod)}
		}
		if ctx.Err() != nil {
			rollback()
			return ctx.Err()
		}
		if e = atomicJSON(filepath.Join(vDir, ".mcl", "mods.json"), records); e != nil {
			rollback()
			return e
		}
		cache.saveLocal(ctx, vDir, fresh)
		p("模组及依赖已安装", len(plan), len(plan))
		return nil
	}
}
