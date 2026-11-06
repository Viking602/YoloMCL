package modjar

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"os"
	"slices"
	"strings"
)

type object struct {
	class  string
	fields map[string]any
	data   any
}
type lambda struct {
	target       reference
	kind         int
	captured     []any
	sam          string
	instantiated string
	iface        string  // functional interface named by the invokedynamic call site
	factory      *lambda // LambdaMetafactory call-site target: invoking it binds captures into a copy
}
type array struct {
	values     []any
	descriptor string
}
type machine struct {
	systemProperties             *object // lazily materialised System.getProperties view
	failedInitializers           map[string]bool
	abortedInitializers          map[string]any
	inspectSource                func(string) ([]byte, error)
	resolveClass                 func(string) ([]byte, error)
	reflectionOpen               func(string) (bool, error)
	missing                      map[string]bool
	classBytes                   int
	ctx                          context.Context
	raw                          map[string][]byte
	classes                      map[string]*class
	statics                      map[string]map[string]any
	initialized                  map[string]bool
	steps, depth, objects, slots int
	host                         *bindings
	inertOrigins                 map[string]int
	weave                        *weaver
	frames                       []string // class of each executing frame, innermost last
}

func newMachine(ctx context.Context, raw map[string][]byte) *machine {
	if raw == nil {
		raw = map[string][]byte{}
	}
	v := &machine{ctx: ctx, raw: raw, classes: map[string]*class{}, statics: map[string]map[string]any{}, initialized: map[string]bool{}, missing: map[string]bool{}}
	for _, data := range raw {
		v.classBytes += len(data)
	}
	return v
}

func (v *machine) fetchClass(name string) bool {
	if v.raw[name] != nil {
		return true
	}
	if v.resolveClass == nil || v.missing[name] || forbiddenClass(name) {
		return false
	}
	if len(v.missing) >= 4096 {
		fail("class resolution budget exceeded")
	}
	data, err := v.resolveClass(name)
	if err != nil {
		fail("class dependency %s: %v", name, err)
	}
	if data == nil {
		v.missing[name] = true
		return false
	}
	if len(data) > 2<<20 || v.classBytes+len(data) > 16<<20 || len(v.raw) >= 4096 {
		fail("registration dependency byte/class limit")
	}
	v.raw[name] = data
	v.classBytes += len(data)
	return true
}

// Inspect metadata without retaining unrelated classes in the execution set.
// Only archive-indexed names are supplied by entrypoint discovery.
func (v *machine) inspectClass(name string) []byte {
	if data := v.raw[name]; data != nil {
		return data
	}
	read := v.inspectSource
	if read == nil {
		read = v.resolveClass
	}
	if read == nil {
		return nil
	}
	data, err := read(name)
	if err != nil {
		fail("class metadata %s: %v", name, err)
	}
	return data
}
func (v *machine) tick() {
	v.steps++
	if v.steps > 20_000_000 {
		fail("registration instruction budget exceeded")
	}
	if v.steps&255 == 0 && v.ctx.Err() != nil {
		fail("%v", v.ctx.Err())
	}
}
func (v *machine) alloc(name string, data any) *object {
	v.objects++
	if v.objects > 262144 {
		fail("registration object budget exceeded")
	}
	return &object{class: name, fields: map[string]any{}, data: data}
}
func forbiddenClass(name string) bool {
	for _, p := range []string{"java/io/", "java/nio/", "java/net/", "java/lang/reflect/", "sun/", "jdk/"} {
		if strings.HasPrefix(name, p) {
			// java.io.File is only a path token and the in-memory reader chain only
			// wraps classpath resource bytes; file streams and channels stay forbidden.
			if name == "java/io/File" || name == "java/io/ByteArrayInputStream" || name == "java/io/InputStreamReader" || name == "java/io/BufferedReader" || name == "java/io/StringReader" || name == "java/io/FileInputStream" || name == "java/io/FileReader" || name == "java/io/FileWriter" || name == "java/io/OutputStreamWriter" || name == "java/io/BufferedWriter" || name == "java/io/StringWriter" || name == "java/io/FileOutputStream" || strings.HasPrefix(name, "java/io/") && (strings.HasSuffix(name, "Exception") || strings.HasSuffix(name, "Error")) {
				continue
			}
			if name == "java/lang/reflect/ParameterizedType" || name == "java/lang/reflect/Type" || name == "java/lang/reflect/GenericArrayType" || name == "java/lang/reflect/WildcardType" || name == "java/lang/reflect/TypeVariable" {
				continue
			}
			return true
		}
	}
	for _, class := range []string{"java/lang/Runtime", "java/lang/Process", "java/lang/ProcessBuilder", "java/lang/ProcessHandle", "java/lang/ProcessEnvironment", "java/lang/ProcessImpl", "java/lang/System"} {
		if name == class || strings.HasPrefix(name, class+"$") {
			return true
		}
	}
	return false
}
func (v *machine) load(name string) *class {
	if forbiddenClass(name) {
		fail("forbidden registration API %s", name)
	}
	if c := v.classes[name]; c != nil {
		return c
	}
	if !v.fetchClass(name) {
		fail("unbound registration class %s", name)
	}
	b := v.raw[name]
	if v.host != nil && v.host.options.JavaMajor > 0 && len(b) >= 8 && int(binary.BigEndian.Uint16(b[6:8])) > v.host.options.JavaMajor+44 {
		raiseJava("java/lang/UnsupportedClassVersionError", name)
	}
	if len(v.classes) >= 1024 {
		fail("registration class budget exceeded")
	}
	var symbols *Symbols
	if v.host != nil {
		symbols = v.host.options.Symbols
	}
	c := parseClass(b, symbols)
	if c.name != name {
		fail("class entry/name mismatch")
	}
	if v.weave != nil && len(v.weave.targets[name]) > 0 {
		v.weaveClass(c)
	}
	v.classes[name] = c
	return c
}
func defaultValue(desc string) any {
	switch desc {
	case "Z", "B", "C", "S", "I":
		return int32(0)
	case "J":
		return int64(0)
	case "F":
		return float32(0)
	case "D":
		return float64(0)
	default:
		return nil
	}
}
func fieldKey(r reference) string { return r.owner + "." + r.name + ":" + r.desc }
func (v *machine) initialize(name string) {
	if p := v.abortedInitializers[name]; p != nil {
		panic(p)
	}
	if v.failedInitializers[name] {
		raiseJava("java/lang/NoClassDefFoundError", name)
	}
	if v.initialized[name] {
		return
	}
	if name == typeResolverClass {
		// Natively bound (java_metadata.go); its <clinit> only prepares
		// sun.misc.Unsafe reflection for the interpreted implementation.
		if v.initialized == nil {
			v.initialized = map[string]bool{}
		}
		v.initialized[name] = true
		return
	}
	c := v.load(name)
	defer func() {
		if p := recover(); p != nil {
			if e, ok := p.(javaException); ok {
				if v.failedInitializers == nil {
					v.failedInitializers = map[string]bool{}
				}
				v.failedInitializers[name] = true
				if os.Getenv("MODJAR_DEBUG_INIT") != "" {
					fmt.Fprintf(os.Stderr, "modjar: <clinit> %s failed: %s: %s\n", name, e.class, e.message)
				}
				if !v.exceptionAssignable(e, "java/lang/Error") {
					panic(javaException{class: "java/lang/ExceptionInInitializerError", message: name, cause: &e})
				}
			} else {
				// An unavailable host API is not a Java initialization failure.
				// Preserve it across source requests; partially written statics
				// must never become an apparently successful second execution.
				if v.abortedInitializers == nil {
					v.abortedInitializers = map[string]any{}
				}
				v.abortedInitializers[name] = p
			}
			panic(p)
		}
	}()
	v.initialized[name] = true
	s := map[string]any{}
	v.statics[name] = s
	for _, f := range c.fields {
		if f.access&8 == 0 {
			continue
		}
		x := f.value
		if x == nil {
			x = defaultValue(f.desc)
		}
		s[fieldKey(reference{name, f.name, f.desc})] = x
	}
	nativeSuper := c.super == "" || c.super == "java/lang/Object" || c.super == "java/lang/Record" || c.super == "java/lang/Enum" || c.super == "terrablender/api/Region" || v.host.extra != nil && v.host.extra.initializes(c.super)
	if _, ok := exceptionSupers[c.super]; ok {
		nativeSuper = true
	}
	if c.super == "java/util/AbstractMap" || c.super == "java/util/AbstractCollection" || c.super == "java/util/AbstractList" || c.super == "java/util/AbstractSet" || c.super == "java/util/AbstractQueue" || c.super == "java/util/AbstractSequentialList" || c.super == "java/util/Vector" || c.super == "java/util/Stack" || c.super == "java/util/concurrent/locks/ReentrantLock" || c.super == "java/util/concurrent/locks/ReentrantReadWriteLock" || c.super == "java/util/concurrent/locks/AbstractQueuedSynchronizer" || c.super == "java/lang/ThreadLocal" || c.super == "java/lang/InheritableThreadLocal" || c.super == "java/io/Reader" || c.super == "java/io/Writer" || c.super == "java/lang/ClassLoader" || c.super == "java/lang/Number" || v.hostOpaque(c.super) {
		nativeSuper = true
	}
	if !nativeSuper && v.fetchClass(c.super) {
		v.initialize(c.super)
	} else if !nativeSuper {
		fail("unbound superclass initialization %s", c.super)
	}
	var clinit *method
	for i := range c.methods {
		if c.methods[i].name == "<clinit>" {
			clinit = &c.methods[i]
			break
		}
	}
	switch {
	case clinit != nil && (len(clinit.injects) > 0 || len(c.mixinInits) > 0):
		v.runInjected(c, clinit, nil, nil, c.mixinInits)
	case clinit != nil:
		v.run(c, clinit, nil, nil)
	case len(c.mixinInits) > 0:
		v.runInjected(c, &method{name: "<clinit>", desc: "()V", access: 8}, nil, nil, c.mixinInits)
	}
}

