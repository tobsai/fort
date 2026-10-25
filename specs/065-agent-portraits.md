# 065: Fort draws its own agent portraits

**Decision:** pending. Asked Toby on 2026-10-25 to OK turning on Codex image generation for a
separate portrait run only, and spending one real test portrait.

## What and why

Toby makes each agent's portrait by hand in Gemini today, and the app only offers three stock
icons (orbit, ember, sprout). He asked for Fort to "build its own agent images". After this
change, the agent editor has **Draw portrait**: Fort writes a prompt from the agent's name, its
instructions and an optional note from Toby, in one fixed house style (the comic look of the
existing Milton, Pascal and Lewis portraits). It generates one square image on his ChatGPT plan
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
- **Data.** Portraits are PNGs stored by the core next to the ledger (at most 2 MB, re-encoded
  and checked as a single square PNG). Removing a portrait deletes only that file.
- **Undo.** Remove the portrait, or revert the PR. Stock icons keep working throughout.

## Plan

- **Runner**: `PortraitTurn` drives one Codex turn and accepts exactly one `imageGeneration`
  item. It takes the base64 `result` (not `savedPath`, which is outside the sandbox), with its
  own frame limit sized for one image. Any other item type (commands, file edits, tool calls)
  ends the turn as refused. `portraitControls` verifies the portrait profile described above.
- **Core**: `POST /v1/agents/{id}/portrait` with `{ note? }` starts a draw on the agent's own
  Codex binding and account. `GET` returns the PNG, and `DELETE` removes it. `Agent` gains
  `portrait?: { revision, createdAt }`. One draw at a time per host.
- **Apps**: `AgentAvatarView` shows the portrait when there is one. The agent editor gets
  Draw, Redraw and Remove, with a note that a draw uses ChatGPT plan usage.
- **Rejected**: the "Sign in with ChatGPT" Responses route, because its preview has no hosted
  tools. Turning image generation on for chat agents, because that widens every chat turn.
  Claude, because it can't make images.
- **Tests**: fixture Codex streams for a good image, a non-image item, a usage-limit failure,
  oversize and non-PNG data, and a control check that fails if any other feature is on. Core
  tests for draw, fetch, remove and binding/account pinning. One real draw on Toby's Mac, after
  his OK.
