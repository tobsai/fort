import Foundation
import FortKit

#if canImport(FoundationNetworking)
import FoundationNetworking
#endif

extension FortKitContractChecks {
    static func agentChannelWireModelsDecodeCanonicalFixtures() throws {
        let options = try JSONDecoder().decode(
            [AgentOption].self,
            from: Data(AgentFixtures.options.utf8)
        )
        agentExpect(options.count == 1, "Agent options did not decode as a top-level list")
        agentExpect(options[0].id == "option-openclaw", "agent_option_id did not remain opaque")
        agentExpect(options[0].binding.seat.agent == "openclaw", "provider-neutral agent identity changed")
        agentExpect(
            options[0].binding.authority.executionPolicy["workspace_policy"] == "selected_project",
            "provider-neutral execution_policy lost an adapter field"
        )

        let channel = try JSONDecoder().decode(
            AgentChannelSummary.self,
            from: Data(AgentFixtures.channelSummary.utf8)
        )
        agentExpect(channel.id == "agent-channel:v1:openclaw", "Agent Channel ID did not decode")
        agentExpect(channel.channel.optionID == "option-openclaw", "Agent Channel option_id did not decode")
        agentExpect(channel.conversations[0].conversation.id == "conversation-1", "nested Conversation ID did not decode")
        agentExpect(channel.conversations[0].pinned, "nested Conversation pin did not decode")
        agentExpect(channel.readiness.state == "ready", "Agent Channel readiness did not decode")

        let detail = try JSONDecoder().decode(
            AgentConversationDetail.self,
            from: Data(AgentFixtures.conversationDetail.utf8)
        )
        agentExpect(detail.channelID == channel.id, "agent_channel_id did not decode")
        agentExpect(detail.participant.seatID == options[0].binding.seat.id, "nested participant changed seats")
        agentExpect(detail.messages.first?.body == "Hello", "nested transcript did not decode")
        agentExpect(detail.binding == options[0].binding, "nested immutable binding changed")

        let first = try JSONDecoder().decode(
            AgentFirstTurnResult.self,
            from: Data(AgentFixtures.firstTurn.utf8)
        )
        agentExpect(first.conversation.conversation.id == first.turn.conversationID, "first turn escaped its new Conversation")
        agentExpect(first.targets.first?.participantID == detail.participant.id, "first turn target changed participant")

        let needs = try JSONDecoder().decode(
            [AgentNeedsYouItem].self,
            from: Data(AgentFixtures.needsYou.utf8)
        )
        agentExpect(needs.first?.agentChannel.id == channel.id, "Needs You lost its owning Agent Channel")
        agentExpect(needs.first?.conversation.id == detail.conversation.id, "Needs You lost its nested Conversation")
        agentExpect(needs.first?.recoveryActions == ["retry"], "Needs You recovery_actions changed")
    }

