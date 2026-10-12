"""Guest-root, fixed synthetic protocol. This is unexecuted prototype source."""
import fcntl
import json
import os
import pathlib
import selectors
import subprocess
import sys
import time

STATE = pathlib.Path('/var/lib/fort-probe')
SESSION = pathlib.Path('/run/fort-probe/session')
RUNTIME = 'fort-runtime.service'
TOOL = 'fort-tool.service'
SLICE = 'fort-attempt.slice'
PREFIX = '/fort.slice/fort-attempt.slice'
ENV = {'PATH': '/usr/sbin:/usr/bin:/sbin:/bin', 'LANG': 'C', 'LC_ALL': 'C'}


def systemctl(*args):
    return subprocess.run(['/usr/bin/systemctl', *args], env=ENV, stdin=subprocess.DEVNULL,
                          capture_output=True, text=True, timeout=5, check=True).stdout


def show(unit):
    return dict(line.split('=', 1) for line in systemctl('show', unit,
                '--property=ActiveState,SubState,InvocationID,ControlGroup,MainPID').splitlines())


def emit(value):
    print(json.dumps(value, sort_keys=True), flush=True)


def atomic(path, value):
    temporary = path.with_suffix('.tmp')
    temporary.write_text(json.dumps(value, sort_keys=True) + '\n')
    os.chmod(temporary, 0o644)
    os.replace(temporary, path)


def boot_id():
    return pathlib.Path('/proc/sys/kernel/random/boot_id').read_text().strip()


def heartbeat():
    atomic(SESSION / 'heartbeat.json', {'monotonic': time.monotonic()})


def exact(identity):
    if identity['guestBootId'] != boot_id():
        raise RuntimeError('Guest reboot: quarantine; never start a replacement')
    current = show(RUNTIME)
    if current['InvocationID'] != identity['invocationId']:
        raise RuntimeError('Invocation changed: quarantine')
    if current['ControlGroup'] != identity['runtimeCgroup']:
        raise RuntimeError('Runtime cgroup changed: quarantine')
    return current


def empty_attempt():
    group = pathlib.Path('/sys/fs/cgroup' + PREFIX)
    if not group.exists():
        return True
    return all(not path.read_text().strip() for path in group.rglob('cgroup.procs'))


def stop_owned(identity):
    if identity['guestBootId'] != boot_id():
        raise RuntimeError('Cannot attribute cleanup across a reboot')
    current = show(RUNTIME)
    if current['ActiveState'] not in ('inactive', 'failed'):
        exact(identity)
    systemctl('stop', TOOL, RUNTIME, SLICE)
    deadline = time.monotonic() + 3
    while time.monotonic() < deadline:
        inactive = all(show(unit)['ActiveState'] in ('inactive', 'failed') for unit in (TOOL, RUNTIME))
        if inactive and empty_attempt():
            receipt = {'event': 'owned-attempt-empty', 'identity': identity}
            atomic(STATE / 'cleanup.json', receipt)
            return receipt
        time.sleep(0.1)
    raise RuntimeError('Cleanup unverified: quarantine')


def prepare():
    if not (STATE / 'installed').is_file():
        raise RuntimeError('Provisioning not verified')
    if not empty_attempt():
        raise RuntimeError('Existing attempt tasks: quarantine')
    consumed = {'installation': 'fort-probe-r1', 'epoch': 1, 'attempt': 'probe-0001',
                'generation': 1, 'guestBootId': boot_id(), 'unitName': RUNTIME}
    # Persist before any launch. An uncertain prepare is never retried as new work.
    with (STATE / 'consumed.json').open('x') as stream:
        json.dump(consumed, stream)
        stream.flush()
        os.fsync(stream.fileno())
    SESSION.mkdir(mode=0o755)
    heartbeat()
    systemctl('start', '--no-block', RUNTIME)
    deadline = time.monotonic() + 4
    while time.monotonic() < deadline:
        # The initial prepare lease is bounded and cannot release tool work.
        heartbeat()
        current = show(RUNTIME)
        if current['ActiveState'] == 'active' and current['InvocationID']:
            if current['ControlGroup'] != PREFIX + '/' + RUNTIME:
                raise RuntimeError('Unexpected cgroup topology')
            identity = dict(consumed, invocationId=current['InvocationID'], runtimeCgroup=current['ControlGroup'])
            atomic(STATE / 'identity.json', identity)
            atomic(SESSION / 'identity.json', identity)
            return identity
        time.sleep(0.05)
    raise RuntimeError('Start acknowledgement unavailable; consumed attempt remains fenced')


def messages():
    selector = selectors.DefaultSelector()
    selector.register(sys.stdin, selectors.EVENT_READ)
    buffer = b''
    while True:
        if not selector.select(1.0):
            raise RuntimeError('Control channel idle')
        chunk = os.read(sys.stdin.fileno(), 4097)
        if not chunk:
            return
        buffer += chunk
        if len(buffer) > 4096:
            raise RuntimeError('Input frame bound exceeded')
        while b'\n' in buffer:
            line, buffer = buffer.split(b'\n', 1)
            yield json.loads(line)


def main():
    if os.geteuid() != 0 or sys.argv[1:]:
        raise RuntimeError('Guest root and no command arguments required')
    lock = open('/run/fort-probe/controller.lock', 'w')
    fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
    identity = None
    granted = False
    sequence = 0
    try:
        for message in messages():
            if not isinstance(message, dict):
                raise RuntimeError('Object required')
            if identity is None:
                if message == {'op': 'inspect'}:
                    emit({'event': 'inspection', 'guestBootId': boot_id(),
                          'consumed': (STATE / 'consumed.json').exists(),
                          'identity': json.loads((STATE / 'identity.json').read_text())
                          if (STATE / 'identity.json').exists() else None,
                          'runtime': show(RUNTIME), 'tool': show(TOOL), 'empty': empty_attempt()})
                    return
                if message != {'op': 'prepare'}:
                    raise RuntimeError('Only prepare or inspect accepted initially')
                identity = prepare()
                emit({'event': 'prepared', 'identity': identity})
                continue
            if set(message) != {'op', 'identity', 'sequence'} or message['identity'] != identity:
                raise RuntimeError('Exact identity required')
            if type(message['sequence']) is not int or message['sequence'] != sequence + 1:
                raise RuntimeError('Nonconsecutive sequence')
            sequence = message['sequence']
            current = exact(identity)
            if message['op'] == 'stop':
                emit(stop_owned(identity))
                identity = None
                return
            if message['op'] == 'grant' and not granted:
                if current['ActiveState'] != 'active':
                    raise RuntimeError('Original runtime is no longer active; grant rejected')
                heartbeat()
                atomic(SESSION / 'grant.json', identity)
                systemctl('start', '--no-block', TOOL)
                granted = True
            elif message['op'] == 'heartbeat':
                heartbeat()
            else:
                raise RuntimeError('Unknown or repeated operation')
            emit({'event': 'accepted', 'sequence': sequence, 'identity': identity})
    finally:
        if identity is not None:
            try:
                emit(stop_owned(identity))
            except Exception:
                atomic(STATE / 'quarantined.json', {'identity': identity, 'reason': 'cleanup-unverified'})
                raise


if __name__ == '__main__':
    main()
