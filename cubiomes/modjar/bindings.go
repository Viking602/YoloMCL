package modjar

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode"

	"github.com/pelletier/go-toml/v2"

	"github.com/Viking602/YoloMCL/cubiomes"
)

type consumer struct{ accept func(any) }
type region struct {
	id         string
	dimension  cubiomes.Dimension
	weight     int
	name, kind *object
}
type registeredRegion struct {
	instance  *object
	id        string
	dimension cubiomes.Dimension
	deferred  bool // registered by an enqueueWork task
}
type parameterBuilder struct {
	changes []cubiomes.ClimateParameterRewrite
	// TerraBlender ModifiedVanillaOverworldBuilder state: replaceBiome by
	// original biome (wins over by-point), replaceBiome by point, and
	// removeParameter. Later puts replace earlier ones, as in its HashMaps.
	byBiome map[string]string
	byPoint map[[7][2]int64]string
	removed map[[7][2]int64]bool
}
type configBuilder struct {
	path    []string
	entries []*configValue
}
type configValue struct {
	path          []string
	def, min, max any
	value         any
	boolean       bool
	// opaque marks overloads whose validator/converter is Java code
	// (predicates, list element checks, enums). The default is exact; a user
	// override is not re-validated here, so it becomes an inert value.
	opaque bool
}
type configSpec struct{ values []*configValue }

// nativeAPI implements shared game/library interfaces. Implementations must
// enforce the same resource limits as bytecode and reject unknown operations.
type nativeAPI interface {
	static(reference) (any, bool)
	call(reference, any, []any) (any, bool)
	initializes(string) bool
}
type bindings struct {
	lambdaClasses map[classLiteral]*lambda
	paths         *pathOverlay
	extra         nativeAPI
	vm            *machine
	options       Options
	regions       []registeredRegion
	listeners     []eventListener
	work          []queuedWork
	inWork        bool
	// tolerateListeners reports (rather than propagates) listener faults
	// while firing a completed lifecycle event; see fireEventForMod.
	tolerateListeners bool
	lifecycleFaults   []string
	skippedWork       []string
	configs           []*object
	configNames       []string
	configOwners      map[*object]string
	moduleOrder       []string
	base              []cubiomes.TerraBlenderParameter
	symbols           map[string]*object
}

func (h *bindings) symbol(key, class string, data any) *object {
	if h.symbols == nil {
		h.symbols = make(map[string]*object)
	}
	if o := h.symbols[key]; o != nil {
		return o
	}
	o := h.vm.alloc(class, data)
	h.symbols[key] = o
	return o
}

func (h *bindings) registryKey(id string) *object {
	return h.symbol("registry:"+id, "net/minecraft/resources/ResourceKey", id)
}

func objectData(x any) any {
	o, ok := x.(*object)
	if !ok || o == nil {
		fail("expected framework object")
	}
	return o.data
}

