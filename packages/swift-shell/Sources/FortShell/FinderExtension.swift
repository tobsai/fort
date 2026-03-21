import Foundation

// MARK: - Finder Quick Action / Finder Sync Extension Stub

/// Handles sending files from Finder to Fort for processing.
///
/// IMPORTANT: In a real implementation, this needs to be a separate
/// Finder Sync Extension target in Xcode, not part of the main app.
///
/// TODO: To properly implement this:
///   1. Add a new "Finder Sync Extension" target in Xcode
///   2. The extension subclasses FIFinderSync (FinderSync framework)
///   3. Register observed directories via FIFinderSyncController.default().directoryURLs
///   4. Implement menu(for:) to add "Send to Fort" context menu item
///   5. The extension communicates with the main app via:
///      - NSXPCConnection (preferred for sandboxed apps)
///      - Or a shared App Group container + Darwin notifications
///   6. Sign with the same team and embed in the main app bundle
///   7. Add NSExtension dictionary to extension's Info.plist
///
/// For now, this class simulates the Finder extension behavior within the main app.
class SendToFort {

    /// File types that Fort can process
    ///
    /// TODO: Map these to proper UTType identifiers:
    ///   - public.plain-text, public.json, public.yaml
    ///   - public.image (png, jpg, etc.)
    ///   - public.pdf
    ///   - com.apple.property-list
    ///   - public.source-code
    static let supportedTypes: [String] = [
        "public.plain-text",
        "public.json",
        "public.image",
        "public.pdf",
        "public.source-code",
        "public.data"
    ]

    /// Reference to WebSocket client for sending file paths to Fort core
    private weak var webSocketClient: WebSocketClient?

    init(webSocketClient: WebSocketClient? = nil) {
        self.webSocketClient = webSocketClient
        print("[FortShell:SendToFort] Initialized (stub — Finder extension not yet a separate target)")
    }

    /// Handle files selected in Finder and send their paths to Fort
    ///
    /// TODO: In the real Finder Sync Extension:
    ///   - This is called from the extension's toolbar/context menu action
    ///   - File URLs come from FIFinderSyncController.default().selectedItemURLs()
    ///   - Use NSXPCConnection to forward to main app, which relays via WebSocket
    ///   - Show progress via FIFinderSyncController badge images
    ///   - Handle file access via security-scoped bookmarks if sandboxed
    func handleFiles(_ urls: [URL]) {
        guard !urls.isEmpty else {
            print("[FortShell:SendToFort] handleFiles called with empty URL list")
            return
        }

        print("[FortShell:SendToFort] handleFiles (stub) — processing \(urls.count) file(s):")
        for url in urls {
            print("[FortShell:SendToFort]   \(url.path)")
        }

        let paths = urls.map { $0.path }

        // TODO: Wire to WebSocket when available
        if let client = webSocketClient {
            client.send(action: "files_received", payload: ["paths": paths])
            print("[FortShell:SendToFort] Sent \(paths.count) file path(s) to Fort core via WebSocket")
        } else {
            print("[FortShell:SendToFort] WebSocket client not available — files not sent")
        }
    }

    /// Check if a file's UTI is in the supported types list
    ///
    /// TODO: Use UTType.conforms(to:) for proper type hierarchy checking
    /// e.g., a .swift file conforms to public.source-code which conforms to public.plain-text
    func isSupported(uti: String) -> Bool {
        print("[FortShell:SendToFort] isSupported (stub) — checking \(uti)")
        return SendToFort.supportedTypes.contains(uti)
    }
}
