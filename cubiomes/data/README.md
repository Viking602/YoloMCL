# Bundled climate data

These files are a small, offline climate subset. The launcher does not download
these sources at runtime. Installed JAR/datapack definitions override them.

## Sources

- Overworld parameter trees and noise-algorithm reference:
  [Cubitect/cubiomes](https://github.com/Cubitect/cubiomes/tree/e61f90580cbdd883214a8054670dacae655e59c0),
  commit `e61f90580cbdd883214a8054670dacae655e59c0`. The MIT license is preserved in
  `LICENSE-cubiomes.txt`. Trees preserve every interval, child order and leaf ID;
  they are not a nearest-neighbor approximation or a biome image.
- The 1.21.5+ tree comes from [cubiomes PR #162](https://github.com/Cubitect/cubiomes/pull/162),
  pinned fork commit `ca9fcb4e2b4f29a88b15a19fe1f00a73bf503ec2` in
  `Zes-MinKey-Young/cubiomes` (same MIT license). This unmerged upstream
  contribution updates 20 dark-forest leaves to pale garden, preserving climate
  intervals and tree order. The author compared the vanilla 1.21.11 data report;
  [Minecraft 1.21.5 release notes](https://www.minecraft.net/en-us/article/minecraft-java-edition-1-21-5)
  confirm the expanded pale-garden distribution. Targeted fixtures cover both
  changed and unchanged pale-garden locations.
- Vanilla registry JSON: [misode/mcmeta](https://github.com/misode/mcmeta), immutable
  release tags such as `1.20.1-summary`. URL pattern:
  `https://raw.githubusercontent.com/misode/mcmeta/<version>-summary/data/worldgen/<registry>/data.min.json`.
  Biome tags use `data/tag/worldgen/biome/data.min.json`.
  `sources.json` records SHA-256 hashes of the exact generator input files.

`registries.json.gz` is projected from 1.20.1 noise, density_function,
noise_settings, world_preset, multi_noise_biome_source_parameter_list, structure
and structure_set data. Only the six climate router fields are retained in
noise settings, plus `spawn_target` for horizontal spawn estimates. End erosion
retains its `end_islands` density function. Density/noise entries remain available for datapack references;
unsupported types fail if reached. The six bundled structure sets retain their
real placement definitions; only biome eligibility is retained for structures.

`registries18.json.gz` uses 1.18.2 noise/density/noise_settings, including the
legacy terrain shaper. It shares the equivalent normal dimension generator
references and named preset descriptors from 1.20.1. The 1.19.2/1.19.4/1.21.4
climate density definitions and the 1.21.11 six router fields were compared with
the common profile. This is not a promise of identical full terrain registries.

`biomes.json.gz` holds biome effects and tags from 1.18.2, 1.19.2, 1.20.1,
1.21.1, 1.21.4 and 1.21.5, selected for their corresponding release families.
The 1.21.5 tag profile includes pale gardens in woodland-mansion eligibility;
1.21.11 was checked for that eligibility. Only explicit grass color affects the
palette; otherwise vanilla IDs use a conventional readable palette and unknown
IDs use a stable categorical color.

| Releases | Tree file | Branching order |
| --- | --- | --- |
| 1.18–1.18.2 | overworld18 | 10 |
| 1.19–1.19.2 | overworld192 | 10 |
| 1.19.3–1.19.4 | overworld19 | 6 |
| 1.20–1.21.3 | overworld20 | 6 |
| 1.21.4 | overworld21 (btree21wd) | 6 |
| 1.21.5–1.21.11 | overworld215 (PR #162) | 6 |

Nether's five parameter entries are expressed directly in `../vanilla.go`.
Preset IDs are Java builtins, so their registry JSON alone cannot supply the
Overworld's thousands of parameter entries. No Java implementation is embedded.

## Native TerraBlender and BOP profiles

`overworld-1.20.1-parameters.json.gz` contains the 7,593 original ordered vanilla
parameter entries needed by TerraBlender's modified builder. Ordered rewrites
preserve duplicate entries; deriving this list from a sorted search tree would
lose registration semantics. It was exported from the actual Minecraft 1.20.1
builder. Source commits, reference procedure and hashes for the native JAR
registration path are in
[`terrablender-1.20.1.provenance.json`](terrablender-1.20.1.provenance.json).

The `terrablender-*.json.gz` files provide version-pinned region tables, End
weights, and ordered enabled-biome fallback choices for the native Go selectors.
They include separately verified TerraBlender-only and TerraBlender+BOP profiles.
Their exact artifact identities, public-source references, config semantics,
test coverage, and license facts are documented in
[`terrablender.README.md`](terrablender.README.md). The implementation composes
these tables with the effective datapack climate graph; it does not require
Java or a network service.

## Format and regeneration

Each `.bin.gz` expands to two little-endian uint32 counts (intervals and nodes),
then pairs of int32 interval endpoints, then uint64 cubiomes node records. The
high 16 bits identify the first child or leaf biome; six low bytes index climate
intervals. Child steps/order follow the corresponding cubiomes table. All
vanilla offsets are zero. The Go decoder bounds counts and indices.

Download the five upstream cubiomes `tables/btree*.h` files and `LICENSE` into a
scratch directory, plus `tables/btree215.h` from the pinned fork. Download the
registry summaries above as `<registry>.json` for
1.20.1 and `<version>-<registry>.json` for other profiles; biome-tag files are
named `biome_tag.json`. Required inputs and their hashes are in `sources.json`.
Run from the repository root:

```sh
go -C tools run ./cmd/seedmap-data ../.cache/seedmap-reference

# Verify generated content without rewriting the embedded files.
go -C tools run ./cmd/seedmap-data --check ../.cache/seedmap-reference
```

The Go tool keeps sources outside the application tree, projects JSON, preserves
tree order and uses gzip with zero timestamps. Its `--check` mode compares binary
tables byte-for-byte after decompression and JSON values without losing integer
precision. Different gzip encoders may produce different compressed bytes.
The application/tests require only the checked-in embedded data and Go. Changes
to bundled semantics, ordering or snapshot interpretation require a snapshot
schema bump so old cached maps are not silently reused.

The historical implementation also ports the pinned cubiomes `layers.c` rules,
biome identifiers/categories from `biomes.c` and the 4,096-entry Beta 1.7 climate
table in `biomenoise.c` (`../beta_table.go`). They use the same MIT license and
revision above. Historical adapter records are generated locally and are not
Minecraft datapacks; their reference fixtures are described in
[`../testdata/README.md`](../testdata/README.md).
