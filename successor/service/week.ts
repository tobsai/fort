import { randomUUID } from 'node:crypto';
import type { Week, WeekBuild, WeekGoal, WeekOwner, WeekPlanItem, WeekProgress, WeekSize, WeekView, WeekZone } from '../contracts/v1.ts';

export class InvalidWeek extends Error {}
export class WeekConflict extends Error {}
export const SIZE_POINTS:Record<WeekSize,number>={small:1,moderate:3,significant:6,milestone:12};
const ZONES:WeekZone[]=['code','money','writing','personal'];
const day=86_400_000;
function reject(message:string):never { throw new InvalidWeek(message); }
function object(value:unknown,fields:string[]):Record<string,unknown> {
  if(!value||typeof value!=='object'||Array.isArray(value))reject('Use a week object.');
  if(Object.keys(value).some(key=>!fields.includes(key)))reject('Unsupported week fields.');
  return value as Record<string,unknown>;
}
function words(value:unknown,limit:number,label:string):string { if(typeof value!=='string'||!value.trim()||value.length>limit)reject(`Use a ${label} of 1 to ${limit} characters.`);return value; }
function size(value:unknown):WeekSize { if(typeof value!=='string'||!(value in SIZE_POINTS))reject('Choose small, moderate, significant or milestone.');return value as WeekSize; }
function owner(value:unknown,agentIds:ReadonlySet<string>):WeekOwner {
  const v=object(value,['kind','agentId']);
  if(v.kind==='you'&&v.agentId===undefined)return {kind:'you'};
  if(v.kind==='agent'&&typeof v.agentId==='string'&&agentIds.has(v.agentId))return {kind:'agent',agentId:v.agentId};
  reject('Assign the work to you or an existing agent.');
}
function date(value:unknown):string {
  if(typeof value!=='string'||!/^\d{4}-\d{2}-\d{2}$/.test(value)||new Date(value+'T00:00:00Z').toISOString().slice(0,10)!==value)reject('Use a valid start date.');return value;
}
function item(v:Record<string,unknown>,goalId:string,agentIds:ReadonlySet<string>):WeekPlanItem {
  return {itemId:randomUUID(),goalId,task:words(v.task,200,'task'),owner:owner(v.owner,agentIds),size:size(v.size),...(v.unlocks===undefined?{}:{unlocks:words(v.unlocks,80,'unlock name')}),doneBuildId:null};
}

