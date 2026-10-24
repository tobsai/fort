import { LOCAL_CONTEXT } from '../modules/local-context.ts';
import { CODEX_RUNTIME } from '../runner/codex-descriptor.ts';
import { test, type TestContext } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createApplication } from './openai.ts';
import { weekProgress } from './week.ts';
import type { Agent, Installation, WeekView } from '../contracts/v1.ts';

async function setup(t:TestContext) {
  const directory=await mkdtemp(join(tmpdir(),'fort-week-unit-'));
  let service=createApplication({context:LOCAL_CONTEXT,provider:CODEX_RUNTIME,dataDirectory:directory});
  await service.start();
  t.after(async()=>{await service.close();await rm(directory,{recursive:true,force:true});});
  const enrolled=await fetch(service.baseURL+'/v1/devices/enroll',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({...service.issueEnrollment(),deviceName:'Synthetic test device'})});
  assert.equal(enrolled.status,201);const {deviceToken}=await enrolled.json() as {deviceToken:string};
  const headers=()=>({'Content-Type':'application/json',Authorization:`Bearer ${deviceToken}`,'X-Fort-Installation-Id':service.installation.installationId});
  async function api<T>(path:string,body?:unknown){
    const response=await fetch(service.baseURL+path,{method:body===undefined?'GET':'POST',headers:headers(),body:body===undefined?undefined:JSON.stringify(body)});
    return {status:response.status,body:await response.json() as T};
  }
  const pascal=(await api<Agent>('/v1/agents',{name:'Pascal',model:'gpt-5.5',instructions:'Code.'})).body;
  return {api,pascal,get service(){return service;},async restart(){await service.close();service=createApplication({context:LOCAL_CONTEXT,provider:CODEX_RUNTIME,dataDirectory:directory});await service.start();}};
}
const found=(agentId:string)=>({name:'Glass Bay',startsOn:'2026-10-26',mainGoalKey:'beta',
  goals:[{key:'beta',text:'Fort beta on TestFlight',zone:'code',lead:{kind:'agent',agentId},targetPoints:9},{key:'life',text:'Your own list',zone:'personal',lead:{kind:'you'},targetPoints:3}],
  plan:[{goalKey:'beta',task:'Claude runner adapter PR',owner:{kind:'agent',agentId},size:'significant'},{goalKey:'beta',task:'Create the App Store Connect API key',owner:{kind:'you'},size:'moderate',unlocks:'Ship to TestFlight'},{goalKey:'life',task:'Book the dentist',owner:{kind:'you'},size:'small'}]});

test('founding a week stores goals and plan, advertises the capability and survives restart',async t=>{
  const h=await setup(t);
  assert.ok((await h.api<Installation>('/v1/installation')).body.capabilities?.includes('weeks'));
  const before=(await h.api<Installation>('/v1/installation')).body.eventCursor;
  const created=await h.api<WeekView>('/v1/weeks',found(h.pascal.agentId));
  assert.equal(created.status,201);
  const week=created.body;
  assert.equal(week.number,1);assert.equal(week.state,'live');assert.equal(week.endsOn,'2026-11-01');
  assert.equal(week.goals.length,2);assert.equal(week.mainGoalId,week.goals[0].goalId);
  assert.deepEqual(week.plan.map(item=>[item.task,item.size,item.doneBuildId]),[['Claude runner adapter PR','significant',null],['Create the App Store Connect API key','moderate',null],['Book the dentist','small',null]]);
  assert.equal(week.plan[1].unlocks,'Ship to TestFlight');
  const events=(await h.api<{events:{kind:string;entityId:string}[]}>(`/v1/events?after=${before}&recoveryEpoch=${h.service.installation.recoveryEpoch}`)).body.events;
  assert.ok(events.some(event=>event.kind==='workspaceChanged'&&event.entityId===week.weekId),'other devices learn about the new week');
  assert.deepEqual(week.progress,{points:0,goals:[{goalId:week.goals[0].goalId,points:0,targetPoints:9,met:false},{goalId:week.goals[1].goalId,points:0,targetPoints:3,met:false}],goalsMet:0,goalsCounted:1,grade:'D'});
  await h.restart();
  assert.deepEqual((await h.api<{weeks:WeekView[]}>('/v1/weeks')).body.weeks,[week]);
});

test('only one week is live; closing it lets the next one found with the next number',async t=>{
  const h=await setup(t);
  const first=(await h.api<WeekView>('/v1/weeks',found(h.pascal.agentId))).body;
  assert.equal((await h.api<{code:string}>('/v1/weeks',found(h.pascal.agentId))).status,409);
  assert.equal((await h.api(`/v1/weeks/${first.weekId}/close`,{expectedRevisionId:'stale'})).status,409);
  const closed=await h.api<WeekView>(`/v1/weeks/${first.weekId}/close`,{expectedRevisionId:first.revisionId});
  assert.equal(closed.status,200);assert.equal(closed.body.state,'closed');
  assert.equal((await h.api(`/v1/weeks/${first.weekId}/plan`,{expectedRevisionId:closed.body.revisionId,goalId:first.goals[0].goalId,task:'Late',owner:{kind:'you'},size:'small'})).status,409);
  const second=await h.api<WeekView>('/v1/weeks',{...found(h.pascal.agentId),name:'Harbor Point',startsOn:'2026-11-02'});
  assert.equal(second.status,201);assert.equal(second.body.number,2);
  assert.deepEqual((await h.api<{weeks:WeekView[]}>('/v1/weeks')).body.weeks.map(week=>week.name),['Harbor Point','Glass Bay']);
});

