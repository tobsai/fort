package ui

// boardHTML is the web control plane served at GET / — the delegation-model
// dashboard (spec 033, from design_handoff_fort_dashboard_redesign/): six views
// behind one top-bar nav. Deck (needs-you inbox + projects + crew), Projects
// (sigil cards with human-checkpoint bars), Assign (give direction + roster),
// Performance (/api/metrics scorecards), Week and Today (per-agent schedule
// grids; Today derives a "You" row of sign-off moments). Vocabulary is human
// (assignment / sign-off / Up next / Start / Draft a plan / checkpoint) over
// the unchanged API: /api/summary /api/machines /api/board /api/backlog
// (+PATCH reassign), /api/chat, /api/breakdown, /api/gate (approve or reject
// with a note), /api/runs/{id}, /api/metrics, /api/events (SSE). Progress is
// only ever human-accepted checkpoints; sigils are deterministic identicons
// whose ring carries status; amber never animates. Control-only mode surfaces
// 409s. The markdown renderer between the md:start/md:end markers is tested
// under goja (ui/md_test.go) and must stay self-contained; the whole const is
// a Go raw string, so no literal backticks anywhere.
const boardHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<title>Fort — Command Deck</title>
<script>(function(){var s=localStorage.getItem('fort-theme');document.documentElement.setAttribute('data-theme',s||(matchMedia('(prefers-color-scheme: light)').matches?'light':'dark'));})();</script>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Instrument+Sans:wght@400;500;600;700&family=Spline+Sans+Mono:wght@400;500;600&display=swap" rel="stylesheet">
<style>
  :root{
    --bg:#0b0e14;--panel:#12161f;--line:#1a212e;--line2:#212938;--raise:#26314a;--outline:#303848;
    --fg:#e8ebf2;--body:#b8bfce;--mut:#8b93a5;--faint:#687183;--dis:#4a5262;
    --brass:#c9a35c;--brass2:#dcb877;--work:#6fa8ff;--need:#e0a458;--ok:#57b98a;--bad:#d96a6a;
    --queued:#2a3650;--sched:#56617a;--seg0:#212938;--sheen:#e4efff;--slip:#3a3020;
    --on-brass:#0b0e14;--on-amber:#12100a;--on-blue:#07101f;--on-green:#07120c;
    --tint-need:rgba(224,164,88,.14);--tint-work:rgba(111,168,255,.13);--tint-ok:rgba(87,185,138,.13);
    --tint-bad:rgba(217,106,106,.13);--tint-brass:rgba(201,163,92,.12);
    --font:'Instrument Sans',system-ui,sans-serif;--mono:'Spline Sans Mono',ui-monospace,Menlo,monospace;
  }
  :root[data-theme=light]{
    --bg:#f4f5f7;--panel:#ffffff;--line:#e4e7ec;--line2:#d8dce3;--raise:#c6cfdd;--outline:#aeb8c8;
    --fg:#1b2230;--body:#3a4658;--mut:#68738a;--faint:#8b94a7;--dis:#aab2c0;
    --brass:#9a7b2e;--brass2:#7a5f16;--work:#2b6fd0;--need:#b07d10;--ok:#2f8f5a;--bad:#c23b34;
    --queued:#dde5f2;--sched:#98a2b8;--seg0:#e4e7ec;--sheen:#f3f7ff;--slip:#e2d2b0;
    --on-brass:#ffffff;--on-amber:#ffffff;--on-blue:#ffffff;--on-green:#ffffff;
    --tint-need:rgba(176,125,16,.13);--tint-work:rgba(43,111,208,.11);--tint-ok:rgba(47,143,90,.12);
    --tint-bad:rgba(194,59,52,.11);--tint-brass:rgba(154,123,46,.12);
  }
  *{box-sizing:border-box}
  body{margin:0;background:var(--bg);color:var(--fg);font:14px/1.5 var(--font);transition:background .15s,color .15s}
  button{font:inherit}
  .mono{font-family:var(--mono)}
  @keyframes spinrace{to{transform:rotate(360deg)}}
  @keyframes sheen{to{background-position:-220% 0}}
  @keyframes dotpulse{0%,100%{opacity:1}50%{opacity:.35}}
  @media (prefers-reduced-motion: reduce){*{animation:none!important}}
  a{color:var(--brass);text-decoration:none}a:hover{color:var(--brass2)}
  a:focus-visible,button:focus-visible,select:focus-visible,input:focus-visible,textarea:focus-visible,[tabindex]:focus-visible{outline:2px solid var(--brass);outline-offset:1px}

  /* ---- top bar ---- */
  header{display:flex;align-items:center;gap:14px;padding:14px 22px;border-bottom:1px solid var(--line)}
  .wordmark{font:700 15px var(--mono);letter-spacing:.22em;color:var(--brass2)}
  nav{display:flex;gap:2px}
  nav button{font-size:13px;color:var(--mut);background:none;border:none;padding:5px 10px;border-radius:7px;cursor:pointer}
  nav button:hover{color:var(--fg)}
  nav button.on{color:var(--brass2);background:var(--tint-brass)}
  .needpill{font-size:12px;padding:3px 10px;border-radius:20px;background:var(--tint-need);color:var(--need);font-weight:600}
  .grow{flex:1}
  .mdot{display:flex;align-items:center;gap:6px;font:12px var(--mono);color:var(--mut)}
  .mdot i{width:7px;height:7px;border-radius:50%;background:var(--ok);display:inline-block}
  .mdot.down i{background:var(--bad)}
  .plane{font:600 10px var(--mono);letter-spacing:.08em;text-transform:uppercase;border:1px solid var(--outline);border-radius:6px;padding:2px 7px;color:var(--mut)}
  .iconbtn{background:transparent;border:1px solid var(--outline);border-radius:7px;color:var(--body);padding:5px 9px;cursor:pointer}
  .iconbtn:hover{border-color:var(--raise)}

  /* ---- buttons / pills / labels ---- */
  .btn{font-weight:600;font-size:13px;padding:7px 14px;border-radius:8px;border:none;cursor:pointer}
  .btn-brass{background:var(--brass);color:var(--on-brass);padding:7px 16px}
  .btn-ok{background:var(--ok);color:var(--on-green)}
  .btn-amber{background:var(--need);color:var(--on-amber);font-size:13.5px;padding:9px 16px}
  .btn-outline{background:transparent;border:1px solid var(--outline);color:var(--fg)}
  .btn-neutral{background:transparent;border:1px solid var(--outline);color:var(--body);font-weight:500;font-size:13.5px;padding:9px 16px}
  .btn-brassline{background:transparent;border:1px solid var(--brass);color:var(--brass2)}
  .btn-ghost{background:transparent;border:none;font-weight:500;color:var(--mut)}
  .btn:hover{filter:brightness(1.08)}
  .pill{font-size:12px;padding:3px 10px;border-radius:20px;font-weight:600;white-space:nowrap}
  .pill-need{background:var(--tint-need);color:var(--need)}
  .pill-work{background:var(--tint-work);color:var(--work)}
  .pill-ok{background:var(--tint-ok);color:var(--ok)}
  .pill-fail{background:var(--tint-bad);color:var(--bad)}
  .pill-idle{background:var(--line);color:var(--faint);font-weight:400}
  .ulabel{font-size:12px;font-weight:600;letter-spacing:.09em;text-transform:uppercase;color:var(--mut)}
  .ulabel.amber{color:var(--need)}
  .subhead{display:flex;align-items:center;gap:16px;padding:14px 22px;border-bottom:1px solid var(--line)}
  .subhead .t{font-size:14px;font-weight:600}
  .subhead .c{font-size:12.5px;color:var(--faint)}
  .subhead .cm{font:12px var(--mono);color:var(--faint)}
  .view[hidden]{display:none}

  /* ---- sigils ---- */
  .ring{display:inline-block;border:2px solid var(--outline);line-height:0;flex:none}
  .ring-sq30{border-radius:8px;padding:4px}
  .ring-sq42{border-radius:10px;padding:5px}
  .ring-work{position:relative;border-radius:50%;padding:8px;border-color:var(--work)}
  .race{position:absolute;inset:-2px;border-radius:50%;background:conic-gradient(rgba(255,255,255,0) 0 70%,#ffffff 82%,rgba(255,255,255,0) 94%);-webkit-mask:radial-gradient(farthest-side,transparent calc(100% - 4px),#000 calc(100% - 3px));mask:radial-gradient(farthest-side,transparent calc(100% - 4px),#000 calc(100% - 3px));animation:spinrace 2.2s linear infinite}

  /* ---- deck ---- */
  .deck{display:flex}
  .deck-left{flex:1.5;padding:22px;display:flex;flex-direction:column;gap:14px;border-right:1px solid var(--line);min-width:0}
  .deck-right{flex:1;padding:22px;display:flex;flex-direction:column;gap:12px;min-width:0}
  .needcard{background:var(--panel);border:1px solid var(--raise);border-left:3px solid var(--need);border-radius:10px;padding:16px 18px;display:flex;flex-direction:column;gap:10px}
  .needcard.fail{border-left-color:var(--bad)}
  .needcard .hd{display:flex;align-items:center;gap:10px}
  .needcard .hd .t{font-size:16px;font-weight:600}
  .needcard .hd .ago{font:11.5px var(--mono);color:var(--faint);margin-left:auto;white-space:nowrap}
  .needcard .bd{font-size:13.5px;color:var(--body);line-height:1.55}
  .needcard .bd .proj{color:var(--brass2);font-weight:600}
  .needcard .acts{display:flex;gap:8px;flex-wrap:wrap}
  .notebox{display:none;flex-direction:column;gap:8px}
  .notebox.open{display:flex}
  .notebox textarea{background:var(--bg);color:var(--fg);border:1px solid var(--raise);border-radius:8px;padding:9px 11px;font:13.5px var(--font);min-height:56px;resize:vertical}
  .alldone{font-size:12.5px;color:var(--faint);padding:4px 2px}
  .projrow{display:flex;align-items:center;gap:12px;padding:11px 12px;background:var(--panel);border:1px solid var(--line);border-radius:10px;cursor:pointer}
  .projrow:hover{border-color:var(--raise)}
  .projrow .nm{font-size:14px;font-weight:600}
  .projrow .cap{font-size:12px;color:var(--mut)}
  .projrow .cap.dim{color:var(--faint)}
  .projrow .col{display:flex;flex-direction:column;gap:2px;min-width:0}
  .crewrow{display:flex;align-items:center;gap:9px;font-size:13px}
  .crewrow .dot{width:8px;height:8px;border-radius:50%;background:var(--outline);flex:none}
  .crewrow .dot.work{background:var(--work);animation:dotpulse 1.6s infinite}
  .crewrow .dot.need{background:var(--need)}
  .crewrow .act{color:var(--mut);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .crewrow .act.dim{color:var(--faint)}

  /* ---- projects ---- */
  .projgrid{display:grid;grid-template-columns:1fr 1fr;gap:16px;padding:22px}
  .pcard{background:var(--panel);border:1px solid var(--line);border-radius:12px;padding:20px;display:flex;flex-direction:column;gap:14px;cursor:pointer}
  .pcard:hover{border-color:var(--raise)}
  .pcard.need{border-color:var(--raise)}
  .pcard.brief{border:1px dashed var(--outline)}
  .pcard .hd{display:flex;align-items:center;gap:14px}
  .pcard .nm{font-size:17px;font-weight:600}
  .pcard .sub{font-size:12.5px;color:var(--mut)}
  .pcard .sub.dim{color:var(--faint)}
  .pcard .hd .pill{margin-left:auto}
  .pcard .col{display:flex;flex-direction:column;gap:2px;min-width:0}
  .bar{display:flex;gap:4px}
  .bar i{flex:1;height:8px;border-radius:4px;background:var(--seg0)}
  .bar i.ok{background:var(--ok)} .bar i.need{background:var(--need)} .bar i.work{background:var(--work)} .bar i.bad{background:var(--bad)}
  .barcap{font-size:12.5px;color:var(--mut)}
  .barcap .need{color:var(--need)} .barcap .work{color:var(--work)}
  .pcard .act{font-size:13.5px;color:var(--body);line-height:1.5}
  .pcard .act.dim{color:var(--faint)}
  .pcard .cta{align-self:flex-start}

  /* ---- assign ---- */
  .assign{display:flex}
  .assign-left{flex:1;padding:24px;border-right:1px solid var(--line);display:flex;flex-direction:column;gap:16px;min-width:0}
  .assign-right{flex:1.2;padding:24px;display:flex;flex-direction:column;gap:12px;min-width:0}
  .assign-left h2{margin:0;font-size:18px;font-weight:600}
  #brief{background:var(--panel);border:1px solid var(--raise);border-radius:10px;padding:14px 16px;min-height:110px;font:14.5px/1.6 var(--font);color:var(--fg);resize:vertical;width:100%}
  #brief::placeholder{color:var(--faint)}
  .chips{display:flex;gap:8px;flex-wrap:wrap}
  .chip{font-size:13px;padding:7px 14px;border-radius:20px;border:1px solid var(--outline);color:var(--body);background:transparent;cursor:pointer}
  .chip.on{font-weight:600;border:1.5px solid var(--brass);background:var(--tint-brass);color:var(--brass2)}
  .chips.machines .chip{font-family:var(--mono);font-size:12px;padding:5px 11px}
  .toggle{display:flex;align-items:center;gap:10px;font-size:13.5px;color:var(--body);cursor:pointer;user-select:none}
  .track{width:34px;height:20px;border-radius:10px;background:var(--brass);position:relative;display:inline-block;flex:none;transition:background .15s}
  .track i{position:absolute;right:2px;top:2px;width:16px;height:16px;border-radius:50%;background:var(--bg);transition:right .15s}
  .toggle.off .track{background:var(--outline)}
  .toggle.off .track i{right:16px}
  .handoff{font-size:14.5px;padding:11px 22px;border-radius:9px;align-self:flex-start}
  .orlink{font-size:12.5px;color:var(--mut);background:none;border:none;cursor:pointer;align-self:flex-start;padding:0}
  .orlink:hover{color:var(--fg)}
  .roster{background:var(--panel);border:1px solid var(--line);border-radius:10px;padding:14px 16px;display:flex;flex-direction:column;gap:8px}
  .roster.need{border-color:var(--raise);border-left:3px solid var(--need)}
  .roster.work{border-left:3px solid var(--work)}
  .roster .hd{display:flex;align-items:center;gap:10px}
  .roster .nm{font-size:15px;font-weight:600}
  .roster.idle .nm{color:var(--mut)}
  .roster .mc{font:11.5px var(--mono);color:var(--faint);margin-left:auto}
  .roster .sent{font-size:13.5px;color:var(--body)}
  .dots{display:flex;gap:5px;align-items:center}
  .dots i{width:9px;height:9px;border-radius:50%;border:1.5px solid var(--outline);flex:none}
  .dots i.ok{background:var(--ok);border-color:var(--ok)}
  .dots i.cur{background:var(--work);border-color:var(--work);animation:dotpulse 1.6s infinite}
  .dots i.wait{border-color:var(--need)}
  .dots .lb{font-size:11.5px;color:var(--faint);margin-left:6px}

  /* ---- performance ---- */
  .perfgrid{display:grid;grid-template-columns:1fr 1fr;gap:16px;padding:22px}
  .scard{background:var(--panel);border:1px solid var(--line);border-radius:12px;padding:18px 20px;display:flex;flex-direction:column;gap:12px}
  .scard.slip{border-color:var(--slip)}
  .scard .hd{display:flex;align-items:center;gap:10px}
  .scard .nm{font-size:16px;font-weight:600}
  .scard .trend{margin-left:auto;font-size:12px;color:var(--mut)}
  .scard .trend.up{color:var(--ok);font-weight:600} .scard .trend.down{color:var(--bad);font-weight:600}
  .mrow{display:flex;gap:22px;align-items:center;flex-wrap:wrap}
  .metric{display:flex;flex-direction:column;gap:2px}
  .metric .v{font:600 22px var(--mono);color:var(--fg)}
  .metric .v.up{color:var(--ok)} .metric .v.down{color:var(--bad)}
  .metric .l{font-size:11.5px;color:var(--faint)}
  .metric .s{font:10.5px var(--mono);color:var(--dis)}
  .mrow svg{margin-left:auto;align-self:center}
  .tchips{display:flex;gap:6px;flex-wrap:wrap;font-size:11.5px}
  .tchips span{padding:3px 9px;border-radius:12px}
  .tchips .good{background:var(--tint-ok);color:var(--ok)}
  .tchips .poor{background:var(--line);color:var(--faint)}
  .scard .foot{display:flex;align-items:center;gap:10px;font-size:12.5px;color:var(--mut)}
  select{background:var(--panel);color:var(--body);border:1px solid var(--outline);border-radius:8px;padding:6px 12px;font:13px var(--font);cursor:pointer}

  /* ---- schedule grids (week + today) ---- */
  .schedwrap{position:relative;padding:20px 22px 8px}
  .weekgrid{display:grid;grid-template-columns:130px repeat(7,1fr);gap:8px 6px;align-items:center}
  .daygrid{display:grid;grid-template-columns:130px repeat(12,1fr);gap:8px 4px;align-items:center}
  .ghead{font:600 11px var(--mono);color:var(--faint);text-align:center}
  .daygrid .ghead{font-size:10.5px}
  .ghead.today{color:var(--brass2)}
  .rowlab{font-size:14px;font-weight:600;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .rowlab.dim{color:var(--mut)}
  .rowlab.you{color:var(--brass2);font-weight:700}
  .blk{height:36px;border-radius:7px;display:flex;align-items:center;padding:0 12px;font-size:12.5px;overflow:hidden;white-space:nowrap;min-width:0}
  .blk-active{background:linear-gradient(105deg,var(--work) 42%,var(--sheen) 50%,var(--work) 58%) 0 0/220% 100%;animation:sheen 2.6s linear infinite;color:var(--on-blue);font-weight:600}
  .blk-next{background:var(--queued);color:var(--body)}
  .blk-wait{background:var(--need);color:var(--on-amber);font-weight:600}
  .blk-sched{border:1.5px dashed var(--sched);color:var(--mut)}
  .blk-pred{border:1.5px dashed var(--need);color:var(--need);padding:0 10px;font-weight:400}
  .blk-you{padding:0 10px;font-size:12px}
  .blk-idle{border:1px solid var(--line);color:var(--dis);font-style:italic;justify-content:center;font-size:12px;cursor:pointer;background:none}
  .blk[draggable=true]{cursor:grab}
  .droprow{outline:1.5px dashed var(--brass);outline-offset:2px;border-radius:7px}
  .legend{display:flex;gap:14px;font-size:11.5px;color:var(--mut);align-items:center}
  .legend .sw{width:10px;height:10px;border-radius:3px;display:inline-block;margin-right:5px;vertical-align:-1px}
  .nowline{position:absolute;width:2px;background:var(--bad);border-radius:1px}
  .nowlab{position:absolute;font:600 10px var(--mono);color:var(--bad)}
  .gridfoot{padding:6px 22px 20px;font-size:12.5px;color:var(--faint)}
  .empty{color:var(--faint);font-size:12.5px;padding:10px 22px}

  /* ---- markdown bodies ---- */
  .mdbody{font-size:12.5px;line-height:1.55;color:var(--body)}
  .mdbody h3,.mdbody h4,.mdbody h5,.mdbody h6{font-size:13px;margin:4px 0;color:var(--fg)}
  .mdbody p{margin:3px 0}
  .mdbody ul,.mdbody ol{margin:3px 0;padding-left:16px}
  .mdbody code{background:var(--line);border-radius:3px;padding:0 3px;font-family:var(--mono);font-size:11.5px}
  .mdbody pre{background:var(--line);border-radius:6px;padding:6px;overflow:auto;margin:4px 0}
  .mdbody a{color:var(--brass2)}
  .mdbody strong{color:var(--fg)}
  .preview{border:1px dashed var(--raise);border-radius:8px;padding:8px 11px;max-height:140px;overflow:auto}

  /* ---- drawer (spec 027) ---- */
  .drawer[hidden]{display:none}
  .drawer{position:fixed;inset:0;z-index:50}
  .drawer-scrim{position:absolute;inset:0;background:rgba(0,0,0,.42)}
  .drawer-panel{position:absolute;top:0;right:0;height:100%;width:min(560px,92vw);background:var(--panel);border-left:1px solid var(--line);display:flex;flex-direction:column}
  .drawer-head{display:flex;justify-content:space-between;align-items:flex-start;gap:10px;padding:14px 16px;border-bottom:1px solid var(--line)}
  .drawer-title{font-size:15px;font-weight:600;color:var(--fg)}
  .drawer-sub{font:11.5px var(--mono);color:var(--faint);margin-top:3px}
  #dw-body{max-height:220px;overflow:auto}
  .drawer-gate{display:flex;flex-direction:column;gap:8px;padding:12px 16px;border-bottom:1px solid var(--line);background:var(--tint-need)}
  .drawer-gate .q{font-size:13px;color:var(--fg);font-weight:600}
  .drawer-gate .acts{display:flex;gap:8px;flex-wrap:wrap}
  .drawer-steps{padding:8px 10px;border-bottom:1px solid var(--line);max-height:38%;overflow:auto;display:flex;flex-direction:column;gap:3px}
  .step{display:flex;align-items:center;gap:8px;padding:5px 8px;border-radius:6px;cursor:pointer;border:1px solid transparent}
  .step:hover{background:var(--line)}
  .step.sel{background:var(--line);border-color:var(--line2)}
  .step .st{font-size:11px;width:14px;text-align:center;color:var(--mut)}
  .step .nm{font-size:12.5px;color:var(--fg)}
  .step .ty{font:10.5px var(--mono);color:var(--faint);margin-left:auto}
  .step.s-succeeded .st,.step.s-approved .st{color:var(--ok)} .step.s-running .st{color:var(--work)} .step.s-failed .st,.step.s-rejected .st{color:var(--bad)} .step.s-waiting .st{color:var(--need)}
  .drawer-log{flex:1;overflow:auto;padding:10px 14px;font-size:11.5px;line-height:1.55;white-space:pre-wrap;color:var(--body);font-family:var(--mono)}
  .drawer-log .ev{padding:1px 0}
  .drawer-log .ev .k{color:var(--faint)}
  .a-tool{color:var(--mut)} .a-sub{color:var(--work);padding-left:14px} .a-msg{color:var(--body)}
</style>
</head>
<body>
<header>
  <span class="wordmark">FORT</span>
  <nav id="nav" aria-label="views">
    <button data-v="deck">Deck</button>
    <button data-v="projects">Projects</button>
    <button data-v="assign">Assign</button>
    <button data-v="perf">Performance</button>
    <button data-v="week">Week</button>
    <button data-v="today">Today</button>
  </nav>
  <span class="needpill" id="needpill" hidden></span>
  <span class="grow"></span>
  <span id="machines" style="display:flex;gap:14px"></span>
  <span class="plane" id="plane" hidden>control only</span>
  <button class="btn btn-brass" id="givedir">Give direction</button>
  <button class="iconbtn" id="theme" title="toggle theme" aria-label="toggle light/dark theme">◐</button>
</header>

<section class="view" id="v-deck">
  <div class="deck">
    <div class="deck-left">
      <div class="ulabel amber">Needs you</div>
      <div id="needlist" style="display:flex;flex-direction:column;gap:14px"></div>
      <div class="alldone" id="alldone"></div>
    </div>
    <div class="deck-right">
      <div class="ulabel">Projects</div>
      <div id="projlist" style="display:flex;flex-direction:column;gap:12px"></div>
      <div class="ulabel" style="margin-top:10px">Crew</div>
      <div id="crewlist" style="display:flex;flex-direction:column;gap:8px"></div>
    </div>
  </div>
</section>

<section class="view" id="v-projects" hidden>
  <div class="subhead"><span class="c" id="projsum"></span><span class="grow"></span><button class="btn btn-brassline" id="newbrief">＋ New brief</button></div>
  <div class="projgrid" id="projgrid"></div>
</section>

<section class="view" id="v-assign" hidden>
  <div class="subhead"><span class="c" id="crewsum"></span></div>
  <div class="assign">
    <div class="assign-left">
      <h2>Give direction</h2>
      <textarea id="brief" placeholder="Describe the outcome you want — like briefing an employee."></textarea>
      <div id="briefpv" class="mdbody preview" hidden></div>
      <div style="display:flex;flex-direction:column;gap:8px">
        <span class="ulabel" style="letter-spacing:.08em">Assign to</span>
        <div class="chips" id="agentchips"></div>
        <div class="chips machines" id="machinechips" hidden></div>
      </div>
      <label class="toggle" id="plantoggle"><span class="track"><i></i></span>Propose a plan first — I&#39;ll sign off before work starts</label>
      <button class="btn btn-brass handoff" id="handoff">Hand it off</button>
      <button class="orlink" id="tobacklog">or add to Up next ›</button>
    </div>
    <div class="assign-right">
      <div class="ulabel">The roster</div>
      <div id="rosterlist" style="display:flex;flex-direction:column;gap:12px"></div>
    </div>
  </div>
</section>

<section class="view" id="v-perf" hidden>
  <div class="subhead"><span class="t">Crew performance</span><span class="c" id="perfsum"></span><span class="grow"></span><select id="lanesel" aria-label="task type filter"><option value="">All task types</option></select></div>
  <div class="perfgrid" id="perfgrid"></div>
  <div class="empty" id="perfempty" hidden></div>
</section>

<section class="view" id="v-week" hidden>
  <div class="subhead"><span class="t">The week ahead</span><span class="cm" id="weekrange"></span><span class="grow"></span>
    <span class="legend"><span><i class="sw" style="background:var(--work)"></i>active now</span><span><i class="sw" style="background:var(--queued)"></i>up next</span><span><i class="sw" style="background:var(--need)"></i>waiting on you</span></span>
  </div>
  <div class="schedwrap"><div class="weekgrid" id="weekgrid"></div></div>
  <div class="gridfoot">&#8220;Up next&#8221; blocks are your Ready queue, ordered — drag between rows to reassign.</div>
</section>

<section class="view" id="v-today" hidden>
  <div class="subhead"><span class="t">Today</span><span class="cm" id="todaydate"></span><span class="grow"></span><span class="c" id="daysum"></span></div>
  <div class="schedwrap" id="todaywrap"><div class="daygrid" id="daygrid"></div><div class="nowline" id="nowline" hidden></div><div class="nowlab" id="nowlab" hidden>NOW</div></div>
  <div class="gridfoot">Your row is derived, not planned: solid amber = a sign-off already waiting; dashed amber = a checkpoint an agent is on pace to reach. Answer them early and the crew&#39;s afternoon compresses left.</div>
</section>

<div id="drawer" class="drawer" hidden>
  <div class="drawer-scrim" onclick="closeDrawer()"></div>
  <aside class="drawer-panel" role="dialog" aria-label="assignment detail">
    <div class="drawer-head">
      <div><div class="drawer-title" id="dw-title">—</div><div class="drawer-sub" id="dw-sub"></div><div class="mdbody" id="dw-body"></div></div>
      <button class="iconbtn" onclick="closeDrawer()" aria-label="close">✕</button>
    </div>
    <div class="drawer-gate" id="dw-gate" hidden></div>
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

// ---- state ----
let hasExec=true;
let model={sum:null,machines:[],runs:[],gates:[],backlog:[],metrics:null};
let agentOfRun={};       // flow run id -> agent of its latest started event
let actByRun={};         // live activity buffers (spec 030)
const ACT_MAX=6;
let dwRun=null, dwNode=null, dwNodes=[], dwEvents=[];
let assignCtx=null;      // {backlogId} when assigning an existing brief
let selAgent='', selMachine='', planFirst=true;
let curView=localStorage.getItem('fort-view')||'deck';
let perfLane='';

const DISP={claude:'Claude Code',codex:'Codex',hermes:'Hermes',openclaw:'OpenClaw'};
function dispName(a){
  if(!a)return 'Fort';
  if(a.indexOf('flow:')===0)return 'Fort';
  return DISP[a]||a.charAt(0).toUpperCase()+a.slice(1);
}
function runAgent(r){
  if(r.agent&&r.agent.indexOf('flow:')===0)return agentOfRun[r.id]||'';
  return r.agent==='unassigned'?'':r.agent;
}
function isLive(r){return r.status==='running'||r.status==='blocked';}
function isDone(r){return r.status==='succeeded'||r.status==='failed'||r.status==='canceled';}

// ---- time helpers ----
function ago(iso){
  if(!iso)return '';
  var s=(Date.now()-Date.parse(iso))/1000;
  if(s<60)return 'just now';
  if(s<3600)return Math.floor(s/60)+'m ago';
  if(s<86400)return Math.floor(s/3600)+'h ago';
  return Math.floor(s/86400)+'d ago';
}
function elapsed(iso){
  if(!iso)return '';
  var s=(Date.now()-Date.parse(iso))/1000;
  if(s<60)return Math.max(1,Math.floor(s))+'s';
  if(s<3600)return Math.floor(s/60)+'m';
  return (s/3600).toFixed(1).replace(/\.0$/,'')+'h';
}
function hr12(h){var x=h%12;return x===0?12:x;}
function ampm(h){return (h%24)<12?'am':'pm';}

// ---- sigils: FNV-1a -> xorshift32 -> mirrored 5x5; mark fill = status color ----
const STATE_COL={working:'#6fa8ff',need:'#e0a458',ok:'#57b98a',idle:'#56617a',failed:'#d96a6a'};
const RING_COL={working:'#6fa8ff',need:'#e0a458',ok:'#57b98a',idle:'#303848',failed:'#d96a6a'};
function sigil(name,size,color){
  var h=2166136261;
  for(var i=0;i<name.length;i++){h^=name.charCodeAt(i);h=Math.imul(h,16777619)>>>0;}
  function rnd(){h^=h<<13;h^=h>>>17;h^=h<<5;h>>>=0;return h/4294967296;}
  var cells=[];
  for(var x=0;x<3;x++)for(var y=0;y<5;y++){if(rnd()>0.55){cells.push([x,y]);if(x<2)cells.push([4-x,y]);}}
  var u=size/5,r='';
  for(var j=0;j<cells.length;j++){
    r+='<rect x="'+(cells[j][0]*u+u*0.04).toFixed(2)+'" y="'+(cells[j][1]*u+u*0.04).toFixed(2)+'" width="'+(u*0.88).toFixed(2)+'" height="'+(u*0.88).toFixed(2)+'" rx="'+(u*0.2).toFixed(2)+'" fill="'+color+'"/>';
  }
  return '<svg width="'+size+'" height="'+size+'" viewBox="0 0 '+size+' '+size+'" aria-hidden="true" style="display:block">'+r+'</svg>';
}
function ringWrap(name,size,state){
  var mark=sigil(name,size,STATE_COL[state]||STATE_COL.idle);
  if(state==='working')return '<span class="ring ring-work"><span class="race"></span>'+mark+'</span>';
  var cls=size>36?'ring-sq42':'ring-sq30';
  return '<span class="ring '+cls+'" style="border-color:'+(RING_COL[state]||RING_COL.idle)+'">'+mark+'</span>';
}

// ---- theme ----
$('#theme').addEventListener('click',function(){
  var cur=document.documentElement.getAttribute('data-theme')==='light'?'dark':'light';
  document.documentElement.setAttribute('data-theme',cur);
  localStorage.setItem('fort-theme',cur);
});

// ---- view router ----
function showView(v){
  curView=v; localStorage.setItem('fort-view',v);
  document.querySelectorAll('.view').forEach(function(s){s.hidden=('v-'+v!==s.id);});
  document.querySelectorAll('#nav button').forEach(function(b){b.classList.toggle('on',b.dataset.v===v);});
  if(v==='perf')fetchMetrics();
  render();
}
document.querySelectorAll('#nav button').forEach(function(b){b.addEventListener('click',function(){showView(b.dataset.v);});});
$('#givedir').addEventListener('click',function(){assignCtx=null;showView('assign');$('#brief').focus();});
$('#newbrief').addEventListener('click',function(){assignCtx=null;showView('assign');$('#brief').focus();});

// ---- SSE + activity buffers (spec 030) ----
function trackEvent(e){
  if(!e||!e.run_id)return;
  if(e.type==='started'&&e.data&&e.data.indexOf('{')!==0)agentOfRun[e.run_id]=e.data;
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
    return '<div class="a-sub">🤖 helper'+(d.description?' · '+esc(d.description):'')+'</div>';
  }
  const t=(e.data||'').split('\n')[0];
  return t?'<div class="a-msg">💬 '+esc(t.length>120?t.slice(0,119)+'…':t)+'</div>':'';
}
function latestActivityText(runID){
  var buf=actByRun[runID];
  if(!buf||!buf.length)return '';
  for(var i=buf.length-1;i>=0;i--){
    var e=buf[i];
    if(e.type==='message'){var t=(e.data||'').split('\n')[0];if(t)return t.length>110?t.slice(0,109)+'…':t;}
    if(e.type==='tool'){try{var d=JSON.parse(e.data||'{}');if(d.name)return 'using '+d.name+(d.summary?' — '+d.summary:'');}catch(err){}}
  }
  return '';
}

// ---- data ----
async function fetchJSON(u){const r=await fetch(u);if(!r.ok)throw new Error(u+' '+r.status);return r.json();}
async function refresh(){
  try{
    const sum=await fetchJSON('/api/summary');
    hasExec=sum.execution;
    $('#plane').hidden=hasExec;
    model.sum=sum;
    model.machines=await fetchJSON('/api/machines')||[];
    const b=await fetchJSON('/api/board');
    model.runs=b.runs||[]; model.gates=b.gates||[];
    model.backlog=await fetchJSON('/api/backlog')||[];
  }catch(err){return;}
  const liveIds=new Set(model.runs.filter(isLive).map(function(r){return r.id;}));
  Object.keys(actByRun).forEach(function(k){if(!liveIds.has(k))delete actByRun[k];});
  // flow runs whose agent we don't know yet: harvest their events once
  model.runs.forEach(function(r){
    if(r.agent&&r.agent.indexOf('flow:')===0&&!agentOfRun[r.id]&&isLive(r)){
      fetchJSON('/api/runs/'+encodeURIComponent(r.id)).then(function(d){
        (d.events||[]).forEach(trackEvent);
      }).catch(function(){});
    }
  });
  render();
  if(dwRun)loadDrawer();
}
async function fetchMetrics(){
  try{model.metrics=await fetchJSON('/api/metrics'+(perfLane?'?lane='+encodeURIComponent(perfLane):''));}catch(err){return;}
  if(curView==='perf')renderPerf();
}

// ---- derived model ----
function runByID(id){for(var i=0;i<model.runs.length;i++)if(model.runs[i].id===id)return model.runs[i];return null;}
function gatesFor(runID){return model.gates.filter(function(g){return g.run_id===runID;});}
function recentFailed(){
  var cut=Date.now()-48*3600*1000;
  return model.runs.filter(function(r){return r.status==='failed'&&Date.parse(r.updated_at||r.created_at)>cut;});
}
function needCount(){return model.gates.length+recentFailed().length;}
function agentSet(){
  var s={};
  model.machines.forEach(function(m){(m.agents||[]).forEach(function(a){s[a]=1;});});
  model.runs.forEach(function(r){var a=runAgent(r);if(a)s[a]=1;});
  if(model.metrics)(model.metrics.agents||[]).forEach(function(a){s[a.agent]=1;});
  return Object.keys(s).sort();
}
function agentStatus(a){
  var waiting=null,working=null;
  model.runs.forEach(function(r){
    if(runAgent(r)!==a)return;
    if(r.status==='blocked'&&gatesFor(r.id).length)waiting=waiting||r;
    if(r.status==='running')working=working||r;
  });
  if(waiting)return {state:'need',run:waiting};
  if(working)return {state:'working',run:working};
  return {state:'idle',run:null};
}
function projectState(r){
  if(r.status==='failed')return 'failed';
  if(gatesFor(r.id).length||r.status==='blocked')return 'need';
  if(r.status==='running')return 'working';
  // a run that finished down a rejected-gate path was closed by your redirect,
  // not delivered — never dress it in green
  if(r.status==='succeeded')return (r.checkpoints&&r.checkpoints.rejected>0)?'idle':'ok';
  return 'idle';
}
// projects = flow runs (all) + live plain runs + backlog briefs
function projects(){
  var out=[];
  model.runs.forEach(function(r){
    var isFlow=r.agent&&r.agent.indexOf('flow:')===0;
    if(isFlow||isLive(r)||r.status==='queued')out.push({kind:'run',run:r,state:projectState(r),t:Date.parse(r.created_at)||0});
  });
  model.backlog.forEach(function(b){out.push({kind:'brief',item:b,state:'idle',t:0});});
  var pri={need:0,failed:1,working:2,ok:3,idle:4};
  out.sort(function(a,b){return (pri[a.state]-pri[b.state])||(b.t-a.t);});
  return out;
}
function ckCaption(r){
  var c=r.checkpoints;
  if(r.status==='queued')return 'Up next · queued';
  if(!c||!c.total){
    if(r.status==='running')return dispName(runAgent(r))+' on it · '+elapsed(r.created_at);
    if(r.status==='succeeded')return 'Delivered '+ago(r.updated_at);
    if(r.status==='failed')return 'Stopped — needs direction';
    return 'Direct assignment — no checkpoints';
  }
  if(c.waiting>0)return c.accepted+' of '+c.total+' checkpoints accepted · '+c.waiting+' awaiting sign-off';
  if(r.status==='running')return c.accepted+' of '+c.total+' accepted · '+dispName(runAgent(r))+' working';
  if(r.status==='succeeded'&&c.rejected>0)return 'Closed after your redirect';
  if(c.accepted===c.total)return 'All '+c.total+' checkpoints accepted';
  if(r.status==='failed')return c.accepted+' of '+c.total+' accepted · stopped';
  return c.accepted+' of '+c.total+' checkpoints accepted';
}
function activitySentence(r){
  var live=latestActivityText(r.id);
  if(live)return live;
  var a=dispName(runAgent(r));
  switch(projectState(r)){
    case 'need':return a+' is waiting on your sign-off.';
    case 'working':return a+' working · '+elapsed(r.created_at)+' in.';
    case 'ok':return 'Delivered '+ago(r.updated_at)+'.';
    case 'failed':return 'Hit a wall '+ago(r.updated_at)+' — open to see what happened.';
  }
  return '';
}
function gateTitle(nodeID){
  if(nodeID==='plan_gate')return 'Sign off on the plan';
  if(nodeID==='merge_gate')return 'Sign off on the merge';
  if(nodeID==='escalate')return 'An assignment needs direction';
  return 'Sign off: '+nodeID.replace(/_/g,' ');
}

// ---- render root ----
function render(){
  renderHeader();
  if(curView==='deck')renderDeck();
  if(curView==='projects')renderProjects();
  if(curView==='assign')renderAssign();
  if(curView==='perf')renderPerf();
  if(curView==='week')renderWeek();
  if(curView==='today')renderToday();
}
function renderHeader(){
  var n=needCount();
  $('#needpill').hidden=n===0;
  $('#needpill').textContent=n+' need'+(n===1?'s':'')+' you';
  $('#machines').innerHTML=model.machines.map(function(m){
    return '<span class="mdot'+(m.reachable?'':' down')+'" title="'+esc((m.agents||[]).join(', '))+'"><i></i>'+esc(m.name)+'</span>';
  }).join('');
}

// ---- Deck (1a) ----
let openNote=null; // gate key with an open note editor — skip re-render while typing
function renderDeck(){
  if(openNote&&document.activeElement&&document.activeElement.tagName==='TEXTAREA')return;
  var cards=[];
  model.gates.forEach(function(g){
    var r=runByID(g.run_id)||{};
    var agent=dispName(runAgent(r)||agentOfRun[g.run_id]);
    var excerpt=(g.input||'').split('\n')[0];
    if(excerpt.length>140)excerpt=excerpt.slice(0,139)+'…';
    var key=g.run_id+'|'+g.node_id;
    cards.push('<div class="needcard">'+
      '<div class="hd"><span class="t">'+esc(gateTitle(g.node_id))+'</span><span class="ago">'+esc(ago(g.since))+'</span></div>'+
      '<div class="bd">'+esc(agent)+' is waiting on <span class="proj">'+esc(r.title||g.run_id)+'</span>'+(excerpt?' — '+esc(excerpt):'')+'</div>'+
      '<div class="acts">'+
        '<button class="btn btn-ok" onclick="decide(\''+g.run_id+'\',\''+esc(g.node_id)+'\',\'approve\')">Approve</button>'+
        '<button class="btn btn-outline" onclick="toggleNote(\''+esc(key)+'\')">Request changes…</button>'+
        '<button class="btn btn-ghost" onclick="openDrawer(\''+g.run_id+'\')">View the plan</button>'+
      '</div>'+
      '<div class="notebox'+(openNote===key?' open':'')+'" id="note-'+cssKey(key)+'">'+
        '<textarea placeholder="What should change?" aria-label="redirect note"></textarea>'+
        '<div style="display:flex;gap:8px"><button class="btn btn-outline" onclick="sendNote(\''+g.run_id+'\',\''+esc(g.node_id)+'\')">Send it back</button>'+
        '<button class="btn btn-ghost" onclick="toggleNote(null)">Cancel</button></div>'+
      '</div>'+
    '</div>');
  });
  recentFailed().forEach(function(r){
    cards.push('<div class="needcard fail">'+
      '<div class="hd"><span class="t">'+esc(r.title||r.id)+' hit a wall</span><span class="ago">'+esc(ago(r.updated_at))+'</span></div>'+
      '<div class="bd">'+esc(dispName(runAgent(r)))+' stopped'+(r.machine?' on <span class="proj">'+esc(r.machine)+'</span>':'')+'. Open it to see what happened and give direction.</div>'+
      '<div class="acts"><button class="btn btn-outline" onclick="openDrawer(\''+r.id+'\')">View what happened</button></div>'+
    '</div>');
  });
  $('#needlist').innerHTML=cards.join('');
  var workers=agentSet().filter(function(a){return agentStatus(a).state==='working';}).length;
  var verb=workers===1?' is':'s are', need=workers===1?'doesn&#39;t':'don&#39;t';
  var msg;
  if(cards.length)msg='That&#39;s everything else — '+workers+' agent'+verb+' working and '+need+' need you.';
  else if(workers)msg='That&#39;s everything — '+workers+' agent'+verb+' working and '+need+' need you.';
  else msg='All quiet — nothing needs you.';
  $('#alldone').innerHTML=msg;

  var rows=projects().slice(0,6).map(function(p){
    if(p.kind==='brief'){
      return '<div class="projrow" tabindex="0" role="button" onclick="assignBrief(\''+p.item.id+'\')" onkeydown="if(event.key===\'Enter\')assignBrief(\''+p.item.id+'\')">'+
        ringWrap(p.item.title,30,'idle')+
        '<div class="col"><span class="nm">'+esc(p.item.title)+'</span><span class="cap dim">Not started · brief drafted</span></div></div>';
    }
    var r=p.run;
    return '<div class="projrow" tabindex="0" role="button" onclick="openDrawer(\''+r.id+'\')" onkeydown="if(event.key===\'Enter\')openDrawer(\''+r.id+'\')">'+
      ringWrap(r.title||r.id,30,p.state==='failed'?'failed':p.state)+
      '<div class="col"><span class="nm">'+esc(r.title||r.id)+'</span><span class="cap'+(p.state==='idle'?' dim':'')+'">'+esc(ckCaption(r))+'</span></div></div>';
  });
  $('#projlist').innerHTML=rows.join('')||'<div class="empty" style="padding:4px 0">Nothing on the board yet — give direction to start.</div>';

  $('#crewlist').innerHTML=agentSet().map(function(a){
    var st=agentStatus(a);
    var dot=st.state==='working'?'work':st.state==='need'?'need':'';
    var act;
    if(st.state==='need')act='waiting on your sign-off';
    else if(st.state==='working'){
      var live=latestActivityText(st.run.id);
      act=(live||('working — '+(st.run.title||'')))+' · '+elapsed(st.run.created_at);
    }else act='idle';
    return '<div class="crewrow"><span class="dot '+dot+'"></span><strong>'+esc(dispName(a))+'</strong><span class="act'+(st.state==='idle'?' dim':'')+'">'+esc(act)+'</span></div>';
  }).join('')||'<div class="empty" style="padding:4px 0">No agents seen yet.</div>';
}
function cssKey(k){return k.replace(/[^a-zA-Z0-9_-]/g,'_');}
function toggleNote(key){
  openNote=(openNote===key?null:key);
  document.querySelectorAll('.notebox').forEach(function(n){n.classList.remove('open');});
  if(openNote){var el=$('#note-'+cssKey(openNote));if(el){el.classList.add('open');el.querySelector('textarea').focus();}}
}
async function sendNote(run,node){
  var el=$('#note-'+cssKey(run+'|'+node));
  var note=el?el.querySelector('textarea').value.trim():'';
  openNote=null;
  await decide(run,node,'reject',note);
}
function assignBrief(id){
  var item=null;
  model.backlog.forEach(function(b){if(b.id===id)item=b;});
  if(!item)return;
  assignCtx={backlogId:id};
  showView('assign');
  $('#brief').value=item.title+(item.body?'\n'+item.body:'');
  selAgent=item.agent||'';
  renderAssignControls();
  renderBriefPreview();
}

// ---- Projects (1b) ----
function renderProjects(){
  var ps=projects();
  var working=agentSet().filter(function(a){return agentStatus(a).state==='working';}).length;
  $('#projsum').textContent=ps.length+' project'+(ps.length===1?'':'s')+' · '+working+' agent'+(working===1?'':'s')+' working';
  $('#projgrid').innerHTML=ps.slice(0,8).map(function(p){
    if(p.kind==='brief'){
      var quote=(p.item.body||p.item.title).split('\n')[0];
      if(quote.length>120)quote=quote.slice(0,119)+'…';
      return '<div class="pcard brief" onclick="assignBrief(\''+p.item.id+'\')">'+
        '<div class="hd">'+ringWrap(p.item.title,42,'idle')+
          '<div class="col"><span class="nm">'+esc(p.item.title)+'</span><span class="sub dim">Brief drafted · no one assigned</span></div></div>'+
        '<div class="act dim">&#8220;'+esc(quote)+'&#8221;</div>'+
        '<button class="btn btn-brassline cta" style="font-size:13.5px" onclick="event.stopPropagation();assignBrief(\''+p.item.id+'\')">Assign an agent</button>'+
      '</div>';
    }
    var r=p.run, c=r.checkpoints, state=p.state;
    var agents=dispName(runAgent(r));
    var sub=agents+(r.machine?' · on '+r.machine:'');
    if(state==='ok')sub=agents+' · finished '+ago(r.updated_at);
    var pill=state==='need'?'<span class="pill pill-need">needs you</span>':
             state==='working'?'<span class="pill pill-work">working</span>':
             state==='ok'?'<span class="pill pill-ok">delivered</span>':
             state==='failed'?'<span class="pill pill-fail">failed</span>':'';
    var bar='',cap='';
    if(c&&c.total){
      var segs=[];
      for(var i=0;i<c.accepted;i++)segs.push('ok');
      for(var i=0;i<c.waiting;i++)segs.push('need');
      for(var i=0;i<c.rejected;i++)segs.push('bad');
      if(r.status==='running')segs.push('work');
      while(segs.length<c.total)segs.push('');
      bar='<div class="bar">'+segs.map(function(s){return '<i'+(s?' class="'+s+'"':'')+'></i>';}).join('')+'</div>';
      var bits=[c.accepted+' accepted'];
      if(c.waiting)bits.push('<span class="need">'+c.waiting+' awaiting your sign-off</span>');
      if(r.status==='running')bits.push('<span class="work">1 in progress</span>');
      var left=c.total-c.accepted-c.waiting-c.rejected-(r.status==='running'?1:0);
      if(left>0)bits.push(left+' not started');
      var capText=c.accepted===c.total?'All '+c.total+' checkpoints accepted':
        (r.status==='succeeded'&&c.rejected>0)?'Closed after your redirect':bits.join(' · ');
      cap='<span class="barcap">'+capText+'</span>';
    }
    var cta='';
    if(state==='need')cta='<button class="btn btn-amber cta" onclick="event.stopPropagation();openDrawer(\''+r.id+'\')">Review the plan</button>';
    else if(state==='working')cta='<button class="btn btn-neutral cta" onclick="event.stopPropagation();openDrawer(\''+r.id+'\')">Watch the work</button>';
    return '<div class="pcard'+(state==='need'?' need':'')+'" onclick="openDrawer(\''+r.id+'\')">'+
      '<div class="hd">'+ringWrap(r.title||r.id,42,state==='failed'?'failed':state)+
        '<div class="col"><span class="nm">'+esc(r.title||r.id)+'</span><span class="sub">'+esc(sub)+'</span></div>'+pill+'</div>'+
      (bar?'<div class="col" style="gap:6px">'+bar+cap+'</div>':'')+
      '<div class="act">'+esc(activitySentence(r))+'</div>'+cta+
    '</div>';
  }).join('')||'<div class="empty">No projects yet — hand off a brief to start one.</div>';
}

// ---- Assign (1c) ----
function renderAssign(){
  var as=agentSet();
  var ms=model.machines;
  $('#crewsum').textContent='Your crew · '+as.length+' agent'+(as.length===1?'':'s')+(ms.length?' · '+ms.length+' machine'+(ms.length===1?'':'s'):'');
  renderAssignControls();
  $('#rosterlist').innerHTML=as.map(function(a){
    var st=agentStatus(a);
    var cls=st.state==='need'?'need':st.state==='working'?'work':'idle';
    if(st.state==='idle'){
      return '<div class="roster idle"><div class="hd"><span class="nm">'+esc(dispName(a))+'</span><span class="pill pill-idle">idle</span>'+
        '<button class="btn btn-brassline" style="margin-left:auto;font-size:12.5px;padding:5px 12px" onclick="pickAgent(\''+esc(a)+'\')">Assign work</button></div></div>';
    }
    var r=st.run;
    var pill=st.state==='need'?'<span class="pill pill-need">waiting on you</span>':
      '<span class="pill pill-work">working · '+esc(elapsed(r.created_at))+'</span>';
    var live=latestActivityText(r.id);
    var sent=(r.title||'')+' — '+(st.state==='need'?'waiting on your sign-off.':(live||'in progress.'));
    var dots='';
    var c=r.checkpoints;
    if(c&&c.total){
      var arr=[];
      for(var i=0;i<c.accepted;i++)arr.push('ok');
      for(var i=0;i<c.waiting;i++)arr.push('wait');
      if(r.status==='running')arr.push('cur');
      while(arr.length<c.total)arr.push('');
      dots='<div class="dots">'+arr.map(function(d){return '<i'+(d?' class="'+d+'"':'')+'></i>';}).join('')+'<span class="lb">checkpoints</span></div>';
    }
    return '<div class="roster '+cls+'"><div class="hd"><span class="nm">'+esc(dispName(a))+'</span>'+pill+
      '<span class="mc">'+esc(r.machine||'')+'</span></div>'+
      '<div class="sent">'+esc(sent)+'</div>'+dots+'</div>';
  }).join('')||'<div class="empty" style="padding:4px 0">No agents seen yet — they appear once work is routed.</div>';
}
function pickAgent(a){selAgent=a;renderAssignControls();$('#brief').focus();}
function pickMachine(m){selMachine=m;renderAssignControls();}
function renderAssignControls(){
  var as=agentSet();
  $('#agentchips').innerHTML='<button class="chip'+(selAgent===''?' on':'')+'" onclick="pickAgent(\'\')">Fort decides</button>'+
    as.map(function(a){return '<button class="chip'+(selAgent===a?' on':'')+'" onclick="pickAgent(\''+esc(a)+'\')">'+esc(dispName(a))+'</button>';}).join('');
  var ms=model.machines;
  $('#machinechips').hidden=ms.length<2;
  if(ms.length>=2){
    $('#machinechips').innerHTML='<button class="chip'+(selMachine===''?' on':'')+'" onclick="pickMachine(\'\')">any machine</button>'+
      ms.map(function(m){return '<button class="chip'+(selMachine===m.name?' on':'')+'" onclick="pickMachine(\''+esc(m.name)+'\')">'+esc(m.name)+'</button>';}).join('');
  }
  $('#plantoggle').classList.toggle('off',!planFirst);
}
$('#plantoggle').addEventListener('click',function(){planFirst=!planFirst;renderAssignControls();});
function renderBriefPreview(){
  var t=$('#brief').value,i=t.indexOf('\n');
  var body=i<0?'':t.slice(i+1).trim();
  var pv=$('#briefpv');
  if(!body){pv.hidden=true;pv.innerHTML='';return;}
  pv.hidden=false;pv.innerHTML='<strong>'+esc(t.slice(0,i).trim())+'</strong>'+md(body);
}
$('#brief').addEventListener('input',renderBriefPreview);
$('#brief').addEventListener('keydown',function(e){if((e.metaKey||e.ctrlKey)&&e.key==='Enter')handoff();});
async function handoff(){
  var text=$('#brief').value;
  if(!text.trim())return;
  if(assignCtx&&assignCtx.backlogId){
    await fetch('/api/backlog/'+assignCtx.backlogId,{method:'PATCH',headers:{'content-type':'application/json'},body:JSON.stringify({agent:selAgent})});
    await fetch('/api/backlog/'+assignCtx.backlogId+'/dispatch',{method:'POST'});
    assignCtx=null;$('#brief').value='';renderBriefPreview();showView('deck');refresh();
    return;
  }
  if(planFirst){
    var r=await fetch('/api/breakdown',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({text:text,agent:selAgent,machine:selMachine})});
    if(r.status===409){alert('Drafting a plan needs the execution plane — start fort serve, or turn the toggle off to hand it off directly.');return;}
  }else{
    await fetch('/api/chat',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({text:text,agent:selAgent,machine:selMachine})});
  }
  $('#brief').value='';renderBriefPreview();showView('deck');refresh();
}
$('#handoff').addEventListener('click',handoff);
$('#tobacklog').addEventListener('click',async function(){
  var t=$('#brief').value;if(!t.trim())return;
  var i=t.indexOf('\n');
  var title=i<0?t.trim():t.slice(0,i).trim();
  var body=i<0?'':t.slice(i+1).trim();
  await fetch('/api/backlog',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({title:title,body:body,agent:selAgent,machine:selMachine})});
  $('#brief').value='';renderBriefPreview();refresh();
});

