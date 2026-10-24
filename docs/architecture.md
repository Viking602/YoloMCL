# Project Architecture

YoloMCL is a desktop launcher for Minecraft: Java Edition. A Go service manages game data and background work, while MyGo renders the native interface. The application runs as one executable.

## Repository layout

```text
main.go                  Application entry point
Makefile                 Build, check, packaging and release commands
internal/
  app/                   Process lifecycle and application assembly
  launcher/              Game instances, accounts, installation and background jobs
    memory/              Physical and available system memory
    proctree/            Game process trees and platform process attributes
    textenc/             Game log character decoding
    vault/               Account credential protection
  nativeui/              Native views, interaction state and presentation
    i18n/                English and Chinese text and category dictionaries
    assets/              Embedded fonts, icons and images
  config/                Publisher identity and content-provider configuration
  buildinfo/             Build identity and update verification metadata
plugins/startup/         MyGo first-frame coordination
tools/                   Pinned formatting and static-analysis dependencies
scripts/                 Bash build, check, signing and packaging implementations
docs/                    Architecture, development and release documentation
build/                   Release packages
.cache/                  Local build and test outputs
```

## Package dependencies

```text
main -> app
app -> launcher, nativeui, buildinfo, plugins/startup
nativeui -> launcher, buildinfo, nativeui/i18n
launcher -> config, launcher/{memory,proctree,textenc,vault}
```

`app`, `nativeui` and `plugins/startup` use MyGo. The launcher service is independent of the UI toolkit. The platform packages do not import their parent service or each other. `config`, `buildinfo` and `nativeui/i18n` use the standard library.

`scripts/check-imports.awk`, invoked by `make check` and `make fmt`, checks these import boundaries.

## Application lifecycle

`main.go` passes command-line arguments to `app.Run` and reports startup errors. `internal/app/run.go` selects desktop, screenshot or diagnostic mode.

Desktop startup is coordinated by `internal/app/desktop.go`:

1. Register the startup plugin and configure the application name and supported languages.
2. Wait for MyGo's native application initialization, then begin font and image preparation.
3. Construct the launcher service and UI state while the native window is being created.
4. Join application preparation before attaching the view, bind the system language, and render the initial page.

`plugins/startup` coordinates the first frame through MyGo's public plugin hooks. Background scans use the UI's first-frame signal before starting deferred work. Screenshot and diagnostic modes have separate entry paths in `internal/app`.

## Launcher service

`internal/launcher.Service` owns configuration, account credentials, game processes, background jobs, content-provider clients and instance caches. The UI calls service operations and reads snapshots of their results. `api.go` dispatches named UI actions to service methods using JSON payloads.

| Area | Implementation | Responsibility |
| --- | --- | --- |
| Service and configuration | `store.go`, `config.go`, `roots.go` | Service initialization, configuration persistence, registered game directories and instance selection |
| Instance inventory | `inventory.go`, `version_scan.go`, `instance_status.go` | Cached snapshots, local version metadata, overview information and storage measurement |
| Game runtime | `runtime_options.go`, `minecraft.go`, `game_launch.go`, `game_process.go` | Runtime configuration inheritance, game dependencies, launch arguments and process supervision |
| Java | `java_scan*.go`, `java_catalog.go`, `javaget.go`, `java_managed.go`, `automemory.go` | Runtime discovery, download catalogs, managed installations and memory allocation |
| Installed content | `mods.go`, `modscan.go`, `mod_metadata.go`, `mod_install.go`, `mod_updates.go`, `assetpacks.go` | Mod metadata, enabled state, installation, updates, resource packs and shaders |
| Resource discovery | `providers.go`, `deps.go` | Modrinth and CurseForge queries, project versions and dependency resolution |
| Modpacks | `packs.go`, `pack_format.go`, `pack_import.go`, `pack_check.go`, `packupdate*.go` | Import, manifests, installation checks, update plans, backups and rollback |
| Accounts | `auth.go`, `auth_microsoft.go`, `auth_yggdrasil.go`, `authservers.go` | Shared account lifecycle, offline profiles, Microsoft authorization, Yggdrasil authentication and authentication servers |
| Worlds | `worlds.go`, `world_nbt.go`, `world_backup.go` | World summaries, NBT metadata, backups and restoration |
| Sessions and logs | `sessions.go`, `playstats.go`, `logtail.go`, `loginsight.go` | Game sessions, play history, log reading and crash information |
| Instance media and preferences | `addonicons.go`, `game_options.go`, `cosmetics.go`, `serverstatus.go`, `recentserver.go` | Local icons, game options, skins, capes and multiplayer server information |
| Service helpers | `data_migrate.go`, `language.go`, `net_fault.go` | Data-directory migration, system-language normalization and per-client network fault simulation |

`Snapshot` combines current settings, accounts, jobs and running versions with cached instance inventories. Inventory refreshes watch known Minecraft metadata and are invalidated by operations that change the instance. The overview loads basic information separately from storage totals so directory walks do not delay its first result.

## Native interface

`internal/nativeui.App` owns the view tree and interaction state. `app_state.go` groups page state, request generations, caches and lock-protected fields. `selection.go` synchronizes instance selection with service snapshots.

