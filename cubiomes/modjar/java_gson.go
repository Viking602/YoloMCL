package modjar

import (
	"encoding/json"
	"strings"
)

func (h *bindings) gsonCall(r reference, receiver any, args []any) (any, bool) {
	v := h.vm
	switch {
	case r.name == "fromJson" && r.desc == "(Ljava/lang/String;Ljava/lang/reflect/Type;)Ljava/lang/Object;":
		if args[0] == nil {
			return nil, true
		}
		raw := javaString(args[0])
		target := args[1]
		return v.gsonDecode(raw, target), true
	case r.name == "fromJson" && r.desc == "(Ljava/lang/String;Ljava/lang/Class;)Ljava/lang/Object;":
		if args[0] == nil {
			return nil, true
		}
		return v.gsonDecode(javaString(args[0]), args[1]), true
	case r.name == "fromJson" && r.desc == "(Ljava/io/Reader;Ljava/lang/Class;)Ljava/lang/Object;":
		if args[0] == nil {
			return nil, true
		}
		return v.gsonDecode(readerText(args[0]), args[1]), true
	case r.name == "fromJson" && r.desc == "(Ljava/io/Reader;Ljava/lang/reflect/Type;)Ljava/lang/Object;":
		if args[0] == nil {
			return nil, true
		}
		return v.gsonDecode(readerText(args[0]), args[1]), true
	case r.name == "toJson" && (r.desc == "(Ljava/lang/Object;)Ljava/lang/String;" || r.desc == "(Ljava/lang/Object;Ljava/lang/reflect/Type;)Ljava/lang/String;"):
		return v.gsonEncode(args[0]), true
	}
	return nil, false
}

func (v *machine) gsonDecode(raw string, target any) any {
	rawType, valueArgs := v.gsonTarget(target)
	switch rawType {
	case "java/util/TreeMap", "java/util/HashMap", "java/util/LinkedHashMap", "java/util/Map":
		var obj map[string]any
		if err := json.Unmarshal([]byte(raw), &obj); err != nil {
			raiseJava("com/google/gson/JsonSyntaxException", err.Error())
		}
		class := rawType
		if class == "java/util/Map" {
			class = "java/util/LinkedHashMap"
		}
		m := &javaMap{}
		valueKind := ""
		if len(valueArgs) >= 2 {
			valueKind = gsonRawName(valueArgs[1])
		}
		for k, val := range obj {
			v.reserveCollection(1)
			m.entries = append(m.entries, javaPair{k, gsonBox(v, val, valueKind)})
		}
		return v.alloc(class, m)
	case "java/util/ArrayList", "java/util/List", "java/util/LinkedList":
		var arr []any
		if err := json.Unmarshal([]byte(raw), &arr); err != nil {
			raiseJava("com/google/gson/JsonSyntaxException", err.Error())
		}
		class := rawType
		if class == "java/util/List" {
			class = "java/util/ArrayList"
		}
		elemKind := ""
		if len(valueArgs) >= 1 {
			elemKind = gsonRawName(valueArgs[0])
		}
		l := &javaList{}
		for _, val := range arr {
			v.reserveCollection(1)
			l.values = append(l.values, gsonBox(v, val, elemKind))
		}
		return v.alloc(class, l)
	case "java/lang/String":
		var s string
		if err := json.Unmarshal([]byte(raw), &s); err != nil {
			return raw
		}
		return s
	case "java/lang/Boolean", "boolean":
		var b bool
		if err := json.Unmarshal([]byte(raw), &b); err != nil {
			raiseJava("com/google/gson/JsonSyntaxException", err.Error())
		}
		bit := int32(0)
		if b {
			bit = 1
		}
		if rawType == "boolean" {
			return bit
		}
		return v.alloc("java/lang/Boolean", bit)
	default:
		return v.gsonDecodePOJO(raw, rawType)
	}
}

