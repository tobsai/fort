"""Read-only physical USTAR examiner; no metadata preprocessing or extraction.

Publisher authentication is separate. Only an unpartitioned ext4 root is accepted.
Budgets describe logical extraction overlap, not Python's process RSS.
"""
import gzip
import hashlib
import io
from pathlib import PurePosixPath
import tarfile
import zlib

RAW_LIMIT=20*1024**3
AUX_LIMIT=192*1024**2


def exact_read(stream,size):
    data=stream.read(size)
    if len(data)!=size:raise ValueError('truncated physical tar record')
    return data


def kernel_facts(data,auxiliary):
    digest=hashlib.sha256(); head=bytearray(); total=0
    compressed=data.startswith(b'\x1f\x8b')
    limit=AUX_LIMIT-auxiliary if compressed else len(data)
    def consume(chunk):
        nonlocal total
        total+=len(chunk)
        if total>limit:raise ValueError('decoded kernel exceeds combined auxiliary peak')
        digest.update(chunk)
        if len(head)<64:head.extend(chunk[:64-len(head)])
    if compressed:
        decoder=zlib.decompressobj(16+zlib.MAX_WBITS)
        pending=data
        while pending:
            chunk=decoder.decompress(pending,min(1024**2,limit-total+1))
            consume(chunk);pending=decoder.unconsumed_tail
        if not decoder.eof or decoder.unused_data:raise ValueError('kernel gzip format incomplete or concatenated')
    else:
        for offset in range(0,len(data),1024**2):consume(data[offset:offset+1024**2])
    if head[56:60]!=b'ARM\x64':raise ValueError('ARM64 Image header absent')
    return total,digest.hexdigest(),auxiliary+(total if compressed else 0)


def inspect_archive(source,expected_sha256,names):
    if set(names)!={'raw','kernel','initrd'} or len(set(names.values()))!=3:raise ValueError('three exact reviewed members required')
    compressed=source.read(768*1024**2+1)
    if len(compressed)>768*1024**2 or hashlib.sha256(compressed).hexdigest()!=expected_sha256:raise ValueError('archive cap or digest mismatch')
    found={};seen=set();headers=0;auxiliary=0;kernel_data=bytearray()
    try:
        with gzip.GzipFile(fileobj=io.BytesIO(compressed)) as stream:
            while True:
                header=exact_read(stream,512)
                if header==b'\0'*512:
                    if exact_read(stream,512)!=b'\0'*512:raise ValueError('tar terminator differs')
                    tail=stream.read(10241)
                    if len(tail)>10240 or any(tail):raise ValueError('tar trailing data or padding cap')
                    break
                headers+=1
                # Inspect the physical type before any library metadata handling.
                if headers>64 or header[156:157] not in (b'0',b'\0',b'5') or header[257:263]!=b'ustar\0':
                    raise ValueError('unsupported physical tar header')
                member=tarfile.TarInfo.frombuf(header,'utf-8','strict')
                path=PurePosixPath(member.name)
                if (not member.name or path.is_absolute() or '..' in path.parts or str(path)!=member.name
                        or member.name in seen or member.linkname or member.size<0):raise ValueError('unsupported archive member')
                seen.add(member.name)
                if member.isdir():
                    if member.size:raise ValueError('directory payload forbidden')
                    continue
                role=next((key for key,value in names.items() if value==member.name),None)
                if role=='raw':
                    if not 0<member.size<=RAW_LIMIT:raise ValueError('raw member cap')
                else:
                    auxiliary+=member.size
                    if auxiliary>AUX_LIMIT:raise ValueError('combined auxiliary member cap')
                digest=hashlib.sha256();head=bytearray();remaining=member.size
                while remaining:
                    chunk=exact_read(stream,min(1024**2,remaining));remaining-=len(chunk);digest.update(chunk)
                    if len(head)<4096:head.extend(chunk[:4096-len(head)])
                    if role=='kernel':kernel_data.extend(chunk)
                padding=exact_read(stream,(-member.size)%512)
                if any(padding):raise ValueError('nonzero tar member padding')
                if role:found[role]={'bytes':member.size,'sha256':digest.hexdigest(),'head':bytes(head)}
    except (tarfile.HeaderError,EOFError,zlib.error,UnicodeError,OSError) as error:
        raise ValueError('archive framing or compression invalid') from error
    if set(found)!=set(names):raise ValueError('exact members not present')
    raw=found['raw']['head']
    if raw[1080:1082]!=b'\x53\xef':raise ValueError('raw layout not unpartitioned ext4; reviewed decoder required')
    label=raw[1144:1160].split(b'\0',1)[0].decode('ascii')
    if label!='cloudimg-rootfs':raise ValueError('root filesystem label differs')
    try:decoded_size,decoded_hash,peak=kernel_facts(kernel_data,auxiliary)
    except zlib.error as error:raise ValueError('kernel gzip invalid') from error
    if not found['initrd']['head'].startswith((b'\x1f\x8b',b'070701',b'070702',b'\xfd7zXZ\x00',b'\x28\xb5\x2f\xfd')):
        raise ValueError('unsupported initrd container')
    return {'raw_member':names['raw'],'kernel_member':names['kernel'],'initrd_member':names['initrd'],
            'raw_bytes':found['raw']['bytes'],'root_label':label,'kernel_arm64':True,
            'member_sha256':{key:value['sha256'] for key,value in found.items()},'archive_sha256':expected_sha256,
            'auxiliary_bytes':auxiliary,'auxiliary_peak_bytes':peak,
            'member_bytes':{key:value['bytes'] for key,value in found.items()},
            'kernel_compressed':kernel_data.startswith(b'\x1f\x8b'),
            'decoded_kernel_bytes':decoded_size,'decoded_kernel_sha256':decoded_hash}
