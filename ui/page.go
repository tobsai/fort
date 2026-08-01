package ui

// boardHTML is the web control plane served at GET / — the delegation-model
// dashboard (spec 033, from design_handoff_fort_dashboard_redesign/): views
// behind one top-bar nav. Deck (needs-you inbox + conversations + crew), Assign
// (work intake + roster),
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
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover"/>
<title>Fort — Command Deck</title>
<link rel="icon" type="image/png" href="/fort-icon.png"/>
<link rel="apple-touch-icon" href="/fort-agent-orb.png"/>
<script>(function(){var s=localStorage.getItem('fort-theme');document.documentElement.setAttribute('data-theme',s||(matchMedia('(prefers-color-scheme: light)').matches?'light':'dark'));})();</script>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Instrument+Sans:wght@400;500;600;700&family=Spline+Sans+Mono:wght@400;500;600&display=swap" rel="stylesheet">
<style>
  :root{
    --bg:#030b14;--panel:#071320;--line:#102235;--line2:#18334d;--raise:#24547f;--outline:#29445f;
    --fg:#f1f6fc;--body:#b9c7d7;--mut:#8596aa;--faint:#60758d;--dis:#41546a;
    --brass:#168cff;--brass2:#60b8ff;--work:#25a4ff;--need:#d69f35;--ok:#66c791;--bad:#dd707b;
    --queued:#152a42;--sched:#59718a;--seg0:#14283d;--sheen:#e5f4ff;--slip:#3a3020;
    --on-brass:#f7fbff;--on-amber:#12100a;--on-blue:#f7fbff;--on-green:#07120c;
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
  html,body{min-height:100%}
  body{margin:0;background:var(--bg);color:var(--fg);font:14px/1.5 var(--font);transition:background .15s,color .15s}
  button{font:inherit}
  .mono{font-family:var(--mono)}
  @keyframes spinrace{to{transform:rotate(360deg)}}
  @keyframes sheen{to{background-position:-220% 0}}
  @keyframes dotpulse{0%,100%{opacity:1}50%{opacity:.35}}
  @keyframes orbCoreDrift{
    0%,100%{transform:rotate(-1.5deg) scale(.985)}
    34%{transform:rotate(2.5deg) scale(1.035)}
    68%{transform:rotate(-2deg) scale(1.01)}
  }
  @keyframes orbEnergyBreathe{
    0%,100%{filter:brightness(.96) saturate(1) drop-shadow(0 0 6px rgba(29,142,255,.3))}
    50%{filter:brightness(1.13) saturate(1.18) drop-shadow(0 0 14px rgba(70,177,255,.78))}
  }
  @keyframes orbReducedEnergyPulse{
    0%,100%{filter:brightness(.98) saturate(1.02) drop-shadow(0 0 6px rgba(29,142,255,.32))}
    50%{filter:brightness(1.07) saturate(1.1) drop-shadow(0 0 11px rgba(70,177,255,.62))}
  }
  .fort-orb{transform-origin:50% 50%}
  .fort-orb.is-thinking{will-change:transform,filter;animation:orbCoreDrift 2.6s cubic-bezier(.45,0,.55,1) infinite,orbEnergyBreathe 1.7s ease-in-out infinite}
  @media (prefers-reduced-motion: reduce){
    *{animation:none!important}
    .fort-orb.is-thinking{will-change:filter;transform:none!important;animation:orbReducedEnergyPulse 4s ease-in-out infinite!important}
    .conversation-sidebar{transition:none!important}
  }
  a{color:var(--brass);text-decoration:none}a:hover{color:var(--brass2)}
  a:focus-visible,button:focus-visible,select:focus-visible,input:focus-visible,textarea:focus-visible,[tabindex]:focus-visible{outline:2px solid var(--brass);outline-offset:1px}

  /* ---- top bar ---- */
  header{display:flex;align-items:center;gap:13px;min-height:53px;padding:9px 16px;border-bottom:1px solid var(--line);background:#020914}
  .brand-icon{width:36px;height:36px;border-radius:50%;display:block;filter:drop-shadow(0 0 10px rgba(29,142,255,.45))}
  .wordmark{font:700 14px var(--mono);letter-spacing:.24em;color:var(--fg)}
  nav{display:flex;gap:2px}
  nav button{font-size:13px;color:var(--mut);background:none;border:none;padding:5px 10px;border-radius:7px;cursor:pointer}
  nav button:hover{color:var(--fg)}
  nav button.on{color:var(--fg);background:transparent}
  .needpill{font-size:12px;padding:4px 10px;border:1px solid rgba(214,159,53,.34);border-radius:20px;background:var(--tint-need);color:var(--need);font-weight:600;cursor:pointer}
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

  /* ---- Fort orb identity ---- */
  .orb-ring{display:inline-flex;align-items:center;justify-content:center;border:1px solid var(--outline);border-radius:50%;padding:2px;line-height:0;flex:none;background:#020812;box-shadow:0 0 13px rgba(29,142,255,.2)}
  .orb-ring img{display:block;border-radius:50%;object-fit:cover}
  .orb-ring.need{border-color:var(--need);box-shadow:0 0 12px rgba(214,159,53,.24)}
  .orb-ring.working{border-color:var(--work);box-shadow:0 0 14px rgba(37,164,255,.34)}
  .orb-ring.ok{border-color:var(--ok)}
  .orb-ring.failed{border-color:var(--bad)}

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

  /* ---- conversation command center (spec 040) ---- */
  #v-deck{height:calc(100vh - 53px);min-height:610px;overflow:hidden}
  .conversation-shell{height:100%;display:grid;grid-template-columns:274px minmax(500px,1fr) 272px;background:var(--bg)}
  .conversation-sidebar,.command-rail{min-width:0;background:#040d18;display:flex;flex-direction:column}
  .conversation-sidebar{border-right:1px solid var(--line)}
  .mobile-sidebar-head,.mobile-conversation-nav,.mobile-conversation-state,.mobile-sidebar-scrim{display:none}
  .command-rail{border-left:1px solid var(--line);padding:17px 16px;gap:19px;overflow:auto}
  .new-conversation{margin:11px 16px 13px;width:calc(100% - 32px);border:1px solid #176bc0;border-radius:7px;background:rgba(19,100,177,.22);box-shadow:0 0 18px rgba(18,128,239,.18) inset;color:var(--brass2);padding:7px 12px;font-size:12.5px;font-weight:600;cursor:pointer}
  .side-scroll{flex:1;overflow:auto;padding:0 10px 16px}
  .side-section{margin-top:4px}
  .side-heading{display:flex;align-items:center;padding:8px 6px 6px;color:#a7b5c7;font-size:11px;letter-spacing:.05em;text-transform:uppercase}
  .side-heading .count{margin-left:auto;color:var(--faint);font:10.5px var(--mono)}
  .side-row{width:100%;display:flex;align-items:center;gap:8px;border:0;background:transparent;color:var(--mut);padding:6px 8px;border-radius:6px;text-align:left;font-size:12px;cursor:pointer;min-width:0}
  .side-row:hover{background:#0a1a2a;color:var(--fg)}
  .side-row.on{background:#0d2237;color:var(--fg);box-shadow:0 0 0 1px rgba(35,127,213,.18) inset}
  .side-row .side-icon{width:16px;height:16px;border-radius:4px;object-fit:cover;flex:none}
  .side-row .side-copy{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;flex:1}
  .side-row .side-time{font:10px var(--mono);color:var(--faint);white-space:nowrap}
  .side-row .attention-label{font-size:9px;font-weight:700;letter-spacing:.02em;color:var(--need);white-space:nowrap}
  .conversation-status{display:inline-flex;align-items:center;gap:5px;font-size:9px;font-weight:650;letter-spacing:.01em;color:var(--faint);white-space:nowrap}
  .conversation-status i{width:6px;height:6px;border-radius:50%;background:currentColor;flex:none}
  .conversation-status.working{color:var(--work)}
  .conversation-status.paused-review,.conversation-status.paused{color:var(--need)}
  .conversation-status.finished{color:var(--ok)}
  .conversation-status.failed,.conversation-status.canceled{color:var(--bad)}
  .conversation-status.starting{color:var(--queued)}
  .side-thread{padding-left:12px;position:relative}
  .side-thread:before{content:"";position:absolute;left:5px;top:8px;bottom:8px;width:1px;background:var(--outline)}
  .side-footer{border-top:1px solid var(--line);padding:12px 16px;display:flex;align-items:center;gap:9px;color:var(--mut);font-size:12px}
  .side-footer .side-icon{width:18px;height:18px;border-radius:5px;object-fit:cover;flex:none}
  .conversation-main{min-width:0;display:grid;grid-template-columns:minmax(0,1fr);grid-template-rows:50px minmax(0,1fr) auto;background:var(--bg)}
  .conversation-head{display:flex;align-items:center;gap:9px;border-bottom:1px solid var(--line);padding:0 22px}
  .conversation-head-copy{display:flex;align-items:center;gap:9px;min-width:0}
  .conversation-head h1{font-size:14px;font-weight:600;margin:0}
  .conversation-head .conversation-meta{font-size:11px;color:var(--faint)}
  .conversation-head .head-actions{margin-left:auto;display:flex;gap:6px}
  .quiet-action{border:0;background:transparent;color:var(--mut);font-size:12px;padding:5px 7px;cursor:pointer}
  .conversation-feed{overflow:auto;padding:15px 20px 10px;display:flex;flex-direction:column;gap:13px}
  .message-row{display:grid;grid-template-columns:38px minmax(0,1fr);gap:10px;max-width:760px}
  .message-avatar{width:38px;height:38px;border-radius:50%;display:block;filter:drop-shadow(0 0 8px rgba(29,142,255,.24))}
  .message-avatar.human-avatar{display:grid;place-items:center;background:#102235;border:1px solid var(--outline);color:#a9b8c9;filter:none}
  .human-avatar svg{width:22px;height:22px;display:block}
  .message-copy{min-width:0}
  .message-byline{display:flex;align-items:baseline;gap:8px;margin-bottom:2px}
  .message-byline strong{font-size:12.5px;color:var(--fg)}
  .message-byline span{font:10.5px var(--mono);color:var(--faint)}
  .message-body{font-size:12.5px;line-height:1.45;color:var(--body);white-space:pre-wrap}
  .model-badge{font:9.5px var(--mono)!important;padding:2px 7px;border-radius:10px;background:#11253a;color:#899cb2!important}
  .turn-work{margin-left:48px;max-width:760px;border:1px solid #176bc0;border-radius:7px;padding:8px 10px;display:flex;align-items:center;gap:10px;background:rgba(10,31,51,.76)}
  .turn-work img{width:34px;height:34px;border-radius:50%;flex:none}
  .turn-work .copy{min-width:0;flex:1}
  .turn-work .title{font-size:11.5px;font-weight:600;color:var(--brass2)}
  .turn-work .detail{font-size:10.5px;color:var(--mut);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .turn-work button{border:1px solid #176bc0;border-radius:6px;background:transparent;color:var(--brass2);padding:6px 11px;font-size:11px;cursor:pointer}
  .assignment-card{margin-left:48px;max-width:588px;border:1px solid var(--line2);border-radius:8px;background:rgba(5,15,26,.72);padding:10px 12px}
  .assignment-card .assignment-head{display:flex;align-items:center;gap:9px;font-size:12px;color:var(--fg)}
  .assignment-card .assignment-head .state{margin-left:auto;color:var(--work);font-size:10.5px}
  .assignment-card .progress-track{height:3px;background:var(--line);border-radius:3px;margin:8px 0;overflow:hidden}
  .assignment-card .progress-track i{display:block;height:100%;background:var(--work);box-shadow:0 0 8px var(--work)}
  .assignment-meta{display:grid;grid-template-columns:repeat(5,1fr);gap:8px;padding-bottom:7px;border-bottom:1px solid var(--line);font-size:9.5px;color:var(--faint)}
  .assignment-meta strong{display:block;font-size:10.5px;color:var(--body);font-weight:500}
  .checkpoint-list{display:flex;flex-direction:column;gap:4px;padding-top:7px}
  .checkpoint{display:flex;align-items:center;gap:7px;font-size:10.5px;color:var(--mut)}
  .checkpoint i{width:11px;height:11px;border-radius:50%;border:1px solid var(--outline);flex:none}
  .checkpoint.done i{background:var(--ok);border-color:var(--ok);box-shadow:0 0 5px rgba(102,199,145,.35)}
  .checkpoint.current{color:var(--body)}
  .checkpoint.current i{border-color:var(--work);box-shadow:0 0 7px var(--work)}
  .checkpoint time{margin-left:auto;color:var(--faint);font:9.5px var(--mono)}
  .activity-timeline{margin-left:48px;max-width:680px;border:1px solid var(--line2);border-radius:9px;background:rgba(5,15,26,.72);padding:10px 12px}
  .activity-head{display:flex;align-items:center;gap:8px;margin-bottom:8px}
  .activity-head strong{font-size:11.5px;color:var(--fg)}
  .activity-head .conversation-status{margin-left:auto}
  .activity-sub{font-size:9.5px;color:var(--faint)}
  .activity-events{display:flex;flex-direction:column;gap:6px}
  .activity-event{display:grid;grid-template-columns:8px minmax(0,1fr) auto;align-items:start;gap:8px;font-size:10.5px;color:var(--body)}
  .activity-event i{width:6px;height:6px;border-radius:50%;background:var(--outline);margin-top:4px}
  .activity-event.active i{background:var(--work);box-shadow:0 0 7px var(--work);animation:dotpulse 1.6s infinite}
  .activity-event.error{color:var(--bad)}
  .activity-event.error i{background:var(--bad)}
  .activity-event time{font:9.5px var(--mono);color:var(--faint);white-space:nowrap}
  .approval-card{margin-left:48px;max-width:680px;border:1px solid rgba(214,159,53,.55);border-radius:9px;background:var(--tint-need);padding:12px 13px;display:flex;flex-direction:column;gap:8px}
  .approval-card .approval-title{display:flex;align-items:center;gap:7px;color:var(--need);font-size:12px;font-weight:700}
  .approval-card .approval-title i{width:7px;height:7px;border-radius:50%;background:var(--need);box-shadow:0 0 8px rgba(214,159,53,.45)}
  .approval-card .approval-copy{font-size:11.5px;line-height:1.45;color:var(--body)}
  .approval-card .approval-actions{display:flex;gap:8px;flex-wrap:wrap}
  .approval-card .approval-actions button{font-size:11px;padding:6px 10px}
  .conversation-compose{margin:0 16px 15px;border:1px solid var(--line2);border-radius:8px;background:rgba(6,17,29,.88);padding:10px;box-shadow:0 12px 34px rgba(0,0,0,.18)}
  #conversationcomposer{display:block;width:100%;height:36px;min-height:36px;max-height:100px;resize:vertical;border:0;background:transparent;color:var(--fg);font:12px/1.45 var(--font);padding:0 2px;outline:none}
  #conversationcomposer::placeholder{color:var(--faint)}
  .compose-actions{display:flex;align-items:center;gap:7px;padding-top:6px}
  .compose-select{appearance:auto;min-width:0;max-width:178px;border:1px solid var(--outline);border-radius:6px;background:#071421;color:var(--body);padding:5px 25px 5px 8px;font:10.5px var(--font)}
  #composerstatus{font-size:10px;color:var(--mut);min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  #composerstatus.error{color:var(--bad)}
  .compose-spacer{flex:1}
  .compose-button{border:1px solid #176bc0;border-radius:6px;background:transparent;color:var(--brass2);padding:6px 12px;font-size:11px;font-weight:600;cursor:pointer}
  .compose-button.primary{background:#168cff;color:white;box-shadow:0 0 14px rgba(26,143,255,.24)}
  .compose-button:disabled,.compose-select:disabled{opacity:.45;cursor:not-allowed}
  .rail-section-title{font-size:10.5px;letter-spacing:.05em;color:#a1afc1;text-transform:uppercase}
  .rail-card{border:1px solid var(--line);border-radius:7px;background:rgba(8,23,38,.72);padding:10px 11px;display:flex;align-items:center;gap:9px}
  .rail-card + .rail-card{margin-top:7px}
  .rail-card img{width:36px;height:36px;border-radius:50%;filter:drop-shadow(0 0 8px rgba(29,142,255,.22));flex:none}
  .rail-card .rail-copy{min-width:0;flex:1}
  .rail-card .rail-name{font-size:12px;font-weight:600;color:var(--fg);display:flex;gap:6px;align-items:center}
  .rail-card .rail-model{font:9px var(--mono);color:var(--faint);padding:2px 5px;background:#102236;border-radius:8px}
  .rail-card .rail-detail{font-size:10px;color:var(--mut);margin-top:2px;line-height:1.35}
  .rail-status{font-size:9.5px;color:var(--ok);white-space:nowrap}
  .rail-status i,.system-status i{display:inline-block;width:6px;height:6px;border-radius:50%;background:var(--ok);margin-right:5px}
  .rail-status.working{color:var(--brass2)}
  .rail-status.working i{background:var(--brass2);box-shadow:0 0 8px rgba(45,159,255,.65)}
  .rail-status.need{color:var(--amber)}
  .rail-status.need i{background:var(--amber)}
  .machine-card img{width:26px;height:26px;border-radius:6px}
  .machine-card.down{opacity:.62}
  .system-status{margin-top:auto;color:var(--mut);font-size:10px;padding-top:10px}

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
  .toggle{display:flex;align-items:center;gap:10px;border:0;background:none;padding:0;text-align:left;font:13.5px var(--font);color:var(--body);cursor:pointer;user-select:none}
  .toggle[hidden]{display:none}
  .track{width:34px;height:20px;border-radius:10px;background:var(--brass);position:relative;display:inline-block;flex:none;transition:background .15s}
  .track i{position:absolute;right:2px;top:2px;width:16px;height:16px;border-radius:50%;background:var(--bg);transition:right .15s}
  .toggle.off .track{background:var(--outline)}
  .toggle.off .track i{right:16px}
  .handoff{font-size:14.5px;padding:11px 22px;border-radius:9px;align-self:flex-start}
  .handoff:disabled{cursor:wait;opacity:.72;box-shadow:none}
  .handoff-status{font-size:12.5px;line-height:1.45;color:var(--work);min-height:18px}
  .handoff-status.fail{color:var(--bad)}
  .handoff-status[hidden]{display:none}
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

  /* ---- playbooks + route preview (Turn 4) ---- */
  .mode-switch{display:inline-flex;align-self:flex-start;padding:3px;background:var(--panel);border:1px solid var(--line);border-radius:9px}
  .mode-switch button{border:0;background:transparent;color:var(--mut);font-size:13px;padding:6px 12px;border-radius:6px;cursor:pointer}
  .mode-switch button.on{background:var(--raise);color:var(--fg);font-weight:600}
  .routecard{background:var(--tint-work);border:1px solid var(--raise);border-radius:10px;padding:13px 15px;display:flex;flex-direction:column;gap:9px}
  .routecard .hd{display:flex;align-items:center;gap:8px}
  .routecard .lb{font-size:12px;font-weight:600;letter-spacing:.08em;text-transform:uppercase;color:var(--work)}
  .routecard .nm{font-size:13.5px;font-weight:600}
  .routecard .change{margin-left:auto;border:0;background:none;padding:0;color:var(--brass2);font-size:12.5px;cursor:pointer}
  .routechain{display:flex;align-items:center;gap:6px;flex-wrap:wrap;font-size:12.5px}
  .routechip{padding:4px 10px;border-radius:14px;background:var(--line);color:var(--body)}
  .routechip .model{font:11px var(--mono);color:var(--mut)}
  .routearrow{color:var(--sched)}
  .routenote{font-size:12px;color:var(--mut)}
  .routepicker{display:flex;gap:7px;flex-wrap:wrap;padding:9px;background:var(--panel);border:1px solid var(--line);border-radius:9px}
  .routepicker[hidden]{display:none}
  .routepicker button{background:transparent;border:1px solid var(--outline);border-radius:16px;color:var(--body);padding:5px 10px;font-size:12.5px;cursor:pointer}
  .routepicker button.on{border-color:var(--brass);background:var(--tint-brass);color:var(--brass2);font-weight:600}
  .quickanswer{background:var(--panel);border:1px solid var(--line);border-radius:10px;padding:13px 15px}
  .quickanswer.fail{border-color:var(--bad)}
  .quickanswer.fail .answerhead{color:var(--bad)}
  .quickanswer[hidden]{display:none}
  .quickanswer .answerhead{font-size:11.5px;font-weight:600;letter-spacing:.08em;text-transform:uppercase;color:var(--work);margin-bottom:6px}

  .playbook-layout{display:flex;min-height:520px}
  .playbook-rail{flex:none;width:250px;border-right:1px solid var(--line);padding:16px 14px;display:flex;flex-direction:column;gap:8px}
  .pbitem{background:var(--panel);border:1px solid var(--line);border-radius:10px;padding:12px 14px;display:flex;flex-direction:column;gap:4px;text-align:left;color:var(--fg);cursor:pointer}
  .pbitem:hover{border-color:var(--raise)}
  .pbitem.on{border-color:var(--raise);border-left:3px solid var(--brass)}
  .pbitem .name{display:flex;align-items:center;gap:8px;font-size:14px;font-weight:600}
  .pbitem .default{font-size:10.5px;padding:2px 7px;border-radius:10px;background:var(--tint-brass);color:var(--brass2)}
  .pbitem .meta{font-size:12px;color:var(--mut)}
  .playbook-main{flex:1;padding:20px 22px;display:flex;flex-direction:column;gap:16px;min-width:0}
  .pbeditor-head{display:flex;align-items:baseline;gap:12px;flex-wrap:wrap}
  .pbeditor-head .title{font-size:17px;font-weight:600}
  .pbeditor-head .trigger{font-size:12.5px;color:var(--mut)}
  .pbeditor-head .gate-label{margin-left:auto}
  .pbeditor-head .edit{border:0;background:none;padding:0;color:var(--brass2);cursor:pointer;font-size:12.5px}
  .pbeditor-head .revision{margin-left:auto;font:11px var(--mono);color:var(--faint)}
  .pipeline{display:flex;align-items:stretch;gap:0;overflow-x:auto;padding-bottom:4px}
  .stagecard{flex:1;min-width:220px;background:var(--panel);border:1px solid var(--raise);border-radius:10px;padding:14px 16px;display:flex;flex-direction:column;gap:9px}
  .stagecard .hd{display:flex;align-items:center;gap:8px}
  .stagecard .num{font:600 11px var(--mono);color:var(--faint)}
  .stagecard .name{font-size:14px;font-weight:600}
  .stagecard .badge{margin-left:auto;border:0;font:600 10.5px var(--font);padding:2px 8px;border-radius:10px;background:var(--line);color:var(--mut)}
  .stagecard button.badge{cursor:pointer}
  .stagecard .badge.memory{background:var(--tint-work);color:var(--work)}
  .stagecard .assignment{display:flex;align-items:center;gap:8px;flex-wrap:wrap}
  .stagecard .tasktype{font-size:11.5px;color:var(--faint);width:62px;flex:none}
  .agentselect,.modelselect{appearance:auto;max-width:150px}
  .agentselect{font-size:13.5px;font-weight:600;padding:5px 9px;border-radius:16px;background:transparent;color:var(--fg)}
  .modelselect{font:11.5px var(--mono);padding:4px 7px;border:0;border-radius:6px;background:var(--line);color:var(--mut)}
  .stagecard .desc{font-size:12px;color:var(--mut);line-height:1.45}
  .pipearrow,.stageadd{flex:none;align-self:center;color:var(--sched);font-size:16px;padding:0 10px}
  .stageadd{border:0;background:none;cursor:pointer;color:var(--brass2)}
  .shortcuts{display:flex;flex-direction:column;gap:9px}
  .shortcutrow{display:flex;align-items:center;gap:12px;background:var(--panel);border:1px solid var(--line);border-radius:10px;padding:12px 15px}
  .shortcutrow .copy{display:flex;flex-direction:column;gap:2px;min-width:0}
  .shortcutrow .title{font-size:13.5px;font-weight:600}
  .shortcutrow .summary{font-size:12px;color:var(--mut)}
  .switchbtn{margin-left:auto;flex:none;width:34px;height:20px;border:0;border-radius:10px;background:var(--outline);position:relative;cursor:pointer;padding:0}
  .switchbtn i{position:absolute;left:2px;top:2px;width:16px;height:16px;border-radius:50%;background:var(--bg);transition:left .15s}
  .switchbtn.on{background:var(--brass)}
  .switchbtn.on i{left:16px}

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
  .blk-active{background:var(--work);color:var(--on-blue);font-weight:600}
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
  @media (max-width:900px){
    header{flex-wrap:wrap;gap:10px;padding:12px 14px}
    nav{order:3;width:100%;overflow-x:auto;padding-bottom:2px}
    nav button{white-space:nowrap}
    #machines{display:none!important}
    .deck,.assign,.playbook-layout{flex-direction:column}
    .deck-left,.assign-left{border-right:0;border-bottom:1px solid var(--line)}
    .perfgrid{grid-template-columns:1fr;padding:14px}
    .deck-left,.deck-right,.assign-left,.assign-right,.playbook-main{padding:16px}
    .playbook-rail{width:auto;border-right:0;border-bottom:1px solid var(--line);display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr))}
    .pipeline{gap:8px}
    .pipearrow{display:none}
    .subhead{padding:12px 14px;flex-wrap:wrap}
    .legend{width:100%;overflow-x:auto}
    #v-deck{height:calc(100vh - 102px)}
    .conversation-shell{grid-template-columns:220px minmax(0,1fr)}
    .command-rail{display:none}
    .assignment-meta{grid-template-columns:repeat(3,1fr)}
  }
  @media (max-width:700px){
    html,body{max-width:100%;overflow-x:hidden}
    body[data-view=deck]{height:100dvh;overflow:hidden;display:flex;flex-direction:column}
    body[data-view=deck] header{flex:none}
    nav{overscroll-behavior-x:contain;scroll-snap-type:x proximity}
    nav button{min-height:44px;flex:none;scroll-snap-align:start}
    .needpill{min-height:44px}
    #v-deck{height:auto;min-height:0;overflow:hidden;flex:1}
    .conversation-shell{display:block;height:100%;min-height:0;overflow:hidden}
    .conversation-sidebar{position:fixed;inset:0 auto 0 0;z-index:70;width:min(88vw,340px);max-width:100%;border-right:1px solid var(--raise);border-bottom:0;padding:0 0 max(10px,env(safe-area-inset-bottom));transform:translateX(-105%);transition:transform .18s ease;visibility:hidden}
    .conversation-sidebar.mobile-open{transform:translateX(0);visibility:visible}
    .mobile-sidebar-head{min-height:56px;padding:max(8px,env(safe-area-inset-top)) 12px 8px 16px;border-bottom:1px solid var(--line);display:flex;align-items:center;gap:10px;font-size:14px}
    .mobile-sidebar-head .quiet-action{margin-left:auto;min-height:44px;padding:8px 10px;color:var(--brass2)}
    .conversation-sidebar .new-conversation{display:block;margin:10px 12px;width:calc(100% - 24px);min-height:44px;font-size:14px}
    .conversation-sidebar .side-scroll{display:block;flex:1;min-height:0;overflow:auto;padding:0 8px 12px;-webkit-overflow-scrolling:touch}
    .conversation-sidebar .side-footer{display:flex;padding:12px}
    .conversation-sidebar .side-row{min-height:44px;font-size:13px}
    .mobile-sidebar-scrim{display:block;position:fixed;inset:0;z-index:65;width:100%;height:100%;border:0;background:rgba(0,4,10,.72);padding:0}
    .mobile-sidebar-scrim[hidden]{display:none}
    .conversation-main{height:100%;min-height:0;width:100%;max-width:100vw;grid-template-rows:56px minmax(0,1fr) auto}
    .conversation-head{display:grid;grid-template-columns:auto minmax(0,1fr) auto;gap:7px;padding:0 9px}
    .mobile-conversation-nav{display:inline-flex;align-items:center;justify-content:center;min-height:44px;border:1px solid var(--outline);border-radius:7px;background:transparent;color:var(--brass2);padding:7px 8px;font-size:11px;font-weight:600;cursor:pointer}
    .conversation-head-copy{display:flex;flex-direction:column;align-items:flex-start;gap:1px;overflow:hidden}
    .conversation-head h1{width:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
    .conversation-head .conversation-meta{display:none}
    .mobile-conversation-state{display:block;line-height:1}
    .mobile-conversation-state .conversation-status{font-size:9.5px}
    .conversation-head .head-actions{margin-left:0}
    .conversation-head .quiet-action{min-height:44px;padding:7px 5px}
    .conversation-feed{min-width:0;overflow:auto;overscroll-behavior:contain;-webkit-overflow-scrolling:touch;padding:14px 11px 10px}
    .message-row{grid-template-columns:32px minmax(0,1fr)}
    .message-avatar{width:32px;height:32px}
    .message-body,.approval-copy,.activity-event span,.assignment-meta strong{overflow-wrap:anywhere}
    .turn-work,.assignment-card,.activity-timeline{margin-left:0;max-width:none}
    .turn-work{align-items:flex-start;flex-wrap:wrap;padding:10px}
    .turn-work .copy{min-width:calc(100% - 44px)}
    .turn-work .detail{white-space:normal}
    .turn-work button{width:100%;min-height:44px}
    .approval-card{margin-left:0;max-width:none;padding:14px 12px}
    .approval-card .approval-title{font-size:13px}
    .approval-card .approval-copy{font-size:13px}
    .approval-card .approval-actions{display:grid;grid-template-columns:1fr;gap:8px}
    .approval-card .approval-actions button{min-height:44px;width:100%}
    .notebox textarea{font-size:16px}
    .notebox button{min-height:44px;width:100%}
    .assignment-meta{grid-template-columns:repeat(2,minmax(0,1fr))}
    .conversation-compose{margin:0 8px max(8px,env(safe-area-inset-bottom));padding:9px}
    #conversationcomposer{height:48px;min-height:48px;font-size:16px}
    .compose-actions{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:7px}
    .compose-select{font-size:16px;padding:7px 25px 7px 8px;min-height:44px;max-width:none;width:100%}
    #composermachine{grid-column:1/-1}
    #composerstatus{grid-column:1/-1;order:initial;width:auto;min-height:18px;white-space:normal}
    .compose-spacer{display:none}
    .compose-button{font-size:13px;min-height:44px;width:100%}
    .compose-chip{display:none}
  }
  @media (max-width:600px){
    header{gap:8px}
    header .grow{display:none}
    .wordmark{flex:none;white-space:nowrap}
    #theme{display:none}
    nav{scrollbar-width:none}
    nav::-webkit-scrollbar{display:none}
    .pipeline{flex-direction:column;overflow-x:visible}
    .stagecard{min-width:0}
    .stageadd{align-self:flex-start;padding:2px 0}
    .pbeditor-head .revision{margin-left:0}
    .pbeditor-head .gate-label{margin-left:0}
    .shortcutrow{align-items:flex-start}
  }
</style>
</head>
<body>
<header>
  <img class="brand-icon fort-orb" id="brandorb" src="/fort-agent-orb.png" alt=""/>
  <span class="wordmark">FORT</span>
  <nav id="nav" aria-label="views">
    <button data-v="deck">Deck</button>
    <button data-v="assign">Assign</button>
    <button data-v="perf">Performance</button>
    <button data-v="week">Week</button>
    <button data-v="today">Today</button>
    <button data-v="playbooks">Playbooks</button>
  </nav>
  <button class="needpill" id="needpill" hidden></button>
  <span class="grow"></span>
  <span id="machines" style="display:flex;gap:14px"></span>
  <span class="plane" id="plane" hidden>control only</span>
  <button class="iconbtn" id="theme" title="toggle theme" aria-label="toggle light/dark theme">◐</button>
</header>

<section class="view" id="v-deck">
  <div class="conversation-shell" data-desktop-command-center>
    <aside class="conversation-sidebar" id="conversationnav" aria-label="conversations">
      <div class="mobile-sidebar-head"><strong>Conversations</strong><button class="quiet-action" id="closeconversationnav" type="button">Close</button></div>
      <button class="new-conversation" id="newconversation">New conversation</button>
      <div class="side-scroll">
        <div class="side-section"><div class="side-heading">Inbox</div><div id="conversationinbox"></div></div>
        <div class="side-section"><div class="side-heading">Conversations <span class="count">Recent</span></div><div id="conversationlist"></div></div>
      </div>
      <div class="side-footer"><img class="side-icon" src="/fort-agent-orb.png" alt=""/> Fort verifies every handoff</div>
    </aside>
    <button class="mobile-sidebar-scrim" id="conversationnavscrim" type="button" aria-label="Close conversations" hidden></button>
    <main class="conversation-main">
      <div class="conversation-head"><button class="mobile-conversation-nav" id="mobileconversationnav" type="button" aria-controls="conversationnav" aria-expanded="false">Conversations</button><div class="conversation-head-copy"><h1 id="conversationtitle">Conversation</h1><span class="conversation-meta" id="conversationmeta"></span><span class="mobile-conversation-state" id="mobileconversationstate"></span></div><div class="head-actions"><button class="quiet-action" id="conversationdetail">Activity</button></div></div>
      <div class="conversation-feed" id="conversationfeed"></div>
      <div class="conversation-compose">
        <textarea id="conversationcomposer" placeholder="Message the current agent…" aria-label="message current agent"></textarea>
        <div class="compose-actions"><select class="compose-select" id="composeragent" aria-label="Agent"></select><select class="compose-select" id="composerprofile" aria-label="Model"></select><select class="compose-select" id="composermachine" aria-label="Machine"></select><span id="composerstatus"></span><span class="compose-spacer"></span><button class="compose-button" id="conversationassign">Assign</button><button class="compose-button primary" id="conversationsend">Send</button></div>
      </div>
    </main>
    <aside class="command-rail" aria-label="agents and machines">
      <div><div class="rail-section-title">Current agent</div><div id="currentagent" style="margin-top:10px"></div></div>
      <div><div class="rail-section-title">Other agents</div><div id="otheragents" style="margin-top:10px"></div></div>
      <div><div class="rail-section-title">Machines</div><div id="machinerail" style="margin-top:10px"></div></div>
      <div class="system-status"><i></i>All systems operational</div>
    </aside>
  </div>
</section>

<section class="view" id="v-assign" hidden>
  <div class="subhead"><span class="c" id="crewsum"></span></div>
  <div class="assign">
    <div class="assign-left">
      <h2>Assign work</h2>
      <div class="mode-switch" role="tablist" aria-label="direction type">
        <button id="modeassignment" class="on" role="tab" aria-selected="true">Assignment</button>
        <button id="modequick" role="tab" aria-selected="false">Quick question</button>
      </div>
      <textarea id="brief" placeholder="Describe the outcome you want — like briefing an employee."></textarea>
      <div id="briefpv" class="mdbody preview" hidden></div>
      <div id="routepreview" class="routecard" hidden></div>
      <div id="routepicker" class="routepicker" hidden></div>
      <button type="button" class="toggle" id="plantoggle" aria-pressed="true"><span class="track"><i></i></span>Propose a plan first — I&#39;ll sign off before work starts</button>
      <button class="btn btn-brass handoff" id="handoff">Hand it off</button>
      <div class="handoff-status" id="handoffstatus" role="status" aria-live="polite" hidden></div>
      <button class="orlink" id="tobacklog">or add to Up next ›</button>
      <div id="quickanswer" class="quickanswer" hidden></div>
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

<section class="view" id="v-playbooks" hidden>
  <div class="subhead">
    <span class="t">Playbooks</span><span class="c">who does what, with which model</span><span class="grow"></span>
    <button class="btn btn-brassline" id="newplaybook">＋ New playbook</button>
  </div>
  <div class="playbook-layout">
    <aside class="playbook-rail" id="playbooklist" aria-label="playbooks"></aside>
    <main class="playbook-main" id="playbookeditor"></main>
  </div>
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
let model={sum:null,machines:[],profiles:[],runs:[],gates:[],backlog:[],metrics:null,playbooks:[]};
let agentOfRun={};       // flow run id -> agent of its latest started event
let actByRun={};         // live activity buffers (spec 030)
const ACT_MAX=20;
let dwRun=null, dwNode=null, dwNodes=[], dwEvents=[];
let assignCtx=null;      // {backlogId} when assigning an existing brief
let assignMode='assignment';
let planFirst=true;
let routePreview=null, routeChoice=null, routePickerOpen=false, routeTimer=null, routeSerial=0;
let handoffPending=false;
let quickAnswer='',quickAnswerError='';
let playbooksLoaded=false,playbooksLoading=false;
let selectedPlaybook=localStorage.getItem('fort-playbook')||'';
let curView=localStorage.getItem('fort-view')||'deck';
if(curView==='projects')curView='deck';
let perfLane='';
let selectedConversation=localStorage.getItem('fort-conversation')||'';
let conversationDetails={},conversationLoading={};
let composerAgent='',composerProfile='',composerMachine='',composerSelectionRun='';
let composingNewConversation=false,conversationSending=false;

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

// ---- theme ----
$('#theme').addEventListener('click',function(){
  var cur=document.documentElement.getAttribute('data-theme')==='light'?'dark':'light';
  document.documentElement.setAttribute('data-theme',cur);
  localStorage.setItem('fort-theme',cur);
});

// ---- view router ----
function setMobileConversationNav(open){
  var panel=$('#conversationnav'),trigger=$('#mobileconversationnav'),scrim=$('#conversationnavscrim');
  panel.classList.toggle('mobile-open',!!open);
  trigger.setAttribute('aria-expanded',open?'true':'false');
  scrim.hidden=!open;
}
function showView(v){
  curView=v; localStorage.setItem('fort-view',v);
  document.body.dataset.view=v;
  if(v!=='deck')setMobileConversationNav(false);
  document.querySelectorAll('.view').forEach(function(s){s.hidden=('v-'+v!==s.id);});
  document.querySelectorAll('#nav button').forEach(function(b){b.classList.toggle('on',b.dataset.v===v);});
  if(v==='perf')fetchMetrics();
  if(v==='playbooks'&&!playbooksLoaded)fetchPlaybooks();
  render();
}
document.querySelectorAll('#nav button').forEach(function(b){b.addEventListener('click',function(){if(b.dataset.v==='assign')beginDirection();else showView(b.dataset.v);});});
function beginDirection(){
  assignCtx=null;assignMode='assignment';planFirst=true;routeChoice=null;routePreview=null;routePickerOpen=false;quickAnswer='';quickAnswerError='';
  showView('assign');$('#brief').focus();
}
function beginConversation(){
  setMobileConversationNav(false);
  selectedConversation='';localStorage.removeItem('fort-conversation');
  composingNewConversation=true;
  composerSelectionRun='';composerAgent='';composerProfile='';composerMachine='';
  $('#conversationcomposer').value='';$('#composerstatus').textContent='';
  showView('deck');renderDeck();$('#conversationcomposer').focus();
}
$('#newconversation').addEventListener('click',beginConversation);
$('#mobileconversationnav').addEventListener('click',function(){setMobileConversationNav(!$('#conversationnav').classList.contains('mobile-open'));});
$('#closeconversationnav').addEventListener('click',function(){setMobileConversationNav(false);$('#mobileconversationnav').focus();});
$('#conversationnavscrim').addEventListener('click',function(){setMobileConversationNav(false);});
$('#needpill').addEventListener('click',openNeedsYou);
$('#conversationdetail').addEventListener('click',function(){if(selectedConversation)openDrawer(selectedConversation);});
$('#conversationassign').addEventListener('click',assignConversation);
$('#conversationsend').addEventListener('click',sendConversation);
$('#conversationcomposer').addEventListener('keydown',function(e){if((e.metaKey||e.ctrlKey)&&e.key==='Enter')sendConversation();});
$('#composeragent').addEventListener('change',function(){
  composerAgent=this.value;var choice=defaultProfileForAgent(composerAgent);
  composerProfile=choice?choice.id:'';composerMachine='';renderComposerControls(selectedConversation&&runByID(selectedConversation));
});
$('#composerprofile').addEventListener('change',function(){
  composerProfile=this.value;var p=profileByID(composerProfile);if(p)composerAgent=p.agent;composerMachine='';renderComposerControls(selectedConversation&&runByID(selectedConversation));
});
$('#composermachine').addEventListener('change',function(){composerMachine=this.value;});
$('#modeassignment').addEventListener('click',function(){setAssignMode('assignment');});
$('#modequick').addEventListener('click',function(){setAssignMode('quick');});
$('#newplaybook').addEventListener('click',duplicatePlaybook);

// ---- SSE + activity buffers (spec 030) ----
const seenActivityEvents={};
const trackedActivityKinds={ingress:1,placement:1,started:1,stdout:1,stderr:1,message:1,tool:1,subagent:1,gate:1,error:1,exited:1,transform:1};
const workEvidenceKinds={started:1,stdout:1,stderr:1,message:1,tool:1,subagent:1};
function trackEvent(e){
  if(!e||!e.run_id)return;
  if(e.type==='started'&&e.data&&e.data.indexOf('{')!==0)agentOfRun[e.run_id]=e.data;
  if(!trackedActivityKinds[e.type])return;
  var key=e.id!==undefined&&e.id!==null?'id:'+e.id:'event:'+e.run_id+':'+e.type+':'+(e.time||'')+':'+(e.data||'');
  if(seenActivityEvents[key])return;
  seenActivityEvents[key]=1;
  const buf=actByRun[e.run_id]||(actByRun[e.run_id]=[]);
  buf.push(e);
  buf.sort(function(a,b){return ((Date.parse(a.time)||0)-(Date.parse(b.time)||0))||((a.id||0)-(b.id||0));});
  if(buf.length>ACT_MAX)buf.splice(0,buf.length-ACT_MAX);
}
function eventPayload(e){
  if(!e||!e.data)return {};
  try{return JSON.parse(e.data)}catch(err){return {};}
}
function activityDescription(e){
  if(!e)return '';
  var data=eventPayload(e),line=String(e.data||'').split('\n')[0];
  if(e.type==='ingress')return 'Accepted by Fort';
  if(e.type==='placement')return 'Placed '+dispName(data.agent||'agent')+(data.machine?' on '+data.machine:'');
  if(e.type==='started')return dispName(line||'agent')+' process started';
  if(e.type==='stdout'){
    if(data.type==='thread.started')return 'Provider session opened';
    if(data.type==='turn.started')return 'Model turn started';
    if(data.type==='turn.completed')return 'Model turn completed';
    return '';
  }
  if(e.type==='stderr'){
    if(line.length>220)line=line.slice(0,219)+'…';
    return line?'Provider reported — '+line:'Provider reported diagnostic output';
  }
  if(e.type==='tool')return 'Using '+(data.name||'tool')+(data.summary?' — '+data.summary:'');
  if(e.type==='subagent')return 'Helper started'+(data.description?' — '+data.description:'');
  if(e.type==='message'){
    if(line.length>110)line=line.slice(0,109)+'…';
    return line?'Response produced — '+line:'Response produced';
  }
  if(e.type==='gate')return data.decision==='approved'?'Checkpoint approved':data.decision==='rejected'?'Changes requested':'Checkpoint updated';
  if(e.type==='error'){
    if(line.length>220)line=line.slice(0,219)+'…';
    return line||'Fort reported an execution error';
  }
  if(e.type==='exited')return e.code&&e.code!==0?'Provider process exited with code '+e.code:'Provider process completed';
  return '';
}
function activityLine(e){
  var text=activityDescription(e);
  return text?'<div class="a-'+esc(e.type)+'">'+esc(text)+'</div>':'';
}
function latestEventMillis(runID){
  var buf=actByRun[runID]||[],latest=0;
  buf.forEach(function(e){latest=Math.max(latest,Date.parse(e.time)||0);});
  return latest;
}
function hasWorkEvidence(runID){
  return (actByRun[runID]||[]).some(function(e){return !!workEvidenceKinds[e.type];});
}
function latestActivityText(runID){
  var buf=actByRun[runID];
  if(!buf||!buf.length)return '';
  for(var i=buf.length-1;i>=0;i--){
    var text=activityDescription(buf[i]);
    if(text)return text;
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
    model.profiles=await fetchJSON('/api/profiles')||[];
    const b=await fetchJSON('/api/board');
    model.runs=b.runs||[]; model.gates=b.gates||[];
    model.backlog=await fetchJSON('/api/backlog')||[];
  }catch(err){return;}
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
async function fetchPlaybooks(){
  if(playbooksLoading)return;
  playbooksLoading=true;
  try{
    var payload=await fetchJSON('/api/playbooks');
    model.playbooks=Array.isArray(payload)?payload:((payload&&payload.playbooks)||[]);
    playbooksLoaded=true;
    if(!selectedPlaybook||!playbookByID(selectedPlaybook)){
      var d=model.playbooks.find(function(p){return p.is_default;})||model.playbooks[0];
      selectedPlaybook=d?d.id:'';
    }
  }catch(err){playbooksLoaded=true;model.playbooks=[];}
  playbooksLoading=false;
  if(curView==='playbooks')renderPlaybooks();
  if(curView==='assign'){renderAssignControls();renderRoutePreview();}
}

// ---- derived model ----
function runByID(id){for(var i=0;i<model.runs.length;i++)if(model.runs[i].id===id)return model.runs[i];return null;}
function gatesFor(runID){return model.gates.filter(function(g){return g.run_id===runID;});}
function hasGate(runID){return gatesFor(runID).length>0;}
function failureIsRecent(r){
  var status=(r.status||'').toLowerCase();
  var cut=Date.now()-48*3600*1000;
  return !hasGate(r.id)&&(status==='failed'||status==='error')&&Date.parse(r.updated_at||r.created_at)>=cut;
}
function recentFailed(){
  return model.runs.filter(failureIsRecent);
}
function needCount(){return model.gates.length+recentFailed().length;}
function agentSet(){
  var s={};
  model.machines.forEach(function(m){(m.agents||[]).forEach(function(a){s[a]=1;});});
  (model.profiles||[]).forEach(function(p){if(p.agent)s[p.agent]=1;});
  model.runs.forEach(function(r){var a=runAgent(r);if(a)s[a]=1;});
  if(model.metrics)(model.metrics.agents||[]).forEach(function(a){s[a.agent]=1;});
  model.playbooks.forEach(function(p){(p.stages||[]).forEach(function(st){(st.assignments||[]).forEach(function(a){if(a.agent)s[a.agent]=1;});});});
  return Object.keys(s).sort();
}
function agentModel(agent){
  var found='';
  model.playbooks.some(function(p){
    return (p.stages||[]).some(function(st){
      return (st.assignments||[]).some(function(a){
        if(a.agent!==agent)return false;
        found=a.model||'';return true;
      });
    });
  });
  return found;
}
function agentStatus(a){
  var waiting=null,working=null;
  model.runs.forEach(function(r){
    if(runAgent(r)!==a)return;
    if(r.status==='blocked'&&gatesFor(r.id).length)waiting=waiting||r;
    if(runState(r)==='working')working=working||r;
  });
  if(waiting)return {state:'need',run:waiting};
  if(working)return {state:'working',run:working};
  return {state:'idle',run:null};
}
function runState(r){
  if(hasGate(r.id))return 'paused-review';
  var status=String(r.status||'').toLowerCase();
  if(status==='succeeded'||status==='done'||status==='failed'||status==='error'||status==='canceled'||status==='cancelled')return 'terminal';
  if(status==='blocked'||status==='paused')return 'paused';
  if(hasWorkEvidence(r.id))return 'working';
  if(status==='running'||status==='queued')return 'starting';
  return 'idle';
}
function runStatusLabel(r){
  var state=runState(r),status=String(r.status||'').toLowerCase();
  if(state==='paused-review')return 'Needs approval';
  if(state==='working')return 'Working';
  if(state==='starting')return 'Starting';
  if(state==='paused')return 'Paused';
  if(status==='succeeded'||status==='done')return 'Finished';
  if(status==='failed'||status==='error')return 'Failed';
  if(status==='canceled'||status==='cancelled')return 'Canceled';
  return 'Ready';
}
function gateActivityMillis(runID){
  var latest=0;
  gatesFor(runID).forEach(function(g){latest=Math.max(latest,Date.parse(g.since)||0);});
  return latest;
}
function conversationRecency(r){
  return Math.max(latestEventMillis(r.id),gateActivityMillis(r.id),Date.parse(r.updated_at)||0,Date.parse(r.created_at)||0);
}
function conversationRuns(){
  var out=model.runs.slice();
  out.sort(function(a,b){return (conversationRecency(b)-conversationRecency(a))||String(a.id).localeCompare(String(b.id));});
  return out;
}
function profileByID(id){
  var found=null;
  (model.profiles||[]).some(function(p){if(p.id===id){found=p;return true;}return false;});
  return found;
}
function profileSelectable(p){return p&&p.state!=='unavailable'&&p.state!=='setup_required';}
function defaultProfileForAgent(agent){
  var choices=(model.profiles||[]).filter(function(p){return p.agent===agent&&profileSelectable(p);});
  return choices.find(function(p){return p.id===agent+':configured-default'&&p.state==='ready';})||
    choices.find(function(p){return p.state==='ready';})||
    choices.find(function(p){return p.id===agent+':configured-default';})||choices[0]||null;
}
function ckCaption(r){
  var c=r.checkpoints;
  var state=runState(r),latest=latestEventMillis(r.id);
  if(state==='starting')return r.status==='queued'?'Queued · waiting to start':'Starting · waiting for the first provider event';
  if(!c||!c.total){
    if(state==='working')return 'Working · last activity '+ago(new Date(latest).toISOString());
    if(r.status==='succeeded')return 'Finished '+ago(r.updated_at);
    if(r.status==='failed')return 'Stopped — needs direction';
    return 'Direct assignment — no checkpoints';
  }
  if(c.waiting>0)return c.accepted+' of '+c.total+' checkpoints accepted · '+c.waiting+' awaiting sign-off';
  if(state==='working')return c.accepted+' of '+c.total+' accepted · activity '+ago(new Date(latest).toISOString());
  if(state==='starting')return c.accepted+' of '+c.total+' accepted · waiting for provider activity';
  if(r.status==='succeeded'&&c.rejected>0)return 'Closed after your redirect';
  if(c.accepted===c.total)return 'All '+c.total+' checkpoints accepted';
  if(r.status==='failed')return c.accepted+' of '+c.total+' accepted · stopped';
  return c.accepted+' of '+c.total+' checkpoints accepted';
}
function activitySentence(r){
  var live=latestActivityText(r.id);
  if(live)return live;
  var a=dispName(runAgent(r));
  switch(runState(r)){
    case 'paused-review':return a+' is paused for your review.';
    case 'paused':return a+' is paused and needs direction.';
    case 'starting':return 'Fort accepted the turn; no provider activity has arrived yet.';
    case 'working':return 'Recorded provider activity '+ago(new Date(latestEventMillis(r.id)).toISOString())+'.';
    case 'terminal':return r.status==='succeeded'?'Finished '+ago(r.updated_at)+'.':'Stopped '+ago(r.updated_at)+' — open to see what happened.';
  }
  return '';
}
function isThinking(r){return !!r&&runState(r)==='working';}
function orbClass(thinking){return 'fort-orb'+(thinking?' is-thinking':'');}
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
  if(curView==='assign')renderAssign();
  if(curView==='perf')renderPerf();
  if(curView==='week')renderWeek();
  if(curView==='today')renderToday();
  if(curView==='playbooks')renderPlaybooks();
}
function renderHeader(){
  var n=needCount();
  $('#brandorb').classList.toggle('is-thinking',model.runs.some(isThinking));
  $('#needpill').hidden=n===0;
  $('#needpill').textContent=n+' need'+(n===1?'s':'')+' you';
  $('#machines').innerHTML=model.machines.map(function(m){
    return '<span class="mdot'+(m.reachable?'':' down')+'" title="'+esc((m.agents||[]).join(', '))+'"><i></i>'+esc(m.name)+'</span>';
  }).join('');
}

// ---- Deck (1a) ----
let openNote=null; // gate key with an open note editor — skip re-render while typing
function runAttentionHTML(r){
  if(hasGate(r.id))return '<span class="attention-label">Needs approval</span>';
  if(failureIsRecent(r)||r.status==='blocked')return '<span class="attention-label">Needs direction</span>';
  return '';
}
function conversationStatusHTML(r){
  var state=runState(r),cls=state;
  if(state==='terminal')cls=(r.status==='succeeded'||r.status==='done')?'finished':(r.status==='canceled'||r.status==='cancelled')?'canceled':'failed';
  return '<span class="conversation-status '+esc(cls)+'"><i></i>'+esc(runStatusLabel(r))+'</span>';
}
function openNeedsYou(){
  var target=model.gates.length?model.gates[0].run_id:((recentFailed()[0]||{}).id||'');
  composingNewConversation=false;
  showView('deck');
  if(target)selectConversation(target);
}
function profileStateText(profile){
  if(!profile)return 'Choose an exact model profile.';
  var state=String(profile.state||'unknown').replace(/_/g,' ');
  var reason=String(profile.reason||'').replace(/_/g,' ');
  if(profile.state==='ready')return (profile.machines||[]).length?'Ready on '+profile.machines.length+' machine'+(profile.machines.length===1?'':'s'):'Ready';
  return state.charAt(0).toUpperCase()+state.slice(1)+(reason?' · '+reason:'');
}
function renderComposerControls(active){
  var selectionKey=active?active.id:'@new';
  if(composerSelectionRun!==selectionKey){
    composerSelectionRun=selectionKey;
    composerAgent=active?runAgent(active):'';
    composerProfile=active&&active.profile?active.profile:'';
    composerMachine=active&&active.machine?active.machine:'';
    var existing=profileByID(composerProfile);
    if(existing)composerAgent=existing.agent;
  }
  var agents=agentSet();
  if(!composerAgent||agents.indexOf(composerAgent)<0)composerAgent=agents.indexOf('codex')>=0?'codex':(agents[0]||'');
  var profiles=(model.profiles||[]).filter(function(p){return p.agent===composerAgent;});
  if(!profileByID(composerProfile)||profileByID(composerProfile).agent!==composerAgent){
    var preferred=defaultProfileForAgent(composerAgent);composerProfile=preferred?preferred.id:'';composerMachine='';
  }
  var selectedProfile=profileByID(composerProfile);
  $('#composeragent').innerHTML=agents.length?agents.map(function(agent){return '<option value="'+esc(agent)+'"'+(agent===composerAgent?' selected':'')+'>'+esc(dispName(agent))+'</option>';}).join(''):'<option value="">No agents</option>';
  $('#composerprofile').innerHTML=profiles.length?profiles.map(function(profile){
    var label=profile.display_name||profile.model||profile.id;
    if(profile.state&&profile.state!=='ready')label+=' · '+String(profile.state).replace(/_/g,' ');
    return '<option value="'+esc(profile.id)+'"'+(profile.id===composerProfile?' selected':'')+(!profileSelectable(profile)?' disabled':'')+'>'+esc(label)+'</option>';
  }).join(''):'<option value="">No model profiles</option>';
  var machines=selectedProfile?(selectedProfile.machines||[]):model.machines.filter(function(m){return m.reachable&&(!composerAgent||(m.agents||[]).indexOf(composerAgent)>=0);}).map(function(m){return m.name;});
  var uniqueMachines=[];machines.forEach(function(name){if(uniqueMachines.indexOf(name)<0)uniqueMachines.push(name);});
  if(composerMachine&&uniqueMachines.indexOf(composerMachine)<0)composerMachine='';
  $('#composermachine').innerHTML='<option value="">Fort places it</option>'+uniqueMachines.map(function(name){return '<option value="'+esc(name)+'"'+(name===composerMachine?' selected':'')+'>'+esc(name)+'</option>';}).join('');
  var blocked=!selectedProfile||!profileSelectable(selectedProfile);
  $('#composeragent').disabled=conversationSending||!agents.length;
  $('#composerprofile').disabled=conversationSending||!profiles.length;
  $('#composermachine').disabled=conversationSending||!uniqueMachines.length;
  $('#conversationsend').disabled=conversationSending||blocked;
  $('#conversationassign').disabled=conversationSending;
  $('#composerstatus').classList.remove('error');
  $('#composerstatus').textContent=conversationSending?'Submitting to Fort…':profileStateText(selectedProfile);
}
function renderDeck(){
  if(openNote&&document.activeElement&&document.activeElement.closest&&document.activeElement.closest('.notebox'))return;
  var runs=conversationRuns();
  var active=runByID(selectedConversation);
  if(!active&&!composingNewConversation&&runs.length){active=runs[0];selectedConversation=active.id;localStorage.setItem('fort-conversation',active.id);}
  var n=needCount(),working=model.runs.filter(function(r){return runState(r)==='working';}).length;
  $('#conversationinbox').innerHTML=
    '<button class="side-row'+(n?' on':'')+'" onclick="openNeedsYou()"><span class="side-copy">Needs you</span><span class="side-time">'+n+'</span></button>'+
    '<button class="side-row"><span class="side-copy">Updates</span><span class="side-time">'+working+'</span></button>';
  $('#conversationlist').innerHTML=runs.map(function(r){
    var recent=conversationRecency(r),recentISO=recent?new Date(recent).toISOString():(r.updated_at||r.created_at);
    return '<div class="side-thread"><button class="side-row'+(active&&active.id===r.id?' on':'')+'" onclick="selectConversation(\''+esc(jsq(r.id))+'\')"><span class="side-copy">'+esc(r.title||r.id)+'</span>'+conversationStatusHTML(r)+'<span class="side-time">'+esc(ago(recentISO))+'</span></button></div>';
  }).join('')||'<div class="empty" style="padding:5px 8px">Start a new conversation.</div>';

  var title=active?(active.title||active.id):'New conversation';
  $('#conversationtitle').textContent=title;
  $('#conversationmeta').textContent=active?ckCaption(active):'Choose an agent, model, and machine';
  $('#mobileconversationstate').innerHTML=active?conversationStatusHTML(active):'<span class="conversation-status"><i></i>Ready</span>';
  $('#conversationdetail').hidden=!active;
  $('#conversationfeed').innerHTML=conversationFeedHTML(active);

  renderComposerControls(active);
  var agents=agentSet();
  var current=active?runAgent(active):composerAgent;
  if(!current)current=agents.indexOf('codex')>=0?'codex':(agents[0]||'');
  $('#currentagent').innerHTML=current?agentRailCard(current,true,active):'<div class="empty" style="padding:8px 0">Fort will choose an agent.</div>';
  $('#otheragents').innerHTML=agents.filter(function(a){return a!==current;}).map(function(a){return agentRailCard(a,false,null);}).join('')||'<div class="empty" style="padding:8px 0">No other agents available.</div>';
  $('#machinerail').innerHTML=model.machines.map(function(m){
    return '<div class="rail-card machine-card'+(m.reachable?'':' down')+'"><div class="rail-copy"><div class="rail-name">'+esc(m.name)+'</div><div class="rail-detail">'+esc((m.agents||[]).map(dispName).join(', ')||'execution node')+'</div></div><span class="rail-status"><i></i>'+(m.reachable?'Ready':'Offline')+'</span></div>';
  }).join('')||'<div class="rail-card machine-card"><div class="rail-copy"><div class="rail-name">This Mac</div><div class="rail-detail">local control plane</div></div><span class="rail-status"><i></i>Ready</span></div>';
  if(active)loadConversationDetail(active.id);
}
function selectConversation(id){
  composingNewConversation=false;composerSelectionRun='';selectedConversation=id;localStorage.setItem('fort-conversation',id);setMobileConversationNav(false);renderDeck();loadConversationDetail(id);
}
async function loadConversationDetail(id){
  if(!id||conversationDetails[id]||conversationLoading[id])return;
  conversationLoading[id]=true;
  try{
    var detail=await fetchJSON('/api/runs/'+encodeURIComponent(id));
    conversationDetails[id]=detail;(detail.events||[]).forEach(trackEvent);
  }catch(err){}finally{delete conversationLoading[id];}
  if(curView==='deck'&&selectedConversation===id)renderDeck();
}
function messageAvatarHTML(role,thinking){
  if(role==='human')return '<span class="message-avatar human-avatar" role="img" aria-label="You"><svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="8" r="3.25" fill="none" stroke="currentColor" stroke-width="1.5"/><path d="M5.5 19c.55-3.65 2.72-5.5 6.5-5.5s5.95 1.85 6.5 5.5" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg></span>';
  return '<img class="message-avatar '+orbClass(!!thinking)+'" src="/fort-agent-orb.png" alt=""/>';
}
function messageHTML(role,name,meta,body,modelName,thinking){
  return '<div class="message-row">'+messageAvatarHTML(role,thinking)+'<div class="message-copy"><div class="message-byline"><strong>'+esc(name)+'</strong>'+
    (modelName?'<span class="model-badge">'+esc(modelName)+'</span>':'')+'<span>'+esc(meta||'')+'</span></div><div class="message-body">'+esc(body||'')+'</div></div></div>';
}
function activityTimelineHTML(run,events){
  var rows=(events||[]).map(function(e){return {event:e,text:activityDescription(e)};}).filter(function(item){return !!item.text;}).slice(-6);
  var state=runState(run),latest=rows.length?Date.parse(rows[rows.length-1].event.time)||0:0;
  if(!rows.length){
    var empty=state==='starting'?'No provider activity yet — Fort is waiting for the first event.':
      state==='paused-review'?'Work is paused for your review.':
      state==='paused'?'Work is paused and needs direction.':
      state==='terminal'?'No provider activity was recorded for this run.':'No activity recorded yet.';
    rows=[{event:{type:'status',time:run.updated_at||run.created_at||''},text:empty}];
  }
  var activeNow=state==='working'&&latest>0&&(Date.now()-latest)<12000;
  return '<div class="activity-timeline" data-activity-timeline><div class="activity-head"><strong>Recorded activity</strong><span class="activity-sub">Fort event log</span>'+conversationStatusHTML(run)+'</div><div class="activity-events">'+rows.map(function(item,index){
    var isLast=index===rows.length-1&&activeNow,error=item.event.type==='error';
    return '<div class="activity-event'+(isLast?' active':'')+(error?' error':'')+'"><i></i><span>'+esc(item.text)+'</span><time>'+esc(ago(item.event.time))+'</time></div>';
  }).join('')+'</div></div>';
}
function canTurnConversationIntoWork(run){
  if(!run||run.flow_id||hasGate(run.id))return false;
  var status=String(run.status||'').toLowerCase();
  return status==='succeeded'||status==='done';
}
function conversationFeedHTML(run){
  if(!run)return messageHTML('agent','Fort','ready','Choose an agent, model, and eligible machine, then start the conversation.','');
  var agent=runAgent(run)||agentOfRun[run.id]||'',name=dispName(agent),detail=conversationDetails[run.id]||{};
  var prompt=runPrompt(run);
  var html=messageHTML('human','You',ago(run.created_at),prompt,'');
  var state=runState(run);
  var terminalStatus=String(run.status||'').toLowerCase();
  var response=state==='paused-review'?'I reached a checkpoint and need your direction before I continue.':
    state==='paused'?'Work is paused. Open the recorded activity to see what needs direction.':
    state==='terminal'&&(terminalStatus==='failed'||terminalStatus==='error')?'I hit a wall. The exact failure is preserved in the activity below.':
    state==='terminal'&&(terminalStatus==='canceled'||terminalStatus==='cancelled')?'This conversation was canceled. Its recorded events are preserved below.':
    state==='terminal'?'This conversation is finished and its recorded events are preserved below.':
    state==='working'?'Work has started. The activity below comes directly from Fort’s event log.':
    'Fort accepted this turn and is waiting for the first provider event.';
  html+=messageHTML('agent',name||'Fort',ago(run.updated_at||run.created_at),response,run.model||agentModel(agent),isThinking(run));
  var conversationEvents=(actByRun[run.id]&&actByRun[run.id].length)?actByRun[run.id]:(detail.events||[]);
  conversationEvents.filter(function(e){return e.type==='message'&&e.data;}).slice(-2).forEach(function(e){
    var line=String(e.data).split('\n')[0];if(line.length>180)line=line.slice(0,179)+'…';
    if(line)html+=messageHTML('agent',name||'Fort',ago(e.time),line,run.model||agentModel(agent));
  });
  html+=approvalCardsHTML(run);
  html+=activityTimelineHTML(run,conversationEvents);
  if(canTurnConversationIntoWork(run))html+='<div class="turn-work"><img src="/fort-agent-orb.png" alt=""/><div class="copy"><div class="title">Turn this into work</div><div class="detail">Create a routed assignment from this conversation.</div></div><button id="turnintowork" onclick="turnConversationIntoWork()">Assign work</button></div>';
  html+=assignmentCardHTML(run,detail.nodes||[]);
  return html;
}
function approvalCardsHTML(run){
  return gatesFor(run.id).map(function(gate){
    var key=run.id+'|'+gate.node_id,box='note-'+cssKey(key);
    var input=String(gate.input||'').trim();if(input.length>220)input=input.slice(0,219)+'…';
    return '<div class="approval-card"><div class="approval-title"><i></i><span>Needs approval · '+esc(gateTitle(gate.node_id))+'</span></div>'+
      '<div class="approval-copy">Work is paused until you approve or request changes.'+(input?'<br>'+esc(input):'')+'</div>'+
      '<div class="approval-actions"><button class="btn btn-amber" onclick="decide(\''+esc(jsq(run.id))+'\',\''+esc(jsq(gate.node_id))+'\',\'approve\',\'\')">Approve & continue</button><button class="btn btn-neutral" onclick="toggleNote(\''+esc(jsq(key))+'\')">Request changes</button></div>'+
      '<div class="notebox'+(openNote===key?' open':'')+'" id="'+esc(box)+'"><textarea placeholder="Describe what should change before work continues."></textarea><button class="btn btn-brassline" onclick="sendNote(\''+esc(jsq(run.id))+'\',\''+esc(jsq(gate.node_id))+'\')">Send changes</button></div></div>';
  }).join('');
}
function assignmentCardHTML(run,nodes){
  var c=run.checkpoints||{},total=c.total||nodes.length||0;
  if(!total&&run.status!=='running'&&run.status!=='blocked')return '';
  var accepted=c.accepted||0,pct=total?Math.round(accepted*100/total):0;
  var rows=nodes.map(function(node){
    var status=(node.status||'').toLowerCase(),done=status==='succeeded'||status==='approved',current=status==='running'||status==='waiting';
    var name=(node.node_id||'checkpoint').replace(/[-_]/g,' ');
    return '<div class="checkpoint'+(done?' done':current?' current':'')+'"><i></i><span>'+esc(name)+'</span></div>';
  });
  if(!rows.length&&total){for(var i=0;i<total;i++)rows.push('<div class="checkpoint'+(i<accepted?' done':i===accepted&&run.status==='running'?' current':'')+'"><i></i><span>Checkpoint '+(i+1)+'</span></div>');}
  return '<div class="assignment-card"><div class="assignment-head"><strong>'+esc(run.title||run.id)+'</strong><span class="state">'+esc(runStatusLabel(run))+'</span></div>'+
    '<div class="progress-track"><i style="width:'+pct+'%"></i></div><div class="assignment-meta"><span>Agent<strong>'+esc(dispName(runAgent(run)))+'</strong></span><span>Model<strong>'+esc(run.model||agentModel(runAgent(run))||'configured')+'</strong></span><span>Machine<strong>'+esc(run.machine||'Fort placed')+'</strong></span><span>Elapsed<strong>'+esc(elapsed(run.created_at))+'</strong></span><span>Progress<strong>'+accepted+' of '+total+'</strong></span></div><div class="checkpoint-list">'+rows.join('')+'</div></div>';
}
function agentRailCard(agent,current,run){
  var st=agentStatus(agent),ready=st.state==='idle'?'Ready':st.state==='need'?'Needs you':'Working';
  var detail=run?activitySentence(run):(st.run?activitySentence(st.run):'Ready for routed work.');
  var modelName=run&&run.model?run.model:agentModel(agent);
  return '<div class="rail-card"><img class="'+orbClass(st.state==='working')+'" src="/fort-agent-orb.png" alt=""/><div class="rail-copy"><div class="rail-name">'+esc(dispName(agent))+(modelName?'<span class="rail-model">'+esc(modelName)+'</span>':'')+'</div><div class="rail-detail">'+esc(detail||'Ready for routed work.')+'</div></div><span class="rail-status '+esc(st.state)+'"><i></i>'+esc(ready)+'</span></div>';
}
function runPrompt(run){
  var title=(run.title||run.id||'').trim(),body=(run.body||'').trim();
  if(!body)return title;
  var lines=body.split('\n');if(lines[0].trim()===title)lines.shift();
  return lines.join('\n').trim()||title;
}
function conversationSeed(){var run=runByID(selectedConversation);if(!run)return '';var prompt=runPrompt(run),title=run.title||run.id;return prompt===title?title:title+'\n'+prompt;}
function prepareConversation(mode,seed,playbook){
  showView('assign');setAssignMode(mode);
  routeChoice=playbook?{id:playbook.id,revision:playbook.revision}:null;routePreview=null;routePickerOpen=false;
  $('#brief').value=(seed||'').trim();renderBriefPreview();renderAssignControls();queueRoutePreview();$('#brief').focus();
}
function turnConversationIntoWork(){prepareConversation('assignment',conversationSeed(),defaultAssignmentPlaybook());}
function assignConversation(){var text=$('#conversationcomposer').value.trim();prepareConversation('assignment',text||conversationSeed());}
async function sendConversation(){
  var text=$('#conversationcomposer').value.trim();if(!text){$('#composerstatus').classList.add('error');$('#composerstatus').textContent='Write a message first.';return;}
  var profile=profileByID(composerProfile);
  if(!profile||!profileSelectable(profile)){$('#composerstatus').classList.add('error');$('#composerstatus').textContent=profileStateText(profile);return;}
  if(conversationSending)return;
  conversationSending=true;renderComposerControls(selectedConversation&&runByID(selectedConversation));
  var request={text:text,agent:composerAgent,profile:composerProfile,machine:composerMachine};
  try{
    var response=await fetch('/api/chat',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify(request)});
    var payload=await response.text();
    if(!response.ok)throw new Error(payload.trim()||('Request failed with '+response.status));
    var result=payload?JSON.parse(payload):{};
    $('#conversationcomposer').value='';composingNewConversation=false;composerSelectionRun='';
    if(result.run_id){selectedConversation=result.run_id;localStorage.setItem('fort-conversation',result.run_id);}
    conversationSending=false;await refresh();renderDeck();
  }catch(err){
    conversationSending=false;renderComposerControls(selectedConversation&&runByID(selectedConversation));
    $('#composerstatus').classList.add('error');
    $('#composerstatus').textContent=err&&err.message?err.message:'Unable to send this conversation.';
  }
}
function cssKey(k){return k.replace(/[^a-zA-Z0-9_-]/g,'_');}
// jsq escapes a value for use inside a single-quoted JS string that lives in an
// HTML attribute: backslash-escape quotes BEFORE esc() entity-encodes them, so
// the browser's attribute decode can't un-quote the JS string.
function jsq(s){return String(s||'').replace(/\\/g,'\\\\').replace(/'/g,"\\'");}
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
// ---- Assign ----
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
        '<button class="btn btn-brassline" style="margin-left:auto;font-size:12.5px;padding:5px 12px" onclick="showView(\'assign\')">Assign work</button></div></div>';
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
function setAssignMode(mode){
  if(mode!=='quick')mode='assignment';
  if(assignMode===mode)return;
  assignMode=mode;routeChoice=null;routePreview=null;routePickerOpen=false;quickAnswer='';quickAnswerError='';
  renderAssignControls();queueRoutePreview();$('#brief').focus();
}
function effectivePlanGate(){return assignMode!=='quick'&&planFirst;}
function renderAssignControls(){
  var quick=assignMode==='quick';
  $('#modeassignment').classList.toggle('on',!quick);
  $('#modeassignment').setAttribute('aria-selected',quick?'false':'true');
  $('#modequick').classList.toggle('on',quick);
  $('#modequick').setAttribute('aria-selected',quick?'true':'false');
  $('#plantoggle').hidden=quick;
  $('#plantoggle').classList.toggle('off',!effectivePlanGate());
  $('#plantoggle').setAttribute('aria-pressed',effectivePlanGate()?'true':'false');
  $('#tobacklog').hidden=quick;
  if(!handoffPending)$('#handoff').textContent=quick?'Ask Fort':'Hand it off';
  $('#brief').placeholder=quick?'Ask a focused question — Fort will answer without creating an assignment.':'Describe the outcome you want — like briefing an employee.';
  var qa=$('#quickanswer');
  qa.hidden=!quickAnswer&&!quickAnswerError;
  qa.classList.toggle('fail',!!quickAnswerError);
  qa.innerHTML=quickAnswerError?'<div class="answerhead">Quick answer failed</div><div class="mdbody">'+md(quickAnswerError)+'</div>':
    quickAnswer?'<div class="answerhead">Quick answer</div><div class="mdbody">'+md(quickAnswer)+'</div>':'';
  renderRoutePreview();
}
$('#plantoggle').addEventListener('click',function(){planFirst=!planFirst;renderAssignControls();queueRoutePreview();});
function renderBriefPreview(){
  var t=$('#brief').value,i=t.indexOf('\n');
  var body=i<0?'':t.slice(i+1).trim();
  var pv=$('#briefpv');
  if(!body){pv.hidden=true;pv.innerHTML='';return;}
  pv.hidden=false;pv.innerHTML='<strong>'+esc(t.slice(0,i).trim())+'</strong>'+md(body);
}
function queueRoutePreview(){
  clearTimeout(routeTimer);routePreview=null;renderRoutePreview();
  routeTimer=setTimeout(function(){previewRoute();},260);
}
async function previewRoute(){
  clearTimeout(routeTimer);routeTimer=null;
  var text=$('#brief').value.trim();
  if(!text){routePreview=null;renderRoutePreview();return null;}
  var seq=++routeSerial;
  var body={text:text,task_type:assignMode==='quick'?'question':'',plan_gate:effectivePlanGate()};
  if(routeChoice){
    body.playbook_id=routeChoice.id;
    body.playbook_revision=routeChoice.revision;
  }else if(assignMode==='quick'){
    var answer=availablePlaybooks()[0];
    if(answer){body.playbook_id=answer.id;body.playbook_revision=answer.revision;}
  }
  try{
    var resp=await fetch('/api/route',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify(body)});
    if(!resp.ok)throw new Error(await resp.text());
    var resolved=await resp.json();
    if(assignMode==='quick'&&resolved.delivery!=='answer')throw new Error('No answer playbook is available.');
    if(seq!==routeSerial)return routePreview;
    routePreview=resolved;renderRoutePreview();return resolved;
  }catch(err){
    if(seq!==routeSerial)return routePreview;
    routePreview={error:String(err&&err.message||err)};renderRoutePreview();return null;
  }
}
function renderRoutePreview(){
  var el=$('#routepreview'),picker=$('#routepicker'),text=$('#brief').value.trim();
  if(!text){el.hidden=true;picker.hidden=true;return;}
  el.hidden=false;
  if(!routePreview){
    el.innerHTML='<div class="hd"><span class="lb">Route</span><span class="routenote">Choosing the best playbook…</span></div>';
  }else if(routePreview.error){
    el.innerHTML='<div class="hd"><span class="lb">Route</span><span class="nm">Preview unavailable</span></div><span class="routenote">'+esc(routePreview.error)+'</span>';
  }else{
    var pb=playbookByID(routePreview.playbook_id);
    var name=routePreview.playbook_name||(pb&&pb.name)||routePreview.playbook_id||'Fort decides';
    var stages=routePreview.stages||[];
    var chain=stages.map(function(st,i){
      var chip='<span class="routechip"><strong>'+esc(dispName(st.agent))+'</strong>'+(st.model?' · <span class="model">'+esc(st.model)+'</span>':'')+(st.memory?' · <span style="color:var(--work)">memory ●</span>':'')+'</span>';
      return (i?'<span class="routearrow">→</span>':'')+chip;
    }).join('');
    var note=routePreview.delivery==='answer'?'Answering inline — no assignment, checkpoints, or schedule entry.':
      routePreview.plan_gate?'Plan first — you sign off before build starts.':'Starts when you hand it off.';
    if(routePreview.task_type)note+=' · '+String(routePreview.task_type).replace(/[-_]/g,' ');
    el.innerHTML='<div class="hd"><span class="lb">Route</span><span class="nm">'+esc(name)+'</span><button id="routechange" class="change" onclick="toggleRoutePicker()">Change…</button></div>'+
      '<div class="routechain">'+chain+'</div><span class="routenote">'+esc(note)+'</span>';
  }
  picker.hidden=!routePickerOpen;
  if(routePickerOpen){
    var auto='<button class="'+(routeChoice?'':'on')+'" onclick="chooseRoute(\'\')">Fort decides</button>';
    picker.innerHTML=auto+availablePlaybooks().map(function(p){
      return '<button class="'+(routeChoice&&routeChoice.id===p.id?'on':'')+'" onclick="chooseRoute(\''+esc(jsq(p.id))+'\')">'+esc(p.name)+'</button>';
    }).join('');
  }
}
function toggleRoutePicker(){routePickerOpen=!routePickerOpen;renderRoutePreview();}
function chooseRoute(id){
  var p=playbookByID(id);routeChoice=p?{id:p.id,revision:p.revision}:null;
  routePickerOpen=false;routePreview=null;renderRoutePreview();previewRoute();
}
$('#brief').addEventListener('input',function(){quickAnswer='';quickAnswerError='';renderBriefPreview();queueRoutePreview();});
$('#brief').addEventListener('keydown',function(e){if((e.metaKey||e.ctrlKey)&&e.key==='Enter')handoff();});
function setHandoffPending(pending,message,isError){
  handoffPending=pending;
  var button=$('#handoff'),status=$('#handoffstatus');
  button.disabled=handoffPending;
  button.setAttribute('aria-busy',handoffPending?'true':'false');
  button.textContent=handoffPending?(assignMode==='quick'?'Asking Fort…':'Handing off…'):(assignMode==='quick'?'Ask Fort':'Hand it off');
  status.hidden=!message;
  status.classList.toggle('fail',!!isError);
  status.textContent=message||'';
}
async function handoff(){
  if(handoffPending)return;
  var text=$('#brief').value;
  if(!text.trim()){setHandoffPending(false,'Describe the outcome before handing it off.',true);return;}
  var settledMessage='',settledError=false;
  setHandoffPending(true,assignMode==='quick'?'Fort is answering this question…':'Fort is confirming the route and starting this assignment…',false);
  try{
    var resolved=routePreview&&!routePreview.error?routePreview:await previewRoute();
    if(!resolved)throw new Error('Fort could not preview this route. Try again in a moment.');
    var request={text:text,task_type:resolved.task_type||'',plan_gate:!!resolved.plan_gate};
    if(resolved.playbook_id)request.playbook_id=resolved.playbook_id;
    if(resolved.playbook_revision!==undefined)request.playbook_revision=resolved.playbook_revision;
    var r=await fetch('/api/chat',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify(request)});
    var responseText=await r.text();
    if(!r.ok){
      var failure=responseText.trim()||'Fort could not start this handoff.';
      if(r.status===409)failure='This route needs the execution plane — start fort serve, or choose a direct route.';
      throw new Error(failure);
    }
    var result=responseText?JSON.parse(responseText):{};
    if(assignMode==='quick'&&result.kind!=='answer')throw new Error('Fort returned an assignment instead of an answer.');
    if(result.kind==='answer'&&r.status!==202){
      quickAnswer=(result.answer||'').trim();quickAnswerError=quickAnswer?'':'Fort returned no answer text.';
      if(quickAnswerError)throw new Error(quickAnswerError);
      renderAssignControls();
      return;
    }
    if(assignCtx&&assignCtx.backlogId)await fetch('/api/backlog/'+assignCtx.backlogId,{method:'DELETE'});
    assignCtx=null;
    $('#brief').value='';routePreview=null;routeChoice=null;quickAnswer='';quickAnswerError='';renderBriefPreview();
    if(result.run_id){selectedConversation=result.run_id;localStorage.setItem('fort-conversation',result.run_id);}
    showView('deck');await refresh();
  }catch(err){
    settledMessage=err&&err.message?err.message:'Fort could not start this handoff.';
    settledError=true;
    if(assignMode==='quick'){quickAnswer='';quickAnswerError=settledMessage;renderAssignControls();}
  }finally{setHandoffPending(false,settledMessage,settledError);}
}
$('#handoff').addEventListener('click',handoff);
$('#tobacklog').addEventListener('click',async function(){
  var t=$('#brief').value;if(!t.trim())return;
  var i=t.indexOf('\n');
  var title=i<0?t.trim():t.slice(0,i).trim();
  var body=i<0?'':t.slice(i+1).trim();
  await fetch('/api/backlog',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({title:title,body:body})});
  $('#brief').value='';renderBriefPreview();refresh();
});

// ---- Playbooks (Turn 4) ----
function assignmentProfilesFor(agent){
  return (model.profiles||[]).filter(function(profile){return profile.agent===agent;});
}
function assignmentProfileFor(agent,modelName){
  var found=null;
  assignmentProfilesFor(agent).some(function(profile){if((profile.model||'')===(modelName||'')){found=profile;return true;}return false;});
  return found;
}
function preferredAssignmentProfile(agent){
  var profiles=assignmentProfilesFor(agent),configured=profiles.find(function(profile){return !profile.model&&profileSelectable(profile);});
  return configured||profiles.find(profileSelectable)||profiles[0]||null;
}
function syncAssignmentProfile(a){var profile=assignmentProfileFor(a.agent||'',a.model||'');if(profile)a.profile=profile.id;else delete a.profile;}
function playbookByID(id){
  for(var i=0;i<model.playbooks.length;i++)if(model.playbooks[i].id===id)return model.playbooks[i];
  return null;
}
function availablePlaybooks(){
  return model.playbooks.filter(function(p){return assignMode==='quick'?p.delivery==='answer':p.delivery!=='answer';});
}
function defaultAssignmentPlaybook(){
  var choices=model.playbooks.filter(function(p){return p.delivery!=='answer';});
  return choices.find(function(p){return p.is_default;})||choices.sort(function(a,b){return String(a.id).localeCompare(String(b.id));})[0]||null;
}
function cloneData(v){return JSON.parse(JSON.stringify(v));}
function stageAssignments(st){return (st.assignments&&st.assignments.length)?st.assignments:[];}
function triggerKind(p){return p&&p.trigger&&p.trigger.kind?p.trigger.kind:'manual';}
function triggerCopy(kind){
  var copy={question:'I ask a question',bug:'direction is a bug report','bug report':'direction is a bug report',feature:'direction describes a new capability',research:'direction asks for research',manual:'I choose it manually'};
  return copy[kind]||kind.replace(/[-_]/g,' ');
}
function playbookMeta(p){
  var n=(p.stages||[]).length,stages=n+' stage'+(n===1?'':'s'),kind=triggerKind(p);
  if(p.delivery==='answer')return stages+' · no checkpoints';
  if(kind==='bug')return stages+' · skips design';
  if(kind==='research')return stages+' · delivers a doc';
  return stages+' · plan gate '+(p.plan_gate?'on':'off');
}
function branchLabel(p,a,branching){
  if(!branching)return '';
  if(a.task_type==='bug')return 'bug fixes';
  if(a.task_type)return a.task_type.replace(/[-_]/g,' ');
  return triggerKind(p)==='feature'?'features':'default';
}
function shortcutRank(p){
  if(p.delivery==='answer')return 0;
  if(triggerKind(p)==='bug')return 1;
  if(triggerKind(p)==='research')return 2;
  return 3;
}
function optionList(values,current,labeler){
  var all=values.slice();if(current&&all.indexOf(current)<0)all.push(current);
  return all.map(function(v){return '<option value="'+esc(v)+'"'+(v===current?' selected':'')+'>'+esc(labeler?labeler(v):v)+'</option>';}).join('');
}
function modelOptionList(agent,current){
  var profiles=assignmentProfilesFor(agent),matched=false;
  var options=profiles.map(function(profile){
    var modelName=profile.model||'',label=profile.display_name||modelName||profile.id;
    if(modelName===current)matched=true;
    if(profile.state&&profile.state!=='ready')label+=' · '+String(profile.state).replace(/_/g,' ');
    return '<option value="'+esc(modelName)+'"'+(modelName===current?' selected':'')+(profileSelectable(profile)?'':' disabled')+'>'+esc(label)+'</option>';
  });
  if(!matched&&current)options.push('<option value="'+esc(current)+'" selected disabled>'+esc(current)+' · not in current catalog</option>');
  return options.join('')||'<option value="" selected disabled>No model profiles</option>';
}
function renderPlaybooks(){
  var list=$('#playbooklist'),editor=$('#playbookeditor');
  if(!playbooksLoaded){list.innerHTML='<div class="empty" style="padding:4px">Loading playbooks…</div>';editor.innerHTML='';return;}
  if(!model.playbooks.length){list.innerHTML='<div class="empty" style="padding:4px">No playbooks configured.</div>';editor.innerHTML='<div class="empty" style="padding:0">Add a playbook through Fort&#39;s configuration, then edit it here.</div>';return;}
  var pb=playbookByID(selectedPlaybook)||model.playbooks[0];
  selectedPlaybook=pb.id;localStorage.setItem('fort-playbook',selectedPlaybook);
  list.innerHTML=model.playbooks.map(function(p){
    return '<button class="pbitem'+(p.id===pb.id?' on':'')+'" onclick="selectPlaybook(\''+esc(jsq(p.id))+'\')">'+
      '<span class="name">'+esc(p.name)+(p.is_default?'<span class="default">default</span>':'')+'</span>'+
      '<span class="meta">'+esc(playbookMeta(p))+'</span></button>';
  }).join('');

  var stages=(pb.stages||[]).slice().sort(function(a,b){return a.order-b.order;});
  var pipeline=stages.map(function(st,si){
    var stageIndex=(pb.stages||[]).indexOf(st);
    var assignments=stageAssignments(st),branching=assignments.length>1;
    var badge=st.memory?'<button class="badge memory" onclick="toggleStageMemory(\''+esc(jsq(pb.id))+'\','+stageIndex+')">memory ●</button>':
      branching?'<span class="badge">by task type</span>':'<button class="badge" onclick="toggleStageMemory(\''+esc(jsq(pb.id))+'\','+stageIndex+')">memory off</button>';
    var rows=assignments.map(function(a,ai){
      var type=branchLabel(pb,a,branching);
      return '<div class="assignment">'+(type?'<span class="tasktype">'+esc(type.replace(/[-_]/g,' '))+'</span>':'')+
        '<select class="agentselect" aria-label="agent for '+esc(st.name)+'" onchange="editStageAssignment(\''+esc(jsq(pb.id))+'\','+stageIndex+','+ai+',\'agent\',this.value)">'+
          optionList(agentSet(),a.agent,dispName)+'</select>'+
        '<select class="modelselect" aria-label="model for '+esc(st.name)+'" onchange="editStageAssignment(\''+esc(jsq(pb.id))+'\','+stageIndex+','+ai+',\'model\',this.value)">'+
          modelOptionList(a.agent,a.model)+'</select></div>';
    }).join('');
    if(!rows)rows='<div class="routenote">No assignment configured.</div>';
    var desc=st.description||st.prompt||('Runs the '+String(st.name||'stage').toLowerCase()+' stage with the selected agent and model.');
    return (si?'<span class="pipearrow">→</span>':'')+'<div class="stagecard">'+
      '<div class="hd"><span class="num">'+esc(String(st.order||si+1))+'</span><span class="name">'+esc(st.name||'Stage')+'</span>'+badge+'</div>'+
      '<div style="display:flex;flex-direction:column;gap:6px">'+rows+'</div><span class="desc">'+esc(desc)+'</span></div>';
  }).join('');
  if(pb.delivery!=='answer')pipeline+='<button class="stageadd" onclick="addPlaybookStage()" aria-label="add stage">＋</button>';

  var shortcuts=model.playbooks.filter(function(p){return p.id!==pb.id&&triggerKind(p)!=='manual';}).sort(function(a,b){return shortcutRank(a)-shortcutRank(b)||a.name.localeCompare(b.name);});
  var shortcutRows=shortcuts.map(function(p){
    var tr=p.trigger||{},st=(p.stages||[])[0]||{},a=stageAssignments(st)[0]||{};
    var title='When '+triggerCopy(triggerKind(p))+' → '+p.name;
    var summary=(a.agent?dispName(a.agent)+(a.model?' · '+a.model:''):'Fort decides')+
      (p.delivery==='answer'?' · replies inline · no checkpoints, nothing scheduled':(p.plan_gate?' · plan gate on':' · starts directly'));
    return '<div class="shortcutrow"><span style="font-size:15px">'+(p.delivery==='answer'?'⚡':'↳')+'</span><div class="copy"><span class="title">'+esc(title)+'</span><span class="summary">'+esc(summary)+'</span></div>'+
      '<button class="switchbtn'+(tr.enabled?' on':'')+'" aria-label="toggle '+esc(p.name)+' shortcut" aria-pressed="'+(tr.enabled?'true':'false')+'" onclick="toggleShortcut(\''+esc(jsq(p.id))+'\')"><i></i></button></div>';
  }).join('')||'<div class="empty" style="padding:2px 0">No shortcut triggers configured.</div>';

  var gateControl=pb.delivery==='answer'?'<span class="trigger" style="margin-left:auto">No checkpoints</span>':
    '<span class="trigger gate-label">Plan gate</span><button class="switchbtn'+(pb.plan_gate?' on':'')+'" style="margin-left:0" aria-label="toggle plan gate" aria-pressed="'+(pb.plan_gate?'true':'false')+'" onclick="togglePlaybookPlanGate()"><i></i></button>';
  editor.innerHTML='<div class="pbeditor-head"><span class="title">'+esc(pb.name)+'</span><span class="trigger">Trigger: '+esc(triggerCopy(triggerKind(pb)))+' · <button class="edit" onclick="editPlaybookTrigger()">edit</button></span>'+gateControl+
    '<span class="revision">rev '+esc(String(pb.revision||1))+'</span></div>'+
    '<div class="pipeline">'+pipeline+'</div><div class="shortcuts"><span class="ulabel">Shortcuts — triggers that skip the chain</span>'+shortcutRows+'</div>';
}
function selectPlaybook(id){selectedPlaybook=id;localStorage.setItem('fort-playbook',id);renderPlaybooks();}
async function savePlaybook(next){
  var resp=await fetch('/api/playbooks',{method:'PUT',headers:{'content-type':'application/json'},body:JSON.stringify(next)});
  if(!resp.ok){
    var message=(await resp.text())||'Fort could not save this playbook.';
    if(resp.status===409){await fetchPlaybooks();alert('This playbook changed in another edit. Fort reloaded the latest revision.');return null;}
    alert(message);return null;
  }
  var saved=await resp.json(),found=false;
  model.playbooks=model.playbooks.map(function(p){if(p.id===saved.id){found=true;return saved;}return p;});
  if(!found)model.playbooks.push(saved);
  selectedPlaybook=saved.id;localStorage.setItem('fort-playbook',saved.id);
  if(routeChoice&&routeChoice.id===saved.id)routeChoice={id:saved.id,revision:saved.revision};
  renderPlaybooks();return saved;
}
async function duplicatePlaybook(){
  var pb=playbookByID(selectedPlaybook)||model.playbooks[0];
  if(!pb){alert('There is no playbook to duplicate yet.');return;}
  var resp=await fetch('/api/playbooks/'+encodeURIComponent(pb.id)+'/duplicate',{method:'POST'});
  if(!resp.ok){alert((await resp.text())||'Fort could not duplicate this playbook.');return;}
  var copy=await resp.json();model.playbooks.push(copy);selectedPlaybook=copy.id;localStorage.setItem('fort-playbook',copy.id);renderPlaybooks();
}
function editPlaybookTrigger(){
  var pb=playbookByID(selectedPlaybook);if(!pb)return;
  var kind=prompt('Trigger kind: question, bug, feature, research, or manual',triggerKind(pb));
  if(kind===null)return;kind=kind.trim().toLowerCase();
  if(['question','bug','feature','research','manual'].indexOf(kind)<0){alert('Use question, bug, feature, research, or manual.');return;}
  var next=cloneData(pb);next.trigger=next.trigger||{};next.trigger.kind=kind;
  if(kind==='manual')next.trigger.enabled=false;
  savePlaybook(next);
}
function togglePlaybookPlanGate(){var pb=playbookByID(selectedPlaybook);if(!pb||pb.delivery==='answer')return;var next=cloneData(pb);next.plan_gate=!next.plan_gate;savePlaybook(next);}
function toggleStageMemory(id,stageIndex){var pb=playbookByID(id);if(!pb)return;var next=cloneData(pb);next.stages[stageIndex].memory=!next.stages[stageIndex].memory;savePlaybook(next);}
function editStageAssignment(id,stageIndex,assignmentIndex,field,value){
  var pb=playbookByID(id);if(!pb)return;var next=cloneData(pb),a=next.stages[stageIndex].assignments[assignmentIndex];a[field]=value;
  if(field==='agent'){var preferred=preferredAssignmentProfile(value);a.model=preferred?(preferred.model||''):'';}
  syncAssignmentProfile(a);
  savePlaybook(next);
}
function addPlaybookStage(){
  var pb=playbookByID(selectedPlaybook);if(!pb||pb.delivery==='answer')return;
  var name=prompt('Stage name','New stage');if(name===null||!name.trim())return;
  var next=cloneData(pb),prev=next.stages.length?next.stages[next.stages.length-1]:null,pa=prev&&stageAssignments(prev)[0];
  var agent=(pa&&pa.agent)||'codex',preferred=preferredAssignmentProfile(agent);
  var assignment={agent:agent,model:pa?(pa.model||''):((preferred&&preferred.model)||'')};syncAssignmentProfile(assignment);
  next.stages.push({order:next.stages.length+1,name:name.trim(),assignments:[assignment],memory:false});
  savePlaybook(next);
}
function toggleShortcut(id){
  var pb=playbookByID(id);if(!pb)return;var next=cloneData(pb);next.trigger=next.trigger||{kind:'manual'};next.trigger.enabled=!next.trigger.enabled;savePlaybook(next);
}

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
      cells.push('<div class="blk blk-idle" style="grid-column:span '+(7-todayIdx)+'" onclick="showView(\'assign\')">open capacity — assign work</div>');
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
    b.addEventListener('dragstart',function(e){dragId=b.dataset.bid;e.dataTransfer.effectAllowed='move';e.dataTransfer.setData('text/plain',dragId);});
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
      cells.push('<div class="blk blk-idle" style="grid-column:span '+(hours-start)+'" onclick="showView(\'assign\')">idle — assign work</div>');
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
      '<div class="acts"><button class="btn btn-ok" onclick="decide(\''+g.run_id+'\',\''+esc(jsq(g.node_id))+'\',\'approve\')">Approve</button>'+
      '<button class="btn btn-outline" onclick="drawerReject(\''+g.run_id+'\',\''+esc(jsq(g.node_id))+'\')">Request changes…</button></div>';
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
    '<div class="step s-'+esc(n.status)+(n.node_id===dwNode?' sel':'')+'" onclick="selectStep(\''+esc(jsq(n.node_id))+'\')">'+
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
document.addEventListener('keydown',e=>{if(e.key==='Escape'){if($('#conversationnav').classList.contains('mobile-open'))setMobileConversationNav(false);else if(openNote)toggleNote(null);else closeDrawer();}});

// ---- actions ----
async function decide(run,node,decision,note){
  const body={run_id:run,node_id:node,decision:decision};
  if(note)body.note=note;
  const r=await fetch('/api/gate',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify(body)});
  const responseBody=await r.text();
  await refresh();
  if(!r.ok){
    const message=responseBody.trim()||('Sign-off failed with '+r.status);
    $('#composerstatus').classList.add('error');
    $('#composerstatus').textContent=message;
  }
}

// ---- boot ----
let refreshQueued=false;
const es=new EventSource('/api/events?since=0');
function onFortEvent(ev){
  try{trackEvent(JSON.parse(ev.data))}catch(err){}
  if(!refreshQueued){refreshQueued=true;setTimeout(function(){refreshQueued=false;refresh();},300);}
}
es.addEventListener('started',onFortEvent);
es.addEventListener('stdout',onFortEvent);
es.addEventListener('stderr',onFortEvent);
es.addEventListener('message',onFortEvent);
es.addEventListener('tool',onFortEvent);
es.addEventListener('subagent',onFortEvent);
es.addEventListener('exited',onFortEvent);
es.addEventListener('error',onFortEvent);
es.addEventListener('ingress',onFortEvent);
es.addEventListener('placement',onFortEvent);
es.addEventListener('transform',onFortEvent);
es.addEventListener('gate',onFortEvent);
setInterval(refresh,3000);
setInterval(fetchMetrics,60000);
showView(curView);
fetchPlaybooks();
refresh();
fetchMetrics();
</script>
</body>
</html>`
