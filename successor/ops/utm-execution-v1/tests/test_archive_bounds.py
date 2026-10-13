import gzip
import hashlib
import io
import tarfile
import unittest
from unittest.mock import patch
from fort_probe import archive as examiner
from test_archive import fixture_archive

NAMES={'raw':'fixture/root.raw','kernel':'fixture/Image','initrd':'fixture/initrd'}

def inspect(data):return examiner.inspect_archive(io.BytesIO(data),hashlib.sha256(data).hexdigest(),NAMES)

def compressed_kernel_archive(initrd_size):
    raw=bytearray(4096);raw[1080:1082]=b'\x53\xef';raw[1144:1160]=b'cloudimg-rootfs'.ljust(16,b'\0')
    kernel=bytearray(100);kernel[56:60]=b'ARM\x64'
    result=io.BytesIO()
    with tarfile.open(fileobj=result,mode='w:gz',format=tarfile.USTAR_FORMAT) as target:
        for name,data in zip(NAMES.values(),(raw,gzip.compress(kernel),b'070701'+b'0'*(initrd_size-6))):
            info=tarfile.TarInfo(name);info.size=len(data);target.addfile(info,io.BytesIO(data))
    return result.getvalue(),bytes(kernel)

class ArchiveBoundsTests(unittest.TestCase):
    def test_special_headers_are_rejected_before_reading_declared_payload(self):
        for kind in (tarfile.XGLTYPE,tarfile.XHDTYPE,tarfile.GNUTYPE_LONGNAME,tarfile.GNUTYPE_LONGLINK,tarfile.GNUTYPE_SPARSE,tarfile.FIFOTYPE,tarfile.CHRTYPE):
            with self.subTest(kind=kind):
                header=tarfile.TarInfo('metadata');header.type=kind;header.size=1024**3
                data=gzip.compress(header.tobuf(format=tarfile.USTAR_FORMAT))
                with self.assertRaises(ValueError):inspect(data)

    def test_decoded_kernel_overlap_is_included_in_auxiliary_peak(self):
        data,kernel=compressed_kernel_archive(70)
        with patch.object(examiner,'AUX_LIMIT',192,create=True):
            with self.assertRaises(ValueError):inspect(data)
        with patch.object(examiner,'AUX_LIMIT',256,create=True):
            facts=inspect(data)
        self.assertEqual(facts['decoded_kernel_bytes'],100)
        self.assertEqual(facts['decoded_kernel_sha256'],hashlib.sha256(kernel).hexdigest())
        self.assertEqual(facts['auxiliary_peak_bytes'],facts['auxiliary_bytes']+100)
        self.assertNotEqual(facts['decoded_kernel_sha256'],facts['member_sha256']['kernel'])

    def test_missing_terminator_or_nonzero_tail_is_not_a_complete_archive(self):
        raw=gzip.decompress(fixture_archive())
        for malformed in (raw[:512],raw+b'not-padding'):
            with self.assertRaises(ValueError):inspect(gzip.compress(malformed))
