# 066: Google sign-in in front of the cloud hub

**Decision:** Toby asked for it on 2026-10-25 ("I want everything to sit behind a google login", limited to himself).

## What and why

The hub at its public Railway address answered anyone. Data was safe behind the owner's device
credential, but that credential lived in the browser's storage, and the claim screen and app were
public. Now, with `FORT_GOOGLE_CLIENT_ID` and `FORT_OWNER_EMAIL` set, a browser gets only a sign-in
page until Toby signs in with his Google account. The app, the claim, and every call made with the
owner's browser credential need that Google session too.

## Risk

Credentials: the hub keeps a random cookie-signing key (`web-session.key`, 0600) and the pinned
Google account ID (`google-owner.json`) on its volume. It stores no Google secret or token: the
client ID is public and the ID token is checked once, then dropped. Sessions last 7 days, are
HttpOnly, Secure, SameSite=Strict, and end on sign-out.

What could go wrong: a wrong email or client ID locks Toby out of the web client. Undo by fixing or
unsetting the two variables on Railway; nothing else changes. Deleting `google-owner.json` unpins
the Google account.

Paired machines are unchanged: the Mac runner (runner token, one-time invitation) and the iPhone
(companion device key) never see the Google prompt. Both are created only from a Google-signed-in
browser and can be revoked there.

## Plan

Google Identity Services in the browser posts an ID token to `POST /auth/google`. The hub checks
Google's RS256 signature against Google's published keys, the issuer, the audience (client ID),
expiry, a one-time nonce it issued, a verified email on the allow list, and, after the first sign-in,
the pinned Google account ID. Then it sets a signed session cookie. Only `/`, `/signin.js`, styles,
icon and manifest load without it.

Rejected: Cloudflare Access in front (needs a custom domain and a service token for the Mac);
server-side OAuth code flow (needs a client secret on the hub for no added protection here).

Tests (`host/cloud-hub-google.test.ts`): only the sign-in page loads before sign-in; the owner signs
in, claims and uses the hub; a copied credential or tampered cookie gets nothing; other accounts,
unverified emails, foreign audiences or issuers, expired, forged, replayed and foreign-origin
sign-ins are refused; the pinned account wins over the same email; the Mac and iPhone paths keep
working without Google.
