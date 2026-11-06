# Seed map

Open an instance's **Seed map** tab beside **Overview**. The page is a map search
workspace: a large map fills the preview frame, with structure search on the
right. The **Save / seed** picker loads a save's seed and dimension settings
from `level.dat`, or selects manual input (initial seed `0`). The cube button in
**Worlds** also opens the same tab. Enter a signed 64-bit integer and choose
**Preview seed**. Drag to pan, scroll or use +/− to zoom, enter center X/Z to jump,
and choose **Origin** to return to X=0, Z=0. The dimension selector lists the
three vanilla dimensions first, followed by available custom dimensions.

Search structures by Chinese name, English name, or namespaced ID. Select a
structure to filter its map icons and automatically locate up to 32 nearest
candidates in the selected radius (2,048–32,768 blocks). Changing the radius
refreshes the results; **Find nearby** repeats the search at the current map
center. Click a result
to center the map and open its coordinates, biome and confidence; coordinates
can be copied. Structure names show both languages when available, including
map-related translations from installed assets. Unknown names retain their ID
in details. The layer button opens a legend of biomes in the visible tiles.
The structure list is built from the current instance's enabled mod and datapack
resources. Their `structure_set`, `structure` and biome tags determine the
entries; the active dimension's possible biomes filter the list, markers and
nearby search together. Pack namespaces need not match mod IDs. Returning from
the Mods tab rereads the sources, and the sidebar's refresh button reloads
external changes while preserving the current seed and map position.

Modern Overworld maps open in **Surface / contours** mode: estimated elevation,
northwest hillshade, contour lines and biomes sampled at the estimated ground
or water surface. Height follows cubiomes' depth estimator, evaluated by the
selected version's native Go density graph. Quart heights are interpolated for
close views. This is a topographic estimate, not generated blocks, caves,
overhangs, surface materials or a final-density heightmap. Supported datapack
depth splines and noise overrides participate automatically. A recognized
Overworld vertical depth gradient is required; older releases, Nether, End and
other depth models retain their biome maps without fabricated elevations.

Strictly positive upper caps and strictly negative lower caps preserve a
depth graph's zero crossing. Height estimation can use the underlying gradient
when those bounds are proven, while biome sampling keeps the entire original
graph. This covers Tectonic 3.0.29's Y-dependent `.8 / 1.3` depth cap without
flattening its mountain estimate. Terralith 2.6.5+26.3, Tectonic 3.0.29 and their
combined installed datapacks have been sampled through this path. It does not
establish an exact final-density heightmap for those packs.

The **Surface view** button switches back from a selected **Y slice**, initially 64.
Enter a height and choose **Show slice**; heights retain their exact block Y,
including negative heights. Maps open at one block per raster pixel. At one or
two blocks per pixel, native biome engines use the release's block-biome zoom
(modern 3D Voronoi, historical 2D Voronoi or native Beta climate). Zooming in
further magnifies by 2×, 4× or 8×. Grid cells represent **16×16-block chunks**;
the grid appears only when cells are large enough to read. Fine contours stay
one display pixel wide during magnification. Overview levels at four or more
blocks per pixel still use quart noise biomes. Coordinate ticks remain anchored
to world coordinates while dragging. Hover displays the sampled biome and
position; click opens its biome ID, chunk coordinates and estimated elevation,
with **Copy /tp** and a vanilla biome wiki link. Inspection uses immutable tile
data, not new synchronous generation. No terrain photographs or textures are bundled.
Structure markers use distinct building, ship, temple, city and fortress icons.
Each category has a consistent color. Nearby markers share a numbered badge;
click it to list and select every member, including structures at identical
coordinates, or zoom into the group. Individual markers show their set ID,
coordinates, source and confidence. **Hide structures** leaves the biome raster
visible without marker overlays.
**Map information** explains unavailable predictions and unverified behavior.

