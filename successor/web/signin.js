// Fort web sign-in (Spec 066). The only script a browser gets before Google sign-in when the hub's
// Google gate is on. It shows Google's button, posts the credential to the hub for a session cookie,
// then loads the app.
const app = document.getElementById('app');

function card(...children) {
  const node = document.createElement('div'); node.className = 'card';
  const title = document.createElement('h1'); title.textContent = 'Fort';
  node.append(title, ...children); app.replaceChildren(node); return node;
}
function line(className, text) { const node = document.createElement('p'); node.className = className; node.textContent = text; return node; }

async function session() {
  const response = await fetch('/auth/session', { cache: 'no-store' });
  if (response.status === 404) return { google: false };
  if (!response.ok) throw new Error(`The hub answered ${response.status}.`);
  return response.json();
}

function loadGoogle() {
  return new Promise((resolve, reject) => {
    if (window.google?.accounts?.id) return resolve(window.google.accounts.id);
    const script = document.createElement('script');
    script.src = 'https://accounts.google.com/gsi/client'; script.async = true;
    script.onload = () => resolve(window.google.accounts.id);
    script.onerror = () => reject(new Error('Google sign-in could not load. Check your connection and try again.'));
    document.head.append(script);
  });
}

async function signIn(state) {
  const error = line('error', ''), button = document.createElement('div');
  card(line('muted', 'Sign in with the Google account that owns this hub.'), button, error);
  const id = await loadGoogle();
  // Each attempt needs a fresh one-time nonce from the hub.
  const arm = ({ clientId, nonce }) => id.initialize({ client_id: clientId, nonce, auto_select: true, ux_mode: 'popup', use_fedcm_for_prompt: true, callback: answered });
  async function answered({ credential }) {
    error.textContent = '';
    const response = await fetch('/auth/google', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ credential }), cache: 'no-store' });
    const value = await response.json().catch(() => ({}));
    if (response.ok) { await import('/app.js'); return; }
    error.textContent = value?.error?.message ?? `The hub answered ${response.status}.`;
    const next = await session().catch(() => null);
    if (next?.nonce) arm(next);
  }
  arm(state);
  id.renderButton(button, { theme: 'filled_black', size: 'large', shape: 'pill', text: 'signin_with' });
  id.prompt();
}

// The hub has one public address. A browser that claimed it on an older address carries its
// credential over in the URL fragment, which never reaches a server, and the new page takes it in.
const STORE = 'fort.device.v1';
function arrive() {
  const moved = /^#move=([A-Za-z0-9_-]+)$/.exec(location.hash)?.[1];
  if (!moved) return;
  history.replaceState(null, '', location.pathname);
  try {
    const device = JSON.parse(decodeURIComponent(escape(atob(moved.replaceAll('-', '+').replaceAll('_', '/')))));
    if (device?.installationId && device?.deviceId && device?.deviceToken && !localStorage.getItem(STORE)) localStorage.setItem(STORE, JSON.stringify(device));
  } catch { /* An unreadable move link changes nothing. */ }
}
function moved(publicOrigin) {
  const saved = localStorage.getItem(STORE);
  const link = document.createElement('a');
  link.className = 'button primary';
  link.href = publicOrigin + '/' + (saved ? '#move=' + btoa(unescape(encodeURIComponent(saved))).replaceAll('+', '-').replaceAll('/', '_').replaceAll('=', '') : '');
  link.textContent = 'Open Fort at ' + new URL(publicOrigin).host;
  card(line('muted', saved ? 'Fort has a new address. This browser’s owner credential moves with you when you open it there.' : 'Fort has a new address.'), link);
}

try {
  arrive();
  const state = await session();
  if (state.publicOrigin && state.publicOrigin !== location.origin) { moved(state.publicOrigin); throw null; }
  if (!state.google || state.signedIn) await import('/app.js');
  else await signIn(state);
} catch (problem) {
  if (problem) card(line('error', problem.message || 'Something went wrong.'));
}
