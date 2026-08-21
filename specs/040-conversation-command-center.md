# Spec 040 — Conversation Command Center

**Status:** approved-by-instruction (Toby selected the attached design, then
superseded its project-room treatment with a conversation-only shell and made
basic agent execution reliability the first-order requirement.)
**Design source:** `/tmp/codex-remote-attachments/019fa0a0-4049-79e2-b385-eaa286e2947b/ECA7DB96-32CE-415B-9AE5-A195F2B54951/1-Photo-1.jpg`

## Goal

Replace the previous Figma-led desktop Command Deck presentation with the
attached conversation-first control center. The desktop surface keeps Fort's
existing deterministic routing and human-checkpoint semantics, but reorganizes
the default view into three persistent regions: conversation navigation, the
active conversation and assignment progress, and live agent/machine status.

The native macOS shell uses the same hierarchy and visual grammar. Narrow web
and iPhone layouts stay responsive instead of squeezing the desktop columns onto
a phone.

This revision closes seven interaction and trust gaps found in live use:

- a generated electric-blue Fort intelligence core is the repeated app-icon,
  brand, and agent identity, with the dimensional energy and asymmetry of the
  supplied mockup rather than a flat castle or static target;
- **New conversation** is the sole conversation-creation action while **Assign**
  remains a separate routed-work view;
- a conversation can choose an exact Fort-owned agent/model profile and an
  eligible machine, with no silent provider, model, or machine substitution;
- the Projects route, tab, pane, and project-room presentation are removed for
  now; the underlying run data and APIs remain intact;
- conversations are ordered by their latest real activity, never by status
  priority, and every row has a readable queued, working, paused/review,
  finished, failed, or canceled state;
- waiting gates appear inline with plain-language **Approve & continue** and
  **Request changes** actions, and every affected conversation row is
  visibly marked **Needs approval**;
- a running conversation shows a short timeline derived only from persisted or
  streamed Fort events. A spinner, elapsed clock, or `running` status by itself
  is not evidence that an agent is doing work.

The live failure behind the basic `Reply OK` report is part of this spec. The
fixed classifier treated conversational reply imperatives as feature requests;
the resulting feature-work route later reached the closed `openclaw:main`
profile. `Reply`, `respond`, `answer`, and `say` imperatives must instead resolve
deterministically to the existing quick-answer playbook. This does not authorize
substituting an unavailable provider in any other playbook: unavailable stages
continue to fail closed and their exact reason must be visible.

## Approach

1. Keep the useful global navigation but remove Projects everywhere it appears
   in the web, macOS, and iOS clients. Restyle the remaining navigation in the
   reference's dark navy and electric-blue language. Rename the native route
   label to **Assign** and remove the duplicate upper-right **Give direction**
   action. A stale saved Projects route falls back to Deck.
2. Make Deck a full-height three-pane workspace:
   - left: New conversation, Needs you, and the complete recent-conversation
     history ordered by latest activity;
   - center: active transcript, inline sign-off, honest checkpoint progress,
     and a working conversation composer;
   - right: current agent, other agents, machines, and operational status.
3. Derive progress and attention from the existing board, gate, backlog,
   machine, playbook, and run-detail endpoints. Do not invent estimated
   progress: only accepted checkpoints appear complete.
4. Add a read-only profile-options endpoint derived from Fort's closed capability
   catalog and latest secret-free readiness snapshot. A selected option sends
   its canonical `profile` ID. The server resolves that ID to its exact
   agent/model pair before deterministic dispatch, and the existing profile gate
   remains authoritative for target-machine readiness. Unknown or unavailable
   profiles are never replaced by a different provider/model.
5. Carry the selected profile and derived model through `task.Task`,
   `runtime.RunSpec`, and the persisted run summary so the UI reports the model
   actually requested rather than guessing from a playbook.
6. Resolve reply imperatives through fixed, inspectable classifier rules to the
   one-stage quick-answer playbook. Preserve explicit playbook and task-type
   precedence, and never rewrite an explicitly selected provider/model/machine.
7. Derive conversation activity from the append-only event stream. Gate and
   terminal states take precedence; a running run with no actual provider event
   says that it is starting or waiting for its first event. Event timestamps,
   event IDs, and exact terminal errors remain visible and deduplicated.