func (v *machine) gsonTarget(target any) (string, []any) {
	if target == nil {
		fail("null gson type")
	}
	if c, ok := target.(classLiteral); ok {
		return string(c), nil
	}
	if p, ok := objectData(target).(*javaParameterized); ok {
		name := ""
		if c, ok := p.raw.(classLiteral); ok {
			name = string(c)
		}
		return name, p.args
	}
	if o, ok := target.(*object); ok && o != nil && v.instanceOf(o, "java/lang/reflect/ParameterizedType") {
		raw := v.method(reference{"java/lang/reflect/ParameterizedType", "getRawType", "()Ljava/lang/reflect/Type;"}, o, true, nil)
		argsArr := v.method(reference{"java/lang/reflect/ParameterizedType", "getActualTypeArguments", "()[Ljava/lang/reflect/Type;"}, o, true, nil)
		name := ""
		switch t := raw.(type) {
		case classLiteral:
			name = string(t)
		case *object:
			if c, ok := objectData(t).(classLiteral); ok {
				name = string(c)
			} else if lit, ok := t.data.(classLiteral); ok {
				name = string(lit)
			} else if s, ok := objectData(t).(string); ok {
				name = strings.ReplaceAll(s, ".", "/")
			}
		}
		var args []any
		if a, ok := argsArr.(*array); ok {
			args = a.values
		}
		return name, args
	}
	fail("unbound gson target %T", target)
	return "", nil
}

func gsonRawName(t any) string {
	if c, ok := t.(classLiteral); ok {
		return string(c)
	}
	if p, ok := objectData(t).(*javaParameterized); ok {
		if c, ok := p.raw.(classLiteral); ok {
			return string(c)
		}
	}
	return ""
}

func gsonBox(v *machine, val any, kind string) any {
	switch x := val.(type) {
	case nil:
		return nil
	case bool:
		bit := int32(0)
		if x {
			bit = 1
		}
		if kind == "boolean" {
			return bit
		}
		return v.alloc("java/lang/Boolean", bit)
	case float64:
		// encoding/json uses float64 for all numbers.
		if kind == "java/lang/Integer" || kind == "int" {
			return v.alloc("java/lang/Integer", int32(x))
		}
		if kind == "java/lang/Long" || kind == "long" {
			return v.alloc("java/lang/Long", int64(x))
		}
		if kind == "java/lang/Double" || kind == "double" || kind == "" {
			if kind == "double" {
				return x
			}
			return v.alloc("java/lang/Double", x)
		}
		if kind == "java/lang/Float" || kind == "float" {
			return v.alloc("java/lang/Float", float32(x))
		}
		return v.alloc("java/lang/Double", x)
	case string:
		return x
	case map[string]any:
		m := &javaMap{}
		for k, child := range x {
			v.reserveCollection(1)
			m.entries = append(m.entries, javaPair{k, gsonBox(v, child, "")})
		}
		return v.alloc("java/util/LinkedHashMap", m)
	case []any:
		l := &javaList{}
		for _, child := range x {
			v.reserveCollection(1)
			l.values = append(l.values, gsonBox(v, child, ""))
		}
		return v.alloc("java/util/ArrayList", l)
	default:
		fail("unbound gson value %T", val)
	}
	return nil
}

func (v *machine) gsonEncode(x any) string {
	if x == nil {
		return "null"
	}
	b, err := json.Marshal(gsonExport(x))
	if err != nil {
		fail("gson encode: %v", err)
	}
	return string(b)
}

func gsonExport(x any) any {
	if x == nil {
		return nil
	}
	switch t := x.(type) {
	case string:
		return t
	case int32:
		return t
	case int64:
		return t
	case float64:
		return t
	case float32:
		return float64(t)
	case bool:
		return t
	case *object:
		return gsonExportObject(t)
	}
	fail("unbound gson encode %T", x)
	return nil
}

