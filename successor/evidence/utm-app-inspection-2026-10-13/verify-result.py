"""Separate read-only receipt reconciliation and post-cleanup host rescan."""
from pathlib import Path
import ast
import datetime
import hashlib
import json
import os
import plistlib
import subprocess

HERE = Path(__file__).resolve().parent
WORKTREE = HERE.parents[2]
ROOT = Path('/Users/tobiasgunn/Library/Application Support/FortUTMIsolation')
PRIOR = HERE.parent / 'utm-assessment-2026-10-13'
PROPOSAL = WORKTREE / 'successor/ops/utm-isolation-proposal-v1'
os.umask(0o077)

for directory, expected in ((PRIOR, 'be08287b55001500b29b1efb4d22cf6312e2ec0a75dd0f7d213090fc9983a8fd'),
                            (PROPOSAL, '99decb7c28b2c2daba3972709642a12ed4d3cce084a40bc3c3dd3e0008899159')):
    data = (directory / 'seal.json').read_bytes()
    assert hashlib.sha256(data).hexdigest() == expected
    for name, pin in json.loads(data)['files'].items():
        assert hashlib.sha256((directory / name).read_bytes()).hexdigest() == pin
for path in HERE.iterdir():
    assert path.is_file() and not path.is_symlink()
    if path.suffix == '.py':
        ast.parse(path.read_text(), filename=str(path))
    elif path.suffix == '.json':
        json.loads(path.read_text())
    elif path.suffix == '.jsonl':
        for line in path.read_text().splitlines():
            json.loads(line)
download = json.loads((HERE / '01-UTM.dmg.json').read_text())
assert download['outcome'] == 'accepted' and download['body_complete']
assert download['observed_bytes'] == 250021057
assert download['observed_digest'] == download['expected_digest'] == 'a8435c93cfb5f8bbfeea4b134cfad1ac66b67632b75e438c63b1a8ae043bef0e'
for name in ('dmg-integrity.json', 'dmg-stapler.json', 'mount-result.json', 'app-codesign-verify.json',
             'app-codesign-display.json', 'app-gatekeeper.json', 'detach-result.json'):
    receipt = json.loads((HERE / name).read_text())
    assert receipt['exit_code'] == 0 and receipt['failure'] is None, name
gate = json.loads((HERE / 'app-gatekeeper.json').read_text())
assert 'accepted\nsource=Notarized Developer ID' in gate['stderr']
main = json.loads((HERE / 'app-signing-and-entitlements.json').read_text())
assert main['team_identifier'] == 'WDNLXAD4W8'
assert main['authorities'] == ['Developer ID Application: Turing Software, LLC (WDNLXAD4W8)',
                              'Developer ID Certification Authority', 'Apple Root CA']
nested = json.loads((HERE / 'nested-signatures-and-entitlements.json').read_text())
assert (nested['macho_file_count'], nested['architecture_signature_count'], nested['code_bundle_count']) == (80, 160, 78)
assert all(s['team_identifier'] == 'WDNLXAD4W8' and s['authorities'] == main['authorities']
           for item in nested['files'] for s in item['signatures'])
receipts = [json.loads(line) for line in (HERE / 'nested-codesign-receipts.jsonl').read_text().splitlines()]
assert len(receipts) == 318 and len({r['receipt'] for r in receipts}) == 318
assert all(receipt['exit_code'] == 0 and receipt['failure'] is None for receipt in receipts)
assert sum(r['receipt'].endswith('-verify') for r in receipts) == 158
assert sum(r['receipt'].endswith('-display') for r in receipts) == 160
manifest = json.loads((HERE / 'download-manifest.json').read_text())
cleanup = json.loads((HERE / 'cleanup-result.json').read_text())
journal = [json.loads(line) for line in (HERE / 'cleanup-journal.jsonl').read_text().splitlines()]
assert len(manifest) == len(journal) == cleanup['owned_objects_removed'] == 5
assert {entry['path'] for entry in manifest} == {entry['path'] for entry in journal}
assert all(entry['removed'] for entry in journal)
paths = set(cleanup['absent_paths'])
assert len(paths) == 24 and all(not os.path.lexists(path) for path in paths)
images = subprocess.run(['/usr/bin/hdiutil', 'info', '-plist'], stdin=subprocess.DEVNULL,
                        capture_output=True, timeout=30, check=True)
