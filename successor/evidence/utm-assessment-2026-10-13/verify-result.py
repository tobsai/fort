"""Independent read-only rescan of absence and retained assessment receipts."""
import datetime
import hashlib
import json
import os
from pathlib import Path
import plistlib
import subprocess

HERE = Path(__file__).resolve().parent
WORKTREE = HERE.parents[2]
ROOT = Path('/Users/tobiasgunn/Library/Application Support/FortUTMIsolation')
APP = ROOT / 'tools/UTM-4.7.5/UTM.app'
COMMIT = 'ec6dd956bd6ebf45ec685331625ccc429e59b7f6'
os.umask(0o077)

manifest = json.loads((HERE / 'download-manifest.json').read_text())
assert len(manifest) == 7
paths = [Path(entry['path']) for entry in manifest] + [APP,
    Path('/Applications/UTM.app'), Path('/Users/tobiasgunn/Applications/UTM.app'),
    Path('/Users/tobiasgunn/Library/Containers/com.utmapp.UTM')]
assert len(set(paths)) == 11
assert all(not os.path.lexists(path) for path in paths)
journal = [json.loads(line) for line in (HERE / 'cleanup-journal.jsonl').read_text().splitlines()]
assert len(journal) == 7 and all(entry['removed'] for entry in journal)
assert {entry['path'] for entry in manifest} == {entry['path'] for entry in journal}
images = subprocess.run(['/usr/bin/hdiutil', 'info', '-plist'], stdin=subprocess.DEVNULL,
                        capture_output=True, timeout=30, check=True)
assert len(images.stdout) + len(images.stderr) < 2 * 1024**2
image_info = plistlib.loads(images.stdout)
matching_images = []
for image in image_info.get('images', []):
    if str(ROOT) in image.get('image-path', '') or any(str(ROOT) in e.get('mount-point', '')
                                                     for e in image.get('system-entities', [])):
        matching_images.append(True)
assert not matching_images
processes = subprocess.run(['/bin/ps', '-axo', 'pid=,comm='], capture_output=True, timeout=10, check=True)
matching_processes = [line for line in processes.stdout.decode().splitlines()
                     if str(ROOT) in line or '/UTM.app/Contents/' in line]
assert not matching_processes
receipt = json.loads((HERE / '01-UTM.dmg.json').read_text())
assert receipt['outcome'] == 'accepted' and receipt['body_complete']
assert receipt['observed_bytes'] == 250021057
assert receipt['expected_digest'] == receipt['observed_digest'] == 'a8435c93cfb5f8bbfeea4b134cfad1ac66b67632b75e438c63b1a8ae043bef0e'
assert json.loads((HERE / 'dmg-integrity.json').read_text())['exit_code'] == 0
assert json.loads((HERE / 'dmg-stapler.json').read_text())['exit_code'] == 0
gate = json.loads((HERE / 'dmg-gatekeeper.json').read_text())
assert gate['exit_code'] == 3 and gate['failure'] is None
assert 'rejected\nsource=no usable signature' in gate['stderr']
proposal = WORKTREE / 'successor/ops/utm-isolation-proposal-v1'
seal_data = (proposal / 'seal.json').read_bytes()
assert hashlib.sha256(seal_data).hexdigest() == '99decb7c28b2c2daba3972709642a12ed4d3cce084a40bc3c3dd3e0008899159'
for name, pin in json.loads(seal_data)['files'].items():
    content = (proposal / name).read_bytes()
    assert hashlib.sha256(content).hexdigest() == pin
    assert content == subprocess.check_output(['git', 'show', COMMIT + ':successor/ops/utm-isolation-proposal-v1/' + name], cwd=WORKTREE)
assert not subprocess.check_output(['git', 'diff', '--name-only'], cwd=WORKTREE).strip()
report = {
    'time_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'outcome': 'stopped-at-dmg-primary-signature-rejection-and-owned-cleanup-complete',
    'approved_proposal_commit': COMMIT,
    'passed': ['proposal seal and eight files match approved commit',
        'fresh private roots and disk reserve preflight', 'one exact official HTTPS download',
        'exact 250021057-byte size and SHA256 pin', 'hdiutil disk-image integrity verification',
        'stapled notarization-ticket validation', 'seven owned objects identity/mode/hash verification',
        'seven owned objects removed with durable journal', 'independent eleven-path absence rescan',
        'independent no-task-image and no-UTM-app-process rescan'],
    'failed': [{'check': 'Gatekeeper DMG open assessment with context:primary-signature',
                'exit_code': 3, 'source': 'no usable signature', 'outside_command_sandbox': True}],
    'unrun': ['read-only DMG attach and app staging', 'actual app Developer ID/certificate/Team ID',
        'app version/build confirmation', 'app and nested-code codesign verification',
        'app Gatekeeper execute assessment', 'entitlement and legal-file inspection',
        'derivation and presence checks of signed group-container identifiers',
        'all application/helper/verifier execution', 'all VM and provider activity'],
    'owned_entries_removed': 7, 'independently_absent_paths': list(map(str, paths)),
    'matching_task_images': [], 'matching_task_or_utm_app_processes': [],
    'detach_required': False, 'retained_evidence_bytes_at_check': sum(p.lstat().st_size for p in HERE.rglob('*') if p.is_file()),
    'not_a_claim': 'DMG rejection is not a measured app rejection. A valid DMG staple is not completed app/nested-code acceptance. No OS isolation or product/provider readiness is established.',
    'next_decision': 'Review whether to authorize read-only mounting of the same pinned DMG solely to inspect its contained app despite this outer-container assessment result. That changes the explicit stop condition; no mount or app launch is authorized by this report.',
    'prohibited_actions_not_performed': ['launching downloaded executables', 'LaunchServices registration/open',
        'VM creation', 'keys/accounts/provider sign-in', 'permission/security/quarantine changes',
        'startup services', 'additional downloads', 'shared OS database edits', 'push/merge/deploy']}
with (HERE / 'validation.json').open('x') as output:
    json.dump(report, output, indent=2)
    output.write('\n')
print(json.dumps({'outcome': report['outcome'], 'absence_checks': len(paths), 'proposal_unchanged': True}))
