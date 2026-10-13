import json
from pathlib import Path
import tempfile
import unittest
from fort_probe.controller import Controller
from support import BINDING, Clock, GuestFixture


class FaultTests(unittest.TestCase):
    def test_idle_without_any_further_request_stops_and_cannot_resume(self):
        with tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as root:
            guest, clock = GuestFixture(), Clock()
            with Controller(root, BINDING, guest, clock) as controller:
                identity = controller.handle({'op': 'prepare', 'sequence': 0})['identity']
                controller.handle({'op': 'grant', 'sequence': 1, 'identity': identity})
                clock.advance(1.01)
                controller.tick()
                self.assertTrue(guest.current()['empty'])
                with self.assertRaises(ValueError): controller.handle({'op': 'heartbeat', 'sequence': 2, 'identity': identity})

    def test_crash_after_persisted_consume_never_retries_start(self):
        with tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as root:
            guest, clock = GuestFixture(), Clock()
            guest.fail_after_start = True
            with Controller(root, BINDING, guest, clock) as controller:
                with self.assertRaises(TimeoutError): controller.handle({'op': 'prepare', 'sequence': 0})
                self.assertTrue(Path(root, 'consumed.json').exists())
                self.assertEqual(controller.inspect()['state'], 'quarantined-unknown')
            guest.fail_after_start = False
            with Controller(root, BINDING, guest, clock) as controller:
                with self.assertRaises(ValueError): controller.handle({'op': 'prepare', 'sequence': 0})
                self.assertFalse(guest.tool_active)

    def test_late_or_mismatched_empty_tree_proof_never_marks_stop_complete(self):
        for fault in ('slow', 'identity', 'remaining'):
            clock = Clock()
            class BadStop(GuestFixture):
                def stop_observation(self, identity):
                    observed = super().stop_observation(identity)
                    if fault == 'slow': clock.advance(5.01)
                    if fault == 'identity': observed['identity']['invocation_id'] = 'f' * 32
                    return observed
            with self.subTest(fault=fault), tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as root:
                guest = BadStop()
                guest.leave_descendant = fault == 'remaining'
                with Controller(root, BINDING, guest, clock) as controller:
                    identity = controller.handle({'op': 'prepare', 'sequence': 0})['identity']
                    for seq in range(1, 21):
                        controller.handle({'op': 'grant' if seq == 1 else 'heartbeat', 'sequence': seq, 'identity': identity})
                        clock.advance(.25)
                    controller.handle({'op': 'stop', 'sequence': 21, 'identity': identity})
                    if fault == 'remaining':
                        self.assertIsNone(controller.poll_stop())
                        clock.advance(5.01)
                    with self.assertRaises(ValueError): controller.poll_stop()
                    self.assertEqual(controller.inspect()['state'], 'quarantined-unknown')

    def test_corrupt_state_releases_exclusive_lock_but_never_restores_authority(self):
        with tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as root:
            Path(root, 'consumed.json').write_text('{}')
            Path(root, 'consumed.json').chmod(0o600)
            Path(root, 'state.json').write_text('invalid')
            Path(root, 'state.json').chmod(0o600)
            with self.assertRaises(ValueError): Controller(root, BINDING, GuestFixture(), Clock())
            Path(root, 'state.json').write_text(json.dumps({'state': 'quarantined-unknown'}))
            with Controller(root, BINDING, GuestFixture(), Clock()) as controller:
                self.assertTrue(controller.inspect()['consumed'])

    def test_slow_prepare_is_uncertain_and_can_never_release_tool(self):
        clock = Clock()
        class SlowGuest(GuestFixture):
            def prepare(self, binding):
                result = super().prepare(binding)
                clock.advance(.501)
                return result
        with tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as root:
            guest = SlowGuest()
            with Controller(root, BINDING, guest, clock) as controller:
                with self.assertRaises(ValueError): controller.handle({'op': 'prepare', 'sequence': 0})
                self.assertFalse(guest.tool_active)
                with self.assertRaises(ValueError): controller.handle({'op': 'prepare', 'sequence': 0})
