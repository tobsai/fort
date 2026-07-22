# Gateway Command Deck design QA

**Findings**

- No actionable P0, P1, or P2 visual findings remain.
- [P3] Project sigils are intentionally absent from the remote project list.
  Location: `CommandDeckSurface` project rows.
  Evidence: handoff view 1a uses deterministic project sigils; the gateway uses the approved title, checkpoint copy, and semantic state rail without substituting a glyph, CSS drawing, handcrafted SVG, or placeholder asset.
  Impact: project identity is slightly less distinctive, but status, hierarchy, and task completion remain unambiguous.
  Fix: package the existing deterministic Fort sigil renderer as a shared, approved image-asset pipeline before adding sigils here.

**Open Questions**

- None blocking. The separate `FORT REMOTE` shell bar is an intentional gateway context layer above the matched Command Deck surface.

## Evidence

- Source visual truth: `/Users/tobiasgunn/dev/fort/design_handoff_fort_dashboard_redesign/Fort Redesign.dc.html#1a`
- Source capture: `/tmp/fort-design-reference.png`
- Implementation capture: `/tmp/fort-gateway-desktop-final.png`
- Combined comparison: `/tmp/fort-command-deck-comparison-final.png`
- Phone captures: `/tmp/fort-gateway-mobile-final.png` and `/tmp/fort-gateway-mobile-bottom.png`
- Desktop CSS viewport: `1280 x 800`; source pixels: `1265 x 791`; implementation pixels: `1265 x 712`; device density: `1x`.
- Comparison normalization: the source Command Deck region (`1109 x 493`) was resampled to `1203 x 534`; the implementation region was cropped at `1203 x 630`. No `@2x` downsampling was required. The combined image is `1243 x 1277`.
- Phone CSS viewport: `390 x 844`; captured top pixels: `375 x 812`; captured lower region: `390 x 844`; measured DOM width `375 == 375` with no horizontal overflow.
- State: dark theme, authenticated-deck equivalent, connected machine, two waiting checkpoints, four projects, four roster agents, and two queued items. Synthetic data was used only in a temporary local visual-QA route; that route and its auth exemption were removed before the production build.

## Required fidelity surfaces

- Fonts and typography: Instrument Sans for interface text and Spline Sans Mono for machine/data text match the handoff hierarchy, weights, tracking, and compact labels. Wrapping and truncation were checked at desktop and phone widths.
- Spacing and layout rhythm: the 1.5:1 inbox-first split, 22 px desktop panes, compact card rhythm, amber attention rail, outer frame, and responsive single-column stack match view 1a. Crew is above Up next so the reference's primary above-the-fold content remains visible.
- Colors and visual tokens: canvas, panel, border, brass, needs-you, working, accepted, failed, and muted text colors use the normative handoff tokens. No gradients were introduced.
- Image and asset fidelity: no raster assets are required. The unavailable deterministic sigils were omitted rather than replaced with fake art; this is classified as the P3 follow-up above.
- Copy and product language: `Needs you`, `Projects`, `Crew`, `Up next`, checkpoint actions, accepted-checkpoint progress, and `Give direction` follow the handoff vocabulary. Agent-estimated percentages are not shown.

## Full-view and focused comparison

- Full-view evidence: `/tmp/fort-command-deck-comparison-final.png` places handoff 1a and the desktop gateway in one normalized comparison input. Column proportions, attention hierarchy, card density, semantic color, and visible Crew content were judged together.
- Focused evidence: the phone top and lower captures verify the toolbar, both decision cards, project list, Crew roster, and Up next rows without horizontal overflow. A separate desktop crop was unnecessary because the normalized combined image already isolates the complete Command Deck frame.

## Comparison history

1. Earlier P2: Up next appeared before Crew, pushing the reference's Crew region below the fold. Fix: reordered the overview to Projects, Crew, then Up next. Post-fix evidence: `/tmp/fort-command-deck-comparison-final.png`.
2. Earlier P2: the attention pane lacked the reference's calm all-clear sentence after waiting cards. Fix: added the accepted-checkpoint-based crew summary. Post-fix evidence: `/tmp/fort-command-deck-comparison-final.png`.
3. Earlier P2: the direction composer dispatched immediately without showing or confirming its playbook route. Fix: added sealed route preview, visible agent/model stages, explicit playbook switching, immutable route pinning, and a second confirmation action. Verified by source-contract and presentation tests in `test/command-deck.test.ts`.
4. Earlier P2: narrow layouts needed confirmation below the first viewport. Fix: stacked the panes and responsive rows; measured `scrollWidth == clientWidth` and captured both top and lower phone regions.

## Interaction and runtime checks

- Primary interactions covered: Deck/Snapshot/Activity tabs, manual refresh, checkpoint accept/redirect, backlog start, two-step Give direction route confirmation, playbook switching, answer-vs-assignment plan-gate handling, error dismissal, and Activity start/stop states.
- Automated checks: `28/28` gateway tests pass, including identity-bound pinning, load coalescing/forced refresh, operation leases, stale direction-preview rejection, fresh playbook-catalog loading, roster truthfulness, route pinning, and relay `bye` cleanup.
- Browser console checked. Successful built-preview captures had no component render exception; recorded errors were limited to expected dev hot-refresh/server-teardown fetch failures after temporary QA routes or preview servers were stopped.

**Implementation Checklist**

- [x] Match the approved desktop attention-first composition.
- [x] Preserve Fort tokens, typography, vocabulary, and accepted-checkpoint progress.
- [x] Keep Crew visible above the fold and Up next available below it.
- [x] Verify phone stacking and horizontal overflow.
- [x] Require route preview and confirmation before handoff.
- [x] Remove the unauthenticated local QA route before the final build.

**Follow-up Polish**

- Share the deterministic sigil renderer as an approved asset pipeline, then add real project marks without code-native approximations.

final result: passed
