"""Only injected OS/child fixtures. No real setters, child startup, ps or signals."""
import importlib
import json
from pathlib import Path
import resource
import signal
import sys
import unittest
from types import SimpleNamespace
from unittest.mock import patch
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
sys.path.insert(0, str(Path(__file__).resolve().parent))
import smoke
from test_smoke import CASES, result


class ResourceTests(unittest.TestCase):
    def test_exact_reviewed_setters_read_back_all_four_profiles_without_rss_as(self):
        for case in CASES:
            calls = []; values = {}
            def set_value(key, value): calls.append(('set', key, value)); values[key] = value
            def get_value(key): calls.append(('get', key)); return values[key]
            with patch.object(smoke.sys, 'platform', 'darwin'), patch.object(resource, 'setrlimit', set_value), patch.object(resource, 'getrlimit', get_value):
                recorder = smoke.LimitRecorder(); setter, profiles = smoke.reviewed_setter(recorder)
                setter(profiles[case['name']])
                self.assertEqual(recorder.observed, smoke.expected_limits(case))
                self.assertEqual([c[1] for c in calls if c[0] == 'set'], list(smoke.LIMITS.values()))
                self.assertEqual([c[0] for c in calls], ['set', 'get'] * 4)
                self.assertEqual(profiles['registry'], profiles['publisher'])

    def test_setter_error_or_wrong_readback_never_reports_success(self):
        for failed in smoke.LIMITS.values():
            def set_value(key, unused):
                if key == failed: raise ValueError('synthetic setter failure')
            writes = []
            with patch.object(smoke.sys, 'platform', 'darwin'), patch.object(resource, 'setrlimit', set_value), \
                 patch.object(resource, 'getrlimit', lambda key: tuple(smoke.expected_limits(CASES[0])[next(n for n, k in smoke.LIMITS.items() if k == key)])), \
                 patch.object(smoke.signal, 'signal'), patch.object(smoke.signal, 'alarm'), \
                 patch.object(smoke.os, 'write', lambda fd, data: writes.append(data)), patch.object(smoke.os, 'close'):
                with self.assertRaises(ValueError): smoke.setup_child(99, CASES[0])
            report = json.loads(writes[0])
            self.assertIn('setup_failure', report)
            self.assertEqual(smoke.LIMITS[report['setup_failure']['setter']], failed)
        with patch.object(resource, 'setrlimit'), patch.object(resource, 'getrlimit', return_value=(999, 999)):
            with self.assertRaises(ValueError): smoke.LimitRecorder().setrlimit(resource.RLIMIT_CORE, (0, 0))
        with patch.object(resource, 'setrlimit') as setter:
            with self.assertRaises(ValueError): smoke.LimitRecorder().setrlimit(resource.RLIMIT_RSS, (1, 1))
            setter.assert_not_called()

    def test_owned_kill_precedes_single_reap_then_existence_only(self):
        calls = []
        def signal_pid(pid, number):
            calls.append(('probe', pid, number))
            if number == 0: raise ProcessLookupError()
        def wait(timeout): calls.append(('reap', timeout)); return -9
        child = SimpleNamespace(pid=123, wait=wait)
        with patch.object(smoke.os, 'killpg', signal_pid), patch.object(smoke.os, 'kill', signal_pid):
            cleanup = smoke.cancel_owned(child)
        self.assertEqual(calls, [('probe', 123, signal.SIGKILL), ('reap', 1), ('probe', 123, 0), ('probe', 123, 0)])
        self.assertEqual(cleanup, {'cancel_sent': True, 'reaped': True, 'child_absent': True,
                                   'group_absent': True, 'returncode': -9})

    def test_remaining_process_or_group_is_not_cleanup_success(self):
        child = SimpleNamespace(pid=123, wait=lambda timeout: -9)
        with patch.object(smoke.os, 'killpg'), patch.object(smoke.os, 'kill'):
            cleanup = smoke.cancel_owned(child)
        self.assertFalse(cleanup['child_absent']); self.assertFalse(cleanup['group_absent'])


class Stream:
    def __init__(self, fd): self.fd = fd
    def fileno(self): return self.fd
    def close(self): pass


