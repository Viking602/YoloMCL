package modjar

import (
	"fmt"
	"strings"
)

// Recipe codecs implement the DynamicOps protocol the same way DFU does: every
// read and write goes through the caller's ops object, results are real DFU
// DataResult values, and unsupported recipe kinds fail closed.

const (
	serializationPkg = "com/mojang/serialization/"
	dataResultClass  = serializationPkg + "DataResult"
	dynamicOpsClass  = serializationPkg + "DynamicOps"
	mapLikeClass     = serializationPkg + "MapLike"
	recordBuilderAPI = serializationPkg + "RecordBuilder"
)

type constSupplier struct{ value any }

func recipeCodec(x any) (any, bool) {
	o, ok := x.(*object)
	if !ok || o == nil {
		return nil, false
	}
	switch o.data.(type) {
	case codecRecipe, recordRecipe:
		return o.data, true
	}
	return nil, false
}

func (h *biomeProbeBindings) dynamicCodecCall(r reference, receiver any, args []any) (any, bool) {
	if !strings.HasPrefix(r.owner, serializationPkg) {
		return nil, false
	}
	recipe, ok := recipeCodec(receiver)
	if !ok {
		return nil, false
	}
	switch r.name + r.desc {
	case "decode(L" + dynamicOpsClass + ";Ljava/lang/Object;)L" + dataResultClass + ";":
		return h.decodeValue(recipe, args[0], args[1]), true
	case "decode(L" + dynamicOpsClass + ";L" + mapLikeClass + ";)L" + dataResultClass + ";":
		return h.decodeMap(recipe, args[0], args[1]), true
	case "encode(Ljava/lang/Object;L" + dynamicOpsClass + ";Ljava/lang/Object;)L" + dataResultClass + ";":
		return h.encodeValue(recipe, args[0], args[1], args[2]), true
	case "encode(Ljava/lang/Object;L" + dynamicOpsClass + ";L" + recordBuilderAPI + ";)L" + recordBuilderAPI + ";":
		return h.encodeMap(recipe, args[0], args[1], args[2]), true
	}
	return nil, false
}

func (h *biomeProbeBindings) ops(ops any, name, desc string, args ...any) any {
	if ops == nil {
		raiseJava("java/lang/NullPointerException", "null DynamicOps")
	}
	return h.host.vm.method(reference{dynamicOpsClass, name, desc}, ops, true, args)
}

func (h *biomeProbeBindings) success(x any) any {
	return h.host.vm.method(reference{dataResultClass, "success", "(Ljava/lang/Object;)L" + dataResultClass + ";"}, nil, false, []any{x})
}

func (h *biomeProbeBindings) failure(format string, args ...any) any {
	message := fmt.Sprintf(format, args...)
	return h.host.vm.method(reference{dataResultClass, "error", "(Ljava/util/function/Supplier;)L" + dataResultClass + ";"}, nil, false, []any{&constSupplier{message}})
}

// unwrap returns the success value, or ok=false when the result is an error.
func (h *biomeProbeBindings) unwrap(result any) (any, bool) {
	if result == nil {
		raiseJava("java/lang/NullPointerException", "null DataResult")
	}
	opt := h.host.vm.method(reference{dataResultClass, "result", "()Ljava/util/Optional;"}, result, true, nil)
	value, ok := objectData(opt).(javaOptional)
	if !ok {
		fail("unbound DataResult optional")
	}
	return value.value, value.value != nil
}

func (h *biomeProbeBindings) pair(first, second any) any {
	return h.host.vm.alloc("com/mojang/datafixers/util/Pair", javaPair{first, second})
}

func (h *biomeProbeBindings) callFunction(f any, args ...any) any {
	return h.host.vm.callSupplier(f, args)
}

func (h *biomeProbeBindings) box(desc string, x any) any {
	owner := map[string]string{"I": "java/lang/Integer", "J": "java/lang/Long", "F": "java/lang/Float", "D": "java/lang/Double", "Z": "java/lang/Boolean", "S": "java/lang/Short", "B": "java/lang/Byte"}[desc]
	return h.host.vm.method(reference{owner, "valueOf", "(" + desc + ")L" + owner + ";"}, nil, false, []any{x})
}

func (h *biomeProbeBindings) unboxNumber(n any, name, desc string) any {
	if n == nil {
		raiseJava("java/lang/NullPointerException", "null number")
	}
	return h.host.vm.method(reference{"java/lang/Number", name, "()" + desc}, n, true, nil)
}

var primitiveCodecs = map[string][2]string{
	"INT": {"intValue", "I"}, "LONG": {"longValue", "J"}, "FLOAT": {"floatValue", "F"}, "DOUBLE": {"doubleValue", "D"}, "SHORT": {"shortValue", "S"}, "BYTE": {"byteValue", "B"},
}

