package modjar

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// These operations use only the supplied archive catalogue and bounded VM
// objects. They cannot inspect the host classloader, filesystem, or JVM.
func (h *bindings) metadataCall(r reference, receiver any, args []any) (any, bool) {
	if value, ok := h.reflectionCall(r, receiver, args); ok {
		return value, true
	}
	v := h.vm
	if r.owner == "java/util/logging/Logger" {
		if r.name == "getLogger" && r.desc == "(Ljava/lang/String;)Ljava/util/logging/Logger;" {
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "null logger name")
			}
			name := javaString(args[0])
			return h.symbol("java.util.logging:"+name, r.owner, name), true
		}
		if r.name == "getName" && r.desc == "()Ljava/lang/String;" {
			return objectData(receiver).(string), true
		}
	}
	if r.owner == "java/lang/Class" && r.name == "getSimpleName" && r.desc == "()Ljava/lang/String;" {
		name, ok := receiver.(classLiteral)
		if !ok {
			fail("invalid receiver for %s.%s", r.owner, r.name)
		}
		return simpleClassName(string(name)), true
	}
	if r.owner == "java/lang/Class" && r.name == "isAssignableFrom" && r.desc == "(Ljava/lang/Class;)Z" {
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "null class")
		}
		to, ok1 := receiver.(classLiteral)
		from, ok2 := args[0].(classLiteral)
		if !ok1 || !ok2 {
			fail("invalid receiver for %s.%s", r.owner, r.name)
		}
		if l := h.lambdaClasses[from]; l != nil {
			from = classLiteral(l.iface)
		}
		if to == from {
			return int32(1), true
		}
		if primitiveClassName(string(to)) || primitiveClassName(string(from)) || strings.HasPrefix(string(to), "[") || strings.HasPrefix(string(from), "[") {
			return int32(0), true
		}
		if to == "java/lang/Object" || v.classAssignable(string(from), string(to)) {
			return int32(1), true
		}
		return int32(0), true
	}
	if r.owner == "java/lang/RuntimePermission" && r.name == "<init>" && r.desc == "(Ljava/lang/String;)V" {
		// A permission is only a name; no SecurityManager exists to check it.
		receiver.(*object).data = javaString(args[0])
		return nil, true
	}
	// Reflection into a game class: the game JAR is not loaded, so the member
	// handle is an unavailable game value. Reading through it stays inert and
	// any branch on the result fails as an unavailable game value.
	if (r.owner == "net/minecraftforge/fml/util/ObfuscationReflectionHelper" || r.owner == "net/neoforged/fml/util/ObfuscationReflectionHelper") && receiver == nil && len(args) >= 2 {
		// The event bus is bound natively (listeners are native state), so its
		// internals are as unavailable as game classes.
		if c, ok := args[0].(classLiteral); ok && (h.vm.hostOpaque(string(c)) || nativeEventBusClass(frameworkName(string(c))) || frameworkReflectable(h.vm, string(c), javaString(args[1]))) {
			switch r.name {
			case "findField", "findMethod", "findConstructor":
				return h.vm.inert(string(c)+"."+r.name+" "+javaString(args[1]), true), true
			}
		}
	}
	if r.owner == typeResolverClass && r.name != "resolveRawArgument" {
		fail("unbound %s.%s%s (TypeResolver is natively bound)", r.owner, r.name, r.desc)
	}
	if r.owner == "net/jodah/typetools/TypeResolver" && r.name == "resolveRawArgument" && r.desc == "(Ljava/lang/Class;Ljava/lang/Class;)Ljava/lang/Class;" {
		if l := h.lambdaClasses[lambdaClassArg(args[1])]; l != nil {
			// A lambda's hidden class implements its functional interface with the
			// instantiated method type recorded by LambdaMetafactory.
			if types, _ := descriptor(l.instantiated); len(types) == 1 && strings.HasPrefix(types[0], "L") {
				return classLiteral(strings.TrimSuffix(types[0][1:], ";")), true
			}
			return classLiteral("net/jodah/typetools/TypeResolver$Unknown"), true
		}
		if args[0] == nil || args[1] == nil {
			raiseJava("java/lang/NullPointerException", "resolveRawArgument")
		}
		target, ok1 := args[0].(classLiteral)
		sub, ok2 := args[1].(classLiteral)
		if ok1 && ok2 {
			return v.resolveRawArgument(string(target), string(sub), 0), true
		}
	}
	// Object.getClass is final, so any owner (an interface such as Consumer
	// via invokeinterface) resolves to it rather than the functional method.
	if r.name == "getClass" && r.desc == "()Ljava/lang/Class;" {
		if receiver == nil {
			raiseJava("java/lang/NullPointerException", "null getClass")
		}
		switch x := receiver.(type) {
		case *object:
			return classLiteral(x.class), true
		case string:
			return classLiteral("java/lang/String"), true
		case classLiteral:
			return classLiteral("java/lang/Class"), true
		case *array:
			return classLiteral(x.descriptor), true
		case *lambda:
			return h.lambdaClass(x), true
		default:
			fail("unbound getClass for %T", receiver)
		}
	}
	if r.owner == "java/lang/Class" && (r.name == "getMethods" || r.name == "getDeclaredMethods") && r.desc == "()[Ljava/lang/reflect/Method;" {
		if _, ok := receiver.(classLiteral); !ok {
			fail("invalid receiver for %s.%s", r.owner, r.name)
		}
		return v.objectArray(0, "[Ljava/lang/reflect/Method;"), true
	}
	if r.owner == "java/lang/Class" && r.name == "getFields" && r.desc == "()[Ljava/lang/reflect/Field;" {
		if _, ok := receiver.(classLiteral); !ok {
			fail("invalid receiver for %s.%s", r.owner, r.name)
		}
		return v.objectArray(0, "[Ljava/lang/reflect/Field;"), true
	}
	if r.owner == "java/lang/Class" && r.name == "getGenericSuperclass" && r.desc == "()Ljava/lang/reflect/Type;" {
		if receiver == nil {
			raiseJava("java/lang/NullPointerException", "null class")
		}
		name, ok := receiver.(classLiteral)
		if !ok {
			fail("invalid receiver for %s.%s", r.owner, r.name)
		}
		s := string(name)
		switch s {
		case "java/lang/Object", "boolean", "byte", "char", "short", "int", "long", "float", "double", "void":
			return nil, true
		}
		if strings.HasPrefix(s, "[") {
			return classLiteral("java/lang/Object"), true
		}
		if !v.fetchClass(s) {
			return classLiteral("java/lang/Object"), true
		}
		c := v.load(s)
		if c.signature != "" {
			if t, ok := parseClassSignatureSuper(c.signature); ok {
				return v.allocReflectType(t), true
			}
		}
		if c.super == "" {
			return nil, true
		}
		return classLiteral(c.super), true
	}
	if r.owner == "java/lang/Class" && (r.name == "getEnclosingClass" || r.name == "getDeclaringClass") && r.desc == "()Ljava/lang/Class;" {
		if receiver == nil {
			raiseJava("java/lang/NullPointerException", "null class")
		}
		name, ok := receiver.(classLiteral)
		if !ok {
			fail("invalid receiver for %s.%s", r.owner, r.name)
		}
		s := string(name)
		if i := strings.LastIndex(s, "$"); i > 0 {
			outer := s[:i]
			// Skip numeric anonymous markers when walking to a named enclosing class.
			for outer != "" {
				if !v.fetchClass(outer) {
					break
				}
				c := v.load(outer)
				if c.access&0x1000 == 0 { // not ACC_SYNTHETIC-only probe; accept named
					return classLiteral(outer), true
				}
				if j := strings.LastIndex(outer, "$"); j > 0 {
					outer = outer[:j]
					continue
				}
				break
			}
			return classLiteral(s[:i]), true
		}
		return nil, true
	}
	if r.owner == "java/lang/Class" && r.name == "getModifiers" && r.desc == "()I" {
		if receiver == nil {
			raiseJava("java/lang/NullPointerException", "null class")
		}
		name, ok := receiver.(classLiteral)
		if !ok {
			fail("invalid receiver for %s.%s", r.owner, r.name)
		}
		s := string(name)
		switch s {
		case "boolean", "byte", "char", "short", "int", "long", "float", "double", "void":
			// ACC_PUBLIC | ACC_FINAL | ACC_ABSTRACT
			return int32(0x411), true
		}
		if strings.HasPrefix(s, "[") {
			return int32(0x411), true
		}
		if !v.fetchClass(s) {
			return int32(1), true // ACC_PUBLIC
		}
		return int32(v.load(s).access & 0xffff), true
	}
	if r.owner == "java/lang/Class" && r.name == "getPackage" && r.desc == "()Ljava/lang/Package;" {
		if receiver == nil {
			raiseJava("java/lang/NullPointerException", "null class")
		}
		name, ok := receiver.(classLiteral)
		if !ok {
			fail("invalid receiver for %s.%s", r.owner, r.name)
		}
		s := strings.ReplaceAll(string(name), "/", ".")
		pkg := ""
		if i := strings.LastIndex(s, "."); i >= 0 {
			pkg = s[:i]
		}
		return v.alloc("java/lang/Package", pkg), true
	}
	if r.owner == "java/lang/Package" && receiver != nil {
		switch {
		case r.name == "getName" && r.desc == "()Ljava/lang/String;":
			if s, ok := objectData(receiver).(string); ok {
				return s, true
			}
		case r.name == "toString" && r.desc == "()Ljava/lang/String;":
			if s, ok := objectData(receiver).(string); ok {
				if s == "" {
					return "package ", true
				}
				return "package " + s, true
			}
		}
	}
	if r.owner == "java/lang/ClassLoader" {
		switch {
		case r.name == "getSystemClassLoader" && r.desc == "()Ljava/lang/ClassLoader;":
			return v.alloc("java/lang/ClassLoader", "system"), true
		case r.name == "<init>" && (r.desc == "()V" || r.desc == "(Ljava/lang/ClassLoader;)V"):
			return nil, true
		case r.name == "getResource" && r.desc == "(Ljava/lang/String;)Ljava/net/URL;":
			return nil, true
		case r.name == "getResources" && r.desc == "(Ljava/lang/String;)Ljava/util/Enumeration;":
			return v.alloc("java/util/Enumeration", &javaList{readonly: true}), true
		case r.name == "getParent" && r.desc == "()Ljava/lang/ClassLoader;":
			return nil, true
		case r.name == "loadClass" && r.desc == "(Ljava/lang/String;)Ljava/lang/Class;":
			n := strings.ReplaceAll(javaString(args[0]), ".", "/")
			if !v.fetchClass(n) {
				raiseJava("java/lang/ClassNotFoundException", javaString(args[0]))
			}
			return classLiteral(n), true
		}
	}
	if r.owner == "java/lang/reflect/ParameterizedType" && receiver != nil {
		pt, ok := objectData(receiver).(*javaParameterized)
		if !ok {
			return nil, false
		}
		switch {
		case r.name == "getRawType" && r.desc == "()Ljava/lang/reflect/Type;":
			return pt.raw, true
		case r.name == "getActualTypeArguments" && r.desc == "()[Ljava/lang/reflect/Type;":
			a := &array{values: append([]any(nil), pt.args...), descriptor: "[Ljava/lang/reflect/Type;"}
			return a, true
		case r.name == "getOwnerType" && r.desc == "()Ljava/lang/reflect/Type;":
			return pt.owner, true
		case r.name == "equals" && r.desc == "(Ljava/lang/Object;)Z":
			if receiver == args[0] {
				return int32(1), true
			}
			other, ok := objectData(args[0]).(*javaParameterized)
			if !ok || !reflectTypeEqual(pt.raw, other.raw) || !reflectTypeEqual(pt.owner, other.owner) || len(pt.args) != len(other.args) {
				return int32(0), true
			}
			for i := range pt.args {
				if !reflectTypeEqual(pt.args[i], other.args[i]) {
					return int32(0), true
				}
			}
			return int32(1), true
		case r.name == "hashCode" && r.desc == "()I":
			h := int32(1)
			for _, a := range pt.args {
				h = 31*h + reflectTypeHash(a)
			}
			return h, true
		case r.name == "toString" && r.desc == "()Ljava/lang/String;":
			return reflectTypeString(pt), true
		}
	}
	if r.owner == "java/lang/Class" && r.name == "getSuperclass" && r.desc == "()Ljava/lang/Class;" {
		if receiver == nil {
			raiseJava("java/lang/NullPointerException", "null class")
		}
		name, ok := receiver.(classLiteral)
		if !ok {
			fail("invalid receiver for %s.%s", r.owner, r.name)
		}
		s := string(name)
		if s == "java/lang/Object" || s == "java/lang/Object;" {
			return nil, true
		}
		switch s {
		case "boolean", "byte", "char", "short", "int", "long", "float", "double", "void":
			return nil, true
		}
		if strings.HasPrefix(s, "[") {
			return classLiteral("java/lang/Object"), true
		}
		if !v.fetchClass(s) {
			if v.hostOpaque(s) {
				return classLiteral("java/lang/Object"), true
			}
			return classLiteral("java/lang/Object"), true
		}
		c := v.load(s)
		if c.super == "" {
			return nil, true
		}
		return classLiteral(c.super), true
	}
	if r.owner == "java/lang/Class" && r.name == "getName" && r.desc == "()Ljava/lang/String;" {
		if receiver == nil {
			raiseJava("java/lang/NullPointerException", "null class")
		}
		name, ok := receiver.(classLiteral)
		if !ok {
			fail("unbound class name metadata")
		}
		return strings.ReplaceAll(string(name), "/", "."), true
	}
	if r.owner == "java/lang/Class" && r.name == "toString" && r.desc == "()Ljava/lang/String;" {
		name, ok := receiver.(classLiteral)
		if !ok {
			fail("invalid receiver for %s.%s", r.owner, r.name)
		}
		if l := h.lambdaClasses[name]; l != nil {
			return "class " + strings.ReplaceAll(string(name), "/", "."), true
		}
		switch {
		case primitiveClassName(string(name)):
			return string(name), true
		case strings.HasPrefix(string(name), "["):
			return "class " + strings.ReplaceAll(string(name), "/", "."), true
		case v.fetchClass(string(name)):
			kind := "class "
			if v.load(string(name)).access&0x0200 != 0 {
				kind = "interface "
			}
			return kind + strings.ReplaceAll(string(name), "/", "."), true
		}
		fail("unbound class metadata toString for %s", name)
	}
	if r.owner == "java/lang/Class" && (r.name == "isInterface" || r.name == "isPrimitive" || r.name == "isArray" || r.name == "desiredAssertionStatus") && r.desc == "()Z" {
		if receiver == nil {
			raiseJava("java/lang/NullPointerException", "null class")
		}
		name, ok := receiver.(classLiteral)
		if !ok {
			fail("unbound class metadata %s", r.name)
		}
		switch r.name {
		case "isPrimitive":
			switch string(name) {
			case "boolean", "byte", "char", "short", "int", "long", "float", "double", "void":
				return int32(1), true
			}
			return int32(0), true
		case "isArray":
			if strings.HasPrefix(string(name), "[") {
				return int32(1), true
			}
			return int32(0), true
		case "isInterface":
			if !v.fetchClass(string(name)) {
				return int32(0), true
			}
			c := v.load(string(name))
			if c.access&0x0200 != 0 {
				return int32(1), true
			}
			return int32(0), true
		default: // desiredAssertionStatus
			return int32(0), true
		}
	}
	if r.owner == "com/llamalad7/mixinextras/MixinExtrasBootstrap" && r.name == "init" && r.desc == "()V" {
		// The interpreter dispatches supported injector APIs itself. Unsupported
		// annotations remain unresolved transformations rather than being ignored.
		return nil, true
	}
	if r.owner == "java/lang/Class" && r.name == "forName" && (r.desc == "(Ljava/lang/String;)Ljava/lang/Class;" || r.desc == "(Ljava/lang/String;ZLjava/lang/ClassLoader;)Ljava/lang/Class;") {
		name := strings.ReplaceAll(javaString(args[0]), ".", "/")
		if forbiddenClass(name) {
			fail("forbidden reflected class %s", name)
		}
		switch name {
		case "java/lang/Object", "java/lang/String", "java/lang/Integer", "java/lang/Long", "java/lang/Boolean", "java/lang/Float", "java/lang/Double", "java/lang/Class":
			return classLiteral(name), true
		}
		if h.options.JavaMajor == 0 {
			fail("game Java compatibility is required for class loading")
		}
		if !v.fetchClass(name) {
			// Optional JDK modules (java.sql, java.desktop, ...) are not part of
			// the registration VM; callers probing them see a missing class,
			// which is the documented outcome on a trimmed runtime image.
			if strings.HasPrefix(name, "java/sql/") || strings.HasPrefix(name, "java/awt/") || strings.HasPrefix(name, "javax/") || strings.HasPrefix(name, "java/beans/") {
				raiseJava("java/lang/ClassNotFoundException", name)
			}
			if strings.HasPrefix(name, "java/") || strings.HasPrefix(name, "net/minecraft/") || strings.HasPrefix(name, "net/neoforged/") || strings.HasPrefix(name, "net/minecraftforge/") {
				fail("unbound reflected platform class %s", name)
			}
			raiseJava("java/lang/ClassNotFoundException", name)
		}
		data := v.raw[name]
		if len(data) >= 8 && int(binary.BigEndian.Uint16(data[6:8])) > h.options.JavaMajor+44 {
			raiseJava("java/lang/UnsupportedClassVersionError", name)
		}
		// forName(name, initialize, loader): one classpath, so the loader is
		// irrelevant; initialization runs only when requested.
		if len(args) == 1 || integer(args[1]) != 0 {
			v.initialize(name)
		}
		return classLiteral(name), true
	}
	if r.owner == "java/lang/String" && r.name == "split" && r.desc == "(Ljava/lang/String;)[Ljava/lang/String;" {
		// Java's literal-delimiter fast path; other regex syntax must be supplied
		// by a Java-compatible regex implementation, never Go regexp guesses.
		pattern := javaString(args[0])
		delimiter := ""
		if len(pattern) == 1 && !strings.ContainsAny(pattern, ".$|()[{^?*+\\") {
			delimiter = pattern
		}
		if len(pattern) == 2 && pattern[0] == '\\' && strings.ContainsAny(pattern[1:], ".$|()[{^?*+\\") {
			delimiter = pattern[1:]
		}
		if delimiter == "" {
			fail("unbound Java split pattern %q", pattern)
		}
		s := javaString(receiver)
		parts := strings.Split(s, delimiter)
		if s != "" {
			for len(parts) > 0 && parts[len(parts)-1] == "" {
				parts = parts[:len(parts)-1]
			}
		}
		a := v.objectArray(len(parts), "[Ljava/lang/String;")
		for i, s := range parts {
			a.values[i] = s
		}
		return a, true
	}
	if r.owner == "java/util/Arrays" && r.name == "copyOf" && r.desc == "([Ljava/lang/Object;I)[Ljava/lang/Object;" {
		original := args[0].(*array)
		a := v.objectArray(int(integer(args[1])), original.descriptor)
		copy(a.values, original.values)
		return a, true
	}
	if r.owner == "java/util/Arrays" && (r.name == "copyOf" || r.name == "copyOfRange") && len(r.desc) > 3 && r.desc[1] == '[' && strings.IndexByte("ZBCSIJFD", r.desc[2]) >= 0 {
		// Primitive overloads: the element type is the descriptor's own.
		original, ok := args[0].(*array)
		if !ok {
			raiseJava("java/lang/NullPointerException", r.name)
		}
		from, to := 0, int(integer(args[1]))
		if r.name == "copyOfRange" {
			from, to = int(integer(args[1])), int(integer(args[2]))
			if from < 0 || from > len(original.values) {
				raiseJava("java/lang/ArrayIndexOutOfBoundsException", "copyOfRange")
			}
			if from > to {
				raiseJava("java/lang/IllegalArgumentException", "from > to")
			}
		}
		a := v.objectArray(to-from, r.desc[1:3])
		zero := defaultValue(r.desc[2:3])
		for i := range a.values {
			a.values[i] = zero
		}
		if from < len(original.values) {
			copy(a.values, original.values[from:])
		}
		return a, true
	}
	if r.owner == "java/lang/String" && r.name == "join" && r.desc == "(Ljava/lang/CharSequence;[Ljava/lang/CharSequence;)Ljava/lang/String;" {
		delimiter := javaString(args[0])
		a := args[1].(*array)
		parts := make([]string, len(a.values))
		size := 0
		for i, x := range a.values {
			s := "null"
			if x != nil {
				s = javaString(x)
			}
			parts[i] = s
			size += len(s) + len(delimiter)
			if size > 65536 {
				fail("string join budget")
			}
		}
		return strings.Join(parts, delimiter), true
	}
	return nil, false
}

