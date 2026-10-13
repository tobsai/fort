"""Pinned streamed HTTPS policy, durable single-attempt storage and disabled transport."""
import hashlib
import http.client
import json
import os
from pathlib import Path
import re
import ssl
import stat
import time
from urllib.parse import urlsplit,urljoin
from .inspection_guard import require_inspection
from .lifecycle import DirectoryChain

INPUTS=Path(__file__).resolve().parents[2]/'utm-synthetic-packet-v1/inputs.json'


def fixed_plan():
    data=INPUTS.read_bytes()
    expected=json.loads((Path(__file__).resolve().parents[1]/'seal.json').read_bytes())
    pin=next(x for x in expected['references'] if x['path']=='../utm-synthetic-packet-v1/inputs.json')
    if len(data)!=pin['bytes'] or hashlib.sha256(data).hexdigest()!=pin['sha256']:raise ValueError('input inventory seal mismatch')
    pins=json.loads(data);entries=[x for x in pins['downloads'] if x['name']!='UTM.dmg']
    if len(entries)!=8 or sum(x['max_bytes'] for x in entries)!=814088192:raise ValueError('eight fixed inputs required')
    return entries,pins['download_policy']


def public_url(url,allowed):
    parts=urlsplit(url)
    if (len(url)>8192 or parts.scheme!='https' or parts.hostname not in allowed or parts.port not in (None,443)
            or parts.username is not None or parts.password is not None or parts.fragment
            or any(ord(c)<33 or ord(c)>126 for c in url)):
        raise ValueError('unapproved public HTTPS destination')
    return parts


class HTTPS:
    def __init__(self):require_inspection()
    def request(self,url,timeout):
        parts=urlsplit(url)
        connection=http.client.HTTPSConnection(parts.hostname,443,timeout=timeout,context=ssl.create_default_context())
        try:
            connection.request('GET',parts.path+('?' + parts.query if parts.query else ''),headers={
                'Accept':'application/octet-stream','Accept-Encoding':'identity','User-Agent':'Fort-pinned-input-inspection'})
            peer_socket=connection.sock
            response=connection.getresponse()
        except BaseException:connection.close();raise
        class Result:
            status=response.status;headers=response.getheaders()
            def set_timeout(self,seconds):peer_socket.settimeout(seconds)
            def read(self,n):return response.read(n)
            def close(self):
                try:response.close()
                finally:connection.close()
        return Result()


class LocalStore:
    def __init__(self,root):
        self.root=Path(root)
        if not self.root.is_absolute() or '..' in self.root.parts:raise ValueError('absolute fresh inspection root required')
        self.directory=None;self.chain=DirectoryChain(self.root.parent);self.diagnostic_bytes=0
        try:
            self.chain.create(self.root.name);self.directory=self.chain.fd
            self.record('attempt.json',{'status':'consumed','recovery':'no implicit retry or adoption'})
        except BaseException:self.close();raise
    def close(self):
        self.chain.close();self.directory=None
    def check(self):
        self.chain.check()
    def create(self,name):
        self.check()
        if not re.fullmatch('[a-zA-Z0-9_.-]{1,100}',name) or name in ('.','..'):raise ValueError('fixed flat artifact name required')
        fd=os.open(name,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW|os.O_NONBLOCK,0o600,dir_fd=self.directory)
        try:
            self.check()
            info=os.fstat(fd);named=os.stat(name,dir_fd=self.directory,follow_symlinks=False)
            if (info.st_dev,info.st_ino)!=(named.st_dev,named.st_ino) or info.st_nlink!=1 or not stat.S_ISREG(info.st_mode):
                raise ValueError('new artifact identity changed')
            return fd
        except BaseException:os.close(fd);raise
    def record(self,name,value,cap=8192):
        data=(json.dumps(value,sort_keys=True,separators=(',',':'),allow_nan=False)+'\n').encode()
        if cap not in (8192,65536) or len(data)>cap or self.diagnostic_bytes+len(data)>262144:raise ValueError('diagnostic cap')
        if not re.fullmatch('[a-zA-Z0-9_.-]{1,100}',name) or name in ('.','..'):raise ValueError('fixed flat artifact name required')
        self.chain.write_file(name,data);self.diagnostic_bytes+=len(data)
    def begin(self,name):return self.create(name+'.partial')
    def publish(self,name,fd,size):
        self.check();os.fsync(fd);info=os.fstat(fd)
        current=os.stat(name+'.partial',dir_fd=self.directory,follow_symlinks=False)
        identity=lambda x:(x.st_dev,x.st_ino,x.st_size,x.st_mtime_ns,x.st_ctime_ns)
        if not stat.S_ISREG(info.st_mode) or info.st_nlink!=1 or info.st_size!=size or identity(info)!=identity(current):
            raise ValueError('partial input changed before publication')
        os.link(name+'.partial',name,src_dir_fd=self.directory,dst_dir_fd=self.directory,follow_symlinks=False)
        linked=os.stat(name,dir_fd=self.directory,follow_symlinks=False)
        if not stat.S_ISREG(linked.st_mode) or (linked.st_dev,linked.st_ino)!=(info.st_dev,info.st_ino):
            raise ValueError('published input ownership unknown')
        self.check()
        partial=os.stat(name+'.partial',dir_fd=self.directory,follow_symlinks=False)
        if not stat.S_ISREG(partial.st_mode) or (partial.st_dev,partial.st_ino)!=(info.st_dev,info.st_ino):
            raise ValueError('partial replaced after publication; preserve uncertain state')
        os.unlink(name+'.partial',dir_fd=self.directory);os.fsync(self.directory);self.check()
        final=os.fstat(fd);named=os.stat(name,dir_fd=self.directory,follow_symlinks=False)
        if (not stat.S_ISREG(final.st_mode) or final.st_nlink!=1 or not stat.S_ISREG(named.st_mode) or named.st_nlink!=1
                or identity(final)!=identity(named)
                or (final.st_dev,final.st_ino,final.st_size,final.st_mtime_ns)!=(info.st_dev,info.st_ino,info.st_size,info.st_mtime_ns)):
            raise ValueError('published input changed during persistence')