| Area | Implementation | Responsibility |
| --- | --- | --- |
| Application shell | `app.go`, `shell.go`, `navbar.go`, `titlebar.go` | Window attachment, navigation and application lifecycle |
| Visual resources | `assets.go`, `images.go`, `theme.go`, `tokens.go` | Embedded and remote images, fonts, colors, surfaces, spacing and dimensions |
| Text and controls | `typography.go`, `display_text.go`, `page_widgets.go`, `buttons.go`, `select_widgets.go`, `vertags.go` | Text layout and formatting, shared controls and version badges |
| Instances | `overview*.go`, `instance*.go`, `addons_*.go`, `addon_*.go` | Instance navigation, overview, installed content and content actions |
| Discovery and installation | `discover*.go`, `project_*.go`, `install_*.go`, `game_install_dialog.go`, `import_dialog.go`, `drop.go` | Provider searches, version filtering, dependency previews and installation dialogs |
| Accounts and wardrobe | `account_*.go`, `authserver_dialog.go`, `wardrobe*.go`, `skin.go`, `avatar.go`, `skin_defaults.go` | Account actions, appearance loading and upload, model rendering and default skins |
| Settings and Java | `settings*.go`, `java_settings.go`, `java_installed.go`, `runtime_dialog.go`, `javaget_ui.go`, `about.go` | Settings drafts, runtime scanning and selection, Java downloads and build information |
| Worlds and modpack updates | `worlds_ui.go`, `packupdate_ui.go`, `packchanges_ui.go` | World management and modpack update review |
| Tasks and logs | `tasks.go`, `task_*.go`, `log_state.go`, `log_loading.go`, `logview.go`, `logpanels.go`, `log_layout.go`, `logsessions.go` | Task progress, log state, asynchronous reads and log navigation |
| Feedback and motion | `error_text.go`, `notices.go`, `empty_state.go`, `motion.go`, `content_motion.go`, `number_motion.go`, `scrollbar.go` | User-facing errors, notifications, empty states and shared transitions |

`nativeui/i18n` contains English and Chinese dictionaries and provider category translations. `log_parse.go` converts plain-text and Log4j XML output into displayable log entries. `skin_defaults.go` caches the embedded Steve and Alex textures.

## State and asynchronous work

Service configuration is protected by `Service.mu`. Mutating operations copy the configuration, validate the draft and write it atomically before publishing the new state. Returned configuration and version maps are copied rather than shared with callers.

UI state is updated on the UI thread. Workers carry a cancellation context and request identity, then return results through `App.post`. Request generations prevent results from a previous instance, account, project or version from replacing the current view. Instance selection reuses cached data; system Java discovery has its own scan lifecycle.

Background installation updates its target instance without changing the user's active selection. Modpack updates retain backups and restore files when application or persistence fails. Rollback failures retain the backup for another attempt.

Network operations use the service's HTTP clients. Resource requests support cancellation, timeouts, response limits and redirect checks. File parsers apply size and depth limits; content scans stay within registered Minecraft data and launcher-owned paths.

## Configuration and storage

- `internal/config` reads publisher-owned Microsoft and CurseForge configuration from build-injected values and supported environment overrides. `dotenv.go` parses the supported fields for runtime `.env` overrides; build scripts read the repository-root `.env` when injecting publisher configuration.
- Player preferences, accounts and registered instance configuration are persisted by `internal/launcher` in the launcher data directory. `MCL_DATA_DIR` can select that directory.
- Instance inventories and downloaded resources are cached under launcher-managed storage. Minecraft files and modpack metadata remain in their game directories.
- `internal/buildinfo` exposes the version, source commit, dirty-state flag, repository identity and embedded update public key.

## Platform services

| Package | Platform implementation |
| --- | --- |
| `launcher/memory` | Physical and available memory on Windows, macOS and Linux |
| `launcher/proctree` | Windows Job Objects and Unix process groups for game-process lifetime management |
| `launcher/textenc` | UTF-8, GB18030 and platform code-page decoding for game output |
| `launcher/vault` | Windows DPAPI for stored account tokens; other platforms return an unsupported-operation error |

Java auto-download currently targets Windows amd64. Platform-specific implementations and tests live in the corresponding package or OS-constrained files.

## Build and release pipeline

| Command | Implementation | Output |
| --- | --- | --- |
| `make build` | `scripts/build.sh` | Launcher executable with injected publisher and build metadata |
| `make fmt` | `scripts/check.sh` | Formatted source and import-boundary validation |
| `make check` | `scripts/check.sh`, `scripts/test.sh` | Formatting, dependency verification, static analysis, Shell regression checks and Go tests |
| `make release TAG=vX.Y.Z` | `scripts/release.sh`, `scripts/release-lib.sh` | Platform archives, signed update manifests and checksums |
| `make verify TAG=vX.Y.Z` | `scripts/release.sh` | Verification of all six platform artifact sets |

`tools/go.mod` and `tools/go.sum` pin `goimports` and `staticcheck` independently of launcher dependencies. `scripts/lib.sh` supplies literal configuration parsing and Git build identity; archive tools and OpenSSL handle packaging and signatures.

Release versions come from an existing Git tag on a clean checkout. The Release workflow runs checks on Windows, macOS and Linux, builds amd64 and arm64 packages, verifies every artifact set, and publishes the GitHub release. macOS bundles are ad-hoc signed on macOS before their update archives are signed.

The launcher reads the latest stable release's platform manifest and uses MyGo's updater to verify and install the archive. Release archives contain application files and licenses; player settings and Minecraft data stay in their existing storage locations.
