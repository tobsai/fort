"""Durable single-attempt controller; guest OS effects are an explicit system boundary."""
from copy import deepcopy
import fcntl
import json
import os
import re
import stat
from . import evidence
from .serial import _pairs, _constant, _float


def same_identity(left, right):
    return json.dumps(left, sort_keys=True, allow_nan=False) == json.dumps(right, sort_keys=True, allow_nan=False)


class Store:
    def __init__(self, root):
        self.directory = os.open(root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        self.lock = None
        try:
            info = os.fstat(self.directory)
            if info.st_uid != os.getuid() or stat.S_IMODE(info.st_mode) != 0o700:
                raise ValueError('private state directory required')
            self.lock = os.open('controller.lock', os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600, dir_fd=self.directory)
            self._check(os.fstat(self.lock))
            fcntl.flock(self.lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except Exception:
            self.close()
            raise

    def _check(self, info):
        if not stat.S_ISREG(info.st_mode) or info.st_uid != os.getuid() or info.st_nlink != 1 or info.st_mode & 0o077:
            raise ValueError('unsafe state file')

    def read(self, name):
        try:
            fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW, dir_fd=self.directory)
        except FileNotFoundError:
            return None
        with os.fdopen(fd, 'rb') as stream:
            self._check(os.fstat(stream.fileno()))
            data = stream.read(8193)
            if len(data) > 8192:
                raise ValueError('state file too large')
            value = json.loads(data, object_pairs_hook=_pairs, parse_constant=_constant, parse_float=_float)
            if not isinstance(value, dict):
                raise ValueError('object state required')
            return value

    def write_new(self, name, value):
        data = (json.dumps(value, sort_keys=True, allow_nan=False) + '\n').encode()
        if len(data) > 8192:
            raise ValueError('state bound exceeded')
        fd = os.open(name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=self.directory)
        with os.fdopen(fd, 'wb') as stream:
            stream.write(data)
            stream.flush()
            os.fsync(stream.fileno())
        os.fsync(self.directory)

    def save(self, value):
        self.write_new('state.pending', value)
        os.replace('state.pending', 'state.json', src_dir_fd=self.directory, dst_dir_fd=self.directory)
        os.fsync(self.directory)

    def close(self):
        if self.lock is not None:
            os.close(self.lock)
            self.lock = None
        if self.directory is not None:
            os.close(self.directory)
            self.directory = None


class Controller:
    def __init__(self, root, binding, guest, clock):
        self.store = Store(root)
        self.binding = deepcopy(binding)
        self.guest = guest
        self.clock = clock
        self.identity = None
        self.sequence = 0
        self.state = 'unconsumed'
        self.started = None
        self.last_activity = None
        self.stop_started = None
        self.stop_delivered = False
        self.closed = False
        self.prepared_runtime = None
        self.live_evidence = None
        self.capture_attempted = False
        try:
            self.consumed = self.store.read('consumed.json') is not None
            self.owns_attempt = not self.consumed
            if self.consumed:
                record = self.store.read('state.json')
                self.state = record['state'] if record else 'quarantined-unknown'
                self.identity = record.get('identity') if record else None
        except Exception:
            self.store.close()
            raise

    def __enter__(self):
        return self

    def __exit__(self, kind, value, traceback):
        self.close()

    def tick(self):
        """Broker timer must call this even when neither serial channel has input."""
        if self.closed or not self.owns_attempt:
            return
        now = self.clock()
        if self.state == 'stopping':
            return self.poll_stop()
        if self.started is None:
            return
        if (now < self.started or now - self.started >= 40
                or (self.state == 'prepared' and now - self.started >= 4)
                or (self.state == 'granted' and now - self.last_activity >= 1)):
            self.abort('independent broker timer expired')

    def preflight(self):
        return evidence.preflight(self.guest.preflight(), self.guest.boot_id)

    def capture_live(self):
        if self.closed or not self.owns_attempt or self.state != 'granted' or self.capture_attempted:
            raise ValueError('live observation requires a running, unobserved grant')
        self.capture_attempted = True
        try:
            self.store.write_new('capture-consumed.json', {'identity': self.identity, 'sequence': self.sequence})
            before = self.clock()
            snapshot = self.guest.live_snapshot(self.identity, self.sequence, before)
            after = self.clock()
            if not 0 <= after - before < .5 or after - self.last_activity >= 1:
                raise ValueError('live observation exceeded deadline; lease not renewed')
            evidence.live(snapshot, self.identity, self.sequence, self.started, after, self.prepared_runtime)
            self.store.write_new('live.json', snapshot)
            self.live_evidence = deepcopy(snapshot)
            return deepcopy(snapshot)
        except Exception:
            self.live_evidence = None
            try:
                self.abort('live observation consumed and failed')
            finally:
                self.owns_attempt = False
            raise

    def _stop_facts(self, observed):
        if (not isinstance(observed, dict)
                or set(observed) != {'identity', 'runtime_active', 'tool_active', 'empty'}
                or not same_identity(observed['identity'], self.identity)
                or any(type(observed[key]) is not bool for key in ('runtime_active', 'tool_active', 'empty'))):
            raise ValueError('complete typed stop observation required')
        return (observed['runtime_active'] is False and observed['tool_active'] is False
                and observed['empty'] is True)

    def inspect(self):
        return {'consumed': self.consumed, 'state': self.state,
                'identity': deepcopy(self.identity), 'guest': self.guest.current()}

    def _save(self, state):
        self.state = state
        self.store.save({'state': state, 'identity': self.identity, 'sequence': self.sequence})

    def handle(self, message):
        try:
            before = self.clock()
            result = self._handle(message)
            if message != {'op': 'inspect'} and not 0 <= self.clock() - before < .5:
                raise ValueError('control operation exceeded deadline')
            return result
        except Exception:
            self.abort('invalid, failed or uncertain control operation')
            raise

    def _handle(self, message):
        if self.closed:
            raise ValueError('closed controller')
        if message == {'op': 'inspect'}:
            return self.inspect()
        if not self.owns_attempt:
            raise ValueError('reopened consumed attempt is inspection-only')
        if message == {'op': 'prepare', 'sequence': 0} and type(message.get('sequence')) is int:
            if self.consumed or not self.guest.ready():
                raise ValueError('attempt already consumed or guest not ready')
            self.preflight()
            self.store.write_new('consumed.json', dict(self.binding, guest_boot_id=self.guest.boot_id))
            self.consumed = True
            self._save('consumed-preparing')
            self.started = self.last_activity = self.clock()
            self.identity = self.guest.prepare(self.binding)
            required = set(self.binding) | {'guest_boot_id', 'invocation_id', 'runtime_cgroup'}
            if (not isinstance(self.identity, dict) or set(self.identity) != required
                    or not same_identity({key: self.identity[key] for key in self.binding}, self.binding)
                    or self.identity['guest_boot_id'] != self.guest.boot_id
                    or not re.fullmatch(r'[a-f0-9]{32}', self.identity['invocation_id'])
                    or self.identity['runtime_cgroup'] != '/fort.slice/fort-attempt.slice/fort-runtime.service'
                    or self.clock() - self.started >= 4):
                raise ValueError('prepared identity/lease mismatch')
            self._save('prepared')
            return {'event': 'prepared', 'identity': deepcopy(self.identity)}
        if not isinstance(message, dict) or set(message) != {'op', 'sequence', 'identity'}:
            raise ValueError('exact command fields required')
        if self.identity is None or not same_identity(message['identity'], self.identity):
            raise ValueError('exact attempt identity required')
        if type(message['sequence']) is not int or message['sequence'] != self.sequence + 1:
            raise ValueError('nonconsecutive sequence')
        now = self.clock()
        if self.started is None or now < self.started or now - self.started >= 40:
            raise ValueError('runtime deadline or clock invalid')
        if self.state == 'prepared' and now - self.started >= 4:
            raise ValueError('prepare lease expired')
        if self.state == 'granted' and (now < self.last_activity or now - self.last_activity >= 1):
            raise ValueError('control idle deadline exceeded')
        observed = self.guest.current()
        if not same_identity(observed['identity'], self.identity) or not observed['runtime_active']:
            raise ValueError('original runtime identity is no longer active')
        operation = message['op']
        if operation == 'grant' and self.state == 'prepared' and message['sequence'] == 1:
            controls = self.guest.prepared_controls(self.identity)
            evidence.prepared(controls, self.identity)
            if self.clock() - self.started >= 4:
                raise ValueError('prepared controls exceeded lease')
            self.prepared_runtime = deepcopy(controls['runtime'])
            self._save('grant-starting')
            self.guest.grant(self.identity)
            self.sequence = 1
            self._save('granted')
        elif operation == 'heartbeat' and self.state == 'granted' and 2 <= message['sequence'] <= 20:
            self.guest.heartbeat(self.identity)
            self.sequence = message['sequence']
            self._save('granted')
        elif operation == 'stop' and self.state == 'granted' and message['sequence'] == 21:
            self.sequence = 21
            self.stop_started = self.clock()
            self._save('stopping')
            self.guest.begin_stop(self.identity)
            if self.clock() - self.stop_started >= 0.5:
                raise ValueError('stop acknowledgement operation exceeded budget')
            return {'event': 'stopping', 'sequence': 21, 'identity': deepcopy(self.identity)}
        else:
            raise ValueError('operation not allowed in this state')
        self.last_activity = self.clock()
        return {'event': 'accepted', 'sequence': self.sequence, 'identity': deepcopy(self.identity)}

    def poll_stop(self):
        if self.closed or self.state != 'stopping' or self.stop_delivered:
            raise ValueError('no pending stop proof')
        try:
            elapsed = self.clock() - self.stop_started
            if not 0 <= elapsed < 5:
                raise ValueError('stop proof deadline expired')
            observed = self.guest.stop_observation(self.identity)
            elapsed = self.clock() - self.stop_started
            if not 0 <= elapsed < 5 or not same_identity(observed['identity'], self.identity):
                raise ValueError('stop proof late or identity changed')
            if not self._stop_facts(observed):
                return None
            self._save('stopped-empty')
            self.stop_delivered = True
            return {'event': 'owned-attempt-empty', 'sequence': 21,
                    'identity': deepcopy(self.identity), 'stop_elapsed_seconds': elapsed,
                    'live_evidence_proven': self.live_evidence is not None}
        except Exception:
            self._save('quarantined-unknown')
            self.owns_attempt = False
            raise

    def abort(self, reason):
        if self.closed or not self.owns_attempt:
            return
        self.owns_attempt = False
        if not self.consumed or self.state in ('stopped-empty', 'quarantined-unknown'):
            return
        state = 'quarantined-unknown'
        try:
            current = self.guest.current()
            if self.identity is not None and same_identity(current['identity'], self.identity):
                if self.stop_started is None:
                    self.stop_started = self.clock()
                    self.guest.begin_stop(self.identity)
                observed = self.guest.stop_observation(self.identity)
                if 0 <= self.clock() - self.stop_started < 5 and self._stop_facts(observed):
                    state = 'stopped-empty'
        except Exception:
            pass
        self.state = state
        self.store.save({'state': state, 'identity': self.identity, 'sequence': self.sequence,
                         'failure': reason, 'experiment_passed': False})

    def close(self):
        if not self.closed:
            try:
                self.abort('controller closed before nominal completion')
            finally:
                self.closed = True
                self.store.close()
