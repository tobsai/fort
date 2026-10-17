# Requirements and architecture review

Read [the constitution](../ARCHITECTURE.md) before writing a new requirement or
materially changing an existing one. Use [TEMPLATE.md](TEMPLATE.md). Put proposed
requirements in a Markdown spec under `specs/`; design notes elsewhere must link
to a reviewed spec before implementation. Historical approval is not a substitute.

## Workflow

1. Write the goal, approach, exact affected files, tests and rollback. Complete all
   four architecture sections, including boundaries, dependency/removal impact,
   acceptance test/measurement, evidence, tradeoffs and unresolved judgment.
2. Run `node ops/architecture/validate-specs.mjs` from the repository root, or
   `npm --prefix successor run check:specs`. This is a **structural** check only.
3. Present the proposal, evidence and conflicts to Toby. Record the real decision
   reference, owner, scope and tradeoffs. Existing authority may be reused only
   for unchanged scope. Do not turn a passing validator into invented approval.
4. Once the design is actually accepted, set `Review status: accepted`, resolve
   every `Unresolved judgment` to `none`, and run
   `node ops/architecture/validate-specs.mjs --digest specs/<NNN>-<slug>.md`.
   Put that digest in `Reviewed content SHA256`; the digest binds all file content
   except its own value. It detects drift, not forged decisions.
5. Before implementation run both the full structural check and
   `node ops/architecture/validate-specs.mjs --implementation specs/<NNN>-<slug>.md`.
   Use a red → green public behavior slice and collect the promised evidence.
   Updating an accepted spec invalidates its digest: return to reassessment and
   obtain an applicable decision before accepting changed content.
6. Before committing a product slice, stage its exact files and run
   `node ops/architecture/validate-specs.mjs --implementation specs/<NNN>-<slug>.md --staged`.
   Every staged path must be declared in that spec's `Implementation paths`
   section. The selected spec itself is implicitly covered. Use one applicable
   spec per bounded slice; split unrelated work rather than claim an arbitrary
   accepted spec applies. The declaration is covered by the reviewed digest.

## Status vocabulary

Review status is one of `draft`, `needs-reassessment`, `ready-for-review`,
`accepted`, `rejected`. Only `accepted` can pass the implementation gate; it
requires a decision reference and matching digest. Blocked principles and unresolved
judgments prevent acceptance. Only actual owner acceptance grants authority for
its bounded plan. The `accepted` string declares that decision; it does not
authenticate it, verify implementation or authorize release.

Each principle has a separate progress status:

| Status | Meaning |
| --- | --- |
| `planned` | Acceptance test/measurement is defined but work has not been verified. |
| `implemented-unverified` | Code exists; the required proof is missing. |
| `verified` | Cite the exact artifact, command, result and limits supporting this claim. |
| `blocked` | Name the failed requirement or decision preventing progression. |
| `not-affected` | Explain why boundaries/dependencies and acceptance remain unchanged. |

All fields are mandatory even when unaffected. Bare `N/A`, `TBD`, `TODO` or a
placeholder is insufficient evidence. Drafts may state a specific unknown and
how to resolve it. Speed reviews require numeric budgets, measurement conditions
and separated provider latency before design acceptance; the validator does not
judge whether prose or budgets meet these substantive requirements.

## Changed-path declaration

The template's `Implementation paths` section contains only exact file paths,
one Markdown bullet/code span per line (for example, `-` followed by a code span
containing `successor/contracts/v1.ts`). No directories, globs or traversal are
accepted. Include tests, dependency/build settings and supporting documents in
the same slice. Deletion needs the old path; a rename needs both old and new paths.
Changing this list changes the reviewed spec digest and requires reassessment.

`--staged` compares Git's index to HEAD by default, and requires the selected spec
on disk to match its index content. It excludes unstaged and untracked files;
stage every intended change before claiming coverage. An empty comparison proves
nothing about previous commits. Do not stage unrelated work to satisfy this check.
For a committed branch/CI range, add `--base <full-40-character-base-commit-SHA>`
with a clean index at the reviewed head. The base must come from trusted review
context, not a convenient commit chosen to hide changes.

This is exact-path coverage, not semantic applicability or authenticated approval.
A reviewer must check that the declared spec actually governs the change, its
real owner decision covers the scope, and promised behavior/evidence is adequate.
There is no automatic mapping for multiple specs or historical code, no approval
service and no remote CI enforcement. See the [proposed CI procedure](../docs/architecture/reassessment-20261017.md#proposed-ci-enforcement-of-change-scope).

## Preservation and enforcement limits

[historical-specs.json](../ops/architecture/historical-specs.json) hashes the
unchanged Markdown at checkpoint `e02eb571e66620e407a360584817616074730b55`,
except Spec 058. Those files pass **preservation only** and cannot pass the
implementation gate. Changes require the new review; missing historical files
fail. Do not grow this baseline to grandfather new requirements. Any baseline
change needs explicit review against preserved Git history. Archived decisions
remain historical evidence; create new decisions rather than rewriting them.

All new/changed Markdown under `specs/` is discovered recursively, including new
decision documents. Only this README and the root template are excluded. Specs
054–057 retained in the primary checkout are not copied into this worktree or
claimed reviewed. Files outside this worktree are outside this local check.

The successor npm `test` and `typecheck` lifecycles run the structural check
first. Direct tool invocations can bypass npm; agents must still run the required
checks. The gate is local; no GitHub workflow or branch protection has been configured.
It validates structure, status and accepted-content integrity. It cannot verify
human approval, evidence truth, actual removability, performance or the completeness
of requirements. Required behavioral/benchmark/removal tests and human review
provide that proof. Test the gate with `npm --prefix successor run test:specs`.
