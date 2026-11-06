package modjar

// Runtime annotation reflection over the supplied class files. Only
// RuntimeVisibleAnnotations are reported, as java.lang.reflect does; element
// values come from the class file and AnnotationDefault attributes.

// annotationInstance backs the proxy returned by getAnnotation.
type annotationInstance struct {
	annotation annotation
	typeName   string // internal name of the annotation interface
}

func markInvisible(list []annotation, attribute string) []annotation {
	if attribute == "RuntimeInvisibleAnnotations" {
		for i := range list {
			list[i].invisible = true
		}
	}
	return list
}

func runtimeAnnotation(list []annotation, internal string) (annotation, bool) {
	want := "L" + internal + ";"
	for _, a := range list {
		if !a.invisible && (a.typeName == want || frameworkName(a.typeName) == frameworkName(want)) {
			return a, true
		}
	}
	return annotation{}, false
}

func (h *bindings) annotationCall(r reference, receiver any, args []any) (any, bool) {
	v := h.vm
	// Proxy element accessors: any no-arg call on an annotation proxy.
	if o, ok := receiver.(*object); ok && o != nil && len(args) == 0 {
		if inst, ok := o.data.(*annotationInstance); ok {
			switch r.name {
			case "annotationType":
				return classLiteral(inst.typeName), true
			case "hashCode", "toString", "getClass":
				return nil, false
			}
			return h.annotationElement(inst, r), true
		}
	}
	present := r.name == "isAnnotationPresent" && r.desc == "(Ljava/lang/Class;)Z"
	get := (r.name == "getAnnotation" || r.name == "getDeclaredAnnotation") && r.desc == "(Ljava/lang/Class;)Ljava/lang/annotation/Annotation;"
	if !present && !get {
		return nil, false
	}
	var list []annotation
	switch r.owner {
	case "java/lang/Class":
		c, ok := receiver.(classLiteral)
		if !ok {
			return nil, false
		}
		s := string(c)
		if len(s) > 0 && s[0] != '[' && !primitiveClassName(s) && v.fetchClass(s) {
			list = v.load(s).annotations
		}
	case "java/lang/reflect/Field":
		f, ok := objectData(receiver).(*reflectedField)
		if !ok {
			return nil, false
		}
		list = f.field.annotations
	default:
		return nil, false
	}
	if args[0] == nil {
		raiseJava("java/lang/NullPointerException", "null annotation class")
	}
	want, ok := args[0].(classLiteral)
	if !ok {
		fail("invalid annotation class for %s.%s", r.owner, r.name)
	}
	a, found := runtimeAnnotation(list, string(want))
	if present {
		if found {
			return int32(1), true
		}
		return int32(0), true
	}
	if !found {
		return nil, true
	}
	return v.alloc(string(want), &annotationInstance{annotation: a, typeName: string(want)}), true
}

func (h *bindings) annotationElement(inst *annotationInstance, r reference) any {
	v := h.vm
	value, ok := inst.annotation.values[r.name]
	if !ok {
		if v.fetchClass(inst.typeName) {
			for _, m := range v.load(inst.typeName).methods {
				if m.name == r.name && m.hasDefault {
					value, ok = m.annotationDefault, true
					break
				}
			}
		}
	}
	if !ok {
		fail("annotation element %s.%s has no value", inst.typeName, r.name)
	}
	_, ret := descriptor(r.desc)
	return h.annotationValue(value, ret)
}

func (h *bindings) annotationValue(value any, desc string) any {
	v := h.vm
	switch x := value.(type) {
	case classLiteral:
		return descClassLiteral(string(x))
	case enumValue:
		owner := x.typeName[1 : len(x.typeName)-1]
		return v.static(reference{owner, x.name, x.typeName})
	case annotation:
		internal := x.typeName[1 : len(x.typeName)-1]
		return v.alloc(internal, &annotationInstance{annotation: x, typeName: internal})
	case []any:
		if len(desc) < 2 || desc[0] != '[' || desc[1] != 'L' {
			fail("unsupported primitive annotation array %s", desc)
		}
		a := v.objectArray(len(x), desc)
		for i := range x {
			a.values[i] = h.annotationValue(x[i], desc[1:])
		}
		return a
	}
	return value
}

// fieldSetCall implements Field.set for fields declared by supplied classes.
func (h *bindings) fieldSetCall(r reference, receiver any, args []any) (any, bool) {
	if r.owner != "java/lang/reflect/Field" || r.name != "set" || r.desc != "(Ljava/lang/Object;Ljava/lang/Object;)V" {
		return nil, false
	}
	v := h.vm
	f, ok := objectData(receiver).(*reflectedField)
	if !ok {
		fail("unbound reflected field")
	}
	if !f.accessible && (f.field.access&1 == 0 || v.load(f.owner).access&1 == 0) {
		fail("nonpublic reflected field access without override")
	}
	if f.field.access&0x10 != 0 && (f.field.access&8 != 0 || !f.accessible) {
		raiseJava("java/lang/IllegalAccessException", "final field "+f.field.name)
	}
	value := args[1]
	if len(f.field.desc) == 1 {
		if value == nil {
			raiseJava("java/lang/IllegalArgumentException", "null primitive")
		}
		value = unbox(value)
	}
	ref := reference{f.owner, f.field.name, f.field.desc}
	if f.field.access&8 != 0 {
		v.initialize(f.owner)
		if v.statics[f.owner] == nil {
			fail("missing static field")
		}
		v.statics[f.owner][fieldKey(ref)] = value
		return nil, true
	}
	if args[0] == nil {
		raiseJava("java/lang/NullPointerException", "null field receiver")
	}
	instance, ok := args[0].(*object)
	if !ok || !v.instanceOf(instance, f.owner) {
		raiseJava("java/lang/IllegalArgumentException", "wrong field receiver")
	}
	if instance.fields == nil {
		instance.fields = map[string]any{}
	}
	instance.fields[fieldKey(ref)] = value
	return nil, true
}
