package launcher

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
)

// LevelInfo is the part of level.dat the launcher shows.
type LevelInfo struct {
	Name          string
	GameType      int32
	Hardcore      bool
	LastPlayed    int64
	RandomSeed    int64
	HasRandomSeed bool
	// Pre-1.16 saves encode the world type outside WorldGenSettings.
	LegacyGenerator, LegacyGeneratorOptions string
	LegacyGeneratorVersion                  int32
	HasLegacyGeneratorVersion               bool
	// EnabledDataPacks preserves Minecraft's low-to-high pack priority.
	EnabledDataPacks []string
	HasDataPacks     bool
	// Generators records saved dimension generator IDs without loading chunks.
	Generators map[string]string
	// Dimensions preserves the saved generator, noise settings and biome source.
	Dimensions              map[string]json.RawMessage
	WorldSpawn, PlayerSpawn *SeedMapSpawn
}

const (
	maxLevelData = 64 << 20
	maxNBTDepth  = 512
)

// ParseLevelDat reads bounded save metadata, including legacy RandomSeed and
// modern WorldGenSettings.seed. The modern seed takes precedence, including 0.
// Unneeded payloads are skipped without allocation; region files are not read.
func ParseLevelDat(r io.Reader) (LevelInfo, error) {
	return parseLevelDat(context.Background(), r)
}

func parseLevelDat(ctx context.Context, r io.Reader) (LevelInfo, error) {
	if err := ctx.Err(); err != nil {
		return LevelInfo{}, err
	}
	gz, e := gzip.NewReader(r)
	if e != nil {
		return LevelInfo{}, e
	}
	defer gz.Close()
	p := &nbtReader{r: &limitedReader{r: bufio.NewReader(modReader{ctx: ctx, r: gz}), n: maxLevelData}}
	var li LevelInfo
	if p.u8() != 10 { // root compound
		return li, errors.New("not an NBT compound")
	}
	p.str()
	modernSeed := false
	spawns := [2]SeedMapSpawn{{Dimension: "minecraft:overworld", Kind: "world"}, {Dimension: "minecraft:overworld", Kind: "player"}}
	spawnAxes := [2]uint8{}
	p.compound(func(path, name string, tag byte) bool {
		if path == "/Data" || path == "/Data/Player" {
			index := 0
			if path == "/Data/Player" {
				index = 1
			}
			if name == "SpawnDimension" && tag == 8 {
				spawns[index].Dimension = seedMapID(p.str())
				return true
			}
			if tag == 3 {
				switch name {
				case "SpawnX":
					spawns[index].X = int(int32(p.u32()))
					spawnAxes[index] |= 1
					return true
				case "SpawnY":
					spawns[index].Y = int(int32(p.u32()))
					spawnAxes[index] |= 2
					return true
				case "SpawnZ":
					spawns[index].Z = int(int32(p.u32()))
					spawnAxes[index] |= 4
					return true
				}
			}
		}
		if path == "/Data/WorldGenSettings" && name == "seed" && tag == 4 {
			li.RandomSeed, li.HasRandomSeed, modernSeed = int64(p.u64()), true, true
			return true
		}
		if path == "/Data/DataPacks" && name == "Enabled" && tag == 9 {
			t, n := p.u8(), int32(p.u32())
			// Large modpacks enable hundreds of mod data packs (FarmingTales: 340).
			if (t != 8 && !(t == 0 && n == 0)) || n < 0 || n > 4096 {
				p.err = errors.New("invalid enabled datapack list")
				return true
			}
			li.HasDataPacks = true
			for i := int32(0); i < n && p.err == nil; i++ {
				li.EnabledDataPacks = append(li.EnabledDataPacks, p.str())
			}
			return true
		}
		if path == "/Data/WorldGenSettings" && name == "dimensions" && tag == 10 {
			budget := 262144
			value := p.worldgenValue(tag, 0, p.consumed(), &budget)
			dimensions, ok := value.(map[string]any)
			if !ok || len(dimensions) > 256 {
				p.err = errors.New("invalid saved dimensions")
				return true
			}
			li.Dimensions, li.Generators = map[string]json.RawMessage{}, map[string]string{}
			for id, dimension := range dimensions {
				b, err := json.Marshal(dimension)
				if err != nil {
					p.err = fmt.Errorf("saved dimension: %w", err)
					return true
				}
				li.Dimensions[id] = b
				if d, ok := dimension.(map[string]any); ok {
					if g, ok := d["generator"].(map[string]any); ok {
						if idType, ok := g["type"].(string); ok {
							li.Generators[id] = idType
						}
					}
				}
			}
			return true
		}
		if path != "/Data" {
			return false
		}
		switch {
		case name == "generatorName" && tag == 8:
			li.LegacyGenerator = p.str()
		case name == "generatorOptions" && tag == 8:
			li.LegacyGeneratorOptions = p.str()
		case name == "generatorOptions" && tag == 10:
			li.LegacyGeneratorOptions = "custom"
			p.skip(tag, nil, "")
		case name == "generatorVersion" && tag == 3:
			li.LegacyGeneratorVersion = int32(p.u32())
			li.HasLegacyGeneratorVersion = true
		case name == "LevelName" && tag == 8:
			li.Name = p.str()
		case name == "GameType" && tag == 3:
			li.GameType = int32(p.u32())
		case name == "hardcore" && tag == 1:
			li.Hardcore = p.u8() != 0
		case name == "LastPlayed" && tag == 4:
			li.LastPlayed = int64(p.u64())
		case name == "RandomSeed" && tag == 4:
			seed := int64(p.u64())
			if !modernSeed {
				li.RandomSeed, li.HasRandomSeed = seed, true
			}
		default:
			return false
		}
		return true
	}, "")
	if p.err != nil {
		return li, p.err
	}
	if spawnAxes[0] == 7 {
		li.WorldSpawn = &spawns[0]
	}
	if spawnAxes[1] == 7 {
		li.PlayerSpawn = &spawns[1]
	}
	return li, nil
}

