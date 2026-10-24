#!/usr/bin/env bash
source "$(dirname "$0")/lib.sh"
source "$mcl_root/scripts/release-lib.sh"
need "${OPENSSL:-openssl}"
umask 077
new_work
fixture="$mcl_work/repo"
mkdir -p "$fixture"
unset MS_CLIENT_ID MCL_MICROSOFT_CLIENT_ID CURSEFORGE_API_KEY MCL_CURSEFORGE_API_KEY
printf 'CURSEFORGE_API_KEY=outside-repo\n' > "$mcl_work/.env"
load_publisher "$fixture"
[[ -z "$mcl_cf_key" ]] || fail 'Configuration leaked from the parent directory.'
cat > "$fixture/.env" <<'EOF'
MS_CLIENT_ID=12345678-1234-1234-1234-123456789abc
CURSEFORGE_API_KEY='$literal`$(no-exec)' # comment
EOF
load_publisher "$fixture"
[[ "$mcl_cf_key" == '$literal`$(no-exec)' ]] || fail '.env content was not preserved literally.'
export MCL_CURSEFORGE_API_KEY=environment-key
load_publisher "$fixture"
[[ "$mcl_cf_key" == environment-key ]] || fail 'Environment did not override .env.'
unset MCL_CURSEFORGE_API_KEY
mcl_cf_key=private-test-key
encode_cf_key
[[ "$mcl_obfuscated" == 1d11055b02124814125612434e5a0054 ]] || fail 'CurseForge key encoding changed.'
for tag in v1.2.3 v1.2.3-rc.1; do valid_tag "$tag" || fail 'Valid tag rejected.'; done
for tag in v1.2..3 v1.2.3-01 'v1.2.3 -X injected=true'; do
  if valid_tag "$tag"; then fail 'Invalid tag accepted.'; fi
done
printf 'CURSEFORGE_API_KEY="hidden" invalid-tail\n' > "$fixture/.env"
if (load_publisher "$fixture") >/dev/null 2>&1; then fail 'Malformed .env accepted.'; fi
# A temporary repository exercises real clean/tagged/dirty build identities.
git -C "$fixture" init -q
printf fixture > "$fixture/file"
git -C "$fixture" -c core.autocrlf=false add .
git -C "$fixture" -c user.name=Test -c user.email=test@example.invalid commit -qm fixture
git -C "$fixture" tag v1.2.3
(cd "$fixture"; build_identity v1.2.3; [[ "$mcl_version" == v1.2.3 && "$mcl_dirty" == false ]])
printf changed > "$fixture/file"
if (cd "$fixture"; build_identity v1.2.3) >/dev/null 2>&1; then fail 'Dirty release accepted.'; fi
# Exercise the real build entry point with a recording Go command, without
# compiling six binaries just to inspect target flags and secret encoding.
mkdir -p "$mcl_work/bin"
cat > "$mcl_work/bin/go" <<'EOF'
#!/usr/bin/env bash
case "$1 ${2:-}" in
  'env GOOS') printf 'windows\n' ;;
  'env GOARCH') printf 'amd64\n' ;;
  'env GOFLAGS') printf '%s\n' "${GOFLAGS:-}" ;;
  build\ *) printf '%s\n' "$@" > "$MCL_TEST_ARGS" ;;
  *) exit 1 ;;
esac
EOF
chmod 755 "$mcl_work/bin/go"
export MCL_TEST_ARGS="$mcl_work/build-args"
for os_name in windows darwin linux; do
  for production in false true; do
    flags=()
    [[ "$production" == false ]] || flags=(--release)
    PATH="$mcl_work/bin:$PATH" MS_CLIENT_ID=12345678-1234-1234-1234-123456789abc \
      CURSEFORGE_API_KEY=private-test-key GOFLAGS=-tags=fixture \
      bash scripts/build.sh --goos "$os_name" --goarch arm64 --output "$mcl_work/launcher" "${flags[@]}" >/dev/null
    if grep -q private-test-key "$MCL_TEST_ARGS"; then fail 'Raw API key leaked into compiler arguments.'; fi
    expected_config=$(printf '{"microsoftClientId":"12345678-1234-1234-1234-123456789abc"}' | base64_line)
    grep -Fq -- "-X github.com/Viking602/YoloMCL/internal/config.encoded=$expected_config" "$MCL_TEST_ARGS" || fail 'Publisher configuration linker target is incorrect.'
    grep -Fq -- '-X github.com/Viking602/YoloMCL/internal/config.cfKeyObf=1d11055b02124814125612434e5a0054' "$MCL_TEST_ARGS" || fail 'CurseForge key linker target is incorrect.'
    if [[ "$os_name" == windows ]]; then grep -q -- '-H=windowsgui' "$MCL_TEST_ARGS"; else
      if grep -q -- '-H=windowsgui' "$MCL_TEST_ARGS"; then fail 'Windows flags leaked into another target.'; fi
    fi
    if [[ "$production" == true ]]; then
      grep -q -- 'mygo_noinspector,fixture' "$MCL_TEST_ARGS"
      grep -q -- 'production=1' "$MCL_TEST_ARGS"
    elif grep -q -- 'production=1' "$MCL_TEST_ARGS"; then fail 'Development build enabled production mode.'; fi
  done
