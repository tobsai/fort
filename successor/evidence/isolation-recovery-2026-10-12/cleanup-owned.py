"""Check then remove only manifest-owned pre-VM artifacts; never follow links."""
import datetime
import hashlib
import json
import os
import pathlib
import stat
import subprocess
import sys

HERE = pathlib.Path(__file__).resolve().parent
APP = pathlib.Path('/Users/tobiasgunn/Library/Application Support/FortIsolation')
STATE_PARENT = pathlib.Path('/Users/tobiasgunn/Library/FortIsolation')
assert sys.argv[1:] in (['inspect'], ['remove'])
mode = sys.argv[1]
entries = json.loads((HERE / 'created-paths.json').read_text())
known = {entry['path']: entry for entry in entries}
assert len(known) == len(entries) == 298
assert all(pathlib.Path(name) == APP or APP in pathlib.Path(name).parents for name in known)
assert not os.path.lexists(STATE_PARENT)
assert json.loads((HERE / 'gatekeeper-confirmation.json').read_text())['exit_code'] == 3
assert not (HERE / 'manager-start-result.json').exists()
os.umask(0o077)


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


checks = []
for argv in [
    ['/usr/sbin/lsof', '-nP', '-iTCP:22058', '-sTCP:LISTEN'],
    ['/usr/sbin/lsof', '-t', '+D', str(APP)],
]:
    result = subprocess.run(argv, stdin=subprocess.DEVNULL, capture_output=True, timeout=30)
    item = {'command': argv, 'exit_code': result.returncode,
            'stdout': result.stdout.decode(), 'stderr': result.stderr.decode()}
    checks.append(item)
    assert result.returncode == 1 and not result.stdout and not result.stderr, item
processes = subprocess.run(['/bin/ps', '-axo', 'pid=,comm='], capture_output=True, timeout=10, check=True)
matching = [line for line in processes.stdout.decode().splitlines() if '/FortIsolation/' in line]
assert not matching

descriptors = {}
actual = {}
events = []


def verify_identity(path, info):
    entry = known[str(path)]
    assert (info.st_uid, info.st_dev, info.st_ino) == (entry['uid'], entry['device'], entry['inode']), str(path)
    assert stat.S_IMODE(info.st_mode) == int(entry['mode'], 8), str(path)


def inspect_directory(path, descriptor):
    verify_identity(path, os.fstat(descriptor))
    descriptors[path] = descriptor
    actual[str(path)] = {'type': 'directory', 'bytes': 0, 'allocated_bytes': 0}
    for name in sorted(os.listdir(descriptor)):
        target = path / name
        assert str(target) in known, 'Unattributed path: ' + str(target)
        entry = known[str(target)]
        info = os.stat(name, dir_fd=descriptor, follow_symlinks=False)
        verify_identity(target, info)
        if stat.S_ISDIR(info.st_mode):
            assert entry['type'] == 'directory'
            child = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=descriptor)
            inspect_directory(target, child)
            continue
        assert info.st_nlink == 1, str(target)
        if stat.S_ISREG(info.st_mode):
            assert entry['type'] == 'regular' and info.st_size == entry['bytes']
            digest = hashlib.sha256()
            fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW, dir_fd=descriptor)
            with os.fdopen(fd, 'rb') as source:
                verify_identity(target, os.fstat(source.fileno()))
                for chunk in iter(lambda: source.read(1024 * 1024), b''):
                    digest.update(chunk)
            assert digest.hexdigest() == entry['sha256'], str(target)
        else:
            assert stat.S_ISLNK(info.st_mode) and entry['type'] == 'symlink'
            assert target == APP / 'tools/lima/2.2.1/share/doc/lima/templates'
            assert os.readlink(name, dir_fd=descriptor) == entry['link_target'] == '../../lima/templates'
        actual[str(target)] = {'type': entry['type'], 'bytes': info.st_size,
                               'allocated_bytes': info.st_blocks * 512}


for ancestor in reversed(APP.parents):
    assert stat.S_ISDIR(ancestor.lstat().st_mode), str(ancestor)
parent = os.open(APP.parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
root = os.open(APP.name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=parent)
try:
    inspect_directory(APP, root)
    assert set(actual) == set(known)
    logical = sum(row['bytes'] for row in actual.values())
    allocated = sum(row['allocated_bytes'] for row in actual.values())
    assert logical < 4 * 1024**3 and allocated < 4 * 1024**3
    copied_evidence = sum(p.lstat().st_size for p in HERE.rglob('*') if p.is_file())
    raw_evidence = sum(row['bytes'] for name, row in actual.items() if '/evidence/' in name)
    assert copied_evidence + raw_evidence < 128 * 1024**2
    for path in (APP / 'evidence/download-diagnostics').iterdir():
        copy = HERE / 'download-receipts' / path.name
        assert hashlib.sha256(copy.read_bytes()).hexdigest() == known[str(path)]['sha256']
    inspection = {'time_utc': now(), 'checks': checks, 'matching_task_processes': matching,
                  'state_parent_absent': True, 'owned_entries': len(actual),
                  'regular_files': sum(row['type'] == 'regular' for row in actual.values()),
                  'directories': len(descriptors), 'internal_documentation_links': 1,
                  'all_ownership_modes_contents_and_receipt_copies_verified': True,
                  'logical_bytes': logical, 'allocated_bytes': allocated,
                  'raw_plus_copied_evidence_bytes': raw_evidence + copied_evidence,
                  'manager_was_not_launched': True, 'fresh_keys_created': False}
    with (HERE / ('cleanup-' + mode + '-preflight.json')).open('x') as output:
        json.dump(inspection, output, indent=2)
        output.write('\n')
    print(json.dumps(inspection))
    if mode == 'remove':
        journal = (HERE / 'cleanup-journal.jsonl').open('x')
        try:
            for path in sorted((pathlib.Path(name) for name in known), key=lambda p: (-len(p.parts), str(p))):
                directory = parent if path == APP else descriptors[path.parent]
                info = os.stat(path.name, dir_fd=directory, follow_symlinks=False)
                verify_identity(path, info)
                if known[str(path)]['type'] == 'directory':
                    assert not os.listdir(descriptors[path])
                    os.rmdir(path.name, dir_fd=directory)
                else:
                    os.unlink(path.name, dir_fd=directory)
                os.fsync(directory)
                event = {'time_utc': now(), 'path': str(path), 'type': known[str(path)]['type'], 'removed': True}
                journal.write(json.dumps(event) + '\n')
                journal.flush()
                os.fsync(journal.fileno())
                events.append(event)
        finally:
            journal.close()
        assert len(events) == 298 and not os.path.lexists(APP)
        print(json.dumps({'removed_owned_entries': len(events), 'task_root_absent': True}))
finally:
    for descriptor in descriptors.values():
        os.close(descriptor)
    os.close(parent)
