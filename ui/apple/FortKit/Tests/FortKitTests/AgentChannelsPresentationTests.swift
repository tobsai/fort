import Foundation
import FortKit

extension FortKitContractChecks {
    static func agentChannelsActivationIsClosedAndOffByDefault() {
        agentPresentationExpect(
            AgentChannelsPresentationMode.resolve(rawValue: nil) == .off,
            "missing Agent Channels activation did not preserve Primary rollback"
        )
        agentPresentationExpect(
            AgentChannelsPresentationMode.resolve(rawValue: "off") == .off,
            "explicit off did not preserve Primary rollback"
        )
        agentPresentationExpect(
            AgentChannelsPresentationMode.resolve(rawValue: "primary") == .primary,
            "exact primary activation did not enable Agent Channels"
        )
        for invalid in ["on", "preview", "Primary", " primary", "primary "] {
            agentPresentationExpect(
                AgentChannelsPresentationMode.resolve(rawValue: invalid) == .off,
                "invalid activation \(invalid) enabled Agent Channels"
            )
        }
    }

    static func agentChannelsStartupRestoresOnlyOwnedOpenConversation() throws {
        let open = try agentChannelFixture(
            channelID: "agent-openclaw",
            conversationID: "conversation-open",
            conversationState: "open"
        )
        let other = try agentChannelFixture(
            channelID: "agent-claude",
            conversationID: "conversation-other",
            conversationState: "open"
        )
        let valid = AgentChannelsSelectionState(
            lastAgentID: open.id,
            lastConversationByAgent: [open.id: "conversation-open"]
        )
        agentPresentationExpect(
            AgentChannelsStartup.restore(valid, from: [open, other])
                == .conversation(channelID: open.id, conversationID: "conversation-open"),
            "startup did not restore the last owned open Conversation"
        )

        let foreign = AgentChannelsSelectionState(
            lastAgentID: open.id,
            lastConversationByAgent: [open.id: "conversation-other"]
        )
        agentPresentationExpect(
            AgentChannelsStartup.restore(foreign, from: [open, other]) == .agent(open.id),
            "startup opened a Conversation through the wrong Agent Channel"
        )

        let archived = try agentChannelFixture(
            channelID: open.id,
            conversationID: "conversation-archived",
            conversationState: "archived"
        )
        let archivedSelection = AgentChannelsSelectionState(
            lastAgentID: open.id,
            lastConversationByAgent: [open.id: "conversation-archived"]
        )
        agentPresentationExpect(
            AgentChannelsStartup.restore(archivedSelection, from: [archived]) == .agent(open.id),
            "startup reopened an archived Conversation"
        )

        let unavailable = AgentChannelsSelectionState(
            lastAgentID: "missing-agent",
            lastConversationByAgent: ["missing-agent": "conversation-open"]
        )
        agentPresentationExpect(
            AgentChannelsStartup.restore(unavailable, from: [open, other]) == .agents,
            "startup substituted an available Agent Channel for a missing saved agent"
        )
        agentPresentationExpect(
            AgentChannelsStartup.restore(.empty, from: [open, other]) == .agents,
            "startup selected an arbitrary agent without saved intent"
        )
    }

    static func agentChannelsSelectionPersistsPerAgentConversation() throws {
        let suite = "FortKitContractChecks.agent-selection.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        defer { defaults.removePersistentDomain(forName: suite) }
        let store = AgentChannelsSelectionStore(defaults: defaults)

        store.selectAgent("agent-openclaw")
        store.selectConversation("conversation-personal", for: "agent-openclaw")
        store.selectAgent("agent-claude")
        store.selectConversation("conversation-church", for: "agent-claude")

        let restored = AgentChannelsSelectionStore(defaults: defaults).load()
        agentPresentationExpect(restored.lastAgentID == "agent-claude", "last Agent Channel did not persist")
        agentPresentationExpect(
            restored.lastConversationByAgent["agent-openclaw"] == "conversation-personal",
            "OpenClaw's last Conversation was overwritten by another agent"
        )
        agentPresentationExpect(
            restored.lastConversationByAgent["agent-claude"] == "conversation-church",
            "Claude's last Conversation did not persist independently"
        )
    }

