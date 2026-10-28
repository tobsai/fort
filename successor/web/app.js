// Fort web client (Spec 063). Served by the hub itself, so every call is same-origin. The browser
// holds one device credential in localStorage; all text is rendered with textContent, never as HTML.
const STORE = 'fort.device.v1';
const app = document.getElementById('app');
const state = { device: load(), installation: null, agents: [], conversations: [], runners: [], current: null, snapshot: null, cursor: 0, polling: false, sheet: false };

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
  state.agents = agents.agents; state.conversations = conversations.conversations; state.runners = runners.runners; state.pool = runners.pool ?? null;
}
async function openConversation(conversationId) {
  state.current = conversationId;
  state.snapshot = conversationId ? await api(`/v1/conversations/${encodeURIComponent(conversationId)}/snapshot`) : null;
}

function render() {
  if (!state.device) return claimView();
  const shell = h('div.shell' + (state.current ? '.open' : ''), {}, sidebar(), chat());
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

function sidebar() {
  const agentsById = new Map(state.agents.map(agent => [agent.agentId, agent]));
  const item = (conversation, title, sub) => h('button.item' + (conversation === state.current ? '.active' : ''), { onclick: () => select(conversation) }, title, h('span.sub', {}, sub));
  const online = state.runners.filter(runner => runner.online);
  return h('aside.side', {},
    h('header', {}, h('h1', {}, 'Fort'), h('button.link', { onclick: () => { state.sheet = true; render(); }, 'aria-label': 'Setup' }, 'Setup')),
    h('nav.list', {},
      h('h2', {}, 'Agents'),
      state.agents.length ? state.agents.map(agent => item(agent.homeConversationId, agent.name, providerName(agent.binding.provider) + ' · ' + agent.binding.selectedModel))
        : h('p.muted.pad', {}, 'No agents yet. Open Setup to add one.'),
      state.conversations.filter(conversation => conversation.kind === 'shared').length ? h('h2', {}, 'Group chats') : null,
      state.conversations.filter(conversation => conversation.kind === 'shared').map(conversation =>
        item(conversation.conversationId, conversation.name, conversation.participantAgentIds.map(id => agentsById.get(id)?.name ?? 'Agent').join(', ')))),
    h('div.status', {}, h('span.dot' + (online.length ? '.on' : '')), online.length ? `${online.map(runner => runner.runnerName).join(', ')} connected` : (state.runners.length ? 'Your Mac is offline. Replies wait for it.' : 'No Mac connected yet.')));
}
const providerName = provider => state.installation?.runtimes?.find(runtime => runtime.provider === provider)?.name ?? provider;
async function select(conversationId) { try { await openConversation(conversationId); } catch (error) { alert(error.message); } render(); }

function chat() {
  if (!state.current || !state.snapshot) return h('section.chat', {}, h('p.empty', {}, 'Pick an agent to start talking.'));
  const conversation = state.conversations.find(item => item.conversationId === state.current);
  const participants = state.agents.filter(agent => conversation?.participantAgentIds.includes(agent.agentId));
  const runs = new Map(state.snapshot.runs.map(run => [run.requestId, run]));
  const rows = [];
  for (const message of state.snapshot.messages) {
    rows.push(h('div.msg.' + message.role, {}, message.role === 'assistant' || participants.length > 1 ? h('span.who', {}, message.role === 'user' ? 'You' + (message.recipientAgentName ? ' → ' + message.recipientAgentName : '') : message.agentName ?? 'Agent') : null, message.text));
  }
  const replied = new Set(state.snapshot.messages.filter(message => message.role === 'assistant').map(message => message.requestId));
  for (const run of runs.values()) {
    if (replied.has(run.requestId) || run.state === 'completed') continue;
    const working = ['queued', 'running', 'cancelRequested'].includes(run.state);
    rows.push(h('div.run' + (working ? '' : '.bad'), {}, working ? (run.state === 'queued' ? 'Waiting for your Mac…' : 'Thinking…') : `${run.state}: ${run.statusDetail ?? 'No reply.'}`));
  }
  const recipient = h('select', { 'aria-label': 'Send to' }, participants.map(agent => h('option', { value: agent.agentId }, agent.name)));
  const input = h('textarea', { rows: 1, placeholder: 'Message', 'aria-label': 'Message', onkeydown: event => { if (event.key === 'Enter' && !event.shiftKey && !event.isComposing && matchMedia('(pointer:fine)').matches) { event.preventDefault(); send(); } } });
  const send = async () => {
    const text = input.value.trim(); if (!text) return;
    const agent = participants.find(item => item.agentId === recipient.value) ?? participants[0];
    if (!agent) return;
    input.disabled = true;
    try {
      await api(`/v1/conversations/${encodeURIComponent(state.current)}/messages`, { requestId: crypto.randomUUID(), text, recipientAgentId: agent.agentId,
        bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: state.installation.recoveryEpoch, skills: [] });
      input.value = ''; await openConversation(state.current); render();
    } catch (error) { alert(error.message); input.disabled = false; }
  };
  return h('section.chat', {},
    h('header', {}, h('button.link.back', { onclick: () => { state.current = null; render(); } }, '‹ Back'), h('h2', {}, conversation?.name ?? 'Chat')),
    h('div.messages', {}, rows.length ? rows : h('p.empty', {}, 'No messages yet.')),
    h('form.composer', { onsubmit: event => { event.preventDefault(); send(); } }, participants.length > 1 ? recipient : null, input, h('button.primary', { type: 'submit' }, 'Send')));
}

// ---------- Setup sheet ----------
function setupSheet() {
  const body = h('div', {}, h('header', {}, h('h2', {}, 'Setup'), h('button.link', { onclick: () => { state.sheet = false; render(); } }, 'Done')));
  body.append(macSection(), accountSection(), agentSection(), phoneSection(), browserSection());
  return h('div.sheet', { onclick: event => { if (event.target.classList.contains('sheet')) { state.sheet = false; render(); } } }, body);
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
  const where = state.pool ? `This hub sends each run to the first connected Mac of ${state.pool.runners.map(id => `“${id}”`).join(', then ')}.` : `This hub sends turns to the Mac named “${node ?? '?'}”.`;
  return h('section', {}, h('h3', {}, 'Macs'), h('p.muted', {}, `Your Macs run the turns; the hub only routes them. ${where}`), list, add, out, error);
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
  const section = h('section', {}, h('h3', {}, 'Accounts'), h('p.muted', {}, 'Confirm which subscription each provider on your Mac is signed in to. Sign-ins happen on the Mac; nothing is stored here.'));
  for (const runtime of state.installation?.runtimes ?? []) {
    const line = h('span', {}, runtime.name, h('span.muted', {}, ' · checking…')), error = h('p.error');
    const button = h('button', { onclick: () => confirmAccount(runtime, line, button, error) }, 'Confirm');
    section.append(h('div.row', {}, line, button), error);
    api('/v1/account/sign-in?provider=' + encodeURIComponent(runtime.provider)).then(status => describeAccount(line, runtime, status)).catch(problem => say(error, problem));
  }
  return section;
}
function describeAccount(line, runtime, status) {
  const result = status.state === 'finished' ? status.result : undefined;
  const detail = status.state === 'unavailable' ? 'not available on this hub'
    : result?.kind === 'signedIn' && result.account ? `${result.account.userLabel}${status.selected ? '' : ' (not selected)'}`
    : status.state === 'challenge' ? 'finish signing in on your Mac' : status.state === 'starting' ? 'asking your Mac…'
    : result ? `last check: ${result.kind}` : 'not confirmed yet';
  line.replaceChildren(runtime.name, h('span.muted', {}, ' · ' + detail));
}
async function confirmAccount(runtime, line, button, error) {
  say(error); button.disabled = true;
  const query = '?provider=' + encodeURIComponent(runtime.provider);
  try {
    let status = await api('/v1/account/sign-in' + query);
    status = await api('/v1/account/sign-in/start' + query, { operationId: crypto.randomUUID(), expectedRevision: status.revision, recoveryEpoch: state.installation.recoveryEpoch, expiresAt: Date.now() + 110_000 });
    while (['starting', 'challenge', 'canceling'].includes(status.state)) {
      describeAccount(line, runtime, status);
      await new Promise(resolve => setTimeout(resolve, 1500));
      status = await api('/v1/account/sign-in' + query);
    }
    if (status.state === 'finished' && status.result.kind === 'signedIn' && status.result.account && !status.selected)
      status = await api('/v1/account/sign-in/select' + query, { operationId: status.operationId, revision: status.revision, recoveryEpoch: state.installation.recoveryEpoch, account: status.result.account });
    describeAccount(line, runtime, status);
  } catch (problem) { say(error, problem); }
  button.disabled = false;
}

function agentSection() {
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
      const agent = await api('/v1/agents', { name: name.value.trim(), provider: provider.value, model: model.value.trim(), instructions: instructions.value });
      await refresh(); await openConversation(agent.homeConversationId); state.sheet = false; render();
    } catch (problem) { say(error, problem); }
  } }, h('label', { for: 'agent-name' }, 'Name'), name, h('label', { for: 'agent-provider' }, 'Provider'), provider,
    h('label', { for: 'agent-model' }, 'Model'), model, models, h('label', { for: 'agent-instructions' }, 'Instructions'), instructions,
    h('div.row', {}, h('span'), h('button.primary', { type: 'submit' }, 'Add agent')), error);
  return h('section', {}, h('h3', {}, 'New agent'), form);
}

