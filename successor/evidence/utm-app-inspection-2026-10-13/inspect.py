"""Approved read-only inspection of the exact UTM artifact; no code launch."""
from pathlib import Path
import hashlib
import json
import os
import plistlib
import shutil
import stat
import subprocess
import sys

HERE = Path(__file__).resolve().parent
WORKTREE = HERE.parents[2]
PRIOR = HERE.parent / 'utm-assessment-2026-10-13'
sys.path.insert(0, str(PRIOR))
import assessment as base
base.HERE = HERE
base.COMMIT = '501dc395c4bf3f7d15d1eb9829a5ad316c844260'
ROOT, DMG, MOUNT = base.ROOT, base.DMG, base.MOUNT
APP = MOUNT / 'UTM.app'
write, run, require, now = base.write, base.run, base.require, base.now


def preflight():
    assert subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=WORKTREE, text=True).strip() == base.COMMIT
    prior_seal = PRIOR / 'seal.json'
    assert base.digest(prior_seal) == 'be08287b55001500b29b1efb4d22cf6312e2ec0a75dd0f7d213090fc9983a8fd'
    for name, pin in json.loads(prior_seal.read_text())['files'].items():
        assert base.digest(PRIOR / name) == pin, name
    assert base.digest(base.PROPOSAL / 'seal.json') == base.SEAL
    for name, pin in json.loads((base.PROPOSAL / 'seal.json').read_text())['files'].items():
        assert base.digest(base.PROPOSAL / name) == pin, name
    base.check_ancestors(ROOT)
    assert all(base.absent(path) for path in base.candidate_paths()), 'Existing/unattributed UTM path'
    disk = shutil.disk_usage(ROOT.parent)
    assert disk.free >= 35 * base.GIB
    ca = Path('/etc/ssl/cert.pem').lstat()
    assert stat.S_ISREG(ca.st_mode) and ca.st_uid == 0 and not ca.st_mode & 0o022
    require(run('host-version.json', ['/usr/bin/sw_vers']))
    assert require(run('host-architecture.json', ['/usr/bin/uname', '-m']))['stdout'].strip() == 'arm64'
    write('authority-and-preflight.json', {
        'time_utc': now(), 'starting_commit': base.COMMIT,
        'prior_assessment_seal_sha256': base.digest(prior_seal),
        'proposal_commit': 'ec6dd956bd6ebf45ec685331625ccc429e59b7f6',
        'proposal_seal_sha256': base.SEAL,
        'approval_reference': 'Sentinel_ea51da06ee408191a8b7ea5327e0f437',
        'approval_date': '2026-10-13',
        'request_reference': 'Sentinel_9e0f867178c481918604946ff36fbd87',
        'clarification_reference': 'Sentinel_16d91dfa308c8191ae23c21f5e29bbd0',
        'scope_change': 'Permit read-only mounting of the same verified DMG solely to inspect the contained app despite the prior outer-container primary-signature rejection.',
        'unchanged_limits': 'Exact source/hash/size, 256 MiB DMG, 2 GiB app, 3 GiB observed task storage and 32 GiB disk reserve. No downloaded executable launch, VM, accounts, keys, permissions, quarantine change or security bypass.',
        'authority_source': 'Delegated owner approval; reference IDs and scope summary only.',
        'candidate_paths_absent': list(map(str, base.candidate_paths())),
        'disk_free_bytes': disk.free, 'task_budget_bytes': 3 * base.GIB,
        'app_cap_bytes': 2 * base.GIB, 'reserve_bytes': 32 * base.GIB,
        'system_ca_regular_root_owned_not_group_or_world_writable': True,
        'inspection_location': 'App assessed in place on read-only mounted volume; no app copy.'})
    for path in (ROOT, ROOT / 'downloads', ROOT / 'mount', MOUNT):
        os.mkdir(path, 0o700)
        with (HERE / 'created-directories.jsonl').open('a') as output:
            output.write(json.dumps(base.record(path)) + '\n')
            output.flush()
            os.fsync(output.fileno())
    print(json.dumps({'preflight': 'passed', 'created_private_directories': 4}))


