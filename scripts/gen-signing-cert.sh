#!/usr/bin/env bash
# Generate a self-signed Code Signing certificate (no Keychain Access, no
# login keychain) and bundle it as a password-protected .p12. The .p12 plus
# its password are the durable signing identity; store them in 1Password and
# delete the local file. Reusing the same cert keeps the binaries' Designated
# Requirement (identifier + cert leaf) stable, so TCC grants survive rebuilds.
set -euo pipefail

OUT="${1:-spacekit-signing.p12}"
CN="${CODESIGN_IDENTITY:-spacekit}"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

p12pw=$(openssl rand -base64 24)
openssl req -x509 -newkey rsa:2048 -keyout "$work/key.pem" -out "$work/cert.pem" \
  -days 3650 -nodes -subj "/CN=$CN" \
  -addext "basicConstraints=critical,CA:FALSE" \
  -addext "keyUsage=critical,digitalSignature" \
  -addext "extendedKeyUsage=critical,codeSigning" 2>/dev/null
openssl pkcs12 -export -inkey "$work/key.pem" -in "$work/cert.pem" \
  -out "$OUT" -name "$CN" -passout "pass:$p12pw" 2>/dev/null

leaf=$(openssl x509 -in "$work/cert.pem" -outform DER | shasum | awk '{print $1}')

cat <<EOF

Created $OUT  (CN=$CN, codeSigning, valid 10 years)
  p12 password : $p12pw
  cert leaf SHA1: $leaf   <- the H"..." that will appear in the Designated Requirement

Store in 1Password (adjust the vault), then remove the local file:

  op item create --category 'API Credential' --title 'spacekit signing' --vault Private \\
    "p12[password]=\$(base64 < $OUT)" \\
    "p12password[password]=$p12pw"
  rm $OUT

The Makefile reads these references (override OP_P12_REF / OP_P12PW_REF if you
name things differently):
  op://Private/spacekit signing/p12
  op://Private/spacekit signing/p12password
EOF