// ---- Performance (2a) ----
$('#lanesel').addEventListener('change',function(){perfLane=this.value;fetchMetrics();});
function renderPerf(){
  var m=model.metrics;
  if(!m){$('#perfsum').textContent='';return;}
  $('#perfsum').textContent='last '+m.window_days+' days · '+m.assignments+' assignment'+(m.assignments===1?'':'s');
  var sel=$('#lanesel'),cur=perfLane;
  sel.innerHTML='<option value="">All task types</option>'+(m.lanes||[]).map(function(l){return '<option value="'+esc(l)+'"'+(l===cur?' selected':'')+'>'+esc(l.replace(/[-_]/g,' '))+'</option>';}).join('');
  var ags=m.agents||[];
  $('#perfempty').hidden=ags.length>0;
  $('#perfempty').textContent='No assignments in the last '+m.window_days+' days.';
  $('#perfgrid').innerHTML=ags.map(function(a){
    var tcls=a.trend==='improving'?'up':a.trend==='slipping'?'down':'';
    var tarrow=a.trend==='improving'?'▲ improving':a.trend==='slipping'?'▼ slipping':'→ steady';
    var scol=a.trend==='improving'?'var(--ok)':a.trend==='slipping'?'var(--bad)':'var(--mut)';
    var pts=(a.spark||[]).map(function(v,i){return (i*15)+','+(30-(v/100)*26).toFixed(1);}).join(' ');
    var fp=a.decided?Math.round(a.first_pass_pct)+'%':'—';
    var cost=a.cost_known&&a.accepted?'$'+a.cost_per_accepted.toFixed(2):'—';
    var chips=(a.best||[]).map(function(b,i){return '<span class="good">'+(i===0?'best at: ':'')+esc(b)+'</span>';}).join('')+
              (a.weak||[]).map(function(w){return '<span class="poor">weak: '+esc(w)+'</span>';}).join('');
    var foot;
    if(!a.decided)foot='No sign-offs in this window yet — numbers appear after your first accepts.';
    else if(a.trend==='improving')foot='First-pass acceptance up '+Math.abs(Math.round(a.trend_delta))+' pts across the window.';
    else if(a.trend==='slipping')foot='First-pass acceptance down '+Math.abs(Math.round(a.trend_delta))+' pts — steer earlier or split the work smaller.';
    else foot='Holding steady on '+a.decided+' sign-off'+(a.decided===1?'':'s')+'.';
    return '<div class="scard'+(a.trend==='slipping'?' slip':'')+'">'+
      '<div class="hd"><span class="nm">'+esc(dispName(a.agent))+'</span><span class="trend '+tcls+'">'+tarrow+'</span></div>'+
      '<div class="mrow">'+
        '<div class="metric"><span class="v '+tcls+'">'+fp+'</span><span class="l">first-pass accepted</span><span class="s">'+a.decided+' signed off</span></div>'+
        '<div class="metric"><span class="v">'+a.redirects_per_assignment.toFixed(1)+'</span><span class="l">redirects / assignment</span><span class="s">'+a.assignments+' assignment'+(a.assignments===1?'':'s')+'</span></div>'+
        '<div class="metric"><span class="v">'+cost+'</span><span class="l">per accepted checkpoint</span><span class="s">'+(a.cost_known?'engine-reported':'no cost data')+'</span></div>'+
        '<svg width="90" height="34" viewBox="0 0 90 34" aria-hidden="true"><polyline points="'+pts+'" fill="none" stroke="'+scol+'" stroke-width="2"/></svg>'+
      '</div>'+
      (chips?'<div class="tchips">'+chips+'</div>':'')+
      '<div class="foot">'+esc(foot)+'</div>'+
    '</div>';
  }).join('');
}