def verify_dmg():
    assert DMG.lstat().st_size == base.SIZE and base.digest(DMG) == base.PIN
    require(run('dmg-integrity.json', ['/usr/bin/hdiutil', 'verify', str(DMG)], 120))
    require(run('dmg-stapler.json', ['/usr/bin/xcrun', '--no-cache', 'stapler', 'validate', str(DMG)], 120))
    write('outer-container-scope-exception.json', {
        'prior_receipt': '../utm-assessment-2026-10-13/dmg-gatekeeper.json',
        'prior_receipt_sha256': base.digest(PRIOR / 'dmg-gatekeeper.json'),
        'same_artifact_pin_confirmed': True, 'prior_rejection_preserved_not_overridden': True,
        'os_policy_changes': False, 'repeat_outer_container_assessment': False,
        'reason': 'Owner explicitly authorized read-only inspection despite the known outer-container result. App and nested-code acceptance remain separate gates.'})


def attach():
    assert json.loads((HERE / 'dmg-stapler.json').read_text())['exit_code'] == 0
    assert DMG.lstat().st_size == base.SIZE and base.digest(DMG) == base.PIN
    assert MOUNT.is_dir() and not list(MOUNT.iterdir()) and not os.path.ismount(MOUNT)
    assert shutil.disk_usage(ROOT.parent).free >= 32 * base.GIB
    result = require(run('mount-result.json', ['/usr/bin/hdiutil', 'attach', '-readonly',
        '-nobrowse', '-noautoopen', '-mountpoint', str(MOUNT), '-plist', str(DMG)], 90))
    mounted = plistlib.loads(result['stdout'].encode())
    entities = mounted.get('system-entities', [])
    matching = [entry for entry in entities if entry.get('mount-point') == str(MOUNT)]
    assert len(matching) == 1 and os.path.ismount(MOUNT)
    assert os.statvfs(MOUNT).f_flag & os.ST_RDONLY
    devices = [entry['dev-entry'] for entry in entities if 'dev-entry' in entry]
    assert devices and all(device.startswith('/dev/disk') for device in devices)
    write('mounted-identity.json', {'time_utc': now(), 'disk_image': str(DMG),
        'mountpoint': str(MOUNT), 'devices': devices, 'mounted_volume_device': matching[0]['dev-entry'],
        'read_only_statvfs': True, 'non_browsing_requested': True, 'autoopen_disabled': True})
    assert APP.is_dir() and not APP.is_symlink()
    entries = base.inventory(APP, hash_content=False)
    logical = sum(entry['bytes'] for entry in entries if entry['type'] == 'regular')
    allocated = sum(entry['allocated_bytes'] for entry in entries)
    assert logical <= 2 * base.GIB and allocated <= 2 * base.GIB
    evidence_bytes = sum(p.stat().st_size for p in HERE.iterdir() if p.is_file())
    assert base.SIZE + max(logical, allocated) + evidence_bytes < 3 * base.GIB
    for entry in entries:
        if entry['type'] == 'symlink':
            resolved = Path(entry['path']).resolve(strict=True)
            assert resolved == APP or APP in resolved.parents, entry['path']
    write('mounted-app-footprint.json', {'time_utc': now(), 'entries': len(entries),
        'logical_file_bytes': logical, 'allocated_app_bytes': allocated,
        'conservative_dmg_plus_app_plus_evidence_bytes': base.SIZE + max(logical, allocated) + evidence_bytes,
        'actual_app_copy_bytes': 0, 'all_app_symlinks_internal': True,
        'disk_free_bytes': shutil.disk_usage(ROOT.parent).free})
    manifest = base.inventory(APP)
    for entry in manifest:
        entry['path'] = str(Path(entry['path']).relative_to(MOUNT))
    write('mounted-app-manifest.json', manifest)
    print(json.dumps({'mounted_read_only': True, 'app_entries': len(entries), 'logical_app_bytes': logical}))


if __name__ == '__main__':
    assert sys.argv[1:] in (['preflight'], ['download'], ['verify-dmg'], ['attach'])
    {'preflight': preflight, 'download': base.download, 'verify-dmg': verify_dmg, 'attach': attach}[sys.argv[1]]()
