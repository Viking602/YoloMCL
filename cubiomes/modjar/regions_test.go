package modjar

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/cubiomes"
)

func TestFrameworkRegistration(t *testing.T) {
	for _, namespace := range []string{"example", "another_mod"} {
		t.Run(namespace, func(t *testing.T) {
			result, err := ReadTerraBlender(context.Background(), regionFixture(t, namespace, true, false), Options{Minecraft: "1.20.1"})
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Regions) != 1 {
				t.Fatalf("registrations: %+v", result)
			}
			r := result.Regions[0]
			if r.ID != namespace+":region" || r.Dimension != cubiomes.Overworld || r.Weight != 3 || len(r.Parameters) != 1 || r.Parameters[0].Biome != namespace+":biome" || r.Parameters[0].Space != ([7][2]int64{}) {
				t.Fatalf("bad dynamic registration: %+v", r)
			}
		})
	}
	result, err := ReadTerraBlender(context.Background(), regionFixture(t, "example", false, false), Options{Minecraft: "1.20.1"})
	if err != nil || len(result.Regions) != 0 || result.Steps == 0 {
		t.Fatalf("conditional registration ignored: %+v, %v", result, err)
	}
}

func TestRegistrationRejectsSideEffectsAndCancels(t *testing.T) {
	result, err := ReadTerraBlender(context.Background(), regionFixture(t, "example", true, true), Options{Minecraft: "1.20.1"})
	if err == nil || !strings.Contains(err.Error(), "forbidden registration API") || len(result.Regions) != 0 {
		t.Fatalf("partially executed side effect: %+v %v", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = ReadTerraBlender(ctx, regionFixture(t, "example", true, false), Options{Minecraft: "1.20.1"})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	_, err = ReadTerraBlender(context.Background(), regionFixture(t, "example", true, false), Options{})
	if err == nil {
		t.Fatal("missing Minecraft version accepted")
	}
	_, err = ReadTerraBlender(context.Background(), fixtureArchive(t, map[string][]byte{"bad.class": []byte("net/minecraftforge/eventbus/api/IEventBus")}), Options{Minecraft: "1.20.1"})
	if err == nil {
		t.Fatal("malformed class accepted")
	}
}

func expectFault(t *testing.T, contains string, fn func()) {
	t.Helper()
	defer func() {
		p := recover()
		if p == nil {
			t.Fatal("expected bounded failure")
		}
		f, ok := p.(fault)
		if !ok || !strings.Contains(f.message, contains) {
			t.Fatalf("unexpected failure: %v", p)
		}
	}()
	fn()
}

func TestRegistrationInstructionBudget(t *testing.T) {
	v := newMachine(context.Background(), nil)
	expectFault(t, "instruction budget", func() {
		v.run(&class{name: "Loop"}, &method{name: "run", access: 8, code: []byte{167, 0, 0}}, nil, nil)
	})
	if v.steps != 20_000_001 {
		t.Fatalf("steps %d", v.steps)
	}
}

func TestJavaNumericSemantics(t *testing.T) {
	checks := []struct{ got, want any }{
		{arithmetic(96, int32(math.MaxInt32), int32(1)), int32(math.MinInt32)},
		{arithmetic(109, int64(math.MinInt64), int64(-1)), int64(math.MinInt64)},
		{arithmetic(112, int32(-5), int32(3)), int32(-2)},
		{bitwise(124, int32(-1), int32(33)), int32(math.MaxInt32)},
		{bitwise(121, int64(3), int32(65)), int64(6)},
		{convertNumber(136, int64(1)<<40|3), int32(3)},
		{convertNumber(142, math.NaN()), int32(0)},
		{convertNumber(143, math.Inf(1)), int64(math.MaxInt64)},
		{convertNumber(145, int32(255)), int32(-1)},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Fatalf("got %v (%T), want %v (%T)", c.got, c.got, c.want, c.want)
		}
	}
	expectJavaException(t, "java/lang/ArithmeticException", func() { arithmetic(108, int32(1), int32(0)) })
	v := newMachine(context.Background(), nil)
	h := &bindings{vm: v}
	v.host = h
	large := int64(1)<<60 | 3
	for method, want := range map[string]any{"longValue": large, "intValue": int32(3)} {
		got, ok := h.call(reference{owner: "java/lang/Long", name: method}, v.alloc("java/lang/Long", large), nil)
		if !ok || got != want {
			t.Fatalf("boxed %s lost precision: %v", method, got)
		}
	}
}

func TestRegistrationConfigIsolationAndDefaults(t *testing.T) {
	v := newMachine(context.Background(), nil)
	h := &bindings{vm: v, options: Options{Config: func(string) ([]byte, error) {
		return []byte("[region]\nweight=999\nenabled=2\ntemperature=0.25\n"), nil
	}}}
	v.host = h
	weight := &configValue{path: []string{"region", "weight"}, def: int32(3), value: int32(3), min: int32(0), max: int32(10)}
	enabled := &configValue{path: []string{"region", "enabled"}, def: int32(1), value: int32(1), boolean: true}
	temperature := &configValue{path: []string{"region", "temperature"}, def: float64(-.7), value: float64(-.7), min: float64(-1), max: float64(1)}
	spec := v.alloc("net/minecraftforge/common/ForgeConfigSpec", &configSpec{[]*configValue{weight, enabled, temperature}})
	h.configure("regional/settings.toml", spec)
	if weight.value != int32(3) || enabled.value != int32(1) || temperature.value != float64(.25) {
		t.Fatal("Forge configuration correction/default semantics changed")
	}
	for _, name := range []string{"../private.toml", "C:/private.toml", "/private.toml", "network.txt"} {
		expectFault(t, "invalid configuration resource name", func() { h.configure(name, spec) })
	}
	const owner = "net/minecraftforge/common/ForgeConfigSpec$Builder"
	builder := v.alloc(owner, &configBuilder{})
	for _, ref := range []reference{
		{owner, "define", "(Ljava/lang/String;Ljava/lang/Object;Ljava/util/function/Predicate;)Lnet/minecraftforge/common/ForgeConfigSpec$ConfigValue;"},
		{owner, "defineInRange", "(Ljava/lang/String;Ljava/util/function/Supplier;III)Lnet/minecraftforge/common/ForgeConfigSpec$IntValue;"},
	} {
		if _, accepted := h.call(ref, builder, nil); accepted {
			t.Fatalf("unimplemented config overload accepted: %s", ref.desc)
		}
	}
}

func FuzzRegistrationClass(f *testing.F) {
	c := &fixtureClass{}
	c.utf("net/minecraftforge/eventbus/api/IEventBus")
	c.method("unused", "()V", 9, []byte{177})
	f.Add(c.bytes("example/Fuzz", "java/lang/Object"))
	f.Add([]byte("invalid net/minecraftforge/eventbus/api/IEventBus"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 2<<20 {
			t.Skip()
		}
		ReadTerraBlender(context.Background(), fixtureArchive(t, map[string][]byte{"example/Fuzz.class": data}), Options{Minecraft: "1.20.1"})
	})
}

const variantConfig = `["YUNG's Cave Biomes"."Frosted Caves"."Biome Noise Parameters"]
"Temperature Min" = -0.25
"Temperature Max" = -0.15
["YUNG's Cave Biomes"."Lost Caves"."Biome Noise Parameters"]
"Depth Min" = 0.4
"Depth Max" = 0.7
["YUNG's Cave Biomes".Other."Vanilla Biome Modifications"."Dripstone Caves"]
"Temperature Min" = 0.2
"Temperature Max" = 0.8
`

func installedRegistrationJar(t testing.TB) *zip.ReadCloser {
	t.Helper()
	path := os.Getenv("MCL_REGISTRATION_JAR")
	if path == "" {
		t.Skip("MCL_REGISTRATION_JAR points to YungsCaveBiomes-1.20.1-Forge-2.0.5.jar")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha512.Sum512(b)) != "c09d75eec4a9e4245e2f0a7cb0b64b6da70ccd46b3d351c2d6baded3deb2f15f6980229be40cc85557b2ccd468f5a4d6b74bb28190723647076aba49bebcaf36" {
		t.Fatal("changed oracle JAR")
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { z.Close() })
	return z
}

// Digests come from normally compiled public YCB/TerraBlender source and the
// Minecraft 1.20.1 public API, not from this interpreter. See provenance.
func TestInstalledRegistrationSourceOracle(t *testing.T) {
	z := installedRegistrationJar(t)
	for _, tc := range []struct{ name, config, want string }{
		{"default", "", "06ae110c9a4311814c85cac66809cf2fd9dd47d79fbe1f695e1741d25697c163"},
		{"changed", variantConfig, "7250e377ae4c52dc8c365a2e514683826d6edb99d1aeacff72d4ca661d97f34c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ReadTerraBlender(context.Background(), &z.Reader, Options{Minecraft: "1.20.1", Config: func(name string) ([]byte, error) {
				if name != "YungsCaveBiomes-forge-1_20_1.toml" {
					t.Fatalf("unexpected config path %q", name)
				}
				return []byte(tc.config), nil
			}})
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Regions) != 1 || result.Regions[0].Weight != 3 || len(result.Regions[0].Parameters) != 7595 {
				t.Fatal("incomplete registration")
			}
			b, err := json.Marshal(result.Regions[0].Parameters)
			if err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(b)); got != tc.want {
				t.Fatalf("ordered parameters: %s want %s", got, tc.want)
			}
		})
	}
}

func BenchmarkInstalledRegistration(b *testing.B) {
	z := installedRegistrationJar(b)
	b.ReportAllocs()
	for b.Loop() {
		r, err := ReadTerraBlender(context.Background(), &z.Reader, Options{Minecraft: "1.20.1"})
		if err != nil || len(r.Regions) != 1 {
			b.Fatal(err)
		}
	}
}
