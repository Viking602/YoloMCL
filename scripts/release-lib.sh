#!/usr/bin/env bash
# Archive and signature operations shared by release.sh and its regression tests.
ossl() { "${OPENSSL:-openssl}" "$@"; }
file_size() { wc -c < "$1" | tr -d '[:space:]'; }

public_der() {
  local public_file=$1 work=$2
  tr -d '[:space:]' < "$public_file" | ossl base64 -d -A > "$work/public.raw"
  [[ $(file_size "$work/public.raw") == 32 ]] || fail 'Invalid Ed25519 public key.'
  printf '\x30\x2a\x30\x05\x06\x03\x2b\x65\x70\x03\x21\x00' > "$work/public.der"
  cat "$work/public.raw" >> "$work/public.der"
}

private_der() {
  local work=$1
  printf '%s' "${MYGO_UPDATER_PRIVATE_KEY:-}" | tr -d '[:space:]' | ossl base64 -d -A > "$work/private.raw"
  [[ $(file_size "$work/private.raw") == 64 ]] || fail 'MYGO_UPDATER_PRIVATE_KEY must contain a 64-byte Ed25519 private key.'
  printf '\x30\x2e\x02\x01\x00\x30\x05\x06\x03\x2b\x65\x70\x04\x22\x04\x20' > "$work/private.der"
  head -c 32 "$work/private.raw" >> "$work/private.der"
  ossl pkey -inform DER -in "$work/private.der" -pubout -outform DER > "$work/derived-public.der"
  tail -c 32 "$work/derived-public.der" > "$work/derived-public.raw"
  tail -c 32 "$work/private.raw" > "$work/saved-public.raw"
  cmp -s "$work/derived-public.raw" "$work/saved-public.raw" || fail 'Invalid private-key backup.'
}

signing_key() {
  local public_file=$1 work=$2
  private_der "$work"
  public_der "$public_file" "$work"
  cmp -s "$work/derived-public.raw" "$work/public.raw" || fail "Signing key does not match the launcher's public key."
}

checksum_list() {
  local directory=$1 name digest
  shift
  printf '%s\n' "$@" | LC_ALL=C sort | while IFS= read -r name; do
    digest=$(ossl dgst -sha256 "$directory/$name")
    printf '%s  %s\n' "${digest##* }" "$name"
  done
}

package_release() {
  local stage=$1 output=$2 tag=$3 commit=$4 target=$5 date=$6 work=$7 repo=$8
  local exe=YoloMCL base="YoloMCL-${tag#v}-$target" name size signed zipdir system_tar
  local tar_options=(--format=ustar) exe_options=() data_options=() names=()
  valid_tag "$tag" || fail 'Invalid release tag.'
  case "$target" in windows-amd64|windows-arm64|linux-amd64|linux-arm64|darwin-amd64|darwin-arm64) ;; *) fail 'Invalid release target.' ;; esac
  [[ "$target" != windows-* ]] || exe+=.exe
  mkdir -p "$output"
  output="$(cd "$output" && pwd)"
  # GNU tar needs explicit modes when cross-packaging Unix targets on Windows.
  # Native Windows paths must not be interpreted as remote host:path archives.
  if tar --version | head -n 1 | grep -q GNU; then
    tar_options+=(--force-local --owner=0 --group=0 --numeric-owner "--mtime=$date")
    exe_options=(--mode=755)
    data_options=(--mode=644)
  fi
  if [[ "$target" == darwin-* ]]; then
    [[ -f "$stage/YoloMCL.app/Contents/MacOS/YoloMCL" ]] || fail 'Missing macOS bundle executable.'
    [[ -z $(find "$stage/YoloMCL.app" -type l -print) ]] || fail 'Release bundle contains symlinks.'
    tar "${tar_options[@]}" -cf "$work/contents.tar" -C "$stage" YoloMCL.app
  else
    for name in "$exe" LICENSE THIRD_PARTY_NOTICES.md; do
      [[ -f "$stage/$name" && ! -L "$stage/$name" ]] || fail 'Release inputs must be regular files.'
    done
    chmod 755 "$stage/$exe"
    chmod 644 "$stage/LICENSE" "$stage/THIRD_PARTY_NOTICES.md"
    # Bash 3.2 treats empty arrays as unset with nounset enabled.
    tar "${tar_options[@]}" ${exe_options[@]+"${exe_options[@]}"} -cf "$work/contents.tar" -C "$stage" "$exe"
    tar "${tar_options[@]}" ${data_options[@]+"${data_options[@]}"} -rf "$work/contents.tar" -C "$stage" LICENSE THIRD_PARTY_NOTICES.md
  fi
  gzip -n -c "$work/contents.tar" > "$output/$base.tar.gz"
  ossl dgst -sha256 -binary "$output/$base.tar.gz" > "$work/digest"
  ossl pkeyutl -sign -rawin -keyform DER -inkey "$work/private.der" -in "$work/digest" -out "$work/signature"
  signed=$(base64_line < "$work/signature")
  size=$(file_size "$output/$base.tar.gz")
  # All string fields come from validated tag/repository/commit identifiers.
  cat > "$output/update-$target.json" <<EOF
{
  "version": "${tag#v}",
  "notes": "Release notes: https://github.com/$repo/releases/tag/$tag",
  "date": "$date",
  "url": "https://github.com/$repo/releases/download/$tag/$base.tar.gz",
  "size": $size,
  "signature": "$signed",
  "commit": "$commit"
}
EOF
  names=("$base.tar.gz" "update-$target.json")
  if [[ "$target" == windows-* ]]; then
    zipdir="$work/zip-$target"
    mkdir -p "$zipdir/YoloMCL"
    cp "$stage/$exe" "$stage/LICENSE" "$stage/THIRD_PARTY_NOTICES.md" "$zipdir/YoloMCL/"
    if command -v zip >/dev/null; then
      (cd "$zipdir" && zip -q -r "$work/$base.zip" YoloMCL)
    elif command -v cygpath >/dev/null; then
      system_tar="$(cygpath -u "${SYSTEMROOT:-${WINDIR:-C:/Windows}}")/System32/tar.exe"
      [[ -x "$system_tar" ]] || fail 'ZIP packaging requires zip or Windows system tar.'
      "$system_tar" -a -cf "$work/$base.zip" -C "$zipdir" YoloMCL
    else
      fail 'Install zip to create Windows release archives.'
    fi
    mv -f -- "$work/$base.zip" "$output/$base.zip"
    names+=("$base.zip")
  fi
  checksum_list "$output" "${names[@]}" > "$output/checksums-$target.txt"
}

