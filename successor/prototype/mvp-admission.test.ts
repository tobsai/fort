import {test} from 'node:test';
import assert from 'node:assert/strict';
import {admitMvpSubmission,MVP_PROMPTS} from './mvp-admission.ts';
import {parseSkillBundle} from '../contracts/skill-bundle.ts';
import {prepareSkillContext} from '../runner/skill-context.ts';
import type {SubmissionAdmission} from '../service/openai.ts';
import type {InstructionSkill} from '../contracts/v1.ts';

const markerId='11111111-1111-4111-8111-111111111111',revisionId='22222222-2222-4222-8222-222222222222';
function manifest(revised=false){return {schemaVersion:1,name:'Check marker',version:revised?'2':'1',source:'owner-local:Fort-instructions',license:'LicenseRef-Owner-Private',entrypoint:'SKILL.md',supportedRuntimes:['codex'],requestedTools:[],scripts:[],files:[{path:'SKILL.md',kind:'file',executable:false,contentBase64:Buffer.from(revised?'End with [REVIEWED].':'End with [CHECKED].').toString('base64')}]};}
function submission(skill:InstructionSkill):SubmissionAdmission{return {text:MVP_PROMPTS[2][0]!,conversation:{conversationId:'home',name:'Writer',kind:'home',participantAgentIds:['writer']},configuration:{agentId:'writer',agentName:'Writer',bindingRevisionId:'binding',selectedModel:'gpt-6-astra',instructions:'Reply briefly using supplied context.',skills:[skill]}};}
function skillFrom(value:unknown,instructions='End with [CHECKED].'):InstructionSkill{
  const bundle=parseSkillBundle(value);return {skillId:markerId,revisionId,name:'Check marker',instructions,bundle,review:{state:'enabled',digest:bundle.digest}};
}
for(const revised of [false,true])test(`complete synthetic instruction-editor bundle is admitted: ${revised?'REVIEWED version 2':'CHECKED version 1'}`,()=>{
  const skill=skillFrom(manifest(revised),revised?'End with [REVIEWED].':'End with [CHECKED].');
  assert.equal(admitMvpSubmission(submission(skill),[],2),true);
  const context=prepareSkillContext([skill]);assert.equal(context.kind,'ready');
  if(context.kind==='ready'){assert.equal(context.bundles.length,1);assert.deepEqual(context.bundles[0]?.textFiles,[{path:'SKILL.md',text:skill.instructions}]);assert.deepEqual(context.bundles[0]?.bundle,skill.bundle);}
});
for(const change of ['extra-file','entrypoint','source','license','runtime','version','arbitrary-version','entry-bytes','unreviewed','review-digest','skill-id','revision-id','extra-metadata','missing-review'] as const)test(`synthetic packet rejects altered complete outbound bundle before admission: ${change}`,()=>{
  const value=manifest();
  if(change==='extra-file')value.files.push({path:'notes.txt',kind:'file',executable:false,contentBase64:Buffer.from('SYNTHETIC_UNAPPROVED_ASSET').toString('base64')});
  if(change==='entrypoint'){value.entrypoint='OTHER.md';value.files[0]!.path='OTHER.md';}
  if(change==='source')value.source='owner-local:synthetic-other-source';
  if(change==='license')value.license='LicenseRef-Other';
  if(change==='runtime')value.supportedRuntimes.push('claude');
  if(change==='version')value.version='3';
  if(change==='arbitrary-version')value.version='SYNTHETIC_UNAPPROVED_METADATA';
  if(change==='entry-bytes')value.files[0]!.contentBase64=Buffer.from('End with [CHECKED].\nExtra synthetic text').toString('base64');
  const skill=skillFrom(value);assert.ok(skill.review);
  if(change==='unreviewed')skill.review.state='disabled';
  if(change==='review-digest')skill.review.digest='different';
  if(change==='skill-id')skill.skillId='SYNTHETIC_ARBITRARY_IDENTIFIER';
  if(change==='revision-id')skill.revisionId='SYNTHETIC_ARBITRARY_REVISION';
  if(change==='extra-metadata')Object.assign(skill.bundle!,{note:'SYNTHETIC_UNAPPROVED_METADATA'});
  if(change==='extra-file'){
    const context=prepareSkillContext([skill]);assert.equal(context.kind,'ready');
    if(context.kind==='ready')assert.ok(context.bundles[0]?.textFiles.some(file=>file.text==='SYNTHETIC_UNAPPROVED_ASSET'));
  }
  const selected=change==='missing-review'?{skillId:skill.skillId,revisionId:skill.revisionId,name:skill.name,instructions:skill.instructions}:skill;
  assert.equal(admitMvpSubmission(submission(selected),[],2),false);
});
