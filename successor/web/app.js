// Fort web client (Spec 063). Served by the hub itself, so every call is same-origin. The browser
// holds one device credential in localStorage; all text is rendered with textContent, never as HTML.
// With Google sign-in on, each browser gets its own credential by signing in (Spec 066, revision 2026-11-02).
import { approvalCopy, canDecide, chatPanels, crewSlot, questLog, resolution, runStatus, setupSteps, transcriptRows } from '/transcript.js';

const STORE = 'fort.device.v1';
const app = document.getElementById('app');
const state = { device: load(), browsers: null, installation: null, agents: [], conversations: [], runners: [], current: null, snapshot: null, cursor: 0, polling: false, sheet: false, page: null, from: null, adding: false, skipped: false, ready: null, problem: null, snapshots: new Map(), cleared: [] };

function load() { try { return JSON.parse(localStorage.getItem(STORE) ?? 'null'); } catch { return null; } }
function save(device) { state.device = device; if (device) localStorage.setItem(STORE, JSON.stringify(device)); else localStorage.removeItem(STORE); }
const secret = () => { const bytes = crypto.getRandomValues(new Uint8Array(32)); return btoa(String.fromCharCode(...bytes)).replaceAll('+', '-').replaceAll('/', '_').replaceAll('=', ''); };
const encode = value => btoa(unescape(encodeURIComponent(JSON.stringify(value)))).replaceAll('+', '-').replaceAll('/', '_').replaceAll('=', '');

class ApiError extends Error { constructor(status, code, message) { super(message); this.status = status; this.code = code; } }
async function api(path, body, { signal } = {}) {
  const headers = { 'x-fort-installation-id': state.device?.installationId ?? '' };
  if (state.device?.deviceToken) headers.authorization = 'Bearer ' + state.device.deviceToken;
  if (body !== undefined) headers['content-type'] = 'application/json';
  const response = await fetch(path, { method: body === undefined ? 'GET' : 'POST', headers, body: body === undefined ? undefined : JSON.stringify(body), signal, cache: 'no-store' });
  const text = await response.text();
  let value; try { value = text ? JSON.parse(text) : {}; } catch { value = {}; }
  // The Google session ran out: start over at the sign-in page.
  if (response.status === 401 && value?.error?.code === 'signInRequired') { location.reload(); throw new ApiError(401, 'signInRequired', 'Sign in with Google again.'); }
  if (!response.ok) throw new ApiError(response.status, value?.error?.code ?? 'error', value?.error?.message ?? `The hub answered ${response.status}.`);
  return value;
}

// Tiny element helper: h('div.class', {attrs}, ...children). Strings become text nodes.
function h(tag, attrs = {}, ...children) {
  const [name, ...classes] = tag.split('.');
  const node = document.createElement(name);
  if (classes.length) node.className = classes.join(' ');
  for (const [key, value] of Object.entries(attrs ?? {})) {
    if (key.startsWith('on')) node.addEventListener(key.slice(2), value);
    else if (value === true) node.setAttribute(key, '');
    else if (value !== false && value !== undefined && value !== null) node.setAttribute(key, String(value));
  }
  for (const child of children.flat()) if (child !== null && child !== undefined && child !== false) node.append(child instanceof Node ? child : document.createTextNode(String(child)));
  return node;
}
const say = (node, error) => { node.textContent = error ? (error.message || 'Something went wrong.') : ''; };

// ---------- Join ----------
// What this browser calls itself in Setup, from what it says it is.
function browserName() {
  const agent = navigator.userAgent;
  const browser = /Edg\//.test(agent) ? 'Edge' : /Firefox\/|FxiOS/.test(agent) ? 'Firefox' : /Chrome\/|CriOS/.test(agent) ? 'Chrome' : /Safari\//.test(agent) ? 'Safari' : 'Browser';
  const device = /iPhone/.test(agent) ? 'iPhone' : /iPad/.test(agent) ? 'iPad' : /Android/.test(agent) ? 'Android' : /Macintosh/.test(agent) ? 'Mac' : /Windows/.test(agent) ? 'Windows' : /Linux/.test(agent) ? 'Linux' : '';
  return device ? `${browser} on ${device}` : browser;
}
const AGAIN = 'fort.signin-again';
// A browser signed in with Google gets its own key from the hub. A hub without Google, or not claimed yet, takes the setup code.
async function join() {
  try {
    const reply = await api('/v1/hub/browsers', { name: browserName() });
    try { sessionStorage.removeItem(AGAIN); } catch { /* private mode */ }
    save({ installationId: reply.installationId, deviceId: reply.deviceId, deviceToken: reply.token });
    return true;
  } catch (problem) {
    if (problem.status === 404 || problem.code === 'notClaimed') { claimView(); return false; }
    // The Google sign-in is too old to add a browser: sign in again, once, without asking.
    let tried = false; try { tried = sessionStorage.getItem(AGAIN) === '1'; sessionStorage.setItem(AGAIN, '1'); } catch { tried = true; }
    if (problem.code === 'signInAgain' && !tried) { await signOut(); location.reload(); return false; }
    app.replaceChildren(h('div.card', {}, h('h1', {}, 'Fort'), h('p.error', {}, problem.message),
      h('button.primary', { onclick: async () => { await signOut(); location.reload(); } }, 'Sign in again'), ' ', h('button', { onclick: () => start() }, 'Try again')));
    return false;
  }
}
const signInAgain = () => { save(null); signOut().finally(() => location.reload()); };

