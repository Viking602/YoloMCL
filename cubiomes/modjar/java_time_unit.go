package modjar

import "math"

const timeUnitClass = "java/util/concurrent/TimeUnit"

var timeUnits = [...]struct {
	name, method string
	nanos        int64
}{
	{"NANOSECONDS", "toNanos", 1}, {"MICROSECONDS", "toMicros", 1000}, {"MILLISECONDS", "toMillis", 1000000},
	{"SECONDS", "toSeconds", 1000000000}, {"MINUTES", "toMinutes", 60000000000}, {"HOURS", "toHours", 3600000000000}, {"DAYS", "toDays", 86400000000000},
}

func (h *bindings) timeUnit(name string) (*object, bool) {
	for i, unit := range timeUnits {
		if unit.name == name {
			o := h.symbol(timeUnitClass+"."+name, timeUnitClass, unit.nanos)
			o.fields["java/lang/Enum.name"] = name
			o.fields["java/lang/Enum.ordinal"] = int32(i)
			return o, true
		}
	}
	return nil, false
}

// TimeUnit's numeric contract is truncation toward zero and saturation on
// overflow. No clock, sleep, thread, or host JVM is involved.
func (h *bindings) timeUnitCall(r reference, receiver any, args []any) (any, bool) {
	if r.owner != timeUnitClass {
		return nil, false
	}
	if r.name == "valueOf" && r.desc == "(Ljava/lang/String;)Ljava/util/concurrent/TimeUnit;" {
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "null time unit")
		}
		if unit, ok := h.timeUnit(javaString(args[0])); ok {
			return unit, true
		}
		raiseJava("java/lang/IllegalArgumentException", "unknown time unit")
	}
	if r.name == "values" && r.desc == "()[Ljava/util/concurrent/TimeUnit;" {
		h.vm.reserveCollection(len(timeUnits))
		h.vm.alloc("[", nil)
		a := &array{descriptor: "[Ljava/util/concurrent/TimeUnit;", values: make([]any, len(timeUnits))}
		for i, unit := range timeUnits {
			a.values[i], _ = h.timeUnit(unit.name)
		}
		return a, true
	}
	if (r.name == "name" || r.name == "toString") && r.desc == "()Ljava/lang/String;" || r.name == "ordinal" && r.desc == "()I" {
		r.owner = "java/lang/Enum"
		return h.javaCall(r, receiver, args)
	}
	target := int64(0)
	if r.desc == "(J)J" {
		for _, unit := range timeUnits {
			if r.name == unit.method {
				target = unit.nanos
				break
			}
		}
	}
	convert := r.name == "convert" && r.desc == "(JLjava/util/concurrent/TimeUnit;)J"
	if target == 0 && !convert {
		return nil, false
	}
	if receiver == nil {
		raiseJava("java/lang/NullPointerException", "null time unit")
	}
	source := objectData(receiver).(int64)
	if convert {
		if args[1] == nil {
			raiseJava("java/lang/NullPointerException", "null source time unit")
		}
		target = source
		source = objectData(args[1]).(int64)
	}
	n := args[0].(int64)
	if source <= target {
		return n / (target / source), true
	}
	factor := source / target
	if n > math.MaxInt64/factor {
		return int64(math.MaxInt64), true
	}
	if n < math.MinInt64/factor {
		return int64(math.MinInt64), true
	}
	return n * factor, true
}
