"""Extract only the inspected registry-verified public archive; run no hooks."""
import datetime
import hashlib
import json
import os
import pathlib
import stat
import tarfile

HERE = pathlib.Path(__file__).resolve().parent
APP = pathlib.Path('/Users/tobiasgunn/Library/Application Support/FortIsolation')
INSTALL = APP / 'tools/openpgp/6.3.2'
manifest_path = HERE / 'created-paths.json'
manifest = json.loads(manifest_path.read_text())
known = {entry['path']: entry for entry in manifest}
inspection = json.loads((HERE / 'openpgp-archive-inspection.json').read_text())
entries = {entry['path']: entry for entry in inspection['entries']}
registry = json.loads((HERE / 'registry-stdout.txt').read_text())
assert json.loads((HERE / 'registry-result.json').read_text())['exit_code'] == 0
assert registry['registry']['registrySignaturesVerified'] == 2
assert registry['verifierArchiveVerified'] is True
assert len(entries) == inspection['members'] == 43
assert inspection['expanded_bytes'] == 17421480 < 24 * 1024**2
os.umask(0o077)


def check_parent(path):
    old = known[str(path)]
    info = path.lstat()
    assert stat.S_ISDIR(info.st_mode) and stat.S_IMODE(info.st_mode) == 0o700
    assert (info.st_uid, info.st_dev, info.st_ino) == (old['uid'], old['device'], old['inode'])


def record(path, role, digest=None):
    info = path.lstat()
    entry = {'path': str(path), 'type': 'directory' if stat.S_ISDIR(info.st_mode) else 'regular',
             'uid': info.st_uid, 'device': info.st_dev, 'inode': info.st_ino,
             'mode': oct(stat.S_IMODE(info.st_mode)), 'bytes': info.st_size,
             'allocated_bytes': info.st_blocks * 512, 'hard_links': info.st_nlink,
             'role': role, 'recorded_at_utc': datetime.datetime.now(datetime.timezone.utc).isoformat()}
    if digest:
        entry['sha256'] = digest
    manifest.append(entry)
    known[str(path)] = entry
    with manifest_path.open('w') as output:
        json.dump(manifest, output, indent=2)
        output.write('\n')
        output.flush()
        os.fsync(output.fileno())


check_parent(APP)
archive = APP / 'downloads/openpgp-6.3.2.tgz'
data = archive.read_bytes()
assert hashlib.sha512(data).hexdigest() == 'c1c653cc7cf8d4b57c638f331ff2650f5253f1874d9a958eb8c02343fa6876fbc2c2c8c1114dfb2b7ce7261e1441c8f7864b04e73f53cef85b51bc33b06be52d'
with tarfile.open(archive, 'r:gz') as source:
    members = source.getmembers()
    assert len(members) == len(entries)
    for member in members:
        path = pathlib.PurePosixPath(member.name)
        assert member.isfile() and not member.mode & 0o6000
        assert not path.is_absolute() and '..' not in path.parts and path.parts[0] == 'package'
        assert member.name == str(path) and member.name in entries
        payload = source.extractfile(member).read()
        assert len(payload) == entries[member.name]['bytes']
        assert hashlib.sha256(payload).hexdigest() == entries[member.name]['sha256']
    directories = {APP / 'tools', APP / 'tools/openpgp', INSTALL}
    for name in entries:
        parent = (INSTALL / name).parent
        while parent != INSTALL:
            directories.add(parent)
            parent = parent.parent
    for directory in sorted(directories, key=lambda value: (len(value.parts), str(value))):
        check_parent(directory.parent)
        directory.mkdir(mode=0o700)
        record(directory, 'private inspected verifier directory')
    for member in members:
        target = INSTALL / member.name
        check_parent(target.parent)
        payload = source.extractfile(member).read()
        descriptor = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        with os.fdopen(descriptor, 'wb') as output:
            output.write(payload)
            output.flush()
            os.fsync(output.fileno())
        record(target, 'inspected public verifier file', hashlib.sha256(payload).hexdigest())
        assert target.read_bytes() == payload
result = {'time_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
          'files': len(members), 'expanded_bytes': inspection['expanded_bytes'],
          'all_file_hashes_match_inspection': True, 'legal_files_preserved': ['package/LICENSE'],
          'hooks_or_builds_run': False, 'dependencies_installed': False,
          'downloaded_code_executed': False, 'install': str(INSTALL)}
with (HERE / 'verifier-extraction.json').open('x') as output:
    json.dump(result, output, indent=2)
    output.write('\n')
print(json.dumps(result))
