package launcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

type bindingLabel struct {
	Names              map[string]string
	Namespace, ModName string
	ownerScore         int
}

// Retain matching labels for up to four recently opened packs, never entire language
// dictionaries or JAR contents. A changed file, pack order or set of actions
// invalidates this cache; changing an actual key assignment does not.
type bindingLabelCache struct {
	sync.Mutex
	entries map[string]map[string]bindingLabel
	order   []string
}

func (c *bindingLabelCache) get(signature string) map[string]bindingLabel {
	c.Lock()
	defer c.Unlock()
	labels := c.entries[signature]
	if labels != nil {
		c.order = append(slices.DeleteFunc(c.order, func(key string) bool { return key == signature }), signature)
	}
	return labels
}

func (c *bindingLabelCache) put(signature string, labels map[string]bindingLabel) {
	c.Lock()
	defer c.Unlock()
	if c.entries == nil {
		c.entries = map[string]map[string]bindingLabel{}
	}
	c.entries[signature] = labels
	c.order = append(slices.DeleteFunc(c.order, func(key string) bool { return key == signature }), signature)
	for len(c.order) > 4 {
		delete(c.entries, c.order[0])
		c.order = c.order[1:]
	}
}

type bindingLabelSource struct {
	path, namespace, locale string
	size, mtime             int64
	resource                bool
}

