package launcher

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// downloadPackFiles resolves provider metadata and downloads verified pack files.
func (s *Service) downloadPackFiles(ctx context.Context, root string, lock *PackLock, p progress) error {
	seen := map[string]bool{}
	for i := range lock.Files {
		f := &lock.Files[i]
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if f.Source == "curseforge" {
			p(fmt.Sprintf("解析整合包模组 (%d/%d): 项目 #%s", i+1, len(lock.Files), f.ProjectID), i, len(lock.Files))
			v, e := s.projectVersion(ctx, "curseforge", f.ProjectID, f.VersionID)
			if e != nil {
				return e
			}
			df, e := primaryFile(v)
			if e != nil {
				return e
			}
			f.Path = "mods/" + df.Name
			f.SHA1 = df.SHA1
			if df.URL == "" {
				var project struct {
					Data struct {
						Name  string `json:"name"`
						Links struct {
							WebsiteURL string `json:"websiteUrl"`
						} `json:"links"`
					} `json:"data"`
				}
				if e = s.cfJSON(ctx, "GET", "/mods/"+f.ProjectID, nil, &project); e != nil {
					return e
				}
				lock.Manual = append(lock.Manual, ManualDownload{ProjectID: f.ProjectID, FileID: f.VersionID, Name: df.Name, URL: project.Data.Links.WebsiteURL + "/files/" + f.VersionID})
				continue
			}
			f.URLs = []string{df.URL}
		}
		if strings.HasPrefix(strings.ToLower(strings.ReplaceAll(f.Path, "\\", "/")), ".mcl/") {
			return errors.New("整合包不能写入启动器元数据目录")
		}
		target, e := safeJoin(root, f.Path)
		if e != nil {
			return e
		}
		key := strings.ToLower(target)
		if seen[key] {
			return errors.New("整合包包含重复文件路径")
		}
		seen[key] = true
		p(fmt.Sprintf("下载整合包文件 (%d/%d): %s", i+1, len(lock.Files), filepath.Base(f.Path)), i+1, len(lock.Files))
		var last error
		for _, raw := range f.URLs {
			last = s.download(ctx, DownloadFile{Name: f.Path, URL: raw, SHA1: f.SHA1, SHA512: f.SHA512}, target)
			if last == nil {
				break
			}
		}
		if last != nil {
			return last
		}
		if len(f.URLs) == 0 {
			return errors.New("整合包文件缺少下载地址")
		}
	}
	return nil
}

// applyPackOverrides records each extracted override in the pack lock.
func applyPackOverrides(ctx context.Context, z *zip.ReadCloser, root, overrides string, lock *PackLock) error {
	// Client overrides are deliberately applied after common overrides, as specified by mrpack.
	prefixes := []string{strings.Trim(overrides, "/")}
	if lock.Source == "modrinth" {
		prefixes = append(prefixes, "client-overrides")
	}
	for _, prefix := range prefixes {
		if prefix == "" {
			continue
		}
		for _, f := range z.File {
			if err := ctx.Err(); err != nil {
				return err
			}
			if !strings.HasPrefix(f.Name, prefix+"/") {
				continue
			}
			name := strings.TrimPrefix(f.Name, prefix+"/")
			if name == "" {
				continue
			}
			if strings.HasPrefix(strings.ToLower(name), ".mcl/") {
				return errors.New("整合包覆盖文件包含保留目录")
			}
			target, e := safeJoin(root, name)
			if e != nil {
				return e
			}
			if f.FileInfo().IsDir() {
				if e = os.MkdirAll(target, 0755); e != nil {
					return e
				}
				continue
			}
			if _, e = os.Stat(target); e == nil {
				if e = os.Remove(target); e != nil {
					return e
				}
			}
			if e = extractZipEntry(f, target); e != nil {
				return e
			}
			sha, sha512, e := fileHashes(target)
			if e != nil {
				return e
			}
			found := false
			for i, existing := range lock.Files {
				if strings.EqualFold(existing.Path, name) {
					lock.Files[i].SHA1 = sha
					lock.Files[i].SHA512 = sha512
					lock.Files[i].Kind = overrideKind(prefix)
					found = true
					break
				}
			}
			if !found {
				lock.Files = append(lock.Files, PackFile{Path: name, SHA1: sha, SHA512: sha512, Source: "override", Kind: overrideKind(prefix)})
			}
		}
	}
	return nil
}
