"""Invoke the unchanged approved downloader once and retain its bounded public receipts."""
import datetime
import json
import os
import pathlib
import subprocess

HERE = pathlib.Path(__file__).resolve().parent
PACKET = HERE.parents[2] / 'successor/ops/isolation-probe-v1'
os.umask(0o077)
started = datetime.datetime.now(datetime.timezone.utc).isoformat()
with (HERE / 'download-stdout.txt').open('x') as output, (HERE / 'download-stderr.txt').open('x') as error:
    result = subprocess.run(['/usr/bin/python3', '-I', str(PACKET / 'fetch-public.py')],
                            cwd=PACKET, stdin=subprocess.DEVNULL, stdout=output, stderr=error,
                            env={'PATH': '/usr/bin:/bin:/usr/sbin:/sbin', 'LANG': 'C', 'LC_ALL': 'C'},
                            close_fds=True, timeout=900, check=False)
receipt = {'started_at_utc': started, 'ended_at_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
           'command': ['/usr/bin/python3', '-I', str(PACKET / 'fetch-public.py')],
           'exit_code': result.returncode, 'retry': False}
with (HERE / 'download-result.json').open('x') as output:
    json.dump(receipt, output, indent=2)
    output.write('\n')
print(json.dumps(receipt))
raise SystemExit(result.returncode)
