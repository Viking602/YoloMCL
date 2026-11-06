package modjar

import (
	"context"
	"strings"
	"testing"
)

const officialSymbolFixture = `# independent mapping fixture
net.minecraft.resources.ResourceLocation -> a:
    12:13:net.minecraft.resources.ResourceLocation fromNamespaceAndPath(java.lang.String,java.lang.String):14:15 -> a
    16:16:java.lang.String getNamespace() -> b
    17:17:java.lang.String getPath() -> c
    18:18:java.lang.String nested.Helper.inlined() -> c
example.Value -> b:
    int first -> a
    java.lang.String second -> a
    int number(int[]) -> b
    int number(example.Value[][]) -> b
`

func TestGameSymbolNamespaces(t *testing.T) {
	s, err := ReadOfficialSymbols(strings.NewReader(officialSymbolFixture))
	if err != nil {
		t.Fatal(err)
	}
	tiny := "v1\tofficial\tintermediary\nCLASS\ta\tnet/minecraft/class_1\nCLASS\tb\tnet/minecraft/class_2\nMETHOD\ta\t(Ljava/lang/String;Ljava/lang/String;)La;\ta\tmethod_1\nMETHOD\tb\t([I)I\tb\tmethod_2\nMETHOD\tb\t([[Lb;)I\tb\tmethod_3\nFIELD\tb\tI\ta\tfield_1\nFIELD\tb\tLjava/lang/String;\ta\tfield_2\n"
	if err := s.AddIntermediary(strings.NewReader(tiny)); err != nil {
		t.Fatal(err)
	}
	srg := "tsrg2 obf srg id\na net/minecraft/src/C_1_ 1\n\ta (Ljava/lang/String;Ljava/lang/String;)La; m_1_ 1\n\t\tstatic\nb net/minecraft/src/C_2_ 2\n\ta f_1_ 1\n\tb ([I)I m_2_ 2\n\t\t0 x p_1_ 1\n"
	if err := s.AddSRG(strings.NewReader(srg)); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ from, want reference }{
		{reference{"net/minecraft/class_1", "method_1", "(Ljava/lang/String;Ljava/lang/String;)Lnet/minecraft/class_1;"}, reference{"net/minecraft/resources/ResourceLocation", "fromNamespaceAndPath", "(Ljava/lang/String;Ljava/lang/String;)Lnet/minecraft/resources/ResourceLocation;"}},
		{reference{"net/minecraft/class_2", "method_3", "([[Lnet/minecraft/class_2;)I"}, reference{"example/Value", "number", "([[Lexample/Value;)I"}},
		{reference{"example/Value", "f_1_", "I"}, reference{"example/Value", "first", "I"}},
		{reference{"net/minecraft/src/C_2_", "f_1_", "Ljava/lang/String;"}, reference{"example/Value", "second", "Ljava/lang/String;"}},
		{reference{"example/Value", "m_2_", "([I)I"}, reference{"example/Value", "number", "([I)I"}},
	} {
		if got := s.ref(tc.from); got != tc.want {
			t.Errorf("%+v -> %+v, want %+v", tc.from, got, tc.want)
		}
	}
	if got := s.typeName("[[La;"); got != "[[Lnet/minecraft/resources/ResourceLocation;" {
		t.Fatal(got)
	}
	if _, err := ReadOfficialSymbols(strings.NewReader("Bad -> a:\n    int x -> a\n    int y -> a\n")); err == nil {
		t.Fatal("conflicting member accepted")
	}
}

func TestMappedBytecodeCallsNativeHostAndPreservesStrings(t *testing.T) {
	s, err := ReadOfficialSymbols(strings.NewReader(officialSymbolFixture))
	if err != nil {
		t.Fatal(err)
	}
	c := &fixtureClass{}
	code := append(c.string("example"), c.string("forest")...)
	code = append(code, c.instruction(184, "a", "a", "(Ljava/lang/String;Ljava/lang/String;)La;")...)
	code = append(code, c.instruction(182, "a", "c", "()Ljava/lang/String;")...)
	c.method("run", "()Ljava/lang/String;", 9, append(code, 176))
	c.method("literal", "()Ljava/lang/String;", 9, append(c.string("[[La;"), 176))
	c.method("type", "()Ljava/lang/Class;", 9, append(append([]byte{19}, words(c.klass("[[La;"))...), 176))
	v := newMachine(context.Background(), map[string][]byte{"example/Main": c.bytes("example/Main", "java/lang/Object")})
	v.host = &bindings{vm: v, options: Options{Minecraft: "1.21.1", Symbols: s}}
	v.host.extra = &biomeProbeBindings{host: v.host}
	for _, tc := range []struct {
		name, desc string
		want       any
	}{
		{"run", "()Ljava/lang/String;", "forest"},
		{"literal", "()Ljava/lang/String;", "[[La;"},
		{"type", "()Ljava/lang/Class;", classLiteral("[[Lnet/minecraft/resources/ResourceLocation;")},
	} {
		if got := v.method(reference{"example/Main", tc.name, tc.desc}, nil, false, nil); got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, got, tc.want)
		}
	}
}
