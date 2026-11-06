package modjar

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"testing"
)

// Tiny valid class files are assembled from the JVM specification. Tests do
// not compile Java, launch a JVM, or require a game/mod installation.
type fixtureClass struct {
	access     int
	pool       [][]byte
	methods    [][]byte
	fields     [][]byte
	boots      []byte
	attrs      [][]byte
	interfaces []string
}

func words(ns ...int) []byte {
	var b []byte
	for _, n := range ns {
		b = binary.BigEndian.AppendUint16(b, uint16(n))
	}
	return b
}
func (c *fixtureClass) cp(tag byte, data []byte) int {
	c.pool = append(c.pool, append([]byte{tag}, data...))
	return len(c.pool)
}
func (c *fixtureClass) utf(s string) int   { return c.cp(1, append(words(len(s)), []byte(s)...)) }
func (c *fixtureClass) klass(s string) int { return c.cp(7, words(c.utf(s))) }
func (c *fixtureClass) ref(tag byte, owner, name, desc string) int {
	ownerID := c.klass(owner)
	nt := c.cp(12, words(c.utf(name), c.utf(desc)))
	return c.cp(tag, words(ownerID, nt))
}
func (c *fixtureClass) instruction(op byte, owner, name, desc string) []byte {
	tag := byte(10)
	if op >= 178 && op <= 181 {
		tag = 9
	}
	if op == 185 {
		tag = 11
	}
	b := append([]byte{op}, words(c.ref(tag, owner, name, desc))...)
	if op == 185 {
		b = append(b, 2, 0)
	}
	return b
}
func (c *fixtureClass) string(s string) []byte {
	return append([]byte{19}, words(c.cp(8, words(c.utf(s))))...)
}
func (c *fixtureClass) newObject(name string) []byte {
	return append(append([]byte{187}, words(c.klass(name))...), 89)
}
func (c *fixtureClass) method(name, desc string, access int, code []byte, handlers ...exceptionHandler) {
	b := words(access, c.utf(name), c.utf(desc), 1, c.utf("Code"))
	body := words(24, 8)
	body = binary.BigEndian.AppendUint32(body, uint32(len(code)))
	body = append(body, code...)
	body = append(body, words(len(handlers))...)
	for _, h := range handlers {
		catch := 0
		if h.catch != "" {
			catch = c.klass(h.catch)
		}
		body = append(body, words(h.start, h.end, h.pc, catch)...)
	}
	body = append(body, words(0)...)
	b = binary.BigEndian.AppendUint32(b, uint32(len(body)))
	c.methods = append(c.methods, append(b, body...))
}
func (c *fixtureClass) field(name, desc string, access int) {
	c.fields = append(c.fields, words(access, c.utf(name), c.utf(desc), 0))
}
func (c *fixtureClass) bytes(name, super string) []byte {
	this, parent := c.klass(name), c.klass(super)
	var interfaces []int
	for _, name := range c.interfaces {
		interfaces = append(interfaces, c.klass(name))
	}
	bootName := 0
	if c.boots != nil {
		bootName = c.utf("BootstrapMethods")
	}
	b := []byte{0xca, 0xfe, 0xba, 0xbe, 0, 0, 0, 61}
	b = append(b, words(len(c.pool)+1)...)
	for _, p := range c.pool {
		b = append(b, p...)
	}
	access := c.access
	if access == 0 {
		access = 0x21
	}
	b = append(b, words(access, this, parent, len(interfaces))...)
	b = append(b, words(interfaces...)...)
	b = append(b, words(len(c.fields))...)
	for _, field := range c.fields {
		b = append(b, field...)
	}
	b = append(b, words(len(c.methods))...)
	for _, m := range c.methods {
		b = append(b, m...)
	}
	attrs := len(c.attrs)
	if c.boots != nil {
		attrs++
	}
	b = append(b, words(attrs)...)
	for _, attr := range c.attrs {
		b = append(b, attr...)
	}
	if c.boots == nil {
		return b
	}
	b = append(b, words(bootName)...)
	b = binary.BigEndian.AppendUint32(b, uint32(len(c.boots)))
	return append(b, c.boots...)
}
func fixtureArchive(t testing.TB, files map[string][]byte) *zip.Reader {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for name, data := range files {
		f, e := w.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = f.Write(data); e != nil {
			t.Fatal(e)
		}
	}
	if e := w.Close(); e != nil {
		t.Fatal(e)
	}
	z, e := zip.NewReader(bytes.NewReader(b.Bytes()), int64(b.Len()))
	if e != nil {
		t.Fatal(e)
	}
	return z
}

func regionFixture(t testing.TB, namespace string, enabled, forbidden bool) *zip.Reader {
	return regionFixtureOverrides(t, namespace, enabled, forbidden, nil)
}

