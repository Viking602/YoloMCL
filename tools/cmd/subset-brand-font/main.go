// Command subset-brand-font rebuilds the fixed Fusion Pixel wordmark in Go.
// It is intentionally not a general-purpose font subsetter: only CFF outlines
// for the unshaped ASCII wordmark are retained, without layout or hint programs.
package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"unicode/utf16"

	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/font/opentype"
	"golang.org/x/image/font/sfnt"

	"github.com/Viking602/YoloMCL/tools/internal/toolutil"
)

const characters = " CLMYlo"
const familyName = "MCL Pixel Brand"
const fullName = "MCL Pixel Brand Regular"
const postScriptName = "MCLPixelBrand-Regular"

var be = binary.BigEndian

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "subset-brand-font:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var output string
	f := flag.NewFlagSet("subset-brand-font", flag.ContinueOnError)
	f.StringVar(&output, "output", "", "destination OTF (default: bundled wordmark)")
	if err := f.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if f.NArg() != 1 {
		return fmt.Errorf("usage: subset-brand-font [--output file.otf] source.otf")
	}
	if output == "" {
		root, err := toolutil.Root()
		if err != nil {
			return err
		}
		output = filepath.Join(root, "internal", "nativeui", "assets", "pixel.otf")
	}
	source, err := os.ReadFile(f.Arg(0))
	if err != nil {
		return err
	}
	result, err := subset(source)
	if err != nil {
		return err
	}
	if err := toolutil.WriteFile(output, result); err != nil {
		return err
	}
	fmt.Printf("Wordmark font: %s (%d bytes)\n", output, len(result))
	return nil
}

func subset(source []byte) ([]byte, error) {
	tables, err := readTables(source)
	if err != nil {
		return nil, err
	}
	for name, minimum := range map[string]int{"CFF ": 4, "head": 54, "hhea": 36, "hmtx": 4, "maxp": 6, "name": 6, "OS/2": 78, "post": 32} {
		if len(tables[name]) < minimum {
			return nil, fmt.Errorf("missing or truncated %q table", name)
		}
	}
	f, err := font.ParseTTF(bytes.NewReader(source))
	if err != nil {
		return nil, fmt.Errorf("source font: %w", err)
	}
	indices := []font.GID{0}
	for _, r := range characters {
		id, ok := f.NominalGlyph(r)
		if !ok || id == 0 {
			return nil, fmt.Errorf("source font has no %q glyph", r)
		}
		indices = append(indices, id)
	}
	var charstrings [][]byte
	var hmtx []byte
	metrics := int(be.Uint16(tables["hhea"][34:36]))
	if metrics == 0 {
		return nil, fmt.Errorf("font has no horizontal metrics")
	}
	for _, id := range indices {
		advance := f.HorizontalAdvance(id)
		outline, ok := f.GlyphDataOutline(id)
		if !ok {
			return nil, fmt.Errorf("source glyph %d has no readable CFF outline", id)
		}
		program, err := charstring(advance, outline.Segments)
		if err != nil {
			return nil, err
		}
		charstrings = append(charstrings, program)
		lsb := int(id)*4 + 2
		if int(id) >= metrics {
			lsb = metrics*4 + (int(id)-metrics)*2
		}
		if lsb+2 > len(tables["hmtx"]) {
			return nil, fmt.Errorf("truncated glyph metric")
		}
		hmtx = be.AppendUint16(hmtx, uint16(advance))
		hmtx = append(hmtx, tables["hmtx"][lsb:lsb+2]...)
	}
	name, err := rename(tables["name"])
	if err != nil {
		return nil, err
	}
	// Keep the original attribution, embedding permissions, vertical metrics,
	// font revision and timestamps. Do not retain tables with old glyph indexes.
	out := map[string][]byte{}
	for _, tag := range []string{"head", "hhea", "OS/2", "post"} {
		out[tag] = bytes.Clone(tables[tag])
	}
	out["name"], out["hmtx"] = name, hmtx
	out["maxp"] = []byte{0, 0, 0x50, 0, 0, byte(len(indices))}
	be.PutUint16(out["hhea"][34:36], uint16(len(indices)))
	be.PutUint32(out["head"][8:12], 0)
	be.PutUint16(out["head"][50:52], 0)
	out["post"] = out["post"][:32]
	be.PutUint32(out["post"][:4], 0x00030000)
	// The subset covers Basic Latin only; keep the original selection/style bits.
	clear(out["OS/2"][42:58])
	be.PutUint32(out["OS/2"][42:46], 1)
	be.PutUint16(out["OS/2"][64:66], uint16(characters[0]))
	be.PutUint16(out["OS/2"][66:68], uint16(characters[len(characters)-1]))
	if len(out["OS/2"]) >= 86 {
		clear(out["OS/2"][78:86])
		be.PutUint32(out["OS/2"][78:82], 1)
	}
	out["cmap"] = cmap()
	out["CFF "] = cff(charstrings, out["head"][36:44], f.Upem())
	result := writeTables(out)
	if _, err := sfnt.Parse(result); err != nil {
		return nil, fmt.Errorf("generated font: %w", err)
	}
	return result, nil
}

