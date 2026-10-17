import type { CalendarOccurrence, ScheduleDefinition } from '../contracts/v1.ts';
import { InvalidRoutine } from './routine.ts';

const day=86_400_000;
function reject(message:string):never { throw new InvalidRoutine(message); }
function object(value:unknown):Record<string,unknown> { if(!value||typeof value!=='object'||Array.isArray(value))reject('Use a schedule object.');return value as Record<string,unknown>; }
function identifier(value:unknown):string { if(typeof value!=='string'||!/^[-A-Za-z0-9_]{1,128}$/.test(value))reject('Pin the exact saved routine.');return value; }
const formatters=new Map<string,Intl.DateTimeFormat>();
function formatter(timezone:string):Intl.DateTimeFormat {
  let value=formatters.get(timezone);
  if(!value){try{value=new Intl.DateTimeFormat('en-CA',{timeZone:timezone,year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',hourCycle:'h23'});}catch{reject('Select an IANA timezone.');}formatters.set(timezone,value);}
  return value;
}
function local(instant:number,timezone:string):string {
  const parts=Object.fromEntries(formatter(timezone).formatToParts(instant).map(part=>[part.type,part.value]));
  return `${parts.year}-${parts.month}-${parts.day}T${parts.hour}:${parts.minute}`;
}
function date(value:unknown):string {
  if(typeof value!=='string'||!/^\d{4}-\d{2}-\d{2}$/.test(value)||!Number.isFinite(Date.parse(value+'T00:00:00Z'))||new Date(value+'T00:00:00Z').toISOString().slice(0,10)!==value)reject('Use a valid calendar date.');return value;
}
export function parseSchedule(value:unknown):ScheduleDefinition {
  const v=object(value);
  if(Object.keys(v).some(key=>!['schemaVersion','routineId','routineRevisionId','timezone','time','kind','date','weekdays','inputs','missed'].includes(key))||v.schemaVersion!==1)reject('Unsupported schedule fields.');
  if(typeof v.timezone!=='string'||v.timezone.length>128||/^[+-]/.test(v.timezone))reject('Select an IANA timezone.');formatter(v.timezone);
  if(typeof v.time!=='string'||!/^([01]\d|2[0-3]):[0-5]\d$/.test(v.time))reject('Use a local time in hours and minutes.');
  if(v.missed!=='skip'&&v.missed!=='catchUpOne')reject('Select skip missed or one catch-up occurrence.');
  const inputs:ScheduleDefinition['inputs']=Object.fromEntries(Object.entries(object(v.inputs)).map(([key,value])=>{if(typeof value!=='string'&&typeof value!=='boolean'&&(typeof value!=='number'||!Number.isFinite(value)))reject('Use typed routine inputs.');return [key,value];}));
  const base:Pick<ScheduleDefinition,'schemaVersion'|'routineId'|'routineRevisionId'|'timezone'|'time'|'inputs'|'missed'>={schemaVersion:1 as const,routineId:identifier(v.routineId),routineRevisionId:identifier(v.routineRevisionId),timezone:v.timezone,time:v.time,inputs,missed:v.missed};
  if(v.kind==='once'&&v.weekdays===undefined)return {...base,kind:'once',date:date(v.date)};
  if(v.kind==='daily'&&v.date===undefined&&v.weekdays===undefined)return {...base,kind:'daily'};
  if(v.kind==='weekly'&&v.date===undefined&&Array.isArray(v.weekdays)&&v.weekdays.length>0&&v.weekdays.length<=7&&v.weekdays.every(value=>Number.isInteger(value)&&value>=1&&value<=7)&&new Set(v.weekdays).size===v.weekdays.length)return {...base,kind:'weekly',weekdays:[...v.weekdays].sort((a,b)=>a-b)};
  reject('Choose one-time, daily or weekly recurrence.');
}
function resolveWall(wall:string,timezone:string):number|undefined {
  const nominal=Date.parse(wall+':00Z'),candidates=new Set<number>();
  for(let delta=-36;delta<=36;delta+=6){
    const sample=nominal+delta*3_600_000,offset=Date.parse(local(sample,timezone)+':00Z')-sample;
    const candidate=nominal-offset;if(local(candidate,timezone)===wall)candidates.add(candidate);
  }
  return candidates.size?Math.min(...candidates):undefined;
}
function onDate(definition:ScheduleDefinition,localDate:string):CalendarOccurrence|undefined {
  const weekday=new Date(localDate+'T00:00:00Z').getUTCDay()||7;
  if(definition.kind==='once'&&definition.date!==localDate||definition.kind==='weekly'&&!definition.weekdays.includes(weekday))return;
  const wall=localDate+'T'+definition.time,instant=resolveWall(wall,definition.timezone);
  if(instant!==undefined)return {localDate,localTime:definition.time,intendedAt:new Date(instant).toISOString(),decisionAt:new Date(instant).toISOString()};
  // Keep the missing wall time visible; it never becomes a runtime command.
  const nominal=Date.parse(wall+':00Z');
  for(let minutes=1;minutes<=1440;minutes++){
    const next=new Date(nominal+minutes*60_000).toISOString().slice(0,16),decision=resolveWall(next,definition.timezone);
    if(decision!==undefined)return {localDate,localTime:definition.time,intendedAt:null,decisionAt:new Date(decision).toISOString()};
  }
  reject('The timezone transition cannot be resolved.');
}
export function calendarBetween(definition:ScheduleDefinition,after:number,through:number):CalendarOccurrence[] {
  if(!Number.isFinite(after)||!Number.isFinite(through)||through<after||through-after>368*day)reject('Inspect at most one year of schedule occurrences at a time.');
  const start=Date.parse(local(after,definition.timezone).slice(0,10)+'T00:00:00Z')-day,end=Date.parse(local(through,definition.timezone).slice(0,10)+'T00:00:00Z')+day,results:CalendarOccurrence[]=[];
  for(let cursor=start;cursor<=end;cursor+=day){const occurrence=onDate(definition,new Date(cursor).toISOString().slice(0,10));if(occurrence&&Date.parse(occurrence.decisionAt)>after&&Date.parse(occurrence.decisionAt)<=through)results.push(occurrence);}
  return results.sort((a,b)=>Date.parse(a.decisionAt)-Date.parse(b.decisionAt));
}
export function previewCalendar(definition:ScheduleDefinition,after:number):CalendarOccurrence[] {
  if(definition.kind==='once'){
    const occurrence=onDate(definition,definition.date);return occurrence&&Date.parse(occurrence.decisionAt)>after?[occurrence]:[];
  }
  return calendarBetween(definition,after,after+35*day).slice(0,5);
}
