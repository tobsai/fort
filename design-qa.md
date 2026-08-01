# Fort Trustworthy Conversations — Design QA

## Comparison target

- Source visual truth: `/tmp/codex-remote-attachments/019fa0a0-4049-79e2-b385-eaa286e2947b/ECA7DB96-32CE-415B-9AE5-A195F2B54951/1-Photo-1.jpg`
- Baseline web capture: `/Users/tobiasgunn/.codex/visualizations/2026/07/26/019fa0a0-4049-79e2-b385-eaa286e2947b/before-trustworthy-conversations-web.png`
- Final web capture: `/Users/tobiasgunn/.codex/visualizations/2026/07/26/019fa0a0-4049-79e2-b385-eaa286e2947b/final-trustworthy-conversations-web.png`
- Final web comparison, source left and implementation right: `/Users/tobiasgunn/.codex/visualizations/2026/07/26/019fa0a0-4049-79e2-b385-eaa286e2947b/comparison-trustworthy-conversations-web.png`
- Final installed-native capture: `/Users/tobiasgunn/.codex/visualizations/2026/07/26/019fa0a0-4049-79e2-b385-eaa286e2947b/final-trustworthy-conversations-macos.png`
- Final native comparison, source left and implementation right: `/Users/tobiasgunn/.codex/visualizations/2026/07/26/019fa0a0-4049-79e2-b385-eaa286e2947b/comparison-trustworthy-conversations-macos.png`
- Thinking-state web capture: `/Users/tobiasgunn/.codex/visualizations/2026/07/26/019fa0a0-4049-79e2-b385-eaa286e2947b/orb-motion-working-web.png`
- Thinking-state comparison, source left and implementation right: `/Users/tobiasgunn/.codex/visualizations/2026/07/26/019fa0a0-4049-79e2-b385-eaa286e2947b/comparison-orb-motion-web.png`
- Final clean reduced-motion capture: `/Users/tobiasgunn/.codex/visualizations/2026/07/26/019fa0a0-4049-79e2-b385-eaa286e2947b/orb-motion-final-clean-web.png`
- Final responsive desktop capture (v047): `/Users/tobiasgunn/.codex/visualizations/2026/07/26/019fa0a0-4049-79e2-b385-eaa286e2947b/final-desktop-command-center-v047.png`
- Final phone capture at 390 x 844 CSS px: `/Users/tobiasgunn/.codex/visualizations/2026/07/26/019fa0a0-4049-79e2-b385-eaa286e2947b/final-mobile-command-center-v047.png`
- Final phone conversation-drawer capture at 390 x 844 CSS px: `/Users/tobiasgunn/.codex/visualizations/2026/07/26/019fa0a0-4049-79e2-b385-eaa286e2947b/final-mobile-conversations-v047.png`
- Compact-phone capture at 320 x 568 CSS px: `/Users/tobiasgunn/.codex/visualizations/2026/07/26/019fa0a0-4049-79e2-b385-eaa286e2947b/final-mobile-compact-v047.png`
- Final native iPhone command-deck capture: `/private/tmp/fort-mobile-deck-final.png`
- Final native compact-phone new-conversation capture: `/private/tmp/fort-mobile-new-se-final.png`
- Final native compact-phone approval capture: `/private/tmp/fort-mobile-approval-se-final.png`

## Normalization and state

