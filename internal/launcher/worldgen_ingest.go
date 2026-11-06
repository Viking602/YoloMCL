package launcher

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/Viking602/YoloMCL/cubiomes/modjar"
)

const (
	// Mods can inline thousands of climate entries in a dimension resource;
	// C6C's ordinary formatted Overworld definition alone is over 4 MiB.
	maxWorldgenJSON           = 16 << 20
	maxWorldgenTotal          = 192 << 20 // large packs (VEFC) carry several 4 MiB slice files
	maxWorldgenEntries        = 32768
	maxWorldgenArchiveEntries = 131072
)

type worldgenCollector struct {
	ctx                            context.Context
	snapshot                       *WorldgenSnapshot
	documents                      map[string]WorldgenDocument
	packBase                       map[string]WorldgenDocument
	format                         *WorldgenPackFormat
	bytes, entries                 int
	languageBytes, languageEntries int
	readConfig                     func(string) ([]byte, error)
	libraries                      []WorldgenSource
	symbols                        *modjar.Symbols
	classPathErr                   error
	// registrationSources limits the registration classpath/mixin weave to
	// worldgen-relevant mod sources. nil means include every catalogue mod
	// (tests that build a collector by hand).
	registrationSources map[string]bool
	// registrationConfigParallelOff forces sequential registration (tests).
	registrationConfigParallelOff bool
	// prefetch carries speculative per-archive results from ingest.sources.
	prefetch *worldgenPrefetcher
}

func worldgenResource(name string) (kind, id string, ok bool) {
	if path.Clean(name) != name || strings.ContainsAny(name, "\\:\x00") || !strings.HasSuffix(name, ".json") {
		return
	}
	parts := strings.Split(strings.TrimSuffix(name, ".json"), "/")
	if len(parts) == 4 && parts[0] == "assets" && parts[1] != "" && parts[2] == "lang" && (parts[3] == "en_us" || parts[3] == "zh_cn") {
		return "language", parts[1] + ":" + parts[3], true
	}
	if len(parts) < 4 || parts[0] != "data" || parts[1] == "" {
		return
	}
	switch parts[2] {
	case "lithostitched":
		if len(parts) < 5 {
			return
		}
		switch parts[3] {
		case "biome_injector", "region", "fast_noise_config", "worldgen_modifier":
			kind, id = "lithostitched_"+parts[3], parts[1]+":"+strings.Join(parts[4:], "/")
		default:
			return
		}
	case "worldgen":
		if len(parts) < 5 {
			return
		}
		kind, id = parts[3], parts[1]+":"+strings.Join(parts[4:], "/")
	case "dimension", "dimension_type":
		kind, id = parts[2], parts[1]+":"+strings.Join(parts[3:], "/")
	case "blueprint":
		// Blueprint datapack registry: data/<ns>/blueprint/modded_biome_slices/<id>.json
		if len(parts) < 5 || parts[3] != "modded_biome_slices" {
			return
		}
		kind, id = "modded_biome_slice", parts[1]+":"+strings.Join(parts[4:], "/")
	case "tags":
		if len(parts) >= 5 && parts[3] == "dimension_type" {
			return "dimension_type_tag", parts[1] + ":" + strings.Join(parts[4:], "/"), true
		}
		if len(parts) >= 6 && parts[3] == "worldgen" && parts[4] == "structure_set" {
			return "structure_set_tag", parts[1] + ":" + strings.Join(parts[5:], "/"), true
		}
		if len(parts) < 6 || parts[3] != "worldgen" || parts[4] != "biome" {
			return
		}
		kind, id = "biome_tag", parts[1]+":"+strings.Join(parts[5:], "/")
	default:
		return
	}
	return kind, id, true
}

