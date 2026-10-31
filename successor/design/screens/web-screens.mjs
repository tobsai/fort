// Side-by-side screenshots, web half: serves the real web client from successor/web with a mock hub
// that answers from fixture.json (the same records FortKit's ScreenshotTests load into the Mac app),
// then captures the Quest log, a chat and Setup in light and noir with Playwright.
//
//   node design/screens/web-screens.mjs <output-directory>
//
// Playwright is not a dependency of Fort; CI installs it for this step only.
import { createServer } from 'node:http';
import { mkdirSync, readFileSync } from 'node:fs';
import { dirname, extname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const web = join(here, '..', '..', 'web');
const fixture = JSON.parse(readFileSync(join(here, 'fixture.json'), 'utf8'));
const out = process.argv[2] ?? join(here, 'out');
mkdirSync(out, { recursive: true });

const types = { '.js': 'text/javascript', '.css': 'text/css', '.html': 'text/html', '.svg': 'image/svg+xml', '.webmanifest': 'application/manifest+json' };
const send = (res, status, value) => { res.writeHead(status, { 'content-type': 'application/json' }); res.end(JSON.stringify(value)); };
function mock(url) {
  const path = url.pathname;
  if (path === '/v1/installation') return fixture.installation;
  if (path === '/v1/agents') return { agents: fixture.agents };
  if (path === '/v1/conversations') return { conversations: fixture.conversations };
  if (path === '/v1/runners') return { runners: fixture.runners, pool: { runners: fixture.runners.map(runner => runner.runnerId) } };
  const snapshot = path.match(/^\/v1\/conversations\/([^/]+)\/snapshot$/);
  if (snapshot) return fixture.snapshots[decodeURIComponent(snapshot[1])];
  if (path === '/v1/account/sign-in') return { state: 'finished', selected: true, revision: 1, result: { kind: 'signedIn', account: { userLabel: 'Your plan' } } };
  if (path === '/v1/models') return { state: 'ready', models: [] };
  if (path === '/v1/pairing/companion') return { pairing: { state: 'none' } };
  return {};
}
const server = createServer((req, res) => {
  const url = new URL(req.url, 'http://fort.test');
  if (url.pathname === '/auth/session') return send(res, 404, {});
  // The event stream never has news here, so the page stays as first drawn.
  if (url.pathname === '/v1/events') return setTimeout(() => send(res, 200, { cursor: 0, events: [] }), 60_000).unref();
  if (url.pathname.startsWith('/v1/')) return send(res, 200, mock(url));
  const file = url.pathname === '/' ? 'index.html' : url.pathname.slice(1);
  try { const body = readFileSync(join(web, file)); res.writeHead(200, { 'content-type': types[extname(file)] ?? 'application/octet-stream' }); res.end(body); }
  catch { res.writeHead(404); res.end(); }
});
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
const origin = `http://127.0.0.1:${server.address().port}`;

const { chromium } = await import('playwright');
const browser = await chromium.launch(process.env.FORT_CHROME_CHANNEL ? { channel: process.env.FORT_CHROME_CHANNEL } : {});
const open = fixture.conversations.find(conversation => conversation.conversationId === fixture.open);
try {
  for (const colorScheme of ['light', 'dark']) {
    const page = await browser.newPage({ viewport: { width: 1280, height: 800 }, deviceScaleFactor: 1, colorScheme });
    await page.addInitScript(device => localStorage.setItem('fort.device.v1', JSON.stringify(device)), { installationId: fixture.installation.installationId, deviceId: 'screens', deviceToken: 'screens' });
    await page.goto(origin + '/');
    await page.getByRole('heading', { name: 'Quest log' }).waitFor();
    await page.screenshot({ path: join(out, `web-quest-log-${colorScheme}.jpg`), quality: 80 });
    await page.locator('.chatpanel', { hasText: open.name }).first().click();
    await page.locator('.chat header h2', { hasText: open.name }).waitFor();
    await page.screenshot({ path: join(out, `web-chat-${colorScheme}.jpg`), quality: 80 });
    await page.getByRole('button', { name: 'Setup' }).click();
    await page.locator('.sheet h3', { hasText: 'Crew' }).waitFor();
    await page.waitForTimeout(300);
    await page.screenshot({ path: join(out, `web-setup-${colorScheme}.jpg`), quality: 80 });
    await page.close();
  }
} finally {
  await browser.close();
  server.close();
}
console.log(`Web screenshots written to ${out}`);