class Selector:
    def __init__(self): self.entries = {}; self.reads = 0
    def __enter__(self): return self
    def __exit__(self, *unused): pass
    def register(self, obj, unused, data): self.entries[obj] = SimpleNamespace(fileobj=obj, data=data)
    def unregister(self, obj): del self.entries[obj]
    def get_map(self): return self.entries
    def select(self, timeout):
        self.reads += 1
        return [(value, None) for value in list(self.entries.values())]


class SupervisorTests(unittest.TestCase):
    def exercise(self, fault=None):
        case = dict(CASES[0], argv=['/synthetic/python'])
        context = SimpleNamespace(identities={'python': 'python-fixture'}, packet_sha='a' * 64)
        ready = result(case)
        for key in ('limits', 'pid', 'rss_samples', 'rss_after_eof', 'max_rss_bytes',
                    'cancel_sent', 'returncode', 'reaped', 'child_absent', 'group_absent'): ready.pop(key)
        queues = {90: [json.dumps({'limits': smoke.expected_limits(case)}).encode(), b''],
                  91: [json.dumps(ready).encode(), b''], 92: [b'']}
        if fault == 'output': queues[91] = [b'x' * 8193]
        if fault == 'stderr': queues[92] = [b'synthetic warning', b'']
        process = SimpleNamespace(pid=123, stdout=Stream(91), stderr=Stream(92))
        cleanup = {'cancel_sent': True, 'reaped': True, 'child_absent': True,
                   'group_absent': True, 'returncode': -9}
        observations = []
        def rss(argv, **options):
            observations.append((argv, options))
            return ({'empty': b'', 'malformed': b'??', 'zero': b'0', 'over': b'999999999'}.get(fault, b'1024'), b'')
        def wait(unused, deadline):
            if fault != 'early-exit': raise TimeoutError()
        host = smoke.Host.__new__(smoke.Host); host.context = context
        host.commands = SimpleNamespace(run=rss, _wait_unreaped=wait)
        ticks = [0]
        def clock():
            if fault == 'completion-deadline' and len(observations) >= 2: return 6
            ticks[0] += .01 if fault != 'deadline' else 1
            return ticks[0]
        with patch.object(smoke, 'verify_tools'), patch.object(smoke, 'check_packet'), \
             patch.object(smoke.os, 'pipe', return_value=(90, 93)), patch.object(smoke.os, 'set_blocking'), \
             patch.object(smoke.os, 'close'), patch.object(smoke.os, 'read', lambda fd, unused: queues[fd].pop(0)), \
             patch.object(smoke.subprocess, 'Popen', return_value=process) as spawn, \
             patch.object(smoke.selectors, 'DefaultSelector', Selector), patch.object(smoke.time, 'monotonic', clock), \
             patch.object(smoke, 'cancel_owned', return_value=cleanup) as cancel:
            try:
                if fault:
                    with self.assertRaises(smoke.SmokeFailure): host.child(case)
                else:
                    got = host.child(case)
                    self.assertTrue(got['rss_after_eof']); self.assertGreaterEqual(got['rss_samples'], 2)
                    self.assertEqual(got['max_rss_bytes'], 1048576)
            finally: cancel.assert_called_once_with(process)
            options = spawn.call_args.kwargs
            self.assertEqual(options['env'], smoke.ENV)
            self.assertTrue(options['close_fds']); self.assertTrue(options['start_new_session'])
            self.assertEqual(options['pass_fds'], (93,))
        for argv, options in observations:
            self.assertEqual(argv, ['/bin/ps', '-o', 'rss=', '-p', '123'])
            self.assertEqual(options, {'timeout': .25, 'cap': 128, 'allow_empty': True})

    def test_pipe_eof_still_requires_live_owned_rss_and_then_cancellation(self): self.exercise()
    def test_observation_startup_output_and_lifetime_failures_always_cancel(self):
        for fault in ('empty', 'malformed', 'zero', 'over', 'output', 'stderr', 'early-exit', 'deadline', 'completion-deadline'):
            with self.subTest(fault=fault): self.exercise(fault)


if __name__ == '__main__': unittest.main(verbosity=2)
