//
//  AgentChannelsModel.swift
//  FortKit
//
//  Navigation persistence and presentation state for the additive Spec 046
//  Agent Channels surface. The legacy Primary Channels model remains intact.
//

import Foundation

#if canImport(Combine)
import Combine
#endif

/// Native product activation mirrors the server's closed `off|primary` mode.
/// Missing, malformed, or future values preserve the Primary rollback shell.
public enum AgentChannelsPresentationMode: String, Sendable, CaseIterable {
    case off
    case primary

    public static func resolve(rawValue: String?) -> AgentChannelsPresentationMode {
        rawValue == AgentChannelsPresentationMode.primary.rawValue ? .primary : .off
    }

    public static var configured: AgentChannelsPresentationMode {
        let environmentValue = ProcessInfo.processInfo.environment["FORT_AGENT_CHANNELS"]
        let bundleValue = Bundle.main.object(forInfoDictionaryKey: "FORT_AGENT_CHANNELS") as? String
        return resolve(rawValue: environmentValue ?? bundleValue)
    }
}

package enum AgentChannelsDestination: Sendable, Hashable {
    case agents
    case agent(String)
    case conversation(channelID: String, conversationID: String)
    case needsYou
    case settings
}

package struct AgentChannelsSelectionState: Codable, Sendable, Equatable {
    package var lastAgentID: String?
    package var lastConversationByAgent: [String: String]

    package init(
        lastAgentID: String?,
        lastConversationByAgent: [String: String]
    ) {
        self.lastAgentID = lastAgentID
        self.lastConversationByAgent = lastConversationByAgent
    }

    package static let empty = AgentChannelsSelectionState(
        lastAgentID: nil,
        lastConversationByAgent: [:]
    )
}

package enum AgentChannelsStartup {
    /// Restores only identity that the current canonical server projection can
    /// prove remains open, owned by the selected Agent Channel, and bound to
    /// that exact Agent Seat. It never substitutes a different available agent.
    package static func restore(
        _ saved: AgentChannelsSelectionState,
        from channels: [AgentChannelSummary]
    ) -> AgentChannelsDestination {
        guard let savedAgentID = saved.lastAgentID,
              let channel = channels.first(where: {
                  $0.channel.id == savedAgentID && $0.channel.state == .open
              })
        else { return .agents }

        guard let savedConversationID = saved.lastConversationByAgent[savedAgentID],
              channel.conversations.contains(where: {
                  $0.conversation.id == savedConversationID
                      && $0.conversation.state == AgentConversationState.open.rawValue
                      && $0.participant.conversationID == savedConversationID
                      && $0.participant.seatID == channel.channel.binding.seat.id
                      && $0.participant.state == "active"
              })
        else { return .agent(savedAgentID) }

        return .conversation(
            channelID: savedAgentID,
            conversationID: savedConversationID
        )
    }
}

package final class AgentChannelsSelectionStore {
    private static let storageKey = "fort.agent-channels.selection.v1"
    private let defaults: UserDefaults
    private let decoder = JSONDecoder()
    private let encoder = JSONEncoder()

    package init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    package func load() -> AgentChannelsSelectionState {
        guard let data = defaults.data(forKey: Self.storageKey),
              let value = try? decoder.decode(AgentChannelsSelectionState.self, from: data)
        else { return .empty }
        return value
    }

    package func selectAgent(_ channelID: String) {
        var value = load()
        value.lastAgentID = channelID
        save(value)
    }

    package func selectConversation(_ conversationID: String, for channelID: String) {
        var value = load()
        value.lastAgentID = channelID
        value.lastConversationByAgent[channelID] = conversationID
        save(value)
    }

    package func clearConversation(for channelID: String) {
        var value = load()
        value.lastConversationByAgent.removeValue(forKey: channelID)
        save(value)
    }

    package func replace(with value: AgentChannelsSelectionState) {
        save(value)
    }

    private func save(_ value: AgentChannelsSelectionState) {
        guard let data = try? encoder.encode(value) else { return }
        defaults.set(data, forKey: Self.storageKey)
    }
}

