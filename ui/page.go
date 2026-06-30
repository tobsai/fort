package ui

// boardHTML is the live board + feed + gate-inbox served at GET / . It consumes
// GET /api/board and the GET /api/events SSE stream, and posts gate decisions
// to POST /api/gate. Brass-on-slate to match the Command Center theme.
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
  header{display:flex;align-items:baseline;gap:14px;padding:14px 20px;border-bottom:1px solid var(--line);background:var(--slate2)}
  header h1{margin:0;font-size:16px;letter-spacing:.18em;color:var(--brass2);text-transform:uppercase}
  header .sub{color:var(--mut);font-size:12px}
  .wrap{display:grid;grid-template-columns:1.4fr 1fr;gap:16px;padding:16px 20px}
  .panel{background:var(--slate2);border:1px solid var(--line);border-radius:8px;overflow:hidden}
  .panel h2{margin:0;padding:10px 14px;font-size:11px;letter-spacing:.16em;text-transform:uppercase;color:var(--brass);border-bottom:1px solid var(--line)}
  .rows{max-height:46vh;overflow:auto}
  .row{display:flex;gap:10px;align-items:center;padding:8px 14px;border-bottom:1px solid #1d2531}
  .row:last-child{border-bottom:0}
  .id{color:var(--mut);font-size:11px}
  .agent{color:var(--brass2)}
  .badge{font-size:10px;padding:2px 7px;border-radius:10px;border:1px solid var(--line);text-transform:uppercase;letter-spacing:.08em}
  .s-succeeded{color:var(--ok);border-color:#2c5238}
  .s-failed{color:var(--bad);border-color:#5a2f2a}
  .s-running,.s-blocked{color:var(--warn);border-color:#5a4a23}
  .feed{font-size:12px;max-height:40vh;overflow:auto;padding:6px 0}
  .ev{padding:2px 14px;white-space:pre-wrap;word-break:break-word}
  .ev .t{color:var(--mut)}
  .gate{display:flex;gap:8px;align-items:center;padding:8px 14px;border-bottom:1px solid #1d2531}
  button{background:transparent;border:1px solid var(--brass);color:var(--brass2);border-radius:6px;padding:3px 10px;cursor:pointer;font:inherit}
  button.rej{border-color:#5a2f2a;color:var(--bad)}
  button:hover{background:#1d2531}
  .empty{padding:14px;color:var(--mut)}
  .grow{flex:1}
</style>
</head>
<body>
<header><h1>Fort</h1><span class="sub">command center · live</span><span class="grow"></span><span class="sub" id="clock"></span></header>
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
  </section>
</div>
<script>
const $=s=>document.querySelector(s);
function badge(st){return '<span class="badge s-'+st+'">'+st+'</span>'}
async function refresh(){
  const b=await (await fetch('/api/board')).json();
  $('#runs').innerHTML=(b.runs&&b.runs.length)?b.runs.map(r=>
    '<div class="row">'+badge(r.status)+'<span class="agent">'+r.agent+'</span>'+
    '<span class="grow">'+(r.title||r.id)+'</span><span class="id">'+r.id.slice(0,8)+'</span></div>').join(''):'<div class="empty">no runs yet</div>';
  $('#gates').innerHTML=(b.gates&&b.gates.length)?b.gates.map(g=>
    '<div class="gate"><span class="grow">'+g.node_id+'<span class="id"> · '+g.run_id.slice(0,8)+'</span></span>'+
    '<button onclick="decide(\''+g.run_id+'\',\''+g.node_id+'\',\'approve\')">approve</button>'+
    '<button class="rej" onclick="decide(\''+g.run_id+'\',\''+g.node_id+'\',\'reject\')">reject</button></div>').join(''):'<div class="empty">no gates awaiting decision</div>';
}
async function decide(run,node,decision){
  await fetch('/api/gate',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({run_id:run,node_id:node,decision})});
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
refresh();
</script>
</body>
</html>`
