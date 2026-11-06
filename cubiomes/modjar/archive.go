package modjar

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
)

// archiveMachine shares the bounded, lazy classpath across worldgen API hosts.
func archiveMachine(ctx context.Context, z *zip.Reader, options Options) (*machine, []string, error) {
	entries := map[string]*zip.File{}
	var total uint64
	for _, entry := range z.File {
		if !strings.HasSuffix(entry.Name, ".class") || strings.HasPrefix(entry.Name, "META-INF/") {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if len(entries) >= 65536 || entry.UncompressedSize64 > 2<<20 {
			return nil, nil, fmt.Errorf("registration archive exceeds class limits")
		}
		total += entry.UncompressedSize64
		if total > 256<<20 {
			return nil, nil, fmt.Errorf("registration archive exceeds byte limits")
		}
		name := strings.TrimSuffix(entry.Name, ".class")
		if _, exists := entries[name]; exists {
			return nil, nil, fmt.Errorf("duplicate class %s", name)
		}
		entries[name] = entry
	}
	// Keep only the index up front. Unrelated items, entities and UI classes in
	// a large mod must not consume the registration VM's 16 MiB working budget.
	v := newMachine(ctx, nil)
	v.reflectionOpen = options.ReflectionOpen
	if v.reflectionOpen == nil {
		explicitModule := false
		for _, entry := range z.File {
			if entry.Name == "module-info.class" || strings.HasPrefix(entry.Name, "META-INF/versions/") && strings.HasSuffix(entry.Name, "/module-info.class") {
				explicitModule = true
			}
		}
		v.reflectionOpen = func(name string) (bool, error) {
			if err := ctx.Err(); err != nil {
				return false, err
			}
			if entries[name] == nil {
				return false, fmt.Errorf("dependency reflection requires classpath module metadata: %s", name)
			}
			return !explicitModule || options.Loader == "fabric" || options.Loader == "quilt", nil
		}
	}
	readLocal := func(name string) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entry := entries[name]
		if entry == nil {
			return nil, nil
		}
		stream, err := entry.Open()
		if err != nil {
			return nil, err
		}
		defer stream.Close()
		data, err := io.ReadAll(io.LimitReader(stream, (2<<20)+1))
		if len(data) > 2<<20 {
			return nil, fmt.Errorf("registration class size limit")
		}
		return data, err
	}
	v.inspectSource = readLocal
	v.resolveClass = func(name string) ([]byte, error) {
		local, err := readLocal(name)
		if err != nil {
			return nil, err
		}
		if options.Class == nil {
			return local, nil
		}
		dependency, err := options.Class(name)
		if err != nil {
			return nil, err
		}
		if local != nil && dependency != nil && !bytes.Equal(local, dependency) {
			return nil, fmt.Errorf("conflicting local/classpath definition: %s", name)
		}
		if local != nil {
			return local, nil
		}
		return dependency, nil
	}
	h := &bindings{vm: v, options: options}
	v.host = h
	var names []string
	for name := range entries {
		names = append(names, name)
	}
	slices.Sort(names)
	return v, names, nil
}
