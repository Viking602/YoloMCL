package modjar

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/cubiomes"
)

func biomeProbeFixture(t *testing.T, name string, forbidden bool) *zip.Reader {
	c := &fixtureClass{}
	const holder = "Lnet/minecraft/core/Holder;"
	const app = "Lcom/mojang/datafixers/kinds/App;"
	const instance = "Lcom/mojang/serialization/codecs/RecordCodecBuilder$Instance;"
	const codec = "Lcom/mojang/serialization/MapCodec;"
	c.field("CODEC", codec, 9)
	c.field("low", holder, 18)
	c.field("high", holder, 18)
	meta := c.ref(10, "java/lang/invoke/LambdaMetafactory", "metafactory", "(Ljava/lang/invoke/MethodHandles$Lookup;Ljava/lang/String;Ljava/lang/invoke/MethodType;Ljava/lang/invoke/MethodType;Ljava/lang/invoke/MethodHandle;Ljava/lang/invoke/MethodType;)Ljava/lang/invoke/CallSite;")
	metaHandle := c.cp(15, append([]byte{6}, words(meta)...))
	var bootData []byte
	makeLambda := func(index, kind int, target, desc, sam, actual, iface string) []byte {
		handle := c.cp(15, append([]byte{byte(kind)}, words(c.ref(10, name, target, desc))...))
		bootData = append(bootData, words(metaHandle, 3, c.cp(16, words(c.utf(sam))), handle, c.cp(16, words(c.utf(actual))))...)
		call := c.cp(18, words(index, c.cp(12, words(c.utf("apply"), c.utf("()L"+iface+";")))))
		return append(append([]byte{186}, words(call)...), 0, 0)
	}
	code := makeLambda(0, 6, "recipe", "("+instance+")"+app, "(Ljava/lang/Object;)Ljava/lang/Object;", "("+instance+")"+app, "java/util/function/Function")
	code = append(code, c.instruction(184, recordBuilderClass, "mapCodec", "(Ljava/util/function/Function;)"+codec)...)
	code = append(code, c.instruction(179, name, "CODEC", codec)...)
	code = append(code, 177)
	c.method("<clinit>", "()V", 8, code)
	code = []byte{42}
	for _, biome := range []string{"FOREST", "DESERT"} {
		code = append(code, c.instruction(178, "net/minecraft/world/level/biome/Biomes", biome, "Lnet/minecraft/resources/ResourceKey;")...)
		code = append(code, c.instruction(184, "net/minecraft/resources/RegistryOps", "retrieveElement", "(Lnet/minecraft/resources/ResourceKey;)L"+recordBuilderClass+";")...)
	}
	code = append(code, c.instruction(182, recordBuilderClass+"$Instance", "group", "("+app+app+")Lcom/mojang/datafixers/Products$P2;")...)
	code = append(code, 42, 42)
	code = append(code, makeLambda(1, 8, "<init>", "("+holder+holder+")V", "(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;", "("+holder+holder+")L"+name+";", "java/util/function/BiFunction")...)
	code = append(code, c.instruction(182, recordBuilderClass+"$Instance", "stable", "(Ljava/lang/Object;)"+app)...)
	code = append(code, c.instruction(182, "com/mojang/datafixers/Products$P2", "apply", "(Lcom/mojang/datafixers/kinds/Applicative;"+app+")"+app)...)
	code = append(code, 176)
	c.method("recipe", "("+instance+")"+app, 10, code)
	c.boots = append(words(2), bootData...)
	code = []byte{42}
	code = append(code, c.instruction(183, biomeSourceClass, "<init>", "()V")...)
	for i, field := range []string{"low", "high"} {
		code = append(code, 42, byte(43+i))
		code = append(code, c.instruction(181, name, field, holder)...)
	}
	code = append(code, 177)
	c.method("<init>", "("+holder+holder+")V", 1, code)
	code = []byte{27, 16, 16}
	code = append(code, c.instruction(184, "java/lang/Math", "floorDiv", "(II)I")...)
	code = append(code, 156, 0, 8, 42)
	code = append(code, c.instruction(180, name, "low", holder)...)
	code = append(code, 176)
	if forbidden {
		code = append(code, c.instruction(184, "java/lang/Runtime", "getRuntime", "()Ljava/lang/Runtime;")...)
		code = append(code, 87)
	}
	code = append(code, 42)
	code = append(code, c.instruction(180, name, "high", holder)...)
	code = append(code, 176)
	c.method("getNoiseBiome", "(IIIL"+climateSamplerClass+";)"+holder, 1, code)
	return fixtureArchive(t, map[string][]byte{name + ".class": c.bytes(name, biomeSourceClass)})
}

