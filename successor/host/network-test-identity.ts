import { mkdtempSync, writeFileSync, readFileSync, rmSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { join } from 'node:path';
import { tmpdir } from 'node:os';

export function networkTestIdentity() {
  const directory=mkdtempSync(join(tmpdir(),'fort-tls-fixture-'));
  const run=(args:string[])=>execFileSync('/usr/bin/openssl',args,{cwd:directory,env:{},stdio:'pipe',timeout:10_000});
  try {
    writeFileSync(join(directory,'root.conf'),'[req]\nprompt=no\ndistinguished_name=dn\nx509_extensions=extensions\n[dn]\nCN=Fort disposable test root\n[extensions]\nbasicConstraints=critical,CA:TRUE\nkeyUsage=critical,keyCertSign,cRLSign\n',{mode:0o600});
    writeFileSync(join(directory,'leaf.conf'),'[req]\nprompt=no\ndistinguished_name=dn\n[dn]\nCN=Fort disposable test server\n[extensions]\nbasicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\nsubjectAltName=IP:127.0.0.1,DNS:fort-fixture.local\n',{mode:0o600});
    run(['req','-x509','-newkey','rsa:2048','-nodes','-sha256','-days','2','-config','root.conf','-keyout','root.key','-out','root.pem']);
    run(['req','-new','-newkey','rsa:2048','-nodes','-sha256','-config','leaf.conf','-keyout','leaf.key','-out','leaf.csr']);
    run(['x509','-req','-in','leaf.csr','-CA','root.pem','-CAkey','root.key','-set_serial','1','-days','2','-sha256','-extfile','leaf.conf','-extensions','extensions','-out','leaf.pem']);
    return {certificate:readFileSync(join(directory,'leaf.pem')),privateKey:readFileSync(join(directory,'leaf.key')),ca:readFileSync(join(directory,'root.pem'))};
  } finally {rmSync(directory,{recursive:true,force:true});}
}
