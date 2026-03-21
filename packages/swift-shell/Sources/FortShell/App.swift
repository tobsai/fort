import AppKit

@main
struct FortShellApp {
    static func main() {
        let app = NSApplication.shared
        app.setActivationPolicy(.accessory) // LSUIElement equivalent: no dock icon

        let delegate = AppDelegate()
        app.delegate = delegate

        app.run()
    }
}

class AppDelegate: NSObject, NSApplicationDelegate {
    private var menuBarManager: MenuBarManager!
    private var webSocketClient: WebSocketClient!
    private var notificationManager: NotificationManager!

    // TODO: Activate these managers once their implementations are complete
    // private var spotlightIndexer: SpotlightIndexer!
    // private var shortcutsProvider: FortShortcutsProvider!
    // private var sendToFort: SendToFort!
    // private var globalHotkeyManager: GlobalHotkeyManager!
    // private var focusModeMonitor: FocusModeMonitor!
    // private var voiceManager: VoiceManager!

    func applicationDidFinishLaunching(_ notification: Notification) {
        notificationManager = NotificationManager()
        notificationManager.requestAuthorization()

        webSocketClient = WebSocketClient()
        menuBarManager = MenuBarManager(webSocketClient: webSocketClient)

        webSocketClient.onStatusChange = { [weak self] status in
            DispatchQueue.main.async {
                self?.menuBarManager.updateStatus(status)
            }
        }

        webSocketClient.onTaskCountChange = { [weak self] count in
            DispatchQueue.main.async {
                self?.menuBarManager.updateTaskCount(count)
            }
        }

        webSocketClient.onNotification = { [weak self] title, body in
            self?.notificationManager.send(title: title, body: body)
        }

        webSocketClient.connect()

        // TODO: Initialize and activate new managers:
        //
        // spotlightIndexer = SpotlightIndexer()
        // spotlightIndexer.reindexAll()
        //
        // shortcutsProvider = FortShortcutsProvider()
        //
        // sendToFort = SendToFort(webSocketClient: webSocketClient)
        //
        // globalHotkeyManager = GlobalHotkeyManager()
        // globalHotkeyManager.register(shortcut: .defaultFortHotkey) { [weak self] in
        //     self?.globalHotkeyManager.showInputPanel()
        // }
        //
        // focusModeMonitor = FocusModeMonitor(notificationManager: notificationManager)
        // focusModeMonitor.startMonitoring()
        //
        // voiceManager = VoiceManager()
    }

    func applicationWillTerminate(_ notification: Notification) {
        webSocketClient.disconnect()

        // TODO: Clean up new managers:
        // globalHotkeyManager?.unregister()
        // focusModeMonitor?.stopMonitoring()
        // voiceManager?.stopListening()
    }
}