    @MainActor
    static func agentChannelsModelUsesAtomicFirstSendAndOwnedMutations() async throws {
        let suite = "FortKitContractChecks.agent-model.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        defer { defaults.removePersistentDomain(forName: suite) }
        let store = AgentChannelsSelectionStore(defaults: defaults)
        let emptyChannel = try agentChannelFixture(
            channelID: "agent-openclaw",
            conversationID: nil,
            conversationState: "open"
        )
        let first = try agentFirstTurnFixture(
            channelID: emptyChannel.id,
            conversationID: "conversation-new"
        )
        let service = AgentPresentationServiceFake(
            channels: [emptyChannel],
            firstTurn: first
        )
        let model = AgentChannelsModel(selectionStore: store)

        await model.reload(using: service, restoreStartup: true)
        agentPresentationExpect(model.destination == .agents, "model selected an arbitrary startup agent")

        await model.selectAgent(channelID: emptyChannel.id, using: service)
        agentPresentationExpect(model.destination == .agent(emptyChannel.id), "empty Agent Channel did not show first-send state")

        let accepted = await model.send(text: "  Start a separate product direction  ", using: service)
        agentPresentationExpect(accepted, "atomic first Send was not accepted")
        agentPresentationExpect(service.firstTurns.count == 1, "first Send did not use the atomic parent endpoint")
        agentPresentationExpect(service.nestedTurns.isEmpty, "first Send incorrectly targeted a nonexistent Conversation")
        agentPresentationExpect(service.firstTurns[0].channelID == emptyChannel.id, "first Send changed Agent Channel")
        agentPresentationExpect(service.firstTurns[0].text == "Start a separate product direction", "first Send changed normalized text")
        agentPresentationExpect(service.firstTurns[0].name == "Start a separate product direction", "first Send derived the wrong Conversation name")
        agentPresentationExpect(
            UUID(uuidString: service.firstTurns[0].clientTurnID) != nil,
            "first Send omitted its durable client-turn UUID"
        )
        agentPresentationExpect(
            model.destination == .conversation(
                channelID: emptyChannel.id,
                conversationID: first.conversation.conversation.id
            ),
            "first Send did not select its atomically created Conversation"
        )
        agentPresentationExpect(
            store.load().lastConversationByAgent[emptyChannel.id] == first.conversation.conversation.id,
            "first Send did not persist the new Conversation under its agent"
        )

        await model.renameSelectedConversation("Renamed direction", using: service)
        await model.setSelectedConversationPinned(true, using: service)
        agentPresentationExpect(
            service.conversationPatches.map(\.channelID) == [emptyChannel.id, emptyChannel.id],
            "nested mutations lost their owning Agent Channel"
        )
        agentPresentationExpect(
            service.conversationPatches.map(\.conversationID)
                == [first.conversation.conversation.id, first.conversation.conversation.id],
            "nested mutations changed Conversation identity"
        )
        agentPresentationExpect(service.conversationPatches[0].name == "Renamed direction", "rename body changed")
        agentPresentationExpect(service.conversationPatches[1].pinned == true, "pin body changed")
    }

