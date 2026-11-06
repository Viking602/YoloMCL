package modjar

import (
	"archive/zip"
	"encoding/json"
	"strings"
)

// Loader entrypoints are separate from framework-specific extension hooks.
// Initialization must happen before TerraBlender consumers are invoked.
func discoverJSONEntrypoints(v *machine, z *zip.Reader, options Options) registrationPlan {
	profile, err := SelectLoader(options.Loader, options.Minecraft, options.LoaderVersion)
	if err != nil {
		fail("entrypoint metadata: %v", err)
	}
	selected, err := profile.Descriptor(z)
	if err != nil {
		fail("entrypoint metadata: %v", err)
	}
	if selected == nil {
		fail("missing %s entrypoint metadata", options.Loader)
	}
	data, err := archiveBytes(v.ctx, selected, 2<<20)
	if err != nil {
		fail("entrypoint metadata: %v", err)
	}
	data = loaderJSON(data)
	var id string
	entries := map[string]json.RawMessage{}
	quilt := selected.Name == "quilt.mod.json"
	if quilt {
		var m struct {
			Schema int `json:"schema_version"`
			Loader struct {
				ID          string                     `json:"id"`
				Entrypoints map[string]json.RawMessage `json:"entrypoints"`
			} `json:"quilt_loader"`
		}
		if err := json.Unmarshal(data, &m); err != nil || m.Schema != 1 {
			fail("unsupported Quilt entrypoint metadata")
		}
		id, entries = m.Loader.ID, m.Loader.Entrypoints
	} else {
		var m struct {
			Schema      int                        `json:"schemaVersion"`
			ID          string                     `json:"id"`
			Entrypoints map[string]json.RawMessage `json:"entrypoints"`
			Environment string                     `json:"environment"`
		}
		if err := json.Unmarshal(data, &m); err != nil || m.Schema != 1 {
			fail("unsupported Fabric entrypoint metadata")
		}
		id, entries = m.ID, m.Entrypoints
		if id == "" || id != options.ModID {
			fail("entrypoint mod identity mismatch")
		}
		if m.Environment != "" && m.Environment != "*" && m.Environment != "client" && m.Environment != "server" {
			fail("invalid mod environment")
		}
		if m.Environment == "client" && options.Side == "DEDICATED_SERVER" || m.Environment == "server" && options.Side != "DEDICATED_SERVER" {
			return registrationPlan{}
		}
	}
	if id == "" || id != options.ModID {
		fail("entrypoint mod identity mismatch")
	}
	appendEntries := func(dst *[]registrationEntry, key, iface, method, desc string) {
		raw := entries[key]
		if raw == nil {
			return
		}
		var list []json.RawMessage
		if err := json.Unmarshal(raw, &list); err != nil {
			if !quilt {
				fail("Fabric entrypoints must be an array")
			}
			list = []json.RawMessage{raw}
		}
		if len(list) > 1024 {
			fail("entrypoint count limit")
		}
		for _, raw := range list {
			var value string
			if json.Unmarshal(raw, &value) != nil {
				var e struct{ Adapter, Value string }
				if json.Unmarshal(raw, &e) != nil || e.Adapter != "" && e.Adapter != "default" {
					fail("unbound loader language adapter")
				}
				value = e.Value
			}
			*dst = append(*dst, defaultEntrypoint(v, value, iface, method, desc))
		}
	}
	var p registrationPlan
	if !options.fullLifecycle {
		appendEntries(&p.after, "terrablender", "terrablender/api/TerraBlenderApi", "onTerraBlenderInitialized", "()V")
		return p
	}
	if quilt {
		appendEntries(&p.pre, "pre_launch", "org/quiltmc/loader/api/entrypoint/PreLaunchEntrypoint", "onPreLaunch", "(Lorg/quiltmc/loader/api/ModContainer;)V")
		appendEntries(&p.before, "init", "org/quiltmc/qsl/base/api/entrypoint/ModInitializer", "onInitialize", "(Lorg/quiltmc/loader/api/ModContainer;)V")
		if options.Side == "DEDICATED_SERVER" {
			appendEntries(&p.setup, "server_init", "org/quiltmc/qsl/base/api/entrypoint/server/DedicatedServerModInitializer", "onInitializeServer", "(Lorg/quiltmc/loader/api/ModContainer;)V")
		} else {
			appendEntries(&p.setup, "client_init", "org/quiltmc/qsl/base/api/entrypoint/client/ClientModInitializer", "onInitializeClient", "(Lorg/quiltmc/loader/api/ModContainer;)V")
		}
	} else {
		appendEntries(&p.pre, "preLaunch", "net/fabricmc/loader/api/entrypoint/PreLaunchEntrypoint", "onPreLaunch", "()V")
		appendEntries(&p.before, "main", "net/fabricmc/api/ModInitializer", "onInitialize", "()V")
		if options.Side == "DEDICATED_SERVER" {
			appendEntries(&p.setup, "server", "net/fabricmc/api/DedicatedServerModInitializer", "onInitializeServer", "()V")
		} else {
			appendEntries(&p.setup, "client", "net/fabricmc/api/ClientModInitializer", "onInitializeClient", "()V")
		}
	}
	appendEntries(&p.after, "terrablender", "terrablender/api/TerraBlenderApi", "onTerraBlenderInitialized", "()V")
	return p
}

// Implements the default loader adapter's class, static-field and method
// forms. Unsupported overload adaptation fails instead of picking a method.
func defaultEntrypoint(v *machine, value, iface, name, desc string) registrationEntry {
	parts := strings.Split(value, "::")
	if len(parts) > 2 || parts[0] == "" {
		fail("invalid loader entrypoint %q", value)
	}
	owner := strings.ReplaceAll(parts[0], ".", "/")
	c := v.load(owner)
	if len(parts) == 1 {
		if !v.instanceOf(&object{class: owner}, iface) {
			fail("entrypoint %s does not implement %s", owner, iface)
		}
		return registrationEntry{ref: reference{owner, name, desc}, instance: true}
	}
	var candidates []method
	for _, m := range c.methods {
		if m.name == parts[1] {
			candidates = append(candidates, m)
		}
	}
	for _, f := range c.fields {
		if f.name != parts[1] {
			continue
		}
		if len(candidates) != 0 || f.access&9 != 9 || !strings.HasPrefix(f.desc, "L") || !v.instanceOf(&object{class: strings.TrimSuffix(f.desc[1:], ";")}, iface) {
			fail("invalid or ambiguous entrypoint field %s", value)
		}
		field := reference{owner, f.name, f.desc}
		return registrationEntry{ref: reference{iface, name, desc}, field: &field}
	}
	if len(candidates) != 1 || candidates[0].desc != desc || candidates[0].access&1 == 0 || parts[1] == "<init>" {
		fail("unbound or ambiguous entrypoint method %s", value)
	}
	m := candidates[0]
	return registrationEntry{ref: reference{owner, m.name, m.desc}, instance: m.access&8 == 0}
}