func unbox(x any) any {
	if o, ok := x.(*object); ok && (o.class == "java/lang/Integer" || o.class == "java/lang/Long" || o.class == "java/lang/Float" || o.class == "java/lang/Double" || o.class == "java/lang/Boolean") {
		return o.data
	}
	return x
}
func (h *bindings) static(r reference) (any, bool) {
	r = frameworkRef(r)
	if value, ok := h.vanillaBiomeStatic(r); ok {
		return value, true
	}
	if r.owner == timeUnitClass && r.desc == "L"+timeUnitClass+";" {
		o, ok := h.timeUnit(r.name)
		return o, ok
	}
	if r.owner == "java/util/Locale" && (r.name == "ROOT" || r.name == "ENGLISH") && r.desc == "Ljava/util/Locale;" {
		return h.symbol(fieldKey(r), r.owner, r.name), true
	}
	if h.extra != nil {
		if x, ok := h.extra.static(r); ok {
			return x, true
		}
	}
	switch r.owner {
	case "net/minecraftforge/fml/loading/FMLEnvironment":
		if r.name == "dist" && r.desc == "Lnet/minecraftforge/api/distmarker/Dist;" {
			return h.static(reference{"net/minecraftforge/api/distmarker/Dist", h.resolvedSide(), r.desc})
		}
		if r.name == "production" && r.desc == "Z" {
			return int32(1), true
		}
	case "net/minecraftforge/api/distmarker/Dist":
		if r.name == "CLIENT" || r.name == "DEDICATED_SERVER" {
			return h.symbol(fieldKey(r), r.owner, r.name), true
		}
	case "net/fabricmc/api/EnvType":
		if r.name == "CLIENT" || r.name == "SERVER" {
			return h.symbol(fieldKey(r), r.owner, r.name), true
		}
	case "java/nio/charset/CodingErrorAction":
		if r.desc == "Ljava/nio/charset/CodingErrorAction;" && (r.name == "REPORT" || r.name == "REPLACE" || r.name == "IGNORE") {
			return h.symbol(fieldKey(r), r.owner, r.name), true
		}
	case "net/minecraftforge/eventbus/api/EventPriority":
		if r.name == "$VALUES" && r.desc == "[Lnet/minecraftforge/eventbus/api/EventPriority;" {
			values := make([]any, 0, 5)
			for _, name := range []string{"HIGHEST", "HIGH", "NORMAL", "LOW", "LOWEST"} {
				values = append(values, h.symbol(fieldKey(reference{r.owner, name, "L" + r.owner + ";"}), r.owner, eventPriority(name)))
			}
			return &array{values: values, descriptor: r.desc}, true
		}
		return h.symbol(fieldKey(r), r.owner, eventPriority(r.name)), true
	case "terrablender/api/RegionType":
		if r.name == "OVERWORLD" {
			return h.symbol(fieldKey(r), r.owner, cubiomes.Overworld), true
		}
		if r.name == "NETHER" {
			return h.symbol(fieldKey(r), r.owner, cubiomes.Nether), true
		}
	case "net/minecraft/world/level/levelgen/Noises":
		if r.desc == "Lnet/minecraft/resources/ResourceKey;" {
			reg := "net/minecraft/core/registries/Registries.NOISE"
			id := "minecraft:" + strings.ToLower(r.name)
			o := h.symbol("key:"+reg+"\x00"+id, "net/minecraft/resources/ResourceKey", id)
			if o.fields == nil {
				o.fields = map[string]any{}
			}
			o.fields["net/minecraft/resources/ResourceKey.registry"] = reg
			return o, true
		}
	case "java/time/Month":
		if r.desc == "Ljava/time/Month;" {
			switch r.name {
			case "JANUARY", "FEBRUARY", "MARCH", "APRIL", "MAY", "JUNE", "JULY", "AUGUST", "SEPTEMBER", "OCTOBER", "NOVEMBER", "DECEMBER":
				return h.symbol(fieldKey(r), r.owner, r.name), true
			}
		}
	case "net/minecraft/core/registries/Registries":
		if r.desc == "Lnet/minecraft/resources/ResourceKey;" {
			return h.registryKey(r.owner + "." + r.name), true
		}
	case "net/minecraftforge/fml/config/ModConfig$Type":
		return h.symbol(fieldKey(r), r.owner, r.name), true
	case "net/minecraftforge/common/MinecraftForge":
		if r.name == "EVENT_BUS" {
			return h.vm.alloc("net/minecraftforge/eventbus/api/IEventBus", nil), true
		}
	}
	return nil, false
}
func (h *bindings) call(r reference, receiver any, args []any) (any, bool) {
	r = frameworkRef(r)
	if value, ok := h.overworldBuilderCall(r, receiver, args); ok {
		return value, true
	}
	if value, ok := h.distributionCall(r, receiver, args); ok {
		return value, true
	}
	if value, ok := h.loaderEnvCall(r, receiver, args); ok {
		return value, true
	}
	if value, ok := h.ioCall(r, receiver, args); ok {
		return value, true
	}
	if value, ok := h.javaCall(r, receiver, args); ok {
		return value, true
	}
	// Forge's CapabilityTokenSubclass transformer gives each anonymous
	// CapabilityToken subclass a getType() returning its type argument's
	// internal name; the untransformed base method throws.
	if r.owner == "net/minecraftforge/common/capabilities/CapabilityToken" && r.name == "getType" && r.desc == "()Ljava/lang/String;" {
		if o, ok := receiver.(*object); ok && o != nil && h.vm.fetchClass(o.class) {
			sig := h.vm.load(o.class).signature
			const marker = "Lnet/minecraftforge/common/capabilities/CapabilityToken<L"
			if i := strings.Index(sig, marker); i >= 0 {
				rest := sig[i+len(marker):]
				if j := strings.IndexAny(rest, ";<"); j > 0 {
					return rest[:j], true
				}
			}
		}
	}
	// CapabilityManager keys capabilities by interned-string identity, which
	// this VM does not model. Registration only stores the handle, so hand
	// back an uninitialised Capability tagged with its type name.
	if r.owner == "net/minecraftforge/common/capabilities/CapabilityManager" && r.name == "get" && receiver == nil &&
		r.desc == "(Lnet/minecraftforge/common/capabilities/CapabilityToken;)Lnet/minecraftforge/common/capabilities/Capability;" {
		name := h.vm.method(reference{"net/minecraftforge/common/capabilities/CapabilityToken", "getType", "()Ljava/lang/String;"}, args[0], true, nil)
		return h.vm.alloc("net/minecraftforge/common/capabilities/Capability", name), true
	}
	// net.minecraft.Util.make is a pure initializer helper:
	// make(T, Consumer<T>) applies the consumer and returns T, and
	// make(Supplier<T>) returns the supplier's value.
	if r.owner == "net/minecraft/Util" && receiver == nil {
		switch {
		case (r.name == "make" || r.name == "m_137469_") && r.desc == "(Ljava/lang/Object;Ljava/util/function/Consumer;)Ljava/lang/Object;":
			h.vm.method(reference{"java/util/function/Consumer", "accept", "(Ljava/lang/Object;)V"}, args[1], true, []any{args[0]})
			return args[0], true
		case (r.name == "make" || r.name == "m_137537_") && r.desc == "(Ljava/util/function/Supplier;)Ljava/lang/Object;":
			return h.vm.method(reference{"java/util/function/Supplier", "get", "()Ljava/lang/Object;"}, args[0], true, nil), true
		}
	}
	if h.extra != nil {
		if x, ok := h.extra.call(r, receiver, args); ok {
			return x, true
		}
	}
	v := h.vm
	if r.owner == "net/minecraftforge/fml/loading/FMLLoader" && r.name == "getGameLayer" && r.desc == "()Ljava/lang/ModuleLayer;" {
		return v.alloc("java/lang/ModuleLayer", nil), true
	}
	if r.owner == "cpw/mods/modlauncher/api/IModuleLayerManager" && r.name == "getLayer" && r.desc == "(Lcpw/mods/modlauncher/api/IModuleLayerManager$Layer;)Ljava/util/Optional;" {
		return v.alloc("java/util/Optional", javaOptional{v.alloc("java/lang/ModuleLayer", nil)}), true
	}
	if (r.owner == "net/minecraftforge/fml/loading/FMLConfig$ConfigValue" || r.owner == "net/neoforged/fml/loading/FMLConfig$ConfigValue") && r.name == "getConfigValue" && len(args) == 1 {
		o := receiver.(*object)
		if args[0] == nil {
			// load() has not run yet: first-launch defaults are the enum defaultValue
			// after entryFunction, matching an empty fml.toml corrected by ConfigSpec.
			def := o.fields[fieldKey(reference{r.owner, "defaultValue", "Ljava/lang/Object;"})]
			fn := o.fields[fieldKey(reference{r.owner, "entryFunction", "Ljava/util/function/Function;"})]
			if fn == nil {
				return def, true
			}
			return v.method(reference{"java/util/function/Function", "apply", "(Ljava/lang/Object;)Ljava/lang/Object;"}, fn, true, []any{def}), true
		}
	}
	if r.owner == "net/minecraftforge/fml/loading/FMLLoader" && r.name == "getDist" && r.desc == "()Lnet/minecraftforge/api/distmarker/Dist;" {
		return h.static(reference{"net/minecraftforge/api/distmarker/Dist", h.resolvedSide(), "Lnet/minecraftforge/api/distmarker/Dist;"})
	}
	if r.owner == "net/minecraftforge/api/distmarker/Dist" && r.desc == "()Z" && (r.name == "isClient" || r.name == "isDedicatedServer") {
		match := r.name == "isClient" && objectData(receiver) == "CLIENT" || r.name == "isDedicatedServer" && objectData(receiver) == "DEDICATED_SERVER"
		if match {
			return int32(1), true
		}
		return int32(0), true
	}
	if r.owner == "net/fabricmc/api/EnvType" && r.desc == "()Z" && (r.name == "isClient" || r.name == "isServer") {
		match := r.name == "isClient" && objectData(receiver) == "CLIENT" || r.name == "isServer" && objectData(receiver) == "SERVER"
		if match {
			return int32(1), true
		}
		return int32(0), true
	}
	if value, ok := h.roundingModeCall(r, receiver, args); ok {
		return value, true
	}
	if value, ok := h.scanDataCall(r, receiver, args); ok {
		return value, true
	}
	if r.owner == "net/minecraftforge/fml/ModList" || r.owner == "net/fabricmc/loader/api/FabricLoader" {
		switch r.name {
		case "get", "getInstance":
			return v.alloc(r.owner, nil), true
		case "isLoaded", "isModLoaded":
			if h.options.Mods == nil {
				fail("installed mod catalogue is required")
			}
			_, exists := h.options.Mods[args[0].(string)]
			if exists {
				return int32(1), true
			}
			return int32(0), true
		case "getEnvironmentType":
			if r.owner != "net/fabricmc/loader/api/FabricLoader" {
				return nil, false
			}
			return h.static(reference{"net/fabricmc/api/EnvType", h.fabricEnvType(), "Lnet/fabricmc/api/EnvType;"})
		case "isDevelopmentEnvironment":
			// Production worldgen host: never a development environment.
			return int32(0), true
		}
	}
	if l, ok := receiver.(*lambda); ok && (r.name == "accept" || r.name == "apply" || r.name == "run" || r.name == "get") {
		return v.callLambda(l, args), true
	}
	if s, ok := receiver.(*constSupplier); ok && r.name == "get" && len(args) == 0 {
		return s.value, true
	}
	if c, ok := receiver.(*consumer); ok && r.name == "accept" {
		c.accept(args[0])
		return nil, true
	}
	if r.owner == "java/lang/Object" && r.name == "<init>" {
		return nil, true
	}
	if r.owner == "com/mojang/logging/LogUtils" && r.name == "getLogger" {
		return v.alloc("org/slf4j/Logger", nil), true
	}
	if r.owner == "org/apache/logging/log4j/LogManager" && r.name == "getLogger" {
		return v.alloc("org/apache/logging/log4j/Logger", loggerName(args)), true
	}
	if (r.owner == "org/slf4j/LoggerFactory" || r.owner == "org/slf4j/ILoggerFactory") && r.name == "getLogger" {
		return v.alloc("org/slf4j/Logger", loggerName(args)), true
	}
	if r.owner == "org/slf4j/LoggerFactory" && r.name == "getILoggerFactory" && r.desc == "()Lorg/slf4j/ILoggerFactory;" {
		// The logger factory is a host token like the loggers it hands out;
		// SLF4J provider discovery never runs.
		return h.symbol("slf4j:factory", "org/slf4j/ILoggerFactory", nil), true
	}
	// Log markers only tag log records; like loggers they are host tokens
	// carrying their name, so logging back-ends are never initialized.
	if (r.owner == "org/slf4j/MarkerFactory" || r.owner == "org/apache/logging/log4j/MarkerManager") && (r.name == "getMarker" || r.name == "getDetachedMarker") && len(args) == 1 {
		if args[0] == nil {
			raiseJava("java/lang/IllegalArgumentException", "Marker name cannot be null")
		}
		class := "org/slf4j/Marker"
		if strings.HasPrefix(r.owner, "org/apache/") {
			class = "org/apache/logging/log4j/Marker"
		}
		if r.name == "getDetachedMarker" {
			return v.alloc(class, javaString(args[0])), true
		}
		return h.symbol("marker:"+class+"\x00"+javaString(args[0]), class, javaString(args[0])), true
	}
	if o, ok := receiver.(*object); ok && o != nil && (o.class == "org/slf4j/Marker" || o.class == "org/apache/logging/log4j/Marker") {
		_, ret := descriptor(r.desc)
		switch {
		case r.name == "getName" && r.desc == "()Ljava/lang/String;":
			return o.data, true
		case ret == "L"+o.class+";":
			// Parent edits only affect log filtering; the token is fluent.
			return o, true
		case ret == "V":
			return nil, true
		case ret == "Z":
			return int32(0), true
		}
	}
	if loggerReceiver(receiver) && r.name == "getName" && r.desc == "()Ljava/lang/String;" {
		if name, ok := objectData(receiver).(string); ok {
			return name, true
		}
	}
	if loggerReceiver(receiver) || strings.HasPrefix(r.owner, "org/apache/logging/log4j/") || strings.HasPrefix(r.owner, "org/slf4j/") {
		switch r.name {
		case "info", "debug", "warn", "error", "trace", "fatal", "log", "printf", "catching", "throwing", "entry", "exit", "isEnabled", "isInfoEnabled", "isDebugEnabled", "isWarnEnabled", "isErrorEnabled", "isTraceEnabled", "isFatalEnabled":
			_, ret := descriptor(r.desc)
			if ret == "Z" {
				return int32(0), true
			}
			if ret == "V" {
				return nil, true
			}
			if strings.HasPrefix(ret, "L") || strings.HasPrefix(ret, "[") {
				// Fluent logger APIs return the receiver.
				return receiver, true
			}
			return defaultValue(ret), true
		}
	}
	if r.owner == "java/lang/CharSequence" {
		r.owner = "java/lang/String"
		if r.name == "length" && r.desc == "()I" {
			s := javaString(receiver)
			return int32(len([]rune(s))), true
		}
		if r.name == "charAt" && r.desc == "(I)C" {
			s := javaString(receiver)
			runes := []rune(s)
			i := int(integer(args[0]))
			if i < 0 || i >= len(runes) {
				raiseJava("java/lang/StringIndexOutOfBoundsException", "charAt")
			}
			return int32(runes[i]), true
		}
		if r.name == "toString" && r.desc == "()Ljava/lang/String;" {
			return javaString(receiver), true
		}
		if r.name == "subSequence" && r.desc == "(II)Ljava/lang/CharSequence;" {
			s := javaString(receiver)
			runes := []rune(s)
			i, j := int(integer(args[0])), int(integer(args[1]))
			if i < 0 || j > len(runes) || i > j {
				raiseJava("java/lang/StringIndexOutOfBoundsException", "subSequence")
			}
			return string(runes[i:j]), true
		}
	}
	if r.owner == "java/lang/String" {
		s, ok := receiver.(string)
		if !ok {
			if o, isObj := receiver.(*object); isObj && o != nil && o.class == "java/lang/String" {
				s, ok = o.data.(string)
			}
		}
		if ok {
			switch r.name {
			case "charAt":
				if r.desc == "(I)C" {
					runes := []rune(s)
					i := int(integer(args[0]))
					if i < 0 || i >= len(runes) {
						raiseJava("java/lang/StringIndexOutOfBoundsException", "charAt")
					}
					return int32(runes[i]), true
				}
			case "equals":
				other, isString := args[0].(string)
				if o, ok := args[0].(*object); ok && o != nil && o.class == "java/lang/String" {
					other, isString = javaString(o), true
				}
				if isString && s == other {
					return int32(1), true
				}
				return int32(0), true
			case "toString":
				return s, true
			case "concat":
				if len(s)+len(args[0].(string)) > 65536 {
					fail("registration string budget exceeded")
				}
				return s + args[0].(string), true
			case "replace":
				if r.desc != "(Ljava/lang/CharSequence;Ljava/lang/CharSequence;)Ljava/lang/String;" && r.desc != "(CC)Ljava/lang/String;" {
					return nil, false
				}
				if r.desc == "(CC)Ljava/lang/String;" {
					return strings.ReplaceAll(s, string(rune(integer(args[0]))), string(rune(integer(args[1])))), true
				}
				return strings.ReplaceAll(s, javaString(args[0]), javaString(args[1])), true
			case "length":
				if r.desc == "()I" {
					return int32(len([]rune(s))), true
				}
			case "isEmpty":
				if r.desc == "()Z" {
					if s == "" {
						return int32(1), true
					}
					return int32(0), true
				}
			case "contains":
				if r.desc == "(Ljava/lang/CharSequence;)Z" {
					if strings.Contains(s, javaString(args[0])) {
						return int32(1), true
					}
					return int32(0), true
				}
			case "startsWith":
				if r.desc == "(Ljava/lang/String;)Z" {
					if strings.HasPrefix(s, javaString(args[0])) {
						return int32(1), true
					}
					return int32(0), true
				}
			case "hashCode":
				if r.desc == "()I" {
					h := int32(0)
					for _, ch := range s {
						h = h*31 + int32(ch)
					}
					return h, true
				}
			case "endsWith":
				if r.desc == "(Ljava/lang/String;)Z" {
					if strings.HasSuffix(s, javaString(args[0])) {
						return int32(1), true
					}
					return int32(0), true
				}
			case "subSequence":
				if r.desc == "(II)Ljava/lang/CharSequence;" {
					runes := []rune(s)
					i, j := int(integer(args[0])), int(integer(args[1]))
					if i < 0 || j > len(runes) || i > j {
						raiseJava("java/lang/StringIndexOutOfBoundsException", "subSequence")
					}
					return string(runes[i:j]), true
				}
			case "substring":
				runes := []rune(s)
				if r.desc == "(I)Ljava/lang/String;" {
					i := int(integer(args[0]))
					if i < 0 || i > len(runes) {
						raiseJava("java/lang/StringIndexOutOfBoundsException", "substring")
					}
					return string(runes[i:]), true
				}
				if r.desc == "(II)Ljava/lang/String;" {
					i, j := int(integer(args[0])), int(integer(args[1]))
					if i < 0 || j > len(runes) || i > j {
						raiseJava("java/lang/StringIndexOutOfBoundsException", "substring")
					}
					return string(runes[i:j]), true
				}
			}
		}
	}
	if r.owner == "java/lang/Boolean" && r.name == "toString" && (r.desc == "()Ljava/lang/String;" && receiver != nil || r.desc == "(Z)Ljava/lang/String;") {
		bit := int32(0)
		if r.desc == "(Z)Ljava/lang/String;" {
			bit = integer(args[0])
		} else if b, ok := objectData(receiver).(int32); ok {
			bit = b
		} else {
			return nil, false
		}
		if bit != 0 {
			return "true", true
		}
		return "false", true
	}
	if r.owner == "java/lang/Boolean" && r.name == "parseBoolean" && r.desc == "(Ljava/lang/String;)Z" {
		s := strings.ToLower(javaString(args[0]))
		if s == "true" {
			return int32(1), true
		}
		return int32(0), true
	}
	if r.owner == "java/lang/Character" {
		switch {
		case (r.desc == "(C)C" || r.desc == "(I)I") && (r.name == "toUpperCase" || r.name == "toLowerCase" || r.name == "toTitleCase"):
			// ASCII case mapping is exact (title case equals upper case there);
			// non-ASCII Unicode tables are not modelled and fail closed.
			c := integer(args[0])
			if c < 0 || c >= 0x80 {
				fail("unbound non-ASCII Character.%s", r.name)
			}
			if r.name == "toLowerCase" && c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			} else if r.name != "toLowerCase" && c >= 'a' && c <= 'z' {
				c -= 'a' - 'A'
			}
			return int32(c), true
		case r.name == "digit" && (r.desc == "(CI)I" || r.desc == "(II)I"):
			c, radix := rune(integer(args[0])), int32(integer(args[1]))
			if radix < 2 || radix > 36 {
				return int32(-1), true
			}
			d := int32(-1)
			switch {
			case c >= '0' && c <= '9':
				d = c - '0'
			case c >= 'a' && c <= 'z':
				d = c - 'a' + 10
			case c >= 'A' && c <= 'Z':
				d = c - 'A' + 10
			case c >= 0x80 && (unicode.IsDigit(c) || unicode.IsLetter(c)):
				fail("unbound non-ASCII Character.digit")
			}
			if d >= radix {
				d = -1
			}
			return d, true
		case (r.desc == "(C)Z" || r.desc == "(I)Z") && (r.name == "isLetter" || r.name == "isDigit" || r.name == "isLetterOrDigit" || r.name == "isAlphabetic" || r.name == "isWhitespace" || r.name == "isUpperCase" || r.name == "isLowerCase" || r.name == "isUnicodeIdentifierStart" || r.name == "isUnicodeIdentifierPart" || r.name == "isJavaIdentifierStart" || r.name == "isJavaIdentifierPart"):
			// ASCII classification is exact; non-ASCII Unicode tables are not
			// modelled and fail closed.
			c := integer(args[0])
			if c < 0 || c >= 0x80 {
				fail("unbound non-ASCII Character.%s", r.name)
			}
			upper, lower, digit := c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9'
			var ok bool
			switch r.name {
			case "isLetter", "isAlphabetic":
				ok = upper || lower
			case "isDigit":
				ok = digit
			case "isLetterOrDigit":
				ok = upper || lower || digit
			case "isWhitespace":
				ok = c == ' ' || c >= '\t' && c <= '\r' || c >= 0x1c && c <= 0x1f
			case "isUpperCase":
				ok = upper
			case "isLowerCase":
				ok = lower
			case "isUnicodeIdentifierStart":
				ok = upper || lower
			case "isJavaIdentifierStart":
				ok = upper || lower || c == '$' || c == '_'
			case "isUnicodeIdentifierPart", "isJavaIdentifierPart":
				ignorable := c <= 8 || c >= 0xe && c <= 0x1b || c == 0x7f
				ok = upper || lower || digit || c == '_' || ignorable || r.name == "isJavaIdentifierPart" && c == '$'
			}
			if ok {
				return int32(1), true
			}
			return int32(0), true
		case r.name == "valueOf" && r.desc == "(C)Ljava/lang/Character;":
			return v.alloc(r.owner, integer(args[0])), true
		case r.name == "charValue" && r.desc == "()C" && receiver != nil:
			return integer(objectData(receiver)), true
		case r.name == "hashCode" && r.desc == "()I" && receiver != nil:
			return integer(objectData(receiver)), true
		case r.name == "toString" && r.desc == "()Ljava/lang/String;" && receiver != nil:
			return string(rune(integer(objectData(receiver)))), true
		}
	}
	// Short/Byte boxing (l2serial's handler tables box primitive defaults).
	if (r.owner == "java/lang/Short" && r.desc == "(S)Ljava/lang/Short;" || r.owner == "java/lang/Byte" && r.desc == "(B)Ljava/lang/Byte;") && r.name == "valueOf" && receiver == nil {
		return v.alloc(r.owner, integer(args[0])), true
	}
	if (r.owner == "java/lang/Short" || r.owner == "java/lang/Byte") && receiver != nil {
		switch r.name + r.desc {
		case "shortValue()S", "byteValue()B", "intValue()I", "hashCode()I":
			return integer(objectData(receiver)), true
		case "longValue()J":
			return int64(integer(objectData(receiver))), true
		}
	}
	for _, name := range []string{"Double", "Float", "Integer", "Long", "Boolean"} {
		if r.owner != "java/lang/"+name && r.owner != "java/lang/Number" {
			continue
		}
		if (r.owner == "java/lang/Integer" && r.name == "parseInt" && r.desc == "(Ljava/lang/String;)I" || r.owner == "java/lang/Long" && r.name == "parseLong" && r.desc == "(Ljava/lang/String;)J") && receiver == nil {
			if args[0] == nil {
				raiseJava("java/lang/NumberFormatException", "Cannot parse null string: null")
			}
			return parseBoxedString(name, javaString(args[0])), true
		}
		if r.name == "valueOf" && len(args) == 1 {
			if text, ok := args[0].(string); ok {
				return v.alloc(r.owner, parseBoxedString(name, text)), true
			}
			return v.alloc(r.owner, args[0]), true
		}
		if r.owner == "java/lang/Number" && r.name == "<init>" && r.desc == "()V" {
			return nil, true
		}
		if o, ok := receiver.(*object); ok && o != nil && r.owner == "java/lang/Number" && boxClassLetter(o.class) == "" && o.class != "java/math/BigInteger" {
			// A bytecode subclass of Number (e.g. a mutable counter) answers
			// its own accessors through virtual dispatch.
			break
		}
		if receiver != nil {
			x := objectData(receiver)
			if b, ok := x.(*javaBigInt); ok {
				// Number's abstract accessors on a BigInteger follow its
				// narrowing conversions: low-order bits for integral types.
				switch r.name {
				case "intValue":
					return int32(b.n.Int64()), true
				case "longValue":
					return b.n.Int64(), true
				case "doubleValue":
					f, _ := new(big.Float).SetInt(b.n).Float64()
					return f, true
				case "floatValue":
					f, _ := new(big.Float).SetInt(b.n).Float32()
					return f, true
				}
			}
			switch r.name {
			case "floatValue":
				if n, ok := x.(int64); ok {
					return float32(n), true
				}
				return float32(number(x)), true
			case "doubleValue":
				return number(x), true
			case "intValue":
				if n, ok := x.(int64); ok {
					return int32(n), true
				}
				return toJavaInt(number(x)), true
			case "longValue":
				if n, ok := x.(int64); ok {
					return n, true
				}
				return toJavaLong(number(x)), true
			case "booleanValue":
				return integer(x), true
			}
		}
	}
	if r.owner == "net/minecraft/resources/ResourceLocation" || r.owner == "net/minecraft/resources/Identifier" {
		if r.owner == "net/minecraft/resources/Identifier" {
			r.owner = "net/minecraft/resources/ResourceLocation"
		}
		switch r.name {
		case "m_135827_":
			r.name = "getPath"
		case "m_135826_":
			r.name = "getNamespace"
		case "m_135815_":
			// toString / getNamespace ambiguity avoided: only rewrite when void-arg String return matches getters above first
			if r.desc == "()Ljava/lang/String;" && receiver != nil {
				r.name = "toString"
			}
		}
		if receiver == nil && (r.name == "fromNamespaceAndPath" || r.name == "parse" || r.name == "withDefaultNamespace") {
			o := v.alloc(r.owner, nil)
			values := args
			if r.name == "withDefaultNamespace" {
				values = []any{"minecraft", args[0]}
			}
			desc := "(Ljava/lang/String;)V"
			if len(values) == 2 {
				desc = "(Ljava/lang/String;Ljava/lang/String;)V"
			}
			if _, ok := h.call(reference{r.owner, "<init>", desc}, o, values); !ok {
				return nil, false
			}
			return o, true
		}
		if r.name == "<init>" {
			if r.desc != "(Ljava/lang/String;)V" && r.desc != "(Ljava/lang/String;Ljava/lang/String;)V" {
				return nil, false
			}
			id := args[0].(string)
			if len(args) == 2 {
				id += ":" + args[1].(string)
			}
			if !strings.Contains(id, ":") {
				id = "minecraft:" + id
			}
			parts := strings.Split(id, ":")
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				fail("invalid resource location")
			}
			for i, p := range parts {
				for _, ch := range p {
					if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || strings.ContainsRune("_.-", ch) || i == 1 && ch == '/') {
						fail("invalid resource location")
					}
				}
			}
			receiver.(*object).data = id
			return nil, true
		}
		if r.name == "toString" {
			return objectData(receiver), true
		}
		if r.name == "equals" && r.desc == "(Ljava/lang/Object;)Z" {
			if other, ok := args[0].(*object); ok && other != nil && frameworkName(other.class) == r.owner && other.data == objectData(receiver) {
				return int32(1), true
			}
			return int32(0), true
		}
		if r.name == "getNamespace" || r.name == "getPath" {
			namespace, path, _ := strings.Cut(objectData(receiver).(string), ":")
			if r.name == "getNamespace" {
				return namespace, true
			}
			return path, true
		}
		if r.name == "hashCode" && r.desc == "()I" {
			s := objectData(receiver).(string)
			h := int32(0)
			for _, ch := range s {
				h = h*31 + int32(ch)
			}
			return h, true
		}
	}
	if r.owner == "net/minecraft/resources/ResourceKey" {
		// Normalize SRG / unmapped names onto official bindings by descriptor.
		switch r.name {
		case "m_135788_", "m_218867_":
			r.name = "createRegistryKey"
		case "m_135735_", "m_218868_":
			r.name = "create"
		case "m_135782_":
			r.name = "location"
		case "m_135785_", "m_135783_", "m_135787_":
			// registry() return type differs by version (ResourceKey vs ResourceLocation).
			r.name = "registry"
		default:
			if receiver == nil {
				switch r.desc {
				case "(Lnet/minecraft/resources/ResourceLocation;)Lnet/minecraft/resources/ResourceKey;":
					r.name = "createRegistryKey"
				case "(Lnet/minecraft/resources/ResourceKey;Lnet/minecraft/resources/ResourceLocation;)Lnet/minecraft/resources/ResourceKey;":
					r.name = "create"
				}
			} else if strings.HasPrefix(r.name, "m_") {
				switch r.desc {
				case "()Lnet/minecraft/resources/ResourceLocation;":
					r.name = "location"
				case "()Lnet/minecraft/resources/ResourceKey;":
					r.name = "registry"
				}
			}
		}
	}
	if r.owner == "net/minecraft/resources/ResourceKey" && receiver != nil {
		if r.name == "hashCode" && r.desc == "()I" {
			s := objectData(receiver).(string)
			h := int32(0)
			for _, ch := range s {
				h = h*31 + int32(ch)
			}
			return h, true
		}
		if r.name == "equals" && r.desc == "(Ljava/lang/Object;)Z" {
			if other, ok := args[0].(*object); ok && other != nil && frameworkName(other.class) == r.owner && other.data == objectData(receiver) {
				return int32(1), true
			}
			return int32(0), true
		}
	}
	if r.owner == "net/minecraft/resources/ResourceKey" && r.name == "location" && receiver != nil {
		o := receiver.(*object)
		id, ok := o.data.(string)
		if !ok || id == "" {
			// Keys synthesized without a location still need a stable ResourceLocation
			// for ForgeRegistries/GameData hashing during registration.
			id = "minecraft:unknown"
			if o.fields != nil {
				if reg, _ := o.fields["net/minecraft/resources/ResourceKey.registry"].(string); reg != "" && strings.Contains(reg, ":") {
					id = reg
				}
			}
		}
		if mapped := nativeRegistryLocation(id); mapped != "" {
			id = mapped
		} else if rest, ok := strings.CutPrefix(id, "net/minecraft/core/registries/Registries."); ok && rest != "" {
			id = "minecraft:" + strings.ToLower(rest)
		} else if !strings.Contains(id, ":") {
			if mapped := nativeRegistryLocation("net/minecraft/core/registries/Registries." + id); mapped != "" {
				id = mapped
			} else {
				id = "minecraft:" + strings.ToLower(id)
			}
		}
		return v.alloc("net/minecraft/resources/ResourceLocation", id), true
	}
	if r.owner == "net/minecraft/resources/ResourceKey" && r.name == "registry" && receiver != nil && (r.desc == "()Lnet/minecraft/resources/ResourceKey;" || r.desc == "()Lnet/minecraft/resources/ResourceLocation;") {
		o := receiver.(*object)
		reg, _ := o.fields["net/minecraft/resources/ResourceKey.registry"].(string)
		if reg == "" {
			for key, sym := range h.symbols {
				if sym != o {
					continue
				}
				rest, ok := strings.CutPrefix(key, "key:")
				if !ok {
					continue
				}
				reg, _, ok = strings.Cut(rest, "\x00")
				if ok && reg != "" {
					o.fields["net/minecraft/resources/ResourceKey.registry"] = reg
					break
				}
			}
		}
		if reg == "" {
			for key, sym := range h.symbols {
				if sym != o {
					continue
				}
				if rest, ok := strings.CutPrefix(key, "registry:"); ok && rest != "" {
					reg = "minecraft:root"
					o.fields["net/minecraft/resources/ResourceKey.registry"] = reg
					break
				}
			}
		}
		if reg == "" {
			reg = "minecraft:root"
			if o.fields == nil {
				o.fields = map[string]any{}
			}
			o.fields["net/minecraft/resources/ResourceKey.registry"] = reg
		}
		if r.desc == "()Lnet/minecraft/resources/ResourceLocation;" {
			id := reg
			if !strings.Contains(id, ":") {
				if mapped := nativeRegistryLocation(id); mapped != "" {
					id = mapped
				}
			}
			if id == "" {
				id = reg
				if !strings.Contains(id, ":") {
					id = "minecraft:" + strings.ToLower(id)
				}
			}
			return v.alloc("net/minecraft/resources/ResourceLocation", id), true
		}
		return h.registryKey(reg), true
	}
	if r.owner == "net/minecraft/resources/ResourceKey" && receiver == nil && r.desc == "(Lnet/minecraft/resources/ResourceLocation;)Lnet/minecraft/resources/ResourceKey;" {
		id := objectData(args[0]).(string)
		o := h.symbol("registry:"+id, r.owner, id)
		if o.fields == nil {
			o.fields = map[string]any{}
		}
		o.fields["net/minecraft/resources/ResourceKey.registry"] = "minecraft:root"
		return o, true
	}
	if r.owner == "net/minecraft/resources/ResourceKey" && receiver == nil && r.desc == "(Lnet/minecraft/resources/ResourceKey;Lnet/minecraft/resources/ResourceLocation;)Lnet/minecraft/resources/ResourceKey;" {
		reg := objectData(args[0]).(string)
		id := objectData(args[1]).(string)
		o := h.symbol("key:"+reg+"\x00"+id, r.owner, id)
		if o.fields == nil {
			o.fields = map[string]any{}
		}
		o.fields["net/minecraft/resources/ResourceKey.registry"] = reg
		return o, true
	}
	if (strings.HasPrefix(r.owner, "net/minecraft/data/worldgen/placement/") || strings.HasPrefix(r.owner, "net/minecraft/world/level/levelgen/placement/")) && receiver == nil {
		if r.name == "<init>" {
			return nil, true
		}
		ret := r.desc[strings.LastIndex(r.desc, ")")+1:]
		switch {
		case ret == "V":
			return nil, true
		case ret == "Ljava/util/List;":
			return v.alloc("java/util/ArrayList", &javaList{}), true
		case strings.HasPrefix(ret, "L") && strings.HasSuffix(ret, ";"):
			return v.inert(r.owner+"."+r.name+" result", true), true
		default:
			return defaultValue(ret), true
		}
	}
	if strings.HasPrefix(r.owner, "net/minecraft/world/level/levelgen/SurfaceRules") && receiver == nil {
		if r.name == "<init>" {
			return nil, true
		}
		ret := r.desc[strings.LastIndex(r.desc, ")")+1:]
		switch {
		case ret == "V":
			return nil, true
		case strings.HasPrefix(ret, "L") && strings.HasSuffix(ret, ";"):
			return v.inert(r.owner+"."+r.name+" result", true), true
		default:
			return defaultValue(ret), true
		}
	}
	if r.owner == "net/minecraft/world/level/levelgen/VerticalAnchor" && receiver == nil {
		ret := r.desc[strings.LastIndex(r.desc, ")")+1:]
		if strings.HasPrefix(ret, "L") && strings.HasSuffix(ret, ";") {
			return v.inert(r.owner+"."+r.name+" result", true), true
		}
	}
	if r.owner == "net/minecraft/world/level/biome/Climate$Parameter" {
		const paramDesc = "Lnet/minecraft/world/level/biome/Climate$Parameter;"
		if receiver == nil {
			switch r.desc {
			case "(FF)" + paramDesc, "(F)" + paramDesc:
				lo := args[0].(float32)
				hi := lo
				if len(args) == 2 {
					hi = args[1].(float32)
				}
				if math.IsNaN(float64(lo)) || math.IsNaN(float64(hi)) || lo > hi || math.Abs(float64(lo)) > 10000 || math.Abs(float64(hi)) > 10000 {
					fail("invalid climate parameter")
				}
				return v.alloc(r.owner, [2]int64{int64(lo * 10000), int64(hi * 10000)}), true
			case "(" + paramDesc + paramDesc + ")" + paramDesc:
				a := objectData(args[0]).([2]int64)
				b := objectData(args[1]).([2]int64)
				lo, hi := a[0], a[1]
				if b[0] < lo {
					lo = b[0]
				}
				if b[1] > hi {
					hi = b[1]
				}
				return v.alloc(r.owner, [2]int64{lo, hi}), true
			}
		} else if span, ok := objectData(receiver).([2]int64); ok {
			switch {
			case (r.name == "min" || r.name == "comp_103" || r.name == "f_186813_") && r.desc == "()J":
				return span[0], true
			case (r.name == "max" || r.name == "comp_104" || r.name == "f_186814_") && r.desc == "()J":
				return span[1], true
			}
		}
	}
	// Record equals/hashCode (java.lang.runtime.ObjectMethods semantics).
	if o, ok := receiver.(*object); ok && o != nil && climateRecord(o) {
		switch {
		case r.name == "equals" && r.desc == "(Ljava/lang/Object;)Z":
			other, ok := args[0].(*object)
			if ok && other != nil && other.class == o.class && other.data == o.data {
				return int32(1), true
			}
			return int32(0), true
		case r.name == "hashCode" && r.desc == "()I":
			long := func(x int64) int32 { return int32(x ^ int64(uint64(x)>>32)) }
			span := func(s [2]int64) int32 { return 31*long(s[0]) + long(s[1]) }
			if s, ok := o.data.([2]int64); ok {
				return span(s), true
			}
			p := o.data.([7][2]int64)
			var h int32
			for i := 0; i < 6; i++ {
				h = 31*h + span(p[i])
			}
			return 31*h + long(p[6][0]), true
		}
	}
	// ParameterPoint record accessors (official and SRG names, component order).
	if o, ok := receiver.(*object); ok && o != nil && o.class == "net/minecraft/world/level/biome/Climate$ParameterPoint" {
		if p, ok := o.data.([7][2]int64); ok {
			names := [...][2]string{{"temperature", "f_186863_"}, {"humidity", "f_186864_"}, {"continentalness", "f_186865_"}, {"erosion", "f_186866_"}, {"depth", "f_186867_"}, {"weirdness", "f_186868_"}}
			for i, n := range names {
				if (r.name == n[0] || r.name == n[1]) && r.desc == "()Lnet/minecraft/world/level/biome/Climate$Parameter;" {
					return v.alloc("net/minecraft/world/level/biome/Climate$Parameter", p[i]), true
				}
			}
			if (r.name == "offset" || r.name == "f_186869_") && r.desc == "()J" {
				return p[6][0], true
			}
		}
	}
	if r.owner == "net/minecraft/world/level/biome/Climate$Parameter" && r.name == "<init>" && r.desc == "(JJ)V" {
		receiver.(*object).data = [2]int64{args[0].(int64), args[1].(int64)}
		return nil, true
	}
	// Record constructor: six parameter spans and the quantized offset.
	if r.owner == "net/minecraft/world/level/biome/Climate$ParameterPoint" && r.name == "<init>" && r.desc == "(Lnet/minecraft/world/level/biome/Climate$Parameter;Lnet/minecraft/world/level/biome/Climate$Parameter;Lnet/minecraft/world/level/biome/Climate$Parameter;Lnet/minecraft/world/level/biome/Climate$Parameter;Lnet/minecraft/world/level/biome/Climate$Parameter;Lnet/minecraft/world/level/biome/Climate$Parameter;J)V" {
		var space [7][2]int64
		for i := 0; i < 6; i++ {
			if args[i] == nil {
				raiseJava("java/lang/NullPointerException", "climate parameter")
			}
			space[i] = objectData(args[i]).([2]int64)
		}
		offset := args[6].(int64)
		space[6] = [2]int64{offset, offset}
		receiver.(*object).data = space
		return nil, true
	}
	// Climate.quantizeCoord / unquantizeCoord (official, intermediary, SRG).
	if r.owner == "net/minecraft/world/level/biome/Climate" && receiver == nil {
		switch {
		case r.desc == "(F)J" && (r.name == "quantizeCoord" || r.name == "method_38665" || r.name == "m_186779_"):
			return int64(args[0].(float32) * 10000), true
		case r.desc == "(J)F" && (r.name == "unquantizeCoord" || r.name == "method_38666" || r.name == "m_186796_"):
			return float32(args[0].(int64)) / 10000, true
		}
	}
	if r.owner == "net/minecraft/world/level/biome/Climate" && receiver == nil && len(args) == 7 && strings.HasSuffix(r.desc, "F)Lnet/minecraft/world/level/biome/Climate$ParameterPoint;") {
		var space [7][2]int64
		for i := 0; i < 6; i++ {
			space[i] = objectData(args[i]).([2]int64)
		}
		offset := args[6].(float32)
		if math.IsNaN(float64(offset)) || math.Abs(float64(offset)) > 10000 {
			fail("invalid climate offset")
		}
		space[6] = [2]int64{int64(offset * 10000), int64(offset * 10000)}
		return v.alloc("net/minecraft/world/level/biome/Climate$ParameterPoint", space), true
	}
	if r.owner == "terrablender/api/Region" {
		switch r.name {
		case "<init>":
			receiver.(*object).data = &region{id: objectData(args[0]).(string), dimension: objectData(args[1]).(cubiomes.Dimension), weight: int(integer(args[2])), name: args[0].(*object), kind: args[1].(*object)}
			return nil, true
		case "getName":
			return objectData(receiver).(*region).name, true
		case "getType":
			return objectData(receiver).(*region).kind, true
		case "getWeight":
			return int32(objectData(receiver).(*region).weight), true
		case "addModifiedVanillaOverworldBiomes":
			builder := v.alloc("terrablender/api/ModifiedVanillaOverworldBuilder", &parameterBuilder{})
			v.callLambda(args[1].(*lambda), []any{builder})
			if h.base == nil {
				var err error
				h.base, err = cubiomes.BuiltinClimateParameters(v.ctx, h.options.Minecraft, "minecraft:overworld")
				if err != nil {
					fail("%v", err)
				}
			}
			pb := builder.data.(*parameterBuilder)
			base := make([]cubiomes.TerraBlenderParameter, 0, len(h.base))
			for _, p := range h.base {
				if pb.removed[p.Space] {
					continue
				}
				if to, ok := pb.byBiome[p.Biome]; ok {
					p.Biome = to
				} else if to, ok := pb.byPoint[p.Space]; ok {
					p.Biome = to
				}
				base = append(base, p)
			}
			values, err := cubiomes.RewriteClimateParameters(v.ctx, base, pb.changes, nil)
			if err != nil {
				fail("%v", err)
			}
			for _, p := range values {
				args[0].(*consumer).accept(p)
			}
			return nil, true
		case "addBiome":
			if len(args) == 3 {
				args[0].(*consumer).accept(cubiomes.TerraBlenderParameter{Biome: objectData(args[2]).(string), Space: objectData(args[1]).([7][2]int64)})
				return nil, true
			}
		}
	}
	if r.owner == "terrablender/api/ModifiedVanillaOverworldBuilder" && (r.name == "replaceBiome" || r.name == "removeParameter") {
		b := objectData(receiver).(*parameterBuilder)
		if len(b.byBiome)+len(b.byPoint)+len(b.removed) >= 4096 {
			fail("too many parameter rewrites")
		}
		switch {
		case r.name == "removeParameter":
			if b.removed == nil {
				b.removed = map[[7][2]int64]bool{}
			}
			b.removed[objectData(args[0]).([7][2]int64)] = true
		case r.desc == "(Lnet/minecraft/resources/ResourceKey;Lnet/minecraft/resources/ResourceKey;)V":
			if b.byBiome == nil {
				b.byBiome = map[string]string{}
			}
			b.byBiome[objectData(args[0]).(string)] = objectData(args[1]).(string)
		default:
			if b.byPoint == nil {
				b.byPoint = map[[7][2]int64]string{}
			}
			b.byPoint[objectData(args[0]).([7][2]int64)] = objectData(args[1]).(string)
		}
		return nil, true
	}
	if r.owner == "terrablender/api/ModifiedVanillaOverworldBuilder" && r.name == "replaceParameter" {
		b := objectData(receiver).(*parameterBuilder)
		if len(b.changes) >= 4096 {
			fail("too many parameter rewrites")
		}
		b.changes = append(b.changes, cubiomes.ClimateParameterRewrite{Match: objectData(args[0]).([7][2]int64), With: cubiomes.TerraBlenderParameter{Space: objectData(args[1]).([7][2]int64)}})
		return nil, true
	}
	if r.owner == "terrablender/api/Regions" && r.name == "register" && len(args) == 1 {
		o := args[0].(*object)
		if _, ok := o.data.(*region); !ok {
			fail("invalid TerraBlender region")
		}
		if len(h.regions) >= 255 {
			fail("too many registered regions")
		}
		name := v.method(reference{"terrablender/api/Region", "getName", "()Lnet/minecraft/resources/ResourceLocation;"}, o, true, nil)
		kind := v.method(reference{"terrablender/api/Region", "getType", "()Lterrablender/api/RegionType;"}, o, true, nil)
		h.regions = append(h.regions, registeredRegion{o, objectData(name).(string), objectData(kind).(cubiomes.Dimension), h.inWork})
		return nil, true
	}
	if r.owner == "net/neoforged/neoforge/common/ModConfigSpec$BuilderContext" || r.owner == "net/minecraftforge/common/ForgeConfigSpec$BuilderContext" {
		if r.name == "<init>" {
			return nil, true
		}
		if r.name == "setClazz" || r.name == "setComment" || r.name == "setTranslationKey" || r.name == "setWorldRestart" {
			return nil, true
		}
		ret := r.desc[strings.LastIndex(r.desc, ")")+1:]
		if strings.HasPrefix(ret, "L") {
			return v.alloc(strings.TrimSuffix(strings.TrimPrefix(ret, "L"), ";"), nil), true
		}
		if ret == "V" {
			return nil, true
		}
	}
	if r.owner == "net/minecraftforge/common/ForgeConfigSpec$Builder" || r.owner == "net/neoforged/neoforge/common/ModConfigSpec$Builder" {
		if r.name == "<init>" {
			receiver.(*object).data = &configBuilder{}
			return nil, true
		}
		b := objectData(receiver).(*configBuilder)
		switch r.name {
		case "push":
			if len(b.path) >= 32 {
				fail("configuration nesting limit")
			}
			b.path = append(b.path, args[0].(string))
			return receiver, true
		case "pop":
			if len(args) != 0 {
				return nil, false
			}
			if len(b.path) == 0 {
				fail("invalid configuration pop")
			}
			b.path = b.path[:len(b.path)-1]
			return receiver, true
		case "comment", "worldRestart", "translation":
			return receiver, true
		case "define", "defineInRange":
			// Predicates, suppliers and other overloads carry semantics of their
			// own. Do not silently treat them as primitive configuration values.
			supported := r.name == "define" && strings.HasPrefix(r.desc, "(Ljava/lang/String;Z)L") && strings.HasSuffix(r.desc, "$BooleanValue;")
			if r.name == "define" && strings.HasPrefix(r.desc, "(Ljava/lang/String;Ljava/lang/Object;)L") && strings.HasSuffix(r.desc, "$ConfigValue;") {
				switch unbox(args[1]).(type) {
				case int32, int64, float64, string:
					supported = true
				}
			}
			if r.name == "defineInRange" {
				switch {
				case strings.HasPrefix(r.desc, "(Ljava/lang/String;III)L") && strings.HasSuffix(r.desc, "$IntValue;"),
					strings.HasPrefix(r.desc, "(Ljava/lang/String;JJJ)L") && strings.HasSuffix(r.desc, "$LongValue;"),
					strings.HasPrefix(r.desc, "(Ljava/lang/String;DDD)L") && strings.HasSuffix(r.desc, "$DoubleValue;"):
					supported = true
				}
			}
			if !supported {
				if len(args) < 2 {
					return nil, false
				}
				return h.genericConfigValue(b, r, args), true
			}
			if len(b.entries) >= 8192 {
				fail("configuration value budget exceeded")
			}
			value := &configValue{path: append(append([]string(nil), b.path...), args[0].(string)), def: unbox(args[1]), value: unbox(args[1])}
			value.boolean = strings.HasSuffix(r.desc, "$BooleanValue;")
			if boxed, ok := args[1].(*object); ok && boxed.class == "java/lang/Boolean" {
				value.boolean = true
			}
			if len(args) == 4 {
				value.min, value.max = unbox(args[2]), unbox(args[3])
			}
			b.entries = append(b.entries, value)
			_, ret := descriptor(r.desc)
			return v.alloc(strings.TrimSuffix(strings.TrimPrefix(ret, "L"), ";"), value), true
		case "build":
			if strings.Contains(r.owner, "neoforged") {
				return v.alloc("net/neoforged/neoforge/common/ModConfigSpec", &configSpec{b.entries}), true
			}
			return v.alloc("net/minecraftforge/common/ForgeConfigSpec", &configSpec{b.entries}), true
		case "configure":
			if r.desc != "(Ljava/util/function/Function;)Lorg/apache/commons/lang3/tuple/Pair;" {
				return nil, false
			}
			value := v.method(reference{"java/util/function/Function", "apply", "(Ljava/lang/Object;)Ljava/lang/Object;"}, args[0], true, []any{receiver})
			spec := v.alloc("net/minecraftforge/common/ForgeConfigSpec", &configSpec{b.entries})
			return v.alloc("org/apache/commons/lang3/tuple/ImmutablePair", javaPair{value, spec}), true
		default:
			if strings.HasPrefix(r.name, "define") && len(args) >= 2 {
				return h.genericConfigValue(b, r, args), true
			}
		}
	}
	if (strings.HasPrefix(r.owner, "net/minecraftforge/common/ForgeConfigSpec$") || strings.HasPrefix(r.owner, "net/neoforged/neoforge/common/ModConfigSpec$")) && r.name == "get" && receiver != nil {
		cv, ok := objectData(receiver).(*configValue)
		if !ok {
			return nil, false
		}
		x := cv.value
		switch x.(type) {
		case string, *object, *lambda, *inertValue, nil:
			return x, true
		}
		name := "Integer"
		switch x.(type) {
		case float64:
			name = "Double"
		case int64:
			name = "Long"
		}
		if objectData(receiver).(*configValue).boolean {
			name = "Boolean"
		}
		return v.alloc("java/lang/"+name, x), true
	}
	if r.owner == "net/minecraftforge/fml/ModLoadingContext" || r.owner == "net/minecraftforge/fml/ModContainer" {
		switch r.name {
		case "get":
			return v.alloc(r.owner, nil), true
		case "getModId":
			if r.owner == "net/minecraftforge/fml/ModContainer" && r.desc == "()Ljava/lang/String;" && h.options.ModID != "" {
				return h.options.ModID, true
			}
			return nil, false
		case "addConfig":
			// A ModConfig built by the mod itself (Ars Nouveau's per-glyph
			// configs) keeps its spec defaults; FML's container bookkeeping is
			// not modelled and such configs do not drive worldgen.
			if r.owner == "net/minecraftforge/fml/ModContainer" && r.desc == "(Lnet/minecraftforge/fml/config/ModConfig;)V" {
				return nil, true
			}
			return nil, false
		case "getActiveContainer":
			return v.alloc("net/minecraftforge/fml/ModContainer", nil), true
		case "getModEventBus", "getEventBus":
			return v.alloc("net/minecraftforge/eventbus/api/IEventBus", nil), true
		case "registerConfig":
			if len(args) != 2 && len(args) != 3 {
				return nil, false
			}
			kind, ok := objectData(args[0]).(string)
			if !ok || kind != "COMMON" && kind != "CLIENT" && kind != "SERVER" && kind != "STARTUP" {
				fail("unknown configuration kind %q", kind)
			}
			name := h.options.ModID + "-" + strings.ToLower(kind) + ".toml"
			if len(args) == 3 {
				name = args[2].(string)
			} else if h.options.ModID == "" {
				fail("configuration default filename requires a mod identity")
			}
			spec := args[1].(*object)
			if kind == "SERVER" {
				name = "serverconfig/" + name
			}
			h.configure(name, spec)
			return nil, true
		}
	}
	if r.owner == "net/minecraftforge/fml/javafmlmod/FMLJavaModLoadingContext" {
		if r.name == "get" {
			return v.alloc(r.owner, nil), true
		}
		if r.name == "getModEventBus" {
			return v.alloc("net/minecraftforge/eventbus/api/IEventBus", nil), true
		}
	}
	// Private buses (BusBuilder: SimpleChannel / NetworkInstance) share the
	// recorded-listener bus. Worldgen never posts their events, so this only
	// avoids running EventBus internals (ModLauncher listener lists).
	if r.owner == "net/minecraftforge/eventbus/api/BusBuilder" {
		switch {
		case r.name == "builder" && receiver == nil:
			return v.alloc("net/minecraftforge/eventbus/api/BusBuilder", nil), true
		case r.name == "build" && r.desc == "()Lnet/minecraftforge/eventbus/api/IEventBus;":
			return v.alloc("net/minecraftforge/eventbus/api/IEventBus", nil), true
		case strings.HasSuffix(r.desc, ")Lnet/minecraftforge/eventbus/api/BusBuilder;") && receiver != nil:
			return receiver, true
		}
	}
	if r.owner == "net/minecraftforge/eventbus/api/IEventBus" && r.name == "addListener" {
		h.addListener(args)
		return nil, true
	}
	// NumberFormat.getNumberInstance: the JDK's decimal pattern (grouping,
	// at most three fraction digits, HALF_EVEN). Only text formatting is bound.
	if (r.owner == "java/text/DecimalFormat" || r.owner == "java/text/NumberFormat") && (r.name == "getNumberInstance" || r.name == "getInstance") && (r.desc == "(Ljava/util/Locale;)Ljava/text/NumberFormat;" || r.desc == "()Ljava/text/NumberFormat;") {
		return v.alloc("java/text/NumberFormat", nil), true
	}
	if r.owner == "java/text/NumberFormat" && r.name == "format" && (r.desc == "(D)Ljava/lang/String;" || r.desc == "(J)Ljava/lang/String;") {
		var f float64
		switch x := args[0].(type) {
		case float64:
			f = x
		case int64:
			f = float64(x)
		}
		return javaNumberFormat(f), true
	}
	// Iterable.forEach default method for interpreted iterables (for example
	// Guava's concatenated views) that inherit it from the JDK interface.
	if (r.owner == "java/lang/Object" || r.owner == "java/lang/Iterable") && r.name == "forEach" && r.desc == "(Ljava/util/function/Consumer;)V" {
		if o, ok := receiver.(*object); ok && o != nil && o.data == nil && v.fetchClass(o.class) {
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "forEach")
			}
			it := v.method(reference{o.class, "iterator", "()Ljava/util/Iterator;"}, o, true, nil)
			for i := 0; integer(v.method(reference{"java/util/Iterator", "hasNext", "()Z"}, it, true, nil)) != 0; i++ {
				v.tick()
				next := v.method(reference{"java/util/Iterator", "next", "()Ljava/lang/Object;"}, it, true, nil)
				v.method(reference{"java/util/function/Consumer", "accept", "(Ljava/lang/Object;)V"}, args[0], true, []any{next})
			}
			return nil, true
		}
	}
	if r.owner == "java/lang/String" && r.name == "join" && (r.desc == "(Ljava/lang/CharSequence;Ljava/lang/Iterable;)Ljava/lang/String;" || r.desc == "(Ljava/lang/CharSequence;[Ljava/lang/CharSequence;)Ljava/lang/String;") {
		if args[0] == nil || args[1] == nil {
			raiseJava("java/lang/NullPointerException", "join")
		}
		var items []any
		if a, ok := args[1].(*array); ok {
			items = a.values
		} else if l, ok := objectData(args[1]).(*javaList); ok {
			items = l.values
		} else {
			it := v.method(reference{"java/lang/Iterable", "iterator", "()Ljava/util/Iterator;"}, args[1], true, nil)
			for integer(v.method(reference{"java/util/Iterator", "hasNext", "()Z"}, it, true, nil)) != 0 {
				v.tick()
				items = append(items, v.method(reference{"java/util/Iterator", "next", "()Ljava/lang/Object;"}, it, true, nil))
			}
		}
		if i := inertOf(append([]any{args[0]}, items...)...); i != nil {
			return v.inert("String.join of "+i.origin, true), true
		}
		parts := make([]string, len(items))
		for k, x := range items {
			if x == nil {
				parts[k] = "null"
			} else {
				parts[k] = javaString(v.method(reference{"java/lang/Object", "toString", "()Ljava/lang/String;"}, x, true, nil))
			}
		}
		return strings.Join(parts, javaString(v.method(reference{"java/lang/Object", "toString", "()Ljava/lang/String;"}, args[0], true, nil))), true
	}
	// Zeta's early Forge config load reflects into FML internals to register
	// the spec as the mod's COMMON config and read the TOML immediately. The
	// observable effect is the same as registerConfig(COMMON, spec).
	if r.owner == "org/violetmoon/zetaimplforge/config/TerribleForgeConfigHackery" && r.name == "registerAndLoadConfigEarlierThanUsual" && r.desc == "(Lnet/minecraftforge/common/ForgeConfigSpec;)V" {
		spec, ok := args[0].(*object)
		if !ok || h.options.ModID == "" {
			fail("early Zeta configuration requires a spec and mod identity")
		}
		h.configure(h.options.ModID+"-common.toml", spec)
		return nil, true
	}
	// Registration runs on one thread, named like FML's mod-loading worker.
	if r.owner == "java/lang/Thread" && r.name == "currentThread" && r.desc == "()Ljava/lang/Thread;" {
		return v.alloc("java/lang/Thread", "modloading-worker-0"), true
	}
	// Background threads a mod starts while constructing (update checks,
	// supporter lists) are never run: their work is asynchronous to, and not
	// awaited by, worldgen registration.
	if r.owner == "java/lang/Thread" && receiver != nil {
		switch r.name + r.desc {
		case "<init>(Ljava/lang/Runnable;)V", "<init>(Ljava/lang/Runnable;Ljava/lang/String;)V", "<init>()V":
			receiver.(*object).data = "Thread-0"
			return nil, true
		case "setDaemon(Z)V", "setName(Ljava/lang/String;)V", "setPriority(I)V", "start()V", "setUncaughtExceptionHandler(Ljava/lang/Thread$UncaughtExceptionHandler;)V":
			return nil, true
		}
	}
	if r.owner == "java/lang/Thread" && r.name == "getName" && r.desc == "()Ljava/lang/String;" {
		if s, ok := objectData(receiver).(string); ok {
			return s, true
		}
	}
	// Direct IForgeRegistry.register outside a RegisterEvent: registering a
	// non-worldgen game object (tree decorators, items) has no effect on biome
	// layout, so the entry is accepted without running ForgeRegistry internals.
	if (r.owner == "net/minecraftforge/registries/IForgeRegistry" || r.owner == "net/minecraftforge/registries/ForgeRegistry") && r.name == "register" && (r.desc == "(Ljava/lang/String;Ljava/lang/Object;)V" || r.desc == "(Lnet/minecraft/resources/ResourceLocation;Ljava/lang/Object;)V") {
		return nil, true
	}
	// RegistryObject.get before its RegisterEvent fired: the game object
	// (block, item) is unavailable here, not absent, so it is inert instead
	// of Forge's "Registry Object not present" NPE. Region code may carry it
	// into surface rules; biome placement never inspects it.
	if r.owner == "net/minecraftforge/registries/RegistryObject" && r.name == "get" && r.desc == "()Ljava/lang/Object;" {
		if o, ok := receiver.(*object); ok && o != nil && o.class == r.owner {
			key := v.instanceField(reference{r.owner, "value", "Ljava/lang/Object;"}, o)
			if x, exists := o.fields[key]; !exists || x == nil || placeholderObject(x) {
				// One RegistryObject names one registered instance for the
				// whole run, like a static Blocks/Items field: identity-keyed.
				name := "?"
				if no, ok := o.fields[v.instanceField(reference{r.owner, "name", "Lnet/minecraft/resources/ResourceLocation;"}, o)].(*object); ok && no != nil {
					id, _ := no.data.(string)
					name = id
				}
				return v.inert("static RegistryObject "+name, true), true
			}
		}
	}
	if r.owner == "net/minecraftforge/eventbus/api/IEventBus" && r.name == "post" && r.desc == "(Lnet/minecraftforge/eventbus/api/Event;)Z" {
		// Deliver to listeners registered for the event's exact class; events
		// built from unavailable game values reach no listener.
		if o, ok := args[0].(*object); ok && o != nil {
			h.fireEvent("L"+o.class+";", o)
			if v.fetchClass(o.class) {
				for _, m := range v.load(o.class).methods {
					if m.name == "isCanceled" && m.desc == "()Z" {
						return v.method(reference{o.class, "isCanceled", "()Z"}, o, true, nil), true
					}
				}
			}
		} else if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "null event")
		}
		return int32(0), true
	}
	if r.owner == "net/minecraftforge/eventbus/api/IEventBus" && r.name == "register" && r.desc == "(Ljava/lang/Object;)V" {
		h.subscribe(args[0])
		return nil, true
	}
	if (r.owner == "net/minecraftforge/fml/event/config/ModConfigEvent" || r.owner == "net/minecraftforge/fml/event/config/ModConfigEvent$Loading") && r.name == "getConfig" {
		return receiver.(*object).data, true
	}
	if r.owner == "net/minecraftforge/fml/config/ModConfig" && r.name == "getSpec" {
		return receiver.(*object).data, true
	}
	if r.owner == "net/minecraftforge/fml/event/lifecycle/FMLCommonSetupEvent" && r.name == "enqueueWork" {
		if len(h.work) >= 1024 {
			fail("deferred registration work budget exceeded")
		}
		h.work = append(h.work, queuedWork{args[0].(*lambda), h.options.ModID})
		return v.alloc("java/util/concurrent/CompletableFuture", nil), true
	}
	return nil, false
}

