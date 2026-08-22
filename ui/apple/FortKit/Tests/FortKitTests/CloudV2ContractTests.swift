import Foundation
import FortKit

#if canImport(FoundationNetworking)
import FoundationNetworking
#endif

extension FortKitContractChecks {
    static func cloudV2WireModelsKeepStableAgentAndGroupIdentity() throws {
        let agents = try JSONDecoder().decode(
            [FortCloudAgentRecord].self,
            from: Data(CloudV2Fixtures.agents.utf8)
        )
        cloudExpect(agents.count == 1, "cloud v2 Agent roster did not decode")
        cloudExpect(agents[0].id == "agent:researcher", "stable Agent id changed")
        cloudExpect(
            agents[0].agent.canonicalConversationID == "conversation:researcher:home",
            "stable Agent lost its canonical Home"
        )
        cloudExpect(agents[0].binding.provider == "openclaw", "exact framework binding disappeared")
        cloudExpect(agents[0].executionSource.framework == "openclaw", "Execution Source family disappeared")

        let groups = try JSONDecoder().decode(
            [FortCloudGroupRecord].self,
            from: Data(CloudV2Fixtures.groups.utf8)
        )
        cloudExpect(groups.first?.group.id == "group:launch", "stable Group id did not decode")
        cloudExpect(groups.first?.membership.members.map(\.agentID) == ["agent:researcher", "agent:builder"], "ordered Group membership changed")

        let groupProjection = try JSONDecoder().decode(
            FortCloudGroupProjection.self,
            from: Data(CloudV2Fixtures.groupProjection.utf8)
        )
        cloudExpect(groupProjection.group.id == "group:launch", "Group projection changed its stable id")
        cloudExpect(groupProjection.turns.first?.initialTargets.map(\.agentID) == ["agent:researcher", "agent:builder"], "Group projection changed its frozen wave")
        cloudExpect(groupProjection.turns.first?.initialTargets.first?.bindingRevisionID == "binding:researcher:1", "Group target lost exact Binding Revision evidence")
        cloudExpect(groupProjection.messages.map(\.body) == ["Build the launch plan.", "The evidence agrees."], "Group transcript did not reconstruct in durable order")
        cloudExpect(groupProjection.messages.last?.authorAgentID == "agent:researcher", "Group Agent response lost stable Agent attribution")

        let projection = try JSONDecoder().decode(
            FortCloudConversationProjection.self,
            from: Data(CloudV2Fixtures.projection.utf8)
        )
        cloudExpect(projection.conversation.link.kind == .canonical, "Home was not distinguished as canonical")
        cloudExpect(projection.messages.first?.body == "Compare the evidence.", "cloud transcript body did not decode")
        cloudExpect(projection.targets.first?.bindingRevisionID == "binding:researcher:1", "target lost exact Binding Revision evidence")
    }