func (v *machine) objectArray(length int, descriptor string) *array {
	if length < 0 {
		raiseJava("java/lang/NegativeArraySizeException", "negative array length")
	}
	if length > 131072-v.slots {
		fail("array slot budget exceeded")
	}
	v.slots += length
	v.alloc("[", nil)
	return &array{descriptor: descriptor, values: make([]any, length)}
}

// javaParameterized is java.lang.reflect.ParameterizedType from a Signature.
type javaParameterized struct {
	raw   any
	args  []any
	owner any
}

type reflectSig struct {
	kind byte // 'L' class, 'T' var, '[' array, 'P' parameterized (encoded in args)
	name string
	args []reflectSig
}

func parseClassSignatureSuper(sig string) (reflectSig, bool) {
	p := 0
	if p < len(sig) && sig[p] == '<' {
		// Skip formal type parameters.
		depth := 1
		p++
		for p < len(sig) && depth > 0 {
			switch sig[p] {
			case '<':
				depth++
			case '>':
				depth--
			}
			p++
		}
	}
	t, np, ok := parseFieldTypeSig(sig, p)
	if !ok || np > len(sig) {
		return reflectSig{}, false
	}
	return t, true
}

func parseFieldTypeSig(sig string, p int) (reflectSig, int, bool) {
	if p >= len(sig) {
		return reflectSig{}, p, false
	}
	switch sig[p] {
	case 'L':
		return parseClassTypeSig(sig, p)
	case 'T':
		p++
		start := p
		for p < len(sig) && sig[p] != ';' {
			p++
		}
		if p >= len(sig) {
			return reflectSig{}, p, false
		}
		return reflectSig{kind: 'T', name: sig[start:p]}, p + 1, true
	case '[':
		elem, np, ok := parseFieldTypeSig(sig, p+1)
		if !ok {
			return reflectSig{}, p, false
		}
		return reflectSig{kind: '[', args: []reflectSig{elem}}, np, true
	case 'B', 'C', 'D', 'F', 'I', 'J', 'S', 'Z':
		return reflectSig{kind: sig[p]}, p + 1, true
	default:
		return reflectSig{}, p, false
	}
}

