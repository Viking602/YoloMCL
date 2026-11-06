package modjar

import "strings"

// inertValue stands for a host game value whose implementation is not part of
// the executable classpath. The game JAR is never loaded, so a static field,
// constructor or method result from the game namespace has no observable
// value here. Registration may store, pass and return such a value, but any
// decision that depends on it (branch, switch, type test, throw, array access,
// hashing, or a native API that needs its contents) fails closed. A region,
// biome key or climate parameter built from an inert value is therefore
// rejected by the native region API rather than guessed.
type inertValue struct {
	origin  string
	nonNull bool
}

// gameNamespace reports classes provided by the game JAR itself. Libraries,
// loader APIs and the JDK must resolve to real bytecode or native bindings.
func gameNamespace(name string) bool {
	return strings.HasPrefix(name, "net/minecraft/") || strings.HasPrefix(name, "com/mojang/blaze3d/") || strings.HasPrefix(name, "com/mojang/math/") || strings.HasPrefix(name, "com/mojang/realmsclient/")
}

// hostOpaque reports a game class that is absent from the classpath.
func (v *machine) hostOpaque(name string) bool {
	name = frameworkName(name)
	return gameNamespace(name) && !v.fetchClass(name)
}

func (v *machine) inert(origin string, nonNull bool) *inertValue {
	v.objects++
	if v.objects > 262144 {
		fail("registration object budget exceeded")
	}
	if v.inertOrigins == nil {
		v.inertOrigins = map[string]int{}
	}
	v.inertOrigins[origin]++
	return &inertValue{origin: origin, nonNull: nonNull}
}

func inertOf(values ...any) *inertValue {
	for _, x := range values {
		if i, ok := x.(*inertValue); ok {
			return i
		}
	}
	return nil
}

func requireDetermined(what string, values ...any) {
	if i := inertOf(values...); i != nil {
		fail("%s depends on unavailable game value %s", what, i.origin)
	}
}

// frozenSystemPropertyKeys lists every key frozenSystemProperty defines.
var frozenSystemPropertyKeys = []string{
	"java.version", "java.vm.version", "java.specification.version", "java.vm.name",
	"java.vendor", "java.vm.vendor", "java.home", "os.name", "os.arch", "os.version",
	"file.separator", "path.separator", "line.separator", "user.language",
	"user.country", "user.region", "file.encoding", "native.encoding",
	"stdout.encoding", "stderr.encoding", "java.class.path", "java.library.path",
	"sun.boot.class.path",
}

func frozenSystemProperty(key string) (string, bool) {
	switch key {
	case "java.version", "java.vm.version":
		return "21.0.0", true
	case "java.specification.version":
		return "21", true
	case "java.vm.name":
		return "YoloMCL Registration VM", true
	case "java.vendor", "java.vm.vendor":
		return "YoloMCL", true
	case "java.home":
		return "/yolomcl/java", true
	case "os.name":
		return "Linux", true
	case "os.arch":
		return "amd64", true
	case "os.version":
		return "6.0", true
	case "file.separator":
		return "/", true
	case "path.separator":
		return ":", true
	case "line.separator":
		return "\n", true
	case "user.language":
		return "en", true
	case "user.country", "user.region":
		return "US", true
	case "file.encoding", "native.encoding", "stdout.encoding", "stderr.encoding":
		return "UTF-8", true
	case "java.class.path", "java.library.path", "sun.boot.class.path":
		return "", true
	default:
		// Unknown properties stay unset so mods that branch on them fail closed
		// when they require a specific host value.
		return "", false
	}
}
