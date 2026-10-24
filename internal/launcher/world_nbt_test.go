package launcher

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"testing"
)

func compressedNBT(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestParseLevelDatRejectsMalformedNBT(t *testing.T) {
	root := []byte{10, 0, 0}
	cases := map[string][]byte{
		"missing compound end": root,
		"truncated name":       {10, 0, 10, 'x'},
		"truncated value":      {10, 0, 0, 4, 0, 0, 1, 2},
		"invalid tag":          {10, 0, 0, 99, 0, 0, 0},
		"negative list":        {10, 0, 0, 9, 0, 0, 1, 255, 255, 255, 255, 0},
		"nonempty end list":    {10, 0, 0, 9, 0, 0, 0, 0, 0, 0, 1, 0},
	}
	for _, tag := range []byte{7, 11, 12} {
		for _, count := range []uint32{0x7fffffff, 0x80000000, 0xffffffff} {
			data := append(bytes.Clone(root), tag, 0, 0)
			data = binary.BigEndian.AppendUint32(data, count)
			data = append(data, 0)
			// These tiny malformed inputs used to allocate count * element size.
			if _, err := ParseLevelDat(bytes.NewReader(compressedNBT(t, data))); err == nil {
				t.Errorf("accepted array tag %d, count %d", tag, count)
			}
		}
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseLevelDat(bytes.NewReader(compressedNBT(t, data))); err == nil {
				t.Fatal("malformed NBT accepted")
			}
		})
	}
}

func TestParseLevelDatBoundsNesting(t *testing.T) {
	for _, tag := range []byte{9, 10} {
		data := []byte{10, 0, 0, tag, 0, 0}
		for range maxNBTDepth + 1 {
			if tag == 9 {
				data = append(data, 9, 0, 0, 0, 1) // list containing another list
			} else {
				data = append(data, 10, 0, 0) // unnamed compound
			}
		}
		data = append(data, make([]byte, maxNBTDepth+10)...)
		if _, err := ParseLevelDat(bytes.NewReader(compressedNBT(t, data))); err == nil {
			t.Errorf("accepted excessive nesting of tag %d", tag)
		}
	}
}

func TestParseLevelDatEmptyArraysAndLists(t *testing.T) {
	data := []byte{10, 0, 0}
	for _, tag := range []byte{7, 9, 11, 12} {
		data = append(data, tag, 0, 0)
		if tag == 9 {
			data = append(data, 0) // TAG_End is valid for an empty list.
		}
		data = append(data, 0, 0, 0, 0)
	}
	data = append(data, 0)
	if _, err := ParseLevelDat(bytes.NewReader(compressedNBT(t, data))); err != nil {
		t.Fatal(err)
	}
}
