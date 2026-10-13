import tempfile
import unittest
from fort_probe.controller import Controller
from support import BINDING, Clock, GuestFixture


class PhaseTests(unittest.TestCase):
    def test_controls_follow_process_lifetime_and_live_capture_does_not_renew_lease(self):
        with tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as root:
            guest, clock = GuestFixture(), Clock()
            with Controller(root, BINDING, guest, clock) as controller:
                self.assertTrue(controller.preflight()['ready'])
                self.assertFalse(guest.runtime_active)
                self.assertFalse(guest.tool_active)
                identity = controller.handle({'op': 'prepare', 'sequence': 0})['identity']
                self.assertTrue(guest.runtime_active)
                self.assertFalse(guest.tool_active)
                controller.handle({'op': 'grant', 'sequence': 1, 'identity': identity})
                clock.advance(.6)
                live = controller.capture_live()
                self.assertEqual(live['sequence'], 1)
                self.assertEqual(live['identity'], identity)
                self.assertEqual(set(live['processes']), {'runtime', 'tool', 'descendant'})
                clock.advance(.41)
                with self.assertRaises(ValueError):
                    controller.handle({'op': 'heartbeat', 'sequence': 2, 'identity': identity})

    def test_failed_runtime_controls_or_early_tool_prevent_grant(self):
        for fault in ('provider', 'early-tool'):
            class BadGuest(GuestFixture):
                def prepared_controls(self, identity):
                    result = super().prepared_controls(identity)
                    result['provider_literal_read'] = fault != 'provider'
                    result['tool_active'] = fault == 'early-tool'
                    return result
            with self.subTest(fault=fault), tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as root:
                guest, clock = BadGuest(), Clock()
                with Controller(root, BINDING, guest, clock) as controller:
                    identity = controller.handle({'op': 'prepare', 'sequence': 0})['identity']
                    with self.assertRaises(ValueError):
                        controller.handle({'op': 'grant', 'sequence': 1, 'identity': identity})
                    self.assertFalse(guest.tool_active)

    def test_missing_stale_replayed_or_unbound_live_evidence_never_passes(self):
        for fault in ('missing', 'stale', 'replayed', 'pid', 'namespace', 'cgroup'):
            class BadGuest(GuestFixture):
                def live_snapshot(self, identity, sequence, now):
                    result = super().live_snapshot(identity, sequence, now)
                    if fault == 'stale': result['captured_at'] -= 1
                    if fault == 'replayed': result['sequence'] -= 1
                    if fault == 'pid': result['processes']['descendant']['start_ticks'] = 0
                    if fault == 'namespace': result['processes']['tool']['namespaces'] = result['processes']['runtime']['namespaces']
                    if fault == 'cgroup': result['processes']['descendant']['cgroup'] = '/unowned'
                    return result
            with self.subTest(fault=fault), tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as root:
                guest, clock = BadGuest(), Clock()
                with Controller(root, BINDING, guest, clock) as controller:
                    identity = controller.handle({'op': 'prepare', 'sequence': 0})['identity']
                    for sequence in range(1, 21):
                        controller.handle({'op': 'grant' if sequence == 1 else 'heartbeat', 'sequence': sequence, 'identity': identity})
                        clock.advance(.25)
                    if fault != 'missing':
                        with self.assertRaises(ValueError): controller.capture_live()
                    controller.handle({'op': 'stop', 'sequence': 21, 'identity': identity})
                    self.assertFalse(controller.poll_stop()['live_evidence_proven'])

    def test_valid_live_evidence_is_single_use_and_survives_only_for_this_attempt(self):
        with tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as root:
            guest, clock = GuestFixture(), Clock()
            with Controller(root, BINDING, guest, clock) as controller:
                identity = controller.handle({'op': 'prepare', 'sequence': 0})['identity']
                for sequence in range(1, 21):
                    controller.handle({'op': 'grant' if sequence == 1 else 'heartbeat', 'sequence': sequence, 'identity': identity})
                    clock.advance(.25)
                controller.capture_live()
                controller.handle({'op': 'stop', 'sequence': 21, 'identity': identity})
                self.assertTrue(controller.poll_stop()['live_evidence_proven'])
                with self.assertRaises(ValueError): controller.capture_live()