assert len(images.stdout) + len(images.stderr) < 2 * 1024**2
assert all(str(ROOT) not in image.get('image-path', '') and all(str(ROOT) not in entity.get('mount-point', '')
           for entity in image.get('system-entities', [])) for image in plistlib.loads(images.stdout).get('images', []))
processes = subprocess.run(['/bin/ps', '-axo', 'pid=,comm='], stdin=subprocess.DEVNULL,
                            capture_output=True, timeout=10, check=True)
assert all(str(ROOT) not in line and '/UTM.app/Contents/' not in line for line in processes.stdout.decode().splitlines())
assert not subprocess.check_output(['git', 'diff', '--name-only'], cwd=WORKTREE).strip()
footprint = json.loads((HERE / 'mounted-app-footprint.json').read_text())
state = json.loads((HERE / 'candidate-state-paths-before-cleanup.json').read_text())
assert len(state['paths']) == 17 and all(not entry['present'] for entry in state['paths'])
report = {
    'time_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'outcome': 'static-artifact-inspection-passed-and-owned-cleanup-complete',
    'approval_reference': 'Sentinel_ea51da06ee408191a8b7ea5327e0f437',
    'artifact': {'name': 'UTM.dmg', 'version': '4.7.5', 'build': '118',
        'bytes': download['observed_bytes'], 'sha256': download['observed_digest']},
    'actual_signer': main['authorities'][0], 'team_identifier': main['team_identifier'],
    'passed': ['exact DMG source/size/hash', 'hdiutil integrity', 'DMG stapled notarization ticket',
        'read-only non-browsing mount', 'actual app version/build/bundle ID',
        'deep strict app code signature', 'app Gatekeeper accepted as Notarized Developer ID',
        '80 Mach-O all-architecture signature verifications', '160 architecture signer/entitlement inspections',
        '78 code-bundle all-architecture signature verifications',
        '1180 app entries rechecked unchanged before detach', '17 candidate state paths absent without opening contents',
        'normal exact-image detach', '5 owned staging objects removed',
        'separate 24-path absence and no-task-image/process rescan', 'prior assessment and proposal seals unchanged',
        'Python source syntax and JSON/JSONL validation'],
    'failed_this_round': [],
    'preserved_prior_failure': 'Same DMG primary-signature open assessment exited 3, no usable signature. The owner changed only that stop condition; OS trust/security settings were not changed.',
    'unrun': ['app/helper/utmctl/QEMU executable launch', 'VM creation/boot', 'guest downloads or verifiers',
        'guest confinement/PTY/NoCloud/process-crash experiment', 'provider or account operations',
        'separate Gatekeeper assessment of each nested helper/library', 'first-launch state/permissions/network behavior',
        'app-specific staple validation (DMG staple and app Gatekeeper notarization assessment were used)',
        'product test suites (no product-source changes)'],
    'limits': ['signatures and notarization are not a security audit or proof of OS isolation',
        'bundle ships QEMU and network/JIT-capable helpers even if a later VM chooses Apple Virtualization',
        'static candidate paths are not a complete first-launch filesystem inventory',
        'legal filename/text inspection is not a redistribution/license-completeness opinion',
        'OS trust-cache effects not erased; no shared OS database edits or packet capture',
        'separate rescan was a second script by this worker, not independent agent review'],
    'app_logical_bytes': footprint['logical_file_bytes'], 'app_copy_bytes': 0,
    'cleanup_owned_objects': 5, 'absent_paths': sorted(paths), 'task_images_attached': 0,
    'downloaded_code_executed': False, 'next_step': 'Prepare and independently review the bounded Apple Virtualization/serial synthetic-probe packet. Any app launch, guest download/verifier execution or VM session needs its separate exact authority; none is authorized by this inspection.'}
with (HERE / 'validation.json').open('x') as output:
    json.dump(report, output, indent=2)
    output.write('\n')
print(json.dumps({'outcome': report['outcome'], 'nested_codesign_commands_passed': len(receipts),
                  'absence_checks': len(paths), 'prior_evidence_unchanged': True}))