done
# Check an actual forbidden dependency, including sources for other platforms.
mkdir -p "$fixture/internal/launcher"
printf 'package launcher\nimport "github.com/egoist/mygo/ui"\n' > "$fixture/internal/launcher/check.go"
if (cd "$fixture"; awk -v module=github.com/Viking602/YoloMCL -f "$mcl_root/scripts/check-imports.awk" internal/launcher/check.go) >/dev/null 2>&1; then fail 'Backend UI dependency accepted.'; fi
printf 'package launcher\nimport "github.com/Viking602/YoloMCL/internal/config"\n' > "$fixture/internal/launcher/check.go"
(cd "$fixture"; awk -v module=github.com/Viking602/YoloMCL -f "$mcl_root/scripts/check-imports.awk" internal/launcher/check.go)
# Comments and embedded source examples are not imports of the enclosing file.
cat > "$fixture/internal/launcher/check.go" <<'EOF'
/*
import "github.com/egoist/mygo/ui"
*/
package launcher

import "github.com/Viking602/YoloMCL/internal/config"

const example = `
import (
  "github.com/egoist/mygo/ui"
)
`
EOF
(cd "$fixture"; awk -v module=github.com/Viking602/YoloMCL -f "$mcl_root/scripts/check-imports.awk" internal/launcher/check.go)
printf 'package launcher\nimport "github.com/egoist/mygo-other"\n' > "$fixture/internal/launcher/check.go"
(cd "$fixture"; awk -v module=github.com/Viking602/YoloMCL -f "$mcl_root/scripts/check-imports.awk" internal/launcher/check.go)
stage="$mcl_work/stage" output="$mcl_work/output"
mkdir -p "$stage/YoloMCL.app/Contents/MacOS"
for name in YoloMCL YoloMCL.exe LICENSE THIRD_PARTY_NOTICES.md .env; do printf fixture > "$stage/$name"; done
printf fixture > "$stage/YoloMCL.app/Contents/MacOS/YoloMCL"
chmod 755 "$stage/YoloMCL.app/Contents/MacOS/YoloMCL"
ossl genpkey -algorithm ED25519 -outform DER -out "$mcl_work/private.der"
ossl pkey -inform DER -in "$mcl_work/private.der" -pubout -outform DER > "$mcl_work/public.der"
commit=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
for os_name in windows darwin linux; do
  for arch in amd64 arm64; do
    package_release "$stage" "$output" v1.0.0 "$commit" "$os_name-$arch" 2023-11-14T22:13:20Z "$mcl_work" example/launcher
    verify_target "$output" v1.0.0 "$commit" "$os_name-$arch" "$mcl_work" example/launcher
  done
done
entries=$(tar -tzf "$output/YoloMCL-1.0.0-linux-arm64.tar.gz")
[[ "$entries" == $'YoloMCL\nLICENSE\nTHIRD_PARTY_NOTICES.md' ]] || fail 'Archive contains unexpected files.'
if (verify_target "$output" v1.0.0 bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb linux-arm64 "$mcl_work" example/launcher) >/dev/null 2>&1; then fail 'Wrong commit accepted.'; fi
printf tampered >> "$output/YoloMCL-1.0.0-linux-arm64.tar.gz"
if (verify_target "$output" v1.0.0 "$commit" linux-arm64 "$mcl_work" example/launcher) >/dev/null 2>&1; then fail 'Tampered archive accepted.'; fi
printf 'Shell regression checks passed: configuration, version gates, six-platform archives and signatures.\n'
