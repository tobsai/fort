import gzip
import hashlib
import io
import json
import tarfile
import unittest
from fort_probe.inspection_archive import verifier_package,ubuntu_inventory
from test_archive import fixture_archive


def package(changes=None,extra=None):
    info={'name':'openpgp','version':'6.3.2','exports':{'.':{'import':'./dist/node/openpgp.mjs'}},'dependencies':{}}
    info.update(changes or {})
    values={'package/package.json':json.dumps(info).encode(),'package/dist/node/openpgp.mjs':b'export const fixture = true;\n','package/LICENSE':b'Synthetic license fixture only.\n'}
    values.update(extra or {});out=io.BytesIO()
    with tarfile.open(fileobj=out,mode='w:gz',format=tarfile.USTAR_FORMAT) as archive:
        for name,data in values.items():
            entry=tarfile.TarInfo(name);entry.size=len(data);archive.addfile(entry,io.BytesIO(data))
    return out.getvalue()

class InspectionArchiveTests(unittest.TestCase):
    def test_verifier_selection_binds_every_member_and_declared_entry_without_execution(self):
        data=package();result=verifier_package(io.BytesIO(data),hashlib.sha512(data).hexdigest())
        self.assertEqual(result['entry'],'package/dist/node/openpgp.mjs')
        self.assertEqual(result['manifest'][result['entry']]['sha256'],hashlib.sha256(result['files'][result['entry']]).hexdigest())
        self.assertIn('package/LICENSE',result['files']);self.assertFalse(result['executed'])
    def test_identity_dependencies_entry_escape_addon_and_bad_hash_fail(self):
        for changes,extra in (({'version':'other'},None),({'dependencies':{'extra':'*'}},None),
                ({'exports':{'.':{'import':'../../other.mjs'}}},None),(None,{'package/native.node':b'x'}),
                (None,{'../outside':b'x'}),(None,{'package/../outside':b'x'})):
            data=package(changes,extra)
            with self.assertRaises(ValueError):verifier_package(io.BytesIO(data),hashlib.sha512(data).hexdigest())
        with self.assertRaises(ValueError):verifier_package(io.BytesIO(package()),'0'*128)
    def test_declared_oversize_and_special_header_are_rejected_without_payload(self):
        for kind,size in ((tarfile.REGTYPE,25*1024**2),(tarfile.XGLTYPE,0),(tarfile.SYMTYPE,0)):
            entry=tarfile.TarInfo('package/metadata');entry.type=kind;entry.size=size
            data=gzip.compress(entry.tobuf(format=tarfile.USTAR_FORMAT))
            with self.assertRaises(ValueError):verifier_package(io.BytesIO(data),hashlib.sha512(data).hexdigest())
    def test_ubuntu_inventory_records_observed_names_without_assigning_kernel_or_executing(self):
        data=fixture_archive();result=ubuntu_inventory(io.BytesIO(data),hashlib.sha256(data).hexdigest())
        self.assertEqual(len(result['entries']),3)
        self.assertEqual(result['raw_candidate'],'fixture/root.raw')
        self.assertEqual(result['selection_status'],'requires-exact-reviewed-member-selection')
        self.assertFalse(result['bootable_claim'])
