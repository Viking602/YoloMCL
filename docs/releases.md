# Publishing YoloMCL

Releases are built from existing Git tags, not a manually edited version string. Use `v1.0.0`, `v1.0.1`, or a prerelease such as `v1.1.0-rc.1`. Never move a published tag to different source code.

## Publisher setup

1. Authenticate the GitHub CLI (`gh auth login`) with repository and workflow access.
2. Put your own `CURSEFORGE_API_KEY` and `MS_CLIENT_ID` in the ignored repository `.env`, or provide them as environment variables.
3. Run `make publish-secrets` from the repository root, with Bash and OpenSSL 3 available. It configures **CURSEFORGE_API_KEY**, **MS_CLIENT_ID**, and **MYGO_UPDATER_PRIVATE_KEY** in this repository's Actions Secrets. GitHub CLI encrypts the values locally; the script supplies them through stdin.
4. Commit `internal/buildinfo/update-public-key.txt`. This public key belongs in the launcher. The private key does not.

The helper creates an Ed25519 signing key once and backs it up outside the repository under the OS configuration directory:

- Windows: `%APPDATA%/YoloMCL/publisher/update-private-key.txt`
- macOS: `~/Library/Application Support/YoloMCL/publisher/update-private-key.txt`
- Linux: `$XDG_CONFIG_HOME/YoloMCL/publisher/update-private-key.txt` (usually `~/.config/...`)

Keep a private backup. Later runs reuse this key and refuse to replace a committed public key with an unrelated one. To publish from a different machine, restore the same private-key backup first. Losing or replacing the signing key prevents existing installations from accepting new updates.

Forks must change the repository constants in `internal/buildinfo`, generate their own update-key pair, and configure their own publisher credentials. Do not replace the upstream public key for an upstream release.

Secrets are scoped to the build/sign step. Pull-request checks have no publisher secrets, the workflow does not write `.env`, and artifacts include only explicit launcher files, licenses, manifests, and checksums. The release private key is never embedded. The client-side CurseForge key is obfuscated, not cryptographically protected from reverse engineering.

## Publish a version

```sh
make check CHECK_ARGS="--cross"
git add <reviewed-files>
git commit -m "chore(release): prepare v1.1.0"
git tag -a v1.1.0 -m "YoloMCL v1.1.0"
git push origin main
git push origin v1.1.0
```

The [Release workflow](../.github/workflows/release.yml) performs checks on all three operating systems and builds their amd64 and arm64 packages. After all jobs pass, the publish job verifies every signature/checksum and uploads every asset to a draft release before publishing it. It refuses to overwrite an already published release. A failed run can be rerun on the same tag; manual dispatch must also select that tag.

Each target publishes:

- `YoloMCL-<version>-<os>-<arch>.tar.gz`: the signed updater archive and Unix download.
- `update-<os>-<arch>.json`: version, commit, download URL, archive size, the archive's Ed25519 signature, and a separately signed minimum supported version.
- `checksums-<os>-<arch>.txt`: checksums for the target's assets.
- Windows additionally publishes a ZIP with a `YoloMCL` folder for Explorer extraction.

macOS archives contain one `YoloMCL.app` bundle, ad-hoc signed on a macOS runner before the update archive is signed. This is not Apple notarization. Windows executables do not currently use an Authenticode certificate.

## Application update behavior

Official builds read their target's manifest through `https://github.com/Viking602/YoloMCL/releases/latest/download/update-<os>-<arch>.json`. GitHub's latest release excludes drafts and prereleases. Checks start after the first frame, once per launch; Settings also offers a manual check. No instance selection triggers a new check.

An available update opens a confirmation dialog. Cancel keeps the current version running and defers the prompt for that launch only; the next launch checks again. An optional prompt waits until an existing form closes. Downloading starts only after confirmation. Settings can reopen the update dialog.

Installation uses MyGo's public updater API. It downloads the archive, verifies its signature, and swaps application files with rollback on replacement failure. Launcher settings and Minecraft files are not packaged or replaced. On success, the user can restart; running games, active jobs, and foreground operations defer the restart button. A renamed executable or unsupported/read-only installation gets the manual download link and remains subject to the version policy.

## Require a security update

Edit [`scripts/update-policy.conf`](../scripts/update-policy.conf) before tagging a release:

```ini
minimum_version=1.0.2
```

Publishing `v1.0.2` with this setting retires all lower launcher versions that implement policy enforcement. They show a mandatory update dialog instead of the launcher interface. Users can update or exit; Escape, clicking outside the dialog, navigation and new game/install commands cannot dismiss the requirement. Failed checks or downloads offer retry. A successful installation stays blocked until the new executable starts. Already running games and tasks are not killed.

The initial value `0.0.0` leaves updates optional. The floor must be a stable semantic version without a `v` prefix and cannot exceed the release being published. Keep the security floor in subsequent releases. To intentionally revise it, publish a newer release with the new floor; do not edit an existing release or move its tag. The configuration is parsed literally, never sourced as shell code. Packaging rejects missing, duplicate, invalid or impossible settings, and release verification checks all six targets against the same configuration before publication.

Each manifest contains `minimumVersion` and `policySignature`. The existing Ed25519 release key signs the UTF-8 text `YoloMCL update policy v1`, repository, platform target, release version and minimum version, in that order, with a newline after every field. Clients verify this signature before enforcing the floor. Archive signatures remain separate; unsigned legacy feeds can offer ordinary updates but cannot retire a version. HTTPS still protects unsigned display metadata and transport.

The launcher saves only the latest verified policy in `update-policy.json` beside its settings (normally a few hundred bytes, bounded to 64 KiB on read). It verifies that cache before displaying the launcher on the next start, so a known retirement remains active offline. Older or same-version feed data cannot overwrite it, and clearing mod metadata does not remove it. Network failure without a previously received retirement leaves the launcher usable; there is no recurring background check. Development builds are excluded.

Policy enforcement is a client feature, not a remote kill switch for arbitrary executables. Releases shipped before this feature cannot be retroactively disabled, and offline clients cannot learn a policy they have never received. Local modification of an open-source executable or removal of its policy cache is outside this mechanism. Roll out a version containing enforcement before relying on it for future security retirements.

## Local packaging

On a clean checkout at the release tag, provide the same publisher variables and `MYGO_UPDATER_PRIVATE_KEY` through the process environment, then run:

```sh
make release TAG=v1.1.0 RELEASE_ARGS="--goos windows --goarch amd64"
make verify TAG=v1.1.0  # after collecting all six target outputs
```

Make delegates to `scripts/release.sh`; it calls `scripts/build.sh`, then prepares archives and manifests using Shell and standard archive tools. OpenSSL 3 performs Ed25519 signing. Existing 64-byte private-key backups and the embedded public key remain compatible. macOS release packaging requires a macOS host for `codesign`; no MyGo module-cache files are edited.
