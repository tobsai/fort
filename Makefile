# fort-native — build & test (backlog AO-043)
VERSION ?= 0.1.0
PKG := ./...

.PHONY: build test race vet install snapshot release clean apple-project apple-build mac-dmg

build: ## build the fort binary into ./bin
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o bin/fort ./cmd/fort

test: ## run the test suite
	go test $(PKG)

race: ## run tests under the race detector
	go test -race $(PKG)

vet: ## static checks
	go vet $(PKG)

install: build ## install fort into GOPATH/bin
	go install ./cmd/fort

snapshot: ## local cross-platform build (no publish); needs goreleaser
	goreleaser build --snapshot --clean

release: ## cut a release + push the Homebrew formula; needs a git tag + GITHUB_TOKEN
	goreleaser release --clean

apple-project: ## generate ui/apple/Fort.xcodeproj from project.yml (needs xcodegen)
	cd ui/apple && xcodegen generate

apple-build: apple-project ## compile-verify FortKit + shipping Apple client targets (unsigned)
	cd ui/apple/FortKit && swift build
	# Use the generic simulator destination so Xcode selects the matching iOS SDK.
	cd ui/apple && xcodebuild -project Fort.xcodeproj -scheme Fort -destination 'generic/platform=iOS Simulator' build CODE_SIGNING_ALLOWED=NO | tail -1
	cd ui/apple && xcodebuild -project Fort.xcodeproj -scheme FortMac -destination 'platform=macOS' build CODE_SIGNING_ALLOWED=NO | tail -1

# --- FortMac signed + notarized DMG (spec 032) ---------------------------------
# Operator runbook-as-Makefile. This CANNOT run in CI: signing, notarization and
# the Developer ID cert are the operator's credentialed steps. See
# docs/notes/mac-app.md. Prereqs (once):
#   * Xcode + a "Developer ID Application" cert (team T3JB5MYZ93) in the login keychain.
#   * A notarytool keychain profile holding an app-specific password:
#       xcrun notarytool store-credentials fort-notary \
#         --apple-id <APPLE_ID> --team-id T3JB5MYZ93 --password <APP_SPECIFIC_PASSWORD>
NOTARY_PROFILE ?= fort-notary
DEVELOPER_ID    ?= Developer ID Application: Maple Tree Enterprises LLC (T3JB5MYZ93)
MAC_ARCHIVE    := build/FortMac.xcarchive
MAC_EXPORT     := build/FortMac-export
MAC_DMG        := build/Fort.dmg

mac-dmg: build apple-project ## archive → sign → notarize → staple → Fort.dmg (operator-only)
	# 1. Bundle the daemon: copy the `make build` fort binary into the app so the
	#    ServiceController can shell out to a co-located `fort` (Contents/Resources).
	#    XcodeGen doesn't stage it, so we inject it into the archived app below.
	# 2. Archive the FortMac scheme (uses the operator's Developer ID signing).
	cd ui/apple && xcodebuild -project Fort.xcodeproj -scheme FortMac \
		-configuration Release -destination 'platform=macOS' \
		-archivePath ../../$(MAC_ARCHIVE) archive
	# 3. Place the bundled daemon into the archived .app before export/sign so it
	#    is covered by the app signature.
	cp bin/fort $(MAC_ARCHIVE)/Products/Applications/FortMac.app/Contents/Resources/fort
	# 4. Export a Developer ID–signed .app (needs the operator's cert + team).
	cd ui/apple && xcodebuild -exportArchive \
		-archivePath ../../$(MAC_ARCHIVE) \
		-exportOptionsPlist ExportOptions-mac.plist \
		-exportPath ../../$(MAC_EXPORT)
	# 5. The daemon was injected after archive, so harden its Developer ID
	#    signature explicitly and reseal the outer app before notarization.
	codesign --force --timestamp --options runtime --sign "$(DEVELOPER_ID)" \
		$(MAC_EXPORT)/FortMac.app/Contents/Resources/fort
	codesign --force --timestamp --options runtime \
		--preserve-metadata=entitlements,requirements --sign "$(DEVELOPER_ID)" \
		$(MAC_EXPORT)/FortMac.app
	codesign --verify --deep --strict --verbose=2 $(MAC_EXPORT)/FortMac.app
	# 6. Build the DMG from the exported app (hdiutil; swap for create-dmg if
	#    you want a styled background/volume icon).
	rm -f $(MAC_DMG)
	hdiutil create -volname Fort -srcfolder $(MAC_EXPORT)/FortMac.app \
		-ov -format UDZO $(MAC_DMG)
	# 7. Sign the container so Gatekeeper can validate the downloaded image
	#    itself before mounting it.
	codesign --force --timestamp --sign "$(DEVELOPER_ID)" $(MAC_DMG)
	codesign --verify --strict --verbose=2 $(MAC_DMG)
	# 8. Notarize the DMG (operator's Apple ID via the keychain profile above).
	xcrun notarytool submit $(MAC_DMG) --keychain-profile $(NOTARY_PROFILE) --wait
	# 9. Staple the notarization ticket so the DMG verifies offline.
	xcrun stapler staple $(MAC_DMG)
	@echo "Notarized DMG: $(MAC_DMG)"

clean:
	rm -rf bin dist build .fort-native ui/apple/Fort.xcodeproj ui/apple/FortKit/.build
