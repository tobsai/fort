"""Verify every regular Mach-O and code bundle with Apple codesign, without running it."""
from pathlib import Path
import contextlib
import hashlib
import importlib.util
import io
import json
import os
import plistlib
import re
import struct

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('utm_inspection_nested', HERE / 'inspect.py')
inspection = importlib.util.module_from_spec(spec)
spec.loader.exec_module(inspection)
base, APP, MOUNT = inspection.base, inspection.APP, inspection.MOUNT
write = inspection.write
MAGIC = {bytes.fromhex(x) for x in ('feedface', 'cefaedfe', 'feedfacf', 'cffaedfe',
                                    'cafebabe', 'bebafeca', 'cafebabf', 'bfbafeca')}


def architectures(path):
    with path.open('rb') as source:
        header = source.read(4096)
    magic = header[:4]
    assert magic in MAGIC
    if magic in (bytes.fromhex('cafebabe'), bytes.fromhex('cafebabf'),
                 bytes.fromhex('bebafeca'), bytes.fromhex('bfbafeca')):
        order = '>' if magic in (bytes.fromhex('cafebabe'), bytes.fromhex('cafebabf')) else '<'
        count = struct.unpack_from(order + 'I', header, 4)[0]
        assert 1 <= count <= 8
        width = 32 if magic in (bytes.fromhex('cafebabf'), bytes.fromhex('bfbafeca')) else 20
        cpus = [struct.unpack_from(order + 'II', header, 8 + index * width) for index in range(count)]
    else:
        order = '<' if magic in (bytes.fromhex('cefaedfe'), bytes.fromhex('cffaedfe')) else '>'
        cpus = [struct.unpack_from(order + 'II', header, 4)]
    result = []
    for cpu, subtype in cpus:
        assert cpu in (0x01000007, 0x0100000C), (str(path), cpu, subtype)
        result.append('x86_64' if cpu == 0x01000007 else 'arm64e' if subtype & 0xFFFFFF == 2 else 'arm64')
    assert len(set(result)) == len(result)
    return result


assert os.path.ismount(MOUNT) and os.statvfs(MOUNT).f_flag & os.ST_RDONLY
assert json.loads((HERE / 'app-gatekeeper.json').read_text())['exit_code'] == 0
manifest = json.loads((HERE / 'mounted-app-manifest.json').read_text())
code, bundles, legal = [], [], []
for entry in manifest:
    path = MOUNT / entry['path']
    if entry['type'] == 'regular':
        with path.open('rb') as source:
            magic = source.read(4)
        if magic in MAGIC:
            code.append({'path': entry['path'], 'sha256': entry['sha256'],
                         'bytes': entry['bytes'], 'architectures': architectures(path)})
        if any(word in path.name.lower() for word in ('license', 'licence', 'copying', 'notice', 'acknowledge', 'credits')):
            assert entry['bytes'] <= 1024**2
            text = path.read_text(errors='replace')
            legal.append({'path': entry['path'], 'bytes': entry['bytes'], 'sha256': entry['sha256'],
                          'copyright_lines': [line.strip() for line in text.splitlines() if 'copyright' in line.lower()][:32],
                          'full_content_not_copied': True})
    if entry['type'] == 'directory' and path.suffix in ('.app', '.xpc', '.framework', '.appex'):
        bundles.append(entry['path'])
assert len(code) == 80
write('nested-code-inventory.json', {'method': 'Regular files selected by Mach-O/fat magic; architecture headers read as data. All architectures verified, each architecture displayed separately.',
    'code_files': code, 'code_bundles': bundles, 'symlink_aliases_not_reexecuted_or_followed_for_enumeration': True})
write('legal-file-inspection.json', {'filename_patterns': ['license', 'licence', 'copying', 'notice', 'acknowledge', 'credits'],
    'matching_regular_files': legal, 'volume_root_names': sorted(p.name for p in MOUNT.iterdir()),
    'scope_limit': 'Static filename/text inspection only; no legal completeness or redistribution opinion. No app UI opened.'})