// ---- schedule shared ----
function agentRows(){
  return agentSet().map(function(a){return {agent:a,st:agentStatus(a)};});
}
function meanDuration(agent){
  var ds=[];
  model.runs.forEach(function(r){
    if(runAgent(r)!==agent||!isDone(r))return;
    var d=(Date.parse(r.updated_at)-Date.parse(r.created_at))/1000;
    if(d>=10&&d<=8*3600)ds.push(d);
  });
  if(ds.length<2)return null;
  return ds.reduce(function(x,y){return x+y;},0)/ds.length;
}
function etaFor(run){
  var a=runAgent(run);if(!a)return null;
  var mu=meanDuration(a);if(mu==null)return null;
  var el=(Date.now()-Date.parse(run.created_at))/1000;
  return new Date(Date.now()+Math.max(mu-el,300)*1000);
}
function spacer(n){return n>0?'<span style="grid-column:span '+n+'"></span>':'';}

// ---- Week (2b) ----
function renderWeek(){
  var now=new Date();
  var mon=new Date(now);mon.setHours(0,0,0,0);mon.setDate(mon.getDate()-((mon.getDay()+6)%7));
  var end=new Date(mon);end.setDate(end.getDate()+6);
  var MONTH=['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'];
  $('#weekrange').textContent=MONTH[mon.getMonth()]+' '+mon.getDate()+' – '+MONTH[end.getMonth()]+' '+end.getDate();
  var todayIdx=Math.max(0,Math.min(6,Math.floor((now-mon)/86400000)));
  var DAYS=['MON','TUE','WED','THU','FRI','SAT','SUN'];
  var html='<span></span>'+DAYS.map(function(d,i){
    return '<span class="ghead'+(i===todayIdx?' today':'')+'">'+d+(i===todayIdx?' ·today':'')+'</span>';
  }).join('');
  var nextByAgent={};
  model.backlog.forEach(function(b){
    if(b.agent)(nextByAgent[b.agent]=nextByAgent[b.agent]||[]).push(b);
  });
  agentRows().forEach(function(row){
    var a=row.agent, cells=[], cur=0;
    function put(day,span,cls,label,attrs){
      day=Math.max(day,cur); if(day>6)return;
      span=Math.min(span,7-day); if(span<1)return;
      if(day>cur)cells.push(spacer(day-cur));
      cells.push('<div class="blk '+cls+'" style="grid-column:span '+span+'"'+(attrs||'')+'>'+label+'</div>');
      cur=day+span;
    }
    model.runs.forEach(function(r){
      if(runAgent(r)!==a)return;
      if(r.status==='running'){
        var startIdx=Math.max(0,Math.floor((Date.parse(r.created_at)-mon)/86400000));
        startIdx=Math.min(startIdx,todayIdx);
        put(startIdx,todayIdx-startIdx+1,'blk-active',esc(r.title||''));
      }else if(r.status==='blocked'&&gatesFor(r.id).length){
        var g=gatesFor(r.id)[0];
        var gd=Math.max(0,Math.min(6,Math.floor((Date.parse(g.since||r.updated_at)-mon)/86400000)));
        put(gd,1,'blk-wait','⏸ on you');
        put(cur,2,'blk-next',esc((r.title||'')+' — once approved'));
      }
    });
    (nextByAgent[a]||[]).forEach(function(b){
      put(Math.max(cur,todayIdx+1),2,'blk-next',esc(b.title),' draggable="true" data-bid="'+b.id+'"');
    });
    if(cells.length===0){
      cells.push(spacer(todayIdx));
      cells.push('<div class="blk blk-idle" style="grid-column:span '+(7-todayIdx)+'" onclick="pickAgent(\''+esc(a)+'\');showView(\'assign\')">open capacity — assign work</div>');
      cur=7;
    }
    if(cur<7)cells.push(spacer(7-cur));
    html+='<span class="rowlab'+(row.st.state==='idle'?' dim':'')+'" data-agent="'+esc(a)+'">'+esc(dispName(a))+'</span>'+cells.join('');
  });
  $('#weekgrid').innerHTML=html;
  wireDrag();
}
function wireDrag(){
  var dragId=null;
  document.querySelectorAll('#weekgrid .blk[draggable=true]').forEach(function(b){
    b.addEventListener('dragstart',function(e){dragId=b.dataset.bid;e.dataTransfer.effectAllowed='move';});
  });
  document.querySelectorAll('#weekgrid .rowlab').forEach(function(lab){
    lab.addEventListener('dragover',function(e){e.preventDefault();lab.classList.add('droprow');});
    lab.addEventListener('dragleave',function(){lab.classList.remove('droprow');});
    lab.addEventListener('drop',async function(e){
      e.preventDefault();lab.classList.remove('droprow');
      if(!dragId)return;
      await fetch('/api/backlog/'+dragId,{method:'PATCH',headers:{'content-type':'application/json'},body:JSON.stringify({agent:lab.dataset.agent})});
      dragId=null;refresh();
    });
  });
}