// worldgenValue materializes only the bounded WorldGenSettings dimension map.
// Other NBT (players, chunks, inventories) keeps using the allocation-free skip.
func (p *nbtReader) worldgenValue(tag byte, depth, start int, budget *int) any {
	*budget--
	if p.err != nil {
		return nil
	}
	if depth > 64 || *budget < 0 || p.consumed()-start > 4<<20 {
		p.err = errors.New("saved worldgen settings exceed limits")
		return nil
	}
	switch tag {
	case 1:
		return int8(p.u8())
	case 2:
		return int16(p.u16())
	case 3:
		return int32(p.u32())
	case 4:
		return int64(p.u64())
	case 5:
		return math.Float32frombits(p.u32())
	case 6:
		return math.Float64frombits(p.u64())
	case 8:
		return p.str()
	case 9:
		t, n := p.u8(), int32(p.u32())
		if n < 0 || n > int32(*budget) || (n > 0 && (t == 0 || t > 12)) {
			p.err = errors.New("invalid saved worldgen list")
			return nil
		}
		out := make([]any, 0, n)
		for i := int32(0); i < n && p.err == nil; i++ {
			out = append(out, p.worldgenValue(t, depth+1, start, budget))
		}
		return out
	case 10:
		out := map[string]any{}
		for p.err == nil {
			t := p.u8()
			if t == 0 {
				break
			}
			name := p.str()
			v := p.worldgenValue(t, depth+1, start, budget)
			if t == 1 && worldgenNBTBools[name] {
				v = v != int8(0)
			}
			out[name] = v
		}
		return out
	default:
		// Arrays are not part of the supported dimension/density JSON codecs.
		p.skip(tag, nil, "")
		return nil
	}
}

func (p *nbtReader) consumed() int { return p.r.read }

// limitedReader counts bytes and fails past the cap, so a malformed length
// can never pull more than maxLevelData out of the decompressor.
type limitedReader struct {
	r    io.ByteReader
	n    int
	read int
}

func (l *limitedReader) ReadByte() (byte, error) {
	if l.n <= 0 {
		return 0, errors.New("level.dat exceeds 64 MiB")
	}
	b, err := l.r.ReadByte()
	if err == nil {
		l.n--
		l.read++
	}
	return b, err
}

