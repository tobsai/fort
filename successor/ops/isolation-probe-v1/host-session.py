"""Future approved synthetic control session only; does not provision or sign in."""
import json
import os
import pathlib
import selectors
import subprocess
import time

STATE = pathlib.Path('/Users/tobiasgunn/Library/FortIsolation/state')
ROOT = pathlib.Path('/Users/tobiasgunn/Library/Application Support/FortIsolation')


def main():
    # These files may exist only after the separate download/boot/key-pinning gate.
    for path in [STATE / '_config/user', ROOT / 'trust/known_hosts']:
        info = path.lstat()
        if path.is_symlink() or info.st_uid != os.getuid() or info.st_mode & 0o077:
            raise RuntimeError('Unsafe fresh management identity/trust file')
    options = ['BatchMode=yes', 'IdentitiesOnly=yes', 'IdentityAgent=none',
               'StrictHostKeyChecking=yes', 'NoHostAuthenticationForLocalhost=no',
               'GlobalKnownHostsFile=/dev/null', 'UserKnownHostsFile=' + str(ROOT / 'trust/known_hosts'),
               'ForwardAgent=no', 'ForwardX11=no', 'ClearAllForwardings=yes',
               'ControlMaster=no', 'ProxyCommand=none', 'ConnectTimeout=5']
    args = ['/usr/bin/ssh', '-F', '/dev/null', '-T', '-p', '22058', '-i', str(STATE / '_config/user')]
    for option in options:
        args.extend(['-o', option])
    args.extend(['fort-admin@127.0.0.1', 'sudo -n /usr/bin/python3 -I /opt/fort-probe/controller.py'])
    env = {'PATH': '/usr/bin:/bin:/usr/sbin:/sbin', 'LANG': 'C', 'LC_ALL': 'C',
           'HOME': str(ROOT / 'home'), 'TMPDIR': str(ROOT / 'tmp')}
    process = subprocess.Popen(args, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                               close_fds=True, env=env)
    selector = selectors.DefaultSelector()
    selector.register(process.stdout, selectors.EVENT_READ)
    buffer = b''

    def exchange(message):
        nonlocal buffer
        process.stdin.write(json.dumps(message).encode() + b'\n')
        process.stdin.flush()
        deadline = time.monotonic() + 6
        while b'\n' not in buffer:
            if time.monotonic() > deadline or not selector.select(0.5):
                if time.monotonic() > deadline:
                    raise RuntimeError('Lost control acknowledgement: no resend')
                continue
            chunk = os.read(process.stdout.fileno(), 4097)
            if not chunk:
                raise RuntimeError('Control channel closed: reconcile')
            buffer += chunk
            if len(buffer) > 4096:
                raise RuntimeError('Oversized control response')
        line, buffer = buffer.split(b'\n', 1)
        receipt = json.loads(line)
        print(json.dumps(receipt, sort_keys=True), flush=True)
        return receipt

    try:
        prepared = exchange({'op': 'prepare'})
        if prepared.get('event') != 'prepared':
            raise RuntimeError('Prepare rejected')
        identity = prepared['identity']
        expected = {'installation': 'fort-probe-r1', 'epoch': 1, 'attempt': 'probe-0001',
                    'generation': 1, 'unitName': 'fort-runtime.service'}
        if any(identity.get(key) != value for key, value in expected.items()):
            raise RuntimeError('Wrong synthetic identity')
        for sequence in range(1, 22):
            operation = 'grant' if sequence == 1 else ('stop' if sequence == 21 else 'heartbeat')
            receipt = exchange({'op': operation, 'identity': identity, 'sequence': sequence})
            if receipt.get('identity') != identity:
                raise RuntimeError('Mismatched receipt')
            if operation == 'stop':
                if receipt.get('event') != 'owned-attempt-empty':
                    raise RuntimeError('Cleanup unverified')
            elif receipt.get('event') != 'accepted' or receipt.get('sequence') != sequence:
                raise RuntimeError('Grant/heartbeat unconfirmed')
            time.sleep(0.25)
        process.stdin.close()
        if process.wait(timeout=5) != 0:
            raise RuntimeError('Controller failed after receipt')
    finally:
        selector.close()
        if not process.stdin.closed:
            process.stdin.close()
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait(timeout=5)
        # Killing SSH is not proof of guest cleanup; absent receipt stays unknown.


if __name__ == '__main__':
    main()
