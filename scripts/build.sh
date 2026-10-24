#!/usr/bin/env bash
# Compile the launcher directly, using only Bash and the Go toolchain.
source "$(dirname "$0")/lib.sh"
need go
need git
need base64
release=false
output=
tag=
goos=${GOOS:-$(go env GOOS)}
goarch=${GOARCH:-$(go env GOARCH)}
while [[ $# -gt 0 ]]; do
  case "$1" in
    --release) release=true; shift ;;
    --output) output=${2:?missing output path}; shift 2 ;;
    --goos) goos=${2:?missing operating system}; shift 2 ;;
    --goarch) goarch=${2:?missing architecture}; shift 2 ;;
    --tag) tag=${2:?missing tag}; shift 2 ;;
    -h|--help) printf 'Usage: build.sh [--release] [--output file] [--goos os] [--goarch arch]\n'; exit 0 ;;
    -*) fail "Unknown option: $1" ;;
    *) [[ -z "$output" ]] || fail 'Only one output path is accepted.'; output=$1; shift ;;
  esac
done
case "$goos/$goarch" in
  windows/amd64|windows/arm64|linux/amd64|linux/arm64|darwin/amd64|darwin/arm64) ;;
  *) fail 'Supported targets are Windows/macOS/Linux on amd64/arm64.' ;;
esac
if [[ -z "$output" ]]; then
  output=.cache/MCL-dev
  [[ "$goos" != windows ]] || output+=.exe
fi
load_publisher "$mcl_root"
if [[ "$release" == true ]]; then
  [[ -n "$mcl_client" && -n "$mcl_cf_key" ]] || fail 'Release requires MS_CLIENT_ID and CURSEFORGE_API_KEY.'
fi
build_identity "$tag"
encode_cf_key
encoded=$(printf '{"microsoftClientId":"%s"}' "$mcl_client" | base64_line)
pkg=github.com/Viking602/YoloMCL/internal
flags="-s -w -X $pkg/config.encoded=$encoded -X $pkg/config.cfKeyObf=$mcl_obfuscated"
flags+=" -X $pkg/buildinfo.version=$mcl_version -X $pkg/buildinfo.commit=$mcl_commit -X $pkg/buildinfo.dirty=$mcl_dirty"
[[ "$goos" != windows ]] || flags+=' -H=windowsgui'
args=(-trimpath)
if [[ "$release" == true ]]; then
  # GOFLAGS tags remain on the command line; a separate -tags must preserve them.
  tags=mygo_noinspector
  goflags=$(go env GOFLAGS)
  while IFS= read -r extra; do [[ -z "$extra" ]] || tags+=",$extra"; done < <(
    printf '%s\n' "$goflags" | sed -nE "s/.*--?tags=[\"']?([^\"'[:space:]]+).*/\1/p")
  args+=("-tags=$tags")
  flags+=' -X github.com/egoist/mygo.production=1'
  if [[ "$mcl_version" != dev ]]; then
    public=$(tr -d '[:space:]' < internal/buildinfo/update-public-key.txt)
    [[ "$public" =~ ^[A-Za-z0-9+/]{43}=$ ]] || fail 'A valid Ed25519 public key is required for online updates.'
    repository
    feed="https://github.com/$mcl_repository/releases/latest/download/update-$goos-$goarch.json"
    flags+=" -X github.com/egoist/mygo.packageUpdateFeed=$feed -X github.com/egoist/mygo.packageUpdateKey=$public"
  fi
fi
mkdir -p "$(dirname "$output")"
GOOS="$goos" GOARCH="$goarch" CGO_ENABLED="${CGO_ENABLED:-0}" \
  go build "${args[@]}" -ldflags "$flags" -o "$output" .
printf 'Built launcher: %s (%s/%s)\n' "$output" "$goos" "$goarch"
