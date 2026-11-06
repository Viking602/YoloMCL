package modjar

import (
	"encoding/base64"
	"errors"
	"math"
	"math/big"
	"math/bits"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

func (h *bindings) javaCall(r reference, receiver any, args []any) (any, bool) {
	if x, ok := h.timeUnitCall(r, receiver, args); ok {
		return x, true
	}
	if (r.owner == "java/util/AbstractMap" || r.owner == "java/util/AbstractCollection" || r.owner == "java/util/AbstractList" || r.owner == "java/util/AbstractSet" || r.owner == "java/util/AbstractQueue" || r.owner == "java/util/AbstractSequentialList" || r.owner == "java/util/Vector" || r.owner == "java/util/Stack" || r.owner == "java/util/concurrent/locks/ReentrantLock" || r.owner == "java/util/concurrent/locks/ReentrantReadWriteLock" || r.owner == "java/util/concurrent/locks/ReentrantReadWriteLock$ReadLock" || r.owner == "java/util/concurrent/locks/ReentrantReadWriteLock$WriteLock" || r.owner == "java/util/concurrent/locks/AbstractQueuedSynchronizer" || r.owner == "java/util/concurrent/locks/AbstractOwnableSynchronizer") && r.name == "<init>" && (r.desc == "()V" || r.desc == "(Z)V" || r.desc == "(I)V" || r.desc == "(IF)V" || r.desc == "(Ljava/util/concurrent/locks/ReentrantReadWriteLock;)V") {
		if r.owner == "java/util/Vector" || r.owner == "java/util/Stack" {
			receiver.(*object).data = &javaList{}
		}
		return nil, true
	}
	if r.owner == "java/util/concurrent/locks/ReentrantReadWriteLock" || r.owner == "java/util/concurrent/locks/ReentrantLock" || r.owner == "java/util/concurrent/locks/ReentrantReadWriteLock$ReadLock" || r.owner == "java/util/concurrent/locks/ReentrantReadWriteLock$WriteLock" || r.owner == "java/util/concurrent/locks/Lock" || r.owner == "java/util/concurrent/locks/ReadWriteLock" {
		switch {
		case r.name == "readLock" && r.desc == "()Ljava/util/concurrent/locks/Lock;":
			return h.vm.alloc("java/util/concurrent/locks/ReentrantReadWriteLock$ReadLock", nil), true
		case r.name == "writeLock" && r.desc == "()Ljava/util/concurrent/locks/Lock;":
			return h.vm.alloc("java/util/concurrent/locks/ReentrantReadWriteLock$WriteLock", nil), true
		case (r.name == "lock" || r.name == "lockInterruptibly" || r.name == "unlock") && r.desc == "()V":
			return nil, true
		case r.name == "tryLock" && (r.desc == "()Z" || r.desc == "(JLjava/util/concurrent/TimeUnit;)Z"):
			return int32(1), true
		case r.name == "newCondition" && r.desc == "()Ljava/util/concurrent/locks/Condition;":
			return h.vm.alloc("java/util/concurrent/locks/Condition", nil), true
		case r.name == "isHeldByCurrentThread" && r.desc == "()Z":
			return int32(0), true
		case r.name == "getReadLockCount" && r.desc == "()I":
			return int32(0), true
		case r.name == "isWriteLocked" && r.desc == "()Z":
			return int32(0), true
		}
	}
	if (r.owner == "java/util/AbstractCollection" || r.owner == "java/util/AbstractList" || r.owner == "java/util/AbstractSet" || r.owner == "java/util/AbstractQueue" || r.owner == "java/util/AbstractSequentialList") && (r.name == "stream" && r.desc == "()Ljava/util/stream/Stream;" || r.name == "toArray" && r.desc == "()[Ljava/lang/Object;") {
		// Collection defaults on interpreted subclasses reduce to iterator().
		if o, ok := receiver.(*object); ok && o.class != r.owner && o.data == nil {
			v := h.vm
			it := v.method(reference{o.class, "iterator", "()Ljava/util/Iterator;"}, o, true, nil)
			var values []any
			for integer(v.method(reference{"java/util/Iterator", "hasNext", "()Z"}, it, true, nil)) != 0 {
				v.reserveCollection(1)
				values = append(values, v.method(reference{"java/util/Iterator", "next", "()Ljava/lang/Object;"}, it, true, nil))
			}
			if r.name == "toArray" {
				return &array{values: values, descriptor: "[Ljava/lang/Object;"}, true
			}
			return v.alloc("java/util/stream/Stream", &javaList{values: values, readonly: true}), true
		}
	}
	if r.owner == "java/math/BigInteger" {
		v := h.vm
		switch {
		case r.name == "valueOf" && r.desc == "(J)Ljava/math/BigInteger;":
			return v.alloc("java/math/BigInteger", &javaBigInt{n: big.NewInt(args[0].(int64))}), true
		case r.name == "ZERO" || r.name == "ONE" || r.name == "TEN":
			return nil, false // static fields via static()
		case receiver != nil:
			x, ok := objectData(receiver).(*javaBigInt)
			if !ok {
				break
			}
			switch {
			case r.name == "multiply" && r.desc == "(Ljava/math/BigInteger;)Ljava/math/BigInteger;":
				y := objectData(args[0]).(*javaBigInt)
				return v.alloc("java/math/BigInteger", &javaBigInt{n: new(big.Int).Mul(x.n, y.n)}), true
			case r.name == "divide" && r.desc == "(Ljava/math/BigInteger;)Ljava/math/BigInteger;":
				y := objectData(args[0]).(*javaBigInt)
				if y.n.Sign() == 0 {
					raiseJava("java/lang/ArithmeticException", "BigInteger divide by zero")
				}
				return v.alloc("java/math/BigInteger", &javaBigInt{n: new(big.Int).Div(x.n, y.n)}), true
			case r.name == "compareTo" && (r.desc == "(Ljava/math/BigInteger;)I" || r.desc == "(Ljava/lang/Object;)I"):
				y := objectData(args[0]).(*javaBigInt)
				return int32(x.n.Cmp(y.n)), true
			case r.name == "equals" && r.desc == "(Ljava/lang/Object;)Z":
				y, ok := objectData(args[0]).(*javaBigInt)
				if !ok || x.n.Cmp(y.n) != 0 {
					return int32(0), true
				}
				return int32(1), true
			case r.name == "toString" && r.desc == "()Ljava/lang/String;":
				return x.n.String(), true
			case r.name == "longValue" && r.desc == "()J":
				return x.n.Int64(), true
			case r.name == "intValue" && r.desc == "()I":
				return int32(x.n.Int64()), true
			}
		}
	}
	if r.owner == "java/lang/Math" {
		switch {
		case r.name == "min" && r.desc == "(II)I":
			a, b := integer(args[0]), integer(args[1])
			if a < b {
				return a, true
			}
			return b, true
		case r.name == "max" && r.desc == "(II)I":
			a, b := integer(args[0]), integer(args[1])
			if a > b {
				return a, true
			}
			return b, true
		case r.name == "min" && r.desc == "(JJ)J":
			a, b := args[0].(int64), args[1].(int64)
			if a < b {
				return a, true
			}
			return b, true
		case r.name == "max" && r.desc == "(JJ)J":
			a, b := args[0].(int64), args[1].(int64)
			if a > b {
				return a, true
			}
			return b, true
		case r.name == "min" && r.desc == "(DD)D":
			return math.Min(number(args[0]), number(args[1])), true
		case r.name == "max" && r.desc == "(DD)D":
			return math.Max(number(args[0]), number(args[1])), true
		case r.name == "ceil" && r.desc == "(D)D":
			return math.Ceil(number(args[0])), true
		case r.name == "floor" && r.desc == "(D)D":
			return math.Floor(number(args[0])), true
		case r.name == "abs" && r.desc == "(I)I":
			n := integer(args[0])
			if n == math.MinInt32 {
				return n, true
			}
			if n < 0 {
				return -n, true
			}
			return n, true
		case r.name == "abs" && r.desc == "(J)J":
			n := args[0].(int64)
			if n == math.MinInt64 {
				return n, true
			}
			if n < 0 {
				return -n, true
			}
			return n, true
		case r.name == "abs" && r.desc == "(D)D":
			return math.Abs(number(args[0])), true
		case r.name == "round" && r.desc == "(D)J":
			return int64(math.Round(number(args[0]))), true
		case r.name == "sqrt" && r.desc == "(D)D":
			return math.Sqrt(number(args[0])), true
		}
	}
	if r.owner == "java/util/regex/Pattern" && r.name == "compile" && (r.desc == "(Ljava/lang/String;)Ljava/util/regex/Pattern;" || r.desc == "(Ljava/lang/String;I)Ljava/util/regex/Pattern;") {
		s := javaString(args[0])
		if len(s) > 65536 {
			fail("pattern budget exceeded")
		}
		flags := int32(0)
		if len(args) == 2 {
			flags = integer(args[1])
		}
		// Compile validates the pattern with Go's RE2-compatible subset only for
		// later matching; unsupported Java-only syntax is stored and fails on use.
		return h.vm.alloc(r.owner, javaPattern{s, flags}), true
	}
	if r.owner == "java/util/regex/Pattern" && receiver != nil {
		p, ok := objectData(receiver).(javaPattern)
		if !ok {
			return nil, false
		}
		switch {
		case r.name == "pattern" && r.desc == "()Ljava/lang/String;":
			return p.source, true
		case r.name == "flags" && r.desc == "()I":
			return p.flags, true
		case r.name == "matcher" && r.desc == "(Ljava/lang/CharSequence;)Ljava/util/regex/Matcher;":
			return h.vm.alloc("java/util/regex/Matcher", &javaMatcher{pattern: p, input: javaString(args[0])}), true
		case r.name == "matcher" && r.desc == "(Ljava/lang/String;)Ljava/util/regex/Matcher;":
			return h.vm.alloc("java/util/regex/Matcher", &javaMatcher{pattern: p, input: javaString(args[0])}), true
		}
	}
	if r.owner == "java/util/Map$Entry" && receiver != nil {
		e, ok := objectData(receiver).(javaPair)
		if !ok {
			return nil, false
		}
		if r.name == "getKey" && r.desc == "()Ljava/lang/Object;" {
			return e.left, true
		}
		if r.name == "getValue" && r.desc == "()Ljava/lang/Object;" {
			return e.right, true
		}
	}
	if x, ok := h.randomCall(r, receiver, args); ok {
		return x, true
	}
	if r.owner == "java/lang/Record" && r.name == "<init>" && r.desc == "()V" {
		return nil, true
	}
	if r.owner == "java/lang/String" && r.name == "formatted" && r.desc == "([Ljava/lang/Object;)Ljava/lang/String;" {
		return h.formatString(javaString(receiver), args[0].(*array).values), true
	}
	if r.owner == "java/lang/String" && r.name == "format" && r.desc == "(Ljava/lang/String;[Ljava/lang/Object;)Ljava/lang/String;" {
		return h.formatString(javaString(args[0]), args[1].(*array).values), true
	}
	if value, ok := h.durationCall(r, receiver, args); ok {
		return value, true
	}
	if value, ok := h.regexStreamCall(r, receiver, args); ok {
		return value, true
	}
	if value, ok := h.charsetCall(r, receiver, args); ok {
		return value, true
	}
	if value, ok := h.invokeCall(r, receiver, args); ok {
		return value, true
	}
	if value, ok := h.collectionCall(r, receiver, args); ok {
		return value, true
	}
	if value, ok := h.serviceCall(r, receiver, args); ok {
		return value, true
	}
	if value, ok := h.javaValuesCall(r, receiver, args); ok {
		return value, true
	}
	if value, ok := h.metadataCall(r, receiver, args); ok {
		return value, true
	}
	v := h.vm
	if r.owner == "java/util/Base64" && r.name == "getDecoder" && r.desc == "()Ljava/util/Base64$Decoder;" {
		return v.alloc("java/util/Base64$Decoder", nil), true
	}
	if r.owner == "java/util/Base64$Decoder" && r.name == "decode" && r.desc == "(Ljava/lang/String;)[B" {
		s := javaString(args[0])
		if len(s) > 65536 {
			fail("base64 input budget exceeded")
		}
		if strings.ContainsAny(s, "\r\n") {
			raiseJava("java/lang/IllegalArgumentException", "invalid base64 character")
		}
		encoding := base64.StdEncoding
		if !strings.Contains(s, "=") {
			encoding = base64.RawStdEncoding
		}
		b, err := encoding.DecodeString(s)
		if err != nil {
			raiseJava("java/lang/IllegalArgumentException", "invalid base64")
		}
		v.slots += len(b)
		if v.slots > 131072 {
			fail("array slot budget exceeded")
		}
		v.alloc("[", nil)
		a := &array{descriptor: "[B", values: make([]any, len(b))}
		for i, x := range b {
			a.values[i] = int32(int8(x))
		}
		return a, true
	}
	if r.owner == "java/lang/String" && r.name == "<init>" {
		o := receiver.(*object)
		switch r.desc {
		case "()V":
			o.data = ""
			return nil, true
		case "(Ljava/lang/String;)V":
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "String.<init>")
			}
			o.data = javaString(args[0])
			return nil, true
		case "([C)V":
			a := args[0].(*array)
			if a.descriptor != "[C" {
				fail("invalid string char array")
			}
			runes := make([]rune, len(a.values))
			for i, x := range a.values {
				runes[i] = rune(integer(x))
			}
			o.data = string(runes)
			return nil, true
		case "([CII)V":
			a, ok := args[0].(*array)
			if !ok || a.descriptor != "[C" {
				raiseJava("java/lang/NullPointerException", "String.<init>")
			}
			off, n := int(integer(args[1])), int(integer(args[2]))
			if off < 0 || n < 0 || off+n > len(a.values) {
				raiseJava("java/lang/StringIndexOutOfBoundsException", "String.<init>")
			}
			runes := make([]rune, n)
			for i := range runes {
				runes[i] = rune(integer(a.values[off+i]))
			}
			o.data = string(runes)
			return nil, true
		case "(Ljava/lang/StringBuilder;)V", "(Ljava/lang/StringBuffer;)V":
			o.data = javaString(args[0])
			return nil, true
		case "([B)V", "([BLjava/nio/charset/Charset;)V":
			if r.desc == "([BLjava/nio/charset/Charset;)V" && objectData(args[1]) != "UTF-8" {
				fail("unbound string charset")
			}
			a := args[0].(*array)
			if a.descriptor != "[B" {
				fail("invalid string byte array")
			}
			b := make([]byte, len(a.values))
			for i, x := range a.values {
				b[i] = byte(integer(x))
			}
			if !utf8.Valid(b) {
				fail("malformed UTF-8 string conversion is not bound")
			}
			if r.desc == "([B)V" {
				for _, x := range b {
					if x >= 128 {
						fail("default charset is required for non-ASCII input")
					}
				}
			}
			o.data = string(b)
			return nil, true
		}
	}

	if r.name == "toString" && r.desc == "()Ljava/lang/String;" && (r.owner == "java/lang/Object" || r.owner == "java/lang/CharSequence" || r.owner == "java/lang/String") {
		// A Java String is a Go string here; String.toString returns itself.
		if s, ok := receiver.(string); ok {
			return s, true
		}
	}
	if r.owner == "java/lang/Object" && r.name == "hashCode" && r.desc == "()I" {
		switch x := receiver.(type) {
		case *object:
			if x.fields == nil {
				x.fields = map[string]any{}
			}
			const key = "java/lang/Object.identityHash:I"
			if h, ok := x.fields[key].(int32); ok {
				return h, true
			}
			h := int32(len(x.fields) + 1)
			for _, ch := range x.class {
				h = h*31 + int32(ch)
			}
			x.fields[key] = h
			return h, true
		case string:
			h := int32(0)
			for _, ch := range x {
				h = h*31 + int32(ch)
			}
			return h, true
		case classLiteral:
			h := int32(0)
			for _, ch := range string(x) {
				h = h*31 + int32(ch)
			}
			return h, true
		default:
			return int32(0), true
		}
	}
	if (r.owner == "java/util/function/Function" || r.owner == "java/util/function/UnaryOperator") && r.name == "identity" && len(args) == 0 {
		return &lambda{kind: 6, target: reference{"java/util/function/Function", "$identity", "(Ljava/lang/Object;)Ljava/lang/Object;"}, sam: "(Ljava/lang/Object;)Ljava/lang/Object;"}, true
	}
	if r.owner == "java/util/function/Function" && r.name == "$identity" && len(args) == 1 {
		return args[0], true
	}
	// AbstractMap.toString / AbstractCollection.toString for native collections.
	if r.name == "toString" && r.desc == "()Ljava/lang/String;" {
		if o, ok := receiver.(*object); ok && o != nil {
			str := func(x any) string {
				if x == receiver {
					if _, isMap := o.data.(*javaMap); isMap {
						return "(this Map)"
					}
					return "(this Collection)"
				}
				return javaString(v.method(reference{"java/lang/String", "valueOf", "(Ljava/lang/Object;)Ljava/lang/String;"}, nil, false, []any{x}))
			}
			var parts []string
			open, end := "[", "]"
			switch d := o.data.(type) {
			case *javaMap:
				if d.baseline != "" {
					break
				}
				open, end = "{", "}"
				for _, e := range d.entries {
					v.tick()
					parts = append(parts, str(e.left)+"="+str(e.right))
				}
				return open + strings.Join(parts, ", ") + end, true
			case *javaList:
				for _, x := range d.values {
					v.tick()
					parts = append(parts, str(x))
				}
				return open + strings.Join(parts, ", ") + end, true
			case *javaSet:
				for _, x := range d.values {
					v.tick()
					parts = append(parts, str(x))
				}
				return open + strings.Join(parts, ", ") + end, true
			}
		}
	}
	// AbstractMap.SimpleEntry/HashMap.Node toString: key + "=" + value.
	if r.owner == "java/util/Map$Entry" && r.name == "toString" && r.desc == "()Ljava/lang/String;" {
		if o, ok := receiver.(*object); ok && o != nil {
			if e, ok := o.data.(javaPair); ok {
				str := reference{"java/lang/String", "valueOf", "(Ljava/lang/Object;)Ljava/lang/String;"}
				return v.method(str, nil, false, []any{e.left}).(string) + "=" + v.method(str, nil, false, []any{e.right}).(string), true
			}
		}
	}
	if r.owner == "java/util/Objects" && r.name == "toString" && r.desc == "(Ljava/lang/Object;)Ljava/lang/String;" {
		return v.method(reference{"java/lang/String", "valueOf", r.desc}, nil, false, args), true
	}
	if r.owner == "java/util/Objects" && r.name == "toString" && r.desc == "(Ljava/lang/Object;Ljava/lang/String;)Ljava/lang/String;" {
		if args[0] == nil {
			return args[1], true
		}
		return v.method(reference{"java/lang/String", "valueOf", "(Ljava/lang/Object;)Ljava/lang/String;"}, nil, false, args[:1]), true
	}
	if r.owner == "java/util/Objects" && r.name == "equals" && r.desc == "(Ljava/lang/Object;Ljava/lang/Object;)Z" {
		if args[0] == nil || args[1] == nil {
			if args[0] == nil && args[1] == nil {
				return int32(1), true
			}
			return int32(0), true
		}
		return v.method(reference{"java/lang/Object", "equals", "(Ljava/lang/Object;)Z"}, args[0], true, []any{args[1]}), true
	}
	if r.owner == "java/util/Objects" && (r.name == "nonNull" || r.name == "isNull") && r.desc == "(Ljava/lang/Object;)Z" {
		if (args[0] == nil) == (r.name == "isNull") {
			return int32(1), true
		}
		return int32(0), true
	}
	if r.owner == "java/util/Objects" && r.name == "hashCode" && r.desc == "(Ljava/lang/Object;)I" {
		if args[0] == nil {
			return int32(0), true
		}
		return integer(v.method(reference{"java/lang/Object", "hashCode", "()I"}, args[0], true, nil)), true
	}
	if r.owner == "java/util/Objects" && r.name == "hash" && r.desc == "([Ljava/lang/Object;)I" {
		a, ok := args[0].(*array)
		if args[0] == nil {
			return int32(0), true
		}
		if !ok {
			return nil, false
		}
		h := int32(1)
		for _, e := range a.values {
			eh := int32(0)
			if e != nil {
				eh = integer(v.method(reference{"java/lang/Object", "hashCode", "()I"}, e, true, nil))
			}
			h = 31*h + eh
		}
		return h, true
	}
	if (r.owner == "java/lang/Long" || r.owner == "java/lang/Integer") && (r.name == "numberOfLeadingZeros" || r.name == "numberOfTrailingZeros" || r.name == "bitCount") && (r.desc == "(J)I" || r.desc == "(I)I") {
		var x uint64
		width := 64
		if r.desc == "(J)I" {
			x = uint64(args[0].(int64))
		} else {
			x = uint64(uint32(integer(args[0])))
			width = 32
		}
		switch r.name {
		case "numberOfLeadingZeros":
			return int32(bits.LeadingZeros64(x) - (64 - width)), true
		case "numberOfTrailingZeros":
			if x == 0 {
				return int32(width), true
			}
			return int32(bits.TrailingZeros64(x)), true
		default:
			return int32(bits.OnesCount64(x)), true
		}
	}
	if (r.owner == "java/lang/Float" && r.desc == "(FF)I" || r.owner == "java/lang/Double" && r.desc == "(DD)I") && r.name == "compare" {
		a, b := javaFloating(args[0]), javaFloating(args[1])
		switch {
		case a < b:
			return int32(-1), true
		case a > b:
			return int32(1), true
		case a == b && !(a == 0 && math.Signbit(a) != math.Signbit(b)):
			return int32(0), true
		case math.IsNaN(a) && math.IsNaN(b):
			return int32(0), true
		case math.IsNaN(a) || (a == 0 && !math.Signbit(a)):
			return int32(1), true
		default:
			return int32(-1), true
		}
	}
	if r.owner == "java/lang/Integer" && r.name == "compare" && r.desc == "(II)I" {
		a, b := integer(args[0]), integer(args[1])
		switch {
		case a < b:
			return int32(-1), true
		case a > b:
			return int32(1), true
		default:
			return int32(0), true
		}
	}
	// No SecurityManager: doPrivileged just runs the action. Checked exceptions
	// from PrivilegedExceptionAction would be wrapped in
	// PrivilegedActionException; Java exceptions propagate through run().
	if r.owner == "java/security/AccessController" && r.name == "doPrivileged" && receiver == nil && len(args) >= 1 {
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "doPrivileged")
		}
		switch {
		case strings.HasPrefix(r.desc, "(Ljava/security/PrivilegedAction;"):
			return v.method(reference{"java/security/PrivilegedAction", "run", "()Ljava/lang/Object;"}, args[0], true, nil), true
		case strings.HasPrefix(r.desc, "(Ljava/security/PrivilegedExceptionAction;"):
			return v.method(reference{"java/security/PrivilegedExceptionAction", "run", "()Ljava/lang/Object;"}, args[0], true, nil), true
		}
	}
	if r.owner == "java/lang/String" && r.name == "compareTo" && (r.desc == "(Ljava/lang/String;)I" || r.desc == "(Ljava/lang/Object;)I") {
		a, ok := stringValue(receiver)
		if ok {
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "compareTo")
			}
			b, ok := stringValue(args[0])
			if !ok {
				raiseJava("java/lang/ClassCastException", "String.compareTo")
			}
			// Lexicographic over UTF-16 units: first differing unit, else length.
			x, y := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
			for i := 0; i < len(x) && i < len(y); i++ {
				if x[i] != y[i] {
					return int32(x[i]) - int32(y[i]), true
				}
			}
			return int32(len(x) - len(y)), true
		}
	}
	if r.owner == "java/lang/Long" && r.name == "compareTo" && (r.desc == "(Ljava/lang/Long;)I" || r.desc == "(Ljava/lang/Object;)I") {
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "compareTo")
		}
		a, ok1 := objectData(receiver).(int64)
		b, ok2 := objectData(args[0]).(int64)
		if !ok1 || !ok2 {
			raiseJava("java/lang/ClassCastException", "Long.compareTo")
		}
		switch {
		case a < b:
			return int32(-1), true
		case a > b:
			return int32(1), true
		default:
			return int32(0), true
		}
	}
	if r.owner == "java/lang/Integer" && r.name == "compareTo" && (r.desc == "(Ljava/lang/Integer;)I" || r.desc == "(Ljava/lang/Object;)I") {
		a := integer(objectData(receiver))
		b := integer(objectData(args[0]))
		switch {
		case a < b:
			return int32(-1), true
		case a > b:
			return int32(1), true
		default:
			return int32(0), true
		}
	}
	if r.owner == "java/lang/Integer" && r.name == "rotateLeft" && r.desc == "(II)I" {
		x := integer(args[0])
		n := integer(args[1]) & 31
		return int32(uint32(x)<<uint32(n) | uint32(x)>>(32-uint32(n))), true
	}
	if r.owner == "java/lang/Integer" && r.name == "rotateRight" && r.desc == "(II)I" {
		x := integer(args[0])
		n := integer(args[1]) & 31
		return int32(uint32(x)>>uint32(n) | uint32(x)<<(32-uint32(n))), true
	}
	if r.owner == "java/lang/Integer" && r.name == "highestOneBit" && r.desc == "(I)I" {
		n := uint32(integer(args[0]))
		if n == 0 {
			return int32(0), true
		}
		n |= n >> 1
		n |= n >> 2
		n |= n >> 4
		n |= n >> 8
		n |= n >> 16
		return int32(n - (n >> 1)), true
	}
	if r.owner == "java/util/BitSet" && r.name == "<init>" && (r.desc == "()V" || r.desc == "(I)V") {
		n := 64
		if r.desc == "(I)V" {
			bits := int(integer(args[0]))
			if bits < 0 {
				raiseJava("java/lang/NegativeArraySizeException", "BitSet")
			}
			if bits > 1<<20 {
				fail("registration BitSet too large")
			}
			n = (bits + 63) / 64
			if n == 0 {
				n = 1
			}
		}
		receiver.(*object).data = make([]uint64, n)
		return nil, true
	}
	if r.owner == "java/util/BitSet" && receiver != nil {
		words, ok := objectData(receiver).([]uint64)
		if !ok {
			return nil, false
		}
		switch {
		case r.name == "set" && r.desc == "(I)V":
			i := int(integer(args[0]))
			if i < 0 {
				raiseJava("java/lang/IndexOutOfBoundsException", "BitSet")
			}
			for i/64 >= len(words) {
				words = append(words, 0)
			}
			words[i/64] |= 1 << uint(i%64)
			receiver.(*object).data = words
			return nil, true
		case r.name == "get" && r.desc == "(I)Z":
			i := int(integer(args[0]))
			if i < 0 {
				raiseJava("java/lang/IndexOutOfBoundsException", "BitSet")
			}
			if i/64 >= len(words) {
				return int32(0), true
			}
			if words[i/64]&(1<<uint(i%64)) != 0 {
				return int32(1), true
			}
			return int32(0), true
		case r.name == "clear" && r.desc == "()V":
			for i := range words {
				words[i] = 0
			}
			return nil, true
		case r.name == "size" && r.desc == "()I":
			return int32(len(words) * 64), true
		}
	}
	if r.owner == "java/lang/StringBuilder" || r.owner == "java/lang/StringBuffer" {
		if r.name == "<init>" {
			switch r.desc {
			case "()V":
				receiver.(*object).data = []byte{}
			case "(I)V":
				receiver.(*object).data = []byte{}
			case "(Ljava/lang/String;)V":
				receiver.(*object).data = []byte(javaString(args[0]))
			case "(Ljava/lang/CharSequence;)V":
				receiver.(*object).data = []byte(javaString(args[0]))
			default:
				return nil, false
			}
			return nil, true
		}
		if receiver == nil {
			return nil, false
		}
		o := receiver.(*object)
		buf, _ := o.data.([]byte)
		switch {
		case r.name == "append":
			var piece string
			switch r.desc {
			case "(Ljava/lang/String;)Ljava/lang/StringBuilder;", "(Ljava/lang/String;)Ljava/lang/StringBuffer;",
				"(Ljava/lang/CharSequence;)Ljava/lang/StringBuilder;", "(Ljava/lang/Object;)Ljava/lang/StringBuilder;",
				"(Ljava/lang/CharSequence;)Ljava/lang/StringBuffer;", "(Ljava/lang/Object;)Ljava/lang/StringBuffer;":
				if args[0] == nil {
					piece = "null"
				} else if s, ok := args[0].(string); ok {
					piece = s
				} else if strings.HasPrefix(r.desc, "(Ljava/lang/Object;)") {
					// AbstractStringBuilder.append(Object) is String.valueOf(obj).
					piece = javaString(v.method(reference{"java/lang/String", "valueOf", "(Ljava/lang/Object;)Ljava/lang/String;"}, nil, false, args))
				} else {
					piece = javaString(args[0])
				}
			case "(Ljava/lang/CharSequence;II)Ljava/lang/StringBuilder;", "(Ljava/lang/CharSequence;II)Ljava/lang/StringBuffer;":
				src := []rune("null")
				if args[0] != nil {
					src = []rune(javaString(args[0]))
				}
				start, end := int(integer(args[1])), int(integer(args[2]))
				if start < 0 || end < start || end > len(src) {
					return nil, false
				}
				piece = string(src[start:end])
			case "(C)Ljava/lang/StringBuilder;", "(C)Ljava/lang/StringBuffer;":
				piece = string(rune(integer(args[0])))
			case "(I)Ljava/lang/StringBuilder;", "(I)Ljava/lang/StringBuffer;":
				piece = strconv.FormatInt(int64(integer(args[0])), 10)
			case "(J)Ljava/lang/StringBuilder;", "(J)Ljava/lang/StringBuffer;":
				piece = strconv.FormatInt(args[0].(int64), 10)
			case "(Z)Ljava/lang/StringBuilder;", "(Z)Ljava/lang/StringBuffer;":
				if integer(args[0]) != 0 {
					piece = "true"
				} else {
					piece = "false"
				}
			default:
				return nil, false
			}
			o.data = append(buf, piece...)
			return receiver, true
		case r.name == "toString" && r.desc == "()Ljava/lang/String;":
			return string(buf), true
		case r.name == "length" && r.desc == "()I":
			return int32(len(buf)), true
		case r.name == "setLength" && r.desc == "(I)V":
			n := int(integer(args[0]))
			if n < 0 {
				raiseJava("java/lang/StringIndexOutOfBoundsException", "setLength")
			}
			if n < len(buf) {
				o.data = buf[:n]
			}
			return nil, true
		}
	}
	if (r.owner == "java/lang/ThreadLocal" || r.owner == "java/lang/InheritableThreadLocal") && r.name == "withInitial" && r.desc == "(Ljava/util/function/Supplier;)Ljava/lang/ThreadLocal;" && receiver == nil {
		return v.alloc(r.owner, map[string]any{"supplier": args[0]}), true
	}
	if (r.owner == "java/lang/ThreadLocal" || r.owner == "java/lang/InheritableThreadLocal") && r.name == "<init>" && r.desc == "()V" {
		receiver.(*object).data = map[string]any{}
		return nil, true
	}
	if (r.owner == "java/lang/ThreadLocal" || r.owner == "java/lang/InheritableThreadLocal") && receiver != nil {
		m, _ := objectData(receiver).(map[string]any)
		if m == nil {
			m = map[string]any{}
			receiver.(*object).data = m
		}
		if r.name == "get" && r.desc == "()Ljava/lang/Object;" {
			if x, ok := m["value"]; ok {
				return x, true
			}
			if sup := m["supplier"]; sup != nil {
				x := v.method(reference{"java/util/function/Supplier", "get", "()Ljava/lang/Object;"}, sup, true, nil)
				m["value"] = x
				return x, true
			}
			return nil, true
		}
		if r.name == "set" && r.desc == "(Ljava/lang/Object;)V" {
			m["value"] = args[0]
			return nil, true
		}
		if r.name == "remove" && r.desc == "()V" {
			delete(m, "value")
			return nil, true
		}
	}
	if r.owner == "java/util/EnumSet" && r.name == "allOf" && r.desc == "(Ljava/lang/Class;)Ljava/util/EnumSet;" {
		return v.alloc(r.owner, &javaSet{}), true
	}
	if r.owner == "java/util/EnumSet" && r.name == "noneOf" && r.desc == "(Ljava/lang/Class;)Ljava/util/EnumSet;" {
		return v.alloc(r.owner, &javaSet{}), true
	}
	if r.owner == "java/util/concurrent/LinkedBlockingQueue" && r.name == "<init>" && (r.desc == "()V" || r.desc == "(I)V") {
		receiver.(*object).data = &javaList{}
		return nil, true
	}
	if r.owner == "java/time/LocalDateTime" && receiver != nil && r.name == "getMonth" && r.desc == "()Ljava/time/Month;" {
		return h.symbol("java/time/Month.JANUARY", "java/time/Month", "JANUARY"), true
	}
	if r.owner == "java/time/LocalDateTime" && receiver != nil && r.name == "getDayOfMonth" && r.desc == "()I" {
		return int32(1), true
	}
	if r.owner == "java/time/LocalDateTime" && receiver != nil && r.name == "getYear" && r.desc == "()I" {
		return int32(1970), true
	}
	if r.owner == "java/time/Month" && r.name == "getValue" && r.desc == "()I" {
		return int32(1), true
	}
	if r.owner == "java/time/LocalDateTime" && r.name == "now" && (r.desc == "()Ljava/time/LocalDateTime;" || r.desc == "(Ljava/time/Clock;)Ljava/time/LocalDateTime;" || r.desc == "(Ljava/time/ZoneId;)Ljava/time/LocalDateTime;") {
		return v.alloc(r.owner, int64(0)), true
	}
	if r.owner == "java/time/LocalDate" && r.name == "now" && (r.desc == "()Ljava/time/LocalDate;" || r.desc == "(Ljava/time/Clock;)Ljava/time/LocalDate;" || r.desc == "(Ljava/time/ZoneId;)Ljava/time/LocalDate;") {
		return v.alloc(r.owner, int64(0)), true
	}
	if r.owner == "java/util/EnumMap" && r.name == "<init>" && r.desc == "(Ljava/lang/Class;)V" {
		receiver.(*object).data = &javaMap{}
		return nil, true
	}
	if r.owner == "java/util/EnumMap" && r.name == "<init>" && r.desc == "(Ljava/util/EnumMap;)V" {
		src, _ := objectData(args[0]).(*javaMap)
		m := &javaMap{}
		if src != nil {
			m.entries = append([]javaPair(nil), src.entries...)
		}
		receiver.(*object).data = m
		return nil, true
	}
	if x, ok := h.atomicCall(r, receiver, args); ok {
		return x, true
	}
	if r.owner == "java/time/Duration" && r.name == "ofNanos" && r.desc == "(J)Ljava/time/Duration;" {
		return v.alloc(r.owner, args[0].(int64)), true
	}
	if r.owner == "java/time/Duration" && r.name == "toNanos" && r.desc == "()J" {
		return objectData(receiver).(int64), true
	}
	if r.owner == "java/time/Duration" && r.name == "toMillis" && r.desc == "()J" {
		return objectData(receiver).(int64) / int64(1_000_000), true
	}
	if r.owner == "java/util/concurrent/atomic/AtomicReferenceArray" && r.name == "<init>" && r.desc == "(I)V" {
		n := int(integer(args[0]))
		if n < 0 || n > 65536 {
			fail("atomic array budget")
		}
		receiver.(*object).data = &array{descriptor: "[Ljava/lang/Object;", values: make([]any, n)}
		return nil, true
	}
	if r.owner == "java/util/concurrent/atomic/AtomicReferenceArray" {
		a := objectData(receiver).(*array)
		if r.name == "get" && r.desc == "(I)Ljava/lang/Object;" {
			i := int(integer(args[0]))
			if i < 0 || i >= len(a.values) {
				raiseJava("java/lang/ArrayIndexOutOfBoundsException", "atomic get")
			}
			return a.values[i], true
		}
		if r.name == "set" && r.desc == "(ILjava/lang/Object;)V" {
			i := int(integer(args[0]))
			if i < 0 || i >= len(a.values) {
				raiseJava("java/lang/ArrayIndexOutOfBoundsException", "atomic set")
			}
			a.values[i] = args[1]
			return nil, true
		}
		if r.name == "length" && r.desc == "()I" {
			return int32(len(a.values)), true
		}
	}
	if r.owner == "java/util/Arrays" && r.name == "hashCode" && r.desc == "([Ljava/lang/Object;)I" {
		if args[0] == nil {
			return int32(0), true
		}
		a := args[0].(*array)
		h := int32(1)
		for _, x := range a.values {
			h *= 31
			if x == nil {
				continue
			}
			h += integer(v.method(reference{"java/lang/Object", "hashCode", "()I"}, x, true, nil))
		}
		return h, true
	}
	if r.owner == "java/util/Arrays" && r.name == "equals" && r.desc == "([Ljava/lang/Object;[Ljava/lang/Object;)Z" {
		if args[0] == nil || args[1] == nil {
			if args[0] == args[1] {
				return int32(1), true
			}
			return int32(0), true
		}
		a, b := args[0].(*array), args[1].(*array)
		if len(a.values) != len(b.values) {
			return int32(0), true
		}
		for i := range a.values {
			if integer(v.method(reference{"java/util/Objects", "equals", "(Ljava/lang/Object;Ljava/lang/Object;)Z"}, nil, false, []any{a.values[i], b.values[i]})) == 0 {
				return int32(0), true
			}
		}
		return int32(1), true
	}
	if r.owner == "java/util/Arrays" && r.name == "stream" && r.desc == "([Ljava/lang/Object;)Ljava/util/stream/Stream;" {
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "null array stream")
		}
		a := args[0].(*array)
		return v.alloc("java/util/stream/Stream", &javaList{values: append([]any(nil), a.values...), readonly: true}), true
	}
	if r.owner == "java/util/OptionalInt" || r.owner == "java/util/OptionalLong" || r.owner == "java/util/OptionalDouble" {
		if receiver == nil {
			if r.name == "empty" {
				return v.alloc(r.owner, nil), true
			}
			if r.name == "of" && len(args) == 1 {
				return v.alloc(r.owner, args[0]), true
			}
			return nil, false
		}
		switch {
		case r.name == "isPresent" && r.desc == "()Z":
			if objectData(receiver) != nil {
				return int32(1), true
			}
			return int32(0), true
		case r.name == "isEmpty" && r.desc == "()Z":
			if objectData(receiver) == nil {
				return int32(1), true
			}
			return int32(0), true
		case r.name == "getAsInt" && r.desc == "()I":
			x := objectData(receiver)
			if x == nil {
				raiseJava("java/util/NoSuchElementException", "OptionalInt")
			}
			return integer(x), true
		case r.name == "getAsLong" && r.desc == "()J":
			x := objectData(receiver)
			if x == nil {
				raiseJava("java/util/NoSuchElementException", "OptionalLong")
			}
			if n, ok := x.(int64); ok {
				return n, true
			}
			return int64(integer(x)), true
		case r.name == "orElse":
			if x := objectData(receiver); x != nil {
				return x, true
			}
			return args[0], true
		}
	}
	if r.owner == "java/util/stream/Collector" && r.name == "of" && (r.desc == "(Ljava/util/function/Supplier;Ljava/util/function/BiConsumer;Ljava/util/function/BinaryOperator;)Ljava/util/stream/Collector;" || r.desc == "(Ljava/util/function/Supplier;Ljava/util/function/BiConsumer;Ljava/util/function/BinaryOperator;Ljava/util/function/Function;[Ljava/util/stream/Collector$Characteristics;)Ljava/util/stream/Collector;" || r.desc == "(Ljava/util/function/Supplier;Ljava/util/function/BiConsumer;Ljava/util/function/BinaryOperator;[Ljava/util/stream/Collector$Characteristics;)Ljava/util/stream/Collector;") {
		return v.alloc(r.owner, javaPair{args[0], args[1]}), true
	}
	if r.owner == "java/util/stream/Collectors" && r.name == "toMap" && r.desc == "(Ljava/util/function/Function;Ljava/util/function/Function;)Ljava/util/stream/Collector;" {
		if args[0] == nil || args[1] == nil {
			raiseJava("java/lang/NullPointerException", "toMap")
		}
		return v.alloc("java/util/stream/Collector", javaToMap{key: args[0], value: args[1]}), true
	}
	if r.owner == "java/util/stream/Collectors" && r.name == "toMap" && r.desc == "(Ljava/util/function/Function;Ljava/util/function/Function;Ljava/util/function/BinaryOperator;Ljava/util/function/Supplier;)Ljava/util/stream/Collector;" {
		if args[0] == nil || args[1] == nil || args[2] == nil || args[3] == nil {
			raiseJava("java/lang/NullPointerException", "toMap")
		}
		return v.alloc("java/util/stream/Collector", javaToMap{args[0], args[1], args[2], args[3]}), true
	}
	if r.owner == "java/util/stream/Collectors" && r.name == "toCollection" && r.desc == "(Ljava/util/function/Supplier;)Ljava/util/stream/Collector;" {
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "toCollection")
		}
		return v.alloc("java/util/stream/Collector", javaToCollection{args[0]}), true
	}
	if r.owner == "java/util/stream/Collectors" && r.name == "toList" && r.desc == "()Ljava/util/stream/Collector;" {
		return v.alloc("java/util/stream/Collector", "toList"), true
	}
	// Collection.stream() is a default method; native sets inherit it.
	if o, ok := receiver.(*object); ok && o != nil && r.name == "stream" && r.desc == "()Ljava/util/stream/Stream;" {
		if s, ok := o.data.(*javaSet); ok {
			return v.alloc("java/util/stream/Stream", &javaList{values: append([]any(nil), s.values...), readonly: true}), true
		}
	}
	if receiver != nil && r.name == "stream" && r.desc == "()Ljava/util/stream/Stream;" && (r.owner == "java/util/Collection" || r.owner == "java/util/List" || r.owner == "java/util/ArrayList") {
		if l, ok := objectData(receiver).(*javaList); ok {
			return v.alloc("java/util/stream/Stream", &javaList{values: append([]any(nil), l.values...), readonly: true}), true
		}
	}
	if r.owner == "java/util/stream/Stream" && r.name == "builder" && r.desc == "()Ljava/util/stream/Stream$Builder;" {
		return v.alloc("java/util/stream/Stream$Builder", &javaList{}), true
	}
	if r.owner == "java/util/stream/Stream$Builder" || r.owner == "java/util/function/Consumer" && receiver != nil && func() bool { o, ok := receiver.(*object); return ok && o.class == "java/util/stream/Stream$Builder" }() {
		b, ok := objectData(receiver).(*javaList)
		if !ok {
			fail("unbound stream builder")
		}
		switch r.name + r.desc {
		case "add(Ljava/lang/Object;)Ljava/util/stream/Stream$Builder;", "accept(Ljava/lang/Object;)V":
			if b.readonly {
				raiseJava("java/lang/IllegalStateException", "stream builder already built")
			}
			v.reserveCollection(1)
			b.values = append(b.values, args[0])
			if r.name == "add" {
				return receiver, true
			}
			return nil, true
		case "build()Ljava/util/stream/Stream;":
			if b.readonly {
				raiseJava("java/lang/IllegalStateException", "stream builder already built")
			}
			b.readonly = true
			return v.alloc("java/util/stream/Stream", &javaList{values: append([]any(nil), b.values...), readonly: true}), true
		}
	}
	if r.owner == "java/util/stream/Stream" && r.name == "reduce" && (r.desc == "(Ljava/lang/Object;Ljava/util/function/BiFunction;Ljava/util/function/BinaryOperator;)Ljava/lang/Object;" || r.desc == "(Ljava/lang/Object;Ljava/util/function/BinaryOperator;)Ljava/lang/Object;") {
		src, ok := objectData(receiver).(*javaList)
		if !ok {
			fail("unbound stream source")
		}
		if args[1] == nil {
			raiseJava("java/lang/NullPointerException", "null stream accumulator")
		}
		// Sequential reduction never consults the combiner.
		acc := args[0]
		for _, item := range src.values {
			v.tick()
			acc = v.method(reference{"java/util/function/BiFunction", "apply", "(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;"}, args[1], true, []any{acc, item})
		}
		return acc, true
	}
	if r.owner == "java/util/stream/Stream" && receiver == nil && r.name == "of" {
		switch r.desc {
		case "([Ljava/lang/Object;)Ljava/util/stream/Stream;":
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "Stream.of")
			}
			return v.alloc(r.owner, &javaList{values: append([]any(nil), args[0].(*array).values...), readonly: true}), true
		case "(Ljava/lang/Object;)Ljava/util/stream/Stream;":
			return v.alloc(r.owner, &javaList{values: []any{args[0]}, readonly: true}), true
		}
	}
	if r.owner == "java/util/stream/Stream" {
		src, ok := objectData(receiver).(*javaList)
		if !ok {
			fail("unbound stream source")
		}
		if (r.name == "anyMatch" || r.name == "allMatch" || r.name == "noneMatch") && r.desc == "(Ljava/util/function/Predicate;)Z" {
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "null stream predicate")
			}
			// Short-circuits in encounter order, as the sequential stream does.
			want := r.name == "anyMatch"
			for _, item := range src.values {
				v.tick()
				hit := integer(v.method(reference{"java/util/function/Predicate", "test", "(Ljava/lang/Object;)Z"}, args[0], true, []any{item})) != 0
				if r.name == "anyMatch" && hit || r.name == "allMatch" && !hit || r.name == "noneMatch" && hit {
					if r.name == "anyMatch" {
						return int32(1), true
					}
					return int32(0), true
				}
			}
			if want {
				return int32(0), true
			}
			return int32(1), true
		}
		if r.name == "filter" && r.desc == "(Ljava/util/function/Predicate;)Ljava/util/stream/Stream;" {
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "null stream predicate")
			}
			out := &javaList{readonly: true}
			for _, item := range src.values {
				v.tick()
				if integer(v.method(reference{"java/util/function/Predicate", "test", "(Ljava/lang/Object;)Z"}, args[0], true, []any{item})) != 0 {
					v.reserveCollection(1)
					out.values = append(out.values, item)
				}
			}
			return v.alloc("java/util/stream/Stream", out), true
		}
		if r.name == "map" && r.desc == "(Ljava/util/function/Function;)Ljava/util/stream/Stream;" {
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "null stream mapper")
			}
			out := &javaList{readonly: true}
			for _, item := range src.values {
				v.tick()
				mapped := v.method(reference{"java/util/function/Function", "apply", "(Ljava/lang/Object;)Ljava/lang/Object;"}, args[0], true, []any{item})
				v.reserveCollection(1)
				out.values = append(out.values, mapped)
			}
			return v.alloc("java/util/stream/Stream", out), true
		}
		if r.name == "flatMap" && r.desc == "(Ljava/util/function/Function;)Ljava/util/stream/Stream;" {
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "null stream mapper")
			}
			out := &javaList{readonly: true}
			for _, item := range src.values {
				v.tick()
				mapped := v.method(reference{"java/util/function/Function", "apply", "(Ljava/lang/Object;)Ljava/lang/Object;"}, args[0], true, []any{item})
				if mapped == nil {
					continue
				}
				inner, ok := objectData(mapped).(*javaList)
				if !ok {
					fail("flatMap result is not a bound stream")
				}
				v.reserveCollection(len(inner.values))
				out.values = append(out.values, inner.values...)
			}
			return v.alloc("java/util/stream/Stream", out), true
		}
		if r.name == "sorted" && r.desc == "(Ljava/util/Comparator;)Ljava/util/stream/Stream;" && args[0] != nil {
			out := &javaList{values: append([]any(nil), src.values...), readonly: true}
			v.sortJavaList(out, args[0])
			return v.alloc("java/util/stream/Stream", out), true
		}
		if r.name == "collect" && r.desc == "(Ljava/util/stream/Collector;)Ljava/lang/Object;" {
			data := objectData(args[0])
			if kind, ok := data.(string); ok && kind == "toList" {
				return v.alloc("java/util/ArrayList", &javaList{values: append([]any(nil), src.values...)}), true
			}
			if tc, ok := data.(javaToCollection); ok {
				acc := v.method(reference{"java/util/function/Supplier", "get", "()Ljava/lang/Object;"}, tc.supplier, true, nil)
				for _, item := range src.values {
					v.tick()
					v.method(reference{"java/util/Collection", "add", "(Ljava/lang/Object;)Z"}, acc, true, []any{item})
				}
				return acc, true
			}
			if tm, ok := data.(javaToMap); ok {
				var m any = v.alloc("java/util/HashMap", &javaMap{})
				if tm.supplier != nil {
					m = v.method(reference{"java/util/function/Supplier", "get", "()Ljava/lang/Object;"}, tm.supplier, true, nil)
				}
				apply := reference{"java/util/function/Function", "apply", "(Ljava/lang/Object;)Ljava/lang/Object;"}
				for _, item := range src.values {
					v.tick()
					k := v.method(apply, tm.key, true, []any{item})
					val := v.method(apply, tm.value, true, []any{item})
					if val == nil {
						raiseJava("java/lang/NullPointerException", "toMap value")
					}
					if tm.merge != nil {
						// Collectors.uniqKeysMapAccumulator vs mapMerger: Map.merge.
						if old := v.method(reference{"java/util/Map", "get", "(Ljava/lang/Object;)Ljava/lang/Object;"}, m, true, []any{k}); old != nil {
							val = v.method(reference{"java/util/function/BinaryOperator", "apply", "(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;"}, tm.merge, true, []any{old, val})
						}
					} else if integer(v.method(reference{"java/util/Map", "containsKey", "(Ljava/lang/Object;)Z"}, m, true, []any{k})) != 0 {
						raiseJava("java/lang/IllegalStateException", "Duplicate key")
					}
					v.method(reference{"java/util/Map", "put", "(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;"}, m, true, []any{k, val})
				}
				return m, true
			}
			if pair, ok := data.(javaPair); ok {
				acc := v.method(reference{"java/util/function/Supplier", "get", "()Ljava/lang/Object;"}, pair.left, true, nil)
				for _, item := range src.values {
					v.tick()
					v.method(reference{"java/util/function/BiConsumer", "accept", "(Ljava/lang/Object;Ljava/lang/Object;)V"}, pair.right, true, []any{acc, item})
				}
				return acc, true
			}
			fail("unbound stream collector %T", data)
		}
		if r.name == "count" && r.desc == "()J" {
			return int64(len(src.values)), true
		}
		if r.name == "peek" && r.desc == "(Ljava/util/function/Consumer;)Ljava/util/stream/Stream;" {
			// Streams here are evaluated eagerly, so the action observes every
			// element in encounter order before downstream stages run.
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "null stream action")
			}
			for _, item := range src.values {
				v.tick()
				v.method(reference{"java/util/function/Consumer", "accept", "(Ljava/lang/Object;)V"}, args[0], true, []any{item})
			}
			return v.alloc("java/util/stream/Stream", &javaList{values: append([]any(nil), src.values...), readonly: true}), true
		}
		if r.name == "toList" && r.desc == "()Ljava/util/List;" {
			return v.alloc("java/util/List", &javaList{values: append([]any(nil), src.values...), readonly: true}), true
		}
		if r.name == "toArray" && r.desc == "(Ljava/util/function/IntFunction;)[Ljava/lang/Object;" {
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "null array generator")
			}
			a, ok := v.method(reference{"java/util/function/IntFunction", "apply", "(I)Ljava/lang/Object;"}, args[0], true, []any{int32(len(src.values))}).(*array)
			if !ok || len(a.values) != len(src.values) {
				raiseJava("java/lang/IllegalStateException", "Begin size does not match array size")
			}
			copy(a.values, src.values)
			return a, true
		}
		if r.name == "toArray" && r.desc == "()[Ljava/lang/Object;" {
			return &array{values: append([]any(nil), src.values...), descriptor: "[Ljava/lang/Object;"}, true
		}
		if r.name == "forEach" && r.desc == "(Ljava/util/function/Consumer;)V" {
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "null stream consumer")
			}
			for _, item := range src.values {
				v.tick()
				v.method(reference{"java/util/function/Consumer", "accept", "(Ljava/lang/Object;)V"}, args[0], true, []any{item})
			}
			return nil, true
		}
	}
	if r.owner == "java/lang/Long" && r.name == "toString" && r.desc == "()Ljava/lang/String;" && receiver != nil {
		if n, ok := objectData(receiver).(int64); ok {
			return strconv.FormatInt(n, 10), true
		}
	}
	if r.owner == "java/lang/Boolean" && r.name == "toString" && r.desc == "()Ljava/lang/String;" && receiver != nil {
		if integer(objectData(receiver)) != 0 {
			return "true", true
		}
		return "false", true
	}
	if r.owner == "java/lang/Integer" && r.name == "toString" && r.desc == "(I)Ljava/lang/String;" {
		return strconv.FormatInt(int64(integer(args[0])), 10), true
	}
	if r.owner == "java/lang/Integer" && r.name == "toString" && r.desc == "()Ljava/lang/String;" {
		return strconv.FormatInt(int64(integer(objectData(receiver))), 10), true
	}
	if r.owner == "java/lang/Double" && receiver == nil && (r.name == "parseDouble" && r.desc == "(Ljava/lang/String;)D" || r.name == "valueOf" && r.desc == "(Ljava/lang/String;)Ljava/lang/Double;") {
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "parseDouble")
		}
		d := javaParseDouble(javaString(args[0]))
		if r.name == "valueOf" {
			return v.alloc("java/lang/Double", d), true
		}
		return d, true
	}
	if r.owner == "java/lang/String" && receiver == nil && (r.name == "valueOf" || r.name == "copyValueOf") && (r.desc == "([C)Ljava/lang/String;" || r.desc == "([CII)Ljava/lang/String;") {
		a, _ := args[0].(*array)
		if a == nil {
			raiseJava("java/lang/NullPointerException", "char[]")
		}
		off, n := 0, len(a.values)
		if len(args) == 3 {
			off, n = int(integer(args[1])), int(integer(args[2]))
			if off < 0 || n < 0 || off+n > len(a.values) {
				raiseJava("java/lang/StringIndexOutOfBoundsException", "valueOf")
			}
		}
		runes := make([]rune, n)
		for i := range runes {
			runes[i] = rune(integer(a.values[off+i]))
		}
		return string(runes), true
	}
	if r.owner == "java/lang/String" && receiver == nil && r.name == "valueOf" && len(args) == 1 && inertOf(args[0]) == nil {
		switch r.desc {
		case "(I)Ljava/lang/String;":
			return strconv.FormatInt(int64(integer(args[0])), 10), true
		case "(J)Ljava/lang/String;":
			return strconv.FormatInt(args[0].(int64), 10), true
		case "(Z)Ljava/lang/String;":
			return strconv.FormatBool(integer(args[0]) != 0), true
		case "(C)Ljava/lang/String;":
			return string(rune(uint16(integer(args[0])))), true
		}
	}
	if r.owner == "java/lang/String" && receiver == nil && r.name == "valueOf" && r.desc == "(Ljava/lang/Object;)Ljava/lang/String;" {
		switch x := args[0].(type) {
		case nil:
			return "null", true
		case string:
			return x, true
		}
		s := v.method(reference{"java/lang/Object", "toString", "()Ljava/lang/String;"}, args[0], true, nil)
		if s == nil {
			return "null", true
		}
		return s, true
	}
	if r.owner == "java/lang/String" && receiver != nil && r.name == "toCharArray" && r.desc == "()[C" {
		// Code-point indexed, matching this VM's String.length/charAt.
		runes := []rune(javaString(receiver))
		v.reserveCollection(len(runes))
		values := make([]any, len(runes))
		for i, c := range runes {
			values[i] = int32(c)
		}
		return &array{values: values, descriptor: "[C"}, true
	}
	if r.owner == "java/lang/String" && receiver != nil && r.name == "indent" && r.desc == "(I)Ljava/lang/String;" {
		return javaIndent(javaString(receiver), int(integer(args[0]))), true
	}
	if r.owner == "java/lang/String" && receiver != nil && (r.name == "trim" || r.name == "strip" || r.name == "stripLeading" || r.name == "stripTrailing") && r.desc == "()Ljava/lang/String;" {
		text := javaString(receiver)
		switch r.name {
		case "trim":
			return strings.TrimFunc(text, func(c rune) bool { return c <= ' ' }), true
		case "strip":
			return strings.TrimSpace(text), true
		case "stripLeading":
			return strings.TrimLeftFunc(text, unicode.IsSpace), true
		default:
			return strings.TrimRightFunc(text, unicode.IsSpace), true
		}
	}
	if r.owner == "java/lang/String" && receiver != nil && r.name == "toUpperCase" && (r.desc == "(Ljava/util/Locale;)Ljava/lang/String;" || r.desc == "()Ljava/lang/String;") {
		text := javaString(receiver)
		ascii := true
		for _, ch := range text {
			if ch > 127 {
				ascii = false
			}
		}
		if len(args) == 1 {
			locale, _ := objectData(args[0]).(string)
			if locale != "ROOT" && locale != "ENGLISH" && !ascii {
				fail("unbound case-mapping locale")
			}
		} else if !ascii || h.options.Language == "tr" || h.options.Language == "az" {
			if h.options.Language == "" || h.options.Language == "tr" || h.options.Language == "az" {
				fail("default locale case mapping")
			}
		}
		return strings.ToUpper(text), true
	}
	if r.owner == "java/lang/Class" && r.name == "getClassLoader" && r.desc == "()Ljava/lang/ClassLoader;" {
		return v.alloc("java/lang/ClassLoader", "registration"), true
	}
	if r.owner == "java/lang/Enum" && receiver != nil {
		switch {
		case r.name == "equals" && r.desc == "(Ljava/lang/Object;)Z":
			if receiver == args[0] {
				return int32(1), true
			}
			return int32(0), true
		case r.name == "compareTo" && len(args) == 1:
			a, _ := receiver.(*object).fields["java/lang/Enum.ordinal"].(int32)
			b := int32(0)
			if o, ok := args[0].(*object); ok && o != nil {
				b, _ = o.fields["java/lang/Enum.ordinal"].(int32)
			}
			return a - b, true
		}
	}
	if r.owner == "java/util/Arrays" && r.name == "copyOfRange" && strings.HasPrefix(r.desc, "([Ljava/lang/Object;II)") {
		original := args[0].(*array)
		from, to := int(integer(args[1])), int(integer(args[2]))
		if from < 0 || from > len(original.values) || from > to {
			raiseJava("java/lang/ArrayIndexOutOfBoundsException", "copyOfRange")
		}
		a := v.objectArray(to-from, original.descriptor)
		copy(a.values, original.values[from:min(to, len(original.values))])
		return a, true
	}
	if r.owner == "java/util/Comparator" {
		switch {
		case r.name == "naturalOrder" && r.desc == "()Ljava/util/Comparator;":
			return v.alloc("java/util/Comparator", &javaComparator{kind: "natural"}), true
		case r.name == "reverseOrder" && r.desc == "()Ljava/util/Comparator;":
			return v.alloc("java/util/Comparator", &javaComparator{kind: "reverse"}), true
		case r.name == "comparingInt" && r.desc == "(Ljava/util/function/ToIntFunction;)Ljava/util/Comparator;":
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "keyExtractor")
			}
			return v.alloc("java/util/Comparator", &javaComparator{kind: "comparingInt", key: args[0]}), true
		case r.name == "comparing" && (r.desc == "(Ljava/util/function/Function;)Ljava/util/Comparator;" || r.desc == "(Ljava/util/function/Function;Ljava/util/Comparator;)Ljava/util/Comparator;"):
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "keyExtractor")
			}
			c := &javaComparator{kind: "comparing", key: args[0]}
			if r.desc != "(Ljava/util/function/Function;)Ljava/util/Comparator;" {
				if args[1] == nil {
					raiseJava("java/lang/NullPointerException", "keyComparator")
				}
				c.cmp = args[1]
			}
			return v.alloc("java/util/Comparator", c), true
		case receiver != nil:
			cmp, ok := objectData(receiver).(*javaComparator)
			if !ok {
				break
			}
			switch {
			case r.name == "compare" && r.desc == "(Ljava/lang/Object;Ljava/lang/Object;)I":
				return int32(v.comparatorCompare(cmp, args[0], args[1])), true
			case r.name == "reversed" && r.desc == "()Ljava/util/Comparator;":
				return v.alloc("java/util/Comparator", &javaComparator{kind: "reversed", cmp: receiver}), true
			case (r.name == "thenComparing" || r.name == "thenComparingInt" || r.name == "thenComparingLong" || r.name == "thenComparingDouble") && (r.desc == "(Ljava/util/Comparator;)Ljava/util/Comparator;" || r.desc == "(Ljava/util/function/Function;)Ljava/util/Comparator;" || r.desc == "(Ljava/util/function/Function;Ljava/util/Comparator;)Ljava/util/Comparator;" || r.desc == "(Ljava/util/function/ToIntFunction;)Ljava/util/Comparator;" || r.desc == "(Ljava/util/function/ToLongFunction;)Ljava/util/Comparator;" || r.desc == "(Ljava/util/function/ToDoubleFunction;)Ljava/util/Comparator;"):
				var next any
				switch {
				case r.desc == "(Ljava/util/Comparator;)Ljava/util/Comparator;":
					if args[0] == nil {
						raiseJava("java/lang/NullPointerException", "other")
					}
					next = args[0]
				case strings.Contains(r.desc, "ToIntFunction") || r.name == "thenComparingInt":
					next = v.alloc("java/util/Comparator", &javaComparator{kind: "comparingInt", key: args[0]})
				case r.desc == "(Ljava/util/function/Function;)Ljava/util/Comparator;":
					next = v.alloc("java/util/Comparator", &javaComparator{kind: "comparing", key: args[0]})
				default:
					if len(args) > 1 {
						next = v.alloc("java/util/Comparator", &javaComparator{kind: "comparing", key: args[0], cmp: args[1]})
					} else {
						next = v.alloc("java/util/Comparator", &javaComparator{kind: "comparing", key: args[0]})
					}
				}
				return v.alloc("java/util/Comparator", &javaComparator{kind: "then", cmp: receiver, key: next}), true
			}
		}
	}
	if r.owner == "java/util/List" && r.name == "sort" && r.desc == "(Ljava/util/Comparator;)V" && receiver != nil {
		if l, ok := objectData(receiver).(*javaList); ok {
			if l.readonly {
				raiseJava("java/lang/UnsupportedOperationException", "sort")
			}
			v.sortJavaList(l, args[0])
			return nil, true
		}
	}
	if r.owner == "java/util/Collections" && receiver == nil {
		switch r.name {
		case "sort":
			if r.desc == "(Ljava/util/List;)V" || r.desc == "(Ljava/util/List;Ljava/util/Comparator;)V" {
				if l, ok := objectData(args[0]).(*javaList); ok {
					if l.readonly {
						raiseJava("java/lang/UnsupportedOperationException", "sort")
					}
					var cmp any
					if r.desc == "(Ljava/util/List;Ljava/util/Comparator;)V" {
						cmp = args[1]
					}
					v.sortJavaList(l, cmp)
					return nil, true
				}
			}
		case "reverse":
			if l, ok := objectData(args[0]).(*javaList); ok && r.desc == "(Ljava/util/List;)V" {
				if l.readonly {
					raiseJava("java/lang/UnsupportedOperationException", "reverse")
				}
				for i, j := 0, len(l.values)-1; i < j; i, j = i+1, j-1 {
					l.values[i], l.values[j] = l.values[j], l.values[i]
				}
				l.generation++
				return nil, true
			}
		case "addAll":
			if r.desc == "(Ljava/util/Collection;[Ljava/lang/Object;)Z" {
				if args[0] == nil || args[1] == nil {
					raiseJava("java/lang/NullPointerException", "addAll")
				}
				changed := int32(0)
				for _, x := range args[1].(*array).values {
					if integer(h.vm.method(reference{"java/util/Collection", "add", "(Ljava/lang/Object;)Z"}, args[0], true, []any{x})) != 0 {
						changed = 1
					}
				}
				return changed, true
			}
		case "singletonList":
			if r.desc == "(Ljava/lang/Object;)Ljava/util/List;" {
				return v.alloc("java/util/List", &javaList{values: []any{args[0]}, readonly: true}), true
			}
		case "singleton":
			if r.desc == "(Ljava/lang/Object;)Ljava/util/Set;" {
				return v.alloc("java/util/Set", &javaSet{values: []any{args[0]}, readonly: true}), true
			}
		case "singletonMap":
			if r.desc == "(Ljava/lang/Object;Ljava/lang/Object;)Ljava/util/Map;" {
				return v.alloc("java/util/Map", &javaMap{entries: []javaPair{{args[0], args[1]}}, readonly: true}), true
			}
		case "newSetFromMap":
			// Only an empty map is accepted (as the JDK requires); the set then
			// owns its storage, which is equivalent while the map stays unused.
			if r.desc == "(Ljava/util/Map;)Ljava/util/Set;" {
				m, ok := objectData(args[0]).(*javaMap)
				if !ok || len(m.entries) != 0 {
					raiseJava("java/lang/IllegalArgumentException", "Map is non-empty")
				}
				return v.alloc("java/util/Set", &javaSet{}), true
			}
		case "unmodifiableSet", "unmodifiableList", "unmodifiableMap", "unmodifiableCollection", "unmodifiableSortedSet", "unmodifiableSortedMap", "unmodifiableNavigableSet", "unmodifiableNavigableMap", "synchronizedList", "synchronizedSet", "synchronizedMap", "synchronizedCollection":
			// Read-through view: shares the backing collection so later
			// writes to the source stay visible, as in the JDK.
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", r.name)
			}
			return args[0], true
		}
	}
	if r.owner == "java/util/Objects" && r.name == "requireNonNull" && r.desc == "(Ljava/lang/Object;Ljava/util/function/Supplier;)Ljava/lang/Object;" {
		if args[0] == nil {
			message := ""
			if args[1] != nil {
				mm := v.method(reference{"java/util/function/Supplier", "get", "()Ljava/lang/Object;"}, args[1], true, nil)
				if inertOf(mm) != nil && lambdaOwner(args[1]) == "net/minecraftforge/registries/RegistryObject" {
					// Forge RegistryObject.get names the absent entry; an unresolved
					// name leaves the message undetermined but the gap is the same.
					mm = contentRegistryGap + "(unresolved name)"
				}
				if m, ok := mm.(string); ok {
					message = m
				}
			}
			raiseJava("java/lang/NullPointerException", message)
		}
		return args[0], true
	}
	if r.owner == "java/util/Objects" && r.name == "requireNonNull" && (r.desc == "(Ljava/lang/Object;)Ljava/lang/Object;" || r.desc == "(Ljava/lang/Object;Ljava/lang/String;)Ljava/lang/Object;") {
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "null value")
		}
		return args[0], true
	}
	if r.owner == "java/lang/String" && (r.name == "indexOf" || r.name == "lastIndexOf") {
		var needle []uint16
		switch r.desc {
		case "(I)I", "(II)I":
			needle = utf16.Encode([]rune{rune(integer(args[0]))})
		case "(Ljava/lang/String;)I", "(Ljava/lang/String;I)I":
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", r.name)
			}
			needle = utf16.Encode([]rune(javaString(args[0])))
		default:
			return nil, false
		}
		hay := utf16.Encode([]rune(javaString(receiver)))
		match := func(i int) bool {
			if i < 0 || i+len(needle) > len(hay) {
				return false
			}
			for k, c := range needle {
				if hay[i+k] != c {
					return false
				}
			}
			return true
		}
		if r.name == "indexOf" {
			from := 0
			if len(args) == 2 {
				from = max(int(integer(args[1])), 0)
			}
			for i := from; i <= len(hay); i++ {
				if match(i) {
					return int32(i), true
				}
			}
			return int32(-1), true
		}
		from := len(hay)
		if len(args) == 2 {
			from = min(int(integer(args[1])), len(hay))
		}
		for i := from; i >= 0; i-- {
			if match(i) {
				return int32(i), true
			}
		}
		return int32(-1), true
	}
	if r.owner == "java/lang/String" && (r.name == "codePoints" || r.name == "chars") && r.desc == "()Ljava/util/stream/IntStream;" {
		// One element per code point, matching this VM's char indexing.
		runes := []rune(javaString(receiver))
		values := make([]any, len(runes))
		for i, c := range runes {
			values[i] = int32(c)
		}
		v.reserveCollection(len(values))
		return v.alloc("java/util/stream/IntStream", &javaList{values: values, readonly: true}), true
	}
	if r.owner == "java/util/stream/IntStream" && receiver != nil {
		if l, ok := objectData(receiver).(*javaList); ok {
			switch {
			case r.name == "forEach" && r.desc == "(Ljava/util/function/IntConsumer;)V", r.name == "forEachOrdered" && r.desc == "(Ljava/util/function/IntConsumer;)V":
				for _, x := range l.values {
					v.method(reference{"java/util/function/IntConsumer", "accept", "(I)V"}, args[0], true, []any{x})
				}
				return nil, true
			case r.name == "count" && r.desc == "()J":
				return int64(len(l.values)), true
			case r.name == "toArray" && r.desc == "()[I":
				a := v.objectArray(len(l.values), "[I")
				copy(a.values, l.values)
				return a, true
			}
		}
	}
	if r.owner == "java/lang/String" && r.name == "getChars" && r.desc == "(II[CI)V" {
		// Indexed by code point, matching this VM's charAt/length.
		runes := []rune(javaString(receiver))
		begin, end, at := int(integer(args[0])), int(integer(args[1])), int(integer(args[3]))
		dst, ok := args[2].(*array)
		if !ok {
			raiseJava("java/lang/NullPointerException", "getChars")
		}
		if begin < 0 || begin > end || end > len(runes) || at < 0 || at+end-begin > len(dst.values) {
			raiseJava("java/lang/StringIndexOutOfBoundsException", "getChars")
		}
		for i := begin; i < end; i++ {
			dst.values[at+i-begin] = int32(runes[i])
		}
		return nil, true
	}
	if r.owner == "java/lang/String" && r.name == "equalsIgnoreCase" && r.desc == "(Ljava/lang/String;)Z" {
		if args[0] == nil {
			return int32(0), true
		}
		if strings.EqualFold(javaString(receiver), javaString(args[0])) {
			return int32(1), true
		}
		return int32(0), true
	}
	if r.owner == "java/lang/String" && r.name == "intern" && r.desc == "()Ljava/lang/String;" {
		if s, ok := receiver.(string); ok {
			return s, true // Go strings compare by value; interning is identity only
		}
	}
	if r.owner == "java/lang/String" && r.name == "toLowerCase" && (r.desc == "(Ljava/util/Locale;)Ljava/lang/String;" || r.desc == "()Ljava/lang/String;") {
		locale := "ROOT"
		if len(args) == 1 {
			locale = objectData(args[0]).(string)
			if locale != "ROOT" && locale != "ENGLISH" {
				fail("unbound case-mapping locale")
			}
		} else {
			locale = h.options.Language
		}
		text := javaString(receiver)
		for _, ch := range text {
			if ch > 127 {
				fail("Unicode case mapping is not bound")
			}
		}
		if len(args) == 0 && strings.Contains(text, "I") {
			if locale == "" {
				fail("Java default locale is required for case mapping")
			}
			if locale == "tr" || locale == "az" {
				return strings.ToLower(strings.ReplaceAll(text, "I", "ı")), true
			}
		}
		return strings.ToLower(text), true
	}
	if strings.HasPrefix(r.owner, "[") && r.name == "clone" && r.desc == "()Ljava/lang/Object;" {
		x := receiver.(*array)
		v.slots += len(x.values)
		if v.slots > 131072 {
			fail("array slot budget exceeded")
		}
		v.alloc("[", nil)
		return &array{values: append([]any(nil), x.values...), descriptor: x.descriptor}, true
	}
	if r.owner == "java/lang/Enum" {
		switch r.name {
		case "<init>":
			if r.desc != "(Ljava/lang/String;I)V" {
				return nil, false
			}
			o := receiver.(*object)
			o.fields["java/lang/Enum.name"] = args[0]
			o.fields["java/lang/Enum.ordinal"] = args[1]
			return nil, true
		case "name", "toString":
			o := receiver.(*object)
			if name, ok := o.fields["java/lang/Enum.name"].(string); ok {
				return name, true
			}
			if frameworkName(o.class) == "net/minecraftforge/api/distmarker/Dist" || o.class == "net/minecraft/world/item/Rarity" || frameworkName(o.class) == "net/minecraftforge/fml/config/ModConfig$Type" {
				return o.data, true
			}
			fail("unbound enum name %s", o.class)
		case "ordinal":
			if ordinal, ok := receiver.(*object).fields["java/lang/Enum.ordinal"]; ok {
				return ordinal, true
			}
			fail("unbound enum ordinal %s", receiver.(*object).class)
		}
	}
	if _, ok := exceptionSupers[r.owner]; ok {
		if r.name == "<init>" && (r.desc == "()V" || r.desc == "(Ljava/lang/String;)V" || r.desc == "(Ljava/lang/String;Ljava/lang/Throwable;)V") {
			if len(args) >= 1 {
				receiver.(*object).data = args[0]
			}
			if len(args) == 2 {
				receiver.(*object).fields["java/lang/Throwable.cause:Ljava/lang/Throwable;"] = args[1]
			}
			return nil, true
		}
		if r.name == "getMessage" && r.desc == "()Ljava/lang/String;" {
			if _, ok := receiver.(*object).data.(unboundExceptionMessage); ok {
				fail("native exception detail message is not bound")
			}
			return receiver.(*object).data, true
		}
	}
	return nil, false
}