    static func cloudV2ClientUsesBearerGatewayAndClosedCommands() async throws {
        CloudV2StubURLProtocol.requests = []
        CloudV2StubURLProtocol.bodies = []
        CloudV2StubURLProtocol.handler = { request in
            let method = request.httpMethod ?? "GET"
            let path = request.url?.path ?? ""
            let body: String
            switch (method, path) {
            case ("GET", "/api/v2/agents"):
                body = CloudV2Fixtures.agents
            case ("GET", "/api/v2/groups"):
                body = CloudV2Fixtures.groups
            case ("GET", "/api/v2/groups/group:launch"):
                body = CloudV2Fixtures.groupProjection
            case ("POST", "/api/v2/groups"):
                body = CloudV2Fixtures.groupRecord
            case ("POST", "/api/v2/groups/group:launch/turns"):
                body = CloudV2Fixtures.groupTurn
            case ("GET", "/api/v2/agents/agent:researcher/conversations"):
                body = "[\(CloudV2Fixtures.conversationRecord)]"
            case ("GET", "/api/v2/agents/agent:researcher/conversations/conversation:researcher:home"):
                body = CloudV2Fixtures.projection
            case ("POST", "/api/v2/agents/agent:researcher/conversations/conversation:researcher:home/turns"):
                body = CloudV2Fixtures.dispatch
            case ("POST", "/api/v2/agents/agent:researcher/conversations/conversation:researcher:home/targets/target:one/retry"):
                body = CloudV2Fixtures.target
            case ("POST", "/api/v2/agents/agent:researcher/conversations/conversation:researcher:home/targets/target:one/cancel"):
                body = CloudV2Fixtures.canceledTarget
            case ("POST", "/api/v2/agents/agent:researcher/conversations"):
                body = CloudV2Fixtures.secondaryConversationRecord
            case ("PATCH", "/api/v2/agents/agent:researcher/conversations/conversation:researcher:market"):
                body = CloudV2Fixtures.secondaryConversationRecord
            default:
                fatalError("unexpected cloud v2 request: \(method) \(path)")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: method == "POST" ? 202 : 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [CloudV2StubURLProtocol.self]
        let client = try FortCloudClient(
            gatewayURL: URL(string: "https://fort-gateway.test")!,
            bearerToken: "native-session-secret",
            session: URLSession(configuration: configuration)
        )
        let deadline = Date(timeIntervalSince1970: 1_787_331_600)

        _ = try await client.agents()
        _ = try await client.groups()
        _ = try await client.group(groupID: "group:launch")
        _ = try await client.createGroup(
            idempotencyKey: "group:one", title: "Product launch",
            agentIDs: ["agent:researcher", "agent:builder"]
        )
        _ = try await client.sendGroup(
            groupID: "group:launch", idempotencyKey: "group-turn:one",
            clientTurnID: "group-client:one", text: "Build the launch plan.",
            selection: .everyone,
            recipientAgentIDs: ["agent:researcher", "agent:builder"],
            concurrencyPolicy: .concurrent, hardDeadline: deadline
        )
        _ = try await client.agentConversations(agentID: "agent:researcher")
        _ = try await client.conversation(agentID: "agent:researcher", conversationID: "conversation:researcher:home")
        _ = try await client.send(
            agentID: "agent:researcher", conversationID: "conversation:researcher:home",
            idempotencyKey: "send:one", clientTurnID: "client-turn:one",
            text: "Compare the evidence.", hardDeadline: deadline
        )
        _ = try await client.retry(
            agentID: "agent:researcher", conversationID: "conversation:researcher:home",
            targetID: "target:one", idempotencyKey: "retry:one"
        )
        _ = try await client.cancel(
            agentID: "agent:researcher", conversationID: "conversation:researcher:home",
            targetID: "target:one", idempotencyKey: "cancel:one"
        )
        _ = try await client.createConversation(
            agentID: "agent:researcher", idempotencyKey: "conversation:create:one",
            title: "Market map"
        )
        _ = try await client.renameConversation(
            agentID: "agent:researcher", conversationID: "conversation:researcher:market",
            idempotencyKey: "conversation:rename:one", expectedTitle: "Market map",
            title: "Market landscape"
        )
        _ = try await client.setConversationPinned(
            agentID: "agent:researcher", conversationID: "conversation:researcher:market",
            idempotencyKey: "conversation:pin:one", pinned: true
        )
        _ = try await client.setConversationPinned(
            agentID: "agent:researcher", conversationID: "conversation:researcher:market",
            idempotencyKey: "conversation:unpin:one", pinned: false
        )
        _ = try await client.setConversationArchived(
            agentID: "agent:researcher", conversationID: "conversation:researcher:market",
            idempotencyKey: "conversation:archive:one", archived: true
        )
        _ = try await client.setConversationArchived(
            agentID: "agent:researcher", conversationID: "conversation:researcher:market",
            idempotencyKey: "conversation:reopen:one", archived: false
        )

        let signatures = CloudV2StubURLProtocol.requests.map { request in
            let query = request.url?.query.map { "?\($0)" } ?? ""
            return "\(request.httpMethod ?? "") \(request.url?.path ?? "")\(query)"
        }
        cloudExpect(signatures == [
            "GET /api/v2/agents?state=open",
            "GET /api/v2/groups?state=open",
            "GET /api/v2/groups/group:launch",
            "POST /api/v2/groups",
            "POST /api/v2/groups/group:launch/turns",
            "GET /api/v2/agents/agent:researcher/conversations",
            "GET /api/v2/agents/agent:researcher/conversations/conversation:researcher:home",
            "POST /api/v2/agents/agent:researcher/conversations/conversation:researcher:home/turns",
            "POST /api/v2/agents/agent:researcher/conversations/conversation:researcher:home/targets/target:one/retry",
            "POST /api/v2/agents/agent:researcher/conversations/conversation:researcher:home/targets/target:one/cancel",
            "POST /api/v2/agents/agent:researcher/conversations",
            "PATCH /api/v2/agents/agent:researcher/conversations/conversation:researcher:market",
            "PATCH /api/v2/agents/agent:researcher/conversations/conversation:researcher:market",
            "PATCH /api/v2/agents/agent:researcher/conversations/conversation:researcher:market",
            "PATCH /api/v2/agents/agent:researcher/conversations/conversation:researcher:market",
            "PATCH /api/v2/agents/agent:researcher/conversations/conversation:researcher:market",
        ], "cloud v2 endpoint surface drifted: \(signatures)")
        for request in CloudV2StubURLProtocol.requests {
            cloudExpect(
                request.value(forHTTPHeaderField: "Authorization") == "Bearer native-session-secret",
                "cloud v2 request omitted the renewable native bearer"
            )
            cloudExpect(request.value(forHTTPHeaderField: "X-Fort-Account-ID") == nil, "native client forged account identity")
            cloudExpect(request.value(forHTTPHeaderField: "X-Fort-Machine-ID") == nil, "owner command selected a machine")
        }
        let createGroup = cloudBody(at: 3)
        cloudExpect(createGroup?["idempotency_key"] as? String == "group:one", "Group create lost its idempotency key")
        cloudExpect(createGroup?["title"] as? String == "Product launch", "Group create lost its title")
        cloudExpect(createGroup?["agent_ids"] as? [String] == ["agent:researcher", "agent:builder"], "Group create changed stable Agent ids")
        let groupSend = cloudBody(at: 4)
        cloudExpect(groupSend?["selection"] as? String == "everyone", "Group Send lost explicit selection")
        cloudExpect(groupSend?["recipient_agent_ids"] as? [String] == ["agent:researcher", "agent:builder"], "Group Send changed ordered recipients")
        cloudExpect(groupSend?["concurrency_policy"] as? String == "concurrent", "Group Send lost its concurrency policy")
        let send = cloudBody(at: 7)
        cloudExpect(send?["idempotency_key"] as? String == "send:one", "Send lost its idempotency key")
        cloudExpect(send?["client_turn_id"] as? String == "client-turn:one", "Send lost its client turn id")
        cloudExpect(send?["text"] as? String == "Compare the evidence.", "Send lost its text")
        cloudExpect(send?["hard_deadline"] != nil, "Send lost its hard deadline")
        for forbidden in ["account_id", "provider", "model", "machine_id", "binding_revision_id", "behavior_revision_id"] {
            cloudExpect(send?[forbidden] == nil, "Send exposed client-selectable \(forbidden)")
        }
        for forbidden in ["account_id", "provider", "model", "machine_id", "binding_revision_id", "behavior_revision_id"] {
            cloudExpect(createGroup?[forbidden] == nil, "Group create exposed client-selectable \(forbidden)")
            cloudExpect(groupSend?[forbidden] == nil, "Group Send exposed client-selectable \(forbidden)")
        }
        cloudExpect(cloudBody(at: 8) == ["idempotency_key": "retry:one"], "retry accepted fields beyond its idempotency key")
        cloudExpect(cloudBody(at: 9) == ["idempotency_key": "cancel:one"], "cancel accepted fields beyond its idempotency key")
        cloudExpect(cloudBody(at: 10) == ["idempotency_key": "conversation:create:one", "title": "Market map"], "Conversation create accepted fields beyond title")
        cloudExpect(cloudBody(at: 11) == [
            "idempotency_key": "conversation:rename:one", "action": "rename",
            "expected_title": "Market map", "title": "Market landscape",
        ], "Conversation rename changed its closed request")
        cloudExpect(cloudBody(at: 12) == ["idempotency_key": "conversation:pin:one", "action": "pin"], "pin changed its closed request")
        cloudExpect(cloudBody(at: 13) == ["idempotency_key": "conversation:unpin:one", "action": "unpin"], "unpin changed its closed request")
        cloudExpect(cloudBody(at: 14) == ["idempotency_key": "conversation:archive:one", "action": "archive"], "archive changed its closed request")
        cloudExpect(cloudBody(at: 15) == ["idempotency_key": "conversation:reopen:one", "action": "reopen"], "reopen changed its closed request")
    }

    static func cloudV2HandoffWireModelsEnforceDurableLinkage() throws {
        let record: FortCloudHandoffRecord
        do {
            record = try JSONDecoder().decode(
                FortCloudHandoffRecord.self,
                from: Data(CloudV2Fixtures.completedHandoff.utf8)
            )
        } catch {
            fatalError("canonical Handoff projection did not decode: \(error)")
        }
        cloudExpect(record.id == "handoff:one", "Handoff changed its stable id")
        cloudExpect(record.handoff.state == .completed, "Handoff lifecycle did not decode")
        cloudExpect(record.handoff.groupTurnID == "group-turn:one", "Handoff lost its source Group Turn")
        cloudExpect(record.target.agentID == "agent:builder", "Handoff target lost its stable Agent")
        cloudExpect(record.target.bindingRevisionID == "binding:builder:1", "Handoff target lost its exact Binding Revision")
        cloudExpect(record.result?.messageID == "43", "Handoff lost its one authoritative result")

        let longRequest = String(repeating: "x", count: 1_024)
        let longRequestRecord = CloudV2Fixtures.completedHandoff.replacingOccurrences(
            of: "Verify the release evidence.", with: longRequest
        )
        _ = try JSONDecoder().decode(FortCloudHandoffRecord.self, from: Data(longRequestRecord.utf8))

        let wrongGroupOutput = CloudV2Fixtures.completedHandoff
            .replacingOccurrences(
                of: #""output_conversation_id":"conversation:launch"#,
                with: #""output_conversation_id":"conversation:other"#
            )
            .replacingOccurrences(
                of: #""conversation_id":"conversation:launch","agent_id":"agent:builder"#,
                with: #""conversation_id":"conversation:other","agent_id":"agent:builder"#
            )
        for invalid in [
            CloudV2Fixtures.completedHandoff.replacingOccurrences(
                of: #""max_agent_messages":10"#,
                with: #""max_agent_messages":9"#
            ),
            CloudV2Fixtures.completedHandoff.replacingOccurrences(
                of: #""target":{"id":"target:one","handoff_id":"handoff:one"#,
                with: #""target":{"id":"target:one","handoff_id":"handoff:other"#
            ),
            CloudV2Fixtures.completedHandoff.replacingOccurrences(
                of: #""result":{"handoff_id":"handoff:one","output_conversation_id":"conversation:launch"#,
                with: #""result":{"handoff_id":"handoff:one","output_conversation_id":"conversation:other"#
            ),
            wrongGroupOutput,
            CloudV2Fixtures.completedHandoff.replacingOccurrences(
                of: #""depth":1,"deadline"#,
                with: #""depth":2,"deadline"#
            ),
            CloudV2Fixtures.completedHandoff.replacingOccurrences(
                of: #""max_depth":3"#,
                with: #""max_depth":2"#
            ),
            CloudV2Fixtures.completedHandoff.replacingOccurrences(
                of: #""deadline":"2026-08-21T16:30:00Z"#,
                with: #""deadline":"not-a-time"#
            ),
            CloudV2Fixtures.completedHandoff.replacingOccurrences(
                of: #""deadline":"2026-08-21T16:30:00Z"#,
                with: #""deadline":"2026-08-21T16:20:00Z"#
            ),
            CloudV2Fixtures.completedHandoff.replacingOccurrences(
                of: #""state":"completed","created_by_kind"#,
                with: #""state":"invented","created_by_kind"#
            ),
        ] {
            do {
                _ = try JSONDecoder().decode(FortCloudHandoffRecord.self, from: Data(invalid.utf8))
                fatalError("invalid Handoff projection unexpectedly decoded")
            } catch {
                // Expected.
            }
        }
    }

    static func cloudV2HandoffClientUsesClosedHumanCommands() async throws {
        CloudV2StubURLProtocol.requests = []
        CloudV2StubURLProtocol.bodies = []
        CloudV2StubURLProtocol.handler = { request in
            let response = HTTPURLResponse(
                url: request.url!, statusCode: request.httpMethod == "POST" ? 202 : 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            let body = request.httpMethod == "GET" && request.url?.path == "/api/v2/handoffs"
                ? "[\(CloudV2Fixtures.completedHandoff)]"
                : CloudV2Fixtures.completedHandoff
            return (response, Data(body.utf8))
        }
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [CloudV2StubURLProtocol.self]
        let client = try FortCloudClient(
            gatewayURL: URL(string: "https://fort-gateway.test")!,
            bearerToken: "native-session-secret",
            session: URLSession(configuration: configuration)
        )

        _ = try await client.handoffs()
        _ = try await client.handoff(handoffID: "handoff:one")
        _ = try await client.createHandoff(
            idempotencyKey: "handoff:create:one",
            sourceConversationID: "conversation:launch",
            sourceMessageID: "41",
            recipientAgentID: "agent:builder",
            contextMessageIDs: ["40", "41"],
            requestedResult: "Verify the release evidence.",
            replyToMessageID: "40",
            hardDeadline: Date(timeIntervalSince1970: 1_787_329_800)
        )
        _ = try await client.cancelHandoff(
            handoffID: "handoff:one",
            idempotencyKey: "handoff:cancel:one"
        )

        let signatures = CloudV2StubURLProtocol.requests.map {
            "\($0.httpMethod ?? "") \($0.url?.path ?? "")"
        }
        cloudExpect(signatures == [
            "GET /api/v2/handoffs",
            "GET /api/v2/handoffs/handoff:one",
            "POST /api/v2/handoffs",
            "POST /api/v2/handoffs/handoff:one/cancel",
        ], "cloud Handoff endpoint surface drifted: \(signatures)")

        let create = cloudBody(at: 2)
        cloudExpect(create?["idempotency_key"] as? String == "handoff:create:one", "Handoff create lost its idempotency key")
        cloudExpect(create?["source_conversation_id"] as? String == "conversation:launch", "Handoff create lost its exact source Conversation")
        cloudExpect(create?["source_message_id"] as? String == "41", "Handoff create lost its exact source message")
        cloudExpect(create?["recipient_agent_id"] as? String == "agent:builder", "Handoff create lost its selected stable Agent")
        cloudExpect(create?["context_message_ids"] as? [String] == ["40", "41"], "Handoff create changed its explicit context")
        cloudExpect(create?["requested_result"] as? String == "Verify the release evidence.", "Handoff create lost its requested result")
        cloudExpect(create?["reply_to_message_id"] as? String == "40", "Handoff create lost its reply linkage")
        cloudExpect(create?["hard_deadline"] != nil, "Handoff create lost its hard deadline")
        cloudExpect(Set(create?.keys.map { $0 } ?? []) == Set([
            "idempotency_key", "source_conversation_id", "source_message_id", "recipient_agent_id",
            "context_message_ids", "requested_result", "reply_to_message_id", "hard_deadline",
        ]), "Handoff create exposed fields beyond human intent")
        for forbidden in ["account_id", "provider", "model", "machine_id", "binding_revision_id", "behavior_revision_id", "authority"] {
            cloudExpect(create?[forbidden] == nil, "Handoff create exposed client-selectable \(forbidden)")
        }
        cloudExpect(cloudBody(at: 3) == ["idempotency_key": "handoff:cancel:one"], "Handoff cancel accepted fields beyond its idempotency key")
    }

    static func cloudV2RoutineWireModelsEnforceExactOwnerAndPins() throws {
        let records = try JSONDecoder().decode(
            [FortCloudRoutineRecord].self,
            from: Data("[\(CloudV2Fixtures.routineRecord)]".utf8)
        )
        let record = records[0]
        cloudExpect(record.id == "routine:weekly", "Routine changed its stable id")
        cloudExpect(record.routine.agentID == "agent:researcher", "Routine lost its exact Agent owner")
        cloudExpect(record.currentRevision.authority == .fortCloud, "Routine lost fort_cloud authority evidence")
        cloudExpect(record.currentRevision.behaviorRevisionID == "behavior:researcher:1", "Routine lost its Behavior Revision pin")
        cloudExpect(record.currentRevision.bindingRevisionID == "binding:researcher:1", "Routine lost its Binding Revision pin")
        cloudExpect(record.currentRevision.resultConversationID == "conversation:researcher:home", "Routine lost its result Conversation")

        let run = try JSONDecoder().decode(
            FortCloudRoutineRunRecord.self,
            from: Data(CloudV2Fixtures.routineRun.utf8)
        )
        cloudExpect(run.run.kind == .test, "Test Routine did not decode as a real test occurrence")
        cloudExpect(run.run.behaviorRevisionID == "behavior:researcher:1", "Routine run lost exact Behavior evidence")
        cloudExpect(run.run.bindingRevisionID == "binding:researcher:1", "Routine run lost exact Binding evidence")
        cloudExpect(run.resultConversationID == "conversation:researcher:home", "Routine run changed its result Conversation")
        cloudExpect(run.activities.first?.sequence == 41, "Routine activity lost its global ledger sequence")

        for invalid in [
            CloudV2Fixtures.routineRecord.replacingOccurrences(of: #""authority":"fort_cloud""#, with: #""authority":"source_native""#),
            CloudV2Fixtures.routineRecord.replacingOccurrences(of: #""agent_id":"agent:researcher","current_revision_id""#, with: #""agent_id":"agent:other","current_revision_id""#),
            CloudV2Fixtures.routineRecord.replacingOccurrences(of: #""current_revision_id":"routine-revision:weekly:1""#, with: #""current_revision_id":"routine-revision:other""#),
            CloudV2Fixtures.routineRecord.replacingOccurrences(of: #""next_occurrence":"2026-08-24T14:00:00Z""#, with: #""next_occurrence":"not-a-time""#),
        ] {
            do {
                _ = try JSONDecoder().decode(FortCloudRoutineRecord.self, from: Data(invalid.utf8))
                fatalError("invalid Routine projection unexpectedly decoded")
            } catch {
                // Expected.
            }
        }

        let mismatchedRun = CloudV2Fixtures.routineRun.replacingOccurrences(
            of: #""occurrence_id":"routine-occurrence:test:one""#,
            with: #""occurrence_id":"routine-occurrence:other""#
        )
        do {
            _ = try JSONDecoder().decode(FortCloudRoutineRunRecord.self, from: Data(mismatchedRun.utf8))
            fatalError("mismatched Routine run unexpectedly decoded")
        } catch {
            // Expected.
        }
    }

    static func cloudV2RoutineClientUsesClosedAgentOwnedCommands() async throws {
        CloudV2StubURLProtocol.requests = []
        CloudV2StubURLProtocol.bodies = []
        CloudV2StubURLProtocol.handler = { request in
            let method = request.httpMethod ?? "GET"
            let path = request.url?.path ?? ""
            let body: String
            switch (method, path) {
            case ("GET", "/api/v2/agents/agent:researcher/routines"):
                body = "[\(CloudV2Fixtures.routineRecord)]"
            case ("POST", "/api/v2/agents/agent:researcher/routines"):
                body = CloudV2Fixtures.routineRecord
            case ("PATCH", "/api/v2/agents/agent:researcher/routines/routine:weekly"):
                body = CloudV2Fixtures.revalidatedRoutineRecord
            case ("POST", "/api/v2/agents/agent:researcher/routines/routine:weekly/test"):
                body = CloudV2Fixtures.routineRun
            default:
                fatalError("unexpected cloud Routine request: \(method) \(path)")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: method == "POST" ? 202 : 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [CloudV2StubURLProtocol.self]
        let client = try FortCloudClient(
            gatewayURL: URL(string: "https://fort-gateway.test")!,
            bearerToken: "native-session-secret",
            session: URLSession(configuration: configuration)
        )

        _ = try await client.routines(agentID: "agent:researcher")
        _ = try await client.createRoutine(
            agentID: "agent:researcher",
            idempotencyKey: "routine:create:weekly",
            trigger: .schedule,
            schedule: "0 9 * * 1",
            timezone: "America/Chicago",
            nextOccurrence: Date(timeIntervalSince1970: 1_787_580_000),
            inputSource: "fort:conversation:research",
            freshnessSeconds: 86_400,
            expectedResult: "Weekly brief",
            resultConversationID: "conversation:researcher:home",
            approvalBoundary: "before_external_side_effect",
            missingInputBehavior: .needsYou,
            retryPolicy: "once",
            catchUpPolicy: "skip",
            latenessPolicy: "within_1h"
        )
        _ = try await client.revalidateRoutine(
            agentID: "agent:researcher", routineID: "routine:weekly",
            idempotencyKey: "routine:revalidate:two"
        )
        _ = try await client.testRoutine(
            agentID: "agent:researcher", routineID: "routine:weekly",
            idempotencyKey: "routine:test:one"
        )

        let signatures = CloudV2StubURLProtocol.requests.map {
            "\($0.httpMethod ?? "") \($0.url?.path ?? "")"
        }
        cloudExpect(signatures == [
            "GET /api/v2/agents/agent:researcher/routines",
            "POST /api/v2/agents/agent:researcher/routines",
            "PATCH /api/v2/agents/agent:researcher/routines/routine:weekly",
            "POST /api/v2/agents/agent:researcher/routines/routine:weekly/test",
        ], "cloud Routine endpoint surface drifted: \(signatures)")

        let create = cloudBody(at: 1)
        cloudExpect(create? ["idempotency_key"] as? String == "routine:create:weekly", "Routine create lost idempotency")
        cloudExpect(create? ["trigger"] as? String == "schedule", "Routine create lost its trigger")
        cloudExpect(create? ["result_conversation_id"] as? String == "conversation:researcher:home", "Routine create changed its result Conversation")
        cloudExpect(Set(create?.keys.map { $0 } ?? []) == Set([
            "idempotency_key", "trigger", "schedule", "timezone", "next_occurrence", "input_source",
            "freshness_seconds", "expected_result", "result_conversation_id", "approval_boundary",
            "missing_input_behavior", "retry_policy", "catch_up_policy", "lateness_policy",
        ]), "Routine create exposed fields beyond approved Routine semantics")
        for forbidden in ["account_id", "provider", "model", "machine_id", "adapter_id", "authority", "behavior_revision_id", "binding_revision_id"] {
            cloudExpect(create?[forbidden] == nil, "Routine create exposed client-selectable \(forbidden)")
        }
        cloudExpect(cloudBody(at: 2) == [
            "idempotency_key": "routine:revalidate:two", "action": "revalidate",
        ], "Routine revalidation changed its closed request")
        cloudExpect(cloudBody(at: 3) == ["idempotency_key": "routine:test:one"], "Test Routine accepted fields beyond idempotency")
    }
}

private func cloudExpect(_ condition: @autoclosure () -> Bool, _ message: String) {
    guard condition() else { fatalError(message) }
}

private func cloudBody(at index: Int) -> [String: AnyHashable]? {
    guard let data = CloudV2StubURLProtocol.bodies[index],
          let object = try? JSONSerialization.jsonObject(with: data) as? [String: AnyHashable]
    else { return nil }
    return object
}

private final class CloudV2StubURLProtocol: URLProtocol {
    static var requests: [URLRequest] = []
    static var bodies: [Data?] = []
    static var handler: ((URLRequest) -> (HTTPURLResponse, Data))?

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        Self.requests.append(request)
        Self.bodies.append(Self.bodyData(for: request))
        guard let handler = Self.handler else { fatalError("CloudV2StubURLProtocol handler missing") }
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

private enum CloudV2Fixtures {
    static let agent = #"{"id":"agent:researcher","account_id":"4af424a4-d81a-47d5-a495-400868883b86","state":"open","canonical_conversation_id":"conversation:researcher:home"}"#
    static let profile = #"{"id":"profile:researcher:1","agent_id":"agent:researcher","name":"Researcher","title":"Evidence and synthesis","pinned":true}"#
    static let binding = #"{"id":"binding:researcher:1","agent_id":"agent:researcher","behavior_revision_id":"behavior:researcher:1","provider":"openclaw","requested_model":"main","resolved_model":"main","computer_id":"worker:studio","adapter_id":"openclaw.chat","adapter_revision":"1"}"#
    static let source = #"{"id":"source:studio:openclaw","framework":"openclaw","display_name":"OpenClaw · Studio"}"#
    static let home = #"{"id":"conversation:researcher:home","title":"Home","state":"open"}"#
    static let agents = #"[{"agent":\#(agent),"profile":\#(profile),"binding":\#(binding),"execution_source":\#(source),"home":\#(home)}]"#
    static let conversationRecord = #"{"conversation":\#(home),"link":{"agent_id":"agent:researcher","conversation_id":"conversation:researcher:home","kind":"canonical"},"pinned":false}"#
    static let secondaryConversationRecord = #"{"conversation":{"id":"conversation:researcher:market","title":"Market map","state":"open"},"link":{"agent_id":"agent:researcher","conversation_id":"conversation:researcher:market","kind":"secondary"},"pinned":false}"#
    static let target = #"{"id":"target:one","turn_id":"turn:one","conversation_id":"conversation:researcher:home","agent_id":"agent:researcher","behavior_revision_id":"behavior:researcher:1","binding_revision_id":"binding:researcher:1","participant_id":"participant:researcher:1","run_id":"run:one","state":"queued","attempt_count":0,"created_at":"2026-08-21T16:20:00Z","updated_at":"2026-08-21T16:20:00Z"}"#
    static let canceledTarget = #"{"id":"target:one","turn_id":"turn:one","conversation_id":"conversation:researcher:home","agent_id":"agent:researcher","behavior_revision_id":"behavior:researcher:1","binding_revision_id":"binding:researcher:1","participant_id":"participant:researcher:1","run_id":"run:one","state":"canceled","attempt_count":0,"created_at":"2026-08-21T16:20:00Z","updated_at":"2026-08-21T16:21:00Z"}"#
    static let projection = #"{"conversation":\#(conversationRecord),"messages":[{"id":1,"conversation_id":"conversation:researcher:home","turn_id":"turn:one","author_kind":"human","author_id":"human:owner","body":"Compare the evidence.","created_at":"2026-08-21T16:20:00Z"}],"turns":[{"id":"turn:one","conversation_id":"conversation:researcher:home","client_turn_id":"client-turn:one","prompt_message_id":1,"through_message_id":1,"membership_revision_id":"membership:home:1","context_manifest_id":"context:one","state":"open","created_at":"2026-08-21T16:20:00Z"}],"targets":[\#(target)]}"#
    static let dispatch = #"{"message":{"id":1,"conversation_id":"conversation:researcher:home","turn_id":"turn:one","author_kind":"human","author_id":"human:owner","body":"Compare the evidence.","created_at":"2026-08-21T16:20:00Z"},"turn":{"id":"turn:one","conversation_id":"conversation:researcher:home","client_turn_id":"client-turn:one","prompt_message_id":1,"through_message_id":1,"membership_revision_id":"membership:home:1","context_manifest_id":"context:one","state":"open","created_at":"2026-08-21T16:20:00Z"},"context":{"id":"context:one","conversation_id":"conversation:researcher:home","through_message_id":1,"message_ids":[1],"digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","created_at":"2026-08-21T16:20:00Z"},"target":\#(target),"created":true}"#
    static let groups = #"[{"group":{"id":"group:launch","account_id":"4af424a4-d81a-47d5-a495-400868883b86","conversation_id":"conversation:launch","state":"open","current_membership_revision_id":"membership:launch:1","created_at":"2026-08-21T16:00:00Z"},"conversation":{"id":"conversation:launch","title":"Product launch","state":"open"},"membership":{"id":"membership:launch:1","group_id":"group:launch","revision":1,"members":[{"agent_id":"agent:researcher","position":0},{"agent_id":"agent:builder","position":1}]},"member_bindings":[{"agent_id":"agent:researcher","behavior_revision_id":"behavior:researcher:1","binding_revision_id":"binding:researcher:1","participant_id":"participant:researcher:group"},{"agent_id":"agent:builder","behavior_revision_id":"behavior:builder:1","binding_revision_id":"binding:builder:1","participant_id":"participant:builder:group"}]}]"#
    static let groupRecord = String(groups.dropFirst().dropLast())
    static let groupTurn = #"{"message":{"id":2,"conversation_id":"conversation:launch","turn_id":"group-turn:one","author_kind":"human","author_id":"human:owner","body":"Build the launch plan.","created_at":"2026-08-21T16:20:00Z"},"envelope":{"id":"group-turn:one","group_id":"group:launch","conversation_id":"conversation:launch","client_turn_id":"group-client:one","membership_revision_id":"membership:launch:1","selection":"everyone","concurrency_policy":"concurrent","max_agent_messages":10,"max_handoff_depth":3,"deadline":"2026-08-21T16:30:00Z","created_at":"2026-08-21T16:20:00Z"},"recipients":[{"agent_id":"agent:researcher","behavior_revision_id":"behavior:researcher:1","binding_revision_id":"binding:researcher:1","participant_id":"participant:researcher:group"},{"agent_id":"agent:builder","behavior_revision_id":"behavior:builder:1","binding_revision_id":"binding:builder:1","participant_id":"participant:builder:group"}],"initial_targets":[{"id":"group-target:researcher","group_turn_id":"group-turn:one","agent_id":"agent:researcher","behavior_revision_id":"behavior:researcher:1","binding_revision_id":"binding:researcher:1","participant_id":"participant:researcher:group","wave":0,"state":"queued","created_at":"2026-08-21T16:20:00Z"},{"id":"group-target:builder","group_turn_id":"group-turn:one","agent_id":"agent:builder","behavior_revision_id":"behavior:builder:1","binding_revision_id":"binding:builder:1","participant_id":"participant:builder:group","wave":0,"state":"queued","created_at":"2026-08-21T16:20:00Z"}]}"#
    static let groupProjection = #"{"group":\#(groupRecord),"turns":[\#(groupTurn)],"messages":[{"id":2,"conversation_id":"conversation:launch","turn_id":"group-turn:one","author_kind":"human","author_id":"human:owner","body":"Build the launch plan.","created_at":"2026-08-21T16:20:00Z"},{"id":3,"conversation_id":"conversation:launch","turn_id":"group-turn:one","target_id":"group-target:researcher","author_kind":"agent","author_id":"participant:researcher:group","author_agent_id":"agent:researcher","body":"The evidence agrees.","created_at":"2026-08-21T16:21:00Z"}]}"#
    static let completedHandoff = #"{"handoff":{"id":"handoff:one","account_id":"4af424a4-d81a-47d5-a495-400868883b86","idempotency_key":"handoff:create:one","state":"completed","created_by_kind":"human","created_by_id":"human:owner","group_turn_id":"group-turn:one","source_message_id":"41","recipient_agent_id":"agent:builder","recipient_behavior_revision_id":"behavior:builder:1","recipient_binding_revision_id":"binding:builder:1","source_conversation_id":"conversation:launch","output_conversation_id":"conversation:launch","context":{"references":[{"kind":"message","id":"40","account_id":"4af424a4-d81a-47d5-a495-400868883b86","immutable":true},{"kind":"message","id":"41","account_id":"4af424a4-d81a-47d5-a495-400868883b86","immutable":true}]},"requested_result":"Verify the release evidence.","reply_to_message_id":"40","root_delegation_grant":{"id":"grant:root","permissions":["read"],"context_record_ids":["message:40","message:41"]},"handoff_policy":{"id":"policy:handoff","permissions":["read"],"context_record_ids":[]},"recipient_binding_policy":{"id":"policy:builder","permissions":["read"],"context_record_ids":[]},"approval_required":false,"requested_authority":["read"],"effective_authority":{"id":"effective","permissions":["read"],"context_record_ids":null},"budget_class":"unknown","max_agent_messages":10,"max_depth":3,"depth":1,"deadline":"2026-08-21T16:30:00Z","ancestor_agent_ids":[],"created_at":"2026-08-21T16:20:00Z"},"target":{"id":"target:one","handoff_id":"handoff:one","conversation_id":"conversation:launch","agent_id":"agent:builder","behavior_revision_id":"behavior:builder:1","binding_revision_id":"binding:builder:1","participant_id":"participant:builder:group","state":"answered","created_at":"2026-08-21T16:20:00Z"},"attempt":{"id":"attempt:one","handoff_id":"handoff:one","lease_id":"lease:one","machine_id":"machine:studio","fence_token":"fence:one","state":"completed","started_at":"2026-08-21T16:21:00Z","lease_expires_at":"2026-08-21T16:26:00Z","terminal_receipt_id":"receipt:one","completed_at":"2026-08-21T16:22:00Z"},"projections":[],"result":{"handoff_id":"handoff:one","output_conversation_id":"conversation:launch","message_id":"43","body":"The evidence is verified."}}"#
    static let routineRecord = #"{"routine":{"id":"routine:weekly","account_id":"4af424a4-d81a-47d5-a495-400868883b86","agent_id":"agent:researcher","current_revision_id":"routine-revision:weekly:1","state":"active","created_at":"2026-08-21T20:00:00Z"},"current_revision":{"id":"routine-revision:weekly:1","routine_id":"routine:weekly","revision":1,"agent_id":"agent:researcher","behavior_revision_id":"behavior:researcher:1","binding_revision_id":"binding:researcher:1","authority":"fort_cloud","trigger":"schedule","schedule":"0 9 * * 1","timezone":"America/Chicago","next_occurrence":"2026-08-24T14:00:00Z","input_source":"fort:conversation:research","freshness_seconds":86400,"expected_result":"Weekly brief","result_conversation_id":"conversation:researcher:home","approval_boundary":"before_external_side_effect","missing_input_behavior":"needs_you","retry_policy":"once","catch_up_policy":"skip","lateness_policy":"within_1h","created_at":"2026-08-21T20:00:00Z"}}"#
    static let revalidatedRoutineRecord = routineRecord
        .replacingOccurrences(of: #""current_revision_id":"routine-revision:weekly:1""#, with: #""current_revision_id":"routine-revision:weekly:2""#)
        .replacingOccurrences(of: #""id":"routine-revision:weekly:1","routine_id""#, with: #""id":"routine-revision:weekly:2","routine_id""#)
        .replacingOccurrences(of: #""revision":1,"agent_id""#, with: #""revision":2,"agent_id""#)
    static let routineRun = #"{"occurrence":{"id":"routine-occurrence:test:one","account_id":"4af424a4-d81a-47d5-a495-400868883b86","routine_id":"routine:weekly","routine_revision_id":"routine-revision:weekly:1","kind":"test","state":"queued","scheduled_for":"2026-08-21T20:05:00Z","idempotency_key":"routine:test:one","approval_evidence_id":"approval:test:one","created_at":"2026-08-21T20:05:00Z","updated_at":"2026-08-21T20:05:00Z"},"run":{"id":"routine-run:test:one","routine_id":"routine:weekly","routine_revision_id":"routine-revision:weekly:1","agent_id":"agent:researcher","behavior_revision_id":"behavior:researcher:1","binding_revision_id":"binding:researcher:1","occurrence_id":"routine-occurrence:test:one","kind":"test","state":"queued","created_at":"2026-08-21T20:05:00Z"},"result_conversation_id":"conversation:researcher:home","activities":[{"sequence":41,"state":"queued","activity":"Routine occurrence queued","created_at":"2026-08-21T20:05:00Z"}]}"#
}
