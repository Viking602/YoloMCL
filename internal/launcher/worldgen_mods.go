package launcher

import (
	"archive/zip"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/Viking602/YoloMCL/cubiomes/modjar"
)

// WorldgenMod identifies an installed source adapter's input. Only mods used
// by native adapters need a full artifact digest; ordinary resource packs are
// already identified by their effective registry content.
type WorldgenMod struct {
	ID, Version, Loader, SHA512, Source string
	TerraBlender                        bool `json:",omitempty"`
	// After/Before are Forge mods.toml dependency ordering edges (ordering=AFTER/BEFORE).
	After, Before []string `json:",omitempty"`
}

func (c *worldgenCollector) selectedLoader() (string, error) {
	if c.snapshot.Loader != "" {
		_, err := modjar.SelectLoader(c.snapshot.Loader, c.snapshot.Minecraft, c.snapshot.LoaderVersion)
		return c.snapshot.Loader, err
	}
	var selected string
	for _, mod := range c.snapshot.Mods {
		if mod.Loader == "" {
			continue
		}
		if selected != "" && selected != mod.Loader {
			return "", errors.New("mixed mod loaders require an instance profile")
		}
		selected = mod.Loader
	}
	return selected, nil
}

func readWorldgenModConfigs(ctx context.Context, root string, issues *[]WorldgenIssue) map[string][]byte {
	configs := make(map[string][]byte)
	for _, name := range worldgenModConfigFiles(root) {
		file, err := safeJoin(root, "config/"+name)
		var data []byte
		if err == nil {
			data, err = readWorldgenFile(ctx, file, 64<<10)
		}
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			*issues = append(*issues, WorldgenIssue{"source_unreadable", "config/" + name})
			configs[name] = nil
		} else {
			configs[name] = data
		}
	}
	if q := readQuarkTerrablender(root); q != nil {
		configs[quarkTerrablenderKey] = q
	}
	if log := readTerraBlenderRegionLog(root); log != nil {
		configs[terraBlenderRegionLogKey] = log
	}
	return configs
}

func (c *worldgenCollector) readModIdentity(z *zip.Reader, file io.ReaderAt, size int64, source string) error {
	var mod WorldgenMod
	mod.Source = source
	// Multi-loader JARs may retain an unexpanded template for a different
	// loader. Select the active descriptor before parsing; archive entry order
	// must not make a valid NeoForge pack disappear because mods.toml is invalid.
	profile, err := modjar.SelectLoader(c.snapshot.Loader, c.snapshot.Minecraft, c.snapshot.LoaderVersion)
	if err != nil {
		return err
	}
	selected, err := profile.Descriptor(z)
	if err != nil {
		return err
	}
	if profile.LegacyForge() {
		identities, err := modjar.LegacyMods(c.ctx, z, profile)
		if err != nil {
			return err
		}
		var mods []WorldgenMod
		for _, identity := range identities {
			mods = append(mods, WorldgenMod{ID: identity.ID, Version: identity.Version, Loader: "forge", Source: source})
		}
		return c.recordMods(z, file, size, mods)
	}
	var additional []WorldgenMod
	if entry := selected; entry != nil {
		switch entry.Name {
		case "quilt.mod.json":
			raw, err := readZipBytesContext(c.ctx, entry, 2<<20)
			if err != nil {
				return err
			}
			var data struct {
				Schema int                          `json:"schema_version"`
				Loader struct{ ID, Version string } `json:"quilt_loader"`
			}
			if err := json.Unmarshal(raw, &data); err != nil || data.Schema != 1 {
				return fmt.Errorf("invalid Quilt metadata")
			}
			mod.ID, mod.Version, mod.Loader = data.Loader.ID, data.Loader.Version, "quilt"
		case "mcmod.info":
			raw, err := readZipBytesContext(c.ctx, entry, 2<<20)
			if err != nil {
				return err
			}
			var list []struct{ ModID, Version string }
			if err := json.Unmarshal(raw, &list); err != nil {
				var meta struct {
					ModList []struct{ ModID, Version string }
				}
				if err := json.Unmarshal(raw, &meta); err != nil {
					return err
				}
				list = meta.ModList
			}
			if len(list) != 0 {
				mod.ID, mod.Version, mod.Loader = list[0].ModID, list[0].Version, "forge"
			}
		case "fabric.mod.json":
			var data struct {
				ID, Version, Environment string
				Depends                  map[string]json.RawMessage `json:"depends"`
			}
			raw, err := readZipBytesContext(c.ctx, entry, 2<<20)
			if err != nil {
				return err
			}
			raw, ok := worldgenJSONComments(raw)
			if !ok {
				return errors.New("invalid mod JSON comments")
			}
			if err := json.Unmarshal(worldgenJSONControls(raw), &data); err != nil {
				return err
			}
			if data.Environment == "server" {
				return nil // This is the launcher's client/integrated-server profile.
			}
			if data.Environment != "" && data.Environment != "*" && data.Environment != "client" {
				return errors.New("invalid Fabric mod environment")
			}
			mod.ID, mod.Version, mod.Loader = data.ID, data.Version, "fabric"
			_, mod.TerraBlender = data.Depends["terrablender"]
		case "META-INF/mods.toml", "META-INF/neoforge.mods.toml":
			data, err := readZipBytesContext(c.ctx, entry, 64<<10)
			if err != nil {
				return err
			}
			var meta struct {
				Mods []struct {
					ID      string `toml:"modId"`
					Version string `toml:"version"`
				} `toml:"mods"`
				Dependencies any `toml:"dependencies"`
			}
			if err := safeTOML(data, &meta); err != nil {
				return err
			}
			for i, identity := range meta.Mods {
				mod = WorldgenMod{ID: identity.ID, Version: identity.Version, Loader: "forge", Source: source}
				// Loaders ignore dependency tables that are not under this mod's
				// ID. Do not reject valid mod identity/data over an unrelated shape
				// (some shipped add-ons use a bare [[dependencies]] array).
				if tables, ok := meta.Dependencies.(map[string]any); ok {
					raw, _ := json.Marshal(tables[mod.ID])
					var dependencies []struct {
						ModID    string `json:"modId"`
						Ordering string `json:"ordering"`
					}
					if json.Unmarshal(raw, &dependencies) == nil {
						for _, dependency := range dependencies {
							mod.TerraBlender = mod.TerraBlender || dependency.ModID == "terrablender"
							switch strings.ToUpper(dependency.Ordering) {
							case "AFTER":
								mod.After = append(mod.After, dependency.ModID)
							case "BEFORE":
								mod.Before = append(mod.Before, dependency.ModID)
							}
						}
					}
				}
				if strings.Contains(entry.Name, "neoforge") {
					mod.Loader = "neoforge"
				}
				if i+1 < len(meta.Mods) {
					additional = append(additional, mod)
				}
			}
		}
	}
	if mod.ID == "" {
		return nil
	}
	additional = append(additional, mod)
	for i := range additional {
		if profile.Loader != "" {
			additional[i].Loader = profile.Loader
		}
	}
	return c.recordMods(z, file, size, additional)
}