func charstring(advance float32, segments []font.Segment) ([]byte, error) {
	var out []byte
	number := func(n float32) error {
		scaled := float64(n) * 65536
		if math.IsNaN(scaled) || scaled < math.MinInt32 || scaled > math.MaxInt32 || math.Trunc(scaled) != scaled {
			return fmt.Errorf("CFF coordinate outside 16.16 range")
		}
		if n != float32(int32(n)) {
			out = append(out, 255)
			out = be.AppendUint32(out, uint32(int32(scaled)))
			return nil
		}
		v := int(n)
		switch {
		case v >= -107 && v <= 107:
			out = append(out, byte(v+139))
		case v >= 108 && v <= 1131:
			v -= 108
			out = append(out, byte(v/256+247), byte(v))
		case v <= -108 && v >= -1131:
			v = -v - 108
			out = append(out, byte(v/256+251), byte(v))
		default:
			out = append(out, 28)
			out = be.AppendUint16(out, uint16(int16(v)))
		}
		return nil
	}
	if err := number(advance); err != nil {
		return nil, err
	}
	var pen font.SegmentPoint
	for _, segment := range segments {
		count, op := 1, byte(0)
		switch segment.Op {
		case opentype.SegmentOpMoveTo:
			op = 21
		case opentype.SegmentOpLineTo:
			op = 5
		case opentype.SegmentOpCubeTo:
			count, op = 3, 8
		default:
			return nil, fmt.Errorf("wordmark requires CFF line/cubic outlines")
		}
		for _, point := range segment.Args[:count] {
			if err := number(point.X - pen.X); err != nil {
				return nil, err
			}
			if err := number(point.Y - pen.Y); err != nil {
				return nil, err
			}
			pen = point
		}
		out = append(out, op)
	}
	return append(out, 14), nil // endchar; defaultWidthX and nominalWidthX are zero
}

// CFF INDEX and DICT offsets use fixed-width encodings so building the Top DICT
// does not require iterative offset adjustment.
func cffIndex(items [][]byte) []byte {
	out := be.AppendUint16(nil, uint16(len(items)))
	if len(items) == 0 {
		return out
	}
	out = append(out, 4)
	offset := uint32(1)
	for _, item := range items {
		out = be.AppendUint32(out, offset)
		offset += uint32(len(item))
	}
	out = be.AppendUint32(out, offset)
	for _, item := range items {
		out = append(out, item...)
	}
	return out
}