// ---- Today (3a) ----
function renderToday(){
  var now=new Date();
  var MONTH=['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'];
  var DAY=['Sun','Mon','Tue','Wed','Thu','Fri','Sat'];
  var hh=now.getHours(),mm=now.getMinutes();
  $('#todaydate').textContent=DAY[now.getDay()]+' '+MONTH[now.getMonth()]+' '+now.getDate()+' · now '+hh+':'+(mm<10?'0':'')+mm;
  var dayStart=8,hours=12;
  var day8=new Date(now);day8.setHours(dayStart,0,0,0);
  function colOf(date){
    var h=(date-day8)/3600000;
    return Math.max(0,Math.min(hours-1,Math.floor(h)));
  }
  var html='<span></span>';
  for(var h=dayStart;h<dayStart+hours;h++){
    html+='<span class="ghead'+(h===hh?' today':'')+'">'+hr12(h)+'</span>';
  }
  // You row: waiting sign-offs + predicted review moments (derived, not planned)
  var youCells=[],youCur=0,preds=[];
  function putYou(col,cls,label){
    col=Math.max(col,youCur); if(col>=hours)return;
    if(col>youCur)youCells.push(spacer(col-youCur));
    youCells.push('<div class="blk blk-you '+cls+'" style="grid-column:span 1">'+label+'</div>');
    youCur=col+1;
  }
  model.gates.forEach(function(g){
    var t=Date.parse(g.since||'');
    putYou(colOf(new Date(isNaN(t)?Date.now():t)),'blk-wait','⚑ sign-off');
  });
  model.runs.forEach(function(r){
    if(r.status!=='running')return;
    var eta=etaFor(r);
    if(eta&&eta.getDate()===now.getDate()&&eta.getHours()>=dayStart&&eta.getHours()<dayStart+hours)preds.push(eta);
  });
  preds.sort(function(a,b){return a-b;});
  preds.forEach(function(eta){putYou(colOf(eta),'blk-pred','~ review');});
  if(youCur<hours)youCells.push(spacer(hours-youCur));
  html+='<span class="rowlab you">You</span>'+youCells.join('');
  // agent rows
  var nextByAgent={};
  model.backlog.forEach(function(b){if(b.agent)(nextByAgent[b.agent]=nextByAgent[b.agent]||[]).push(b);});
  agentRows().forEach(function(row){
    var a=row.agent,cells=[],cur=0;
    function put(col,span,cls,label){
      col=Math.max(col,cur);if(col>=hours)return;
      span=Math.max(1,Math.min(span,hours-col));
      if(col>cur)cells.push(spacer(col-cur));
      cells.push('<div class="blk '+cls+'" style="grid-column:span '+span+'">'+label+'</div>');
      cur=col+span;
    }
    model.runs.forEach(function(r){
      if(runAgent(r)!==a)return;
      if(r.status==='running'){
        var sc=colOf(new Date(Date.parse(r.created_at)));
        var eta=etaFor(r);
        var ec=eta?colOf(eta)+1:colOf(now)+2;
        var label=esc(r.title||'');
        if(eta){
          var c=r.checkpoints;
          var nextCk=c&&c.total>c.accepted+c.rejected;
          label+=' → '+(nextCk?'checkpoint '+(c.accepted+1):'done')+' ~'+hr12(eta.getHours())+ampm(eta.getHours());
        }
        put(sc,Math.max(2,ec-sc),'blk-active',label);
      }else if(r.status==='blocked'&&gatesFor(r.id).length){
        var g=gatesFor(r.id)[0];
        var t=Date.parse(g.since||r.updated_at);
        put(colOf(new Date(isNaN(t)?Date.now():t)),2,'blk-wait','⏸ waiting on you');
        put(cur,5,'blk-next',esc((r.title||'')+' — starts on your approval'));
      }
    });
    (nextByAgent[a]||[]).slice(0,1).forEach(function(b){
      put(Math.max(cur,colOf(now)+1),3,'blk-next','then: '+esc(b.title));
    });
    if(cells.length===0){
      var start=colOf(now)+1;
      cells.push(spacer(start));
      cells.push('<div class="blk blk-idle" style="grid-column:span '+(hours-start)+'" onclick="pickAgent(\''+esc(a)+'\');showView(\'assign\')">idle — assign work</div>');
      cur=hours;
    }
    if(cur<hours)cells.push(spacer(hours-cur));
    html+='<span class="rowlab'+(row.st.state==='idle'?' dim':'')+'">'+esc(dispName(a))+'</span>'+cells.join('');
  });
  $('#daygrid').innerHTML=html;
  // NOW line
  var frac=(hh+mm/60-dayStart)/hours;
  var line=$('#nowline'),lab=$('#nowlab');
  if(frac>=0&&frac<=1){
    line.hidden=false;lab.hidden=false;
    line.style.left='calc(130px + (100% - 152px - 130px) * '+frac.toFixed(3)+' + 22px)';
    line.style.top='16px';line.style.bottom='4px';
    lab.style.left='calc(130px + (100% - 152px - 130px) * '+frac.toFixed(3)+' - 2px)';lab.style.top='2px';
  }else{line.hidden=true;lab.hidden=true;}
  // day summary
  var g=model.gates.length,m=preds.length;
  var sumtxt;
  if(g===0&&m===0)sumtxt='nothing needs you yet — the crew is heads-down';
  else{
    var latest=preds.length?preds[preds.length-1]:null;
    var evening=(!latest||latest.getHours()<17)?' · evening is clear':'';
    sumtxt=(g?g+' sign-off'+(g===1?'':'s')+' waiting':'')+
           (g&&m?' · ':'')+(m?m+' more expected today':'')+evening;
  }
  $('#daysum').textContent=sumtxt;
}

