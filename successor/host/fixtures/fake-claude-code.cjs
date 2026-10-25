// A stand-in for the owner's installed Claude Code, used only by the host journey test. It speaks the same
// stream-json control protocol and `auth status --json` output as the real CLI, and signs in by writing a marker
// into its HOME once the (fake) browser login completes. It never touches the network.
const { existsSync, writeFileSync } = require('node:fs');
const { join } = require('node:path');

const VERSION = process.env.FAKE_CLAUDE_VERSION || '9.9.9';
const LOGIN = 'https://claude.com/cai/oauth/authorize?code=true&client_id=fixture&response_type=code&redirect_uri=http%3A%2F%2Flocalhost%3A54545%2Fcallback&state=fixture';
const EMAIL = 'owner@example.com', ORG = '0f6e7c1a-1111-4222-8333-944445555666';
const marker = join(process.env.HOME || '/nonexistent', 'fake-claude-signed-in');
const signedIn = () => existsSync(marker);
const args = process.argv.slice(2);
const out = frame => process.stdout.write(JSON.stringify(frame) + '\n');

// Like Claude Code 2.1.299 on macOS: a session ignores TMPDIR and opens /tmp/claude-<uid> unless CLAUDE_CODE_TMPDIR
// is set. The App Sandbox denies /tmp, so without it the real CLI exits before answering (`auth status` is unaffected).
if (args[0] === '-p' && !process.env.CLAUDE_CODE_TMPDIR) { process.stderr.write(`EPERM: operation not permitted, open '/tmp/claude-${process.getuid()}'\n`); process.exit(1); }
if (args[0] === '--version') { process.stdout.write(VERSION + ' (Claude Code)\n'); process.exit(0); }
// The same fields Claude Code 2.1.x prints. With only `fake-claude-credential-only` in HOME it reports what a
// credential without an account record looks like: signed in, but with no email or organization.
if (args[0] === 'auth' && args[1] === 'status') {
  const config = join(process.env.HOME || '/nonexistent', '.claude'), common = { analyticsDisabled: false, projectsDirectory: join(config, 'projects'), configDirectory: config };
  const credentialOnly = !signedIn() && existsSync(join(process.env.HOME || '/nonexistent', 'fake-claude-credential-only'));
  process.stdout.write(JSON.stringify(signedIn()
    ? { loggedIn: true, authMethod: 'claude.ai', apiProvider: 'firstParty', ...common, email: EMAIL, orgId: ORG, orgName: 'Owner', subscriptionType: 'max' }
    : credentialOnly ? { loggedIn: true, authMethod: 'claude.ai', apiProvider: 'firstParty', ...common, email: null, orgId: null, orgName: null, subscriptionType: null }
    : { loggedIn: false, authMethod: 'none', apiProvider: 'firstParty', ...common }, null, 2) + '\n');
  process.exit(signedIn() || credentialOnly ? 0 : 1);
}

const modelArg = args.indexOf('--model'), model = modelArg >= 0 ? args[modelArg + 1] : 'claude-sonnet-5-5';
const models = [
  { value: 'default', resolvedModel: 'claude-opus-5-5', displayName: 'Default (recommended)' },
  { value: 'opus', resolvedModel: 'claude-opus-5-5', displayName: 'Opus' },
  { value: 'sonnet', resolvedModel: 'claude-sonnet-5-5', displayName: 'Sonnet' },
];
const initialize = () => ({ commands: [], models, claude_code_version: VERSION, current_permission_mode: 'default', analytics_disabled: true,
  remote_control_auto_enable: false, remote_control_auto_connect_default: false, remote_control_auto_on_by_default: false,
  account: signedIn() ? { email: EMAIL, organization: 'Owner', subscriptionType: 'max', apiProvider: 'firstParty' } : {} });
const reply = text => [
  { type: 'system', subtype: 'init', session_id: 'session-1', cwd: process.cwd(), model, tools: [], mcp_servers: [], slash_commands: [], skills: [],
    permissionMode: 'default', apiKeySource: 'none', claude_code_version: VERSION, messaging_socket_path: process.env.XDG_RUNTIME_DIR + '/cc-socks/1.sock' },
  { type: 'assistant', uuid: 'u-1', parent_tool_use_id: null, session_id: 'session-1', message: { id: 'msg-1', role: 'assistant', model, content: [{ type: 'text', text }] } },
  { type: 'result', subtype: 'success', is_error: false, result: text, session_id: 'session-1', permission_denials: [] },
];
const success = (id, response) => out({ type: 'control_response', response: { subtype: 'success', request_id: id, response } });

let buffer = '';
process.stdin.on('data', chunk => {
  buffer += chunk.toString();
  for (let end = buffer.indexOf('\n'); end >= 0; end = buffer.indexOf('\n')) {
    const frame = JSON.parse(buffer.slice(0, end)); buffer = buffer.slice(end + 1);
    const subtype = frame.type === 'control_request' ? frame.request.subtype : undefined;
    if (subtype === 'initialize') success(frame.request_id, initialize());
    else if (subtype === 'claude_authenticate') writeFileSync(join(process.env.HOME || '/nonexistent', 'fake-claude-login-shown'), 'synthetic\n', { mode: 0o600 }), success(frame.request_id, { manualUrl: LOGIN.replace('code=true', 'code=manual'), automaticUrl: LOGIN });
    else if (subtype === 'claude_oauth_wait_for_completion') setTimeout(() => { writeFileSync(marker, 'synthetic\n', { mode: 0o600 }); success(frame.request_id, { account: { email: EMAIL, subscriptionType: 'max' } }); }, 300);
    else if (subtype === 'interrupt') success(frame.request_id, {});
    else if (frame.type === 'user') for (const reply_ of reply('Synthetic Claude reply')) out(reply_);
  }
});
process.stdin.on('end', () => process.exit(0));
