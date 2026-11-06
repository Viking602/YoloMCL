package cubiomes

import (
	"compress/gzip"
	"encoding/csv"
	"math"
	"os"
	"strconv"
	"testing"
)

func TestCellularPublicSourceReference(t *testing.T) {
	f, err := os.Open("testdata/lithostitched-cellular.csv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	rows, err := csv.NewReader(z).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for i, row := range rows {
		seed, err := strconv.ParseInt(row[0], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		v := make([]float64, 4)
		for j := range v {
			v[j], err = strconv.ParseFloat(row[j+1], 64)
			if err != nil {
				t.Fatal(err)
			}
		}
		want, err := strconv.ParseUint(row[5], 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		n := cellularNoise{seed: int32(seed) + 304658814, salt: 304658814, frequency: .002, jitter: float32(v[0])}
		if got := math.Float32bits(float32(n.sample(v[1], v[2], v[3]))); got != uint32(want) {
			t.Fatalf("row %d: %x != %x", i, got, want)
		}
	}
	t.Logf("%d bit-exact samples from unchanged public FNL source", len(rows))
}
