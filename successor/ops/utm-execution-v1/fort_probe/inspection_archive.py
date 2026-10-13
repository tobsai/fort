"""Bounded inert archive inventory and authenticated verifier-member selection."""
import gzip
import hashlib
import io
import json
from pathlib import PurePosixPath
import tarfile
import zlib
from .archive import exact_read
from .serial import _pairs,_constant,_float


def pinned_bytes(source,algorithm,expected,cap):
    data=source.read(cap+1)
    if len(data)>cap or hashlib.new(algorithm,data).hexdigest()!=expected:raise ValueError('archive cap or pinned digest differs')
    return data


def scan(data,package=False):
    limit=24*1024**2 if package else 20*1024**3+192*1024**2
    max_entries=256 if package else 64
    seen=set();total=0;auxiliary=0;raw=None;records=[];files={}
    try:
        with gzip.GzipFile(fileobj=io.BytesIO(data)) as stream:
            while True:
                header=exact_read(stream,512)
                if header==bytes(512):
                    if exact_read(stream,512)!=bytes(512):raise ValueError('tar terminator differs')
                    tail=stream.read(10241)
                    if len(tail)>10240 or any(tail):raise ValueError('trailing archive bytes')
                    break
                if len(seen)>=max_entries or header[156:157] not in (b'0',b'\0',b'5') or header[257:263]!=b'ustar\0':
                    raise ValueError('unsupported physical archive header')
                item=tarfile.TarInfo.frombuf(header,'utf-8','strict');path=PurePosixPath(item.name)
                if (not item.name or '\\' in item.name or any(ord(c)<32 for c in item.name) or path.is_absolute() or '..' in path.parts or str(path)!=item.name or item.name in seen
                        or item.linkname or item.size<0 or item.mode & 0o6000):raise ValueError('unsafe archive entry')
                seen.add(item.name);total+=item.size
                if total>limit:raise ValueError('aggregate archive budget')
                if item.isdir():
                    if item.size:raise ValueError('directory payload')
                    continue
                if package and (not item.name.startswith('package/') or path.suffix in ('.node','.dylib','.so')):
                    raise ValueError('verifier package path or native addon forbidden')
                if item.size>(24*1024**2 if package else 20*1024**3):raise ValueError('member budget')
                head=exact_read(stream,min(item.size,4096));digest=hashlib.sha256(head);retained=bytearray(head) if package else None
                kind='regular'
                if not package:
                    if head[1080:1082]==b'\x53\xef':
                        if raw is not None:raise ValueError('multiple raw filesystem candidates')
                        raw=item.name;kind='unpartitioned-ext4-candidate'
                    else:
                        auxiliary+=item.size
                        if auxiliary>192*1024**2:raise ValueError('auxiliary budget or unsupported raw layout')
                remaining=item.size-len(head)
                while remaining:
                    chunk=exact_read(stream,min(1024**2,remaining));remaining-=len(chunk);digest.update(chunk)
                    if package:retained.extend(chunk)
                if any(exact_read(stream,(-item.size)%512)):raise ValueError('member padding differs')
                records.append({'name':item.name,'bytes':item.size,'sha256':digest.hexdigest(),'kind':kind})
                if package:files[item.name]=bytes(retained)
    except (tarfile.HeaderError,EOFError,zlib.error,UnicodeError,OSError) as error:raise ValueError('archive framing invalid') from error
    return records,files,raw


def verifier_package(source,expected_sha512):
    data=pinned_bytes(source,'sha512',expected_sha512,8*1024**2)
    records,files,unused=scan(data,package=True)
    required={'package/package.json','package/dist/node/openpgp.mjs','package/LICENSE'}
    if not required<=set(files) or not all(files[name] for name in required):raise ValueError('verifier entry, metadata or license missing')
    if len(files['package/package.json'])>65536:raise ValueError('package metadata cap')
    info=json.loads(files['package/package.json'],object_pairs_hook=_pairs,parse_constant=_constant,parse_float=_float)
    if (info.get('name')!='openpgp' or info.get('version')!='6.3.2'
            or info.get('exports',{}).get('.',{}).get('import')!='./dist/node/openpgp.mjs'
            or info.get('dependencies',{})!={} or info.get('optionalDependencies',{})!={}):
        raise ValueError('verifier package identity, entry or dependencies differ')
    return {'entry':'package/dist/node/openpgp.mjs','archive_sha512':expected_sha512,'executed':False,'files':files,
            'manifest':{row['name']:{'bytes':row['bytes'],'sha256':row['sha256']} for row in records}}


def ubuntu_inventory(source,expected_sha256):
    data=pinned_bytes(source,'sha256',expected_sha256,768*1024**2)
    records,unused,raw=scan(data)
    if raw is None:raise ValueError('unpartitioned root candidate absent; reviewed decoder required')
    return {'archive_sha256':expected_sha256,'entries':records,'raw_candidate':raw,
            'selection_status':'requires-exact-reviewed-member-selection','bootable_claim':False}
