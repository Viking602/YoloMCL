# TerraBlender numeric profiles

These files supply numeric biome-selection data for the native Go implementation.
They contain no Java implementation, mod artwork, HTTP service, or game runtime.
The exact supported reference artifacts and hashes are recorded in
`terrablender.provenance.json` and `terrablender-only-26.3.provenance.json`.

## Minecraft 1.21.1 profile

`terrablender-bop-1.21.1.json.gz` and its fallback table target TerraBlender
4.1.0.8, BOP 21.1.0.14 and GlitchCore 2.1.0.0. Installed Fabric artifacts are
checked by SHA-512. This profile is not a binding for other TerraBlender consumers
or for NeoForge's Regions Unexplored. The separately named TerraBlender-only
profile uses the same versioned native selectors.

The reference compiles unchanged public BOP regions and builders against the
game and TerraBlender APIs, with minimal config holders supplying toggle masks.
It is not a full Fabric boot. The 69 toggle flags, default, all-disabled,
single-enabled, priority and mixed configurations yield 92 complete table hashes
covering 2,311,868 entries. Source-file hashes, commits, reference-artifact hashes
and profile-artifact identities are in
[`terrablender-1.21.1.provenance.json`](terrablender-1.21.1.provenance.json).
Only numeric maps, identifiers and expected results are bundled.

The 4.1.0.8 End source passes its edge entries to the island-area constructor;
the native implementation preserves that release's behavior and island size.
Its source contract and preview loading are checked, but there is not yet an
independent complete 1.21.1 End raster fixture. Real saved Overworld comparisons,
including BOP and Lithostitched injection, are documented in
[`seed-map-packs-verification-20261028.md`](../../docs/seed-map-packs-verification-20261028.md).

## Minecraft 26.3 profiles

The profiles cover Minecraft 26.3 and TerraBlender 26.3.0.0.9, either alone or
with Biomes O' Plenty 26.3.0.0.15. The BOP reference also loaded GlitchCore
26.3.0.0.5. Both references used Fabric API 0.165.0+26.3 and Fabric Loader 0.20.1.
The Go engine does not load those components; the artifact identities let callers
select the matching rules for an installed game instance.

`terrablender-bop-26.3.json.gz` preserves the ordered parameter boxes returned by
the public `Region.addBiomes` API, region indices and weights, End biome weights,
and the default enabled-biome configuration. The quantized integer axes are
temperature, humidity, continentalness, erosion, depth, weirdness, and offset.
Box order, duplicate boxes, and End entry order are significant. Region zero has
no embedded parameter boxes: it uses the supplied generator's effective base
tree, including datapack replacements. Default BOP regions contain 8,373 boxes
each in the Overworld and five boxes each in the Nether.

`terrablender-bop-26.3-fallbacks.json.gz` represents all 68 enabled-biome toggles
as 246 ordered candidate chains indexed by climate entry. Some entries have four
BOP candidates before their vanilla fallback. Disabling a biome therefore cannot
be implemented as one global name replacement. The chains were derived through
unmodified public region constructors in isolated loader instances with their
configuration supplied before the first config access: 68 single-enabled cases,
one all-disabled case, nine priority-disambiguation cases, and eight independent
mixed masks. All 86 complete ordered tables match the Go factories, covering
2,161,094 entry comparisons. Parameter geometry was hashed independently on every
reference run and remained unchanged. End toggles omit the disabled registrations
without reordering the survivors.

TerraBlender accepts region and End sizes from two through six. Its config reader
reverts invalid numbers to that field's default, rather than clamping. Weights
range from zero through signed 32-bit maximum; an overflowing total is invalid.
Zero-total End tables return `minecraft:ocean`: `BiomeInitialLayer` overrides
the generic weighted layer's default index. The exported raw registry-ID-zero
metadata is informational and does not determine this fallback. A mixed config
oracle with zero-total midland and island tables exercises the distinction.

`terrablender-only-26.3.json.gz` records a separate boot without BOP or GlitchCore.
TerraBlender still changes End biome selection when no additional End biomes are
registered: its erosion lookup uses quart-cell block coordinates. Both profiles
therefore use the native TerraBlender End path.

The reference was collected through the published APIs of the unmodified,
hash-verified artifacts. Fabric's normal loader initialized the mods; dynamic
registries were loaded with its public resource/registry interfaces. No game
server, world, listening socket, private-field access, or decompilation was
needed. Reference Java files and runtimes are kept outside this Go repository.
`Region.addBiomes`, `IExtendedParameterList.getUniqueness/findValuePositional`,
`EndBiomeRegistry`, and the actual biome-source resolver supplied the data and
expected results independently of the Go implementation.

The default BOP and TerraBlender-only fixtures each check 110,592 raster cells
across three seeds, all three dimensions, negative coordinates, and world-border
coordinates. They also retain 3,584 and 1,024 ordered climate-boundary targets,
respectively. Raster hashes are FNV-1a-64 over biome identifier plus newline in
z-major order, starting with a fresh resolver for each raster. Region hashes use
decimal region index plus newline. Coordinates in raster fixtures are block
coordinates; the explicit boundary targets use quart coordinates.

The BOP plus datapack fixture adds 245,760 cells for vanilla, Terralith 2.6.5,
Tectonic 3.0.29, and their ordered combination. Each reference loaded the actual
datapacks above the mod resources. The complete BOP region arrays were unchanged
across these four cases; their base parameter tree and effective noise definitions
were sampled separately. This is why a region table can be composed with a
compiled datapack generator without replacing its climate graph.

Two further config fixtures contain 221,184 raster cells: all 68 biomes disabled
with default region settings, and a mixed mask with size endpoints two/six,
zero-weight regions, nondefault positive weights, and zero-total End tables.
The runtime table contents were again compared directly with the derived
fallback choices before packaging the independent resolver results.

Public source references:

- [TerraBlender region initialization and base-tree selection](https://github.com/Glitchfiend/TerraBlender/blob/7d7a778ab9c14d0091581ed51ada0821e69761b8/common/src/main/java/terrablender/mixin/MixinParameterList.java)
- [TerraBlender dimension/source initialization](https://github.com/Glitchfiend/TerraBlender/blob/7d7a778ab9c14d0091581ed51ada0821e69761b8/common/src/main/java/terrablender/util/LevelUtils.java)
- [TerraBlender End selection](https://github.com/Glitchfiend/TerraBlender/blob/7d7a778ab9c14d0091581ed51ada0821e69761b8/common/src/main/java/terrablender/mixin/MixinTheEndBiomeSource.java)
- [TerraBlender configuration bounds](https://github.com/Glitchfiend/TerraBlender/blob/7d7a778ab9c14d0091581ed51ada0821e69761b8/common/src/main/java/terrablender/config/TerraBlenderConfig.java)
- [BOP enabled-biome fallback behavior](https://github.com/Glitchfiend/BiomesOPlenty/blob/cc292da4989591b081bc57326648a4ac3724ca8d/common/src/main/java/biomesoplenty/util/biome/BiomeUtil.java)

These source commits identify the public source inspected, not an assertion about
the artifact publisher's build checkout. Artifact hashes identify the executed
reference. TerraBlender's relevant region, noise-layer, and initialization sources
carry LGPL-3.0-or-later notices; inspect individual file notices as well. BOP's
repository license says “All rights reserved.” No BOP implementation source or
assets are included here: its profile consists of functional numeric mappings,
biome identifiers, configuration values, and independent expected results.
