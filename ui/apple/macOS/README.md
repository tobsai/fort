# FortMac — macOS Primary Channels

FortMac is the native Mac Phase 1 surface. Its main window shares
`PrimaryChannelsView` with iPhone, while `MenuBarExtra` provides a bounded glance
at the same durable Channel state. It does not compile a second presentation or
legacy orchestration dashboard.

- **Platform:** macOS 13+
- **Shipping bundle:** `io.mtree.fort.mac`
- **Local transport:** the installed Fort daemon on loopback
- **Shared dependency:** the local FortKit Swift package

## Shipping sources

The XcodeGen target has an explicit source allowlist in `../project.yml`:

| File | Role |
|---|---|
| `FortMacApp.swift` | Window and menu-bar scenes, shared `FortClient`, and the `PrimaryChannelsView` root. |
| `MenuContent.swift` | A compact Channel/Needs You glance and contextual service recovery. |
| `Assets.xcassets` | The canonical Fort app icon and orbital agent artwork. |

Adding another presentation file requires an intentional project manifest
change; dropping a Swift file into this directory does not compile it into the
shipping app.

## Build

```sh
cd ui/apple
xcodegen generate
xcodebuild -project Fort.xcodeproj -scheme FortMac \
  -destination 'platform=macOS' \
  CODE_SIGNING_ALLOWED=NO build
```

`LSUIElement` is false: launching the app opens its window and keeps a Dock icon,
while the menu-bar scene remains available. The Mac client connects to the local
daemon through FortKit; release packaging and notarization are documented in
[`../../../docs/notes/mac-app.md`](../../../docs/notes/mac-app.md).
