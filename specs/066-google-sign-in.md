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

The Mac runner is unchanged (runner token, one-time invitation created from a Google-signed-in
browser). The iPhone signs in with Google too (Toby chose this on 2026-10-25): with
`FORT_GOOGLE_IOS_CLIENT_ID` set, every iPhone call needs a Google session on top of its device key.
The app signs in through Google's iOS client (no secret, PKCE), posts the ID token to
`POST /auth/google/native`, keeps the returned session value in its Keychain and sends it as
`X-Fort-Session`. That endpoint refuses browsers, so the web page never sees a session value.
Until the iOS variable is set, iPhones keep working on their device key alone.

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

## Revision 2026-11-02: more than one browser

Before: the hub had one browser key, made by the one-time setup code. Any other browser signed in
with Google still hit the setup code screen and could not get in.

Now any browser that signs in with Google as the owner asks `POST /v1/hub/browsers` for its own key.
The hub mints a fresh 32-byte key, keeps only its SHA-256, and lists the browser by name in Setup.
Nothing is copied from the owner browser, and the owner key is untouched.

Risk and limits:
- A key comes only from the hub's own page (its origin and its session cookie), from a Google
  sign-in made in the last ten minutes, and only once the hub is claimed. A hub without the Google
  gate adds no browsers; its setup code stays the only way in.
- An added browser's key, like the owner's, does nothing without the Google session.
- Removing a browser ends its key at once and refuses any key request from a sign-in made before the
  removal, so getting back in takes a new Google sign-in. At most ten added browsers.
- Added browsers chat, approve and manage Macs. Provider account sign-in, iPhone pairing and host
  stop or restart stay with the owner key. A fresh owner setup or disconnect drops every added browser.

Tests (`host/cloud-hub-browsers.test.ts`): a second and third browser get their own keys while the
owner keeps working; the owner-only routes refuse them; no key without a fresh sign-in, the page's
origin or the gate, or before the hub is claimed; a copied key alone gets nothing; removal ends one
key, leaves the rest, and needs a new sign-in to come back; the cap holds.
