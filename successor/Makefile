.PHONY: install test test-mac beta mac-package

install:
	npm ci --ignore-scripts --no-audit --no-fund

test:
	npm run typecheck
	npm test

test-mac: test
	npm run test:macos

beta:
	./release/beta.sh

# Builds Fort.app with its local runner, signs it ad hoc and checks every signature. Nothing is uploaded.
mac-package:
	FORT_MAC_SIGNING=adhoc ./release/mac-testflight.sh