func parseClassTypeSig(sig string, p int) (reflectSig, int, bool) {
	if p >= len(sig) || sig[p] != 'L' {
		return reflectSig{}, p, false
	}
	p++
	start := p
	for p < len(sig) && sig[p] != '<' && sig[p] != ';' && sig[p] != '.' {
		p++
	}
	if p >= len(sig) {
		return reflectSig{}, p, false
	}
	name := sig[start:p]
	var args []reflectSig
	if sig[p] == '<' {
		p++
		for p < len(sig) && sig[p] != '>' {
			if sig[p] == '*' {
				args = append(args, reflectSig{kind: '*'})
				p++
				continue
			}
			if sig[p] == '+' || sig[p] == '-' {
				p++
			}
			a, np, ok := parseFieldTypeSig(sig, p)
			if !ok {
				return reflectSig{}, p, false
			}
			args = append(args, a)
			p = np
		}
		if p >= len(sig) || sig[p] != '>' {
			return reflectSig{}, p, false
		}
		p++
	}
	// Nested .Inner ? fold into binary name.
	for p < len(sig) && sig[p] == '.' {
		p++
		start = p
		for p < len(sig) && sig[p] != '<' && sig[p] != ';' && sig[p] != '.' {
			p++
		}
		name += "$" + sig[start:p]
		if p < len(sig) && sig[p] == '<' {
			// Rare nested type args: parse and ignore for owner chain simplicity
			// by appending onto args of the innermost.
			inner, np, ok := parseClassTypeSig("L"+sig[start:], 0)
			_ = inner
			if !ok {
				return reflectSig{}, p, false
			}
			p = start + (np - 1)
		}
	}
	if p >= len(sig) || sig[p] != ';' {
		return reflectSig{}, p, false
	}
	t := reflectSig{kind: 'L', name: name, args: args}
	return t, p + 1, true
}