// MARK: - Bounded presentation client seam

package enum AgentChannelsServiceError: Error {
    case unavailable
    case invalidProjection
}

@MainActor
package protocol AgentChannelsServing: AnyObject {
    func agentOptions() async throws -> [AgentOption]
    func recheckAgentOptions() async throws -> [AgentOption]
    func agentChannels(state: AgentChannelFilter) async throws -> [AgentChannelSummary]
    func agentChannel(id: String) async throws -> AgentChannelDetail
    func agentConversations(
        channelID: String,
        state: AgentConversationFilter
    ) async throws -> [AgentConversationSummary]
    func createAgentChannel(optionID: String, name: String) async throws -> AgentChannelDetail
    func updateAgentChannel(id: String, name: String?, state: AgentChannelState?) async throws
    func agentConversation(
        channelID: String,
        conversationID: String
    ) async throws -> AgentConversationDetail
    func createAgentConversation(channelID: String, name: String) async throws -> AgentConversationDetail
    func updateAgentConversation(
        channelID: String,
        conversationID: String,
        name: String?,
        state: AgentConversationState?,
        pinned: Bool?
    ) async throws
    func postFirstAgentTurn(
        channelID: String,
        name: String,
        clientTurnID: String,
        text: String
    ) async throws -> AgentFirstTurnResult
    func postAgentTurn(
        channelID: String,
        conversationID: String,
        clientTurnID: String,
        text: String
    ) async throws -> AgentTurnResult
    func retryAgentTarget(
        channelID: String,
        conversationID: String,
        targetID: String
    ) async throws -> AgentTarget
    func cancelAgentTarget(
        channelID: String,
        conversationID: String,
        targetID: String
    ) async throws
    func agentNeedsYou() async throws -> [AgentNeedsYouItem]
    func agentConversationEvents(
        channelID: String,
        conversationID: String
    ) -> AsyncThrowingStream<AgentConversationDetail, Error>
}

@MainActor
package extension AgentChannelsServing {
    func agentOptions() async throws -> [AgentOption] { [] }
    func recheckAgentOptions() async throws -> [AgentOption] { [] }
    func agentChannels(state: AgentChannelFilter) async throws -> [AgentChannelSummary] { [] }
    func agentChannel(id: String) async throws -> AgentChannelDetail {
        throw AgentChannelsServiceError.unavailable
    }
    func agentConversations(
        channelID: String,
        state: AgentConversationFilter
    ) async throws -> [AgentConversationSummary] { [] }
    func createAgentChannel(optionID: String, name: String) async throws -> AgentChannelDetail {
        throw AgentChannelsServiceError.unavailable
    }
    func updateAgentChannel(id: String, name: String?, state: AgentChannelState?) async throws {
        throw AgentChannelsServiceError.unavailable
    }
    func agentConversation(
        channelID: String,
        conversationID: String
    ) async throws -> AgentConversationDetail {
        throw AgentChannelsServiceError.unavailable
    }
    func createAgentConversation(channelID: String, name: String) async throws -> AgentConversationDetail {
        throw AgentChannelsServiceError.unavailable
    }
    func updateAgentConversation(
        channelID: String,
        conversationID: String,
        name: String?,
        state: AgentConversationState?,
        pinned: Bool?
    ) async throws {
        throw AgentChannelsServiceError.unavailable
    }
    func postFirstAgentTurn(
        channelID: String,
        name: String,
        clientTurnID: String,
        text: String
    ) async throws -> AgentFirstTurnResult {
        throw AgentChannelsServiceError.unavailable
    }
    func postAgentTurn(
        channelID: String,
        conversationID: String,
        clientTurnID: String,
        text: String
    ) async throws -> AgentTurnResult {
        throw AgentChannelsServiceError.unavailable
    }
    func retryAgentTarget(
        channelID: String,
        conversationID: String,
        targetID: String
    ) async throws -> AgentTarget {
        throw AgentChannelsServiceError.unavailable
    }
    func cancelAgentTarget(
        channelID: String,
        conversationID: String,
        targetID: String
    ) async throws {
        throw AgentChannelsServiceError.unavailable
    }
    func agentNeedsYou() async throws -> [AgentNeedsYouItem] { [] }
    func agentConversationEvents(
        channelID: String,
        conversationID: String
    ) -> AsyncThrowingStream<AgentConversationDetail, Error> {
        AsyncThrowingStream { $0.finish() }
    }
}

