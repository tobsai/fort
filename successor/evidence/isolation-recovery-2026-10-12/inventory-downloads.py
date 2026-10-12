"""Inventory only the accepted downloader's owned public outputs and diagnostics."""
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
app = pathlib.Path(scope['app_root'])
manifest = json.loads((HERE / 'created-paths.json').read_text())
known = {entry['path']: entry for entry in manifest}
allowed_directories = {app, app / 'downloads', app / 'evidence', app / 'evidence/download-diagnostics'}
allowed_files = {}
for index, entry in enumerate(scope['downloads'], 1):
    for suffix in ('', '.partial'):
        allowed_files[app / 'downloads' / (entry['name'] + suffix)] = 'public-download'
    stem = '%02d-%s' % (index, entry['name'])
    suffixes = ['.json', '.json.pending', '.failure.json', '.failure.json.pending']
    if entry['name'] in ('lima.pub', 'ubuntu.pub'):
        suffixes += ['.untrusted.bin', '.untrusted.bin.pending']
    for suffix in suffixes:
        allowed_files[app / 'evidence/download-diagnostics' / (stem + suffix)] = 'download-diagnostic'


def hash_file(path, algorithm='sha256'):
    digest = hashlib.new(algorithm)
    with path.open('rb') as source:
        while True:
            chunk = source.read(1024 * 1024)
            if not chunk:
                break
            digest.update(chunk)
    return digest.hexdigest()


actual = []
for path in [app] + sorted(app.rglob('*')):
    info = path.lstat()
    assert info.st_uid == os.getuid() and info.st_dev == known[str(app)]['device'], str(path)
    assert not info.st_mode & 0o077, str(path)
    if path in allowed_directories:
        assert stat.S_ISDIR(info.st_mode), str(path)
        kind = 'directory'
    else:
        assert path in allowed_files and stat.S_ISREG(info.st_mode), str(path)
        assert info.st_nlink in (1, 2) and not info.st_mode & 0o111, str(path)
        kind = 'regular'
    if str(path) in known:
        old = known[str(path)]
        assert (info.st_uid, info.st_dev, info.st_ino) == (old['uid'], old['device'], old['inode']), str(path)
    item = {'path': str(path), 'type': kind, 'uid': info.st_uid, 'device': info.st_dev,
            'inode': info.st_ino, 'mode': oct(stat.S_IMODE(info.st_mode)),
            'bytes': info.st_size, 'allocated_bytes': info.st_blocks * 512,
            'hard_links': info.st_nlink, 'recorded_at_utc': datetime.datetime.now(datetime.timezone.utc).isoformat()}
    if kind == 'regular':
        item.update({'role': allowed_files[path], 'sha256': hash_file(path)})
    else:
        item['role'] = 'private task-owned directory'
    actual.append(item)
    if str(path) not in known:
        manifest.append(item)
        known[str(path)] = item
(HERE / 'created-paths.json').write_text(json.dumps(manifest, indent=2) + '\n')

stage = json.loads((HERE / 'download-result.json').read_text())
receipts = []
gate_errors = []
expected_downloads = {entry['name'] for entry in scope['downloads']}
observed_downloads = {path.name for path in (app / 'downloads').iterdir()}
if observed_downloads != expected_downloads:
    gate_errors.append('missing/unexpected/partial download names')
if stage['exit_code'] != 0:
    gate_errors.append('downloader exit was not zero')
diagnostics = app / 'evidence/download-diagnostics'
expected_receipts = {'%02d-%s.json' % (index, entry['name']) for index, entry in enumerate(scope['downloads'], 1)}
if {path.name for path in diagnostics.iterdir()} != expected_receipts:
    gate_errors.append('failure, pending, missing or unexpected diagnostic files')
for index, entry in enumerate(scope['downloads'], 1):
    final = app / 'downloads' / entry['name']
    terminal = diagnostics / ('%02d-%s.json' % (index, entry['name']))
    if not final.exists() or not terminal.exists():
        continue
    algorithm = 'sha512' if 'sha512' in entry else 'sha256'
    value = json.loads(terminal.read_text())
    matches = (value.get('schema') == 2 and value.get('phase') == 'terminal' and
               value.get('outcome') == 'accepted' and value.get('failure') is None and
               value.get('input_name') == entry['name'] and value.get('input_url') == entry['url'] and
               value.get('algorithm') == algorithm and value.get('expected_digest') == entry[algorithm] and
               value.get('observed_digest') == entry[algorithm] and
               value.get('observed_bytes') == final.stat().st_size <= entry['max_bytes'] and
               value.get('body_complete') is True and value.get('http', {}).get('status') == 200 and
               value.get('untrusted_body') is None and value.get('publisher_signature_verified') is False and
               hash_file(final, algorithm) == entry[algorithm])
    receipts.append({'name': entry['name'], 'accepted_receipt_and_pin_match': matches,
                     'bytes': final.stat().st_size, algorithm: hash_file(final, algorithm)})
    if not matches:
        gate_errors.append('receipt/input mismatch: ' + entry['name'])
diagnostic_bytes = sum(item['bytes'] for item in actual if item['role'] == 'download-diagnostic')
assert diagnostic_bytes <= scope['download_diagnostics']['aggregate_max_bytes']
allocated = sum(item['allocated_bytes'] for item in actual)
assert allocated <= scope['resources']['auxiliary_budget_bytes']
result = {'time_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'files_and_directories': actual,
          'input_checks': receipts, 'gate_errors': gate_errors,
          'accepted_download_stage': not gate_errors and len(receipts) == 12,
          'diagnostic_bytes': diagnostic_bytes, 'allocated_bytes': allocated,
          'free_disk_bytes': shutil.disk_usage(app).free,
          'publisher_signatures_verified': False}
with (HERE / 'download-inventory.json').open('x') as output:
    json.dump(result, output, indent=2)
    output.write('\n')
print(json.dumps({key: value for key, value in result.items() if key not in ('files_and_directories', 'input_checks')}, indent=2))