func (v *machine) allocReflectType(t reflectSig) any {
	switch t.kind {
	case 'L':
		raw := classLiteral(t.name)
		if len(t.args) == 0 {
			return raw
		}
		args := make([]any, len(t.args))
		for i, a := range t.args {
			args[i] = v.allocReflectType(a)
		}
		return v.alloc("java/lang/reflect/ParameterizedType", &javaParameterized{raw: raw, args: args})
	case '[':
		// Represent as GenericArrayType only when needed; Class form is enough for BOP.
		if len(t.args) == 1 {
			elem := v.allocReflectType(t.args[0])
			if cl, ok := elem.(classLiteral); ok {
				return classLiteral("[" + "L" + string(cl) + ";")
			}
		}
		fail("unbound generic array signature")
	case 'T', '*':
		return classLiteral("java/lang/Object")
	case 'B':
		return classLiteral("byte")
	case 'C':
		return classLiteral("char")
	case 'D':
		return classLiteral("double")
	case 'F':
		return classLiteral("float")
	case 'I':
		return classLiteral("int")
	case 'J':
		return classLiteral("long")
	case 'S':
		return classLiteral("short")
	case 'Z':
		return classLiteral("boolean")
	default:
		fail("unbound signature kind %q", t.kind)
	}
	return nil
}

