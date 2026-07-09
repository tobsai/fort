package ui

// boardHTML is the web control-plane client served at GET / . It renders a
// three-zone dashboard (Define · Ready · In progress) that replaces the
// kanban (spec 031): a multiline markdown compose (first line = title) with
// Run/Add-to-Ready/Break-down actions and a live preview, a Ready zone
// merging backlog items and queued runs behind Start buttons, and an
// In-progress zone for running/blocked runs with nested tool/subagent
// activity (spec 030) and inline gate approvals, plus a collapsed Recent
// strip for finished runs. It consumes /api/summary, /api/machines,
// /api/board and /api/backlog, streams /api/events (SSE), posts chat to
// /api/chat, dispatches backlog items via /api/backlog/{id}/dispatch, and
// posts gate decisions to /api/gate. It adapts to control-only mode
// (execution:false) by badging the plane and surfacing the 409 on gates.
const boardHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<title>Fort — Command Center</title>
<script>(function(){var s=localStorage.getItem('fort-theme');document.documentElement.setAttribute('data-theme',s||(matchMedia('(prefers-color-scheme: light)').matches?'light':'dark'));})();</script>
<style>
  :root{
    --bg:#0b0e14;--panel:#12161f;--card:#12161f;--line:#212938;--line2:#1a212e;
    --fg:#e6eaf2;--fg2:#b9c0ce;--mut:#5a6373;--brass:#c8a45c;--brass2:#e3c785;
    --run:#e0a93b;--block:#5b9bf0;--ok:#4fb477;--fail:#e0655b;
    --run-bg:#241c0e;--block-bg:#111c2e;
  }
  :root[data-theme=light]{
    --bg:#f4f5f7;--panel:#ffffff;--card:#ffffff;--line:#e2e5ea;--line2:#edeff2;
    --fg:#1b2230;--fg2:#3a4658;--mut:#8b94a7;--brass:#9a7b2e;--brass2:#7a5f16;
    --run:#b07d10;--block:#2b6fd0;--ok:#2f8f5a;--fail:#c23b34;
    --run-bg:#f7efdd;--block-bg:#e7f0fc;
  }
  *{box-sizing:border-box}
  body{margin:0;background:var(--bg);color:var(--fg);font:13px/1.5 ui-monospace,Menlo,Consolas,monospace;transition:background .15s,color .15s}
  header{display:flex;align-items:center;gap:12px;padding:12px 18px;border-bottom:1px solid var(--line2)}
  header h1{margin:0;font-size:14px;letter-spacing:.18em;color:var(--brass2);text-transform:uppercase}
  .plane{font-size:10px;letter-spacing:.08em;text-transform:uppercase;border:1px solid var(--line);border-radius:6px;padding:1px 7px;color:var(--brass)}
  .grow{flex:1}
  .counts{display:flex;gap:12px;font-size:12px;color:var(--mut)}
  .counts b{color:var(--fg2)}
  .iconbtn{background:transparent;border:1px solid var(--line);border-radius:7px;color:var(--fg2);padding:5px 9px;cursor:pointer;font:inherit}
  .iconbtn:hover{background:var(--line2)}
  .machines{display:flex;gap:14px;padding:8px 18px;font-size:11.5px;color:var(--mut);border-bottom:1px solid var(--line2);flex-wrap:wrap}
  .dot{display:inline-block;width:6px;height:6px;border-radius:50%;margin-right:5px;vertical-align:middle}
  .dot.up{background:var(--ok)}.dot.down{background:var(--fail)}
  .card{background:var(--card);border:1px solid var(--line);border-left:2px solid var(--edge,#3b4557);border-radius:8px;padding:9px 10px}
  .card .title{font-size:12.5px;line-height:1.4;margin-bottom:7px;color:var(--fg)}
  .card.done .title{color:var(--fg2)}
  .meta{display:flex;align-items:center;gap:7px}
  .meta .ag{font-size:10.5px;color:var(--brass2)}
  .meta .mc{font-size:10.5px;color:var(--mut)}
  .e-running{--edge:var(--run)} .e-blocked{--edge:var(--block)} .e-ok{--edge:var(--ok)} .e-fail{--edge:var(--fail)} .e-neutral{--edge:#3b4557}
  .card.item .src{width:6px;height:6px;border-radius:50%;background:var(--mut);display:inline-block}
  .card.item .src.agent{background:var(--brass)}
  .gateact{display:flex;gap:6px;margin-top:8px}
  .gateact button{font-size:10.5px;padding:1px 8px;border-radius:5px;background:transparent;border:1px solid var(--line);color:var(--fg2);cursor:pointer}
  .gateact button.ok{color:var(--ok);border-color:var(--ok)}
  .gateact button.no{color:var(--fail);border-color:var(--fail)}
  .runbtn{margin-top:7px;font-size:10.5px;padding:1px 9px;border-radius:5px;background:transparent;border:1px solid var(--brass);color:var(--brass2);cursor:pointer}
  .empty{color:var(--mut);font-size:11.5px;padding:8px 4px}
  a:focus-visible,button:focus-visible,select:focus-visible,input:focus-visible,textarea:focus-visible,.card.item:focus-visible{outline:2px solid var(--brass);outline-offset:1px}
  .dash{max-width:900px;margin:0 auto;padding:14px 18px;display:flex;flex-direction:column;gap:20px}
  .zonehead{display:flex;align-items:center;gap:8px;font-size:11px;letter-spacing:.05em;color:var(--mut);text-transform:uppercase;margin-bottom:9px}
  .zonehead .n{background:var(--line2);border-radius:20px;padding:0 6px;min-width:18px;text-align:center}
  .zone{display:flex;flex-direction:column;gap:9px}
  textarea#msg{width:100%;resize:vertical;min-height:64px;background:var(--panel);color:var(--fg);border:1px solid var(--line);border-radius:8px;padding:9px 11px;font:inherit;font-size:12.5px}
  .preview{border:1px dashed var(--line);border-radius:8px;padding:8px 11px;max-height:140px;overflow:auto}
  .define-actions{display:flex;gap:8px;margin-top:8px;align-items:center}
  .define-actions select{background:var(--panel);color:var(--fg);border:1px solid var(--line);border-radius:7px;padding:7px 9px;font:inherit;font-size:12px;cursor:pointer}
  .define-actions button{border-radius:7px;padding:7px 13px;font:inherit;font-size:12px;cursor:pointer;border:1px solid var(--line);background:var(--panel);color:var(--fg2)}
  .define-actions button.run{border-color:var(--brass);color:var(--brass2);background:transparent}
  .define-actions button:hover{background:var(--line2)}
  .startbtn{font-size:10.5px;padding:1px 9px;border-radius:5px;background:transparent;border:1px solid var(--brass);color:var(--brass2);cursor:pointer;margin-top:7px}
  .queuedtag{font-size:10px;color:var(--mut);border:1px solid var(--line);border-radius:5px;padding:0 6px;margin-top:7px;display:inline-block}
  .activity{margin-top:7px;display:flex;flex-direction:column;gap:2px;font-size:11px;color:var(--fg2)}
  .activity .a-tool{color:var(--mut)}
  .activity .a-sub{color:var(--block);padding-left:14px}
  .activity .a-msg{color:var(--fg2)}
  .recent summary{cursor:pointer;font-size:11px;letter-spacing:.05em;color:var(--mut);text-transform:uppercase;margin:4px 0 9px}
  .recent .n{background:var(--line2);border-radius:20px;padding:0 6px}
  .mdbody{margin:2px 0 7px;font-size:11.5px;line-height:1.5;color:var(--fg2);max-height:76px;overflow:hidden}
  .mdbody h3,.mdbody h4,.mdbody h5,.mdbody h6{font-size:12px;margin:4px 0;color:var(--fg)}
  .mdbody p{margin:3px 0}
  .mdbody ul,.mdbody ol{margin:3px 0;padding-left:16px}
  .mdbody code{background:var(--line2);border-radius:3px;padding:0 3px}
  .mdbody pre{background:var(--line2);border-radius:6px;padding:6px;overflow:auto;margin:4px 0}
  .mdbody a{color:var(--brass2)}
  .mdbody strong{color:var(--fg)}
  .run-card{cursor:pointer}
  .drawer[hidden]{display:none}
  .drawer{position:fixed;inset:0;z-index:50}
  .drawer-scrim{position:absolute;inset:0;background:rgba(0,0,0,.42)}
  .drawer-panel{position:absolute;top:0;right:0;height:100%;width:min(560px,92vw);background:var(--panel);border-left:1px solid var(--line);display:flex;flex-direction:column;box-shadow:-10px 0 30px rgba(0,0,0,.35)}
  .drawer-head{display:flex;justify-content:space-between;align-items:flex-start;gap:10px;padding:14px 16px;border-bottom:1px solid var(--line2)}
  .drawer-title{font-size:13px;color:var(--fg)}
  .drawer-sub{font-size:11px;color:var(--mut);margin-top:3px}
  #dw-body{max-height:220px;overflow:auto}
  .drawer-steps{padding:8px 10px;border-bottom:1px solid var(--line2);max-height:38%;overflow:auto;display:flex;flex-direction:column;gap:3px}
  .step{display:flex;align-items:center;gap:8px;padding:5px 8px;border-radius:6px;cursor:pointer;border:1px solid transparent}
  .step:hover{background:var(--line2)}
  .step.sel{background:var(--line2);border-color:var(--line)}
  .step .st{font-size:11px;width:14px;text-align:center;color:var(--mut)}
  .step .nm{font-size:12px;color:var(--fg)}
  .step .ty{font-size:10.5px;color:var(--mut);margin-left:auto}
  .step.s-succeeded .st{color:var(--ok)} .step.s-running .st{color:var(--run)} .step.s-failed .st{color:var(--fail)} .step.s-waiting .st{color:var(--block)}
  .drawer-log{flex:1;overflow:auto;padding:10px 14px;font-size:11.5px;line-height:1.55;white-space:pre-wrap;color:var(--fg2)}
  .drawer-log .ev{padding:1px 0}
  .drawer-log .ev .k{color:var(--mut)}
</style>
</head>
<body>
<header>
  <h1>Fort</h1>
  <span class="plane" id="plane">…</span>
  <span class="grow"></span>
  <span class="counts" id="counts"></span>
  <button class="iconbtn" id="theme" title="toggle theme" aria-label="toggle light/dark theme" onclick="toggleTheme()">◐</button>
  <span class="counts" id="clock"></span>
</header>
<div class="machines" id="machines" style="display:none"></div>
<div class="dash">
  <section>
    <div class="zonehead"><span>Define</span></div>
    <textarea id="msg" rows="3" placeholder="first line = title; the rest is the body (markdown)…"></textarea>
    <div id="preview" class="mdbody preview" hidden></div>
    <div class="define-actions">
      <select id="machine" title="target machine"><option value="">any machine</option></select>
      <select id="agent" title="agent"><option value="">auto agent</option></select>
      <span class="grow"></span>
      <button onclick="addToReady()">Add to Ready</button>
      <button onclick="breakdownTask()">Break down</button>
      <button class="run" onclick="runNow()">Run ▸</button>
    </div>
  </section>
  <section>
    <div class="zonehead"><span>Ready</span><span class="n" id="n-ready">0</span></div>
    <div class="zone" id="z-ready"></div>
  </section>
  <section>
    <div class="zonehead"><span>In progress</span><span class="n" id="n-progress">0</span></div>
    <div class="zone" id="z-progress"></div>
    <details class="recent"><summary>Recent <span class="n" id="n-recent">0</span></summary><div class="zone" id="z-recent"></div></details>
  </section>
</div>
<div id="drawer" class="drawer" hidden>
  <div class="drawer-scrim" onclick="closeDrawer()"></div>
  <aside class="drawer-panel" role="dialog" aria-label="run detail">
    <div class="drawer-head">
      <div><div class="drawer-title" id="dw-title">—</div><div class="drawer-sub" id="dw-sub"></div><div class="mdbody" id="dw-body"></div></div>
      <button class="iconbtn" onclick="closeDrawer()" aria-label="close">✕</button>
    </div>
    <div class="drawer-steps" id="dw-steps"></div>
    <div class="drawer-log" id="dw-log"></div>
  </aside>
</div>
<script>
const $=s=>document.querySelector(s);
/* md:start (spec 029) — escape-first safe-subset markdown. The ONLY sanctioned
   innerHTML source for user/agent-authored bodies: the WHOLE source is
   HTML-escaped first, then a fixed, closed set of tags is applied around the
   escaped text — a mis-parse can only fail to format, never inject. */
const esc=s=>(s||'').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;').replace(/'/g,'&#39;');
const BT=String.fromCharCode(96), FENCE=BT+BT+BT;
/* placeholder sentinels: U+E000/U+E001 (private use) — can't collide with
   prose (unlike space-delimited integers) and survive trim; if forged in
   input they restore to held content or empty, both inert. */
const S0=String.fromCharCode(57344), S1=String.fromCharCode(57345);
function md(src){
  if(!src||!String(src).trim())return '';
  let s=esc(String(src)).replace(/\r\n?/g,'\n');
  const hold=[];
  const stash=h=>{hold.push(h);return S0+(hold.length-1)+S1;};
  // fenced code first: hold escaped content out of all further formatting
  s=s.split(FENCE).map((seg,i)=>i%2?stash('<pre><code>'+seg.replace(/^[a-zA-Z0-9_-]*\n/,'')+'</code></pre>'):seg).join('');
  // inline code: same hold-out
  s=s.split(BT).map((seg,i)=>i%2?stash('<code>'+seg+'</code>'):seg).join('');
  // headings #..###### -> h3..h6 (capped: page hierarchy can't be hijacked)
  s=s.replace(/^(#{1,6})[ \t]+(.+)$/gm,(m,h,t)=>{const n=Math.min(h.length+2,6);return '<h'+n+'>'+t+'</h'+n+'>';});
  // lists: runs of "- "/"* " -> ul; "1. " -> ol
  s=s.replace(/((?:^[-*][ \t]+.+\n?)+)/gm,m=>'<ul>'+m.trim().split('\n').map(l=>'<li>'+l.replace(/^[-*][ \t]+/,'')+'</li>').join('')+'</ul>\n');
  s=s.replace(/((?:^\d+\.[ \t]+.+\n?)+)/gm,m=>'<ol>'+m.trim().split('\n').map(l=>'<li>'+l.replace(/^\d+\.[ \t]+/,'')+'</li>').join('')+'</ol>\n');
  // emphasis
  s=s.replace(/\*\*([^*]+)\*\*/g,'<strong>$1</strong>');
  s=s.replace(/\*([^*\n]+)\*/g,'<em>$1</em>');
  s=s.replace(/(^|[\s(])_([^_\n]+)_(?=$|[\s).,;:!?])/gm,'$1<em>$2</em>');
  // links: http(s) ONLY — anything else stays literal escaped text. The URL
  // class excludes S0 so held content can never be spliced into an href.
  s=s.replace(new RegExp('\\[([^\\]\\n]+)\\]\\((https?://[^\\s)'+S0+']+)\\)','g'),'<a href="$2" rel="noopener nofollow" target="_blank">$1</a>');
  // paragraphs: block tags and held blocks stand alone; single \n -> <br>
  s=s.split(/\n{2,}/).map(p=>{
    p=p.trim(); if(!p)return '';
    if(/^<(h\d|ul|ol|pre)/.test(p)||new RegExp('^'+S0+'\\d+'+S1+'$').test(p))return p;
    return '<p>'+p.replace(/\n/g,'<br>')+'</p>';
  }).join('');
  // restore held code (unknown indexes render empty, never 'undefined')
  return s.replace(new RegExp(S0+'(\\d+)'+S1,'g'),(m,i)=>hold[+i]!==undefined?hold[+i]:'');
}
/* md:end */
let hasExec=true, machines=[];

// ---- theme (init runs as a blocking <head> script to avoid FOUC) ----
function toggleTheme(){
  const cur=document.documentElement.getAttribute('data-theme')==='light'?'dark':'light';
  document.documentElement.setAttribute('data-theme',cur);
  localStorage.setItem('fort-theme',cur);
}

// ---- agent picker: union of agents, filtered by chosen machine ----
function agentsFor(machineName){
  let set=new Set();
  machines.forEach(m=>{ if(!machineName||m.name===machineName)(m.agents||[]).forEach(a=>set.add(a)); });
  return [...set].sort();
}
function syncAgentOptions(){
  const asel=$('#agent'), cur=asel.value, opts=agentsFor($('#machine').value);
  asel.innerHTML='<option value="">auto agent</option>'+opts.map(a=>'<option value="'+esc(a)+'">'+esc(a)+'</option>').join('');
  asel.value=opts.includes(cur)?cur:'';
}

// ---- rendering ----
function edgeFor(status){return status==='succeeded'?'e-ok':status==='failed'?'e-fail':status==='running'?'e-running':status==='blocked'?'e-blocked':'e-neutral';}
function runCard(r){
  const done=(r.status==='succeeded'||r.status==='failed'||r.status==='canceled');
  return '<div class="card run-card '+(done?'done ':'')+edgeFor(r.status)+'" tabindex="0" onclick="openDrawer(\''+r.id+'\')" onkeydown="if(event.key===\'Enter\')openDrawer(\''+r.id+'\')">'+
    '<div class="title">'+esc(r.title||r.id)+'</div>'+
    '<div class="meta"><span class="ag">'+esc(r.agent)+'</span>'+(r.machine?'<span class="mc">'+esc(r.machine)+'</span>':'')+'</div></div>';
}
// ---- live activity: per-run buffer fed by the SSE stream (spec 030) ----
const ACT_MAX=6;
let actByRun={};
function trackEvent(e){
  if(!e||!e.run_id)return;
  if(e.type!=='tool'&&e.type!=='subagent'&&e.type!=='message')return;
  const buf=actByRun[e.run_id]||(actByRun[e.run_id]=[]);
  buf.push(e); if(buf.length>ACT_MAX)buf.shift();
}
function activityLine(e){
  if(e.type==='tool'){
    let d={}; try{d=JSON.parse(e.data||'{}')}catch(err){}
    return '<div class="a-tool">🔧 '+esc(d.name||'tool')+(d.summary?' · '+esc(d.summary):'')+'</div>';
  }
  if(e.type==='subagent'){
    let d={}; try{d=JSON.parse(e.data||'{}')}catch(err){}
    return '<div class="a-sub">🤖 subagent'+(d.agent?' ('+esc(d.agent)+')':'')+(d.description?' · '+esc(d.description):'')+'</div>';
  }
  const t=(e.data||'').split('\n')[0];
  return t?'<div class="a-msg">💬 '+esc(t.length>120?t.slice(0,119)+'…':t)+'</div>':'';
}

// ---- zone renderers ----
function readyItem(b){
  return '<div class="card item e-neutral" data-id="'+b.id+'">'+
    '<div class="title">'+esc(b.title)+'</div>'+
    (b.body?'<div class="mdbody">'+md(b.body)+'</div>':'')+
    '<div class="meta"><span class="src '+(b.source==='agent'?'agent':'')+'"></span>'+
    (b.agent?'<span class="ag">'+esc(b.agent)+'</span>':'')+(b.machine?'<span class="mc">'+esc(b.machine)+'</span>':'')+'</div>'+
    '<button class="startbtn" onclick="dispatchItem(\''+b.id+'\')">Start ▸</button></div>';
}
function queuedItem(r){
  return '<div class="card run-card e-neutral" tabindex="0" onclick="openDrawer(\''+r.id+'\')" onkeydown="if(event.key===\'Enter\')openDrawer(\''+r.id+'\')">'+
    '<div class="title">'+esc(r.title||r.id)+'</div>'+
    '<div class="meta"><span class="ag">'+esc(r.agent)+'</span>'+(r.machine?'<span class="mc">'+esc(r.machine)+'</span>':'')+'</div>'+
    '<span class="queuedtag">queued</span></div>';
}
function progressItem(r,gates){
  const g=gates.filter(x=>x.run_id===r.id);
  const acts=(actByRun[r.id]||[]).map(activityLine).join('');
  return '<div class="card run-card '+edgeFor(r.status)+'" tabindex="0" onclick="openDrawer(\''+r.id+'\')" onkeydown="if(event.key===\'Enter\')openDrawer(\''+r.id+'\')">'+
    '<div class="title">'+esc(r.title||r.id)+'</div>'+
    (r.body?'<div class="mdbody">'+md(r.body)+'</div>':'')+
    '<div class="meta"><span class="ag">'+esc(r.agent)+'</span>'+(r.machine?'<span class="mc">'+esc(r.machine)+'</span>':'')+
    '<span class="mc">'+esc(r.status)+'</span></div>'+
    (acts?'<div class="activity">'+acts+'</div>':'')+
    g.map(x=>'<div class="gateact"><span class="mc">gate · '+esc(x.node_id)+'</span>'+
      '<button class="ok" onclick="event.stopPropagation();decide(\''+x.run_id+'\',\''+esc(x.node_id)+'\',\'approve\')">approve</button>'+
      '<button class="no" onclick="event.stopPropagation();decide(\''+x.run_id+'\',\''+esc(x.node_id)+'\',\'reject\')">reject</button></div>').join('')+
    '</div>';
}
function zone(id,html,nid,count){$('#'+id).innerHTML=html||'<div class="empty">—</div>';$('#'+nid).textContent=count;}
// ---- run drill-down drawer (spec 027) ----
let dwRun=null, dwNode=null, dwNodes=[], dwEvents=[];
function stepIcon(s){return s==='succeeded'?'✓':s==='failed'?'✕':s==='running'?'▸':s==='waiting'?'⏸':'▫';}
async function openDrawer(runID){ dwRun=runID; dwNode=null; $('#drawer').hidden=false; await loadDrawer(); }
function closeDrawer(){ dwRun=null; dwNode=null; $('#drawer').hidden=true; }
async function loadDrawer(){
  if(!dwRun) return;
  const id=dwRun;
  const d=await (await fetch('/api/runs/'+encodeURIComponent(id))).json();
  if(dwRun!==id) return; // a stale in-flight fetch: the drawer moved to another run
  dwNodes=d.nodes||[]; dwEvents=d.events||[];
  $('#dw-title').textContent=d.run.title||d.run.id;
  $('#dw-sub').textContent=[d.run.agent,d.run.status,d.run.machine].filter(Boolean).join(' · ');
  $('#dw-body').innerHTML=d.run.body?md(d.run.body):'';
  renderSteps(); renderLog();
}
function renderSteps(){
  const el=$('#dw-steps');
  if(!dwNodes.length){ el.style.display='none'; return; }
  el.style.display='flex';
  el.innerHTML=dwNodes.map(n=>
    '<div class="step s-'+esc(n.status)+(n.node_id===dwNode?' sel':'')+'" onclick="selectStep(\''+esc(n.node_id)+'\')">'+
    '<span class="st">'+stepIcon(n.status)+'</span><span class="nm">'+esc(n.node_id)+'</span>'+
    '<span class="ty">'+esc(n.type)+'</span></div>').join('');
}
function selectStep(nodeID){ dwNode=(dwNode===nodeID?null:nodeID); renderSteps(); renderLog(); }
function renderLog(){
  const log=$('#dw-log');
  const evs=dwEvents.filter(e=>!dwNode||e.node_id===dwNode);
  const atBottom=log.scrollHeight-log.scrollTop-log.clientHeight<24;
  const prev=log.scrollTop;
  if(!evs.length){ log.innerHTML='<div class="empty">waiting…</div>'; return; }
  log.innerHTML=evs.map(e=>{
    if(e.type==='tool'||e.type==='subagent')return '<div class="ev">'+activityLine(e)+'</div>';
    return '<div class="ev"><span class="k">'+esc(e.type)+'</span> '+esc(e.data||'')+'</div>';
  }).join('');
  log.scrollTop=atBottom?log.scrollHeight:prev;
}
document.addEventListener('keydown',e=>{if(e.key==='Escape')closeDrawer();});

async function refresh(){
  const sum=await (await fetch('/api/summary')).json();
  hasExec=sum.execution;
  $('#plane').textContent=hasExec?'full plane':'control only';
  $('#counts').innerHTML=['running','blocked','done'].map(k=>{
    const v=k==='done'?(sum.succeeded+sum.failed):(sum[k]||0);
    return '<span>'+k+' <b>'+v+'</b></span>';
  }).join('');
  machines=await (await fetch('/api/machines')).json()||[];
  const mbar=$('#machines');
  if(machines.length){
    mbar.style.display='flex';
    mbar.innerHTML=machines.map(m=>'<span><span class="dot '+(m.reachable?'up':'down')+'"></span>'+esc(m.name)+(m.local?' (local)':'')+' <b style="color:var(--fg2)">'+(m.agents||[]).map(esc).join(', ')+'</b></span>').join('');
    const sel=$('#machine'),cur=sel.value;
    sel.innerHTML='<option value="">any machine</option>'+machines.map(m=>'<option value="'+esc(m.name)+'">'+esc(m.name)+'</option>').join('');
    sel.value=cur;
  }else mbar.style.display='none';
  syncAgentOptions();

  const b=await (await fetch('/api/board')).json();
  const runs=b.runs||[], gates=b.gates||[];
  const items=await (await fetch('/api/backlog')).json()||[];
  const queued=runs.filter(r=>r.status==='queued');
  const live=runs.filter(r=>r.status==='running'||r.status==='blocked');
  const done=runs.filter(r=>r.status==='succeeded'||r.status==='failed'||r.status==='canceled');
  // prune activity buffers for runs that left In-progress (long-lived tabs)
  const liveIds=new Set(live.map(r=>r.id));
  Object.keys(actByRun).forEach(k=>{if(!liveIds.has(k))delete actByRun[k];});
  zone('z-ready',items.map(readyItem).join('')+queued.map(queuedItem).join(''),'n-ready',items.length+queued.length);
  zone('z-progress',live.map(r=>progressItem(r,gates)).join(''),'n-progress',live.length);
  zone('z-recent',done.map(runCard).join(''),'n-recent',done.length);
  if(dwRun) loadDrawer();
}

// ---- actions ----
function splitMsg(){
  const t=$('#msg').value; const i=t.indexOf('\n');
  return i<0?{title:t.trim(),body:''}:{title:t.slice(0,i).trim(),body:t.slice(i+1).trim()};
}
async function runNow(){
  const el=$('#msg'); if(!el.value.trim())return; const text=el.value; el.value=''; renderPreview();
  await fetch('/api/chat',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({text,machine:$('#machine').value,agent:$('#agent').value})});
  refresh();
}
async function addToReady(){
  const {title,body}=splitMsg(); if(!title)return; $('#msg').value=''; renderPreview();
  await fetch('/api/backlog',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({title,body,machine:$('#machine').value,agent:$('#agent').value})});
  refresh();
}
async function breakdownTask(){
  const el=$('#msg'); if(!el.value.trim())return; const text=el.value; el.value=''; renderPreview();
  const r=await fetch('/api/breakdown',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({text,machine:$('#machine').value,agent:$('#agent').value})});
  if(r.status===409)alert('Breakdown needs an execution plane — start fort serve.');
  refresh();
}
function renderPreview(){
  const {title,body}=splitMsg(); const pv=$('#preview');
  if(!body){pv.hidden=true;pv.innerHTML='';return;}
  pv.hidden=false; pv.innerHTML='<strong>'+esc(title)+'</strong>'+md(body);
}
$('#msg').addEventListener('input',renderPreview);
$('#msg').addEventListener('keydown',e=>{if((e.metaKey||e.ctrlKey)&&e.key==='Enter')runNow();});
async function dispatchItem(id){
  await fetch('/api/backlog/'+id+'/dispatch',{method:'POST'});
  refresh();
}
async function decide(run,node,decision){
  const r=await fetch('/api/gate',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({run_id:run,node_id:node,decision})});
  if(r.status===409)alert('No execution plane — start fort serve to act on gates.');
  refresh();
}

$('#machine').addEventListener('change',syncAgentOptions);

const es=new EventSource('/api/events?since=0');
es.onmessage=ev=>{try{trackEvent(JSON.parse(ev.data))}catch(err){} refresh();};
setInterval(()=>{$('#clock').textContent=new Date().toLocaleTimeString()},1000);
setInterval(refresh,3000);
refresh();
</script>
</body>
</html>`