// ---------- Claim ----------
function claimView() {
  const error = h('p.error');
  const code = h('input', { id: 'code', autocomplete: 'off', autocapitalize: 'off', spellcheck: 'false', required: true });
  const form = h('form', { onsubmit: async event => {
    event.preventDefault(); say(error); form.querySelector('button').disabled = true;
    const device = { deviceId: crypto.randomUUID(), deviceToken: secret() };
    try {
      const receipt = await api('/v1/hub/setup', { setupCode: code.value.trim(), ...device });
      save({ installationId: receipt.installationId ?? receipt.installation?.installationId, ...device });
      await start();
    } catch (problem) { say(error, problem); form.querySelector('button').disabled = false; }
  } }, h('label', { for: 'code' }, 'Setup code'), code, h('button.primary', { type: 'submit' }, 'Claim this hub'), error);
  app.replaceChildren(h('div.card', {}, h('h1', {}, 'Fort'),
    h('p.muted', {}, 'This browser becomes the owner of this hub. Enter the one-time setup code you were given when the hub was deployed.'), form));
}

// ---------- Main ----------
async function refresh() {
  const [agents, conversations, runners] = await Promise.all([api('/v1/agents'), api('/v1/conversations'), api('/v1/runners').catch(() => ({ runners: [] }))]);
  state.pool = runners.pool ?? null; state.cloud = runners.cloud ?? null;
  state.agents = agents.agents; state.conversations = conversations.conversations; state.runners = runners.runners;
  // The Quest log reads every chat; a chat that can't be read is left out until the next update.
  const snapshots = await Promise.all(state.conversations.map(conversation => api(`/v1/conversations/${encodeURIComponent(conversation.conversationId)}/snapshot`).catch(() => null)));
  state.snapshots = new Map(state.conversations.map((conversation, index) => [conversation.conversationId, snapshots[index]]).filter(([, snapshot]) => snapshot));
}
async function openConversation(conversationId) {
  state.current = conversationId;
  state.snapshot = conversationId ? await api(`/v1/conversations/${encodeURIComponent(conversationId)}/snapshot`) : null;
}

function render() {
  if (!state.device) return claimView();
  // The main screen is the Quest log and the Chats, nothing else. Until there is an Agent, the first run stands in for it.
  const shell = h('div.shell', {}, topBar(), state.current ? chat() : firstRun() ? welcome() : desk());
  const scroller = app.querySelector('.messages'), atBottom = !scroller || scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight < 40;
  const draft = app.querySelector('.composer textarea')?.value;
  // The sheet keeps its own state (an invitation on screen, a half-filled form) across live updates.
  state.sheetNode = state.sheet ? state.sheetNode ?? setupSheet() : null;
  app.replaceChildren(shell, ...(state.sheetNode ? [state.sheetNode] : []));
  const textarea = app.querySelector('.composer textarea');
  if (textarea && draft) textarea.value = draft;
  const messages = app.querySelector('.messages');
  if (messages && atBottom) messages.scrollTop = messages.scrollHeight;
}

function topBar() {
  return h('header.top', {}, h('h1.wordmark', { 'aria-label': 'Fort' }, 'FORT'), h('span.grow'),
    h('button', { onclick: () => openSetup(null) }, 'Setup'));
}

// ---------- First run ----------
// Signed in, nothing set up yet: the two steps that make Fort usable, then a clear hand-off to the Quest log.
// Devices are optional (Spec 063, revision 2026-11-03: the hub runs Agents itself), so they are mentioned, never asked for.
const accountReady = () => (state.cloud?.lanes ?? []).some(lane => lane.signedIn) || state.runners.some(runner => runner.online);
const firstRun = () => state.ready !== null || (!state.agents.length && !state.skipped);
function welcome() {
  const plan = setupSteps({ account: accountReady(), agents: state.agents.length });
  if (state.ready !== null && plan.done) {
    const finish = conversationId => { state.ready = null; if (conversationId) select(conversationId); else render(); };
    return h('main.welcome', {}, h('div.welcomebox', {},
      h('span.caption.done', {}, 'Setup done'),
      h('h2.title.big', {}, 'You’re set'),
      h('p.lede', {}, 'From here on Fort opens on two things: the Quest log, for anything that needs you, and your Chats.'),
      h('div.actions', {}, h('button.primary', { onclick: () => finish(state.ready) }, 'Start chatting'), h('button.link', { onclick: () => finish(null) }, 'Go to the Quest log'))));
  }
  const step = (item, index) => h('li.step' + (item.done ? '.done' : item.current ? '.current' : ''), {},
    h('span.num', { 'aria-hidden': 'true' }, item.done ? '✓' : String(index + 1)),
    h('div', {}, h('strong', {}, item.title), h('p.muted', {}, item.done ? 'Done.' : item.line)),
    item.done ? null : h('button' + (item.current ? '.primary' : ''), { onclick: () => openSetup(item.page, 'welcome') }, item.action));
  return h('main.welcome', {}, h('div.welcomebox', {},
    h('h2.title.big', {}, 'Set up Fort'),
    h('p.lede', {}, 'Two steps and you’re in. Fort runs your agents in the cloud, so there’s nothing to install.'),
    h('ol.steps', {}, plan.steps.map(step)),
    h('p.muted.later', {}, 'Optional, any time: connect a Mac or pair the iPhone app from Setup, Devices.'),
    state.agents.length ? h('button.link', { onclick: () => { state.ready = null; state.skipped = true; render(); } }, 'Skip to the Quest log') : null));
}

