#!/bin/bash
# CI only: put the signing identities from APPLE_SIGNING_P12_BASE64 into a throwaway keychain for this job.
# The .p12 should hold Apple Development, Apple Distribution and Mac Installer Distribution identities
# (export all three from Keychain Access at once). Prints identity names only.
#   successor/release/ci-keychain.sh          # set up
#   successor/release/ci-keychain.sh remove   # tear down
set -euo pipefail
keychain="${RUNNER_TEMP:?Run inside GitHub Actions}/fort-signing.keychain-db"
if [ "${1:-}" = remove ]; then security delete-keychain "$keychain" 2>/dev/null || true; exit 0; fi
: "${APPLE_SIGNING_P12_BASE64:?Set the APPLE_SIGNING_P12_BASE64 secret}"
: "${APPLE_SIGNING_P12_PASSWORD:?Set the APPLE_SIGNING_P12_PASSWORD secret}"
password="$(uuidgen)"
security create-keychain -p "$password" "$keychain"
security set-keychain-settings -lut 21600 "$keychain"
security unlock-keychain -p "$password" "$keychain"
p12="$RUNNER_TEMP/fort-signing.p12"
umask 077
printf '%s' "$APPLE_SIGNING_P12_BASE64" | base64 --decode > "$p12"
security import "$p12" -k "$keychain" -P "$APPLE_SIGNING_P12_PASSWORD" \
  -T /usr/bin/codesign -T /usr/bin/productbuild -T /usr/bin/productsign -T /usr/bin/security
rm -f "$p12"
security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$password" "$keychain" > /dev/null
# shellcheck disable=SC2046
security list-keychains -d user -s "$keychain" $(security list-keychains -d user | tr -d '"')
echo "Signing identities:"
security find-identity -v "$keychain" | sed -E 's/^ *[0-9]+\) [0-9A-F]+ /  /'
