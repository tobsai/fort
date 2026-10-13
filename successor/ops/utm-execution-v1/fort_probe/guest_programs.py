"""Fixed synthetic programs. OS effects are supplied by the Linux kernel adapter."""
PROVIDER = b'synthetic-provider-canary-only\n'
PUBLIC = b'fort-utm-public-synthetic-canary-v1\n'
OPEN_PROBES = (
    ('C01', '/var/lib/fort-probe/provider/synthetic-canary', False),
    ('C02', '/run/fort-probe/session/identity.json', False),
    ('C02', '/run/fort-probe/session/identity.json', True),
    ('C03', '/var/lib/fort-probe/sibling/canary', False),
    ('C03', '/var/lib/fort-probe/sibling/canary', True),
    ('C04', '/var/lib/cloud/seed/nocloud/user-data', False),
    ('C05', '/dev/hvc0', False), ('C05', '/dev/hvc1', False),
    ('C09', '/sys/fs/cgroup/fort.slice/fort-attempt.slice/cgroup.procs', True),
)


class Runtime:
    def __init__(self, io, clock, boot_id, invocation):
        self.io, self.clock = io, clock
        self.boot_id, self.invocation = boot_id, invocation
        self.started = None
        self.failed = False
        self.granted = False
        self.last_lease = None

    def start(self):
        if self.started is not None: raise ValueError('runtime already started')
        self.started = self.clock()
        if self.io.provider() != PROVIDER: raise ValueError('provider positive control failed')
        self.io.report({'provider_literal_read': True, 'guest_boot_id': self.boot_id,
                        'invocation_id': self.invocation})
        self.io.notify('READY=1')

    def step(self):
        if self.started is None or self.failed: return False
        now = self.clock()
        lease = self.io.heartbeat()
        valid = 0 <= now - self.started < 40
        if lease is None:
            valid = valid and not self.granted and now - self.started < 4
        else:
            valid = valid and (set(lease) == {'guest_boot_id', 'invocation_id', 'at', 'granted'}
                and lease['guest_boot_id'] == self.boot_id and lease['invocation_id'] == self.invocation
                and type(lease['at']) in (float, int) and 0 <= now - lease['at'] < 1.5
                and type(lease['granted']) is bool
                and (self.last_lease is None or lease['at'] >= self.last_lease))
            if valid:
                valid = not self.granted or lease['granted']
                self.granted = self.granted or lease['granted']
                self.last_lease = lease['at']
                valid = valid and (self.granted or now - self.started < 4)
        if not valid:
            self.failed = True
            self.io.notify('STOPPING=1')
            return False
        self.io.notify('WATCHDOG=1')
        return True


def tool_probe(io):
    opens = [{'case': case, 'path': path, 'write': write, 'errno': io.denied_open(path, write)}
             for case, path, write in OPEN_PROBES]
    return {'opens': opens, 'ipc': io.management_connect(), 'scratch': io.own_scratch(),
            'socketpair': io.own_socketpair(), 'ip': {'inet': io.ip_socket('inet'), 'inet6': io.ip_socket('inet6')},
            'descendant': io.descendant()}
