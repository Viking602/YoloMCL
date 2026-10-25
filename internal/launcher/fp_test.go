package launcher

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
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

func TestFingerprintFileMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(17))
	path := filepath.Join(t.TempDir(), "pack.zip")
	for _, size := range []int{0, 1, 3, 4, 65535, 65536, 65537, 2<<20 + 3} {
		data := make([]byte, size)
		_, _ = rng.Read(data)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := fingerprintFile(context.Background(), path)
		if err != nil || got != refFingerprint(data) {
			t.Fatalf("size=%d: fingerprint=%d err=%v", size, got, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fingerprintFile(ctx, path); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled fingerprint continued: %v", err)
	}
	if _, err := fingerprintFile(context.Background(), path+".missing"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file error was lost: %v", err)
	}
}

func BenchmarkFingerprintFile(b *testing.B) {
	for _, size := range []int{64 << 10, 32 << 20} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			path := filepath.Join(b.TempDir(), "pack.zip")
			data := make([]byte, size)
			_, _ = rand.New(rand.NewSource(7)).Read(data)
			if err := os.WriteFile(path, data, 0o600); err != nil {
				b.Fatal(err)
			}
			want := curseFingerprint(data)
			data = nil
			b.ReportAllocs()
			b.SetBytes(int64(size))
			for b.Loop() {
				got, err := fingerprintFile(context.Background(), path)
				if err != nil || got != want {
					b.Fatalf("fingerprint=%d err=%v", got, err)
				}
			}
		})
	}
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
		for _, chunk := range []int{1, 3, 4, 5, 17} {
			got, err := streamFingerprint(context.Background(), bytes.NewReader(b), uint32(fingerprintLength(b)), make([]byte, chunk))
			if err != nil || got != refFingerprint(b) {
				t.Fatalf("stream chunk=%d mismatch for %v: %d, %v", chunk, b, got, err)
			}
		}
	}
}
