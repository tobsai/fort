# Candidate B — attended Mac test

**Not authorized to run yet.** After the exact execution decision, completed
disabled wiring, verified pins and successful runtime preflight, the operator
will announce that the candidate is ready. Be present before the clock starts.
Each of the two possible sessions includes all setup and cleanup within five
minutes. If a step fails or time runs out, stop; do not retry or add a message.

## First session

1. Open only the operator-identified pinned FortMac app. In **Connection**, enter
   the supplied loopback address, installation ID and single-use local enrollment
   code, then choose **Enroll this device**. Keep the code out of chat and
   screenshots. This creates one Fort-only Keychain credential; it is not an
   OpenAI login. If macOS blocks launch or credential storage, tell the operator
   and stop. Do not change system security settings.
2. Use **Workspace** to create these exact items. Both Agents use model
   `gpt-6-astra`. Save each item before moving on.

| Item | Name | Instructions / participants |
| --- | --- | --- |
| Agent | Writer | `Reply briefly using supplied context.` |
| Agent | Reviewer | `Check the sample claim.` |
| Skill | Check marker | `End with [CHECKED].` |
| Conversation | Sample planning | Writer and Reviewer |

3. Send the following messages once each, in order. Wait for the operator to
   confirm the durable completed run after each reply. Select the **Check marker**
   skill only for sample 3; turn it off before sample 4.

| # | Conversation / recipient | Exact message | Observe |
| --- | --- | --- | --- |
| 1 | Writer Home / Writer | `Remember the sample word cedar. Reply with that word only.` | One Writer reply containing only `cedar`. |
| 2 | Reviewer Home / Reviewer | `In one sentence, explain why a checklist is useful.` | One brief Reviewer reply in its separate Home. |
| 3 | Writer Home / Writer; Check marker selected | `Suggest one short title for a sample packing list.` | One title ending with `[CHECKED]`; the run shows the selected skill. |
| 4 | Sample planning / explicitly choose Reviewer | `Our sample plan is to pack a red notebook. Repeat the item.` | One Reviewer-attributed reply repeating the red notebook. |

An unexpected reply is a test result, not a reason to send another prompt. Stop
and report it. Do not click approval, retry or close-unknown controls to force a
pass. The operator records the four receipt/run IDs and checks every completion.

4. Quit Fort fully when instructed. The operator confirms that the actual native
   pending queue is empty, the listener is closed, and app/service/runtime cleanup
   is complete. A closed window alone is insufficient. If any gate fails, there
   is no second session.

## Conditional second session

The operator checks the empty queue before launching the app and revalidates the
four completed receipts in the same ledger. After the second runtime preflight,
reopen that exact app and verify Writer, Reviewer, Check marker, Sample planning
and their prior messages are still present. Do not enroll again or change items.

Open Writer Home, ensure no skill is selected, and send once:

> What sample word did I ask you to remember earlier? Reply with that word only.

Observe one Writer reply containing only `cedar`, then quit when instructed.
The operator confirms the fifth completion, empty queue and final cleanup.
The app, ledger and Fort credential remain local; no permanent service stays up.

Tell the operator whether navigation, recipient labels, skill selection, readable
text and history restoration behaved as described, and quote any visible error
without credentials. This is manual observation, not accessibility/screenshot
acceptance, a physical-iPhone test, or proof of host isolation.