    static func clientUsesAgentChannelEndpointsAndClosedBodies() async throws {
        AgentStubURLProtocol.requests = []
        AgentStubURLProtocol.bodies = []
        AgentStubURLProtocol.handler = { request in
            let method = request.httpMethod ?? "GET"
            let path = request.url?.path ?? ""
            let json: String
            let status: Int
            switch (method, path) {
            case ("GET", "/api/agent-options"), ("POST", "/api/agent-options/recheck"):
                json = AgentFixtures.options
                status = 200
            case ("GET", "/api/agent-channels"):
                json = "[\(AgentFixtures.channelSummary)]"
                status = 200
            case ("POST", "/api/agent-channels"):
                json = AgentFixtures.channelSummary
                status = 201
            case ("GET", "/api/agent-channels/agent-channel:v1:openclaw"):
                json = AgentFixtures.channelSummary
                status = 200
            case ("PATCH", "/api/agent-channels/agent-channel:v1:openclaw"):
                json = ""
                status = 204
            case ("GET", "/api/agent-channels/agent-channel:v1:openclaw/conversations"):
                json = "[\(AgentFixtures.conversationSummary)]"
                status = 200
            case ("POST", "/api/agent-channels/agent-channel:v1:openclaw/conversations"):
                json = AgentFixtures.conversationDetail
                status = 201
            case ("GET", "/api/agent-channels/agent-channel:v1:openclaw/conversations/conversation-1"):
                json = AgentFixtures.conversationDetail
                status = 200
            case ("PATCH", "/api/agent-channels/agent-channel:v1:openclaw/conversations/conversation-1"),
                 ("POST", "/api/agent-channels/agent-channel:v1:openclaw/conversations/conversation-1/targets/target-1/cancel"):
                json = ""
                status = 204
            case ("POST", "/api/agent-channels/agent-channel:v1:openclaw/turns"):
                json = AgentFixtures.firstTurn
                status = 202
            case ("POST", "/api/agent-channels/agent-channel:v1:openclaw/conversations/conversation-1/turns"):
                json = AgentFixtures.turnResult
                status = 202
            case ("POST", "/api/agent-channels/agent-channel:v1:openclaw/conversations/conversation-1/targets/target-1/retry"):
                json = AgentFixtures.target
                status = 202
            case ("GET", "/api/agent-needs-you"):
                json = AgentFixtures.needsYou
                status = 200
            default:
                fatalError("unexpected Agent Channel request: \(method) \(path)")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: status, httpVersion: "HTTP/1.1",
                headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(json.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [AgentStubURLProtocol.self]
        let client = FortClient(
            baseURL: URL(string: "https://fort.test")!,
            session: URLSession(configuration: configuration)
        )
        let channelID = "agent-channel:v1:openclaw"
        let conversationID = "conversation-1"
        let turnID = "11111111-1111-4111-8111-111111111111"

        _ = try await client.agentOptions()
        _ = try await client.recheckAgentOptions()
        _ = try await client.agentChannels(state: .archived)
        _ = try await client.createAgentChannel(optionID: "option-openclaw", name: "OpenClaw — Studio")
        _ = try await client.agentChannel(id: channelID)
        try await client.updateAgentChannel(id: channelID, name: "OpenClaw")
        try await client.updateAgentChannel(id: channelID, state: .archived)
        _ = try await client.agentConversations(channelID: channelID, state: .all)
        _ = try await client.createAgentConversation(channelID: channelID, name: "Product direction")
        _ = try await client.agentConversation(channelID: channelID, conversationID: conversationID)
        try await client.updateAgentConversation(channelID: channelID, conversationID: conversationID, name: "Direction")
        try await client.updateAgentConversation(channelID: channelID, conversationID: conversationID, state: .archived)
        try await client.updateAgentConversation(channelID: channelID, conversationID: conversationID, pinned: true)
        _ = try await client.postFirstAgentTurn(
            channelID: channelID,
            name: "New conversation",
            clientTurnID: turnID,
            text: "Hello"
        )
        _ = try await client.postAgentTurn(
            channelID: channelID,
            conversationID: conversationID,
            clientTurnID: turnID,
            text: "Follow up"
        )
        _ = try await client.retryAgentTarget(
            channelID: channelID,
            conversationID: conversationID,
            targetID: "target-1"
        )
        try await client.cancelAgentTarget(
            channelID: channelID,
            conversationID: conversationID,
            targetID: "target-1"
        )
        _ = try await client.agentNeedsYou()

        let signatures = AgentStubURLProtocol.requests.map { request in
            let query = request.url?.query.map { "?\($0)" } ?? ""
            return "\(request.httpMethod ?? "") \(request.url?.path ?? "")\(query)"
        }
        agentExpect(signatures == [
            "GET /api/agent-options",
            "POST /api/agent-options/recheck",
            "GET /api/agent-channels?state=archived",
            "POST /api/agent-channels",
            "GET /api/agent-channels/agent-channel:v1:openclaw",
            "PATCH /api/agent-channels/agent-channel:v1:openclaw",
            "PATCH /api/agent-channels/agent-channel:v1:openclaw",
            "GET /api/agent-channels/agent-channel:v1:openclaw/conversations?state=all",
            "POST /api/agent-channels/agent-channel:v1:openclaw/conversations",
            "GET /api/agent-channels/agent-channel:v1:openclaw/conversations/conversation-1",
            "PATCH /api/agent-channels/agent-channel:v1:openclaw/conversations/conversation-1",
            "PATCH /api/agent-channels/agent-channel:v1:openclaw/conversations/conversation-1",
            "PATCH /api/agent-channels/agent-channel:v1:openclaw/conversations/conversation-1",
            "POST /api/agent-channels/agent-channel:v1:openclaw/turns",
            "POST /api/agent-channels/agent-channel:v1:openclaw/conversations/conversation-1/turns",
            "POST /api/agent-channels/agent-channel:v1:openclaw/conversations/conversation-1/targets/target-1/retry",
            "POST /api/agent-channels/agent-channel:v1:openclaw/conversations/conversation-1/targets/target-1/cancel",
            "GET /api/agent-needs-you",
        ], "Agent Channel endpoint surface drifted: \(signatures)")

        agentExpect(AgentStubURLProtocol.bodies[0] == nil, "Agent option GET encoded a body")
        agentExpect(AgentStubURLProtocol.bodies[1] == nil, "Agent option Recheck encoded a body")
        agentExpect(agentBody(at: 3) == ["agent_option_id": "option-openclaw", "name": "OpenClaw — Studio"], "Agent Channel creation accepted identity fields")
        agentExpect(agentBody(at: 5) == ["name": "OpenClaw"], "Agent Channel rename body drifted")
        agentExpect(agentBody(at: 6) == ["state": "archived"], "Agent Channel state body drifted")
        agentExpect(agentBody(at: 8) == ["name": "Product direction"], "Conversation creation body drifted")
        agentExpect(agentBody(at: 10) == ["name": "Direction"], "Conversation rename body drifted")
        agentExpect(agentBody(at: 11) == ["state": "archived"], "Conversation state body drifted")
        agentExpect(agentBody(at: 12) == ["pinned": true], "Conversation pin body drifted")
        agentExpect(agentBody(at: 13) == [
            "name": "New conversation", "client_turn_id": turnID, "text": "Hello",
        ], "atomic first-turn body drifted")
        agentExpect(agentBody(at: 14) == ["client_turn_id": turnID, "text": "Follow up"], "nested turn body drifted")
        agentExpect(AgentStubURLProtocol.bodies[15] == nil, "retry encoded a JSON body")
        agentExpect(AgentStubURLProtocol.bodies[16] == nil, "cancel encoded a JSON body")
    }

    static func agentChannelPatchesRequireExactlyOneMutableField() async throws {
        let client = FortClient(baseURL: URL(string: "https://fort.test")!)
        do {
            try await client.updateAgentChannel(id: "channel", name: "name", state: .archived)
            fatalError("Agent Channel accepted a multi-field patch")
        } catch AgentChannelRequestError.exactlyOneChangeRequired {
            // Expected.
        }
        do {
            try await client.updateAgentConversation(
                channelID: "channel", conversationID: "conversation", name: "name", pinned: true
            )
            fatalError("nested Conversation accepted a multi-field patch")
        } catch AgentConversationRequestError.exactlyOneChangeRequired {
            // Expected.
        }
    }

    static func agentConversationEventsDecodeStrictNestedSnapshots() async throws {
        AgentStubURLProtocol.requests = []
        AgentStubURLProtocol.bodies = []
        let updated = AgentFixtures.conversationDetail.replacingOccurrences(of: "Product direction", with: "Updated direction")
        AgentStubURLProtocol.handler = { request in
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200, httpVersion: "HTTP/1.1",
                headerFields: ["Content-Type": "text/event-stream"]
            )!
            return (response, Data("data: \(AgentFixtures.conversationDetail)\n\ndata: \(updated)\n\n".utf8))
        }
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [AgentStubURLProtocol.self]
        let client = FortClient(
            baseURL: URL(string: "https://fort.test")!,
            session: URLSession(configuration: configuration)
        )

        var titles: [String] = []
        for try await snapshot in client.agentConversationEvents(
            channelID: "agent-channel:v1:openclaw", conversationID: "conversation-1"
        ) {
            titles.append(snapshot.conversation.title)
        }
        agentExpect(titles == ["Product direction", "Updated direction"], "nested SSE did not replace snapshots")
        agentExpect(
            AgentStubURLProtocol.requests.first?.url?.path
                == "/api/agent-channels/agent-channel:v1:openclaw/conversations/conversation-1/events",
            "nested SSE used the wrong ownership-qualified path"
        )

        let frame = Data("data: \(AgentFixtures.conversationDetail)\n\n".utf8)
        let split = frame.count / 2
        let relay = AgentRelayStub(chunks: [Data(frame.prefix(split)), Data(frame.suffix(from: split))])
        let relayClient = FortClient(
            baseURL: URL(string: "https://fort.test")!,
            relayTransport: relay
        )
        var relayTitles: [String] = []
        for try await snapshot in relayClient.agentConversationEvents(
            channelID: "agent-channel:v1:openclaw", conversationID: "conversation-1"
        ) {
            relayTitles.append(snapshot.conversation.title)
        }
        agentExpect(relayTitles == ["Product direction"], "relay SSE did not decode the nested snapshot")
        agentExpect(
            relay.lastEventPath == "/api/agent-channels/agent-channel:v1:openclaw/conversations/conversation-1/events",
            "relay SSE used the wrong ownership-qualified path"
        )

        AgentStubURLProtocol.handler = { request in
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200, httpVersion: "HTTP/1.1",
                headerFields: ["Content-Type": "text/event-stream"]
            )!
            return (response, Data("data: {not-json}\n\n".utf8))
        }
        do {
            for try await _ in client.agentConversationEvents(
                channelID: "agent-channel:v1:openclaw", conversationID: "conversation-1"
            ) {}
            fatalError("malformed nested Agent Conversation snapshot was ignored")
        } catch is DecodingError {
            // Strict snapshots fail visibly instead of leaving stale native state.
        }
    }
}

private func agentExpect(_ condition: @autoclosure () -> Bool, _ message: String) {
    guard condition() else { fatalError(message) }
}

private func agentBody(at index: Int) -> [String: AnyHashable]? {
    guard let data = AgentStubURLProtocol.bodies[index],
          let object = try? JSONSerialization.jsonObject(with: data) as? [String: AnyHashable]
    else { return nil }
    return object
}

private final class AgentStubURLProtocol: URLProtocol {
    static var requests: [URLRequest] = []
    static var bodies: [Data?] = []
    static var handler: ((URLRequest) -> (HTTPURLResponse, Data))?

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        Self.requests.append(request)
        Self.bodies.append(Self.bodyData(for: request))
        guard let handler = Self.handler else { fatalError("AgentStubURLProtocol handler missing") }
        let (response, data) = handler(request)
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
        if !data.isEmpty { client?.urlProtocol(self, didLoad: data) }
        client?.urlProtocolDidFinishLoading(self)
    }

