"""Assess the pinned disk image with existing Apple tools. No app launch."""
from pathlib import Path
import sys
sys.path.insert(0, str(Path(__file__).resolve().parent))
from assessment import DMG, PIN, SIZE, digest, require, run

assert DMG.lstat().st_size == SIZE and digest(DMG) == PIN
require(run('dmg-integrity.json', ['/usr/bin/hdiutil', 'verify', str(DMG)], 120))
require(run('dmg-stapler.json', ['/usr/bin/xcrun', '--no-cache', 'stapler', 'validate', str(DMG)], 120))
require(run('dmg-gatekeeper.json', ['/usr/sbin/spctl', '--assess', '--type', 'open',
    '--context', 'context:primary-signature', '--verbose=4', str(DMG)], 120))
