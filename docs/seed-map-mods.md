# Native terrain-mod compatibility

The production sampler is entirely Go. Effective datapack JSON feeds the density compiler; TerraBlender registration data feeds native regional biome selection. MCL's map uses no HTTP bridge, JVM, loader or mod process. The optional `cubiomes/modjar` package interprets bounded registration and custom biome-query bytecode in Go. TerraBlender sampling uses compiled native parameters; custom biome sources can retain interpreted query code with native noise inputs. Neither path establishes complete mod-loader compatibility.

## Automatic JAR registration

### Additional 1.21.1 pack support

Fabric TerraBlender 4.1.0.8 / BOP 21.1.0.14 / GlitchCore 2.1.0.0 has a
separately pinned native profile and 92 source-reference configuration tests.
Lithostitched 1.7.13 biome injectors compose with that profile and with effective
datapack climate definitions. The implemented codecs are partial replacement,
full replacement and force placement, with weighted regions, biome tags, climate
or supported density conditions, and FastNoiseLite cellular CellValue noise.
This covers Legendary Monuments 8.1's `dyna_plains` and `plains_of_death` rules
without a per-biome hard-coded adapter. Unimplemented injector codecs, load
predicates and custom generators remain unsupported. Overlapping equal-priority
rules also need an independent reference before their ordering can be certified.

Global Packs configured required/optional paths and Paxi's pack directories,
base-directory setting and explicit load order participate in registry loading.
Saved worlds use their recorded enabled-pack order. Global Packs' default order
was observed in this installed pack; its published source URL is unavailable.
System-wide global packs and nested-JAR registry resources are not yet covered.
The [multi-pack report](seed-map-packs-verification-20261028.md) separates preview
loading from actual saved-world comparisons and lists remaining gaps.

### Shared registration loader

The first verified contract is **Forge / Minecraft 1.20.1, TerraBlender 3.0.1.10, and YUNG's Cave Biomes 2.0.5**. The framework binding is versioned; consumer mod names, configuration filenames, region IDs, weights, conditional registration and climate parameters come from the installed JAR. There is no production YUNG-specific parameter table or filename rule.

The common path is:

1. Read mod identities and effective worldgen resources from installed JARs. Forge's `${file.jarVersion}` is resolved from the main manifest section. Unrelated mods do not enter the registration interpreter merely because they contain classes or mixins.
2. After the complete mod catalogue is available, inspect actual `Region` inheritance and `Regions.register` references, including optional integrations without a declared TerraBlender dependency. A dependency, mixin, class name or string alone is not a registration. Discovery uses four workers; execution remains ordered. Discovery establishes possible code paths, not whether a configuration enables them.
3. Resolve Forge/NeoForge annotated constructors and MOD-bus subscribers, or Fabric's dedicated `terrablender` entrypoints (also usable through Quilt's Fabric compatibility). Bind installed-mod queries, common configurations, event priorities and deferred work through shared framework APIs. Production no longer forces Forge 1.20.1 through static-module discovery; the original independently checked static route remains an explicit diagnostic option. Capture ordered region parameters and explicitly requested TOML configurations. Include results and participating dependency-class hashes in the immutable snapshot; refresh does not modify the instance.
4. Compile the captured rules into the existing native climate trees and regional selector. The sampling loop never runs bytecode. Structure choices continue to come from the effective installed resources.

Two independently compiled public-source configurations produce **7,595 ordered parameters each**, matching the Go JAR interpreter's complete hashes. A further **10,000 actual Minecraft 1.20.1 `Climate.ParameterList` lookups** match the native tree, including interval endpoints, ties and previous-leaf history. An installed-instance regression checks that the discovered TOML changes the snapshot and native parameters, and that the custom cave biomes reach the sampler. [Provenance and hashes](../cubiomes/data/terrablender-1.20.1.provenance.json) distinguish registration/tree verification from full world generation.

The decoder no longer rejects an entrypoint solely because Minecraft is not 1.20.1. Shared protocol fixtures exercise annotations, framework API relocation, metadata entrypoints and disabled branches using version labels from 1.18 through 26.3. Separate integration tests feed arbitrary consumer IDs into the existing 1.21.1 and 26.3 native kernels. These tests establish protocol behavior, **not whole-game support for those releases**. Native kernel/artifact contracts and ordered vanilla climate-table availability remain restricted to independently verified implementations.

