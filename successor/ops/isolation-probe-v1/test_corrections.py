"""Safe readiness/control mocks and the installed SSH config parser; no connection."""
import importlib.util
import json
import pathlib
import subprocess
import stat
import tempfile
import time
import unittest
from unittest import mock
from types import SimpleNamespace

ROOT = pathlib.Path(__file__).resolve().parent


def module(name, relative):
    spec = importlib.util.spec_from_file_location(name, ROOT / relative)
    loaded = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(loaded)
    return loaded


class RuntimeReadiness(unittest.TestCase):
    def notifications(self, identity, grant, invocation):
        runtime = module('runtime_under_test', 'guest/runtime.py')
        events = []
        notify = mock.Mock()
        notify.send.side_effect = events.append
        with tempfile.TemporaryDirectory() as directory:
            runtime.ROOT = pathlib.Path(directory)
            (runtime.ROOT / 'heartbeat.json').write_text(json.dumps({'monotonic': time.monotonic()}))
            for name, value in [('identity', identity), ('grant', grant)]:
                if value is not None:
                    (runtime.ROOT / (name + '.json')).write_text(json.dumps(value))
            with mock.patch.object(runtime.socket, 'socket', return_value=notify), \
                 mock.patch.dict(runtime.os.environ, {'NOTIFY_SOCKET': '/mock-only', 'INVOCATION_ID': invocation}, clear=True), \
                 mock.patch.object(runtime.time, 'sleep', side_effect=RuntimeError('test loop stop')):
                with self.assertRaises((RuntimeError, FileNotFoundError)):
                    runtime.main()
        return events

    def test_replacement_with_stale_identity_and_grant_never_reports_ready(self):
        stale = {'invocationId': 'original'}
        self.assertEqual([], self.notifications(stale, stale, 'replacement'))

    def test_replacement_before_grant_never_reports_ready(self):
        self.assertEqual([], self.notifications({'invocationId': 'original'}, None, 'replacement'))

    def test_grant_without_matching_identity_never_reports_ready(self):
        self.assertEqual([], self.notifications(None, {'invocationId': 'original'}, 'replacement'))

    def test_original_invocation_can_report_ready(self):
        events = self.notifications({'invocationId': 'original'}, None, 'original')
        self.assertTrue(events[0].startswith(b'READY=1'))

    def test_initial_barrier_without_identity_can_report_ready(self):
        events = self.notifications(None, None, 'original')
        self.assertTrue(events[0].startswith(b'READY=1'))


class ControllerGrant(unittest.TestCase):
    def grant_calls(self, active_state):
        controller = module('controller_under_test', 'guest/controller.py')
        identity = {'invocationId': 'original'}
        requests = [{'op': 'prepare'}, {'op': 'grant', 'identity': identity, 'sequence': 1}]
        with mock.patch.object(controller.os, 'geteuid', return_value=0), \
             mock.patch.object(controller.sys, 'argv', ['controller.py']), \
             mock.patch('builtins.open', mock.mock_open()), \
             mock.patch.object(controller.fcntl, 'flock'), \
             mock.patch.object(controller, 'messages', return_value=iter(requests)), \
             mock.patch.object(controller, 'prepare', return_value=identity), \
             mock.patch.object(controller, 'exact', return_value={'ActiveState': active_state}), \
             mock.patch.object(controller, 'heartbeat'), \
             mock.patch.object(controller, 'atomic'), \
             mock.patch.object(controller, 'emit'), \
             mock.patch.object(controller, 'stop_owned'), \
             mock.patch.object(controller, 'systemctl') as calls:
            try:
                controller.main()
            except RuntimeError:
                pass
        return calls.call_args_list

    def test_inactive_original_cannot_release_tool(self):
        self.assertNotIn(mock.call('start', '--no-block', 'fort-tool.service'), self.grant_calls('inactive'))

    def test_active_original_can_release_tool(self):
        self.assertIn(mock.call('start', '--no-block', 'fort-tool.service'), self.grant_calls('active'))


class SSHKnownHosts(unittest.TestCase):
    def option(self):
        host = module('host_under_test', 'host-session.py')
        captured = []

        class Captured(Exception):
            pass

        def capture(args, **kwargs):
            captured.extend(args)
            raise Captured()

        info = SimpleNamespace(st_mode=stat.S_IFREG | 0o600, st_uid=host.os.getuid())
        with mock.patch.object(pathlib.Path, 'lstat', return_value=info), \
             mock.patch.object(host.subprocess, 'Popen', side_effect=capture):
            with self.assertRaises(Captured):
                host.main()
        return next(value for value in captured if value.startswith('UserKnownHostsFile='))

    def parse(self, option, extra):
        return subprocess.run(['/usr/bin/ssh', '-G', '-F', '/dev/null',
            '-o', 'CanonicalizeHostname=no', '-o', 'IdentityFile=none', '-o', 'IdentityAgent=none',
            '-o', 'GlobalKnownHostsFile=/dev/null', '-o', option + ' ' + ' '.join('/unused-' + str(i) for i in range(extra)),
            '127.0.0.1'], capture_output=True, text=True,
            env={'PATH': '/usr/bin:/bin', 'LANG': 'C', 'LC_ALL': 'C'}, timeout=5)

    def test_application_support_path_occupies_one_known_hosts_slot(self):
        result = self.parse(self.option(), 31)
        self.assertEqual(0, result.returncode, result.stderr)

    def test_parser_control_rejects_more_than_32_known_hosts_files(self):
        result = self.parse('UserKnownHostsFile="/single path/known_hosts"', 32)
        self.assertNotEqual(0, result.returncode)
        self.assertIn('too many userknownhostsfile entries', result.stderr)


if __name__ == '__main__':
    unittest.main()
