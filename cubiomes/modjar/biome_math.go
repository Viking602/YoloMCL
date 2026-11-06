package modjar

import "math"

const simplexClass = "net/minecraft/world/level/levelgen/synth/SimplexNoise"

// Math operations used by numerical worldgen programs. Keep float32 rounding,
// signed zero, NaNs and Java int overflow, including abs(MIN_VALUE).
func biomeMath(r reference, receiver any, args []any) (any, bool) {
	if r.owner == simplexClass && r.name == "getValue" && r.desc == "(DD)D" {
		if f, ok := objectData(receiver).(func(float64, float64) float64); ok {
			return f(args[0].(float64), args[1].(float64)), true
		}
	}
	if r.owner != "java/lang/Math" && r.owner != "net/minecraft/util/Mth" {
		return nil, false
	}
	if !pureBiomeNative(r) {
		return nil, false
	}
	switch r.name + r.desc {
	case "abs(I)I":
		x := integer(args[0])
		if x < 0 {
			x = -x
		}
		return x, true
	case "abs(F)F":
		return math.Float32frombits(math.Float32bits(args[0].(float32)) & 0x7fffffff), true
	case "sqrt(F)F":
		return float32(math.Sqrt(float64(args[0].(float32)))), true
	case "min(FF)F":
		return float32(math.Min(float64(args[0].(float32)), float64(args[1].(float32)))), true
	case "max(FF)F":
		return float32(math.Max(float64(args[0].(float32)), float64(args[1].(float32)))), true
	case "clamp(FFF)F":
		a, b, c := args[0].(float32), args[1].(float32), args[2].(float32)
		if a < b {
			return b, true
		}
		if a > c {
			return c, true
		}
		return a, true
	}
	return nil, false
}
