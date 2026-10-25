# 065: Fort draws its own agent portraits

**Decision:** Toby approved on 2026-10-25 ("OK on PR 47", Fort project thread), covering Codex image
generation for the separate portrait run only and one real test portrait on his Mac.

## What and why

Toby makes each agent's portrait by hand in Gemini today, and the app only offers three stock
icons (orbit, ember, sprout). He asked for Fort to "build its own agent images". After this
change, the agent editor has **Draw portrait**: Fort writes a prompt from the agent's name, its
instructions and an optional note from Toby, in one fixed house style (the comic look of the
existing Milton, Pascal and Lewis portraits). It generates one image (asked for square) on his ChatGPT plan
and saves it as that agent's portrait. The editor, chat and city then show the portrait in place
of the stock icon. Redraw replaces it, and Remove goes back to the stock icon.

Claude has no image generation, so this is ChatGPT-only.

## Risk

- **Billing/provider path.** This uses Codex 0.163.0's built-in image generation on the
  signed-in ChatGPT account, which draws on plan usage. No API key, so there is no API billing.
  A third-party write-up says an image turn uses 3 to 5 times the plan usage of a text turn
  (not confirmed by OpenAI). Fort never retries a draw by itself, and each draw is one press by
  Toby. A `usageLimitExceeded` failure from Codex is shown as a plan limit, and nothing else is
  tried.
- **Weaker controls, kept narrow.** Chat runs keep `image_generation` off, unchanged. The portrait
  run is its own Codex app-server process with a separate control check that requires
  `image_generation` on and every other disabled feature still off, an empty workspace, no shell,
  no MCP and web search disabled. It gets no conversation history, skills or memory, only the
  built prompt. If the check fails, nothing is sent.
- **Credentials.** No change. The portrait run uses the same Codex sign-in and the same pinned
  account as the agent's binding. Claude-bound agents don't get the button.
- **Data.** Each portrait is one PNG (each side 256 to 2048 px, no more than 2:1, at most 5 MB, so it fits the Mac app's 8 MiB host channel), stored in the ledger
  as a `portraits` row keyed by agent. Removing a portrait deletes only that row. Codex also keeps
  its own copy under the private runtime's `generated_images/`; Fort leaves that folder alone.
- **Chat while drawing.** A draw holds the runtime for up to about two minutes. Messages sent
  meanwhile queue and run when it finishes. A restart mid-draw marks it failed, and nothing is saved.
- **Undo.** Remove the portrait, or revert the PR. Stock icons keep working throughout.

## Plan

- **Runner**: `PortraitTurn` drives one Codex turn and accepts exactly one `imageGeneration`
  item. It takes the base64 `result` (not `savedPath`, which is outside the sandbox), with its
  own frame limit sized for one image. Any other item type (commands, file edits, tool calls)
  ends the turn as refused. `portraitControls` verifies the portrait profile described above.
- **Executor**: a `portrait` operation on the existing worker wire. The worker launches the same
  private Codex directories with the portrait profile and an 8 MiB frame limit.
- **Core**: `POST /v1/agents/{id}/portrait` with `{ note? }` returns 202 and draws in the
  background, on the agent's own runtime, model and selected account. `GET` returns
  `{ revision, png }` (base64), and `POST …/portrait/remove` removes it. `Agent` gains `portrait?: { revision,
  createdAt, size }` and `portraitDraw?: { state: drawing | failed, detail }`. One draw at a time.
- **Apps**: `AgentAvatarView` shows the portrait when there is one. The agent editor gets
  Draw, Redraw and Remove, with a note that a draw uses ChatGPT plan usage.
- **Rejected**: the "Sign in with ChatGPT" Responses route, because its preview has no hosted
  tools. Turning image generation on for chat agents, because that widens every chat turn.
  Claude, because it can't make images.
- **Tests**: fixture Codex streams for a good image, a non-image item, a usage-limit failure,
  oversize and non-PNG data, and a control check that fails if any other feature is on. Core
  tests for draw, fetch, remove and binding/account pinning. One real draw on Toby's Mac, after
  his OK.
