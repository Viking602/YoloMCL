package launcher

import (
	"encoding/json"
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
	switch loader {
	case "neoforge":
		return versionArgument(v, "--fml.neoForgeVersion", "-Dfml.neoForgeVersion")
	case "forge":
		return versionArgument(v, "--fml.forgeVersion", "-Dfml.forgeVersion")
	}
	return ""
}

// Merged launcher profiles can rename id and remove inheritsFrom while the
// actual Minecraft/loader builds remain in FML arguments. Read both modern
// argument arrays (including conditional values) and the legacy string.
func versionArgument(v gameVersion, names ...string) string {
	args := strings.Fields(v.MinecraftArguments)
	var appendValue func(json.RawMessage)
	appendValue = func(raw json.RawMessage) {
		var text string
		if json.Unmarshal(raw, &text) == nil {
			args = append(args, text)
			return
		}
		var values []json.RawMessage
		if json.Unmarshal(raw, &values) == nil {
			for _, value := range values {
				appendValue(value)
			}
			return
		}
		var conditional struct {
			Value json.RawMessage `json:"value"`
		}
		if json.Unmarshal(raw, &conditional) == nil && len(conditional.Value) > 0 {
			appendValue(conditional.Value)
		}
	}
	for _, raw := range append(append([]json.RawMessage{}, v.Arguments.Game...), v.Arguments.JVM...) {
		appendValue(raw)
	}
	for i := len(args) - 1; i >= 0; i-- {
		for _, name := range names {
			if args[i] == name && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				return args[i+1]
			}
			if value, ok := strings.CutPrefix(args[i], name+"="); ok && value != "" {
				return value
			}
		}
	}
	return ""
}

func minecraftVersionOf(v gameVersion) string {
	if mc := versionArgument(v, "--fml.mcVersion", "--fml.minecraftVersion", "-Dfml.mcVersion"); mc != "" {
		return mc
	}
	if v.ClientVersion != "" {
		return v.ClientVersion
	}
	if v.InheritsFrom != "" {
		return v.InheritsFrom
	}
	for _, lib := range v.Libraries {
		parts := strings.Split(lib.Name, ":")
		if len(parts) >= 3 && (parts[0] == "net.minecraft" && parts[1] == "client" || parts[0] == "net.fabricmc" && parts[1] == "intermediary") {
			return parts[2]
		}
	}
	if v.Jar != "" {
		return v.Jar
	}
	return v.ID
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
