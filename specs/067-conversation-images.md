# 067: Agents draw images in chat

**Decision:** Toby asked on 2026-10-25 ("add image generation through Codex or an API call as a fast follow",
Fort project thread). This covers the Codex route below. The API-key route is not built (see Plan).
A real test image on his Mac still needs his OK, as each one uses ChatGPT plan usage.
Toby decided on 2026-10-28 to run image generation through the ChatGPT subscription to start, with an API route possibly later. Renumbered from 066 because Spec 066 is Google sign-in.

## What and why

Spec 065 lets Fort draw agent portraits on Toby's ChatGPT plan. Toby wants agents to make images
generally. After this change, typing `/image <description>` in a conversation with a ChatGPT
Agent and pressing Send asks that Agent for one image. The message shows "Drawing on your
ChatGPT plan…" and the Agent's reply shows the image, in the Mac and iPhone apps. A failed draw
is the Agent's reply, saying why ("Your ChatGPT plan image limit was reached", for example).

## Risk

- **Billing/provider path.** Same as Spec 065: Codex 0.163.0's built-in image generation on the
  signed-in ChatGPT account, on the Agent's own model and selected account. No API key, so no API
  billing. Each image is one Send by Toby; Fort never retries or draws on its own.
- **Controls.** No new process type and no new controls. The image reuses the portrait run:
  its own Codex process, image generation on, every other disabled feature still off, empty
  workspace, no shell, no MCP, no web search, ephemeral thread with no history, skills or memory.
  Only the prompt differs (the owner's words instead of the portrait template), with its own
  developer instructions. Chat runs keep image generation off, unchanged.
- **Credentials.** No change.
- **Data.** Each image is one PNG (same bounds as portraits: 256 to 2048 px a side, at most 2:1,
  5 MB) in the ledger as an `images` row, plus two messages in the conversation. Deleting history
  removes images with the messages. This also fixes history deletion refusing to run once a
  portrait or week existed (their kinds were missing from its lists).
- **Chat while drawing.** One draw at a time, shared with portraits. Chat messages queue until it
  finishes. A restart mid-draw posts a reply saying the image didn't finish.
- **Undo.** Revert the PR. Older apps ignore the new message fields and show the text.

## Plan

- **Wire**: the portrait executor operation carries either a portrait `subject` or an image
  `prompt` (at most 2000 bytes), never both.
- **Core**: `POST /v1/conversations/{id}/images` with `{ requestId, agentId, prompt }` returns 202
  and the user message (`drawsImage: true`). The reply message carries `image: { imageId, width,
  height }`; `GET /v1/images/{imageId}` returns the PNG. Capability `conversationImages`.
- **Apps**: `/image ` in the composer routes to the draw, with a one-line plan-usage note.
- **Rejected**: turning image generation on for chat turns, so an Agent could decide to draw by
  itself. That widens every chat turn and spends plan usage without a press; it can be a later
  spec. An OpenAI API-key route (`gpt-image` over the Images API): it is API billing, which
  spec 058 §9 forbids unless Toby approves a separate spec. Claude, which can't make images.
- **Tests**: runner fixtures for an image prompt, refusals and bad prompts; wire round trip and
  rejection of subject+prompt; core tests for draw, fetch, failure reply, one-at-a-time with
  portraits, restart, and history deletion; Swift client and decoding tests. One real image on
  Toby's Mac, after his OK.
