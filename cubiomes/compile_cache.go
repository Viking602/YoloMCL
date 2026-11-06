package cubiomes

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"sort"
	"strings"
	"sync"
)

// Bounded by entries rather than seed count. Keys include every registry used
// by this compiler. Colors, translations and structures do not affect biomes.
var compiledPrograms struct {
	sync.Mutex
	clock   uint64
	entries [16]compiledProgram
}

type compiledProgram struct {
	key   [32]byte
	stamp uint64
	g     *Generator
}

func programKey(ctx context.Context, docs map[string]Document, def Definition) ([32]byte, error) {
	h := sha256.New()
	var size [8]byte
	write := func(v []byte) {
		binary.LittleEndian.PutUint64(size[:], uint64(len(v)))
		_, _ = h.Write(size[:])
		_, _ = h.Write(v)
	}
	write([]byte(def.Version))
	write(def.Settings)
	write(def.BiomeSource)
	keys := make([]string, 0, len(docs))
	for key := range docs {
		kind, _, _ := strings.Cut(key, "/")
		switch kind {
		case "density_function", "noise", "noise_settings", "multi_noise_biome_source_parameter_list", "lithostitched_fast_noise_config":
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for i, key := range keys {
		if i&63 == 0 {
			if err := ctx.Err(); err != nil {
				return [32]byte{}, err
			}
		}
		write([]byte(key))
		write(docs[key].Data)
	}
	var key [32]byte
	h.Sum(key[:0])
	return key, ctx.Err()
}

func cachedProgram(key [32]byte) *Generator {
	compiledPrograms.Lock()
	defer compiledPrograms.Unlock()
	for i := range compiledPrograms.entries {
		e := &compiledPrograms.entries[i]
		if e.g != nil && e.key == key {
			compiledPrograms.clock++
			e.stamp = compiledPrograms.clock
			return e.g
		}
	}
	return nil
}

func rememberProgram(key [32]byte, g *Generator) {
	compiledPrograms.Lock()
	defer compiledPrograms.Unlock()
	oldest := 0
	for i, e := range compiledPrograms.entries {
		if e.g != nil && e.key == key {
			return
		}
		if e.stamp < compiledPrograms.entries[oldest].stamp {
			oldest = i
		}
	}
	compiledPrograms.clock++
	compiledPrograms.entries[oldest] = compiledProgram{key, compiledPrograms.clock, g}
}
