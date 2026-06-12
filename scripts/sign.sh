#!/usr/bin/env bash
# Sign the given binaries with the mac-knob self-signed identity, pulled from
# 1Password into a throwaway keychain that is torn down on exit. Nothing
# persistent is written to any keychain (login or otherwise). If the secret
# or the 1Password CLI is unavailable, fall back to ad-hoc signing with a
# stable identifier and warn (ad-hoc grants do not survive rebuilds).
set -euo pipefail

IDENTITY="${CODESIGN_IDENTITY:-mac-knob}"
P12_REF="${OP_P12_REF:-op://Private/mac-knob signing/p12}"
P12PW_REF="${OP_P12PW_REF:-op://Private/mac-knob signing/p12password}"
bins=("$@")

adhoc() {
  echo "warning: ad-hoc signing — Accessibility/Screen Recording grants will NOT persist across rebuilds." >&2
  for b in "${bins[@]}"; do
    codesign --force --identifier "bz.ceh.$(basename "$b")" --sign - "$b"
  done
}

if ! command -v op >/dev/null 2>&1; then
  echo "1Password CLI (op) not found." >&2; adhoc; exit 0
fi
if ! p12_b64=$(op read "$P12_REF" 2>/dev/null); then
  echo "signing secret not available ($P12_REF); run scripts/gen-signing-cert.sh and store it, or 'op signin'." >&2
  adhoc; exit 0
fi
p12pw=$(op read "$P12PW_REF")

work=$(mktemp -d)
kc="$work/sign.keychain-db"
kcpw=$(openssl rand -hex 16)
orig=$(security list-keychains -d user | sed 's/[" ]//g')

cleanup() {
  security list-keychains -d user -s $orig >/dev/null 2>&1 || true
  security delete-keychain "$kc" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT

printf '%s' "$p12_b64" | base64 -d > "$work/cert.p12"
security create-keychain -p "$kcpw" "$kc"
security set-keychain-settings "$kc"           # no auto-lock during the build
security unlock-keychain -p "$kcpw" "$kc"
security import "$work/cert.p12" -k "$kc" -P "$p12pw" -T /usr/bin/codesign >/dev/null
security list-keychains -d user -s "$kc" $orig
security set-key-partition-list -S apple-tool:,apple: -s -k "$kcpw" "$kc" >/dev/null 2>&1

for b in "${bins[@]}"; do
  codesign --force --identifier "bz.ceh.$(basename "$b")" --sign "$IDENTITY" --keychain "$kc" "$b"
  echo "signed $b with $IDENTITY"
done
