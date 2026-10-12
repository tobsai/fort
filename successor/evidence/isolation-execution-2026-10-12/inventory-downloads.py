"""Inventory only the task-owned downloader output; never read credentials."""
import datetime
import hashlib
import json
import os
import pathlib
import shutil
import stat

HERE = pathlib.Path(__file__).resolve().parent
PACKET = HERE.parents[2] / 'successor/ops/isolation-probe-v1'
scope = json.loads((PACKET / 'scope.json').read_text())
root = pathlib.Path(scope['app_root'])
manifest = json.loads((HERE / 'created-paths.json').read_text())
known = {entry['path']: entry for entry in manifest}
for path in (root, root / 'downloads'):
    info = path.lstat()
    expected = known[str(path)]
    assert stat.S_ISDIR(info.st_mode) and info.st_uid == expected['uid'] == os.getuid()
    assert (info.st_dev, info.st_ino) == (expected['device'], expected['inode'])
    assert stat.S_IMODE(info.st_mode) == 0o700
pins = {entry['name']: entry for entry in scope['downloads']}
actual = []
for path in sorted((root / 'downloads').iterdir()):
    info = path.lstat()
    assert stat.S_ISREG(info.st_mode) and info.st_uid == os.getuid() and info.st_nlink == 1
    assert not info.st_mode & 0o077 and path.name in pins
    pin = pins[path.name]
    algorithm = 'sha512' if 'sha512' in pin else 'sha256'
    digest = hashlib.new(algorithm)
    with path.open('rb') as source:
        while True:
            chunk = source.read(1024 * 1024)
            if not chunk:
                break
            digest.update(chunk)
    receipt = {'path': str(path), 'name': path.name, 'type': 'regular', 'uid': info.st_uid,
               'device': info.st_dev, 'inode': info.st_ino, 'mode': oct(stat.S_IMODE(info.st_mode)),
               'hard_links': info.st_nlink, 'bytes': info.st_size,
               'allocated_bytes': info.st_blocks * 512, algorithm: digest.hexdigest(),
               'within_byte_cap': info.st_size <= pin['max_bytes'],
               'matches_pin': digest.hexdigest() == pin[algorithm],
               'recorded_at_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
               'role': pin['kind']}
    actual.append(receipt)
    assert str(path) not in known
    manifest.append(receipt)
result = {'files': actual, 'missing': sorted(set(pins) - {item['name'] for item in actual}),
          'allocated_bytes': sum(item['allocated_bytes'] for item in actual),
          'logical_bytes': sum(item['bytes'] for item in actual),
          'free_disk_bytes': shutil.disk_usage(root).free,
          'all_present_and_pinned': len(actual) == len(pins) and all(item['within_byte_cap'] and item['matches_pin'] for item in actual)}
with (HERE / 'download-inventory.json').open('x') as output:
    json.dump(result, output, indent=2)
    output.write('\n')
(HERE / 'created-paths.json').write_text(json.dumps(manifest, indent=2) + '\n')
print(json.dumps({key: value for key, value in result.items() if key != 'files'}))