func (v *machine) instanceField(ref reference, receiver *object) string {
	owner := ref.owner
	for n := 0; n < 64 && owner != ""; n++ {
		if v.hostOpaque(owner) {
			return fieldKey(reference{owner, ref.name, ref.desc})
		}
		c := v.load(owner)
		for _, f := range c.fields {
			if f.name == ref.name && f.desc == ref.desc && f.access&8 == 0 {
				if !v.instanceOf(receiver, owner) {
					fail("field receiver is not a %s", owner)
				}
				return fieldKey(reference{owner, ref.name, ref.desc})
			}
		}
		owner = c.super
	}
	fail("missing instance field %s.%s", ref.owner, ref.name)
	return ""
}
func (v *machine) static(r reference) any {
	if x, ok := v.host.static(r); ok {
		return x
	}
	jdkEnum := r.desc == "L"+r.owner+";" && (r.owner == "java/util/stream/Collector$Characteristics" && (r.name == "CONCURRENT" || r.name == "UNORDERED" || r.name == "IDENTITY_FINISH") ||
		r.owner == "java/nio/file/StandardOpenOption" && slices.Contains([]string{"READ", "WRITE", "APPEND", "TRUNCATE_EXISTING", "CREATE", "CREATE_NEW", "DELETE_ON_CLOSE", "SPARSE", "SYNC", "DSYNC"}, r.name) ||
		r.owner == "java/nio/file/StandardCopyOption" && (r.name == "REPLACE_EXISTING" || r.name == "COPY_ATTRIBUTES" || r.name == "ATOMIC_MOVE") ||
		r.owner == "java/nio/file/LinkOption" && r.name == "NOFOLLOW_LINKS")
	if r.owner == roundingModeClass && r.desc == "L"+roundingModeClass+";" {
		if o, ok := v.host.roundingMode(r.name); ok {
			return o
		}
	}
	if r.owner == "java/math/BigInteger" && (r.name == "ZERO" || r.name == "ONE" || r.name == "TEN") && r.desc == "Ljava/math/BigInteger;" {
		if v.statics[r.owner] == nil {
			v.statics[r.owner] = map[string]any{}
		}
		key := fieldKey(r)
		if x, ok := v.statics[r.owner][key]; ok {
			return x
		}
		n := big.NewInt(0)
		switch r.name {
		case "ONE":
			n = big.NewInt(1)
		case "TEN":
			n = big.NewInt(10)
		}
		x := v.alloc("java/math/BigInteger", &javaBigInt{n: n})
		v.statics[r.owner][key] = x
		return x
	}
	if r.owner == "java/nio/charset/StandardCharsets" && r.desc == "Ljava/nio/charset/Charset;" {
		// Charsets are identified by canonical name, as Charset.forName does.
		if name, ok := standardCharset(strings.ReplaceAll(r.name, "_", "-")); ok {
			return v.charset(name)
		}
	}
	if jdkEnum {
		if v.statics[r.owner] == nil {
			v.statics[r.owner] = map[string]any{}
		}
		key := fieldKey(r)
		if x, ok := v.statics[r.owner][key]; ok {
			return x
		}
		x := v.alloc(r.owner, r.name)
		v.statics[r.owner][key] = x
		return x
	}
	if r.name == "TYPE" && r.desc == "Ljava/lang/Class;" {
		prim := map[string]string{
			"java/lang/Boolean": "boolean", "java/lang/Byte": "byte", "java/lang/Character": "char",
			"java/lang/Short": "short", "java/lang/Integer": "int", "java/lang/Long": "long",
			"java/lang/Float": "float", "java/lang/Double": "double", "java/lang/Void": "void",
		}
		if name, ok := prim[r.owner]; ok {
			return classLiteral(name)
		}
	}
	if r.owner == "java/lang/Boolean" && (r.name == "TRUE" || r.name == "FALSE") && r.desc == "Ljava/lang/Boolean;" {
		// Canonical boxed constants keep reference identity across reads.
		if v.statics[r.owner] == nil {
			v.statics[r.owner] = map[string]any{}
		}
		key := fieldKey(r)
		if x, ok := v.statics[r.owner][key]; ok {
			return x
		}
		bit := int32(0)
		if r.name == "TRUE" {
			bit = 1
		}
		x := v.alloc(r.owner, bit)
		v.statics[r.owner][key] = x
		return x
	}
	if r.owner == "java/lang/System" && (r.name == "out" || r.name == "err") && r.desc == "Ljava/io/PrintStream;" {
		// Console output has no worldgen effect: a stable discarding stream.
		if v.statics[r.owner] == nil {
			v.statics[r.owner] = map[string]any{}
		}
		key := fieldKey(r)
		if x, ok := v.statics[r.owner][key]; ok {
			return x
		}
		x := v.alloc("java/io/PrintStream", discardStream(r.name))
		v.statics[r.owner][key] = x
		return x
	}
	if !v.fetchClass(r.owner) {
		if v.hostOpaque(r.owner) {
			nonNull := strings.HasPrefix(r.desc, "L") || strings.HasPrefix(r.desc, "[")
			return v.inert("static "+r.owner+"."+r.name, nonNull)
		}
		fail("unbound static field %s.%s:%s", r.owner, r.name, r.desc)
	}
	v.initialize(r.owner)
	x, ok := v.statics[r.owner][fieldKey(r)]
	if !ok {
		fail("missing static field %s.%s", r.owner, r.name)
	}
	return x
}
func (v *machine) method(r reference, receiver any, virtual bool, args []any) any {
	v.tick()
	if forbiddenClass(r.owner) {
		if value, ok := v.host.reflectionCall(r, receiver, args); ok {
			return value
		}
		if r.owner == "java/nio/charset/Charset" {
			switch r.name + r.desc {
			case "forName(Ljava/lang/String;)Ljava/nio/charset/Charset;":
				if args[0] == nil {
					raiseJava("java/lang/IllegalArgumentException", "null charset name")
				}
				name, ok := standardCharset(javaString(args[0]))
				if !ok {
					raiseJava("java/nio/charset/UnsupportedCharsetException", javaString(args[0]))
				}
				return v.charset(name)
			case "name()Ljava/lang/String;", "displayName()Ljava/lang/String;", "toString()Ljava/lang/String;":
				return objectData(receiver).(string)
			case "newDecoder()Ljava/nio/charset/CharsetDecoder;":
				// Only a reporting-free UTF-8 decoder feeding a reader is bound.
				return v.alloc("java/nio/charset/CharsetDecoder", objectData(receiver).(string))
			}
		}
		if r.owner == "java/nio/charset/CharsetDecoder" {
			switch r.name + r.desc {
			case "onMalformedInput(Ljava/nio/charset/CodingErrorAction;)Ljava/nio/charset/CharsetDecoder;", "onUnmappableCharacter(Ljava/nio/charset/CodingErrorAction;)Ljava/nio/charset/CharsetDecoder;":
				// Readers decode UTF-8 and fail closed on malformed input, so
				// the replacement policy is never consulted.
				return receiver
			case "charset()Ljava/nio/charset/Charset;":
				return v.charset(objectData(receiver).(string))
			}
		}
		if o, ok := receiver.(*object); ok && o != nil && r.owner == "java/io/PrintStream" {
			if _, discard := o.data.(discardStream); discard {
				switch r.name {
				case "print", "println", "write", "flush":
					return nil
				case "printf", "format", "append":
					return receiver
				}
			}
		}
		if r.owner == "java/lang/System" && r.name == "getSecurityManager" && r.desc == "()Ljava/lang/SecurityManager;" {
			return nil
		}
		if value, ok := v.host.pathCall(r, receiver, args); ok {
			return value
		}
		if r.owner == "java/io/Reader" || r.owner == "java/io/Writer" || r.owner == "java/io/InputStream" {
			// Abstract bases of the in-memory stream chain; bound in ioCall,
			// which only accepts receivers backed by overlay bytes.
			if value, ok := v.host.ioCall(r, receiver, args); ok {
				return value
			}
		}
		if r.owner == "java/nio/file/Paths" && r.name == "get" || r.owner == "java/nio/file/Path" && r.name == "of" && receiver == nil {
			// A path value is only a name; any filesystem access through it
			// stays forbidden because java/nio/file/Files remains unbound.
			return v.inert("path "+r.owner+"."+r.name, true)
		}
		if _, inert := receiver.(*inertValue); inert && r.owner == "java/nio/file/Path" {
			switch r.name {
			case "resolve", "resolveSibling", "getParent", "getFileName", "getRoot", "normalize", "toAbsolutePath", "relativize":
				// Pure path algebra on an inert name; no filesystem access.
				return v.inert("path "+r.owner+"."+r.name, true)
			}
		}
		if r.owner == "java/lang/System" && (r.name == "nanoTime" || r.name == "currentTimeMillis") && r.desc == "()J" {
			// Registration must be reproducible: expose a frozen clock rather than
			// the host process time. Timing-only Stopwatch use then stays inert.
			return int64(0)
		}
		if r.owner == "java/lang/System" && r.name == "arraycopy" && r.desc == "(Ljava/lang/Object;ILjava/lang/Object;II)V" {
			src, dest := args[0], args[2]
			srcPos, destPos, length := int(integer(args[1])), int(integer(args[3])), int(integer(args[4]))
			if src == nil || dest == nil {
				raiseJava("java/lang/NullPointerException", "arraycopy")
			}
			sa, ok1 := src.(*array)
			da, ok2 := dest.(*array)
			if !ok1 || !ok2 {
				fail("arraycopy requires arrays")
			}
			if srcPos < 0 || destPos < 0 || length < 0 || srcPos+length > len(sa.values) || destPos+length > len(da.values) {
				raiseJava("java/lang/ArrayIndexOutOfBoundsException", "arraycopy")
			}
			if sa == da && destPos > srcPos {
				for i := length - 1; i >= 0; i-- {
					da.values[destPos+i] = sa.values[srcPos+i]
				}
			} else {
				for i := 0; i < length; i++ {
					da.values[destPos+i] = sa.values[srcPos+i]
				}
			}
			return nil
		}
		// The registration VM runs with a hermetic, empty process environment so
		// host variables cannot change worldgen.
		if r.owner == "java/lang/System" && r.name == "getenv" && r.desc == "()Ljava/util/Map;" {
			return v.alloc("java/util/Map", &javaMap{readonly: true})
		}
		if r.owner == "java/lang/System" && r.name == "getenv" && r.desc == "(Ljava/lang/String;)Ljava/lang/String;" {
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "getenv")
			}
			return nil
		}
		if r.owner == "java/lang/System" && r.name == "getProperties" && r.desc == "()Ljava/util/Properties;" {
			return v.systemPropertiesView()
		}
		if r.owner == "java/lang/System" && r.name == "getProperty" && (r.desc == "(Ljava/lang/String;)Ljava/lang/String;" || r.desc == "(Ljava/lang/String;Ljava/lang/String;)Ljava/lang/String;") {
			key := javaString(args[0])
			if v.systemProperties != nil {
				// Once exposed, the Properties object is the single source of
				// truth so writes through it are observed by getProperty.
				for _, e := range objectData(v.systemProperties).(*javaMap).entries {
					if s, ok := e.left.(string); ok && s == key {
						if value, ok := e.right.(string); ok {
							return value
						}
					}
				}
			} else if value, ok := frozenSystemProperty(key); ok {
				return value
			}
			if len(args) == 2 && args[1] != nil {
				return javaString(args[1])
			}
			return nil
		}
		// Array.newInstance(Class, int) for a reference component type is an
		// ordinary array allocation (Zeta's message serializer builds handler
		// tables this way).
		if r.owner == "java/lang/reflect/Array" && r.name == "newInstance" && r.desc == "(Ljava/lang/Class;I)Ljava/lang/Object;" {
			if c, ok := args[0].(classLiteral); ok {
				s := string(c)
				prim := map[string]struct {
					d string
					z any
				}{"boolean": {"[Z", int32(0)}, "byte": {"[B", int32(0)}, "char": {"[C", int32(0)}, "short": {"[S", int32(0)}, "int": {"[I", int32(0)}, "long": {"[J", int64(0)}, "float": {"[F", float32(0)}, "double": {"[D", float64(0)}}
				if p, ok := prim[s]; ok {
					n := int(integer(args[1]))
					if n < 0 {
						raiseJava("java/lang/NegativeArraySizeException", "")
					}
					if n > 65536 || v.slots+n > 131072 {
						fail("registration array budget exceeded")
					}
					v.slots += n
					v.alloc("[", nil)
					values := make([]any, n)
					for i := range values {
						values[i] = p.z
					}
					return &array{values: values, descriptor: p.d}
				}
				switch s {
				case "void":
				default:
					d := "[L" + s + ";"
					if strings.HasPrefix(s, "[") {
						d = "[" + s
					}
					return v.objectArray(int(integer(args[1])), d)
				}
			}
		}
		// Invoking an unavailable reflection handle (e.g. a framework Method
		// resolved to an inert handle) yields an unavailable value; it never
		// executes host code.
		if reflectionOwner(r.owner) && r.owner != "java/lang/reflect/Array" {
			if i, ok := receiver.(*inertValue); ok {
				_, ret := descriptor(r.desc)
				if ret == "V" {
					return nil
				}
				return v.inert(r.owner+"."+r.name+" of "+i.origin, strings.HasPrefix(ret, "L") || strings.HasPrefix(ret, "["))
			}
		}
		fail("forbidden registration API %s.%s", r.owner, r.name)
	}
	if i, ok := receiver.(*inertValue); ok {
		if r.name == "asItem" && r.desc == "()Lnet/minecraft/world/item/Item;" && registeredContentConstant(i.origin) {
			// ItemLike.asItem of a registered constant is another fixed
			// registered instance, identity-keyed like the receiver.
			return v.inert(i.origin+".asItem", true)
		}
		if r.name == "hashCode" && r.desc == "()I" && registeredContentConstant(i.origin) {
			// Block and Item keep Object.hashCode: an identity hash with no
			// meaning beyond this run, so any stable value is faithful.
			h := int32(17)
			for _, ch := range i.origin {
				h = h*31 + int32(ch)
			}
			return h
		}
		if r.name == "equals" && r.desc == "(Ljava/lang/Object;)Z" && registeredContentConstant(i.origin) {
			// Each Blocks/Items field holds its own instance; Object.equals is
			// identity, so two reads of one field are equal and others are not.
			o, ok := args[0].(*inertValue)
			if ok && o.origin == i.origin {
				return int32(1)
			}
			if !ok && args[0] != nil && inertOf(args[0]) == nil {
				return int32(0)
			}
			if ok && registeredContentConstant(o.origin) || args[0] == nil {
				return int32(0)
			}
		}
		// During mod construction no mod's entries are in the vanilla
		// registries yet (RegisterEvent has not fired), so looking up a
		// non-vanilla id in an unavailable registry is known to miss.
		if r.owner == "net/minecraft/core/Registry" && r.name == "containsKey" && r.desc == "(Lnet/minecraft/resources/ResourceLocation;)Z" {
			if id, ok := objectData(args[0]).(string); ok && strings.Contains(id, ":") && !strings.HasPrefix(id, "minecraft:") {
				return int32(0)
			}
		}
		if r.name == "<init>" {
			return nil
		}
		_, ret := descriptor(r.desc)
		nonNull := strings.HasPrefix(ret, "L") || strings.HasPrefix(ret, "[")
		return v.inert(r.owner+"."+r.name+" result", nonNull)
	}
	// Gson map/list codecs are bound natively: the reflective TypeAdapter path
	// is not part of registration's executable surface.
	if (r.owner == "com/google/gson/Gson" || strings.HasPrefix(r.owner, "com/google/gson/Gson$")) && (r.name == "fromJson" || r.name == "toJson") {
		if x, ok := v.host.gsonCall(r, receiver, args); ok {
			return x
		}
	}
	// Java dispatch must prefer a mod override over a native base-class API.
	if virtual {
		if o, ok := receiver.(*object); ok && o != nil {
			owner := o.class
			for n := 0; n < 64 && owner != r.owner; n++ {
				// Native objects carry their state in data. Loading a library must
				// not dispatch them into an uninitialised Java field layout. A mod
				// override still executes before reaching its native superclass.
				if o.data != nil {
					inherited := r
					inherited.owner = owner
					if x, ok := v.host.call(inherited, receiver, args); ok {
						return x
					}
				}
				if !v.fetchClass(owner) {
					break
				}
				c := v.load(owner)
				for i := range c.methods {
					m := &c.methods[i]
					if m.name == r.name && m.desc == r.desc {
						return v.run(c, m, receiver, args)
					}
				}
				owner = c.super
			}
		}
	}
	if x, ok := v.host.call(r, receiver, args); ok {
		return x
	}
	owner := r.owner
	if reflectionOwner(r.owner) {
		if i := inertOf(append([]any{receiver}, args...)...); i != nil {
			// Reflection over an unavailable member (a game or event-bus field
			// handle) yields an equally unavailable handle or value; any later
			// decision on it fails closed.
			_, ret := descriptor(r.desc)
			if ret == "V" {
				return nil
			}
			return v.inert(r.owner+"."+r.name+" of "+i.origin, strings.HasPrefix(ret, "L") || strings.HasPrefix(ret, "["))
		}
	}
	if virtual {
		if l, ok := receiver.(*lambda); ok {
			// A default method of the functional interface (e.g. Registrate's
			// NonNullSupplier.lazy()) runs its own body with the lambda as `this`;
			// only the abstract SAM dispatches to the lambda target.
			if l.iface != "" {
				if m, c := v.defaultMethod("", reference{l.iface, r.name, r.desc}); m != nil {
					return v.run(c, m, receiver, args)
				}
			}
			return v.callLambda(l, args)
		}
		o, ok := receiver.(*object)
		if !ok || o == nil {
			fail("invalid receiver for %s.%s", r.owner, r.name)
		}
		owner = o.class
	}
	start := owner
	if receiver == nil && r.name != "<clinit>" {
		if v.hostOpaque(owner) {
			_, ret := descriptor(r.desc)
			nonNull := strings.HasPrefix(ret, "L") || strings.HasPrefix(ret, "[")
			return v.inert(owner+"."+r.name+" result", nonNull)
		}
		if !v.fetchClass(owner) {
			fail("unbound registration method %s.%s%s", owner, r.name, r.desc)
		}
		v.initialize(owner)
	}
	for n := 0; n < 64 && owner != ""; n++ {
		if owner != r.owner {
			inherited := r
			inherited.owner = owner
			if x, ok := v.host.call(inherited, receiver, args); ok {
				return x
			}
		}
		if v.hostOpaque(owner) {
			// A game superclass constructor or inherited game method: its state
			// and result are unavailable without the game JAR.
			if r.name == "<init>" {
				return nil
			}
			_, ret := descriptor(r.desc)
			nonNull := strings.HasPrefix(ret, "L") || strings.HasPrefix(ret, "[")
			return v.inert(owner+"."+r.name+" result", nonNull)
		}
		if !v.fetchClass(owner) {
			if m, c := v.defaultMethod(start, r); m != nil {
				return v.run(c, m, receiver, args)
			}
			// java.util.Collection.stream() default method for bytecode
			// collections that inherit it without overriding (JDK classes are
			// not on the registration classpath).
			if x, ok := v.jdkCollectionDefault(owner, r, receiver, args, virtual); ok {
				return x
			}
			fail("unbound registration method %s.%s%s (from %s)", owner, r.name, r.desc, start)
		}
		c := v.load(owner)
		for i := range c.methods {
			m := &c.methods[i]
			if m.name == r.name && m.desc == r.desc && m.access&0x400 == 0 {
				return v.run(c, m, receiver, args)
			}
		}
		owner = c.super
	}
	if m, c := v.defaultMethod(start, r); m != nil {
		return v.run(c, m, receiver, args)
	}
	fail("unbound registration method %s.%s%s", r.owner, r.name, r.desc)
	return nil
}