func javaString(x any) string {
	if x == nil {
		raiseJava("java/lang/NullPointerException", "null string")
	}
	if s, ok := x.(string); ok {
		return s
	}
	if o, ok := x.(*object); ok && o != nil && o.class == "java/lang/String" {
		if s, ok := o.data.(string); ok {
			return s
		}
	}
	fail("expected Java string")
	return ""
}

func javaFloating(x any) float64 {
	if f, ok := x.(float32); ok {
		return float64(f)
	}
	return x.(float64)
}

// javaToMap is Collectors.toMap(keyMapper, valueMapper): a HashMap that
// rejects duplicate keys and null values.
// merge and supplier are set by the four-argument overload; a nil merge
// throws on duplicates and a nil supplier yields a HashMap.
type javaToMap struct{ key, value, merge, supplier any }

// javaComparator is a java.util.Comparator built from the factory helpers.
type javaComparator struct {
	kind string // natural, reverse, comparing, comparingInt, reversed, then
	key  any
	cmp  any
}

func (v *machine) sortJavaList(l *javaList, cmp any) {
	c, _ := objectData(cmp).(*javaComparator)
	if cmp != nil && c == nil {
		// Interpreted Comparator subtype: dispatch compare virtually.
		c = &javaComparator{kind: "virtual", cmp: cmp}
	}
	if c == nil {
		c = &javaComparator{kind: "natural"}
	}
	values := l.values
	for i := 1; i < len(values); i++ {
		j := i
		for j > 0 {
			v.tick()
			if v.comparatorCompare(c, values[j-1], values[j]) <= 0 {
				break
			}
			values[j-1], values[j] = values[j], values[j-1]
			j--
		}
	}
	l.generation++
}