@MainActor
extension FortClient: AgentChannelsServing {}

// MARK: - Pure presentation truth

package enum AgentTargetAction: Sendable, Equatable {
    case cancel
    case retry
    case recheckAndRetry
}

package enum AgentTargetRecovery {
    package static func action(for target: AgentTarget) -> AgentTargetAction? {
        switch target.state {
        case "queued", "working":
            return .cancel
        case "failed":
            break
        default:
            return nil
        }
        switch target.errorCode {
        case "seat_unready", "primary_agent_unready", "primary_agent_drift", "chat_policy_unavailable":
            return .recheckAndRetry
        case "daemon_interrupted", "provider_result_unknown", "provider_incomplete", "provider_failed":
            return .retry
        default:
            return nil
        }
    }

    package static func action(
        for item: AgentNeedsYouItem
    ) -> AgentTargetAction? {
        let allowed = Set(item.recoveryActions.map {
            $0.replacingOccurrences(of: "-", with: "_")
        })
        let inferred = action(for: item.target)
        switch inferred {
        case .retry where allowed.contains("retry"):
            return .retry
        case .recheckAndRetry where allowed.contains("recheck_and_retry"):
            return .recheckAndRetry
        case .recheckAndRetry where allowed.contains("retry"):
            return .retry
        default:
            return nil
        }
    }
}

package struct AgentIdentityInspection: Sendable, Equatable {
    package let name: String
    package let seatID: String
    package let agent: String
    package let profile: String
    package let seatModel: String
    package let requestedModel: String
    package let resolvedModel: String
    package let machine: String
    package let adapterID: String
    package let adapterRevision: String
    package let authority: String
    package let policyID: String
    package let policyRevision: String
    package let runtimeContract: String
    package let sessionMode: String
    package let memoryMode: String
    package let readiness: String
    package let readinessReason: String?

    package init(channel: AgentChannelSummary) {
        let binding = channel.channel.binding
        name = channel.channel.name
        seatID = Self.exact(binding.seat.id)
        agent = Self.exact(binding.seat.agent)
        profile = Self.exact(binding.seat.profile)
        seatModel = Self.exact(binding.seat.model)
        requestedModel = Self.exact(binding.authority.requestedModel)
        resolvedModel = Self.exact(binding.authority.resolvedModel)
        machine = Self.exact(binding.seat.machine)
        adapterID = Self.exact(binding.authority.adapterID)
        adapterRevision = Self.exact(binding.authority.adapterRevision)
        authority = Self.exact(binding.authority.authority)
        policyID = Self.exact(binding.authority.policyID)
        policyRevision = Self.exact(binding.authority.policyRevision)
        runtimeContract = Self.exact(binding.authority.runtimeContract)
        sessionMode = Self.exact(binding.authority.sessionMode)
        memoryMode = Self.exact(binding.authority.memoryMode)
        readiness = Self.exact(channel.readiness.state)
        readinessReason = channel.readiness.reason.flatMap { $0.isEmpty ? nil : $0 }
    }

    private static func exact(_ value: String) -> String {
        value.isEmpty ? "unknown" : value
    }

}

package enum AgentConversationName {
    package static func from(_ text: String) -> String {
        let compact = text.split(whereSeparator: \Character.isWhitespace).joined(separator: " ")
        guard compact.count > 54 else { return compact.isEmpty ? "New conversation" : compact }
        return String(compact.prefix(51)) + "…"
    }
}

package enum AgentSendOutcome: Sendable, Equatable {
    case accepted
    case terminalRejection
    case ambiguous
}

