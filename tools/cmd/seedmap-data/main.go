// Command seedmap-data rebuilds embedded climate data from pinned local sources.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Viking602/YoloMCL/tools/internal/toolutil"
)

type builder struct {
	source string
	hashes map[string]string
	files  map[string][]byte
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "seedmap-data:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("seedmap-data", flag.ContinueOnError)
	output := flags.String("output", "", "output directory (default: repository cubiomes/data)")
	check := flags.Bool("check", false, "compare decompressed data and metadata without changing files")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "Usage: seedmap-data [--output directory] [--check] reference-directory")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("expected one reference directory; use --help")
	}
	if *output == "" {
		root, err := toolutil.Root()
		if err != nil {
			return err
		}
		*output = filepath.Join(root, "cubiomes", "data")
	}
	files, err := generate(flags.Arg(0))
	if err != nil {
		return err
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		path := filepath.Join(*output, name)
		if *check {
			existing, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if err := compareData(name, existing, files[name]); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		} else if err := toolutil.WriteFile(path, files[name]); err != nil {
			return err
		}
	}
	if *check {
		fmt.Printf("Verified %d generated files against %s (decompressed content).\n", len(names), *output)
	} else {
		fmt.Printf("Generated %d files in %s.\n", len(names), *output)
	}
	return nil
}

func (b *builder) read(name string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(b.source, filepath.FromSlash(name)))
	if err != nil {
		return nil, err
	}
	b.hashes[name] = fmt.Sprintf("%x", sha256.Sum256(data))
	return data, nil
}

func (b *builder) object(name string) (map[string]any, error) {
	data, err := b.read(name + ".json")
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		return nil, fmt.Errorf("%s.json: %w", name, err)
	}
	if obj == nil {
		return nil, fmt.Errorf("%s.json must contain an object", name)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s.json has trailing data", name)
	}
	return obj, nil
}

func object(value any, where string) (map[string]any, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an object", where)
	}
	return m, nil
}

func (b *builder) gzip(name string, data []byte) error {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return err
	}
	if _, err = w.Write(data); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	b.files[name] = buf.Bytes()
	return nil
}

func (b *builder) jsonGzip(name string, value any) error {
	data, err := toolutil.JSON(value, false)
	if err != nil {
		return err
	}
	return b.gzip(name, data)
}

var pairsPattern = regexp.MustCompile(`\{\s*(-?\d+),\s*(-?\d+)\s*\}`)
var nodesPattern = regexp.MustCompile(`0x([A-Fa-f0-9]{16})`)

func climateTable(text string) ([]byte, error) {
	_, param, ok := strings.Cut(text, "param[][2] =")
	if !ok {
		return nil, fmt.Errorf("missing climate parameters")
	}
	param, _, ok = strings.Cut(param, "};")
	if !ok {
		return nil, fmt.Errorf("unterminated climate parameters")
	}
	_, nodes, ok := strings.Cut(text, "nodes[] =")
	if !ok {
		return nil, fmt.Errorf("missing climate nodes")
	}
	nodes, _, ok = strings.Cut(nodes, "};")
	if !ok {
		return nil, fmt.Errorf("unterminated climate nodes")
	}
	pairs := pairsPattern.FindAllStringSubmatch(param, -1)
	records := nodesPattern.FindAllStringSubmatch(nodes, -1)
	if len(pairs) == 0 || len(pairs) > 256 || len(records) == 0 || uint64(len(records)) > uint64(^uint32(0)) {
		return nil, fmt.Errorf("invalid climate table counts: %d / %d", len(pairs), len(records))
	}
	data := binary.LittleEndian.AppendUint32(nil, uint32(len(pairs)))
	data = binary.LittleEndian.AppendUint32(data, uint32(len(records)))
	for _, pair := range pairs {
		for _, value := range pair[1:] {
			n, err := strconv.ParseInt(value, 10, 32)
			if err != nil {
				return nil, err
			}
			data = binary.LittleEndian.AppendUint32(data, uint32(int32(n)))
		}
	}
	for _, record := range records {
		n, err := strconv.ParseUint(record[1], 16, 64)
		if err != nil {
			return nil, err
		}
		data = binary.LittleEndian.AppendUint64(data, n)
	}
	return data, nil
}