func TestBiomeSourceCodecExecutesArbitraryClass(t *testing.T) {
	for _, name := range []string{"unknown/CustomSource", "unrelated/OtherSource"} {
		t.Run(name, func(t *testing.T) {
			z := biomeProbeFixture(t, name, false)
			contracts, err := InspectBiomeSources(context.Background(), z)
			if err != nil || len(contracts) != 1 || contracts[0].Class != name || len(contracts[0].CodecFields) != 1 {
				t.Fatalf("discovery %+v %v", contracts, err)
			}
			q := []BiomeProbeQuery{{X: -1}, {X: 0}, {X: -16}, {X: 16}, {X: -17}}
			got, err := ProbeBiomeSource(context.Background(), z, Options{Minecraft: "1.21.1"}, name, "CODEC", q)
			want := []string{"minecraft:forest", "minecraft:desert", "minecraft:forest", "minecraft:desert", "minecraft:forest"}
			if err != nil || !slices.Equal(got.Biomes, want) {
				t.Fatalf("query %+v %v", got, err)
			}
		})
	}
}

func TestBiomeProgramValidatesFactoryStateAndAllQueryBranches(t *testing.T) {
	const name = "unrelated/RegisteredSource"
	for _, unbound := range []bool{false, true} {
		z := biomeProbeFixture(t, name, unbound)
		e, err := BootstrapBiomeSources(context.Background(), []Archive{{Name: "fixture", Reader: z}}, Options{Minecraft: "1.21.1", Loader: "neoforge"})
		if err != nil {
			t.Fatal(err)
		}
		defer e.Close()
		codec := e.vm.static(reference{name, "CODEC", "Lcom/mojang/serialization/MapCodec;"})
		e.api.register(biomeSourceRegistry, "test:registered", codec)
		program, err := e.BiomeProgram("test:registered")
		if unbound {
			if err == nil || !strings.Contains(err.Error(), "unresolved biome API") {
				t.Fatalf("unqueried unsafe branch accepted: %v", err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err = program.NewRuntime(context.Background()); err != nil {
			t.Fatal(err)
		}
		program.State[name+".low:Lnet/minecraft/core/Holder;"] = "H:test:different"
		if _, err = program.NewRuntime(context.Background()); err == nil || !strings.Contains(err.Error(), "uncaptured initialization state") {
			t.Fatalf("changed factory state accepted: %v", err)
		}
	}
}

func TestBiomeProbeFailureIsAtomic(t *testing.T) {
	const name = "test/Source"
	q := []BiomeProbeQuery{{X: -1}, {X: 1}}
	z := biomeProbeFixture(t, name, true)
	got, err := ProbeBiomeSource(context.Background(), z, Options{Minecraft: "1.21.1"}, name, "CODEC", q)
	if err == nil || !strings.Contains(err.Error(), "forbidden") || len(got.Biomes) != 0 || len(got.CodecBiomes) != 0 {
		t.Fatalf("partial result %+v %v", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ProbeBiomeSource(ctx, z, Options{Minecraft: "1.21.1"}, name, "CODEC", q); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := InspectBiomeSources(ctx, fixtureArchive(t, nil)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := ProbeBiomeSource(context.Background(), z, Options{Minecraft: "1.21.1"}, name, "CODEC", make([]BiomeProbeQuery, 4097)); err == nil {
		t.Fatal("unbounded batch")
	}
	files := archiveFiles(t, z)
	files["wrong.class"] = files[name+".class"]
	delete(files, name+".class")
	if _, err := InspectBiomeSources(context.Background(), fixtureArchive(t, files)); err == nil {
		t.Fatal("invalid class entry accepted")
	}
}

func TestBiomeProgramTransformScopeDoesNotIgnoreUsedNoise(t *testing.T) {
	const name = "unrelated/ScopedSource"
	z := biomeProbeFixture(t, name, false)
	e, err := BootstrapBiomeSources(context.Background(), []Archive{{Name: "fixture", Reader: z}}, Options{Minecraft: "1.21.1", Loader: "neoforge"})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	codec := e.vm.static(reference{name, "CODEC", "Lcom/mojang/serialization/MapCodec;"})
	e.api.register(biomeSourceRegistry, "test:scoped", codec)
	e.MixinEffects = []MixinEffect{{Target: "net/minecraft/world/level/levelgen/synth/PerlinNoise", Class: "unknown/Transform", Method: "sample", Kind: "Lorg/spongepowered/asm/mixin/Overwrite;"}}
	if _, err = e.BiomeProgram("test:scoped"); err == nil {
		t.Fatal("unknown scope ignored transform")
	}
	p, err := e.BiomeProgram("test:scoped", cubiomes.NoiseRequirements{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.BiomeProgram("test:scoped", cubiomes.NoiseRequirements{NormalNoise: true}); err == nil {
		t.Fatal("required noise transform ignored")
	}
	router, err := cubiomes.CompileNoiseRouter(context.Background(), nil, cubiomes.Definition{Version: "1.21.1", Settings: json.RawMessage(`{"noise_router":{"temperature":0,"vegetation":0,"continents":0,"erosion":{"type":"end_islands"},"depth":0,"ridges":0}}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.NewBiomeSampler(context.Background(), router.NewSampler()); err == nil || !strings.Contains(err.Error(), "scope") {
		t.Fatal("changed density graph accepted", err)
	}
	e.MixinEffects = []MixinEffect{{Target: name, Class: "unknown/Transform", Method: "irrelevant", Selectors: []string{"unusedMethod"}}}
	if _, err = e.BiomeProgram("test:scoped"); err != nil {
		t.Fatal("unreached helper method blocked query", err)
	}
	e.MixinEffects[0].Selectors = []string{"<init>"}
	if _, err = e.BiomeProgram("test:scoped"); err == nil {
		t.Fatal("source constructor transform ignored")
	}
}

// A numerical oracle for the installed closed-source artifact; no mod classes
// or implementation are distributed. The input generator is independent of
// the VM. See testdata for the isolated Java API reference environment/scope.
func biomeProbeQueries() []BiomeProbeQuery {
	var out []BiomeProbeQuery
	state := uint64(0xc0b10be5)
	next := func() uint64 { state ^= state << 13; state ^= state >> 7; state ^= state << 17; return state }
	for i := 0; i < 32768; i++ {
		q := BiomeProbeQuery{X: int32(next()%2097153) - 1048576, Y: int32(next()%257) - 64, Z: int32(next()%2097153) - 1048576}
		for j := range q.Climate {
			q.Climate[j] = float64(int64(next()%40001)-20000) / 10000
		}
		out = append(out, q)
	}
	coords := []int32{-1025, -1024, -1023, -257, -256, -255, -65, -64, -63, -1, 0, 1, 63, 64, 65, 255, 256, 257, 1023, 1024, 1025}
	for _, x := range coords {
		for _, z := range coords {
			for _, e := range []float64{-2, -1, -.25, -.21875, -.2, 0, .25, .4, 1, 2} {
				out = append(out, BiomeProbeQuery{X: x, Y: 16, Z: z, Climate: [6]float64{.1, -.1, 0, e, 0, 0}})
			}
		}
	}
	return out
}

func TestInstalledBiomeSourceProbe(t *testing.T) {
	jar := os.Getenv("MCL_BIOME_SOURCE_JAR")
	if jar == "" {
		t.Skip("set MCL_BIOME_SOURCE_JAR to the numerical oracle's installed artifact")
	}
	var oracle struct {
		JarSHA256, Class, Minecraft, InputSHA256, BiomesSHA256 string
		Queries                                                int
		CodecBiomes                                            []string
	}
	b, err := os.ReadFile("testdata/biome-source-c6c-1.2.1.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &oracle); err != nil {
		t.Fatal(err)
	}
	b, err = os.ReadFile(jar)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(b)) != oracle.JarSHA256 {
		t.Fatal("artifact differs from the independent oracle")
	}
	z, err := zip.OpenReader(jar)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	queries := biomeProbeQueries()
	b, err = json.Marshal(queries)
	if err != nil {
		t.Fatal(err)
	}
	if len(queries) != oracle.Queries || fmt.Sprintf("%x", sha256.Sum256(b)) != oracle.InputSHA256 {
		t.Fatal("oracle input changed")
	}
	hash := sha256.New()
	counts := map[string]int{}
	for offset := 0; offset < len(queries); offset += 256 {
		r, err := ProbeBiomeSource(context.Background(), &z.Reader, Options{Minecraft: oracle.Minecraft}, oracle.Class, "CODEC", queries[offset:min(offset+256, len(queries))])
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(r.CodecBiomes, oracle.CodecBiomes) {
			t.Fatalf("registry codec lookup differs: %v", r.CodecBiomes)
		}
		for _, id := range r.Biomes {
			fmt.Fprintln(hash, id)
			counts[id]++
		}
	}
	if fmt.Sprintf("%x", hash.Sum(nil)) != oracle.BiomesSHA256 {
		t.Fatal("query results differ from Java")
	}
	t.Logf("%d queries, %d returned biomes match the independent Java hash", len(queries), len(counts))
}