// The home screen: the Quest log of everything waiting on you, beside a panel for every chat.
function desk() {
  const quests = questLog(state.conversations, state.snapshots, state.agents);
  const panels = chatPanels(state.conversations, state.snapshots, state.agents, quests);
  const cleared = state.cleared.filter(item => !quests.some(quest => quest.id === item.id)).slice(-3);
  return h('main.desk', {},
    h('section.col', { 'aria-labelledby': 'quest-title' },
      h('div.sechead', {}, h('h2.title', { id: 'quest-title' }, 'Quest log'), quests.length ? h('span.caption.gold', {}, quests.length === 1 ? '1 needs you' : `${quests.length} need you`) : null),
      state.problem ? h('p.error', { role: 'alert' }, state.problem) : null,
      quests.length ? quests.map(questCard) : h('p.panel.calm', {}, 'Nothing needs you. Your crew is on it.'),
      cleared.map(item => h('div.panel.cleared', {}, h('span.stamp' + (item.dropped ? '.dropped' : ''), {}, item.dropped ? 'Dropped' : 'Cleared'), h('span.muted', {}, item.title)))),
    h('section.col', { 'aria-labelledby': 'chats-title' },
      h('div.sechead', {}, h('h2.title', { id: 'chats-title' }, 'Chats')),
      panels.length ? h('div.panels', {}, panels.map(chatPanel)) : h('p.panel.calm', {}, 'No agents yet. Open Setup to add one.')));
}

const KIND = { approval: 'Approval', failed: 'Failed', waiting: 'Waiting' };
function questCard(quest) {
  const open = () => select(quest.conversationId);
  const actions = [];
  if (quest.kind === 'approval') {
    const copy = approvalCopy(quest.approval);
    const decide = approve => async event => {
      for (const button of event.currentTarget.parentElement.querySelectorAll('button')) button.disabled = true;
      state.problem = null;
      try {
        await api(`/v1/approvals/${encodeURIComponent(quest.approval.approvalId)}/resolve`, resolution(quest.approval, approve));
        state.cleared.push({ id: quest.id, title: quest.title, dropped: !approve });
        await refresh();
      } catch (error) { state.problem = error.message; }
      render();
    };
    actions.push(h('button.primary', { onclick: decide(true) }, copy.approve), h('button', { onclick: decide(false) }, copy.reject));
  }
  actions.push(h('button' + (quest.kind === 'approval' ? '.link' : '.primary'), { onclick: open }, 'Open chat'));
  return h('article.panel.quest.' + quest.kind, {},
    h('div.questhead', {}, h('span.mark', { 'aria-hidden': 'true' }, h('span', {}, '!')),
      h('div', {}, h('strong', {}, quest.title),
        h('div.meta', {}, quest.agent ? h('span.portrait.crew-' + crewSlot(quest.agent.agentId), { 'aria-hidden': 'true' }, [...quest.agent.name.trim()][0]?.toUpperCase() ?? '?') : null,
          [quest.agent?.name, quest.chat !== quest.agent?.name ? quest.chat : null, KIND[quest.kind]].filter(Boolean).join(' · ')))),
    quest.detail ? h('p', {}, quest.detail) : null,
    h('div.actions', {}, actions));
}

function chatPanel(panel) {
  const line = [panel.quests ? (panel.quests === 1 ? '1 needs you' : `${panel.quests} need you`) : null, panel.live.some(run => run.tone === 'working') ? `${panel.live.filter(run => run.tone === 'working').length} running` : null].filter(Boolean).join(' · ') || 'Quiet';
  return h('button.panel.chatpanel', { onclick: () => select(panel.conversationId), 'aria-label': `${panel.name}: ${line}` },
    h('span.strip.halftone.' + panel.tone, {}, h('span.caption', {}, panel.name), h('span.grow'), h('span.line', {}, line)),
    h('span.crew', {}, panel.crew.map(member => h('span.portrait.crew-' + member.slot, { title: member.name }, member.initial))),
    h('span.runs', {}, panel.live.length ? panel.live.map(run => h('span.runline', {},
      h('span.meter.' + run.tone, { 'aria-hidden': 'true' }, [0, 1, 2].map(index => h('span.pip' + (index < run.filled ? '.on' : '')))),
      h('span', {}, (run.agentName ? run.agentName + ': ' : '') + run.text))) : h('span.muted', {}, 'Nothing running')));
}
const providerName = provider => state.installation?.runtimes?.find(runtime => runtime.provider === provider)?.name ?? provider;
// An Agent's badge: its initial on its crew colour, the same slot as FortTheme.crewSlot in the apps.
const portrait = agent => h('span.portrait.crew-' + crewSlot(agent.agentId), { 'aria-hidden': 'true', title: agent.name }, [...agent.name.trim()][0]?.toUpperCase() ?? '?');
// The chat header's one light and sentence, like ChatQuiet.header in the apps.
function headerLine() {
  // The hub runs Agents itself once an account is signed in there, so a missing Mac is only news when nothing else can run.
  const ready = accountReady();
  const [tone, text] = ready ? ['done', 'Ready'] : state.runners.length ? ['failed', 'Your Mac is offline'] : ['off', 'No account signed in'];
  return h('span.headline', {}, h('span.lamp.' + tone, { 'aria-hidden': 'true' }), text);
}
async function select(conversationId) { state.problem = null; try { await openConversation(conversationId); } catch (error) { state.problem = error.message; } render(); }

