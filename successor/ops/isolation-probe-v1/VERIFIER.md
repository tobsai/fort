# Proposed private OpenPGP verifier

**OpenPGP.js 6.3.2, using existing Node 26.5.0. Proposed only; no package has been
downloaded, installed or executed.** This resolves the earlier unspecified verifier
choice. It does not waive publisher signatures or authorize execution.

| Item | Exact proposed bound |
| --- | --- |
| Package / archive | `openpgp@6.3.2`, `https://registry.npmjs.org/openpgp/-/openpgp-6.3.2.tgz` |
| Integrity | `sha512-wcZTzHz41LV8Y48zH/JlD1JT8YdNmpWOuMAjQ/podvvCwsjBEU37K3znJh4UQcj3hksE5z9TzvhbUbwzsGvlLQ==` |
| Bounds | 8 MiB compressed; 24 MiB complete extraction; registry reports 17,421,480 unpacked bytes |
| Installation | `/Users/tobiasgunn/Library/Application Support/FortIsolation/tools/openpgp/6.3.2` |
| Node | `/opt/homebrew/opt/node@26/bin/node`, observed `v26.5.0`; no new runtime |
| Declared ESM entrypoint | `package/dist/node/openpgp.mjs`; actual archive contents still need inspection |
| Dependencies/hooks | No runtime/optional dependencies in metadata. `prepare`, build and other hooks exist but none may run. No npm/brew/pip install, build, development dependencies or alternate bundle |
| License | Metadata says `LGPL-3.0+`; preserve all package legal/attribution files in the temporary private installation. Excluded from Fort's app/distribution dependency graph |
| Inputs | Public signing keys, original detached signatures/checksum bytes and exact payload hashes; no private key, owner keyring, sign-in, signing, decryption or key generation |
| Network | Only added host `registry.npmjs.org` for the three fixed URLs below. Offline verification grants Node no network permission |
| Runtime restrictions | Explicit environment, read-only allowed packet/download/tool paths, no write/network/child/worker/addon permission, 256 MiB JS heap limit, 90-second verifier deadline and 120-second parent process timeout |
| Budget / removal | 32 MiB inside the existing 4 GiB auxiliary allowance; margin becomes 288 MiB. Remove only owned archive/extraction after evidence retention |

## Public registry metadata and trust

The additional metadata URLs are `https://registry.npmjs.org/openpgp/6.3.2` and
`https://registry.npmjs.org/-/npm/v1/keys`. Their exact original-byte SHA256 values,
the archive's decoded SHA512, byte caps and exact filenames are in `scope.json`.
The downloader now has twelve entries. Read-only re-fetches matched the review's
metadata; both ECDSA signatures verify using existing Node built-in crypto against
the pinned key `SHA256:DhQ8wR5APBvFHLF/+Tc+AYvPOdTpcIDqOhxsBHRwC7U`.

Registry trust begins at the official npm HTTPS metadata/key endpoint and its
pinned bytes. It authenticates `openpgp@6.3.2:<integrity>` under that key. It is not
an independent maintainer OpenPGP signature and does not yet validate the absent
tarball. Neither the registry metadata nor tests claim publisher signatures passed.
The wrapper contains no custom OpenPGP implementation.

## Staged operations after owner approval

1. Download only the twelve pinned entries with `fetch-public.py`. Verify each
   SHA256 or SHA512 and bound. Run `python3 -B verify-offline.py --registry-only`.
   This uses Node built-ins only: hashes all downloaded files and verifies the two
   registry signatures/fingerprint. It exits before any package import or need for
   an extracted library. Reject changed, missing, expired or ambiguous registry keys.
2. Inspect the verifier tar table and contents with existing tools. Require only
   safe relative paths under `package/`, no links escaping that root, special files,
   setuid/setgid files or unlisted extraction. Bound aggregate extracted size to
   24 MiB and compressed archive to 8 MiB. Inspect `package.json`, legal files and
   the bundled ESM import graph; stop on any unexpected dependency, native addon,
   external module or network fetch. Do not run hooks. Extract only to the fresh
   proposed private directory, preserve its legal files, and record/verify the
   resulting file hashes before import. No payload from this package is copied
   into the Fort application.
3. Run `python3 -B verify-offline.py`. It repeats input integrity and registry
   checks, confirms package identity/dependencies/entrypoint, then imports the
   selected ESM bundle under existing Node's permission controls. There is no
   write/network/child/worker/addon grant. These controls are additional bounds for
   this trusted verification tool, not proof of the product's hostile-runner sandbox.
4. Parse exactly one public primary key; require the full publisher fingerprint.
   Validate its revocation/self-signature/expiration status at current time. Verify
   the original checksum bytes against the detached signature with unchanged
   OpenPGP.js defaults, await **every** signature's `verified` result, and validate
   each signing key/subkey at current time. Missing, invalid, unresolved or expired/
   revoked results fail. Do not disable policy, backdate verification, or accept one
   valid signature while ignoring an unresolved additional signature.
5. Require exactly one signed entry for each selected filename, matching both the
   packet pin and the separately computed payload SHA256. The wrapper then repeats
   verification with changed checksum bytes, damaged signature bytes and the other
   publisher's unrelated public key; all three must fail for each publisher.
   These are future checks using existing public inputs, with no key generation.
6. Only after both publisher signatures and negative controls pass may the runbook
   continue to inspect/run the Lima manager. Any rejection stops; library policy
   compatibility with the actual publisher signatures remains untested.

Lima primary fingerprint: `C020EA876CE4E06C7AB95AEF49524C6F9F638F1A`.
Ubuntu primary fingerprint: `D2EB44626FDDC30B513D5BB71A5D6C4C7DB87C81`.
No custom time, key source, filename, library path or weakening option is exposed
by the command. A failed verification does not authorize another tool or package.

## What the current regressions prove

The real public registry metadata signatures, tampered registry metadata/signature
and missing registry key cases run with existing Node crypto. Detached verification
tests use a labeled contract double: they prove wrapper fingerprint checks, exact
entry/digest checks, awaited failures and default-policy forwarding. They do **not**
prove OpenPGP cryptography or compatibility, and never load the uninstalled library.
Actual public-signature/tampered-input checks are mandatory staged work above.

Primary sources inspected: [OpenPGP.js detached verification](https://docs.openpgpjs.org/),
[pinned verification API](https://raw.githubusercontent.com/openpgpjs/openpgpjs/v6.3.2/src/openpgp.js),
[pinned key validity API](https://raw.githubusercontent.com/openpgpjs/openpgpjs/v6.3.2/src/key/key.js),
[npm registry signature format](https://docs.npmjs.com/about-registry-signatures/).
