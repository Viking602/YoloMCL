package modjar

// Annotation values are metadata, not executable discovery hints. The loader
// uses the same mod ID, bus and entrypoint declarations as Forge/Fabric do.
type annotation struct {
	typeName string
	values   map[string]any
	// invisible marks RuntimeInvisibleAnnotations (CLASS retention), which
	// Java reflection does not report.
	invisible bool
}

type enumValue struct{ typeName, name string }

func (c *class) readAnnotations(data []byte) []annotation {
	list, _ := c.annotationData(data, false)
	return list
}

// readAnnotationDefault decodes a method's AnnotationDefault element_value.
func (c *class) readAnnotationDefault(data []byte) any {
	_, value := c.annotationData(data, true)
	return value
}

func (c *class) annotationData(data []byte, element bool) ([]annotation, any) {
	r := reader{b: data}
	budget := 8192
	var value func(int) any
	read := func(depth int) annotation {
		if depth > 32 || budget <= 0 {
			fail("annotation nesting or value budget exceeded")
		}
		budget--
		a := annotation{typeName: c.descriptor(r.u2()), values: map[string]any{}}
		for n := r.u2(); n > 0; n-- {
			key := c.text(r.u2())
			if _, exists := a.values[key]; exists {
				fail("duplicate annotation element")
			}
			a.values[key] = value(depth + 1)
		}
		return a
	}
	value = func(depth int) any {
		if depth > 32 || budget <= 0 {
			fail("annotation nesting or value budget exceeded")
		}
		budget--
		switch r.u1() {
		case 'B', 'C', 'D', 'F', 'I', 'J', 'S', 'Z':
			return c.literal(r.u2())
		case 's':
			return c.text(r.u2())
		case 'e':
			typ, name := c.descriptor(r.u2()), c.text(r.u2())
			if len(typ) < 3 || typ[0] != 'L' || typ[len(typ)-1] != ';' {
				fail("invalid enum annotation descriptor")
			}
			owner := typ[1 : len(typ)-1]
			return enumValue{typ, c.symbols.ref(reference{owner, name, typ}).name}
		case 'c':
			return classLiteral(c.descriptor(r.u2()))
		case '@':
			return read(depth + 1)
		case '[':
			n := r.u2()
			if n > budget {
				fail("annotation array budget exceeded")
			}
			values := make([]any, n)
			for i := range values {
				values[i] = value(depth + 1)
			}
			return values
		default:
			fail("invalid annotation value")
			return nil
		}
	}
	var result []annotation
	var single any
	if element {
		single = value(0)
	} else {
		for n := r.u2(); n > 0; n-- {
			result = append(result, read(0))
		}
	}
	if r.p != len(data) {
		fail("trailing annotation data")
	}
	return result, single
}

func findAnnotation(values []annotation, name string) (annotation, bool) {
	for _, a := range values {
		if frameworkName(a.typeName) == name {
			return a, true
		}
	}
	return annotation{}, false
}