func cff(programs [][]byte, bounds []byte, upem uint16) []byte {
	names := cffIndex([][]byte{[]byte(postScriptName)})
	strings := [][]byte{[]byte(fullName), []byte(familyName), []byte("Regular")}
	charset := []byte{0}
	for i, r := range characters {
		name := string(r)
		if r == ' ' {
			name = "space"
		}
		strings = append(strings, []byte(name))
		charset = be.AppendUint16(charset, uint16(394+i))
	}
	stringIndex := cffIndex(strings)
	top := func(charsetOffset, charstringsOffset int) []byte {
		var b []byte
		integer := func(n int) { b = append(b, 29); b = be.AppendUint32(b, uint32(int32(n))) }
		for i, op := range []byte{2, 3, 4} {
			integer(391 + i)
			b = append(b, op)
		}
		// Fusion Pixel uses 1200 units per em. CFF's default matrix assumes
		// 1000, so write an explicit matrix for consumers that honor it.
		scale := cffReal(1 / float64(upem))
		b = append(b, scale...)
		integer(0)
		integer(0)
		b = append(b, scale...)
		integer(0)
		integer(0)
		b = append(b, 12, 7) // FontMatrix
		for i := 0; i < 8; i += 2 {
			integer(int(int16(be.Uint16(bounds[i : i+2]))))
		}
		b = append(b, 5)
		integer(charsetOffset)
		b = append(b, 15)
		integer(charstringsOffset)
		b = append(b, 17)
		return cffIndex([][]byte{b})
	}
	charsetOffset := 4 + len(names) + len(top(0, 0)) + len(stringIndex) + 2
	out := []byte{1, 0, 4, 4}
	out = append(out, names...)
	out = append(out, top(charsetOffset, charsetOffset+len(charset))...)
	out = append(out, stringIndex...)
	out = append(out, 0, 0) // no global subroutines
	out = append(out, charset...)
	out = append(out, cffIndex(programs)...)
	return out
}

func cffReal(value float64) []byte {
	// DICT real numbers use decimal nibbles; fixed notation avoids exponent
	// handling for this small positive reciprocal of unitsPerEm.
	var nibbles []byte
	for _, r := range strconv.FormatFloat(value, 'f', -1, 64) {
		if r == '.' {
			nibbles = append(nibbles, 10)
		} else {
			nibbles = append(nibbles, byte(r-'0'))
		}
	}
	nibbles = append(nibbles, 15)
	if len(nibbles)%2 != 0 {
		nibbles = append(nibbles, 15)
	}
	result := []byte{30}
	for i := 0; i < len(nibbles); i += 2 {
		result = append(result, nibbles[i]<<4|nibbles[i+1])
	}
	return result
}

func cmap() []byte {
	// Format 4 with one segment per retained ASCII character, plus the sentinel.
	n := len(characters) + 1
	sub := []byte{0, 4}
	sub = be.AppendUint16(sub, uint16(16+8*n))
	sub = append(sub, 0, 0)
	sub = be.AppendUint16(sub, uint16(2*n))
	sub = be.AppendUint16(sub, 16)
	sub = be.AppendUint16(sub, 3)
	sub = be.AppendUint16(sub, 0)
	for _, r := range characters {
		sub = be.AppendUint16(sub, uint16(r))
	}
	sub = be.AppendUint16(sub, 0xffff)
	sub = append(sub, 0, 0)
	for _, r := range characters {
		sub = be.AppendUint16(sub, uint16(r))
	}
	sub = be.AppendUint16(sub, 0xffff)
	for i, r := range characters {
		sub = be.AppendUint16(sub, uint16(i+1-int(r)))
	}
	sub = be.AppendUint16(sub, 1)
	sub = append(sub, make([]byte, 2*n)...)
	out := []byte{0, 0, 0, 2, 0, 0, 0, 3, 0, 0, 0, 20, 0, 3, 0, 1, 0, 0, 0, 20}
	return append(out, sub...)
}

