"""Expected-behavior regressions against the immutable reviewed source snapshot.

Only temporary ordinary files and the supplied GuestFixture are used. No kernel,
host collector, PTY, VM, network listener, or guest adapter is invoked.
"""
import json
from pathlib import Path
import sys
import tempfile
import unittest

HERE = Path(__file__).resolve().parent
SOURCE = HERE.parent
sys.path.insert(0, str(SOURCE))
sys.path.insert(0, str(SOURCE / 'tests'))
from fort_probe.controller import Controller
from fort_probe.lifecycle import OwnedTree
from support import BINDING, Clock, GuestFixture
from test_lifecycle import PROCESSES

OBSERVATIONS = []


def grant_through_twenty(controller, clock):
    identity = controller.handle({'op': 'prepare', 'sequence': 0})['identity']
    for sequence in range(1, 21):
        controller.handle({'op': 'grant' if sequence == 1 else 'heartbeat',
                           'sequence': sequence, 'identity': identity})
        clock.advance(.25)
    return identity


class ReviewRegressions(unittest.TestCase):
    def test_failed_live_canary_cannot_be_recaptured_into_success(self):
        class FailedThenValid(GuestFixture):
            failed_once = False
            def live_snapshot(self, identity, sequence, now):
                result = super().live_snapshot(identity, sequence, now)
                if not self.failed_once:
                    self.failed_once = True
                    result['controls']['C06'] = False
                return result
        with tempfile.TemporaryDirectory(prefix='fort-review-live-') as root:
            guest, clock = FailedThenValid(), Clock()
            with Controller(root, BINDING, guest, clock) as controller:
                identity = grant_through_twenty(controller, clock)
                with self.assertRaises(ValueError):
                    controller.capture_live()
                after_failure = controller.state
                recapture_succeeded = False
                try:
                    controller.capture_live()
                    recapture_succeeded = True
                except ValueError:
                    pass
                terminal = None
                if controller.state == 'granted':
                    controller.handle({'op': 'stop', 'sequence': 21, 'identity': identity})
                    terminal = controller.poll_stop()
                OBSERVATIONS.append({'case': 'failed-live-recapture',
                                     'state_after_failed_canary': after_failure,
                                     'recapture_succeeded': recapture_succeeded,
                                     'terminal': terminal})
                self.assertFalse(recapture_succeeded, 'a failed canary must irreversibly fail this attempt')

    def test_unknown_stop_facts_cannot_become_empty_tree_proof(self):
        class UnknownStop(GuestFixture):
            def begin_stop(self, identity):
                pass
            def stop_observation(self, identity):
                return {'identity': identity, 'runtime_active': None,
                        'tool_active': None, 'empty': 'unknown'}
        with tempfile.TemporaryDirectory(prefix='fort-review-stop-') as root:
            guest, clock = UnknownStop(), Clock()
            with Controller(root, BINDING, guest, clock) as controller:
                identity = grant_through_twenty(controller, clock)
                controller.handle({'op': 'stop', 'sequence': 21, 'identity': identity})
                terminal = None
                try:
                    terminal = controller.poll_stop()
                except (ValueError, TypeError, KeyError):
                    pass
                OBSERVATIONS.append({'case': 'unknown-stop-facts', 'terminal': terminal,
                                     'state': controller.state,
                                     'fixture_runtime_still_active': guest.runtime_active})
                self.assertEqual(controller.state, 'quarantined-unknown')

    def test_abort_cannot_classify_unknown_facts_as_stopped_empty(self):
        class UnknownStop(GuestFixture):
            def begin_stop(self, identity):
                pass
            def stop_observation(self, identity):
                return {'identity': identity, 'runtime_active': None,
                        'tool_active': None, 'empty': 'unknown'}
        with tempfile.TemporaryDirectory(prefix='fort-review-abort-') as root:
            guest, clock = UnknownStop(), Clock()
            with Controller(root, BINDING, guest, clock) as controller:
                grant_through_twenty(controller, clock)
                controller.abort('review fixture: uncertain observation')
                OBSERVATIONS.append({'case': 'unknown-abort-facts', 'state': controller.state,
                                     'fixture_runtime_still_active': guest.runtime_active})
                self.assertEqual(controller.state, 'quarantined-unknown')

    def test_invalid_inventory_leaves_no_root_and_valid_retry_succeeds(self):
        with tempfile.TemporaryDirectory(prefix='fort-review-root-') as parent:
            root = Path(parent).resolve() / 'owned'
            with self.assertRaises(ValueError):
                OwnedTree(root, BINDING['vm_package_uuid'], [])
            root_created_on_error = root.exists()
            corrected_retry_succeeded = False
            try:
                with OwnedTree(root, BINDING['vm_package_uuid'], PROCESSES):
                    corrected_retry_succeeded = True
            except FileExistsError:
                pass
            OBSERVATIONS.append({'case': 'invalid-inventory',
                                 'root_created_on_error': root_created_on_error,
                                 'corrected_retry_succeeded': corrected_retry_succeeded})
            self.assertFalse(root_created_on_error, 'pure input rejection must precede root creation')
            self.assertTrue(corrected_retry_succeeded)
