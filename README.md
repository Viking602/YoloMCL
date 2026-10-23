<p align="center"><img src="docs/logo.png" width="128" alt="YoloMCL"></p>

# YoloMCL

A native Minecraft: Java Edition launcher written in Go, powered by [MyGo](https://github.com/egoist/mygo). No WebView, HTML frontend, or local web server.

[Download](https://github.com/Viking602/YoloMCL/releases/latest) · [中文介绍](README.zh-CN.md) · [Contributing](CONTRIBUTING.md) · [Release guide](docs/releases.md)

## Features

- Manage vanilla, Fabric, Quilt, Forge, and NeoForge instances and modpacks.
- Browse Modrinth and CurseForge with continuous loading; install mods, resource packs, shaders, and modpacks.
- Import `.mrpack` and CurseForge `.zip` files, including drag and drop. Review pack updates and keep backups of overwritten files.
- Manage worlds, backups, screenshots, recent multiplayer servers, and structured game/crash logs.
- Detect installed Java in the background after the first frame. Cache detection across instance switches and allocate memory based on pack size and available RAM.
- Use offline accounts, Microsoft sign-in, or Yggdrasil/authlib-injector services, subject to the platform limits below.
- Preview skins and capes. English and Simplified Chinese follow the system language by default.
- Check for launcher updates through GitHub Releases. Downloads are verified with an embedded Ed25519 public key before installation; restarting remains an explicit action.

Minecraft content scans stay within registered Minecraft directories and launcher configuration. Java detection separately checks system Java installations, environment paths, registered locations, and known runtimes; it does not traverse entire disks.

## Downloads and updates

Release versions come from Git tags, starting with **v1.0.0**. Settings shows the version, source commit, and repository links.

| Platform | Architecture | Download |
| --- | --- | --- |
| Windows | x64 / ARM64 | `YoloMCL-<version>-windows-<arch>.zip` |
| macOS | Intel / Apple Silicon | `YoloMCL-<version>-darwin-<arch>.tar.gz` containing `YoloMCL.app` |
| Linux | x64 / ARM64 | `YoloMCL-<version>-linux-<arch>.tar.gz` |

Extract the complete archive before launching. Keep the `YoloMCL.exe` / `YoloMCL` filename and install in a directory you can write to. On macOS, move the entire `.app` bundle to a writable Applications directory. Linux requires GTK 3, Pango, Cairo, and Fontconfig; install a CJK font for Chinese text.

In **Settings**, use **Check for updates → Download update → Restart launcher**. The automatic check runs once after the first frame and follows stable releases. Development builds, renamed executables, and read-only installations provide a GitHub download link instead of in-place updates. Update signing is separate from operating-system trust: Windows packages currently have no Authenticode certificate, and macOS bundles are ad-hoc signed, not notarized.

### Current platform limits

All six targets are built by the release workflow. Some existing features are not yet equivalent across platforms:

- Automatic Temurin downloads currently support Windows x64; other platforms can select a system Java installation.
- Secure account-token storage currently uses Windows DPAPI. Microsoft and third-party account persistence on macOS/Linux is not yet supported; tokens are not stored in plaintext as a fallback.

## Build from source

Use the Go version in [go.mod](go.mod), currently **Go 1.27.3**. No frontend toolchain is required.

```sh
go run .                       # development build
go run ./cmd/check             # formatting, dependency boundaries, vet, staticcheck, tests
go run ./cmd/check -cross       # also vet/build all six OS/architecture targets
go run ./cmd/build -output build/YoloMCL
```

On Windows, use `-output build/YoloMCL.exe`. The wrappers `scripts/build.ps1` and `scripts/build.sh` call the same Go build command. Use `-goos` and `-goarch` for cross-compilation. `-release` removes the MyGo inspector and requires publisher configuration; a modified or untagged checkout still identifies itself as `dev`.

Publisher configuration is read from the environment or a local, ignored `.env`:

```dotenv
CURSEFORGE_API_KEY=
MS_CLIENT_ID=
```

Use your own CurseForge API key and Microsoft application client ID for forks. Never commit `.env` or publisher JSON. Actions receives these values through repository Secrets, not workflow literals. A Microsoft client ID is public application identity; an API key embedded in a distributed client can still be extracted. For a server-held CurseForge key, use the optional [catalog relay](cmd/catalog-relay).

## Tagged releases

[Release](.github/workflows/release.yml) runs on `v*` tags. It checks the code on Windows, macOS, and Linux, builds amd64/arm64 packages, signs platform update archives, verifies all six sets, and publishes a complete GitHub Release. Failed checks or builds leave the release unpublished. The tag must be a valid version such as `v1.0.0` and point to a clean commit.

See [docs/releases.md](docs/releases.md) for secret setup, signing-key backup, and publishing. The updater uses MyGo's public API; no MyGo source files are patched.

## Project layout

- `main.go`: native application entry point and startup coordination.
- `internal/nativeui`: UI, interaction, and view state.
- `internal/launcher`: Minecraft services, files, accounts, and background jobs.
- `internal/buildinfo`: version, commit, repository, and update verification key.
- `cmd/build`, `cmd/release`, `cmd/check`: shared build, release, and validation tools.
- `plugins/startup`: repository-owned MyGo startup integration.

See [architecture](docs/architecture.md) for package boundaries. Formatting and import ordering are enforced by `gofmt`/`goimports` and `.editorconfig`, suitable for GoLand and other Go editors.

## License

[MIT](LICENSE) © 2026 Viking602. Dependency and asset licenses are listed in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). YoloMCL is not affiliated with Mojang or Microsoft.