func (c *worldgenCollector) accept(name, source string, data []byte, readErr error) {
	kind, id, ok := worldgenResource(name)
	if !ok {
		return
	}
	key := kind + "/" + id
	previous := c.documents[key].Data
	if c.packBase != nil {
		previous = c.packBase[key].Data
	}
	// A broken high-priority entry must not silently resurrect vanilla data.
	doc := WorldgenDocument{ID: id, Kind: kind, Source: source, Data: json.RawMessage(`null`)}
	if readErr == nil {
		var ok bool
		data, ok = worldgenJSONComments(data)
		if !ok {
			readErr = errors.New("invalid worldgen JSON comments")
		} else {
			data = worldgenJSONControls(data)
			data = worldgenJSONArrayElisions(data)
		}
	}
	if readErr == nil && boundedWorldgenJSON(data) {
		doc.Data = json.RawMessage(data)
		if kind == "language" {
			merged, err := seedMapLanguage(previous, data)
			if err != nil {
				c.snapshot.issue("invalid_json", source+":"+name)
				doc.Data = json.RawMessage(`null`)
			} else {
				doc.Data = merged
			}
		}
		if kind == "biome_tag" || kind == "dimension_type_tag" || kind == "structure_set_tag" {
			merged, err := mergeSeedMapTag(previous, doc.Data)
			if err != nil {
				c.snapshot.issue("invalid_json", source+":"+name)
				doc.Data = json.RawMessage(`null`)
			} else {
				doc.Data = merged
			}
		}
	} else {
		c.snapshot.issue("invalid_json", source+":"+name)
	}
	c.documents[key] = doc
}

// Gson's quoted-string reader accepts literal line breaks/control characters.
// Preserve their values by escaping them for encoding/json. Never rewrite
// controls outside strings or alter already escaped content.
func worldgenJSONControls(data []byte) []byte {
	var out []byte
	quoted, escaped := false, false
	start := 0
	for i, ch := range data {
		if escaped {
			escaped = false
			continue
		}
		if quoted && ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			quoted = !quoted
			continue
		}
		if quoted && ch < 32 {
			out = append(out, data[start:i]...)
			out = append(out, '\\', 'u', '0', '0', "0123456789abcdef"[ch>>4], "0123456789abcdef"[ch&15])
			start = i + 1
		}
	}
	if out == nil {
		return data
	}
	return append(out, data[start:]...)
}

// Minecraft's Gson resource reader permits comments. Replace them with
// whitespace without touching quoted strings or joining adjacent tokens.
// Other malformed JSON still fails the normal bounded decoder below.
func worldgenJSONComments(data []byte) ([]byte, bool) {
	if len(data) > maxWorldgenJSON {
		return nil, false
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	out := data
	quoted, escaped, copied := false, false, false
	blank := func(i int) {
		if !copied {
			out = bytes.Clone(data)
			copied = true
		}
		if out[i] != '\r' && out[i] != '\n' {
			out[i] = ' '
		}
	}
	for i := 0; i < len(data); i++ {
		ch := data[i]
		if quoted {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				quoted = false
			}
			continue
		}
		if ch == '"' {
			quoted = true
			continue
		}
		if ch == '#' || ch == '/' && i+1 < len(data) && data[i+1] == '/' {
			for i < len(data) && data[i] != '\n' && data[i] != '\r' {
				blank(i)
				i++
			}
		} else if ch == '/' && i+1 < len(data) && data[i+1] == '*' {
			blank(i)
			i++
			blank(i)
			i++
			for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
				blank(i)
				i++
			}
			if i+1 >= len(data) {
				return nil, false
			}
			blank(i)
			i++
			blank(i)
		}
	}
	return out, true
}

func boundedWorldgenJSON(data []byte) bool {
	if len(data) > maxWorldgenJSON {
		return false
	}
	depth, members, quoted, escaped := 0, 0, false, false
	for _, ch := range data {
		if quoted {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				quoted = false
			}
			continue
		}
		switch ch {
		case ',', ':':
			members++
			if members > 1<<21 {
				return false
			}
		case '"':
			quoted = true
		case '{', '[':
			depth++
			if depth > 64 {
				return false
			}
		case '}', ']':
			depth--
		}
	}
	return json.Valid(data)
}

