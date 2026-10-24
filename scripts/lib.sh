#!/usr/bin/env bash
# Shared environment and literal configuration for the repository's Bash scripts.
set +x
set -euo pipefail
shopt -s extglob
mcl_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$mcl_root"

fail() { printf '%s\n' "$*" >&2; exit 1; }
need() { command -v "$1" >/dev/null || fail "Required tool not found: $1"; }
trim() { mcl_value=${1##+([[:space:]])}; mcl_value=${mcl_value%%+([[:space:]])}; }
base64_line() { base64 | tr -d '\r\n'; }

load_publisher() {
  local root=$1 line key value quote tail number=0 file_ms= file_cf= file_ms_alias= file_cf_alias=
  if [[ -f "$root/.env" ]]; then
    while IFS= read -r line || [[ -n "$line" ]]; do
      number=$((number + 1))
      trim "${line#$'\xef\xbb\xbf'}"; line=$mcl_value
      if [[ "$line" == export[[:blank:]]* ]]; then trim "${line:6}"; line=$mcl_value; fi
      [[ "$line" == *=* ]] || continue
      trim "${line%%=*}"; key=$mcl_value
      case "$key" in MS_CLIENT_ID|MCL_MICROSOFT_CLIENT_ID|CURSEFORGE_API_KEY|MCL_CURSEFORGE_API_KEY) ;; *) continue ;; esac
      trim "${line#*=}"; value=$mcl_value
      case "$value" in
        \"*|\'*)
          quote=${value:0:1}; tail=${value:1}
          [[ "$tail" == *"$quote"* ]] || fail "Invalid quoted configuration at line $number"
          value=${tail%%"$quote"*}; tail=${tail#*"$quote"}
          trim "$tail"
          [[ -z "$mcl_value" || "$mcl_value" == \#* ]] || fail "Unexpected configuration text at line $number"
          ;;
        \#*) value= ;;
        *) trim "${value%%[[:blank:]]#*}"; value=$mcl_value ;;
      esac
      case "$key" in
        MS_CLIENT_ID) file_ms=$value ;;
        MCL_MICROSOFT_CLIENT_ID) file_ms_alias=$value ;;
        CURSEFORGE_API_KEY) file_cf=$value ;;
        MCL_CURSEFORGE_API_KEY) file_cf_alias=$value ;;
      esac
    done < "$root/.env"
  fi
  mcl_client=
  for value in "${MS_CLIENT_ID:-}" "${MCL_MICROSOFT_CLIENT_ID:-}" "$file_ms" "$file_ms_alias"; do
    trim "$value"; if [[ -n "$mcl_value" ]]; then mcl_client=$mcl_value; break; fi
  done
  mcl_cf_key=
  for value in "${CURSEFORGE_API_KEY:-}" "${MCL_CURSEFORGE_API_KEY:-}" "$file_cf" "$file_cf_alias"; do
    trim "$value"; if [[ -n "$mcl_value" ]]; then mcl_cf_key=$mcl_value; break; fi
  done
  [[ -z "$mcl_client" || "$mcl_client" =~ ^[a-fA-F0-9]{8}(-[a-fA-F0-9]{4}){3}-[a-fA-F0-9]{12}$ ]] || fail 'Invalid Microsoft application ID.'
}

valid_tag() {
  local part suffix=$1
  [[ "$1" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$ ]] || return 1
  if [[ "$1" == *-* ]]; then
    suffix=${1#*-}
    while [[ -n "$suffix" ]]; do
      part=${suffix%%.*}
      [[ ! "$part" =~ ^0[0-9]+$ ]] || return 1
      [[ "$suffix" == *.* ]] || break
      suffix=${suffix#*.}
    done
  fi
}

build_identity() {
  local requested=${1:-} tag
  mcl_commit=$(git rev-parse HEAD)
  [[ "$mcl_commit" =~ ^[a-f0-9]{40,64}$ ]] || fail 'Invalid Git commit.'
  mcl_dirty=false
  [[ -z $(git status --porcelain --untracked-files=normal) ]] || mcl_dirty=true
  mcl_version=dev
  if [[ -n "$requested" ]]; then
    valid_tag "$requested" || fail 'Release requires a v-prefixed semantic version tag.'
    [[ "$mcl_dirty" == false && $(git rev-parse "$requested^{commit}") == "$mcl_commit" ]] || fail 'Release requires a clean checkout at the selected existing tag.'
    mcl_version=$requested
  elif [[ "$mcl_dirty" == false ]]; then
    while IFS= read -r tag; do
      if valid_tag "$tag"; then mcl_version=$tag; fi
    done < <(git tag --points-at HEAD)
  fi
}

repository() {
  mcl_repository=$(sed -nE 's/^[[:space:]]*Repository[[:space:]]*=[[:space:]]*"([A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+)"$/\1/p' "$mcl_root/internal/buildinfo/info.go")
  [[ "$mcl_repository" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || fail 'Cannot read repository identity.'
}

encode_cf_key() {
  local LC_ALL=C mask=mcl-cf-9f3a7c1e-launcher i left right byte
  mcl_obfuscated=
  for ((i=0; i<${#mcl_cf_key}; i++)); do
    printf -v left '%d' "'${mcl_cf_key:i:1}"
    printf -v right '%d' "'${mask:i%${#mask}:1}"
    printf -v byte '%02x' "$((left ^ right))"
    mcl_obfuscated+=$byte
  done
}

new_work() {
  mkdir -p "$mcl_root/.cache"
  mcl_work=$(mktemp -d "$mcl_root/.cache/release.XXXXXX")
  trap cleanup_work EXIT
}
cleanup_work() {
  # The resolved prefix confines recursive cleanup to our own staging directory.
  case "$mcl_work" in
    "$mcl_root"/.cache/release.*) rm -rf -- "$mcl_work" ;;
    *) printf 'Refusing to remove an unexpected staging path.\n' >&2 ;;
  esac
}
