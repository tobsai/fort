import Foundation

/// The shared status grammar used by every Apple Command Deck surface.
public enum FortProjectState: String, Sendable, CaseIterable {
    case needsYou
    case working
    case delivered
    case failed
    case idle

    public static func resolve(run: RunSummary, gates: [GateItem]) -> FortProjectState {
        let status = run.status.lowercased()
        if gates.contains(where: { $0.runID == run.id }) { return .needsYou }
        if ["failed", "error"].contains(status) { return .failed }
        switch status {
        case "running": return .working
        case "succeeded", "done":
            return (run.checkpoints?.rejected ?? 0) > 0 ? .idle : .delivered
        case "blocked", "paused": return .needsYou
        default: return .idle
        }
    }

    public var label: String {
        switch self {
        case .needsYou: return "Needs you"
        case .working: return "Working"
        case .delivered: return "Delivered"
        case .failed: return "Failed"
        case .idle: return "Idle"
        }
    }
}

/// Time-bounded work that still belongs in the Command Deck's attention inbox.
/// Older failures remain visible in project history without staying actionable
/// forever.
public enum FortAttention {
    public static let recentFailureWindow: TimeInterval = 48 * 60 * 60

    public static func isRecentFailure(
        _ run: RunSummary,
        gates: [GateItem],
        now: Date = Date()
    ) -> Bool {
        guard !gates.contains(where: { $0.runID == run.id }),
              ["failed", "error"].contains(run.status.lowercased()),
              let date = timestamp(for: run) else {
            return false
        }
        return date >= now.addingTimeInterval(-recentFailureWindow)
    }

    public static func recentFailures(
        in runs: [RunSummary],
        gates: [GateItem],
        now: Date = Date()
    ) -> [RunSummary] {
        let cutoff = now.addingTimeInterval(-recentFailureWindow)
        let gatedRunIDs = Set(gates.map(\.runID))
        return runs.compactMap { run -> (run: RunSummary, date: Date)? in
            guard !gatedRunIDs.contains(run.id),
                  ["failed", "error"].contains(run.status.lowercased()),
                  let date = timestamp(for: run),
                  date >= cutoff else {
                return nil
            }
            return (run, date)
        }
        .sorted { $0.date > $1.date }
        .map { $0.run }
    }

    fileprivate static func timestamp(for run: RunSummary) -> Date? {
        guard let value = run.updatedAt ?? run.createdAt else { return nil }
        let formatter = ISO8601DateFormatter()
        if let date = formatter.date(from: value) { return date }
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return formatter.date(from: value)
    }
}

/// Deterministic project ordering shared by the native Command Decks. Current
/// human attention and active work stay prominent; failures older than the
/// attention window remain visible as history after the other project states.
public enum FortProjectOrdering {
    public static func sorted(
        _ runs: [RunSummary],
        gates: [GateItem],
        now: Date = Date()
    ) -> [RunSummary] {
        runs.enumerated().sorted { left, right in
            let leftRank = rank(left.element, gates: gates, now: now)
            let rightRank = rank(right.element, gates: gates, now: now)
            if leftRank != rightRank { return leftRank < rightRank }

            let leftDate = FortAttention.timestamp(for: left.element)
            let rightDate = FortAttention.timestamp(for: right.element)
            if leftDate != rightDate {
                return (leftDate ?? .distantPast) > (rightDate ?? .distantPast)
            }
            if left.element.id != right.element.id {
                return left.element.id < right.element.id
            }
            return left.offset < right.offset
        }
        .map(\.element)
    }

    private static func rank(
        _ run: RunSummary,
        gates: [GateItem],
        now: Date
    ) -> Int {
        switch FortProjectState.resolve(run: run, gates: gates) {
        case .needsYou: return 0
        case .failed: return FortAttention.isRecentFailure(run, gates: gates, now: now) ? 1 : 5
        case .working: return 2
        case .idle: return 3
        case .delivered: return 4
        }
    }
}