func reflectTypeEqual(a, b any) bool {
	if a == nil || b == nil {
		return a == b
	}
	if ca, ok := a.(classLiteral); ok {
		cb, ok := b.(classLiteral)
		return ok && ca == cb
	}
	pa, ok := objectData(a).(*javaParameterized)
	if !ok {
		return false
	}
	pb, ok := objectData(b).(*javaParameterized)
	if !ok || !reflectTypeEqual(pa.raw, pb.raw) || !reflectTypeEqual(pa.owner, pb.owner) || len(pa.args) != len(pb.args) {
		return false
	}
	for i := range pa.args {
		if !reflectTypeEqual(pa.args[i], pb.args[i]) {
			return false
		}
	}
	return true
}

func reflectTypeHash(t any) int32 {
	if t == nil {
		return 0
	}
	if c, ok := t.(classLiteral); ok {
		h := int32(0)
		for _, r := range string(c) {
			h = 31*h + int32(r)
		}
		return h
	}
	if p, ok := objectData(t).(*javaParameterized); ok {
		h := reflectTypeHash(p.raw)
		for _, a := range p.args {
			h = 31*h + reflectTypeHash(a)
		}
		return h
	}
	return 1
}

func reflectTypeString(p *javaParameterized) string {
	s := ""
	if c, ok := p.raw.(classLiteral); ok {
		s = strings.ReplaceAll(string(c), "/", ".")
	} else {
		s = "?"
	}
	if len(p.args) == 0 {
		return s
	}
	s += "<"
	for i, a := range p.args {
		if i > 0 {
			s += ", "
		}
		if c, ok := a.(classLiteral); ok {
			s += strings.ReplaceAll(string(c), "/", ".")
		} else if q, ok := objectData(a).(*javaParameterized); ok {
			s += reflectTypeString(q)
		} else {
			s += "?"
		}
	}
	return s + ">"
}