export function foundWeek(body:unknown,agentIds:ReadonlySet<string>,number:number):Week {
  const v=object(body,['name','startsOn','goals','plan','mainGoalKey']);
  if(!Array.isArray(v.goals)||v.goals.length<1||v.goals.length>6)reject('Pick 1 to 6 goals.');
  if(!Array.isArray(v.plan)||v.plan.length>100)reject('Plan at most 100 tasks.');
  const keys=new Map<string,string>();
  const goals=v.goals.map((value):WeekGoal=>{
    const g=object(value,['key','text','zone','lead','targetPoints']),key=words(g.key,64,'goal key');
    if(keys.has(key))reject('Use a distinct key for each goal.');
    if(typeof g.zone!=='string'||!ZONES.includes(g.zone as WeekZone))reject('Choose the code, money, writing or personal zone.');
    if(typeof g.targetPoints!=='number'||!Number.isInteger(g.targetPoints)||g.targetPoints<1||g.targetPoints>500)reject('Use a whole target of 1 to 500 points.');
    const goalId=randomUUID();keys.set(key,goalId);
    return {goalId,text:words(g.text,120,'goal'),zone:g.zone as WeekZone,lead:owner(g.lead,agentIds),targetPoints:g.targetPoints};
  });
  const goalOf=(key:unknown)=>{const goalId=typeof key==='string'?keys.get(key):undefined;if(!goalId)reject('Place each task under a chosen goal.');return goalId;};
  const plan=v.plan.map(value=>{const p=object(value,['goalKey','task','owner','size','unlocks']);return item(p,goalOf(p.goalKey),agentIds);});
  const startsOn=date(v.startsOn);
  return {weekId:randomUUID(),revisionId:randomUUID(),number,name:words(v.name,60,'city name'),startsOn,endsOn:new Date(Date.parse(startsOn+'T00:00:00Z')+6*day).toISOString().slice(0,10),state:'live',mainGoalId:v.mainGoalKey===undefined?null:goalOf(v.mainGoalKey),goals,plan,builds:[]};
}
function goalIn(week:Week,value:unknown):string { if(typeof value!=='string'||!week.goals.some(goal=>goal.goalId===value))reject('Choose one of this week’s goals.');return value; }
export function addPlanItem(week:Week,body:unknown,agentIds:ReadonlySet<string>):Week {
  const v=object(body,['expectedRevisionId','goalId','task','owner','size','unlocks']);
  if(week.plan.length>=100)reject('Plan at most 100 tasks.');
  return {...week,revisionId:randomUUID(),plan:[...week.plan,item(v,goalIn(week,v.goalId),agentIds)]};
}
/** Only the owner's device calls this: an agent can suggest a size, but nothing lands until the owner confirms it. */
export function placeBuild(week:Week,body:unknown,agentIds:ReadonlySet<string>,placedAt:string):Week {
  const v=object(body,['expectedRevisionId','itemId','goalId','task','by','size','evidence']);
  if(week.builds.length>=500)reject('A week holds at most 500 builds.');
  const evidence=v.evidence===undefined?{}:{evidence:words(v.evidence,200,'evidence note')},buildId=randomUUID();
  let build:WeekBuild;
  if(v.itemId!==undefined){
    if(v.goalId!==undefined||v.task!==undefined||v.by!==undefined)reject('A planned task already names its goal, task and owner.');
    const planned=week.plan.find(entry=>entry.itemId===v.itemId);
    if(!planned)reject('Choose a task from this week’s plan.');
    if(planned.doneBuildId)throw new WeekConflict('That task is already built.');
    build={buildId,goalId:planned.goalId,itemId:planned.itemId,task:planned.task,size:size(v.size),by:planned.owner,placedAt,...evidence};
  } else build={buildId,goalId:goalIn(week,v.goalId),itemId:null,task:words(v.task,200,'task'),size:size(v.size),by:owner(v.by,agentIds),placedAt,...evidence};
  return {...week,revisionId:randomUUID(),builds:[...week.builds,build],plan:week.plan.map(entry=>entry.itemId===build.itemId?{...entry,doneBuildId:buildId}:entry)};
}
export function closeWeek(week:Week,body:unknown,closedAt:string):Week { object(body,['expectedRevisionId']);return {...week,revisionId:randomUUID(),state:'closed',closedAt}; }

export function weekProgress(week:Pick<Week,'goals'|'builds'>):WeekProgress {
  const goals=week.goals.map(goal=>{const points=week.builds.filter(build=>build.goalId===goal.goalId).reduce((sum,build)=>sum+SIZE_POINTS[build.size],0);return {goalId:goal.goalId,points,targetPoints:goal.targetPoints,met:points>=goal.targetPoints};});
  // Your own list adds points but never moves the grade: the grade is about the crew's goals.
  const counted=goals.filter((_,index)=>week.goals[index].zone!=='personal'),met=counted.filter(goal=>goal.met).length,share=counted.length?met/counted.length:0;
  return {points:goals.reduce((sum,goal)=>sum+goal.points,0),goals,goalsMet:met,goalsCounted:counted.length,grade:!counted.length?null:share>=1?'A':share>=2/3?'B':share>=1/3?'C':'D'};
}
export function viewWeek(week:Week):WeekView { return {...week,progress:weekProgress(week)}; }
export function decodeWeek(value:unknown):Week {
  const v=value as Week;
  if(!v||typeof v!=='object'||typeof v.weekId!=='string'||typeof v.revisionId!=='string'||(v.state!=='live'&&v.state!=='closed')||!Array.isArray(v.goals)||!Array.isArray(v.plan)||!Array.isArray(v.builds)||v.builds.some(build=>!(build.size in SIZE_POINTS)))throw new Error('Invalid stored week');
  return v;
}
