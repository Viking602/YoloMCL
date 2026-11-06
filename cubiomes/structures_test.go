package cubiomes

import (
	"math"
	"testing"
)

func TestRandomSpreadJavaReference(t *testing.T) {
	r := newNoiseRandom(0, true)
	for _, want := range []int{60, 48, 29, 47, 15} {
		if got := r.nextInt(100); got != want {
			t.Fatalf("Java nextInt: %d != %d", got, want)
		}
	}
	for _, tc := range []struct {
		seed                                       int64
		rx, rz                                     int
		linearX, linearZ, triangularX, triangularZ int
	}{
		{0, 0, 0, 248, 40, 344, 296},
		{0, -1, -1, -136, -536, -472, -312},
		{-1, -17, 31, -9160, 17112, -8456, 16296},
		{math.MinInt64, 22, -33, 12360, -17656, 11448, -16728},
	} {
		for _, p := range []RandomSpread{{34, 8, 10387312, false}, {32, 5, 10387313, true}} {
			x, z, ok := p.Candidate(tc.seed, tc.rx, tc.rz)
			wx, wz := tc.linearX, tc.linearZ
			if p.Triangular {
				wx, wz = tc.triangularX, tc.triangularZ
			}
			if !ok || x != wx || z != wz {
				t.Fatalf("%+v seed %d: (%d,%d) != (%d,%d)", p, tc.seed, x, z, wx, wz)
			}
		}
	}
	for _, p := range []RandomSpread{{}, {1, 1, 0, false}, {4097, 0, 0, false}, {32, 4, -1, false}} {
		if _, _, ok := p.Candidate(0, 0, 0); ok {
			t.Fatalf("accepted %+v", p)
		}
	}
	p := RandomSpread{34, 8, 10387312, false}
	if _, _, ok := p.Candidate(0, math.MaxInt, math.MinInt); ok {
		t.Fatal("overflow accepted")
	}
	if n := testing.AllocsPerRun(100, func() { p.Candidate(-1, -37, 17) }); n != 0 {
		t.Fatalf("candidate allocates: %g", n)
	}
}

func BenchmarkRandomSpread(b *testing.B) {
	p := RandomSpread{34, 8, 10387312, false}
	b.ReportAllocs()
	var seed int64
	for b.Loop() {
		seed++
		if _, _, ok := p.Candidate(seed, -17, 31); !ok {
			b.Fatal("invalid placement")
		}
	}
}