// defaultMethod resolves an interface default method (JVMS 5.4.3.3 maximally
// specific superinterface method) for a class whose superclass chain does not
// declare the method.
func (v *machine) defaultMethod(start string, r reference) (*method, *class) {
	seen := map[string]bool{}
	var queue []string
	for owner, n := start, 0; owner != "" && n < 64; n++ {
		if v.hostOpaque(owner) || !v.fetchClass(owner) {
			break
		}
		c := v.load(owner)
		queue = append(queue, c.interfaces...)
		owner = c.super
	}
	if r.owner != start {
		queue = append(queue, r.owner)
	}
	for len(queue) > 0 && len(seen) < 512 {
		name := queue[0]
		queue = queue[1:]
		if seen[name] || v.hostOpaque(name) || !v.fetchClass(name) {
			continue
		}
		seen[name] = true
		c := v.load(name)
		for i := range c.methods {
			m := &c.methods[i]
			if m.name == r.name && m.desc == r.desc && m.access&0x0408 == 0 {
				return m, c
			}
		}
		queue = append(queue, c.interfaces...)
	}
	return nil, nil
}
func (v *machine) callLambda(l *lambda, args []any) any {
	values := append(append([]any(nil), l.captured...), args...)
	if l.factory != nil {
		c := *l.factory
		c.captured = append(append([]any(nil), l.factory.captured...), values...)
		v.alloc("java/lang/invoke/Lambda", nil) // shared object budget
		return &c
	}
	if l.kind == 6 {
		return v.adaptLambdaResult(l, v.method(l.target, nil, false, adaptLambdaArgs(l.target.desc, values)))
	}
	if l.kind == 5 || l.kind == 7 || l.kind == 9 {
		if len(values) == 0 {
			fail("missing lambda receiver")
		}
		return v.adaptLambdaResult(l, v.method(l.target, values[0], l.kind != 7, adaptLambdaArgs(l.target.desc, values[1:])))
	}
	if l.kind == 8 {
		values = adaptLambdaArgs(l.target.desc, values)
		// Same rule as the `new` opcode: only bytecode-backed classes run <clinit>;
		// host/library classes are allocated so their native constructor binds.
		if forbiddenClass(l.target.owner) {
			fail("forbidden allocation %s", l.target.owner)
		}
		if v.fetchClass(l.target.owner) {
			v.initialize(l.target.owner)
		}
		o := v.alloc(l.target.owner, nil)
		v.method(l.target, o, false, values)
		return o
	}
	fail("unsupported method-handle kind %d", l.kind)
	return nil
}
func width(x any) int {
	switch x.(type) {
	case int64, float64:
		return 2
	default:
		return 1
	}
}
func integer(x any) int32 {
	if n, ok := x.(int32); ok {
		return n
	}
	fail("expected Java int")
	return 0
}
func equal(a, b any) bool {
	// All interpreter references are pointers, strings, or class literals.
	return a == b
}
func (v *machine) run(c *class, m *method, receiver any, args []any) (result any) {
	if m.origin != nil {
		c = m.origin
	}
	if len(m.injects) > 0 {
		return v.runInjected(c, m, receiver, args, nil)
	}
	v.depth++
	v.frames = append(v.frames, c.name)
	defer func() {
		v.depth--
		v.frames = v.frames[:len(v.frames)-1]
	}()
	if v.depth > 96 || m.locals > 1024 || m.stack > 1024 {
		fail("registration frame budget exceeded")
	}
	if len(m.code) == 0 {
		fail("registration method has no executable body: %s.%s", c.name, m.name)
	}
	r := reader{b: m.code}
	defer func() {
		if p := recover(); p != nil {
			if f, ok := p.(fault); ok {
				panic(fault{fmt.Sprintf("%s at %s.%s:%d", f.message, c.name, m.name, r.p)})
			}
			if _, ok := p.(javaException); !ok {
				panic(fault{fmt.Sprintf("invalid VM operation: %v at %s.%s:%d", p, c.name, m.name, r.p)})
			}
			panic(p)
		}
	}()
	locals := make([]any, m.locals)
	index := 0
	putLocal := func(i int, x any) {
		if i < 0 || i >= len(locals) {
			fail("invalid local index")
		}
		locals[i] = x
	}
	if m.access&8 == 0 {
		putLocal(0, receiver)
		index = 1
	}
	for _, arg := range args {
		putLocal(index, arg)
		index += width(arg)
	}
	stack := make([]any, 0, m.stack)
	push := func(x any) {
		if len(stack) >= m.stack {
			fail("operand stack overflow")
		}
		stack = append(stack, x)
	}
	discard := func() any {
		if len(stack) == 0 {
			fail("operand stack underflow")
		}
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		return x
	}
	pop := func() any {
		x := discard()
		if unknown, ok := x.(unusedNativeResult); ok {
			fail("%s", unknown.reason)
		}
		return x
	}
	load := func(i int) {
		if i < 0 || i >= len(locals) {
			fail("invalid local index")
		}
		push(locals[i])
	}
	jump := func(start, offset int) {
		target := start + offset
		if target < 0 || target >= len(m.code) {
			fail("invalid branch target")
		}
		r.p = target
	}
	pc := 0
	execute := func() (value any, done bool) {
		defer func() {
			if p := recover(); p != nil {
				if e, ok := p.(javaException); ok {
					for _, handler := range m.handlers {
						if pc >= handler.start && pc < handler.end && (handler.catch == "" || v.exceptionAssignable(e, handler.catch)) {
							if e.value == nil {
								e.value = v.alloc(e.class, unboundExceptionMessage{e.message})
							}
							stack = stack[:0]
							push(e.value)
							r.p = handler.pc
							return
						}
					}
				}
				panic(p)
			}
		}()
		for r.p < len(r.b) {
			v.tick()
			start := r.p
			pc = start
			op := r.u1()
			switch {
			case op == 0:
			case op == 1:
				push(nil)
			case op >= 2 && op <= 8:
				push(int32(op - 3))
			case op == 9 || op == 10:
				push(int64(op - 9))
			case op >= 11 && op <= 13:
				push(float32(op - 11))
			case op == 14 || op == 15:
				push(float64(op - 14))
			case op == 16:
				push(int32(int8(r.u1())))
			case op == 17:
				push(int32(int16(r.u2())))
			case op == 18:
				push(c.literal(r.u1()))
			case op == 19 || op == 20:
				push(c.literal(r.u2()))
			case op >= 21 && op <= 25:
				load(r.u1())
			case op >= 26 && op <= 45:
				load((op - 26) % 4)
			case op >= 46 && op <= 53:
				i := int(integer(pop()))
				a := arrayAccess(pop(), i)
				push(a.values[i])
			case op >= 54 && op <= 58:
				putLocal(r.u1(), pop())
			case op >= 59 && op <= 78:
				putLocal((op-59)%4, pop())
			case op >= 79 && op <= 86:
				x := pop()
				i := int(integer(pop()))
				a := arrayAccess(pop(), i)
				if op == 83 && x != nil && a.descriptor != "" {
					component := a.descriptor[1:]
					if strings.HasPrefix(component, "L") {
						component = strings.TrimSuffix(component[1:], ";")
					}
					// inert game values may be stored; decisions on them still fail closed
					if _, inert := x.(*inertValue); !inert && !v.instanceOf(x, component) {
						raiseJava("java/lang/ArrayStoreException", component)
					}
				}
				if op == 84 {
					if a.descriptor == "[Z" {
						x = integer(x) & 1
					} else {
						x = int32(int8(integer(x)))
					}
				}
				if op == 85 {
					x = int32(uint16(integer(x)))
				}
				if op == 86 {
					x = int32(int16(integer(x)))
				}
				a.values[i] = x
			case op == 87:
				discard()
			case op == 88:
				if width(discard()) == 1 {
					discard()
				}
			case op == 89:
				x := pop()
				push(x)
				push(x)
			case op == 90:
				a, b := pop(), pop()
				push(a)
				push(b)
				push(a)
			case op == 91:
				a, b := pop(), pop()
				if width(b) == 2 {
					push(a)
					push(b)
					push(a)
				} else {
					d := pop()
					push(a)
					push(d)
					push(b)
					push(a)
				}
			case op == 92:
				a := pop()
				if width(a) == 2 {
					push(a)
					push(a)
				} else {
					b := pop()
					push(b)
					push(a)
					push(b)
					push(a)
				}
			case op == 95:
				a, b := pop(), pop()
				push(a)
				push(b)
			case op >= 96 && op <= 115:
				b, a := pop(), pop()
				if i := inertOf(a, b); i != nil {
					push(i)
					break
				}
				push(arithmetic(op, a, b))
			case op >= 116 && op <= 119:
				switch x := pop().(type) {
				case *inertValue:
					push(x)
				case int32:
					push(-x)
				case int64:
					push(-x)
				case float32:
					push(-x)
				case float64:
					push(-x)
				default:
					fail("invalid negation")
				}
			case op >= 120 && op <= 131:
				b, a := pop(), pop()
				if i := inertOf(a, b); i != nil {
					push(i)
					break
				}
				push(bitwise(op, a, b))
			case op == 132:
				i, delta := r.u1(), int32(int8(r.u1()))
				if i >= len(locals) {
					fail("invalid local index")
				}
				if _, ok := locals[i].(*inertValue); ok {
					break
				}
				putLocal(i, integer(locals[i])+delta)
			case op >= 133 && op <= 147:
				x := pop()
				if i := inertOf(x); i != nil {
					push(i)
					break
				}
				push(convertNumber(op, x))
			case op >= 148 && op <= 152:
				b, a := pop(), pop()
				if i := inertOf(a, b); i != nil {
					push(i)
					break
				}
				var cmp int32
				if op == 148 {
					aa, bb := a.(int64), b.(int64)
					if aa < bb {
						cmp = -1
					} else if aa > bb {
						cmp = 1
					}
				} else {
					aa, bb := number(a), number(b)
					if math.IsNaN(aa) || math.IsNaN(bb) {
						if op == 149 || op == 151 {
							cmp = -1
						} else {
							cmp = 1
						}
					} else if aa < bb {
						cmp = -1
					} else if aa > bb {
						cmp = 1
					}
				}
				push(cmp)
			case op >= 153 && op <= 158:
				offset := int(int16(r.u2()))
				x := pop()
				requireDetermined("branch", x)
				a := integer(x)
				if comparison(op-153, int64(a), 0) {
					jump(start, offset)
				}
			case op >= 159 && op <= 164:
				offset := int(int16(r.u2()))
				bx, ax := pop(), pop()
				requireDetermined("branch", ax, bx)
				b, a := integer(bx), integer(ax)
				if comparison(op-159, int64(a), int64(b)) {
					jump(start, offset)
				}
			case op == 165 || op == 166:
				offset := int(int16(r.u2()))
				b, a := pop(), pop()
				requireDetermined("reference comparison", a, b)
				if equal(a, b) == (op == 165) {
					jump(start, offset)
				}
			case op == 167:
				jump(start, int(int16(r.u2())))
			case op == 170 || op == 171:
				for r.p%4 != 0 {
					r.u1()
				}
				offset := int(int32(r.u4()))
				kx := pop()
				requireDetermined("switch", kx)
				key := integer(kx)
				if op == 170 {
					lo, hi := int64(int32(r.u4())), int64(int32(r.u4()))
					if hi < lo || hi-lo > 16384 {
						fail("invalid switch")
					}
					for i := lo; i <= hi; i++ {
						o := int(int32(r.u4()))
						if int64(key) == i {
							offset = o
						}
					}
				} else {
					n := int32(r.u4())
					if n < 0 || n > 16384 {
						fail("invalid switch")
					}
					for i := int32(0); i < n; i++ {
						k, o := int32(r.u4()), int(int32(r.u4()))
						if k == key {
							offset = o
						}
					}
				}
				jump(start, offset)
			case op >= 172 && op <= 176:
				return pop(), true
			case op == 177:
				return nil, true
			case op == 178:
				push(v.static(c.ref(r.u2())))
			case op == 179:
				ref := c.ref(r.u2())
				if v.hostOpaque(ref.owner) {
					discard()
					break
				}
				v.initialize(ref.owner)
				key := fieldKey(ref)
				if _, ok := v.statics[ref.owner][key]; !ok {
					fail("missing static field")
				}
				v.statics[ref.owner][key] = pop()
			case op == 180:
				ref := c.ref(r.u2())
				recv := pop()
				if i := inertOf(recv); i != nil {
					push(v.inert("field "+ref.owner+"."+ref.name, false))
					break
				}
				o, ok := recv.(*object)
				if !ok || o == nil {
					fail("invalid field receiver")
				}
				key := v.instanceField(ref, o)
				x, exists := o.fields[key]
				if !exists {
					x = defaultValue(ref.desc)
					if owner, _, _ := strings.Cut(key, "."); v.hostOpaque(owner) {
						// Assigned by an unavailable game constructor.
						x = v.inert("field "+owner+"."+ref.name, false)
					}
				}
				push(x)
			case op == 181:
				ref := c.ref(r.u2())
				x := pop()
				recv := pop()
				if inertOf(recv) != nil {
					break
				}
				o, ok := recv.(*object)
				if !ok || o == nil {
					fail("invalid field receiver")
				}
				key := v.instanceField(ref, o)
				if _, ok := o.fields[key]; !ok {
					v.slots++
					if v.slots > 131072 {
						fail("registration field budget exceeded")
					}
				}
				o.fields[key] = x
			case op >= 182 && op <= 185:
				ref := c.ref(r.u2())
				if op == 185 {
					r.take(2)
				}
				types, ret := descriptor(ref.desc)
				values := make([]any, len(types))
				for i := len(values) - 1; i >= 0; i-- {
					values[i] = pop()
				}
				var recv any
				if op != 184 {
					recv = pop()
					if recv == nil {
						raiseJava("java/lang/NullPointerException", fmt.Sprintf("null invocation %s.%s%s in %s.%s", ref.owner, ref.name, ref.desc, c.name, m.name))
					}
				}
				x := v.method(ref, recv, op == 182 || op == 185, values)
				if ret != "V" {
					push(x)
				}
			case op == 186:
				dynamic := c.cp(r.u2(), 18)
				r.take(2)
				nt := c.cp(dynamic.b, 12)
				types, _ := descriptor(c.descriptor(nt.b))
				captured := make([]any, len(types))
				for i := len(captured) - 1; i >= 0; i-- {
					captured[i] = pop()
				}
				if dynamic.a >= len(c.boots) {
					fail("invalid bootstrap index")
				}
				boot := c.boots[dynamic.a]
				bootstrapRef := c.ref(c.cp(boot.method, 15).b)
				if bootstrapRef.owner == "java/lang/invoke/LambdaMetafactory" && (bootstrapRef.name == "metafactory" && len(boot.args) == 3 || bootstrapRef.name == "altMetafactory" && len(boot.args) >= 4) {
					// altMetafactory shares metafactory's first three static
					// arguments; its flags only add Serializable/marker/bridge
					// metadata, which has no registration-time behaviour here.
					h := c.cp(boot.args[1], 15)
					sam := c.descriptor(c.cp(boot.args[0], 16).a)
					instantiated := c.descriptor(c.cp(boot.args[2], 16).a)
					v.alloc("java/lang/invoke/Lambda", nil) // shared object budget
					_, site := descriptor(c.descriptor(nt.b))
					push(&lambda{target: c.ref(h.b), kind: h.a, captured: captured, sam: sam, instantiated: instantiated, iface: strings.TrimSuffix(strings.TrimPrefix(site, "L"), ";")})
				} else if bootstrapRef.owner == "java/lang/invoke/StringConcatFactory" && (bootstrapRef.name == "makeConcatWithConstants" || bootstrapRef.name == "makeConcat") {
					if i := inertOf(captured...); i != nil {
						// Text built from an unavailable game value stays unavailable.
						push(v.inert("string concat of "+i.origin, true))
					} else {
						push(javaConcat(c, boot, c.descriptor(nt.b), captured, bootstrapRef.name == "makeConcatWithConstants"))
					}
				} else {
					fail("unsupported registration bootstrap %s.%s", bootstrapRef.owner, bootstrapRef.name)
				}
			case op == 187:
				name := c.className(r.u2())
				if forbiddenClass(name) {
					fail("forbidden allocation %s", name)
				}
				// Game classes are allocated as empty hosts even without bytecode so
				// native constructors (ResourceLocation, Climate, …) can bind.
				if v.fetchClass(name) {
					v.initialize(name)
				}
				push(v.alloc(name, nil))
			case op == 188 || op == 189:
				var zero any
				var arrayType string
				if op == 188 {
					kind := r.u1()
					arrayType = map[int]string{4: "[Z", 5: "[C", 6: "[F", 7: "[D", 8: "[B", 9: "[S", 10: "[I", 11: "[J"}[kind]
					switch kind {
					case 4, 5, 8, 9, 10:
						zero = int32(0)
					case 6:
						zero = float32(0)
					case 7:
						zero = float64(0)
					case 11:
						zero = int64(0)
					default:
						fail("invalid array type")
					}
				} else {
					component := c.className(r.u2())
					if strings.HasPrefix(component, "[") {
						arrayType = "[" + component
					} else {
						arrayType = "[L" + component + ";"
					}
				}
				n := int(integer(pop()))
				if n < 0 {
					raiseJava("java/lang/NegativeArraySizeException", "")
				}
				if n > 65536 || v.slots+n > 131072 {
					fail("registration array budget exceeded")
				}
				v.slots += n
				v.alloc("[", nil) // shared object budget, including zero-length arrays
				values := make([]any, n)
				for i := range values {
					values[i] = zero
				}
				push(&array{values: values, descriptor: arrayType})
			case op == 190:
				x := pop()
				requireDetermined("array length", x)
				if x == nil {
					raiseJava("java/lang/NullPointerException", fmt.Sprintf("null arraylength in %s.%s", c.name, m.name))
				}
				a, ok := x.(*array)
				if !ok || a == nil {
					fail("invalid array length")
				}
				push(int32(len(a.values)))
			case op == 191:
				x := pop()
				requireDetermined("throw", x)
				if x == nil {
					raiseJava("java/lang/NullPointerException", fmt.Sprintf("null athrow in %s.%s", c.name, m.name))
				}
				o, ok := x.(*object)
				if !ok || o == nil || !v.exceptionAssignable(javaException{class: o.class, value: o}, "java/lang/Throwable") {
					fail("invalid thrown value")
				}
				panic(javaException{class: o.class, value: o})
			case op == 192:
				name := c.className(r.u2())
				x := pop()
				if inertOf(x) != nil {
					// The declared game type is unknown; the cast is deferred to use.
					push(x)
					break
				}
				if x != nil && !v.instanceOf(x, name) {
					raiseJava("java/lang/ClassCastException", name)
				}
				push(x)
			case op == 193:
				name := c.className(r.u2())
				x := pop()
				if i := inertOf(x); i != nil {
					// Unavailable game values are not instances of any known type,
					// except that registered Item/Block content is always ItemLike.
					if name == "net/minecraft/world/level/ItemLike" && strings.HasPrefix(i.origin, "static RegistryObject ") {
						push(int32(1))
						break
					}
					push(int32(0))
					break
				}
				if v.instanceOf(x, name) {
					push(int32(1))
				} else {
					push(int32(0))
				}
			case op == 198 || op == 199:
				offset := int(int16(r.u2()))
				x := pop()
				if i := inertOf(x); i != nil && !i.nonNull {
					requireDetermined("null test", x)
				}
				if (x == nil) == (op == 198) {
					jump(start, offset)
				}
			case op == 194 || op == 195:
				// monitorenter / monitorexit: registration is single-threaded.
				pop()
			case op == 200:
				jump(start, int(int32(r.u4())))
			default:
				fail("unsupported registration opcode 0x%02x", op)
			}
		}
		fail("registration fell off method body")
		return nil, true
	}
	for {
		value, done := execute()
		if done {
			return value
		}
	}
}
func (v *machine) instanceOf(x any, name string) bool {
	name = frameworkName(name)
	if x == nil {
		return false
	}
	if o, ok := x.(*object); ok {
		if o == nil {
			return false
		}
		if _, ok := exceptionSupers[o.class]; ok {
			return v.exceptionAssignable(javaException{class: o.class}, name)
		}
	}
	if name == "java/lang/Object" {
		return true
	}
	switch o := x.(type) {
	case *array:
		return name == "java/lang/Cloneable" || name == "java/io/Serializable" || v.arrayAssignable(o.descriptor, name)
	case string:
		return name == "java/lang/String" || name == "java/lang/CharSequence" || name == "java/lang/Comparable" || name == "java/io/Serializable"
	case classLiteral:
		return nativeClassAssignable(name)
	case *lambda:
		return v.interfaceExtends(o.iface, name) || strings.HasPrefix(name, "java/util/function/") || name == "java/lang/Runnable" || name == "com/google/common/base/Supplier" || name == "com/google/common/base/Function" || name == "com/google/common/base/Predicate"
	case *consumer:
		return name == "java/util/function/Consumer"
	case *object:
		if o.class == "java/lang/Class" && nativeClassAssignable(name) {
			return true
		}
		if _, ok := o.data.(*javaSet); ok && (name == "java/util/Set" || name == "java/util/Collection" || name == "java/lang/Iterable") {
			return true
		}
		if _, ok := o.data.(*javaList); ok && (name == "java/util/List" || name == "java/util/Collection" || name == "java/lang/Iterable") {
			return true
		}
		if _, ok := o.data.(*javaMap); ok && name == "java/util/Map" {
			return true
		}
		if o.class == "java/util/LinkedHashMap" && (name == "java/util/HashMap" || name == "java/util/AbstractMap" || name == "java/lang/Cloneable" || name == "java/io/Serializable") {
			return true
		}
		if o.class == "java/util/concurrent/ConcurrentHashMap" && name == "java/util/concurrent/ConcurrentMap" {
			return true
		}
		if (o.class == "com/google/common/base/Supplier" || strings.HasPrefix(o.class, "com/google/common/base/Suppliers$") || o.class == "java/util/function/Supplier" || strings.HasPrefix(o.class, "java/util/function/Supplier")) && (name == "java/util/function/Supplier" || name == "com/google/common/base/Supplier") {
			return true
		}
		if _, ok := o.data.(*lambda); ok && (name == "com/google/common/base/Supplier" || name == "java/util/function/Supplier") {
			return true
		}
		if (o.class == "com/google/common/base/Function" || strings.HasPrefix(o.class, "com/google/common/base/Functions$")) && name == "java/util/function/Function" {
			return true
		}
		if o.class == "org/apache/commons/lang3/tuple/ImmutablePair" && (name == "org/apache/commons/lang3/tuple/Pair" || name == "java/util/Map$Entry") {
			return true
		}
		if o.class == "java/lang/String" && (name == "java/lang/CharSequence" || name == "java/io/Serializable" || name == "java/lang/Comparable") {
			return true
		}
		if frameworkName(o.class) == name {
			return true
		}
		if frameworkName(o.class) == "net/minecraftforge/fml/event/config/ModConfigEvent$Loading" && name == "net/minecraftforge/fml/event/config/ModConfigEvent" {
			return true
		}
		switch o.class {
		case "java/lang/Boolean", "java/lang/Character", "java/lang/Byte", "java/lang/Short", "java/lang/Integer", "java/lang/Long", "java/lang/Float", "java/lang/Double":
			if name == "java/lang/Comparable" || name == "java/io/Serializable" {
				return true
			}
		}
		if name == "java/lang/Number" && (o.class == "java/lang/Double" || o.class == "java/lang/Float" || o.class == "java/lang/Integer" || o.class == "java/lang/Long" || o.class == "java/lang/Byte" || o.class == "java/lang/Short") {
			return true
		}
		return v.classAssignable(o.class, name)
	}
	return false
}