func rename(table []byte) ([]byte, error) {
	if be.Uint16(table[:2]) != 0 {
		return nil, fmt.Errorf("wordmark requires a format 0 name table")
	}
	count, offset := int(be.Uint16(table[2:4])), int(be.Uint16(table[4:6]))
	if offset < 6+count*12 || offset > len(table) {
		return nil, fmt.Errorf("invalid name table")
	}
	names := map[uint16]string{1: familyName, 2: "Regular", 3: fullName, 4: familyName, 6: postScriptName, 16: familyName, 17: "Regular"}
	out := bytes.Clone(table[:6+count*12])
	be.PutUint16(out[4:6], uint16(len(out)))
	var storage []byte
	for i := 0; i < count; i++ {
		record := out[6+i*12 : 6+(i+1)*12]
		start, length := offset+int(be.Uint16(record[10:12])), int(be.Uint16(record[8:10]))
		if start > len(table) || length > len(table)-start {
			return nil, fmt.Errorf("truncated name string")
		}
		value := table[start : start+length]
		if replacement, ok := names[be.Uint16(record[6:8])]; ok {
			platform := be.Uint16(record[:2])
			encoding := be.Uint16(record[2:4])
			if platform == 0 || platform == 3 || platform == 2 && encoding == 1 {
				value = nil
				for _, r := range utf16.Encode([]rune(replacement)) {
					value = be.AppendUint16(value, r)
				}
			} else if platform == 1 && encoding == 0 || platform == 2 && encoding == 0 {
				value = []byte(replacement)
			} else {
				return nil, fmt.Errorf("unsupported name encoding %d/%d", platform, encoding)
			}
		}
		if len(storage)+len(value) > math.MaxUint16 {
			return nil, fmt.Errorf("name table string storage too large")
		}
		be.PutUint16(record[8:10], uint16(len(value)))
		be.PutUint16(record[10:12], uint16(len(storage)))
		storage = append(storage, value...)
	}
	return append(out, storage...), nil
}

func readTables(data []byte) (map[string][]byte, error) {
	if len(data) < 12 || string(data[:4]) != "OTTO" {
		return nil, fmt.Errorf("source must be a CFF OpenType (.otf) font")
	}
	n := int(be.Uint16(data[4:6]))
	if 12+n*16 > len(data) {
		return nil, fmt.Errorf("truncated OTF directory")
	}
	tables := map[string][]byte{}
	for i := 0; i < n; i++ {
		record := data[12+i*16 : 12+(i+1)*16]
		tag := string(record[:4])
		offset, length := uint64(be.Uint32(record[8:12])), uint64(be.Uint32(record[12:16]))
		if offset+length > uint64(len(data)) {
			return nil, fmt.Errorf("truncated %q table", tag)
		}
		if _, ok := tables[tag]; ok {
			return nil, fmt.Errorf("duplicate %q table", tag)
		}
		tables[tag] = data[offset : offset+length]
	}
	return tables, nil
}

func checksum(data []byte) uint32 {
	var sum uint32
	for i := 0; i < len(data); i += 4 {
		var word [4]byte
		copy(word[:], data[i:min(i+4, len(data))])
		sum += be.Uint32(word[:])
	}
	return sum
}

func writeTables(tables map[string][]byte) []byte {
	tags := make([]string, 0, len(tables))
	for tag := range tables {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	n := len(tags)
	power, selector := 1, 0
	for power*2 <= n {
		power *= 2
		selector++
	}
	out := make([]byte, 12+n*16)
	copy(out, "OTTO")
	be.PutUint16(out[4:6], uint16(n))
	be.PutUint16(out[6:8], uint16(power*16))
	be.PutUint16(out[8:10], uint16(selector))
	be.PutUint16(out[10:12], uint16((n-power)*16))
	head := 0
	for i, tag := range tags {
		record := out[12+i*16 : 12+(i+1)*16]
		data := tables[tag]
		copy(record, tag)
		be.PutUint32(record[4:8], checksum(data))
		be.PutUint32(record[8:12], uint32(len(out)))
		be.PutUint32(record[12:16], uint32(len(data)))
		if tag == "head" {
			head = len(out)
		}
		out = append(out, data...)
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
	}
	be.PutUint32(out[head+8:head+12], 0xb1b0afba-checksum(out))
	return out
}
