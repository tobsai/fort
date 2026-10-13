"""Bounded read-only archive examiner, never an extractor or downloader.

Caller must separately verify publisher metadata before accepting these facts.
Only a raw unpartitioned ext4 filesystem is currently supported. Other layouts
are a precise inspection result requiring a reviewed decoder, never a fallback.
"""
import hashlib
import io
from pathlib import PurePosixPath
import tarfile
import zlib


def inspect_archive(source,expected_sha256,names):
    if set(names)!={'raw','kernel','initrd'} or len(set(names.values()))!=3: raise ValueError('three exact reviewed members required')
    compressed=source.read(768*1024**2+1)
    if len(compressed)>768*1024**2 or hashlib.sha256(compressed).hexdigest()!=expected_sha256: raise ValueError('archive cap or digest mismatch')
    found={}; seen=set(); expanded=0
    with tarfile.open(fileobj=io.BytesIO(compressed),mode='r|gz') as archive:
        for member in archive:
            path=PurePosixPath(member.name)
            if (len(seen)>=64 or path.is_absolute() or '..' in path.parts or str(path)!=member.name
                    or member.name in seen or member.issparse() or member.pax_headers
                    or not (member.isfile() or member.isdir())): raise ValueError('unsupported archive member')
            seen.add(member.name);expanded+=member.size
            if expanded>20*1024**3+192*1024**2: raise ValueError('expanded archive cap')
            if not member.isfile():continue
            role=next((key for key,value in names.items() if value==member.name),None)
            cap=20*1024**3 if role=='raw' else 192*1024**2
            if member.size>cap: raise ValueError('member cap')
            digest=hashlib.sha256();head=bytearray();total=0;kernel_data=bytearray()
            stream=archive.extractfile(member)
            while True:
                chunk=stream.read(1024**2)
                if not chunk:break
                total+=len(chunk)
                if total>member.size:raise ValueError('member expanded beyond declaration')
                digest.update(chunk)
                if len(head)<4096:head.extend(chunk[:4096-len(head)])
                if role=='kernel':kernel_data.extend(chunk)
            if total!=member.size:raise ValueError('truncated member')
            if role: found[role]={'bytes':total,'sha256':digest.hexdigest(),'head':bytes(head),'kernel':bytes(kernel_data)}
    if set(found)!=set(names):raise ValueError('exact members not present')
    raw=found['raw']['head']
    if raw[1080:1082]!=b'\x53\xef': raise ValueError('raw layout not unpartitioned ext4; reviewed decoder required')
    label=raw[1144:1160].split(b'\0',1)[0].decode('ascii')
    if label!='cloudimg-rootfs':raise ValueError('root filesystem label differs')
    kernel=found['kernel']['kernel']
    if kernel.startswith(b'\x1f\x8b'):
        decoder=zlib.decompressobj(16+zlib.MAX_WBITS)
        kernel=decoder.decompress(kernel,192*1024**2+1)
        if len(kernel)>192*1024**2 or not decoder.eof or decoder.unconsumed_tail or decoder.unused_data:raise ValueError('kernel decompression bound/format')
    if kernel[56:60]!=b'ARM\x64':raise ValueError('ARM64 Image header absent')
    if not found['initrd']['head'].startswith((b'\x1f\x8b',b'070701',b'070702',b'\xfd7zXZ\x00',b'\x28\xb5\x2f\xfd')):
        raise ValueError('unsupported initrd container')
    return {'raw_member':names['raw'],'kernel_member':names['kernel'],'initrd_member':names['initrd'],
            'raw_bytes':found['raw']['bytes'],'root_label':label,'kernel_arm64':True,
            'member_sha256':{key:value['sha256'] for key,value in found.items()},'archive_sha256':expected_sha256}