// lambdaClass names a lambda's hidden class; equal lambdas share no class.
func (h *bindings) lambdaClass(l *lambda) classLiteral {
	for name, x := range h.lambdaClasses {
		if x == l {
			return name
		}
	}
	if h.lambdaClasses == nil {
		h.lambdaClasses = map[classLiteral]*lambda{}
	}
	if len(h.lambdaClasses) >= 4096 {
		fail("lambda class budget exceeded")
	}
	name := classLiteral(fmt.Sprintf("%s$$Lambda/%d", l.iface, len(h.lambdaClasses)))
	h.lambdaClasses[name] = l
	return name
}

func lambdaClassArg(x any) classLiteral {
	c, _ := x.(classLiteral)
	return c
}

func primitiveClassName(s string) bool {
	switch s {
	case "boolean", "byte", "char", "short", "int", "long", "float", "double", "void":
		return true
	}
	return false
}

// simpleClassName mirrors Class.getSimpleName for binary names: arrays append
// "[]", nested classes drop their enclosing prefix, anonymous classes are "".
func simpleClassName(name string) string {
	if strings.HasPrefix(name, "[") {
		elem := strings.TrimPrefix(name, "[")
		switch {
		case strings.HasPrefix(elem, "L") && strings.HasSuffix(elem, ";"):
			elem = elem[1 : len(elem)-1]
		case strings.HasPrefix(elem, "["):
		default:
			elem = map[string]string{"Z": "boolean", "B": "byte", "C": "char", "S": "short", "I": "int", "J": "long", "F": "float", "D": "double"}[elem]
		}
		return simpleClassName(elem) + "[]"
	}
	name = name[strings.LastIndex(name, "/")+1:]
	if i := strings.LastIndex(name, "$"); i >= 0 {
		name = name[i+1:]
		if name != "" && strings.Trim(name, "0123456789") == "" {
			return ""
		}
	}
	return name
}