8. Preserve every normalized provider message in that activity stream, but use
   only the terminal normalized message as the task output passed downstream or
   returned as a quick answer. Startup warnings and intermediate commentary must
   never be concatenated into an otherwise exact response.
9. Use the generated nuanced Fort intelligence-core raster as the canonical app
   icon and repeated brand/avatar asset. Generate size-appropriate derivatives
   for web, macOS, iOS, and watchOS. No model call enters the routing path;
   profile/catalog lookups and task classification stay pure and deterministic.
10. A handoff is a single-flight action. In the same frame as activation, the
   client disables the handoff control, marks it busy, and announces that Fort
   is starting the selected route. Repeated activation while that request is in
   flight dispatches nothing. Transport and HTTP failures appear in the same
   inline status region and restore the control; success opens the resulting
   conversation, whose event timeline remains the evidence that work continued.
11. Superseded for the Fort orbital mark by Spec 045: the intelligence-core
   raster always carries restrained ambient drift and energy, while truthful
   Working state increases that energy and retains explicit text. It is not a
   generic spinner and motion alone never communicates runtime status. Reduced-
   motion settings suppress spatial drift, rotation, and scaling but preserve a
   slow non-spatial glow pulse.
12. **Turn this into work** is a one-way promotion offered only on a completed
   direct conversation with no `flow_id`. It never appears on starting, working,
   paused, gated, failed, canceled, or already routed flow/playbook assignments;
   the resulting assignment therefore cannot promote itself again. Promotion
   explicitly pins the current default assignment playbook and its immutable
   revision before route preview, so answer-like wording in the completed turn
   cannot be reclassified as an inline answer or conflict with the plan gate.
   The untouched default assignment must also remain executable across its
   post-approval stages on the accepted single-machine profile set: Feature
   work Break down and Design both use exact `codex:gpt-5.5`. Fort appends this
   as a new immutable playbook revision, preserves the prior OpenClaw revision
   for audit/replay, and never overwrites user-edited playbooks.
13. Human-authored transcript rows use an explicit human role and a neutral
   person avatar. Fort and agent rows alone use the Fort intelligence core;
   display text such as `You` is never used to infer identity.
14. The model chooser exposes the current closed catalog-v2 profiles and their
   real readiness. Codex Sol, Terra, and Luna map to their exact provider IDs,
   and playbook model controls derive from the same `/api/profiles` response as
   the conversation composer. A second hard-coded model list is forbidden.
15. At phone widths, the conversation history remains available in an
   off-canvas navigation sheet opened from the active-conversation header; it
   is never removed from the interface. The transcript owns the remaining
   viewport and scrolls independently while the composer remains reachable at
   the bottom. The active state and any waiting sign-off remain readable
   without opening a detail view. Agent, exact model, and machine controls use
   touch-sized rows, sign-off actions become full-width targets, and every
   phone layout contains overflow within its own scroller instead of widening
   the page.
16. The native iPhone client uses that same conversation-first hierarchy rather
   than opening a generic run inspector. **Conversations** is newest-first and
   attention-aware, **New** opens a direct conversation with exact Agent, Model,
   and Machine controls, and **Assign** remains the distinct routed-work surface.
   Selecting a conversation opens a full-height transcript with its readable
   state, persisted activity, inline sign-off, checkpoint progress, and a pinned
   composer. The history can be reopened from that thread without losing the
   selected conversation.
17. Native iPhone submission and promotion are single-flight. The tapped control
   becomes busy synchronously, the resulting run opens on success, and any exact
   server failure including its request ID remains visible in the conversation
   instead of disappearing behind an alert-only spinner. **Turn this into work**
   uses the default assignment playbook once and never reappears on its routed
   result.
18. A TestFlight release is cut only after FortKit contract checks, an unsigned
   simulator build, a signed archive, and a simulator visual comparison of the
   reference identity and mobile hierarchy pass. The release increments the
   build number and App Store Connect must acknowledge the uploaded build.

## Affected files

- `ui/page.go` — desktop shell, transcript rendering, assignment progress, and
  truthful event activity, newest-first conversations, and responsive fallback.
- `ui/contract.go`, `ui/server.go` — closed profile option and exact one-run
  profile handoff contracts.
- `core/task`, `core/engine`, `core/store` — exact profile/model propagation and
  honest run metadata.
