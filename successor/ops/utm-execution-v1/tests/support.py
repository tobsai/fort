"""Explicit external-system/clock doubles. These do not emulate Linux confinement."""
from copy import deepcopy

BINDING = {'installation': 'fort-utm-probe-v1', 'epoch': 1, 'attempt': 'probe-0001',
           'generation': 1, 'runtime_unit': 'fort-runtime.service',
           'vm_package_uuid': '148664c8-5334-5c17-8b7a-d60351f3a79f', 'packet_sha256': 'a' * 64}


class Clock:
    def __init__(self):
        self.value = 0.0

    def __call__(self):
        return self.value

    def advance(self, seconds):
        self.value += seconds


class GuestFixture:
    """Systemd/guest-root public boundary double with observable unit state."""
    boot_id = '12345678-1234-1234-1234-123456789abc'

    def __init__(self):
        self.identity = None
        self.runtime_active = False
        self.tool_active = False
        self.descendants = []
        self.leave_descendant = False
        self.fail_after_start = False

    def ready(self):
        return not self.runtime_active and not self.tool_active and not self.descendants

    def prepare(self, binding):
        self.identity = dict(binding, guest_boot_id=self.boot_id, invocation_id='1' * 32,
                             runtime_cgroup='/fort.slice/fort-attempt.slice/fort-runtime.service')
        self.runtime_active = True
        if self.fail_after_start:
            raise TimeoutError('fixture lost start observation')
        return deepcopy(self.identity)

    def current(self):
        return {'identity': deepcopy(self.identity), 'runtime_active': self.runtime_active,
                'tool_active': self.tool_active, 'empty': not self.descendants and not self.runtime_active and not self.tool_active}

    def grant(self, identity):
        if identity != self.identity or not self.runtime_active:
            raise ValueError('fixture original invocation is not active')
        self.tool_active = True
        self.descendants = [{'pid': 42, 'start': 100, 'setsid': True}]

    def heartbeat(self, identity):
        if identity != self.identity or not self.runtime_active:
            raise ValueError('fixture heartbeat target changed')

    def stop(self, identity):
        if identity != self.identity:
            raise ValueError('fixture cannot attribute stop')
        self.runtime_active = self.tool_active = False
        if not self.leave_descendant:
            self.descendants = []
        return self.current()

    def begin_stop(self, identity):
        self.stop(identity)

    def stop_observation(self, identity):
        if identity != self.identity:
            raise ValueError('fixture stop observation identity changed')
        return self.current()

    def preflight(self):
        return {'guest_boot_id': self.boot_id, 'runtime_active': self.runtime_active,
                'tool_active': self.tool_active, 'empty': self.current()['empty'],
                'root_controls': dict.fromkeys(('provider', 'identity_store', 'sibling', 'seed', 'serial', 'ipc_listener'), True),
                'nics': [], 'shares': []}

    def prepared_controls(self, identity):
        return {'identity': deepcopy(self.identity), 'runtime': fixture_process('runtime'),
                'tool_active': self.tool_active, 'provider_literal_read': True}

    def live_snapshot(self, identity, sequence, now):
        return {'identity': deepcopy(self.identity), 'sequence': sequence, 'captured_at': now,
                'processes': {role: fixture_process(role) for role in ('runtime', 'tool', 'descendant')},
                'setsid': True, 'controls': dict.fromkeys(('C01', 'C02', 'C03', 'C04', 'C05', 'C06', 'C07', 'C08'), True)}


def fixture_process(role):
    runtime = role == 'runtime'
    return {'pid': {'runtime': 40, 'tool': 41, 'descendant': 42}[role], 'start_ticks': 100,
            'uid': 2001 if runtime else 61184,
            'cgroup': '/fort.slice/fort-attempt.slice/fort-' + ('runtime' if runtime else 'tool') + '.service',
            'namespaces': {'net': 11 if runtime else 21, 'mnt': 12 if runtime else 22, 'ipc': 13 if runtime else 23},
            'capabilities': '0000000000000000', 'no_new_privileges': True, 'fd_classes': ['stdio', 'journal'], 'alive': True}