func (h *bindings) configure(name string, spec *object) {
	if len(name) > 256 || strings.ContainsAny(name, "\\:") || strings.HasPrefix(name, "/") || strings.Contains(name, "..") || !strings.HasSuffix(name, ".toml") {
		fail("invalid configuration resource name")
	}
	if len(h.configNames) >= 512 {
		fail("too many configuration resources")
	}
	h.configNames = append(h.configNames, name)
	h.configs = append(h.configs, spec)
	if h.configOwners == nil {
		h.configOwners = map[*object]string{}
	}
	h.configOwners[spec] = h.options.ModID
	var data map[string]any
	if h.options.Config != nil {
		raw, err := h.options.Config(name)
		if err != nil {
			fail("configuration %s: %v", name, err)
		}
		if len(raw) > 512<<10 { // large modpack configs (quark-common.toml ~80 KiB)
			fail("configuration resource too large")
		}
		if len(raw) > 0 {
			if err := toml.Unmarshal(raw, &data); err != nil {
				fail("configuration %s: %v", name, err)
			}
		}
	}
	for _, value := range spec.data.(*configSpec).values {
		var x any = data
		for _, key := range value.path {
			m, ok := x.(map[string]any)
			if !ok {
				x = nil
				break
			}
			x = m[key]
		}
		if x == nil {
			continue
		}
		if value.opaque {
			// Matching the default's shape does not prove that an unexecuted
			// Java validator accepts an override. Keep the prediction opaque.
			value.value = h.vm.inert("config override "+strings.Join(value.path, "."), true)
			continue
		}
		switch def := value.def.(type) {
		case string:
			if s, ok := x.(string); ok {
				value.value = s
			}
		case float64:
			var f float64
			switch n := x.(type) {
			case float64:
				f = n
			case int64:
				f = float64(n)
			default:
				continue
			}
			if !math.IsNaN(f) && (value.min == nil || f >= number(value.min) && f <= number(value.max)) {
				value.value = f
			}
		case int32:
			if value.boolean {
				if b, ok := x.(bool); ok {
					if b {
						value.value = int32(1)
					} else {
						value.value = int32(0)
					}
				}
				continue
			}
			if n, ok := x.(int64); ok && n >= math.MinInt32 && n <= math.MaxInt32 && (value.min == nil || float64(n) >= number(value.min) && float64(n) <= number(value.max)) {
				value.value = int32(n)
			}
		case int64:
			if n, ok := x.(int64); ok && (value.min == nil || n >= value.min.(int64) && n <= value.max.(int64)) {
				value.value = n
			}
		default:
			fail("unsupported configuration default %T (%v)", def, fmt.Sprint(value.path))
		}
	}
}

