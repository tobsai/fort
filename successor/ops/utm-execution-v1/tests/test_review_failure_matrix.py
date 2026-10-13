import tempfile
from pathlib import Path
import unittest
from fort_probe.controller import Controller
from support import BINDING,Clock,GuestFixture


class ReviewFailureMatrix(unittest.TestCase):
    def test_all_failed_captures_irreversibly_revoke_grant_and_survive_reopen(self):
        for fault in ('canary','malformed','timeout','persist'):
            clock=Clock()
            class BadLive(GuestFixture):
                def live_snapshot(self,*args):
                    value=super().live_snapshot(*args)
                    if fault=='canary': value['controls']['C06']=False
                    if fault=='malformed': value.pop('processes')
                    if fault=='timeout': clock.advance(.501)
                    return value
            with self.subTest(fault=fault),tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as root:
                guest=BadLive()
                with Controller(root,BINDING,guest,clock) as controller:
                    identity=controller.handle({'op':'prepare','sequence':0})['identity']
                    controller.handle({'op':'grant','sequence':1,'identity':identity})
                    if fault=='persist': Path(root,'live.json').mkdir()
                    with self.assertRaises((ValueError,OSError)): controller.capture_live()
                    with self.assertRaises(ValueError): controller.capture_live()
                    with self.assertRaises(ValueError): controller.handle({'op':'heartbeat','sequence':2,'identity':identity})
                    self.assertIsNone(controller.live_evidence)
                with Controller(root,BINDING,guest,clock) as reopened:
                    with self.assertRaises(ValueError): reopened.capture_live()
                    with self.assertRaises(ValueError): reopened.handle({'op':'grant','sequence':1,'identity':identity})

    def test_all_unknown_stop_fields_quarantine_in_poll_and_abort(self):
        variants=[{'runtime_active':None},{'tool_active':'false'},{'empty':1},{'empty':'unknown'},
                  {'unexpected':True},{'remove':'empty'}]
        for mutation in variants:
            for route in ('poll','abort'):
                class BadStop(GuestFixture):
                    def begin_stop(self,identity): pass
                    def stop_observation(self,identity):
                        value={'identity':identity,'runtime_active':False,'tool_active':False,'empty':True}
                        if 'remove' in mutation: value.pop(mutation['remove'])
                        else: value.update(mutation)
                        return value
                with self.subTest(mutation=mutation,route=route),tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as root:
                    clock,guest=Clock(),BadStop()
                    with Controller(root,BINDING,guest,clock) as controller:
                        identity=controller.handle({'op':'prepare','sequence':0})['identity']
                        for seq in range(1,21):
                            controller.handle({'op':'grant' if seq==1 else 'heartbeat','sequence':seq,'identity':identity});clock.advance(.25)
                        if route=='abort': controller.abort('fixture')
                        else:
                            controller.handle({'op':'stop','sequence':21,'identity':identity})
                            with self.assertRaises(ValueError): controller.poll_stop()
                        self.assertEqual(controller.state,'quarantined-unknown')