This is not complete loader lifecycle execution. Missing Java/game APIs, Fabric intermediary symbol mapping, arbitrary Mixins, custom codecs and procedural algorithms still require matching shared semantics. Multiple region-producing JARs for the same dimension require the loader's cross-JAR registration order and shared initialization state; MCL reports unresolved order instead of assuming filename order. A JAR producing no recognized entrypoint is not evidence that all its world generation is supported. In particular, Aeroblender's Aether-specific hooks are outside this vanilla-dimension contract. Existing source-verified BOP profiles remain as a fallback; new consumers do not get new per-mod parameter tables.

Class loading is indexed and lazy. Each class is bounded to 2 MiB, archive inspection to 256 MiB and the retained execution class set to 16 MiB; unrelated classes do not exhaust the execution budget. Named dependencies can be supplied by the installed JAR catalogue. Conflicting definitions fail rather than guessing classpath order. Unknown executed APIs/opcodes discard partial parameters. Instruction, frame, object, array and output bounds and cancellation remain enforced; no process, network, arbitrary file, reflection or native-call API is exposed. Only explicit callbacks supply named classes/configurations. Sampling never retains a Java runtime.

This makes the unit of extension a **framework API and registration pattern**, rather than every consuming mod. Standard datapack resources share the existing compiler; mods using a supported registration pattern share the interpreter and bindings. New procedural algorithms still need matching native semantics or additional bounded interpreter bindings and independent parity tests. These fixtures do not establish coverage for every mod in a large installed pack.

### Generic loading target

#### Shared-session implementation status (2026-10-28)

`modjar.NewClasspath` now follows declared Forge/NeoForge JarJar, Fabric `jars`
and Quilt `quilt_loader.jars` archives. Quilt metadata takes precedence over a
retained Fabric descriptor. This resolver is connected to the launcher's
registration reader; snapshot schema 19 invalidates older cached results.

