"""Read-only source-closure check. Does not import or execute the probe components."""
import ast
import hashlib
import json
from pathlib import Path
import stat

root = Path(__file__).resolve().parent
seal = json.loads((root / 'seal.json').read_bytes())
expected = {entry['path']: entry for entry in seal['files']}
actual = set()
for path in root.rglob('*'):
    info = path.lstat()
    if stat.S_ISLNK(info.st_mode): raise SystemExit('unsealed symlink: ' + str(path))
    if path.is_dir(): continue
    name = path.relative_to(root).as_posix()
    if name == 'seal.json': continue
    if not stat.S_ISREG(info.st_mode): raise SystemExit('unexpected file type: ' + name)
    actual.add(name)
    entry = expected.get(name)
    data = path.read_bytes()
    if entry is None or len(data) != entry['bytes'] or hashlib.sha256(data).hexdigest() != entry['sha256']:
        raise SystemExit('source closure mismatch: ' + name)
    if path.suffix == '.py': ast.parse(data, filename=name)
if actual != set(expected): raise SystemExit('sealed path set mismatch')
for entry in seal['references']:
    path = root / entry['path']
    data = path.read_bytes()
    if len(data) != entry['bytes'] or hashlib.sha256(data).hexdigest() != entry['sha256']:
        raise SystemExit('referenced source mismatch: ' + entry['path'])
revision = json.loads((root / 'execution-revision.json').read_bytes())
for key in ('execution_authorized', 'ready_for_execution', 'approval_request_ready_to_send'):
    if revision[key] is not False: raise SystemExit('activation flag changed: ' + key)
guard = ast.parse((root / 'fort_probe/bounded.py').read_bytes())
assignments = [node for node in guard.body if isinstance(node, ast.Assign) and any(isinstance(t, ast.Name) and t.id == 'LIVE_EXECUTION_ENABLED' for t in node.targets)]
if len(assignments) != 1 or not isinstance(assignments[0].value, ast.Constant) or assignments[0].value.value is not False:
    raise SystemExit('live construction guard changed')
inspection = ast.parse((root / 'fort_probe/inspection_guard.py').read_bytes())
flags = [n for n in inspection.body if isinstance(n, ast.Assign) and any(isinstance(t,ast.Name) and t.id == 'INSPECTION_ENABLED' for t in n.targets)]
if len(flags)!=1 or not isinstance(flags[0].value,ast.Constant) or flags[0].value.value is not False:
    raise SystemExit('input inspection guard changed')
if (root / 'inspection-verifier.mjs').read_text().count('export const INSPECTION_ENABLED = false;') != 1:
    raise SystemExit('offline verifier inspection guard changed')
if revision['input_inspection_authorized'] is not False:raise SystemExit('input inspection authority changed')
print(json.dumps({'status': 'passed', 'files': len(actual), 'references': len(seal['references']),
                  'seal_sha256': hashlib.sha256((root / 'seal.json').read_bytes()).hexdigest(),
                  'execution_enabled': False}, sort_keys=True))