**Structure preview** opens a resizable 3D panel. Its model, height gauge,
mouse controls, Y range, block count and fidelity notes occupy separate areas.
Long notes wrap; short windows can scroll the content while the header stays
visible. The header's **Expand preview** button uses the full launcher content
area; **Restore preview size** or Esc returns to the floating panel. Click the
fidelity summary to read its complete explanation inline. Scrolling over the
model still zooms it; use the surrounding content or right gutter to scroll
the panel. The structure appears before surrounding terrain finishes loading;
that terrain is added in the background without resetting the camera. Dragging,
panning, fly-to and wheel zoom use adaptive lower-resolution frames, returning
to full resolution when interaction ends. Closing or replacing the preview
cancels its outstanding terrain work.

Green flags show saved **world spawn** and the local player's recorded **respawn
point** (bed/anchor), including its dimension. These come from `Data.SpawnX/Y/Z`
and `Data.Player.SpawnX/Y/Z/SpawnDimension`, not the player's current position.
All three coordinate fields must exist; zero is a valid coordinate. Click a flag
to see/copy its coordinates or use the spawn buttons to jump across dimensions.
The save is read only; server saves without embedded player respawn data have
no player flag. External multiplayer `playerdata` files are not scanned.

Versions with verified spawn-target rules show a clearly labeled **estimated
spawn area** for manual seeds and saves without spawn data, using `spawn_target` and vanilla horizontal
climate search. Terrain/surface checks and exact spawn Y are unavailable, so the
estimate displays only X/Z. Editing a save's seed hides its old saved flags.
Historical releases, 26.3 and unsupported dimensions currently show saved spawn
flags only, without a generated spawn-area estimate.

The preview runs entirely in Go. Data-pack definitions and source-based mod
adapters share the density/noise and biome-selection engine. There is no JVM,
HTTP service, or child process in this path. Opening the map does not start a
game, download a server, generate chunks or scan region files. See
[mod compatibility](seed-map-mods.md) for the exact verified releases. Existing
Worlds-tab storage accounting remains separate.

## Working coverage

### Local texture feasibility (not connected to the map renderer yet)

A standalone Go resource probe read the installed Mechanomania 1.1.6 client,
218 JAR files and its five enabled file resource packs in priority order. The
effective inventory contained 6,283 blockstates, 30,913 models and 18,619 texture
files. These counts describe resources, not supported world-generation mods.
Parent models, texture references and top faces resolved grass/sand/stone/snow
to the enabled Faithful 64x pack, AE2 sky stone/fluix to Upscaled AE2 32x and
Create limestone/casing to Improved Create 32x. A separate BOP 26.3 artifact
probe resolved its own 16x black/orange sand. No Java was started.

Minecraft already supplies some map decoration icons, including village types,
mansion, monument, jungle temple and trial chambers. These should be preferred
for corresponding markers. The installed Create JAR also supplies a station
decoration; this is an asset example, not a promise to predict player-built
stations. There is no universal structure-to-icon mapping for every installed
mod. Other markers need explicit metadata or an appropriate installed item/mod
icon, while the structure catalogue remains data-driven.

