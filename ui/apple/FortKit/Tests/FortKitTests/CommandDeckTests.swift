import Foundation
import FortKit

#if canImport(FoundationNetworking)
import FoundationNetworking
#endif

@main
struct FortKitContractChecks {
    static func main() async throws {
        try boardDecodesRedesignFields()
        try metricsDecodeScorecards()
        try gateDecisionEncodesRedirectNote()
        try backlogPatchEncodesAgent()
        try playbookCatalogDecodesWireContract()
        try routePreviewDecodesResolvedStages()
        try chatOverrideEncodesAndAnswerDecodes()
        try await clientUsesPlaybookEndpoints()
        try gatewayAccountPersistsNativeSession()
        try gatewayAddressNormalizesProductionOrigin()
        try await gatewayRelayRetriesHandshakeAndExplainsFailures()
        try secureRelayMatchesGoNoiseVector()
        try quickModePinsAnswerPlaybookWhenTriggerIsDisabled()
        answerOutcomeSurfacesFailureStates()
        sigilIsDeterministicMirroredAndStable()
        projectStatePrioritizesHumanAttention()
        recentFailuresStayActionableForFortyEightHours()
        projectOrderingDemotesHistoricalFailures()
        meshStatusUsesCrossMachineLanguage()
        try macSidebarSelectionUsesNonOptionalTags()
        checkpointCaptionUsesAcceptedProgress()
        displayedWeekIsMondayThroughSunday()
        print("FortKit contract checks passed")
    }

    private static func expect(_ condition: @autoclosure () -> Bool, _ message: String) {
        guard condition() else { fatalError(message) }
    }

    private static func boardDecodesRedesignFields() throws {
        let json = #"{"runs":[{"id":"run-1","title":"Ship Fort","body":"Polish the clients","agent":"codex","status":"running","machine":"studio","flow_id":"release","created_at":"2026-07-19T14:00:00Z","updated_at":"2026-07-19T15:00:00Z","checkpoints":{"total":4,"accepted":2,"waiting":1,"rejected":0,"done":3}}],"gates":[{"run_id":"run-1","node_id":"review","input":"Review the build","since":"2026-07-19T15:00:00Z"}]}"#
        let board = try JSONDecoder().decode(Board.self, from: Data(json.utf8))
        expect(board.runs[0].machine == "studio", "machine did not decode")
        expect(board.runs[0].createdAt == "2026-07-19T14:00:00Z", "created_at did not decode")
        expect(board.runs[0].checkpoints?.accepted == 2, "checkpoints did not decode")
        expect(board.gates[0].since == "2026-07-19T15:00:00Z", "gate since did not decode")
    }

    private static func metricsDecodeScorecards() throws {
        let json = #"{"window_days":30,"assignments":8,"agents":[{"agent":"codex","assignments":8,"decided":6,"first_pass":5,"first_pass_pct":83.3,"accepted":6,"redirects":1,"redirects_per_assignment":0.125,"cost_usd":2.4,"cost_per_accepted":0.4,"cost_known":true,"trend":"improving","trend_delta":8.0,"spark":[50,60,60,75,75,80,83.3],"best":["feature"],"weak":["docs"]}],"lanes":["feature","docs"]}"#
        let metrics = try JSONDecoder().decode(MetricsResponse.self, from: Data(json.utf8))
        expect(metrics.windowDays == 30, "metrics window did not decode")
        expect(abs(metrics.agents[0].firstPassPct - 83.3) < 0.01, "first-pass metric did not decode")
        expect(metrics.agents[0].best == ["feature"], "best lanes did not decode")
    }

    private static func gateDecisionEncodesRedirectNote() throws {
        let data = try JSONEncoder().encode(GateDecision(
            runID: "run-1", nodeID: "review", decision: "reject",
            note: "Tighten the empty state"
        ))
        let object = try JSONSerialization.jsonObject(with: data) as? [String: String]
        expect(object?["note"] == "Tighten the empty state", "redirect note did not encode")
    }