class Acquisition:
    def __init__(self,entries,policy,transport,store,clock=time.monotonic,progress=None):
        self.entries,self.policy,self.transport,self.store,self.clock=entries,policy,transport,store,clock
        self.used=False;self.progress=progress
        if not entries or len({e['name'] for e in entries})!=len(entries) or policy['automatic_retry'] is not False:
            raise ValueError('unique fixed non-retrying inputs required')
        for e in entries:
            if not re.fullmatch('[a-zA-Z0-9_.-]{1,80}',e['name']) or e['name'] in ('.','..'):raise ValueError('unsafe input name')
            public_url(e['url'],policy['allowed_hosts'])
            alg='sha512' if 'sha512' in e else 'sha256'
            if type(e['max_bytes']) is not int or e['max_bytes']<=0 or not re.fullmatch('[0-9a-f]{'+str(128 if alg=='sha512' else 64)+'}',e[alg]):
                raise ValueError('exact input cap and digest required')
    def run(self):
        if self.used:raise ValueError('input attempt already consumed')
        self.used=True;start=self.clock();results=[];active=None
        try:
            for entry in self.entries:
                active=entry['name']
                deadline=min(start+900,self.clock()+self.policy['per_input_total_timeout_seconds'])
                if self.progress is not None:self.progress(entry['name'])
                result=self.fetch(entry,deadline);self.store.record('receipt-'+entry['name']+'.json',result);results.append(result)
            self.store.record('complete.json',{'status':'complete','inputs':[r['name'] for r in results]})
            return {'completed':len(results),'receipts':results,'publisher_authentication':False}
        except BaseException as error:
            try:self.store.record('failure.json',{'status':'failed','input':active,'error_class':type(error).__name__,'accepted':False})
            except BaseException:pass  # Missing completion is itself a durable veto.
            raise
        finally:self.store.close()
    def fetch(self,entry,deadline):
        def remaining():
            value=deadline-self.clock()
            if value<=0:raise TimeoutError('input total deadline')
            return min(value,self.policy['socket_timeout_seconds'])
        url=entry['url'];seen=set();response=None;redirects=0
        try:
            while True:
                public_url(url,self.policy['allowed_hosts'])
                if url in seen:raise ValueError('redirect cycle')
                seen.add(url);response=self.transport.request(url,remaining());remaining()
                headers={}
                for key,value in response.headers:
                    key=key.lower()
                    if key in headers and key in ('location','content-length','transfer-encoding','content-encoding'):raise ValueError('ambiguous HTTP framing')
                    headers[key]=value
                if response.status in (301,302,303,307,308):
                    if redirects>=self.policy['maximum_redirects'] or 'location' not in headers:raise ValueError('redirect bound or location')
                    next_url=urljoin(url,headers['location']);public_url(next_url,self.policy['allowed_hosts'])
                    response.close();response=None;url=next_url;redirects+=1;continue
                if response.status!=200:raise ValueError('HTTP status not accepted')
                if headers.get('content-encoding','identity')!='identity':raise ValueError('HTTP content transformation forbidden')
                if headers.get('transfer-encoding','chunked')!='chunked' or ('transfer-encoding' in headers and 'content-length' in headers):
                    raise ValueError('HTTP transfer framing differs')
                length=headers.get('content-length')
                if length is not None and (not re.fullmatch('[0-9]{1,12}',length) or int(length)>entry['max_bytes']):raise ValueError('HTTP length cap')
                break
            fd=self.store.begin(entry['name']);size=0;algorithm='sha512' if 'sha512' in entry else 'sha256';digest=hashlib.new(algorithm)
            try:
                while True:
                    response.set_timeout(remaining());chunk=response.read(min(65536,entry['max_bytes']-size+1));remaining()
                    if not chunk:break
                    size+=len(chunk)
                    if size>entry['max_bytes']:raise ValueError('input body cap')
                    digest.update(chunk);view=memoryview(chunk)
                    while view:
                        written=os.write(fd,view)
                        if written<=0:raise OSError('input write stalled')
                        view=view[written:]
                if length is not None and size!=int(length):raise ValueError('HTTP body length mismatch')
                if digest.hexdigest()!=entry[algorithm]:raise ValueError('input pinned digest mismatch')
                response.close();response=None;remaining();self.store.publish(entry['name'],fd,size)
            finally:os.close(fd)
            return {'status':'verified-by-pin','name':entry['name'],'bytes':size,algorithm:digest.hexdigest(),
                    'redirects':redirects,'final_host':urlsplit(url).hostname}
        finally:
            if response is not None:response.close()
