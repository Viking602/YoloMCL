package modjar

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type LegacyMod struct{ ID, Version, Class string }

// LegacyMods follows FML's annotation discovery. mcmod.info supplements actual
// @Mod declarations; a metadata-only entry does not activate a mod.
func LegacyMods(ctx context.Context, z *zip.Reader, profile LoaderProfile) (mods []LegacyMod, err error) {
	defer func() {
		if p := recover(); p != nil {
			mods, err = nil, fmt.Errorf("legacy mod metadata: %v", p)
		}
	}()
	if !profile.LegacyForge() {
		return nil, fmt.Errorf("not a legacy Forge profile")
	}
	annotationName := "Lnet/minecraftforge/fml/common/Mod;"
	if compareMaven(mavenVersion(profile.Minecraft), mavenVersion("1.8")) < 0 {
		annotationName = "Lcpw/mods/fml/common/Mod;"
	}
	var bytesRead uint64
	for _, f := range z.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !strings.HasSuffix(f.Name, ".class") || strings.HasPrefix(f.Name, "META-INF/") {
			continue
		}
		bytesRead += f.UncompressedSize64
		if bytesRead > 512<<20 {
			return nil, fmt.Errorf("legacy discovery budget")
		}
		data, err := archiveBytes(ctx, f, 2<<20)
		if err != nil {
			return nil, err
		}
		if !bytes.Contains(data, []byte(annotationName)) {
			continue
		}
		c := parseClass(data)
		for _, a := range c.annotations {
			if a.typeName != annotationName {
				continue
			}
			id, _ := a.values["modid"].(string)
			version, _ := a.values["version"].(string)
			if id == "" || len(id) > 64 || id != strings.ToLower(id) {
				return nil, fmt.Errorf("invalid legacy mod identity")
			}
			mods = append(mods, LegacyMod{ID: id, Version: version, Class: c.name})
		}
	}
	if len(mods) == 0 {
		return nil, nil
	}
	metadata := map[string]string{}
	if raw, err := mixinFile(ctx, z, "mcmod.info"); err != nil {
		return nil, err
	} else if raw != nil {
		// FML ignores malformed optional metadata, retaining @Mod declarations.
		if root, err := legacyJSON(raw); err == nil {
			if obj, ok := root.(map[string]any); ok {
				root = obj["modList"]
			}
			if list, ok := root.([]any); ok {
				for _, value := range list {
					if obj, ok := value.(map[string]any); ok {
						metadata[legacyString(obj["modid"])] = legacyString(obj["version"])
					}
				}
			}
		}
	}
	var properties map[string]string
	for i := range mods {
		if mods[i].Version != "" {
			continue
		}
		if properties == nil {
			properties = map[string]string{}
			if raw, err := mixinFile(ctx, z, "version.properties"); err != nil {
				return nil, err
			} else if raw != nil {
				properties, err = readJavaProperties(raw)
				if err != nil {
					return nil, err
				}
			}
		}
		mods[i].Version = properties[mods[i].ID+".version"]
		if mods[i].Version == "" {
			mods[i].Version = metadata[mods[i].ID]
		}
		if mods[i].Version == "" {
			mods[i].Version = "1.0"
		}
	}
	return mods, nil
}

func legacyString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case json.Number:
		return string(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	}
	return ""
}