func (v *machine) comparatorCompare(c *javaComparator, a, b any) int {
	if c == nil {
		return v.naturalCompare(a, b)
	}
	switch c.kind {
	case "natural":
		return v.naturalCompare(a, b)
	case "reverse":
		return -v.naturalCompare(a, b)
	case "reversed":
		inner, _ := objectData(c.cmp).(*javaComparator)
		if inner == nil && c.cmp != nil {
			inner = &javaComparator{kind: "virtual", cmp: c.cmp}
		}
		return -v.comparatorCompare(inner, a, b)
	case "comparingInt":
		ia := integer(v.method(reference{"java/util/function/ToIntFunction", "applyAsInt", "(Ljava/lang/Object;)I"}, c.key, true, []any{a}))
		ib := integer(v.method(reference{"java/util/function/ToIntFunction", "applyAsInt", "(Ljava/lang/Object;)I"}, c.key, true, []any{b}))
		switch {
		case ia < ib:
			return -1
		case ia > ib:
			return 1
		}
		return 0
	case "comparing":
		ka := v.method(reference{"java/util/function/Function", "apply", "(Ljava/lang/Object;)Ljava/lang/Object;"}, c.key, true, []any{a})
		kb := v.method(reference{"java/util/function/Function", "apply", "(Ljava/lang/Object;)Ljava/lang/Object;"}, c.key, true, []any{b})
		if c.cmp != nil {
			inner, _ := objectData(c.cmp).(*javaComparator)
			if inner == nil {
				inner = &javaComparator{kind: "virtual", cmp: c.cmp}
			}
			return v.comparatorCompare(inner, ka, kb)
		}
		return v.naturalCompare(ka, kb)
	case "then":
		primary, _ := objectData(c.cmp).(*javaComparator)
		if primary == nil {
			primary = &javaComparator{kind: "virtual", cmp: c.cmp}
		}
		if r := v.comparatorCompare(primary, a, b); r != 0 {
			return r
		}
		secondary, _ := objectData(c.key).(*javaComparator)
		if secondary == nil {
			secondary = &javaComparator{kind: "virtual", cmp: c.key}
		}
		return v.comparatorCompare(secondary, a, b)
	case "virtual":
		return int(integer(v.method(reference{"java/util/Comparator", "compare", "(Ljava/lang/Object;Ljava/lang/Object;)I"}, c.cmp, true, []any{a, b})))
	default:
		fail("unbound comparator kind %q", c.kind)
	}
	return 0
}

