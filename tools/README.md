# Go developer tools

Run these commands from the repository root. All three utilities use Go; Python
and fontTools are not required. The isolated `tools/go.mod` pins their dependencies
without adding them to the launcher's runtime module. `go -C tools` changes the
working directory, so explicit relative file arguments start from `tools/`.
Put flags before the positional source argument; quote paths containing spaces.

## Embedded seed map data

```sh
go -C tools run ./cmd/seedmap-data ../.cache/seedmap-reference
go -C tools run ./cmd/seedmap-data --check ../.cache/seedmap-reference
go -C tools run ./cmd/seedmap-data --output ../.cache/generated-data ../.cache/seedmap-reference
```

Reads the pinned local inputs listed in [`cubiomes/data/sources.json`](../cubiomes/data/sources.json)
and generates 11 files. The default output is `cubiomes/data/`. `--check` leaves
the output untouched and checks decompressed tables, exact JSON numeric values,
source hashes and the license. gzip timestamps are zero; compressed bytes may
differ from older encoders while the content stays the same. See the
[data provenance](../cubiomes/data/README.md) for obtaining the versioned inputs.

## C/Go performance comparison

```sh
go -C tools run ./cmd/cubiomes-benchmark --verify-only
go -C tools run ./cmd/cubiomes-benchmark --count 5
go -C tools run ./cmd/cubiomes-benchmark --fetch --reference-dir ../.cache/c-reference
go -C tools run ./cmd/cubiomes-benchmark --baseline /absolute/path/to/before.test.exe
```

GCC or Clang is required only for compiling the C reference; override its path
with `--cc`. `--fetch` downloads public source files at the pinned cubiomes
revision. Without it, all reference inputs are local. The default reference and
output directories are `.cache/seedmap-reference` and `.cache/cubiomes-comparison`.
Override the latter with `--output-dir`.

The command verifies C and Go hashes before running timed jobs serially. It keeps
the existing `metadata.json`, `c.csv`, `c-init.csv`, `go.txt`, `mcl.txt`,
`latest.txt`, `viewport.txt` and optional `before.txt` report formats. `--count`
controls Go repetitions; the C harness always performs five. A failed command
returns a nonzero status and never replaces a completed report with partial
output. See [benchmark methodology](../cubiomes/BENCHMARKS.md).

## Fusion Pixel wordmark

```sh
go -C tools run ./cmd/subset-brand-font /absolute/path/to/upstream.otf
go -C tools run ./cmd/subset-brand-font --output ../.cache/pixel.otf /absolute/path/to/upstream.otf
```

The default destination is `internal/nativeui/assets/pixel.otf`. This command
rebuilds the unshaped ASCII `YoloMCL` wordmark and space from the upstream Fusion
Pixel CFF OpenType font, with the modified family name `MCL Pixel Brand`. It
preserves the glyph outlines, advances, line metrics, copyright/license name
records and timestamps. Layout tables and hint programs are not carried into
this fixed pixel wordmark. It is not a general font subsetter for user text or
other font formats. The Go output need not be byte-identical to an older OTF.

`go-text/typesetting` reads the outlines, and `golang.org/x/image/font/sfnt`
independently validates the generated OTF. Both versions match those already
used by the launcher. The font remains under its [original OFL notices](../docs/THIRD_PARTY_NOTICES.md).

## Checks

```sh
go -C tools test ./...
bash scripts/check.sh --cross
```

The regular project checks include formatting, architecture, dependency
verification, vet, staticcheck, tests and cross-compilation for these commands.
Tests compare the generated wordmark against the checked-in font. To verify a
full upstream font as well, set `MCL_WORDMARK_SOURCE` to its absolute path and
run `go -C tools test ./cmd/subset-brand-font -run Wordmark -count=1`.
