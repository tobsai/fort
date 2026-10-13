"""Remove only the seven recorded pre-mount objects after the required stop."""
from pathlib import Path
import hashlib
import json
import os
import plistlib
import shutil
import stat
import subprocess
import sys
sys.path.insert(0, str(Path(__file__).resolve().parent))
from assessment import HERE, ROOT, DMG, MOUNT, APP, GIB, candidate_paths, absent, check_ancestors, now, run, write

assert sys.argv[1:] in (['inspect'], ['remove'])
mode = sys.argv[1]
known_list = json.loads((HERE / 'download-manifest.json').read_text())
known = {entry['path']: entry for entry in known_list}
assert len(known) == len(known_list) == 7
assert sum(entry['type'] == 'regular' for entry in known_list) == 1
assert known[str(DMG)]['type'] == 'regular'
assert all(Path(path) == ROOT or ROOT in Path(path).parents for path in known)
gate = json.loads((HERE / 'dmg-gatekeeper.json').read_text())
assert gate['exit_code'] == 3 and 'source=no usable signature' in gate['stderr']
assert absent(APP)
assert not (HERE / 'mount-result.json').exists()

# Avoid retaining unrelated mounted-image names or process information.
images = subprocess.run(['/usr/bin/hdiutil', 'info', '-plist'], capture_output=True,
                        stdin=subprocess.DEVNULL, close_fds=True, timeout=30)
assert len(images.stdout) + len(images.stderr) < 2 * 1024**2
assert images.returncode == 0, 'Cannot establish mount state'
info = plistlib.loads(images.stdout)
matches = []
for image in info.get('images', []):
    image_path = image.get('image-path', '')
    mountpoints = [entity.get('mount-point', '') for entity in image.get('system-entities', [])]
    if image_path == str(DMG) or image_path.startswith(str(ROOT) + '/') or str(MOUNT) in mountpoints:
        matches.append({'image_path': image_path, 'mountpoints': mountpoints})
assert not matches and not os.path.ismount(MOUNT), 'Owned disk image is still attached'
write('cleanup-' + mode + '-mount-check.json', {
    'time_utc': now(), 'command': ['/usr/bin/hdiutil', 'info', '-plist'],
    'exit_code': images.returncode, 'matching_task_images': matches,
    'task_mountpoint_is_mount': False, 'unrelated_image_metadata_not_retained': True,
    'detach_required': False, 'reason': 'Assessment stopped before any attach command.'})
open_files = run('cleanup-' + mode + '-open-files.json', ['/usr/sbin/lsof', '-nP', '-t', '+D', str(ROOT)], 30)
assert open_files['exit_code'] == 1 and not open_files['stdout'] and not open_files['stderr']
processes = subprocess.run(['/bin/ps', '-axo', 'pid=,comm='], capture_output=True, timeout=10, check=True)
matching_processes = [line for line in processes.stdout.decode().splitlines()
                     if str(ROOT) in line or '/UTM.app/Contents/' in line]
assert not matching_processes
check_ancestors(ROOT)

descriptors = {}
actual = {}


def verify_identity(path, info):
    entry = known[str(path)]
    assert (info.st_uid, info.st_dev, info.st_ino) == (entry['uid'], entry['device'], entry['inode']), str(path)
    assert stat.S_IMODE(info.st_mode) == int(entry['mode'], 8), str(path)
    assert info.st_uid == os.getuid() and not info.st_mode & 0o077, str(path)


def inspect(path, fd):
    verify_identity(path, os.fstat(fd))
    descriptors[path] = fd
    actual[str(path)] = 'directory'
    for name in sorted(os.listdir(fd)):
        child = path / name
        assert str(child) in known, 'Unattributed object: ' + str(child)
        entry = known[str(child)]
        info = os.stat(name, dir_fd=fd, follow_symlinks=False)
        verify_identity(child, info)
        if stat.S_ISDIR(info.st_mode):
            assert entry['type'] == 'directory'
            child_fd = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=fd)
            inspect(child, child_fd)
        else:
            assert stat.S_ISREG(info.st_mode) and entry['type'] == 'regular'
            assert info.st_nlink == 1 and info.st_size == entry['bytes']
            file_fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW, dir_fd=fd)
            hasher = hashlib.sha256()
            with os.fdopen(file_fd, 'rb') as source:
                verify_identity(child, os.fstat(source.fileno()))
                for chunk in iter(lambda: source.read(1024**2), b''):
                    hasher.update(chunk)
            assert hasher.hexdigest() == entry['sha256']
            actual[str(child)] = 'regular'


parent = os.open(ROOT.parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
try:
    root_fd = os.open(ROOT.name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=parent)
    inspect(ROOT, root_fd)
    assert set(actual) == set(known)
    evidence_bytes = sum(p.lstat().st_size for p in HERE.rglob('*') if p.is_file())
    logical_bytes = sum(entry['bytes'] for entry in known_list if entry['type'] == 'regular')
    allocated_bytes = sum(entry['allocated_bytes'] for entry in known_list)
    disk = shutil.disk_usage(ROOT.parent)
    assert max(logical_bytes, allocated_bytes) + evidence_bytes < 3 * GIB
    assert evidence_bytes < 64 * 1024**2 and disk.free >= 32 * GIB
    write('cleanup-' + mode + '-preflight.json', {
        'time_utc': now(), 'owned_entries': len(actual), 'regular_files': 1, 'directories': 6,
        'all_path_identities_modes_and_file_hashes_verified': True,
        'matching_task_or_utm_app_processes': matching_processes,
        'logical_file_bytes': logical_bytes, 'allocated_task_bytes': allocated_bytes,
        'retained_evidence_bytes_at_check': evidence_bytes, 'disk_free_bytes': disk.free,
        'budget_bytes': 3 * GIB, 'reserve_bytes': 32 * GIB,
        'app_never_staged': True, 'mount_never_attached': True,
        'downloaded_code_never_executed': True})
    if mode == 'remove':
        with (HERE / 'cleanup-journal.jsonl').open('x') as journal:
            for path in sorted(map(Path, known), key=lambda p: (-len(p.parts), str(p))):
                fd = parent if path == ROOT else descriptors[path.parent]
                verify_identity(path, os.stat(path.name, dir_fd=fd, follow_symlinks=False))
                if known[str(path)]['type'] == 'directory':
                    assert not os.listdir(descriptors[path])
                    os.rmdir(path.name, dir_fd=fd)
                else:
                    os.unlink(path.name, dir_fd=fd)
                os.fsync(fd)
                journal.write(json.dumps({'time_utc': now(), 'path': str(path),
                                         'type': known[str(path)]['type'], 'removed': True}) + '\n')
                journal.flush()
                os.fsync(journal.fileno())
        assert all(absent(Path(path)) for path in known)
        assert all(absent(path) for path in candidate_paths())
        write('cleanup-result.json', {'time_utc': now(), 'removed_owned_entries': len(actual),
            'recorded_owned_paths_absent': list(known), 'candidate_paths_absent': list(map(str, candidate_paths())),
            'detach_required': False, 'shared_os_databases_not_edited': True,
            'disk_free_bytes_after': shutil.disk_usage(ROOT.parent).free})
    print(json.dumps({'mode': mode, 'verified_owned_entries': len(actual), 'root_absent': absent(ROOT)}))
finally:
    for fd in descriptors.values():
        os.close(fd)
    os.close(parent)
