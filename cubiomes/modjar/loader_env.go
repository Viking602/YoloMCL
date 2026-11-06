package modjar

import (
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Dedicated-server loader environment: the paths and mod list a Forge,
// NeoForge, or Fabric server exposes after discovery, derived only from the
// installed mod catalogue and the mods' own metadata files.

type loadingMods struct {
	files []*loadingModFile
	byID  map[string]*loadingModFile
}

type loadingModFile struct {
	config map[string]any
	mods   []*loadingModInfo
}

type loadingModInfo struct {
	id, version, display string
	file                 *loadingModFile
	properties           map[string]any
}

func (h *bindings) loaderEnvCall(r reference, receiver any, args []any) (any, bool) {
	v := h.vm
	switch {
	case r.owner == "net/minecraftforge/fml/loading/FMLPaths" && r.name == "get" && r.desc == "()Ljava/nio/file/Path;":
		o, ok := receiver.(*object)
		if !ok || o == nil {
			return nil, false
		}
		// A launched server has absolutePath = gameDir.resolve(relativePath);
		// the emulated game directory is the path root.
		var abs, rel any
		for key, x := range o.fields {
			switch {
			case strings.HasSuffix(key, ".absolutePath:Ljava/nio/file/Path;"):
				abs = x
			case strings.HasSuffix(key, ".relativePath:Ljava/nio/file/Path;"):
				rel = x
			}
		}
		if abs != nil {
			return abs, true
		}
		if _, ok := objectData(rel).(javaPath); !ok {
			return nil, false
		}
		return rel, true
	case r.owner == "net/minecraftforge/fml/loading/FMLPaths" && r.name == "getOrCreateGameRelativePath" && r.desc == "(Ljava/nio/file/Path;)Ljava/nio/file/Path;":
		p, ok := pathArg(args[0])
		if !ok || p.foreign {
			return nil, false
		}
		o := h.overlay()
		for q := p.p; q != "" && q != "."; q = parentPath(q) {
			o.dirs[q] = true
		}
		return args[0], true
	case r.owner == "net/fabricmc/loader/api/FabricLoader" && receiver != nil && r.desc == "()Ljava/nio/file/Path;" && (r.name == "getConfigDir" || r.name == "getGameDir"):
		if r.name == "getConfigDir" {
			return h.newPath(javaPath{p: "config"}), true
		}
		return h.newPath(javaPath{}), true
	case r.owner == "net/fabricmc/loader/api/FabricLoader" && receiver != nil && r.desc == "()Ljava/io/File;" && (r.name == "getConfigDirectory" || r.name == "getGameDirectory"):
		if r.name == "getConfigDirectory" {
			return v.alloc("java/io/File", &javaFile{path: "config"}), true
		}
		return v.alloc("java/io/File", &javaFile{path: "."}), true
	case r.owner == "net/minecraftforge/fml/loading/FMLLoader" && r.name == "getLoadingModList" && r.desc == "()Lnet/minecraftforge/fml/loading/LoadingModList;",
		r.owner == "net/minecraftforge/fml/loading/LoadingModList" && r.name == "get" && r.desc == "()Lnet/minecraftforge/fml/loading/LoadingModList;":
		return v.alloc("net/minecraftforge/fml/loading/LoadingModList", h.loadingModList()), true
	}
	o, ok := receiver.(*object)
	if !ok || o == nil {
		return nil, false
	}
	switch data := o.data.(type) {
	case *loadingMods:
		switch r.name + r.desc {
		case "getMods()Ljava/util/List;":
			var values []any
			for _, f := range data.files {
				for _, m := range f.mods {
					values = append(values, v.alloc("net/minecraftforge/fml/loading/moddiscovery/ModInfo", m))
				}
			}
			return v.alloc("java/util/ArrayList", &javaList{values: values, readonly: true}), true
		case "getModFiles()Ljava/util/List;":
			var values []any
			for _, f := range data.files {
				values = append(values, v.alloc("net/minecraftforge/fml/loading/moddiscovery/ModFileInfo", f))
			}
			return v.alloc("java/util/ArrayList", &javaList{values: values, readonly: true}), true
		case "getModFileById(Ljava/lang/String;)Lnet/minecraftforge/fml/loading/moddiscovery/ModFileInfo;":
			f := data.byID[javaString(args[0])]
			if f == nil {
				return nil, true
			}
			return v.alloc("net/minecraftforge/fml/loading/moddiscovery/ModFileInfo", f), true
		case "getErrors()Ljava/util/List;":
			return v.alloc("java/util/ArrayList", &javaList{readonly: true}), true
		}
	case *loadingModFile:
		switch r.name + r.desc {
		case "getMods()Ljava/util/List;":
			var values []any
			for _, m := range data.mods {
				values = append(values, v.alloc("net/minecraftforge/fml/loading/moddiscovery/ModInfo", m))
			}
			return v.alloc("java/util/ArrayList", &javaList{values: values, readonly: true}), true
		case "getConfigElement([Ljava/lang/String;)Ljava/util/Optional;":
			keys, ok := args[0].(*array)
			if !ok {
				return nil, false
			}
			var x any = data.config
			for _, k := range keys.values {
				m, ok := x.(map[string]any)
				if !ok {
					x = nil
					break
				}
				x = m[javaString(k)]
			}
			if x == nil {
				return v.alloc("java/util/Optional", javaOptional{}), true
			}
			return v.alloc("java/util/Optional", javaOptional{h.tomlValue(x, true)}), true
		}
	case *loadingModInfo:
		switch r.name + r.desc {
		case "getModId()Ljava/lang/String;":
			return data.id, true
		case "getDisplayName()Ljava/lang/String;":
			return data.display, true
		case "getOwningFile()Lnet/minecraftforge/fml/loading/moddiscovery/ModFileInfo;", "getOwningFile()Lnet/minecraftforge/forgespi/language/IModFileInfo;", "getOwningFile()Lnet/minecraftforge/fml/loading/moddiscovery/IModFileInfo;":
			return v.alloc("net/minecraftforge/fml/loading/moddiscovery/ModFileInfo", data.file), true
		case "getModProperties()Ljava/util/Map;":
			return h.tomlValue(data.properties, true), true
		case "getVersion()Lorg/apache/maven/artifact/versioning/ArtifactVersion;":
			return v.inert("ModInfo.getVersion "+data.id, true), true
		}
	}
	return nil, false
}

func parentPath(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i]
	}
	return ""
}