// genericConfigValue records any config definition overload (lists, enums,
// predicates, supplier defaults) by its default value. The default is what a
// fresh install sees, so worldgen code reading it observes real behaviour.
func (h *bindings) genericConfigValue(b *configBuilder, r reference, args []any) any {
	v := h.vm
	if len(b.entries) >= 8192 {
		fail("configuration value budget exceeded")
	}
	def := args[1]
	if l, ok := def.(*lambda); ok {
		params, _ := descriptor(r.desc)
		if len(params) > 1 && params[1] == "Ljava/util/function/Supplier;" {
			def = v.callLambda(l, nil)
		}
	}
	path := ""
	switch x := args[0].(type) {
	case string:
		path = x
	default:
		path = "list"
	}
	value := &configValue{path: append(append([]string(nil), b.path...), path), def: unbox(def), value: unbox(def), opaque: true}
	if boxed, ok := def.(*object); ok && boxed != nil && boxed.class == "java/lang/Boolean" {
		value.boolean = true
	}
	b.entries = append(b.entries, value)
	_, ret := descriptor(r.desc)
	return v.alloc(strings.TrimSuffix(strings.TrimPrefix(ret, "L"), ";"), value)
}

func loggerReceiver(x any) bool {
	o, ok := x.(*object)
	if !ok || o == nil {
		return false
	}
	return strings.HasPrefix(o.class, "org/apache/logging/log4j/") || strings.HasPrefix(o.class, "org/slf4j/") || o.class == "org/apache/logging/log4j/Logger" || o.class == "org/slf4j/Logger"
}

