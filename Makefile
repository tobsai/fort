# fort-native — build & test (backlog AO-043)
VERSION ?= 0.1.0
PKG := ./...

.PHONY: build test race vet install snapshot release clean

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

apple-build: apple-project ## compile-verify FortKit + all Apple client targets (unsigned)
	cd ui/apple/FortKit && swift build
	cd ui/apple && xcodebuild -project Fort.xcodeproj -scheme Fort -sdk iphonesimulator -destination 'generic/platform=iOS Simulator' build CODE_SIGNING_ALLOWED=NO | tail -1
	cd ui/apple && xcodebuild -project Fort.xcodeproj -scheme FortMac -destination 'platform=macOS' build CODE_SIGNING_ALLOWED=NO | tail -1
	cd ui/apple && xcodebuild -project Fort.xcodeproj -scheme FortWatch -sdk watchsimulator -destination 'generic/platform=watchOS Simulator' build CODE_SIGNING_ALLOWED=NO | tail -1
	cd ui/apple && xcodebuild -project Fort.xcodeproj -scheme FortComplication -sdk watchsimulator -destination 'generic/platform=watchOS Simulator' build CODE_SIGNING_ALLOWED=NO | tail -1

clean:
	rm -rf bin dist .fort-native ui/apple/Fort.xcodeproj ui/apple/FortKit/.build