// tomlValue converts a mods.toml value the way FML's NightConfigWrapper
// exposes it: a table read through getConfigElement is its value map, scalars
// are boxed, arrays are lists. Nested tables are night-config objects the
// host does not model, so they stay undecidable.
func (h *bindings) tomlValue(x any, top bool) any {
	v := h.vm
	switch t := x.(type) {
	case string:
		return t
	case bool:
		if t {
			return v.alloc("java/lang/Boolean", int32(1))
		}
		return v.alloc("java/lang/Boolean", int32(0))
	case int64:
		if t >= -1<<31 && t < 1<<31 {
			return v.alloc("java/lang/Integer", int32(t))
		}
		return v.alloc("java/lang/Long", t)
	case float64:
		return v.alloc("java/lang/Double", t)
	case []any:
		values := make([]any, len(t))
		for i, e := range t {
			values[i] = h.tomlValue(e, false)
		}
		return v.alloc("java/util/ArrayList", &javaList{values: values})
	case map[string]any:
		if !top {
			return v.inert("mods.toml nested table", true)
		}
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		m := &javaMap{}
		for _, k := range keys {
			m.entries = append(m.entries, javaPair{k, h.tomlValue(t[k], false)})
		}
		return v.alloc("java/util/HashMap", m)
	case nil:
		return nil
	}
	return v.inert("mods.toml value", true)
}

func (h *bindings) loadingModList() *loadingMods {
	if h.options.Mods == nil {
		fail("installed mod catalogue is required")
	}
	if h.options.Resources == nil {
		fail("mod metadata classpath is required")
	}
	out := &loadingMods{byID: map[string]*loadingModFile{}}
	// Each loader reads only its own descriptor: Forge reads mods.toml, and
	// NeoForge reads neoforge.mods.toml (mods.toml only on the 1.20.1 line,
	// where no neoforge.mods.toml exists). Foreign descriptors are never parsed.
	names := []string{"META-INF/mods.toml"}
	if h.options.Loader == "neoforge" {
		names = []string{"META-INF/neoforge.mods.toml"}
	}
	for i := 0; i < len(names); i++ {
		name := names[i]
		docs, err := h.options.Resources(name)
		if err != nil {
			fail("mod metadata %s: %v", name, err)
		}
		if name == "META-INF/neoforge.mods.toml" && len(docs) == 0 {
			names = append(names, "META-INF/mods.toml")
		}
		for _, raw := range docs {
			var doc map[string]any
			if err := toml.Unmarshal(raw, &doc); err != nil {
				fail("mod metadata %s: %v", name, err)
			}
			file := &loadingModFile{config: doc}
			mods, _ := doc["mods"].([]any)
			for _, entry := range mods {
				m, _ := entry.(map[string]any)
				id, _ := m["modId"].(string)
				version, installed := h.options.Mods[id]
				if id == "" || !installed || out.byID[id] != nil {
					continue
				}
				display, _ := m["displayName"].(string)
				if display == "" {
					display = id
				}
				props, _ := doc["modproperties"].(map[string]any)
				own, _ := props[id].(map[string]any)
				if own == nil {
					own = map[string]any{}
				}
				info := &loadingModInfo{id: id, version: version, display: display, file: file, properties: own}
				file.mods = append(file.mods, info)
				out.byID[id] = file
			}
			if len(file.mods) > 0 {
				out.files = append(out.files, file)
			}
		}
	}
	return out
}
