#!/usr/bin/env bash
source "$(dirname "$0")/lib.sh"
source "$mcl_root/scripts/release-lib.sh"
need go
need git
need "${OPENSSL:-openssl}"
umask 077
tag=${TAG:-}
goos=${TARGET_OS:-${GOOS:-$(go env GOOS)}}
goarch=${TARGET_ARCH:-${GOARCH:-$(go env GOARCH)}}
output=build/release
verify=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    --tag) tag=${2:?missing tag}; shift 2 ;;
    --goos) goos=${2:?missing operating system}; shift 2 ;;
    --goarch) goarch=${2:?missing architecture}; shift 2 ;;
    --output) output=${2:?missing output directory}; shift 2 ;;
    --verify) verify=true; shift ;;
    -h|--help) printf 'Usage: release.sh --tag vX.Y.Z [--goos os] [--goarch arch] [--output dir] [--verify]\n'; exit 0 ;;
    *) fail "Unknown option: $1" ;;
  esac
done
[[ -n "$tag" ]] || fail 'Set TAG=vX.Y.Z or pass --tag.'
build_identity "$tag"
repository
load_update_policy "$tag"
new_work
if [[ "$verify" == true ]]; then
  public_der internal/buildinfo/update-public-key.txt "$mcl_work"
  for os_name in windows darwin linux; do
    for arch in amd64 arm64; do
      verify_target "$output" "$tag" "$mcl_commit" "$os_name-$arch" "$mcl_work" "$mcl_repository"
    done
  done
  printf 'All six release archives, signatures and checksums verified.\n'
  exit 0
fi
case "$goos/$goarch" in
  windows/amd64|windows/arm64|linux/amd64|linux/arm64|darwin/amd64|darwin/arm64) ;;
  *) fail 'Unsupported release target.' ;;
esac
if [[ "$goos" == darwin ]]; then
  [[ $(go env GOHOSTOS) == darwin ]] || fail 'macOS bundles require a macOS host for codesign.'
fi
signing_key internal/buildinfo/update-public-key.txt "$mcl_work"
stage="$mcl_work/stage"
mkdir -p "$stage"
exe=YoloMCL
[[ "$goos" != windows ]] || exe+=.exe
bash scripts/build.sh --release --tag "$tag" --goos "$goos" --goarch "$goarch" --output "$stage/$exe"
cp LICENSE docs/THIRD_PARTY_NOTICES.md "$stage/"
if [[ "$goos" == darwin ]]; then mac_bundle "$stage" "$tag" "$mcl_commit"; fi
package_release "$stage" "$output" "$tag" "$mcl_commit" "$goos-$goarch" "$(git show -s --format=%cI HEAD)" "$mcl_work" "$mcl_repository"
printf 'Built and signed %s %s/%s (%s)\n' "$tag" "$goos" "$goarch" "${mcl_commit:0:8}"