    @MainActor
    static func agentPendingSendsPersistAndReuseIdempotencyKeys() async throws {
        let suite = "FortKitContractChecks.agent-pending.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        defer { defaults.removePersistentDomain(forName: suite) }
        let selectionStore = AgentChannelsSelectionStore(defaults: defaults)
        let pendingStore = AgentPendingTurnStore(defaults: defaults)
        let emptyChannel = try agentChannelFixture(
            channelID: "agent-openclaw",
            conversationID: nil,
            conversationState: "open"
        )
        let first = try agentFirstTurnFixture(
            channelID: emptyChannel.id,
            conversationID: "conversation-new"
        )
        let service = AgentPresentationServiceFake(channels: [emptyChannel], firstTurn: first)
        service.firstTurnFailures = [
            URLError(.networkConnectionLost),
            URLError(.timedOut),
            FortClientError.httpStatus(
                status: 409,
                body: #"{"code":"seat_unready","error":"not ready"}"#
            ),
        ]

        var model = AgentChannelsModel(
            selectionStore: selectionStore,
            pendingTurnStore: pendingStore
        )
        await model.reload(using: service, restoreStartup: true)
        await model.selectAgent(channelID: emptyChannel.id, using: service)
        agentPresentationExpect(model.pendingTurns.isEmpty, "pending store was not initially empty")
        let firstAccepted = await model.send(text: "Original first draft", using: service)
        agentPresentationExpect(
            firstAccepted == false,
            "ambiguous first Send was reported as accepted"
        )
        let firstID = service.firstTurns[0].clientTurnID
        agentPresentationExpect(
            pendingStore.load().values.first?.clientTurnID == firstID,
            "ambiguous atomic first Send did not persist its idempotency key"
        )

        model = AgentChannelsModel(
            selectionStore: selectionStore,
            pendingTurnStore: pendingStore
        )
        await model.reload(using: service, restoreStartup: true)
        let retryAccepted = await model.send(text: "Different draft must not replace it", using: service)
        agentPresentationExpect(
            retryAccepted == false,
            "second ambiguous first Send was reported as accepted"
        )
        agentPresentationExpect(
            service.firstTurns[1].clientTurnID == firstID
                && service.firstTurns[1].text == "Original first draft"
                && service.firstTurns[1].name == "Original first draft",
            "atomic first Send retry changed its persisted ID, text, or name"
        )
        _ = await model.send(text: "Still different", using: service)
        agentPresentationExpect(
            pendingStore.load().isEmpty,
            "terminal coded first-Send rejection retained a futile retry"
        )

        let nestedChannel = try agentChannelFixture(
            channelID: "agent-openclaw",
            conversationID: first.conversation.conversation.id,
            conversationState: "open"
        )
        let nestedService = AgentPresentationServiceFake(channels: [nestedChannel], firstTurn: first)
        nestedService.nestedTurnFailures = [
            URLError(.networkConnectionLost),
            URLError(.networkConnectionLost),
        ]
        let nestedModel = AgentChannelsModel(
            selectionStore: AgentChannelsSelectionStore(defaults: defaults),
            pendingTurnStore: pendingStore
        )
        await nestedModel.reload(using: nestedService, restoreStartup: false)
        await nestedModel.selectConversation(
            channelID: nestedChannel.id,
            conversationID: first.conversation.conversation.id,
            using: nestedService
        )
        _ = await nestedModel.send(text: "Original nested draft", using: nestedService)
        _ = await nestedModel.send(text: "Different nested draft", using: nestedService)
        agentPresentationExpect(
            nestedService.nestedTurns[0].clientTurnID == nestedService.nestedTurns[1].clientTurnID,
            "nested retry generated a second idempotency key"
        )
        agentPresentationExpect(
            nestedService.nestedTurns[1].text == "Original nested draft",
            "nested retry replaced the ambiguous draft"
        )

        let reconciledStore = AgentPendingTurnStore(
            defaults: defaults,
            key: "fort.agent-channels.pending-turns.reconciled"
        )
        let reconciledService = AgentPresentationServiceFake(channels: [nestedChannel], firstTurn: first)
        reconciledService.nestedTurnFailures = [URLError(.networkConnectionLost)]
        let authoritativeID = "22222222-2222-4222-8222-222222222222"
        reconciledStore.save([
            "seed": AgentPendingTurn(
                channelID: nestedChannel.id,
                conversationID: first.conversation.conversation.id,
                name: "",
                text: "Authoritative draft",
                clientTurnID: authoritativeID
            ),
        ])
        let reconciledModel = AgentChannelsModel(
            selectionStore: AgentChannelsSelectionStore(defaults: defaults),
            pendingTurnStore: reconciledStore
        )
        await reconciledModel.reload(using: reconciledService, restoreStartup: false)
        await reconciledModel.selectConversation(
            channelID: nestedChannel.id,
            conversationID: first.conversation.conversation.id,
            using: reconciledService
        )
        reconciledService.conversationDetailOverride = try agentConversationDetailFixture(
            channelID: nestedChannel.id,
            conversationID: first.conversation.conversation.id,
            clientTurnID: authoritativeID
        )
        let reconciled = await reconciledModel.send(
            text: "Authoritative draft",
            using: reconciledService
        )
        agentPresentationExpect(reconciled, "authoritative nested snapshot did not prove ambiguous Send success")
        agentPresentationExpect(
            reconciledStore.load().isEmpty,
            "authoritative turn retained an already-committed idempotency key"
        )
    }

