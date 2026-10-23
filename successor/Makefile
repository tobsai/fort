.PHONY: install test test-mac beta

install:
	npm ci --ignore-scripts --no-audit --no-fund

test:
	npm run typecheck
	npm test

test-mac: test
	npm run test:macos

beta:
	./release/beta.sh