    override func stopLoading() {}

    private static func bodyData(for request: URLRequest) -> Data? {
        if let body = request.httpBody { return body.isEmpty ? nil : body }
        guard let stream = request.httpBodyStream else { return nil }
        stream.open()
        defer { stream.close() }
        var data = Data()
        var buffer = [UInt8](repeating: 0, count: 4_096)
        while stream.hasBytesAvailable {
            let count = stream.read(&buffer, maxLength: buffer.count)
            guard count > 0 else { break }
            data.append(buffer, count: count)
        }
        return data.isEmpty ? nil : data
    }
}

private final class AgentRelayStub: FortRelayTransporting, @unchecked Sendable {
    private let lock = NSLock()
    private let chunks: [Data]
    private var eventPath: String?

    init(chunks: [Data]) { self.chunks = chunks }

    var lastEventPath: String? {
        lock.lock()
        defer { lock.unlock() }
        return eventPath
    }

    func request(
        path: String,
        method: String,
        headers: [String: String]?,
        body: Data?,
        requestID: String
    ) async throws -> (data: Data, status: Int, requestID: String) {
        throw URLError(.unsupportedURL)
    }

    func events(path: String, requestID: String) -> AsyncThrowingStream<Data, Error> {
        lock.lock()
        eventPath = path
        lock.unlock()
        let chunks = self.chunks
        return AsyncThrowingStream { continuation in
            for chunk in chunks { continuation.yield(chunk) }
            continuation.finish()
        }
    }
}