- `core/capability`, `exec/capability` — approved catalog-v2 Codex profiles and
  the verified locally installed Codex app executable/schema contract.
- `core/playbook` — deterministic reply-imperative classification.
- `ui/page_source_test.go` — source-level layout and interaction contract.
- `ui/apple/macOS/FortWindow.swift`, `ui/apple/iOS/BoardView.swift` — native
  conversation-only navigation, status badges, and truthful event activity.
- `ui/apple/FortKit/Sources/FortKit/CommandDeckStyle.swift` — shared navy and
  electric-blue visual tokens used by the native desktop shell.
- `ui/apple/FortKit/Sources/FortKit/{Models,FortClient,CommandDeck}.swift` —
  profile options, exact handoff, newest-first ordering, and pure event-derived
  conversation presentation.
- `assets/fort-icon.png`, platform `AppIcon.appiconset` sources,
  `ui/fort-icon.png`, `ui/fort-agent-orb.png`, and the native
  `FortAgentOrb.imageset` — generated Jarvis-like raster identity.
- `design-qa.md` — visual comparison evidence and result.

## Test criteria

- The web source contract pins the three-pane shell, raster identity, active
  conversation, explicit profile/machine controls, inline gate actions,
  readable state badges, event-derived activity, newest-first sorting, no
  Projects presentation, and a single creation CTA.
- Handoff interaction tests prove immediate visible busy state, an accessible
  live status message, duplicate-click suppression, inline transport failure,
  and control restoration after the request settles.
- Orb-motion tests prove the existing raster identity retains ambient motion,
  evidence-backed Working state is more energetic, and reduced-motion clients
  preserve only the slow non-spatial energy pulse.
- Conversation-promotion tests prove only a completed direct conversation can
  show **Turn this into work**, that promotion pins the default non-answer
  playbook revision, and that its resulting playbook assignment cannot enter the
  promotion flow again.
- Transcript-role tests prove a human row has a person avatar and no Fort raster
  or thinking class, while an agent row retains the truthful Fort identity.
- Catalog and UI tests prove Sol, Terra, and Luna lower to their exact provider
  IDs, readiness is derived from the approved executable's live catalog, and
  playbook controls consume `/api/profiles` rather than a duplicate model table.
- API and engine tests prove an exact catalog profile survives
  request -> task -> persisted run -> `runtime.RunSpec`, while unknown or
  mismatched profiles dispatch nothing.
- Classifier and catalog tests prove `Reply OK` and `Reply with exactly OK`
  resolve to the quick-answer route, while ordinary feature work remains feature
  work and explicit route selection retains precedence.
- Graph tests prove intermediate provider messages remain in the event log while
  only the terminal message becomes the task output and quick-answer result.
- Conversation tests prove states do not reorder history, event time advances a
  conversation, gates override work activity, terminal state wins, and no run
  is reported as actively working without a real event.
- Phone source-contract tests prove conversation history opens and closes from
  the active thread, the selected thread state stays visible in the header,
  the transcript/composer fill one viewport without document-width overflow,
  and model, machine, send, assignment, and sign-off controls retain 44px touch
  targets.
- Existing page JavaScript, markdown safety, playbook, gate, and API tests stay
  green.
- `go test ./...` passes; concurrency behavior is unchanged.
- FortKit contract checks cover profile-option decoding/requests, exact profile
  encoding, newest-first ordering, activity-state resolution, and Swift source
  parsing.
- iPhone source-contract checks pin the raster Fort orb, a distinct human avatar,
  direct-conversation Agent/Model/Machine controls, real event consumption,
  inline **Approve & continue** and **Request changes**, single-flight send and
  assignment actions, one-way promotion, and the absence of Projects and Give
  direction presentation.
- The iOS app builds for the iPhone simulator, and the release archive uploads
  under a new build number that App Store Connect accepts for TestFlight
  processing.
- Web is rendered at the reference's 1280 x 904 viewport, primary controls are
  exercised (including the sign-off state), console errors are checked, and the
  result is compared with the source in one combined image. The installed native
  Mac app is captured in the same sign-off state.

## Rollback

Revert this visual layer, classifier extension, profile-option contract, and
additive `run.profile` / `run.model` columns. Existing databases remain readable
because the migration is additive and legacy rows decode empty profile/model
values. App-icon rollback is a normal Git restore of the tracked raster slots.
