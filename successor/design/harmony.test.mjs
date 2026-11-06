import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { approvalCopy, runStatus } from '../web/transcript.js';

// The web client and the Mac and iPhone apps are one product: the same screens, words and order.
// These checks read both sources, so a label changed on one side and not the other fails here.
const read = path => readFileSync(new URL(path, import.meta.url), 'utf8');
const web = read('../web/app.js');
const kit = name => read(`../apple/FortKit/Sources/FortKit/${name}.swift`);
const fixture = JSON.parse(read('./screens/fixture.json'));

test('Setup has the same pages in the same order on the web and in the apps', () => {
  const apps = [...kit('PauseMenu').match(/var title: String \{ switch self \{([^}]*)\}/)[1].matchAll(/"([^"]+)"/g)].map(match => match[1]);
  assert.deepEqual(apps, ['Agents', 'Routines', 'Accounts', 'Devices']);
  // Setup opens on a menu; each entry is its own page. Routines live in the apps only until the hub serves them.
  const pages = [...web.match(/const SETUP = \[([\s\S]*?)\];/)[1].matchAll(/title: '([^']+)'/g)].map(match => match[1]);
  assert.deepEqual(pages, apps.filter(title => title !== 'Routines'));
  assert.ok(/page: 'devices', title: 'Devices', optional: true/.test(web) && kit('PauseMenuView').includes('"Optional"'), 'Devices is marked optional on both');
  for (const sub of ['Macs', 'iPhone', 'Browsers', 'New agent']) assert.ok(web.includes(`h('h4', {}, '${sub}')`), `web Setup has ${sub}`);
  assert.ok(!/Playbook|PAUSED|RESUME|RECRUIT/.test(kit('PauseMenuView')), 'the apps say Setup, Done and New agent, like the web');
});

test('the main screen is the Quest log and the Chats, with the first run in front of it until there is an Agent', async () => {
  const quest = kit('QuestLogView');
  assert.ok(!/statusBar/.test(quest) && !/statusBar|footer\.status/.test(web), 'no status bar under the main screen');
  assert.ok(!/deskSummary/.test(web) && !/summary:/.test(kit('ComicChat')), 'no summary caption in the top bar');
  const { setupSteps } = await import('../web/transcript.js');
  const titles = setupSteps({ account: false, agents: 0 }).steps.map(step => step.title);
  const swift = kit('SetupSteps');
  for (const title of titles) assert.ok(swift.includes(`"${title}"`), `the apps' first run says ${title}`);
  for (const words of ['Set up Fort', 'You’re set', 'Start chatting', 'Go to the Quest log'])
    assert.ok(web.includes(`'${words}'`) && kit('GetStartedView').includes(`"${words}"`), `both say ${words}`);
});

test('the chrome uses the same words everywhere', () => {
  const chat = kit('ComicChat'), messenger = kit('MessengerView'), composer = kit('MessageComposer');
  assert.ok(web.includes("'Setup')") && chat.includes('Button("Setup",'), 'the top bar opens Setup');
  assert.ok(web.includes("'‹ Quest log'") && messenger.includes('Button("‹ Quest log",'), 'a chat goes back to the Quest log');
  assert.ok(web.includes("placeholder: 'Message'") && composer.includes('("Message" as NSString)') && messenger.includes('TextField("Message",'), 'the composer says Message');
  assert.ok(web.includes("'Send')") && messenger.includes('Button("Send")'), 'the composer sends with Send');
  assert.ok(web.includes("'Done')") && kit('PauseMenuView').includes('Text("Done")'), 'Setup closes with Done');
});

test('approvals read the same on the web and in the apps', () => {
  const swift = kit('ComicChat');
  // The same vectors are in FortKit's ComicChatTests.
  const vectors = [[undefined, 'pending', 'Approve', null], ['routineGate', 'pending', 'Continue routine', null], [undefined, 'approved', 'Approve', 'Approved'],
    ['routineGate', 'approved', 'Continue routine', 'Continued'], [undefined, 'rejected', 'Approve', 'Rejected'], [undefined, 'expired', 'Approve', 'Expired before a decision']];
  for (const [kind, state, approve, decided] of vectors) {
    const copy = approvalCopy({ kind, state });
    assert.equal(copy.approve, approve); assert.equal(copy.reject, 'Reject'); assert.equal(copy.decided, decided);
    if (decided) assert.ok(swift.includes(`"${decided}"`), `the apps say ${decided}`);
  }
  assert.ok(swift.includes('"Needs you"') && web.includes("'Needs you'"), 'an open approval is labelled Needs you');
});

test('every run state has the same sentence in the apps as on the web', () => {
  const contract = kit('Contract');
  for (const state of ['queued', 'running', 'cancelRequested', 'waitingForApproval', 'completed', 'failed', 'outcomeUnknown', 'canceled', 'closedUnknown'])
    assert.ok(contract.includes(`case "${state}": "${runStatus({ state }).text}"`), `${state} reads "${runStatus({ state }).text}" in the apps`);
});

test('the screenshot fixture is one consistent hub', () => {
  const agents = new Set(fixture.agents.map(agent => agent.agentId));
  for (const conversation of fixture.conversations) {
    for (const id of conversation.participantAgentIds) assert.ok(agents.has(id), `${conversation.name} names a known agent`);
    const snapshot = fixture.snapshots[conversation.conversationId];
    assert.ok(snapshot, `${conversation.name} has a snapshot`);
    assert.equal(snapshot.conversationId, conversation.conversationId);
    for (const approval of snapshot.approvals) assert.ok(snapshot.runs.some(run => run.runId === approval.runId), 'an approval belongs to a run');
  }
  assert.ok(fixture.snapshots[fixture.open], 'the chat to open exists');
});
