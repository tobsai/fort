"""Disabled single-process workers with wall, output, CPU, file and observed RSS limits.

RSS checks request 100 ms spacing; each ps call has a 250 ms budget.
This is an observed stop threshold with possible overshoot, not a macOS quota.
Only fixed repository workers run. No downloaded module may spawn children.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import resource
import selectors
import signal
import subprocess
import sys
import time
from .bounded import Commands
from .inspection_guard import require_inspection
from .acquisition import INPUTS
from .source_identity import verify_source

SOURCE=Path(__file__).resolve().parents[1]
NODE='/opt/homebrew/opt/node@26/bin/node'
PROFILES={
    'acquire':{'wall':900,'cpu':120,'rss':256*1024**2,'file':805306368},
    'registry':{'wall':90,'cpu':80,'rss':1024**3,'file':0},
    'verifier_archive':{'wall':30,'cpu':25,'rss':256*1024**2,'file':24*1024**2},
    'publisher':{'wall':90,'cpu':80,'rss':1024**3,'file':0},
    'ubuntu_inventory':{'wall':300,'cpu':280,'rss':3*1024**3,'file':0},
    'ubuntu_selected':{'wall':300,'cpu':280,'rss':3*1024**3,'file':0},
}


def worker_command(operation,root,selection_sha256=None,names=None):
    if operation not in PROFILES or not root.is_absolute() or '..' in root.parts:raise ValueError('fixed worker operation and root required')
    if operation in ('registry','publisher'):
        if operation=='publisher' and not re.fullmatch('[a-f0-9]{64}',selection_sha256 or ''):raise ValueError('selected module manifest digest required')
        allowed=[SOURCE/'inspection-verifier.mjs',SOURCE/'verify-inputs.mjs',INPUTS,root/'downloads',root/'verifier',root/'verifier-selection.json']
        return [NODE,'--permission','--max-old-space-size=256','--disable-proto=throw',
                *['--allow-fs-read='+str(path) for path in allowed],str(SOURCE/'inspection-verifier.mjs'),operation,str(root),selection_sha256 or '']
    if operation=='ubuntu_selected' and (not isinstance(names,dict) or set(names)!={'raw','kernel','initrd'}):raise ValueError('exact observed member selection required')
    code="import sys;sys.path.insert(0,"+repr(str(SOURCE))+");from fort_probe.inspection_worker import main;main()"
    return ['/usr/bin/python3','-I','-B','-c',code,operation,str(root),json.dumps(names if operation=='ubuntu_selected' else None,separators=(',',':'))]


def rss_bytes(raw):
    text=raw.decode().strip()
    if not re.fullmatch('[0-9]{1,12}',text):raise ValueError('worker RSS observation incomplete')
    return int(text)*1024


def child_limits(profile):
    resource.setrlimit(resource.RLIMIT_CORE,(0,0))
    resource.setrlimit(resource.RLIMIT_CPU,(profile['cpu'],profile['cpu']+1))
    if sys.platform!='darwin':resource.setrlimit(resource.RLIMIT_RSS,(profile['rss'],profile['rss']))
    resource.setrlimit(resource.RLIMIT_FSIZE,(profile['file'],profile['file']))
    resource.setrlimit(resource.RLIMIT_NOFILE,(64,64))


class WorkerOutput:
    def __init__(self,operation,inputs,started):
        self.operation,self.inputs=operation,list(inputs)
        self.index=0;self.pending=bytearray();self.result=None
        self.input_deadline=started+600 if operation=='acquire' else None
    def check_deadline(self,now):
        if self.input_deadline is not None and now>=self.input_deadline:raise TimeoutError('per-input worker deadline')
    def feed(self,chunk,now):
        self.check_deadline(now);self.pending.extend(chunk)
        if len(self.pending)>65536:raise ValueError('worker framing cap')
        while b'\n' in self.pending:
            line,unused,tail=self.pending.partition(b'\n');self.pending=bytearray(tail)
            value=json.loads(line)
            if self.result is not None:raise ValueError('worker output after terminal result')
            if isinstance(value,dict) and set(value)=={'progress_input'}:
                if self.operation!='acquire' or self.index>=len(self.inputs) or value['progress_input']!=self.inputs[self.index]:
                    raise ValueError('input progress skipped, repeated or unbound')
                self.index+=1;self.input_deadline=now+600
            else:
                if self.operation=='acquire' and self.index!=len(self.inputs):raise ValueError('input progress incomplete')
                self.result=value
    def finish(self):
        if self.pending or self.result is None:raise ValueError('worker result framing incomplete')
        return self.result


class Worker:
    def __init__(self,accepted_source_sha256=None):
        require_inspection()
        if not re.fullmatch('[a-f0-9]{64}',accepted_source_sha256 or ''):raise ValueError('exact accepted source seal required')
        self.accepted_source_sha256=accepted_source_sha256;self.commands=Commands(guard=require_inspection)
    def run(self,operation,root,**parameters):
        verify_source(self.accepted_source_sha256)
        profile=PROFILES[operation];argv=worker_command(operation,root,**parameters)
        pins=json.loads(INPUTS.read_bytes())
        for executable in (argv[0],'/bin/ps'):
            pin=next(item for item in pins['existing_host_tools'] if item['path']==executable)
            data=Path(executable).read_bytes()
            if len(data)!=pin['bytes'] or hashlib.sha256(data).hexdigest()!=pin['sha256']:raise ValueError('inspection tool pin changed')
        child=subprocess.Popen(argv,stdin=subprocess.DEVNULL,stdout=subprocess.PIPE,stderr=subprocess.PIPE,
            cwd=str(SOURCE),env={'PATH':'/usr/bin:/bin','LANG':'C','LC_ALL':'C','TZ':'UTC'},close_fds=True,
            start_new_session=True,preexec_fn=lambda:child_limits(profile))
        started=time.monotonic();deadline=started+profile['wall'];output=[bytearray(),bytearray()];next_rss=0
        parser=WorkerOutput(operation,[e['name'] for e in pins['downloads'] if e['name']!='UTM.dmg'],started)
        try:
            with selectors.DefaultSelector() as selector:
                for index,stream in enumerate((child.stdout,child.stderr)):
                    os.set_blocking(stream.fileno(),False);selector.register(stream,selectors.EVENT_READ,index)
                exited=False
                while not exited or selector.get_map():
                    now=time.monotonic();parser.check_deadline(now)
                    if now>=deadline:raise TimeoutError('inspection worker deadline')
                    if not exited and now>=next_rss:
                        raw,unused=self.commands.run(['/bin/ps','-o','rss=','-p',str(child.pid)],timeout=.25,cap=128,allow_empty=True)
                        if raw:
                            if rss_bytes(raw)>profile['rss']:raise ValueError('worker resident-memory threshold')
                        else:
                            self.commands._wait_unreaped(child,min(deadline,time.monotonic()+.01))
                            exited=True
                        next_rss=time.monotonic()+.1
                    for key,unused in selector.select(min(.1,max(0,deadline-time.monotonic()))):
                        chunk=os.read(key.fileobj.fileno(),4096)
                        if not chunk:selector.unregister(key.fileobj)
                        else:
                            output[key.data].extend(chunk)
                            if sum(map(len,output))>65536:raise ValueError('worker output cap')
                            if key.data==0:parser.feed(chunk,time.monotonic())
                    if not exited:
                        limit=min(deadline,time.monotonic()+.05)
                        if parser.input_deadline is not None:limit=min(limit,parser.input_deadline)
                        try:self.commands._wait_unreaped(child,limit)
                        except TimeoutError:pass
                        else:exited=True
                    parser.check_deadline(time.monotonic())
                    if time.monotonic()>=deadline:raise TimeoutError('inspection worker completion deadline')
        finally:
            try:
                try:os.killpg(child.pid,signal.SIGKILL)
                except ProcessLookupError:pass
                finally:child.wait(timeout=1)
            finally:child.stdout.close();child.stderr.close()
        if child.returncode!=0 or output[1]:raise ValueError('inspection worker failed; no acceptance')
        return parser.finish()
