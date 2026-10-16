import {object} from './codex-rpc.ts';
import {StartupCheckError} from './startup-diagnostic.ts';

export const CODEX_PERMISSION_PROFILE='fort_candidate_b_no_files';
export const CODEX_PERMISSION_OVERRIDE='permissions.fort_candidate_b_no_files={filesystem={"/**"="deny"},network={enabled=false}}';

const nullableNetworkFields=['proxy_url','enable_socks5','socks_url','enable_socks5_udp','allow_upstream_proxy','dangerously_allow_non_loopback_proxy','dangerously_allow_all_unix_sockets','mode','domains','unix_sockets','allow_local_binding','mitm'];
function exactFields(value:Record<string,unknown>,required:Readonly<Record<string,unknown>>,nullable:readonly string[]=[]):boolean {
  return Object.entries(required).every(([key,expected])=>value[key]===expected) &&
    Object.keys(value).every(key=>Object.hasOwn(required,key)||(nullable.includes(key)&&value[key]==null));
}

export function verifyPermissionDeclaration(config:Record<string,unknown>):void {
  try {
    const profile=object(object(config.permissions)[CODEX_PERMISSION_PROFILE]);
    const filesystem=object(profile.filesystem),network=object(profile.network);
    if (!exactFields(profile,{filesystem,network},['description','extends','workspace_roots']) ||
        !exactFields(filesystem,{'/**':'deny'},['glob_scan_max_depth']) ||
        !exactFields(network,{enabled:false},nullableNetworkFields)) throw new Error('Unexpected profile');
  } catch {throw new StartupCheckError({stage:'permissions',check:'declaration'});}
}

export function verifyPermissionCatalog(values:unknown[]):void {
  try {
    const profiles=new Map<string,boolean>();
    for(const value of values){
      const profile=object(value);
      if(typeof profile.id!=='string'||typeof profile.allowed!=='boolean'||profiles.has(profile.id))throw new Error('Invalid catalog');
      profiles.set(profile.id,profile.allowed);
    }
    if(profiles.get(CODEX_PERMISSION_PROFILE)!==true)throw new Error('Unconfirmed profile');
  } catch {throw new StartupCheckError({stage:'permissions',check:'catalog'});}
}

export function verifyPermissionSelection(value:unknown):void {
  try {
    if(!exactFields(object(value),{id:CODEX_PERMISSION_PROFILE,extends:null}))throw new Error('Unconfirmed selection');
  } catch {throw new StartupCheckError({stage:'permissions',check:'selection'});}
}