/// Status-neutral ordering for the conversation history. Urgency belongs in
/// the separate Needs You inbox; the history itself follows the newest real
/// run, gate, or append-only event timestamp.
public enum FortConversationOrdering {
    public static func newestFirst(
        _ runs: [RunSummary],
        gates: [GateItem],
        events: [Event]
    ) -> [RunSummary] {
        let eventDates = Dictionary(grouping: events, by: \.runID).mapValues { values in
            values.compactMap { parse($0.time) }.max()
        }
        let gateDates = Dictionary(grouping: gates, by: \.runID).mapValues { values in
            values.compactMap { parse($0.since) }.max()
        }

        return runs.enumerated().sorted { left, right in
            let leftDate = newestDate(for: left.element, eventDates: eventDates, gateDates: gateDates)
            let rightDate = newestDate(for: right.element, eventDates: eventDates, gateDates: gateDates)
            if leftDate != rightDate {
                return leftDate > rightDate
            }
            if left.element.id != right.element.id {
                return left.element.id < right.element.id
            }
            return left.offset < right.offset
        }
        .map(\.element)
    }

    private static func newestDate(
        for run: RunSummary,
        eventDates: [String: Date?],
        gateDates: [String: Date?]
    ) -> Date {
        [
            eventDates[run.id] ?? nil,
            gateDates[run.id] ?? nil,
            parse(run.updatedAt),
            parse(run.createdAt),
        ]
        .compactMap { $0 }
        .max() ?? .distantPast
    }

    fileprivate static func parse(_ value: String?) -> Date? {
        guard let value, !value.isEmpty else { return nil }
        let formatter = ISO8601DateFormatter()
        if let date = formatter.date(from: value) { return date }
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return formatter.date(from: value)
    }
}

/// A truthful conversation state derived from durable run/gate state and
/// actual provider events. A running flag alone is intentionally not enough to
/// claim that an agent is working.
public enum FortConversationActivity: Equatable, Sendable {
    case starting
    case working
    case pausedForReview
    case paused
    case finished
    case failed
    case canceled
    case ready

    public static func resolve(
        run: RunSummary,
        gates: [GateItem],
        events: [Event]
    ) -> FortConversationActivity {
        if gates.contains(where: { $0.runID == run.id }) {
            return .pausedForReview
        }

        switch run.status.lowercased() {
        case "succeeded", "done": return .finished
        case "failed", "error": return .failed
        case "canceled", "cancelled": return .canceled
		case "blocked", "paused": return .paused
        default: break
        }

        let workKinds: Set<String> = [
			"started", "stdout", "stderr", "message", "tool", "subagent",
        ]
        if events.contains(where: { $0.runID == run.id && workKinds.contains($0.type.lowercased()) }) {
            return .working
        }

        switch run.status.lowercased() {
        case "running", "queued": return .starting
        default: return .ready
        }
    }

    public var label: String {
        switch self {
        case .starting: return "Starting"
        case .working: return "Working"
        case .pausedForReview: return "Paused for review"
        case .paused: return "Paused"
        case .finished: return "Finished"
        case .failed: return "Failed"
        case .canceled: return "Canceled"
        case .ready: return "Ready"
        }
    }

    public var projectState: FortProjectState {
        switch self {
        case .working: return .working
        case .pausedForReview: return .needsYou
        case .finished: return .delivered
        case .failed: return .failed
        case .starting, .paused, .canceled, .ready: return .idle
        }
    }
}

/// A direct answer can become routed work once. Assignments already carrying a
/// flow id are work, so they never re-enter the promotion path.
public enum FortConversationPromotion {
    public static func isEligible(_ run: RunSummary, gates: [GateItem]) -> Bool {
        let finished = ["succeeded", "done"].contains(run.status.lowercased())
        let direct = run.flowID?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ?? true
        let waitingForHuman = gates.contains { $0.runID == run.id }
        return finished && direct && !waitingForHuman
    }
}

/// Collapses repeated assignments into one project-room row without removing
/// any run from conversation history. Ordering happens before grouping so a
/// waiting sign-off remains the representative for its normalized title.
public enum FortProjectRooms {
    public static func unique(
        _ runs: [RunSummary],
        gates: [GateItem],
        now: Date = Date()
    ) -> [RunSummary] {
        var seen: Set<String> = []
        return FortProjectOrdering.sorted(runs, gates: gates, now: now).filter { run in
            let title = normalizedTitle(run.title)
            let key = title.isEmpty ? "run:\(run.id)" : "title:\(title)"
            return seen.insert(key).inserted
        }
    }

    private static func normalizedTitle(_ title: String) -> String {
        title
            .components(separatedBy: .whitespacesAndNewlines)
            .filter { !$0.isEmpty }
            .joined(separator: " ")
            .lowercased()
    }
}