function phoneSection() {
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
  return h('section', {}, h('h3', {}, 'iPhone'), h('p.muted', {}, 'The Fort iPhone app talks to this hub directly, from anywhere.'), out, error);
}

function browserSection() {
  return h('section', {}, h('h3', {}, 'This browser'), h('p.muted', {}, 'This browser is the hub’s owner. Forgetting it here cannot be undone without a fresh setup code.'),
    h('button.link', { onclick: () => signOut().then(() => location.reload()) }, 'Sign out of Google'), ' ',
    h('button.link', { onclick: () => { if (confirm('Forget this browser’s owner credential? You will need a new setup to get back in.')) { save(null); signOut().finally(() => location.reload()); } } }, 'Forget this browser'));
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
      else if (error.status === 401) { app.replaceChildren(h('div.card', {}, h('h1', {}, 'Fort'), h('p.error', {}, 'This browser is no longer paired with the hub.'))); break; }
      else await new Promise(resolve => setTimeout(resolve, 3000));
    }
  }
  state.polling = false;
}

async function start() {
  if (!state.device) return render();
  try {
    state.installation = await api('/v1/installation');
    state.cursor = state.installation.eventCursor ?? 0;
    await refresh();
    if (state.current) await openConversation(state.current);
    render(); void poll();
  } catch (error) {
    app.replaceChildren(h('div.card', {}, h('h1', {}, 'Fort'), h('p.error', {}, error.message),
      h('button', { onclick: () => start() }, 'Try again'), ' ', error.status === 401 ? h('button.link', { onclick: () => { save(null); render(); } }, 'Forget this browser') : null));
  }
}
start();