func generate(source string) (map[string][]byte, error) {
	b := &builder{source: source, hashes: map[string]string{}, files: map[string][]byte{}}
	for _, profile := range [][2]string{{"18", "18"}, {"192", "192"}, {"19", "19"}, {"20", "20"}, {"21wd", "21"}, {"215", "215"}} {
		name := "tables/btree" + profile[0] + ".h"
		text, err := b.read(name)
		if err != nil {
			return nil, err
		}
		data, err := climateTable(string(text))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if err := b.gzip("overworld"+profile[1]+".bin.gz", data); err != nil {
			return nil, err
		}
	}
	for _, profile := range [][2]string{{"", ""}, {"18", "1.18.2-"}} {
		registries := map[string]any{}
		for _, kind := range []string{"noise", "density_function", "noise_settings"} {
			v, err := b.object(profile[1] + kind)
			if err != nil {
				return nil, err
			}
			registries[kind] = v
		}
		presets, err := b.object("world_preset")
		if err != nil {
			return nil, err
		}
		if presets["normal"] == nil {
			return nil, fmt.Errorf("missing normal world preset")
		}
		registries["world_preset"] = map[string]any{"normal": presets["normal"]}
		registries["multi_noise_biome_source_parameter_list"], err = b.object("multi_noise_biome_source_parameter_list")
		if err != nil {
			return nil, err
		}
		settings := registries["noise_settings"].(map[string]any)
		for name, value := range settings {
			definition, err := object(value, "noise_settings/"+name)
			if err != nil {
				return nil, err
			}
			for key := range definition {
				if !slices.Contains([]string{"legacy_random_source", "noise_router", "noise", "sea_level", "spawn_target"}, key) {
					delete(definition, key)
				}
			}
			router, err := object(definition["noise_router"], "noise_router/"+name)
			if err != nil {
				return nil, err
			}
			for key := range router {
				if !slices.Contains([]string{"temperature", "vegetation", "continents", "erosion", "depth", "ridges"}, key) {
					delete(router, key)
				}
			}
		}
		if profile[0] == "" {
			sets, err := b.object("structure_set")
			if err != nil {
				return nil, err
			}
			selected, ids := map[string]any{}, map[string]bool{}
			for _, name := range []string{"villages", "desert_pyramids", "jungle_temples", "swamp_huts", "ocean_monuments", "woodland_mansions"} {
				set, err := object(sets[name], "structure_set/"+name)
				if err != nil {
					return nil, err
				}
				selected[name] = set
				entries, ok := set["structures"].([]any)
				if !ok {
					return nil, fmt.Errorf("%s: missing structures array", name)
				}
				for _, entry := range entries {
					m, err := object(entry, name)
					if err != nil {
						return nil, err
					}
					id, ok := m["structure"].(string)
					if !ok {
						return nil, fmt.Errorf("%s: missing structure ID", name)
					}
					ids[strings.TrimPrefix(id, "minecraft:")] = true
				}
			}
			structures, err := b.object("structure")
			if err != nil {
				return nil, err
			}
			filtered := map[string]any{}
			for name, value := range structures {
				if ids[name] {
					m, err := object(value, "structure/"+name)
					if err != nil {
						return nil, err
					}
					if m["biomes"] == nil {
						return nil, fmt.Errorf("%s: missing biome filter", name)
					}
					filtered[name] = map[string]any{"biomes": m["biomes"]}
				}
			}
			registries["structure_set"], registries["structure"] = selected, filtered
		}
		if err := b.jsonGzip("registries"+profile[0]+".json.gz", registries); err != nil {
			return nil, err
		}
	}
	profiles := map[string]any{}
	for _, profile := range [][2]string{{"18", "1.18.2-"}, {"19", "1.19.2-"}, {"20", ""}, {"21", "1.21.1-"}, {"214", "1.21.4-"}, {"215", "1.21.5-"}} {
		tags, err := b.object(profile[1] + "biome_tag")
		if err != nil {
			return nil, err
		}
		biomes, err := b.object(profile[1] + "biome")
		if err != nil {
			return nil, err
		}
		for name, value := range biomes {
			m, err := object(value, "biome/"+name)
			if err != nil {
				return nil, err
			}
			if m["effects"] == nil {
				return nil, fmt.Errorf("%s: missing biome effects", name)
			}
			biomes[name] = map[string]any{"effects": m["effects"]}
		}
		profiles[profile[0]] = map[string]any{"biome_tag": tags, "biome": biomes}
	}
	if err := b.jsonGzip("biomes.json.gz", profiles); err != nil {
		return nil, err
	}
	license, err := b.read("LICENSE")
	if err != nil {
		return nil, err
	}
	b.files["LICENSE-cubiomes.txt"] = license
	b.files["sources.json"], err = toolutil.JSON(b.hashes, true)
	return b.files, err
}

func compareData(name string, left, right []byte) error {
	if strings.HasSuffix(name, ".gz") {
		unzip := func(data []byte) ([]byte, error) {
			r, err := gzip.NewReader(bytes.NewReader(data))
			if err != nil {
				return nil, err
			}
			defer r.Close()
			return io.ReadAll(r)
		}
		var err error
		left, err = unzip(left)
		if err != nil {
			return err
		}
		right, err = unzip(right)
		if err != nil {
			return err
		}
		name = strings.TrimSuffix(name, ".gz")
	}
	if bytes.Equal(left, right) {
		return nil
	}
	if strings.HasSuffix(name, ".json") {
		// Compare numeric values exactly, without rounding integers through float64.
		decode := func(data []byte) (any, error) {
			dec := json.NewDecoder(bytes.NewReader(data))
			dec.UseNumber()
			var v any
			if err := dec.Decode(&v); err != nil {
				return nil, err
			}
			var extra any
			if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("JSON has trailing data")
			}
			return normalize(v), nil
		}
		a, err := decode(left)
		if err != nil {
			return err
		}
		b, err := decode(right)
		if err != nil {
			return err
		}
		if reflect.DeepEqual(a, b) {
			return nil
		}
	}
	return fmt.Errorf("generated content differs")
}

func normalize(value any) any {
	switch v := value.(type) {
	case json.Number:
		n, ok := new(big.Rat).SetString(string(v))
		if ok {
			return json.Number(n.RatString())
		}
	case map[string]any:
		for key, x := range v {
			v[key] = normalize(x)
		}
	case []any:
		for i, x := range v {
			v[i] = normalize(x)
		}
	}
	return value
}