func (v *machine) classAssignable(owner, target string) bool {
	pending := []string{owner}
	seen := map[string]bool{}
	for len(pending) > 0 {
		owner = pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		v.tick()
		if frameworkName(owner) == target {
			return true
		}
		if owner == "" || seen[owner] {
			continue
		}
		seen[owner] = true
		if len(seen) > 4096 {
			fail("type hierarchy budget")
		}
		var parents []string
		switch owner {
		case timeUnitClass:
			parents = []string{"java/lang/Enum", "java/lang/Comparable", "java/io/Serializable"}
		case "java/nio/file/StandardOpenOption":
			parents = []string{"java/lang/Enum", "java/lang/Comparable", "java/io/Serializable", "java/nio/file/OpenOption"}
		case "java/nio/file/StandardCopyOption":
			parents = []string{"java/lang/Enum", "java/lang/Comparable", "java/io/Serializable", "java/nio/file/CopyOption"}
		case "java/nio/file/LinkOption":
			parents = []string{"java/lang/Enum", "java/lang/Comparable", "java/io/Serializable", "java/nio/file/OpenOption", "java/nio/file/CopyOption"}
		case "java/lang/reflect/ParameterizedType":
			parents = []string{"java/lang/reflect/Type"}
		case "java/lang/ClassLoader":
			parents = []string{"java/lang/Object"}
		case "java/io/BufferedReader", "java/io/InputStreamReader", "java/io/FileReader", "java/io/StringReader":
			parents = []string{"java/io/Reader"}
		case "java/io/Reader":
			parents = []string{"java/lang/Object", "java/lang/Readable", "java/io/Closeable", "java/lang/AutoCloseable"}
		case "java/io/BufferedWriter", "java/io/OutputStreamWriter", "java/io/FileWriter", "java/io/StringWriter":
			parents = []string{"java/io/Writer"}
		case "java/io/Writer":
			parents = []string{"java/lang/Object", "java/lang/Appendable", "java/io/Closeable", "java/lang/AutoCloseable", "java/io/Flushable"}
		case "java/io/ByteArrayInputStream", "java/io/FileInputStream":
			parents = []string{"java/io/InputStream"}
		case "java/io/InputStream":
			parents = []string{"java/lang/Object", "java/io/Closeable", "java/lang/AutoCloseable"}
		case "java/io/FileOutputStream":
			parents = []string{"java/io/OutputStream"}
		case "java/io/OutputStream":
			parents = []string{"java/lang/Object", "java/io/Closeable", "java/lang/AutoCloseable", "java/io/Flushable"}
		case "net/neoforged/neoforge/registries/DeferredItem", "net/neoforged/neoforge/registries/DeferredBlock":
			parents = []string{neoDeferredHolder, "net/minecraft/world/level/ItemLike"}
		case neoDeferredHolder:
			parents = []string{"net/minecraft/core/Holder", "java/util/function/Supplier"}
		case neoDeferredRegister + "$Items", neoDeferredRegister + "$Blocks":
			parents = []string{neoDeferredRegister}
		default:
			if v.fetchClass(owner) {
				c := v.load(owner)
				parents = append(parents, c.super)
				parents = append(parents, c.interfaces...)
			}
		}
		if len(pending)+len(parents) > 4096 {
			fail("type hierarchy budget")
		}
		pending = append(pending, parents...)
	}
	return false
}

