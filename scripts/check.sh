#!/usr/bin/env bash
source "$(dirname "$0")/lib.sh"
need go
fix=false
tests=true
cross=false
format_only=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    --fix) fix=true; shift ;;
    --no-tests) tests=false; shift ;;
    --cross) cross=true; shift ;;
    --format-only) format_only=true; shift ;;
    -h|--help) printf 'Usage: check.sh [--fix] [--format-only] [--no-tests] [--cross]\n'; exit 0 ;;
    *) fail "Unknown option: $1" ;;
  esac
done
files=()
while IFS= read -r -d '' file; do files+=("$file"); done < <(
  find . -type d \( -name '.?*' -o -name build -o -name vendor -o -name node_modules -o -name testdata \) -prune -o -type f -name '*.go' -print0)
mode=-l
if [[ "$fix" == true ]]; then
  mode=-w
fi
for ((start=0; start<${#files[@]}; start+=40)); do
  output=$(go tool -modfile=tools/go.mod goimports -local github.com/Viking602/YoloMCL "$mode" "${files[@]:start:40}")
  [[ -z "$output" ]] || { printf '%s\n' "$output"; fail 'Formatting required; run make fmt.'; }
done
# Batch paths to remain below Windows command-line limits.
for ((start=0; start<${#files[@]}; start+=40)); do
  awk -v module=github.com/Viking602/YoloMCL -f scripts/check-imports.awk "${files[@]:start:40}"
done
printf 'Formatting and architecture: %s Go files checked\n' "${#files[@]}"
if [[ "$format_only" == true ]]; then exit 0; fi
for module in . tools; do
  diff=$(go -C "$module" mod tidy -diff)
  [[ -z "$diff" ]] || { printf '%s\n' "$diff"; fail "Module $module requires go mod tidy."; }
  go -C "$module" mod verify
done
go vet ./...
go tool -modfile=tools/go.mod staticcheck ./...
go tool -modfile=tools/go.mod staticcheck -tags=mygo_noinspector ./...
if [[ "$tests" == true ]]; then
  bash scripts/test.sh
  go test ./...
  go test -tags=mygo_noinspector ./...
fi
if [[ "$cross" == true ]]; then
  for goos in windows darwin linux; do
    for arch in amd64 arm64; do
      printf 'Cross-platform: %s/%s\n' "$goos" "$arch"
      GOOS="$goos" GOARCH="$arch" CGO_ENABLED=0 go vet ./...
      GOOS="$goos" GOARCH="$arch" CGO_ENABLED=0 go build ./...
      GOOS="$goos" GOARCH="$arch" CGO_ENABLED=0 go build -tags=mygo_noinspector ./...
    done
  done
fi
printf 'All requested checks passed.\n'
