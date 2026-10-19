# Developer ID G2 certificate continuation

October 19, 2026. This records the separate direct-distribution certificate task,
not TestFlight provisioning or installed Fort acceptance. The coordinator supplied
owner approval at October 18 23:51:54 UTC, `Sentinel_1757ba0eab18819187f679639d1d3dd5`,
answering exact request `Sentinel_351c5023315081918f3b0c001153fde4`: create/install
Developer ID Application G2 for Maple Tree Enterprises LLC, team `T3JB5MYZ93`,
keep the key on this Mac and preserve existing certificates. Do not revoke,
export keys, change trust/account access or install Xcode components.

## Completed locally

Authenticated Apple Developer page reconfirmed Tobias Gunn and the exact team.
The Certificate Assistant workflow was opened through Keychain Access, following
[Apple's CSR instructions](https://developer.apple.com/help/account/certificates/create-a-certificate-signing-request/).
Its startup Passwords choice required opening Keychain Access again before the
CSR menu opened the assistant. No keychain credential entries were inspected.

The assistant created a 2048-bit RSA key/request using Saved to disk. Public CSR:
`/tmp/fort059-g2-certificate-20261019/Fort-Developer-ID-G2-20261019.certSigningRequest`.
It is 952 bytes. SHA-256:
`f27f4269c9a07066446b6ad85c0888c37458de29bdcd8e5bbaaeb6ee9ea460e4`.
Public SubjectPublicKeyInfo DER SHA-256:
`68029e4058cd51404076d987e544f5dd25cf91de0b3b5b54ea77bef797cc46c2`.
`openssl req -noout -verify -subject` returned verify OK. The subject retained
`Toby Gunn`, the selected `tobias@mtree.io` email and US; a requested Fort common
name did not persist through the native form. No duplicate key was created to
change the label. Use the public-key fingerprint to correlate the eventual leaf.
Private-key bytes were never read, copied or exported. No certificate is installed
for this new key yet, so pairing/signing usability remains unverified.

The save-panel destination assertion first failed before writing; the corrected
native Go to Folder control selected the exact temporary directory. CSR bytes
are outside Git; only identifiers and hashes are recorded here.

An auxiliary process lookup triggered a new ChatGPT-to-System Events automation
permission prompt. The lookup was stopped and Don't Allow selected. No additional
automation permission was granted. Subsequent Keychain interaction used already
available accessibility access, not Apple Events. Xcode's Install Required action
was not used.

## Upload blocked; no new Apple certificate

The portal's Developer ID Application form has **G2 Sub-CA** selected. The public
CSR upload was rejected by automatic approval review before the command ran:
it could not verify explicit authorization for disclosure of CSR identity metadata
and the public key to Apple. No workaround or indirect upload was attempted.
The source-thread lookup returned no turns, so it did not independently recover
that approval text. A precise user approval request is pending through the supported
input panel. The unsubmitted file picker was canceled. Read-only portal inspection
then confirmed **no file selected** and Continue disabled.

Do not retry upload until the supported approval requirement is resolved. Once
resolved, reuse this CSR/key, verify the exact team/G2/filename, create one leaf,
download/install only its public certificate and verify issuer OU=G2, team, expiry,
public-key match and paired signing identity. Preserve existing certificates.
[Apple's G2 replacement workflow](https://developer.apple.com/help/account/certificates/replace-developer-id-certificates/).

Read-only `security find-identity -v -p codesigning` still lists the same four valid
identities, including original Developer ID SHA-1 fingerprints
`BD4AFBD1CE2E7FAF1298638AF291EF6377524D7F` and
`A30AA59E86481C457B5A4F6C61D9CB4E1F43508C`. The earlier public inventory identifies
both as previous-authority leaves expiring February 1, 2027. Apple Development
`58F40D1AB480565B89109B853235AC3A7B2E7AD1` and Apple Distribution
`F7D83FE9BDCE8D61D47A810B612DFE39302ECA32` also remain. No new G2 leaf was created,
downloaded, installed or used for signing, and no existing certificate was revoked.

The signed Mac development artifact and its profiles are unchanged. No Fort
installation/launch, startup registration, provider credential/network action,
archive, upload or TestFlight operation occurred in this G2 continuation.
