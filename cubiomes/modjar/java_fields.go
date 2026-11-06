package modjar

import "strings"

// Reflection here refers only to fields declared by the supplied class files.
// It cannot access Go objects, host runtime classes or the filesystem.
type reflectedField struct {
	owner      string
	field      field
	accessible bool
}

func (h *bindings) fieldMetadataCall(r reference, receiver any, args []any) (any, bool) {
	v := h.vm
	if r.owner == "java/lang/Class" && r.name == "getDeclaredField" && r.desc == "(Ljava/lang/String;)Ljava/lang/reflect/Field;" {
		owner, ok := receiver.(classLiteral)
		if !ok {
			fail("unbound field metadata receiver")
		}
		name := javaString(args[0])
		c := v.load(string(owner))
		for _, f := range c.fields {
			if f.name == name {
				return v.alloc("java/lang/reflect/Field", &reflectedField{owner: string(owner), field: f}), true
			}
		}
		raiseJava("java/lang/NoSuchFieldException", name)
	}
	if r.owner == "java/lang/Class" && r.name == "getDeclaredFields" && r.desc == "()[Ljava/lang/reflect/Field;" {
		owner, ok := receiver.(classLiteral)
		if !ok {
			fail("invalid receiver for %s.%s", r.owner, r.name)
		}
		var out []any
		if s := string(owner); !strings.HasPrefix(s, "[") && v.fetchClass(s) {
			for _, f := range v.load(s).fields {
				v.tick()
				out = append(out, v.alloc("java/lang/reflect/Field", &reflectedField{owner: s, field: f}))
			}
		}
		a := v.objectArray(len(out), "[Ljava/lang/reflect/Field;")
		copy(a.values, out)
		return a, true
	}
	if r.owner == "java/lang/reflect/Field" && (r.name == "getType" && r.desc == "()Ljava/lang/Class;" || r.name == "getName" && r.desc == "()Ljava/lang/String;" || r.name == "getModifiers" && r.desc == "()I" || r.name == "getDeclaringClass" && r.desc == "()Ljava/lang/Class;") {
		f, ok := objectData(receiver).(*reflectedField)
		if !ok {
			fail("unbound reflected field")
		}
		switch r.name {
		case "getType":
			return descClassLiteral(f.field.desc), true
		case "getName":
			return f.field.name, true
		case "getModifiers":
			return int32(f.field.access & 0xffff), true
		}
		return classLiteral(f.owner), true
	}
	getter := r.name == "getBoolean" && r.desc == "(Ljava/lang/Object;)Z"
	boxed := r.name == "get" && r.desc == "(Ljava/lang/Object;)Ljava/lang/Object;"
	accessFlag := r.name == "setAccessible" && r.desc == "(Z)V"
	if r.owner != "java/lang/reflect/Field" || !getter && !boxed && !accessFlag {
		return nil, false
	}
	o, ok := receiver.(*object)
	if !ok || o == nil {
		raiseJava("java/lang/NullPointerException", "null field")
	}
	f, ok := o.data.(*reflectedField)
	if !ok {
		fail("unbound reflected field")
	}
	if accessFlag {
		allow := integer(args[0]) != 0
		if allow && v.reflectionOpen != nil {
			open, err := v.reflectionOpen(f.owner)
			if err != nil {
				fail("reflection module access: %v", err)
			}
			if !open {
				fail("reflection into an unresolved named module")
			}
		}
		f.accessible = allow
		return nil, true
	}
	if !f.accessible && (f.field.access&1 == 0 || v.load(f.owner).access&1 == 0) {
		fail("nonpublic reflected field access without override")
	}
	if getter && f.field.desc != "Z" {
		raiseJava("java/lang/IllegalArgumentException", "field is not boolean")
	}
	ref := reference{f.owner, f.field.name, f.field.desc}
	box := func(value any) any {
		if !boxed {
			return value
		}
		wrapper := map[string]string{"Z": "Boolean", "B": "Byte", "S": "Short", "C": "Character", "I": "Integer", "J": "Long", "F": "Float", "D": "Double"}[f.field.desc]
		if wrapper != "" {
			return v.alloc("java/lang/"+wrapper, value)
		}
		return value
	}
	if f.field.access&8 != 0 {
		return box(v.static(ref)), true
	}
	if args[0] == nil {
		raiseJava("java/lang/NullPointerException", "null field receiver")
	}
	instance, ok := args[0].(*object)
	if !ok || !v.instanceOf(instance, f.owner) {
		raiseJava("java/lang/IllegalArgumentException", "wrong field receiver")
	}
	if value, ok := instance.fields[fieldKey(ref)]; ok {
		return box(value), true
	}
	return box(defaultValue(f.field.desc)), true
}