func readerText(x any) string {
	if x == nil {
		return ""
	}
	switch d := objectData(x).(type) {
	case *javaBytes:
		if d.pos >= len(d.data) {
			return ""
		}
		return string(d.data[d.pos:])
	case *javaStringReader:
		if d.pos >= len(d.runes) {
			return ""
		}
		return string(d.runes[d.pos:])
	}
	fail("unbound gson reader %T", objectData(x))
	return ""
}

func (v *machine) gsonDecodePOJO(raw, className string) any {
	var root any
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		raiseJava("com/google/gson/JsonSyntaxException", err.Error())
	}
	return v.gsonConvertToClass(root, className)
}

func (v *machine) gsonConvertToDesc(val any, desc string) any {
	if val == nil {
		return nil
	}
	switch desc {
	case "Z":
		switch b := val.(type) {
		case bool:
			if b {
				return int32(1)
			}
			return int32(0)
		case float64:
			if b != 0 {
				return int32(1)
			}
			return int32(0)
		case int32:
			if b != 0 {
				return int32(1)
			}
			return int32(0)
		default:
			fail("gson bool field %T", val)
		}
	case "B", "S", "I", "C":
		n, ok := val.(float64)
		if !ok {
			fail("gson int field")
		}
		return int32(n)
	case "J":
		n, ok := val.(float64)
		if !ok {
			fail("gson long field")
		}
		return int64(n)
	case "F":
		n, ok := val.(float64)
		if !ok {
			fail("gson float field")
		}
		return float32(n)
	case "D":
		n, ok := val.(float64)
		if !ok {
			fail("gson double field")
		}
		return n
	case "Ljava/lang/String;":
		s, ok := val.(string)
		if !ok {
			fail("gson string field")
		}
		return s
	case "Ljava/lang/Boolean;":
		bit := int32(0)
		switch b := val.(type) {
		case bool:
			if b {
				bit = 1
			}
		case float64:
			if b != 0 {
				bit = 1
			}
		case int32:
			if b != 0 {
				bit = 1
			}
		default:
			fail("gson Boolean field %T", val)
		}
		return v.alloc("java/lang/Boolean", bit)
	case "Ljava/lang/Integer;":
		n, ok := val.(float64)
		if !ok {
			fail("gson Integer field")
		}
		return v.alloc("java/lang/Integer", int32(n))
	case "Ljava/lang/Long;":
		n, ok := val.(float64)
		if !ok {
			fail("gson Long field")
		}
		return v.alloc("java/lang/Long", int64(n))
	case "Ljava/lang/Double;":
		n, ok := val.(float64)
		if !ok {
			fail("gson Double field")
		}
		return v.alloc("java/lang/Double", n)
	case "Ljava/lang/Float;":
		n, ok := val.(float64)
		if !ok {
			fail("gson Float field")
		}
		return v.alloc("java/lang/Float", float32(n))
	case "Ljava/lang/Object;":
		return gsonBox(v, val, "")
	}
	if strings.HasPrefix(desc, "L") && strings.HasSuffix(desc, ";") {
		return v.gsonConvertToClass(val, desc[1:len(desc)-1])
	}
	if strings.HasPrefix(desc, "[") {
		arr, ok := val.([]any)
		if !ok {
			fail("gson array field")
		}
		out := &array{descriptor: desc, values: make([]any, len(arr))}
		for i, child := range arr {
			v.reserveCollection(1)
			out.values[i] = v.gsonConvertToDesc(child, desc[1:])
		}
		return out
	}
	fail("unbound gson field descriptor %s", desc)
	return nil
}

