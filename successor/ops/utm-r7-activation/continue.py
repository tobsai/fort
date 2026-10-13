"""One conditional acquisition dispatch. Run only after the separately approved fixed smoke."""
import json
import os
from pathlib import Path
import re
import sys
import time
sys.path.insert(0, str(Path(__file__).resolve().parent))
from packet_support import (ENV, Journal, check_packet, inspect_environment, require_active,
                            source_module, verify_tools)
from smoke import validate_result


def run_continuation(context, journal, preflight, execute, monotonic=time.monotonic, epoch=time.time):
    case_files = [c['name'].replace('_', '-') + '.json' for c in context.cases]
    if set(journal.names()) != set(['attempt.json', 'complete.json'] + case_files):
        raise ValueError('smoke incomplete, failed, changed or continuation already consumed')
    journal.record('continue-begun.json', {'packet_sha': context.packet_sha, 'source_seal': context.active_sha,
                                           'status': 'consumed'})
    try:
        receipt = journal.read('complete.json')
        if (receipt.get('status') != 'passed' or receipt.get('packet_sha') != context.packet_sha or
                receipt.get('source_seal') != context.active_sha or
                receipt.get('cases') != [c['name'] for c in context.cases] or
                receipt.get('parent_limits_unchanged') is not True or
                not receipt.get('parent_before') or receipt.get('parent_before') != receipt.get('parent_after')):
            raise ValueError('smoke success is not bound to this operation')
        for key, now in (('completed_monotonic', monotonic()), ('completed_epoch', epoch())):
            recorded = receipt.get(key)
            if type(recorded) not in (int, float) or not 0 <= now - recorded <= 300:
                raise ValueError('smoke success is not fresh; no retry under this packet')
        for case, name in zip(context.cases, case_files):
            validate_result(journal.read(name), case, context.identities)
        observations = preflight()
        journal.record('preflight.json', observations)
        journal.record('dispatch.json', {'packet_sha': context.packet_sha, 'source_seal': context.active_sha,
                                        'argv': context.manifest['acquisition']['argv'], 'environment': ENV,
                                        'status': 'dispatch-intended; completion belongs to acquisition receipts'})
        return execute(context.manifest['acquisition']['argv'], dict(ENV))
    except BaseException as error:
        journal.record('continue-failure.json', {'type': type(error).__name__, 'acquisition_success': False})
        raise


def fresh_preflight(context):
    check_packet(context.packet_sha, 'activated')
    require_active(); verify_tools(context)
    inspect_environment(os.environ, 'python')
    identity = '.'.join(map(str, sys.version_info[:3])) + '@' + str(Path(sys.executable).resolve())
    if identity != context.identities['python'] or sys.platform != 'darwin':
        raise ValueError('controller runtime differs')
    commands = source_module('bounded').Commands(guard=require_active)
    root = Path(context.manifest['acquisition']['root'])
    chain_type = source_module('lifecycle').DirectoryChain
    with chain_type(root.parent) as parent:
        try: os.stat(root.name, dir_fd=parent.fd, follow_symlinks=False)
        except FileNotFoundError: pass
        else: raise ValueError('R7 root exists; no adoption, replacement, deletion or retry')
        disk = os.fstatvfs(parent.fd)
        available = disk.f_bavail * disk.f_frsize
        budget = context.manifest['acquisition']['logical_byte_budget']
        if available < budget: raise ValueError('current available disk below logical budget')
        physical, errors = commands.run(['/usr/sbin/sysctl', '-n', 'hw.memsize'], timeout=2, cap=1024)
        if errors or not re.fullmatch(rb'[0-9]{1,15}\s*', physical): raise ValueError('physical memory observation incomplete')
        physical = int(physical)
        if physical < max(c['rss'] for c in context.cases): raise ValueError('physical memory below largest RSS stop threshold')
        pressure, errors = commands.run(['/usr/bin/memory_pressure', '-Q'], timeout=2, cap=4096)
        observed = re.findall(rb'System-wide memory free percentage: ([0-9]{1,3})%', pressure)
        if errors or len(observed) != 1 or not 0 <= int(observed[0]) <= 100:
            raise ValueError('current memory query incomplete')
        parent.check()
        try: os.stat(root.name, dir_fd=parent.fd, follow_symlinks=False)
        except FileNotFoundError: pass
        else: raise ValueError('R7 root appeared during preflight')
        ancestors = [{'path_component': name, **facts} for name, unused, facts in parent.entries]
    return {'packet_sha': context.packet_sha, 'source_seal': context.active_sha,
            'root': str(root), 'root_absent': True, 'nofollow_ancestors': ancestors,
            'disk_available_bytes': available, 'logical_byte_budget': budget,
            'filesystem_overhead_additional': True, 'capacity_reserved': False,
            'physical_memory_bytes': physical, 'memory_free_percentage': int(observed[0]),
            'memory_observation_is_not_a_reservation_or_hard_limit': True,
            'runtime_support': 'fresh matching tool pins plus successful fixed smoke permission/startup receipt',
            'checked_epoch': time.time()}


def main():
    if len(sys.argv) != 3 or sys.argv[1] != '--run-once':
        sys.stderr.write('Inert packet: exact approved --run-once packet digest required.\n'); return 78
    context = check_packet(sys.argv[2], 'activated')
    require_active(); inspect_environment(os.environ, 'python')
    journal = Journal(context)
    try:
        def execute(argv, env):
            # CLOEXEC closes held evidence descriptors. An exec failure is recorded before closing them.
            os.execve(argv[0], argv, env)
        run_continuation(context, journal, lambda: fresh_preflight(context), execute)
    finally: journal.close()
    raise RuntimeError('acquisition exec unexpectedly returned')


if __name__ == '__main__':
    try: raise SystemExit(main())
    except Exception as error:
        sys.stderr.write('Continuation refused or failed: ' + type(error).__name__ + '; no retry.\n')
        raise SystemExit(1)
