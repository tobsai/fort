"""Validate root-observed fixture facts; a tool's stdout is never a root observation."""
import json
import math

RUNTIME_CGROUP = '/fort.slice/fort-attempt.slice/fort-runtime.service'
TOOL_CGROUP = '/fort.slice/fort-attempt.slice/fort-tool.service'


def exact(left, right):
    return json.dumps(left, sort_keys=True, allow_nan=False) == json.dumps(right, sort_keys=True, allow_nan=False)


def process(value, cgroup, uid=None):
    required = {'pid', 'start_ticks', 'uid', 'cgroup', 'namespaces', 'capabilities', 'no_new_privileges', 'fd_classes', 'alive'}
    if not isinstance(value, dict) or set(value) != required:
        raise ValueError('incomplete trusted process observation')
    for key in ('pid', 'start_ticks', 'uid'):
        if type(value[key]) is not int or value[key] <= 0:
            raise ValueError('invalid process identity')
    if value['cgroup'] != cgroup or value['alive'] is not True:
        raise ValueError('process outside live owned cgroup')
    if uid is not None and value['uid'] != uid:
        raise ValueError('wrong process UID')
    ns = value['namespaces']
    if set(ns) != {'net', 'mnt', 'ipc'} or any(type(v) is not int or v <= 0 for v in ns.values()):
        raise ValueError('namespace identity missing')
    if value['capabilities'] != '0000000000000000' or value['no_new_privileges'] is not True:
        raise ValueError('unexpected privilege')
    if value['fd_classes'] != ['stdio', 'journal']:
        raise ValueError('unexpected descriptor class')


def preflight(value, boot_id):
    if set(value) != {'guest_boot_id', 'runtime_active', 'tool_active', 'empty', 'root_controls', 'nics', 'shares'}:
        raise ValueError('incomplete preflight')
    if (value['guest_boot_id'] != boot_id or value['runtime_active'] is not False
            or value['tool_active'] is not False or value['empty'] is not True
            or value['nics'] != [] or value['shares'] != []):
        raise ValueError('preflight requires inactive units and no network/shares')
    controls = value['root_controls']
    if set(controls) != {'provider', 'identity_store', 'sibling', 'seed', 'serial', 'ipc_listener'} or any(v is not True for v in controls.values()):
        raise ValueError('root positive controls incomplete')
    return dict(value, ready=True)


def prepared(value, identity):
    if set(value) != {'identity', 'runtime', 'tool_active', 'provider_literal_read'}:
        raise ValueError('prepared control fields incomplete')
    if not exact(value['identity'], identity) or value['tool_active'] is not False or value['provider_literal_read'] is not True:
        raise ValueError('prepared runtime controls failed')
    process(value['runtime'], RUNTIME_CGROUP, 2001)


def live(value, identity, sequence, started, now, runtime):
    if set(value) != {'identity', 'sequence', 'captured_at', 'processes', 'setsid', 'controls'}:
        raise ValueError('live snapshot fields incomplete')
    captured = value['captured_at']
    if (type(captured) not in (int, float) or not math.isfinite(captured)
            or not started <= captured <= now or now - captured >= .5
            or type(value['sequence']) is not int or value['sequence'] != sequence
            or not exact(value['identity'], identity)):
        raise ValueError('stale or replayed live snapshot')
    p = value['processes']
    if set(p) != {'runtime', 'tool', 'descendant'}:
        raise ValueError('required live process missing')
    process(p['runtime'], RUNTIME_CGROUP, 2001)
    if not exact(p['runtime'], runtime):
        raise ValueError('runtime process changed since prepare')
    process(p['tool'], TOOL_CGROUP)
    process(p['descendant'], TOOL_CGROUP, p['tool']['uid'])
    if not 61184 <= p['tool']['uid'] <= 65519 or value['setsid'] is not True:
        raise ValueError('tool UID or detached descendant invalid')
    if len({v['pid'] for v in p.values()}) != 3:
        raise ValueError('process identities are not distinct')
    if any(p['runtime']['namespaces'][name] == p['tool']['namespaces'][name] for name in ('net', 'mnt', 'ipc')):
        raise ValueError('tool namespaces not isolated from runtime')
    if not exact(p['descendant']['namespaces'], p['tool']['namespaces']):
        raise ValueError('descendant namespaces changed')
    if set(value['controls']) != {'C01', 'C02', 'C03', 'C04', 'C05', 'C06', 'C07', 'C08'} or any(v is not True for v in value['controls'].values()):
        raise ValueError('tool controls incomplete or failed')
