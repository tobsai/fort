import { createHash } from 'node:crypto';
import type { RoutineDefinition, RoutineExecution, RoutineInput, RoutineStep, RoutineStepProgress, RoutineValue } from '../contracts/v1.ts';

export class InvalidRoutine extends Error {}
function reject(message: string): never { throw new InvalidRoutine(message); }
function object(value: unknown): Record<string,unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) reject('Use a routine object.');
  return value as Record<string,unknown>;
}
function only(value:Record<string,unknown>, keys:readonly string[]):void { if(Object.keys(value).some(key=>!keys.includes(key)))reject('Unsupported routine field.'); }
function text(value: unknown, limit=8192): string {
  if (typeof value !== 'string' || !value.isWellFormed() || !value.trim() || Buffer.byteLength(value)>limit) reject('A routine field is missing or exceeds its limit.');
  return value;
}
function id(value: unknown): string { const result=text(value,128);if(!/^[A-Za-z0-9_-]+$/.test(result))reject('Use a stable routine identifier.');return result; }
function list(value: unknown, limit: number): unknown[] { if(!Array.isArray(value)||value.length>limit)reject('A routine list exceeds its limit.');return value; }
function ids(value: unknown): string[] { const values=list(value,16).map(id);if(new Set(values).size!==values.length)reject('Routine references cannot be repeated.');return values; }
export const routineDigest = (value: unknown) => createHash('sha256').update(JSON.stringify(value)).digest('hex');
export const outputDigest = (value: string) => createHash('sha256').update(value).digest('hex');

export function parseRoutineDefinition(value: unknown): RoutineDefinition {
  const v=object(value);
  if(v.schemaVersion!==1)reject('Unsupported routine schema.');
  if(Object.keys(v).some(key=>!['schemaVersion','name','agentId','conversationId','bindingRevisionId','instructions','inputs','skills','steps'].includes(key)))reject('Unsupported routine field.');
  const inputs=list(v.inputs,16).map((value):RoutineInput=>{
    const input=object(value);only(input,['name','type','required']);if(!['text','number','boolean'].includes(String(input.type))||typeof input.required!=='boolean')reject('Declare each input type and whether it is required.');
    if(input.type!=='text'&&input.type!=='number'&&input.type!=='boolean')reject('Unsupported routine input type.');
    return {name:id(input.name),type:input.type,required:input.required};
  });
  if(new Set(inputs.map(input=>input.name)).size!==inputs.length)reject('Input names must be unique.');
  const skills=list(v.skills,8).map(value=>{const skill=object(value);only(skill,['skillId','revisionId','digest']);if(typeof skill.digest!=='string'||!/^[0-9a-f]{64}$/.test(skill.digest))reject('Pin the exact skill digest.');return {skillId:id(skill.skillId),revisionId:id(skill.revisionId),digest:text(skill.digest,64)};});
  if(new Set(skills.map(skill=>skill.skillId)).size!==skills.length)reject('Select each skill once.');
  const rawSteps=list(v.steps,16);
  if(rawSteps.length===0)reject('Add at least one routine step.');
  const seen=new Set<string>(), tasks=new Set<string>();
  const steps:RoutineStep[]=rawSteps.map(value=>{
    const step=object(value),stepId=id(step.stepId),name=text(step.name,128);
    if(seen.has(stepId))reject('Step identifiers must be unique.');seen.add(stepId);
    if(step.type==='approval'){only(step,['stepId','name','type','prompt']);return {stepId,name,type:'approval',prompt:text(step.prompt)};}
    if(step.type!=='task')reject('Unsupported routine step type.');
    only(step,['stepId','name','type','instructions','inputNames','outputStepIds']);
    const inputNames=ids(step.inputNames),outputStepIds=ids(step.outputStepIds);
    if(inputNames.some(name=>!inputs.some(input=>input.name===name))||outputStepIds.some(stepId=>!tasks.has(stepId)))reject('A task may read declared inputs and earlier task outputs only.');
    tasks.add(stepId);
    return {stepId,name,type:'task',instructions:text(step.instructions),inputNames,outputStepIds};
  });
  return {schemaVersion:1,name:text(v.name,128),agentId:id(v.agentId),conversationId:id(v.conversationId),bindingRevisionId:id(v.bindingRevisionId),instructions:text(v.instructions,16_384),inputs,skills,steps};
}

