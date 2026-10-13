import copy
from pathlib import Path
import sys
import unittest
from types import SimpleNamespace
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from smoke import run_smoke, validate_result, inspect_environment

CASES = [
    {'name': 'acquire', 'runtime': 'python', 'cpu': 120, 'file': 805306368, 'rss': 268435456},
    {'name': 'registry', 'runtime': 'node', 'cpu': 80, 'file': 0, 'rss': 1073741824},
    {'name': 'verifier_archive', 'runtime': 'python', 'cpu': 25, 'file': 25165824, 'rss': 268435456},
    {'name': 'ubuntu_inventory', 'runtime': 'python', 'cpu': 280, 'file': 0, 'rss': 3221225472},
]
ENV = {'PATH': '/usr/bin:/bin', 'LANG': 'C', 'LC_ALL': 'C', 'TZ': 'UTC'}


def result(case):
    return {'case': case['name'], 'runtime': case['runtime'],
            'limits': {'core': [0, 0], 'cpu': [case['cpu'], case['cpu'] + 1],
                       'file': [case['file'], case['file']], 'descriptors': [64, 64]},
            'environment': ENV, 'launcher_keys': ['SDKROOT'] if case['runtime'] == 'python' else [],
            'runtime_identity': 'python-fixture' if case['runtime'] == 'python' else 'node-fixture',
            'permissions': {'read_allowed': True, 'read_denied': True, 'global_read': False,
                            'write': False, 'child': False, 'worker': False} if case['runtime'] == 'node' else None,
            'pid': 42, 'rss_samples': 2, 'rss_after_eof': True, 'max_rss_bytes': 1024,
            'cancel_sent': True, 'returncode': -9, 'reaped': True, 'child_absent': True, 'group_absent': True}


class Journal:
    def __init__(self): self.records = {}
    def record(self, name, value):
        if name in self.records: raise FileExistsError(name)
        self.records[name] = copy.deepcopy(value)


class Host:
    def __init__(self, fault=None): self.calls = []; self.fault = fault; self.snapshots = 0
    def limits(self):
        self.snapshots += 1
        return {'core': [0, 999 if self.fault == 'parent' and self.snapshots > 1 else 1000]}
    def child(self, case):
        self.calls.append(case['name'])
        if case['name'] == self.fault: raise ValueError('fixture startup failure')
        return result(case)


class SmokeTests(unittest.TestCase):
    def setUp(self):
        self.context = SimpleNamespace(cases=CASES, packet_sha='a' * 64, active_sha='b' * 64,
                                      identities={'python': 'python-fixture', 'node': 'node-fixture'})

    def test_all_distinct_profiles_complete_before_success(self):
        host, journal = Host(), Journal()
        receipt = run_smoke(host, journal, self.context, lambda: 0)
        self.assertEqual(host.calls, [c['name'] for c in CASES])
        self.assertTrue(receipt['parent_limits_unchanged'])
        self.assertEqual(journal.records['complete.json'], receipt)

    def test_every_failure_stops_later_children_and_prevents_acceptance(self):
        for index, case in enumerate(CASES):
            with self.subTest(case=case['name']):
                host, journal = Host(case['name']), Journal()
                with self.assertRaises(ValueError): run_smoke(host, journal, self.context, lambda: 0)
                self.assertEqual(host.calls, [c['name'] for c in CASES[:index + 1]])
                self.assertIn('failure.json', journal.records)
                self.assertNotIn('complete.json', journal.records)

    def test_parent_limit_change_or_total_deadline_is_not_success(self):
        for fault in ('parent', 'deadline'):
            host, journal = Host(fault), Journal(); ticks = iter([0, 31])
            with self.assertRaises((ValueError, TimeoutError)):
                run_smoke(host, journal, self.context, (lambda: next(ticks)) if fault == 'deadline' else lambda: 0)
            self.assertNotIn('complete.json', journal.records)

    def test_false_limit_rss_runtime_permission_or_cleanup_evidence_fails(self):
        case = CASES[1]
        for key, value in [('limits', {}), ('max_rss_bytes', 0), ('max_rss_bytes', case['rss'] + 1),
                           ('rss_samples', 1), ('rss_after_eof', False), ('cancel_sent', False),
                           ('returncode', 0), ('reaped', False), ('child_absent', False), ('group_absent', False),
                           ('runtime_identity', 'different'), ('permissions', {}), ('environment', {})]:
            with self.subTest(key=key, value=value):
                bad = result(case); bad[key] = value
                with self.assertRaises(ValueError): validate_result(bad, case, self.context.identities)

    def test_environment_never_reports_unknown_values(self):
        env = dict(ENV, SDKROOT='synthetic sdk metadata')
        self.assertEqual(inspect_environment(env, 'python'), (ENV, ['SDKROOT']))
        with self.assertRaises(ValueError): inspect_environment(env, 'node')
        with self.assertRaises(ValueError): inspect_environment(dict(ENV, UNKNOWN='do-not-log'), 'python')
        with self.assertRaises(ValueError): inspect_environment(dict(ENV, PATH='different'), 'python')


if __name__ == '__main__': unittest.main(verbosity=2)