const typeResolverClass = "net/jodah/typetools/TypeResolver"

// resolveRawArgument mirrors TypeResolver.resolveRawArgument for a class: the
// single type argument `sub` (or a supertype) supplies to generic `target`.
// Type variables are resolved through the subclass's supertype arguments;
// anything unresolved is TypeResolver.Unknown, as in the library.
func (v *machine) resolveRawArgument(target, sub string, depth int) classLiteral {
	unknown := classLiteral("net/jodah/typetools/TypeResolver$Unknown")
	if depth > 32 || sub == "" || sub == "java/lang/Object" || !v.fetchClass(sub) {
		return unknown
	}
	c := v.load(sub)
	var supers []reflectSig
	if c.signature != "" {
		sig, p := c.signature, 0
		if sig[0] == '<' {
			for d := 0; p < len(sig); {
				if sig[p] == '<' {
					d++
				} else if sig[p] == '>' {
					d--
					if d == 0 {
						p++
						break
					}
				}
				p++
			}
		}
		for p < len(sig) {
			t, np, ok := parseFieldTypeSig(sig, p)
			if !ok {
				return unknown
			}
			supers, p = append(supers, t), np
		}
	} else {
		supers = append(supers, reflectSig{kind: 'L', name: c.super})
		for _, i := range c.interfaces {
			supers = append(supers, reflectSig{kind: 'L', name: i})
		}
	}
	for _, s := range supers {
		if s.name == target {
			if len(s.args) == 1 && s.args[0].kind == 'L' {
				return classLiteral(s.args[0].name)
			}
			return unknown
		}
	}
	for _, s := range supers {
		if s.kind == 'L' && s.name != "" {
			if x := v.resolveRawArgument(target, s.name, depth+1); x != unknown {
				// Only concrete arguments survive; a variable bound by this
				// level would need its own actual argument here.
				return x
			}
		}
	}
	return unknown
}

// discardStream is the data of the System.out/err PrintStream the VM hands
// out; writes are dropped.
type discardStream string

func nativeEventBusClass(name string) bool {
	return strings.HasPrefix(name, "net/minecraftforge/eventbus/") || strings.HasPrefix(name, "net/neoforged/bus/")
}

// frameworkReflectable reports a loader-framework class (Forge/NeoForge
// registry API, e.g. RegistryObject.empty looked up by Registrate) declaring
// the named member. The interpreter does not model framework reflection
// handles, so the member is an unavailable handle rather than an
// UnableToFind*Exception the real loader never throws.
func frameworkReflectable(v *machine, class, name string) bool {
	if !strings.HasPrefix(class, "net/minecraftforge/") && !strings.HasPrefix(class, "net/neoforged/") {
		return false
	}
	if !v.fetchClass(class) {
		return true
	}
	for _, m := range v.load(class).methods {
		if m.name == name {
			return true
		}
	}
	return false
}
