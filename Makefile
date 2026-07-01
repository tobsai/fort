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

clean:
	rm -rf bin dist .fort-native
