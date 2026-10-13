"""Use existing Apple inspection tools only; never execute any bundle member."""
from pathlib import Path
import importlib.util
import json
import os
import plistlib
import re
import sys

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('utm_inspection', HERE / 'inspect.py')
inspection = importlib.util.module_from_spec(spec)
spec.loader.exec_module(inspection)
APP = inspection.APP
base = inspection.base
run, require, write = inspection.run, inspection.require, inspection.write


def parse_display(result):
    text = result['stderr']
    match = re.search(r'^TeamIdentifier=(.+)$', text, re.M)
    identifier = re.search(r'^Identifier=(.+)$', text, re.M)
    authorities = re.findall(r'^Authority=(.+)$', text, re.M)
    out = result['stdout']
    entitlements = plistlib.loads(out.encode()) if out.strip() else {}
    return {'team_identifier': match[1] if match else None,
            'identifier': identifier[1] if identifier else None,
            'authorities': authorities, 'entitlements': entitlements,
            'cdhashes': re.findall(r'^CDHash=(.+)$', text, re.M)}


def main():
    assert os.path.ismount(inspection.MOUNT) and os.statvfs(inspection.MOUNT).f_flag & os.ST_RDONLY
    assert json.loads((HERE / 'mounted-identity.json').read_text())['read_only_statvfs']
    info_path = APP / 'Contents/Info.plist'
    assert info_path.stat().st_size < 1024**2
    info = plistlib.loads(info_path.read_bytes())
    fields = {key: info.get(key) for key in ('CFBundleIdentifier', 'CFBundleShortVersionString',
        'CFBundleVersion', 'CFBundleExecutable', 'LSMinimumSystemVersion', 'LSUIElement',
        'LSApplicationCategoryType', 'NSMicrophoneUsageDescription', 'NSCameraUsageDescription',
        'NSAppleEventsUsageDescription')}
    write('app-metadata.json', fields)
    assert fields['CFBundleIdentifier'] == 'com.utmapp.UTM'
    assert fields['CFBundleShortVersionString'] == '4.7.5' and str(fields['CFBundleVersion']) == '118'
    require(run('app-codesign-verify.json', ['/usr/bin/codesign', '--verify', '--deep', '--strict',
                                           '--verbose=2', str(APP)], 180))
    display = require(run('app-codesign-display.json', ['/usr/bin/codesign', '-d', '--verbose=4',
                                                   '--entitlements', ':-', str(APP)], 60))
    details = parse_display(display)
    write('app-signing-and-entitlements.json', details)
    assert details['team_identifier'] and details['team_identifier'] != 'not set'
    assert details['authorities'] and details['authorities'][0].startswith('Developer ID Application:')
    gate = require(run('app-gatekeeper.json', ['/usr/sbin/spctl', '--assess', '--type', 'execute',
                                             '--verbose=4', str(APP)], 180))
    assert 'accepted' in gate['stderr'] and 'source=Notarized Developer ID' in gate['stderr']
    groups = details['entitlements'].get('com.apple.security.application-groups', [])
    assert isinstance(groups, list)
    candidates = [Path('/Users/tobiasgunn/Library/Containers') / fields['CFBundleIdentifier']]
    for group in groups:
        assert isinstance(group, str) and re.fullmatch(r'[A-Za-z0-9.-]+', group)
        candidates.append(Path('/Users/tobiasgunn/Library/Group Containers') / group)
    write('signed-container-paths.json', {'time_utc': base.now(),
        'source': 'Accepted app signature and entitlements; paths checked with lstat only.',
        'paths': [{'path': str(path), 'present': os.path.lexists(path)} for path in candidates],
        'existing_contents_opened': False})
    assert all(base.absent(path) for path in candidates), 'Existing UTM-specific container; stop without reading contents'
    print(json.dumps({'app_gatekeeper': 'accepted-notarized-developer-id',
        'team_identifier': details['team_identifier'], 'authorities': details['authorities'],
        'signed_group_ids': groups}), flush=True)


if __name__ == '__main__':
    main()