// Gson's lenient resource reader treats absent array elements as null, including
// [a,] and [,a]. Do not remove those elements: that would shift feature steps.
// Object trailing commas remain errors. All normal JSON limits still apply.
func worldgenJSONArrayElisions(data []byte) []byte {
	if json.Valid(data) {
		return data
	}
	var out []byte
	var stack [64]byte
	depth, copied := 0, 0
	quoted, escaped := false, false
	var previous byte
	for i, ch := range data {
		if quoted {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				quoted = false
			}
			continue
		}
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' {
			continue
		}
		if depth > 0 && stack[depth-1] == '[' && (ch == ',' && (previous == '[' || previous == ',') || ch == ']' && previous == ',') {
			out = append(out, data[copied:i]...)
			out = append(out, "null"...)
			copied = i
			if len(out) > maxWorldgenJSON {
				return nil
			}
		}
		switch ch {
		case '"':
			quoted = true
		case '[', '{':
			if depth == len(stack) {
				return nil
			}
			stack[depth] = ch
			depth++
		case ']', '}':
			if depth == 0 {
				return nil
			}
			depth--
		}
		previous = ch
	}
	if out == nil {
		return data
	}
	return append(out, data[copied:]...)
}

func (c *worldgenCollector) reserve(size int64) error {
	if err := c.ctx.Err(); err != nil {
		return err
	}
	c.entries++
	if size < 0 || size > maxWorldgenJSON || c.entries > maxWorldgenEntries || size > int64(maxWorldgenTotal-c.bytes) {
		return errors.New("worldgen resource budget exceeded")
	}
	c.bytes += int(size)
	return nil
}

func (c *worldgenCollector) reserveResource(name string, size int64) error {
	if kind, _, _ := worldgenResource(name); kind == "language" {
		// Translation bundles may cover thousands of unrelated mods. Their
		// bounded parsing budget must not consume world-generation capacity;
		// only map labels survive seedMapLanguage's filtering.
		c.languageEntries++
		if size < 0 || size > maxWorldgenJSON || c.languageEntries > 8192 || size > int64((64<<20)-c.languageBytes) {
			return errors.New("worldgen language budget exceeded")
		}
		c.languageBytes += int(size)
		return nil
	}
	return c.reserve(size)
}

func (c *worldgenCollector) readSource(source WorldgenSource) error {
	return c.readSourcePrefetched(source, nil)
}

func (c *worldgenCollector) zipBytes(pf *zipPrefetch, entry *zip.File) ([]byte, error) {
	if pf != nil {
		if data, ok := pf.data[entry]; ok {
			if err := c.ctx.Err(); err != nil {
				return nil, err
			}
			return data, nil
		}
	}
	return readZipBytesContext(c.ctx, entry, maxWorldgenJSON)
}

