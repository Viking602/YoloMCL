package modjar

import (
	"strings"
)

// Forge's ModFileScanData is the class-annotation index FML builds for each
// mod file. It is pure metadata, so it is rebuilt here from the same class
// files: ModList.getModFileById(id).getFile().getScanResult().getAnnotations().
type modFileHandle struct{ id string }

const (
	scanDataClass       = "net/minecraftforge/forgespi/language/ModFileScanData"
	annotationDataClass = "net/minecraftforge/forgespi/language/ModFileScanData$AnnotationData"
	asmTypeClass        = "org/objectweb/asm/Type"
)

type scanAnnotation struct {
	annotationType, clazz *object
	data                  *object
}

func (h *bindings) scanDataCall(r reference, receiver any, args []any) (any, bool) {
	v := h.vm
	switch {
	case r.owner == "net/minecraftforge/fml/ModList" && r.name == "getModFileById" && len(args) == 1:
		id, ok := args[0].(string)
		if !ok {
			return nil, true
		}
		if _, exists := h.options.Mods[id]; !exists && id != h.options.ModID {
			return nil, true
		}
		return v.alloc("net/minecraftforge/forgespi/language/IModFileInfo", modFileHandle{id}), true
	case r.name == "getFile" && r.desc == "()Lnet/minecraftforge/forgespi/locating/IModFile;":
		if m, ok := objectData(receiver).(modFileHandle); ok {
			return v.alloc("net/minecraftforge/forgespi/locating/IModFile", m), true
		}
	case r.name == "getScanResult" && r.desc == "()Lnet/minecraftforge/forgespi/language/ModFileScanData;":
		if m, ok := objectData(receiver).(modFileHandle); ok {
			return v.alloc(scanDataClass, m), true
		}
	case r.owner == scanDataClass && r.name == "getAnnotations" && r.desc == "()Ljava/util/Set;":
		if m, ok := objectData(receiver).(modFileHandle); ok {
			return v.alloc("java/util/HashSet", &javaSet{values: h.scanAnnotations(m.id), readonly: true}), true
		}
	case r.owner == annotationDataClass:
		a, ok := objectData(receiver).(*scanAnnotation)
		if !ok {
			return nil, false
		}
		switch r.name {
		case "annotationType":
			return a.annotationType, true
		case "clazz":
			return a.clazz, true
		case "annotationData":
			return a.data, true
		case "memberName":
			return "", true
		case "targetType":
			return nil, true
		}
	}
	return nil, false
}

// asmType builds a real org.objectweb.asm.Type through its own bytecode.
func (h *bindings) asmType(descriptor string) *object {
	t, _ := h.vm.method(reference{asmTypeClass, "getType", "(Ljava/lang/String;)Lorg/objectweb/asm/Type;"}, nil, false, []any{descriptor}).(*object)
	return t
}

func (h *bindings) scanAnnotations(id string) []any {
	if h.options.MixinArchives == nil {
		fail("mod file scan data needs the loader classpath")
	}
	archives, err := h.options.MixinArchives()
	if err != nil {
		fail("mod file scan data: %v", err)
	}
	v := h.vm
	cp := &Classpath{ctx: v.ctx}
	var out []any
	for _, a := range archives {
		if !containsString(cp.archiveModIDs(a), id) {
			continue
		}
		for _, f := range a.Reader.File {
			if !strings.HasSuffix(f.Name, ".class") || strings.HasPrefix(f.Name, "META-INF/") {
				continue
			}
			b, err := archiveBytes(v.ctx, f, 1<<20)
			if err != nil || !strings.Contains(string(b), "RuntimeVisibleAnnotations") && !strings.Contains(string(b), "RuntimeInvisibleAnnotations") {
				continue
			}
			name := strings.TrimSuffix(f.Name, ".class")
			c := v.load(name)
			for _, an := range c.annotations {
				out = append(out, v.alloc(annotationDataClass, &scanAnnotation{
					annotationType: h.asmType(an.typeName),
					clazz:          h.asmType("L" + name + ";"),
					data:           h.annotationMap(an),
				}))
			}
		}
		break
	}
	v.reserveCollection(len(out))
	return out
}

// annotationMap mirrors FML's ModAnnotationVisitor: explicit elements only,
// primitives boxed by the element's declared type, arrays as ArrayList.
func (h *bindings) annotationMap(an annotation) *object {
	v := h.vm
	returns := map[string]string{}
	if strings.HasPrefix(an.typeName, "L") {
		func() {
			defer func() { recover() }()
			for _, m := range v.load(strings.TrimSuffix(an.typeName[1:], ";")).methods {
				if i := strings.LastIndexByte(m.desc, ')'); i >= 0 {
					returns[m.name] = m.desc[i+1:]
				}
			}
		}()
	}
	var convert func(x any, desc string) any
	convert = func(x any, desc string) any {
		switch t := x.(type) {
		case []any:
			values := make([]any, len(t))
			for i, e := range t {
				values[i] = convert(e, strings.TrimPrefix(desc, "["))
			}
			return v.alloc("java/util/ArrayList", &javaList{values: values})
		case enumValue:
			return t.name
		case classLiteral:
			d := string(t)
			if !strings.HasPrefix(d, "L") && !strings.HasPrefix(d, "[") && len(d) > 1 {
				d = "L" + d + ";"
			}
			return h.asmType(d)
		case annotation:
			return h.annotationMap(t)
		}
		switch desc {
		case "Z":
			return v.alloc("java/lang/Boolean", x)
		case "I", "B", "C", "S":
			return v.alloc("java/lang/Integer", x)
		case "J":
			return v.alloc("java/lang/Long", x)
		case "F":
			return v.alloc("java/lang/Float", x)
		case "D":
			return v.alloc("java/lang/Double", x)
		}
		return x
	}
	m := &javaMap{}
	for k, x := range an.values {
		m.entries = append(m.entries, javaPair{k, convert(x, returns[k])})
	}
	return v.alloc("java/util/HashMap", m)
}

func containsString(values []string, s string) bool {
	for _, x := range values {
		if x == s {
			return true
		}
	}
	return false
}
