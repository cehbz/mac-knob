#!/usr/bin/env bash
# Sign the given binaries with the spacekit self-signed identity, pulled from
# 1Password into a throwaway keychain that is torn down on exit. Nothing
# persistent is written to any keychain (login or otherwise).
#
# If the cert is unavailable this FAILS rather than ad-hoc signing: an ad-hoc
# binary's TCC grant is keyed to its cdhash, so silently installing one would
# orphan the Accessibility grant for the spaceswitch daemon and re-prompt.
# Set ALLOW_ADHOC=1 to opt into ad-hoc signing anyway (e.g. a fresh clone with
# no cert), accepting that grants will not persist across rebuilds.
set -euo pipefail

IDENTITY="${CODESIGN_IDENTITY:-spacekit}"
P12_REF="${OP_P12_REF:-op://Private/spacekit signing/p12}"
P12PW_REF="${OP_P12PW_REF:-op://Private/spacekit signing/p12password}"
bins=("$@")

adhoc() {
  echo "warning: ad-hoc signing (ALLOW_ADHOC) — TCC grants will NOT persist across rebuilds." >&2
  for b in "${bins[@]}"; do
    codesign --force --identifier "bz.ceh.$(basename "$b")" --sign - "$b"
  done
}

fail() {
  echo "sign.sh: $1" >&2
  echo "  Refusing to ad-hoc sign (would orphan the spaceswitch Accessibility grant)." >&2
  echo "  Fix: 'op signin' (see README 'Stable signing'), or set ALLOW_ADHOC=1 to sign ad-hoc anyway." >&2
  exit 1
}

if ! command -v op >/dev/null 2>&1; then
  [ "${ALLOW_ADHOC:-}" = "1" ] && { adhoc; exit 0; }
  fail "1Password CLI (op) not found"
fi

# `op read` auto-prompts (Touch ID) when the 1Password desktop-app integration
# is on, which is the normal case. If it fails, the integration is off / there
# is no session — run `op signin` first, then retry.
if ! p12_b64=$(op read "$P12_REF" 2>/dev/null); then
  [ "${ALLOW_ADHOC:-}" = "1" ] && { adhoc; exit 0; }
  fail "could not read the signing cert ($P12_REF) — unlock 1Password or run 'op signin'"
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