function chat() {
  if (!state.current || !state.snapshot) return h('section.chat', {}, h('p.empty', {}, 'Pick an agent to start talking.'), state.problem ? h('p.error.problem', { role: 'alert' }, state.problem) : null);
  const conversation = state.conversations.find(item => item.conversationId === state.current);
  const participants = state.agents.filter(agent => conversation?.participantAgentIds.includes(agent.agentId));
  const rows = transcriptRows(state.snapshot).map(row => {
    if (row.kind === 'message') {
      // A speech bubble, the same as the apps: an Agent on the left beside its badge, you on the right.
      const message = row.message, mine = message.role === 'user';
      const author = mine ? null : state.agents.find(agent => agent.agentId === message.agentId) ?? state.agents.find(agent => agent.name === message.agentName);
      const bubble = h('div.msg.' + message.role, {}, !mine || participants.length > 1 ? h('span.who', {}, mine ? 'You' + (message.recipientAgentName ? ' → ' + message.recipientAgentName : '') : message.agentName ?? 'Agent') : null, message.text);
      return h('div.msgrow.' + message.role, {}, mine ? null : portrait(author ?? { agentId: message.agentName ?? '', name: message.agentName ?? 'Agent' }), bubble);
    }
    if (row.kind === 'run') {
      const status = runStatus(row.run);
      return h('div.run.' + status.tone, {}, h('span.lamp'), status.text, status.detail && status.tone !== 'needsYou' ? h('span.muted', {}, ' · ' + status.detail) : null);
    }
    return approvalCard(row.approval);
  });
  const recipient = h('select', { 'aria-label': 'Send to' }, participants.map(agent => h('option', { value: agent.agentId }, agent.name)));
  const input = h('textarea', { rows: 1, placeholder: 'Message', 'aria-label': 'Message', onkeydown: event => { if (event.key === 'Enter' && !event.shiftKey && !event.isComposing && matchMedia('(pointer:fine)').matches) { event.preventDefault(); send(); } } });
  const send = async () => {
    const text = input.value.trim(); if (!text) return;
    const agent = participants.find(item => item.agentId === recipient.value) ?? participants[0];
    if (!agent) return;
    input.disabled = true; state.problem = null;
    try {
      await api(`/v1/conversations/${encodeURIComponent(state.current)}/messages`, { requestId: crypto.randomUUID(), text, recipientAgentId: agent.agentId,
        bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: state.installation.recoveryEpoch, skills: [] });
      input.value = ''; await openConversation(state.current); render();
    } catch (error) { state.problem = error.message; render(); }
  };
  return h('section.chat', {},
    h('header', {}, h('button.back', { onclick: () => { state.current = null; state.problem = null; render(); } }, '‹ Quest log'), h('h2', {}, conversation?.name ?? 'Chat'), headerLine()),
    h('div.messages', {}, rows.length ? rows : h('p.empty', {}, 'No messages yet.')),
    state.problem ? h('p.error.problem', { role: 'alert' }, state.problem) : null,
    h('form.composer', { onsubmit: event => { event.preventDefault(); send(); } }, participants.length > 1 ? recipient : null, input, h('button.primary', { type: 'submit' }, 'Send')));
}

// An agent asking to go on: what it wants, in its words, with Approve and Reject while it can still be decided.
function approvalCard(approval) {
  const copy = approvalCopy(approval), open = canDecide(approval);
  const decide = approve => async event => {
    for (const button of event.currentTarget.parentElement.querySelectorAll('button')) button.disabled = true;
    state.problem = null;
    try { await api(`/v1/approvals/${encodeURIComponent(approval.approvalId)}/resolve`, resolution(approval, approve)); await openConversation(state.current); }
    catch (error) { state.problem = error.message; }
    render();
  };
  const run = state.snapshot.runs.find(item => item.runId === approval.runId);
  return h('div.approval' + (open ? '.open' : ''), {},
    h('span.label', {}, open ? 'Needs you' : copy.decided ?? approval.state),
    h('strong', {}, approval.actionDescription),
    run?.statusDetail && run.statusDetail !== approval.actionDescription ? h('p', {}, run.statusDetail) : null,
    open ? h('div.actions', {}, h('button.primary', { onclick: decide(true) }, copy.approve), h('button', { onclick: decide(false) }, copy.reject)) : null);
}

