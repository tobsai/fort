import Foundation

// TODO: Import AppIntents when building with Xcode (requires macOS 13+)
// AppIntents is not reliably available in command-line Swift builds
// #if canImport(AppIntents)
// import AppIntents
// #endif

// MARK: - Intent Definitions (Stubs)
// These are plain classes for now. When AppIntents is available, they should
// conform to AppIntent and use the @Parameter property wrapper.

/// Send a prompt to Fort and get a response
///
/// TODO: When wiring to AppIntents:
///   - Conform to AppIntent protocol
///   - Add @Parameter for prompt: String
///   - perform() should send prompt via WebSocketClient and await response
///   - Return an IntentResult with the Fort response text
///   - Add AppShortcutsProvider conformance with phrases like "Ask Fort..."
class AskFortIntent {
    var prompt: String = ""

    func perform() -> String {
        print("[FortShell:AskFortIntent] perform (stub) — prompt: \(prompt)")
        // TODO: Wire to WebSocketClient.send(action: "ask", payload: ["prompt": prompt])
        // and await the response via a continuation
        return "Fort response placeholder — not yet implemented"
    }
}

/// Query current task status
///
/// TODO: When wiring to AppIntents:
///   - Conform to AppIntent
///   - Optional @Parameter for taskId: String?
///   - perform() should query task status via WebSocketClient
///   - Return task status summary
class GetTaskStatusIntent {
    var taskId: String?

    func perform() -> String {
        print("[FortShell:GetTaskStatusIntent] perform (stub) — taskId: \(taskId ?? "all")")
        // TODO: Wire to WebSocketClient.send(action: "get_task_status", payload: [...])
        return "Task status placeholder — not yet implemented"
    }
}

/// Trigger a named routine (e.g., "morning review", "daily digest")
///
/// TODO: When wiring to AppIntents:
///   - Conform to AppIntent
///   - @Parameter for routineName: String
///   - perform() should trigger routine via WebSocketClient
///   - Return confirmation or routine output summary
class RunRoutineIntent {
    var routineName: String = ""

    func perform() -> String {
        print("[FortShell:RunRoutineIntent] perform (stub) — routine: \(routineName)")
        // TODO: Wire to WebSocketClient.send(action: "run_routine", payload: ["name": routineName])
        return "Routine '\(routineName)' triggered — not yet implemented"
    }
}

/// Search Fort's memory graph
///
/// TODO: When wiring to AppIntents:
///   - Conform to AppIntent
///   - @Parameter for query: String
///   - perform() should search memory via WebSocketClient
///   - Return matching memory nodes as a formatted list
class SearchMemoryIntent {
    var query: String = ""

    func perform() -> String {
        print("[FortShell:SearchMemoryIntent] perform (stub) — query: \(query)")
        // TODO: Wire to WebSocketClient.send(action: "search_memory", payload: ["query": query])
        return "Memory search results placeholder — not yet implemented"
    }
}

/// Create a new Fort task
///
/// TODO: When wiring to AppIntents:
///   - Conform to AppIntent
///   - @Parameter for title: String, description: String?
///   - perform() should create task via WebSocketClient
///   - Return the new task ID and confirmation
class CreateTaskIntent {
    var title: String = ""
    var taskDescription: String?

    func perform() -> String {
        print("[FortShell:CreateTaskIntent] perform (stub) — title: \(title)")
        // TODO: Wire to WebSocketClient.send(action: "create_task", payload: ["title": title, ...])
        return "Task '\(title)' created — not yet implemented"
    }
}

// MARK: - Shortcuts Provider

/// Registers all Fort intents with macOS Shortcuts app
///
/// TODO: When AppIntents is available:
///   - Conform to AppShortcutsProvider
///   - Define static var appShortcuts: [AppShortcut] with phrases:
///     - "Ask Fort \(.applicationName)"
///     - "Check task status in \(.applicationName)"
///     - "Run routine in \(.applicationName)"
///     - "Search \(.applicationName) memory"
///     - "Create task in \(.applicationName)"
///   - Each shortcut maps to the corresponding intent class above
class FortShortcutsProvider {
    static let shared = FortShortcutsProvider()

    /// All available intent types
    let availableIntents: [String] = [
        "AskFortIntent",
        "GetTaskStatusIntent",
        "RunRoutineIntent",
        "SearchMemoryIntent",
        "CreateTaskIntent"
    ]

    init() {
        print("[FortShell:ShortcutsProvider] Initialized (stub — AppIntents not yet wired)")
        print("[FortShell:ShortcutsProvider] Available intents: \(availableIntents.joined(separator: ", "))")
    }
}