// parseBoxedString implements the String overloads of the boxed valueOf
// factories, raising NumberFormatException exactly where Java does.
func parseBoxedString(box, text string) any {
	switch box {
	case "Boolean":
		if strings.EqualFold(text, "true") {
			return int32(1)
		}
		return int32(0)
	case "Integer":
		n, err := strconv.ParseInt(text, 10, 32)
		if err != nil {
			raiseJava("java/lang/NumberFormatException", "For input string: \""+text+"\"")
		}
		return int32(n)
	case "Long":
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			raiseJava("java/lang/NumberFormatException", "For input string: \""+text+"\"")
		}
		return n
	case "Float", "Double":
		bits := 64
		if box == "Float" {
			bits = 32
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimRight(text, "dDfF")), bits)
		if err != nil {
			raiseJava("java/lang/NumberFormatException", "For input string: \""+text+"\"")
		}
		if box == "Float" {
			return float32(f)
		}
		return f
	}
	fail("valueOf(String) on java/lang/Number")
	return nil
}

// loggerName is the name a logger factory assigns: the given string, the
// binary name of the given class, or log4j's caller-less default "".
func loggerName(args []any) string {
	if len(args) == 0 || args[0] == nil {
		return ""
	}
	if name, ok := args[0].(string); ok {
		return name
	}
	if c, ok := args[0].(classLiteral); ok {
		return strings.ReplaceAll(string(c), "/", ".")
	}
	if o, ok := args[0].(*object); ok {
		if name, ok := o.data.(string); ok {
			return name
		}
	}
	return ""
}

func javaNumberFormat(f float64) string {
	if math.IsNaN(f) {
		return "NaN"
	}
	if math.IsInf(f, 0) {
		if f < 0 {
			return "-∞"
		}
		return "∞"
	}
	s := strconv.FormatFloat(math.RoundToEven(f*1000)/1000, 'f', 3, 64)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	whole, frac, _ := strings.Cut(s, ".")
	frac = strings.TrimRight(frac, "0")
	var g strings.Builder
	for i, c := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			g.WriteByte(',')
		}
		g.WriteRune(c)
	}
	out := g.String()
	if frac != "" {
		out += "." + frac
	}
	if neg && strings.Trim(out, "0.,") != "" {
		out = "-" + out
	}
	return out
}

// placeholderObject is a bare java.lang.Object standing in for registered
// content whose real game instance is unavailable.
func placeholderObject(x any) bool {
	o, ok := x.(*object)
	return ok && o != nil && o.class == "java/lang/Object"
}
