import Foundation

/// The sections of Setup, the one place for everything off the map, in the same order as the web: Crew, Routines, Accounts, Devices.
enum PauseTab: String, CaseIterable, Identifiable, Sendable {
    case crew, playbook, accounts, devices
    var id: String { rawValue }
    var title: String { switch self { case .crew: "Crew"; case .playbook: "Routines"; case .accounts: "Accounts"; case .devices: "Devices" } }
    var symbol: String { switch self { case .crew: "person.3.fill"; case .playbook: "play.rectangle.on.rectangle"; case .accounts: "bolt.fill"; case .devices: "desktopcomputer" } }
}

/// Which pause-menu tabs show and which carry a "needs you" count. Built from plain facts so it is testable without a view.
struct PauseMenu: Equatable, Sendable {
    struct Facts: Equatable, Sendable {
        var hasWorkspace = false
        var supportsRoutines = false
        var canManageAccount = false
        var accountSelected = false
        var showsDevices = true
        var runtimeState: String?
        var pendingDecisions = 0
    }
    struct Entry: Equatable, Identifiable, Sendable {
        let tab: PauseTab
        let badge: Int
        var id: String { tab.id }
    }
    let entries: [Entry]

    static func make(_ facts: Facts) -> PauseMenu {
        var entries: [Entry] = []
        if facts.hasWorkspace { entries.append(Entry(tab:.crew,badge:facts.pendingDecisions)) }
        if facts.hasWorkspace || facts.supportsRoutines { entries.append(Entry(tab:.playbook,badge:0)) }
        if facts.canManageAccount { entries.append(Entry(tab:.accounts,badge:facts.accountSelected ? 0 : 1)) }
        if facts.showsDevices { entries.append(Entry(tab:.devices,badge:["stopped","restartRequired"].contains(facts.runtimeState ?? "") ? 1 : 0)) }
        return PauseMenu(entries:entries)
    }
    /// The asked-for tab when it exists, otherwise the first tab that needs you, otherwise the first tab.
    func opening(_ preferred: PauseTab?) -> PauseTab? {
        if let preferred, entries.contains(where:{ $0.tab == preferred }) { return preferred }
        return (entries.first { $0.badge > 0 } ?? entries.first)?.tab
    }
    /// An agent's class is the name of the runtime it is bound to, such as "Claude" or "ChatGPT".
    static func className(provider: String, runtimes: [RuntimeDescriptor]) -> String {
        runtimes.first { $0.provider == provider }?.name ?? provider
    }
}
