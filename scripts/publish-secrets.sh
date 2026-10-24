#!/usr/bin/env bash
source "$(dirname "$0")/lib.sh"
source "$mcl_root/scripts/release-lib.sh"
[[ $# == 0 ]] || fail 'publish-secrets.sh takes no arguments.'
need gh
need "${OPENSSL:-openssl}"
umask 077
load_publisher "$mcl_root"
[[ -n "$mcl_client" && -n "$mcl_cf_key" ]] || fail 'MS_CLIENT_ID and CURSEFORGE_API_KEY are required.'
repository
new_work
case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*) config_dir=$(cygpath -u "${APPDATA:?APPDATA is required}") ;;
  Darwin) config_dir="$HOME/Library/Application Support" ;;
  *) config_dir=${XDG_CONFIG_HOME:-$HOME/.config} ;;
esac
key_path="$config_dir/YoloMCL/publisher/update-private-key.txt"
public_file=internal/buildinfo/update-public-key.txt
if [[ ! -f "$key_path" ]]; then
  [[ -z $(tr -d '[:space:]' < "$public_file") ]] || fail 'Restore the existing private-key backup; refusing to rotate the published key.'
  ossl genpkey -algorithm ED25519 -outform DER -out "$mcl_work/generated.der"
  ossl pkey -inform DER -in "$mcl_work/generated.der" -pubout -outform DER > "$mcl_work/generated-public.der"
  { tail -c 32 "$mcl_work/generated.der"; tail -c 32 "$mcl_work/generated-public.der"; } > "$mcl_work/generated.raw"
  mkdir -p "$(dirname "$key_path")"
  (set -o noclobber; base64_line < "$mcl_work/generated.raw" > "$key_path")
  tail -c 32 "$mcl_work/generated-public.der" | base64_line > "$public_file"
  printf '\n' >> "$public_file"
fi
export MYGO_UPDATER_PRIVATE_KEY
MYGO_UPDATER_PRIVATE_KEY=$(tr -d '[:space:]' < "$key_path")
if [[ -z $(tr -d '[:space:]' < "$public_file") ]]; then
  private_der "$mcl_work"
  base64_line < "$mcl_work/derived-public.raw" > "$public_file"
  printf '\n' >> "$public_file"
fi
signing_key "$public_file" "$mcl_work"
printf '%s' "$mcl_cf_key" | gh secret set CURSEFORGE_API_KEY --repo "$mcl_repository"
printf '%s' "$mcl_client" | gh secret set MS_CLIENT_ID --repo "$mcl_repository"
printf '%s' "$MYGO_UPDATER_PRIVATE_KEY" | gh secret set MYGO_UPDATER_PRIVATE_KEY --repo "$mcl_repository"
printf 'Configured Actions Secrets. Private signing-key backup: %s\n' "$key_path"
