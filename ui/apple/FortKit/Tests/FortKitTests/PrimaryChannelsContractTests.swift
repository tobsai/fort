import Foundation
import FortKit

#if canImport(FoundationNetworking)
import FoundationNetworking
#endif

extension FortKitContractChecks {
    static func primarySendOutcomeDistinguishesAcceptedDeterministicAndAmbiguous() throws {
        let pending = PrimaryPendingTurn(
            channelID: "channel-1",
            text: "Keep my draft",
            clientTurnID: "11111111-1111-4111-8111-111111111111"
        )
        let drift = FortClientError.httpStatus(
            status: 409,
            body: #"{"code":"primary_agent_drift","error":"authority drifted"}"#
        )
        let oversized = FortClientError.httpStatus(status: 400, body: #"{"error":"too large"}"#)

        for error in [drift, oversized] {
            let outcome = PrimarySendOutcomeReducer.failure(for: error)
            primaryExpect(outcome == .deterministicFailure, "HTTP rejection was treated as ambiguous")
            primaryExpect(
                PrimarySendOutcomeReducer.pendingTurn(for: outcome, submission: pending) == nil,
                "deterministic rejection retained a futile retry"
            )
        }

        for status in [408, 429, 500, 503] {
            let error = FortClientError.httpStatus(status: status, body: #"{"error":"uncertain"}"#)
            let outcome = PrimarySendOutcomeReducer.failure(for: error)
            primaryExpect(outcome == .ambiguous, "HTTP \(status) discarded an ambiguous client turn ID")
            primaryExpect(
                PrimarySendOutcomeReducer.pendingTurn(for: outcome, submission: pending) == pending,
                "HTTP \(status) changed the ambiguous pending turn"
            )
        }

        let ambiguous = PrimarySendOutcomeReducer.failure(for: URLError(.networkConnectionLost))
        primaryExpect(ambiguous == .ambiguous, "transport loss was treated as deterministic")
        primaryExpect(
            PrimarySendOutcomeReducer.pendingTurn(for: ambiguous, submission: pending) == pending,
            "ambiguous transport changed the pending client turn"
        )

        let detail = try JSONDecoder().decode(
            PrimaryChannelDetail.self,
            from: Data(PrimaryFixtures.channelDetail.utf8)
        )
        primaryExpect(
            PrimarySendOutcomeReducer.reconcile(.ambiguous, submission: pending, detail: detail) == .accepted,
            "authoritative turn did not prove an ambiguous send was accepted"
        )
        let unmatched = PrimaryPendingTurn(
            channelID: pending.channelID,
            text: pending.text,
            clientTurnID: "22222222-2222-4222-8222-222222222222"
        )
        primaryExpect(
            PrimarySendOutcomeReducer.reconcile(.deterministicFailure, submission: unmatched, detail: detail)
                == .deterministicFailure,
            "deterministic rejection was mistaken for acceptance because pending state cleared"
        )
    }

    static func primaryScheduleDetailAndOccurrenceActionsStayTruthful() throws {
        let list = try JSONDecoder().decode(
            PrimaryScheduleList.self,
            from: Data(PrimaryFixtures.scheduleList.utf8)
        ).items[0]
        let changedJSON = PrimaryFixtures.scheduleDetail
            .replacingOccurrences(of: "Daily brief", with: "Updated durable brief")
            .replacingOccurrences(of: "America/Chicago", with: "America/New_York")
            .replacingOccurrences(of: #""name":"Thrawn""#, with: #""name":"Updated Channel""#)
        let changed = try JSONDecoder().decode(
            PrimaryScheduleDetail.self,
            from: Data(changedJSON.utf8)
        )
        let displayed = PrimaryScheduleDetailPresentation.item(listItem: list, detail: changed)
        primaryExpect(displayed.title == "Updated durable brief", "schedule detail kept stale list title")
        primaryExpect(displayed.timezone == "America/New_York", "schedule detail kept stale list timezone")
        primaryExpect(displayed.relatedChannel?.name == "Updated Channel", "schedule detail kept stale Channel link")
        primaryExpect(
            PrimaryScheduleDetailPresentation.item(listItem: list, detail: nil) == list,
            "schedule detail did not retain list fallback while loading"
        )

        let actions: [(String, PrimaryScheduleOccurrenceAction?)] = [
            ("scheduled", .viewSchedule),
            ("fired", .openRun),
            ("running", .openRun),
            ("succeeded", .viewResult),
            ("failed", .reviewFailure),
            ("canceled", nil),
        ]
        for (state, expected) in actions {
            primaryExpect(
                PrimarySchedulePresentation.occurrenceAction(for: state) == expected,
                "schedule occurrence \(state) received the wrong action"
            )
        }
        primaryExpect(PrimaryScheduleOccurrenceAction.viewSchedule.title == "View schedule", "Upcoming action copy drifted")
        primaryExpect(PrimaryScheduleOccurrenceAction.openRun.title == "Open run", "active run action copy drifted")
        primaryExpect(PrimaryScheduleOccurrenceAction.viewResult.title == "View result", "completed action copy drifted")
        primaryExpect(PrimaryScheduleOccurrenceAction.reviewFailure.title == "Review failure", "failed action copy drifted")
    }

    static func primaryTransportGenerationChangesAcrossSameOriginMachineSwitch() throws {
        let publicKey = "86nFNj7PKVZC81MQ2j3/1YOsYiryw9jUK1csCZWnB3c="
        let fingerprint = RelayFingerprint.of(publicKey: Data(base64Encoded: publicKey)!)
        func machine(_ id: String) throws -> GatewayMachine {
            let json = #"{"machine_id":"\#(id)","name":"\#(id)","fingerprint":"\#(fingerprint)","online":true,"public_key":"\#(publicKey)"}"#
            return try JSONDecoder().decode(GatewayMachine.self, from: Data(json.utf8))
        }
        let first = try machine("machine-a")
        let second = try machine("machine-b")
        let account = GatewayAccount(
            gatewayURL: URL(string: "https://fort-gateway.example")!,
            selectedMachineID: first.machineID,
            bearerToken: "native-token",
            pinnedPublicKeys: [first.machineID: publicKey, second.machineID: publicKey]
        )
        let client = FortClient.gatewayOnly()
        let initialGeneration = client.transportGeneration
        try client.useGateway(account: account, machine: first)
        let firstGeneration = client.transportGeneration
        let firstURL = client.baseURL
        try client.useGateway(account: account, machine: second)

        primaryExpect(firstURL == client.baseURL, "same-origin machine switch changed gateway origin")
        primaryExpect(firstGeneration > initialGeneration, "first gateway transport did not advance generation")
        primaryExpect(client.transportGeneration > firstGeneration, "same-origin machine switch did not advance generation")
    }

    static func primaryPendingTurnsPersistAndReconcileAuthoritativeTurns() throws {
        let suiteName = "FortKitContractChecks.pending.\(UUID().uuidString)"
        guard let defaults = UserDefaults(suiteName: suiteName) else {
            fatalError("could not create isolated pending-turn defaults")
        }
        defaults.removePersistentDomain(forName: suiteName)
        defer { defaults.removePersistentDomain(forName: suiteName) }

        let pending = PrimaryPendingTurn(
            channelID: "channel-1",
            text: "Keep this exact message",
            clientTurnID: "11111111-1111-4111-8111-111111111111"
        )
        PrimaryPendingTurnStore(defaults: defaults).save([pending.channelID: pending])

        let restored = PrimaryPendingTurnStore(defaults: defaults).load()[pending.channelID]
        primaryExpect(restored?.channelID == pending.channelID, "pending turn lost its Channel across relaunch")
        primaryExpect(restored?.text == pending.text, "pending turn lost its exact text across relaunch")
        primaryExpect(restored?.clientTurnID == pending.clientTurnID, "pending turn changed client_turn_id across relaunch")

        let readyDetailJSON = PrimaryFixtures.channelDetail.replacingOccurrences(
            of: #""readiness":{"state":"drifted","reason":"primary_agent_drift""#,
            with: #""readiness":{"state":"ready","reason":null"#
        )
        let detail = try JSONDecoder().decode(
            PrimaryChannelDetail.self,
            from: Data(readyDetailJSON.utf8)
        )
        let reconciled = PrimaryPendingTurnStore(defaults: defaults).reconciled(
            [pending.channelID: pending], with: detail
        )
        primaryExpect(reconciled[pending.channelID] == nil, "authoritative client_turn_id did not clear pending retry")
        PrimaryPendingTurnStore(defaults: defaults).save(reconciled)
        primaryExpect(
            PrimaryPendingTurnStore(defaults: defaults).load()[pending.channelID] == nil,
            "reconciled pending retry returned after relaunch"
        )

        let unmatched = PrimaryPendingTurn(
            channelID: pending.channelID,
            text: pending.text,
            clientTurnID: "22222222-2222-4222-8222-222222222222"
        )
        let retained = PrimaryPendingTurnStore(defaults: defaults).reconciled(
            [unmatched.channelID: unmatched], with: detail
        )
        primaryExpect(retained[unmatched.channelID] == unmatched, "unmatched pending turn was cleared without evidence")

        let driftedDetail = try JSONDecoder().decode(
            PrimaryChannelDetail.self,
            from: Data(PrimaryFixtures.channelDetail.utf8)
        )
        let driftCleared = PrimaryPendingTurnStore(defaults: defaults).reconciled(
            [unmatched.channelID: unmatched], with: driftedDetail
        )
        primaryExpect(
            driftCleared[unmatched.channelID] == unmatched,
            "readiness drift discarded an unmatched ambiguous draft after relaunch"
        )
        primaryExpect(
            pending.retained(after: .primaryAgentDrift) == nil,
            "primary_agent_drift must still clear pending retry state"
        )
    }

    static func primarySettingsGroupsOptionsByComputer() throws {
        let first = try JSONDecoder().decode(
            PrimaryAgentView.self,
            from: Data(PrimaryFixtures.agentView.utf8)
        ).options[0]
        let secondJSON = PrimaryFixtures.agentView
            .replacingOccurrences(of: "phase1-qa", with: "phase1-lab")
            .replacingOccurrences(of: "primary-option:v1:abc", with: "primary-option:v1:def")
        let second = try JSONDecoder().decode(
            PrimaryAgentView.self,
            from: Data(secondJSON.utf8)
        ).options[0]
        let thirdJSON = PrimaryFixtures.agentView
            .replacingOccurrences(of: "primary-option:v1:abc", with: "primary-option:v1:ghi")
        let third = try JSONDecoder().decode(
            PrimaryAgentView.self,
            from: Data(thirdJSON.utf8)
        ).options[0]

        let groups = PrimaryAgentOptionGrouping.groups(for: [first, second, third])
        primaryExpect(groups.map(\.machine) == ["phase1-qa", "phase1-lab"], "computer groups changed server order")
        primaryExpect(
            groups[0].options.map(\.optionID) == [first.optionID, third.optionID],
            "options on one computer changed order"
        )
        primaryExpect(groups[1].options == [second], "second computer group lost its option")
    }

    static func primarySchedulesUseCanonicalLabelsAndConfiguredTimezones() {
        let labels: [(String?, String)] = [
            (nil, "Upcoming"),
            ("scheduled", "Upcoming"),
            ("upcoming", "Upcoming"),
            ("fired", "Fired"),
            ("running", "Running"),
            ("succeeded", "Completed"),
            ("completed", "Completed"),
            ("canceled", "Canceled"),
            ("failed", "Failed"),
        ]
        for (wireState, expected) in labels {
            primaryExpect(
                PrimarySchedulePresentation.occurrenceLabel(for: wireState) == expected,
                "schedule state \(wireState ?? "nil") did not present as \(expected)"
            )
        }
        primaryExpect(
            PrimarySchedulePresentation.definitionLabel(enabled: true) == "Active",
            "enabled schedule did not present as Active"
        )
        primaryExpect(
            PrimarySchedulePresentation.definitionLabel(enabled: false) == "Paused",
            "paused schedule did not present as Paused"
        )

        let instant = "2026-08-09T14:00:00Z"
        let chicago = PrimaryDateText.full(instant, timeZoneID: "America/Chicago")
        let losAngeles = PrimaryDateText.full(instant, timeZoneID: "America/Los_Angeles")
        primaryExpect(chicago != losAngeles, "schedule timestamp ignored its configured IANA timezone")
    }

    static func primaryModelDisclosurePreservesRequestedAndResolvedModels() {
        let exact = PrimaryModelDisclosure(requested: "gpt-5.6-sol", resolved: "gpt-5.6-sol-2026-08-01")
        primaryExpect(exact.requested == "gpt-5.6-sol", "requested model was not preserved")
        primaryExpect(exact.resolved == "gpt-5.6-sol-2026-08-01", "resolved model was not preserved")

        let unresolved = PrimaryModelDisclosure(requested: "gpt-5.6-sol", resolved: nil)
        primaryExpect(unresolved.requested == "gpt-5.6-sol", "requested model fallback drifted")
        primaryExpect(unresolved.resolved == "unknown", "missing resolved model must disclose unknown")
    }

    static func primaryWireModelsDecodeCanonicalFixtures() throws {
        let agent = try JSONDecoder().decode(
            PrimaryAgentView.self,
            from: Data(PrimaryFixtures.agentView.utf8)
        )
        primaryExpect(agent.selection?.optionID == "primary-option:v1:abc", "Primary selection option_id did not decode")
        primaryExpect(agent.selection?.policy.requestTimeoutMillis == 120_000, "Primary policy timeout did not decode")
        primaryExpect(agent.options.first?.authority.offerVersion == 1, "Primary authority offer did not decode")
        primaryExpect(agent.scheduleInventory?.state == "accepted", "schedule inventory did not decode")

        let detail = try JSONDecoder().decode(
            PrimaryChannelDetail.self,
            from: Data(PrimaryFixtures.channelDetail.utf8)
        )
        primaryExpect(detail.conversation.id == "channel-1", "Primary conversation did not decode")
        primaryExpect(detail.messages.first?.id == 9_007_199_254_740_991, "64-bit message ID lost precision")
        primaryExpect(detail.turns.first?.clientTurnID == "11111111-1111-4111-8111-111111111111", "client_turn_id did not decode")
        primaryExpect(detail.targets.last?.receipt?.cachedInputTokens == 13, "target receipt did not decode")
        primaryExpect(detail.primaryIdentity?.participantID == "participant-1", "Primary identity did not decode")
        primaryExpect(detail.readiness.reason == "primary_agent_drift", "readiness reason did not decode")

        let schedules = try JSONDecoder().decode(
            PrimaryScheduleList.self,
            from: Data(PrimaryFixtures.scheduleList.utf8)
        )
        primaryExpect(schedules.snapshotID == "snapshot-1", "schedule snapshot_id did not decode")
        primaryExpect(schedules.items.first?.nextFireAt == nil, "null next_fire_at must remain nil")
        primaryExpect(schedules.items.first?.latestOccurrence?.state == "failed", "latest occurrence did not decode")
        primaryExpect(schedules.items.first?.relatedChannel?.id == "channel-1", "related Channel did not decode")
    }

    static func clientUsesPrimaryEndpointsAndNoContentCommands() async throws {
        PrimaryStubURLProtocol.requests = []
        PrimaryStubURLProtocol.bodies = []
        PrimaryStubURLProtocol.handler = { request in
            let path = request.url?.path ?? ""
            let method = request.httpMethod ?? "GET"
            let json: String
            let status: Int
            switch (method, path) {
            case ("GET", "/api/settings/primary-agent"),
                 ("PUT", "/api/settings/primary-agent"),
                 ("POST", "/api/settings/primary-agent/recheck"):
                json = PrimaryFixtures.agentView
                status = 200
            case ("DELETE", "/api/settings/primary-agent"),
                 ("PATCH", "/api/channels/channel-1"),
                 ("POST", "/api/channels/channel-1/targets/target-2/cancel"):
                json = ""
                status = 204
            case ("GET", "/api/channels"):
                json = "[]"
                status = 200
            case ("POST", "/api/channels"), ("GET", "/api/channels/channel-1"):
                json = PrimaryFixtures.channelDetail
                status = 200
            case ("POST", "/api/channels/channel-1/turns"):
                json = PrimaryFixtures.turnResult
                status = 202
            case ("POST", "/api/channels/channel-1/targets/target-2/retry"),
                 ("POST", "/api/channels/channel-1/targets/target-2/recheck-and-retry"):
                json = PrimaryFixtures.target
                status = 202
            case ("GET", "/api/needs-you"):
                json = PrimaryFixtures.needsYou
                status = 200
            case ("GET", "/api/schedules"):
                json = PrimaryFixtures.scheduleList
                status = 200
            case ("GET", "/api/schedules/schedule-1"):
                json = PrimaryFixtures.scheduleDetail
                status = 200
            case ("GET", "/api/schedules/schedule-1/occurrences"):
                json = "[\(PrimaryFixtures.occurrence)]"
                status = 200
            default:
                fatalError("unexpected Primary client request: \(method) \(path)")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: status, httpVersion: "HTTP/1.1",
                headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(json.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [PrimaryStubURLProtocol.self]
        let client = FortClient(
            baseURL: URL(string: "https://fort.test")!,
            session: URLSession(configuration: configuration)
        )
        let pending = PrimaryPendingTurn(
            channelID: "channel-1",
            text: "Hello",
            clientTurnID: "11111111-1111-4111-8111-111111111111"
        )

        _ = try await client.primaryAgent()
        _ = try await client.setPrimaryAgent(optionID: "primary-option:v1:abc")
        try await client.clearPrimaryAgent()
        _ = try await client.recheckPrimaryAgent()
        _ = try await client.primaryChannels(state: .archived)
        _ = try await client.createPrimaryChannel(name: "Thrawn")
        _ = try await client.primaryChannel(id: "channel-1")
        try await client.updatePrimaryChannel(id: "channel-1", name: "Thrawn")
        try await client.updatePrimaryChannel(id: "channel-1", state: .archived)
        try await client.updatePrimaryChannel(id: "channel-1", pinned: true)
        _ = try await client.postPrimaryTurn(
            channelID: pending.channelID,
            clientTurnID: pending.clientTurnID,
            text: pending.text
        )
        _ = try await client.retryPrimaryTarget(channelID: "channel-1", targetID: "target-2")
        _ = try await client.recheckAndRetryPrimaryTarget(channelID: "channel-1", targetID: "target-2")
        try await client.cancelPrimaryTarget(channelID: "channel-1", targetID: "target-2")
        _ = try await client.primaryNeedsYou()
        _ = try await client.primarySchedules(filter: .paused)
        _ = try await client.primarySchedule(id: "schedule-1")
        _ = try await client.primaryScheduleOccurrences(
            id: "schedule-1",
            before: "2026-08-09T12:00:00Z", beforeID: "occurrence:1", limit: 25
        )

        let signatures = PrimaryStubURLProtocol.requests.map { request in
            let query = request.url?.query.map { "?\($0)" } ?? ""
            return "\(request.httpMethod ?? "") \(request.url?.path ?? "")\(query)"
        }
        primaryExpect(signatures == [
            "GET /api/settings/primary-agent",
            "PUT /api/settings/primary-agent",
            "DELETE /api/settings/primary-agent",
            "POST /api/settings/primary-agent/recheck",
            "GET /api/channels?state=archived",
            "POST /api/channels",
            "GET /api/channels/channel-1",
            "PATCH /api/channels/channel-1",
            "PATCH /api/channels/channel-1",
            "PATCH /api/channels/channel-1",
            "POST /api/channels/channel-1/turns",
            "POST /api/channels/channel-1/targets/target-2/retry",
            "POST /api/channels/channel-1/targets/target-2/recheck-and-retry",
            "POST /api/channels/channel-1/targets/target-2/cancel",
            "GET /api/needs-you",
            "GET /api/schedules?state=paused",
            "GET /api/schedules/schedule-1",
            "GET /api/schedules/schedule-1/occurrences?limit=25&before=2026-08-09T12:00:00Z&before_id=occurrence:1",
        ], "Primary endpoint surface drifted: \(signatures)")

        let noBodyIndices = [2, 3, 11, 12, 13]
        for index in noBodyIndices {
            primaryExpect(PrimaryStubURLProtocol.bodies[index] == nil, "empty Primary command encoded a JSON body at index \(index)")
        }
        let turnBody = PrimaryStubURLProtocol.bodies[10].flatMap {
            try? JSONSerialization.jsonObject(with: $0) as? [String: String]
        }
        primaryExpect(turnBody?["client_turn_id"] == pending.clientTurnID, "post turn changed client_turn_id")
        primaryExpect(turnBody?["text"] == pending.text, "post turn changed text")
    }

    static func clientSurfacesTypedPrimaryErrorCodes() async throws {
        PrimaryStubURLProtocol.requests = []
        PrimaryStubURLProtocol.bodies = []
        PrimaryStubURLProtocol.handler = { request in
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 409, httpVersion: "HTTP/1.1",
                headerFields: ["Content-Type": "application/json"]
            )!
            return (
                response,
                Data(#"{"code":"primary_agent_drift","error":"Primary Agent authority drifted on phase1-qa"}"#.utf8)
            )
        }
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [PrimaryStubURLProtocol.self]
        let client = FortClient(
            baseURL: URL(string: "https://fort.test")!,
            session: URLSession(configuration: configuration)
        )
        let pending = PrimaryPendingTurn(
            channelID: "channel-1", text: "Hello",
            clientTurnID: "11111111-1111-4111-8111-111111111111"
        )

        do {
            _ = try await client.postPrimaryTurn(
                channelID: pending.channelID,
                clientTurnID: pending.clientTurnID,
                text: pending.text
            )
            fatalError("coded Primary error unexpectedly decoded")
        } catch let error as FortClientError {
            primaryExpect(error.codedError?.code == .primaryAgentDrift, "coded error lost primary_agent_drift")
            primaryExpect(error.codedError?.message.contains("phase1-qa") == true, "coded error lost server message")
            primaryExpect(error.serverCode == .primaryAgentDrift, "serverCode convenience lost typed code")
        }
    }

    static func primaryChannelEventsDecodeStrictReplacementSnapshots() async throws {
        PrimaryStubURLProtocol.requests = []
        PrimaryStubURLProtocol.bodies = []
        let updated = PrimaryFixtures.channelDetail.replacingOccurrences(of: "Thrawn", with: "Thrawn II")
        PrimaryStubURLProtocol.handler = { request in
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200, httpVersion: "HTTP/1.1",
                headerFields: ["Content-Type": "text/event-stream"]
            )!
            let frames = "data: \(PrimaryFixtures.channelDetail)\n\ndata: \(updated)\n\n"
            return (response, Data(frames.utf8))
        }
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [PrimaryStubURLProtocol.self]
        let client = FortClient(
            baseURL: URL(string: "https://fort.test")!,
            session: URLSession(configuration: configuration)
        )

        var titles: [String] = []
        for try await snapshot in client.primaryChannelEvents(channelID: "channel-1") {
            titles.append(snapshot.conversation.title)
        }
        primaryExpect(titles == ["Thrawn", "Thrawn II"], "Primary SSE did not emit replacement snapshots")
        primaryExpect(
            PrimaryStubURLProtocol.requests.first?.url?.path == "/api/channels/channel-1/events",
            "Primary SSE used the wrong direct path"
        )

        PrimaryStubURLProtocol.handler = { request in
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200, httpVersion: "HTTP/1.1",
                headerFields: ["Content-Type": "text/event-stream"]
            )!
            return (response, Data("data: {not-json}\n\n".utf8))
        }
        do {
            for try await _ in client.primaryChannelEvents(channelID: "channel-1") {}
            fatalError("malformed Primary snapshot was silently ignored")
        } catch is DecodingError {
            // Strict replacement snapshots must fail the stream visibly.
        }
    }

    static func primaryChannelEventsUseGatewayRelayPath() async throws {
        let frame = Data("data: \(PrimaryFixtures.channelDetail)\n\n".utf8)
        let split = frame.count / 2
        let relay = PrimaryRelayStub(chunks: [Data(frame.prefix(split)), Data(frame.suffix(from: split))])
        let client = FortClient(
            baseURL: URL(string: "https://fort.test")!,
            relayTransport: relay
        )

        var snapshots: [PrimaryChannelDetail] = []
        for try await snapshot in client.primaryChannelEvents(channelID: "channel-1") {
            snapshots.append(snapshot)
        }
        primaryExpect(snapshots.map(\.conversation.title) == ["Thrawn"], "relay SSE did not decode its snapshot")
        primaryExpect(relay.lastEventPath == "/api/channels/channel-1/events", "relay SSE used the wrong path")
    }

    static func primaryStatusProgressivelyDisclosesLatestAttempt() {
        let failed = PrimaryFixtures.makeTarget(id: "target-1", attempt: 1, state: "failed", errorCode: "provider_failed")
        let answered = PrimaryFixtures.makeTarget(id: "target-2", attempt: 2, state: "answered")
        let latest = PrimaryTargetStatusReducer.latestAttempt(
            in: [answered, failed], turnID: "turn-1", participantID: "participant-1"
        )
        primaryExpect(latest?.id == "target-2", "latest target attempt was not selected deterministically")
        primaryExpect(PrimaryTargetStatusReducer.presentation(for: latest, machine: "phase1-qa") == nil, "answered target must hide durable status")

        let drift = PrimaryFixtures.makeTarget(id: "target-3", attempt: 3, state: "failed", errorCode: "primary_agent_drift")
        let driftPresentation = PrimaryTargetStatusReducer.presentation(for: drift, machine: "phase1-qa")
        primaryExpect(driftPresentation?.title == "This didn’t start", "pre-start drift used the wrong progressive status")
        primaryExpect(driftPresentation?.action == .recheckAndRetry, "drift did not expose recheck and retry")
        primaryExpect(driftPresentation?.showsDetails == true, "failed status did not expose optional details")

        let queued = PrimaryFixtures.makeTarget(id: "target-4", attempt: 1, state: "queued")
        let queuedPresentation = PrimaryTargetStatusReducer.presentation(for: queued, machine: "phase1-qa")
        primaryExpect(queuedPresentation?.title == "Starting Primary Agent…", "queued status drifted")
        primaryExpect(queuedPresentation?.showsDetails == false, "queued status must keep durable details collapsed")
        primaryExpect(queuedPresentation?.action == .cancel, "queued status lost cancel")

        let canceled = PrimaryFixtures.makeTarget(id: "target-5", attempt: 1, state: "canceled")
        let canceledPresentation = PrimaryTargetStatusReducer.presentation(for: canceled, machine: "phase1-qa")
        primaryExpect(canceledPresentation?.title == "Canceled by you", "canceled status drifted")
        primaryExpect(canceledPresentation?.showsDetails == false, "canceled status must remain a compact transcript note")
        primaryExpect(canceledPresentation?.action == nil, "canceled status invented a recovery action")
    }

    static func primaryRecoveryActionsStayClosed() {
        primaryExpect(
            PrimaryTargetStatusReducer.recoveryActions(for: "primary_agent_drift") == [.recheckAndRetry, .retry],
            "drift recovery allowlist changed"
        )
        primaryExpect(
            PrimaryTargetStatusReducer.recoveryActions(for: "daemon_interrupted") == [.retry],
            "interruption recovery allowlist changed"
        )
        primaryExpect(
            PrimaryTargetStatusReducer.recoveryActions(for: "provider_refusal").isEmpty,
            "provider refusal must not invent a recovery action"
        )
        primaryExpect(
            PrimaryTargetStatusReducer.recoveryActions(for: "future_error").isEmpty,
            "unknown errors must fail closed"
        )
    }

    static func primaryPendingTurnPreservesClientTurnID() {
        let uuid = UUID(uuidString: "AAAAAAAA-BBBB-4CCC-8DDD-EEEEEEEEEEEE")!
        let pending = PrimaryPendingTurn(channelID: "channel-1", text: "Hello", uuid: uuid)
        primaryExpect(
            pending.clientTurnID == "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee",
            "new pending turn did not canonicalize its UUID"
        )
        primaryExpect(
            pending.retained(after: FortServerErrorCode("deterministic_rejection")) == nil,
            "coded HTTP rejection retained a futile pending retry"
        )
        primaryExpect(
            pending.retained(after: nil)?.clientTurnID == pending.clientTurnID,
            "transport failure changed client_turn_id"
        )
        primaryExpect(
            pending.retained(after: .primaryAgentDrift) == nil,
            "authority drift must clear a pending turn instead of retrying an obsolete identity"
        )
        primaryExpect(pending.request.clientTurnID == pending.clientTurnID, "request projection changed client_turn_id")
    }
}

private func primaryExpect(_ condition: @autoclosure () -> Bool, _ message: String) {
    guard condition() else { fatalError(message) }
}

private final class PrimaryStubURLProtocol: URLProtocol {
    static var requests: [URLRequest] = []
    static var bodies: [Data?] = []
    static var handler: ((URLRequest) -> (HTTPURLResponse, Data))?

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        Self.requests.append(request)
        Self.bodies.append(Self.bodyData(for: request))
        guard let handler = Self.handler else { fatalError("PrimaryStubURLProtocol handler missing") }
        let (response, data) = handler(request)
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
        if !data.isEmpty {
            client?.urlProtocol(self, didLoad: data)
        }
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

private final class PrimaryRelayStub: FortRelayTransporting, @unchecked Sendable {
    private let lock = NSLock()
    private let chunks: [Data]
    private var eventPath: String?

    init(chunks: [Data]) {
        self.chunks = chunks
    }

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
            for chunk in chunks {
                continuation.yield(chunk)
            }
            continuation.finish()
        }
    }
}

private enum PrimaryFixtures {
    static let policy = #"{"policy_id":"codex-subscription-chat-v1","policy_revision":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","adapter_id":"model.chat.text-only.codex-subscription","adapter_revision":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","codex_version":"codex-cli 1.0","codex_executable_revision":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","codex_schema_revision":"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd","runtime_contract":"codex_subscription_exec_v1","reasoning_effort":"medium","reasoning_context":"current_turn","request_timeout_millis":120000,"developer_instruction_revision":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee","account_type":"chatgpt","account_plan":"plus","thread_mode":"ephemeral","sandbox_mode":"readOnly","approval_policy":"never","workdir_mode":"empty_per_target","dynamic_tools_mode":"none","mcp_mode":"none","command_policy":"deny_and_fail","file_read_policy":"deny_and_fail","isolation_revision":"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"}"#
    static let seat = #"{"id":"seat-1","profile":"codex-subscription:gpt-5.6-sol","agent":"codex-subscription","model":"gpt-5.6-sol","machine":"phase1-qa","display_name":"Primary Agent","state":"ready"}"#
    static let authorityOffer = #"{"offer_version":1,"machine_id":"phase1-qa","seat_id":"seat-1","agent_key":"codex-subscription","profile_id":"codex-subscription:gpt-5.6-sol","requested_model":"gpt-5.6-sol","resolved_model":"unknown","account_type":"chatgpt","account_plan":"plus","policy_id":"codex-subscription-chat-v1","policy_revision":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","runtime_contract":"codex_subscription_exec_v1","reasoning_effort":"medium","reasoning_context":"current_turn","request_timeout_millis":120000,"developer_instruction_revision":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee","adapter_id":"model.chat.text-only.codex-subscription","adapter_revision":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","codex_version":"codex-cli 1.0","codex_executable_revision":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","codex_schema_revision":"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd","thread_mode":"ephemeral","sandbox_mode":"readOnly","approval_policy":"never","workdir_mode":"empty_per_target","dynamic_tools_mode":"none","mcp_mode":"none","command_policy":"deny_and_fail","file_read_policy":"deny_and_fail","isolation_revision":"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"}"#
    static let identity = #"{"conversation_id":"channel-1","participant_id":"participant-1","authority":"chat_subscription_isolated_v1","policy":\#(policy),"created_at":"2026-08-09T10:00:00Z"}"#
    static let participant = #"{"id":"participant-1","conversation_id":"channel-1","seat_id":"seat-1","profile":"codex-subscription:gpt-5.6-sol","agent":"codex-subscription","model":"gpt-5.6-sol","machine":"phase1-qa","display_name":"Primary Agent","position":0,"state":"active","created_at":"2026-08-09T10:00:00Z"}"#
    static let target = #"{"id":"target-2","turn_id":"turn-1","participant_id":"participant-1","run_id":"run-2","attempt":2,"state":"answered","authority":{"authority":"chat_subscription_isolated_v1","policy":\#(policy),"requested_model":"gpt-5.6-sol"},"receipt":{"observed_adapter_id":"model.chat.text-only.codex-subscription","cached_input_tokens":13,"output_tokens":21},"created_at":"2026-08-09T10:01:00Z","updated_at":"2026-08-09T10:02:00Z"}"#
    static let channelDetail = #"{"conversation":{"id":"channel-1","title":"Thrawn","state":"open","created_at":"2026-08-09T10:00:00Z","updated_at":"2026-08-09T10:02:00Z"},"participants":[\#(participant)],"messages":[{"id":9007199254740991,"conversation_id":"channel-1","turn_id":"turn-1","author_kind":"human","author_id":"user","body":"Hello","created_at":"2026-08-09T10:01:00Z"}],"turns":[{"id":"turn-1","conversation_id":"channel-1","client_turn_id":"11111111-1111-4111-8111-111111111111","prompt_message_id":9007199254740991,"through_message_id":9007199254740991,"created_at":"2026-08-09T10:01:00Z"}],"targets":[{"id":"target-1","turn_id":"turn-1","participant_id":"participant-1","run_id":"run-1","attempt":1,"state":"failed","error_code":"primary_agent_drift","error":"authority drifted","created_at":"2026-08-09T10:01:00Z","updated_at":"2026-08-09T10:01:30Z"},\#(target)],"primary_identity":\#(identity),"readiness":{"state":"drifted","reason":"primary_agent_drift","observed_at":"2026-08-09T10:02:00Z"}}"#
    static let turnResult = #"{"turn":{"id":"turn-1","conversation_id":"channel-1","client_turn_id":"11111111-1111-4111-8111-111111111111","prompt_message_id":1,"through_message_id":1,"created_at":"2026-08-09T10:01:00Z"},"targets":[\#(target)]}"#
    static let agentView = #"{"selection":{"option_id":"primary-option:v1:abc","seat":\#(seat),"authority":"chat_subscription_isolated_v1","policy":\#(policy),"updated_at":"2026-08-09T10:00:00Z"},"state":"ready","options":[{"option_id":"primary-option:v1:abc","state":"ready","seat":\#(seat),"authority":\#(authorityOffer),"display_name":"Primary Agent on phase1-qa"}],"schedule_inventory":{"current_digest":"digest-1","accepted_digest":"digest-1","state":"accepted","items":[{"id":"schedule-1","kind":"cron","expression":"0 0 9 * * *","timezone":"America/Chicago","flow_id":"daily","flow_digest":"flow-digest"}]}}"#
    static let occurrence = #"{"id":"occurrence:1","schedule_id":"schedule-1","run_id":"run-1","scheduled_for":"2026-08-09T14:00:00Z","state":"failed","error":"provider failed","created_at":"2026-08-09T14:00:00Z","updated_at":"2026-08-09T14:01:00Z"}"#
    static let scheduleItem = #"{"id":"schedule-1","title":"Daily brief","enabled":false,"kind":"cron","expression":"0 0 9 * * *","recurrence":"Every day at 9:00 AM","timezone":"America/Chicago","next_fire_at":null,"last_fire_at":"2026-08-09T14:00:00Z","target_kind":"flow","target_id":"daily","related_channel":{"id":"channel-1","name":"Thrawn"},"latest_occurrence":\#(occurrence),"scheduler_ownership":"inactive","observed_at":"2026-08-09T15:00:00Z"}"#
    static let scheduleList = #"{"snapshot_id":"snapshot-1","observed_at":"2026-08-09T15:00:00Z","items":[\#(scheduleItem)]}"#
    static let scheduleDetail = #"{"item":\#(scheduleItem),"upcoming":[],"recent":[\#(occurrence)]}"#
    static let needsYou = #"[{"channel":{"conversation":{"id":"channel-1","title":"Thrawn","state":"open","created_at":"2026-08-09T10:00:00Z","updated_at":"2026-08-09T10:02:00Z"},"participant":\#(participant),"primary_identity":\#(identity),"pinned":true,"pinned_at":"2026-08-09T10:00:00Z"},"target":\#(target),"recovery_actions":["retry"]}]"#

    static func makeTarget(
        id: String,
        attempt: Int,
        state: String,
        errorCode: String? = nil
    ) -> PrimaryTarget {
        PrimaryTarget(
            id: id,
            turnID: "turn-1",
            participantID: "participant-1",
            runID: "run-\(attempt)",
            attempt: attempt,
            state: state,
            errorCode: errorCode,
            error: errorCode,
            authority: nil,
            receipt: nil,
            createdAt: "2026-08-09T10:00:00Z",
            updatedAt: "2026-08-09T10:00:00Z"
        )
    }
}