test('placing builds marks plan items done, credits goals and grades only crew goals',async t=>{
  const h=await setup(t);
  let week=(await h.api<WeekView>('/v1/weeks',found(h.pascal.agentId))).body;
  const [runner,key,dentist]=week.plan;
  const placed=await h.api<WeekView>(`/v1/weeks/${week.weekId}/builds`,{expectedRevisionId:week.revisionId,itemId:runner.itemId,size:'significant',evidence:'PR #27'});
  assert.equal(placed.status,200);week=placed.body;
  assert.equal(week.builds.length,1);assert.deepEqual(week.builds[0].by,{kind:'agent',agentId:h.pascal.agentId});
  assert.equal(week.plan[0].doneBuildId,week.builds[0].buildId);
  assert.equal((await h.api(`/v1/weeks/${week.weekId}/builds`,{expectedRevisionId:week.revisionId,itemId:runner.itemId,size:'small'})).status,409);
  week=(await h.api<WeekView>(`/v1/weeks/${week.weekId}/builds`,{expectedRevisionId:week.revisionId,itemId:key.itemId,size:'moderate'})).body;
  week=(await h.api<WeekView>(`/v1/weeks/${week.weekId}/builds`,{expectedRevisionId:week.revisionId,itemId:dentist.itemId,size:'small'})).body;
  const adhoc=await h.api<WeekView>(`/v1/weeks/${week.weekId}/builds`,{expectedRevisionId:week.revisionId,goalId:week.goals[1].goalId,task:'Cleaned out the garage',by:{kind:'you'},size:'moderate'});
  assert.equal(adhoc.status,200);week=adhoc.body;
  assert.equal(week.builds.at(-1)?.itemId,null);
  assert.deepEqual(week.progress,{points:13,goals:[{goalId:week.goals[0].goalId,points:9,targetPoints:9,met:true},{goalId:week.goals[1].goalId,points:4,targetPoints:3,met:true}],goalsMet:1,goalsCounted:1,grade:'A'});
});

test('plan items can be added mid-week and invalid weeks are refused without writes',async t=>{
  const h=await setup(t);
  const base=found(h.pascal.agentId);
  for(const bad of [{...base,name:''},{...base,startsOn:'2026-02-31'},{...base,goals:[]},{...base,extra:true},{...base,mainGoalKey:'missing'},
    {...base,goals:[{...base.goals[0],zone:'military'}]},{...base,goals:[{...base.goals[0],targetPoints:0}]},
    {...base,goals:[{...base.goals[0],lead:{kind:'agent',agentId:'not-an-agent'}}]},
    {...base,plan:[{...base.plan[0],size:'huge'}]},{...base,plan:[{...base.plan[0],goalKey:'nope'}]},{...base,plan:[{...base.plan[0],task:'x'.repeat(201)}]}])
    assert.equal((await h.api('/v1/weeks',bad)).status,400,JSON.stringify(bad).slice(0,120));
  assert.deepEqual((await h.api<{weeks:WeekView[]}>('/v1/weeks')).body.weeks,[]);
  const week=(await h.api<WeekView>('/v1/weeks',base)).body;
  const added=await h.api<WeekView>(`/v1/weeks/${week.weekId}/plan`,{expectedRevisionId:week.revisionId,goalId:week.goals[0].goalId,task:'Write the release notes',owner:{kind:'agent',agentId:h.pascal.agentId},size:'small'});
  assert.equal(added.status,200);assert.equal(added.body.plan.length,4);assert.notEqual(added.body.revisionId,week.revisionId);
  assert.equal((await h.api(`/v1/weeks/${week.weekId}/plan`,{expectedRevisionId:week.revisionId,goalId:week.goals[0].goalId,task:'Stale',owner:{kind:'you'},size:'small'})).status,409);
  assert.equal((await h.api(`/v1/weeks/${week.weekId}/builds`,{expectedRevisionId:added.body.revisionId,goalId:'nope',task:'x',by:{kind:'you'},size:'small'})).status,400);
  assert.equal((await h.api('/v1/weeks/missing/close',{expectedRevisionId:'x'})).status,404);
});

test('week progress grades by the share of crew goals met',()=>{
  const goal=(goalId:string,zone:'code'|'personal',targetPoints:number)=>({goalId,text:goalId,zone,lead:{kind:'you' as const},targetPoints});
  const build=(goalId:string,size:'small'|'milestone')=>({buildId:goalId+size,goalId,itemId:null,task:'t',size,by:{kind:'you' as const},placedAt:'2026-10-26T00:00:00.000Z'});
  const week=(builds:ReturnType<typeof build>[])=>({goals:[goal('a','code',12),goal('b','code',12),goal('c','code',12),goal('p','personal',1)],builds});
  assert.equal(weekProgress(week([])).grade,'D');
  assert.equal(weekProgress(week([build('a','milestone')])).grade,'C');
  assert.equal(weekProgress(week([build('a','milestone'),build('b','milestone')])).grade,'B');
  assert.equal(weekProgress(week([build('a','milestone'),build('b','milestone'),build('c','milestone'),build('p','small')])).grade,'A');
  assert.equal(weekProgress({goals:[goal('p','personal',1)],builds:[]}).grade,null);
});
