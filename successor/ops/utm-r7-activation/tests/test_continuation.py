import copy
import importlib
from pathlib import Path
import sys
import unittest
from types import SimpleNamespace
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
sys.path.insert(0, str(Path(__file__).resolve().parent))
from test_smoke import CASES, ENV, result
continuation = importlib.import_module('continue')


class Journal:
    def __init__(self, context):
        self.records = {'attempt.json': {}, 'complete.json': {
            'packet_sha': context.packet_sha, 'source_seal': context.active_sha,
            'cases': [c['name'] for c in CASES], 'parent_before': {'core': [0, 100]},
            'parent_after': {'core': [0, 100]}, 'parent_limits_unchanged': True,
            'status': 'passed', 'completed_monotonic': 10, 'completed_epoch': 20}}
        for case in CASES: self.records[case['name'].replace('_', '-') + '.json'] = result(case)
    def names(self): return list(self.records)
    def read(self, name): return copy.deepcopy(self.records[name])
    def record(self, name, value):
        if name in self.records: raise FileExistsError(name)
        self.records[name] = copy.deepcopy(value)


class ContinuationTests(unittest.TestCase):
    def setUp(self):
        self.context = SimpleNamespace(cases=CASES, packet_sha='a' * 64, active_sha='b' * 64,
            identities={'python': 'python-fixture', 'node': 'node-fixture'},
            manifest={'acquisition': {'argv': ['/fixed/python', '/fixed/acquire', 'b' * 64]}})
        self.journal = Journal(self.context); self.calls = []
    def preflight(self): self.calls.append('preflight'); return {'fresh': True}
    def execute(self, argv, env): self.calls.append(('execute', argv, env))
    def run_packet(self):
        return continuation.run_continuation(self.context, self.journal, self.preflight, self.execute,
                                             monotonic=lambda: 11, epoch=lambda: 21)

    def test_success_runs_fresh_preflight_then_one_exact_exec(self):
        self.run_packet()
        self.assertEqual(self.calls, ['preflight', ('execute', self.context.manifest['acquisition']['argv'], ENV)])
        self.assertIn('continue-begun.json', self.journal.records)
        self.assertIn('dispatch.json', self.journal.records)
        with self.assertRaises(ValueError): self.run_packet()
        self.assertEqual(len(self.calls), 2)

    def test_failure_or_incomplete_or_foreign_smoke_never_reaches_preflight(self):
        mutations = [lambda j: j.records.pop('complete.json'),
                     lambda j: j.records.update({'failure.json': {}}),
                     lambda j: j.records['complete.json'].update(source_seal='foreign'),
                     lambda j: j.records['complete.json'].update(parent_after={'core': [0, 101]}),
                     lambda j: j.records['registry.json'].update(permissions={}),
                     lambda j: j.records['ubuntu-inventory.json'].update(group_absent=False),
                     lambda j: j.records['complete.json'].update(completed_monotonic=-400),
                     lambda j: j.records['complete.json'].update(completed_epoch=500)]
        for mutation in mutations:
            with self.subTest(mutation=mutation):
                self.journal = Journal(self.context); mutation(self.journal)
                with self.assertRaises((ValueError, KeyError)): self.run_packet()
                self.assertEqual(self.calls, [])

    def test_preflight_and_exec_failure_consume_continuation(self):
        for where in ('preflight', 'execute'):
            self.journal = Journal(self.context); self.calls = []
            def fail(*unused): raise OSError('synthetic failure')
            with self.assertRaises(OSError):
                continuation.run_continuation(self.context, self.journal,
                    fail if where == 'preflight' else self.preflight,
                    fail if where == 'execute' else self.execute, monotonic=lambda: 11, epoch=lambda: 21)
            self.assertIn('continue-failure.json', self.journal.records)
            with self.assertRaises(ValueError): self.run_packet()


if __name__ == '__main__': unittest.main(verbosity=2)