func nativeClassAssignable(name string) bool {
	return name == "java/lang/Class" || name == "java/io/Serializable" || name == "java/lang/reflect/GenericDeclaration" || name == "java/lang/reflect/Type" || name == "java/lang/reflect/AnnotatedElement"
}

func (v *machine) arrayAssignable(from, to string) bool {
	if from == to {
		return true
	}
	if !strings.HasPrefix(from, "[") || !strings.HasPrefix(to, "[") {
		return false
	}
	a, b := from[1:], to[1:]
	if strings.HasPrefix(a, "[") {
		return b == "Ljava/lang/Object;" || b == "Ljava/lang/Cloneable;" || b == "Ljava/io/Serializable;" || v.arrayAssignable(a, b)
	}
	if strings.HasPrefix(a, "L") && strings.HasPrefix(b, "L") {
		return v.instanceOf(&object{class: a[1 : len(a)-1]}, b[1:len(b)-1])
	}
	return false
}
func comparison(op int, a, b int64) bool {
	switch op {
	case 0:
		return a == b
	case 1:
		return a != b
	case 2:
		return a < b
	case 3:
		return a >= b
	case 4:
		return a > b
	case 5:
		return a <= b
	}
	return false
}
func number(x any) float64 {
	switch n := x.(type) {
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	case float32:
		return float64(n)
	case float64:
		return n
	}
	fail("invalid numeric value %T", x)
	return 0
}
func arithmetic(op int, a, b any) any {
	kind, operator := (op-96)%4, (op-96)/4
	switch kind {
	case 0:
		x, y := a.(int32), b.(int32)
		if y == 0 && operator >= 3 {
			raiseJava("java/lang/ArithmeticException", "/ by zero")
		}
		switch operator {
		case 0:
			return x + y
		case 1:
			return x - y
		case 2:
			return x * y
		case 3:
			return x / y
		case 4:
			return x % y
		}
	case 1:
		x, y := a.(int64), b.(int64)
		if y == 0 && operator >= 3 {
			raiseJava("java/lang/ArithmeticException", "/ by zero")
		}
		switch operator {
		case 0:
			return x + y
		case 1:
			return x - y
		case 2:
			return x * y
		case 3:
			return x / y
		case 4:
			return x % y
		}
	case 2:
		x, y := a.(float32), b.(float32)
		switch operator {
		case 0:
			return x + y
		case 1:
			return x - y
		case 2:
			return x * y
		case 3:
			return x / y
		case 4:
			return float32(math.Mod(float64(x), float64(y)))
		}
	case 3:
		x, y := a.(float64), b.(float64)
		switch operator {
		case 0:
			return x + y
		case 1:
			return x - y
		case 2:
			return x * y
		case 3:
			return x / y
		case 4:
			return math.Mod(x, y)
		}
	}
	fail("unsupported arithmetic")
	return nil
}
func bitwise(op int, a, b any) any {
	if op <= 125 {
		n := uint32(integer(b))
		if op%2 == 0 {
			x := a.(int32)
			n &= 31
			switch op {
			case 120:
				return x << n
			case 122:
				return x >> n
			case 124:
				return int32(uint32(x) >> n)
			}
		} else {
			x := a.(int64)
			n &= 63
			switch op {
			case 121:
				return x << n
			case 123:
				return x >> n
			case 125:
				return int64(uint64(x) >> n)
			}
		}
	} else if op%2 == 0 {
		x, y := a.(int32), b.(int32)
		switch op {
		case 126:
			return x & y
		case 128:
			return x | y
		case 130:
			return x ^ y
		}
	} else {
		x, y := a.(int64), b.(int64)
		switch op {
		case 127:
			return x & y
		case 129:
			return x | y
		case 131:
			return x ^ y
		}
	}
	fail("unsupported bit operation")
	return nil
}
func toJavaInt(x float64) int32 {
	if math.IsNaN(x) {
		return 0
	}
	if x >= math.MaxInt32 {
		return math.MaxInt32
	}
	if x <= math.MinInt32 {
		return math.MinInt32
	}
	return int32(x)
}
func toJavaLong(x float64) int64 {
	if math.IsNaN(x) {
		return 0
	}
	if x >= float64(math.MaxInt64) {
		return math.MaxInt64
	}
	if x <= math.MinInt64 {
		return math.MinInt64
	}
	return int64(x)
}
func convertNumber(op int, x any) any {
	switch op {
	case 133:
		return int64(x.(int32))
	case 134:
		return float32(x.(int32))
	case 135:
		return float64(x.(int32))
	case 136:
		return int32(x.(int64))
	case 137:
		return float32(x.(int64))
	case 138:
		return float64(x.(int64))
	case 139, 142:
		return toJavaInt(number(x))
	case 140, 143:
		return toJavaLong(number(x))
	case 141:
		return float64(x.(float32))
	case 144:
		return float32(x.(float64))
	case 145:
		return int32(int8(integer(x)))
	case 146:
		return int32(uint16(integer(x)))
	case 147:
		return int32(int16(integer(x)))
	}
	fail("unsupported numeric conversion")
	return nil
}

