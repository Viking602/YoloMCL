package modjar

import "strings"

const forgeEventClass = "net/minecraftforge/eventbus/api/Event"

type reflectedConstructor struct {
	owner, desc string
	access      int
	accessible  bool
	// superInit: Forge EventBus 6's EventSubclassTransformer gives every
	// Event subclass without <init>()V a public one that calls super();
	// newInstance runs that synthetic body.
	superInit string
}

// Reflection dispatch is restricted to class files supplied to this Go VM.
// It never invokes the host's constructors, methods, runtime or filesystem.
func (h *bindings) reflectionCall(r reference, receiver any, args []any) (any, bool) {
	if x, ok := h.fieldSetCall(r, receiver, args); ok {
		return x, true
	}
	if x, ok := h.annotationCall(r, receiver, args); ok {
		return x, true
	}
	if x, ok := h.fieldMetadataCall(r, receiver, args); ok {
		return x, true
	}
	v := h.vm
	if r.owner == "java/lang/reflect/Modifier" && receiver == nil && r.desc == "(I)Z" {
		m := integer(args[0])
		bit := map[string]int32{
			"isPublic": 0x1, "isPrivate": 0x2, "isProtected": 0x4, "isStatic": 0x8,
			"isFinal": 0x10, "isSynchronized": 0x20, "isVolatile": 0x40, "isTransient": 0x80,
			"isNative": 0x100, "isInterface": 0x200, "isAbstract": 0x400, "isStrict": 0x800,
		}[r.name]
		if bit == 0 && r.name != "isPublic" {
			return nil, false
		}
		if m&bit != 0 {
			return int32(1), true
		}
		return int32(0), true
	}
	if r.owner == "java/lang/Class" && (r.name == "getConstructor" || r.name == "getDeclaredConstructor") && r.desc == "([Ljava/lang/Class;)Ljava/lang/reflect/Constructor;" {
		owner, ok := receiver.(classLiteral)
		if !ok {
			fail("unbound constructor class metadata")
		}
		c := v.load(string(owner))
		var params []any
		if args[0] != nil {
			params = args[0].(*array).values
		}
		if len(params) > 255 {
			fail("constructor parameter budget")
		}
		var desc strings.Builder
		desc.WriteByte('(')
		for _, p := range params {
			v.tick()
			if p == nil {
				raiseJava("java/lang/NoSuchMethodException", "null constructor parameter type")
			}
			name, ok := p.(classLiteral)
			if !ok {
				fail("invalid constructor parameter metadata")
			}
			if desc.Len()+len(name)+5 > 65535 {
				fail("constructor descriptor limit")
			}
			if strings.HasPrefix(string(name), "[") {
				desc.WriteString(string(name))
			} else {
				desc.WriteByte('L')
				desc.WriteString(string(name))
				desc.WriteByte(';')
			}
		}
		desc.WriteString(")V")
		for _, m := range c.methods {
			v.tick()
			if m.name == "<init>" && m.desc == desc.String() && (r.name == "getDeclaredConstructor" || m.access&1 != 0) {
				return v.alloc("java/lang/reflect/Constructor", &reflectedConstructor{owner: string(owner), desc: m.desc, access: m.access}), true
			}
		}
		if desc.String() == "()V" && c.name != forgeEventClass && v.classAssignable(c.name, forgeEventClass) {
			return v.alloc("java/lang/reflect/Constructor", &reflectedConstructor{owner: string(owner), desc: "()V", access: 1, superInit: c.super}), true
		}
		raiseJava("java/lang/NoSuchMethodException", "constructor not declared")
	}
	if r.owner == "java/lang/Class" && (r.name == "getConstructors" || r.name == "getDeclaredConstructors") && r.desc == "()[Ljava/lang/reflect/Constructor;" {
		owner, ok := receiver.(classLiteral)
		if !ok {
			fail("invalid receiver for %s.%s", r.owner, r.name)
		}
		var out []any
		if s := string(owner); !strings.HasPrefix(s, "[") && v.fetchClass(s) {
			c := v.load(s)
			for _, m := range c.methods {
				v.tick()
				if m.name == "<init>" && (r.name == "getDeclaredConstructors" || m.access&1 != 0) {
					out = append(out, v.alloc("java/lang/reflect/Constructor", &reflectedConstructor{owner: s, desc: m.desc, access: m.access}))
				}
			}
		}
		a := v.objectArray(len(out), "[Ljava/lang/reflect/Constructor;")
		copy(a.values, out)
		return a, true
	}
	if r.owner == "java/lang/reflect/Constructor" && (r.name == "getParameterTypes" && r.desc == "()[Ljava/lang/Class;" || r.name == "getParameterCount" && r.desc == "()I" || r.name == "getDeclaringClass" && r.desc == "()Ljava/lang/Class;" || r.name == "getModifiers" && r.desc == "()I") {
		c, ok := objectData(receiver).(*reflectedConstructor)
		if !ok {
			fail("invalid constructor metadata")
		}
		types, _ := descriptor(c.desc)
		switch r.name {
		case "getParameterCount":
			return int32(len(types)), true
		case "getDeclaringClass":
			return classLiteral(c.owner), true
		case "getModifiers":
			return int32(c.access & 0xffff), true
		}
		a := v.objectArray(len(types), "[Ljava/lang/Class;")
		for i, t := range types {
			a.values[i] = descClassLiteral(t)
		}
		return a, true
	}
	if r.owner != "java/lang/reflect/Constructor" {
		return nil, false
	}
	create := r.name == "newInstance" && r.desc == "([Ljava/lang/Object;)Ljava/lang/Object;"
	access := r.name == "setAccessible" && r.desc == "(Z)V"
	if !create && !access {
		return nil, false
	}
	if receiver == nil {
		raiseJava("java/lang/NullPointerException", "null constructor")
	}
	c, ok := objectData(receiver).(*reflectedConstructor)
	if !ok {
		fail("invalid constructor metadata")
	}
	if access {
		allow := integer(args[0]) != 0
		if allow {
			v.requireReflectionModule(c.owner)
		}
		c.accessible = allow
		return nil, true
	}
	owner := v.load(c.owner)
	if !c.accessible && (c.access&1 == 0 || owner.access&1 == 0) {
		fail("nonpublic constructor requires caller access or explicit override")
	}
	v.requireReflectionModule(c.owner)
	if owner.access&0x400 != 0 {
		raiseJava("java/lang/InstantiationException", "abstract constructor")
	}
	if owner.access&0x4000 != 0 {
		raiseJava("java/lang/IllegalArgumentException", "enum constructor")
	}
	var values []any
	if args[0] != nil {
		values = args[0].(*array).values
	}
	types, _ := descriptor(c.desc)
	if len(values) != len(types) {
		raiseJava("java/lang/IllegalArgumentException", "constructor argument count")
	}
	for i, typ := range types {
		if len(typ) == 1 {
			fail("reflected primitive constructor conversion is not bound")
		}
		if values[i] == nil {
			continue
		}
		target := typ
		if typ[0] == 'L' {
			target = typ[1 : len(typ)-1]
		}
		if !v.instanceOf(values[i], target) {
			raiseJava("java/lang/IllegalArgumentException", "constructor argument type")
		}
	}
	v.initialize(c.owner)
	return v.invokeConstructor(c, values), true
}

