"""Narrow UTM 4.7.5 source schema, pinned to the retained official Swift definitions.

Parses supplied raw registry data without resolving bookmarks or touching app state.
A live collector must supply fresh persisted/in-memory agreement and kernel/URL facts.
"""
import hashlib
from uuid import UUID
from .evidence import exact

SOURCE_COMMIT='048ca7498ea3a374439149d51739d94c5300bcda'
NAME='fort-utm-isolation-probe'


def pinned_schema(binding):
    uuid=str(UUID(binding['vm_package_uuid']))
    return {'source_commit':SOURCE_COMMIT,'version':4,
            'information':{'Name':NAME,'UUID':uuid,'IconCustom':False}}


def registry_entry(registry,vm_uuid,package_path,resolution=None):
    uuid=str(UUID(vm_uuid))
    if not isinstance(registry,dict):raise ValueError('raw Registry dictionary required')
    matches=[value for key,value in registry.items() if isinstance(key,str) and key.lower()==uuid]
    if len(matches)!=1:raise ValueError('exact registry UUID entry required')
    entry=matches[0]
    required={'Name','Package','UUID','Suspended','ExternalDrives','SharedDirectories','WindowSettings'}
    optional={'TerminalSettings','ResolutionSettings','MigratedConfig','MacRecoveryIpsw'}
    if not isinstance(entry,dict) or not required<=set(entry) or set(entry)-required-optional:
        raise ValueError('registry entry schema differs')
    if (not isinstance(entry['UUID'],str) or entry['UUID'].lower()!=uuid or entry['Name']!=NAME or entry['Suspended'] is not False
            or entry['ExternalDrives']!={} or entry['SharedDirectories']!=[] or entry.get('MacRecoveryIpsw') is not None):
        raise ValueError('registry identity, attachment or share differs')
    package=entry['Package']
    if (not isinstance(package,dict) or not {'Path','Bookmark','ReadOnly'}<=set(package)
            or set(package)-{'Path','Bookmark','ReadOnly','BookmarkRemote'} or package['Path']!=package_path
            or package['ReadOnly'] is not False or type(package['Bookmark']) is not bytes
            or package.get('BookmarkRemote') not in (None,b'')):
        raise ValueError('registry package identity differs')
    if package['Bookmark']:
        expected={'bookmark_sha256':hashlib.sha256(package['Bookmark']).hexdigest(),'path':package_path,
                  'stale':False,'without_ui':True,'without_mounting':True}
        if not exact(resolution,expected):raise ValueError('package bookmark unresolved or points elsewhere')
    elif resolution is not None:raise ValueError('unexpected bookmark resolution')
    return {'uuid':uuid,'shares':[],'complete':True}


def registry_snapshot(snapshot,vm_uuid,package_path):
    if (not isinstance(snapshot,dict) or set(snapshot)!={'persisted','in_memory','bookmark_resolution'}
            or snapshot['persisted']!=snapshot['in_memory']):
        raise ValueError('fresh persisted and in-memory registry agreement required')
    return registry_entry(snapshot['persisted'],vm_uuid,package_path,snapshot['bookmark_resolution'])