func (c *worldgenCollector) recordMods(z *zip.Reader, file io.ReaderAt, size int64, mods []WorldgenMod) error {
	for i := range mods {
		mod := &mods[i]
		// Forge substitutes this value from the main JAR manifest. It is not a
		// version literal, and filenames are not a reliable replacement for it.
		if mod.Version == "${file.jarVersion}" {
			var err error
			mod.Version, err = worldgenManifestVersion(c.ctx, z)
			if err != nil {
				return err
			}
		}
		if len(mod.ID) > 256 || len(mod.Version) > 256 {
			return errors.New("mod identity exceeds worldgen limits")
		}
		switch mod.ID {
		case "terrablender", "biomesoplenty", "glitchcore", "fabric-api":
			h := sha512.New()
			if _, err := io.Copy(h, modReader{ctx: c.ctx, r: io.NewSectionReader(file, 0, size)}); err != nil {
				return err
			}
			mod.SHA512 = hex.EncodeToString(h.Sum(nil))
		}
	}
	c.snapshot.Mods = append(c.snapshot.Mods, mods...)
	return nil
}

func (c *worldgenCollector) registrationConfig(name string) ([]byte, error) {
	if data, exists := c.snapshot.ModConfigs[name]; exists {
		if data == nil {
			return nil, fmt.Errorf("unreadable configuration %s", name)
		}
		return data, nil
	}
	if c.readConfig == nil {
		return nil, nil
	}
	if len(c.snapshot.ModConfigs) >= 128 {
		return nil, errors.New("too many worldgen configurations")
	}
	data, err := c.readConfig(name)
	if err == nil && len(data) > worldgenModConfigLimit {
		err = errors.New("configuration too large")
	}
	if data == nil && err == nil {
		return nil, nil
	}
	if c.snapshot.ModConfigs == nil {
		c.snapshot.ModConfigs = make(map[string][]byte)
	}
	if err != nil {
		c.snapshot.ModConfigs[name] = nil
		return nil, err
	}
	c.snapshot.ModConfigs[name] = append([]byte{}, data...)
	return data, nil
}

func worldgenManifestVersion(ctx context.Context, z *zip.Reader) (string, error) {
	for _, entry := range z.File {
		if entry.Name != "META-INF/MANIFEST.MF" {
			continue
		}
		data, err := readZipBytesContext(ctx, entry, 64<<10)
		if err != nil {
			return "", err
		}
		// Continuation lines start with one space. Stop at the first empty line:
		// named entry sections may have a different Implementation-Version.
		main, _, _ := strings.Cut(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n\n")
		main = strings.ReplaceAll(main, "\n ", "")
		for _, line := range strings.Split(main, "\n") {
			key, value, ok := strings.Cut(line, ": ")
			if ok && strings.EqualFold(key, "Implementation-Version") {
				return strings.TrimSpace(value), nil
			}
		}
		break
	}
	return "", nil
}

// Config values are source-defined knobs, not code to execute. Missing and
// out-of-range numeric values use the source release's defaults.
type worldgenModSettings map[string]map[string]int64

func parseWorldgenModSettings(snapshot *WorldgenSnapshot, file string, defaults worldgenModSettings) (worldgenModSettings, error) {
	settings := make(worldgenModSettings, len(defaults))
	for section, values := range defaults {
		settings[section] = make(map[string]int64, len(values))
		for key, value := range values {
			settings[section][key] = value
		}
	}
	raw, exists := snapshot.ModConfigs[file]
	if !exists {
		return settings, nil
	}
	if raw == nil {
		return nil, fmt.Errorf("unreadable worldgen configuration %s", file)
	}
	var overrides worldgenModSettings
	if err := toml.Unmarshal(raw, &overrides); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", file, err)
	}
	for section, values := range overrides {
		for key, value := range values {
			if _, ok := settings[section][key]; !ok || value < 0 || value > 2147483647 {
				continue
			}
			if strings.HasSuffix(key, "_size") && (value < 2 || value > 6) {
				continue
			}
			settings[section][key] = value
		}
	}
	return settings, nil
}