func (s *Service) resolveKeyBindingLabels(ctx context.Context, r Root, view *KeyBindingsView) error {
	wanted := map[string]bool{}
	for _, row := range view.Rows {
		if !row.Vanilla {
			wanted[strings.TrimPrefix(row.ID, "key_")] = true
		}
	}
	if len(wanted) == 0 {
		return ctx.Err()
	}
	sources, incomplete := bindingLabelSources(ctx, r.VersionDir(), view.minecraft, view.options)
	if err := ctx.Err(); err != nil {
		return err
	}
	h := sha256.New()
	fmt.Fprintf(h, "%q\n", r.VersionDir())
	for _, key := range slices.Sorted(maps.Keys(wanted)) {
		fmt.Fprintf(h, "%q\n", key)
	}
	for _, source := range sources {
		fmt.Fprintf(h, "%q %d %d %t\n", source.path, source.size, source.mtime, source.resource)
	}
	signature := hex.EncodeToString(h.Sum(nil))
	cache := &s.keyBindingLabels
	var labels map[string]bindingLabel
	if !incomplete {
		labels = cache.get(signature)
	}
	if labels == nil {
		scan := bindingLabelScan{ctx: ctx, wanted: wanted, labels: map[string]bindingLabel{}, owners: map[string]string{}, bytesLeft: 256 << 20, nestedBytesLeft: 256 << 20, nestedLeft: 512, incomplete: incomplete}
		for _, source := range sources {
			if err := ctx.Err(); err != nil {
				return err
			}
			scan.readSource(source)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		for key, label := range scan.labels {
			label.ModName = scan.owners[label.Namespace]
			scan.labels[key] = label
		}
		labels, incomplete = scan.labels, scan.incomplete
		if !incomplete {
			cache.put(signature, labels)
		}
	}
	for i := range view.Rows {
		row := &view.Rows[i]
		if row.Vanilla {
			continue
		}
		label := labels[strings.TrimPrefix(row.ID, "key_")]
		row.Names, row.Namespace, row.ModName = maps.Clone(label.Names), label.Namespace, label.ModName
	}
	view.LabelsIncomplete = incomplete
	return nil
}

func bindingLabelSources(ctx context.Context, dir, minecraft string, options []byte) ([]bindingLabelSource, bool) {
	var sources []bindingLabelSource
	incomplete := false
	add := func(path, namespace, locale string, resource bool) {
		if len(sources) >= 8192 {
			incomplete = true
			return
		}
		st, err := os.Stat(path)
		if err != nil || !st.Mode().IsRegular() {
			incomplete = true
			return
		}
		sources = append(sources, bindingLabelSource{path: path, size: st.Size(), mtime: st.ModTime().UnixNano(), namespace: namespace, locale: locale, resource: resource})
	}
	modDirs := []string{"mods"}
	if minecraft != "" && filepath.Base(minecraft) == minecraft {
		modDirs = append(modDirs, "mods/"+minecraft)
	}
	for _, relative := range modDirs {
		path, err := safeJoin(dir, relative)
		if err != nil {
			incomplete = true
			continue
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			incomplete = incomplete || !os.IsNotExist(err)
			continue
		}
		for _, entry := range entries {
			if ctx.Err() != nil {
				return sources, true
			}
			if !entry.Type().IsRegular() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".jar") {
				continue
			}
			path, err := safeJoin(dir, relative+"/"+entry.Name())
			if err != nil {
				incomplete = true
				continue
			}
			add(path, "", "", false)
		}
	}
	// Resource packs are ordered from lowest to highest priority in options.
	// Older Minecraft versions stored bare filenames instead of file/<name>.
	var packs []string
	for _, line := range strings.Split(strings.TrimPrefix(string(options), "\ufeff"), "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "resourcePacks:"); ok {
			if err := json.Unmarshal([]byte(value), &packs); err != nil {
				incomplete = true
			}
		}
	}
	if len(packs) > 128 {
		packs = packs[len(packs)-128:]
		incomplete = true
	}
	for _, name := range packs {
		if ctx.Err() != nil {
			return sources, true
		}
		external := strings.HasPrefix(name, "file/")
		if !external {
			switch name {
			case "vanilla", "mod_resources", "fabric", "Fabric Mods", "programmer_art", "programer_art", "high_contrast", "server", "world":
				continue
			}
			// Loader-registered packs have runtime identifiers, not files.
			// Their ordinary mod assets were already read from installed JARs.
			if strings.Contains(name, ":") || strings.Contains(name, "/") && !strings.HasSuffix(strings.ToLower(name), ".zip") {
				continue
			}
		}
		name = strings.TrimPrefix(name, "file/")
		if name == "" || name == "." || filepath.Base(name) != name || strings.ContainsAny(name, "/\\") || strings.HasSuffix(strings.ToLower(name), ".disabled") {
			incomplete = true
			continue
		}
		path, err := safeJoin(dir, "resourcepacks/"+name)
		if err != nil {
			incomplete = true
			continue
		}
		st, err := os.Stat(path)
		if err != nil {
			// Legacy packs used bare filenames/folders. A bare name with no
			// matching folder may also be a dynamically registered pack.
			if os.IsNotExist(err) && !external && !strings.HasSuffix(strings.ToLower(name), ".zip") {
				continue
			}
			incomplete = true
			continue
		}
		if !st.IsDir() {
			add(path, "", "", true)
			continue
		}
		assets, err := safeJoin(path, "assets")
		if err != nil {
			incomplete = true
			continue
		}
		namespaces, err := os.ReadDir(assets)
		if err != nil {
			incomplete = incomplete || !os.IsNotExist(err)
			continue
		}
		if len(namespaces) > 4096 {
			namespaces = namespaces[:4096]
			incomplete = true
		}
		for _, ns := range namespaces {
			if !ns.IsDir() {
				continue
			}
			langDir, err := safeJoin(path, "assets/"+ns.Name()+"/lang")
			if err != nil {
				incomplete = true
				continue
			}
			files, err := os.ReadDir(langDir)
			if err != nil {
				incomplete = incomplete || !os.IsNotExist(err)
				continue
			}
			// Legacy files are read first so JSON wins if both are supplied.
			slices.SortStableFunc(files, func(a, b os.DirEntry) int {
				return strings.Compare(strings.ToLower(filepath.Ext(b.Name())), strings.ToLower(filepath.Ext(a.Name())))
			})
			for _, entry := range files {
				if ctx.Err() != nil {
					return sources, true
				}
				namespace, locale := bindingLanguagePath("assets/" + ns.Name() + "/lang/" + entry.Name())
				if locale == "" || !entry.Type().IsRegular() {
					continue
				}
				file, err := safeJoin(path, "assets/"+ns.Name()+"/lang/"+entry.Name())
				if err != nil {
					incomplete = true
					continue
				}
				add(file, namespace, locale, true)
			}
		}
	}
	return sources, incomplete
}

func bindingLanguagePath(name string) (namespace, locale string) {
	parts := strings.Split(name, "/")
	if len(parts) != 4 || parts[0] != "assets" || parts[2] != "lang" || parts[1] == "" || parts[1] == "." || parts[1] == ".." || strings.ContainsAny(name, "\\\x00") {
		return "", ""
	}
	ext := strings.ToLower(filepath.Ext(parts[3]))
	if ext != ".json" && ext != ".lang" {
		return "", ""
	}
	locale = strings.ToLower(strings.TrimSuffix(parts[3], filepath.Ext(parts[3])))
	switch locale {
	case "zh_cn", "zh_tw", "en_us":
		return parts[1], locale
	}
	return "", ""
}