manifest_field() {
  local file=$1 key=$2 kind=${3:-string}
  if [[ "$kind" == number ]]; then
    mcl_field=$(sed -nE "s/^[[:space:]]*\"$key\":[[:space:]]*([0-9]+),?[[:space:]]*$/\1/p" "$file")
  else
    mcl_field=$(sed -nE "s/^[[:space:]]*\"$key\":[[:space:]]*\"([^\"]*)\",?[[:space:]]*$/\1/p" "$file")
  fi
  [[ -n "$mcl_field" && "$mcl_field" != *$'\n'* ]] || fail "Missing or duplicate manifest field: $key"
}

verify_target() {
  local output=$1 tag=$2 commit=$3 target=$4 work=$5 repo=$6
  local base="YoloMCL-${tag#v}-$target" manifest="$output/update-$target.json" names=()
  manifest_field "$manifest" version
  [[ "$mcl_field" == "${tag#v}" ]] || fail "$target: wrong release version."
  manifest_field "$manifest" commit
  [[ "$mcl_field" == "$commit" ]] || fail "$target: wrong release commit."
  manifest_field "$manifest" url
  [[ "$mcl_field" == "https://github.com/$repo/releases/download/$tag/$base.tar.gz" ]] || fail "$target: wrong archive URL."
  manifest_field "$manifest" size number
  [[ "$mcl_field" == "$(file_size "$output/$base.tar.gz")" ]] || fail "$target: archive size mismatch."
  manifest_field "$manifest" signature
  [[ "$mcl_field" =~ ^[A-Za-z0-9+/]{86}==$ ]] || fail "$target: invalid signature encoding."
  printf '%s' "$mcl_field" | ossl base64 -d -A > "$work/signature"
  ossl dgst -sha256 -binary "$output/$base.tar.gz" > "$work/digest"
  ossl pkeyutl -verify -pubin -rawin -keyform DER -inkey "$work/public.der" -in "$work/digest" -sigfile "$work/signature" >/dev/null
  names=("$base.tar.gz" "update-$target.json")
  [[ "$target" != windows-* ]] || names+=("$base.zip")
  checksum_list "$output" "${names[@]}" > "$work/checksums"
  cmp -s "$work/checksums" "$output/checksums-$target.txt" || fail "$target: checksum mismatch."
}

mac_bundle() {
  local stage=$1 tag=$2 commit=$3 contents="$1/YoloMCL.app/Contents" version png size byte
  version=${tag#v}; version=${version%%-*}
  mkdir -p "$contents/MacOS" "$contents/Resources"
  mv "$stage/YoloMCL" "$contents/MacOS/YoloMCL"
  cat > "$contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleName</key><string>YoloMCL</string>
<key>CFBundleDisplayName</key><string>YoloMCL</string>
<key>CFBundleIdentifier</key><string>io.github.viking602.yolomcl</string>
<key>CFBundleExecutable</key><string>YoloMCL</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleShortVersionString</key><string>$version</string>
<key>CFBundleVersion</key><string>$version</string>
<key>CFBundleIconFile</key><string>app.icns</string>
<key>NSHighResolutionCapable</key><true/>
<key>LSMinimumSystemVersion</key><string>12.0</string>
<key>YoloMCLCommit</key><string>$commit</string>
</dict></plist>
EOF
  png="$mcl_root/internal/nativeui/assets/brand/brand-256.png"
  size=$(file_size "$png")
  { printf icns; uint32 "$((size + 16))"; printf ic08; uint32 "$((size + 8))"; cat "$png"; } > "$contents/Resources/app.icns"
  cp LICENSE docs/THIRD_PARTY_NOTICES.md "$contents/Resources/"
  find "$stage/YoloMCL.app" -type d -exec chmod 755 {} +
  find "$stage/YoloMCL.app" -type f -exec chmod 644 {} +
  chmod 755 "$contents/MacOS/YoloMCL"
  (umask 022; codesign --force --sign - --timestamp=none "$stage/YoloMCL.app")
  codesign --verify --deep --strict "$stage/YoloMCL.app"
}

uint32() {
  local shift octal
  for shift in 24 16 8 0; do
    printf -v octal '\\%03o' "$((($1 >> shift) & 255))"
    printf '%b' "$octal"
  done
}
