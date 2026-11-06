package modjar

import (
	"context"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestJavaRandomIndependentOracleAndUnknownSeed(t *testing.T) {
	v := newMachine(context.Background(), nil)
	v.host = &bindings{vm: v}
	o := v.alloc("java/util/Random", nil)
	call := func(name, desc string, args ...any) any {
		return v.method(reference{o.class, name, desc}, o, true, args)
	}
	call("<init>", "()V")
	expectFault(t, "time-seeded", func() { call("nextInt", "()I") })
	data, err := os.ReadFile("testdata/java-random-21.tsv")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 8 {
			t.Fatal(line)
		}
		seed, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		if parts[1] == "0" {
			call("setSeed", "(J)V", seed)
		}
		ints := []int64{int64(call("nextInt", "()I").(int32)), call("nextLong", "()J").(int64), int64(call("nextInt", "(I)I", int32(1073741825)).(int32)), int64(call("nextInt", "(I)I", int32(32)).(int32))}
		for i, got := range ints {
			want, err := strconv.ParseInt(parts[i+2], 10, 64)
			if err != nil || got != want {
				t.Fatalf("%s sample %s #%d: %d want %d", parts[0], parts[1], i, got, want)
			}
		}
		for i, got := range []uint64{math.Float64bits(call("nextDouble", "()D").(float64)), uint64(math.Float32bits(call("nextFloat", "()F").(float32)))} {
			want, err := strconv.ParseUint(parts[i+6], 16, 64)
			if err != nil || got != want {
				t.Fatalf("random float mismatch %s", line)
			}
		}
		count++
	}
	if count != 240 {
		t.Fatal("incomplete oracle", count)
	}
}