// interfaceExtends reports whether interface from is name or inherits it,
// walking loadable superinterfaces (bounded).
func (v *machine) interfaceExtends(from, name string) bool {
	queue, seen := []string{from}, map[string]bool{}
	for len(queue) > 0 && len(seen) < 128 {
		n := queue[0]
		queue = queue[1:]
		if n == "" || seen[n] {
			continue
		}
		if n == name {
			return true
		}
		seen[n] = true
		if v.hostOpaque(n) || !v.fetchClass(n) {
			continue
		}
		queue = append(queue, v.load(n).interfaces...)
	}
	return false
}

// standardCharset maps the charsets every JVM must provide (and their
// historical aliases) to canonical names. Other names stay unsupported.
func standardCharset(name string) (string, bool) {
	switch strings.ToUpper(name) {
	case "UTF-8", "UTF8":
		return "UTF-8", true
	case "US-ASCII", "ASCII":
		return "US-ASCII", true
	case "ISO-8859-1", "ISO8859-1", "ISO_8859_1", "LATIN1":
		return "ISO-8859-1", true
	case "UTF-16", "UTF16":
		return "UTF-16", true
	case "UTF-16BE":
		return "UTF-16BE", true
	case "UTF-16LE":
		return "UTF-16LE", true
	}
	return "", false
}

