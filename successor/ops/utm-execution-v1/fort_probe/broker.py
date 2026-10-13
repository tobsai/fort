"""Fixed serial broker and independent timer. No shell or caller-selected OS paths."""
import os
import select
import time
from .serial import Frames, encode
from .evidence import exact


class Broker:
    def __init__(self, controller, clock):
        self.controller,self.clock=controller,clock
        self.frames={channel:Frames() for channel in ('control','inspection')}
        self.started=clock(); self.failed=False
        self.terminal_emitted=False

    def receive(self, channel, chunk):
        if self.failed or channel not in self.frames: raise ValueError('broker channel closed')
        try:
            responses=[]
            for message in self.frames[channel].feed(chunk):
                if channel=='control':
                    if message.get('op') not in ('prepare','grant','heartbeat','stop'): raise ValueError('control operation forbidden')
                    response=self.controller.handle(message)
                elif message=={'op':'preflight'}:
                    response={'event':'preflight','facts':self.controller.preflight()}
                elif message=={'op':'postflight'}:
                    response={'event':'postflight','facts':self.controller.inspect()}
                elif (set(message)=={'op','identity'} and message['op']=='live'
                      and exact(message['identity'],self.controller.identity)):
                    response={'event':'live','facts':self.controller.capture_live()}
                else: raise ValueError('inspection operation/identity forbidden')
                responses.append((channel,response))
            return responses
        except Exception:
            self.failed=True
            self.controller.abort('broker input failed')
            raise

    def tick(self):
        if self.failed: return []
        if not 0<=self.clock()-self.started<45:
            self.failed=True; self.controller.abort('host/guest session deadline'); return []
        receipt=self.controller.tick()
        if receipt is not None and not self.terminal_emitted:
            self.terminal_emitted=True
            return [('control',receipt)]
        return []

    def close(self):
        self.failed=True
        self.controller.close()


def serve(broker, descriptors, clock=time.monotonic):
    """Called only by the gated guest entry. Descriptor setup/ownership is external."""
    if set(descriptors)!={'control','inspection'}: raise ValueError('two exact channels required')
    pending={channel:bytearray() for channel in descriptors}; totals=dict.fromkeys(descriptors,0)
    for fd in descriptors.values(): os.set_blocking(fd,False)
    def queue(responses):
        for channel,response in responses:
            data=encode(response); totals[channel]+=len(data)
            if totals[channel]>262144 or len(pending[channel])+len(data)>8192: raise ValueError('broker output bound')
            pending[channel].extend(data)
    try:
        while not broker.failed:
            queue(broker.tick())
            readers,writers,_=select.select(list(descriptors.values()),[descriptors[c] for c in pending if pending[c]],[],.05)
            for channel,fd in descriptors.items():
                if fd in readers:
                    chunk=os.read(fd,4096)
                    if not chunk: raise EOFError('guest serial hangup')
                    queue(broker.receive(channel,chunk))
                if fd in writers and pending[channel]:
                    try: count=os.write(fd,pending[channel])
                    except BlockingIOError: continue
                    if count<=0: raise EOFError('guest serial write closed')
                    del pending[channel][:count]
    finally: broker.close()
