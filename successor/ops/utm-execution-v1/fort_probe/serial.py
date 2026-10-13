"""Bounded JSON serial framing and request/response transport."""
import json
import math
from .evidence import exact
import os
import select
import time


def _pairs(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError('duplicate JSON key')
        result[key] = value
    return result


def _constant(value):
    raise ValueError('nonfinite JSON number')


def _float(value):
    number = float(value)
    if not math.isfinite(number):
        raise ValueError('nonfinite JSON number')
    return number


def encode(message):
    if not isinstance(message, dict):
        raise ValueError('object required')
    data = (json.dumps(message, ensure_ascii=False, allow_nan=False, separators=(',', ':')) + '\n').encode()
    if len(data) > 4096:
        raise ValueError('frame bound exceeded')
    return data


class Frames:
    def __init__(self):
        self.pending = bytearray()
        self.total = 0
        self.count = 0
        self.failed = False

    def feed(self, chunk):
        if self.failed:
            raise ValueError('channel failed')
        try:
            self.total += len(chunk)
            if len(self.pending) + len(chunk) > 8192 or self.total > 262144:
                raise ValueError('channel byte bound exceeded')
            self.pending.extend(chunk)
            messages = []
            while b'\n' in self.pending:
                line, _, remaining = self.pending.partition(b'\n')
                if len(line) + 1 > 4096 or self.count >= 64:
                    raise ValueError('frame/count bound exceeded')
                message = json.loads(line.decode('utf-8'), object_pairs_hook=_pairs, parse_constant=_constant, parse_float=_float)
                if not isinstance(message, dict):
                    raise ValueError('object required')
                self.pending = bytearray(remaining)
                self.count += 1
                messages.append(message)
            if len(self.pending) >= 4096:
                raise ValueError('incomplete frame exceeds bound')
            return messages
        except (ValueError, RecursionError) as error:
            self.failed = True
            raise ValueError('invalid or excessive serial input') from error

    def finish(self):
        if self.failed or self.pending:
            self.failed = True
            raise ValueError('truncated/failed stream')


class Exchange:
    """One in-flight bounded exchange on an already-owned descriptor; no reconnect/resend."""
    def __init__(self, fd, timeout=0.75):
        if not 0 < timeout <= 0.75:
            raise ValueError('invalid exchange deadline')
        self.fd = fd
        self.timeout = timeout
        self.failed = False
        self.outcome = 'pending'
        self.frames = Frames()
        self.sent_frames = 0
        self.sent_bytes = 0
        os.set_blocking(fd, False)

    def request(self, message, accept):
        return self._exchange(message, [accept], None)

    def stop(self, message, terminal_timeout=5):
        if (set(message) != {'op', 'sequence', 'identity'} or message['op'] != 'stop'
                or type(message['sequence']) is not int or message['sequence'] != 21
                or not 0 < terminal_timeout <= 5):
            self.failed = True
            self.outcome = 'quarantined-unknown'
            raise ValueError('invalid fixed Stop request/deadline')
        ack = {'event': 'stopping', 'sequence': 21, 'identity': message['identity']}
        def terminal(value):
            required = {'event', 'sequence', 'identity', 'stop_elapsed_seconds', 'live_evidence_proven'}
            elapsed = value.get('stop_elapsed_seconds')
            return (set(value) == required and value['event'] == 'owned-attempt-empty'
                    and type(value['sequence']) is int and value['sequence'] == 21
                    and exact(value['identity'], message['identity'])
                    and type(elapsed) in (int, float) and math.isfinite(elapsed) and 0 <= elapsed < 5
                    and type(value['live_evidence_proven']) is bool)
        return self._exchange(message, [lambda value: exact(value, ack), terminal], terminal_timeout)

    def _exchange(self, message, acceptors, terminal_timeout):
        if self.failed or self.outcome != 'pending':
            raise ValueError('terminal exchange cannot send or resend')
        try:
            payload = encode(message)
            if self.sent_frames >= 64 or self.sent_bytes + len(payload) > 262144:
                raise ValueError('transmission budget exhausted')
            if select.select([self.fd], [], [], 0)[0]:
                raise ValueError('unsolicited data or closed channel before request')
            self.sent_frames += 1
            self.sent_bytes += len(payload)
            started = time.monotonic()
            deadline = started + self.timeout
            offset = 0
            while offset < len(payload):
                remaining = deadline - time.monotonic()
                if remaining <= 0 or not select.select([], [self.fd], [], remaining)[1]:
                    raise TimeoutError('serial write deadline')
                try:
                    count = os.write(self.fd, payload[offset:])
                except BlockingIOError:
                    continue
                if count <= 0:
                    raise EOFError('serial write closed')
                offset += count
            accepted = []
            while True:
                remaining = deadline - time.monotonic()
                ready = remaining > 0 and select.select([self.fd], [], [], remaining)[0]
                if not ready:
                    if terminal_timeout is not None and len(accepted) == 2 and not self.frames.pending:
                        self.outcome = 'stopped-empty'
                        return accepted[-1]
                    raise TimeoutError('serial response deadline')
                chunk = os.read(self.fd, 4096)
                if not chunk:
                    self.frames.finish()
                    raise EOFError('serial channel closed')
                if time.monotonic() >= deadline:
                    raise TimeoutError('late serial response')
                messages = self.frames.feed(chunk)
                for receipt in messages:
                    if len(accepted) >= len(acceptors) or acceptors[len(accepted)](receipt) is not True:
                        raise ValueError('unexpected serial receipt')
                    accepted.append(receipt)
                    if terminal_timeout is not None:
                        deadline = started + terminal_timeout
                if terminal_timeout is None and accepted:
                    if self.frames.pending:
                        raise ValueError('unsolicited partial response')
                    return accepted[0]
        except Exception:
            self.outcome = 'quarantined-unknown'
            self.failed = True
            raise
