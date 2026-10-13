"""Detach only the recorded image, then remove only manifest-owned staging objects."""
from pathlib import Path
import hashlib
import importlib.util
import json
import os
import plistlib
import re
import shutil
import stat
import subprocess
import sys

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('utm_inspection_cleanup', HERE / 'inspect.py')
inspection = importlib.util.module_from_spec(spec)
spec.loader.exec_module(inspection)
base, ROOT, DMG, MOUNT = inspection.base, inspection.ROOT, inspection.DMG, inspection.MOUNT
write, run, require = inspection.write, inspection.run, inspection.require
known_list = json.loads((HERE / 'download-manifest.json').read_text())
known = {entry['path']: entry for entry in known_list}
assert len(known) == len(known_list) == 5
assert all(Path(path) == ROOT or ROOT in Path(path).parents for path in known)
assert sys.argv[1:] in (['detach'], ['remove'])


def image_state():
    result = subprocess.run(['/usr/bin/hdiutil', 'info', '-plist'], stdin=subprocess.DEVNULL,
                            capture_output=True, close_fds=True, timeout=30, check=True)
    assert len(result.stdout) + len(result.stderr) < 2 * 1024**2
    info = plistlib.loads(result.stdout)
    matches = []
    for image in info.get('images', []):
        mounts = [entity.get('mount-point', '') for entity in image.get('system-entities', [])]
        if str(ROOT) in image.get('image-path', '') or str(MOUNT) in mounts:
            matches.append({'image_path': image.get('image-path'),
                            'system_entities': image.get('system-entities', [])})
    return matches


def task_processes():
    result = subprocess.run(['/bin/ps', '-axo', 'pid=,comm='], stdin=subprocess.DEVNULL,
                            capture_output=True, timeout=10, check=True)
    matching = [line for line in result.stdout.decode().splitlines()
                if str(ROOT) in line or '/UTM.app/Contents/' in line]
    assert not matching
    return matching


def candidate_paths():
    signatures = json.loads((HERE / 'nested-signatures-and-entitlements.json').read_text())
    bundle_ids, groups = set(), set()
    for item in signatures['files']:
        for signature in item['signatures']:
            ent = signature['entitlements']
            if ent.get('com.apple.security.app-sandbox'):
                identifier = signature['identifier']
                assert re.fullmatch(r'[A-Za-z0-9.-]+', identifier)
                bundle_ids.add(identifier)
            for group in ent.get('com.apple.security.application-groups', []):
                assert re.fullmatch(r'[A-Za-z0-9.-]+', group)
                groups.add(group)
    library = Path('/Users/tobiasgunn/Library')
    paths = [library / 'Group Containers' / group for group in sorted(groups)]
    for identifier in sorted(bundle_ids):
        paths.extend([library / 'Containers' / identifier,
            library / 'Preferences' / (identifier + '.plist'),
            library / 'Caches' / identifier,
            library / 'Saved Application State' / (identifier + '.savedState')])
    return paths


def detach():
    assert os.path.ismount(MOUNT) and os.statvfs(MOUNT).f_flag & os.ST_RDONLY
    recorded = json.loads((HERE / 'mounted-identity.json').read_text())
    current = image_state()
    assert len(current) == 1 and current[0]['image_path'] == str(DMG)
    devices = {entity['dev-entry'] for entity in current[0]['system_entities'] if 'dev-entry' in entity}
    assert devices == set(recorded['devices'])
    whole = [device for device in devices if re.fullmatch(r'/dev/disk[0-9]+', device)]
    assert len(whole) == 1
    paths = candidate_paths()
    write('candidate-state-paths-before-cleanup.json', {'time_utc': base.now(),
        'derivation': 'Signed identifiers of sandboxed code plus declared app groups. Preferences/cache/saved-state are candidate names, not a complete first-launch footprint.',
        'paths': [{'path': str(path), 'present': os.path.lexists(path)} for path in paths],
        'existing_contents_opened': False})
    assert all(base.absent(path) for path in paths), 'Existing state candidate; never delete it'
    opened = run('pre-detach-open-files.json', ['/usr/sbin/lsof', '-nP', '-t', '+D', str(MOUNT)], 45)
    assert opened['exit_code'] == 1 and not opened['stdout'] and not opened['stderr']
    processes = task_processes()
    assert DMG.stat().st_size == base.SIZE and base.digest(DMG) == base.PIN
    before_manifest = json.loads((HERE / 'mounted-app-manifest.json').read_text())
    after_manifest = base.inventory(inspection.APP)
    for entry in after_manifest:
        entry['path'] = str(Path(entry['path']).relative_to(MOUNT))
    assert before_manifest == after_manifest, 'Read-only app inventory changed'
    write('pre-detach-validation.json', {'time_utc': base.now(), 'matching_owned_image': current[0],
        'exact_whole_disk_to_detach': whole[0], 'read_only_statvfs': True,
        'app_manifest_identities_modes_hashes_links_unchanged': True,
        'app_manifest_entries': len(after_manifest), 'matching_task_or_utm_app_processes': processes,
        'dmg_pin_unchanged': True, 'no_open_files_on_mounted_volume': True})
    require(run('detach-result.json', ['/usr/bin/hdiutil', 'detach', whole[0]], 60))
    assert not os.path.ismount(MOUNT) and not image_state()
    write('post-detach-validation.json', {'time_utc': base.now(), 'matching_task_images': [],
        'task_mountpoint_is_mount': False, 'mountpoint_present': os.path.lexists(MOUNT),
        'force_detach_used': False})
    print(json.dumps({'detached': whole[0], 'matching_task_images': 0}), flush=True)


