import type {SubmissionAdmission} from '../service/openai.ts';
import type {Run} from '../contracts/v1.ts';
export function admitOwnerDemoSubmission(submission:SubmissionAdmission,runs:readonly Run[]):boolean {
  const c=submission.configuration,room=submission.conversation;
  return runs.length<12&&runs.every(run=>run.conversationId===room.conversationId)&&
    submission.text.length>0&&submission.text.isWellFormed()&&Buffer.byteLength(submission.text)<=65536&&
    c.selectedModel==='gpt-6-astra'&&c.agentName==='Writer'&&c.instructions==='Reply briefly using supplied context.'&&
    c.skills.length===0&&c.memory===undefined&&room.kind==='home'&&room.participantAgentIds.length===1&&room.participantAgentIds[0]===c.agentId;
}
