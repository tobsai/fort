# UTM 4.7.5 artifact assessment — stopped and cleaned

On 2026-10-13 the exact official DMG passed its byte/hash pin, disk-image integrity
check and stapled notarization-ticket validation. Its Gatekeeper **DMG open /
primary-signature** assessment returned exit **3**, `rejected`,
`source=no usable signature`. The command ran outside the command sandbox; this
is an OS assessment result, not an automatic approval-review rejection.

The approved scope explicitly required stopping on a rejection. No image was
attached, no app was copied, and no downloaded code was launched. The seven
recorded staging objects were removed. A separate read-only verification script
confirmed all eleven recorded/candidate paths absent and found no task image
or UTM app process. This is a stopped package assessment, **not an accepted UTM
installation or an OS-isolation result**.

## Authority and exact artifact

- Owner approval: `Sentinel_ba1df7211ad48191994b113b470cfe43`, 2026-10-13 00:10 UTC;
  reminder `Sentinel_8046e6f9b2708191b1af3e4b8c328e10`; original request
  `Sentinel_19acd0b905fc81919a84acf25719506d`. Only identifiers and the delegated
  scope summary were recorded; historical message bodies were not retrieved.
- Approved proposal commit: `ec6dd956bd6ebf45ec685331625ccc429e59b7f6`;
  proposal seal SHA256:
  `99decb7c28b2c2daba3972709642a12ed4d3cce084a40bc3c3dd3e0008899159`.
  All eight sealed files still match the commit. The proposal's historical
  draft/unapproved wording is preserved; this assessment's authority is recorded
  separately in [authority-and-preflight.json](authority-and-preflight.json).
- Source: [official UTM v4.7.5 DMG](https://github.com/utmapp/UTM/releases/download/v4.7.5/UTM.dmg).
  Exact bytes: **250,021,057**; cap: 268,435,456.
- SHA256: `a8435c93cfb5f8bbfeea4b134cfad1ac66b67632b75e438c63b1a8ae043bef0e`.
- Host: arm64 macOS 27.2 (26B5101f). App version 4.7.5 / build 118 was the
  expected value; contained-app metadata was not inspected.

## Results and evidence

| Check | Actual result | Receipt |
| --- | --- | --- |
| Fresh owner-only roots, 3 GiB task budget, 32 GiB free-disk reserve | Passed | [preflight](authority-and-preflight.json) |
| Exact official download, size and SHA256 | Passed | [download receipt](01-UTM.dmg.json) |
| `hdiutil verify` | Passed, exit 0 | [integrity](dmg-integrity.json) |
| `xcrun --no-cache stapler validate` | Passed, exit 0 | [notarization ticket](dmg-stapler.json) |
| `spctl --assess --type open --context context:primary-signature --verbose=4` | Rejected, exit 3, no usable signature | [Gatekeeper](dmg-gatekeeper.json) |
| Mount, app copy, app/nested-code `codesign`, app Gatekeeper execute assessment | Unrun after stop | [validation](validation.json) |
| Actual signer/certificate/Team ID, entitlements, legal files, signed container IDs | Unverified; unrun after stop | [validation](validation.json) |
| Owned cleanup | Seven objects removed: one DMG and six private directories | [manifest](download-manifest.json), [journal](cleanup-journal.jsonl), [result](cleanup-result.json) |
| Separate post-cleanup script | Eleven paths absent; no task mount or UTM process | [validation](validation.json) |

The unchanged, previously tested downloader's `download` function was invoked for
one fixed input; its multi-input `main` was not called. Only GitHub and its official
release-assets host were allowed, with TLS verification, proxies disabled, no
cookies/auth/netrc, at most four redirects, a 600-second overall deadline and a
streaming byte cap. Signed redirect query strings were omitted from evidence.
See [downloader provenance](downloader-provenance.json). The only other expected
network activity was existing Apple trust/notarization checks; traffic was not
packet-captured.

The temporary root was
`/Users/tobiasgunn/Library/Application Support/FortUTMIsolation`. Its app and mount
subdirectories were empty throughout. Before cleanup, exact device/inode/owner,
mode, file size and hash were rechecked, unknown objects were refused, and no open
task file or task process was found. Cleanup used recorded directory descriptors
without following links, with a durable per-object journal. No detach was needed.
Existing OS audit/trust databases were not edited; normal OS trust-cache effects
are not claimed absent. Derived group-container paths could not be checked because
signed entitlements were never reached. The known app-container path remained
absent, as did both standard UTM installation paths.

No application/helper/verifier launch, LaunchServices invocation, VM, additional
package download, key generation, account/provider access, permission/security
exception, quarantine modification, re-signing, startup service, push, merge or
deployment occurred. Product tests were not rerun: this stage changed no product
code. The separate verification script is a second read-only check by the same
worker, not an independent human/agent review.

## Next narrow decision

This result does **not** establish that the contained UTM app is unsigned,
unnotarized or rejected: it was never assessed. Conversely, the DMG's valid
stapled ticket does not establish the app's actual signer or nested-code integrity.

Review whether to permit a read-only mount of this same pinned DMG solely to
inspect and assess its contained app despite the outer-container primary-signature
result. That would change the explicit stop condition and requires new owner
authority before execution. All existing byte/storage limits, exact-source pins,
no-launch restrictions, app/nested-code acceptance gates and owned cleanup would
remain. No app launch or VM session is authorized or proposed by this assessment.

Branch: `codex/fort-core-openmaus-20261011`; worktree:
`/private/tmp/fort-core-openmaus-20261011`. The existing `reviews/` directory and
all prior source, proposals and evidence were left untouched.