    private static func backlogPatchEncodesAgent() throws {
        let data = try JSONEncoder().encode(BacklogPatch(agent: "codex"))
        let object = try JSONSerialization.jsonObject(with: data) as? [String: String]
        expect(object?["agent"] == "codex", "backlog agent did not encode")
    }

    private static func playbookCatalogDecodesWireContract() throws {
        let json = #"[{"id":"feature-work","name":"Feature work","revision":3,"is_default":true,"plan_gate":true,"delivery":"assignment","trigger":{"kind":"feature","enabled":true},"stages":[{"order":1,"name":"Research","prompt":"Map the surface","description":"Inspect the existing system","assignments":[{"agent":"claude","model":"opus"},{"task_type":"bug","agent":"codex","model":"gpt-5.4"}],"memory":true},{"order":2,"name":"Build","assignments":[{"agent":"codex"}]}]},{"id":"quick-answer","name":"Quick answer","revision":1,"delivery":"answer","trigger":{"kind":"question","enabled":true},"stages":[{"order":1,"name":"Answer","assignments":[{"agent":"claude"}]}]}]"#
        let playbooks = try JSONDecoder().decode([Playbook].self, from: Data(json.utf8))

        expect(playbooks.count == 2, "playbook catalog did not decode")
        expect(playbooks[0].isDefault == true, "is_default did not decode")
        expect(playbooks[0].planGate == true, "plan_gate did not decode")
        expect(playbooks[0].stages[0].assignments[1].taskType == "bug", "task_type branch did not decode")
        expect(playbooks[0].stages[0].memory == true, "stage memory did not decode")
        expect(playbooks[1].delivery == "answer", "answer delivery did not decode")
        expect(playbooks[1].isDefault == nil, "omitted is_default must stay omitted")
        expect(playbooks[1].planGate == nil, "omitted plan_gate must stay omitted")
    }

    private static func routePreviewDecodesResolvedStages() throws {
        let json = #"{"playbook_id":"feature-work","playbook_revision":3,"playbook_name":"Feature work","task_type":"feature","source":"default","plan_gate":true,"delivery":"assignment","stages":[{"order":1,"name":"Research","prompt":"Map the surface","agent":"claude","model":"opus","memory":true},{"order":2,"name":"Build","agent":"codex"}]}"#
        let preview = try JSONDecoder().decode(RoutePreview.self, from: Data(json.utf8))

        expect(preview.playbookID == "feature-work", "route playbook_id did not decode")
        expect(preview.playbookRevision == 3, "route revision did not decode")
        expect(preview.stages[0].agent == "claude", "resolved stage agent did not decode")
        expect(preview.stages[1].model == nil, "omitted resolved model must stay omitted")
    }

    private static func chatOverrideEncodesAndAnswerDecodes() throws {
        let request = ChatRequest(
            text: "Why was it skipped?",
            playbookID: "quick-answer",
            playbookRevision: 1,
            taskType: "question",
            planGate: false
        )
        let data = try JSONEncoder().encode(request)
        let object = try JSONSerialization.jsonObject(with: data) as? [String: Any]
        expect(object?["playbook_id"] as? String == "quick-answer", "chat playbook_id did not encode")
        expect(object?["playbook_revision"] as? Int == 1, "chat playbook revision did not encode")
        expect(object?["task_type"] as? String == "question", "chat task_type did not encode")
        expect(object?["plan_gate"] as? Bool == false, "chat plan_gate false override was omitted")

        let json = #"{"kind":"answer","run_id":"run-answer","answer":"The retry window closed.","playbook_id":"quick-answer","playbook_revision":1}"#
        let result = try JSONDecoder().decode(ChatResult.self, from: Data(json.utf8))
        expect(result.kind == "answer", "answer kind did not decode")
        expect(result.answer == "The retry window closed.", "answer text did not decode")
        expect(result.playbookID == "quick-answer", "answer playbook_id did not decode")
    }