    @MainActor
    static func agentArchivedConversationsRemainReopenable() async throws {
        let suite = "FortKitContractChecks.agent-archive.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        defer { defaults.removePersistentDomain(forName: suite) }
        let channelID = "agent-openclaw"
        let conversationID = "conversation-archived"
        let openChannel = try agentChannelFixture(
            channelID: channelID,
            conversationID: nil,
            conversationState: "open"
        )
        let archivedProjection = try agentChannelFixture(
            channelID: channelID,
            conversationID: conversationID,
            conversationState: "archived"
        ).conversations[0]
        let first = try agentFirstTurnFixture(
            channelID: channelID,
            conversationID: conversationID
        )
        let service = AgentPresentationServiceFake(channels: [openChannel], firstTurn: first)
        service.archivedConversations[channelID] = [archivedProjection]
        let model = AgentChannelsModel(
            selectionStore: AgentChannelsSelectionStore(defaults: defaults),
            pendingTurnStore: AgentPendingTurnStore(defaults: defaults)
        )

        await model.reload(using: service, restoreStartup: false)
        agentPresentationExpect(
            model.archivedConversationsByAgent[channelID]?.map(\.id) == [conversationID],
            "archived Conversation disappeared from its Agent Channel"
        )
        await model.reopenConversation(
            channelID: channelID,
            conversationID: conversationID,
            using: service
        )
        agentPresentationExpect(
            service.conversationPatches.last?.channelID == channelID
                && service.conversationPatches.last?.conversationID == conversationID
                && service.conversationPatches.last?.state == .open,
            "reopen lost the archived Conversation's exact parent identity"
        )
    }

    static func agentTargetRecoveryStaysClosedAndDurable() throws {
        let queued = try agentTargetFixture(state: "queued", errorCode: nil)
        let working = try agentTargetFixture(state: "working", errorCode: nil)
        let retryable = try agentTargetFixture(state: "failed", errorCode: "provider_failed")
        let drifted = try agentTargetFixture(state: "failed", errorCode: "primary_agent_drift")
        let unknown = try agentTargetFixture(state: "failed", errorCode: "future_failure")

        agentPresentationExpect(AgentTargetRecovery.action(for: queued) == .cancel, "Queued lost Cancel")
        agentPresentationExpect(AgentTargetRecovery.action(for: working) == .cancel, "Working lost Cancel")
        agentPresentationExpect(AgentTargetRecovery.action(for: retryable) == .retry, "retryable failure lost Retry")
        agentPresentationExpect(
            AgentTargetRecovery.action(for: drifted) == .recheckAndRetry,
            "readiness drift did not require Recheck before Retry"
        )
        agentPresentationExpect(AgentTargetRecovery.action(for: unknown) == nil, "unknown failure invented recovery")
        agentPresentationExpect(
            FortMarkActivity.fromDurableTargetState(working.state) == .working,
            "durable Working did not select stronger Fort product energy"
        )
        agentPresentationExpect(
            FortMarkActivity.fromDurableTargetState(queued.state) == .ambient,
            "Queued incorrectly selected Working product energy"
        )
    }

