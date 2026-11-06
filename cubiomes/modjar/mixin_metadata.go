package modjar

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type mixinConfig struct {
	Package                 string `json:"package"`
	Mixins, Client, Server  []*string
	Plugin                  string
	Priority, MixinPriority int
	Injectors               struct {
		DefaultRequire int `json:"defaultRequire"`
	}
}

func mixinFile(ctx context.Context, z *zip.Reader, name string) ([]byte, error) {
	var selected *zip.File
	for _, f := range z.File {
		if f.Name == name {
			if selected != nil {
				return nil, fmt.Errorf("duplicate mixin resource %s", name)
			}
			selected = f
		}
	}
	if selected == nil {
		return nil, nil
	}
	return archiveBytes(ctx, selected, 2<<20)
}

// Only loader-declared configs are inputs. A filename containing "mixin" does
// not activate it, and client lists are not part of dedicated-server worldgen.
func declaredMixins(ctx context.Context, z *zip.Reader, options Options) ([]string, error) {
	var result []string
	if options.Loader == "neoforge" {
		profile, err := SelectLoader(options.Loader, options.Minecraft, options.LoaderVersion)
		if err != nil {
			return nil, err
		}
		selected, err := profile.Descriptor(z)
		if err != nil {
			return nil, err
		}
		var raw []byte
		if selected != nil {
			raw, err = archiveBytes(ctx, selected, 2<<20)
			if err != nil {
				return nil, err
			}
		}
		if raw != nil {
			var meta struct {
				Mixins []struct {
					Config       string
					RequiredMods []string
				}
			}
			if err := toml.Unmarshal(raw, &meta); err != nil {
				return nil, err
			}
			for _, m := range meta.Mixins {
				active := true
				for _, id := range m.RequiredMods {
					if _, ok := options.Mods[id]; !ok {
						active = false
					}
				}
				if active && m.Config != "" {
					result = append(result, m.Config)
				}
			}
		}
	}
	if options.Loader == "forge" || options.Loader == "neoforge" {
		raw, err := mixinFile(ctx, z, "META-INF/MANIFEST.MF")
		if err != nil {
			return nil, err
		}
		text := strings.ReplaceAll(string(raw), "\r\n", "\n")
		text = strings.ReplaceAll(text, "\n ", "")
		for line := range strings.SplitSeq(text, "\n") {
			if line == "" {
				break
			}
			key, value, ok := strings.Cut(line, ":")
			if ok && strings.EqualFold(key, "MixinConfigs") {
				for name := range strings.SplitSeq(value, ",") {
					if name = strings.TrimSpace(name); name != "" {
						result = append(result, name)
					}
				}
			}
		}
	}
	if options.Loader == "fabric" || options.Loader == "quilt" {
		name := "fabric.mod.json"
		if options.Loader == "quilt" {
			raw, e := mixinFile(ctx, z, "quilt.mod.json")
			if e != nil {
				return nil, e
			}
			if raw != nil {
				name = "quilt.mod.json"
			}
		}
		raw, err := mixinFile(ctx, z, name)
		if err != nil {
			return nil, err
		}
		if raw != nil {
			raw = loaderJSON(raw)
			var meta map[string]json.RawMessage
			if err = json.Unmarshal(raw, &meta); err != nil {
				return nil, err
			}
			list := meta["mixins"]
			if name == "quilt.mod.json" {
				list = meta["mixin"]
			}
			if len(list) > 0 {
				var entries []json.RawMessage
				if list[0] == '[' {
					if err = json.Unmarshal(list, &entries); err != nil {
						return nil, err
					}
				} else {
					entries = []json.RawMessage{list}
				}
				for _, entry := range entries {
					var config string
					if json.Unmarshal(entry, &config) == nil {
						result = append(result, config)
						continue
					}
					var item struct{ Config, Environment string }
					if err = json.Unmarshal(entry, &item); err != nil {
						return nil, err
					}
					active := item.Environment == "" || item.Environment == "*" || item.Environment == "server" && options.Side != "CLIENT" || item.Environment == "client" && options.Side == "CLIENT"
					if active {
						result = append(result, item.Config)
					}
				}
			}
		}
	}
	seen := map[string]bool{}
	unique := make([]string, 0, len(result))
	for _, name := range result {
		if name == "" || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || strings.Contains(name, "..") {
			return nil, fmt.Errorf("invalid declared mixin resource %q", name)
		}
		if !seen[name] {
			seen[name] = true
			unique = append(unique, name)
		}
	}
	if len(unique) > 4096 {
		return nil, fmt.Errorf("mixin config limit")
	}
	return unique, nil
}

