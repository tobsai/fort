#!/bin/bash
# Build Fort for Mac with its local runner inside (the FortHost helper with Node, Codex and the process
# guardian), sign it for the Mac App Store and upload it to TestFlight. Run on an Apple-silicon Mac with
# Xcode and Node 26.5.0.
#
#   make -C successor mac-package             # ad-hoc signed, layout checked, nothing uploaded (CI runs this)
#   make -C successor beta PLATFORM=mac       # distribution signed and uploaded to TestFlight
#
# Distribution needs, already in place on the Mac (this script never creates, exports or stores keys):
#   FORT_TEAM_ID              Apple developer team ID
#   ASC_KEY_ID, ASC_ISSUER_ID, ASC_KEY_PATH   App Store Connect API key, for profiles and the upload
#   Apple Distribution and Mac Installer Distribution identities in the keychain
#   App IDs for the app and helper registered with the App Group
# Mac App Store profiles are fetched (or created) with the API key unless FORT_MAC_PROFILE_APP and
# FORT_MAC_PROFILE_HOST point at profile files.
# Optional: FORT_BUNDLE_ID_MAC (default io.mtree.fort), FORT_HOST_BUNDLE_ID_MAC (default <app>.host),
#   FORT_APP_GROUP_MAC (default group.<app>), FORT_MAC_APP_IDENTITY (default "Apple Distribution"),
#   FORT_MAC_INSTALLER_IDENTITY (default "3rd Party Mac Developer Installer"), FORT_MARKETING_VERSION,
#   BUILD_NUMBER, FORT_SKIP_UPLOAD=1, FORT_MAC_SIGNING=adhoc|distribution.
set -euo pipefail

mode="${FORT_MAC_SIGNING:-distribution}"
case "$mode" in distribution|adhoc) ;; *) echo "FORT_MAC_SIGNING must be distribution or adhoc" >&2; exit 2 ;; esac

here="$(cd "$(dirname "$0")/.." && pwd)"
build="${BUILD_NUMBER:-$(date -u +%Y%m%d%H%M)}"
client="${FORT_BUNDLE_ID_MAC:-io.mtree.fort}"
service="${FORT_HOST_BUNDLE_ID_MAC:-$client.host}"
group="${FORT_APP_GROUP_MAC:-group.$client}"
node_version=v26.5.0
node_sha256=ee920559aaa2391569cff4d737e3b83963430e3a14dedd91bfe0ff53171b5af9   # node-v26.5.0-darwin-arm64.tar.gz
codex_sha256=8813367285524ac798dd96d1b071aea30d58708c7226fd236cd275cf1cfdee31  # codex 0.163.0, aarch64-apple-darwin

if [ "$mode" = distribution ]; then
  team="${FORT_TEAM_ID:?Set FORT_TEAM_ID to the Apple developer team ID}"
  if [ "${FORT_SKIP_UPLOAD:-}" != 1 ] || [ -z "${FORT_MAC_PROFILE_APP:-}" ] || [ -z "${FORT_MAC_PROFILE_HOST:-}" ]; then
    : "${ASC_KEY_ID:?Set ASC_KEY_ID to the App Store Connect API key ID}"
    : "${ASC_ISSUER_ID:?Set ASC_ISSUER_ID to the App Store Connect issuer ID}"
    : "${ASC_KEY_PATH:?Set ASC_KEY_PATH to the .p8 key file}"
  fi
else
  team="${FORT_TEAM_ID:-AAAAAAAAAA}"   # placeholder; an ad-hoc build is never uploaded or run as the host
fi
[ "$(uname -s)/$(uname -m)" = Darwin/arm64 ] || { echo "Build on an Apple-silicon Mac" >&2; exit 2; }
[ "$(node --version)" = "$node_version" ] || { echo "Put Node $node_version first on PATH" >&2; exit 2; }
[ -x "$here/node_modules/typescript/bin/tsc" ] || npm --prefix "$here" ci --ignore-scripts --no-audit --no-fund

out="$here/release/build/mac-$build"
work="$out/work"
[ ! -e "$out" ] || { echo "$out already exists; set another BUILD_NUMBER" >&2; exit 2; }
umask 022
mkdir -p "$work"

echo "1/7 Building FortMac and FortHost (unsigned, arm64, build $build)"
version_args=()
[ -n "${FORT_MARKETING_VERSION:-}" ] && version_args=(MARKETING_VERSION="$FORT_MARKETING_VERSION")
for scheme in FortMac FortHost; do
  xcodebuild build -quiet \
    -project "$here/apple/FortSpec058.xcodeproj" -scheme "$scheme" -configuration Release \
    -destination 'generic/platform=macOS' -derivedDataPath "$work/derived" \
    ARCHS=arm64 ONLY_ACTIVE_ARCH=NO CODE_SIGNING_ALLOWED=NO CODE_SIGN_IDENTITY= \
    CURRENT_PROJECT_VERSION="$build" ${version_args[@]+"${version_args[@]}"} \
    INFOPLIST_KEY_LSApplicationCategoryType=public.app-category.productivity \
    INFOPLIST_KEY_ITSAppUsesNonExemptEncryption=NO