    static func agentIdentityInspectionKeepsExactAuthority() throws {
        let channel = try agentChannelFixture(
            channelID: "agent-openclaw",
            conversationID: nil,
            conversationState: "open"
        )
        let facts = AgentIdentityInspection(channel: channel)
        agentPresentationExpect(facts.seatID == "seat-1", "identity inspection omitted seat ID")
        agentPresentationExpect(facts.agent == "test-agent", "identity inspection changed agent")
        agentPresentationExpect(facts.profile == "test:personal", "identity inspection changed profile")
        agentPresentationExpect(facts.seatModel == "exact-model", "identity inspection omitted seat model")
        agentPresentationExpect(facts.requestedModel == "exact-model", "inspection omitted requested model")
        agentPresentationExpect(facts.resolvedModel == "exact-model", "inspection omitted resolved model")
        agentPresentationExpect(facts.machine == "studio", "inspection omitted computer")
        agentPresentationExpect(facts.adapterID == "adapter-1", "inspection omitted adapter ID")
        agentPresentationExpect(facts.adapterRevision == "adapter-r1", "inspection omitted adapter revision")
        agentPresentationExpect(facts.authority == "test_authority", "inspection omitted authority")
        agentPresentationExpect(facts.policyID == "policy-1", "inspection omitted policy ID")
        agentPresentationExpect(facts.policyRevision == "policy-r1", "inspection omitted policy revision")
        agentPresentationExpect(facts.sessionMode == "ephemeral", "inspection omitted session mode")
        agentPresentationExpect(facts.memoryMode == "ephemeral", "inspection omitted memory mode")
        agentPresentationExpect(facts.readiness == "ready", "inspection omitted current readiness")
    }

    static func agentChannelsNativeSurfaceKeepsIdentityHierarchy() throws {
        let root = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
        let source = try String(
            contentsOf: root.appendingPathComponent("Sources/FortKit/AgentChannelsView.swift"),
            encoding: .utf8
        )
        for required in [
            "public struct AgentChannelsView",
            "public struct FortNativeChatView",
            "FortProductMarkView",
            "AgentIdentityView",
            "PINNED CONVERSATIONS",
            "RECENT CONVERSATIONS",
            "ARCHIVED CONVERSATIONS",
            "Retry send",
            "New conversation",
            "Needs You",
            "Requested model",
            "Resolved model",
            "Session mode",
            "Memory mode",
        ] {
            agentPresentationExpect(source.contains(required), "native Agent Channels surface missing \(required)")
        }
        agentPresentationExpect(
            !source.contains("FortAgentOrbView"),
            "new Agent Channels surface reused the Fort mark as an agent avatar"
        )
        agentPresentationExpect(
            source.contains("case .off:") && source.contains("PrimaryChannelsView()"),
            "native product root lost its explicit Primary rollback branch"
        )
        agentPresentationExpect(
            source.contains("case .primary:") && source.contains("AgentChannelsView()"),
            "native product root lost its exact Agent Channels activation branch"
        )
    }
}

private func agentPresentationExpect(_ condition: @autoclosure () -> Bool, _ message: String) {
    guard condition() else { fatalError(message) }
}

private func agentChannelFixture(
    channelID: String,
    conversationID: String?,
    conversationState: String
) throws -> AgentChannelSummary {
    let seat = #"{"id":"seat-1","profile":"test:personal","agent":"test-agent","model":"exact-model","machine":"studio"}"#
    let authority = #"{"requested_model":"exact-model","resolved_model":"exact-model","authority":"test_authority","policy_id":"policy-1","policy_revision":"policy-r1","adapter_id":"adapter-1","adapter_revision":"adapter-r1","runtime_contract":"runtime-v1","session_mode":"ephemeral","memory_mode":"ephemeral","execution_policy":{"tools":"none"}}"#
    let binding = #"{"seat":\#(seat),"authority":\#(authority)}"#
    let conversations: String
    if let conversationID {
        let conversation = #"{"id":"\#(conversationID)","title":"Conversation","state":"\#(conversationState)","created_at":"2026-08-20T12:00:00Z","updated_at":"2026-08-20T12:00:00Z"}"#
        let participant = #"{"id":"participant-1","conversation_id":"\#(conversationID)","seat_id":"seat-1","profile":"test:personal","agent":"test-agent","model":"exact-model","machine":"studio","display_name":"Test Agent","position":0,"state":"active","created_at":"2026-08-20T12:00:00Z"}"#
        conversations = #"[{"conversation":\#(conversation),"participant":\#(participant),"pinned":false}]"#
    } else {
        conversations = "[]"
    }
    let json = #"{"channel":{"id":"\#(channelID)","name":"Test Agent","state":"open","option_id":"option-1","binding":\#(binding),"created_at":"2026-08-20T12:00:00Z"},"conversations":\#(conversations),"readiness":{"state":"ready","observed_at":"2026-08-20T12:00:00Z"}}"#
    return try JSONDecoder().decode(AgentChannelSummary.self, from: Data(json.utf8))
}

