package modjar

// javaDuration is java.time.Duration: seconds plus a nanosecond adjustment
// in [0, 1e9). Only exact constructors and readers are bound; overflow raises
// ArithmeticException as in the JDK.
type javaDuration struct {
	seconds int64
	nanos   int32
}

func durationOf(seconds int64, nanos int64) javaDuration {
	q, r := nanos/1e9, nanos%1e9
	if r < 0 {
		q, r = q-1, r+1e9
	}
	return javaDuration{addExact(seconds, q), int32(r)}
}

func multiplyExact(a, b int64) int64 {
	if a != 0 && (a*b/a != b || a == -1 && b == -1<<63 || b == -1 && a == -1<<63) {
		raiseJava("java/lang/ArithmeticException", "long overflow")
	}
	return a * b
}

func (h *bindings) durationCall(r reference, receiver any, args []any) (any, bool) {
	if r.owner != "java/time/Duration" {
		return nil, false
	}
	v := h.vm
	make := func(d javaDuration) any {
		if d == (javaDuration{}) {
			return h.symbol("java/time/Duration.ZERO", r.owner, d)
		}
		return v.alloc(r.owner, d)
	}
	if receiver == nil {
		switch r.name + r.desc {
		case "ofSeconds(J)Ljava/time/Duration;":
			return make(javaDuration{args[0].(int64), 0}), true
		case "ofSeconds(JJ)Ljava/time/Duration;":
			return make(durationOf(args[0].(int64), args[1].(int64))), true
		case "ofMillis(J)Ljava/time/Duration;":
			ms := args[0].(int64)
			return make(durationOf(ms/1000, (ms%1000)*1e6)), true
		case "ofNanos(J)Ljava/time/Duration;":
			return make(durationOf(0, args[0].(int64))), true
		case "ofMinutes(J)Ljava/time/Duration;":
			return make(javaDuration{multiplyExact(args[0].(int64), 60), 0}), true
		case "ofHours(J)Ljava/time/Duration;":
			return make(javaDuration{multiplyExact(args[0].(int64), 3600), 0}), true
		case "ofDays(J)Ljava/time/Duration;":
			return make(javaDuration{multiplyExact(args[0].(int64), 86400), 0}), true
		}
		return nil, false
	}
	d, ok := objectData(receiver).(javaDuration)
	if !ok {
		return nil, false
	}
	switch r.name + r.desc {
	case "getSeconds()J", "toSeconds()J":
		return d.seconds, true
	case "getNano()I":
		return d.nanos, true
	case "isZero()Z":
		return boolInt(d == javaDuration{}), true
	case "isNegative()Z":
		return boolInt(d.seconds < 0), true
	case "toMillis()J":
		s := d.seconds
		n := int64(d.nanos)
		if s < 0 {
			s, n = s+1, n-1e9
		}
		return addExact(multiplyExact(s, 1000), n/1e6), true
	case "toNanos()J":
		s := d.seconds
		n := int64(d.nanos)
		if s < 0 {
			s, n = s+1, n-1e9
		}
		return addExact(multiplyExact(s, 1e9), n), true
	case "compareTo(Ljava/time/Duration;)I":
		o := objectData(args[0]).(javaDuration)
		switch {
		case d.seconds != o.seconds:
			return boolSign(d.seconds < o.seconds), true
		case d.nanos != o.nanos:
			return boolSign(d.nanos < o.nanos), true
		}
		return int32(0), true
	case "equals(Ljava/lang/Object;)Z":
		o, same := objectData(args[0]).(javaDuration)
		return boolInt(same && o == d), true
	case "hashCode()I":
		return int32(d.seconds^d.seconds>>32) + 51*d.nanos, true
	}
	return nil, false
}

func addExact(a, b int64) int64 {
	c := a + b
	if (a >= 0) == (b >= 0) && (c >= 0) != (a >= 0) {
		raiseJava("java/lang/ArithmeticException", "long overflow")
	}
	return c
}

func boolInt(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

func boolSign(less bool) int32 {
	if less {
		return -1
	}
	return 1
}
