package launcher

import (
	"archive/zip"
	"bufio"
	"errors"
	"io"
	"strings"
)

// packUnavailable explains why a pack cannot be imported by this build.
func (s *Service) packUnavailable(lock PackLock) string {
	if lock.Source == "curseforge" && len(lock.Files) > 0 && !s.CurseForgeAvailable() {
		return curseForgeUnavailable
	}
	return ""
}

// estimateDownloadMB is a rough import size: declared pack file sizes
// (Modrinth) or ~3 MiB per mod (CurseForge), plus the game: client,
// libraries and assets (~650 MiB for 1.13+, ~350 MiB before).
func estimateDownloadMB(lock PackLock, z *zip.ReadCloser) int {
	game := 650
	if mcBefore(lock.Minecraft, 13) {
		game = 350
	}
	var files int64
	for _, f := range z.File {
		if f.Name == "modrinth.index.json" {
			var idx struct {
				Files []struct {
					Size int64 `json:"fileSize"`
					Env  struct {
						Client string `json:"client"`
					} `json:"env"`
				} `json:"files"`
			}
			if readZipJSON(f, &idx) == nil {
				for _, x := range idx.Files {
					if x.Env.Client != "unsupported" {
						files += x.Size
					}
				}
			}
		}
	}
	if lock.Source == "curseforge" {
		files = int64(len(lock.Files)) * 3 << 20
	}
	return game + int(files>>20)
}

// parseMultiMC reads a MultiMC / Prism Launcher instance export:
// instance.cfg + mmc-pack.json (optionally inside one top folder) with the
// game files under .minecraft/ or minecraft/. ok reports a MultiMC zip.
func parseMultiMC(z *zip.ReadCloser) (PackLock, string, bool, error) {
	var cfg, pack *zip.File
	prefix := ""
	for _, f := range z.File {
		dir, base := "", f.Name
		if i := strings.LastIndex(f.Name, "/"); i >= 0 {
			dir, base = f.Name[:i+1], f.Name[i+1:]
		}
		if strings.Count(dir, "/") > 1 {
			continue
		}
		switch base {
		case "instance.cfg":
			cfg, prefix = f, dir
		case "mmc-pack.json":
			pack = f
		}
	}
	if cfg == nil || pack == nil || !strings.HasPrefix(pack.Name, prefix) {
		return PackLock{}, "", false, nil
	}
	var mmc struct {
		Components []struct {
			UID     string `json:"uid"`
			Version string `json:"version"`
		} `json:"components"`
	}
	if e := readZipJSON(pack, &mmc); e != nil {
		return PackLock{}, "", true, errors.New("MultiMC 实例的 mmc-pack.json 无法解析")
	}
	lock := PackLock{Source: "multimc", Loader: "vanilla", Files: []PackFile{}, Manual: []ManualDownload{}}
	for _, c := range mmc.Components {
		switch c.UID {
		case "net.minecraft":
			lock.Minecraft = c.Version
		case "net.fabricmc.fabric-loader":
			lock.Loader, lock.LoaderVersion = "fabric", c.Version
		case "org.quiltmc.quilt-loader":
			lock.Loader, lock.LoaderVersion = "quilt", c.Version
		case "net.minecraftforge":
			lock.Loader, lock.LoaderVersion = "forge", c.Version
		case "net.neoforged":
			lock.Loader, lock.LoaderVersion = "neoforge", c.Version
		}
	}
	if lock.Minecraft == "" {
		return lock, "", true, errors.New("MultiMC 实例未指定 Minecraft 版本")
	}
	if rc, e := cfg.Open(); e == nil {
		sc := bufio.NewScanner(io.LimitReader(rc, 64<<10))
		for sc.Scan() {
			if k, v, ok := strings.Cut(sc.Text(), "="); ok && strings.TrimSpace(k) == "name" {
				lock.Name = strings.TrimSpace(v)
			}
		}
		rc.Close()
	}
	game := prefix + ".minecraft"
	for _, f := range z.File {
		if strings.HasPrefix(f.Name, prefix+"minecraft/") {
			game = prefix + "minecraft"
			break
		}
	}
	return lock, game, true, nil
}

// ArchiveKind classifies a dropped .zip: "modpack", "resourcepack" (has
// pack.mcmeta), "shader" (has a shaders/ folder), or an error explaining
// what it is not. Zips are never guessed into a category.
func (s *Service) ArchiveKind(path string) (string, error) {
	_, packErr := s.InspectPack(path)
	if packErr == nil {
		return "modpack", nil
	}
	z, e := zip.OpenReader(path)
	if e != nil {
		return "", errors.New("无法读取压缩文件")
	}
	defer z.Close()
	kind := ""
	for _, f := range z.File {
		parts := strings.Split(strings.TrimSuffix(f.Name, "/"), "/")
		base := parts[len(parts)-1]
		depth := len(parts) - 1 // 0 at the root, 1 inside one wrapping folder
		switch {
		case depth <= 1 && (base == "modrinth.index.json" || base == "manifest.json" || base == "instance.cfg" || base == "mmc-pack.json"):
			return "", packErr // a modpack, but a broken or unsupported one
		case depth <= 1 && base == "pack.mcmeta":
			kind = "resourcepack"
		case kind == "" && (parts[0] == "shaders" || len(parts) > 1 && parts[1] == "shaders"):
			kind = "shader"
		}
	}
	if kind != "" {
		return kind, nil
	}
	return "", errors.New("无法识别此压缩包：它不是整合包（Modrinth / CurseForge / MultiMC），也不是材质包（缺少 pack.mcmeta）或光影包（缺少 shaders 文件夹）")
}
