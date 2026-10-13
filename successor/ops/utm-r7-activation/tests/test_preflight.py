"""Fresh preflight observes only injected synthetic directories/tools."""
import importlib
from pathlib import Path
import sys
import unittest
from types import SimpleNamespace
from unittest.mock import patch
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
sys.path.insert(0, str(Path(__file__).resolve().parent))
from test_smoke import CASES
continuation = importlib.import_module('continue')


class Chain:
    fd = 99
    entries = [('synthetic', 99, {'device': 1, 'inode': 2, 'uid': 3, 'mode': 448, 'kind': 16384})]
    def __init__(self, path):
        if path != Path('/synthetic'): raise ValueError('unexpected path')
    def __enter__(self): return self
    def __exit__(self, *unused): pass
    def check(self): pass


class PreflightTests(unittest.TestCase):
    def exercise(self, fault=None):
        context = SimpleNamespace(packet_sha='a' * 64, active_sha='b' * 64, cases=CASES,
            identities={'python': '.'.join(map(str,sys.version_info[:3]))+'@'+str(Path(sys.executable).resolve())},
            manifest={'acquisition': {'root': '/synthetic/new-R7', 'logical_byte_budget': 839843840}})
        stats = []; commands = []
        def stat(name, *, dir_fd, follow_symlinks):
            stats.append(name)
            self.assertEqual((name, dir_fd, follow_symlinks), ('new-R7', 99, False))
            if fault == 'existing' or (fault == 'appeared' and len(stats) > 1): return SimpleNamespace()
            raise FileNotFoundError()
        def run(argv, **unused):
            commands.append(argv)
            if argv[0] == '/usr/sbin/sysctl': return (b'bad' if fault == 'physical' else b'68719476736\n'), b''
            self.assertEqual(argv, ['/usr/bin/memory_pressure', '-Q'])
            return (b'bad' if fault == 'pressure' else b'System-wide memory free percentage: 80%\n'), b''
        def module(name):
            if name == 'bounded': return SimpleNamespace(Commands=lambda guard: SimpleNamespace(run=run))
            if name == 'lifecycle': return SimpleNamespace(DirectoryChain=Chain)
            raise AssertionError(name)
        def tool_pins(unused):
            if fault == 'tools': raise ValueError('synthetic pin mismatch')
        with patch.object(continuation, 'check_packet'), patch.object(continuation, 'require_active'), \
             patch.object(continuation, 'verify_tools', tool_pins), patch.object(continuation, 'inspect_environment'), \
             patch.object(continuation, 'source_module', module), patch.object(continuation.os, 'stat', stat), \
             patch.object(continuation.os, 'fstatvfs', return_value=SimpleNamespace(f_bavail=1 if fault == 'disk' else 10**12, f_frsize=1)):
            if fault:
                with self.assertRaises(ValueError): continuation.fresh_preflight(context)
            else:
                got = continuation.fresh_preflight(context)
                self.assertTrue(got['root_absent']); self.assertFalse(got['capacity_reserved'])
                self.assertEqual(got['memory_free_percentage'], 80)
                self.assertEqual(len(stats), 2); self.assertEqual(len(commands), 2)
        if fault in ('existing', 'disk', 'tools'): self.assertEqual(commands, [])

    def test_current_capacity_pins_and_two_nofollow_absence_checks(self): self.exercise()
    def test_existing_or_appearing_root_or_changed_tools_or_invalid_capacity_fails(self):
        for fault in ('existing', 'appeared', 'tools', 'disk', 'physical', 'pressure'):
            with self.subTest(fault=fault): self.exercise(fault)


if __name__ == '__main__': unittest.main(verbosity=2)