// ---------- Setup ----------
// Setup is a short menu; each entry opens its own page (Revision 2026-11-06). The pages are the same, in the same order,
// as the tabs of Setup in the Mac and iPhone apps: Agents, Accounts, Devices (Routines sit between Agents and Accounts there).
const SETUP = [
  { page: 'agents', title: 'Agents' },
  { page: 'accounts', title: 'Accounts' },
  { page: 'devices', title: 'Devices', optional: true },
];
// `from` is where Setup was opened: from the first run, the back button returns there instead of to the menu.
function openSetup(page, from = null) { state.sheet = true; state.page = page; state.from = from; state.adding = page === 'agents' && !state.agents.length; state.sheetNode = null; render(); }
function closeSetup() { state.sheet = false; state.page = null; state.from = null; state.adding = false; state.sheetNode = null; render(); }

function setupSheet() {
  const entry = SETUP.find(item => item.page === state.page);
  const back = () => { if (state.from === 'welcome') return closeSetup(); openSetup(null); };
  const head = entry
    ? h('header', {}, h('button.back', { onclick: back }, state.from === 'welcome' ? '‹ Back' : '‹ Setup'), h('h2.title', {}, entry.title), h('button.primary', { onclick: closeSetup }, 'Done'))
    : h('header', {}, h('h2.title', {}, 'Setup'), h('button.primary', { onclick: closeSetup }, 'Done'));
  const page = !entry ? setupMenu() : entry.page === 'agents' ? crewSection() : entry.page === 'accounts' ? accountSection() : devicesSection();
  const body = h('div', {}, head, page);
  return h('div.sheet', { onclick: event => { if (event.target.classList.contains('sheet')) closeSetup(); } }, body);
}

// One row per page: its name, one line on where it stands, and an Optional tag on Devices.
function setupMenu() {
  const lanes = (state.cloud?.lanes ?? []).filter(lane => lane.signedIn).map(lane => lane.name);
  const online = state.runners.filter(runner => runner.online).length;
  const lines = {
    agents: state.agents.length ? state.agents.map(agent => agent.name).join(', ') : 'None yet. Create your first one.',
    accounts: state.browsers?.you === 'browser' ? 'Managed from the owner browser' : lanes.length ? `${lanes.join(' and ')} signed in on the hub` : 'Sign in so the hub can run your agents',
    devices: online ? `${online === 1 ? '1 Mac' : online + ' Macs'} connected` : 'Macs, iPhone and browsers',
  };
  return h('nav.menu', { 'aria-label': 'Setup' },
    SETUP.map(item => h('button.menurow', { onclick: () => openSetup(item.page) },
      h('span.menutext', {}, h('strong', {}, item.title, item.optional ? h('span.tag', {}, 'Optional') : null), h('span.muted', {}, lines[item.page])),
      h('span.chev', { 'aria-hidden': 'true' }, '›'))),
    h('div.menufoot', {}, h('button.link', { onclick: () => signOut().then(() => location.reload()) }, 'Sign out of Google')));
}

// Devices extend the hub; none is needed to use Fort.
function devicesSection() {
  return h('section', {}, h('p.muted', {}, 'Optional. The hub runs your agents on its own. Connect a Mac for what the hub can’t do, or pair the iPhone app.'),
    macSection(), phoneSection(), browserSection());
}

function macSection() {
  const out = h('div'), error = h('p.error');
  const list = state.runners.map(runner => h('div.row', {}, h('span', {}, h('span.dot' + (runner.online ? '.on' : '')), runner.runnerName, h('span.muted', {}, ' · ' + runner.runnerId + ' · ' + macState(runner))),
    h('button.link', { onclick: async () => { if (!confirm(`Remove ${runner.runnerName}? It stops taking turns at once.`)) return; try { await api('/v1/runners/revoke', { runnerId: runner.runnerId }); await refresh(); state.sheetNode = null; render(); } catch (problem) { say(error, problem); } } }, 'Remove')));
  const add = h('button', { onclick: async () => {
    say(error); const invitation = { invitationId: crypto.randomUUID(), code: secret() };
    try {
      const reply = await api('/v1/runners/invitation', invitation);
      out.replaceChildren(h('p.muted', {}, `In Fort on your Mac, open Devices, choose Connect to cloud hub, and paste this within 5 minutes (until ${new Date(reply.expiresAt).toLocaleTimeString()}). It is used once.`),
        h('div.code', {}, 'fort-runner:' + encode({ v: 1, hub: location.origin, ...invitation, runnerId: nextRunnerId() })));
    } catch (problem) { say(error, problem); }
  } }, 'Add a Mac');
  const node = state.installation?.runtimes?.[0]?.hostId;
  const where = state.pool ? `the first connected Mac of ${state.pool.runners.map(id => `“${id}”`).join(', then ')}` : `the Mac named “${node ?? '?'}”`;
  // Spec 063, revision 2026-11-03: the hub runs Agents itself; a Mac is an extension for what the hub can't do.
  const lanes = state.cloud?.lanes ?? [], own = lanes.filter(lane => lane.signedIn).map(lane => lane.name);
  const story = !lanes.length ? `Your Macs run the turns; the hub only routes them to ${where}.`
    : own.length ? `The hub runs ${own.join(' and ')} Agents itself, no Mac needed. Macs are optional: they take what the hub can't run, sent to ${where}.`
    : `The hub can run ${lanes.map(lane => lane.name).join(' and ')} Agents itself once you sign in under Accounts. Until then, turns go to ${where}.`;
  return h('div.sub', {}, h('h4', {}, 'Macs'), h('p.muted', {}, story), list, add, out, error);
}
// A pooled hub names its Macs; an invitation is for the first one not enrolled yet.
function nextRunnerId() {
  return state.pool?.runners.find(id => !state.runners.some(runner => runner.runnerId === id)) ?? state.installation.runtimes[0].hostId;
}
// Online and running, online and idle, or when the hub last heard from it since it started.
function macState(runner) {
  if (runner.online) return runner.turns ? `running ${runner.turns === 1 ? 'a turn' : runner.turns + ' turns'}` : 'connected';
  if (!runner.lastSeen) return 'offline';
  const minutes = Math.max(0, Math.round((Date.now() - Date.parse(runner.lastSeen)) / 60_000));
  return minutes < 1 ? 'offline, seen just now' : minutes < 120 ? `offline, last seen ${minutes} min ago` : `offline since ${new Date(runner.lastSeen).toLocaleString()}`;
}

