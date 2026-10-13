"""Resource API fixtures only; no child, real setter or operation-root access."""
import json
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import patch
from fort_probe import inspection_workers as workers
from fort_probe.inspection import ROOT
from fort_probe.inspection_guard import INSPECTION_ENABLED


class ResourceFixture:
    RLIMIT_CORE = 4
    RLIMIT_CPU = 0
    RLIMIT_RSS = RLIMIT_AS = 5
    RLIMIT_FSIZE = 1
    RLIMIT_NOFILE = 8

    def __init__(self, failure=None):
        self.failure = failure
        self.calls = []

    def setrlimit(self, identifier, values):
        self.calls.append((identifier, values))
        if identifier == self.failure:
            raise ValueError('current limit exceeds maximum limit')


def expected_calls(profile, platform):
    calls = [(4, (0, 0)), (0, (profile['cpu'], profile['cpu'] + 1))]
    if platform != 'darwin':
        calls.append((5, (profile['rss'], profile['rss'])))
    return calls + [(1, (profile['file'], profile['file'])), (8, (64, 64))]


class MacOSMemoryPolicyTests(unittest.TestCase):
    def apply(self, profile, platform, resource):
        with patch.object(workers, 'resource', resource), \
             patch.object(workers, 'sys', SimpleNamespace(platform=platform), create=True):
            workers.child_limits(profile)

    def test_macos_never_calls_aliased_rss_or_as_and_keeps_other_limits(self):
        for operation, profile in workers.PROFILES.items():
            with self.subTest(operation=operation):
                resource = ResourceFixture(failure=ResourceFixture.RLIMIT_RSS)
                self.apply(profile, 'darwin', resource)
                self.assertEqual(resource.calls, expected_calls(profile, 'darwin'))

    def test_linux_keeps_all_five_setters_for_every_profile(self):
        for operation, profile in workers.PROFILES.items():
            with self.subTest(operation=operation):
                resource = ResourceFixture()
                self.apply(profile, 'linux', resource)
                self.assertEqual(resource.calls, expected_calls(profile, 'linux'))

    def test_setup_failures_propagate_without_fallback_or_later_calls(self):
        for platform in ('darwin', 'linux'):
            for operation, profile in workers.PROFILES.items():
                calls = expected_calls(profile, platform)
                for index, (identifier, _) in enumerate(calls):
                    with self.subTest(platform=platform, operation=operation, limit=identifier):
                        resource = ResourceFixture(failure=identifier)
                        with self.assertRaisesRegex(ValueError, 'current limit exceeds maximum limit'):
                            self.apply(profile, platform, resource)
                        self.assertEqual(resource.calls, calls[:index + 1])

    def test_sampled_rss_thresholds_are_unchanged(self):
        self.assertEqual({operation: profile['rss'] for operation, profile in workers.PROFILES.items()}, {
            'acquire': 256 * 1024**2, 'registry': 1024**3,
            'verifier_archive': 256 * 1024**2, 'publisher': 1024**3,
            'ubuntu_inventory': 3 * 1024**3, 'ubuntu_selected': 3 * 1024**3})

    def test_revision_is_disabled_and_uses_a_distinct_proposed_root(self):
        self.assertIs(INSPECTION_ENABLED, False)
        with self.assertRaisesRegex(RuntimeError, 'disabled'):
            workers.Worker('a' * 64)
        expected = '/Users/tobiasgunn/Library/Application Support/FortUTMInputInspectionR7'
        self.assertEqual(str(ROOT), expected)
        operation = json.loads((Path(__file__).parents[1] / 'activation-operation.json').read_bytes())
        self.assertEqual(operation['root'], expected)
        self.assertIs(operation['activation_source_enabled'], False)
        self.assertIs(operation['input_inspection_authorized'], False)
