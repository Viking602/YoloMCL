#!/usr/bin/env bash
# Internal test fixture: package a replacement for the real MyGo updater test.
source "$(dirname "$0")/lib.sh"
source "$mcl_root/scripts/release-lib.sh"
[[ $# == 6 || $# == 7 ]] || fail 'Expected stage, output, public key, tag, commit, target and optional minimum version.'
umask 077
stage=$1 output=$2 public=$3
if command -v cygpath >/dev/null; then
  stage=$(cygpath -u "$stage"); output=$(cygpath -u "$output"); public=$(cygpath -u "$public")
fi
new_work
signing_key "$public" "$mcl_work"
repository
printf 'minimum_version=%s\n' "${7:-0.0.0}" > "$mcl_work/update-policy.conf"
package_release "$stage" "$output" "$4" "$5" "$6" 2023-11-14T22:13:20Z "$mcl_work" "$mcl_repository" "$mcl_work/update-policy.conf"
