package launcher

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// AddonIcon returns the embedded icon of an installed mod, resource pack or
// shader pack: fabric/quilt "icon", (neo)forge mods.toml logoFile, or
// pack.png. Folders are read the same way. kind is "mod", "resourcepack"
// or "shader".
func (s *Service) AddonIcon(rootID, versionID, kind, file string) ([]byte, error) {
	r, e := s.rootAt(rootID, versionID)
	if e != nil {
		return nil, e
	}
	sub := "mods"
	if kind != "mod" {
		if sub, e = packFolder(kind); e != nil {
			return nil, e
		}
	}
	p, e := safeJoin(r.VersionDir(), sub+"/"+file)
	if e != nil {
		return nil, e
	}
	if st, e := os.Stat(p); e == nil && st.IsDir() {
		for _, n := range []string{"pack.png", "icon.png", "logo.png"} {
			if b, e := readCapped(filepath.Join(p, n)); e == nil {
				return b, nil
			}
		}
		return nil, errors.New("no icon")
	}
	z, e := zip.OpenReader(p)
	if e != nil {
		return nil, e
	}
	defer z.Close()
	files := map[string]*zip.File{}
	for _, f := range z.File {
		files[strings.TrimPrefix(f.Name, "/")] = f
	}
	read := func(name string) ([]byte, bool) {
		f := files[strings.TrimPrefix(path.Clean("/"+name), "/")]
		if f == nil || f.UncompressedSize64 > 4<<20 {
			return nil, false
		}
		rc, e := f.Open()
		if e != nil {
			return nil, false
		}
		defer rc.Close()
		b, e := io.ReadAll(io.LimitReader(rc, 4<<20))
		return b, e == nil && len(b) > 0
	}
	var candidates []string
	if b, ok := read("fabric.mod.json"); ok {
		var m struct{ Icon json.RawMessage }
		if json.Unmarshal(b, &m) == nil {
			candidates = append(candidates, iconField(m.Icon))
		}
	}
	if b, ok := read("quilt.mod.json"); ok {
		var m struct {
			QuiltLoader struct {
				Metadata struct{ Icon json.RawMessage } `json:"metadata"`
			} `json:"quilt_loader"`
		}
		if json.Unmarshal(b, &m) == nil {
			candidates = append(candidates, iconField(m.QuiltLoader.Metadata.Icon))
		}
	}
	for _, toml := range []string{"META-INF/neoforge.mods.toml", "META-INF/mods.toml"} {
		if b, ok := read(toml); ok {
			if m := logoFileRE.FindSubmatch(b); m != nil {
				candidates = append(candidates, string(m[1]))
			}
		}
	}
	candidates = append(candidates, "pack.png", "icon.png", "logo.png")
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if b, ok := read(c); ok {
			return b, nil
		}
	}
	return nil, errors.New("no icon")
}

var logoFileRE = regexp.MustCompile(`(?m)^\s*logoFile\s*=\s*["']([^"']+)["']`)

// iconField reads fabric's icon: a path, or {"16": path, "128": path}
// (largest wins).
func iconField(raw json.RawMessage) string {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return one
	}
	var many map[string]string
	if json.Unmarshal(raw, &many) != nil || len(many) == 0 {
		return ""
	}
	best, bestN := "", -1
	for k, v := range many {
		if n, _ := strconv.Atoi(k); n > bestN {
			best, bestN = v, n
		}
	}
	return best
}

func readCapped(p string) ([]byte, error) {
	st, e := os.Stat(p)
	if e != nil {
		return nil, e
	}
	if st.Size() > 4<<20 {
		return nil, errors.New("icon too large")
	}
	return os.ReadFile(p)
}

// VersionIcon returns an instance's own icon when one is on disk: the
// version folder's icon.png (HMCL / imports), PCL's Logo.png, or the
// pack icon MCL saved at import (.mcl/icon.png).
func (s *Service) VersionIcon(rootID, versionID string) ([]byte, error) {
	r, e := s.getRoot(rootID)
	if e != nil {
		return nil, e
	}
	vDir, e := safeJoin(r.Path, "versions/"+versionID)
	if e != nil {
		return nil, e
	}
	for _, p := range []string{
		filepath.Join(vDir, "icon.png"),
		filepath.Join(vDir, "PCL", "Logo.png"),
		filepath.Join(vDir, ".mcl", "icon.png"),
		filepath.Join(r.Path, ".mcl", "icon.png"),
	} {
		if b, e := readCapped(p); e == nil && len(b) > 0 {
			return b, nil
		}
	}
	return nil, errors.New("no icon")
}
