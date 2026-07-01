package ui

// boardHTML is the web control-plane client served at GET / . It consumes
// /api/summary, /api/board and the /api/events SSE stream, posts chat to
// /api/chat, and posts gate decisions to /api/gate. It adapts to control-only
// mode (execution:false) by badging the plane and surfacing the 409 on gates.
// Brass-on-slate to match the Command Center theme.
const boardHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<title>Fort — Command Center</title>
<style>
  :root{--slate:#10141b;--slate2:#171c26;--line:#26303f;--brass:#c8a45c;--brass2:#e3c785;--ink:#cdd6e3;--mut:#7f8da3;--ok:#5fb87a;--warn:#d8a657;--bad:#d2766a;}
  *{box-sizing:border-box}
  body{margin:0;background:var(--slate);color:var(--ink);font:14px/1.5 ui-monospace,Menlo,Consolas,monospace}
  header{display:flex;align-items:center;gap:14px;padding:14px 20px;border-bottom:1px solid var(--line);background:var(--slate2)}
  header h1{margin:0;font-size:16px;letter-spacing:.18em;color:var(--brass2);text-transform:uppercase}
  .plane{font-size:10px;letter-spacing:.1em;text-transform:uppercase;border:1px solid var(--line);border-radius:10px;padding:2px 8px;color:var(--mut)}
  .plane.full{color:var(--ok);border-color:#2c5238}
  .plane.control{color:var(--warn);border-color:#5a4a23}
  .grow{flex:1}
  .counts{display:flex;gap:10px;padding:8px 20px;border-bottom:1px solid var(--line);background:#0d1117;font-size:12px;flex-wrap:wrap}
  .count{color:var(--mut)}.count b{color:var(--brass2)}
  .wrap{display:grid;grid-template-columns:1.4fr 1fr;gap:16px;padding:16px 20px}
  .panel{background:var(--slate2);border:1px solid var(--line);border-radius:8px;overflow:hidden;display:flex;flex-direction:column}
  .panel h2{margin:0;padding:10px 14px;font-size:11px;letter-spacing:.16em;text-transform:uppercase;color:var(--brass);border-bottom:1px solid var(--line)}
  .rows{max-height:44vh;overflow:auto}
  .row{display:flex;gap:10px;align-items:center;padding:8px 14px;border-bottom:1px solid #1d2531}
  .row:last-child{border-bottom:0}
  .id{color:var(--mut);font-size:11px}
  .agent{color:var(--brass2)}
  .badge{font-size:10px;padding:2px 7px;border-radius:10px;border:1px solid var(--line);text-transform:uppercase;letter-spacing:.08em}
  .s-succeeded{color:var(--ok);border-color:#2c5238}
  .s-failed{color:var(--bad);border-color:#5a2f2a}
  .s-running,.s-blocked{color:var(--warn);border-color:#5a4a23}
  .s-queued{color:var(--mut)}
  .mtag{font-size:10px;color:var(--mut);border:1px solid var(--line);border-radius:10px;padding:1px 6px}
  .dot{display:inline-block;width:7px;height:7px;border-radius:50%;margin-right:5px;vertical-align:middle}
  .dot.up{background:var(--ok)}.dot.down{background:var(--bad)}
  .chat select{background:#0d1117;border:1px solid var(--line);border-radius:6px;color:var(--ink);padding:7px 8px;font:inherit}
  .feed{font-size:12px;max-height:26vh;overflow:auto;padding:6px 0}
  .ev{padding:2px 14px;white-space:pre-wrap;word-break:break-word}
  .ev .t{color:var(--mut)}
  .gate{display:flex;gap:8px;align-items:center;padding:8px 14px;border-bottom:1px solid #1d2531}
  button{background:transparent;border:1px solid var(--brass);color:var(--brass2);border-radius:6px;padding:4px 10px;cursor:pointer;font:inherit}
  button.rej{border-color:#5a2f2a;color:var(--bad)}
  button:hover{background:#1d2531}
  .empty{padding:14px;color:var(--mut)}
  .chat{display:flex;gap:8px;padding:10px 14px;border-top:1px solid var(--line)}
  .chat input{flex:1;background:#0d1117;border:1px solid var(--line);border-radius:6px;color:var(--ink);padding:7px 10px;font:inherit}
</style>
</head>
<body>
<header>
  <h1>Fort</h1>
  <span class="plane" id="plane">…</span>
  <span class="grow"></span>
  <span class="id" id="clock"></span>
</header>
<div class="counts" id="counts"></div>
<div class="counts" id="machines" style="display:none"></div>
<div class="wrap">
  <section class="panel">
    <h2>Runs</h2>
    <div class="rows" id="runs"><div class="empty">loading…</div></div>
  </section>
  <section class="panel">
    <h2>Gate inbox</h2>
    <div id="gates"><div class="empty">no gates awaiting decision</div></div>
    <h2 style="border-top:1px solid var(--line)">Live feed</h2>
    <div class="feed" id="feed"></div>
    <div class="chat">
      <select id="machine" title="target machine"><option value="">any machine</option></select>
      <input id="msg" placeholder="chat… (try: ship dark mode)" onkeydown="if(event.key==='Enter')send()"/>
      <button onclick="send()">send</button>
    </div>
  </section>
</div>
<script>
const $=s=>document.querySelector(s);
let hasExec=true;
function badge(st){return '<span class="badge s-'+st+'">'+st+'</span>'}
async function refresh(){
  const sum=await (await fetch('/api/summary')).json();
  hasExec=sum.execution;
  $('#plane').textContent=hasExec?'full plane':'control only';
  $('#plane').className='plane '+(hasExec?'full':'control');
  $('#counts').innerHTML=['running','queued','blocked','succeeded','failed']
    .map(k=>'<span class="count">'+k+' <b>'+(sum[k]||0)+'</b></span>').join('')+
    '<span class="count">total <b>'+sum.total+'</b></span>';
  const ms=await (await fetch('/api/machines')).json();
  const mbar=$('#machines');
  if(ms&&ms.length){
    mbar.style.display='flex';
    mbar.innerHTML='<span class="count">machines</span>'+ms.map(m=>
      '<span class="count"><span class="dot '+(m.reachable?'up':'down')+'"></span>'+m.name+(m.local?' (local)':'')+
      ' <b>'+(m.agents||[]).join(',')+'</b></span>').join('');
    const sel=$('#machine');const cur=sel.value;
    sel.innerHTML='<option value="">any machine</option>'+ms.map(m=>'<option value="'+m.name+'">'+m.name+'</option>').join('');
    sel.value=cur;
  }else{mbar.style.display='none';}
  const b=await (await fetch('/api/board')).json();
  $('#runs').innerHTML=(b.runs&&b.runs.length)?b.runs.map(r=>
    '<div class="row">'+badge(r.status)+'<span class="agent">'+r.agent+'</span>'+
    (r.machine?'<span class="mtag">'+r.machine+'</span>':'')+
    '<span class="grow">'+(r.title||r.id)+'</span><span class="id">'+r.id.slice(0,8)+'</span></div>').join(''):'<div class="empty">no runs yet</div>';
  $('#gates').innerHTML=(b.gates&&b.gates.length)?b.gates.map(g=>
    '<div class="gate"><span class="grow">'+g.node_id+'<span class="id"> · '+g.run_id.slice(0,8)+'</span></span>'+
    '<button onclick="decide(\''+g.run_id+'\',\''+g.node_id+'\',\'approve\')">approve</button>'+
    '<button class="rej" onclick="decide(\''+g.run_id+'\',\''+g.node_id+'\',\'reject\')">reject</button></div>').join(''):'<div class="empty">no gates awaiting decision</div>';
}
async function decide(run,node,decision){
  const r=await fetch('/api/gate',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({run_id:run,node_id:node,decision})});
  if(r.status===409)alert('No execution plane — start fort serve to act on gates.');
  refresh();
}
async function send(){
  const el=$('#msg');const text=el.value.trim();if(!text)return;el.value='';
  const machine=$('#machine')?$('#machine').value:'';
  await fetch('/api/chat',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({text,machine})});
  refresh();
}
const es=new EventSource('/api/events?since=0');
es.onmessage=e=>{
  const d=JSON.parse(e.data);
  const el=document.createElement('div');el.className='ev';
  el.innerHTML='<span class="t">'+d.type+'</span> '+(d.data||'').replace(/</g,'&lt;');
  const f=$('#feed');f.prepend(el);while(f.childElementCount>200)f.lastChild.remove();
  refresh();
};
setInterval(()=>{$('#clock').textContent=new Date().toLocaleTimeString()},1000);
setInterval(refresh,3000);
refresh();
</script>
</body>
</html>`