package enum AgentSendOutcomeReducer {
    package static func failure(for error: Error) -> AgentSendOutcome {
        if let clientError = error as? FortClientError,
           case .httpStatus(let status, _, _) = clientError,
           (400..<500).contains(status),
           status != 408,
           status != 429 {
            return .terminalRejection
        }
        return .ambiguous
    }
}

package struct AgentPendingTurn: Codable, Sendable, Equatable {
    package let channelID: String
    package let conversationID: String?
    package let name: String
    package let text: String
    package let clientTurnID: String

    package init(
        channelID: String,
        conversationID: String?,
        name: String,
        text: String,
        clientTurnID: String = UUID().uuidString.lowercased()
    ) {
        self.channelID = channelID
        self.conversationID = conversationID
        self.name = name
        self.text = text
        self.clientTurnID = clientTurnID
    }

    package var key: String {
        conversationID.map { "conversation:\(channelID):\($0)" }
            ?? "agent:\(channelID):new-conversation"
    }
}

package struct AgentPendingTurnStore {
    package static let defaultsKey = "fort.agent-channels.pending-turns.v1"

    private let defaults: UserDefaults
    private let key: String

    package init(
        defaults: UserDefaults = .standard,
        key: String = AgentPendingTurnStore.defaultsKey
    ) {
        self.defaults = defaults
        self.key = key
    }

    package func load() -> [String: AgentPendingTurn] {
        guard let data = defaults.data(forKey: key),
              let records = try? JSONDecoder().decode([AgentPendingTurn].self, from: data)
        else { return [:] }
        return records.reduce(into: [:]) { result, pending in
            result[pending.key] = pending
        }
    }

    package func save(_ pending: [String: AgentPendingTurn]) {
        guard !pending.isEmpty else {
            defaults.removeObject(forKey: key)
            return
        }
        let records = pending.values.sorted { $0.key < $1.key }
        guard let data = try? JSONEncoder().encode(records) else { return }
        defaults.set(data, forKey: key)
    }
}

// MARK: - Agent-first model

