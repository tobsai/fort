import unittest
from fort_probe.linux_kernel import decode_process,decode_units,PROPERTIES
from fort_probe.bounded import Commands
from fort_probe.guest_entry import main


def proc(pid=42,start=12345):
    return str(pid)+' (fixture) S 1 42 41 '+' '.join(['0']*15+[str(start)]+['0']*5)


def unit_output():
    common={'NoNewPrivileges':'yes','PrivateNetwork':'yes','PrivateIPC':'yes','PrivateDevices':'yes','PrivateTmp':'yes',
            'ProtectSystem':'strict','ProtectHome':'yes','ProtectControlGroups':'yes','CapabilityBoundingSet':'','AmbientCapabilities':'',
            'RestrictAddressFamilies':'AF_UNIX','KillMode':'control-group','Restart':'no','TimeoutStopUSec':'2s','MemoryMax':'134217728','CPUQuotaPerSecUSec':'250ms'}
    result=[]
    for role in ('runtime','tool'):
        fields=dict.fromkeys(PROPERTIES,'');fields.update(common)
        fields.update(Id='fort-'+role+'.service',ActiveState='active',SubState='running',MainPID='40' if role=='runtime' else '41',
                      InvocationID=('1' if role=='runtime' else '2')*32,ControlGroup='/fort.slice/fort-attempt.slice/fort-'+role+'.service',
                      RuntimeMaxUSec='40s' if role=='runtime' else '38s',TasksMax='8' if role=='runtime' else '16',
                      User='fort-runtime' if role=='runtime' else '',DynamicUser='no' if role=='runtime' else 'yes',WatchdogUSec='2s' if role=='runtime' else '0')
        result.append('\n'.join(k+'='+v for k,v in fields.items()))
    return '\n\n'.join(result)


class KernelParsingTests(unittest.TestCase):
    def test_raw_unit_facts_accept_exact_policy_and_reject_weakened_or_missing_fields(self):
        raw=unit_output();self.assertTrue(decode_units(raw)['runtime']['active'])
        transitional=decode_units(raw.replace('ActiveState=active','ActiveState=activating'))['runtime']
        self.assertTrue(transitional['active']); self.assertFalse(transitional['ready'])
        for bad in (raw.replace('PrivateNetwork=yes','PrivateNetwork=no'),raw.replace('MemoryMax=134217728','MemoryMax=infinity'),raw.replace('MainPID=40\n','')):
            with self.assertRaises(ValueError):decode_units(bad)

    def test_process_collector_rejects_pid_reuse_extra_fd_and_capability(self):
        status='Uid:\t61184 61184 61184 61184\nNoNewPrivs:\t1\n'+''.join(k+':\t0000000000000000\n' for k in ('CapInh','CapPrm','CapEff','CapBnd','CapAmb'))
        args=[proc(),status,'0::/fort.slice/fort-attempt.slice/fort-tool.service\n',{'net':1,'mnt':2,'ipc':3},{'0':'/dev/null','1':'socket:[2]','2':'socket:[2]'},proc()]
        self.assertEqual(decode_process(*args)['start_ticks'],12345)
        for fault in ('reuse','fd','cap'):
            copy=list(args)
            if fault=='reuse':copy[-1]=proc(start=12346)
            if fault=='fd':copy[4]=dict(args[4],**{'3':'/run/fort-probe/session'})
            if fault=='cap':copy[1]=status.replace('CapBnd:\t0000000000000000','CapBnd:\t0000000000000001')
            with self.assertRaises(ValueError):decode_process(*copy)

    def test_live_construction_and_all_guest_roles_remain_disabled(self):
        with self.assertRaises(RuntimeError):Commands()
        for role in ('bootstrap','broker','runtime','tool'):
            with self.assertRaises(RuntimeError):main([role])
