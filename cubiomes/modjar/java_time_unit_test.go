package modjar

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestTimeUnitConversionBoundaries(t *testing.T) {
	v := newMachine(context.Background(), nil)
	v.host = &bindings{vm: v}
	values := []int64{math.MinInt64, math.MinInt64 + 1, -1000000001, -1000, -999, -1, 0, 1, 999, 1000, 1000000001, math.MaxInt64 - 1, math.MaxInt64}
	max, min := big.NewInt(math.MaxInt64), big.NewInt(math.MinInt64)
	for _, source := range timeUnits {
		for _, target := range timeUnits {
			src, _ := v.host.timeUnit(source.name)
			dst, _ := v.host.timeUnit(target.name)
			for _, n := range append(append([]int64(nil), values...), math.MaxInt64/source.nanos, math.MaxInt64/source.nanos+1) {
				exact := new(big.Int).Mul(big.NewInt(n), big.NewInt(source.nanos))
				exact.Quo(exact, big.NewInt(target.nanos))
				if exact.Cmp(max) > 0 {
					exact.Set(max)
				}
				if exact.Cmp(min) < 0 {
					exact.Set(min)
				}
				got := v.method(reference{timeUnitClass, "convert", "(JLjava/util/concurrent/TimeUnit;)J"}, dst, true, []any{n, src})
				direct := v.method(reference{timeUnitClass, target.method, "(J)J"}, src, true, []any{n})
				if got != exact.Int64() || direct != got {
					t.Fatalf("%s -> %s (%d): %v / %v / %s", source.name, target.name, n, got, direct, exact)
				}
			}
		}
	}
	for i, unit := range timeUnits {
		o := v.static(reference{timeUnitClass, unit.name, "L" + timeUnitClass + ";"})
		if got := v.method(reference{"java/lang/Enum", "ordinal", "()I"}, o, true, nil); got != int32(i) {
			t.Fatal(got)
		}
	}
	expectJavaException(t, "java/lang/NullPointerException", func() {
		v.method(reference{timeUnitClass, "convert", "(JLjava/util/concurrent/TimeUnit;)J"}, nil, true, []any{int64(1), nil})
	})
}