- The source is 1280 x 910. The browser capture is 1265 x 893 from a 1280 x 904 CSS viewport; browser chrome accounts for the capture variance.
- The installed macOS capture is 1077 x 768. Its source reference was proportionally normalized to the same canvas for the combined comparison.
- Web QA used an isolated local database with real Fort response shapes and a genuine waiting gate. Native QA used current live Fort data.
- The final web state is paused for review, with inline approval choices and recorded activity. The final native state is a completed exact-reply run with its provider start, message, and exit evidence visible.
- Focused review covered the brand orb and conversation-status rail, the approval/activity region, and the profile/model/machine composer. The direct captures above retain those regions at readable resolution; the combined comparisons establish overall geometry and palette.
- Orb-motion follow-up used the in-app browser's 968 x 844 CSS viewport at device pixel ratio 2; its saved capture is 953 x 831 after browser chrome. The 1280 x 910 source was proportionally normalized to the same 831-pixel height for the combined focused comparison. Final interaction QA used real Terra provider activity under the browser's Reduce Motion preference; no synthetic Working state was needed.
- Responsive follow-up used the 1280 x 910 source and the browser-rendered v047 desktop (1265 x 893 output from a 1280 x 904 CSS viewport), phone (390 x 844), compact phone (320 x 568), and tablet (768 x 1024) states. The source and latest rendered captures were opened together at original resolution for visual comparison. The source has no phone composition, so phone QA evaluates a responsive translation of its hierarchy, palette, typography, orb identity, status semantics, and controls rather than claiming a pixel-identical mobile layout.
- Mobile and desktop captures used device pixel ratio 1. At 390 CSS px the document, main surface, and scroll width all measured 390 px; at 320 CSS px they measured 320 px. At 768 CSS px the two-column shell measured 753 px inside the scrollbar gutter with no horizontal overflow.
- Native iPhone QA used a 402 x 874 point iPhone 17 Pro at 3x (1206 x 2622 captured pixels) and a 375 x 667 point iPhone SE at 2x (750 x 1334 captured pixels). Browser/CSS measurements do not apply to these SwiftUI surfaces. The 1280 x 910 source and the final iPhone capture were opened together at original resolution. Because the source is a desktop composition, the native phone is evaluated as a faithful translation of its hierarchy, navy/brass/cyan palette, orb identity, state semantics, and control density rather than as a pixel-identical reflow.
- Focused native review covered the real raster orb and conversation header, newest-first state rows, exact agent/model/machine selectors, the live activity transcript, and the persistent approval dock. The original-resolution full capture made the brand, cards, controls, and navigation legible; the compact approval capture separately verifies the most constrained gated state.

## Findings

No actionable P0, P1, or P2 visual or interaction differences remain.

- The generated electric-blue intelligence core provides the nuanced, asymmetric Jarvis-like identity from the mockup across the app icon, brand mark, avatars, and agent cards.
- The three-pane command-center hierarchy, dense navy surfaces, cyan focus states, compact type, borders, radii, spacing, and status colors remain faithful to the reference.
- Projects is removed from visible web, macOS, and iOS navigation. The left rail is now one newest-first conversation list rather than a second project hierarchy.
- `New conversation` is the sole creation action. The competing upper-right `Give Direction` action is gone; `Assign` remains a deliberate composer action inside the conversation flow.
- At phone widths the conversation list becomes an off-canvas drawer instead of disappearing or compressing the transcript. Its trigger keeps the selected conversation's state visible in the header; opening the drawer exposes newest-first status rows and closing it restores the full transcript.
- The mobile transcript and composer occupy the full viewport width with no horizontal overflow. The transcript scrolls independently, while model, agent, machine, Assign, Send, and sign-off controls stay reachable. Interactive phone controls and approval actions measure at least 44 px high.
- Conversation rows expose Finished, Working, Paused, Needs approval, and Failed states with text as well as color. Needs-attention counts are reachable from both the top bar and the relevant row.
- A selected gate explains that work is paused and exposes `Approve & continue` and `Request changes` inline.
- Agent, exact model/profile, and eligible machine are explicit composer controls. The live preview exposes the configured default, GPT-5.5, GPT-5.6 Sol, GPT-5.6 Terra, and GPT-5.6 Luna; all five are currently ready on this Mac. Unavailable profiles remain visible with their closed reason and are not silently substituted.
- The selected transcript shows persisted provider activity rather than a decorative spinner. Named SSE events are subscribed explicitly, deduplicated, replayed after reconnect, and shown chronologically.
- Hand it off is now a single-flight interaction: the control becomes disabled and reads `Handing off…` in the activation frame, while an accessible inline status explains that Fort is confirming the route and starting the assignment. Transport and HTTP failures restore the control with inline error copy.
- Evidence-backed Working state now gives the existing intelligence-core raster a restrained asymmetric drift and blue energy pulse. The persistent Fort brand and active agent response move; human rows use a neutral person glyph, while handoff-decoration, paused, approval, terminal, and idle orbs remain still. Text labels continue to carry the state. Reduce Motion removes drift, rotation, and scale but deliberately preserves a slow non-spatial energy/glow pulse.
- **Turn this into work** appears only for a completed direct conversation. Promotion now pins the default assignment playbook and immutable revision before preview, so answer-like wording cannot be reclassified into Quick answer or conflict with the plan gate. The resulting routed run cannot expose the promotion action again.
- P3 observation: the web Send button retains its bright treatment when the composer is empty, although an empty submission safely no-ops. This does not obscure or block the primary flow.
- The native phone uses one clear information architecture: Chats, New, Assign, Today, and More. Projects and `Give Direction` are absent, and neither `Hand it off` nor duplicate creation vocabulary appears in the mobile flow.
- Human messages use a neutral person avatar. Fort and agent messages use the supplied intelligence-core raster; only evidence-backed Working state moves, with Reduce Motion preserving a restrained non-spatial glow pulse.
- The native composer exposes exact agent, latest model/profile, and eligible machine choices. Long names live in menus while compact labels preserve the phone layout.
- A blocked run replaces the composer with a persistent gate dock. Its explanation, full-width `Approve & continue`, and `Request changes` actions remain visible together on the 375 x 667 point compact device, and approval foreground/background contrast exceeds 9:1.
- Native conversation rows are newest-first and label Finished, Working, Paused, Needs approval, and Failed states with words, not color alone. Controls meet a 44 point minimum and include VoiceOver labels.

