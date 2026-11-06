package modjar

// The no-argument constructor has a process/time-dependent seed. Preserve that
// unknown state until setSeed supplies an input; never invent a world seed.
type javaRandom struct {
	seed  uint64
	known bool
}

func (h *bindings) randomCall(r reference, receiver any, args []any) (any, bool) {
	if r.owner != "java/util/Random" {
		return nil, false
	}
	if r.name == "<init>" && (r.desc == "()V" || r.desc == "(J)V") {
		o := receiver.(*object)
		if o.class != r.owner {
			fail("Random subclass initialization is not bound")
		}
		x := &javaRandom{}
		if len(args) == 1 {
			x.seed = (uint64(args[0].(int64)) ^ 0x5deece66d) & ((1 << 48) - 1)
			x.known = true
		}
		o.data = x
		return nil, true
	}
	x, ok := objectData(receiver).(*javaRandom)
	if !ok {
		return nil, false
	}
	if r.name == "setSeed" && r.desc == "(J)V" {
		x.seed = (uint64(args[0].(int64)) ^ 0x5deece66d) & ((1 << 48) - 1)
		x.known = true
		return nil, true
	}
	next := func(bits uint) int32 {
		h.vm.tick()
		if !x.known {
			fail("time-seeded Random was observed before setSeed")
		}
		x.seed = (x.seed*0x5deece66d + 11) & ((1 << 48) - 1)
		return int32(x.seed >> (48 - bits))
	}
	switch r.name + r.desc {
	case "nextInt()I":
		return next(32), true
	case "nextInt(I)I":
		bound := integer(args[0])
		if bound <= 0 {
			raiseJava("java/lang/IllegalArgumentException", "bound must be positive")
		}
		if bound&(bound-1) == 0 {
			return int32(int64(bound) * int64(next(31)) >> 31), true
		}
		for {
			bits := next(31)
			value := bits % bound
			if bits-value+(bound-1) >= 0 {
				return value, true
			}
		}
	case "nextLong()J":
		return int64(next(32))<<32 + int64(next(32)), true
	case "nextBoolean()Z":
		return next(1), true
	case "nextFloat()F":
		return float32(next(24)) / (1 << 24), true
	case "nextDouble()D":
		return float64(int64(next(26))<<27+int64(next(27))) / (1 << 53), true
	}
	return nil, false
}
