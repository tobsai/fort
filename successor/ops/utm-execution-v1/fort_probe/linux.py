"""Linux policy adapter. Kernel supplies bounded OS effects; fixtures supply raw facts."""
from copy import deepcopy
import errno
import re
from .evidence import exact, RUNTIME_CGROUP, TOOL_CGROUP
from .guest_programs import OPEN_PROBES


def parse_stat(raw):
    head, rest = raw.rsplit(') ',1)
    fields=rest.split()
    return {'pid':int(head.split(' (',1)[0]), 'parent':int(fields[1]),
            'session':int(fields[3]), 'start_ticks':int(fields[19])}


def canary_results(report, before, after):
    if set(report) != {'opens','ipc','scratch','socketpair','ip','descendant'}:
        raise ValueError('incomplete tool report')
    if len(report['opens']) != len(OPEN_PROBES): raise ValueError('missing open probe')
    controls = {name: True for name in ('C01','C02','C03','C04','C05')}
    for row, (case,path,write) in zip(report['opens'], OPEN_PROBES):
        allowed = (errno.EACCES,errno.EPERM,errno.ENOENT)
        if case=='C09': allowed=(errno.EACCES,errno.EPERM,errno.EROFS)
        if (set(row)!={'case','path','write','errno'} or row['case']!=case or row['path']!=path
                or row['write'] is not write or type(row['errno']) is not int or row['errno'] not in allowed):
            raise ValueError('canary access denial not proven')
    from .ipc import accept_denial
    accept_denial(before, after, report['ipc'], hidden=True)
    if report['scratch'] is not True or report['socketpair'] is not True: raise ValueError('own positive control failed')
    if set(report['ip'])!={'inet','inet6'} or any(type(n) is not int or n not in (errno.EPERM,errno.EACCES,errno.EAFNOSUPPORT) for n in report['ip'].values()):
        raise ValueError('IP socket creation denial not proven')
    controls.update(C06=True,C07=True,C08=True)
    return controls


class LinuxGuest:
    def __init__(self, kernel, clock):
        self.kernel, self.clock = kernel, clock
        self.boot_id=kernel.boot()
        self.identity=None
        self.ipc_before=None
        self.stop_sent=False

    def _same(self, identity):
        if not exact(identity,self.identity) or self.kernel.boot()!=self.boot_id:
            raise ValueError('guest/attempt identity changed')
        unit=self.kernel.units()['runtime']
        if unit['active'] and (unit['invocation']!=identity['invocation_id'] or unit['cgroup']!=RUNTIME_CGROUP):
            raise ValueError('runtime invocation replaced')

    def current(self):
        units=self.kernel.units()
        if self.identity is not None and units['runtime']['active']:
            self._same(self.identity)
        return {'identity':deepcopy(self.identity),'runtime_active':units['runtime']['active'],
                'tool_active':units['tool']['active'],'empty':not self.kernel.pids() and not any(u['active'] for u in units.values())}

    def ready(self):
        return self.current()['empty'] and self.identity is None

    def preflight(self):
        state=self.current(); topology=self.kernel.topology()
        return dict(guest_boot_id=self.boot_id, runtime_active=state['runtime_active'],tool_active=state['tool_active'],
                    empty=state['empty'],root_controls=self.kernel.root_controls(),nics=topology['nics'],shares=topology['shares'])

    def prepare(self, binding):
        if not self.ready(): raise ValueError('guest already consumed or not empty')
        self.kernel.run('start-runtime')
        deadline=self.clock()+.35
        while True:
            units=self.kernel.units(); unit=units['runtime']
            if unit['active'] and unit.get('ready',True): break
            if self.clock()>=deadline: raise TimeoutError('runtime readiness deadline')
            self.kernel.wait(.01)
        if units['tool']['active'] or unit['cgroup']!=RUNTIME_CGROUP or not re.fullmatch('[0-9a-f]{32}',unit['invocation']):
            raise ValueError('unexpected prepared unit state')
        self.identity=dict(binding,guest_boot_id=self.boot_id,invocation_id=unit['invocation'],runtime_cgroup=RUNTIME_CGROUP)
        self.kernel.write('identity',self.identity)
        self._lease(False)
        return deepcopy(self.identity)

    def prepared_controls(self, identity):
        self._same(identity)
        units=self.kernel.units(); report=self.kernel.runtime_report()
        if report!={'provider_literal_read':True,'guest_boot_id':self.boot_id,'invocation_id':identity['invocation_id']}:
            raise ValueError('runtime report unbound or failed')
        return {'identity':deepcopy(identity),'runtime':self.kernel.process(units['runtime']['pid']),
                'tool_active':units['tool']['active'],'provider_literal_read':True}

    def _lease(self, granted):
        self.kernel.write('heartbeat',{'guest_boot_id':self.boot_id,'invocation_id':self.identity['invocation_id'],
                                      'at':self.clock(),'granted':granted})

    def grant(self, identity):
        self._same(identity)
        if self.kernel.units()['tool']['active'] or self.ipc_before is not None: raise ValueError('tool grant already consumed')
        self.ipc_before=self.kernel.ipc_start(); self.ipc_at=self.clock()
        self._lease(True)
        self.kernel.run('start-tool')

    def heartbeat(self, identity):
        self._same(identity)
        if self.stop_sent: raise ValueError('heartbeat after stop')
        self._lease(True)

    def live_snapshot(self, identity, sequence, now):
        self._same(identity)
        units=self.kernel.units()
        if not all(unit['active'] for unit in units.values()): raise ValueError('live units required')
        report=self.kernel.tool_report(units['tool'])
        descendant=report['descendant']
        if set(descendant)!={'pid','start_ticks'} or type(descendant['pid']) is not int: raise ValueError('descendant identity missing')
        processes={'runtime':self.kernel.process(units['runtime']['pid']), 'tool':self.kernel.process(units['tool']['pid']),
                   'descendant':self.kernel.process(descendant['pid'])}
        if processes['descendant']['start_ticks']!=descendant['start_ticks'] or set(self.kernel.pids())!={v['pid'] for v in processes.values()}:
            raise ValueError('owned process set/start identity changed')
        if self.ipc_before is None or not 0<=self.clock()-self.ipc_at<6: raise ValueError('IPC observation window expired')
        controls=canary_results(report,self.ipc_before,self.kernel.ipc_finish())
        return {'identity':deepcopy(identity),'sequence':sequence,'captured_at':self.clock(),'processes':processes,
                'setsid':self.kernel.detached(descendant['pid'],units['tool']['pid']),'controls':controls}

    def begin_stop(self, identity):
        self._same(identity)
        if self.stop_sent: raise ValueError('stop cannot be resent')
        self.stop_sent=True
        self.kernel.run('stop')

    def stop_observation(self, identity):
        self._same(identity)
        return self.current()
