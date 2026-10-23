package launcher

import (
	"encoding/binary"
	"math/rand"
	"testing"
)

func refFingerprint(data []byte) uint32 {
	b := make([]byte, 0, len(data))
	for _, v := range data {
		if v != 9 && v != 10 && v != 13 && v != 32 {
			b = append(b, v)
		}
	}
	const mul uint32 = 0x5bd1e995
	h := uint32(1) ^ uint32(len(b))
	for len(b) >= 4 {
		k := binary.LittleEndian.Uint32(b)
		k *= mul
		k ^= k >> 24
		k *= mul
		h *= mul
		h ^= k
		b = b[4:]
	}
	switch len(b) {
	case 3:
		h ^= uint32(b[2]) << 16
		fallthrough
	case 2:
		h ^= uint32(b[1]) << 8
		fallthrough
	case 1:
		h ^= uint32(b[0])
		h *= mul
	}
	h ^= h >> 13
	h *= mul
	h ^= h >> 15
	return h
}

func TestCurseFingerprintMatchesReference(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for n := 0; n < 300; n++ {
		b := make([]byte, r.Intn(70))
		for i := range b {
			b[i] = []byte{9, 10, 13, 32, 'a', 0xff, 0}[r.Intn(7)]
		}
		if curseFingerprint(b) != refFingerprint(b) {
			t.Fatalf("mismatch for %v", b)
		}
	}
}