private func agentFirstTurnFixture(
    channelID: String,
    conversationID: String
) throws -> AgentFirstTurnResult {
    let detail = agentConversationDetailJSON(channelID: channelID, conversationID: conversationID)
    let turn = #"{"id":"turn-1","conversation_id":"\#(conversationID)","client_turn_id":"11111111-1111-4111-8111-111111111111","prompt_message_id":1,"through_message_id":1,"created_at":"2026-08-20T12:01:00Z"}"#
    let target = try agentTargetFixtureJSON(state: "queued", errorCode: nil)
    let json = #"{"conversation":\#(detail),"turn":\#(turn),"targets":[\#(target)]}"#
    return try JSONDecoder().decode(AgentFirstTurnResult.self, from: Data(json.utf8))
}

private func agentConversationDetailJSON(channelID: String, conversationID: String) -> String {
    let seat = #"{"id":"seat-1","profile":"test:personal","agent":"test-agent","model":"exact-model","machine":"studio"}"#
    let authority = #"{"requested_model":"exact-model","resolved_model":"exact-model","authority":"test_authority","policy_id":"policy-1","policy_revision":"policy-r1","adapter_id":"adapter-1","adapter_revision":"adapter-r1","runtime_contract":"runtime-v1","session_mode":"ephemeral","memory_mode":"ephemeral","execution_policy":{"tools":"none"}}"#
    let binding = #"{"seat":\#(seat),"authority":\#(authority)}"#
    let conversation = #"{"id":"\#(conversationID)","title":"Start a separate product direction","state":"open","created_at":"2026-08-20T12:01:00Z","updated_at":"2026-08-20T12:01:00Z"}"#
    let participant = #"{"id":"participant-1","conversation_id":"\#(conversationID)","seat_id":"seat-1","profile":"test:personal","agent":"test-agent","model":"exact-model","machine":"studio","display_name":"Test Agent","position":0,"state":"active","created_at":"2026-08-20T12:01:00Z"}"#
    return #"{"agent_channel_id":"\#(channelID)","conversation":\#(conversation),"participant":\#(participant),"messages":[],"turns":[],"targets":[],"readiness":{"state":"ready","observed_at":"2026-08-20T12:01:00Z"},"binding":\#(binding),"pinned":false}"#
}