## Comparison history

1. The baseline exposed the reported gaps: generic/static identity, competing creation actions, no exact model chooser, duplicated project/conversation concepts, weak review affordance, and spinner-only activity.
2. The first implementation capture exposed out-of-order transcript events; event normalization and chronological sorting corrected it.
3. The interaction audit found that named SSE events were not reaching the browser and that a blocked run with historical provider evidence could be mislabeled Working. Explicit event listeners, event taxonomy, and paused-state precedence fixed both; a synthetic named `tool` event was observed live and then removed from the isolated QA database.
4. The first installed-native pass showed no execution evidence for a selected run and exposed internal `flow:` pseudo-agents in the rail. Run-detail replay, stderr support, and pseudo-agent filtering fixed those issues.
5. The final source/web and source/native comparison pairs were inspected together. No P0/P1/P2 mismatch remained.
6. Follow-up interaction QA reproduced an apparently inert Hand it off control: three clicks in five seconds created three real runs because the synchronous request had no in-flight UI state. The repaired control showed its disabled busy state and live status immediately; one verification click created exactly one run.
7. Orb-motion QA first confirmed that a bare running row remained Starting with zero animated orbs. After one persisted `started` event, the header and active response gained `orbCoreDrift, orbEnergyBreathe`; sampled transform and glow values changed across 360 ms. Changing the same run to succeeded removed every motion class despite the historical event.
8. Final reduced-motion QA launched a real GPT-5.6 Terra run. While provider activity was live, the Fort orb used `orbReducedEnergyPulse` for four seconds with `transform: none`; sampled brightness/saturation/glow values changed across 850 ms. The class disappeared at completion and the final response was `OK`.
9. Promotion QA reproduced the loop as an answer-classifier/plan-gate conflict. The repaired CTA previewed Feature work without error; one click immediately displayed the disabled `Handing off…` state, created one routed run, completed its first model turn, and paused at a visible sign-off gate. The routed run had no promotion CTA.
10. Approval-continuation QA reproduced the user's immediate failure on the isolated `Motion verification` fixture. Approval event 1964 had persisted correctly, but immutable Feature Work revision 1 dispatched stage 2 to absent `openclaw:main` 3.38 ms later. The catalog repair appended Feature Work revision 2 with exact `codex:gpt-5.5` for Design while preserving revision 1 and user edits. A fresh real run used Terra for the direct reply, Codex GPT-5.5 for plan and design, and Claude Sonnet for build; all three task stages completed after `Approve & continue`.
11. The first phone capture exposed a P1 broken layout: the desktop shell occupied only about 220 px of a 390 px viewport and left the rest blank. The responsive repair made conversations an off-canvas drawer, anchored transcript/composer to the viewport, stacked controls, and added touch-size actions. Post-fix captures at 390 x 844 and 320 x 568 show full-width content with no horizontal overflow; the drawer opened and closed with accurate `aria-expanded` state.
12. The first native iPhone pass duplicated navigation titles and allowed agent/model/machine labels to wrap. Hiding the redundant title, shortening the primary tab to Chats, and using compact selector labels restored the reference's dense hierarchy.
13. The first native approval pass let the composer compete with the human gate and pushed actions out of view. Replacing the composer with a persistent approval dock made the pause reason and both decisions immediately actionable.
14. Compact-device review found wrapping, low-contrast secondary text, truncated sign-off evidence, and an overly pale approval foreground. Those issues were corrected and both 375 x 667 point states were recaptured. The final source/native and compact-state comparisons have no actionable P0, P1, or P2 difference.