func (v *machine) naturalCompare(a, b any) int {
	if a == nil || b == nil {
		raiseJava("java/lang/NullPointerException", "compareTo")
	}
	if sa, ok := a.(string); ok {
		sb, ok := b.(string)
		if !ok {
			fail("natural order type mismatch")
		}
		switch {
		case sa < sb:
			return -1
		case sa > sb:
			return 1
		}
		return 0
	}
	return int(integer(v.method(reference{"java/lang/Comparable", "compareTo", "(Ljava/lang/Object;)I"}, a, true, []any{b})))
}

type javaBigInt struct{ n *big.Int }

// javaIndent follows String.indent: split as String.lines, adjust leading
// white space per line, and terminate every line with \n.
func javaIndent(s string, n int) string {
	if s == "" {
		return ""
	}
	var lines []string
	for len(s) > 0 {
		i := strings.IndexAny(s, "\r\n")
		if i < 0 {
			lines = append(lines, s)
			break
		}
		lines = append(lines, s[:i])
		if s[i] == '\r' && i+1 < len(s) && s[i+1] == '\n' {
			i++
		}
		s = s[i+1:]
	}
	var b strings.Builder
	for _, line := range lines {
		switch {
		case n > 0:
			b.WriteString(strings.Repeat(" ", n))
		case n < 0:
			k := 0
			for k < len(line) && k < -n && javaWhitespace(rune(line[k])) {
				k++
			}
			line = line[k:]
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func javaWhitespace(c rune) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r' || c >= 0x1c && c <= 0x1f
}

var javaDecimalFloat = regexp.MustCompile(`^[+-]?(?:[0-9]+\.?[0-9]*|\.[0-9]+)(?:[eE][+-]?[0-9]+)?[fFdD]?$`)

// javaParseDouble follows Double.parseDouble for decimal, NaN and Infinity
// forms; hexadecimal literals are not modelled and fail closed.
func javaParseDouble(s string) float64 {
	s = strings.TrimFunc(s, func(c rune) bool { return c <= ' ' })
	body := strings.TrimLeft(s, "+-")
	if len(s)-len(body) <= 1 && (body == "NaN" || body == "Infinity") {
		if body == "NaN" {
			return math.NaN()
		}
		if s[0] == '-' {
			return math.Inf(-1)
		}
		return math.Inf(1)
	}
	if strings.HasPrefix(body, "0x") || strings.HasPrefix(body, "0X") {
		fail("unbound hexadecimal Double.parseDouble")
	}
	if !javaDecimalFloat.MatchString(s) {
		raiseJava("java/lang/NumberFormatException", "For input string: \""+s+"\"")
	}
	s = strings.TrimRight(s, "fFdD")
	d, err := strconv.ParseFloat(s, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		raiseJava("java/lang/NumberFormatException", "For input string: \""+s+"\"")
	}
	return d
}

func lambdaOwner(x any) string {
	l, ok := x.(*lambda)
	if !ok {
		l, ok = objectData(x).(*lambda)
	}
	if !ok || l == nil {
		return ""
	}
	return l.target.owner
}

// stringValue reads a Java String held as a Go string or as a String object.
func stringValue(x any) (string, bool) {
	switch s := x.(type) {
	case string:
		return s, true
	case *object:
		if s != nil && s.class == "java/lang/String" {
			str, ok := s.data.(string)
			return str, ok
		}
	}
	return "", false
}

// javaToCollection is Collectors.toCollection(supplier).
type javaToCollection struct{ supplier any }