func (c *worldgenCollector) readSourcePrefetched(source WorldgenSource, pf *zipPrefetch) error {
	defer pf.close()
	c.packBase = make(map[string]WorldgenDocument)
	for key, doc := range c.documents {
		if doc.Kind == "biome_tag" || doc.Kind == "dimension_type_tag" || doc.Kind == "structure_set_tag" || doc.Kind == "language" {
			c.packBase[key] = doc
		}
	}
	defer func() { c.packBase = nil }()
	var (
		info os.FileInfo
		f    *os.File
		z    *zip.Reader
		err  error
	)
	if pf != nil {
		info, f, z = pf.info, pf.f, pf.z
	} else {
		info, err = os.Lstat(source.Path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("worldgen source is a symlink")
		}
		if info.IsDir() {
			return c.readDirectory(source)
		}
		if !info.Mode().IsRegular() {
			return errors.New("worldgen source is not a regular file")
		}
		f, err = os.Open(source.Path)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := worldgenZipEnvelope(f, info.Size()); err != nil {
			return err
		}
		z, err = zip.NewReader(f, info.Size())
		if err != nil {
			return err
		}
		if len(z.File) > maxWorldgenArchiveEntries {
			return errors.New("too many archive entries")
		}
	}
	if strings.HasPrefix(source.Name, "mod:") {
		profile, err := modjar.SelectLoader(c.snapshot.Loader, c.snapshot.Minecraft, c.snapshot.LoaderVersion)
		if err != nil {
			return err
		}
		selected, err := profile.Descriptor(z)
		if err != nil {
			return err
		}
		if selected == nil && c.snapshot.Loader != "" && !profile.LegacyForge() {
			library, err := profile.Library(c.ctx, z)
			if err != nil {
				return err
			}
			if library {
				c.libraries = append(c.libraries, source)
			}
			return nil // Neither inactive files nor declared libraries are mod resource packs.
		}
		before := len(c.snapshot.Mods)
		if err := c.readModIdentity(z, f, info.Size(), source.Name); err != nil {
			return err
		}
		if len(c.snapshot.Mods) == before && c.snapshot.Loader != "" {
			return nil
		}
	}
	if strings.HasPrefix(source.Name, tectonicEmbeddedSource) {
		z = &zip.Reader{File: embeddedPackFiles(z.File, tectonicPackRoot)}
	}
	// ZIP order is not pack priority; sort for stable hashes and duplicate handling.
	sort.SliceStable(z.File, func(i, j int) bool { return z.File[i].Name < z.File[j].Name })
	var overlays []string
	for _, entry := range z.File {
		if entry.Name == "version.json" && strings.HasPrefix(source.Name, "vanilla:") {
			data, readErr := c.zipBytes(pf, entry)
			if readErr == nil {
				c.readVersionFormat(data)
			}
		}
	}
	for _, entry := range z.File {
		if entry.Name == "pack.mcmeta" {
			data, readErr := c.zipBytes(pf, entry)
			if readErr != nil {
				return readErr
			}
			overlays, err = worldgenPackOverlays(data, c.format)
			if err != nil {
				c.snapshot.issue("pack_metadata", source.Name)
				return err
			}
			if strings.HasPrefix(source.Name, tectonicEmbeddedSource) {
				overlays = append(overlays, tectonicNeoforgeOverlays(data, parseTectonicConfig(c.snapshot.ModConfigs["tectonic.json"]), func(id string) bool { return worldgenHasMod(c.snapshot, id) })...)
			}
		} else if strings.HasPrefix(entry.Name, "resourcepacks/") && strings.HasSuffix(entry.Name, "/pack.mcmeta") {
			// Enabling embedded packs is a loader API decision. Their presence is
			// not authorization to apply them, nor proof that they are disabled.
			c.snapshot.issue("embedded_pack", source.Name+":"+entry.Name)
		}
	}
	for _, prefix := range append([]string{""}, overlays...) {
		for _, entry := range z.File {
			if err := c.ctx.Err(); err != nil {
				return err
			}
			name := entry.Name
			provenance := source.Name
			if prefix != "" {
				if !strings.HasPrefix(name, prefix+"/") {
					continue
				}
				name = strings.TrimPrefix(name, prefix+"/")
				provenance += "#" + prefix
			}
			if _, _, ok := worldgenResource(name); !ok {
				continue
			}
			if err := c.reserveResource(name, int64(entry.UncompressedSize64)); err != nil {
				c.accept(name, provenance, nil, err)
				return err
			}
			if entry.Mode()&os.ModeSymlink != 0 {
				c.accept(name, provenance, nil, errors.New("symlink entry"))
				continue
			}
			data, err := c.zipBytes(pf, entry)
			c.accept(name, provenance, data, err)
		}
	}
	return nil
}

