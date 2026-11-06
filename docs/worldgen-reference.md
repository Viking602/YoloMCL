# Latest Minecraft worldgen oracle

`LatestReference.java` executes the public biome source and noise APIs in the
official hash-verified Minecraft server artifact. It does not launch a server.
Registry data comes from the server's built-in data generator and registry JSON.

The checked profiles are **26.1, 26.1.1, 26.1.2, 26.2, and 26.3**. Each profile has
its own manifest/server hashes, extracted registry bundle, and Java fixture.
26.1.1 and 26.1.2 have byte-identical projected registries and identical oracle
cases to 26.1; this was checked from their separate official server artifacts.
Unverified releases and snapshots are not accepted by prefix.

## Reproduction

The project contains Go source, pinned registry data and independent expected
results. Reference programs, Java libraries and their build scripts live outside
this project. They are not dependencies of the library, launcher, Go build or tests.

Run the checked-in fixtures without Java:

```text
go test ./cubiomes -run 'TestLatest|TestTerraBlender' -count=1
go test ./internal/launcher -run TestWorldgenNativeModsOracle -count=1
```

The external reference process verifies each official manifest/server SHA-1
before executing public noise/biome APIs. Provenance also records the inner
server SHA-256 and uncompressed registry bundle SHA-256. Generated expected
results are copied into `cubiomes/testdata`; the Go library never executes the
reference program. Do not add the external Java projects to `tools`.

Each release fixture covers four profiles, six seeds (including both signed
64-bit extrema), 1,392 individual positions, and 168,960 raster cells represented
by 72 name hashes. Coordinates include negatives, world-border positions and
multiple heights. The individual points check quantized climate, fresh biome
lookup and sequential lookup. A raster retains Java's previous equal-distance
winner and starts a fresh source for each raster.

External dense exports can be checked by setting `MCL_LATEST_DENSE_ORACLE`
or `MCL_TERRABLENDER_DENSE` to the generated JSON path before running the
corresponding Go tests. The checked-in compact fixtures already hash every
raster cell and retain selected points for diagnostics.

The independent 26.3 boundary probe compared tree orders 2–32 at 120,000
targets. Only order 19 matched every result. The first 4,096 targets are kept
as a compressed fixture to check ties independently of the noise implementation.

## Arithmetic and sources

The 26.3 implementation evaluates density arithmetic and noise components in
single precision, including intermediate products. Coordinates remain double
precision until the Perlin cell offsets are computed. Legacy Nether components
use their own float normalization and accumulation order. The End central-island
distance no longer overflows, while the seeded outer-island lattice is retained.
The code is checked against the official Java oracle, not against a version alias
or an older C implementation.

Source references:

