"""Trusted synthetic Python payload: report runtime/env, close pipes, await cancellation."""
import json
import os
from pathlib import Path
import sys
import time
sys.path.insert(0, str(Path(__file__).resolve().parent))
from packet_support import inspect_environment, require_active


def main():
    if len(sys.argv) != 2 or sys.argv[1] not in ('acquire', 'verifier_archive', 'ubuntu_inventory'):
        return 78
    require_active()
    environment, keys = inspect_environment(os.environ, 'python')
    ready = {'case': sys.argv[1], 'runtime': 'python', 'environment': environment,
             'launcher_keys': keys, 'runtime_identity': '.'.join(map(str, sys.version_info[:3])) + '@' + str(Path(sys.executable).resolve()),
             'permissions': None}
    os.write(1, (json.dumps(ready, separators=(',', ':')) + '\n').encode())
    os.close(1); os.close(2)
    while True: time.sleep(1)


if __name__ == '__main__': raise SystemExit(main())