func (h *biomeProbeBindings) decodeValue(recipe any, ops, input any) any {
	switch c := recipe.(type) {
	case recordRecipe:
		m := h.ops(ops, "getMap", "(Ljava/lang/Object;)L"+dataResultClass+";", input)
		like, ok := h.unwrap(m)
		if !ok {
			return m
		}
		result := h.decodeMap(recipe, ops, like)
		value, ok := h.unwrap(result)
		if !ok {
			return result
		}
		return h.success(h.pair(value, input))
	case codecRecipe:
		if p, ok := primitiveCodecs[c.kind]; ok {
			result := h.ops(ops, "getNumberValue", "(Ljava/lang/Object;)L"+dataResultClass+";", input)
			n, ok := h.unwrap(result)
			if !ok {
				return result
			}
			return h.success(h.pair(h.box(p[1], h.unboxNumber(n, p[0], p[1])), h.ops(ops, "empty", "()Ljava/lang/Object;")))
		}
		switch c.kind {
		case "BOOL", "STRING":
			name := "getBooleanValue"
			if c.kind == "STRING" {
				name = "getStringValue"
			}
			result := h.ops(ops, name, "(Ljava/lang/Object;)L"+dataResultClass+";", input)
			value, ok := h.unwrap(result)
			if !ok {
				return result
			}
			return h.success(h.pair(value, h.ops(ops, "empty", "()Ljava/lang/Object;")))
		case "resourceKey":
			result := h.ops(ops, "getStringValue", "(Ljava/lang/Object;)L"+dataResultClass+";", input)
			value, ok := h.unwrap(result)
			if !ok {
				return result
			}
			id, valid := resourceLocation(javaString(value))
			if !valid {
				return h.failure("Not a valid resource location: %s", javaString(value))
			}
			registry := objectData(c.args[0]).(string)
			key := h.host.symbol("key:"+registry+"\x00"+id, "net/minecraft/resources/ResourceKey", id)
			return h.success(h.pair(key, h.ops(ops, "empty", "()Ljava/lang/Object;")))
		case "xmap":
			result := h.host.vm.method(reference{serializationPkg + "Decoder", "decode", "(L" + dynamicOpsClass + ";Ljava/lang/Object;)L" + dataResultClass + ";"}, c.args[0], true, []any{ops, input})
			p, ok := h.unwrap(result)
			if !ok {
				return result
			}
			pair := objectData(p).(javaPair)
			return h.success(h.pair(h.callFunction(c.args[1], pair.left), pair.right))
		}
		fail("unbound codec decode %s", c.kind)
	}
	fail("unbound codec decode")
	return nil
}

func (h *biomeProbeBindings) decodeMap(recipe any, ops, like any) any {
	v := h.host.vm
	switch c := recipe.(type) {
	case recordRecipe:
		values := make([]any, len(c.elements))
		for i, element := range c.elements {
			getter, ok := objectData(element).(codecRecipe)
			if !ok || getter.kind != "getter" {
				fail("unbound record element decode")
			}
			result := v.method(reference{serializationPkg + "MapDecoder", "decode", "(L" + dynamicOpsClass + ";L" + mapLikeClass + ";)L" + dataResultClass + ";"}, getter.args[0], true, []any{ops, like})
			value, ok := h.unwrap(result)
			if !ok {
				return result
			}
			values[i] = value
		}
		return h.success(v.callLambda(c.factory, values))
	case codecRecipe:
		switch c.kind {
		case "field", "optional":
			name := javaString(c.args[1])
			value := v.method(reference{mapLikeClass, "get", "(Ljava/lang/String;)Ljava/lang/Object;"}, like, true, []any{name})
			if value == nil {
				if c.kind == "field" {
					return h.failure("No key %s in %v", name, like)
				}
				if len(c.args) == 3 {
					return h.success(c.args[2])
				}
				return h.success(v.alloc("java/util/Optional", javaOptional{}))
			}
			result := v.method(reference{serializationPkg + "Decoder", "parse", "(L" + dynamicOpsClass + ";Ljava/lang/Object;)L" + dataResultClass + ";"}, c.args[0], true, []any{ops, value})
			parsed, ok := h.unwrap(result)
			if !ok {
				return result
			}
			if c.kind == "optional" && len(c.args) == 2 {
				return h.success(v.alloc("java/util/Optional", javaOptional{parsed}))
			}
			return h.success(parsed)
		}
		fail("unbound map codec decode %s", c.kind)
	}
	fail("unbound map codec decode")
	return nil
}

