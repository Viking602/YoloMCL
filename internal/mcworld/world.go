// Package mcworld reads saved Minecraft worlds (level.dat and Anvil region
// files) so seed-map simulations can be checked against real generated
// chunks. It is read-only: it never writes to a save.
package mcworld

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"
	"os"
	"path/filepath"
	"strings"

	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// LevelSeed reads the world seed from level.dat (1.16+ WorldGenSettings).
func LevelSeed(worldDir string) (int64, error) {
	data, err := os.ReadFile(filepath.Join(worldDir, "level.dat"))
	if err != nil {
		return 0, err
	}
	root, err := structure3d.DecodeNBT(data)
	if err != nil {
		return 0, err
	}
	d, _ := root["Data"].(map[string]any)
	if wg, ok := d["WorldGenSettings"].(map[string]any); ok {
		if s, ok := wg["seed"].(int64); ok {
			return s, nil
		}
	}
	if s, ok := d["RandomSeed"].(int64); ok {
		return s, nil
	}
	return 0, fmt.Errorf("mcworld: no seed in level.dat")
}

// Region is one r.X.Z.mca file held in memory.
type Region struct {
	X, Z int
	data []byte
}

// OpenRegion reads a region file.
func OpenRegion(path string) (*Region, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var x, z int
	if _, err := fmt.Sscanf(filepath.Base(path), "r.%d.%d.mca", &x, &z); err != nil {
		return nil, fmt.Errorf("mcworld: bad region name %s", path)
	}
	return &Region{X: x, Z: z, data: data}, nil
}

// RegionPaths lists the region files of a dimension folder ("" = overworld).
func RegionPaths(worldDir, dim string) ([]string, error) {
	return filepath.Glob(filepath.Join(worldDir, dim, "region", "*.mca"))
}

// Chunk decodes the chunk at local index (0..31, 0..31); nil when absent.
func (r *Region) Chunk(lx, lz int) (*Chunk, error) {
	i := 4 * ((lx & 31) + (lz&31)*32)
	if len(r.data) < 8192 {
		return nil, nil
	}
	loc := binary.BigEndian.Uint32(r.data[i : i+4])
	off, n := int(loc>>8)*4096, int(loc&0xff)*4096
	if off == 0 || n == 0 || off+5 > len(r.data) {
		return nil, nil
	}
	length := int(binary.BigEndian.Uint32(r.data[off : off+4]))
	if length < 1 || off+4+length > len(r.data) {
		return nil, fmt.Errorf("mcworld: chunk %d,%d out of range", lx, lz)
	}
	comp := r.data[off+4]
	payload := r.data[off+5 : off+4+length]
	var rd io.Reader
	switch comp & 0x7f {
	case 1:
		g, err := gzip.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		rd = g
	case 2:
		z, err := zlib.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		rd = z
	case 3:
		rd = bytes.NewReader(payload)
	default:
		return nil, fmt.Errorf("mcworld: compression %d unsupported", comp)
	}
	raw, err := io.ReadAll(rd)
	if err != nil {
		return nil, err
	}
	root, err := structure3d.DecodeNBT(raw)
	if err != nil {
		return nil, err
	}
	return newChunk(root), nil
}

// Chunk is one decoded chunk (1.18+ format).
type Chunk struct {
	X, Z     int
	Status   string
	Root     map[string]any
	sections map[int]*section
}

type section struct {
	palette []string
	data    []int64
	bits    int
	biomes  []string // 4x4x4 biome palette
	bdata   []int64
}

func newChunk(root map[string]any) *Chunk {
	c := &Chunk{Root: root, sections: map[int]*section{}}
	c.X, c.Z = int(i32(root["xPos"])), int(i32(root["zPos"]))
	c.Status, _ = root["Status"].(string)
	secs, _ := root["sections"].([]any)
	for _, s := range secs {
		m, _ := s.(map[string]any)
		bs, _ := m["block_states"].(map[string]any)
		if bs == nil {
			continue
		}
		pal, _ := bs["palette"].([]any)
		sec := &section{}
		for _, p := range pal {
			pm, _ := p.(map[string]any)
			name, _ := pm["Name"].(string)
			sec.palette = append(sec.palette, name)
		}
		sec.data, _ = bs["data"].([]int64)
		if len(sec.palette) > 1 {
			sec.bits = max(4, bits.Len(uint(len(sec.palette)-1)))
		}
		if bm, _ := m["biomes"].(map[string]any); bm != nil {
			pal, _ := bm["palette"].([]any)
			for _, p := range pal {
				name, _ := p.(string)
				sec.biomes = append(sec.biomes, name)
			}
			sec.bdata, _ = bm["data"].([]int64)
		}
		c.sections[int(i32(m["Y"]))] = sec
	}
	return c
}

