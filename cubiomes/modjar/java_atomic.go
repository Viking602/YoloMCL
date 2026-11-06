package modjar

// Registration runs single-threaded, so atomics are plain cells. Their
// update functions run the supplied lambda exactly once, as an uncontended
// compare-and-set loop would.

type atomicCell struct{ value any }

func atomicBool(x any) int32 {
	if integer(x) != 0 {
		return 1
	}
	return 0
}

type semaphore struct{ permits int32 }

// semaphoreCall binds Semaphore for single-threaded registration: an acquire
// that would block can never be released by another thread, so it fails.
func (h *bindings) semaphoreCall(r reference, receiver any, args []any) (any, bool) {
	if r.owner != "java/util/concurrent/Semaphore" {
		return nil, false
	}
	o, ok := receiver.(*object)
	if !ok || o == nil {
		return nil, false
	}
	if r.name == "<init>" && (r.desc == "(I)V" || r.desc == "(IZ)V") {
		o.data = &semaphore{integer(args[0])}
		return nil, true
	}
	s, ok := o.data.(*semaphore)
	if !ok {
		return nil, false
	}
	switch r.name + r.desc {
	case "acquire()V", "acquireUninterruptibly()V":
		if s.permits <= 0 {
			fail("semaphore acquire would block registration")
		}
		s.permits--
		return nil, true
	case "tryAcquire()Z":
		if s.permits <= 0 {
			return int32(0), true
		}
		s.permits--
		return int32(1), true
	case "release()V":
		s.permits++
		return nil, true
	case "availablePermits()I":
		return s.permits, true
	}
	return nil, false
}

func (h *bindings) atomicCall(r reference, receiver any, args []any) (any, bool) {
	if x, ok := h.semaphoreCall(r, receiver, args); ok {
		return x, true
	}
	v := h.vm
	var zero any
	switch r.owner {
	case "java/util/concurrent/atomic/AtomicReference":
		zero = nil
	case "java/util/concurrent/atomic/AtomicBoolean":
		zero = int32(0)
	case "java/util/concurrent/atomic/AtomicLong":
		zero = int64(0)
	case "java/util/concurrent/atomic/AtomicInteger":
		zero = int32(0)
	default:
		return nil, false
	}
	o, ok := receiver.(*object)
	if !ok || o == nil {
		return nil, false
	}
	if r.name == "<init>" {
		switch r.desc {
		case "()V":
			o.data = &atomicCell{zero}
		case "(Ljava/lang/Object;)V":
			o.data = &atomicCell{args[0]}
		case "(Z)V":
			o.data = &atomicCell{atomicBool(args[0])}
		case "(J)V":
			o.data = &atomicCell{args[0].(int64)}
		case "(I)V":
			o.data = &atomicCell{integer(args[0])}
		default:
			return nil, false
		}
		return nil, true
	}
	c, ok := o.data.(*atomicCell)
	if !ok {
		return nil, false
	}
	add := func(d int64) any {
		if n, ok := c.value.(int64); ok {
			return n + d
		}
		return integer(c.value) + int32(d)
	}
	same := func(a, b any) bool {
		if r.owner == "java/util/concurrent/atomic/AtomicReference" {
			return equal(a, b) // identity, as in the JDK
		}
		return a == b
	}
	arg := func(x any) any {
		if r.owner == "java/util/concurrent/atomic/AtomicBoolean" {
			return atomicBool(x)
		}
		return x
	}
	switch r.name {
	case "get", "getPlain", "getAcquire", "getOpaque", "intValue", "longValue":
		if len(args) == 0 {
			return c.value, true
		}
	case "set", "lazySet", "setPlain", "setRelease", "setOpaque":
		if len(args) == 1 {
			c.value = arg(args[0])
			return nil, true
		}
	case "getAndSet":
		old := c.value
		c.value = arg(args[0])
		return old, true
	case "compareAndSet", "weakCompareAndSet", "weakCompareAndSetPlain", "weakCompareAndSetVolatile":
		if same(c.value, arg(args[0])) {
			c.value = arg(args[1])
			return int32(1), true
		}
		return int32(0), true
	case "incrementAndGet", "decrementAndGet", "getAndIncrement", "getAndDecrement", "addAndGet", "getAndAdd":
		if r.owner != "java/util/concurrent/atomic/AtomicInteger" && r.owner != "java/util/concurrent/atomic/AtomicLong" {
			return nil, false
		}
		d := int64(1)
		switch r.name {
		case "decrementAndGet", "getAndDecrement":
			d = -1
		case "addAndGet", "getAndAdd":
			if n, ok := args[0].(int64); ok {
				d = n
			} else {
				d = int64(integer(args[0]))
			}
		}
		old := c.value
		c.value = add(d)
		if r.name == "getAndIncrement" || r.name == "getAndDecrement" || r.name == "getAndAdd" {
			return old, true
		}
		return c.value, true
	case "updateAndGet", "getAndUpdate":
		l, ok := args[0].(*lambda)
		if !ok {
			return nil, false
		}
		old := c.value
		c.value = v.callLambda(l, []any{old})
		if r.name == "getAndUpdate" {
			return old, true
		}
		return c.value, true
	case "accumulateAndGet", "getAndAccumulate":
		l, ok := args[1].(*lambda)
		if !ok {
			return nil, false
		}
		old := c.value
		c.value = v.callLambda(l, []any{old, args[0]})
		if r.name == "getAndAccumulate" {
			return old, true
		}
		return c.value, true
	case "toString":
		return nil, false
	}
	return nil, false
}
