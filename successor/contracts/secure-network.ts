export type NetworkOwner = Readonly<{installationId:string;deviceId:string}>;
export type NetworkConfiguration = Readonly<{
  bindAddress:string;prefixLength:number;port:number;serverName:string;
  certificate:string;privateKey:string;
}>;
export type NetworkCommand =
  | Readonly<{operation:'inspect';owner:NetworkOwner}>
  | Readonly<{operation:'start';owner:NetworkOwner;sessionId:string;generation:number;configuration:NetworkConfiguration}>
  | Readonly<{operation:'stop';owner:NetworkOwner;sessionId:string;generation:number}>;
export type NetworkListenerState =
  | Readonly<{state:'stopped';cause:'initial'|'requested'|'startFailed'|'interrupted'}>
  | Readonly<{state:'starting'|'stopping'|'unconfirmed'}>
  | Readonly<{state:'active';url:string}>;
export type NetworkStatus = Readonly<{
  owner:NetworkOwner;sessionId:string;generation:number;listener:NetworkListenerState;
}>;
export type SecureNetworkControl = {
  execute(command:unknown):Promise<NetworkStatus>;
  close():Promise<void>;
};
