# Recommended immediate owner decision: inspect the pinned app only

**Draft; not sent or approved.** This is smaller than the eventual VM request and
can resolve actual signing, entitlement and footprint questions without launching
UTM or adapting the runtime. The parent should bind it to this reviewed proposal
commit/seal if presenting it.

> Approve downloading only the official UTM 4.7.5 `UTM.dmg` at the exact URL,
> SHA256 and 256 MiB byte cap in `scope.json`. Permit fresh owner-only task paths
> under `/Users/tobiasgunn/Library/Application Support/FortUTMIsolation` for that
> download, a read-only non-browsing mount, bounded public assessment evidence,
> and an unchanged private copy of the app for signature inspection. Bound the
> staged app to 2 GiB and total observed task storage to 3 GiB; retain 32 GiB
> host disk reserve. Reject existing/unattributed task paths.
>
> Use only existing macOS hash, `hdiutil`, `codesign`, `spctl` and `xcrun stapler`
> tools to verify the DMG, notarization ticket, actual app and nested code;
> inspect public signer identity/Team ID, version/build, entitlements, legal files
> and declared app/group-container identifiers. Record the exact candidate state
> paths and check their presence without opening existing contents. Stop on any
> unexpected permission, license prompt, altered pin, rejection or larger footprint.
>
> No application/helper/verifier launch, invocation of LaunchServices to register
> or open the app, Ubuntu image
> or other package download, account access, key generation, VM, permission change,
> security exception, re-signing or quarantine modification. The GitHub HTTPS
> download and existing Apple OS trust/notarization checks are the only expected
> network operations. No purchase is involved.
>
> Save bounded redacted evidence in the isolated worktree, verify no task process
> uses the mount, detach the exact owned disk image, and remove only recorded
> owned artifacts. Do not edit shared macOS audit/registration databases. An
> incomplete detach/cleanup stays reported, with no broader deletion. This
> approval would not authorize a later app launch or synthetic VM session.

Why a new decision is needed: the prior approved scope names Lima and different
paths/payloads. The accepted Gatekeeper review explicitly requires new approval
for this material manager/distribution change. This is not an automatic approval
review rejection. No owner prompt was issued during this preparation.