/// Cross-machine copy for the native clients. A non-empty roster is a mesh,
/// never a selected execution target: placement remains Fort's deterministic
/// responsibility.
public struct FortMeshSummary: Equatable, Sendable {
    public let title: String
    public let detail: String?
    public let reachable: Int
    public let total: Int

    public static func resolve(_ machines: [MachineSummary]) -> FortMeshSummary {
        guard !machines.isEmpty else {
            return FortMeshSummary(title: "This Mac", detail: nil, reachable: 1, total: 1)
        }
        let reachable = machines.filter(\.reachable).count
        return FortMeshSummary(
            title: "All machines",
            detail: "\(reachable)/\(machines.count) online",
            reachable: reachable,
            total: machines.count
        )
    }
}

public extension CheckpointSummary {
    var deckCaption: String {
        var parts = ["\(accepted) of \(total) checkpoints accepted"]
        if waiting > 0 { parts.append("\(waiting) awaiting sign-off") }
        if rejected > 0 { parts.append("\(rejected) redirected") }
        return parts.joined(separator: " · ")
    }
}

/// Pure client-side selection used by the explicit Quick question mode. It
/// intentionally ignores trigger enablement: an explicit human choice is an
/// override, while the trigger only controls automatic routing.
public enum FortPlaybookRouting {
    public static func defaultAssignment(in playbooks: [Playbook]) -> Playbook? {
        playbooks
            .filter { $0.delivery != "answer" }
            .sorted {
                if ($0.isDefault == true) != ($1.isDefault == true) {
                    return $0.isDefault == true
                }
                return $0.id < $1.id
            }
            .first
    }

    public static func quickAnswer(in playbooks: [Playbook]) -> Playbook? {
        playbooks
            .filter { $0.delivery == "answer" }
            .sorted {
                let leftQuestion = $0.trigger.kind == "question"
                let rightQuestion = $1.trigger.kind == "question"
                if leftQuestion != rightQuestion { return leftQuestion }
                return $0.id < $1.id
            }
            .first
    }
}

/// Honest delivery state for native handoff surfaces. A missing synchronous
/// answer body or explicit failure kind must never render as success. A 202
/// accepted answer is an observable run whose output arrives through replay/SSE.
public enum FortHandoffOutcome: Equatable, Sendable {
    case assignment
    case answer(String)
    case failure(String)
}

public extension ChatResult {
    var handoffOutcome: FortHandoffOutcome {
        let text = answer?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        switch kind.lowercased() {
        case "answer":
            if accepted == true, !runID.isEmpty { return .assignment }
            return text.isEmpty ? .failure("Quick answer returned no text.") : .answer(text)
        case "error", "failed", "failure":
            return .failure(text.isEmpty ? "Fort reported that the handoff failed." : text)
        default:
            return .assignment
        }
    }
}

/// Deterministic 5x5 project mark matching the web dashboard implementation.
public enum FortSigil {
    public struct Cell: Hashable, Sendable {
        public let x: Int
        public let y: Int

        public init(x: Int, y: Int) {
            self.x = x
            self.y = y
        }
    }

    public static func cells(for name: String) -> [Cell] {
        var hash: UInt32 = 2_166_136_261
        for unit in name.utf16 {
            hash ^= UInt32(unit)
            hash = hash &* 16_777_619
        }

        func random() -> Double {
            hash ^= hash << 13
            hash ^= hash >> 17
            hash ^= hash << 5
            return Double(hash) / 4_294_967_296
        }

        var cells: [Cell] = []
        for x in 0..<3 {
            for y in 0..<5 where random() > 0.55 {
                cells.append(Cell(x: x, y: y))
                if x < 2 { cells.append(Cell(x: 4 - x, y: y)) }
            }
        }
        return cells
    }
}

/// Calendar helpers shared by the native schedule views. Fort's displayed week
/// is always Monday through Sunday, independent of the user's locale setting.
public enum FortSchedule {
    public static func weekdayIndex(for date: Date, calendar: Calendar = .current) -> Int {
        (calendar.component(.weekday, from: date) + 5) % 7
    }

    public static func isInDisplayedWeek(
        _ date: Date,
        containing reference: Date = Date(),
        calendar: Calendar = .current
    ) -> Bool {
        let referenceDay = calendar.startOfDay(for: reference)
        guard let start = calendar.date(
            byAdding: .day,
            value: -weekdayIndex(for: reference, calendar: calendar),
            to: referenceDay
        ), let end = calendar.date(byAdding: .day, value: 7, to: start) else {
            return false
        }
        return date >= start && date < end
    }
}