## Functional verification

- Browser: selected conversations and Needs You, opened and dismissed Request changes without submitting it, inspected model choices, and confirmed the final console had no warnings or errors.
- Browser handoff: confirmed the click now exposes `Handing off…` as disabled, announces `Fort is confirming the route and starting this assignment…`, and suppresses a second submit while the first request remains in flight.
- Browser SSE: injected one isolated named `tool` event, observed `Using live SSE verification` appear without reload, deleted the probe rows, verified no matches remained, and reloaded the clean review state.
- Browser orb motion: verified 0 -> active -> 0 motion-bearing state across a real Terra run; confirmed idle agents did not move; and sampled two distinct reduced-motion energy/glow filters while spatial transform remained `none`.
- Browser promotion: the completed direct `Reply with exactly OK` conversation pinned Feature work, previewed without error, accepted exactly one handoff, and produced one `Needs approval` assignment with `Approve & continue` and `Request changes` actions. The conversation list placed that new activity first.
- Browser approval continuation: a fresh `Approval verification 2026-07-28` direct run returned exactly `OK` on GPT-5.6 Terra. Its promoted Feature Work revision 2 paused at the plan gate, accepted approval, then completed Codex Design and Claude Build with exactly `OK`. No OpenClaw process was started. The six exact QA runs and their 161 events/9 node rows were removed from the isolated preview afterward; the database and three work directories remain recoverable from the backups below.
- Browser mobile: verified the phone conversation drawer opens and closes, `aria-expanded` moves from `false` to `true` and back, status remains visible in the mobile header, composer/profile/machine controls stay onscreen, the first approval button measures 327 x 44 px at 390 width, and no document horizontal overflow exists at 390, 320, or 768 CSS px.
- Browser console: no warnings or errors were present after the final responsive reload and interaction pass.
- Runtime acceptance: a direct GPT-5.6 Terra `Reply with exactly OK` run returned exactly `OK`. A second live-duration Terra run exercised the orb animation and also completed with final output `OK`. Provider warnings remain in the event log but do not contaminate terminal node output.
- Native: the installed app displays current run-detail activity, exact model/profile controls, complete newest-first conversations, and clear terminal/paused/failure states.
- Native iPhone: deterministic simulator states verified New, Conversation, and Approval on both full-size and compact phones. The transcript, model/profile/machine menus, tabs, promotion action, and persistent gate controls remain reachable without clipping or horizontal overflow.
- Native motion/accessibility: the real orb is still for idle, paused, approval, failure, and terminal states; truthful Working state animates; Reduce Motion removes spatial movement. Human rows never reuse the Fort identity. All compact primary controls meet the 44 point target and expose VoiceOver labels.
- Automated: `go test ./... -count=1`, targeted Go race tests, FortKit contract checks, and `git diff --check` passed after the final implementation changes.

