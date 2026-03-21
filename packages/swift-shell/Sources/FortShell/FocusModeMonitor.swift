import AppKit

/// Monitors macOS Focus/Do Not Disturb state to gate Fort notifications
///
/// TODO: macOS Focus mode detection approaches:
///   1. **UserDefaults (private API, fragile)**:
///      Read ~/Library/DoNotDisturb/DB/Assertions.json or
///      com.apple.controlcenter.plist for DND state
///
///   2. **NSWorkspace notifications**:
///      - There is no official public API for Focus mode detection in macOS
///      - NSDistributedNotificationCenter can observe "com.apple.notificationcenterui.enableDND"
///        but this is undocumented and may break across OS versions
///
///   3. **UNUserNotificationCenter (indirect)**:
///      - Check notification delivery settings via getNotificationSettings()
///      - If authorizationStatus is .authorized but notifications aren't delivering,
///        Focus mode may be active
///
///   4. **Recommended approach**: Use DNDManager (private framework) via runtime
///      or accept that Focus mode detection is best-effort and fall back to
///      user-configured schedules in Fort's own config
///
///   5. **macOS 15+ (Sequoia)**: Check for any new public APIs in the Focus framework
class FocusModeMonitor {

    /// Notification categories and whether they should be suppressed during Focus
    enum NotificationCategory: String, CaseIterable {
        case taskComplete = "task_complete"
        case taskFailed = "task_failed"
        case agentMessage = "agent_message"
        case routineReminder = "routine_reminder"
        case systemAlert = "system_alert"
        case memoryUpdate = "memory_update"
    }

    /// Categories that should always get through, even during Focus
    private let alwaysAllowedCategories: Set<NotificationCategory> = [
        .taskFailed,
        .systemAlert
    ]

    /// Current detected focus mode state
    private(set) var isFocusModeActive: Bool = false

    /// Callback when focus mode state changes
    var onFocusModeChange: ((Bool) -> Void)?

    /// Reference to NotificationManager for gating
    private weak var notificationManager: NotificationManager?

    private var distributedObserver: NSObjectProtocol?

    init(notificationManager: NotificationManager? = nil) {
        self.notificationManager = notificationManager
        print("[FortShell:FocusModeMonitor] Initialized (stub)")
    }

    // MARK: - Monitoring

    /// Start monitoring for Focus/DND state changes
    ///
    /// TODO: Subscribe to actual system notifications:
    ///   - NSDistributedNotificationCenter.default().addObserver(
    ///       forName: NSNotification.Name("com.apple.notificationcenterui.enableDND"),
    ///       ...)
    ///   - Also try observing NSWorkspace.didWakeNotification and
    ///     NSWorkspace.willSleepNotification for sleep/wake transitions
    ///   - Poll periodically as a fallback since Focus changes may not have
    ///     reliable notifications on all macOS versions
    func startMonitoring() {
        print("[FortShell:FocusModeMonitor] startMonitoring (stub) — not yet observing system focus state")

        // TODO: Wire to actual distributed notification
        // distributedObserver = DistributedNotificationCenter.default().addObserver(
        //     forName: NSNotification.Name("com.apple.notificationcenterui.enableDND"),
        //     object: nil,
        //     queue: .main
        // ) { [weak self] notification in
        //     self?.handleFocusChange(notification)
        // }

        print("[FortShell:FocusModeMonitor] Would observe: com.apple.notificationcenterui.enableDND")
        print("[FortShell:FocusModeMonitor] Focus mode detection is best-effort due to limited public APIs")
    }

    /// Stop monitoring focus mode changes
    func stopMonitoring() {
        print("[FortShell:FocusModeMonitor] stopMonitoring (stub)")

        if let observer = distributedObserver {
            DistributedNotificationCenter.default().removeObserver(observer)
            distributedObserver = nil
        }
    }

    // MARK: - Focus State

    /// Returns the current focus mode state
    ///
    /// TODO: Implement actual detection — see class-level TODO for approaches
    var currentFocusMode: Bool {
        print("[FortShell:FocusModeMonitor] currentFocusMode (stub) — returning \(isFocusModeActive)")
        return isFocusModeActive
    }

    /// Determine whether a notification of the given category should be suppressed
    ///
    /// Logic:
    ///   - If Focus mode is NOT active, never suppress
    ///   - If Focus mode IS active, suppress unless category is in alwaysAllowed set
    ///   - Critical categories (task failures, system alerts) always get through
    func shouldSuppressNotification(category: String) -> Bool {
        guard isFocusModeActive else {
            return false
        }

        guard let cat = NotificationCategory(rawValue: category) else {
            // Unknown category — suppress during focus by default
            print("[FortShell:FocusModeMonitor] Unknown category '\(category)' — suppressing during focus")
            return true
        }

        let suppress = !alwaysAllowedCategories.contains(cat)
        print("[FortShell:FocusModeMonitor] shouldSuppressNotification (stub) — category=\(category) suppress=\(suppress)")
        return suppress
    }

    // MARK: - Private

    private func handleFocusChange(_ notification: Notification) {
        // TODO: Parse the notification to determine new focus state
        // The notification userInfo may contain the enabled/disabled state
        let wasActive = isFocusModeActive
        // isFocusModeActive = ... parse from notification ...

        if wasActive != isFocusModeActive {
            print("[FortShell:FocusModeMonitor] Focus mode changed: \(isFocusModeActive)")
            onFocusModeChange?(isFocusModeActive)
        }
    }

    deinit {
        stopMonitoring()
    }
}
