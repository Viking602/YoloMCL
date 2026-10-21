package launcher

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

// loaderVersion reads the loader build from the version JSON's libraries.
func loaderVersion(v gameVersion, loader, mc string) string {
	for _, lib := range v.Libraries {
		parts := strings.Split(lib.Name, ":")
		if len(parts) < 3 {
			continue
		}
		g, a, ver := parts[0], parts[1], parts[2]
		switch {
		case loader == "fabric" && g == "net.fabricmc" && a == "fabric-loader",
			loader == "quilt" && g == "org.quiltmc" && a == "quilt-loader",
			loader == "neoforge" && g == "net.neoforged" && (a == "neoforge" || a == "forge"):
			return strings.TrimPrefix(ver, mc+"-")
		case loader == "forge" && g == "net.minecraftforge" && (a == "forge" || a == "fmlloader"):
			ver = strings.TrimPrefix(ver, mc+"-")
			if i := strings.Index(ver, "-"); a == "fmlloader" && i > 0 {
				ver = ver[:i]
			}
			return ver
		}
	}
	return ""
}

var snapshotRe = regexp.MustCompile(`^\d{2}w\d{2}[a-z]$|-(pre|rc)\d+$| Pre-Release \d+$`)

// isSnapshotID reports whether a Minecraft id is a snapshot / pre-release.
func isSnapshotID(id string) bool { return snapshotRe.MatchString(id) }

// SetVersionDisplayName stores (or, with "", clears) the name the user
// gave a version. Names are plain text kept in the root's config, so a
// copied data folder shows them on any OS.
func (s *Service) SetVersionDisplayName(rootID, versionID, name string) error {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > 64 {
		return errors.New("名称最多 64 个字符")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, r := range s.cfg.Roots {
		if r.ID != rootID {
			continue
		}
		if s.cfg.Roots[i].VersionConfigs == nil {
			s.cfg.Roots[i].VersionConfigs = map[string]VersionConfig{}
		}
		vc := s.cfg.Roots[i].VersionConfigs[versionID]
		vc.DisplayName = name
		if vc == (VersionConfig{}) {
			delete(s.cfg.Roots[i].VersionConfigs, versionID)
		} else {
			s.cfg.Roots[i].VersionConfigs[versionID] = vc
		}
		return s.saveLocked()
	}
	return errors.New("目录不存在")
}
