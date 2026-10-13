import tempfile
import unittest
from fort_probe.broker import Broker
from fort_probe.controller import Controller
from fort_probe.serial import encode
from support import BINDING,Clock,GuestFixture


class BrokerTests(unittest.TestCase):
    def test_inspection_does_not_renew_lease_and_timer_runs_without_input(self):
        with tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as root:
            guest,clock=GuestFixture(),Clock()
            with Controller(root,BINDING,guest,clock) as controller:
                broker=Broker(controller,clock)
                response=broker.receive('control',encode({'op':'prepare','sequence':0}))[0][1]
                identity=response['identity']
                broker.receive('control',encode({'op':'grant','sequence':1,'identity':identity}))
                clock.advance(.6)
                broker.receive('inspection',encode({'op':'live','identity':identity}))
                clock.advance(.41); broker.tick()
                self.assertTrue(guest.current()['empty'])

    def test_two_stop_responses_are_emitted_on_control_channel_exactly_once(self):
        with tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as root:
            guest,clock=GuestFixture(),Clock()
            with Controller(root,BINDING,guest,clock) as controller:
                broker=Broker(controller,clock)
                identity=broker.receive('control',encode({'op':'prepare','sequence':0}))[0][1]['identity']
                for seq in range(1,21):
                    broker.receive('control',encode({'op':'grant' if seq==1 else 'heartbeat','sequence':seq,'identity':identity})); clock.advance(.25)
                self.assertEqual(broker.receive('control',encode({'op':'stop','sequence':21,'identity':identity}))[0][1]['event'],'stopping')
                self.assertEqual(broker.tick()[0][1]['event'],'owned-attempt-empty')
                self.assertEqual(broker.tick(),[])

    def test_arbitrary_inspection_arguments_and_cross_channel_control_abort(self):
        for message in ({'op':'preflight','path':'/etc/passwd'},{'op':'grant','sequence':1,'identity':{}}):
            with tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as root:
                with Controller(root,BINDING,GuestFixture(),Clock()) as controller:
                    broker=Broker(controller,Clock())
                    with self.assertRaises(ValueError): broker.receive('inspection',encode(message))
                    with self.assertRaises(ValueError): broker.receive('control',encode({'op':'prepare','sequence':0}))
