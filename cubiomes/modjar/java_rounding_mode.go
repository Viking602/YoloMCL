package modjar

import "slices"

const roundingModeClass = "java/math/RoundingMode"

var roundingModes = []string{"UP", "DOWN", "CEILING", "FLOOR", "HALF_UP", "HALF_DOWN", "HALF_EVEN", "UNNECESSARY"}

// RoundingMode is a plain JDK enum; Guava's IntMath switches on its ordinal.
func (h *bindings) roundingMode(name string) (*object, bool) {
	i := slices.Index(roundingModes, name)
	if i < 0 {
		return nil, false
	}
	o := h.symbol(roundingModeClass+"."+name, roundingModeClass, name)
	o.fields["java/lang/Enum.name"] = name
	o.fields["java/lang/Enum.ordinal"] = int32(i)
	return o, true
}

func (h *bindings) roundingModeCall(r reference, receiver any, args []any) (any, bool) {
	if r.owner != roundingModeClass {
		return nil, false
	}
	switch {
	case r.name == "values" && r.desc == "()[Ljava/math/RoundingMode;":
		h.vm.reserveCollection(len(roundingModes))
		h.vm.alloc("[", nil)
		a := &array{descriptor: "[Ljava/math/RoundingMode;", values: make([]any, len(roundingModes))}
		for i, name := range roundingModes {
			a.values[i], _ = h.roundingMode(name)
		}
		return a, true
	case r.name == "valueOf" && r.desc == "(Ljava/lang/String;)Ljava/math/RoundingMode;":
		if o, ok := h.roundingMode(javaString(args[0])); ok {
			return o, true
		}
		raiseJava("java/lang/IllegalArgumentException", "unknown rounding mode")
	case (r.name == "name" || r.name == "toString") && r.desc == "()Ljava/lang/String;" || r.name == "ordinal" && r.desc == "()I" || r.name == "hashCode" || r.name == "equals":
		r.owner = "java/lang/Enum"
		return h.javaCall(r, receiver, args)
	}
	return nil, false
}
