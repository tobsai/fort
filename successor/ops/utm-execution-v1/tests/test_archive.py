import gzip
import hashlib
import io
import tarfile
import unittest
from fort_probe.archive import inspect_archive


def fixture_archive(link=False):
    raw=bytearray(4096);raw[1080:1082]=b'\x53\xef';raw[1144:1160]=b'cloudimg-rootfs'.ljust(16,b'\0')
    kernel=bytearray(64);kernel[56:60]=b'ARM\x64'
    output=io.BytesIO()
    with tarfile.open(fileobj=output,mode='w:gz') as tar:
        for name,data in (('fixture/root.raw',raw),('fixture/Image',kernel),('fixture/initrd',b'070701fixture')):
            info=tarfile.TarInfo(name);info.size=len(data);tar.addfile(info,io.BytesIO(data))
        if link:
            info=tarfile.TarInfo('fixture/link');info.type=tarfile.SYMTYPE;info.linkname='/etc/passwd';tar.addfile(info)
    return output.getvalue()


class ArchiveTests(unittest.TestCase):
    def test_synthetic_raw_kernel_and_initrd_are_bound_without_writing_or_guessing_members(self):
        data=fixture_archive();names={'raw':'fixture/root.raw','kernel':'fixture/Image','initrd':'fixture/initrd'}
        result=inspect_archive(io.BytesIO(data),hashlib.sha256(data).hexdigest(),names)
        self.assertEqual(result['root_label'],'cloudimg-rootfs');self.assertTrue(result['kernel_arm64'])
        self.assertEqual(result['raw_member'],names['raw']);self.assertEqual(result['raw_bytes'],4096)

    def test_links_missing_members_or_wrong_digest_stop(self):
        for fault in ('link','missing','digest'):
            data=fixture_archive(fault=='link'); names={'raw':'fixture/root.raw','kernel':'fixture/Image','initrd':'fixture/initrd'}
            if fault=='missing':names['raw']='not-present'
            digest=hashlib.sha256(data).hexdigest() if fault!='digest' else '0'*64
            with self.assertRaises(ValueError):inspect_archive(io.BytesIO(data),digest,names)