results = []
with (HERE / 'nested-codesign-receipts.jsonl').open('x') as journal:
    def record(name, value):
        data = json.dumps({'receipt': name, **value}) + '\n'
        assert len(data.encode()) < 2 * 1024**2
        journal.write(data)
        journal.flush()
        os.fsync(journal.fileno())

    base.write = record
    def command(name, argv):
        with contextlib.redirect_stdout(io.StringIO()):
            return base.require(base.run(name, argv, 60))

    for index, item in enumerate(code, 1):
        path = str(MOUNT / item['path'])
        command('macho-%03d-verify' % index, ['/usr/bin/codesign', '--verify', '--strict',
                 '--all-architectures', '--verbose=2', path])
        arch_details = []
        for arch in item['architectures']:
            result = command('macho-%03d-%s-display' % (index, arch), ['/usr/bin/codesign', '-d',
                '--arch', arch, '--verbose=4', '--entitlements', ':-', path])
            stderr = result['stderr']
            team = re.search(r'^TeamIdentifier=(.+)$', stderr, re.M)
            identifier = re.search(r'^Identifier=(.+)$', stderr, re.M)
            authorities = re.findall(r'^Authority=(.+)$', stderr, re.M)
            entitlements = plistlib.loads(result['stdout'].encode()) if result['stdout'].strip() else {}
            details = {'architecture': arch, 'team_identifier': team[1] if team else None,
                'identifier': identifier[1] if identifier else None, 'authorities': authorities,
                'entitlements': entitlements, 'cdhashes': re.findall(r'^CDHash=(.+)$', stderr, re.M)}
            # Inspection records the actual signer; a different valid signer is not silently rewritten.
            assert authorities, 'Missing certificate authority: ' + path
            arch_details.append(details)
        results.append({'path': item['path'], 'all_architectures_verified': True, 'signatures': arch_details})
        if index % 20 == 0:
            print(json.dumps({'macho_files_verified': index}), flush=True)
    for index, relative in enumerate(bundles, 1):
        command('bundle-%03d-verify' % index, ['/usr/bin/codesign', '--verify', '--strict',
            '--all-architectures', '--verbose=2', str(MOUNT / relative)])

# Every derived app-group/container candidate is checked by name only, never opened.
candidates = set()
for item in results:
    for signature in item['signatures']:
        ent = signature['entitlements']
        for group in ent.get('com.apple.security.application-groups', []):
            assert re.fullmatch(r'[A-Za-z0-9.-]+', group)
            candidates.add('/Users/tobiasgunn/Library/Group Containers/' + group)
        app_identifier = ent.get('com.apple.application-identifier')
        team = signature['team_identifier']
        if app_identifier and team and app_identifier.startswith(team + '.'):
            bundle_id = app_identifier[len(team) + 1:]
            assert re.fullmatch(r'[A-Za-z0-9.-]+', bundle_id)
            candidates.add('/Users/tobiasgunn/Library/Containers/' + bundle_id)
write('nested-signatures-and-entitlements.json', {'files': results,
    'macho_file_count': len(code), 'architecture_signature_count': sum(len(item['signatures']) for item in results),
    'code_bundle_count': len(bundles), 'all_codesign_commands_passed': True})
write('all-signed-container-paths.json', {'time_utc': base.now(),
    'paths': [{'path': path, 'present': os.path.lexists(path)} for path in sorted(candidates)],
    'method': 'Exact IDs derived from signed entitlements, lstat-only presence checks; no existing contents opened.'})
assert all(not os.path.lexists(path) for path in candidates), 'Existing signed container path; no cleanup authority over it'
print(json.dumps({'macho_files_verified': len(code), 'architecture_signatures_inspected': sum(len(item['signatures']) for item in results),
                  'code_bundles_verified': len(bundles), 'signed_container_candidates_absent': len(candidates)}))
