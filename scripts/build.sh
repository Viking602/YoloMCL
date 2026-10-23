#!/usr/bin/env bash
# Usage: scripts/build.sh [--release] [--publisher-config file] [output]
# The Go build command owns all .env parsing, validation and linker settings.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
args=()
while [ "$#" -gt 0 ]; do
  case "$1" in
    --release) args+=(-release); shift ;;
    --publisher-config) args+=(-publisher-config "$(cd "$(dirname "${2:?missing publisher file}")" && pwd)/$(basename "$2")"); shift 2 ;;
    --output) args+=(-output "${2:?missing output path}"); shift 2 ;;
    --*) echo "Unknown option: $1" >&2; exit 1 ;;
    *) args+=(-output "$1"); shift ;;
  esac
done
cd "$root"
target_os="$(go env GOOS)"
target_arch="$(go env GOARCH)"
host_os="$(go env GOHOSTOS)"
host_arch="$(go env GOHOSTARCH)"
GOOS="$host_os" GOARCH="$host_arch" go run ./cmd/build -goos "$target_os" -goarch "$target_arch" "${args[@]}"
