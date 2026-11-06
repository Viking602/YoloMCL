package modjar

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type javaPair struct{ left, right any }
type javaUUID struct{ high, low int64 }

// Value types have no process, clock, filesystem, or platform dependency.
func (h *bindings) javaValuesCall(r reference, receiver any, args []any) (any, bool) {
	if r.name == "equals" && r.desc == "(Ljava/lang/Object;)Z" {
		if a, ok := receiver.(*object); ok && a != nil {
			var equal bool
			b, ok := args[0].(*object)
			same := ok && b != nil && b.class == a.class
			switch a.class {
			case "java/lang/String":
				if text, ok := args[0].(string); ok {
					equal = javaString(a) == text
				} else if same {
					equal = javaString(a) == javaString(b)
				}
			case "java/lang/Boolean", "java/lang/Byte", "java/lang/Short", "java/lang/Character", "java/lang/Integer", "java/lang/Long":
				equal = same && a.data == b.data
			case "java/lang/Float":
				if same {
					x, y := a.data.(float32), b.data.(float32)
					equal = math.Float32bits(x) == math.Float32bits(y) || math.IsNaN(float64(x)) && math.IsNaN(float64(y))
				}
			case "java/lang/Double":
				if same {
					x, y := a.data.(float64), b.data.(float64)
					equal = math.Float64bits(x) == math.Float64bits(y) || math.IsNaN(x) && math.IsNaN(y)
				}
			default:
				return h.nonNumericValueCall(r, receiver, args)
			}
			if equal {
				return int32(1), true
			}
			return int32(0), true
		}
	}
	return h.nonNumericValueCall(r, receiver, args)
}

func (h *bindings) nonNumericValueCall(r reference, receiver any, args []any) (any, bool) {
	v := h.vm
	if r.name == "equals" && r.desc == "(Ljava/lang/Object;)Z" {
		if a, ok := receiver.(*object); ok && a != nil && nativeDeferredHolder(a.class) {
			if left, ok := a.data.(deferredValue); ok {
				if b, ok := args[0].(*object); ok && b != nil && nativeDeferredHolder(b.class) {
					right, ok := b.data.(deferredValue)
					if !ok {
						fail("unbound holder comparison")
					}
					if left.registry == right.registry && left.id == right.id {
						return int32(1), true
					}
					return int32(0), true
				}
				if v.instanceOf(args[0], "net/minecraft/core/Holder") {
					fail("unbound holder comparison")
				}
				return int32(0), true
			}
		}
	}
	if (r.owner == "java/lang/Object" || r.owner == "net/minecraft/resources/ResourceKey") && r.name == "equals" && r.desc == "(Ljava/lang/Object;)Z" {
		if receiver != nil && receiver == args[0] {
			return int32(1), true
		}
		if o, ok := receiver.(*object); ok {
			switch o.data.(type) {
			case *javaMap, *javaSet, *javaList, javaPair:
				fail("native structural equality is not bound")
			}
		}
		if receiver == nil {
			raiseJava("java/lang/NullPointerException", "null equality receiver")
		}
		if o, ok := receiver.(*object); ok && o.class == "java/util/UUID" {
			r.owner = o.class
			return h.javaValuesCall(r, receiver, args)
		}
		if o, ok := receiver.(*object); ok && o.class == "net/minecraft/resources/ResourceLocation" {
			r.owner = o.class
			return h.call(r, receiver, args)
		}
		if a, ok := receiver.(string); ok {
			b, ok := args[0].(string)
			if o, isObject := args[0].(*object); isObject && o != nil && o.class == "java/lang/String" {
				b, ok = javaString(o), true
			}
			if ok && a == b {
				return int32(1), true
			}
			return int32(0), true
		}
		if receiver == args[0] {
			return int32(1), true
		}
		return int32(0), true
	}
	if r.owner == "org/apache/commons/lang3/tuple/Pair" || r.owner == "org/apache/commons/lang3/tuple/ImmutablePair" {
		switch {
		case r.name == "of" && r.desc == "(Ljava/lang/Object;Ljava/lang/Object;)Lorg/apache/commons/lang3/tuple/Pair;":
			return v.alloc("org/apache/commons/lang3/tuple/ImmutablePair", javaPair{args[0], args[1]}), true
		case (r.name == "getLeft" || r.name == "getKey") && r.desc == "()Ljava/lang/Object;":
			return objectData(receiver).(javaPair).left, true
		case (r.name == "getRight" || r.name == "getValue") && r.desc == "()Ljava/lang/Object;":
			return objectData(receiver).(javaPair).right, true
		}
	}
	if r.owner == "com/mojang/datafixers/util/Pair" {
		switch {
		case r.name == "of" && r.desc == "(Ljava/lang/Object;Ljava/lang/Object;)Lcom/mojang/datafixers/util/Pair;":
			return v.alloc(r.owner, javaPair{args[0], args[1]}), true
		case r.name == "<init>" && r.desc == "(Ljava/lang/Object;Ljava/lang/Object;)V":
			receiver.(*object).data = javaPair{args[0], args[1]}
			return nil, true
		case r.name == "getFirst" && r.desc == "()Ljava/lang/Object;":
			return objectData(receiver).(javaPair).left, true
		case r.name == "getSecond" && r.desc == "()Ljava/lang/Object;":
			return objectData(receiver).(javaPair).right, true
		}
	}
	if r.owner != "java/util/UUID" {
		return nil, false
	}
	switch {
	case r.name == "fromString" && r.desc == "(Ljava/lang/String;)Ljava/util/UUID;":
		s := javaString(args[0])
		parts := strings.Split(s, "-")
		if len(s) > 36 || len(parts) != 5 {
			raiseJava("java/lang/IllegalArgumentException", "invalid UUID")
		}
		var values [5]int64
		for i, p := range parts {
			n, err := strconv.ParseInt(p, 16, 64)
			if err != nil {
				raiseJava("java/lang/NumberFormatException", "invalid UUID number")
			}
			values[i] = n
		}
		u := javaUUID{(values[0]&0xffffffff)<<32 | (values[1]&0xffff)<<16 | values[2]&0xffff, (values[3]&0xffff)<<48 | values[4]&0xffffffffffff}
		return v.alloc(r.owner, u), true
	case r.name == "<init>" && r.desc == "(JJ)V":
		receiver.(*object).data = javaUUID{args[0].(int64), args[1].(int64)}
		return nil, true
	case r.name == "getMostSignificantBits" && r.desc == "()J":
		return objectData(receiver).(javaUUID).high, true
	case r.name == "getLeastSignificantBits" && r.desc == "()J":
		return objectData(receiver).(javaUUID).low, true
	case r.name == "toString" && r.desc == "()Ljava/lang/String;":
		u := objectData(receiver).(javaUUID)
		return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", uint64(u.high)>>32, uint64(u.high)>>16&0xffff, uint64(u.high)&0xffff, uint64(u.low)>>48, uint64(u.low)&0xffffffffffff), true
	case r.name == "equals" && r.desc == "(Ljava/lang/Object;)Z":
		other, ok := args[0].(*object)
		if ok && other != nil && other.class == r.owner && other.data == objectData(receiver) {
			return int32(1), true
		}
		return int32(0), true
	case r.name == "hashCode" && r.desc == "()I":
		u := objectData(receiver).(javaUUID)
		x := u.high ^ u.low
		return int32(x>>32) ^ int32(x), true
	}
	return nil, false
}