// charset returns the single Charset instance per canonical name, so
// StandardCharsets fields and forName results compare identical.
func (v *machine) charset(name string) *object {
	const owner = "java/nio/charset/Charset"
	if v.statics[owner] == nil {
		v.statics[owner] = map[string]any{}
	}
	if x, ok := v.statics[owner]["instance "+name].(*object); ok {
		return x
	}
	x := v.alloc(owner, name)
	v.statics[owner]["instance "+name] = x
	return x
}

// systemPropertiesView exposes the frozen system properties as one mutable
// Properties object per machine, shared with System.getProperty.
func (v *machine) systemPropertiesView() *object {
	if v.systemProperties == nil {
		m := &javaMap{}
		for _, key := range frozenSystemPropertyKeys {
			value, _ := frozenSystemProperty(key)
			m.entries = append(m.entries, javaPair{key, value})
		}
		v.systemProperties = v.alloc("java/util/Properties", m)
	}
	return v.systemProperties
}

var boxClasses = map[string]string{"I": "java/lang/Integer", "J": "java/lang/Long", "F": "java/lang/Float", "D": "java/lang/Double", "Z": "java/lang/Boolean", "B": "java/lang/Byte", "S": "java/lang/Short", "C": "java/lang/Character"}

// adaptLambdaArgs applies LambdaMetafactory's argument adaptation: a boxed
// value passed through an erased functional signature is unboxed (and
// widened) into the implementation method's primitive parameter.
func adaptLambdaArgs(desc string, values []any) []any {
	params, _ := descriptor(desc)
	if len(params) != len(values) {
		return values
	}
	var out []any
	for i, p := range params {
		if len(p) != 1 {
			continue
		}
		o, ok := values[i].(*object)
		if values[i] == nil {
			raiseJava("java/lang/NullPointerException", "unboxing null lambda argument")
		}
		if !ok || !strings.HasPrefix(o.class, "java/lang/") || boxClassLetter(o.class) == "" {
			continue
		}
		if out == nil {
			out = append([]any(nil), values...)
		}
		out[i] = widenPrimitive(o.data, p)
	}
	if out == nil {
		return values
	}
	return out
}