done
products="$work/derived/Build/Products/Release"
# Bundle IDs are set on the built Info.plist rather than on the xcodebuild command line, which would also
# rename FortKit's resource bundle and collide with the app's ID.
mkdir -p "$work/apps"
ditto "$products/FortMac.app" "$work/apps/FortMac.app"
ditto "$products/FortHost.app" "$work/apps/FortHost.app"
plutil -replace CFBundleIdentifier -string "$client" "$work/apps/FortMac.app/Contents/Info.plist"
plutil -replace CFBundleName -string Fort "$work/apps/FortMac.app/Contents/Info.plist"
plutil -replace CFBundleIdentifier -string "$service" "$work/apps/FortHost.app/Contents/Info.plist"

echo "2/7 Building the process guardian"
xcrun swiftc -O -target arm64-apple-macos14.0 "$here/host/mac-process-guardian.swift" -o "$work/FortProcessGuardian"

echo "3/7 Building host resources"
node "$here/packaging/host-resources.ts" --destination "$work/host-resources" > "$work/host-resources.json"

echo "4/7 Bundling Node $node_version"
curl -fsSL -o "$work/node.tar.gz" "https://nodejs.org/dist/$node_version/node-$node_version-darwin-arm64.tar.gz"
echo "$node_sha256  $work/node.tar.gz" | shasum -a 256 -c -
mkdir "$work/node-dist" && tar -xzf "$work/node.tar.gz" -C "$work/node-dist"
node "$here/packaging/node-bundle.ts" --official --runtime "$work/node-dist/node-$node_version-darwin-arm64/bin/node" --destination "$work/node-runtime"

echo "5/7 Fetching Codex 0.163.0"
if [ -n "${FORT_CODEX_PATH:-}" ]; then cp "$FORT_CODEX_PATH" "$work/codex"
else
  curl -fsSL -o "$work/codex.tar.gz" https://github.com/openai/codex/releases/download/rust-v0.163.0/codex-aarch64-apple-darwin.tar.gz
  mkdir "$work/codex-dist" && tar -xzf "$work/codex.tar.gz" -C "$work/codex-dist"
  cp "$work/codex-dist/codex-aarch64-apple-darwin" "$work/codex"
fi
chmod 755 "$work/codex"
echo "$codex_sha256  $work/codex" | shasum -a 256 -c -

echo "6/7 Assembling Fort.app with the helper inside"
printf '{"format":"1","clientIdentifier":"%s","serviceIdentifier":"%s","signingTeam":"%s","appGroup":"%s"}\n' \
  "$client" "$service" "$team" "$group" > "$work/fort-host.json"
node "$here/packaging/mac-application.ts" \
  --client "$work/apps/FortMac.app" --helper "$work/apps/FortHost.app" \
  --host-resources "$work/host-resources" --node-runtime "$work/node-runtime" \
  --codex "$work/codex" --guardian "$work/FortProcessGuardian" \
  --configuration "$work/fort-host.json" --destination "$out/assembled"

if [ "$mode" = distribution ]; then
  if [ -z "${FORT_MAC_PROFILE_APP:-}" ]; then
    node "$here/release/asc-profile.ts" --bundle-id "$client" --type MAC_APP_STORE --output "$work/Fort.provisionprofile"
    FORT_MAC_PROFILE_APP="$work/Fort.provisionprofile"
  fi
  if [ -z "${FORT_MAC_PROFILE_HOST:-}" ]; then
    node "$here/release/asc-profile.ts" --bundle-id "$service" --type MAC_APP_STORE --output "$work/FortHost.provisionprofile"
    FORT_MAC_PROFILE_HOST="$work/FortHost.provisionprofile"
  fi
fi

echo "7/7 Signing ($mode) and packaging"
sign=(--app "$out/assembled/Fort.app" --mode "$mode" --helper-entitlements "$here/apple/Host/FortHost.entitlements"
  --package "$out/Fort.pkg" --report "$out/signing-report.json")
if [ "$mode" = distribution ]; then
  sign+=(--app-identity "${FORT_MAC_APP_IDENTITY:-Apple Distribution}"
    --installer-identity "${FORT_MAC_INSTALLER_IDENTITY:-3rd Party Mac Developer Installer}"
    --client-profile "$(cd "$(dirname "$FORT_MAC_PROFILE_APP")" && pwd)/$(basename "$FORT_MAC_PROFILE_APP")"
    --helper-profile "$(cd "$(dirname "$FORT_MAC_PROFILE_HOST")" && pwd)/$(basename "$FORT_MAC_PROFILE_HOST")")
fi
node "$here/packaging/mac-sign.ts" "${sign[@]}"

if [ "$mode" = adhoc ] || [ "${FORT_SKIP_UPLOAD:-}" = 1 ]; then
  echo "Built $out/Fort.pkg ($mode). Not uploaded."
  exit 0
fi

echo "Uploading build $build to App Store Connect"
keys="$(mktemp -d)"
trap 'rm -rf "$keys"' EXIT
cp "$ASC_KEY_PATH" "$keys/AuthKey_$ASC_KEY_ID.p8"
API_PRIVATE_KEYS_DIR="$keys" xcrun altool --upload-app --type macos --file "$out/Fort.pkg" \
  --apiKey "$ASC_KEY_ID" --apiIssuer "$ASC_ISSUER_ID"
echo "Uploaded build $build. It appears in TestFlight once Apple finishes processing."
