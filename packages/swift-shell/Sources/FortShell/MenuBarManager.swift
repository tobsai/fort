import AppKit

enum SystemStatus: String {
    case healthy
    case degraded
    case error
    case disconnected

    var color: NSColor {
        switch self {
        case .healthy:      return .systemGreen
        case .degraded:     return .systemYellow
        case .error:        return .systemRed
        case .disconnected: return .systemGray
        }
    }

    var label: String {
        switch self {
        case .healthy:      return "Healthy"
        case .degraded:     return "Degraded"
        case .error:        return "Error"
        case .disconnected: return "Disconnected"
        }
    }
}

class MenuBarManager {
    private let statusItem: NSStatusItem
    private let menu: NSMenu
    private let webSocketClient: WebSocketClient

    private var status: SystemStatus = .disconnected
    private var taskCount: Int = 0

    // Menu items that get updated dynamically
    private let statusMenuItem = NSMenuItem(title: "Status: Disconnected", action: nil, keyEquivalent: "")
    private let taskMenuItem = NSMenuItem(title: "Active Tasks: 0", action: nil, keyEquivalent: "")

    init(webSocketClient: WebSocketClient) {
        self.webSocketClient = webSocketClient

        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
        menu = NSMenu()

        buildMenu()
        updateStatusIcon()

        statusItem.menu = menu
    }

    // MARK: - Menu Construction

    private func buildMenu() {
        // Status section
        statusMenuItem.isEnabled = false
        menu.addItem(statusMenuItem)

        taskMenuItem.isEnabled = false
        menu.addItem(taskMenuItem)

        menu.addItem(.separator())

        // Quick actions
        let dashboardItem = NSMenuItem(
            title: "Open Dashboard",
            action: #selector(openDashboard),
            keyEquivalent: "d"
        )
        dashboardItem.keyEquivalentModifierMask = [.command]
        dashboardItem.target = self
        menu.addItem(dashboardItem)

        let doctorItem = NSMenuItem(
            title: "Run Doctor",
            action: #selector(runDoctor),
            keyEquivalent: ""
        )
        doctorItem.target = self
        menu.addItem(doctorItem)

        menu.addItem(.separator())

        // New feature items (Coming Soon)
        let voiceItem = NSMenuItem(
            title: "Voice Mode (Coming Soon)",
            action: nil,
            keyEquivalent: ""
        )
        voiceItem.isEnabled = false
        menu.addItem(voiceItem)

        let hotkeyItem = NSMenuItem(
            title: "Global Hotkey: Cmd+Shift+F (Coming Soon)",
            action: nil,
            keyEquivalent: ""
        )
        hotkeyItem.isEnabled = false
        menu.addItem(hotkeyItem)

        let spotlightItem = NSMenuItem(
            title: "Reindex Spotlight (Coming Soon)",
            action: nil,
            keyEquivalent: ""
        )
        spotlightItem.isEnabled = false
        menu.addItem(spotlightItem)

        menu.addItem(.separator())

        let reconnectItem = NSMenuItem(
            title: "Reconnect",
            action: #selector(reconnect),
            keyEquivalent: "r"
        )
        reconnectItem.keyEquivalentModifierMask = [.command]
        reconnectItem.target = self
        menu.addItem(reconnectItem)

        menu.addItem(.separator())

        let quitItem = NSMenuItem(
            title: "Quit Fort",
            action: #selector(quit),
            keyEquivalent: "q"
        )
        quitItem.keyEquivalentModifierMask = [.command]
        quitItem.target = self
        menu.addItem(quitItem)
    }

    // MARK: - Status Updates

    func updateStatus(_ newStatus: SystemStatus) {
        status = newStatus
        statusMenuItem.title = "Status: \(status.label)"
        updateStatusIcon()
    }

    func updateTaskCount(_ count: Int) {
        taskCount = count
        taskMenuItem.title = "Active Tasks: \(count)"
    }

    private func updateStatusIcon() {
        guard let button = statusItem.button else { return }

        let size = NSSize(width: 18, height: 18)
        let image = NSImage(size: size, flipped: false) { rect in
            let dotRect = NSRect(x: 4, y: 4, width: 10, height: 10)
            let path = NSBezierPath(ovalIn: dotRect)
            self.status.color.setFill()
            path.fill()
            return true
        }
        image.isTemplate = false
        button.image = image
    }

    // MARK: - Actions

    @objc private func openDashboard() {
        webSocketClient.send(action: "open_dashboard")
        // Fall back to opening in browser if the core supports it
        if let url = URL(string: "http://localhost:4000") {
            NSWorkspace.shared.open(url)
        }
    }

    @objc private func runDoctor() {
        webSocketClient.send(action: "run_doctor")
    }

    @objc private func reconnect() {
        webSocketClient.disconnect()
        webSocketClient.connect()
        updateStatus(.disconnected)
    }

    @objc private func quit() {
        NSApplication.shared.terminate(nil)
    }
}