private func agentConversationDetailFixture(
    channelID: String,
    conversationID: String,
    clientTurnID: String
) throws -> AgentConversationDetail {
    let turn = #"{"id":"turn-authoritative","conversation_id":"\#(conversationID)","client_turn_id":"\#(clientTurnID)","prompt_message_id":1,"through_message_id":1,"created_at":"2026-08-20T12:01:00Z"}"#
    let json = agentConversationDetailJSON(channelID: channelID, conversationID: conversationID)
        .replacingOccurrences(of: #""turns":[]"#, with: #""turns":[\#(turn)]"#)
    return try JSONDecoder().decode(AgentConversationDetail.self, from: Data(json.utf8))
}

private func agentTargetFixture(state: String, errorCode: String?) throws -> AgentTarget {
    let json = try agentTargetFixtureJSON(state: state, errorCode: errorCode)
    return try JSONDecoder().decode(AgentTarget.self, from: Data(json.utf8))
}

private func agentTargetFixtureJSON(state: String, errorCode: String?) throws -> String {
    let errorFields = errorCode.map { #", "error_code":"\#($0)","error":"failed""# } ?? ""
    return #"{"id":"target-1","turn_id":"turn-1","participant_id":"participant-1","run_id":"run-1","attempt":1,"state":"\#(state)"\#(errorFields),"created_at":"2026-08-20T12:01:00Z","updated_at":"2026-08-20T12:01:00Z"}"#
}

@MainActor
private final class AgentPresentationServiceFake: AgentChannelsServing {
    struct FirstTurnCall {
        let channelID: String
        let name: String
        let clientTurnID: String
        let text: String
    }

    struct NestedTurnCall {
        let channelID: String
        let conversationID: String
        let clientTurnID: String
        let text: String
    }

    struct ConversationPatch {
        let channelID: String
        let conversationID: String
        let name: String?
        let state: AgentConversationState?
        let pinned: Bool?
    }

    var channels: [AgentChannelSummary]
    let firstTurn: AgentFirstTurnResult
    var firstTurns: [FirstTurnCall] = []
    var nestedTurns: [NestedTurnCall] = []
    var conversationPatches: [ConversationPatch] = []
    var archivedConversations: [String: [AgentConversationSummary]] = [:]
    var firstTurnFailures: [Error] = []
    var nestedTurnFailures: [Error] = []
    var conversationDetailOverride: AgentConversationDetail?

    init(channels: [AgentChannelSummary], firstTurn: AgentFirstTurnResult) {
        self.channels = channels
        self.firstTurn = firstTurn
    }

    func agentChannels(state: AgentChannelFilter) async throws -> [AgentChannelSummary] {
        state == .open ? channels : []
    }

    func agentConversations(
        channelID: String,
        state: AgentConversationFilter
    ) async throws -> [AgentConversationSummary] {
        state == .archived ? archivedConversations[channelID] ?? [] : []
    }

    func postFirstAgentTurn(
        channelID: String,
        name: String,
        clientTurnID: String,
        text: String
    ) async throws -> AgentFirstTurnResult {
        firstTurns.append(FirstTurnCall(
            channelID: channelID,
            name: name,
            clientTurnID: clientTurnID,
            text: text
        ))
        if !firstTurnFailures.isEmpty { throw firstTurnFailures.removeFirst() }
        channels = [try agentChannelFixture(
            channelID: channelID,
            conversationID: firstTurn.conversation.conversation.id,
            conversationState: "open"
        )]
        return firstTurn
    }

    func agentConversation(
        channelID: String,
        conversationID: String
    ) async throws -> AgentConversationDetail {
        guard channelID == firstTurn.conversation.channelID,
              conversationID == firstTurn.conversation.conversation.id
        else { throw AgentChannelsServiceError.invalidProjection }
        return conversationDetailOverride ?? firstTurn.conversation
    }

    func postAgentTurn(
        channelID: String,
        conversationID: String,
        clientTurnID: String,
        text: String
    ) async throws -> AgentTurnResult {
        nestedTurns.append(NestedTurnCall(
            channelID: channelID,
            conversationID: conversationID,
            clientTurnID: clientTurnID,
            text: text
        ))
        if !nestedTurnFailures.isEmpty { throw nestedTurnFailures.removeFirst() }
        struct TurnEnvelope: Encodable {
            let turn: AgentTurn
            let targets: [AgentTarget]
        }
        let data = try JSONEncoder().encode(TurnEnvelope(
            turn: firstTurn.turn,
            targets: firstTurn.targets
        ))
        return try JSONDecoder().decode(AgentTurnResult.self, from: data)
    }

    func updateAgentConversation(
        channelID: String,
        conversationID: String,
        name: String?,
        state: AgentConversationState?,
        pinned: Bool?
    ) async throws {
        conversationPatches.append(ConversationPatch(
            channelID: channelID,
            conversationID: conversationID,
            name: name,
            state: state,
            pinned: pinned
        ))
        if state == .open {
            channels = [try agentChannelFixture(
                channelID: channelID,
                conversationID: conversationID,
                conversationState: "open"
            )]
            archivedConversations[channelID] = []
        }
    }
}