    private static func clientUsesPlaybookEndpoints() async throws {
        StubURLProtocol.requests = []
        StubURLProtocol.bodies = []
        StubURLProtocol.handler = { request in
            let path = request.url?.path ?? ""
            let method = request.httpMethod ?? "GET"
            let json: String
            switch (method, path) {
            case ("GET", "/api/playbooks"):
                json = "[]"
            case ("PUT", "/api/playbooks"), ("POST", "/api/playbooks/feature-work/duplicate"):
                json = #"{"id":"feature-work","name":"Feature work","revision":4,"delivery":"assignment","trigger":{"kind":"feature","enabled":true},"stages":[{"order":1,"name":"Build","assignments":[{"agent":"codex"}]}]}"#
            case ("POST", "/api/route"):
                json = #"{"playbook_id":"feature-work","playbook_revision":4,"playbook_name":"Feature work","task_type":"feature","source":"manual","plan_gate":false,"delivery":"assignment","stages":[{"order":1,"name":"Build","agent":"codex"}]}"#
            case ("POST", "/api/chat"):
                json = #"{"kind":"answer","run_id":"answer-1","answer":"Done."}"#
            default:
                fatalError("unexpected client request: \(method) \(path)")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200, httpVersion: "HTTP/1.1",
                headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(json.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [StubURLProtocol.self]
        let client = FortClient(
            baseURL: URL(string: "https://fort.test")!,
            session: URLSession(configuration: configuration)
        )
        let playbook = Playbook(
            id: "feature-work", name: "Feature work", revision: 3,
            delivery: "assignment", trigger: PlaybookTrigger(kind: "feature", enabled: true),
            stages: [PlaybookStage(order: 1, name: "Build", assignments: [PlaybookAssignment(agent: "codex")])]
        )

        _ = try await client.playbooks()
        _ = try await client.savePlaybook(playbook)
        _ = try await client.duplicatePlaybook(playbook.id)
        _ = try await client.route(RouteRequest(text: "Build it", playbookID: playbook.id, playbookRevision: 3))
        _ = try await client.chat(ChatRequest(text: "Answer it", taskType: "question", planGate: false))

        let signatures = StubURLProtocol.requests.map { "\($0.httpMethod ?? "") \($0.url?.path ?? "")" }
        expect(signatures == [
            "GET /api/playbooks",
            "PUT /api/playbooks",
            "POST /api/playbooks/feature-work/duplicate",
            "POST /api/route",
            "POST /api/chat",
        ], "FortClient playbook endpoint surface drifted: \(signatures)")

        let routeBody = StubURLProtocol.bodies[3].flatMap {
            try? JSONSerialization.jsonObject(with: $0) as? [String: Any]
        }
        expect(routeBody?["playbook_revision"] as? Int == 3, "route request lost immutable revision")
        let chatBody = StubURLProtocol.bodies[4].flatMap {
            try? JSONSerialization.jsonObject(with: $0) as? [String: Any]
        }
        expect(chatBody?["plan_gate"] as? Bool == false, "chat request lost false plan gate")
    }

    private static func gatewayAccountPersistsNativeSession() throws {
        let suite = "FortKitContractChecks.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        defer { defaults.removePersistentDomain(forName: suite) }
        let account = GatewayAccount(
            gatewayURL: URL(string: "https://fort.example")!,
            selectedMachineID: "machine-1",
            bearerToken: "native-token",
            pinnedPublicKeys: ["machine-1": "86nFNj7PKVZC81MQ2j3/1YOsYiryw9jUK1csCZWnB3c="]
        )
        account.save(to: defaults)
        expect(GatewayAccount.load(from: defaults) == account, "native gateway session did not persist")
    }

    private static func gatewayAddressNormalizesProductionOrigin() throws {
        let expected = URL(string: "https://fort-gateway.vercel.app")!
        for raw in [
            "https://fort-gateway.vercel.app",
            "https://fort-gateway.vercel.app/",
            "https://fort-gateway.vercel.app/native",
            "  https://fort-gateway.vercel.app/native?callback=stale#fragment  ",
        ] {
            let normalized = try GatewayAddress.normalize(raw)
            expect(
                normalized == expected,
                "gateway address did not normalize \(raw)"
            )
        }
        for invalid in [
            "fort-gateway.vercel.app",
            "ftp://fort-gateway.vercel.app",
            "https://user:secret@fort-gateway.vercel.app",
            "https://fort-gateway.vercel.app/not-native",
            "https://fort-gateway.tobias-053.workers.dev",
        ] {
            do {
                _ = try GatewayAddress.normalize(invalid)
                fatalError("accepted invalid gateway address \(invalid)")
            } catch is GatewayAddressError {
                // expected
            }
        }
    }

    private static func gatewayRelayRetriesHandshakeAndExplainsFailures() async throws {
        var attempts = 0
        let result = try await GatewayRelayRetry.handshake {
            attempts += 1
            if attempts == 1 {
                throw GatewayRelayError.httpStatus(
                    502,
                    #"{"error":"worker req: 504 {\"error\":\"daemon did not respond\"}"}"#
                )
            }
            return "connected"
        }
        expect(result == "connected", "transient relay handshake did not recover")
        expect(attempts == 2, "transient relay handshake must retry exactly once")

        attempts = 0
        do {
            _ = try await GatewayRelayRetry.handshake {
                attempts += 1
                throw GatewayRelayError.httpStatus(401, #"{"error":"unauthorized"}"#)
            } as String
            fatalError("non-transient relay failure unexpectedly succeeded")
        } catch {
            expect(attempts == 1, "non-transient relay failure must not retry")
        }

        let description = GatewayRelayError.httpStatus(
            502,
            #"{"error":"worker req: 504 {\"error\":\"daemon did not respond\"}"}"#
        ).localizedDescription
        expect(description.contains("502"), "relay failure description omitted the HTTP status")
        expect(description.contains("daemon did not respond"), "relay failure description omitted gateway detail")
        expect(!description.contains("GatewayRelayError error"), "relay failure leaked an opaque Swift enum code")
    }

    private static func secureRelayMatchesGoNoiseVector() throws {
        let initiatorStatic = try RelayKeypair(
            privateKey: Data(base64Encoded: "lfM23kK1E/kHySaXdRbRpdh+Wf/4mbu7wJcIq34eHUE=")!
        )
        let initiatorEphemeral = try RelayKeypair(
            privateKey: Data(base64Encoded: "BlmfkXmXtH/gYxqLYk1O2yUfA6K7M2eiFx8D6agjkXs=")!
        )
        let responderPublic = Data(base64Encoded: "86nFNj7PKVZC81MQ2j3/1YOsYiryw9jUK1csCZWnB3c=")!
        let handshake = try RelayNoiseInitiator(
            staticKeypair: initiatorStatic,
            responderPublicKey: responderPublic,
            ephemeralKeypair: initiatorEphemeral
        )
        let message1 = try handshake.writeMessage(Data("fort ik handshake payload one".utf8))
        expect(
            message1.base64EncodedString() == "3Ngz4RsEAJbFaKPfC78UHmXYfopX27T1c/YzzhtFimwbN4CCW7FVgyT0AXT/WZNHA8VJMe56k5XPw81eu0OOuyK8f/0kWAet1pU2E5K2BykL5N+ecUgldzTDB3JvsUlhaPAJ7BjU9iGf5+NZF/IlKWuCNP7R8DrUKVT9rtA=",
            "Swift Noise IK message 1 drifted from Go: \(message1.base64EncodedString())"
        )
        _ = try handshake.readMessage(Data(base64Encoded: "akgBv7rKL76QfAvC8LhPmKQOMw0Yleh2pbyd11MoBxuzUU9gZfDWnogHx4GLQpBZSD0utIGG3OJQp87ViOrQe/Z+9+Lf/KTrmI+/nI4=")!)
        let session = try handshake.session()
        let sealed = try session.seal(Data("transport frame: initiator to responder".utf8))
        expect(
            sealed.base64EncodedString() == "a4n+3ewv7yRq+z+r3c0TQD2rTCE918BA03Hvb2Eue97JfHWzfpaY+rG8E9XPCPszhc/RWyGTzQ==",
            "Swift Noise transport drifted from Go"
        )
        let opened = try session.open(Data(base64Encoded: "89dlOS+4ArIFQZwq5otVTJJMfnR9z0CiRovKSqb9skHOZsyBhmfR0gCXqhuiALaQ9xeup+Z6dQ==")!)
        expect(
            opened == Data("transport frame: responder to initiator".utf8),
            "Swift Noise response transport drifted from Go"
        )
    }

    private static func quickModePinsAnswerPlaybookWhenTriggerIsDisabled() throws {
        let feature = Playbook(
            id: "feature-work", name: "Feature work", revision: 8, isDefault: true,
            delivery: "assignment", trigger: PlaybookTrigger(kind: "feature", enabled: true),
            stages: [PlaybookStage(order: 1, name: "Build", assignments: [PlaybookAssignment(agent: "codex")])]
        )
        let quick = Playbook(
            id: "quick-answer", name: "Quick answer", revision: 4,
            delivery: "answer", trigger: PlaybookTrigger(kind: "question", enabled: false),
            stages: [PlaybookStage(order: 1, name: "Answer", assignments: [PlaybookAssignment(agent: "hermes")])]
        )

        let selected = FortPlaybookRouting.quickAnswer(in: [feature, quick])
        expect(selected?.id == "quick-answer", "Quick mode must select an answer route even with its trigger off")
        expect(selected?.revision == 4, "Quick mode must pin the answer playbook's immutable revision")

        let data = try JSONEncoder().encode(RouteRequest(
            text: "Why was it skipped?", playbookID: selected?.id,
            playbookRevision: selected?.revision, taskType: "question", planGate: false
        ))
        let object = try JSONSerialization.jsonObject(with: data) as? [String: Any]
        expect(object?["playbook_id"] as? String == "quick-answer", "Quick route must encode an explicit answer playbook")
        expect(object?["playbook_revision"] as? Int == 4, "Quick route must encode the exact answer revision")
        expect(object?["plan_gate"] as? Bool == false, "Quick route must explicitly disable the plan gate")
    }

    private static func answerOutcomeSurfacesFailureStates() {
        expect(
            ChatResult(kind: "answer", runID: "ok", answer: "The retry window closed.").handoffOutcome
                == .answer("The retry window closed."),
            "answer delivery did not preserve text"
        )
        expect(
            ChatResult(kind: "answer", runID: "empty").handoffOutcome
                == .failure("Quick answer returned no text."),
            "empty answer must be surfaced as failure"
        )
        expect(
            ChatResult(kind: "error", runID: "failed", answer: "Provider exited 1").handoffOutcome
                == .failure("Provider exited 1"),
            "wire error kind must be surfaced as failure"
        )
    }

    private static func sigilIsDeterministicMirroredAndStable() {
        let first = FortSigil.cells(for: "Fort Dashboard")
        expect(first == FortSigil.cells(for: "Fort Dashboard"), "sigil is not deterministic")
        expect(first == [
            .init(x: 0, y: 1), .init(x: 4, y: 1), .init(x: 0, y: 2), .init(x: 4, y: 2),
            .init(x: 0, y: 4), .init(x: 4, y: 4), .init(x: 1, y: 0), .init(x: 3, y: 0),
            .init(x: 1, y: 1), .init(x: 3, y: 1), .init(x: 1, y: 4), .init(x: 3, y: 4),
            .init(x: 2, y: 1), .init(x: 2, y: 4),
        ], "sigil algorithm drifted")
        for cell in first where cell.x != 2 {
            expect(first.contains(.init(x: 4 - cell.x, y: cell.y)), "sigil is not mirrored")
        }
    }

    private static func projectStatePrioritizesHumanAttention() {
        let run = RunSummary(id: "r", title: "Project", agent: "codex", status: "running")
        let gate = GateItem(runID: "r", nodeID: "review")
        expect(FortProjectState.resolve(run: run, gates: [gate]) == .needsYou, "gate must win status")
        expect(FortProjectState.resolve(run: run, gates: []) == .working, "running must be working")
        expect(FortProjectState.resolve(
            run: RunSummary(id: "d", title: "Done", agent: "codex", status: "succeeded"), gates: []
        ) == .delivered, "succeeded must be delivered")
        let failed = RunSummary(id: "f", title: "Failed", agent: "codex", status: "failed")
        expect(FortProjectState.resolve(run: failed, gates: []) == .failed, "failed must need attention")
        expect(
            FortProjectState.resolve(run: failed, gates: [GateItem(runID: "f", nodeID: "review")]) == .needsYou,
            "a waiting gate must take priority over terminal failure history"
        )
        expect(FortProjectState.resolve(
            run: RunSummary(id: "i", title: "Idle", agent: "codex", status: "queued"), gates: []
        ) == .idle, "queued must remain idle")
        expect(FortProjectState.resolve(
            run: RunSummary(id: "p", title: "Paused", agent: "codex", status: "paused"), gates: []
        ) == .needsYou, "paused must need attention")
        expect(FortProjectState.resolve(
            run: RunSummary(
                id: "r", title: "Redirected", agent: "codex", status: "succeeded",
                checkpoints: CheckpointSummary(total: 2, accepted: 1, waiting: 0, rejected: 1, done: 1)
            ),
            gates: []
        ) == .idle, "a redirected run must not be delivered")
    }

    private static func recentFailuresStayActionableForFortyEightHours() {
        let formatter = ISO8601DateFormatter()
        let now = formatter.date(from: "2026-07-22T12:00:00Z")!
        let runs = [
            RunSummary(
                id: "recent", title: "Recent", agent: "codex", status: "failed",
                createdAt: "2026-07-18T12:00:00Z", updatedAt: "2026-07-22T11:00:00Z"
            ),
            RunSummary(
                id: "boundary", title: "Boundary", agent: "hermes", status: "error",
                updatedAt: "2026-07-20T12:00:00Z"
            ),
            RunSummary(
                id: "stale", title: "Stale", agent: "claude", status: "failed",
                updatedAt: "2026-07-20T11:59:59Z"
            ),
            RunSummary(
                id: "working", title: "Working", agent: "codex", status: "running",
                updatedAt: "2026-07-22T11:30:00Z"
            ),
            RunSummary(
                id: "gated", title: "Gated", agent: "hermes", status: "failed",
                updatedAt: "2026-07-22T11:55:00Z"
            ),
            RunSummary(id: "undated", title: "Undated", agent: "codex", status: "failed"),
        ]

        let recent = FortAttention.recentFailures(
            in: runs,
            gates: [GateItem(runID: "gated", nodeID: "review")],
            now: now
        )
        expect(recent.map(\.id) == ["recent", "boundary"], "recent failures must use an inclusive 48-hour window, exclude gated runs, and sort newest-first")
        expect(FortProjectState.failed.label == "Failed", "historical project failure must be labeled Failed")
    }

    private static func projectOrderingDemotesHistoricalFailures() {
        let formatter = ISO8601DateFormatter()
        let now = formatter.date(from: "2026-07-22T12:00:00Z")!
        let runs = [
            RunSummary(
                id: "historical-failure", title: "Historical failure", agent: "codex", status: "failed",
                updatedAt: "2026-07-20T11:59:59Z"
            ),
            RunSummary(
                id: "new-success", title: "New success", agent: "hermes", status: "succeeded",
                updatedAt: "2026-07-22T11:45:00Z"
            ),
            RunSummary(
                id: "working", title: "Working", agent: "claude", status: "running",
                updatedAt: "2026-07-22T11:30:00Z"
            ),
            RunSummary(
                id: "recent-failure", title: "Recent failure", agent: "codex", status: "error",
                updatedAt: "2026-07-22T11:00:00Z"
            ),
            RunSummary(
                id: "waiting", title: "Waiting", agent: "hermes", status: "queued",
                updatedAt: "2026-07-18T12:00:00Z"
            ),
        ]
        let ordered = FortProjectOrdering.sorted(
            runs,
            gates: [GateItem(runID: "waiting", nodeID: "review")],
            now: now
        )

        expect(
            ordered.map(\.id) == ["waiting", "recent-failure", "working", "new-success", "historical-failure"],
            "projects must rank gates, recent failures, active work, and newer terminal work ahead of historical failures"
        )
    }

    private static func meshStatusUsesCrossMachineLanguage() {
        let single = FortMeshSummary.resolve([])
        expect(single.title == "This Mac", "single-machine mode must say This Mac")
        expect(single.detail == nil, "single-machine mode must not invent a mesh count")

        let mesh = FortMeshSummary.resolve([
            MachineSummary(name: "mac-mini", local: true, reachable: true),
            MachineSummary(name: "mbp", local: false, reachable: false),
        ])
        expect(mesh.title == "All machines", "configured mesh must not imply one selected machine")
        expect(mesh.detail == "1/2 online", "mesh status must report reachable and total machines")
        expect(mesh.reachable == 1 && mesh.total == 2, "mesh status counts drifted")
    }

    private static func macSidebarSelectionUsesNonOptionalTags() throws {
        let appleRoot = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
        let source = try String(
            contentsOf: appleRoot.appendingPathComponent("macOS/FortWindow.swift"),
            encoding: .utf8
        )

        expect(source.contains("List(selection: $route)"), "macOS sidebar selection contract disappeared")
        expect(source.contains(".tag(item)"), "optional macOS sidebar tags do not update the nonoptional route selection")
        expect(!source.contains(".tag(Optional(item))"), "macOS sidebar tags must match the nonoptional List selection value")
    }

    private static func checkpointCaptionUsesAcceptedProgress() {
        let checkpoints = CheckpointSummary(total: 5, accepted: 3, waiting: 1, rejected: 0, done: 4)
        expect(checkpoints.deckCaption == "3 of 5 checkpoints accepted · 1 awaiting sign-off", "caption drifted")
        let mixed = CheckpointSummary(total: 5, accepted: 2, waiting: 1, rejected: 1, done: 3)
        expect(
            mixed.deckCaption == "2 of 5 checkpoints accepted · 1 awaiting sign-off · 1 redirected",
            "mixed checkpoint caption drifted"
        )
    }

    private static func displayedWeekIsMondayThroughSunday() {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = TimeZone(secondsFromGMT: 0)!
        let formatter = ISO8601DateFormatter()
        let monday = formatter.date(from: "2026-07-20T12:00:00Z")!
        let sunday = formatter.date(from: "2026-07-26T12:00:00Z")!
        let priorSunday = formatter.date(from: "2026-07-19T12:00:00Z")!

        expect(FortSchedule.weekdayIndex(for: monday, calendar: calendar) == 0, "Monday must be column zero")
        expect(FortSchedule.weekdayIndex(for: sunday, calendar: calendar) == 6, "Sunday must be column six")
        expect(FortSchedule.isInDisplayedWeek(sunday, containing: monday, calendar: calendar), "Sunday must share Monday's displayed week")
        expect(!FortSchedule.isInDisplayedWeek(priorSunday, containing: monday, calendar: calendar), "prior Sunday leaked into the displayed week")
    }
}

private final class StubURLProtocol: URLProtocol {
    static var requests: [URLRequest] = []
    static var bodies: [Data?] = []
    static var handler: ((URLRequest) -> (HTTPURLResponse, Data))?

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        Self.requests.append(request)
        Self.bodies.append(Self.bodyData(for: request))
        guard let handler = Self.handler else { fatalError("StubURLProtocol handler missing") }
        let (response, data) = handler(request)
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: data)
        client?.urlProtocolDidFinishLoading(self)
    }

    override func stopLoading() {}

    private static func bodyData(for request: URLRequest) -> Data? {
        if let body = request.httpBody { return body }
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
