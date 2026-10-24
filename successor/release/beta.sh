#!/bin/bash
# Archive Fort and upload it to TestFlight. Run on a Mac with Xcode.
#   make -C successor beta            # iPhone
#   make -C successor beta PLATFORM=mac
# Needs an App Store Connect API key and the real App Store bundle IDs.
set -euo pipefail

platform="${PLATFORM:-ios}"
: "${FORT_TEAM_ID:?Set FORT_TEAM_ID to the Apple developer team ID}"
: "${ASC_KEY_ID:?Set ASC_KEY_ID to the App Store Connect API key ID}"
: "${ASC_ISSUER_ID:?Set ASC_ISSUER_ID to the App Store Connect issuer ID}"
: "${ASC_KEY_PATH:?Set ASC_KEY_PATH to the .p8 key file}"

case "$platform" in
  ios) scheme=FortiOS; destination='generic/platform=iOS'; bundle="${FORT_BUNDLE_ID_IOS:?Set FORT_BUNDLE_ID_IOS to the App Store bundle ID}" ;;
  mac) scheme=FortMac; destination='generic/platform=macOS'; bundle="${FORT_BUNDLE_ID_MAC:?Set FORT_BUNDLE_ID_MAC to the App Store bundle ID}" ;;
  *) echo "PLATFORM must be ios or mac" >&2; exit 2 ;;
esac

here="$(cd "$(dirname "$0")/.." && pwd)"
build="${BUILD_NUMBER:-$(date -u +%Y%m%d%H%M)}"
out="$here/release/build/$platform-$build"
mkdir -p "$out"
auth=(-allowProvisioningUpdates
  -authenticationKeyID "$ASC_KEY_ID"
  -authenticationKeyIssuerID "$ASC_ISSUER_ID"
  -authenticationKeyPath "$ASC_KEY_PATH")

echo "Archiving $scheme build $build as $bundle"
xcodebuild archive \
  -project "$here/apple/FortSpec058.xcodeproj" \
  -scheme "$scheme" -configuration Release -destination "$destination" \
  -archivePath "$out/Fort.xcarchive" \
  CODE_SIGNING_ALLOWED=YES CODE_SIGN_STYLE=Automatic \
  DEVELOPMENT_TEAM="$FORT_TEAM_ID" PRODUCT_BUNDLE_IDENTIFIER="$bundle" \
  CURRENT_PROJECT_VERSION="$build" \
  "${auth[@]}"

cat > "$out/ExportOptions.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>method</key><string>app-store-connect</string>
  <key>destination</key><string>upload</string>
  <key>teamID</key><string>$FORT_TEAM_ID</string>
  <key>signingStyle</key><string>automatic</string>
</dict></plist>
PLIST

echo "Uploading to App Store Connect"
xcodebuild -exportArchive \
  -archivePath "$out/Fort.xcarchive" \
  -exportOptionsPlist "$out/ExportOptions.plist" \
  -exportPath "$out/export" \
  "${auth[@]}"

echo "Uploaded build $build. It will appear in TestFlight after Apple finishes processing."