type nbtReader struct {
	r     *limitedReader
	depth int
	done  bool
	err   error
}

func (p *nbtReader) take(n int) []byte {
	if p.err != nil || n < 0 {
		return nil
	}
	b := make([]byte, n)
	for i := range b {
		v, err := p.r.ReadByte()
		if err != nil {
			p.err = errors.New("truncated NBT")
			return nil
		}
		b[i] = v
	}
	return b
}

func (p *nbtReader) discard(n int) {
	if p.err != nil || n < 0 {
		return
	}
	for range n {
		if _, err := p.r.ReadByte(); err != nil {
			p.err = errors.New("truncated NBT")
			return
		}
	}
}

func (p *nbtReader) u8() byte {
	if p.err != nil {
		return 0
	}
	b, err := p.r.ReadByte()
	if err != nil {
		p.err = errors.New("truncated NBT")
		return 0
	}
	return b
}

func (p *nbtReader) u16() uint16 {
	if b := p.take(2); len(b) == 2 {
		return binary.BigEndian.Uint16(b)
	}
	return 0
}
func (p *nbtReader) u32() uint32 {
	if b := p.take(4); len(b) == 4 {
		return binary.BigEndian.Uint32(b)
	}
	return 0
}
func (p *nbtReader) u64() uint64 {
	if b := p.take(8); len(b) == 8 {
		return binary.BigEndian.Uint64(b)
	}
	return 0
}
func (p *nbtReader) str() string { return string(p.take(int(p.u16()))) }

// enter bounds recursive lists and compounds independently of the input size.
func (p *nbtReader) enter() bool {
	if p.err != nil {
		return false
	}
	if p.depth >= maxNBTDepth {
		p.err = errors.New("NBT nesting limit exceeded")
		return false
	}
	p.depth++
	return true
}

// array validates the count before consuming it, including on 32-bit hosts.
func (p *nbtReader) array(width int) {
	n := int32(p.u32())
	if p.err != nil {
		return
	}
	if n < 0 || int64(n) > int64(p.r.n/width) {
		p.err = errors.New("invalid NBT array length")
		return
	}
	p.discard(int(n) * width)
}

// compound walks a compound's entries; want reads a value itself and
// returns true, else the value is skipped. done stops at the first point
// the caller has everything it needs.
func (p *nbtReader) compound(want func(path, name string, tag byte) bool, path string) {
	if !p.enter() {
		return
	}
	defer func() { p.depth-- }()
	for p.err == nil && !p.done {
		tag := p.u8()
		if p.err != nil || p.done || tag == 0 {
			return
		}
		name := p.str()
		if want != nil && want(path, name, tag) {
			continue
		}
		p.skip(tag, want, path+"/"+name)
	}
}

func (p *nbtReader) skip(tag byte, want func(string, string, byte) bool, path string) {
	if p.err != nil || p.done {
		return
	}
	switch tag {
	case 1:
		p.discard(1)
	case 2:
		p.discard(2)
	case 3, 5:
		p.discard(4)
	case 4, 6:
		p.discard(8)
	case 7:
		p.array(1)
	case 8:
		p.discard(int(p.u16()))
	case 9:
		if !p.enter() {
			return
		}
		defer func() { p.depth-- }()
		t, n := p.u8(), int(int32(p.u32()))
		if p.err != nil {
			return
		}
		// Every non-empty list element requires at least one byte.
		if n < 0 || n > p.r.n || (n > 0 && (t == 0 || t > 12)) {
			p.err = errors.New("invalid NBT list length or type")
			return
		}
		for i := 0; i < n && p.err == nil; i++ {
			p.skip(t, nil, path)
		}
	case 10:
		p.compound(want, path)
	case 11:
		p.array(4)
	case 12:
		p.array(8)
	default:
		p.err = fmt.Errorf("bad NBT tag %d", tag)
	}
}

// worldgenNBTBools are codec booleans that level.dat stores as NBT bytes.
var worldgenNBTBools = map[string]bool{"legacy_random_source": true, "large_biomes": true, "legacy_biome_init_layer": true, "aquifers_enabled": true, "ore_veins_enabled": true, "disable_mob_generation": true, "add_stone_depth": true, "has_precipitation": true}
