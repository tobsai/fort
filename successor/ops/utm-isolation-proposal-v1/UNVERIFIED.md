# Unverified assumptions and stop conditions

| Gate | What is established | What remains necessary |
| --- | --- | --- |
| Actual distribution | Official release asset ID, SHA256, byte size and versioned Developer ID/notarization workflow | Download under new authority; match all pins, inspect version/build, real certificate chain/Team ID/entitlements, stapled ticket and local Gatekeeper acceptance. No independent certificate pin is claimed. |
| Archive and direct boot | Official Ubuntu tar URL/hash; checksum text matches the previously verified metadata pin; UTM supports raw disks and direct kernel/initrd boot | Inspect authenticated tar members, disk format/layout/root label, kernel architecture/compression and initrd. Bound extraction and verify actual boot. Do not invent member names or use the prior QCOW2 bytes as raw. |
| Resource ceiling | Concrete 24 GiB envelope and 4 GiB auxiliary breakdown | Actual app expansion and app state sizes, extraction peaks and no duplicate raw disk. Stop before an allocation that cannot fit; no optimistic sparse-file accounting. |
| Host app state | Source uses ordinary UserDefaults registry/bookmarks and app/group sandbox entitlements; common app/container paths currently absent | Resolve signed group IDs and exact preferences/cache/temp paths, record absence/ownership, inspect LaunchServices interaction. A private app location does not isolate standard bundle-ID state. No claim that cleanup can delete protected containers without owner assistance. |
| In-place registration | Pinned UTM import function supports a shortcut without a package copy | Confirm the actual GUI exposes the appropriate route. Do not default to copying a 20 GiB disk or changing storage preferences. |
| Permission footprint | Manual GUI route omits Apple Events/Automation; source declares broad hardware/network entitlements | Actual OS prompts and runtime behavior; any new permission stops. Bundle entitlement presence does not imply that a physical device was attached or a TCC grant is available. |
| Host network | Published release-note request and source endpoint; optional update setting | First-launch behavior and no unexpected requests/listeners; host egress is not technically confined by this plan. Apple trust services are OS-managed. |
| NoCloud | Local CIDATA seeds are documented | Author exact seed/install changes, validate modules/schema/content, suppress network and SSH generation, ensure package/update services stay off; actual image package versions and label must match. |
| PTY management | Pinned UTM source creates raw PTYs and VZ serial attachments; documentation exposes the path in VM details | Test-first fixed transport/broker implementation; device ownership, raw mode, exclusivity, process/package correlation, guest hvc ordering, no login/getty/log collision, frame/deadline bounds and uncertainty behavior. A PTY path or nonce alone is not identity authentication. |
| Guest boundary | Prior reviewed guest fixture source provides a starting point | Preserve runtime/controller state invariants while replacing SSH/install dependencies. Actual root-owned units, UID/capability/namespace/cgroup/device denials and positive controls remain unproven. |
| Lifecycle/cleanup | Owner can operate UTM; source exposes normal/forced stop and closes serial interfaces | Observe actual guest and host termination, exact process identities and closed PTYs before deletion. GUI status or app disappearance alone does not prove every VZ process is gone. No kill-by-name fallback. |

Minimum source work before an execution packet: (1) bounded PTY transport and
fixed inspection/root bridge with meaningful red→green tests; (2) adapted NoCloud
seed and no-NIC install/readiness checks; (3) exact UTM config/registration and
signature/manifest recorder; (4) bounded owner-assisted lifecycle and cleanup;
(5) independent review and execution seal. None is implemented in this proposal.

The no-NIC choice removes the original Lima DHCP/NAT/bootstrap exposure but also
changes the management trust model. It cannot establish remote runner enrollment,
TLS, provider access, real Claude/Codex behavior or unattended hosting. One nominal
session would establish only the measured synthetic boundary/lifetime behavior;
the larger failure matrix and T07 remain separate acceptance work.

If raw archive compatibility or owner-scoped PTY control fails, stop and bring a
new bounded design to review. Do not add a converter, SSH network, installer ISO,
Automation permission or new VM as an unreported fallback. If the owner requires
fully unattended management now, this deliberately manual proposal is insufficient.
