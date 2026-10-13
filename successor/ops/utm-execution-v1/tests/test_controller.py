import tempfile
import unittest
from fort_probe.controller import Controller
from support import BINDING, Clock, GuestFixture


class ControllerTests(unittest.TestCase):
    def test_one_nominal_grant_and_stop_survive_restart_as_consumed(self):
        with tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as root:
            guest, clock = GuestFixture(), Clock()
            with Controller(root, BINDING, guest, clock) as controller:
                prepared = controller.handle({'op': 'prepare', 'sequence': 0})
                identity = prepared['identity']
                self.assertEqual(prepared['event'], 'prepared')
                self.assertFalse(guest.current()['tool_active'])

                for sequence in range(1, 22):
                    op = 'grant' if sequence == 1 else 'stop' if sequence == 21 else 'heartbeat'
                    receipt = controller.handle({'op': op, 'sequence': sequence, 'identity': identity})
                    clock.advance(0.25)
                self.assertEqual(receipt['event'], 'stopping')
                self.assertEqual(controller.poll_stop()['event'], 'owned-attempt-empty')
                self.assertTrue(guest.current()['empty'])
            with Controller(root, BINDING, guest, clock) as reopened:
                self.assertTrue(reopened.inspect()['consumed'])
                with self.assertRaises(ValueError):
                    reopened.handle({'op': 'prepare', 'sequence': 0})

    def test_restarted_controller_cannot_grant_an_old_prepared_attempt(self):
        with tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as root:
            guest, clock = GuestFixture(), Clock()
            with Controller(root, BINDING, guest, clock) as first:
                identity = first.handle({'op': 'prepare', 'sequence': 0})['identity']
            with Controller(root, BINDING, guest, clock) as restarted:
                with self.assertRaises(ValueError):
                    restarted.handle({'op': 'grant', 'sequence': 1, 'identity': identity})
                self.assertFalse(guest.current()['tool_active'])

    def test_expired_prepare_lease_cannot_release_tool_work(self):
        with tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as root:
            guest, clock = GuestFixture(), Clock()
            with Controller(root, BINDING, guest, clock) as controller:
                identity = controller.handle({'op': 'prepare', 'sequence': 0})['identity']
                clock.advance(4.01)
                with self.assertRaises(ValueError):
                    controller.handle({'op': 'grant', 'sequence': 1, 'identity': identity})
                self.assertFalse(guest.current()['tool_active'])

    def test_idle_or_changed_invocation_quarantines_and_cannot_resume(self):
        for problem in ('idle', 'changed-invocation', 'malformed'):
            with self.subTest(problem=problem), tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as root:
                guest, clock = GuestFixture(), Clock()
                with Controller(root, BINDING, guest, clock) as controller:
                    identity = controller.handle({'op': 'prepare', 'sequence': 0})['identity']
                    controller.handle({'op': 'grant', 'sequence': 1, 'identity': identity})
                    if problem == 'idle':
                        clock.advance(1.01)
                    elif problem == 'changed-invocation':
                        guest.identity['invocation_id'] = '2' * 32
                    message = {'op': 'heartbeat', 'sequence': 2, 'identity': identity}
                    if problem == 'malformed':
                        message['arbitrary'] = 'forbidden'
                    with self.assertRaises(ValueError):
                        controller.handle(message)
                    self.assertIn(controller.inspect()['state'], ('stopped-empty', 'quarantined-unknown'))
                    with self.assertRaises(ValueError):
                        controller.handle({'op': 'heartbeat', 'sequence': 2, 'identity': identity})

    def test_boolean_sequence_or_identity_number_is_not_an_integer_grant(self):
        for mutation in ('prepare-sequence', 'identity-epoch'):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as root:
                guest, clock = GuestFixture(), Clock()
                with Controller(root, BINDING, guest, clock) as controller:
                    if mutation == 'prepare-sequence':
                        with self.assertRaises(ValueError):
                            controller.handle({'op': 'prepare', 'sequence': False})
                    else:
                        identity = controller.handle({'op': 'prepare', 'sequence': 0})['identity']
                        identity['epoch'] = True
                        with self.assertRaises(ValueError):
                            controller.handle({'op': 'grant', 'sequence': 1, 'identity': identity})
                    self.assertFalse(guest.current()['tool_active'])

    def test_readiness_identity_substitution_is_rejected_before_grant(self):
        class WrongGuest(GuestFixture):
            def prepare(self, binding):
                result = super().prepare(binding)
                result['generation'] = 2
                return result
        with tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as root:
            guest, clock = WrongGuest(), Clock()
            with Controller(root, BINDING, guest, clock) as controller:
                with self.assertRaises(ValueError):
                    controller.handle({'op': 'prepare', 'sequence': 0})
                self.assertTrue(controller.inspect()['consumed'])
                self.assertFalse(guest.current()['tool_active'])

    def test_stop_acknowledges_promptly_then_proves_two_second_teardown(self):
        clock = Clock()
        class DelayedGuest(GuestFixture):
            def begin_stop(self, identity):
                self.stop_at = clock()

            def stop_observation(self, identity):
                if clock() - self.stop_at >= 2:
                    self.stop(identity)
                return self.current()
        with tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as root:
            guest = DelayedGuest()
            with Controller(root, BINDING, guest, clock) as controller:
                identity = controller.handle({'op': 'prepare', 'sequence': 0})['identity']
                for sequence in range(1, 21):
                    controller.handle({'op': 'grant' if sequence == 1 else 'heartbeat',
                                       'sequence': sequence, 'identity': identity})
                    clock.advance(0.25)
                before = clock()
                receipt = controller.handle({'op': 'stop', 'sequence': 21, 'identity': identity})
                self.assertEqual(receipt['event'], 'stopping')
                self.assertLess(clock() - before, 0.5)
                clock.advance(1)
                self.assertIsNone(controller.poll_stop())
                clock.advance(1)
                self.assertEqual(controller.poll_stop()['event'], 'owned-attempt-empty')
                self.assertTrue(guest.current()['empty'])
                with self.assertRaises(ValueError):
                    controller.poll_stop()
