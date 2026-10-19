# Fort submission draft

Local preparation under [Spec 059](../../specs/059-modular-node-v1.md), October 19,
2026. Nothing here has been entered into App Store Connect, submitted or published.
The current Mac development package is signed but unlaunched; the iPhone device
build is unsigned. Installed provider behavior, physical devices and TestFlight
delivery have not passed acceptance.

## App and beta text

App name: Fort. Keep the current version 0.1.0 and build 1 as local development
values until the selected remote record and its uploaded build history are checked.
Do not reuse them as a release-number decision. The existing remote Fort Orchestrator
record, Apple ID 6786748810, bundle io.mtree.fort and SKU fort-ios, is preserved.
Its observed iOS 1.0 is Prepare for Submission; naming/platform migration is unresolved.

Beta description draft:

> Fort is a personal workspace for AI conversations and reusable instructions.
> This beta is intended to test setup, conversation history and recovery on the
> selected Mac. Access requires the separately configured Fort host and an explicitly
> selected provider account. Use only the build and account approved for your test.

This describes the intended bounded beta, not completed installed acceptance.
Before external testers receive it, replace the setup sentence with the verified
instructions for that exact signed build. Do not advertise working iPhone pairing,
Claude availability, voice, attachments, arbitrary tools, unattended startup,
complete host isolation or included paid-provider access. Those claims lack current
installed acceptance. Local image display is not provider image transmission.

What to test, after the exact installed grant and build have been accepted:

- Open Fort from a stopped state and check that its first window appears.
- Confirm the displayed account/workspace and selected model before sending.
- Send one authorized plain-text message. Verify the reply and retained history.
- Quit/reopen, stop/resume and reconnect using the approved procedure. Report
  missing history, incorrect account identity or an unconfirmed result.
- Disconnect and remove the approved installation while keeping history unless
  deletion was separately requested.

Do not invite testers to run this list before setup and authority exist. Fixture
screenshots are internal evidence, not App Store screenshots or provider proof.

## Reviewer and contact fields

| Field | Local draft or required decision |
| --- | --- |
| Beta app description | Text above, held until exact installed acceptance |
| Feedback email and review contact | Owner must designate actual monitored contact details; do not infer them from a login account |
| Support and privacy-policy URLs | No live URLs verified for the successor; prepare/review before publication |
| Reviewer setup | Explain the single-owner Mac host, explicit provider sign-in/account selection and the verified cleanup path; no credentials in this file |
| Review access | A separately approved bounded test account/setup is needed if Apple requires access; do not share the owner's credentials |
| Category, age rating, content rights and territories | Undecided; answer from the final shipped behavior and owner's actual rights/availability decisions |
| Screenshots and accessibility declarations | Capture/evaluate the final signed installed build; current synthetic UI checks establish only their documented limited behavior |
| Pricing, subscriptions and in-app purchases | No pricing or purchase offering selected by this work |

Apple requires a beta description and feedback address for the external testing
workflow. Beta text can differ from later store text. See Apple's
[TestFlight information guide](https://developer.apple.com/help/app-store-connect/test-a-beta-version/provide-test-information/).

## Privacy facts and unresolved answers

This is a data-flow worksheet, not submitted privacy-label answers or a published
privacy policy. Apple's questions cover integrated third parties as well as Fort,
and its collection definition concerns data transmitted off-device and retained
beyond servicing the request. Local storage alone does not justify a blanket
"Data Not Collected" answer when the selected provider receives content. See
[Apple's privacy definitions](https://developer.apple.com/app-store/app-privacy-details/).

| Data or boundary | Current source behavior | Remaining verification |
| --- | --- | --- |
| Messages, replies, notes, instruction bundles and routine definitions | Fort's local ledger/cache retains content; accepted execution supplies relevant text/context to the selected runtime | Exact provider payload/retention and final device behavior |
| Provider account/workspace identifiers | Explicit owner selection is pinned and checked through dispatch | Actual signed provider identity and final data-disclosure classification |
| Pairing and runtime credentials | Native pairing uses its credential boundary; the owned runtime has a separate private session/auth location | Final signed storage and removal proof; do not claim every credential is in Keychain |
| Imported images | Native source supports local image caching/display; current execution remains plain text | Installed file-picker permission and actual export/backup behavior |
| Phone transport | Optional authenticated private transport has synthetic evidence; normal Mac bootstrap leaves it disabled | Exact network grants, native composition and physical phone acceptance |
| Analytics and tracking | No Fort analytics SDK or tracking service was added by this release work | Review the complete shipped Node/Codex dependency payload and provider practices before answering final labels |
| Retention and deletion | Ordinary disconnect/removal keeps history; deletion is a separate choice | Final installed removal/recovery, backup behavior and external-provider retention |

The [static source and payload inventory](../evidence/2026-10-17-node-foundation/privacy-runtime-inventory.md)
now covers all 30 signed Mac Mach-O objects and the unsigned iPhone binary. All
31 undefined-symbol reads passed; neither artifact contains a privacy manifest.
Native file-metadata checks and Mac shutdown timers have traced callers. Node,
SQLite, libuv, OpenSSL and Codex imports still need exact use/manifest review.
CFPreferences signals are tracked separately from Apple's listed UserDefaults API.

This completes the bounded static inventory, not the complete runtime privacy audit.
Candidate reasons and platform-scope limits are in the evidence; none was submitted
or encoded in a manifest. Existing synthetic runtime-control checks passed 407 tests
with one optional budget skip. They support the specific telemetry/update opt-outs,
not provider retention or a blanket Data Not Collected answer. Do not add an empty
manifest or choose reasons solely to silence an upload warning. Inspect accurate
manifest placement and the final archive report after remaining uses are resolved.

## Encryption and agreements

The bundle includes native secure storage/transport code, a Node/OpenSSL dependency
closure and the official runtime's networking. Do not copy a historical export
answer or set `ITSAppUsesNonExemptEncryption` to false without reviewing the final
payload. Document algorithms, uses and distribution countries, then have the owner
resolve the applicable questionnaire/documentation. Apple's
[export-compliance workflow](https://developer.apple.com/help/app-store-connect/manage-app-information/overview-of-export-compliance/)
requires a determination even for use of standard or operating-system encryption.
No classification or exemption is asserted here.

The coordinator supplied Apple confirmation message `1a155282da6a6b05`, received
October 19 at 17:14:14 UTC, that the Program License Agreement was signed for
Maple Tree Enterprises LLC, team T3JB5MYZ93. It does not name the agreement version.
The authenticated portal matches that team. Do not request acceptance of that same
agreement again. Business lists Free Apps Agreement as Active through September 11,
2027, and Paid Apps Agreement as Active with New Agreement Available. The separate
Schedule 2 update banner does not alone establish a blocker for the intended beta.
Do not accept its new terms or infer acceptance from the earlier email.

## Existing artwork

The catalog reuses Fort's tracked native icon bytes from commit
`0960a39b294e2231614c47efb03ac2072f8ef5b0`, following the earlier Fort brand refresh.
Seven source PNGs supply eleven Mac/iPhone icon slots. No donor artwork, trademark,
mascot or legacy implementation was imported. Image provenance and compiled-bundle
checks are in [release-assets.md](../evidence/2026-10-17-node-foundation/release-assets.md).
This preserves existing artwork; it is not a new legal ownership certification.
