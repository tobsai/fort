from copy import deepcopy
import hashlib
import plistlib
import unittest
from pathlib import Path
from fort_probe.utm_schema import pinned_schema,registry_entry
from fort_probe.config import candidate
from test_config import IMAGE
from support import BINDING

PACKAGE='/fixture/vm/fort-utm-isolation-probe.utm'
def registry():
    return {BINDING['vm_package_uuid'].upper():{'Name':'fort-utm-isolation-probe','UUID':BINDING['vm_package_uuid'].upper(),
        'Package':{'Path':PACKAGE,'Bookmark':b'','ReadOnly':False},'Suspended':False,
        'ExternalDrives':{},'SharedDirectories':[],'WindowSettings':{}}}

class UTMSchemaTests(unittest.TestCase):
    def test_pinned_schema_emits_required_icon_boolean_and_exact_version(self):
        config=plistlib.loads(candidate(BINDING,IMAGE,pinned_schema(BINDING))['config_plist'])
        self.assertIs(config['Information']['IconCustom'],False)
        self.assertEqual(config['ConfigurationVersion'],4)
        for fault in ('version','icon','origin'):
            schema=pinned_schema(BINDING)
            if fault=='version':schema['version']=5
            elif fault=='icon':schema['information'].pop('IconCustom')
            else:schema['source_commit']='f'*40
            with self.assertRaises(ValueError):candidate(BINDING,IMAGE,schema)

    def test_raw_registry_preserves_forbidden_entries_instead_of_filtering_invalid_bookmarks(self):
        self.assertEqual(registry_entry(registry(),BINDING['vm_package_uuid'],PACKAGE)['shares'],[])
        for fault in ('missing','share','external','package','suspended','uuid','bookmark','remote'):
            value=registry();entry=next(iter(value.values()))
            if fault=='missing':entry.pop('SharedDirectories')
            elif fault=='share':entry['SharedDirectories']=[{'Bookmark':b'invalid'}]
            elif fault=='external':entry['ExternalDrives']={'other':{'Bookmark':b'invalid'}}
            elif fault=='package':entry['Package']['Path']='/unowned/vm.utm'
            elif fault=='suspended':entry['Suspended']=True
            elif fault=='uuid':entry['UUID']='f'*32
            elif fault=='bookmark':entry['Package']['Bookmark']=b'unresolved'
            elif fault=='remote':entry['Package']['BookmarkRemote']=b'unresolved'
            with self.subTest(fault=fault),self.assertRaises(ValueError):registry_entry(value,BINDING['vm_package_uuid'],PACKAGE)

    def test_bookmark_requires_bound_nonstale_no_ui_no_mount_resolution(self):
        value=registry();next(iter(value.values()))['Package']['Bookmark']=b'synthetic-bookmark'
        resolution={'bookmark_sha256':hashlib.sha256(b'synthetic-bookmark').hexdigest(),'path':PACKAGE,
                    'stale':False,'without_ui':True,'without_mounting':True}
        self.assertTrue(registry_entry(value,BINDING['vm_package_uuid'],PACKAGE,resolution)['complete'])
        for key,bad in (('bookmark_sha256','a'*64),('path','/other'),('stale',True),('without_ui',False),('without_mounting',False)):
            with self.assertRaises(ValueError):registry_entry(value,BINDING['vm_package_uuid'],PACKAGE,dict(resolution,**{key:bad}))

    def test_registry_snapshot_requires_in_memory_and_persisted_agreement(self):
        from fort_probe.utm_schema import registry_snapshot
        value={'persisted':registry(),'in_memory':registry(),'bookmark_resolution':None}
        self.assertTrue(registry_snapshot(value,BINDING['vm_package_uuid'],PACKAGE)['complete'])
        next(iter(value['in_memory'].values()))['SharedDirectories']=[{'Bookmark':b'x'}]
        with self.assertRaises(ValueError):registry_snapshot(value,BINDING['vm_package_uuid'],PACKAGE)

    def test_decoded_kernel_identity_is_carried_to_the_data_file_plan(self):
        from test_archive_bounds import compressed_kernel_archive,inspect
        data,kernel=compressed_kernel_archive(70)
        result=candidate(BINDING,inspect(data),pinned_schema(BINDING))
        self.assertEqual(result['data_files']['Data/kernel.Image'],{'bytes':100,'sha256':hashlib.sha256(kernel).hexdigest()})