func (v *machine) requireReflectionModule(owner string) {
	if v.reflectionOpen != nil {
		open, err := v.reflectionOpen(owner)
		if err != nil {
			fail("reflection module access: %v", err)
		}
		if !open {
			fail("reflection into an unresolved named module")
		}
	}
}

func (v *machine) invokeConstructor(c *reflectedConstructor, args []any) (value any) {
	defer func() {
		if p := recover(); p != nil {
			if e, ok := p.(javaException); ok {
				panic(javaException{class: "java/lang/reflect/InvocationTargetException", cause: &e})
			}
			panic(p)
		}
	}()
	o := v.alloc(c.owner, nil)
	if c.superInit != "" {
		v.method(reference{c.superInit, "<init>", "()V"}, o, false, nil)
		return o
	}
	v.method(reference{c.owner, "<init>", c.desc}, o, false, args)
	return o
}

// descClassLiteral is the Class a field descriptor names.
func descClassLiteral(t string) classLiteral {
	switch t {
	case "Z":
		return "boolean"
	case "B":
		return "byte"
	case "C":
		return "char"
	case "S":
		return "short"
	case "I":
		return "int"
	case "J":
		return "long"
	case "F":
		return "float"
	case "D":
		return "double"
	case "V":
		return "void"
	}
	if strings.HasPrefix(t, "L") {
		return classLiteral(t[1 : len(t)-1])
	}
	return classLiteral(t)
}
