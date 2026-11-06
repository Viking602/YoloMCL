# Mechanomania seed-map verification — 2026-10-27

## Actual game and input

Minecraft 1.21.1, NeoForge 21.1.219, installed Mechanomania 1.1.6,
seed `2904241419426706349`. Minecraft was launched through the launcher, a copy
of the save was opened, and `/seed` confirmed the value in game. Subsequent
checks ran in the background, using read-only extraction of already generated
MCA chunks and headless UI tests. No `/locate` result was obtained; structure
checks below use saved structure starts, not an assumed command response.

The original save was not modified. Production generation remains pure Go,
without a JVM, HTTP bridge, game process or MCA scan. The installed game was
used only as an independent development oracle.

## Fixes

The launcher omitted `kubejs/data`, which supplies the pack's Tectonic density
functions, noise settings, structure placement and biome-tag overrides. At
`X=664, Y=68, Z=280`, vanilla Go and the pinned C cubiomes reference both returned
`minecraft:stony_shore`; the actual pack returns `minecraft:deep_ocean`.
The corrected instance snapshot now returns **deep ocean** there.

Ingest now handles KubeJS loose data and ZIPs, dimension-type resources, the
active loader's descriptor in multi-loader JARs, and Gson's omitted array values
without shifting feature-step indices. The density compiler accepts equal
spline knots in the older supported releases. Surface estimation can unwrap
zero additions and proven sign-preserving range caps, while climate sampling
still evaluates the complete graph. Cache schema 11 invalidates previous input.

The pack overrides trial-chamber biome eligibility with
`c6c:end_flower_forest`. The trial-chamber marker therefore no longer appears in
the Overworld. KubeJS also changes the windmill placement parameters; the old
marker near the screenshot was produced from the wrong resource layer.
Structure details and copied coordinates no longer present the selected slice
Y as a generated structure's height; copied coordinates use `X ~ Z`.

KubeJS ordering follows the installed 2101.7.2 build 348 source at
[commit 46feea3451092cd85d46aba61237527bbd33f07c](https://github.com/KubeJS-Mods/KubeJS/blob/46feea3451092cd85d46aba61237527bbd33f07c/src/main/java/dev/latvian/mods/kubejs/server/ServerScriptManager.java).
This is not verification of every historical KubeJS implementation or arbitrary
JavaScript registration callback.

## Measured correctness

| Check | Result |
| --- | --- |
| Normal service ingest of the selected save | 216 top-level mod identities, 4,883 effective resources; zero unreadable worldgen sources |
| Service-path regression | All 36 independently observed block-biome points matched; seed, resource provenance and excluded trial chambers checked |
| Saved full-status chunks | 3,484 Overworld chunks |
| Quart-biome samples at Y −28, 4, 68, 132, 196 | 278,652 / 278,720 identical names; **68 differing boundary choices** |
| Saved random-spread structure start chunks | 18 matched, zero wrong start chunks |
| Other saved structure starts | Two not covered by this placement check: vanilla stronghold and Repurposed Structures taiga temple |

All 68 differing biome samples select parameters at exactly the same minimal
climate distance as the saved biome. Minecraft retains the previous winning
leaf on ties, so sampling history matters. The preview's tile traversal does not
reconstruct the game's generation-thread history. These are still name
differences, and are **not counted as exact matches**.

Matching random-spread start chunks does not establish that every displayed
candidate builds successfully. Actual structure height, terrain suitability,
weighted choices, jigsaw pieces and custom code are not all simulated. Slice-Y
biome filtering can omit a structure that is eligible at its true generation
height. C6C's custom End biome source remains unsupported. This check therefore
does not certify every mod, datapack, dimension or structure.

Relief is still the documented depth-based estimate. At the screenshot point
the estimate is approximately 36.34; the game reports ocean floor 26 and water
surface 62. A biome-correct map is not a final generated-block heightmap.

## Performance and rendering

Windows amd64, Ryzen 7 9800X3D, Go 1.27.3; three benchmark runs. These measure
CPU-side work, not GPU utilization or displayed FPS.

| Work | Previous path | Retained tiles |
| --- | --- | --- |
| Cached pan frame | 104.7–117.3 µs | 4.74–4.83 µs |
| Allocation per cached pan | 1,583,673–1,583,674 bytes | 10,744 bytes |
| Fine-contour paint command construction, same 768×512 viewport | 1.80–1.88 ms | 1.65–1.73 µs |
| Paint primitives in that viewport | 22,368 | 4 |

The contour benchmark excludes first-time texture baking. Its vector baseline
culls offscreen contours. Headless tests cover fractional panning, reuse without
texture uploads, bounded memory, reduced motion, coalesced deliveries and zero
animation requests after settling. The real pack's service snapshot was also
rendered headlessly at the reported seed and coordinates and visually checked.

## Reproduction

Run the normal checks with `bash scripts/check.sh --cross`. Focused regression
tests and benchmarks:

```text
go test ./cubiomes -run 'TestRepeatedSplineKnotsBeforeFloatRelease|TestSurfaceDepthCaps' -count=1
go test ./internal/launcher -run 'TestSeedMapServiceKubeJSResources|TestWorldgenKubeJS2101PackOrder|TestWorldgenSelectsActiveModDescriptor|TestWorldgenArrayElisions' -count=1
go test ./internal/launcher -run '^$' -bench BenchmarkSeedMapCachedPan -benchmem -count=3
go test ./internal/nativeui -run '^$' -bench BenchmarkSeedMapReliefPaint -benchmem -count=3
```

`MCL_WORLDGEN_SAVE_CASE` enables `TestWorldgenInstalledSave`. Its external JSON
contains `World` (absolute save directory), `Seed` (decimal string), `Points`
(`Dimension`, `X`, `Y`, `Z`, `Biome`), `Sources` (effective resource key to source
ID), and `AbsentStructures` (`Dimension`, `ID`). Expected coordinates must come
from the game or saved chunks, not from this engine. The test uses temporary
launcher configuration and cache paths. `MCL_WORLDGEN_SAVE_EXPORT` optionally
exports the effective snapshot. Set `MCL_WORLDGEN_SNAPSHOT` to that export and
`MCL_WORLDGEN_SEED` to the observed seed for `TestSeedMapInstalledSnapshotPreview`;
`MCL_NATIVE_QA_OUTPUT` saves its headless PNG.

The local bulk comparison inputs and logs are in `.cache/seedmap-game-oracle.json`,
`.cache/seedmap-mismatches.json`, `.cache/seedmap-structures-compare.log` and
`.cache/seedmap-installed-test.log`. They are development evidence, not bundled
game data or a production dependency.