func boxClassLetter(class string) string {
	for k, c := range boxClasses {
		if c == class {
			return k
		}
	}
	return ""
}

func widenPrimitive(x any, to string) any {
	switch n := x.(type) {
	case int32:
		switch to {
		case "J":
			return int64(n)
		case "F":
			return float32(n)
		case "D":
			return float64(n)
		}
	case int64:
		switch to {
		case "F":
			return float32(n)
		case "D":
			return float64(n)
		}
	case float32:
		if to == "D" {
			return float64(n)
		}
	}
	return x
}

// adaptLambdaResult boxes a primitive implementation result returned through
// a functional signature whose erased return type is a reference.
func (v *machine) adaptLambdaResult(l *lambda, x any) any {
	sam := l.sam
	if !strings.HasPrefix(sam, "(") {
		return x
	}
	_, want := descriptor(sam)
	_, got := descriptor(l.target.desc)
	if len(want) < 2 || len(got) != 1 {
		return x
	}
	if class := boxClasses[got]; class != "" {
		return v.alloc(class, x)
	}
	return x
}

// jdkCollectionDefault runs JDK collection methods that bytecode collections
// (Guava views, night-config maps) inherit without overriding: the JDK is not
// on the registration classpath, so the inherited body is mirrored here in
// terms of the receiver's own virtual size()/iterator().
func (v *machine) jdkCollectionDefault(owner string, r reference, receiver any, args []any, virtual bool) (any, bool) {
	if !virtual {
		return nil, false
	}
	switch owner {
	case "java/lang/Object", "java/util/AbstractCollection", "java/util/AbstractSet", "java/util/AbstractList", "java/util/AbstractQueue":
	default:
		return nil, false
	}
	switch {
	case r.name == "stream" && r.desc == "()Ljava/util/stream/Stream;":
		return v.alloc("java/util/stream/Stream", &javaList{values: append([]any(nil), v.collectionValues(receiver)...), readonly: true}), true
	case owner != "java/lang/Object" && r.name == "isEmpty" && r.desc == "()Z":
		if integer(v.method(reference{"java/util/Collection", "size", "()I"}, receiver, true, nil)) == 0 {
			return int32(1), true
		}
		return int32(0), true
	case owner != "java/lang/Object" && r.name == "contains" && r.desc == "(Ljava/lang/Object;)Z":
		for _, x := range v.collectionValues(receiver) {
			if v.collectionEquals(args[0], x, false) {
				return int32(1), true
			}
		}
		return int32(0), true
	}
	return nil, false
}

// registeredContentConstant reports a vanilla Blocks/Items static field.
// Those classes do not override equals or hashCode.
func registeredContentConstant(origin string) bool {
	return strings.HasPrefix(origin, "static net/minecraft/world/level/block/Blocks.") || strings.HasPrefix(origin, "static net/minecraft/world/item/Items.") || strings.HasPrefix(origin, "static RegistryObject ")
}

// reflectionOwner reports JDK reflection and method-handle APIs.
func reflectionOwner(owner string) bool {
	return strings.HasPrefix(owner, "java/lang/reflect/") || strings.HasPrefix(owner, "java/lang/invoke/MethodHandle") || owner == "java/lang/invoke/VarHandle"
}