- [Official 26.3 release notes: density functions, noises and spawn targets](https://www.minecraft.net/en-us/article/minecraft-java-edition-26-3)
- [NeoForge 26.3 API migration primer](https://github.com/neoforged/.github/blob/2bb6f9f3bdaec6efe2b69c3241b15fb47b64fc53/primers/26.3/index.md)
- [C2ME 26.3 Perlin implementation](https://github.com/RelativityMC/C2ME-fabric/blob/dev/26.3.0/c2me-opts-math/src/main/java/com/ishland/c2me/opts/math/mixin/MixinPerlinNoiseSampler.java), Git blob `4f1a3798608defd4095ab19b6306e5cf46bda57f`
- [C2ME 26.3 native math implementation](https://github.com/RelativityMC/C2ME-fabric/blob/dev/26.3.0/c2me-opts-natives-math/src/c/includes/ext_math.h)

C2ME math code is MIT licensed; its notice is retained in
`cubiomes/data/latest-LICENSE-C2ME.txt`. No OpenCL module code is used.

The Go compiler accepts the verified bundled 26.3 noise definitions and the
explicit `normalize: "legacy"` form used by tested packs. Other new normalization
modes and density operators need an additional native implementation before
those definitions can be sampled.
It does not claim to generate terrain blocks, surface rules, or exact spawn
positions from the new arbitrary-dimensional 26.3 spawn-target format.

## Shared Java runtime numerical references

The shared Go JAR interpreter has two additional independent numeric fixtures:

| Fixture | Reference and scope | SHA-256 |
| --- | --- | --- |
| `cubiomes/modjar/testdata/maven-versions-3.8.5.tsv` | Apache Maven Artifact 3.8.5 `ComparableVersion`; all 1,600 ordered pairs of 40 versions | `d6be6e3adb7548cd8d7081a705dedecfbc7dad0a17e348d5be6fa2af4869f1d7` |
| `cubiomes/modjar/testdata/java-random-21.tsv` | Java 21 `Random`; six seeds × 40 sequences, six outputs per sequence | `86043cfb7706a7615f58596724f9e451f9d75dbb5dcaecade3c42ca386c3a8bd` |

The Maven reference artifact's SHA-256 is
`91172bc294d6eab02fc9f45f4ea01fd0e418962d128cf489abea7b6957d988ee`.
The external `VersionProbe.java` helper has SHA-256
`c9d18891f22f7cae272e8bac0dff8899561668a532a7a95dff025ece571d3e69`;
`RandomProbe.java` has SHA-256
`40394c82c7ec14139975991325be6802b53ba1cd269e651a44546e54c6919934`.
Both helpers live outside this project under the developer's
`mcl-worldgen-reference/2026-10-28/maven` directory. No Java implementation or
runtime was added to `tools`, the launcher or the Go test prerequisites.

Each Random sequence calls, in order, `nextInt()`, `nextLong()`,
`nextInt(1073741825)`, `nextInt(32)`, `nextDouble()` and `nextFloat()`.
Seeds are 0, 1, -1, 42 and both signed 64-bit extrema. The large bound exercises
rejection sampling; floating-point outputs retain their IEEE bit patterns.
Time-seeded sampling remains unsupported until an explicit `setSeed` call.
Maven tests include qualifier aliases, release/prerelease ordering, nested
hyphen/digit transitions and numeric components beyond 64 bits.

Run the checked-in independent outputs without Java:

```text
go test ./cubiomes/modjar -run 'TestMavenIndependentVersionOracle|TestJavaRandomIndependentOracleAndUnknownSeed' -count=1
```

These references verify numeric primitives. They do not establish arbitrary
mod initialization, loader-specific JarJar tie-breaking, all Java Random APIs,
or correctness of a complete modpack's generation.

### Shared classpath and native value contracts (2026-10-28)

- Dependency selection reuses the launcher's installed-version inheritance,
  library rules and artifact path resolution. Tests cover replacement, exclusion,
  duplicate paths, cycles, cancellation, service resources and resource isolation.
- NeoForge's [DeferredRegister](https://github.com/neoforged/NeoForge/blob/1.21.1/src/main/java/net/neoforged/neoforge/registries/DeferredRegister.java),
  [DeferredItem](https://github.com/neoforged/NeoForge/blob/1.21.1/src/main/java/net/neoforged/neoforge/registries/DeferredItem.java),
  [DeferredBlock](https://github.com/neoforged/NeoForge/blob/1.21.1/src/main/java/net/neoforged/neoforge/registries/DeferredBlock.java)
  and [DeferredHolder](https://github.com/neoforged/NeoForge/blob/1.21.1/src/main/java/net/neoforged/neoforge/registries/DeferredHolder.java)
  define the factory/return-type and holder key contracts. The tests assemble
  class files in Go to verify virtual overrides, explicit super calls and
  interface inheritance. These branch sources are not an all-version oracle.
- [TimeUnit's specified conversion contract](https://docs.oracle.com/en/java/javase/21/docs/api/java.base/java/util/concurrent/TimeUnit.html#convert(long,java.util.concurrent.TimeUnit))
  is independently checked with Go arbitrary-precision multiplication/division
  and final clamping for all 49 unit pairs and signed boundary inputs.
- Native/bytecode state tests retain a real field-based override alongside an
  opaque native pair, preventing an installed dependency from changing dispatch.

All these tests run without Java. Their success does not resolve process-clock
dependent initialization, arbitrary Java collections or complete loader behavior.
