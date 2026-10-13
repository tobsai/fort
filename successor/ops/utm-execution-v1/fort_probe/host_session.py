"""One bounded host session over already-attributed control/inspection channels."""
from .evidence import exact


def run(control,inspection,binding,clock,wait):
    started=clock()
    def within():
        if not 0<=clock()-started<45: raise TimeoutError('host session deadline')
    preflight=inspection.request({'op':'preflight'},lambda r: set(r)=={'event','facts'} and r['event']=='preflight' and r['facts'].get('ready') is True)
    boot=preflight['facts']['guest_boot_id']
    def prepared(value):
        if set(value)!={'event','identity'} or value['event']!='prepared': return False
        identity=value['identity']
        return (set(identity)==set(binding)|{'guest_boot_id','invocation_id','runtime_cgroup'}
                and exact({k:identity[k] for k in binding},binding) and identity['guest_boot_id']==boot
                and identity['runtime_cgroup']=='/fort.slice/fort-attempt.slice/fort-runtime.service')
    identity=control.request({'op':'prepare','sequence':0},prepared)['identity']
    for sequence in range(1,21):
        within()
        if sequence>1: wait(.25)
        message={'op':'grant' if sequence==1 else 'heartbeat','sequence':sequence,'identity':identity}
        control.request(message,lambda r: exact(r,{'event':'accepted','sequence':sequence,'identity':identity}))
    live=inspection.request({'op':'live','identity':identity},lambda r:set(r)=={'event','facts'} and r['event']=='live'
                            and exact(r['facts'].get('identity'),identity) and type(r['facts'].get('sequence')) is int and r['facts']['sequence']==20)
    within()
    terminal=control.stop({'op':'stop','sequence':21,'identity':identity})
    if terminal['live_evidence_proven'] is not True: raise ValueError('live evidence remains unproven')
    post=inspection.request({'op':'postflight'},lambda r:set(r)=={'event','facts'} and r['event']=='postflight'
                            and exact(r['facts'].get('identity'),identity) and r['facts'].get('state')=='stopped-empty'
                            and r['facts'].get('guest',{}).get('empty') is True)
    within()
    return {'identity':identity,'live':live['facts'],'stop':terminal,'postflight':post['facts']}