func (v *machine) gsonConvertToClass(val any, className string) any {
	if val == nil {
		return nil
	}
	switch className {
	case "java/lang/String":
		s, ok := val.(string)
		if !ok {
			fail("gson String")
		}
		return s
	case "java/lang/Boolean":
		return v.gsonConvertToDesc(val, "Ljava/lang/Boolean;")
	case "java/lang/Integer":
		return v.gsonConvertToDesc(val, "Ljava/lang/Integer;")
	case "java/lang/Long":
		return v.gsonConvertToDesc(val, "Ljava/lang/Long;")
	case "java/lang/Double":
		return v.gsonConvertToDesc(val, "Ljava/lang/Double;")
	case "java/lang/Float":
		return v.gsonConvertToDesc(val, "Ljava/lang/Float;")
	case "java/util/Map", "java/util/LinkedHashMap", "java/util/HashMap", "java/util/TreeMap":
		obj, ok := val.(map[string]any)
		if !ok {
			fail("gson map")
		}
		m := &javaMap{}
		for k, child := range obj {
			v.reserveCollection(1)
			m.entries = append(m.entries, javaPair{k, gsonBox(v, child, "")})
		}
		class := className
		if class == "java/util/Map" {
			class = "java/util/LinkedHashMap"
		}
		return v.alloc(class, m)
	case "java/util/List", "java/util/ArrayList", "java/util/LinkedList":
		arr, ok := val.([]any)
		if !ok {
			fail("gson list")
		}
		l := &javaList{}
		for _, child := range arr {
			v.reserveCollection(1)
			l.values = append(l.values, gsonBox(v, child, ""))
		}
		class := className
		if class == "java/util/List" {
			class = "java/util/ArrayList"
		}
		return v.alloc(class, l)
	}
	obj, ok := val.(map[string]any)
	if !ok {
		fail("gson object for %s", className)
	}
	c := v.load(className)
	var comps []field
	for _, f := range c.fields {
		if f.access&8 != 0 {
			continue
		}
		if f.name == "this$0" || strings.HasPrefix(f.name, "this$") {
			continue
		}
		comps = append(comps, f)
	}
	hasDefault := false
	for _, m := range c.methods {
		if m.name == "<init>" && m.desc == "()V" {
			hasDefault = true
			break
		}
	}
	inst := v.alloc(className, nil)
	if hasDefault {
		v.method(reference{className, "<init>", "()V"}, inst, false, nil)
		for _, f := range comps {
			jv, ok := obj[f.name]
			if !ok {
				continue
			}
			key := fieldKey(reference{className, f.name, f.desc})
			if inst.fields == nil {
				inst.fields = map[string]any{}
			}
			inst.fields[key] = v.gsonConvertToDesc(jv, f.desc)
		}
		return inst
	}
	args := make([]any, len(comps))
	var b strings.Builder
	b.WriteByte('(')
	for i, f := range comps {
		args[i] = v.gsonConvertToDesc(obj[f.name], f.desc)
		b.WriteString(f.desc)
	}
	b.WriteString(")V")
	initDesc := b.String()
	found := false
	for _, m := range c.methods {
		if m.name == "<init>" && m.desc == initDesc {
			found = true
			break
		}
	}
	if !found {
		fail("gson constructor %s%s", className, initDesc)
	}
	v.method(reference{className, "<init>", initDesc}, inst, false, args)
	return inst
}

func gsonExportObject(o *object) any {
	if o == nil {
		return nil
	}
	switch d := o.data.(type) {
	case int32:
		if o.class == "java/lang/Boolean" {
			return d != 0
		}
		return d
	case int64:
		return d
	case float64:
		return d
	case float32:
		return float64(d)
	case *javaMap:
		out := map[string]any{}
		for _, e := range d.entries {
			out[javaString(e.left)] = gsonExport(e.right)
		}
		return out
	case *javaList:
		out := make([]any, len(d.values))
		for i, e := range d.values {
			out[i] = gsonExport(e)
		}
		return out
	}
	out := map[string]any{}
	for key, val := range o.fields {
		rest := key
		if i := strings.Index(rest, "."); i >= 0 {
			rest = rest[i+1:]
		}
		name, desc, _ := strings.Cut(rest, ":")
		if name == "" || strings.HasPrefix(name, "this$") {
			continue
		}
		if desc == "Z" {
			out[name] = integer(val) != 0
			continue
		}
		out[name] = gsonExport(val)
	}
	return out
}

var _ = strings.Contains
