"""Fixed untrusted-workload canary; never launches a provider or arbitrary command."""
import json
import os
import pathlib
import socket
import time


def main():
    denied = []
    for path in ['/var/lib/fort-probe/provider/synthetic-canary',
                 '/run/fort-probe/session/identity.json',
                 '/home/fort-admin/.ssh/authorized_keys', '/mnt/lima-cidata/user-data']:
        try:
            pathlib.Path(path).read_bytes()
        except (PermissionError, FileNotFoundError):
            denied.append(path)
        else:
            raise RuntimeError('Protected canary readable: ' + path)
    try:
        socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    except OSError:
        pass
    else:
        raise RuntimeError('INET socket creation unexpectedly allowed')
    pathlib.Path('/tmp/tool-positive-control').write_text('synthetic\n')
    child = os.fork()
    if child == 0:
        os.setsid()
        grandchild = os.fork()
        if grandchild != 0:
            os._exit(0)
        print(json.dumps({'event': 'setsid-descendant', 'pid': os.getpid()}), flush=True)
        time.sleep(35)
        os._exit(0)
    os.waitpid(child, 0)
    print(json.dumps({'event': 'canaries-denied', 'paths': denied, 'uid': os.getuid()}), flush=True)
    time.sleep(35)


if __name__ == '__main__':
    main()