export function parseRoutineInputs(definition: RoutineDefinition, value: unknown): Record<string,RoutineValue> {
  const supplied=object(value),entries:[string,RoutineValue][]=[];
  if(Object.keys(supplied).some(name=>!definition.inputs.some(input=>input.name===name)))reject('Only declared routine inputs are accepted.');
  for(const input of definition.inputs){
    const value=supplied[input.name];
    if(value===undefined){if(input.required)reject('A required routine input is missing.');continue;}
    if(input.type==='text'&&typeof value==='string'&&value.isWellFormed()&&Buffer.byteLength(value)<=4096)entries.push([input.name,value]);
    else if(input.type==='number'&&typeof value==='number'&&Number.isFinite(value))entries.push([input.name,value]);
    else if(input.type==='boolean'&&typeof value==='boolean')entries.push([input.name,value]);
    else reject('A routine input does not match its declared type or size.');
  }
  return Object.fromEntries(entries);
}

export function startRoutine(routineId: string, revisionId: string, definition: RoutineDefinition, inputs: Record<string,RoutineValue>): RoutineExecution {
  return {routineId,revisionId,definition,inputs,steps:definition.steps.map(step=>({stepId:step.stepId,stepRevisionId:routineDigest(step),state:'pending'}))};
}

export function parseRoutineExecution(value: unknown): RoutineExecution {
  const v=object(value),definition=parseRoutineDefinition(v.definition),inputs=parseRoutineInputs(definition,v.inputs),raw=list(v.steps,16);
  if(raw.length!==definition.steps.length)reject('Routine checkpoints do not match the accepted steps.');
  const steps:RoutineStepProgress[]=raw.map((value,index)=>{
    const step=object(value),expected=definition.steps[index]!;
    if(step.stepId!==expected.stepId||step.stepRevisionId!==routineDigest(expected))reject('A routine checkpoint revision changed.');
    const identity={stepId:expected.stepId,stepRevisionId:routineDigest(expected)};
    if(step.state==='pending')return {...identity,state:'pending'};
    if(step.state==='waitingForApproval'||step.state==='approved'||step.state==='rejected'){
      if(expected.type!=='approval')reject('Only approval steps can hold owner decisions.');
      return {...identity,state:step.state,attemptId:id(step.attemptId),approvalId:id(step.approvalId)};
    }
    if(step.state==='running'){if(expected.type!=='task')reject('Only tasks can invoke a runtime.');return {...identity,state:'running',attemptId:id(step.attemptId)};}
    if(step.state==='completed'){
      if(expected.type!=='task')reject('Only tasks can hold runtime outputs.');
      const output=object(step.output);
      if(typeof output.text!=='string'||!output.text.isWellFormed()||Buffer.byteLength(output.text)>65_536||output.digest!==outputDigest(output.text))reject('A routine output does not match its digest.');
      return {...identity,state:'completed',attemptId:id(step.attemptId),output:{messageId:id(output.messageId),text:output.text,digest:output.digest}};
    }
    if(step.state==='failed'||step.state==='outcomeUnknown'||step.state==='canceled')return {...identity,state:step.state,detail:text(step.detail),...(step.attemptId===undefined?{}:{attemptId:id(step.attemptId)})};
    reject('Unsupported routine checkpoint.');
  });
  return {routineId:id(v.routineId),revisionId:id(v.revisionId),definition,inputs,steps};
}

export function taskInput(routine: RoutineExecution, stepId: string): string {
  const step=routine.definition.steps.find(step=>step.stepId===stepId);
  if(!step||step.type!=='task')throw new InvalidRoutine('The selected step is not a task.');
  const previousOutputs=step.outputStepIds.map(stepId=>{
    const prior=routine.steps.find(step=>step.stepId===stepId);
    if(!prior||prior.state!=='completed')throw new InvalidRoutine('A required earlier output is not complete.');
    return {stepId,...prior.output};
  });
  return JSON.stringify({routineId:routine.routineId,routineRevisionId:routine.revisionId,stepId,instructions:step.instructions,inputs:Object.fromEntries(step.inputNames.filter(name=>name in routine.inputs).map(name=>[name,routine.inputs[name]])),previousOutputs});
}

export function stopRoutine(routine: RoutineExecution, state:'failed'|'outcomeUnknown'|'canceled', detail:string): RoutineExecution {
  return {...routine,steps:routine.steps.map(step=>{
    const identity={stepId:step.stepId,stepRevisionId:step.stepRevisionId};
    if(step.state==='running')return {...identity,state,attemptId:step.attemptId,detail};
    if((step.state==='pending'||step.state==='waitingForApproval')&&state!=='outcomeUnknown')return {...identity,state:'canceled',detail};
    return step;
  })};
}