// Bound the central directory before archive/zip allocates its file index.
// ZIP64 uses the same bounded index; resources are read individually, never
// extracted. Large art inventories must not discard a mod's small data pack.
func worldgenZipEnvelope(f *os.File, size int64) error {
	if size < 22 || size > 8<<30 {
		return errors.New("worldgen archive size limit")
	}
	tail := make([]byte, min(size, 65557))
	if _, err := f.ReadAt(tail, size-int64(len(tail))); err != nil {
		return err
	}
	for at := len(tail) - 22; at >= 0; at-- {
		b := tail[at:]
		if !bytes.HasPrefix(b, []byte{'P', 'K', 5, 6}) || 22+int(binary.LittleEndian.Uint16(b[20:22])) != len(b) {
			continue
		}
		entries, central := uint64(binary.LittleEndian.Uint16(b[10:12])), uint64(binary.LittleEndian.Uint32(b[12:16]))
		centralOffset := uint64(binary.LittleEndian.Uint32(b[16:20]))
		end := size - int64(len(tail)) + int64(at)
		if entries == 65535 || central == 0xffffffff || centralOffset == 0xffffffff {
			var locator [20]byte
			if end < 20 {
				return errors.New("invalid ZIP64 locator")
			}
			if _, err := f.ReadAt(locator[:], end-20); err != nil {
				return err
			}
			if binary.LittleEndian.Uint32(locator[:4]) != 0x07064b50 || binary.LittleEndian.Uint32(locator[4:8]) != 0 || binary.LittleEndian.Uint32(locator[16:20]) != 1 {
				return errors.New("invalid ZIP64 disks")
			}
			offset := binary.LittleEndian.Uint64(locator[8:16])
			if offset > uint64(end-20) || uint64(end-20)-offset < 56 {
				return errors.New("invalid ZIP64 bounds")
			}
			var record [56]byte
			if _, err := f.ReadAt(record[:], int64(offset)); err != nil {
				return err
			}
			length := binary.LittleEndian.Uint64(record[4:12])
			if binary.LittleEndian.Uint32(record[:4]) != 0x06064b50 || length < 44 || length != uint64(end-20)-offset-12 || binary.LittleEndian.Uint64(record[16:24]) != 0 {
				return errors.New("invalid ZIP64 directory")
			}
			entries = binary.LittleEndian.Uint64(record[32:40])
			if binary.LittleEndian.Uint64(record[24:32]) != entries {
				return errors.New("split ZIP64 directory")
			}
			central, centralOffset = binary.LittleEndian.Uint64(record[40:48]), binary.LittleEndian.Uint64(record[48:56])
			end = int64(offset)
		}
		if centralOffset > uint64(end) || central != uint64(end)-centralOffset {
			return errors.New("invalid worldgen archive directory bounds")
		}
		if entries > maxWorldgenArchiveEntries || central > 32<<20 || binary.LittleEndian.Uint16(b[4:6]) != 0 || binary.LittleEndian.Uint16(b[6:8]) != 0 {
			return errors.New("worldgen archive directory limit")
		}
		return nil
	}
	return errors.New("invalid worldgen archive")
}

