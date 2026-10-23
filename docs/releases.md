# Publishing YoloMCL

Releases are built from existing Git tags, not a manually edited version string. Use `v1.0.0`, `v1.0.1`, or a prerelease such as `v1.1.0-rc.1`. Never move a published tag to different source code.

## Publisher setup

1. Authenticate the GitHub CLI (`gh auth login`) with repository and workflow access.
2. Put your own `CURSEFORGE_API_KEY` and `MS_CLIENT_ID` in the ignored repository `.env`, or provide them as environment variables.
3. Run `go run ./cmd/publish-secrets` from the repository root. It configures **CURSEFORGE_API_KEY**, **MS_CLIENT_ID**, and **MYGO_UPDATER_PRIVATE_KEY** in this repository's Actions Secrets. GitHub CLI encrypts the values locally; the helper supplies them through stdin and prints only secret names.
4. Commit `internal/buildinfo/update-public-key.txt`. This public key belongs in the launcher. The private key does not.

The helper creates an Ed25519 signing key once and backs it up outside the repository under the OS configuration directory:

- Windows: `%APPDATA%/YoloMCL/publisher/update-private-key.txt`
- macOS: `~/Library/Application Support/YoloMCL/publisher/update-private-key.txt`
- Linux: `$XDG_CONFIG_HOME/YoloMCL/publisher/update-private-key.txt` (usually `~/.config/...`)

Keep a private backup. Later runs reuse this key and refuse to replace a committed public key with an unrelated one. To publish from a different machine, restore the same private-key backup first. Losing or replacing the signing key prevents existing installations from accepting new updates.

Forks must change the repository constants in `internal/buildinfo`, generate their own update-key pair, and configure their own publisher credentials. Do not replace the upstream public key for an upstream release.

Secrets are scoped to the build/sign step. Pull-request checks have no publisher secrets, the workflow does not write `.env`, and artifacts include only explicit launcher files, licenses, manifests, and checksums. The release private key is never embedded. The client-side CurseForge key is obfuscated, not cryptographically protected from reverse engineering; use a server-side relay when that boundary matters.

## Publish a version

```sh
go run ./cmd/check -cross
git add <reviewed-files>
git commit -m "Prepare release"
git tag -a v1.0.0 -m "YoloMCL v1.0.0"
git push origin main
git push origin v1.0.0
```

The [Release workflow](../.github/workflows/release.yml) performs checks on all three operating systems and builds their amd64 and arm64 packages. After all jobs pass, the publish job verifies every signature/checksum and uploads every asset to a draft release before publishing it. It refuses to overwrite an already published release. A failed run can be rerun on the same tag; manual dispatch must also select that tag.

Each target publishes:

- `YoloMCL-<version>-<os>-<arch>.tar.gz`: the signed updater archive and Unix download.
- `update-<os>-<arch>.json`: version, commit, download URL, archive size, and Ed25519 signature of the archive's SHA-256 digest.
- `checksums-<os>-<arch>.txt`: checksums for the target's assets.
- Windows additionally publishes a ZIP with a `YoloMCL` folder for Explorer extraction.

macOS archives contain one `YoloMCL.app` bundle, ad-hoc signed on a macOS runner before the update archive is signed. This is not Apple notarization. Windows executables do not currently use an Authenticode certificate.

## Application update behavior

Official builds read their target's manifest through `https://github.com/Viking602/YoloMCL/releases/latest/download/update-<os>-<arch>.json`. GitHub's latest release excludes drafts and prereleases. Checks start after the first frame, once per launch; Settings also offers a manual check. No instance selection triggers a new check.

Installation uses MyGo's public updater API. It downloads the archive, verifies its signature, and swaps application files with rollback on replacement failure. Launcher settings and Minecraft files are not packaged or replaced. On success, the user can restart; running games, active jobs, and foreground operations defer the restart button. A renamed executable or unsupported/read-only installation gets the manual download link.

The updater verifies the archive before extraction. HTTPS protects manifest transport; the manifest's display text is not itself signed. Publish through the protected repository account, and retain control of the update signing key.

## Local packaging

On a clean checkout at the release tag, provide the same publisher variables and `MYGO_UPDATER_PRIVATE_KEY` through the process environment, then run:

```sh
go run ./cmd/release -tag v1.0.0 -goos windows -goarch amd64
go run ./cmd/release -verify -tag v1.0.0  # after collecting all six target outputs
```

macOS release packaging requires a macOS host for `codesign`. The shared build and archive code is in this repository; no MyGo module-cache files are edited.
