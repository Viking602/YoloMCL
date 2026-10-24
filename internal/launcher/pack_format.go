package launcher

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type mrPack struct {
	FormatVersion int               `json:"formatVersion"`
	Game          string            `json:"game"`
	VersionID     string            `json:"versionId"`
	Name          string            `json:"name"`
	Dependencies  map[string]string `json:"dependencies"`
	Files         []struct {
		Path   string `json:"path"`
		Hashes struct {
			SHA1   string `json:"sha1"`
			SHA512 string `json:"sha512"`
		} `json:"hashes"`
		Env struct {
			Client string `json:"client"`
		} `json:"env"`
		Downloads []string `json:"downloads"`
		Size      int64    `json:"fileSize"`
	} `json:"files"`
}
type cfPack struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	ManifestType    string `json:"manifestType"`
	ManifestVersion int    `json:"manifestVersion"`
	Overrides       string `json:"overrides"`
	Minecraft       struct {
		Version string `json:"version"`
		Loaders []struct {
			ID      string `json:"id"`
			Primary bool   `json:"primary"`
		} `json:"modLoaders"`
	} `json:"minecraft"`
	Files []struct {
		ProjectID int  `json:"projectID"`
		FileID    int  `json:"fileID"`
		Required  bool `json:"required"`
	} `json:"files"`
}

func parsePack(z *zip.ReadCloser) (PackLock, string, error) {
	for _, f := range z.File {
		if f.Name == "modrinth.index.json" {
			var index mrPack
			if e := readZipJSON(f, &index); e != nil {
				return PackLock{}, "", e
			}
			if index.FormatVersion != 1 || index.Game != "minecraft" {
				return PackLock{}, "", errors.New("不支持此 Modrinth 整合包格式")
			}
			lock := PackLock{Name: index.Name, Version: index.VersionID, Source: "modrinth", Minecraft: index.Dependencies["minecraft"], Loader: "vanilla", Files: []PackFile{}, Manual: []ManualDownload{}}
			for key, loader := range map[string]string{"fabric-loader": "fabric", "quilt-loader": "quilt", "forge": "forge", "neoforge": "neoforge"} {
				if v := index.Dependencies[key]; v != "" {
					if lock.Loader != "vanilla" {
						return lock, "", errors.New("整合包声明了多个加载器")
					}
					lock.Loader = loader
					lock.LoaderVersion = v
				}
			}
			for _, file := range index.Files {
				if file.Env.Client == "unsupported" {
					continue
				}
				if len(file.Downloads) == 0 || len(file.Hashes.SHA1) != 40 || len(file.Hashes.SHA512) != 128 {
					return lock, "", errors.New("整合包缺少下载地址或文件校验值")
				}
				lock.Files = append(lock.Files, PackFile{Path: file.Path, SHA1: file.Hashes.SHA1, SHA512: file.Hashes.SHA512, Source: "modrinth", Kind: "remote", Size: file.Size, URLs: file.Downloads})
			}
			if lock.Minecraft == "" {
				return lock, "", errors.New("整合包未指定 Minecraft 版本")
			}
			return lock, "overrides", nil
		}
	}
	for _, f := range z.File {
		if f.Name == "manifest.json" {
			var index cfPack
			if e := readZipJSON(f, &index); e != nil {
				return PackLock{}, "", e
			}
			if index.ManifestType != "minecraftModpack" || index.ManifestVersion != 1 {
				return PackLock{}, "", errors.New("不是受支持的 CurseForge 整合包")
			}
			lock := PackLock{Name: index.Name, Version: index.Version, Source: "curseforge", Minecraft: index.Minecraft.Version, Loader: "vanilla", Files: []PackFile{}, Manual: []ManualDownload{}}
			for i, loader := range index.Minecraft.Loaders {
				if !loader.Primary && i > 0 {
					continue
				}
				for _, kind := range []string{"neoforge", "forge", "fabric", "quilt"} {
					if strings.HasPrefix(loader.ID, kind+"-") {
						lock.Loader = kind
						lock.LoaderVersion = strings.TrimPrefix(loader.ID, kind+"-")
						break
					}
				}
				if loader.Primary {
					break
				}
			}
			for _, f := range index.Files {
				lock.Files = append(lock.Files, PackFile{Source: "curseforge", ProjectID: fmt.Sprint(f.ProjectID), VersionID: fmt.Sprint(f.FileID)})
			}
			if lock.Minecraft == "" {
				return lock, "", errors.New("整合包未指定 Minecraft 版本")
			}
			return lock, index.Overrides, nil
		}
	}
	if lock, overrides, ok, e := parseMultiMC(z); ok || e != nil {
		return lock, overrides, e
	}
	return PackLock{}, "", errors.New("压缩包根目录中找不到 modrinth.index.json、CurseForge manifest.json 或 MultiMC instance.cfg")
}
func (s *Service) InspectPack(path string) (PackPreview, error) {
	p, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return PackPreview{}, err
	}
	z, err := zip.OpenReader(p)
	if err != nil {
		return PackPreview{}, fmt.Errorf("无法读取整合包压缩文件: %w", err)
	}
	defer z.Close()
	lock, _, err := parsePack(z)
	if err != nil {
		return PackPreview{}, err
	}
	name := strings.TrimSpace(lock.Name)
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
	}
	return PackPreview{
		Path:          p,
		Name:          name,
		Version:       lock.Version,
		Source:        lock.Source,
		Minecraft:     lock.Minecraft,
		Loader:        lock.Loader,
		LoaderVersion: lock.LoaderVersion,
		FileCount:     len(lock.Files),
		ModCount:      countMods(lock),
		DownloadMB:    estimateDownloadMB(lock, z),
		Unavailable:   s.packUnavailable(lock),
	}, nil
}

func readPack(root string) *PackLock {
	path, e := safeJoin(root, ".mcl/pack-lock.json")
	if e != nil {
		return nil
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return nil
	}
	var p PackLock
	if json.Unmarshal(b, &p) != nil {
		return nil
	}
	upgradeLock(&p)
	return &p
}