@MainActor
package final class AgentChannelsModel: ObservableObject {
    @Published package var channels: [AgentChannelSummary] = []
    @Published package var archivedChannels: [AgentChannelSummary] = []
    @Published package var archivedConversationsByAgent: [String: [AgentConversationSummary]] = [:]
    @Published package var options: [AgentOption] = []
    @Published package var needsYou: [AgentNeedsYouItem] = []
    @Published package var conversationDetail: AgentConversationDetail?
    @Published package var destination: AgentChannelsDestination = .agents
    @Published package var errorMessage: String?
    @Published package private(set) var pendingTurns: [String: AgentPendingTurn]
    @Published package var busy = false

    private let selectionStore: AgentChannelsSelectionStore
    private let pendingTurnStore: AgentPendingTurnStore

    package init(
        selectionStore: AgentChannelsSelectionStore = AgentChannelsSelectionStore(),
        pendingTurnStore: AgentPendingTurnStore = AgentPendingTurnStore()
    ) {
        self.selectionStore = selectionStore
        self.pendingTurnStore = pendingTurnStore
        pendingTurns = pendingTurnStore.load()
    }

    package var selectedChannelID: String? {
        switch destination {
        case .agent(let id): return id
        case .conversation(let channelID, _): return channelID
        default: return nil
        }
    }

    package var selectedConversationID: String? {
        guard case .conversation(_, let conversationID) = destination else { return nil }
        return conversationID
    }

    package var selectedChannel: AgentChannelSummary? {
        selectedChannelID.flatMap { id in channels.first { $0.id == id } }
    }

    package var selectedActivity: FortMarkActivity {
        let working = conversationDetail?.targets.contains { $0.state == "working" } ?? false
        return working ? .working : .ambient
    }

    package var selectedPendingTurn: AgentPendingTurn? {
        switch destination {
        case .agent(let channelID):
            return pendingTurns[AgentPendingTurn(
                channelID: channelID,
                conversationID: nil,
                name: "",
                text: "",
                clientTurnID: ""
            ).key]
        case .conversation(let channelID, let conversationID):
            return pendingTurns[AgentPendingTurn(
                channelID: channelID,
                conversationID: conversationID,
                name: "",
                text: "",
                clientTurnID: ""
            ).key]
        default:
            return nil
        }
    }

    package func run(using service: any AgentChannelsServing) async {
        await reload(using: service, restoreStartup: true)
        while !Task.isCancelled {
            do { try await Task.sleep(nanoseconds: 8_000_000_000) }
            catch { return }
            await reload(using: service, restoreStartup: false)
        }
    }

    package func reload(
        using service: any AgentChannelsServing,
        restoreStartup: Bool
    ) async {
        do {
            async let open = service.agentChannels(state: .open)
            async let archived = service.agentChannels(state: .archived)
            async let availableOptions = service.agentOptions()
            async let actionable = service.agentNeedsYou()
            let snapshot = try await (open, archived, availableOptions, actionable)
            channels = snapshot.0
            archivedChannels = snapshot.1
            options = snapshot.2
            needsYou = snapshot.3
            var archivedConversations: [String: [AgentConversationSummary]] = [:]
            for channel in channels {
                archivedConversations[channel.id] = try await service.agentConversations(
                    channelID: channel.id,
                    state: .archived
                )
            }
            archivedConversationsByAgent = archivedConversations
            errorMessage = nil

            if restoreStartup {
                destination = AgentChannelsStartup.restore(selectionStore.load(), from: channels)
            } else {
                validateCurrentDestination()
            }
            if case .conversation(let channelID, let conversationID) = destination {
                await loadConversation(
                    channelID: channelID,
                    conversationID: conversationID,
                    using: service
                )
            } else {
                conversationDetail = nil
            }
        } catch {
            errorMessage = Self.describe(error)
        }
    }

    package func selectAgent(
        channelID: String,
        using service: any AgentChannelsServing
    ) async {
        guard channels.contains(where: { $0.id == channelID && $0.channel.state == .open }) else {
            destination = .agents
            conversationDetail = nil
            return
        }
        selectionStore.selectAgent(channelID)
        destination = AgentChannelsStartup.restore(selectionStore.load(), from: channels)
        if case .conversation(let ownerID, let conversationID) = destination {
            await loadConversation(
                channelID: ownerID,
                conversationID: conversationID,
                using: service
            )
        } else {
            conversationDetail = nil
        }
    }

    package func selectConversation(
        channelID: String,
        conversationID: String,
        using service: any AgentChannelsServing
    ) async {
        guard ownsOpenConversation(channelID: channelID, conversationID: conversationID) else {
            await selectAgent(channelID: channelID, using: service)
            return
        }
        selectionStore.selectConversation(conversationID, for: channelID)
        destination = .conversation(channelID: channelID, conversationID: conversationID)
        await loadConversation(
            channelID: channelID,
            conversationID: conversationID,
            using: service
        )
    }

    package func startNewConversation(channelID: String) {
        guard channels.contains(where: { $0.id == channelID && $0.channel.state == .open }) else { return }
        selectionStore.selectAgent(channelID)
        destination = .agent(channelID)
        conversationDetail = nil
    }

    package func send(text: String, using service: any AgentChannelsServing) async -> Bool {
        let normalized = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !normalized.isEmpty, !busy,
              let channel = selectedChannel,
              channel.readiness.state == "ready"
        else {
            errorMessage = "This agent is not Ready. Recheck its identity before sending."
            return false
        }
        let submission: AgentPendingTurn
        switch destination {
        case .agent(let channelID):
            let candidate = AgentPendingTurn(
                channelID: channelID,
                conversationID: nil,
                name: AgentConversationName.from(normalized),
                text: normalized
            )
            submission = pendingTurns[candidate.key] ?? candidate
        case .conversation(let channelID, let conversationID):
            let candidate = AgentPendingTurn(
                channelID: channelID,
                conversationID: conversationID,
                name: "",
                text: normalized
            )
            submission = pendingTurns[candidate.key] ?? candidate
        default:
            return false
        }
        setPendingTurn(submission)
        busy = true
        defer { busy = false }
        var outcome = AgentSendOutcome.accepted
        var submissionError: String?
        do {
            if let conversationID = submission.conversationID {
                _ = try await service.postAgentTurn(
                    channelID: submission.channelID,
                    conversationID: conversationID,
                    clientTurnID: submission.clientTurnID,
                    text: submission.text
                )
                await loadConversation(
                    channelID: submission.channelID,
                    conversationID: conversationID,
                    using: service
                )
            } else {
                let result = try await service.postFirstAgentTurn(
                    channelID: submission.channelID,
                    name: submission.name,
                    clientTurnID: submission.clientTurnID,
                    text: submission.text
                )
                try acceptFirstTurn(result, expectedChannel: channel)
            }
            setPendingTurn(nil, key: submission.key)
            errorMessage = nil
        } catch {
            outcome = AgentSendOutcomeReducer.failure(for: error)
            if outcome == .terminalRejection {
                setPendingTurn(nil, key: submission.key)
            }
            submissionError = Self.describe(error)
            errorMessage = submissionError
            if let conversationID = submission.conversationID {
                await loadConversation(
                    channelID: submission.channelID,
                    conversationID: conversationID,
                    using: service
                )
                if conversationDetail?.turns.contains(where: {
                    $0.clientTurnID == submission.clientTurnID
                }) == true {
                    outcome = .accepted
                    setPendingTurn(nil, key: submission.key)
                    errorMessage = nil
                } else if let submissionError {
                    errorMessage = submissionError
                }
            }
        }
        return outcome == .accepted
    }

    package func consumeSelectedConversationEvents(
        using service: any AgentChannelsServing
    ) async {
        guard case .conversation(let channelID, let conversationID) = destination else { return }
        do {
            for try await detail in service.agentConversationEvents(
                channelID: channelID,
                conversationID: conversationID
            ) {
                guard case .conversation(channelID, conversationID) = destination else { return }
                if valid(detail: detail, channelID: channelID, conversationID: conversationID) {
                    conversationDetail = detail
                }
            }
        } catch is CancellationError {
            return
        } catch {
            await loadConversation(
                channelID: channelID,
                conversationID: conversationID,
                using: service
            )
        }
    }

    package func renameSelectedConversation(
        _ name: String,
        using service: any AgentChannelsServing
    ) async {
        guard case .conversation(let channelID, let conversationID) = destination else { return }
        await mutateConversation(channelID: channelID, conversationID: conversationID, using: service) {
            try await service.updateAgentConversation(
                channelID: channelID,
                conversationID: conversationID,
                name: name,
                state: nil,
                pinned: nil
            )
        }
    }

    package func setSelectedConversationPinned(
        _ pinned: Bool,
        using service: any AgentChannelsServing
    ) async {
        guard case .conversation(let channelID, let conversationID) = destination else { return }
        await mutateConversation(channelID: channelID, conversationID: conversationID, using: service) {
            try await service.updateAgentConversation(
                channelID: channelID,
                conversationID: conversationID,
                name: nil,
                state: nil,
                pinned: pinned
            )
        }
    }

    package func archiveSelectedConversation(using service: any AgentChannelsServing) async {
        guard case .conversation(let channelID, let conversationID) = destination else { return }
        do {
            try await service.updateAgentConversation(
                channelID: channelID,
                conversationID: conversationID,
                name: nil,
                state: .archived,
                pinned: nil
            )
            selectionStore.clearConversation(for: channelID)
            destination = .agent(channelID)
            conversationDetail = nil
            await reload(using: service, restoreStartup: false)
        } catch {
            errorMessage = Self.describe(error)
        }
    }

    package func reopenConversation(
        channelID: String,
        conversationID: String,
        using service: any AgentChannelsServing
    ) async {
        guard archivedConversationsByAgent[channelID]?.contains(where: {
            $0.conversation.id == conversationID
                && $0.participant.conversationID == conversationID
        }) == true else { return }
        do {
            try await service.updateAgentConversation(
                channelID: channelID,
                conversationID: conversationID,
                name: nil,
                state: .open,
                pinned: nil
            )
            await reload(using: service, restoreStartup: false)
            if ownsOpenConversation(channelID: channelID, conversationID: conversationID) {
                await selectConversation(
                    channelID: channelID,
                    conversationID: conversationID,
                    using: service
                )
            } else {
                destination = .agent(channelID)
            }
        } catch {
            errorMessage = Self.describe(error)
        }
    }

    package func renameSelectedChannel(
        _ name: String,
        using service: any AgentChannelsServing
    ) async {
        guard let channelID = selectedChannelID else { return }
        do {
            try await service.updateAgentChannel(id: channelID, name: name, state: nil)
            await reload(using: service, restoreStartup: false)
        } catch {
            errorMessage = Self.describe(error)
        }
    }

    package func reopenChannel(
        channelID: String,
        using service: any AgentChannelsServing
    ) async {
        guard archivedChannels.contains(where: { $0.id == channelID }) else { return }
        do {
            try await service.updateAgentChannel(id: channelID, name: nil, state: .open)
            selectionStore.selectAgent(channelID)
            await reload(using: service, restoreStartup: false)
            if channels.contains(where: { $0.id == channelID }) {
                destination = .agent(channelID)
                conversationDetail = nil
            }
        } catch {
            errorMessage = Self.describe(error)
        }
    }

    package func createAgentChannel(
        optionID: String,
        name: String,
        using service: any AgentChannelsServing
    ) async -> Bool {
        do {
            let detail = try await service.createAgentChannel(optionID: optionID, name: name)
            selectionStore.selectAgent(detail.id)
            await reload(using: service, restoreStartup: false)
            if channels.contains(where: { $0.id == detail.id }) {
                destination = .agent(detail.id)
            }
            return true
        } catch {
            errorMessage = Self.describe(error)
            return false
        }
    }

    package func recheckOptions(using service: any AgentChannelsServing) async {
        do {
            options = try await service.recheckAgentOptions()
            await reload(using: service, restoreStartup: false)
        } catch {
            errorMessage = Self.describe(error)
        }
    }

    package func recover(
        target: AgentTarget,
        action: AgentTargetAction,
        channelID: String? = nil,
        conversationID: String? = nil,
        using service: any AgentChannelsServing
    ) async {
        guard !busy,
              let ownerID = channelID ?? selectedChannelID,
              let childID = conversationID ?? selectedConversationID,
              AgentTargetRecovery.action(for: target) == action
        else { return }
        await performRecovery(
            target: target,
            action: action,
            channelID: ownerID,
            conversationID: childID,
            using: service
        )
    }

    package func recoverNeedsYou(
        _ item: AgentNeedsYouItem,
        action: AgentTargetAction,
        using service: any AgentChannelsServing
    ) async {
        guard !busy, AgentTargetRecovery.action(for: item) == action else { return }
        await performRecovery(
            target: item.target,
            action: action,
            channelID: item.agentChannel.id,
            conversationID: item.conversation.id,
            using: service
        )
    }

    private func performRecovery(
        target: AgentTarget,
        action: AgentTargetAction,
        channelID ownerID: String,
        conversationID childID: String,
        using service: any AgentChannelsServing
    ) async {
        busy = true
        defer { busy = false }
        do {
            switch action {
            case .cancel:
                try await service.cancelAgentTarget(
                    channelID: ownerID,
                    conversationID: childID,
                    targetID: target.id
                )
            case .retry:
                _ = try await service.retryAgentTarget(
                    channelID: ownerID,
                    conversationID: childID,
                    targetID: target.id
                )
            case .recheckAndRetry:
                options = try await service.recheckAgentOptions()
                _ = try await service.retryAgentTarget(
                    channelID: ownerID,
                    conversationID: childID,
                    targetID: target.id
                )
            }
            errorMessage = nil
            if selectedChannelID == ownerID && selectedConversationID == childID {
                await loadConversation(
                    channelID: ownerID,
                    conversationID: childID,
                    using: service
                )
            }
            needsYou = try await service.agentNeedsYou()
        } catch {
            errorMessage = Self.describe(error)
        }
    }

    private func mutateConversation(
        channelID: String,
        conversationID: String,
        using service: any AgentChannelsServing,
        mutation: () async throws -> Void
    ) async {
        do {
            try await mutation()
            errorMessage = nil
            await reload(using: service, restoreStartup: false)
            if selectedChannelID == channelID && selectedConversationID == conversationID {
                await loadConversation(
                    channelID: channelID,
                    conversationID: conversationID,
                    using: service
                )
            }
        } catch {
            errorMessage = Self.describe(error)
        }
    }

    private func acceptFirstTurn(
        _ result: AgentFirstTurnResult,
        expectedChannel: AgentChannelSummary
    ) throws {
        let detail = result.conversation
        let conversationID = detail.conversation.id
        guard valid(
            detail: detail,
            channelID: expectedChannel.id,
            conversationID: conversationID
        ),
        result.turn.conversationID == conversationID,
        detail.participant.seatID == expectedChannel.channel.binding.seat.id,
        result.targets.allSatisfy({ $0.participantID == detail.participant.id })
        else { throw AgentChannelsServiceError.invalidProjection }
        conversationDetail = detail
        selectionStore.selectConversation(conversationID, for: expectedChannel.id)
        destination = .conversation(
            channelID: expectedChannel.id,
            conversationID: conversationID
        )
    }

    private func loadConversation(
        channelID: String,
        conversationID: String,
        using service: any AgentChannelsServing
    ) async {
        do {
            let detail = try await service.agentConversation(
                channelID: channelID,
                conversationID: conversationID
            )
            guard valid(detail: detail, channelID: channelID, conversationID: conversationID) else {
                throw AgentChannelsServiceError.invalidProjection
            }
            conversationDetail = detail
            for pending in pendingTurns.values where pending.channelID == channelID
                && pending.conversationID == conversationID
                && detail.turns.contains(where: { $0.clientTurnID == pending.clientTurnID }) {
                setPendingTurn(nil, key: pending.key)
            }
            errorMessage = nil
        } catch {
            errorMessage = Self.describe(error)
        }
    }

    private func validateCurrentDestination() {
        switch destination {
        case .agent(let channelID):
            if !channels.contains(where: { $0.id == channelID }) {
                destination = .agents
            }
        case .conversation(let channelID, let conversationID):
            if !ownsOpenConversation(channelID: channelID, conversationID: conversationID) {
                selectionStore.clearConversation(for: channelID)
                destination = channels.contains(where: { $0.id == channelID })
                    ? .agent(channelID)
                    : .agents
            }
        default:
            break
        }
    }

    private func ownsOpenConversation(channelID: String, conversationID: String) -> Bool {
        guard let channel = channels.first(where: { $0.id == channelID }) else { return false }
        return channel.conversations.contains {
            $0.conversation.id == conversationID
                && $0.conversation.state == AgentConversationState.open.rawValue
                && $0.participant.conversationID == conversationID
                && $0.participant.seatID == channel.channel.binding.seat.id
        }
    }

    private func valid(
        detail: AgentConversationDetail,
        channelID: String,
        conversationID: String
    ) -> Bool {
        detail.channelID == channelID
            && detail.conversation.id == conversationID
            && detail.participant.conversationID == conversationID
            && detail.participant.seatID == detail.binding.seat.id
    }

    private static func describe(_ error: Error) -> String {
        if let coded = (error as? FortClientError)?.codedError {
            return coded.message
        }
        if let localized = error as? LocalizedError,
           let description = localized.errorDescription,
           !description.isEmpty {
            return description
        }
        return "Fort could not complete that Agent Channel request."
    }

    private func setPendingTurn(_ pending: AgentPendingTurn?, key: String? = nil) {
        let resolvedKey = key ?? pending?.key
        guard let resolvedKey else { return }
        pendingTurns[resolvedKey] = pending
        pendingTurnStore.save(pendingTurns)
    }
}