private enum AgentFixtures {
    static let seat = #"{"id":"seat-openclaw","profile":"openclaw:personal","agent":"openclaw","model":"openclaw-local","machine":"studio"}"#
    static let authority = #"{"requested_model":"openclaw-local","resolved_model":"openclaw-local","authority":"openclaw_personal_v1","policy_id":"openclaw-personal-v1","policy_revision":"policy-r1","adapter_id":"model.chat.text-only.openclaw","adapter_revision":"adapter-r1","runtime_contract":"openclaw_exec_v1","session_mode":"durable","memory_mode":"agent_managed","execution_policy":{"workspace_policy":"selected_project","approval_policy":"ask"}}"#
    static let binding = #"{"seat":\#(seat),"authority":\#(authority)}"#
    static let channel = #"{"id":"agent-channel:v1:openclaw","name":"OpenClaw — Studio","state":"open","option_id":"option-openclaw","binding":\#(binding),"created_at":"2026-08-20T12:00:00Z"}"#
    static let conversation = #"{"id":"conversation-1","title":"Product direction","state":"open","created_at":"2026-08-20T12:01:00Z","updated_at":"2026-08-20T12:02:00Z"}"#
    static let participant = #"{"id":"participant-1","conversation_id":"conversation-1","seat_id":"seat-openclaw","profile":"openclaw:personal","agent":"openclaw","model":"openclaw-local","machine":"studio","display_name":"OpenClaw","position":0,"state":"active","created_at":"2026-08-20T12:01:00Z"}"#
    static let conversationSummary = #"{"conversation":\#(conversation),"participant":\#(participant),"pinned":true,"pinned_at":"2026-08-20T12:01:30Z"}"#
    static let readiness = #"{"state":"ready","observed_at":"2026-08-20T12:02:00Z"}"#
    static let turn = #"{"id":"turn-1","conversation_id":"conversation-1","client_turn_id":"11111111-1111-4111-8111-111111111111","prompt_message_id":1,"through_message_id":1,"created_at":"2026-08-20T12:02:00Z"}"#
    static let target = #"{"id":"target-1","turn_id":"turn-1","participant_id":"participant-1","run_id":"run-1","attempt":1,"state":"failed","error_code":"provider_failed","error":"provider failed","created_at":"2026-08-20T12:02:00Z","updated_at":"2026-08-20T12:03:00Z"}"#
    static let conversationDetail = #"{"agent_channel_id":"agent-channel:v1:openclaw","conversation":\#(conversation),"participant":\#(participant),"messages":[{"id":1,"conversation_id":"conversation-1","turn_id":"turn-1","author_kind":"human","author_id":"user","body":"Hello","created_at":"2026-08-20T12:02:00Z"}],"turns":[\#(turn)],"targets":[\#(target)],"readiness":\#(readiness),"binding":\#(binding),"pinned":true,"pinned_at":"2026-08-20T12:01:30Z"}"#
    static let channelSummary = #"{"channel":\#(channel),"conversations":[\#(conversationSummary)],"readiness":\#(readiness)}"#
    static let options = #"[{"agent_option_id":"option-openclaw","state":"ready","display_name":"OpenClaw on studio","binding":\#(binding)}]"#
    static let turnResult = #"{"turn":\#(turn),"targets":[\#(target)]}"#
    static let firstTurn = #"{"conversation":\#(conversationDetail),"turn":\#(turn),"targets":[\#(target)]}"#
    static let needsYou = #"[{"agent_channel":\#(channel),"conversation":\#(conversation),"target":\#(target),"recovery_actions":["retry"]}]"#
}