def remove():
    assert json.loads((HERE / 'detach-result.json').read_text())['exit_code'] == 0
    assert not image_state() and not os.path.ismount(MOUNT)
    processes = task_processes()
    opened = run('pre-remove-open-files.json', ['/usr/sbin/lsof', '-nP', '-t', '+D', str(ROOT)], 30)
    assert opened['exit_code'] == 1 and not opened['stdout'] and not opened['stderr']
    base.check_ancestors(ROOT)
    descriptors, actual = {}, {}
    missing_mount = base.absent(MOUNT)
    if missing_mount:
        assert json.loads((HERE / 'post-detach-validation.json').read_text())['mountpoint_present'] is False

    def identity(path, info):
        item = known[str(path)]
        assert (info.st_uid, info.st_dev, info.st_ino) == (item['uid'], item['device'], item['inode']), str(path)
        assert info.st_uid == os.getuid() and stat.S_IMODE(info.st_mode) == int(item['mode'], 8)
        assert not info.st_mode & 0o077

    def inspect(path, fd):
        identity(path, os.fstat(fd))
        descriptors[path] = fd
        actual[str(path)] = 'directory'
        for name in sorted(os.listdir(fd)):
            child = path / name
            assert str(child) in known, 'Unattributed path: ' + str(child)
            item = known[str(child)]
            info = os.stat(name, dir_fd=fd, follow_symlinks=False)
            identity(child, info)
            if stat.S_ISDIR(info.st_mode):
                assert item['type'] == 'directory'
                inspect(child, os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=fd))
            else:
                assert item['type'] == 'regular' and stat.S_ISREG(info.st_mode) and info.st_nlink == 1
                assert info.st_size == item['bytes']
                file_fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW, dir_fd=fd)
                hasher = hashlib.sha256()
                with os.fdopen(file_fd, 'rb') as source:
                    identity(child, os.fstat(source.fileno()))
                    for chunk in iter(lambda: source.read(1024**2), b''):
                        hasher.update(chunk)
                assert hasher.hexdigest() == item['sha256']
                actual[str(child)] = 'regular'

    parent = os.open(ROOT.parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        inspect(ROOT, os.open(ROOT.name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=parent))
        expected = set(known) - ({str(MOUNT)} if missing_mount else set())
        assert set(actual) == expected
        evidence = sum(p.stat().st_size for p in HERE.iterdir() if p.is_file())
        footprint = json.loads((HERE / 'mounted-app-footprint.json').read_text())
        conservative_total = base.SIZE + max(footprint['logical_file_bytes'], footprint['allocated_app_bytes']) + evidence
        assert conservative_total < 3 * base.GIB and evidence < 64 * 1024**2
        assert shutil.disk_usage(ROOT.parent).free >= 32 * base.GIB
        write('cleanup-preflight.json', {'time_utc': base.now(), 'owned_entries': len(known),
            'entries_present_for_removal': len(actual), 'mountpoint_removed_by_detach': missing_mount,
            'all_present_identities_modes_and_file_hashes_verified': True,
            'matching_task_images': [], 'matching_task_or_utm_app_processes': processes,
            'evidence_bytes_at_check': evidence, 'conservative_dmg_app_evidence_bytes': conservative_total,
            'disk_free_bytes': shutil.disk_usage(ROOT.parent).free})
        with (HERE / 'cleanup-journal.jsonl').open('x') as journal:
            if missing_mount:
                journal.write(json.dumps({'time_utc': base.now(), 'path': str(MOUNT), 'removed': True,
                    'method': 'normal hdiutil detach removed its recorded mountpoint'}) + '\n')
                journal.flush()
                os.fsync(journal.fileno())
            for path in sorted(map(Path, actual), key=lambda p: (-len(p.parts), str(p))):
                fd = parent if path == ROOT else descriptors[path.parent]
                identity(path, os.stat(path.name, dir_fd=fd, follow_symlinks=False))
                if actual[str(path)] == 'directory':
                    assert not os.listdir(descriptors[path])
                    os.rmdir(path.name, dir_fd=fd)
                else:
                    os.unlink(path.name, dir_fd=fd)
                os.fsync(fd)
                journal.write(json.dumps({'time_utc': base.now(), 'path': str(path), 'removed': True,
                                         'method': 'verified no-follow directory descriptor'}) + '\n')
                journal.flush()
                os.fsync(journal.fileno())
        paths = [Path(path) for path in known] + candidate_paths() + base.candidate_paths()[:3]
        assert all(base.absent(path) for path in paths)
        write('cleanup-result.json', {'time_utc': base.now(), 'owned_objects_removed': len(known),
            'absent_paths': list(map(str, sorted(set(paths)))), 'task_images_attached': 0,
            'app_was_not_copied_or_launched': True, 'mount_detached_without_force': True,
            'shared_os_databases_not_edited': True, 'disk_free_bytes': shutil.disk_usage(ROOT.parent).free})
        print(json.dumps({'owned_objects_removed': len(known), 'task_root_absent': base.absent(ROOT)}))
    finally:
        for fd in descriptors.values():
            os.close(fd)
        os.close(parent)


{'detach': detach, 'remove': remove}[sys.argv[1]]()