func (h *biomeProbeBindings) encodeValue(recipe any, value, ops, prefix any) any {
	v := h.host.vm
	switch c := recipe.(type) {
	case recordRecipe:
		builder := h.ops(ops, "mapBuilder", "()L"+recordBuilderAPI+";")
		builder = h.encodeMap(recipe, value, ops, builder)
		return v.method(reference{recordBuilderAPI, "build", "(Ljava/lang/Object;)L" + dataResultClass + ";"}, builder, true, []any{prefix})
	case codecRecipe:
		var encoded any
		if p, ok := primitiveCodecs[c.kind]; ok {
			create := map[string]string{"I": "createInt", "J": "createLong", "F": "createFloat", "D": "createDouble", "S": "createShort", "B": "createByte"}[p[1]]
			encoded = h.ops(ops, create, "("+p[1]+")Ljava/lang/Object;", h.unboxNumber(value, p[0], p[1]))
		} else {
			switch c.kind {
			case "BOOL":
				encoded = h.ops(ops, "createBoolean", "(Z)Ljava/lang/Object;", v.method(reference{"java/lang/Boolean", "booleanValue", "()Z"}, value, true, nil))
			case "STRING":
				encoded = h.ops(ops, "createString", "(Ljava/lang/String;)Ljava/lang/Object;", javaString(value))
			case "resourceKey":
				id, ok := objectData(value).(string)
				if !ok {
					fail("unbound resource key encode")
				}
				encoded = h.ops(ops, "createString", "(Ljava/lang/String;)Ljava/lang/Object;", id)
			case "xmap":
				return v.method(reference{serializationPkg + "Encoder", "encode", "(Ljava/lang/Object;L" + dynamicOpsClass + ";Ljava/lang/Object;)L" + dataResultClass + ";"}, c.args[0], true, []any{h.callFunction(c.args[2], value), ops, prefix})
			default:
				fail("unbound codec encode %s", c.kind)
			}
		}
		return h.ops(ops, "mergeToPrimitive", "(Ljava/lang/Object;Ljava/lang/Object;)L"+dataResultClass+";", prefix, encoded)
	}
	fail("unbound codec encode")
	return nil
}

func (h *biomeProbeBindings) encodeMap(recipe any, value, ops, builder any) any {
	v := h.host.vm
	switch c := recipe.(type) {
	case recordRecipe:
		for _, element := range c.elements {
			getter, ok := objectData(element).(codecRecipe)
			if !ok || getter.kind != "getter" {
				fail("unbound record element encode")
			}
			part := h.callFunction(getter.args[1], value)
			builder = v.method(reference{serializationPkg + "MapEncoder", "encode", "(Ljava/lang/Object;L" + dynamicOpsClass + ";L" + recordBuilderAPI + ";)L" + recordBuilderAPI + ";"}, getter.args[0], true, []any{part, ops, builder})
		}
		return builder
	case codecRecipe:
		switch c.kind {
		case "field", "optional":
			name := javaString(c.args[1])
			if c.kind == "optional" {
				if len(c.args) == 3 {
					if javaEquals(v, value, c.args[2]) {
						return builder
					}
				} else {
					opt, ok := objectData(value).(javaOptional)
					if !ok {
						fail("unbound optional field encode")
					}
					if opt.value == nil {
						return builder
					}
					value = opt.value
				}
			}
			encoded := v.method(reference{serializationPkg + "Encoder", "encodeStart", "(L" + dynamicOpsClass + ";Ljava/lang/Object;)L" + dataResultClass + ";"}, c.args[0], true, []any{ops, value})
			return v.method(reference{recordBuilderAPI, "add", "(Ljava/lang/String;L" + dataResultClass + ";)L" + recordBuilderAPI + ";"}, builder, true, []any{name, encoded})
		}
		fail("unbound map codec encode %s", c.kind)
	}
	fail("unbound map codec encode")
	return nil
}

func javaEquals(v *machine, a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return integer(v.method(reference{"java/lang/Object", "equals", "(Ljava/lang/Object;)Z"}, a, true, []any{b})) != 0
}

// resourceLocation applies ResourceLocation's character rules, defaulting the
// namespace to minecraft as ResourceLocation.parse does.
func resourceLocation(s string) (string, bool) {
	namespace, path, found := strings.Cut(s, ":")
	if !found {
		namespace, path = "minecraft", s
	}
	if namespace == "" {
		namespace = "minecraft"
	}
	for _, r := range namespace {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.') {
			return "", false
		}
	}
	for _, r := range path {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.' || r == '/') {
			return "", false
		}
	}
	return namespace + ":" + path, true
}