func (c *worldgenCollector) readDirectory(source WorldgenSource) error {
	r, err := os.OpenRoot(source.Path)
	if err != nil {
		return err
	}
	defer r.Close()
	var overlays []string
	if file, openErr := r.Open("pack.mcmeta"); openErr == nil {
		data, readErr := io.ReadAll(io.LimitReader(modReader{ctx: c.ctx, r: file}, maxWorldgenJSON+1))
		file.Close()
		if readErr != nil {
			return readErr
		}
		overlays, err = worldgenPackOverlays(data, c.format)
		if err != nil {
			c.snapshot.issue("pack_metadata", source.Name)
			return err
		}
	} else if !errors.Is(openErr, fs.ErrNotExist) {
		return openErr
	}
	prefix, provenance := "", source.Name
	visited := 0
	var walk func(string, int) error
	walk = func(dir string, depth int) error {
		if err := c.ctx.Err(); err != nil {
			return err
		}
		if depth > 32 {
			return errors.New("worldgen directory depth limit")
		}
		f, err := r.Open(dir)
		if errors.Is(err, fs.ErrNotExist) && dir == "data" {
			return nil
		}
		if err != nil {
			return err
		}
		entries, err := f.ReadDir(maxWorldgenEntries + 1)
		f.Close()
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		visited += len(entries)
		if visited > maxWorldgenEntries {
			return errors.New("worldgen directory entry limit")
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, en := range entries {
			if err := c.ctx.Err(); err != nil {
				return err
			}
			fileName := path.Join(dir, en.Name())
			name := strings.TrimPrefix(fileName, prefix)
			if en.Type()&os.ModeSymlink != 0 {
				c.snapshot.issue("source_unreadable", source.Name+":"+name)
				continue
			}
			if en.IsDir() {
				// Ignore functions, loot tables, textures and region trees entirely.
				parts := strings.Split(name, "/")
				if parts[0] == "assets" && len(parts) >= 3 && parts[2] != "lang" {
					continue
				}
				if parts[0] == "data" && len(parts) == 3 && parts[2] != "worldgen" && parts[2] != "dimension" && parts[2] != "dimension_type" && parts[2] != "tags" && parts[2] != "lithostitched" {
					continue
				}
				if len(parts) == 4 && parts[2] == "tags" && parts[3] != "worldgen" && parts[3] != "dimension_type" {
					continue
				}
				if len(parts) == 5 && parts[2] == "tags" && parts[3] == "worldgen" && parts[4] != "biome" {
					continue
				}
				if err := walk(fileName, depth+1); err != nil {
					return err
				}
				continue
			}
			if _, _, ok := worldgenResource(name); !ok {
				continue
			}
			info, err := en.Info()
			if err != nil {
				c.accept(name, provenance, nil, err)
				continue
			}
			if err := c.reserveResource(name, info.Size()); err != nil {
				c.accept(name, provenance, nil, err)
				return err
			}
			file, err := r.Open(fileName)
			if err != nil {
				c.accept(name, provenance, nil, err)
				continue
			}
			data, err := io.ReadAll(io.LimitReader(modReader{ctx: c.ctx, r: file}, maxWorldgenJSON+1))
			file.Close()
			c.accept(name, provenance, data, err)
		}
		return nil
	}
	for _, layer := range append([]string{""}, overlays...) {
		prefix, provenance = "", source.Name
		if layer != "" {
			prefix, provenance = layer+"/", source.Name+"#"+layer
		}
		for _, root := range []string{prefix + "data", prefix + "assets"} {
			if _, err := r.Stat(root); errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err := walk(root, 0); err != nil {
				return err
			}
		}
	}
	return nil
}

// Keep only map labels, merging language keys in pack order. Item/block prose
// and the rest of a mod's translations do not enter the worldgen snapshot.
func seedMapLanguage(previous, current []byte) (json.RawMessage, error) {
	labels := map[string]string{}
	if len(previous) > 0 && string(previous) != "null" {
		if err := json.Unmarshal(previous, &labels); err != nil {
			return nil, err
		}
	}
	var incoming map[string]json.RawMessage
	if err := json.Unmarshal(current, &incoming); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(incoming))
	for key := range incoming {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		raw := incoming[key]
		if !strings.HasPrefix(key, "biome.") && !strings.HasPrefix(key, "dimension.") && !strings.HasPrefix(key, "structure.") && !strings.HasPrefix(key, "structure_set.") {
			continue
		}
		var value string
		_, existing := labels[key]
		if json.Unmarshal(raw, &value) == nil && len(key) <= 256 && len(value) <= 512 && (len(labels) < 8192 || existing) {
			labels[key] = value
		}
	}
	return json.Marshal(labels)
}

func readWorldgenFile(ctx context.Context, file string, limit int64) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > limit {
		return nil, errors.New("worldgen file too large")
	}
	b, err := io.ReadAll(io.LimitReader(modReader{ctx: ctx, r: f}, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, errors.New("worldgen file too large")
	}
	return b, nil
}
