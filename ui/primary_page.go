package ui

import "net/http"

// handlePrimaryPage serves the Phase 1 private Channels shell. The page is a
// pure client of the narrow Primary Agent, Channel, Needs-you, and schedule
// read APIs; it contains no seeded transcript or schedule state.
func (s *Server) handlePrimaryPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self'; connect-src 'self'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(primaryPageHTML))
}

const primaryPageHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<meta name="color-scheme" content="dark light">
<title>Fort · Private Channels</title>
<link rel="icon" href="/fort-icon.png">
<style>
:root{
  color-scheme:dark;
  --bg:#03101d;--rail:#061522;--panel:#081a2a;--raised:#0b2033;
  --line:#17334b;--line-strong:#23577d;--text:#edf6ff;--body:#cad8e6;
  --muted:#8da1b5;--faint:#63798e;--accent:#2a8dff;--accent-soft:#102f50;
  --accent-ink:#ffffff;--good:#6bc99a;--warn:#e7bd63;--bad:#ee7c79;
  --shadow:0 18px 54px rgba(0,0,0,.28);--orb-filter:none;
  --font:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;
  --mono:"SFMono-Regular",Consolas,"Liberation Mono",monospace;
}
body[data-theme="private-channels"]{
  color-scheme:dark;
  --bg:#121715;--rail:#151b18;--panel:#191f1c;--raised:#202821;
  --line:#303a31;--line-strong:#596845;--text:#f1f4ec;--body:#d8ddd2;
  --muted:#9da79a;--faint:#747e71;--accent:#c3e35c;--accent-soft:#303923;
  --accent-ink:#162007;--good:#a8cd65;--warn:#f2b54a;--bad:#e78568;
  --shadow:0 18px 54px rgba(0,0,0,.24);--orb-filter:hue-rotate(52deg) saturate(1.2);
}
body[data-theme="native-daylight"]{
  color-scheme:light;
  --bg:#f7f8fa;--rail:#ffffff;--panel:#ffffff;--raised:#f1f5fa;
  --line:#d9e1eb;--line-strong:#a9c7ed;--text:#17212b;--body:#344252;
  --muted:#687789;--faint:#8794a3;--accent:#0878f9;--accent-soft:#e9f2ff;
  --accent-ink:#ffffff;--good:#25834d;--warn:#9b6b11;--bad:#bd3838;
  --shadow:0 18px 54px rgba(27,46,67,.12);--orb-filter:brightness(1.18) saturate(.82);
}
*{box-sizing:border-box}
html,body{height:100%;margin:0;overflow-x:hidden}
body{background:var(--bg);color:var(--text);font:14px/1.45 var(--font);text-rendering:optimizeLegibility}
button,input,textarea,select{font:inherit}
button,input,textarea,select,a{outline:none}
button:focus-visible,input:focus-visible,textarea:focus-visible,select:focus-visible,a:focus-visible{outline:3px solid var(--accent);outline-offset:2px}
button{min-height:44px;color:inherit}
button:disabled{cursor:not-allowed;opacity:.48}
.skip-link{position:fixed;left:16px;top:10px;z-index:100;background:var(--accent);color:var(--accent-ink);padding:9px 13px;border-radius:7px;transform:translateY(-180%)}
.skip-link:focus{transform:translateY(0)}
.app-shell{height:100dvh;display:grid;grid-template-columns:260px minmax(0,1fr);overflow:hidden}
.channel-rail{min-width:0;display:flex;flex-direction:column;background:var(--rail);border-right:1px solid var(--line);z-index:30}
.brand{height:67px;display:flex;align-items:center;gap:11px;padding:12px 17px;border-bottom:1px solid var(--line)}
.brand img{width:34px;height:34px;border-radius:50%;filter:var(--orb-filter);object-fit:cover}
.brand strong{font-size:13px;letter-spacing:.22em}
.rail-close{display:none;margin-left:auto;border:0;background:transparent;color:var(--muted);padding:0 8px}
.rail-primary{padding:13px 14px 10px}
.accent-button,.secondary-button,.quiet-button,.danger-button,.nav-button,.channel-row,.schedule-row,.option-card button{min-height:44px;border-radius:8px;cursor:pointer}
.accent-button,.secondary-button,.danger-button{display:inline-flex;align-items:center;justify-content:center;text-decoration:none}
.accent-button{border:1px solid var(--accent);background:var(--accent);color:var(--accent-ink);font-weight:700;padding:8px 14px}
.secondary-button{border:1px solid var(--line-strong);background:var(--accent-soft);color:var(--text);padding:8px 13px}
.quiet-button{border:1px solid transparent;background:transparent;color:var(--muted);padding:8px 10px}
.quiet-button:hover{background:var(--raised);color:var(--text)}
.danger-button{border:1px solid color-mix(in srgb,var(--bad) 55%,var(--line));background:transparent;color:var(--bad);padding:8px 12px}
.wide{width:100%}
.rail-scroll{min-height:0;flex:1;overflow:auto;padding:4px 10px 14px}
.rail-section{margin:8px 0 18px}
.rail-label{display:flex;align-items:center;justify-content:space-between;min-height:28px;padding:0 8px;color:var(--faint);font-size:10px;font-weight:750;letter-spacing:.09em;text-transform:uppercase}
.channel-list{display:flex;flex-direction:column;gap:3px}
.channel-row{width:100%;min-height:44px;border:1px solid transparent;background:transparent;text-align:left;padding:7px 9px;color:var(--body)}
.channel-row:hover{background:var(--raised)}
.channel-row[aria-current="page"]{border-color:var(--line-strong);background:var(--accent-soft);color:var(--text)}
.channel-name{display:block;font-weight:650;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.channel-meta{display:flex;justify-content:space-between;gap:8px;margin-top:2px;color:var(--faint);font-size:10px}
.channel-meta span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.empty-rail{padding:8px;color:var(--faint);font-size:12px}
.rail-navigation{flex:none;border-top:1px solid var(--line);padding:9px 10px 14px;display:grid;gap:3px}
.nav-button{width:100%;border:1px solid transparent;background:transparent;text-align:left;padding:7px 10px;color:var(--body);display:flex;align-items:center;justify-content:space-between;gap:10px}
.nav-button:hover,.nav-button[aria-current="page"]{background:var(--raised);color:var(--text)}
.count{min-width:22px;text-align:center;border:1px solid var(--line);border-radius:12px;padding:1px 6px;font:10px var(--mono);color:var(--muted)}
.count[hidden]{display:none}
.main-surface{width:100%;min-width:0;display:grid;grid-template-columns:minmax(0,1fr);grid-template-rows:58px minmax(0,1fr);background:var(--bg)}
.surface-header{display:flex;align-items:center;gap:10px;padding:0 19px;border-bottom:1px solid var(--line);background:var(--panel)}
.mobile-menu{display:none;border:1px solid var(--line);background:transparent;border-radius:8px;padding:7px 11px}
.view-tabs{display:flex;align-items:center;gap:3px}
.view-tab{border:0;background:transparent;color:var(--muted);font-weight:650;padding:7px 11px;cursor:pointer;border-radius:7px}
.view-tab[aria-current="page"]{background:var(--raised);color:var(--text)}
.header-status{margin-left:auto;min-width:0;color:var(--muted);font-size:11px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.surface-notice{position:fixed;top:68px;left:50%;z-index:60;max-width:min(560px,calc(100vw - 28px));transform:translateX(-50%);border:1px solid var(--line-strong);border-radius:8px;background:var(--panel);box-shadow:var(--shadow);padding:9px 12px;color:var(--body)}
.surface-notice[hidden]{display:none}
.view{min-height:0;overflow:hidden}
.view[hidden]{display:none}
.channel-view{width:100%;height:100%;display:grid;grid-template-columns:minmax(0,1fr);grid-template-rows:auto minmax(0,1fr) auto}
.channel-heading{min-height:76px;display:flex;align-items:center;gap:14px;padding:12px 22px;border-bottom:1px solid var(--line)}
.channel-heading-copy{min-width:0}
.channel-heading h1{margin:0;font-size:18px;line-height:1.2;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.eyebrow{margin:3px 0 0;color:var(--muted);font-size:11px}
.identity-line{margin-top:7px;color:var(--body);font-size:11px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.channel-heading-actions{margin-left:auto;display:flex;gap:3px;flex-wrap:wrap;justify-content:flex-end}
.feed{min-height:0;overflow:auto;padding:22px max(22px,calc((100% - 780px)/2)) 18px;scrollbar-gutter:stable}
.message{max-width:760px;margin:0 0 21px;display:grid;grid-template-columns:38px minmax(0,1fr);gap:11px}
.message.human{padding-left:49px;display:block}
.message img{width:38px;height:38px;border-radius:50%;filter:var(--orb-filter);object-fit:cover}
.message-byline{display:flex;align-items:baseline;gap:8px;margin-bottom:4px}
.message-byline strong{font-size:12.5px}
.message-byline time{color:var(--faint);font:10px var(--mono)}
.message-attribution{margin:0 0 5px;color:var(--faint);font:10px/1.35 var(--mono);overflow-wrap:anywhere}
.message-body{margin:0;color:var(--body);font:13px/1.5 var(--font);white-space:pre-wrap;overflow-wrap:anywhere}
.turn-status{max-width:711px;margin:-12px 0 19px 49px}
.target-state{min-height:48px;border:1px solid var(--line);border-radius:9px;background:var(--panel);padding:10px 11px}
.target-state.compact{min-height:44px;border-color:transparent;background:transparent;padding:5px 8px}
.target-state.failed,.target-state.interrupted{border-color:color-mix(in srgb,var(--bad) 68%,var(--line));background:color-mix(in srgb,var(--bad) 4%,var(--panel))}
.target-state.canceled{min-height:44px;border-color:transparent;background:transparent;padding:5px 8px}
.target-state-head{display:flex;align-items:center;gap:10px;min-height:32px}
.target-state img{width:30px;height:30px;border-radius:50%;filter:var(--orb-filter);object-fit:cover;flex:none}
.target-state.compact img{width:26px;height:26px}
.target-state img.working{animation:working-pulse 1.6s ease-in-out infinite}
.target-copy{min-width:0;flex:1}
.target-copy strong{display:block;font-size:13px}
.target-copy span{display:block;margin-top:3px;color:var(--muted);font-size:11.5px;line-height:1.45;overflow-wrap:anywhere}
.target-actions{display:flex;gap:5px;flex:none}
.target-controls{display:flex;align-items:flex-start;justify-content:flex-end;gap:7px;margin:8px 0 0 40px;flex-wrap:wrap}
.target-details{margin:0}
.target-details[open]{flex-basis:100%;border-top:1px solid var(--line);padding-top:7px}
.target-details summary{min-height:44px;width:max-content;max-width:100%;border:1px solid var(--line-strong);border-radius:8px;padding:0 11px;color:var(--text);font-size:11.5px;font-weight:700;line-height:42px;list-style-position:inside;cursor:pointer}
.target-details[open] summary{margin-left:auto}
.target-details .fact-grid{margin-top:3px}
.target-retry-note{margin:10px 0 2px;color:var(--muted);font-size:10.5px}
.target-link{color:var(--body);font:10px var(--mono);text-decoration:none}
.target-link:hover,.target-link:focus-visible{color:var(--accent);text-decoration:underline}
.target-state:target{border-color:var(--accent);box-shadow:0 0 0 2px var(--accent-soft)}
@keyframes working-pulse{50%{opacity:.62;transform:scale(.94)}}
.empty-state,.loading-state,.error-state{min-height:100%;display:grid;place-items:center;text-align:center;padding:34px;color:var(--muted)}
.empty-state h2,.error-state h2{margin:0 0 7px;color:var(--text);font-size:20px}
.empty-state p,.error-state p{max-width:500px;margin:0 auto 17px}
.state-actions{display:flex;justify-content:center;gap:8px;flex-wrap:wrap}
.composer-shell{border-top:1px solid var(--line);padding:11px max(18px,calc((100% - 800px)/2)) 15px;background:var(--panel)}
.composer-shell[hidden]{display:none}
.composer-boundary{display:flex;align-items:center;justify-content:space-between;gap:12px;margin:0 2px 7px;color:var(--faint);font-size:10.5px}
.composer-boundary button{min-height:30px;padding:3px 5px}
.composer{display:flex;align-items:flex-end;gap:9px;border:1px solid var(--line-strong);border-radius:10px;background:var(--bg);padding:8px}
.composer textarea{min-width:0;min-height:44px;max-height:150px;flex:1;resize:vertical;border:0;background:transparent;color:var(--text);padding:11px;line-height:1.4}
.composer textarea::placeholder{color:var(--faint)}
.composer-status{min-height:18px;padding:4px 3px 0;color:var(--muted);font-size:10.5px}
.composer-status.error{color:var(--bad)}
.scheduled-view{height:100%;overflow:auto;padding:24px max(20px,calc((100% - 980px)/2)) 50px}
.scheduled-heading{display:flex;align-items:flex-end;gap:12px;margin-bottom:19px}
.scheduled-heading h1{margin:0;font-size:22px}
.scheduled-heading p{margin:3px 0 0;color:var(--muted);font-size:11px}
.filter-group{margin-left:auto;display:flex;gap:4px}
.filter-button{border:1px solid var(--line);background:var(--panel);color:var(--muted);border-radius:7px;padding:7px 10px;cursor:pointer}
.filter-button[aria-pressed="true"]{border-color:var(--line-strong);background:var(--accent-soft);color:var(--text)}
.schedule-section{margin:0 0 22px}
.schedule-section h2{margin:0 0 7px;color:var(--faint);font-size:10px;letter-spacing:.08em;text-transform:uppercase}
.schedule-list{display:grid;gap:6px}
.schedule-row{width:100%;min-height:72px;border:1px solid var(--line);background:var(--panel);color:var(--body);padding:10px 12px;text-align:left;display:grid;grid-template-columns:minmax(205px,1.2fr) minmax(140px,.8fr) minmax(150px,.85fr) minmax(130px,.75fr) auto;align-items:center;gap:10px}
.schedule-row-main{display:contents}.schedule-row:has(.schedule-row-main:focus-visible){border-color:var(--accent);box-shadow:0 0 0 2px var(--accent-soft)}
.schedule-row:hover{border-color:var(--line-strong);background:var(--raised)}
.schedule-title{min-width:0}
.schedule-title strong{display:-webkit-box;color:var(--text);overflow:hidden;-webkit-box-orient:vertical;-webkit-line-clamp:2;white-space:normal;line-height:1.25}
.schedule-title span,.schedule-facts span{display:block;color:var(--muted);font-size:10.5px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.schedule-facts strong{display:block;font-size:11px;font-weight:650;color:var(--body)}
.schedule-state{font-size:11px;font-weight:700;text-transform:capitalize;color:var(--muted)}
.schedule-state span{display:block;max-width:150px;margin-top:2px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:currentColor;font-size:10px;font-weight:550;line-height:1.25;text-transform:none}
.schedule-state.failed{color:var(--bad)}
.schedule-state.running,.schedule-state.fired{color:var(--warn)}
.schedule-state.succeeded,.schedule-state.completed{color:var(--good)}
.schedule-actions{display:flex;justify-content:flex-end;gap:5px;flex-wrap:wrap}
.mobile-tabs{display:none}
.rail-backdrop{display:none}
dialog{width:min(650px,calc(100vw - 28px));max-height:min(82dvh,760px);padding:0;border:1px solid var(--line-strong);border-radius:12px;background:var(--panel);color:var(--text);box-shadow:var(--shadow)}
dialog::backdrop{background:rgba(0,7,13,.7)}
.dialog-head{position:sticky;top:0;z-index:2;display:flex;align-items:flex-start;gap:12px;padding:16px 18px;border-bottom:1px solid var(--line);background:var(--panel)}
.dialog-head-copy{min-width:0;flex:1}
.dialog-head h2{margin:0;font-size:17px}
.dialog-head p{margin:3px 0 0;color:var(--muted);font-size:11px}
.dialog-close{border:1px solid var(--line);border-radius:7px;background:transparent;color:var(--muted);padding:7px 11px}
.dialog-body{padding:17px 18px 21px;overflow:auto}
.field{display:grid;gap:6px;margin-bottom:14px}
.field label{font-size:11px;font-weight:700;color:var(--body)}
.field input,.field select{width:100%;min-height:44px;border:1px solid var(--line-strong);border-radius:8px;background:var(--bg);color:var(--text);padding:9px 11px}
.dialog-actions{display:flex;justify-content:flex-end;gap:7px;flex-wrap:wrap;margin-top:14px}
.settings-section{padding:0 0 20px;margin:0 0 20px;border-bottom:1px solid var(--line)}
.settings-section:last-child{border-bottom:0;margin-bottom:0;padding-bottom:0}
.settings-title{display:flex;align-items:center;justify-content:space-between;gap:10px;margin:0 0 9px}
.settings-title h3{margin:0;font-size:13px}
.settings-copy{margin:0 0 12px;color:var(--muted);font-size:11.5px}
.machine-group{margin:13px 0}
.machine-group h4{margin:0 0 7px;color:var(--body);font-size:11px}
.option-card,.needs-card,.archive-row,.identity-card,.occurrence-card{border:1px solid var(--line);border-radius:9px;background:var(--bg);padding:11px;margin:7px 0}
.option-head,.needs-head,.archive-row,.occurrence-head{display:flex;align-items:flex-start;gap:10px}
.option-head strong,.needs-head strong,.occurrence-head strong{min-width:0;flex:1}
.state-label{color:var(--muted);font-size:10px;text-transform:capitalize}
.state-label.ready,.state-label.answered,.state-label.succeeded,.state-label.completed{color:var(--good)}
.state-label.failed,.state-label.unavailable,.state-label.ineligible{color:var(--bad)}
.state-label.working,.state-label.queued,.state-label.setup-required{color:var(--warn)}
.fact-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:7px 14px;margin:10px 0 0}
.fact-grid div{min-width:0}
.fact-grid dt{color:var(--faint);font-size:9.5px;text-transform:uppercase;letter-spacing:.05em}
.fact-grid dd{margin:2px 0 0;color:var(--body);font:10.5px/1.35 var(--mono);overflow-wrap:anywhere}
.card-note{margin:8px 0 0;color:var(--muted);font-size:11px;overflow-wrap:anywhere}
.card-actions{display:flex;justify-content:flex-end;gap:6px;margin-top:9px;flex-wrap:wrap}
.inventory-list{display:grid;gap:7px;margin-top:10px}
.inventory-row{border:1px solid var(--line);border-radius:8px;background:var(--bg);padding:9px 10px}
.inventory-row h4{margin:0;color:var(--text);font-size:11px}
.archive-row{align-items:center}
.archive-row span{min-width:0;flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.identity-orb{width:44px;height:44px;border-radius:50%;filter:var(--orb-filter);object-fit:cover;float:left;margin:0 12px 7px 0}
.boundary-copy{clear:both;border-left:3px solid var(--accent);padding:8px 10px;margin:15px 0;background:var(--accent-soft);color:var(--body);font-size:11.5px}
.needs-card p,.occurrence-card p{margin:5px 0;color:var(--muted);font-size:11px;overflow-wrap:anywhere}
.occurrence-section h3{margin:18px 0 7px;color:var(--faint);font-size:10px;letter-spacing:.08em;text-transform:uppercase}
.occurrence-head time{margin-left:auto;color:var(--faint);font:10px var(--mono)}
.inline-error{color:var(--bad);font-size:11px;margin:7px 0}
.visually-hidden{position:absolute!important;width:1px!important;height:1px!important;padding:0!important;margin:-1px!important;overflow:hidden!important;clip:rect(0,0,0,0)!important;white-space:nowrap!important;border:0!important}
[hidden]{display:none!important}
@media (max-width:860px){
  .app-shell{display:block;height:100dvh;padding-bottom:60px}
  .channel-rail{position:fixed;inset:0 auto 0 0;width:min(310px,88vw);box-shadow:var(--shadow);transform:translateX(-105%);transition:transform .18s ease}
  .channel-rail[data-open="true"]{transform:translateX(0)}
  .rail-close,.mobile-menu{display:inline-flex;align-items:center;justify-content:center}
  .rail-backdrop{position:fixed;inset:0;z-index:25;border:0;background:rgba(0,7,13,.65);min-height:0;width:100%}
  .rail-backdrop[data-open="true"]{display:block}
  .main-surface{height:calc(100dvh - 60px);grid-template-rows:56px minmax(0,1fr)}
  .surface-header{padding:0 11px}
  .view-tabs{display:none}
  .channel-heading{min-height:74px;padding:10px 13px;align-items:flex-start}
  .channel-heading-actions{gap:0}
  .channel-heading-actions .quiet-button{font-size:11px;padding:6px 7px;min-height:38px}
  .identity-line{max-width:calc(100vw - 36px)}
  .feed{padding:16px 14px 10px}
  .message{grid-template-columns:32px minmax(0,1fr);gap:9px}
  .message img{width:32px;height:32px}
  .message.human{padding-left:41px}
  .turn-status{max-width:none;margin:-8px 0 17px}
  .target-state-head{align-items:flex-start;flex-wrap:wrap}
  .target-copy{min-width:calc(100% - 42px)}
  .target-actions{width:100%;justify-content:flex-start}
  .target-actions button{flex:1}
  .target-controls{margin-left:0}
  .target-details{flex-basis:100%}
  .target-details summary{width:100%;padding:0 12px;text-align:center}
  .composer-shell{padding:9px 10px 10px}
  .composer-boundary{font-size:10px}
  .scheduled-view{padding:18px 11px 34px}
  .scheduled-heading{align-items:flex-start;flex-direction:column}
  .filter-group{margin-left:0;width:100%}
  .filter-button{flex:1}
  .schedule-row{grid-template-columns:minmax(0,1fr) auto;gap:7px 12px}
  .schedule-facts{grid-column:1/-1;display:grid;grid-template-columns:1fr 1fr;gap:10px}
  .schedule-actions{grid-column:1/-1;justify-content:flex-start}
  .mobile-tabs{position:fixed;inset:auto 0 0;z-index:20;height:60px;display:grid;grid-template-columns:repeat(4,1fr);border-top:1px solid var(--line);background:var(--panel);padding-bottom:env(safe-area-inset-bottom)}
  .mobile-tabs button{border:0;background:transparent;color:var(--muted);font-size:10px;padding:5px 2px}
  .mobile-tabs button[aria-current="page"]{color:var(--accent);font-weight:750}
  dialog{max-height:88dvh}
  .fact-grid{grid-template-columns:1fr}
  .target-details .fact-grid div{display:grid;grid-template-columns:minmax(92px,.72fr) minmax(0,1.28fr);gap:10px;padding:7px 0;border-bottom:1px solid var(--line)}
  .target-details .fact-grid div:last-child{border-bottom:0}
  .target-details .fact-grid dd{margin:0}
}
@media (max-width:430px){
  .channel-heading h1{font-size:16px}
  .channel-heading-actions{max-width:150px}
  .channel-heading-actions .quiet-button{min-width:44px}
  .composer{padding:6px}
  .composer textarea{padding:9px 7px}
  .composer .accent-button{padding:8px 12px}
}
@media (prefers-reduced-motion:reduce){
  *,*::before,*::after{scroll-behavior:auto!important;animation-duration:.001ms!important;animation-iteration-count:1!important;transition-duration:.001ms!important}
}
</style>
</head>
<body data-theme="quiet-intelligence">
<a class="skip-link" href="#primary-content">Skip to content</a>
<div class="app-shell">
  <aside class="channel-rail" id="channel-rail" data-open="false" aria-label="Private Channels">
    <div class="brand">
      <img src="/fort-agent-orb.png" alt="Fort orbital core">
      <strong>FORT</strong>
      <button class="rail-close" id="rail-close" type="button">Close</button>
    </div>
    <div class="rail-primary">
      <button class="accent-button wide" id="new-channel-button" type="button">New Channel</button>
    </div>
    <div class="rail-scroll">
      <section class="rail-section" aria-labelledby="pinned-label">
        <div class="rail-label" id="pinned-label">Pinned</div>
        <div class="channel-list" id="pinned-channels" aria-live="polite"><p class="empty-rail">Loading Channels…</p></div>
      </section>
      <section class="rail-section" aria-labelledby="recent-label">
        <div class="rail-label" id="recent-label">Recent</div>
        <div class="channel-list" id="recent-channels" aria-live="polite"><p class="empty-rail">Loading Channels…</p></div>
      </section>
    </div>
    <nav class="rail-navigation" aria-label="Fort destinations">
      <button class="nav-button" id="scheduled-navigation" type="button"><span>Scheduled</span><span class="count" id="schedule-count" hidden></span></button>
      <button class="nav-button" id="needs-you-navigation" type="button"><span>Needs you</span><span class="count" id="needs-you-count" hidden></span></button>
      <button class="nav-button" id="settings-navigation" type="button"><span>Settings</span></button>
    </nav>
  </aside>
  <button class="rail-backdrop" id="rail-backdrop" type="button" data-open="false" aria-label="Close Channels menu"></button>
  <main class="main-surface" id="primary-content" tabindex="-1">
    <header class="surface-header">
      <button class="mobile-menu" id="mobile-menu" type="button" aria-controls="channel-rail" aria-expanded="false">Menu</button>
      <nav class="view-tabs" aria-label="Primary destinations">
        <button class="view-tab" id="channels-tab" type="button" aria-current="page">Channels</button>
        <button class="view-tab" id="scheduled-tab" type="button">Scheduled</button>
      </nav>
      <p class="header-status" id="header-status" aria-live="polite">Loading current state…</p>
    </header>
    <section class="view channel-view" id="channels-view" aria-labelledby="channel-title">
      <header class="channel-heading" id="channel-heading" hidden>
        <div class="channel-heading-copy">
          <h1 id="channel-title">Private Channel</h1>
          <p class="eyebrow">Private Channel · one immutable Primary Agent</p>
          <p class="identity-line" id="identity-summary"></p>
        </div>
        <div class="channel-heading-actions">
          <button class="quiet-button" id="identity-button" type="button">Identity</button>
          <button class="quiet-button" id="rename-channel-button" type="button">Rename</button>
          <button class="quiet-button" id="pin-channel-button" type="button">Pin</button>
          <button class="quiet-button" id="archive-channel-button" type="button">Archive</button>
        </div>
      </header>
      <div class="feed" id="channel-feed" role="log" aria-live="off" aria-relevant="additions text">
        <div class="loading-state"><p>Loading private Channels…</p></div>
      </div>
      <div id="target-status-announcer" class="visually-hidden" role="status" aria-live="polite"></div>
      <div class="composer-shell" id="composer-shell" hidden>
        <div class="composer-boundary">
          <span>Text-only chat · this Channel context only</span>
          <button class="quiet-button" id="boundary-button" type="button">Read boundary</button>
        </div>
        <form class="composer" id="channel-composer">
          <label class="visually-hidden" for="composer-input">Message this Channel's Primary Agent</label>
          <textarea id="composer-input" name="message" rows="1" maxlength="65536" placeholder="Message this private Channel" required></textarea>
          <button class="accent-button" id="send-button" type="submit">Send</button>
        </form>
        <div class="composer-status" id="composer-status" aria-live="polite"></div>
      </div>
    </section>
    <section class="view scheduled-view" id="scheduled-view" aria-labelledby="scheduled-title" hidden>
      <header class="scheduled-heading">
        <div>
          <h1 id="scheduled-title">Scheduled</h1>
          <p id="schedule-freshness">Loading durable schedule definitions…</p>
        </div>
        <div class="filter-group" role="group" aria-label="Filter schedules">
          <button class="filter-button" type="button" data-schedule-filter="all" aria-pressed="true">All</button>
          <button class="filter-button" type="button" data-schedule-filter="active" aria-pressed="false">Active</button>
          <button class="filter-button" type="button" data-schedule-filter="paused" aria-pressed="false">Paused</button>
        </div>
      </header>
      <div id="schedule-content" aria-live="polite"><div class="loading-state"><p>Loading schedules…</p></div></div>
    </section>
  </main>
</div>
<nav class="mobile-tabs" aria-label="Primary destinations on phone">
  <button id="mobile-channels-tab" type="button" aria-current="page">Channels</button>
  <button id="mobile-scheduled-tab" type="button">Scheduled</button>
  <button id="mobile-needs-tab" type="button">Needs you</button>
  <button id="mobile-settings-tab" type="button">Settings</button>
</nav>
<div class="surface-notice" id="surface-notice" role="status" aria-live="polite" hidden></div>

<dialog id="new-channel-dialog" aria-modal="true" aria-labelledby="new-channel-title">
  <div class="dialog-head"><div class="dialog-head-copy"><h2 id="new-channel-title">New private Channel</h2><p>Its Primary Agent identity is fixed when the Channel is created.</p></div><button class="dialog-close" type="button" data-close-dialog>Close</button></div>
  <form class="dialog-body" id="new-channel-form">
    <div class="field"><label for="new-channel-name">Channel name</label><input id="new-channel-name" name="name" maxlength="120" autocomplete="off" required></div>
    <p class="inline-error" id="new-channel-error" aria-live="polite" hidden></p>
    <div class="dialog-actions"><button class="quiet-button" type="button" data-close-dialog>Cancel</button><button class="accent-button" id="create-channel-button" type="submit">Create Channel</button></div>
  </form>
</dialog>

<dialog id="rename-channel-dialog" aria-modal="true" aria-labelledby="rename-channel-title">
  <div class="dialog-head"><div class="dialog-head-copy"><h2 id="rename-channel-title">Rename Channel</h2><p>The stored Primary Agent identity does not change.</p></div><button class="dialog-close" type="button" data-close-dialog>Close</button></div>
  <form class="dialog-body" id="rename-channel-form">
    <div class="field"><label for="rename-channel-name">Channel name</label><input id="rename-channel-name" name="name" maxlength="120" autocomplete="off" required></div>
    <p class="inline-error" id="rename-channel-error" aria-live="polite" hidden></p>
    <div class="dialog-actions"><button class="quiet-button" type="button" data-close-dialog>Cancel</button><button class="accent-button" id="save-channel-name" type="submit">Save</button></div>
  </form>
</dialog>

<dialog id="identity-dialog" aria-modal="true" aria-labelledby="identity-title">
  <div class="dialog-head"><div class="dialog-head-copy"><h2 id="identity-title">Primary Agent identity</h2><p>The exact identity and text-only boundary stored with this Channel.</p></div><button class="dialog-close" type="button" data-close-dialog>Close</button></div>
  <div class="dialog-body" id="identity-content"></div>
</dialog>

<dialog id="needs-you-dialog" aria-modal="true" aria-labelledby="needs-you-title">
  <div class="dialog-head"><div class="dialog-head-copy"><h2 id="needs-you-title">Needs you</h2><p>Current recoverable failures in open private Channels.</p></div><button class="dialog-close" type="button" data-close-dialog>Close</button></div>
  <div class="dialog-body" id="needs-you-content" aria-live="polite"></div>
</dialog>

<dialog id="settings-dialog" aria-modal="true" aria-labelledby="settings-title">
  <div class="dialog-head"><div class="dialog-head-copy"><h2 id="settings-title">Settings</h2><p>Choose the exact Primary Agent for Channels created from now on.</p></div><button class="dialog-close" type="button" data-close-dialog>Close</button></div>
  <div class="dialog-body">
    <section class="settings-section" aria-labelledby="appearance-title">
      <div class="settings-title"><h3 id="appearance-title">Appearance</h3></div>
      <p class="settings-copy">This theme is stored only in this browser. It never changes Channel identity or provider input.</p>
      <div class="field"><label for="theme-select">Presentation theme</label><select id="theme-select"><option value="quiet-intelligence">Quiet Intelligence</option><option value="private-channels">Private Channels</option><option value="native-daylight">Native Daylight</option></select></div>
    </section>
    <section class="settings-section" aria-labelledby="primary-agent-settings-title">
      <div class="settings-title"><h3 id="primary-agent-settings-title">Primary Agent</h3><button class="secondary-button" id="recheck-agent-button" type="button">Recheck</button></div>
      <p class="settings-copy" id="settings-agent-state">Loading current selection and eligible options…</p>
      <div id="primary-agent-options" aria-live="polite"></div>
      <div class="dialog-actions"><button class="danger-button" id="clear-primary-agent" type="button" hidden>Clear default</button></div>
    </section>
    <section class="settings-section" aria-labelledby="schedule-inventory-title">
      <div class="settings-title"><h3 id="schedule-inventory-title">Schedule inventory</h3></div>
      <p class="settings-copy">Primary promotion requires the exact reviewed digest for every enabled durable definition and loaded flow.</p>
      <div id="schedule-inventory" aria-live="polite"><p class="settings-copy">Loading the read-only inventory boundary…</p></div>
    </section>
    <section class="settings-section" aria-labelledby="archived-title">
      <div class="settings-title"><h3 id="archived-title">Archived Channels</h3><button class="quiet-button" id="refresh-archived-button" type="button">Refresh</button></div>
      <p class="settings-copy">Reopening changes presentation state only. The stored Primary Agent identity remains fixed.</p>
      <div id="archived-channels" aria-live="polite"><p class="settings-copy">Open Settings to load archived Channels.</p></div>
    </section>
  </div>
</dialog>

<dialog id="schedule-detail-dialog" aria-modal="true" aria-labelledby="schedule-detail-title">
  <div class="dialog-head"><div class="dialog-head-copy"><h2 id="schedule-detail-title">Schedule</h2><p id="schedule-detail-subtitle">Loading durable schedule evidence…</p></div><button class="dialog-close" type="button" data-close-dialog>Close</button></div>
  <div class="dialog-body" id="schedule-detail-content" aria-live="polite"></div>
</dialog>

<script>
'use strict';
const API = Object.freeze({
  agent:'/api/settings/primary-agent',
  recheck:'/api/settings/primary-agent/recheck',
  channels:'/api/channels',
  needs:'/api/needs-you',
  schedules:'/api/schedules',
  scheduleList:'/api/schedules?state=',
  scheduleDetail:'/api/schedules/'
});
const THEME_KEY='fort.primary.theme.v1';
const THEMES=Object.freeze(['quiet-intelligence','private-channels','native-daylight']);
const app={
  agent:null,channels:[],needs:[],schedules:[],activeSummary:null,activeDetail:null,
  view:'channels',scheduleFilter:'all',eventSource:null,eventChannelID:'',channelRequest:0,
  sending:false,creating:false,pendingTurn:null,refreshTimer:0,targetAnnouncement:''
};
const byId=id=>document.getElementById(id);
const node=(tag,className,text)=>{const value=document.createElement(tag);if(className)value.className=className;if(text!==undefined)value.textContent=String(text);return value};
const clear=value=>{while(value.firstChild)value.removeChild(value.firstChild)};
const list=value=>Array.isArray(value)?value:[];
const first=(...values)=>values.find(value=>value!==undefined&&value!==null&&value!=='');
const errorText=error=>error&&error.message?error.message:'The request could not be completed.';
const pretty=value=>String(value||'unknown').replaceAll('_',' ').replaceAll('-',' ');
const titleCase=value=>pretty(value).replace(/\b\w/g,letter=>letter.toUpperCase());
const formatTime=value=>{
  if(!value)return 'Not recorded';
  const parsed=new Date(value);
  if(Number.isNaN(parsed.getTime()))return String(value);
  return new Intl.DateTimeFormat(undefined,{dateStyle:'medium',timeStyle:'short'}).format(parsed);
};
const relativeTime=value=>{
  if(!value)return 'No activity yet';
  const parsed=new Date(value);if(Number.isNaN(parsed.getTime()))return 'Activity recorded';
  const seconds=Math.round((parsed.getTime()-Date.now())/1000);const amount=Math.abs(seconds);
  const formatter=new Intl.RelativeTimeFormat(undefined,{numeric:'auto'});
  if(amount<60)return formatter.format(seconds,'second');
  if(amount<3600)return formatter.format(Math.round(seconds/60),'minute');
  if(amount<86400)return formatter.format(Math.round(seconds/3600),'hour');
  return formatter.format(Math.round(seconds/86400),'day');
};
async function request(path,options={}){
  const init={...options,headers:{Accept:'application/json',...(options.headers||{})}};
  if(init.body&&!init.headers['Content-Type'])init.headers['Content-Type']='application/json';
  const response=await fetch(path,init);
  const raw=await response.text();let payload=null;
  if(raw){try{payload=JSON.parse(raw)}catch(_){payload=null}}
  if(!response.ok){const message=first(payload&&payload.error,payload&&payload.message,response.statusText,'Request failed');const failure=new Error(message);failure.status=response.status;failure.code=payload&&payload.code;throw failure}
  return payload;
}
function announce(message,tone='normal'){
  const notice=byId('surface-notice');notice.textContent=message;notice.hidden=!message;
  notice.style.color=tone==='error'?'var(--bad)':'var(--body)';
  window.clearTimeout(announce.timer);if(message)announce.timer=window.setTimeout(()=>{notice.hidden=true},5000);
}
function applyTheme(value,persist){
  const theme=THEMES.includes(value)?value:THEMES[0];
  document.body.dataset.theme=theme;byId('theme-select').value=theme;
  if(persist){try{localStorage.setItem(THEME_KEY,theme)}catch(_){}}
}
function loadTheme(){
  let theme='';try{theme=localStorage.getItem(THEME_KEY)||''}catch(_){}
  applyTheme(theme,false);
}
function dialogOpen(id){const value=byId(id);if(!value.open)value.showModal()}
function dialogClose(value){if(value&&value.open)value.close()}
function setRail(open){
  const rail=byId('channel-rail');const compact=window.matchMedia('(max-width:860px)').matches;const hadFocus=rail.contains(document.activeElement);rail.dataset.open=String(open);byId('rail-backdrop').dataset.open=String(open);byId('mobile-menu').setAttribute('aria-expanded',String(open));
  rail.inert=compact&&!open;
  if(compact&&open){rail.setAttribute('role','dialog');rail.setAttribute('aria-modal','true');byId('primary-content').inert=true;document.querySelector('.mobile-tabs').inert=true;byId('rail-close').focus()}
  else{rail.removeAttribute('role');rail.removeAttribute('aria-modal');byId('primary-content').inert=false;document.querySelector('.mobile-tabs').inert=false;if(compact&&hadFocus)byId('mobile-menu').focus()}
}
function trapRailFocus(event){
  if(event.key!=='Tab'||byId('channel-rail').dataset.open!=='true')return;const controls=[...byId('channel-rail').querySelectorAll('button:not([disabled]),a[href],input:not([disabled]),select:not([disabled]),textarea:not([disabled])')].filter(control=>control.getClientRects().length);if(!controls.length)return;const firstControl=controls[0];const lastControl=controls[controls.length-1];if(event.shiftKey&&document.activeElement===firstControl){event.preventDefault();lastControl.focus()}else if(!event.shiftKey&&document.activeElement===lastControl){event.preventDefault();firstControl.focus()}
}
function normalizeAgent(payload){
  const value=payload||{};
  const direct=value.option_id?value:null;
  return {selection:first(value.selection,value.setting,value.primary_agent,direct,null),options:list(first(value.options,value.items,[])),state:first(value.state,value.readiness&&value.readiness.state,''),reason:first(value.reason,value.readiness&&value.readiness.reason,''),scheduleInventory:first(value.schedule_inventory,null)};
}
function agentConfigured(){return Boolean(app.agent&&app.agent.selection&&app.agent.selection.option_id)}
function channelItems(payload){return Array.isArray(payload)?payload:list(first(payload&&payload.items,payload&&payload.channels,[]))}
function needsItems(payload){return Array.isArray(payload)?payload:list(first(payload&&payload.items,payload&&payload.needs_you,[]))}
function scheduleItems(payload){return list(first(payload&&payload.items,Array.isArray(payload)?payload:[]))}
function channelConversation(item){return first(item&&item.conversation,item,{})}
function channelID(item){const conversation=channelConversation(item);return first(conversation.id,item&&item.id,item&&item.conversation_id,'')}
function channelName(item){const conversation=channelConversation(item);return first(conversation.title,conversation.name,item&&item.title,item&&item.name,'Untitled Channel')}
function channelUpdated(item){const conversation=channelConversation(item);return first(conversation.updated_at,item&&item.updated_at,conversation.created_at,item&&item.created_at,'')}
function channelIsPinned(item){return Boolean(item&&first(item.pinned,item.is_pinned,false))}
function channelState(item){const conversation=channelConversation(item);return first(conversation.state,item&&item.state,'open')}
function participantFor(detail,summary){return first(detail&&detail.participant,list(detail&&detail.participants)[0],summary&&summary.participant,{})}
function identityFor(detail,summary){return first(detail&&detail.primary_identity,detail&&detail.identity,summary&&summary.primary_identity,summary&&summary.identity,{})}
function policyFor(detail,summary){const identity=identityFor(detail,summary);return first(identity.policy,detail&&detail.policy,{})}
function selectedModel(participant,identity){return first(participant&&participant.model,identity&&identity.requested_model,identity&&identity.seat&&identity.seat.model,'unknown')}
function primaryLabel(participant,identity){
  const model=selectedModel(participant,identity);
  return first(participant&&participant.display_name,identity&&identity.seat&&identity.seat.display_name,model==='unknown'?'Primary Agent':'Codex '+titleCase(model));
}
function channelReadinessState(detail){
  return String(first(detail&&detail.primary_state,detail&&detail.readiness&&detail.readiness.state,'')).toLowerCase();
}
function channelReadinessLabel(detail){
  const state=channelReadinessState(detail);
  return state?titleCase(state):'Stored identity';
}
function channelCanSend(detail){return channelReadinessState(detail)==='ready'}
function markChannelAuthorityDrifted(detail){
  if(!detail)return;
  detail.primary_state='drifted';
  detail.readiness={...(detail.readiness||{}),state:'drifted',reason:'primary_agent_drift'};
}
function channelComposerBlockMessage(detail){
  const state=channelReadinessState(detail);
  if(state==='drifted')return "This Channel's saved Primary Agent authority no longer matches the current verified authority. Create a new Channel to continue.";
  return 'The Primary Agent is '+titleCase(state||'unready')+' for this Channel. Recheck Settings before sending.';
}
function renderHeaderStatus(){
  const status=byId('header-status');
  if(!app.agent){status.textContent='Primary Agent state unavailable';return}
  if(!agentConfigured()){status.textContent='Choose a Primary Agent to create a private Channel';return}
  const selection=app.agent.selection;const seat=selection.seat||{};const policy=selection.policy||{};
  status.textContent=[primaryLabel(seat,selection),policy.account_plan?'ChatGPT '+titleCase(policy.account_plan):'',seat.machine||'',titleCase(app.agent.state||seat.state||'stored')].filter(Boolean).join(' · ');
}
async function loadAgent(){
  try{app.agent=normalizeAgent(await request(API.agent));renderHeaderStatus();renderAgentOptions();return app.agent}
  catch(error){app.agent=null;renderHeaderStatus();renderAgentOptions(error);throw error}
}
function renderRail(){
  const pinned=byId('pinned-channels');const recent=byId('recent-channels');clear(pinned);clear(recent);
  const pinnedItems=app.channels.filter(channelIsPinned);const recentItems=app.channels.filter(item=>!channelIsPinned(item));
  renderChannelGroup(pinned,pinnedItems,'No pinned Channels');renderChannelGroup(recent,recentItems,'No recent Channels');
}
function renderChannelGroup(container,items,emptyText){
  if(!items.length){container.append(node('p','empty-rail',emptyText));return}
  // The API supplies pinned-first/newest-first canonical order; the client preserves it.
  items.forEach(item=>{
    const button=node('button','channel-row');button.type='button';const id=channelID(item);button.dataset.channelId=id;
    button.setAttribute('aria-current',app.activeSummary&&channelID(app.activeSummary)===id?'page':'false');
    button.append(node('span','channel-name',channelName(item)));
    const meta=node('span','channel-meta');meta.append(node('span','',channelIsPinned(item)?'Pinned':'Private'));meta.append(node('span','',relativeTime(channelUpdated(item))));button.append(meta);
    button.addEventListener('click',()=>selectChannel(id));container.append(button);
  });
}
async function loadChannels(preferredID=''){
  try{
    const payload=await request(API.channels+'?state=open');app.channels=channelItems(payload);renderRail();
    const current=preferredID||app.activeSummary&&channelID(app.activeSummary);const next=app.channels.find(item=>channelID(item)===current)||app.channels[0]||null;
    if(next){app.activeSummary=next;renderRail();await loadChannel(channelID(next))}else{app.activeSummary=null;app.activeDetail=null;disconnectEvents();renderEmptyChannel()}
    return app.channels;
  }catch(error){app.channels=[];app.activeSummary=null;app.activeDetail=null;renderRail();renderChannelError(error);throw error}
}
function setChannelLoading(){
  byId('channel-heading').hidden=true;byId('composer-shell').hidden=true;const feed=byId('channel-feed');clear(feed);const state=node('div','loading-state');state.append(node('p','', 'Loading this private Channel…'));feed.append(state);
}
async function loadChannel(id){
	return updateChannelDetail(id,true,true);
}
async function refreshChannelDetail(id){
	const focused=focusedTargetControl();const detail=await updateChannelDetail(id,false,false);if(focused.targetID)focusTarget(focused.targetID,false,focused.control);return detail;
}
async function updateChannelDetail(id,showLoading,connect){
  const token=++app.channelRequest;if(showLoading)setChannelLoading();
  try{
    const detail=await request(API.channels+'/'+encodeURIComponent(id));if(token!==app.channelRequest)return;
    app.activeDetail=detail&&detail.detail?detail.detail:detail;renderActiveChannel();if(connect)connectEvents(id);return app.activeDetail;
  }catch(error){if(token!==app.channelRequest)return;if(showLoading){app.activeDetail=null;renderChannelError(error,()=>loadChannel(id))}else{setComposerStatus('Live refresh failed: '+errorText(error),true)}throw error}
}
function renderEmptyChannel(){
  byId('channel-heading').hidden=true;byId('composer-shell').hidden=true;const feed=byId('channel-feed');clear(feed);const wrap=node('div','empty-state');const copy=node('div');
  if(!agentConfigured()){
    copy.append(node('h2','', 'Private Channels begin with one exact agent'));
    copy.append(node('p','', 'Choose a ready ChatGPT subscription-backed Primary Agent. Every Channel created afterward keeps that exact identity.'));
    const action=node('button','accent-button','Choose Primary Agent');action.type='button';action.addEventListener('click',openSettings);copy.append(action);
  }else{
    copy.append(node('h2','', 'No open Channels'));
    copy.append(node('p','', 'Create a private Channel to start a durable text-only conversation with your configured Primary Agent.'));
    const action=node('button','accent-button','New Channel');action.type='button';action.addEventListener('click',openNewChannel);copy.append(action);
  }
  wrap.append(copy);feed.append(wrap);
}
function renderChannelError(error,retry=()=>loadChannels()){
  byId('channel-heading').hidden=true;byId('composer-shell').hidden=true;const feed=byId('channel-feed');clear(feed);const wrap=node('div','error-state');const copy=node('div');copy.append(node('h2','', 'This Channel is unavailable'));copy.append(node('p','',errorText(error)));const action=node('button','secondary-button','Retry');action.type='button';action.addEventListener('click',retry);copy.append(action);wrap.append(copy);feed.append(wrap);
}
function activeConversation(){return first(app.activeDetail&&app.activeDetail.conversation,channelConversation(app.activeSummary),{})}
function activeChannelID(){return first(activeConversation().id,app.activeSummary&&channelID(app.activeSummary),'')}
function renderActiveChannel(){
  const conversation=activeConversation();const participant=participantFor(app.activeDetail,app.activeSummary);const identity=identityFor(app.activeDetail,app.activeSummary);const policy=policyFor(app.activeDetail,app.activeSummary);
  byId('channel-heading').hidden=false;byId('channel-title').textContent=first(conversation.title,conversation.name,channelName(app.activeSummary));
  const readiness=channelReadinessLabel(app.activeDetail);
  const canSend=channelCanSend(app.activeDetail);
  byId('identity-summary').textContent=[primaryLabel(participant,identity),policy.account_plan?'ChatGPT '+titleCase(policy.account_plan):'',participant.machine||identity.seat&&identity.seat.machine||'',readiness].filter(Boolean).join(' · ');
  byId('pin-channel-button').textContent=channelIsPinned(app.activeSummary)?'Unpin':'Pin';
  byId('archive-channel-button').disabled=channelState(app.activeSummary)==='archived';
  renderMessages(participant);renderIdentity();byId('composer-shell').hidden=channelState(app.activeSummary)!=='open';byId('composer-input').disabled=!canSend;byId('send-button').disabled=!canSend;
  const status=byId('composer-status');
  if(!canSend){setComposerStatus(channelComposerBlockMessage(app.activeDetail),true);status.dataset.authorityBlock='true'}
  else if(status.dataset.authorityBlock==='true'){setComposerStatus('');delete status.dataset.authorityBlock}
}
function answerTarget(message){
  const targetID=message&&message.target_id;if(!targetID)return null;
  return list(app.activeDetail&&app.activeDetail.targets).find(target=>String(target.id)===String(targetID))||null;
}
function answerAttribution(message,fallbackParticipant){
  const target=answerTarget(message);if(!target)return {label:primaryLabel(fallbackParticipant,identityFor(app.activeDetail,app.activeSummary)),detail:''};
  const authority=target.authority||{};const receipt=target.receipt||{};const participant=list(app.activeDetail&&app.activeDetail.participants).find(value=>String(value.id)===String(target.participant_id))||fallbackParticipant;
  const resolved=String(first(receipt.resolved_model,''));const model=resolved&&resolved!=='unknown'?resolved:first(authority.requested_model,participant&&participant.model,'unknown');
  const label=primaryLabel({...participant,model},{...authority,requested_model:model});
  const detail=['Attempt '+String(target.attempt||1),model!=='unknown'?model:'',receipt.provider_terminal_status].filter(Boolean).join(' · ');
  return {label,detail};
}
function renderMessages(participant){
  const feed=byId('channel-feed');const wasAtBottom=feed.scrollHeight-feed.scrollTop-feed.clientHeight<=48;const previousScrollTop=feed.scrollTop;const openDetails=new Set([...feed.querySelectorAll('.target-state details[open]')].map(details=>String(details.closest('.target-state')&&details.closest('.target-state').dataset.targetId||'')));clear(feed);const raw=list(app.activeDetail&&app.activeDetail.messages);const seen=new Set();const targets=list(app.activeDetail&&app.activeDetail.targets).slice().sort((a,b)=>String(a.created_at||'').localeCompare(String(b.created_at||''))||Number(a.attempt||0)-Number(b.attempt||0));const turns=list(app.activeDetail&&app.activeDetail.turns);const renderedTargets=new Set();
  const messages=raw.slice().sort((a,b)=>Number(a.id||0)-Number(b.id||0)||String(a.created_at||'').localeCompare(String(b.created_at||''))).filter(message=>{const key=first(message.id,[message.author_kind,message.author_id,message.created_at,message.body].join('|'));if(seen.has(key))return false;seen.add(key);return true});
  if(!messages.length&&!targets.length){const empty=node('div','empty-state');const copy=node('div');copy.append(node('h2','', 'A quiet place for one durable conversation'));copy.append(node('p','', 'Messages stay in this private Channel and every answer keeps its exact Primary Agent attribution.'));empty.append(copy);feed.append(empty)}
  messages.forEach(message=>{
    const kind=String(message.author_kind||'system');const article=node('article','message '+(kind==='human'?'human':'agent'));
    if(kind!=='human'){const image=node('img');image.src='/fort-agent-orb.png';image.alt=kind==='agent'?'Primary Agent orbital core':'Fort orbital core';article.append(image)}
    const copy=node('div');const byline=node('div','message-byline');const attribution=kind==='agent'?answerAttribution(message,participant):null;const author=kind==='human'?'You':kind==='agent'?attribution.label:'Fort';byline.append(node('strong','',author));const time=node('time','',formatTime(message.created_at));if(message.created_at)time.dateTime=message.created_at;byline.append(time);copy.append(byline);if(attribution&&attribution.detail)copy.append(node('p','message-attribution',attribution.detail));copy.append(node('p','message-body',message.body||''));article.append(copy);feed.append(article);
    if(kind==='human')latestTargetsForTurn(turnIDForMessage(message,turns),targets).forEach(target=>{renderedTargets.add(String(target.id||''));const status=renderTargetState(target,targets,openDetails);if(status)feed.append(status)});
  });
  targets.filter(target=>isLatestTargetAttempt(target,targets)&&!renderedTargets.has(String(target.id||''))).forEach(target=>{const status=renderTargetState(target,targets,openDetails);if(status)feed.append(status)});
  announceLatestTargetStatus(targets);
  window.requestAnimationFrame(()=>{feed.scrollTop=wasAtBottom?feed.scrollHeight:previousScrollTop});
}
function targetAnchorID(targetID){return 'target-'+encodeURIComponent(String(targetID||''))}
function focusedTargetControl(){const active=document.activeElement;const target=active&&active.closest?active.closest('.target-state'):null;return target?{targetID:String(target.dataset.targetId||''),control:String(active.dataset&&active.dataset.targetFocus||'card')}:{targetID:'',control:'card'}}
function focusTarget(targetID,scroll,control='card'){const target=[...document.querySelectorAll('.target-state')].find(value=>value.dataset.targetId===String(targetID));if(!target)return false;const focus=control==='card'?target:target.querySelector('[data-target-focus="'+control+'"]')||target;focus.tabIndex=focus===target?-1:focus.tabIndex;focus.focus({preventScroll:true});if(scroll)window.requestAnimationFrame(()=>target.scrollIntoView({block:'center'}));return true}
function isLatestTargetAttempt(target,targets){
  return !targets.some(candidate=>String(candidate.turn_id)===String(target.turn_id)&&String(candidate.participant_id)===String(target.participant_id)&&Number(candidate.attempt||0)>Number(target.attempt||0));
}
function turnIDForMessage(message,turns){const direct=String(message&&message.turn_id||'');if(direct)return direct;const turn=turns.find(value=>Number(value.prompt_message_id)===Number(message&&message.id));return String(turn&&turn.id||'')}
function latestTargetsForTurn(turnID,targets){if(!turnID)return [];return targets.filter(target=>String(target.turn_id)===String(turnID)&&isLatestTargetAttempt(target,targets))}
function failedRecoveryAction(code){code=String(code||'');if(['primary_agent_unready','primary_agent_drift','chat_policy_unavailable','seat_unready'].includes(code))return 'Recheck and retry';if(['daemon_interrupted','provider_result_unknown','provider_incomplete','provider_failed'].includes(code))return 'Retry';return ''}
function targetPresentation(target,machine){const state=String(target&&target.state||'queued');if(state==='answered')return null;if(state==='queued')return {kind:'queued',title:'Starting Primary Agent…',body:'',action:'Cancel',details:false};if(state==='working')return {kind:'working',title:'Primary Agent is working',body:'',action:'Cancel',details:false};if(state==='canceled')return {kind:'canceled',title:'Canceled by you',body:'',action:'',details:true};const code=String(target&&target.error_code||'');if(code==='daemon_interrupted')return {kind:'interrupted',title:'Answer interrupted',body:'Fort kept your message. Retry uses the same saved Primary Agent.',action:'Retry',details:true};if(code==='primary_agent_drift')return {kind:'failed',title:'This didn’t start',body:'The saved Primary Agent on '+(machine||'its saved computer')+' changed before Fort could begin. Fort kept your message.',action:'Recheck and retry',details:true};if(['primary_agent_unready','seat_unready','chat_policy_unavailable'].includes(code))return {kind:'failed',title:'This didn’t start',body:'The saved Primary Agent was not ready before Fort could begin. Fort kept your message.',action:'Recheck and retry',details:true};return {kind:'failed',title:'Answer failed',body:'Fort couldn’t finish this answer. Fort kept your message.',action:failedRecoveryAction(code),details:true}}
function announceLatestTargetStatus(targets){const current=targets.filter(target=>isLatestTargetAttempt(target,targets));const target=current[current.length-1];const announcer=byId('target-status-announcer');if(!target){app.targetAnnouncement='';announcer.textContent='';return}const signature=[target.id,target.state,target.attempt,target.error_code,target.updated_at].join('|');if(signature===app.targetAnnouncement)return;app.targetAnnouncement=signature;const participant=list(app.activeDetail&&app.activeDetail.participants).find(value=>String(value.id)===String(target.participant_id))||{};if(String(target.state)==='answered'){const message=list(app.activeDetail&&app.activeDetail.messages).find(value=>String(value.target_id)===String(target.id));announcer.textContent=['Primary Agent answered.',message&&message.body].filter(Boolean).join(' ');return}const presentation=targetPresentation(target,participant.machine);announcer.textContent=presentation?[presentation.title,presentation.body].filter(Boolean).join('. '):''}
function renderTargetState(target,targets,openDetails){
  const participant=list(app.activeDetail&&app.activeDetail.participants).find(value=>String(value.id)===String(target.participant_id))||{};const machine=String(participant.machine||'');const presentation=targetPresentation(target,machine);if(!presentation)return null;
  const wrap=node('div','turn-status');const card=node('section','target-state '+presentation.kind+(['queued','working'].includes(presentation.kind)?' compact':''));card.id=targetAnchorID(target.id);card.dataset.targetId=String(target.id||'');const head=node('div','target-state-head');
  if(presentation.kind!=='canceled'){const image=node('img',presentation.kind==='working'?'working':'');image.src='/fort-agent-orb.png';image.alt='';head.append(image)}
  const copy=node('div','target-copy');const title=presentation.kind==='canceled'?[presentation.title,formatTime(target.updated_at)].filter(Boolean).join(' · '):presentation.title;copy.append(node('strong','',title));if(presentation.body)copy.append(node('span','',presentation.body));head.append(copy);const latest=isLatestTargetAttempt(target,targets);const actions=targetActions(activeChannelID(),target,latest);if(actions&&!presentation.details)head.append(actions);card.append(head);
  if(presentation.details){const controls=node('div','target-controls');if(actions)controls.append(actions);const turn=list(app.activeDetail&&app.activeDetail.turns).find(value=>String(value.id)===String(target.turn_id));const details=node('details','target-details');details.open=openDetails&&openDetails.has(String(target.id||''));const summary=node('summary','', 'Details');summary.dataset.targetFocus='details';details.append(summary);const link=node('a','target-link','Primary Agent');link.dataset.targetFocus='target-link';link.href='#'+card.id;details.append(factGrid([['Reason',target.error||titleCase(target.error_code||'unknown')],['Attempt',String(target.attempt||1)],['Target',link],['Client turn ID',turn&&turn.client_turn_id],['Computer',machine||'unknown'],['Error code',target.error_code]]));if(String(target.state)==='failed'&&presentation.action)details.append(node('p','target-retry-note','Retry keeps this client turn ID and creates Attempt '+String(Number(target.attempt||1)+1)+'.'));controls.append(details);card.append(controls)}
  wrap.append(card);return wrap;
}
function targetActions(channelID,target,latest){
  if(!latest)return null;const state=String(target.state||'');if(!['queued','working','failed'].includes(state))return null;const wrap=node('div','target-actions');
  if(state==='queued'||state==='working'){
    const cancel=node('button','quiet-button','Cancel');cancel.type='button';cancel.dataset.targetFocus='cancel';cancel.addEventListener('click',()=>cancelTarget(channelID,target.id,cancel));wrap.append(cancel);return wrap;
  }
  const label=failedRecoveryAction(target.error_code);if(!label)return null;const recheck=label==='Recheck and retry';const retry=node('button','secondary-button',label);retry.type='button';retry.dataset.targetFocus='recovery';retry.addEventListener('click',()=>retryTarget(channelID,target.id,recheck,retry));wrap.append(retry);return wrap;
}
function factGrid(facts){
  const dl=node('dl','fact-grid');facts.filter(([,value])=>value!==undefined&&value!==null&&value!=='').forEach(([label,value])=>{const item=node('div');item.append(node('dt','',label));const dd=node('dd');if(value&&typeof value==='object'&&value.nodeType)dd.append(value);else dd.textContent=value;item.append(dd);dl.append(item)});return dl;
}
function renderIdentity(){
  const content=byId('identity-content');clear(content);const participant=participantFor(app.activeDetail,app.activeSummary);const identity=identityFor(app.activeDetail,app.activeSummary);const policy=policyFor(app.activeDetail,app.activeSummary);const image=node('img','identity-orb');image.src='/fort-agent-orb.png';image.alt='Fort orbital core';content.append(image);content.append(node('h3','',primaryLabel(participant,identity)));
  content.append(node('p','card-note','This identity was stored when the Channel was created. Changing Settings does not retarget or relabel it.'));
  content.append(factGrid([
    ['Participant',participant.id],['Seat',participant.seat_id||identity.seat&&identity.seat.id],['Profile',participant.profile||identity.seat&&identity.seat.profile],['Provider',participant.agent||identity.seat&&identity.seat.agent],['Requested model',selectedModel(participant,identity)],['Resolved model',first(app.activeDetail&&app.activeDetail.resolved_model,'unknown')],['Computer',participant.machine||identity.seat&&identity.seat.machine],['Account',policy.account_type&&policy.account_plan?titleCase(policy.account_type)+' · '+titleCase(policy.account_plan):''],['Policy',policy.policy_id],['Policy revision',policy.policy_revision],['Adapter',policy.adapter_id],['Adapter revision',policy.adapter_revision],['Runtime contract',policy.runtime_contract],['Codex version',policy.codex_version],['Executable revision',policy.codex_executable_revision],['Schema revision',policy.codex_schema_revision],['Isolation revision',policy.isolation_revision]
  ]));
  content.append(node('p','boundary-copy','Fort sends only this Channel’s frozen text context. MCP and dynamic tools are disabled. Any command, tool, or file-access attempt fails the target. Read-only sandboxing does not by itself make host inspection impossible.'));
  content.append(factGrid([
    ['Thread',policy.thread_mode],['Sandbox',policy.sandbox_mode],['Approval',policy.approval_policy],['Work directory',policy.workdir_mode],['Dynamic tools',policy.dynamic_tools_mode],['MCP',policy.mcp_mode],['Commands',policy.command_policy],['File reads',policy.file_read_policy],['Reasoning',policy.reasoning_effort],['Context',policy.reasoning_context],['Timeout',policy.request_timeout_millis?String(policy.request_timeout_millis)+' ms':'']
  ]));
}
function openIdentity(){if(!app.activeDetail)return;renderIdentity();dialogOpen('identity-dialog')}
function connectEvents(channelID){
  if(app.eventSource&&app.eventChannelID===channelID)return;
  disconnectEvents();if(!window.EventSource)return;
  const source=new EventSource(API.channels+'/'+encodeURIComponent(channelID)+'/events');app.eventSource=source;
  app.eventChannelID=channelID;
  source.onmessage=()=>scheduleChannelRefresh(channelID);
  source.addEventListener('conversation',()=>scheduleChannelRefresh(channelID));
  source.onerror=()=>{if(app.eventSource===source){byId('composer-status').textContent='Live updates disconnected. Reconnecting…'}};
  source.onopen=()=>{if(app.eventSource===source&&byId('composer-status').textContent.includes('Reconnecting'))byId('composer-status').textContent='Live updates connected'};
}
function disconnectEvents(){if(app.eventSource){app.eventSource.close();app.eventSource=null}app.eventChannelID=''}
function scheduleChannelRefresh(channelID){
  window.clearTimeout(app.refreshTimer);app.refreshTimer=window.setTimeout(async()=>{if(channelID!==activeChannelID())return;await Promise.allSettled([refreshChannelDetail(channelID),refreshChannelListOnly(),loadNeeds()])},160);
}
async function refreshChannelListOnly(){try{app.channels=channelItems(await request(API.channels+'?state=open'));app.activeSummary=app.channels.find(item=>channelID(item)===activeChannelID())||app.activeSummary;renderRail()}catch(_){} }
function submissionFailurePresentation(error,authorityBlock){const authorityDrift=Boolean(error&&error.code==='primary_agent_drift');return authorityDrift?{authorityDrift:true,keepPending:false,message:authorityBlock}:{authorityDrift:false,keepPending:true,message:errorText(error)+' Retry keeps the same client turn ID.'}}
async function sendMessage(event){
  event.preventDefault();if(app.sending||!activeChannelID())return;const input=byId('composer-input');const text=input.value.trim();if(!text)return;
  if(!channelCanSend(app.activeDetail)){setComposerStatus(channelComposerBlockMessage(app.activeDetail),true);return}
  if(!app.pendingTurn||app.pendingTurn.text!==text)app.pendingTurn={id:crypto.randomUUID(),text};
  app.sending=true;byId('send-button').disabled=true;input.disabled=true;setComposerStatus('Saving this turn…');
  try{
    await request(API.channels+'/'+encodeURIComponent(activeChannelID())+'/turns',{method:'POST',body:JSON.stringify({client_turn_id:app.pendingTurn.id,text:app.pendingTurn.text})});
    input.value='';app.pendingTurn=null;setComposerStatus('Queued durably. Waiting for current activity…');const [channelRefresh]=await Promise.allSettled([loadChannel(activeChannelID()),refreshChannelListOnly()]);if(channelRefresh.status==='fulfilled')setComposerStatus('');
  }catch(error){
    if(error&&error.code==='primary_agent_drift')markChannelAuthorityDrifted(app.activeDetail);const failure=submissionFailurePresentation(error,channelComposerBlockMessage(app.activeDetail));if(!failure.keepPending)app.pendingTurn=null;setComposerStatus(failure.message,true);if(failure.authorityDrift)await Promise.allSettled([refreshChannelDetail(activeChannelID()),loadAgent()])
  }
  finally{app.sending=false;const canSend=channelCanSend(app.activeDetail);byId('send-button').disabled=!canSend;input.disabled=!canSend;if(canSend)input.focus()}
}
function setComposerStatus(message,isError=false){const status=byId('composer-status');status.textContent=message;status.classList.toggle('error',isError)}
function openNewChannel(){if(!agentConfigured()){openSettings();return}byId('new-channel-error').hidden=true;byId('new-channel-name').value='';dialogOpen('new-channel-dialog');byId('new-channel-name').focus()}
async function createChannel(event){
  event.preventDefault();if(app.creating)return;const name=byId('new-channel-name').value.trim();if(!name)return;app.creating=true;byId('create-channel-button').disabled=true;const error=byId('new-channel-error');error.hidden=true;
  try{const created=await request(API.channels,{method:'POST',body:JSON.stringify({name})});dialogClose(byId('new-channel-dialog'));const id=first(created&&created.conversation&&created.conversation.id,created&&created.id,created&&created.conversation_id,'');await loadChannels(id);announce('Private Channel created.')}
  catch(failure){error.textContent=errorText(failure);error.hidden=false}
  finally{app.creating=false;byId('create-channel-button').disabled=false}
}
function openRename(){if(!app.activeSummary)return;byId('rename-channel-error').hidden=true;byId('rename-channel-name').value=channelName(app.activeSummary);dialogOpen('rename-channel-dialog');byId('rename-channel-name').focus()}
async function renameChannel(event){
  event.preventDefault();const name=byId('rename-channel-name').value.trim();if(!name||!activeChannelID())return;const button=byId('save-channel-name');button.disabled=true;const error=byId('rename-channel-error');error.hidden=true;
  try{await patchChannel(activeChannelID(),{name});dialogClose(byId('rename-channel-dialog'));await loadChannels(activeChannelID());announce('Channel renamed.')}
  catch(failure){error.textContent=errorText(failure);error.hidden=false}
  finally{button.disabled=false}
}
async function patchChannel(id,change){return request(API.channels+'/'+encodeURIComponent(id),{method:'PATCH',body:JSON.stringify(change)})}
async function togglePin(){if(!app.activeSummary)return;const want=!channelIsPinned(app.activeSummary);const button=byId('pin-channel-button');button.disabled=true;try{await patchChannel(activeChannelID(),{pinned:want});await loadChannels(activeChannelID());announce(want?'Channel pinned.':'Channel unpinned.')}catch(error){announce(errorText(error),'error')}finally{button.disabled=false}}
async function archiveChannel(){
  if(!activeChannelID()||!window.confirm('Archive this private Channel? You can reopen it from Settings.'))return;const button=byId('archive-channel-button');button.disabled=true;
  try{await patchChannel(activeChannelID(),{state:'archived'});await loadChannels();announce('Channel archived.')}catch(error){announce(errorText(error),'error')}finally{button.disabled=false}
}
async function cancelTarget(channelID,targetID,button){button.disabled=true;try{await request(API.channels+'/'+encodeURIComponent(channelID)+'/targets/'+encodeURIComponent(targetID)+'/cancel',{method:'POST'});await Promise.allSettled([loadChannel(channelID),loadNeeds()]);announce('Cancellation recorded.')}catch(error){announce(errorText(error),'error')}finally{button.disabled=false}}
async function retryTarget(channelID,targetID,recheck,button){
  button.disabled=true;try{const action=recheck?'recheck-and-retry':'retry';await request(API.channels+'/'+encodeURIComponent(channelID)+'/targets/'+encodeURIComponent(targetID)+'/'+action,{method:'POST'});await Promise.allSettled([loadChannel(channelID),loadNeeds()]);announce(recheck?'Readiness rechecked; retry queued on the same identity.':'Retry queued on the same identity.')}catch(error){announce(errorText(error),'error')}finally{button.disabled=false}
}
async function loadNeeds(){
  try{app.needs=needsItems(await request(API.needs));renderNeeds();return app.needs}catch(error){app.needs=[];renderNeeds(error);throw error}
}
function needChannelID(item){return first(item&&item.channel_id,item&&item.conversation_id,item&&item.channel&&item.channel.conversation&&item.channel.conversation.id,item&&item.channel&&item.channel.id,item&&item.conversation&&item.conversation.id,'')}
function needTarget(item){return first(item&&item.target,item&&item.latest_target,{})}
function renderNeeds(failure){
  const count=byId('needs-you-count');count.hidden=!app.needs.length;count.textContent=String(app.needs.length);const content=byId('needs-you-content');clear(content);
  if(failure){const error=node('div','error-state');const copy=node('div');copy.append(node('h2','', 'Needs-you state is unavailable'));copy.append(node('p','',errorText(failure)));const retry=node('button','secondary-button','Retry');retry.type='button';retry.addEventListener('click',()=>loadNeeds());copy.append(retry);error.append(copy);content.append(error);return}
  if(!app.needs.length){const empty=node('div','empty-state');const copy=node('div');copy.append(node('h2','', 'Nothing needs you'));copy.append(node('p','', 'No open private Channel has a current recoverable failure.'));empty.append(copy);content.append(empty);return}
  app.needs.forEach(item=>{
    const target=needTarget(item);const channelIDValue=needChannelID(item);const targetID=first(target.id,item&&item.target_id,'');const card=node('article','needs-card');const head=node('div','needs-head');head.append(node('strong','',first(item.channel_name,item.name,item.channel&&item.channel.conversation&&item.channel.conversation.title,item.channel&&item.channel.title,'Private Channel')));head.append(node('span','state-label failed','Failed'));card.append(head);card.append(node('p','',first(target.error,item.error,target.error_code,item.error_code,'A durable target needs recovery.')));
    const actions=node('div','card-actions');if(channelIDValue){const open=node('button','quiet-button','Open Channel');open.type='button';open.addEventListener('click',()=>{dialogClose(byId('needs-you-dialog'));selectChannel(channelIDValue,targetID)});actions.append(open)}
    if(channelIDValue&&targetID){const recovery=list(item.recovery_actions);const code=String(first(target.error_code,item.error_code,''));const recheck=recovery.includes('recheck_and_retry')||['primary_agent_unready','primary_agent_drift','chat_policy_unavailable','seat_unready'].includes(code);const retry=node('button','secondary-button',recheck?'Recheck and retry':'Retry');retry.type='button';retry.addEventListener('click',()=>retryTarget(channelIDValue,targetID,recheck,retry));actions.append(retry)}card.append(actions);content.append(card);
  });
}
async function openNeeds(){dialogOpen('needs-you-dialog');await loadNeeds().catch(()=>{})}
async function selectChannel(id,targetID=''){
  showView('channels');const summary=app.channels.find(item=>channelID(item)===id);if(summary)app.activeSummary=summary;renderRail();setRail(false);await loadChannel(id);
  if(targetID)focusTarget(targetID,true)
}
function optionAuthority(option){return first(option&&option.authority,option&&option.offer,option,{})}
function optionMachine(option){const authority=optionAuthority(option);return first(authority.machine_id,option.machine_id,option.machine,option.seat&&option.seat.machine,'Unknown computer')}
function optionState(option){return String(first(option.state,option.readiness&&option.readiness.state,option.seat&&option.seat.state,'unavailable'))}
function renderScheduleInventory(){
  const container=byId('schedule-inventory');if(!container)return;clear(container);const inventory=app.agent&&app.agent.scheduleInventory;
  if(!inventory){container.append(node('p','settings-copy','Schedule inventory is unavailable in this mode.'));return}
  const state=String(first(inventory.state,'drift'));const card=node('article','option-card');const head=node('div','option-head');head.append(node('strong','',state==='accepted'?'Reviewed inventory accepted':'Promotion review required'));head.append(node('span','state-label '+(state==='accepted'?'ready':'failed'),titleCase(state)));card.append(head);
  card.append(factGrid([['Current digest',inventory.current_digest],['Accepted digest',inventory.accepted_digest||'Not accepted'],['Enabled definitions',String(list(inventory.items).length)]]));
  const rows=node('div','inventory-list');list(inventory.items).forEach(item=>{const row=node('article','inventory-row');row.append(node('h4','',first(item.id,'Unknown schedule')));row.append(factGrid([['ID',item.id],['Kind',item.kind],['Expression',item.expression],['Timezone',item.timezone],['Flow ID',item.flow_id],['Flow digest',item.flow_digest||'Missing']]));rows.append(row)});card.append(rows);
  if(state!=='accepted')card.append(node('p','card-note',state==='unaccepted'?'Review this exact digest before Primary mode can start.':'The enabled schedule or loaded flow inventory differs from the reviewed digest; Primary mode is blocked.'));
  container.append(card);
}
function renderAgentOptions(failure){
  const container=byId('primary-agent-options');if(!container)return;clear(container);renderScheduleInventory();const stateCopy=byId('settings-agent-state');const clearButton=byId('clear-primary-agent');clearButton.hidden=!agentConfigured();
  if(failure){stateCopy.textContent='Primary Agent settings are unavailable.';const error=node('p','inline-error',errorText(failure));container.append(error);return}
  if(!app.agent){stateCopy.textContent='Loading current selection and eligible options…';return}
  if(agentConfigured()){
    const selection=app.agent.selection;const seat=selection.seat||{};stateCopy.textContent='Current default: '+[primaryLabel(seat,selection),seat.machine||'',titleCase(app.agent.state||seat.state||'stored')].filter(Boolean).join(' · ');
  }else{stateCopy.textContent='No Primary Agent is selected. New Channels are unavailable until one ready option is chosen.'}
  if(!app.agent.options.length){container.append(node('p','settings-copy','No eligible exact text-only subscription options are currently advertised. Recheck after Codex is ready and signed into a supported ChatGPT plan.'));return}
  const groups=new Map();app.agent.options.forEach(option=>{const machine=optionMachine(option);if(!groups.has(machine))groups.set(machine,[]);groups.get(machine).push(option)});
  groups.forEach((options,machine)=>{const group=node('section','machine-group');group.append(node('h4','',machine));options.forEach(option=>group.append(renderOption(option)));container.append(group)});
}
function renderOption(option){
  const card=node('article','option-card');const head=node('div','option-head');const seat=option.seat||{};const authority=optionAuthority(option);const label=first(option.display_name,seat.display_name,authority.profile_id,seat.profile,'Codex subscription option');head.append(node('strong','',label));const state=optionState(option);head.append(node('span','state-label '+state,titleCase(state)));card.append(head);
  card.append(factGrid([
    ['Option',option.option_id],['Profile',first(authority.profile_id,seat.profile)],['Provider',first(authority.agent_key,seat.agent)],['Requested model',first(authority.requested_model,seat.model)],['Resolved model',first(authority.resolved_model,'unknown')],['Computer',optionMachine(option)],['Account',authority.account_type&&authority.account_plan?titleCase(authority.account_type)+' · '+titleCase(authority.account_plan):''],['Adapter',authority.adapter_id],['Adapter revision',authority.adapter_revision],['Codex',authority.codex_version],['Executable revision',authority.codex_executable_revision],['Schema revision',authority.codex_schema_revision],['Policy',authority.policy_id],['Policy revision',authority.policy_revision],['Runtime',authority.runtime_contract],['Isolation revision',authority.isolation_revision]
  ]));
  const note=first(option.reason,option.readiness&&option.readiness.reason,'Ephemeral thread · empty work directory · read-only sandbox · never approve · no dynamic tools or MCP · command and file-read attempts fail.');card.append(node('p','card-note',note.includes('_')?titleCase(note):note));
  const actions=node('div','card-actions');const selected=app.agent.selection&&app.agent.selection.option_id===option.option_id;const choose=node('button',selected?'quiet-button':'secondary-button',selected?'Selected':'Choose');choose.type='button';choose.disabled=selected||state!=='ready';choose.addEventListener('click',()=>chooseAgent(option.option_id,choose));actions.append(choose);card.append(actions);return card;
}
async function chooseAgent(optionID,button){button.disabled=true;try{app.agent=normalizeAgent(await request(API.agent,{method:'PUT',body:JSON.stringify({option_id:optionID})}));renderAgentOptions();renderHeaderStatus();if(!app.channels.length)renderEmptyChannel();announce('Primary Agent selected for future Channels.')}catch(error){announce(errorText(error),'error');button.disabled=false}}
async function clearAgent(){const button=byId('clear-primary-agent');button.disabled=true;try{await request(API.agent,{method:'DELETE'});await loadAgent();if(!app.channels.length)renderEmptyChannel();announce('Default cleared. Existing Channels are unchanged.')}catch(error){announce(errorText(error),'error')}finally{button.disabled=false}}
async function recheckAgent(){const button=byId('recheck-agent-button');button.disabled=true;byId('settings-agent-state').textContent='Rechecking installed Codex readiness without dispatching a turn…';try{await request(API.recheck,{method:'POST'});await loadAgent();announce('Primary Agent readiness rechecked.')}catch(error){renderAgentOptions(error);announce(errorText(error),'error')}finally{button.disabled=false}}
async function loadArchived(){
  const container=byId('archived-channels');clear(container);container.append(node('p','settings-copy','Loading archived Channels…'));
  try{const items=channelItems(await request(API.channels+'?state=archived'));clear(container);if(!items.length){container.append(node('p','settings-copy','No archived Channels.'));return}items.forEach(item=>{const row=node('div','archive-row');row.append(node('span','',channelName(item)));const reopen=node('button','secondary-button','Reopen');reopen.type='button';reopen.addEventListener('click',()=>reopenChannel(channelID(item),reopen));row.append(reopen);container.append(row)})}
  catch(error){clear(container);container.append(node('p','inline-error',errorText(error)))}
}
async function reopenChannel(id,button){button.disabled=true;try{await patchChannel(id,{state:'open'});dialogClose(byId('settings-dialog'));await loadChannels(id);announce('Channel reopened with its stored Primary Agent identity.')}catch(error){announce(errorText(error),'error')}finally{button.disabled=false}}
async function openSettings(){dialogOpen('settings-dialog');renderAgentOptions();await Promise.allSettled([loadAgent(),loadArchived()])}
function showView(view){
  app.view=view;const channels=view==='channels';byId('channels-view').hidden=!channels;byId('scheduled-view').hidden=channels;byId('channels-tab').setAttribute('aria-current',channels?'page':'false');byId('scheduled-tab').setAttribute('aria-current',channels?'false':'page');byId('mobile-channels-tab').setAttribute('aria-current',channels?'page':'false');byId('mobile-scheduled-tab').setAttribute('aria-current',channels?'false':'page');byId('scheduled-navigation').setAttribute('aria-current',channels?'false':'page');if(!channels)loadSchedules(app.scheduleFilter).catch(()=>{});setRail(false);
}
async function loadSchedules(filter='all'){
  app.scheduleFilter=filter;document.querySelectorAll('[data-schedule-filter]').forEach(button=>button.setAttribute('aria-pressed',String(button.dataset.scheduleFilter===filter)));const content=byId('schedule-content');clear(content);const loading=node('div','loading-state');loading.append(node('p','', 'Loading durable schedule definitions…'));content.append(loading);
  try{const payload=await request(API.scheduleList+encodeURIComponent(filter));app.schedules=scheduleItems(payload);renderSchedules(payload);const count=byId('schedule-count');count.hidden=!app.schedules.length;count.textContent=String(app.schedules.length);return app.schedules}
  catch(error){app.schedules=[];renderScheduleError(error);throw error}
}
function renderSchedules(payload){
  byId('schedule-freshness').textContent='Read-only schedule data · observed '+formatTime(payload&&payload.observed_at);const content=byId('schedule-content');clear(content);
  if(!app.schedules.length){const empty=node('div','empty-state');const copy=node('div');copy.append(node('h2','', 'No schedules in this view'));copy.append(node('p','', 'Fort found no durable definitions matching this read-only filter.'));empty.append(copy);content.append(empty);return}
  const groups=[['Active · next fire',item=>item.enabled&&item.next_fire_at],['Active · no next fire',item=>item.enabled&&!item.next_fire_at],['Paused',item=>!item.enabled]];
  groups.forEach(([label,predicate])=>{const items=app.schedules.filter(predicate);if(!items.length)return;const section=node('section','schedule-section');section.append(node('h2','',label));const rows=node('div','schedule-list');items.forEach(item=>rows.append(renderScheduleRow(item)));section.append(rows);content.append(section)});
}
function latestOccurrence(item){return first(item.latest_occurrence,item.occurrence,{})}
function occurrenceState(occurrence){const value=String(first(occurrence&&occurrence.state,'upcoming'));return value==='scheduled'?'upcoming':value==='succeeded'?'completed':value}
function renderScheduleRow(item){
  const row=node('article','schedule-row');const button=node('button','schedule-row-main');button.type='button';button.setAttribute('aria-label','View evidence for '+first(item.title,item.id,'schedule'));const title=node('div','schedule-title');title.append(node('strong','',first(item.title,item.id,'Untitled schedule')));title.append(node('span','',first(item.id,'Unknown ID')+' · '+(item.enabled?'Active':'Paused')));button.append(title);
  const cadence=node('div','schedule-facts');const cadenceCopy=node('div');cadenceCopy.append(node('strong','',first(item.recurrence,titleCase(item.kind),'Cadence unavailable')));cadenceCopy.append(node('span','',first(item.timezone,'Timezone unavailable')));cadence.append(cadenceCopy);const fireCopy=node('div');fireCopy.append(node('strong','',item.next_fire_at?'Next '+formatTime(item.next_fire_at):'No next fire'));fireCopy.append(node('span','',item.last_fire_at?'Last '+formatTime(item.last_fire_at):'No last fire recorded'));cadence.append(fireCopy);button.append(cadence);
  const target=node('div','schedule-facts');target.append(node('strong','',titleCase(first(item.target_kind,'flow'))+' · '+first(item.target_id,'Unknown target')));target.append(node('span','',item.related_channel?'Related Channel · '+first(item.related_channel.name,item.related_channel.id):'System schedule'));target.append(node('span','',titleCase(first(item.scheduler_ownership,'unknown'))+' scheduler · observed '+formatTime(item.observed_at)));button.append(target);
  const occurrence=latestOccurrence(item);const state=occurrenceState(occurrence);const status=node('div','schedule-state '+state,titleCase(state));if(occurrence.error)status.append(node('span','',occurrence.error));if(occurrence.run_id)status.append(node('span','',occurrence.run_id));button.append(status);button.addEventListener('click',()=>openSchedule(item.id));row.append(button);
  const actions=node('div','schedule-actions');const evidence=node('button','quiet-button',scheduleActionLabel(state));evidence.type='button';evidence.addEventListener('click',()=>openSchedule(item.id));actions.append(evidence);if(item.related_channel){const open=node('button','quiet-button','Open Channel');open.type='button';open.addEventListener('click',()=>selectChannel(item.related_channel.id));actions.append(open)}row.append(actions);return row;
}
function scheduleActionLabel(state){return state==='upcoming'?'View schedule':state==='fired'||state==='running'?'Open run':state==='completed'?'View result':state==='failed'?'Review failure':'View evidence'}
function renderScheduleError(error){
  byId('schedule-freshness').textContent='Schedule projection unavailable';const content=byId('schedule-content');clear(content);const wrap=node('div','error-state');const copy=node('div');copy.append(node('h2','', 'Scheduled is unavailable'));copy.append(node('p','',errorText(error)));const retry=node('button','secondary-button','Retry');retry.type='button';retry.addEventListener('click',()=>loadSchedules(app.scheduleFilter));copy.append(retry);wrap.append(copy);content.append(wrap);
}
async function openSchedule(id){
  byId('schedule-detail-title').textContent='Schedule';byId('schedule-detail-subtitle').textContent='Loading durable schedule evidence…';const content=byId('schedule-detail-content');clear(content);content.append(node('p','settings-copy','Loading upcoming and recent occurrence evidence…'));dialogOpen('schedule-detail-dialog');
  try{const [detail,history]=await Promise.all([request(API.scheduleDetail+encodeURIComponent(id)),request(API.scheduleDetail+encodeURIComponent(id)+'/occurrences?limit=50')]);renderScheduleDetail(detail,history)}
  catch(error){clear(content);const wrap=node('div','error-state');const copy=node('div');copy.append(node('h2','', 'Schedule detail is unavailable'));copy.append(node('p','',errorText(error)));const retry=node('button','secondary-button','Retry');retry.type='button';retry.addEventListener('click',()=>openSchedule(id));copy.append(retry);wrap.append(copy);content.append(wrap)}
}
function renderScheduleDetail(detail,historyPayload){
  const item=first(detail&&detail.item,detail,{});byId('schedule-detail-title').textContent=first(item.title,item.id,'Schedule');byId('schedule-detail-subtitle').textContent=[item.enabled?'Active':'Paused',first(item.recurrence,titleCase(item.kind)),item.timezone].filter(Boolean).join(' · ');const content=byId('schedule-detail-content');clear(content);
  content.append(factGrid([
    ['Schedule ID',item.id],['Definition',item.enabled?'Active':'Paused'],['Kind',item.kind],['Expression',item.expression],['Timezone',item.timezone],['Next fire',formatTime(item.next_fire_at)],['Last fire',formatTime(item.last_fire_at)],['Target',titleCase(first(item.target_kind,'flow'))+' · '+first(item.target_id,'unknown')],['Scheduler ownership',titleCase(first(item.scheduler_ownership,'unknown'))],['Projection observed',formatTime(item.observed_at)]
  ]));
  if(item.related_channel){const open=node('button','secondary-button','Open Related Channel');open.type='button';open.addEventListener('click',()=>{dialogClose(byId('schedule-detail-dialog'));selectChannel(item.related_channel.id)});content.append(open)}
  const upcoming=list(detail&&detail.upcoming);const recent=list(detail&&detail.recent);const history=Array.isArray(historyPayload)?historyPayload:list(historyPayload&&historyPayload.items);
  appendOccurrenceSection(content,'Upcoming',upcoming,item);appendOccurrenceSection(content,'Recent',recent,item);appendOccurrenceSection(content,'Occurrence history',history,item);
}
function appendOccurrenceSection(content,label,items,schedule){
  const section=node('section','occurrence-section');section.append(node('h3','',label));if(!items.length){section.append(node('p','settings-copy','No '+label.toLowerCase()+' evidence.'));content.append(section);return}
  items.forEach(occurrence=>{const card=node('article','occurrence-card');const head=node('div','occurrence-head');const state=occurrenceState(occurrence);head.append(node('strong','state-label '+state,titleCase(state)));const time=node('time','',formatTime(occurrence.scheduled_for));if(occurrence.scheduled_for)time.dateTime=occurrence.scheduled_for;head.append(time);card.append(head);card.append(node('p','',occurrence.error||('Occurrence '+first(occurrence.id,'identity unavailable'))));if(occurrence.run_id)card.append(node('p','card-note','Observed run · '+occurrence.run_id));section.append(card)});content.append(section);
}
function bind(){
  byId('theme-select').addEventListener('change',event=>applyTheme(event.target.value,true));
  byId('mobile-menu').addEventListener('click',()=>setRail(true));byId('rail-close').addEventListener('click',()=>setRail(false));byId('rail-backdrop').addEventListener('click',()=>setRail(false));
  byId('channel-rail').addEventListener('keydown',trapRailFocus);
  byId('new-channel-button').addEventListener('click',openNewChannel);byId('new-channel-form').addEventListener('submit',createChannel);byId('rename-channel-form').addEventListener('submit',renameChannel);byId('channel-composer').addEventListener('submit',sendMessage);
  byId('identity-button').addEventListener('click',openIdentity);byId('boundary-button').addEventListener('click',openIdentity);byId('rename-channel-button').addEventListener('click',openRename);byId('pin-channel-button').addEventListener('click',togglePin);byId('archive-channel-button').addEventListener('click',archiveChannel);
  byId('channels-tab').addEventListener('click',()=>showView('channels'));byId('scheduled-tab').addEventListener('click',()=>showView('scheduled'));byId('scheduled-navigation').addEventListener('click',()=>showView('scheduled'));byId('mobile-channels-tab').addEventListener('click',()=>showView('channels'));byId('mobile-scheduled-tab').addEventListener('click',()=>showView('scheduled'));
  byId('needs-you-navigation').addEventListener('click',openNeeds);byId('mobile-needs-tab').addEventListener('click',openNeeds);byId('settings-navigation').addEventListener('click',openSettings);byId('mobile-settings-tab').addEventListener('click',openSettings);
  byId('recheck-agent-button').addEventListener('click',recheckAgent);byId('clear-primary-agent').addEventListener('click',clearAgent);byId('refresh-archived-button').addEventListener('click',loadArchived);
  document.querySelectorAll('[data-schedule-filter]').forEach(button=>button.addEventListener('click',()=>loadSchedules(button.dataset.scheduleFilter)));
  document.querySelectorAll('[data-close-dialog]').forEach(button=>button.addEventListener('click',()=>dialogClose(button.closest('dialog'))));
  document.querySelectorAll('dialog').forEach(dialog=>dialog.addEventListener('click',event=>{if(event.target===dialog)dialogClose(dialog)}));
  document.addEventListener('keydown',event=>{if(event.key==='Escape'&&byId('channel-rail').dataset.open==='true')setRail(false)});
  window.matchMedia('(max-width:860px)').addEventListener('change',()=>setRail(false));
}
async function start(){
  loadTheme();bind();renderAgentOptions();const results=await Promise.allSettled([loadAgent(),loadNeeds(),loadSchedules('all')]);
  try{await loadChannels()}catch(_){}
  const failures=results.filter(result=>result.status==='rejected');if(failures.length)announce('Some current Fort state could not be loaded. Available sections remain usable.','error');
}
start();
</script>
</body>
</html>`