## Local installation

- Current coordinator binary: `/Users/tobiasgunn/.local/bin/fort-0.12.15-mobile-async`
- Current coordinator SHA-256: `84d5a9ee0f49c2163ef52a618e246756b1d2658ffabc8d2212e38c7edc56edd0`
- Current launch-agent backup: `/Users/tobiasgunn/Library/LaunchAgents/io.tobsai.fort.plist.pre-0.12.15-20260728-1628`
- TestFlight: Fort Orchestrator `1.0.1 (2607281)` is uploaded, processed, and visible in the signed-in TestFlight library with a July 28, 2026 release date and a 90-day testing window.
- Stable signed archive: `/private/tmp/fort-testflight-2607281.VGqUTk/Fort-1.0.1-2607281-xcode26.xcarchive`, built by Xcode 26.6 (`17F113`) against `iphoneos26.5` (`23F81a`). The App Store Connect receipt records build `2607281`, state `PROCESSING` at handoff, zero errors, zero warnings, and `UPLOAD SUCCEEDED`; TestFlight availability was then verified independently in the TestFlight app.
- Release snapshot: `/private/tmp/fort-testflight-2607281.VGqUTk/apple`, source HEAD `292f7768715602cb5ca97de3611ee47cd0ea87d5`, tracked Apple diff SHA-256 `14b04aa97fbfb1acfdb3f66a906c312e8cd680605403284202ec8ff14f63ffa2`, full Apple source SHA-256 `c6b3a75e8906b65ddcae855adabfb83de3ab74c330263d933bb3db23b699df0e`.
- Earlier coordinator binary: `/Users/tobiasgunn/.local/bin/fort-0.12.10-thinking-orbs`
- Earlier SHA-256: `7a3a23e98fc9b2369a4f7fbf292aef84e7a0331b803559327d63c0eff11a3030`
- Orb-motion launch-agent backup: `/Users/tobiasgunn/Library/LaunchAgents/io.tobsai.fort.plist.pre-0.12.10-20260727-1836`
- Launch-agent backup: `/Users/tobiasgunn/Library/LaunchAgents/io.tobsai.fort.plist.pre-0.12.9-20260727-1009`
- Installed app: `/Applications/FortMac.app`, version `1.0.1 (2607234)`. This local Release build is unsigned and was launch-tested on this Mac.
- App backups: `/private/tmp/FortMac-2607233-backup-20260727-1018.app` and `/private/tmp/FortMac-2607234-pre-activityfix-20260727-1025.app`
- The native macOS source now gives the raster the same truthful drift/pulse and honors Reduce Motion, and FortKit contract checks pass. The installed app was not rebuilt in this follow-up because this Mac currently exposes Command Line Tools rather than a full Xcode developer directory.
- Isolated final web preview: `/private/tmp/fort-design-qa-019fa0a0/fort-conversation-v047`, version `0.12.14-mobile-command-center`, SHA-256 `1439eac145aefc96a0e4707951870c36c6fc9be338f4d13bbdd4c9ef1db50c3c`, running at `http://127.0.0.1:4187/` against the isolated QA database.
- Pre-repair database backup: `/private/tmp/fort-design-qa-019fa0a0/fort.db.pre-ready-feature-v046-20260727`.
- Post-verification/pre-cleanup database backup: `/private/tmp/fort-design-qa-019fa0a0/fort.db.pre-mobile-cleanup-v047-20260728`; recoverable QA work directories: `/private/tmp/fort-design-qa-019fa0a0/qa-work-v046-backup-20260728/`.
- The live 4087 coordinator runs `0.12.15-mobile-async`, reports healthy, serves the catalog-v2 local capability surface, and has passed real asynchronous direct and routed `Reply with exactly OK` runs. The connected Mac Mini still reports catalog version 0, so no claim is made that the remote node has been upgraded.

## Final result

passed