// ---- drawer (spec 027) ----
function stepIcon(s){return s==='succeeded'||s==='approved'?'✓':s==='failed'||s==='rejected'?'✕':s==='running'?'▸':s==='waiting'?'⏸':'▫';}
async function openDrawer(runID){ dwRun=runID; dwNode=null; $('#drawer').hidden=false; await loadDrawer(); }
function closeDrawer(){ dwRun=null; dwNode=null; $('#drawer').hidden=true; }
async function loadDrawer(){
  if(!dwRun) return;
  const id=dwRun;
  const d=await (await fetch('/api/runs/'+encodeURIComponent(id))).json();
  if(dwRun!==id) return; // a stale in-flight fetch: the drawer moved to another run
  dwNodes=d.nodes||[]; dwEvents=d.events||[];
  dwEvents.forEach(trackEvent);
  $('#dw-title').textContent=d.run.title||d.run.id;
  $('#dw-sub').textContent=[dispName(runAgent(d.run)),d.run.status,d.run.machine].filter(Boolean).join(' · ');
  $('#dw-body').innerHTML=d.run.body?md(d.run.body):'';
  var gs=gatesFor(id), gb=$('#dw-gate');
  if(gs.length){
    var g=gs[0];
    gb.hidden=false;
    gb.innerHTML='<div class="q">'+esc(gateTitle(g.node_id))+'</div>'+
      '<div class="acts"><button class="btn btn-ok" onclick="decide(\''+g.run_id+'\',\''+esc(g.node_id)+'\',\'approve\')">Approve</button>'+
      '<button class="btn btn-outline" onclick="drawerReject(\''+g.run_id+'\',\''+esc(g.node_id)+'\')">Request changes…</button></div>';
  }else{gb.hidden=true;gb.innerHTML='';}
  renderSteps(); renderLog();
}
async function drawerReject(run,node){
  var note=prompt('What should change?')||'';
  await decide(run,node,'reject',note);
  loadDrawer();
}
function renderSteps(){
  const el=$('#dw-steps');
  if(!dwNodes.length){ el.style.display='none'; return; }
  el.style.display='flex';
  el.innerHTML=dwNodes.map(n=>
    '<div class="step s-'+esc(n.status)+(n.node_id===dwNode?' sel':'')+'" onclick="selectStep(\''+esc(n.node_id)+'\')">'+
    '<span class="st">'+stepIcon(n.status)+'</span><span class="nm">'+esc(n.node_id)+'</span>'+
    '<span class="ty">'+esc(n.type==='gate'?'checkpoint':n.type)+'</span></div>').join('');
}
function selectStep(nodeID){ dwNode=(dwNode===nodeID?null:nodeID); renderSteps(); renderLog(); }
function renderLog(){
  const log=$('#dw-log');
  const evs=dwEvents.filter(e=>!dwNode||e.node_id===dwNode);
  const atBottom=log.scrollHeight-log.scrollTop-log.clientHeight<24;
  const prev=log.scrollTop;
  if(!evs.length){ log.innerHTML='<div class="empty" style="padding:0">waiting…</div>'; return; }
  log.innerHTML=evs.map(e=>{
    if(e.type==='tool'||e.type==='subagent')return '<div class="ev">'+activityLine(e)+'</div>';
    return '<div class="ev"><span class="k">'+esc(e.type)+'</span> '+esc(e.data||'')+'</div>';
  }).join('');
  log.scrollTop=atBottom?log.scrollHeight:prev;
}
document.addEventListener('keydown',e=>{if(e.key==='Escape'){if(openNote)toggleNote(null);else closeDrawer();}});

// ---- actions ----
async function decide(run,node,decision,note){
  const body={run_id:run,node_id:node,decision:decision};
  if(note)body.note=note;
  const r=await fetch('/api/gate',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify(body)});
  if(r.status===409)alert('No execution plane — start fort serve to act on sign-offs.');
  refresh();
}

// ---- boot ----
let refreshQueued=false;
const es=new EventSource('/api/events?since=0');
es.onmessage=ev=>{
  try{trackEvent(JSON.parse(ev.data))}catch(err){}
  if(!refreshQueued){refreshQueued=true;setTimeout(function(){refreshQueued=false;refresh();},300);}
};
setInterval(refresh,3000);
setInterval(fetchMetrics,60000);
showView(curView);
refresh();
fetchMetrics();
</script>
</body>
</html>`
