import type {SubmissionAdmission} from '../service/openai.ts';
import type {Run,InstructionSkill} from '../contracts/v1.ts';
import {parseSkillBundle} from '../contracts/skill-bundle.ts';

export const MVP_PROMPTS:Readonly<Record<1|2|3,readonly string[]>>={
  1:['What sample packing item is saved in this conversation\'s memory?','Our sample code word is cedar. Reply with that word only.','What sample code word did I give you earlier?','What sample packing item and code word have we used?','Reply with the single word ready.'],
  2:['Suggest one short title for a sample packing list.','Suggest another short title for a sample packing list.'],
  3:['Our sample plan is to pack a red notebook. Repeat the item.','Check that sample plan and name the packing item.','What departure is saved in this shared conversation?','Reply with the single word ready.','Write a short synthetic packing checklist for a red notebook.'],
};
const samples=[['End with [CHECKED].','1'],['End with [REVIEWED].','2']] as const;
const sampleDigests=new Map<string,string>(samples.map(([instructions,version])=>[instructions,parseSkillBundle({schemaVersion:1,name:'Check marker',version,source:'owner-local:Fort-instructions',license:'LicenseRef-Owner-Private',entrypoint:'SKILL.md',supportedRuntimes:['codex'],requestedTools:[],scripts:[],files:[{path:'SKILL.md',kind:'file',executable:false,contentBase64:Buffer.from(instructions).toString('base64')}]}).digest]));
const generatedId=/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
function permittedSampleSkill(skill:InstructionSkill):boolean {
  if(!generatedId.test(skill.skillId)||!generatedId.test(skill.revisionId)||skill.name!=='Check marker'||skill.review?.state!=='enabled')return false;
  try{
    const bundle=parseSkillBundle(skill.bundle),expected=sampleDigests.get(skill.instructions);
    return expected!==undefined&&bundle.digest===expected&&skill.review.digest===expected;
  }catch{return false;}
}
export function admitMvpSubmission(submission:SubmissionAdmission,runs:readonly Run[],session:1|2|3):boolean {
  const c=submission.configuration,room=submission.conversation;
  if(runs.length>=12||!MVP_PROMPTS[session].includes(submission.text)||c.selectedModel!=='gpt-6-astra')return false;
  if(c.instructions!==(c.agentName==='Writer'?'Reply briefly using supplied context.':c.agentName==='Reviewer'?'Check the sample claim.':null))return false;
  if(room.kind==='shared'&&(session!==3||room.participantAgentIds.length!==2))return false;
  if(room.kind==='home'&&(room.participantAgentIds.length!==1||room.participantAgentIds[0]!==c.agentId))return false;
  const note=c.memory?.text??'';
  if(!['',...(session===1?['The sample packing item is a red notebook.']:session===3?['The shared sample departure is Monday.']:[])].includes(note))return false;
  if(c.skills.length>1||(c.skills.length>0&&session!==2))return false;
  return c.skills.every(permittedSampleSkill);
}