function accountSection() {
  // Provider accounts and iPhone pairing belong to the owner browser; an added browser says so instead.
  if (state.browsers?.you === 'browser') return h('section', {}, h('p.muted', {}, 'Sign in to ChatGPT and Claude from the owner browser.'));
  const cloud = new Set((state.cloud?.lanes ?? []).map(lane => lane.provider));
  const intro = cloud.size ? 'Sign in to your subscriptions here and the hub runs those Agents itself. A provider the hub can’t run yet is confirmed on your Mac.'
    : 'Confirm which subscription each provider on your Mac is signed in to. Sign-ins happen on the Mac; nothing is stored here.';
  const section = h('section', {}, h('p.muted', {}, intro));
  for (const runtime of state.installation?.runtimes ?? []) {
    const line = h('span', {}, runtime.name, h('span.muted', {}, ' · checking…')), error = h('p.error');
    const challenge = h('div');
    const button = h('button', { onclick: () => confirmAccount(runtime, line, button, error, challenge) }, cloud.has(runtime.provider) ? 'Sign in' : 'Confirm');
    section.append(h('div.row', {}, line, button), challenge, error);
    api('/v1/account/sign-in?provider=' + encodeURIComponent(runtime.provider)).then(status => describeAccount(line, runtime, status)).catch(problem => say(error, problem));
  }
  return section;
}
function describeAccount(line, runtime, status, challenge) {
  const result = status.state === 'finished' ? status.result : undefined;
  const onHub = (state.cloud?.lanes ?? []).some(lane => lane.provider === runtime.provider);
  const detail = status.state === 'unavailable' ? 'not available on this hub'
    : result?.kind === 'signedIn' && result.account ? `${result.account.userLabel}${status.selected ? '' : ' (not selected)'}${onHub ? ' · runs on the hub' : ''}`
    : status.state === 'challenge' ? (onHub ? (status.challenge?.kind === 'browserCode' ? 'paste the code below' : 'enter the code below') : 'finish signing in on your Mac') : status.state === 'starting' ? (onHub ? 'starting…' : 'asking your Mac…')
    : result ? `last check: ${result.kind}` : onHub ? 'not signed in' : 'not confirmed yet';
  line.replaceChildren(runtime.name, h('span.muted', {}, ' · ' + detail));
  // A device code from the hub: open the provider's own page and type the code there. Only the provider's https page is linked.
  const code = status.state === 'challenge' ? status.challenge : undefined;
  if (!challenge) return;
  // Claude on the hub: open Claude's own sign-in page, then paste the code it shows. The form is drawn once per sign-in, so
  // polling never clears what is being typed. The code goes to the hub once and is never kept in the page.
  if (onHub && code?.kind === 'browserCode' && /^https:\/\/(claude\.com|claude\.ai)\//.test(code.authorizationURL ?? '')) {
    if (challenge.dataset.operation === status.operationId) return;
    challenge.dataset.operation = status.operationId;
    const input = h('input', { type: 'text', autocomplete: 'off', spellcheck: 'false', placeholder: 'Paste the code', 'aria-label': 'Sign-in code' }), sent = h('p.error');
    const send = h('button.primary', { type: 'submit' }, 'Finish sign-in');
    challenge.replaceChildren(h('p.muted', {}, 'Open ', h('a', { href: code.authorizationURL, target: '_blank', rel: 'noopener noreferrer' }, new URL(code.authorizationURL).host),
      ', sign in with your Claude subscription, then copy the code it shows and paste it here:'),
      h('form.row', { onsubmit: async event => {
        event.preventDefault(); say(sent); send.disabled = true;
        try { await api('/v1/account/sign-in/code?provider=' + encodeURIComponent(runtime.provider), { operationId: status.operationId, revision: status.revision, recoveryEpoch: state.installation.recoveryEpoch, code: input.value.trim() }); input.value = ''; input.disabled = true; }
        catch (problem) { say(sent, problem); send.disabled = false; }
      } }, input, send), sent);
    return;
  }
  delete challenge.dataset.operation;
  if (!code || !/^https:\/\/[a-z0-9.-]+\//.test(code.verificationURL ?? '')) { challenge.replaceChildren(); return; }
  challenge.replaceChildren(h('p.muted', {}, 'Open ', h('a', { href: code.verificationURL, target: '_blank', rel: 'noopener noreferrer' }, new URL(code.verificationURL).host),
    ', sign in with your subscription and enter this code:'), h('div.code', {}, code.userCode));
}
async function confirmAccount(runtime, line, button, error, challenge) {
  say(error); button.disabled = true;
  const query = '?provider=' + encodeURIComponent(runtime.provider);
  try {
    let status = await api('/v1/account/sign-in' + query);
    status = await api('/v1/account/sign-in/start' + query, { operationId: crypto.randomUUID(), expectedRevision: status.revision, recoveryEpoch: state.installation.recoveryEpoch, expiresAt: Date.now() + 110_000 });
    while (['starting', 'challenge', 'canceling'].includes(status.state)) {
      describeAccount(line, runtime, status, challenge);
      await new Promise(resolve => setTimeout(resolve, 1500));
      status = await api('/v1/account/sign-in' + query);
    }
    if (status.state === 'finished' && status.result.kind === 'signedIn' && status.result.account && !status.selected)
      status = await api('/v1/account/sign-in/select' + query, { operationId: status.operationId, revision: status.revision, recoveryEpoch: state.installation.recoveryEpoch, account: status.result.account });
    describeAccount(line, runtime, status, challenge);
    // The hub now runs this provider itself: show the Macs as optional.
    // From the first run, a finished sign-in goes straight back to the next step.
    if (status.state === 'finished' && status.result.kind === 'signedIn') { await refresh(); if (state.from === 'welcome') return closeSetup(); state.sheetNode = null; render(); }
  } catch (problem) { say(error, problem); }
  button.disabled = false;
}

// Every Agent the way the apps list it (badge, name, "provider · model"), then the form for a new one.
function crewSection() {
  const error = h('p.error'), runtimes = state.installation?.runtimes ?? [];
  const name = h('input', { id: 'agent-name', maxlength: 128, required: true });
  const provider = h('select', { id: 'agent-provider' }, runtimes.map(runtime => h('option', { value: runtime.provider }, runtime.name)));
  const models = h('datalist', { id: 'agent-models' });
  const model = h('input', { id: 'agent-model', list: 'agent-models', required: true, autocomplete: 'off', placeholder: 'Checking models…' });
  const instructions = h('textarea', { id: 'agent-instructions', rows: 3, required: true, placeholder: 'Who is this agent and how should it answer?' });
  const check = async () => {
    models.replaceChildren(); model.placeholder = 'Checking models on your Mac…';
    try {
      const catalog = await api('/v1/models?provider=' + encodeURIComponent(provider.value));
      if (catalog.state === 'ready') { models.append(...catalog.models.filter(item => item.availability === 'available').map(item => h('option', { value: item.id }, item.name))); model.placeholder = 'Pick or type a model'; if (!model.value && catalog.models[0]) model.value = catalog.models[0].id; }
      else model.placeholder = catalog.detail;
    } catch (problem) { model.placeholder = problem.message; }
  };
  provider.addEventListener('change', check); queueMicrotask(check);
  const form = h('form', { onsubmit: async event => {
    event.preventDefault(); say(error);
    try {
      const first = !state.agents.length;
      const agent = await api('/v1/agents', { name: name.value.trim(), provider: provider.value, model: model.value.trim(), instructions: instructions.value });
      await refresh();
      // The first Agent ends setup: show "You're set" before Fort opens on its chat. Later ones open their chat at once.
      if (first) { state.ready = agent.homeConversationId; closeSetup(); return; }
      await openConversation(agent.homeConversationId); closeSetup();
    } catch (problem) { say(error, problem); }
  } }, h('label', { for: 'agent-name' }, 'Name'), name, h('label', { for: 'agent-provider' }, 'Provider'), provider,
    h('label', { for: 'agent-model' }, 'Model'), model, models, h('label', { for: 'agent-instructions' }, 'Instructions'), instructions,
    h('div.row', {}, h('span'), h('button.primary', { type: 'submit' }, 'Add agent')), error);
  const crew = state.agents.map(agent => h('div.row.crewrow', {}, h('span.crewline', {}, portrait(agent), h('span', {}, h('strong', {}, agent.name), h('span.muted.small', {}, [providerName(agent.binding.provider), agent.binding.selectedModel].filter(Boolean).join(' · '))))));
  // The list first; the form for a new Agent opens on request, or at once when there are none.
  const add = state.adding ? [h('h4', {}, 'New agent'), form] : [h('button', { onclick: () => { state.adding = true; state.sheetNode = null; render(); } }, 'New agent')];
  return h('section', {}, crew.length ? crew : h('p.muted', {}, 'No agents yet.'), ...add);
}

function phoneSection() {
  if (state.browsers?.you === 'browser') return h('div.sub', {}, h('h4', {}, 'iPhone'), h('p.muted', {}, 'Pair the Fort iPhone app from the owner browser.'));
  const out = h('div'), error = h('p.error');
  api('/v1/pairing/companion').then(overview => {
    const pairing = overview.pairing;
    if (pairing.state === 'paired') out.replaceChildren(h('div.row', {}, h('span', {}, pairing.deviceName, h('span.muted', {}, pairing.active ? ' · paired' : ' · expired')),
      h('button.link', { onclick: async () => { if (!confirm(`Unpair ${pairing.deviceName}?`)) return; try { await api('/v1/pairing/revoke-companion', { deviceId: pairing.deviceId }); state.sheetNode = null; render(); } catch (problem) { say(error, problem); } } }, 'Unpair')));
    else out.replaceChildren(h('button', { onclick: invite }, 'Pair iPhone'));
  }).catch(problem => say(error, problem));
  async function invite() {
    say(error); const invitation = { invitationId: crypto.randomUUID(), code: secret() };
    try {
      const reply = await api('/v1/pairing/invitation', invitation);
      out.replaceChildren(h('p.muted', {}, `In Fort on your iPhone, choose to pair and paste this within 5 minutes (until ${new Date(reply.expiresAt).toLocaleTimeString()}).`),
        h('div.code', {}, 'fort-invite:' + encode({ v: 1, address: location.origin, installationId: state.device.installationId, invitationId: invitation.invitationId, code: invitation.code })));
    } catch (problem) { say(error, problem); }
  }
  return h('div.sub', {}, h('h4', {}, 'iPhone'), h('p.muted', {}, 'The Fort iPhone app talks to this hub directly, from anywhere.'), out, error);
}

function browserSection() {
  const error = h('p.error'), info = state.browsers, added = info?.you === 'browser';
  const remove = entry => async () => {
    if (!confirm(entry.current ? 'Forget this browser? Sign in with Google again to get back in.' : `Remove ${entry.name}? It loses access until you sign in with Google there again.`)) return;
    try {
      await api('/v1/hub/browsers/remove', { deviceId: entry.deviceId });
      if (entry.current) return signInAgain();
      state.browsers = await api('/v1/hub/browsers').catch(() => state.browsers); state.sheetNode = null; render();
    } catch (problem) { say(error, problem); }
  };
  const rows = (info?.browsers ?? []).map(entry => h('div.row', {}, h('span', {}, entry.name, h('span.muted', {}, entry.current ? ' · this browser' : ' · added ' + new Date(entry.addedAt).toLocaleDateString())),
    h('button.link', { onclick: remove(entry) }, entry.current ? 'Forget' : 'Remove')));
  const about = added ? 'This browser got its own key when you signed in with Google. The owner browser handles provider accounts and iPhone pairing.'
    : info?.google ? 'This browser is the hub’s owner. Any other browser gets its own key when you sign in with Google there. Forgetting this one cannot be undone without a fresh setup code.'
    : 'This browser is the hub’s owner. Forgetting it here cannot be undone without a fresh setup code.';
  return h('div.sub', {}, h('h4', {}, 'Browsers'), h('p.muted', {}, about),
    added ? null : h('div.row', {}, h('span', {}, 'This browser', h('span.muted', {}, ' · owner'))), rows, error,
    added ? null : h('button.link', { onclick: () => { if (confirm('Forget this browser’s owner credential? You will need a new setup to get back in.')) { save(null); signOut().finally(() => location.reload()); } } }, 'Forget this browser'));
}

// Ends this browser's Google session on the hub, if the hub has the Google gate on.
const signOut = () => fetch('/auth/signout', { method: 'POST', cache: 'no-store' }).catch(() => {});

// ---------- Live updates ----------
async function poll() {
  if (state.polling) return; state.polling = true;
  while (state.device) {
    try {
      const page = await api(`/v1/events?after=${state.cursor}&wait=25000&recoveryEpoch=${encodeURIComponent(state.installation.recoveryEpoch)}`);
      state.cursor = page.cursor;
      if (page.events.length) {
        await refresh();
        if (state.current) await openConversation(state.current).catch(() => { state.current = null; });
        render();
      }
    } catch (error) {
      if (error.status === 409) { state.installation = await api('/v1/installation').catch(() => state.installation); state.cursor = state.installation.eventCursor ?? 0; }
      else if (error.status === 401) { app.replaceChildren(h('div.card', {}, h('h1', {}, 'Fort'), h('p.error', {}, 'This browser is no longer paired with the hub.'),
        state.browsers?.you === 'browser' ? h('button.primary', { onclick: signInAgain }, 'Sign in again') : null)); break; }
      else await new Promise(resolve => setTimeout(resolve, 3000));
    }
  }
  state.polling = false;
}

async function start() {
  if (!state.device && !(await join())) return;
  try {
    state.installation = await api('/v1/installation');
    state.browsers = await api('/v1/hub/browsers').catch(() => null);
    state.cursor = state.installation.eventCursor ?? 0;
    await refresh();
    if (state.current) await openConversation(state.current);
    render(); void poll();
  } catch (error) {
    app.replaceChildren(h('div.card', {}, h('h1', {}, 'Fort'), h('p.error', {}, error.message),
      h('button', { onclick: () => start() }, 'Try again'), ' ', error.status === 401 ? h('button.link', { onclick: () => { save(null); start(); } }, 'Forget this browser') : null));
  }
}
start();
