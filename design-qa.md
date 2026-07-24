# Fort Dashboard Redesign - Design QA

## Scope and status

- Web control plane: passed visible and functional QA.
- Shared FortKit contracts and native source integration: passed contract and syntax checks.
- Native iPhone/macOS visual fidelity: blocked because this host has Command Line Tools only, without full Xcode or iOS Simulator.

## Source and implementation evidence

- Source: `design_handoff_fort_dashboard_redesign/Fort Redesign.dc.html`, Turn 4 (`4a` Playbooks and `4b` route preview).
- Implementation: `http://127.0.0.1:4088/`, dark theme, seeded default catalog.
- Matched desktop viewport: 1280 x 720.
- Playbooks same-frame comparison: `/private/tmp/fort-dashboard-preview-019f87bb/playbooks-side-by-side-final-v4.png` (source left, implementation right).
- Route-preview same-frame comparison: `/private/tmp/fort-dashboard-preview-019f87bb/route-side-by-side-final.png` (source left, implementation right).
- Phone-width web evidence: `/private/tmp/fort-dashboard-preview-019f87bb/playbooks-phone-web-final-v3.png` at 390 x 844.

## Web visual findings

- The Playbooks rail, selected/default treatment, stage cards, agent/model controls, memory badges, plan-gate control, shortcuts, typography, borders, radii, spacing, and brass/blue state colors match the handoff's visual grammar.
- Feature-work branches now appear in the source order: features, then bug fixes. Shortcut rows likewise lead with Quick answer, then Bug fix.
- The route card preserves the handoff hierarchy: resolved playbook, agent/model chain, memory state, plan-gate consequence, and one-step route switching.
- At 390 px the playbook rail becomes a two-column selector and stages stack vertically; the header remains horizontally navigable without an exposed scrollbar.
- No open P0, P1, or P2 web visual defect remained after the final comparison.

## Functional browser checks

1. Edited and restored the default playbook plan gate; each save appended an immutable revision.
2. Previewed deterministic automatic routing, switched to an explicit Bug-fix route, and switched back.
3. Toggled the per-assignment plan gate and confirmed the preview changed before execution.
4. Handed off a Feature-work assignment, observed the plan sign-off in Needs you, approved it, and confirmed execution resumed and completed.
5. Asked a Quick question, received an inline answer, and confirmed no Quick-answer run appeared on the board.
6. Verified Quick mode hides and forces off the plan-gate control.

## Intentional spec-honest differences

- Conditional prose such as "skip Design when no UI work" is not guessed by a model; it remains deferred until Fort has an approved deterministic predicate grammar.
- Method-version promotion, recurring schedules, durable project grouping, and agent+model historical scorecards remain explicitly deferred in `specs/036-playbooks.md`.
- The application retains Fort's canonical global navigation around the Playbooks surface instead of reproducing the handoff's isolated mock-frame header.

## Native verification boundary

- `FortKitContractChecks` passed, including playbook catalog, route-preview, Quick-answer, and request-encoding contracts.
- Swift source parsing and XcodeGen project generation passed during the native implementation audit.
- A rendered iPhone/macOS comparison cannot be captured on this host. Full native visual approval requires Xcode plus Simulator screenshots at iPhone 390 x 844 and macOS 1200 x 800.

## Final result

blocked
