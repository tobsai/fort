from pathlib import Path
import tempfile
import unittest
from fort_probe.inspection import Inspector
from fort_probe.acquisition import fixed_plan
from test_config import IMAGE
from fort_probe.inspection_workers import Worker,worker_command,PROFILES,rss_bytes

class Boundary:
    def __init__(self,failure=None):self.calls=[];self.failure=failure
    def run(self,operation,root,**kwargs):
        self.calls.append(operation)
        if operation==self.failure:raise TimeoutError('fixture')
        pins={e['name']:e for e in fixed_plan()[0]}
        details={'acquire':{'completed':8},'registry':{'registry_signatures':2,'archive_sha512':pins['openpgp-6.3.2.tgz']['sha512']},
                 'verifier_archive':{'selection_sha256':'a'*64},'publisher':{'negative_controls':{'rejected':4},'module_sha256':'a'*64,'publisher':{'filename':'noble-server-cloudimg-arm64.tar.gz','sha256':pins['noble-server-cloudimg-arm64.tar.gz']['sha256'],'primaryFingerprint':'D2EB44626FDDC30B513D5BB71A5D6C4C7DB87C81','signaturesVerified':1}},
                 'ubuntu_inventory':{'selection_status':'requires-exact-reviewed-member-selection','entries':[{'name':name} for name in ('fixture/root.raw','fixture/Image','fixture/initrd')]},
                 'ubuntu_selected':dict(IMAGE,archive_sha256=pins['noble-server-cloudimg-arm64.tar.gz']['sha256'],raw_member='fixture/root.raw',kernel_member='fixture/Image',initrd_member='fixture/initrd')}[operation]
        return {'operation':operation,'status':'complete','details':details}

class InspectionTests(unittest.TestCase):
    def test_authentication_precedes_module_selection_loading_and_ubuntu_inspection(self):
        with tempfile.TemporaryDirectory() as temp:
            boundary=Boundary();root=Path(temp).resolve()/'attempt'
            result=Inspector(root,boundary).run()
            self.assertEqual(boundary.calls,['acquire','registry','verifier_archive','publisher','ubuntu_inventory'])
            self.assertFalse(result['image_bound']);self.assertFalse(result['runtime_authorized'])
            self.assertTrue((root/'inspection-complete.json').exists())
            with self.assertRaises(FileExistsError):Inspector(root,Boundary()).run()
    def test_each_stage_failure_prevents_all_subsequent_work_and_acceptance(self):
        stages=['acquire','registry','verifier_archive','publisher','ubuntu_inventory']
        for index,stage in enumerate(stages):
            with tempfile.TemporaryDirectory() as temp:
                root=Path(temp).resolve()/'attempt';boundary=Boundary(stage)
                with self.assertRaises(TimeoutError):Inspector(root,boundary).run()
                self.assertEqual(boundary.calls,stages[:index+1]);self.assertTrue((root/'inspection-failure.json').exists())
                self.assertFalse((root/'inspection-complete.json').exists())
    def test_worker_plan_has_process_limits_and_node_has_no_effect_permissions(self):
        with self.assertRaisesRegex(RuntimeError,'input inspection disabled'):Worker()
        for operation in PROFILES:
            command=worker_command(operation,Path('/fixture'),selection_sha256='a'*64,names={'raw':'r','kernel':'k','initrd':'i'})
            self.assertTrue(command[0].startswith('/'))
            self.assertGreater(PROFILES[operation]['rss'],0);self.assertLessEqual(PROFILES[operation]['wall'],900)
            if operation in ('registry','publisher'):
                self.assertIn('--permission',command);self.assertIn('--max-old-space-size=256',command)
                for forbidden in ('--allow-net','--allow-child-process','--allow-worker','--allow-addons','--allow-ffi','--allow-fs-write'):
                    self.assertFalse(any(x.startswith(forbidden) for x in command))
        self.assertEqual(rss_bytes(b'123\n'),123*1024)
        for invalid in (b'',b'1\n2',b'unknown',b'-1'):
            with self.assertRaises(ValueError):rss_bytes(invalid)

    def test_unbound_authentication_results_cannot_produce_complete_inspection(self):
        for fault in ('registry-digest','publisher-missing','publisher-digest','publisher-fingerprint','publisher-signature-type'):
            class Corrupted(Boundary):
                def run(self,operation,root,**kwargs):
                    response=super().run(operation,root,**kwargs);d=response['details']
                    if fault=='registry-digest' and operation=='registry':d['archive_sha512']='f'*128
                    if operation=='publisher':
                        if fault=='publisher-missing':d.pop('publisher')
                        elif fault=='publisher-digest':d['publisher']['sha256']='f'*64
                        elif fault=='publisher-fingerprint':d['publisher']['primaryFingerprint']='F'*40
                        elif fault=='publisher-signature-type':d['publisher']['signaturesVerified']=True
                    return response
            with tempfile.TemporaryDirectory() as temp:
                root=Path(temp).resolve()/'attempt'
                with self.subTest(fault=fault),self.assertRaises(ValueError):Inspector(root,Corrupted()).run()
                self.assertFalse((root/'inspection-complete.json').exists())

    def test_staging_deadline_cannot_turn_slow_success_into_acceptance(self):
        from support import Clock
        clock=Clock()
        class Slow(Boundary):
            def run(self,*args,**kwargs):
                result=super().run(*args,**kwargs);clock.advance(1801);return result
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp).resolve()/'attempt';boundary=Slow()
            with self.assertRaises(TimeoutError):Inspector(root,boundary,clock).run()
            self.assertEqual(boundary.calls,['acquire']);self.assertFalse((root/'inspection-complete.json').exists())

    def test_selected_image_requires_observed_names_complete_facts_and_original_archive_pin(self):
        selection={'raw':'fixture/root.raw','kernel':'fixture/Image','initrd':'fixture/initrd'}
        with tempfile.TemporaryDirectory() as temp:
            result=Inspector(Path(temp).resolve()/'attempt',Boundary()).run(selection)
            self.assertTrue(result['image_bound']);self.assertFalse(result['runtime_authorized'])
        for fault in ('unknown-name','archive-digest','missing-facts'):
            class Changed(Boundary):
                def run(self,operation,root,**kwargs):
                    response=super().run(operation,root,**kwargs)
                    if operation=='ubuntu_selected':
                        if fault=='archive-digest':response['details']['archive_sha256']='f'*64
                        if fault=='missing-facts':response['details'].pop('decoded_kernel_sha256')
                    return response
            names=dict(selection)
            if fault=='unknown-name':names['kernel']='outside'
            with tempfile.TemporaryDirectory() as temp:
                root=Path(temp).resolve()/'attempt'
                with self.subTest(fault=fault),self.assertRaises(ValueError):Inspector(root,Changed()).run(names)
                self.assertFalse((root/'inspection-complete.json').exists())
