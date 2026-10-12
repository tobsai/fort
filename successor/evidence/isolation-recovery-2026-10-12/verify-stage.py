"""Capture the fixed offline verification stages with bounded logs and lifetime."""
import datetime
import json
import os
import pathlib
import selectors
import signal
import subprocess
import sys
import time

HERE = pathlib.Path(__file__).resolve().parent
PACKET = HERE.parents[2] / 'successor/ops/isolation-probe-v1'
if sys.argv[1:] not in (['registry'], ['publisher']):
    raise SystemExit('Choose registry or publisher')
stage = sys.argv[1]
command = ['/usr/bin/python3', '-B', str(PACKET / 'verify-offline.py')]
if stage == 'registry':
    command.append('--registry-only')
os.umask(0o077)
started = datetime.datetime.now(datetime.timezone.utc).isoformat()
streams = {name: (HERE / (stage + '-' + name + '.txt')).open('xb') for name in ('stdout', 'stderr')}
process = subprocess.Popen(command, cwd=PACKET, stdin=subprocess.DEVNULL,
    stdout=subprocess.PIPE, stderr=subprocess.PIPE, close_fds=True, start_new_session=True,
    env={'PATH': '/usr/bin:/bin', 'LANG': 'C', 'LC_ALL': 'C'})
selector = selectors.DefaultSelector()
selector.register(process.stdout, selectors.EVENT_READ, 'stdout')
selector.register(process.stderr, selectors.EVENT_READ, 'stderr')
deadline = time.monotonic() + 130
size = 0
bounded_failure = None
try:
    while selector.get_map():
        if time.monotonic() >= deadline:
            bounded_failure = 'verification capture deadline exceeded'
            break
        for key, _ in selector.select(0.5):
            chunk = os.read(key.fileobj.fileno(), 65536)
            if not chunk:
                selector.unregister(key.fileobj)
                key.fileobj.close()
                continue
            if size + len(chunk) > 4 * 1024**2:
                bounded_failure = 'verification output cap exceeded'
                break
            streams[key.data].write(chunk)
            size += len(chunk)
        if bounded_failure:
            break
    if bounded_failure:
        os.killpg(process.pid, signal.SIGKILL)
    code = process.wait(timeout=5)
finally:
    selector.close()
    for output in streams.values():
        output.flush()
        os.fsync(output.fileno())
        output.close()
result = {'stage': stage, 'command': command, 'started_at_utc': started,
          'ended_at_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
          'exit_code': code, 'captured_bytes': size, 'bounded_failure': bounded_failure}
with (HERE / (stage + '-result.json')).open('x') as output:
    json.dump(result, output, indent=2)
    output.write('\n')
print(json.dumps(result))
raise SystemExit(code if code else (1 if bounded_failure else 0))