func i32(v any) int32 {
	switch n := v.(type) {
	case int32:
		return n
	case int64:
		return int32(n)
	}
	return 0
}

// Block returns the block name at chunk-local x,z (0..15) and world y.
func (c *Chunk) Block(x, y, z int) string {
	sec := c.sections[y>>4]
	if sec == nil || len(sec.palette) == 0 {
		return "minecraft:air"
	}
	if sec.bits == 0 {
		return sec.palette[0]
	}
	idx := ((y & 15) << 8) | ((z & 15) << 4) | (x & 15)
	per := 64 / sec.bits
	word := idx / per
	if word >= len(sec.data) {
		return "minecraft:air"
	}
	v := int((uint64(sec.data[word]) >> (uint(idx%per) * uint(sec.bits))) & (1<<uint(sec.bits) - 1))
	if v >= len(sec.palette) {
		return "minecraft:air"
	}
	return sec.palette[v]
}

// SectionYs returns the section indices present.
func (c *Chunk) SectionYs() []int {
	out := make([]int, 0, len(c.sections))
	for y := range c.sections {
		out = append(out, y)
	}
	return out
}

// CountBlocks counts every block by name.
func (c *Chunk) CountBlocks(match func(string) bool) map[string]int {
	out := map[string]int{}
	for sy, sec := range c.sections {
		if len(sec.palette) == 0 {
			continue
		}
		if sec.bits == 0 {
			if match(sec.palette[0]) {
				out[sec.palette[0]] += 4096
			}
			continue
		}
		for y := 0; y < 16; y++ {
			for z := 0; z < 16; z++ {
				for x := 0; x < 16; x++ {
					if b := c.Block(x, sy*16+y, z); match(b) {
						out[b]++
					}
				}
			}
		}
	}
	return out
}

// Piece is one saved structure piece.
type Piece struct {
	ID       string
	Template string
	Rotation string
	BB       [6]int32
	Raw      map[string]any
}

// Start is a structure start saved in its origin chunk.
type Start struct {
	ID             string
	ChunkX, ChunkZ int
	Pieces         []Piece
}

// Starts returns the structure starts saved in this chunk.
func (c *Chunk) Starts() []Start {
	st, _ := c.Root["structures"].(map[string]any)
	starts, _ := st["starts"].(map[string]any)
	var out []Start
	for id, v := range starts {
		m, _ := v.(map[string]any)
		if s, _ := m["id"].(string); s == "INVALID" || s == "" {
			continue
		}
		s := Start{ID: id, ChunkX: int(i32(m["ChunkX"])), ChunkZ: int(i32(m["ChunkZ"]))}
		children, _ := m["Children"].([]any)
		for _, ch := range children {
			cm, _ := ch.(map[string]any)
			p := Piece{Raw: cm}
			p.ID, _ = cm["id"].(string)
			p.Template, _ = cm["Template"].(string)
			p.Rotation, _ = cm["Rot"].(string)
			if bb, ok := cm["BB"].([]int32); ok && len(bb) == 6 {
				copy(p.BB[:], bb)
			}
			if p.Template == "" {
				if pe, ok := cm["pool_element"].(map[string]any); ok {
					if loc, ok := pe["location"].(string); ok {
						p.Template = loc
					}
				}
			}
			if p.Rotation == "" {
				p.Rotation, _ = cm["rotation"].(string)
			}
			s.Pieces = append(s.Pieces, p)
		}
		out = append(out, s)
	}
	return out
}

// IsOre reports a block id that is an ore (any namespace).
func IsOre(name string) bool {
	return strings.HasSuffix(name, "_ore") || strings.Contains(name, "_ore_") || name == "minecraft:ancient_debris"
}

// Biome returns the stored biome at chunk-local x,z and world y (4x4x4 cells).
func (c *Chunk) Biome(x, y, z int) string {
	sec := c.sections[y>>4]
	if sec == nil || len(sec.biomes) == 0 {
		return ""
	}
	if len(sec.biomes) == 1 {
		return sec.biomes[0]
	}
	b := bits.Len(uint(len(sec.biomes) - 1))
	idx := ((y&15)>>2)<<4 | ((z&15)>>2)<<2 | (x&15)>>2
	per := 64 / b
	if idx/per >= len(sec.bdata) {
		return ""
	}
	v := int((uint64(sec.bdata[idx/per]) >> (uint(idx%per) * uint(b))) & (1<<uint(b) - 1))
	if v >= len(sec.biomes) {
		return ""
	}
	return sec.biomes[v]
}