The selected installed profile's inherited library catalogue now supplies lazy
class and service resolution to both registration and biome bootstrapping. These
libraries are not treated as extra mods or datapacks. This does not yet reproduce
arbitrary dependency locators, the complete game classpath and symbol mappings,
or a shared lifecycle for every provider; see the current pack verification log.
Duplicate, unused class entries do not invalidate unrelated classes. Identical
definitions are accepted; conflicting definitions fail when that class is
requested. Active metadata or nested-library path ambiguity remains an error.
The metadata reader preserves quoted control characters accepted by the
[Fabric JSON reader](https://github.com/FabricMC/fabric-loader/blob/master/src/main/java/net/fabricmc/loader/impl/lib/gson/JsonReader.java).

Small nested archives retain at most 16 MiB in aggregate. Larger archives use
64 KiB read windows and retain their decompression cursor across adjacent reads,
instead of replaying the whole stream for each directory window. Root archives
are caller-owned; `Classpath.Close` and `Environment.Close` release nested
streams. No files are extracted, and no JVM or HTTP transport is involved.

The new `modjar.Load` API executes a bounded, shared initialization session:
caller-supplied ordering constraints, declared pre-initialization/constructor
entrypoints, registry events, supported common configuration, setup callbacks,
queued work and framework callbacks. Fabric and Quilt default method/field/class
entrypoint forms are decoded from metadata. Static state, class definitions and
captured registries are shared across modules. Mod event buses, configuration
events and queued work retain the owning mod identity. Deferred factories run
only at their attached register's event; unsupported factories fail rather than
being silently skipped. Failures return no partial environment.

This shared-session API is **not yet the production map's initialization path**.
Only its nested-archive resolver has been connected to production registration.
It resolves a uniquely eligible embedded-library version from intersected Maven
ranges, but does not yet implement every dependency selection rule, initialize
a complete installed-mod dependency closure, supply
version-specific registry event sequences, remap Fabric intermediary symbols,
apply arbitrary Mixin/ASM transformations, or provide the complete game API.
`Module.Before/After` and `Options.RegistryOrder` must come from a verified loader
profile; the session does not invent them from filenames or constant-pool order.
Recorded codec declarations are not a complete registry JSON decoder. The
separate custom-source production path now binds final biome registries and
real density-router inputs, subject to its immutable-query and transformation
checks below. Existing pinned BOP profiles and the legacy
1.20.1 registration path have not been removed.

Protocol fixtures cover shared state/order, actual registry IDs, callback owners,
deferred factories, declared nested libraries, cancellation, conflicting classes,
large-archive windows, Java exception handlers and failed static initialization.
Host capability/budget failures cannot be swallowed by Java `catch` clauses.
These are protocol and interpreter tests, not proof of complete Minecraft or
modpack compatibility. The strict installed-instance matrix still has five
passing and four failing cases; see the [verification report](seed-map-packs-verification-20261028.md).

#### Remaining complete-flow work

Terrain involvement is a discovery signal, not sufficient execution semantics.
The target is to load installed JARs and their dependencies through a shared
classpath, execute their actual registration and generation code, and resolve
resource IDs through the resulting registries. No new mod-name branches,
class-name-to-resource-ID guesses, or replacement parameter tables are part of
that target. Minecraft/loader/library API implementations are shared by every
caller of that ABI; implementing one is different from rewriting each mod.

The remaining runtime work is substantial: dependency and initialization order,
loader lifecycle and common configuration, registry/Codec decoding, transformed
classes from Mixins/Coremods, standard Java semantics and the game's noise APIs.
Discovery must include optional integrations and dependencies; executing only
classes whose names contain terrain terms is not a substitute. Existing pinned
BOP fallbacks have not yet been replaced by this complete runtime. A classifier
that recognizes a terrain mod cannot establish that arbitrary JARs or releases
will execute correctly in the current restricted interpreter.

Immutable registration outputs can be compiled into the existing native engine.
Procedural generation that remains in bytecode needs bounded query execution,
actual noise-router inputs and safe reusable state before it can become a map
sampler. Unimplemented calls must remain explicit failures, not no-op execution
or silent vanilla substitution. Full in-game comparison remains a separate
acceptance step from successfully reading resources or running an entrypoint.

#### Shared runtime additions (2026-10-28)

The production registration classpath now reads declared service configuration
resources and executes providers lazily. Bounded implementations cover the
used `ServiceLoader`/`Optional`, UUID, collection/value, reflection, configuration,
distribution-supplier and seeded `java.util.Random` APIs. This is a supported
subset: unordered collection iteration, custom key hashing, unavailable initial
registry contents and arbitrary standard-library calls are not fabricated.
Private field reads require an explicit accessibility override and a supported
module-access policy. Constructor lookup and invocation execute only supplied
class-file bodies, preserve initialization and target-exception boundaries, and
require that same module policy. Primitive argument conversions and general
`Method.invoke` remain unsupported. A `Class` literal now has its correct array
assignability; the earlier erroneous `ArrayStoreException` blocked Quark's proxy
construction before its actual initialization could run. Runtime process access remains unavailable; constructing
an ordinary `RuntimeException` is no longer misclassified as process access.

JarJar version ranges use Maven ordering, including qualifier aliases and
arbitrary-length numeric components within input limits. Exactly one eligible
version can be selected without guessing a loader's tie-breaking rule. Multiple
eligible versions, unsupported ranges and conflicting class contents remain
explicit failures. This selects executable class/service resources; it does not
yet integrate every nested archive's datapack resources into the pack resolver.

Requested common configs come from `config`. Requested server configs use the
selected save's `serverconfig`, then `defaultconfigs`; a new-world preview uses
defaults. Reads stay bounded to explicitly requested resources. An unknown old
value from a known map write may only be discarded immediately with bytecode
`POP`/`POP2`; consuming it, storing it or branching on it still fails.

Custom biome programs scope noise transformations against the effective six
climate channels, including spline children. A noise injection can be irrelevant
only if the actual router cannot reach that noise family. Reusing a captured
program with broader noise requirements fails. In Mechanomania, KubeJS changes
End temperature and vegetation to shifted normal noise, so the C2ME Perlin
injection remains relevant and cannot be omitted merely because this is the End.

Independent comparisons cover 1,600 Maven version pairs and 240 Java Random
sequences (1,440 scalar outputs, with floating-point bits checked). Together with
runtime/lifecycle regression tests these validate shared primitives, not whole
pack generation. [Inputs, hashes and reproduction](worldgen-reference.md#shared-java-runtime-numerical-references).

### Direct JAR biome-source execution (experimental)

`InspectBiomeSources` inventories direct named `BiomeSource` declarations and
their codec fields. `ProbeBiomeSource` executes a supplied registry-only
`MapCodec` and `getNoiseBiome` method in Go using shared DFU record builders,
holder keys, deferred declarations, coordinate conversion and density APIs.
It shares the registration interpreter's lazy classpath and execution limits.
The probe remains a diagnostic API. The separate production path now uses
`BootstrapBiomeSources` and `BiomeProgram` to obtain actual registered IDs and
compile immutable holder inputs against the effective biome registry. It reads
loader-declared Mixin configurations, executes supported static bootstrap
HEAD/RETURN hooks and plugin conditions, and binds a validated shared End-height
overwrite to native seeded noise. It does **not** run the entire loader or
apply arbitrary transformations. Opaque item
declarations can be retained during static initialization; item behavior and
property reads are not implemented and remain unsupported.

C6C 1.2.1 has no public source according to the user. Its unmodified installed
JAR was executed through both this Go probe and an external Java API reference:
**37,178 queries and all 12 returned biome IDs match**, including the ordered
12-holder codec inputs. The test supplies quart coordinates and constant raw
climate fields, with negative coordinates and boundary targets. No C6C biome
selection algorithm or implementation classes are copied into the project.
The reference initializes an empty FML loading catalogue and the game bootstrap,
then invokes the public codec and query APIs; it is not a full modpack launch.
The Java helper remains outside this repository and is not a launcher dependency.
[Artifact hashes and scope](../cubiomes/modjar/testdata/biome-source-c6c-1.2.1.json)
retain these limitations. Actual noise fields, active dimension routing, pack
order, Mixin effects, heights and structures are **not verified by this test**.

A separate seeded test now compares **680 coordinates across five seeds** with
an independent Java reference, using the same installed JAR and Minecraft
1.21.1 native registry/noise APIs. Raw End erosion values match bit-for-bit and
biome IDs match at every coordinate; these samples encounter four biome IDs.
The constant-climate test above covers all twelve. The reference explicitly
binds the actual JAR's End-height overwrite to a seeded Minecraft SimplexNoise;
it is an isolated numerical test, not a fully transformed pack launch. Expected
values are in `cubiomes/modjar/testdata/biome-source-c6c-1.2.1-seeded.json`
(SHA-256 `0886be3d474ef3fbd3a7b9661dac36b59ded89529b498a847b2e33616eb9f57e`).
The external `SeededProbe.java` source has SHA-256
`d92ac5b2d336e7e9ade79280e69f23de6c781a3da8f724a3066066c13043f888`.

Sampling preserves raw coordinates, independently seeded workers, cancellation,
and data-driven biome injections after the custom source. Captured factories
must reproduce their immutable field state in a fresh worker. Query graphs with
mutable state or unbound operations fail rather than returning vanilla results.
Plugin metadata reads are limited to supplied class files, including Java
class-version compatibility and bounded field access. The configured game's
Java version is read from its release metadata without starting it.

Remaining gaps include complete loader dependency mediation, initialization
ordering, dynamic/inherited Mixin configurations and refmaps, general ASM and
injection execution, further configuration libraries, module-layer services and wider
Java API semantics. Existing versioned BOP profiles still exist. The latest
strict installed matrix remains **five passing / four failing cases**, including
Mechanomania's complete custom End. Numerical success above must not be reported
as full-pack compatibility.

To reproduce the Go side without Java:

```powershell
$env:MCL_BIOME_SOURCE_JAR = 'path/to/installed/c6c-1.2.1.jar'
go test ./cubiomes/modjar -run 'BiomeSource|BiomeProbe|SeededBiomeProgram' -count=1 -v
```

Synthetic classes with unrelated names exercise codec construction, dispatch,
negative-coordinate arithmetic, cancellation and atomic failure without any
installed mod. The optional real-JAR test compares a committed independent Java
result hash; it does not simply compare two invocations of the Go interpreter.

## Verified releases and results

The following results use Minecraft **26.3** and pinned artifacts. [TerraBlender provenance](../cubiomes/data/terrablender.provenance.json) records artifact hashes, source commits, export methods and fixture hashes.

| Configuration | Native Go verification |
| --- | --- |
| Terralith 2.6.5+26.3 | 9 Java raster hashes / 147,456 cells match |
| Tectonic 3.0.29 datapack | 9 Java raster hashes / 147,456 cells match |
| Terralith then Tectonic | 9 Java raster hashes / 147,456 cells match |
| TerraBlender 26.3.0.0.9 + Biomes O' Plenty 26.3.0.0.15 | 27 Java raster hashes / 110,592 cells match across Overworld, Nether and End |
| TerraBlender 26.3.0.0.9 alone | 27 Java raster hashes / 110,592 cells and 1,024 boundary targets match |
| TB/BOP with no pack, Terralith, Tectonic, or both in order | 24 Java raster hashes / 245,760 cells and climate vectors match |
| TB/BOP disabled and mixed biome/size/weight configurations | 54 Java raster hashes / 221,184 cells and 5,632 boundary targets match |

BOP fixtures cover seeds 0, -1 and 1234567890123456789; grids include positive/negative positions, Y=64/-32, and coordinates near the 30-million-block world border. All 73,728 Overworld/Nether climate vectors in the external dense export also match. Another 3,584 synthetic boundary targets exercise exact tree ordering, per-region history and deferred base-region lookup. Normal Go tests check numeric IDs, names, reseeding and zero per-point allocations.

The combined-pack fixture uses seeds 42, -1 and 1234567890123456789. Its MCL test ingests the actual pinned mod JARs and ordered datapack ZIPs, binds the native adapter, checks every dense climate/region/biome result, and verifies rendered 128×128 pixels against the numerical palette. The same BOP region tables remain valid under these packs; the effective base climate tree and density graph change.

All 68 BOP biome toggles are implemented through ordered fallback choices. The exported tables agree with 86 actual registration configurations, including single-enabled, precedence and mixed cases. The additional raster fixtures exercise all-disabled biomes, mixed toggles, size limits 2 and 6, and zero/nondefault region and End weights. All 147,456 Overworld/Nether climate vectors in these dense configuration exports also match. Their launcher test parses TOML/JSON configuration contents through ingestion and checks the resulting rendered output.

The reference loaded Fabric Loader 0.20.1, Fabric API 0.165.0+26.3, GlitchCore 26.3.0.0.5 and the pinned TB/BOP artifacts. It exported public registration tables and sampled actual biome sources in an isolated registry test. It did not start a server, accept an EULA, open a saved world or expose a listener. Reference projects and runtime artifacts live outside this repository; this repository retains numerical fixtures and provenance.

The datapack suite compares `WorldgenIngest` → `NewLocalSampler` → `Sample`, coverage, colors and full numerical raster hashes. Another 49,152 dense climate vectors verify normalization and rounding edge cases. These are finite regression fixtures for the named releases. Older vanilla support has separate tests and does not certify older mod releases automatically.

## Native TerraBlender behavior

`WithTerraBlender` wraps a Go climate generator with ordered registered regions. A weighted integer field uses the world seed, then one fuzzy zoom and `3 + regionSize` majority zooms. Integer overflow, negative coordinates, random draw order and ties follow the source contract. Bounded caches belong to each sampler; workers share immutable compiled trees.

Fields with only one possible output are constant-folded, omitting their zoom caches. This removes the region cache for TB-only vanilla regions and three default End fields while preserving the same result.

Each region has a compact climate R-tree and independent previous-leaf history. A `terrablender:deferred_placeholder` result searches region zero. Region zero uses the supplied generator's actual climate table, preserving datapack overrides. Flattening the regions into one climate tree would produce different results.

`WithTerraBlenderEnd` implements four independently weighted End areas and their sizes. It retains the central island rule and reads erosion at the actual quart-cell block position. This differs from vanilla's section-center sampling; TerraBlender enables it even without BOP. Duplicate weighted biome names remain equal during majority zooms. All-zero End weights return `minecraft:ocean`, as the source's biome layer overrides the generic region-zero fallback.

Injection follows the source's dimension-type tags, not merely dimension names or climate preset IDs. Noise chunk generators with an End source take the End hook. Otherwise End-tagged non-End sources get no biome hook; Nether tags take precedence over Overworld tags, and those tags require a multi-noise source. Untagged custom dimension types remain unaffected even when they reuse an Overworld preset.

Supported region/End sizes are 2–6. Vanilla weights are nonnegative Java integers; combined weight sums must fit a signed Java integer. MCL follows the source's default-value behavior for out-of-range configuration values; the explicit Go API rejects invalid values. All 68 known BOP toggles use versioned fallback data. New releases should extend shared framework contracts and parity fixtures, with an algorithm-specific implementation only where the generation semantics actually differ. Datapack resources from mods continue through the datapack compiler.

Adapter selection reads mod identities, effective worldgen resources, actual registration references and relevant settings. An integration whose executed branch contributes no regions is not an active TerraBlender provider. Arbitrary class or mixin presence does not disable biome prediction. Version and artifact checks apply to framework contracts and the dimensions using them. The old 1.20.1 TerraBlender contract has no End biome hook and does not require BOP. Custom generator or codec support is resolved per dimension.

MCL retains at most eight immutable compiled mod generators, keyed by snapshot and dimension. Changing the seed reuses those trees and rebuilds seed-dependent sampler state. Each worker owns its region caches and previous-leaf history.

## Datapack handling

Resolution applies source order first, then matching overlays in declared order. An overlay replaces resources within its pack before tags merge with earlier packs. ZIPs and directories use the same rule, including root-tag replacement.

Registry JSON ingestion accepts game-style line/block comments and a UTF-8 BOM, preserving quoted strings. Other malformed JSON remains an error. This matters for actual YUNG's biome resources and several structure tags; losing those definitions would remove their native biome entries or markers.

`pack.mcmeta` supports legacy `formats`, inclusive bounds and newer `min_format`/`max_format` major/minor values. Installed official `version.json` overrides the verified fallback. Formats are 26.1/26.1.1/26.1.2 = 101.1, 26.2 = 107.1 and 26.3 = 121.0. Unknown versions with conditional overlays and unsafe overlay paths fail explicitly.

Embedded packs under a mod's `resourcepacks` directory are detected; activation is not inferred from presence. Static ingestion reports unresolved embedded packs. Custom codecs and arbitrary injected behavior require additional native implementations and independent fixtures.

The 26.3 compiler implements the tested density operators, float evaluation, duplicate spline knots and normalized noise schema. For `normalize: "legacy"`, trailing zero octaves retain active-span normalization. Using declared octave count instead caused an observable 8/7 climate-scale error in Tectonic island ridges; the implementation now matches the reference.

## Measured sampling latency

The original 1.20.1 JAR registration benchmark separately measured **10.469 ms median**
(9.061–10.690 ms across five 500 ms rounds) to capture 7,595 parameters from the
installed YUNG's JAR, with about 10.25 MB allocated per load. It includes class
decompression, discovery and execution, with the ZIP reader already open and
the shared vanilla table warmed. This cost occurs during resource loading, not
per pixel or tile. Ordered parameter rewrites use an original-box lookup map,
reducing repeated comparisons from O(n×m) to O(n+m) without changing duplicate
entries or last-write precedence. [Raw timings](../cubiomes/testdata/benchmarks/2026-10-27-modjar/go.txt)
and [method](../cubiomes/testdata/benchmarks/2026-10-27-modjar/metadata.json) are retained.

After the shared-loader changes, the same installed-JAR benchmark measured
**11.118 ms median** (9.833–12.458 ms, five 500 ms rounds, `CGO_ENABLED=0`),
with approximately 10.93 MB allocated per load. Raw results are in
`.cache/seedmap-coverage-audit-20261028/registration-benchmark.log`. This is a
separate run rather than a controlled speedup claim. The additional discovery
and validation work occurs at load time; no bytecode work was added to sampling.

On Windows amd64 / Ryzen 7 9800X3D, Go 1.27.3 with `CGO_ENABLED=0` and Zulu Java 26.0.2, the same TB/BOP seed-42 grid contains 16,384 cells at X=8192, Y=64, Z=-8192 with four-block spacing. All three complete ordered biome-name hashes match. These are median CPU times over five rounds; other agents' CPU work was paused.

| Dimension | Actual Java mod logic | Native Go | Go minimum | Java/Go time ratio |
| --- | ---: | ---: | ---: | ---: |
| Overworld | 32.832 ms | 25.814 ms | 25.061 ms | 1.27× |
| Nether | 4.494 ms | 3.921 ms | 3.867 ms | 1.15× |
| End | 100.854 ms | 1.968 ms | 1.918 ms | 51.24× |

Go uses reusable sampler scratch and output storage with **0 B/op, 0 allocs/op** in all dimensions. It recomputes each raster; it does not return a cached completed tile. Bounded region memoization, constant fields and the native End noise cache reduce repeated work. Java recreates the actual caching resolver per raster and reuses its output array. Each Java dimension receives two seconds of warmup, then five rounds of at least 500 ms; Go uses five 500 ms benchmark rounds. Initial registry loading, startup, I/O, queues and rendering are excluded. The End improvement measures this biome sampling workload, not complete chunk generation.

Warm MCL session construction with a changed seed, all three dimensions and metadata has a median **2.549 ms**, 392,995 B/op and 4,238 allocs/op. This uses the eight-entry compiled-generator cache and is separate from raster timing.

[Raw final Go timings](../cubiomes/testdata/benchmarks/2026-10-27-native-mods/go.txt), [fresh Java timings](../cubiomes/testdata/benchmarks/2026-10-27-native-mods/java-fresh.json), [MCL session timings](../cubiomes/testdata/benchmarks/2026-10-27-native-mods/mcl-seed.txt) and [method/summary](../cubiomes/testdata/benchmarks/2026-10-27-native-mods/metadata.json) retain every round. The [first Go run](../cubiomes/testdata/benchmarks/2026-10-27-native-mods/go-first.txt) is also retained; its medians were 25.369/3.701/1.854 ms. The table uses the later Go run immediately following the fresh Java run. Five rounds do not establish p95 latency.

## Reproduce

```powershell
$env:CGO_ENABLED = '0'
go test ./cubiomes -run '^TestTerraBlender' -count=1
go test ./cubiomes -run '^$' -bench '^BenchmarkTerraBlenderTile26_3$' -benchtime=500ms -benchmem -count=5
```

Set `MCL_TERRABLENDER_DENSE` to the external dense JSON export to check every stored climate vector alongside the committed full-raster hashes. `MCL_TERRABLENDER_CONFIG_DENSE` optionally names the parent directory containing `native-variant-disabled` and `native-variant-mixed` dense exports. The benchmark uses seed 42, X=8192, Y=64, Z=-8192 and 128×128 quart cells. It recomputes cells with reusable storage rather than returning a cached tile. General C/Go and MCL measurements are in [the performance report](../cubiomes/BENCHMARKS.md).

For real archive ingestion, set `MCL_WORLDGEN_FIXTURES` to the directory containing the pinned mod JARs and datapack ZIPs, then run `go test ./internal/launcher -run '^TestWorldgenNativeMod(Packs|Config)Oracle$' -count=1`. `MCL_TERRABLENDER_PACKS_DENSE` optionally names the external combined-pack dense export. Neither test needs Java.

For 1.20.1, set `MCL_REGISTRATION_JAR` to the installed `YungsCaveBiomes-1.20.1-Forge-2.0.5.jar` and run `go test ./cubiomes/modjar -count=1`. Set `MCL_REGISTRATION_INSTANCE` to an instance containing that JAR and `TerraBlender-forge-1.20.1-3.0.1.10.jar`, then run `go test ./internal/launcher -run '^TestWorldgenInstalledRegistrationAndConfig$' -count=1`. Without these optional archives, synthetic class fixtures assembled in Go and the committed 10,000-query climate fixture still run in ordinary tests. Neither path starts Java.

## Scope and source provenance

This predicts newly generated biome assignments. The map also estimates modern Overworld elevation from supported depth graphs, including the pinned Terralith/Tectonic combination; see [surface view](seed-map.md). Existing chunks, upgrade blending, blocks, exact final-density heightmaps and final structure existence remain separate concerns. Structure markers remain placement candidates.

The Go region algorithm was independently written from the mathematical and observable source contract, using the library's existing LCG and a histogram formulation of zoom selection. Public references include [ordered region lookup](https://github.com/Glitchfiend/TerraBlender/blob/7d7a778ab9c14d0091581ed51ada0821e69761b8/common/src/main/java/terrablender/mixin/MixinParameterList.java), [layered fields](https://github.com/Glitchfiend/TerraBlender/blob/7d7a778ab9c14d0091581ed51ada0821e69761b8/common/src/main/java/terrablender/worldgen/noise/LayeredNoiseUtil.java) and [End selection](https://github.com/Glitchfiend/TerraBlender/blob/7d7a778ab9c14d0091581ed51ada0821e69761b8/common/src/main/java/terrablender/mixin/MixinTheEndBiomeSource.java). Their notices are LGPL-3.0-or-later; individual utility notices can differ. BOP's source is all rights reserved. No BOP implementation, artwork, texture or other mod asset is copied into this repository. Numerical mappings and identifiers were exported through public runtime APIs.
