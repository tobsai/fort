"""One consumed input-inspection attempt. No UTM, guest, account or provider effects."""
import hashlib
import json
from pathlib import Path
import re
import time
from .acquisition import LocalStore,fixed_plan
from .config import validate_image
from .inspection_guard import require_inspection
from .inspection_workers import Worker

ROOT=Path('/Users/tobiasgunn/Library/Application Support/FortUTMIsolation')


class Inspector:
    def __init__(self,root,worker,clock=time.monotonic):self.root,self.worker,self.clock=Path(root),worker,clock
    def run(self,selection=None):
        entries,_=fixed_plan();pins={e['name']:e for e in entries}
        store=LocalStore(self.root);results={};deadline=self.clock()+1800
        try:
            stages=['acquire','registry','verifier_archive','publisher','ubuntu_inventory']
            if selection is not None:stages.append('ubuntu_selected')
            for operation in stages:
                if self.clock()>=deadline:raise TimeoutError('inspection staging deadline')
                store.record('begun-'+operation+'.json',{'operation':operation,'status':'consumed'})
                arguments={}
                if operation=='publisher':arguments['selection_sha256']=results['verifier_archive']['selection_sha256']
                if operation=='ubuntu_selected':
                    if not isinstance(selection,dict) or set(selection)!={'raw','kernel','initrd'} or len(set(selection.values()))!=3:
                        raise ValueError('three distinct reviewed member names required')
                    observed={row['name'] for row in results['ubuntu_inventory']['entries']}
                    if not set(selection.values())<=observed:raise ValueError('selection absent from authenticated inventory')
                    arguments['names']=selection
                response=self.worker.run(operation,self.root,**arguments)
                if self.clock()>=deadline:raise TimeoutError('inspection staging deadline')
                if (not isinstance(response,dict) or set(response)!={'operation','status','details'}
                        or response['operation']!=operation or response['status']!='complete' or not isinstance(response['details'],dict)):
                    raise ValueError('worker result unbound or incomplete')
                details=response['details']
                if operation=='acquire' and (type(details.get('completed')) is not int or details['completed']!=8):raise ValueError('input set incomplete')
                if operation=='registry' and (type(details.get('registry_signatures')) is not int or details['registry_signatures']!=2):raise ValueError('registry authentication incomplete')
                if operation=='registry' and details.get('archive_sha512')!=pins['openpgp-6.3.2.tgz']['sha512']:raise ValueError('registry archive identity differs')
                if operation=='verifier_archive' and not re.fullmatch('[a-f0-9]{64}',details.get('selection_sha256','')):raise ValueError('selected module provenance missing')
                if operation=='publisher':
                    controls=details.get('negative_controls',{})
                    if set(controls)!={'rejected'} or type(controls['rejected']) is not int or controls['rejected']!=4:raise ValueError('publisher negative controls incomplete')
                    publisher=details.get('publisher',{})
                    if (set(publisher)!={'filename','sha256','primaryFingerprint','signaturesVerified'}
                            or publisher['filename']!='noble-server-cloudimg-arm64.tar.gz'
                            or publisher['sha256']!=pins[publisher['filename']]['sha256']
                            or publisher['primaryFingerprint']!='D2EB44626FDDC30B513D5BB71A5D6C4C7DB87C81'
                            or type(publisher['signaturesVerified']) is not int or publisher['signaturesVerified']<1
                            or not re.fullmatch('[a-f0-9]{64}',details.get('module_sha256',''))):raise ValueError('publisher identity or signature result incomplete')
                if operation=='ubuntu_inventory' and details.get('selection_status')!='requires-exact-reviewed-member-selection':raise ValueError('inventory selection state differs')
                if operation=='ubuntu_selected':
                    validate_image(details)
                    archive=next(e for e in fixed_plan()[0] if e['name']=='noble-server-cloudimg-arm64.tar.gz')
                    if details['archive_sha256']!=archive['sha256'] or any(details[role+'_member']!=name for role,name in selection.items()):raise ValueError('selected image binding differs')
                store.record('result-'+operation+'.json',response,cap=65536);results[operation]=details
            result={'inputs_authenticated':True,'image_bound':selection is not None,'member_selection_required':selection is None,'runtime_authorized':False}
            store.record('inspection-complete.json',result);return result
        except BaseException as error:
            try:store.record('inspection-failure.json',{'status':'failed','error_class':type(error).__name__,'accepted':False})
            except BaseException:pass
            raise
        finally:store.close()


def main(accepted_source_sha256):
    require_inspection()
    from .source_identity import verify_source
    verify_source(accepted_source_sha256)
    return Inspector(ROOT,Worker(accepted_source_sha256)).run()
