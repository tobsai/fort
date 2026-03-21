import AppKit

/// Represents a keyboard shortcut combination
struct KeyCombo {
    let keyCode: UInt16
    let modifiers: NSEvent.ModifierFlags

    /// Default Fort hotkey: Cmd+Shift+F
    static let defaultFortHotkey = KeyCombo(
        keyCode: 3,  // 'F' key
        modifiers: [.command, .shift]
    )

    var description: String {
        var parts: [String] = []
        if modifiers.contains(.command) { parts.append("Cmd") }
        if modifiers.contains(.shift) { parts.append("Shift") }
        if modifiers.contains(.option) { parts.append("Opt") }
        if modifiers.contains(.control) { parts.append("Ctrl") }
        parts.append("Key(\(keyCode))")
        return parts.joined(separator: "+")
    }
}

/// Manages global keyboard shortcuts for Fort
///
/// TODO: Accessibility permissions are REQUIRED for global hotkeys:
///   1. The app must be added to System Settings > Privacy & Security > Accessibility
///   2. On first launch, prompt the user via AXIsProcessTrusted() check
///   3. If not trusted, open the pref pane:
///      NSWorkspace.shared.open(URL(string: "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility")!)
///   4. Use AXIsProcessTrustedWithOptions() with kAXTrustedCheckOptionPrompt to show system dialog
///
/// TODO: For production, consider using CGEvent.tapCreate() instead of NSEvent monitors:
///   - CGEvent taps are more reliable for global hotkeys
///   - They can intercept events before they reach other apps
///   - Require the same Accessibility permissions
///   - Use CGEvent.tapCreate(tap:place:options:eventsOfInterest:callback:userInfo:)
class GlobalHotkeyManager {

    private var monitor: Any?
    private var registeredAction: (() -> Void)?
    private var registeredCombo: KeyCombo?

    /// Whether a hotkey is currently registered and active
    var isRegistered: Bool {
        return monitor != nil
    }

    init() {
        print("[FortShell:GlobalHotkeyManager] Initialized (stub)")
    }

    // MARK: - Registration

    /// Register a global keyboard shortcut
    ///
    /// TODO: Replace NSEvent.addGlobalMonitorForEvents with CGEvent tap for reliability:
    ///   - let eventMask = (1 << CGEventType.keyDown.rawValue)
    ///   - let tap = CGEvent.tapCreate(tap: .cgSessionEventTap, ...)
    ///   - let runLoopSource = CFMachPortCreateRunLoopSource(...)
    ///   - CFRunLoopAddSource(CFRunLoopGetCurrent(), runLoopSource, .commonModes)
    ///   - CGEvent.tapEnable(tap: tap, enable: true)
    func register(shortcut: KeyCombo, action: @escaping () -> Void) {
        // Remove existing registration first
        unregister()

        registeredCombo = shortcut
        registeredAction = action

        print("[FortShell:GlobalHotkeyManager] register (stub) — registering \(shortcut.description)")
        print("[FortShell:GlobalHotkeyManager] NOTE: Global monitor requires Accessibility permissions")

        // TODO: Check accessibility permissions first
        // if !AXIsProcessTrusted() {
        //     print("[FortShell:GlobalHotkeyManager] Accessibility not granted — prompting user")
        //     let options = [kAXTrustedCheckOptionPrompt.takeRetainedValue(): true] as CFDictionary
        //     AXIsProcessTrustedWithOptions(options)
        //     return
        // }

        monitor = NSEvent.addGlobalMonitorForEvents(matching: .keyDown) { [weak self] event in
            guard let self = self, let combo = self.registeredCombo else { return }

            let matchesKey = event.keyCode == combo.keyCode
            let matchesModifiers = event.modifierFlags.intersection(.deviceIndependentFlagsMask) == combo.modifiers

            if matchesKey && matchesModifiers {
                print("[FortShell:GlobalHotkeyManager] Hotkey triggered: \(combo.description)")
                DispatchQueue.main.async {
                    self.registeredAction?()
                }
            }
        }

        if monitor != nil {
            print("[FortShell:GlobalHotkeyManager] Global monitor registered successfully")
        } else {
            print("[FortShell:GlobalHotkeyManager] Failed to register global monitor (Accessibility permissions?)")
        }
    }

    /// Remove the registered global hotkey
    func unregister() {
        if let monitor = monitor {
            NSEvent.removeMonitor(monitor)
            self.monitor = nil
            print("[FortShell:GlobalHotkeyManager] Unregistered global hotkey")
        }
        registeredCombo = nil
        registeredAction = nil
    }

    // MARK: - Input Panel

    /// Show a floating input panel for quick Fort interaction
    ///
    /// TODO: Implement a proper NSPanel-based floating input:
    ///   1. Create an NSPanel with .nonactivatingPanel style mask
    ///   2. Set panel.level = .floating
    ///   3. Set panel.isFloatingPanel = true
    ///   4. Add an NSTextField for text input
    ///   5. Add a "Send" button and handle Enter key
    ///   6. Position at screen center or near mouse cursor
    ///   7. On submit, send text to Fort via WebSocket
    ///   8. Show response in the same panel or a result popover
    ///   9. Dismiss on Escape or click outside
    ///   10. Animate in/out with NSAnimationContext
    func showInputPanel() {
        print("[FortShell:GlobalHotkeyManager] showInputPanel (stub) — floating input panel not yet implemented")
        // TODO: Create and display NSPanel with text input
    }

    deinit {
        unregister()
    }
}