func regionFixtureOverrides(t testing.TB, namespace string, enabled, forbidden bool, overrides func(*fixtureClass)) *zip.Reader {
	const provider = "example/Registration"
	const region = "example/Region"
	const setup = "(Lnet/minecraftforge/fml/event/lifecycle/FMLCommonSetupEvent;)V"
	const ctor = "(Lnet/minecraft/resources/ResourceLocation;Lterrablender/api/RegionType;I)V"
	const param = "Lnet/minecraft/world/level/biome/Climate$Parameter;"
	const point = "Lnet/minecraft/world/level/biome/Climate$ParameterPoint;"
	const key = "Lnet/minecraft/resources/ResourceKey;"
	const loc = "Lnet/minecraft/resources/ResourceLocation;"
	const consumer = "Ljava/util/function/Consumer;"
	p, r := &fixtureClass{}, &fixtureClass{}
	var code []byte
	code = append(code, p.instruction(184, "net/minecraftforge/fml/javafmlmod/FMLJavaModLoadingContext", "get", "()Lnet/minecraftforge/fml/javafmlmod/FMLJavaModLoadingContext;")...)
	code = append(code, p.instruction(182, "net/minecraftforge/fml/javafmlmod/FMLJavaModLoadingContext", "getModEventBus", "()Lnet/minecraftforge/eventbus/api/IEventBus;")...)
	meta := p.ref(10, "java/lang/invoke/LambdaMetafactory", "metafactory", "(Ljava/lang/invoke/MethodHandles$Lookup;Ljava/lang/String;Ljava/lang/invoke/MethodType;Ljava/lang/invoke/MethodType;Ljava/lang/invoke/MethodHandle;Ljava/lang/invoke/MethodType;)Ljava/lang/invoke/CallSite;")
	metaHandle := p.cp(15, append([]byte{6}, words(meta)...))
	target := p.cp(15, append([]byte{6}, words(p.ref(10, provider, "setup", setup))...))
	sam := p.cp(16, words(p.utf("(Ljava/lang/Object;)V")))
	actual := p.cp(16, words(p.utf(setup)))
	p.boots = words(1, metaHandle, 3, sam, target, actual)
	callsite := p.cp(18, words(0, p.cp(12, words(p.utf("accept"), p.utf("()"+consumer)))))
	code = append(code, 186)
	code = append(code, words(callsite)...)
	code = append(code, 0, 0)
	code = append(code, p.instruction(185, "net/minecraftforge/eventbus/api/IEventBus", "addListener", "("+consumer+")V")...)
	code = append(code, 177)
	p.method("initialize", "()V", 9, code)
	flag := byte(3)
	if enabled {
		flag = 4
	}
	code = []byte{flag, 153, 0, 0}
	code = append(code, p.newObject(region)...)
	code = append(code, p.newObject("net/minecraft/resources/ResourceLocation")...)
	code = append(code, p.string(namespace)...)
	code = append(code, p.string("region")...)
	code = append(code, p.instruction(183, "net/minecraft/resources/ResourceLocation", "<init>", "(Ljava/lang/String;Ljava/lang/String;)V")...)
	code = append(code, p.instruction(178, "terrablender/api/RegionType", "OVERWORLD", "Lterrablender/api/RegionType;")...)
	code = append(code, 6)
	code = append(code, p.instruction(183, region, "<init>", ctor)...)
	code = append(code, p.instruction(184, "terrablender/api/Regions", "register", "(Lterrablender/api/Region;)V")...)
	code = append(code, 177)
	binary.BigEndian.PutUint16(code[2:4], uint16(len(code)-2))
	p.method("setup", setup, 9, code)
	code = []byte{42, 43, 44, 29}
	code = append(code, r.instruction(183, "terrablender/api/Region", "<init>", ctor)...)
	code = append(code, 177)
	r.method("<init>", ctor, 1, code)
	code = []byte{11}
	code = append(code, r.instruction(184, "net/minecraft/world/level/biome/Climate$Parameter", "point", "(F)"+param)...)
	code = append(code, 78, 42, 44)
	code = append(code, 45, 45, 45, 45, 45, 45, 11)
	code = append(code, r.instruction(184, "net/minecraft/world/level/biome/Climate", "parameters", "("+param+param+param+param+param+param+"F)"+point)...)
	code = append(code, r.instruction(178, "net/minecraft/core/registries/Registries", "BIOME", key)...)
	code = append(code, r.newObject("net/minecraft/resources/ResourceLocation")...)
	code = append(code, r.string(namespace)...)
	code = append(code, r.string("biome")...)
	code = append(code, r.instruction(183, "net/minecraft/resources/ResourceLocation", "<init>", "(Ljava/lang/String;Ljava/lang/String;)V")...)
	code = append(code, r.instruction(184, "net/minecraft/resources/ResourceKey", "create", "("+key+loc+")"+key)...)
	code = append(code, r.instruction(182, "terrablender/api/Region", "addBiome", "("+consumer+point+key+")V")...)
	if forbidden {
		code = append(code, r.instruction(184, "java/lang/Runtime", "getRuntime", "()Ljava/lang/Runtime;")...)
		code = append(code, 87)
	}
	code = append(code, 177)
	r.method("addBiomes", "(Lnet/minecraft/core/Registry;"+consumer+")V", 1, code)
	if overrides != nil {
		overrides(r)
	}
	return fixtureArchive(t, map[string][]byte{provider + ".class": p.bytes(provider, "java/lang/Object"), region + ".class": r.bytes(region, "terrablender/api/Region")})
}
