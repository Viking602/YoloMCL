package main

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

func TestClimateTableBinaryLayout(t *testing.T) {
	source := `static const int param[][2] = {{-10,20},{30,40}};
static const uint64_t nodes[] = {0x0123456789ABCDEF,0xFFFFFFFFFFFFFFFF};`
	got, err := climateTable(source)
	if err != nil {
		t.Fatal(err)
	}
	want, err := hex.DecodeString("0200000002000000f6ffffff140000001e00000028000000efcdab8967452301ffffffffffffffff")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("binary order/width changed: %x", got)
	}
	for _, bad := range []string{"", strings.ReplaceAll(source, "-10", "2147483648"), strings.ReplaceAll(source, "};", "")} {
		if _, err := climateTable(bad); err == nil {
			t.Fatalf("accepted malformed source %q", bad)
		}
	}
}

func TestComparisonKeepsLargeIntegersExact(t *testing.T) {
	for _, tc := range []struct {
		left, right string
		equal       bool
	}{
		{`{"a":9007199254740993,"b":1.0}`, `{"b":1e0,"a":9007199254740993}`, true},
		{`{"a":9007199254740993}`, `{"a":9007199254740992}`, false},
		{`{"a":1} true`, `{"a":1}`, false},
	} {
		err := compareData("data.json", []byte(tc.left), []byte(tc.right))
		if (err == nil) != tc.equal {
			t.Fatalf("compare %s and %s: %v", tc.left, tc.right, err)
		}
	}
}

func TestGzipReproducible(t *testing.T) {
	a, b := &builder{files: map[string][]byte{}}, &builder{files: map[string][]byte{}}
	for _, builder := range []*builder{a, b} {
		if err := builder.gzip("test", []byte("stable content")); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(a.files["test"], b.files["test"]) {
		t.Fatal("gzip changed across runs")
	}
	if !bytes.Equal(a.files["test"][4:8], []byte{0, 0, 0, 0}) {
		t.Fatal("gzip includes a timestamp")
	}
}
