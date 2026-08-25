# Fort platform adapter for Hermes

This directory is the Hermes-side connector for Spec 053. Fort remains the
messaging platform; this package is installed inside Hermes only because a
`BasePlatformAdapter` plugin is Hermes' supported third-party platform seam.

It is currently pinned to Hermes Agent `0.20.5` at
`981101239a064c020a9d18fc3b1060ae306934ed`. The Hermes host must expose the
additive public `PluginContext.profile_identity` mapping. Hermes owns the
precedence `Bot Mode title -> profile display name -> canonical profile ID`.
The adapter intentionally fails closed if that accessor is unavailable and
never reads `ui_meta` itself.

## Configuration contract

Install this directory as an enabled Hermes `kind: platform` plugin for each
profile that should appear in Fort. Set these values in that profile's Hermes
secret scope:

```text
FORT_PLATFORM_URL=ws://127.0.0.1:4087/platforms/hermes
FORT_PLATFORM_TOKEN=<derived credential for this exact canonical profile ID>
FORT_ALLOWED_USERS=human:toby
FORT_ALLOW_ALL_USERS=false
```

Fort provisions a distinct derived credential for each canonical Hermes
profile. The machine-scoped root key never leaves Fort. The derivation is
base64url-without-padding of:

```text
HMAC-SHA256(
  profile_token_key,
  "fort-hermes-profile-token:v1\n" +
  decimal_utf8_byte_length(canonical_profile_id) + ":" +
  canonical_profile_id
)
```

The adapter sends the canonical profile ID in
`X-Fort-Hermes-Profile`; Fort validates the derived bearer before accepting
the WebSocket upgrade and then requires the registration frame to repeat that
exact profile. A profile credential cannot register another profile.

Hermes owns and enforces the allowed-user policy. Fort never reads or mirrors
it. Do not set `FORT_ALLOW_ALL_USERS=true` merely to make a test pass.

The matching owner-only Fort data-directory file is:

```json
{
  "profile_token_key": "<owner-only machine-scoped root key>",
  "human_id": "human:toby",
  "human_name": "Toby"
}
```

It is named `hermes-platform.json`. Fort fails startup if this and the
completed Spec 052 `hermes-messaging.json` proof configuration are both
active; migration must be explicit. The same migration must back up and remove
the old proof's `GATEWAY_RELAY_*` values from Hermes' secret scope before the
gateway restarts. Leaving `GATEWAY_RELAY_URL` set makes Hermes prefer its relay
connector and disable this directly connected platform adapter by default.
Do not enable `GATEWAY_RELAY_ALLOW_DIRECT_PLATFORMS` as a coexistence shortcut;
Spec 053 has no relay fallback. Fort derives the Messaging Source identity from
the stable `machine_id` in its existing `relay.yaml`; neither the adapter nor
this file may choose a source or machine identity.

## Verification boundary

Run the adapter contract tests against the pinned Hermes checkout:

```sh
PYTHONDONTWRITEBYTECODE=1 \
PYTHONPATH=/Users/tobiasgunn/.hermes/hermes-agent \
/Users/tobiasgunn/.hermes/hermes-agent/venv/bin/python -m unittest discover \
  -s integrations/hermes/fort-platform/tests -p 'test_*.py' -v
```

These tests prove the bounded wire contract only. They do not prove that the
plugin is installed, enabled, connected, authorized for a recipient, backed by
a durable transcript, or usable from a processed TestFlight build.
