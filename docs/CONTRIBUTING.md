# Development Guidelines

## Code Style and Formatting

This repository follows standard [Effective Go](https://go.dev/doc/effective_go) conventions and enforces consistent formatting across command-line tools and editors through [`../.editorconfig`](../.editorconfig).

- **Encoding and whitespace**: UTF-8, LF line endings, final newline required, trailing whitespace trimmed (except in Markdown). Go files use tabs (`tab_width = 4`); YAML, JSON, and shell scripts use 2 spaces.
- **Formatting and imports**: Format all Go source files with `gofmt` and `goimports` using `-local github.com/Viking602/YoloMCL`.
- **Import grouping**: Let `goimports` maintain a single `import (...)` declaration, with blank lines separating these groups:
  1. Standard library packages
  2. Third-party packages
  3. Local repository packages (`github.com/Viking602/YoloMCL/...`)
  Remove redundant import aliases when the package name matches the path suffix. Do not preprocess Go source with text substitutions to format imports: raw string literals can contain import-like text.
- **Naming conventions**:
  - Package names must be short, concise, lowercase words without underscores or mixedCaps.
  - File names must reflect domain responsibility (such as `mod_install.go` or `log_parse_test.go`), never iteration numbers or temporary labels.
  - Keep standard initialisms in consistent case (`ID`, `URL`, `HTTP`, `JSON`, `API`).
  - Write doc comments beginning with the exported identifier's name. Each package needs a package comment (prefer `doc.go`) describing its responsibility and relevant concurrency or platform constraints.
- **Error handling and credentials**:
  - Handle every error explicitly or document why ignoring it is safe.
  - Preserve error chains with `%w` (`fmt.Errorf("...: %w", err)`) when callers inspect underlying causes.
  - Never include API keys, tokens, or secret payloads in error strings, logs, or UI text.
- **Editor configuration**: Configure your editor to apply `.editorconfig` and run `gofmt` and `goimports` (`-local github.com/Viking602/YoloMCL`) with standard-library, third-party, and project-local import grouping on save. `tools/` is an isolated Go module for developer tooling; if your IDE prompts to register `tools/go.mod` as an additional module in the project, you may enable it locally, but never commit personal workspace files (`go.work`, `go.work.sum`) or IDE project directories.

## Local Checks

Run checks from the repository root. All three desktop platforms use the same `Makefile` targets. Requires GNU Make, Bash, OpenSSL 3, and standard tar/gzip/zip utilities (Windows uses its built-in tar for ZIP). On Windows, run from Git Bash with GNU Make installed; on macOS, set `OPENSSL` if needed to point to Homebrew OpenSSL 3:

```sh
make check
make fmt
make check CHECK_ARGS="--no-tests --cross"
```

By default, `make check` runs in order:

1. Source formatting and import group order (`goimports -local github.com/Viking602/YoloMCL`).
2. Package dependency boundary checks (`scripts/check-imports.awk`).
3. Module tidiness (`go mod tidy -diff`) and dependency verification (`go mod verify`) for both `.` and `tools/`.
4. `go vet ./...`.
5. `staticcheck ./...` with all correctness, simplification and style checks enabled by `staticcheck.conf`, in both development and release (`-tags=mygo_noinspector`) modes.
6. Shell regression tests (`scripts/test.sh`) and Go tests (`go test ./...`) in both development and release modes.

`make fmt` (or `bash scripts/check.sh --fix --format-only`) normalizes formatting and checks imports, without compiling or running tests. It never rewrites application logic. Use `make check CHECK_ARGS="--fix"` to format and then run all checks. `--cross` additionally runs `go vet` and `go build` (both development and release tags) across `windows`, `darwin`, and `linux` on `amd64` and `arm64`. Cross-compilation verifies buildability, not target-OS runtime behavior.

Tool versions (`goimports`, `staticcheck`) are locked in `tools/go.mod` and `tools/go.sum` so they do not enter launcher runtime dependencies. When updating tools, update and verify both files explicitly.

GitHub Actions runs the same check command on Windows, macOS, and Linux runners. Unexecuted CI is never evidence that tests pass. When changing business behavior, add deterministic tests that catch real regressions (using `t.TempDir()` for temporary files); do not duplicate tests when merely moving code between files.

## Building and Configuration

```sh
make build
make build BUILD_ARGS="--release --output build/YoloMCL.exe"
make build BUILD_ARGS="--goos linux --goarch arm64 --output build/YoloMCL-linux-arm64"
```

`Makefile` delegates to `scripts/build.sh`, which runs `go build` directly from Bash (you can also invoke `bash scripts/build.sh` with the same flags). Bash parses configuration and resolves Git metadata; standard archive tools and OpenSSL 3 handle packaging and Ed25519 signing. The build has no Python dependency. No auxiliary Go build executables or server-side proxy components are kept in the repository.

Publisher configuration is read in strict precedence order:

1. Process environment variables (`MS_CLIENT_ID` / `MCL_MICROSOFT_CLIENT_ID`, `CURSEFORGE_API_KEY` / `MCL_CURSEFORGE_API_KEY`)
2. Repository-root `.env` file

`.env` is parsed as literal key-value text: no shell execution, no variable expansion, and no parent or user-directory traversal. Quote values containing special characters and place inline `#` comments after whitespace. Never commit `.env` or real publisher configuration files.

## Architecture and Platform Boundaries

For layout and idioms, use the official [module layout guide](https://go.dev/doc/modules/layout) and [Go code review comments](https://go.dev/wiki/CodeReviewComments). Split files by responsibility within an existing package before adding a new package. A new package needs a distinct owner/responsibility and an acyclic dependency boundary; there is no arbitrary file-length limit.

Keep small private helpers and constants beside the feature that owns them; do not create a standalone file for a lone constant or a trivial forwarding function. Split a file when it mixes independent responsibilities (such as protocol authentication, inventory scanning, resource loading, or widget styling), and move the related tests with the feature. Stateless logic alone does not justify a child package or a layer of forwarding functions. Preserve explicit platform, lifecycle, credential, build configuration, and translation-data boundaries.

Follow the package dependency direction defined in [`architecture.md`](architecture.md) and enforced by `scripts/check-imports.awk`:

- `main` → `internal/app`
- `internal/app` → `internal/buildinfo`, `internal/launcher`, `internal/nativeui`
- `internal/nativeui` → `internal/launcher`, `internal/buildinfo`, `internal/nativeui/i18n`
- `internal/launcher` → `internal/config`, `internal/launcher/{memory,proctree,textenc,vault}`
- `internal/config`, `internal/buildinfo`, and leaf sub-packages → no other internal package dependencies

Platform and boundary rules:

- **UI and worker isolation**: Only `internal/app` and `internal/nativeui` may import `github.com/egoist/mygo`. UI code must not directly implement game installation, file backup, or disk scanning; background workers must not mutate UI state directly and must post results via `App.post` with generation checks.
- **Configuration ownership**: Hold `Service.mu`, clone the configuration (including nested maps), mutate the draft, then call `saveConfigLocked`. Publish only after successful persistence. Never return shared mutable maps or change process-wide HTTP clients from package initialization.
- **External input**: Bound decompressed sizes, image dimensions, collection lengths and nesting depth before allocation. Carry request cancellation into network and filesystem work, and preserve recoverable backups when persistence fails.
- **Path and OS isolation**: Use `path/filepath` for local filesystem paths and `path` for URLs and protocol paths. Place OS-specific APIs in `*_windows.go`, `*_darwin.go`, `*_linux.go`, or build-constrained files rather than scattering platform branches across shared logic.
- **Bounded disk scanning**: Modpack and instance scans only access registered Minecraft directories and isolated launcher configuration. Java detection checks installation directories, environment variables, and OS registries in the background after the first frame, and must not restart on every instance switch.
- **MyGo dependency**: Keep the `github.com/egoist/mygo` version pinned and unmodified. Application startup belongs in `internal/app` and uses MyGo's public APIs; never modify the Go module cache, vendor MyGo, or commit `replace` directives pointing to private forks. Verify window startup, renderer settings and UI behavior before upgrading MyGo.
- **Build artifacts**: Write build deliverables to `build/`, local temporary files to `.cache/`, and test artifacts to `t.TempDir()`. Keep `rsrc_windows_amd64.syso` in the repository root as the Windows `amd64` linker resource input; it is unused on macOS, Linux, and Windows `arm64`.
