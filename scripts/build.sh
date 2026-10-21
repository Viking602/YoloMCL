#!/usr/bin/env bash
# Build MCL with the CurseForge API key embedded (obfuscated). Never echoes the key.
# Usage: scripts/build.sh [output]   (honours GOOS/GOARCH; default .cache/MCL-dev[.exe])
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
goos="${GOOS:-$(go env GOOS)}"
ext=""; [ "$goos" = windows ] && ext=".exe"
out="${1:-.cache/MCL-dev$ext}"
key="${CURSEFORGE_API_KEY:-}"
if [ -z "$key" ] && [ -f .env ]; then
  key="$(sed -nE 's/^[[:space:]]*(export[[:space:]]+)?CURSEFORGE_API_KEY[[:space:]]*=[[:space:]]*//p' .env | tail -n1 | tr -d '\r' | sed -E "s/^[\"']//; s/[\"']\$//")"
fi
obf=""
if [ -n "$key" ]; then
  # mask must match internal/distribution/cfkey.go
  obf="$(MCL_K="$key" python3 -c 'import os;k=os.environ["MCL_K"].strip().encode();m=b"mcl-cf-9f3a7c1e-launcher";print(bytes(c^m[i%len(m)] for i,c in enumerate(k)).hex())')"
  echo "CurseForge key: embedded (obfuscated)"
else
  echo "CurseForge key: not found (CF features need env/.env at runtime)"
fi
flags="-s -w -X mcl/internal/distribution.cfKeyObf=$obf"
[ "$goos" = windows ] && flags="$flags -H=windowsgui"
mkdir -p "$(dirname "$out")"
go build -trimpath -ldflags "$flags" -o "$out" .
echo "Native launcher: $out"
