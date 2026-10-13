# Proposed host effects and owner interaction

**Future effects only. None occurred during this preparation.** The later owner
request covers one attended run after implementation/review, not background
permission to develop an unrestricted VM service.

## Files and storage

The fresh private root is exactly
`/Users/tobiasgunn/Library/Application Support/FortUTMIsolation`.
It holds nine pinned public downloads, an unchanged private UTM app, the temporary
OpenPGP.js verifier, one `.utm` package/raw disk, kernel/initrd, a synthetic CIDATA
seed, a public host canary and bounded evidence. Parent directory permissions are
preserved. Nothing is installed in `/Applications` or global PATH. No package
manager, installer hook, kernel extension, host account, login item or host service
is installed. Creating a private app copy is a new effect beyond the completed
read-only inspection.

The raw disk is at most **20 GiB**, kept at its original accepted logical size;
all auxiliary storage is at most **4 GiB**, making **24 GiB total**. Full logical
size counts for sparse/cloned files; allocated size is also recorded. No second
raw copy, conversion, expansion, snapshot or suspend image is allowed. The app
copy stage completes and its DMG is detached/removed before Ubuntu extraction.
Count the mounted view separately in peak reporting; it must not conceal a cap
overrun. Guest RAM is **4 GiB**, with **2 vCPUs**. Preserve **32 GiB free host disk**
and an observed **8 GiB reclaimable host memory beyond the guest allocation**.
These are measured limits and stop gates, not hard OS reservations. UTM/VZ host
overhead is additional, observed and charged against the reserve.

The auxiliary allocation is 256 MiB DMG, 2,048 MiB app, 768 MiB Ubuntu archive,
32 MiB verifier, 192 MiB kernel/initrd/extraction scratch, 16 MiB seed source/ISO,
128 MiB combined raw and retained evidence/canaries, 512 MiB UTM state/cache/temp,
and 144 MiB margin. All live copies count; a copy stage must fit before it starts.

## Normal macOS app state

A private app path does not isolate its bundle-ID state. The signed group is
`WDNLXAD4W8.com.utmapp.UTM`; sandboxed code identifiers are `com.utmapp.UTM`,
`com.utmapp.utmctl`, `com.utmapp.QEMUHelper` and `com.utmapp.QEMULauncher`.
`scope.json` enumerates the 17 exact container, group, preference, cache and saved
state candidates already checked absent during app inspection. Recheck by `lstat`
before any launch; never open pre-existing contents. Stop if any affected location
exists. No CLI/QEMU execution is permitted merely because its candidate is listed.

First launch can also create sandbox bookmarks, VM registry/preferences, temporary
and cache entries under the current Darwin user temp/cache directories. Before
approval, the recorder must bind those parent locations and define a narrowly
attributed before/after observation strategy; it must not copy unrelated filenames
or contents into evidence. A newly observed path is not automatically owned by
Fort. Require time/process/path identity evidence, record each exact object, and
leave ambiguous objects alone. There is no wildcard temp or container cleanup.

Normal launch may add shared macOS LaunchServices, Gatekeeper, trust-cache and
audit/log metadata. These shared records are not edited or purged. No promise of
zero OS residue or secure erasure is made. Protected sandbox state may require
the owner's normal supported removal action; a denial remains a reported residual
and does not authorize Full Disk Access or a TCC/security reset.

## Owner's attended actions

Reserve a **15-minute window**, keep the Mac awake and remain present. The owner:

1. Launches only the exact privately staged, freshly verified UTM app using the
   normal macOS application flow. Ordinary approved first-open confirmation is
   distinguishable from a security rejection; a rejection stops. No “Open Anyway,”
   re-signing, quarantine stripping or bypass is permitted.
2. Disables optional update checks; leaves UTM Server, template galleries, guest
   tools, USB autoconnect and launch-at-login off. Does not sign in to any account.
3. Registers the exact package **as a shortcut/in place**, checks its UUID and
   complete device configuration, and verifies no disk was copied. If that route
   is unavailable or ambiguous, stops rather than changing storage defaults.
4. Starts that VM once and supplies both displayed PTY paths. The controller
   validates their ownership, type and process/package attribution before opening
   either. The owner does not paste a shell command into a guest terminal.
5. After the bounded session or on a stop condition, stops only the exact VM,
   removes its registry shortcut without deleting unverified content, quits UTM
   and permits only recorded owned cleanup. If normal stop is unavailable, a
   force-stop of that same positively identified VM may be explicitly included in
   the eventual request; broad host process killing is excluded.

The future workflow does not use `utmctl`, AppleScript, Shortcuts, Accessibility
automation or an Automation grant. Native screenshots may document only the UTM
window with owner consent; no whole-desktop capture is implied. Unknown prompts
for admin, microphone, USB, local network, filesystem access beyond the exact
owned package, Automation, Accessibility or Full Disk Access stop the run. Do not
click through. Exact package selection is the intended user-selected file access.

## Network, cost and trusted software

Host network activity includes only the nine bounded public inputs, existing Apple
trust/notarization services and the disclosed UTM first-launch release-note request
to `api.github.com`. Disabling optional update checks does not suppress that
release-note request. UTM retains outbound network entitlement; this design adds
no host firewall or host egress sandbox and cannot certify absence of other
traffic. A new listener, unexpected download or observed unexplained connection
blocks continuation. Do not add packet-capture privileges to investigate.

The guest has **zero NICs from first boot**: no NAT, bridge, DHCP, host port or
network management path. It also has no shared directory, clipboard, physical USB
device, audio, graphics, keyboard/pointer or Rosetta. A virtual xHCI controller may
exist as UTM constructs it on this macOS version; it must have zero attachments.
Two owner-scoped serial PTYs are the only management transport. No SSH keys,
guest passwords, provider stores or owner files are supplied.

UTM and the existing Apple tools run as trusted host software. A temporary pinned
OpenPGP.js verification library would also execute under existing Node, after its
own registry/integrity checks, with no network/write/child/worker/addon grant.
This is a new software-execution effect requiring the consolidated approval;
Node permissions are not the product's hostile-code sandbox. Guest root runs
reviewed synthetic provisioning/broker code, creates only locked/no-login workload
identities and starts no runtime/tool until the exact grant.

No purchase or paid service is involved. Costs are public download bandwidth,
local disk/RAM/CPU/power and the owner's attendance. No provider or remote cleanup
authority is extended by this packet.
