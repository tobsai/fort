"""Trusted synthetic main process; PID 1 owns the complete unit lifetime."""
import json
import os
import pathlib
import socket
import time

ROOT = pathlib.Path('/run/fort-probe/session')


def main():
    started = time.monotonic()
    invocation = os.environ['INVOCATION_ID']
    identity_path = ROOT / 'identity.json'
    identity = json.loads(identity_path.read_text()) if identity_path.exists() else None
    if identity is not None and identity['invocationId'] != invocation:
        raise RuntimeError('Replacement invocation cannot report ready')
    grant_path = ROOT / 'grant.json'
    if grant_path.exists():
        grant = json.loads(grant_path.read_text())
        if identity is None or grant != identity or grant['invocationId'] != invocation:
            raise RuntimeError('Stale grant cannot report ready')
    notify = socket.socket(socket.AF_UNIX, socket.SOCK_DGRAM)
    address = os.environ['NOTIFY_SOCKET']
    notify.connect(('\0' + address[1:]) if address.startswith('@') else address)
    notify.send(b'READY=1\nSTATUS=Waiting for exact synthetic grant')
    granted = False
    while time.monotonic() - started < 38:
        heartbeat = json.loads((ROOT / 'heartbeat.json').read_text())
        age = time.monotonic() - heartbeat['monotonic']
        if not 0 <= age <= 1.5:
            raise RuntimeError('Control heartbeat expired')
        grant_path = ROOT / 'grant.json'
        if grant_path.exists():
            grant = json.loads(grant_path.read_text())
            if grant != json.loads((ROOT / 'identity.json').read_text()):
                raise RuntimeError('Grant mismatch')
            if grant['invocationId'] != invocation:
                raise RuntimeError('Wrong unit invocation')
            if not granted:
                canary = pathlib.Path('/var/lib/fort-probe/provider/synthetic-canary').read_text()
                if canary != 'synthetic-provider-canary-only\n':
                    raise RuntimeError('Wrong synthetic store')
                print(json.dumps({'event': 'synthetic-grant-observed', 'invocationId': invocation}), flush=True)
                granted = True
        notify.send(b'WATCHDOG=1')
        time.sleep(0.2)


if __name__ == '__main__':
    main()
