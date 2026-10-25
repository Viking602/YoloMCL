package launcher

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// LevelInfo is the part of level.dat the launcher shows.
type LevelInfo struct {
	Name       string
	GameType   int32
	Hardcore   bool
	LastPlayed int64
}

const (
	maxLevelData = 64 << 20
	maxNBTDepth  = 512
)

// ParseLevelDat reads LevelName, GameType, hardcore and LastPlayed from a
// gzipped NBT level.dat without decoding anything else. Reading stops once
// those fields are known, so a huge payload after them is never loaded.
func ParseLevelDat(r io.Reader) (LevelInfo, error) {
	gz, e := gzip.NewReader(r)
	if e != nil {
		return LevelInfo{}, e
	}
	defer gz.Close()
	p := &nbtReader{r: &limitedReader{r: bufio.NewReader(gz), n: maxLevelData}}
	var li LevelInfo
	if p.u8() != 10 { // root compound
		return li, errors.New("not an NBT compound")
	}
	p.str()
	got := 0
	p.compound(func(path, name string, tag byte) bool {
		if path != "/Data" {
			return false
		}
		switch {
		case name == "LevelName" && tag == 8:
			li.Name = p.str()
		case name == "GameType" && tag == 3:
			li.GameType = int32(p.u32())
		case name == "hardcore" && tag == 1:
			li.Hardcore = p.u8() != 0
		case name == "LastPlayed" && tag == 4:
			li.LastPlayed = int64(p.u64())
		default:
			return false
		}
		got++
		if got == 4 {
			p.done = true
		}
		return true
	}, "")
	if p.err != nil {
		return li, p.err
	}
	return li, nil
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