func mixinTargets(a annotation) []string {
	var targets []string
	for _, key := range []string{"value", "targets"} {
		for _, raw := range annotationList(a.values[key]) {
			var name string
			switch x := raw.(type) {
			case classLiteral:
				name = strings.TrimSuffix(strings.TrimPrefix(string(x), "L"), ";")
			case string:
				name = x
			}
			if name != "" {
				targets = append(targets, strings.ReplaceAll(name, ".", "/"))
			}
		}
	}
	return targets
}

func annotationList(value any) []any {
	if value == nil {
		return nil
	}
	if list, ok := value.([]any); ok {
		return list
	}
	return []any{value}
}

// Mixin's Gson reader enables comments, unlike Fabric's strict mod descriptor.
// Whitespace preserves token boundaries and comment-like text inside strings.
func mixinJSON(raw []byte) ([]byte, error) {
	data := loaderJSON(raw)
	out := make([]byte, 0, len(data))
	quoted := false
	for i := 0; i < len(data); i++ {
		ch := data[i]
		if quoted {
			out = append(out, ch)
			if ch == '\\' && i+1 < len(data) {
				i++
				out = append(out, data[i])
			} else if ch == '"' {
				quoted = false
			}
			continue
		}
		if ch == '"' {
			quoted = true
			out = append(out, ch)
			continue
		}
		if ch == '#' || ch == '/' && i+1 < len(data) && data[i+1] == '/' {
			for i < len(data) && data[i] != '\n' {
				i++
			}
			out = append(out, '\n')
			continue
		}
		if ch == '/' && i+1 < len(data) && data[i+1] == '*' {
			i += 2
			for i+1 < len(data) && (data[i] != '*' || data[i+1] != '/') {
				i++
			}
			if i+1 >= len(data) {
				return nil, fmt.Errorf("unterminated mixin JSON comment")
			}
			i++
			out = append(out, ' ')
			continue
		}
		out = append(out, ch)
	}
	return mixinArrayHoles(out), nil
}

// Gson treats omitted array elements (including a trailing comma) as null.
// MixinConfig skips null class names. Object commas retain strict semantics.
func mixinArrayHoles(data []byte) []byte {
	out := make([]byte, 0, len(data))
	stack := []byte{}
	quoted := false
	var previous byte
	for i := 0; i < len(data); i++ {
		ch := data[i]
		if quoted {
			out = append(out, ch)
			if ch == '\\' && i+1 < len(data) {
				i++
				out = append(out, data[i])
			} else if ch == '"' {
				quoted = false
			}
			continue
		}
		if ch == '"' {
			quoted = true
			previous = ch
			out = append(out, ch)
			continue
		}
		if len(stack) > 0 && stack[len(stack)-1] == '[' && (ch == ',' && (previous == '[' || previous == ',') || ch == ']' && previous == ',') {
			out = append(out, "null"...)
		}
		if ch == '[' || ch == '{' {
			stack = append(stack, ch)
		} else if (ch == ']' || ch == '}') && len(stack) > 0 {
			stack = stack[:len(stack)-1]
		}
		out = append(out, ch)
		if ch != ' ' && ch != '\n' && ch != '\r' && ch != '\t' {
			previous = ch
		}
	}
	return out
}