The map currently has biome IDs and estimated heights, not surface block states.
Accurate material selection needs a shared surface-rule evaluator and its
height/stone-depth/water/noise context; simply assigning grass to an entire
biome would discard sand, stone, snow and mod material rules. Rendering also
needs blockstate variants, model UV/rotation, biome tinting and PNG animation
metadata. The probe does not render custom Java model loaders or generate the
two dynamically registered resource packs in this instance. Static standard
JSON assets can share one resolver across mods, following the documented
[model format](https://docs.neoforged.net/docs/1.21.1/resources/client/models/).

Keep the visible grid at 16x16 blocks. Detailed views can use texture atlases;
overview levels should use cached downsampled colors, with no per-frame JAR
reads or one UI element per block. These are the proposed texture-rendering
steps, not features of the current elevation build.

### Generation coverage

| Input | Behavior |
| --- | --- |
| Beta 1.7/1.8 and releases 1.0–1.17.1 | Historical vanilla biome sources; large biomes from 1.3, End from 1.0 and outer End biomes from 1.9. Beta 1.7 shows climate biomes without sea-level estimation |
| Vanilla releases 1.18–1.18.2, 1.19–1.19.4, 1.20–1.20.6, 1.21–1.21.11 | Bundled climate registry data and versioned Overworld/Nether biome parameter trees; local vanilla JAR data overrides bundled definitions |
| Vanilla 26.1, 26.1.1, 26.1.2, 26.2, 26.3 | Independently pinned official data and Java reference results; 26.3 uses its float noise pipeline and new tree order |
| `minecraft:multi_noise` | Six climate channels and vanilla nearest-parameter lookup; normal, large-biome and amplified noise settings |
| `minecraft:the_end` | Legacy-seeded Simplex island density, five End biomes and the settings' erosion channel; supported datapack overrides are respected |
| Explicit biome parameter lists | Scalar/interval parameters, offsets, stable RTree construction and search; version-appropriate branching order |
| Named parameter lists / presets | Registry references, vanilla `overworld`/`nether` presets, and custom lists encoded with `biomes` |
| Installed mods and file datapacks | Noise settings, named density functions, noise parameters, dimension definitions, biome effects/tags and map-related English/Chinese asset labels override lower-priority data |
| Saved worlds | Modern `WorldGenSettings.seed`, complete supported dimension/generator/biome-source settings, inline noise settings and references; modern seed wins over legacy `RandomSeed` |
| Legacy saved worlds | `RandomSeed`, `generatorName`, `generatorVersion`, and empty `generatorOptions`; normal, large-biome and amplified biome maps. `default_1_1`, customized and legacy flat option strings stay unsupported |
| `minecraft:fixed`, flat-world biome settings, `minecraft:checkerboard` | Existing categorical previews, including saved settings and negative coordinates |
| `minecraft:random_spread` | Java LCG placement with linear/triangular spread, spacing, separation and salt; biome-tag filtering when the selected slice can be sampled |
| Custom Java density/generator/biome-source types | Affected dimension has no biome prediction and an opaque/unsupported diagnostic; independent supported dimensions still work |
| TerraBlender 26.3.0.0.9 / Biomes O’ Plenty 26.3.0.0.15 | Native Go weighted region layers, independent climate-tree histories and End selection; pinned installed artifacts and configuration select the adapter |

The pure-Go noise stack implements Xoroshiro128++, the legacy Java LCG,
positional factories and noise-ID hashing, ImprovedNoise, octave/amplitude
PerlinNoise and double Perlin NormalNoise. The climate compiler supports
`constant`, `noise`, `shifted_noise`, `shift`, `shift_a`, `shift_b`, named density
references, nested cubic `spline`, and the older 1.18 `terrain_shaper_spline`.
It also supports `add`, `mul`, `min`, `max`, `abs`, `square`, `cube`,
`half_negative`, `quarter_negative`, `squeeze`, `clamp`, `y_clamped_gradient`
and `range_choice`. Climate cache/interpolation markers pass through, as in
Minecraft's direct climate sampler. Blending uses new-chunk values
(`blend_alpha=1`, `blend_offset=0`); this is not a terrain interpolation engine.

Colors use explicit biome JSON `grass_color` when present, otherwise a readable
vanilla palette (blue oceans, green forests, sandy deserts, snowy whites, Nether
reds/teals and End yellows); unknown biome IDs use a stable categorical color.
Gray squares indicate unavailable
biome coverage. Manual previews start from the normal world preset, with
explicit dimension definitions taking precedence. Saved previews use the saved
dimension set; they do not substitute the current normal preset.
For old saves, recognized world-type metadata is normalized into explicit
dimension records. Old releases without a `world_preset` registry use internal
adapter records; these are not Minecraft datapacks. Overrides of legacy noise
settings disable the affected prediction instead of reverting to vanilla.

## Structures and confidence

Markers identify **structure sets**, at the centers of potential start chunks,
not a final `/locate` position. A structure's `biomes` field can be an ID, list,
or nested biome tag. Tag overrides append unless `replace` is true; missing
optional entries are ignored. The allowed biomes are the union of a set's
weighted alternatives. Matching candidates have `Confidence: biome_candidate`
and record the biome and Y used. Unresolved lists/tags keep unfiltered
`candidate` markers and report incomplete coverage.

Structure filtering uses the entered Y, also in surface mode. A candidate excluded at Y=64 can be
eligible at its actual start height, so changing Y can change markers. No
exact structure start height, terrain suitability, weighted structure choice or successful
building generation is promised. Modded sets have unverified dimension
eligibility and are displayed provisionally in selectable dimensions. Known
vanilla sets are restricted to their known dimension.

Bundled placement definitions cover six Overworld sets for 1.19–1.21: villages,
desert pyramids, jungle temples, swamp huts, ocean monuments and woodland
mansions. Installed definitions can add others. Unsupported rules (stronghold
rings, frequency reduction, exclusion zones, nonzero locate offsets and custom
placement codecs) are omitted with a diagnostic. Bundled biome tags distinguish
version changes such as pale-garden mansion eligibility from 1.21.5.

## Accuracy and deliberate limits

- Close views show **block biomes**, including native Voronoi boundaries.
  Overview levels show quart-cell noise biomes; fixed/checkerboard JSON sources
  retain their categorical preview. Surface blocks, existing generated chunks,
  upgrade blending and retrogen are outside this preview.
- RTree distance, float-to-long quantization, stable ordering and strict tie
  comparisons follow vanilla. Vanilla retains the preceding leaf on equal
  distances (the MC-241546 family of history-dependent biome boundaries).
  Tiles use deterministic raster-order history reset per tile; `BiomeAt` and
  structure checks use fresh searches. Boundary ties can consequently differ
  from a particular game's chunk-generation/query history or another zoom.
- All density types actually reached by the six climate channels must be
  supported. Missing/cyclic definitions and excessive graphs disable that
  dimension with a diagnostic. Unsupported functions used only in final terrain
  density do not prevent valid climate prediction.
- Injection-based generation uses source-verified native adapters. The adapter
  registry binds each mod's configuration and version to shared engine rules;
  the bounded `cubiomes/modjar` interpreter can also extract verified registration
  patterns from installed JARs. It does not execute arbitrary game logic during
  sampling. TerraBlender binding follows
  dimension-type tags, including custom types, nested tags and pack overrides.
  Mod manifests select known adapters; the presence of class files, entrypoints
  or mixins does not decide whether a map may render. Unrelated mods do not
  disable biome sampling. Unsupported generators or adapter inputs affect the
  dimensions that use them.
  Pack metadata and ordered overlays are supported. Enabled save file-packs
  follow `DataPacks.Enabled`; manual file-pack order is provisional. Additional
  mod families need their own source adapter and independent result fixtures.
  With KubeJS installed, instance `kubejs/data` and its ZIP packs participate in
  resource ingest. Ordering is checked against KubeJS 2101.7.2 build 348 source,
  including the interaction with the last enabled world ZIP. Stale directories
  without the KubeJS mod do not override a world. Active-loader metadata wins
  over an unused descriptor shipped in a multi-loader JAR. JSON array elisions
  preserve Gson's null slots instead of rejecting the entire resource.
- The bundle is a climate/biome/placement subset, not the entire version registry.
  Releases outside the listed ranges and snapshots have no bundled preset
  promise. Supported installed explicit JSON may still be sampled. Saved worlds
  with no usable dimension settings retain an honest candidate grid.
- Refresh or reopen the map after editing worldgen resources. Changing only the seed reuses
  the current immutable snapshot. The launcher neither enables packs nor changes
  the world or its contents.

## Ownership and resource use

`internal/launcher/worldgen*.go` owns bounded read-only ingest and content-addressed
snapshots. Each JSON resource may contain up to 16 MiB, including formatted
inline biome parameter lists. The combined resource byte budget remains 32 MiB;
nesting, entry and compiler limits still apply. Cache schema 11 invalidates older
snapshots and includes the active loader, KubeJS sources and their precedence.
The public [`cubiomes`](../cubiomes/README.md) package owns the pure-Go
noise, historical layers, density compiler, climate trees, End biomes, random-spread placement and
horizontal spawn estimate. Generators are immutable; each sampling job borrows
exclusive scratch from a reclaimable pool. Large legacy raster buffers are
discarded after the job. Block rasters reuse quart climate values and Voronoi
corner jitter while preserving every climate-tree lookup in raster order.
They do not cache the winning biome across history-dependent ties. Scratch is
bounded to 3 MiB per sampler and allocated lazily; sparse/wide ranges fall back
to scalar queries. `seedmap_structures.go` owns tag
resolution. `seedmap_sampler.go` draws tiles; `seedmap_tiles.go` owns the LRU,
cancellation and composition. `seedmap_search.go` owns bounded nearby searches;
`seedmap_spawn.go` owns the lightweight save picker and estimate presentation.
`internal/nativeui/seedmap.go` controls lifecycle; `seedmap_ui.go` lays out the
map and icons. `seedmap_render.go` retains per-tile MyGo bitmaps and receives
generation-checked, coalesced `App.post` callbacks. The launcher pins upstream MyGo v0.7.4;
no vendoring, local replacement or module-cache edits are used.

One shared tile/search worker sleeps on a channel when idle, with no ticker,
sweep or polling. Searching cancels stale prefetch; navigation cancels searches.
New viewports cancel prior jobs and replace queued work. The UI renders central
tiles first and receives immutable partial frames after the first new tile and
subsequent batches of at least four new tiles (at most sixteen partial frames).
Frames share immutable tiles rather than copying a full viewport image;
cached viewports need only one final frame. Visible tiles render before their
one-tile border. Closing, navigation, seed changes and shutdown
cancel the session. Zoom is 1–32 blocks/pixel, plus up to 8× magnification at
block precision. Y is bounded to −2048…2047 and viewport coordinates near the world
border. Tile identity includes dimension, Y, zoom, surface/contour mode and structure filter within
one seed/snapshot.

Camera movement uses fractional world coordinates and immediately transforms
cached tiles. During a drag, sampling is refreshed only on tile-boundary changes
and at most once per 80 ms. A release requests the final viewport. Inertia lasts
at most 450 ms; reduced-motion preferences disable it and tile fades. New tiles
fade in over 160 ms with at most 30 animation frames/second, keeping the previous
zoom layer behind them while loading. Loading dots repaint at 12 Hz. Settled
maps request no animation frames. Marker layouts are retained until the camera
or candidates change.

The display texture cache is separate from the CPU raster LRU: at most 192
entries and 64 MiB. Fine contours are baked once into magnified nearest-neighbor
bitmaps, avoiding thousands of vector commands on every pointer move. They are
rebaked when scale or theme changes; no per-frame contour traversal is needed.

The 192-entry LRU holds at most 12 MiB of 128×128 RGBA pixels plus point data
(under 37 MiB combined for all-surface tiles), shared palettes and bounded markers.
Surface tiles retain a one-pixel halo for continuous lighting/contours at seams.
Each sampler lazily allocates 480 KiB for bounded height/climate caches. Climate
vectors are cached instead of final biome IDs, retaining previous-leaf tie behavior.
Visible rasters are at most 1536×1024, preserving the actual preview frame's
aspect ratio while scaling to cover the frame. Climate graphs memoize results
per sample and use exclusively borrowed scratch. Limits include 128 dimensions (vanilla dimensions first), 4,096 graph
nodes, 2,048 noise octaves and 32,768 compilation steps per climate compiler,
16,384 parameter-list entries, and bounded nesting. Cumulative graph compilation
is capped around 32,768 nodes. There are at most 128 placement sets, 4,096
candidate checks and 256 hits per tile, and 2,048 hits per frame. The native UI
groups overlapping markers using a spatial index and caches each frame's layout;
structure symbols are never baked into biome pixels. Truncated output asks the
user to zoom in; cancellation is checked during compilation, raster
rows, batches of pixels and candidate traversal.

Ingest limits remain 16 MiB/JSON, 32 MiB total worldgen JSON, 32,768 worldgen resources, 64 nesting
levels, and 262,144 JSON separators. Saved dimension NBT has independent 4 MiB,
262,144-value, 256-dimension and nesting limits. Archives are bounded to 8 GiB,
131,072 entries and a 32 MiB central directory, including bounded ZIP64 indexes.
Language extraction has a separate 64 MiB / 8,192-entry budget and retains only
map labels; large translations cannot consume the worldgen byte budget. Directory
packs use `os.Root`, skip symlinks and enforce depth/entry limits.

Snapshots live in the launcher's `cache/seedmap/`, retain four entries and use
atomic writes plus content verification. Schema 12 hashes effective content,
provenance, version, selected world and packs, saved settings and diagnostics,
not file timestamps. Opening rereads local resources to compute the hash.
Native mod identity includes full artifact hashes and the relevant configuration
files. Seed changes reuse immutable compiled rules through an eight-entry mod
generator cache; resource/configuration changes produce a new snapshot key.

## Verification and data provenance

Run `bash scripts/check.sh`. Focused tests are in `seedmap_*_test.go`,
`worldgen_test.go` and `internal/nativeui/seedmap_test.go`; optional native PNGs
use `MCL_NATIVE_QA_OUTPUT`. Tests cover overrides, saved NBT, unsupported/cyclic
functions, splines, interval lookup/ties, tag merging/filtering, bounded work,
negative/extreme seeds, cancellation/LRU/idle behavior and native lifecycle/Y
changes. Nearby search checks at most 65,536 regions and returns at most 64
results (UI: 32), marking truncated work. It uses the same worker as map tiles.
The save picker reads metadata for at most 128 saves without region scans.
An opt-in `TestWorldgenInstalledSave` uses an external independently observed
case via `MCL_WORLDGEN_SAVE_CASE`, reads the installed save through the normal
service path and writes only temporary launcher caches. See the
[Mechanomania validation report](seed-map-verification-20261027.md) for the
real-game comparison, its remaining differences and the rendering benchmarks.
Map labels retain at most 8,192 keys per language document, excluding item/block
prose. Native tests cover bilingual search, result navigation, spawn identity,
dimension switching and full canvas aspect at 900×600 through 1920×1080.

Independent fixtures contain **792 Overworld/Nether biome queries** plus climate
vectors across seven release profiles and large biomes, with seven seeds including
both signed 64-bit extremes. Separate cubiomes fixtures cover both RNGs,
ImprovedNoise and NormalNoise (including skipped/weighted octaves), and **154
End biome queries** cover all five End biomes and far-End rings. Fixture
sources, generation commands and the one-unit depth tolerance are documented in
[`cubiomes/testdata/README.md`](../cubiomes/testdata/README.md).
These compare noise-biome algorithms; they are not a test of generated chunks.
The library additionally checks 1,792 C-reference Voronoi queries, shared-generator
concurrency, allocation-free sampling, bounded range validation and End cache
height/collision behavior.

Bundled data format, input hashes, version profiles and regeneration instructions:
[`cubiomes/data/README.md`](../cubiomes/data/README.md).

Algorithm references include [cubiomes](https://github.com/Cubitect/cubiomes/tree/e61f90580cbdd883214a8054670dacae655e59c0),
Minecraft's [RandomState](https://github.com/mahtomedi/minecraft/blob/main/src/main/java/net/minecraft/world/level/levelgen/RandomState.java),
[DensityFunctions](https://github.com/mahtomedi/minecraft/blob/main/src/main/java/net/minecraft/world/level/levelgen/DensityFunctions.java),
[CubicSpline](https://github.com/mahtomedi/minecraft/blob/main/src/main/java/net/minecraft/util/CubicSpline.java),
[Climate RTree](https://github.com/mahtomedi/minecraft/blob/main/src/main/java/net/minecraft/world/level/biome/Climate.java),
and the older [TerrainShaper](https://github.com/jacobo-mc/mc_1.18.1_src/blob/main/src/main/java/net/minecraft/world/level/biome/TerrainShaper.java).
