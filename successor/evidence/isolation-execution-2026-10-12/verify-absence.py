"""Separate post-cleanup path/listener scan; no mutation of host resource roots."""
import datetime
import json
import os
import pathlib
import subprocess

HERE = pathlib.Path(__file__).resolve().parent
manifest = json.loads((HERE / 'created-paths.json').read_text())
paths = [entry['path'] for entry in manifest] + [
    '/Users/tobiasgunn/Library/Application Support/FortIsolation/downloads/ubuntu.pub.partial',
    '/Users/tobiasgunn/Library/Application Support/FortIsolation/tools',
    '/Users/tobiasgunn/Library/FortIsolation',
    '/Users/tobiasgunn/Library/FortIsolation/state',
    '/Users/tobiasgunn/Library/FortIsolation/state/_config/user',
    '/Users/tobiasgunn/Library/FortIsolation/state/fort-isolation-probe',
]
scan = [{'path': path, 'absent': not os.path.lexists(path)} for path in paths]
listener = subprocess.run(['/usr/sbin/lsof', '-nP', '-iTCP:22058', '-sTCP:LISTEN'],
                          stdin=subprocess.DEVNULL, capture_output=True, text=True, timeout=15)
result = {'time_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'paths': scan,
          'port_22058_query': {'command': listener.args, 'exit_code': listener.returncode,
                               'stdout': listener.stdout, 'stderr': listener.stderr},
          'all_paths_absent': all(item['absent'] for item in scan),
          'port_22058_unused': listener.returncode == 1 and not listener.stdout and not listener.stderr,
          'process_identity_check': 'Not applicable: no Lima manager, host agent, VZ process or guest session was ever started.'}
with (HERE / 'absence.json').open('x') as output:
    json.dump(result, output, indent=2)
    output.write('\n')
print(json.dumps({'all_paths_absent': result['all_paths_absent'], 'port_22058_unused': result['port_22058_unused']}))
assert result['all_paths_absent'] and result['port_22058_unused']
