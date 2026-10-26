<p align="center"><img src="docs/logo.png" width="128" alt="YoloMCL"></p>

# YoloMCL

A native Minecraft: Java Edition launcher written in Go with [MyGo](https://github.com/egoist/mygo) for Windows, macOS, and Linux.

[Download](https://github.com/Viking602/YoloMCL/releases/latest) · [中文介绍](docs/README.zh-CN.md) · [Development guidelines](docs/CONTRIBUTING.md) · [Architecture](docs/architecture.md) · [Release guide](docs/releases.md)

## Screenshots

### Instance overview

![Instance overview with installed content, recent worlds, Java runtime, and storage usage](docs/screenshots/instance-overview.png)

### Mod management

![Installed mods with search, enabled and update filters, and per-mod actions](docs/screenshots/mod-management.png)

### Resource discovery

![Modpack discovery with Modrinth and CurseForge sources, version filters, and categories](docs/screenshots/resource-discovery.png)

## Features

- **Instances and modpacks**: Install and manage vanilla, Fabric, Quilt, Forge, and NeoForge instances. Import `.mrpack` and CurseForge `.zip` modpacks (including drag-and-drop), export installed instances in either format, review pack updates, and keep backups of overwritten files.
- **Content discovery**: Browse Modrinth and CurseForge with continuous loading; install mods, resource packs, shaders, and modpacks with automatic dependency resolution.
- **Game management**: Manage worlds, backups, screenshots, recent multiplayer servers, and structured game/crash logs.
- **Java and memory**: Detect installed Java runtimes and allocate memory from pack size and available RAM.
- **Accounts and cosmetics**: Sign in with offline profiles, Microsoft accounts, or Yggdrasil/authlib-injector services, subject to the platform limits below. Preview skins and switch capes.
- **Desktop UI**: High-DPI scaling, bundled JetBrains Mono font with system CJK fallback, and English / Simplified Chinese localization following the system language by default.
- **Rendering**: Windows defaults to software rendering; interface hardware acceleration can be enabled in Settings (restart required).
- **Verified updates**: Check for launcher updates through GitHub Releases, with confirmation before downloading and signed policies for mandatory security updates. Downloads are verified with an embedded Ed25519 public key before installation; restarting remains an explicit action.

### Export a modpack

Select an instance, then open **Settings → Export modpack → Export…**. Choose **Modrinth (.mrpack)** or **CurseForge (.zip)**, enter a pack name and version, and select the folders/files to include. **Save and export** opens the native save dialog. Progress and cancellation are available in **Tasks & Logs**; a completed task has **Show exported file**.

The export uses the selected instance's Minecraft and loader versions, including isolated version folders. Mods, configuration and scripts are selected by default; worlds, screenshots, server lists and personal options are opt-in. Launcher credentials, runtimes, game libraries, logs and caches are excluded. Disabled mods keep their disabled filenames.

Verified platform files become download references; local, modified or unidentified files go into `overrides/`. If a provider cannot be reached, unidentified files are bundled so local exports can still finish. CurseForge references use project/file IDs, while Modrinth references include download URLs and SHA-1/SHA-512 hashes. Files are streamed with bounded buffers, and an existing export is replaced only after the new archive finishes. Save exports outside the game directory.

## Downloads and updates

Release versions come from Git tags, starting with **v1.0.0**. Settings shows the version, source commit, and repository links.

| Platform | Architecture | Download |
| --- | --- | --- |
| Windows | x64 / ARM64 | `YoloMCL-<version>-windows-<arch>.zip` |
| macOS | Intel / Apple Silicon | `YoloMCL-<version>-darwin-<arch>.tar.gz` containing `YoloMCL.app` |
| Linux | x64 / ARM64 | `YoloMCL-<version>-linux-<arch>.tar.gz` |

Extract the complete archive before launching. Keep the `YoloMCL.exe` / `YoloMCL` filename and install in a directory you can write to. On macOS, move the entire `.app` bundle to a writable Applications directory. Linux requires GTK 3, Pango, Cairo, and Fontconfig; install a CJK font for Chinese text.

The launcher checks once after the first frame and offers available stable updates with confirmation and cancellation. You can also use **Check for updates** in **Settings**. A verified minimum-version policy can require an update before continuing; publisher configuration and compatibility limits are described in the [release guide](docs/releases.md#require-a-security-update). Restarting after installation remains an explicit action. Development builds, renamed executables, and read-only installations provide a GitHub download link instead of in-place updates. Update signing is separate from operating-system trust: Windows packages currently have no Authenticode certificate, and macOS bundles are ad-hoc signed, not notarized.

### Current platform limits

All six targets are built by the release workflow. Some existing features are not yet equivalent across platforms:

- Automatic Temurin downloads currently support Windows x64; other platforms can select a system Java installation.
- Secure account-token storage currently uses Windows DPAPI. Microsoft and third-party account persistence on macOS/Linux is not yet supported; tokens are not stored in plaintext as a fallback.

## Build from source

Use the Go version in [`go.mod`](go.mod), currently **Go 1.27.3**, GNU Make, and Bash. Checks and signed releases also require OpenSSL 3 and standard archive tools (`tar`, `gzip`, and `zip`; Windows uses its built-in `tar` for ZIP). On Windows, run Make from Git Bash and install GNU Make if needed.

```sh
make build                      # development executable in .cache/
make check                      # formatting, dependency boundaries, vet, staticcheck, tests
make check CHECK_ARGS="--cross" # also vet/build all six OS/architecture targets
make fmt                        # format code and check import boundaries
make build BUILD_ARGS="--goos linux --goarch arm64 --output build/YoloMCL-linux-arm64"
```

`make build` writes `.cache/MCL-dev.exe` on Windows and `.cache/MCL-dev` on macOS/Linux. `--release` disables the MyGo inspector and requires publisher configuration; a modified or untagged checkout still reports `dev`. Use `make build BUILD_ARGS="--release --output build/YoloMCL.exe"` for a Windows release binary.

Publisher configuration is read from the environment or a local, ignored `.env`:

```dotenv
CURSEFORGE_API_KEY=
MS_CLIENT_ID=
```

Use your own CurseForge API key and Microsoft application client ID for forks. Never commit `.env` or real publisher credentials. Actions receives these values through repository Secrets, not workflow literals. A Microsoft client ID is public application identity; an API key embedded in a distributed client can still be extracted. The launcher calls CurseForge directly.

## Tagged releases

[Release](.github/workflows/release.yml) runs on `v*` tags. It checks the code on Windows, macOS, and Linux, builds amd64/arm64 packages, signs platform update archives, verifies all six sets, and publishes a complete GitHub Release. Failed checks or builds leave the release unpublished. The tag must be a valid version such as `v1.0.0` and point to a clean commit.

See [`docs/releases.md`](docs/releases.md) for secret setup, signing-key backup, and publishing.

## Project layout

- `main.go`: the launcher's single executable entry point.
- `internal/app`: command-line options, application assembly, startup coordination, and explicitly requested diagnostic reports.
- `internal/nativeui`: native UI views, interaction state, and view helpers.
- `internal/i18n`: shared translations loaded from bundled JSON catalogs; see [Translation guide](docs/i18n.md).
- `internal/launcher`: Minecraft services, instances, accounts, and background jobs, with isolated platform packages `memory`, `proctree`, `textenc`, and `vault`.
- `internal/config`: publisher service configuration, identity, and credential loading.
- `internal/buildinfo`: version, commit, repository, and update verification key.
- `Makefile` → `scripts/*.sh`: build, checks, signed releases, and publisher setup.

See [Architecture](docs/architecture.md) for package boundaries and [Development guidelines](docs/CONTRIBUTING.md) for coding standards. Formatting and import ordering are enforced by `gofmt`/`goimports` and `.editorconfig`.

## License

[MIT](LICENSE) © 2026 Viking602. Dependency and asset licenses are listed in [docs/THIRD_PARTY_NOTICES.md](docs/THIRD_PARTY_NOTICES.md). YoloMCL is not affiliated with Mojang or Microsoft.
