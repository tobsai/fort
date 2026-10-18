import Foundation
import Testing
@testable import FortKit

private actor WorkspaceServer {
    var paging = false
    func enablePaging() { paging = true }
    static let registry = Registry()
    actor Registry {
        var servers: [String:WorkspaceServer] = [:]
        func add(_ server: WorkspaceServer, host: String) { servers[host] = server }
        func response(_ request: URLRequest) async throws -> (Int,Data) { try await servers[request.url!.host!]!.response(request) }
    }
    var priorSkill: [String:Any]?
    func editSkillKeepingPrevious() { priorSkill = skill; revision = "two" }
    var runtimeSupport = false, runtimeRestarts = 0, runtimeReady = false
    func enableRuntimeRecovery() { runtimeSupport = true }
    func runtimeRestartCount() -> Int { runtimeRestarts }
    var runtime: [String:Any] { ["state":runtimeReady ? "ready" : "restartRequired","canRestart":!runtimeReady,"detail":"Synthetic runtime readiness"] }
    var memorySupport = false, memoryCursor = 0
    var memories: [String:[String:Any]] = [:]
    func enableMemory() { memorySupport = true }
    func advanceMemory(_ conversation: String, text: String) {
        memoryCursor += 1; memories[conversation] = ["conversationId":conversation,"revisionId":"memory-"+String(memoryCursor),"text":text]
    }
    var corruptCancellation = false
    func setCorruptCancellation(_ value: Bool) { corruptCancellation = value }
    var cancellationSupport = false, loseCancelResponse = false
    var cancellations: [String] = []
    var holdAcceptance = false, acceptanceHeld = false
    var releaseAcceptance: CheckedContinuation<Void,Never>?, heldWaiter: CheckedContinuation<Void,Never>?
    func enableCancellation() { cancellationSupport = true }
    func disconnect() { offline = true }
    func loseCancellationResponse() { loseCancelResponse = true }
    func pauseAcceptanceResponse() { holdAcceptance = true }
    func waitForHeldAcceptance() async { if !acceptanceHeld { await withCheckedContinuation { heldWaiter = $0 } } }
    func resumeAcceptanceResponse() { releaseAcceptance?.resume(); releaseAcceptance = nil }
    func cancellationSummary() -> [String] { cancellations }
    func holdResponseIfRequested() async {
        if holdAcceptance { holdAcceptance = false; acceptanceHeld = true; heldWaiter?.resume(); heldWaiter = nil; await withCheckedContinuation { releaseAcceptance = $0 } }
    }
    var routineSupport = false, routineVersion = "routine-one"
    func enableRoutines() { routineSupport = true }
    func advanceRoutine() { routineVersion = "routine-two" }
    var routine: [String:Any] {
        ["routineId":"routine","revisionId":routineVersion,"digest":"synthetic","agentSnapshot":agent("writer"),"skills":[],"definition":["schemaVersion":1,"name":"Summary","agentId":"writer","conversationId":"writer-home","bindingRevisionId":"writer-one","instructions":"Follow the declared steps.","inputs":[["name":"topic","type":"text","required":true]],"skills":[],"steps":[["stepId":"draft","name":"Draft","type":"task","instructions":"Summarize the topic.","inputNames":["topic"],"outputStepIds":[]]]]]
    }
    var reviewState: String? = "enabled"
    func setReviewState(_ value: String) { reviewState = value }
    var revision = "one", offline = false, loseNextResponse = false
    var savedRevisions: [String] = [], submissions: [[String:Any]] = []
    var acceptance: [String:Any]?
    var installation: [String:Any] { ["contractVersion":"1.0","installationId":"model-test","recoveryEpoch":"epoch","mode":"openai","serviceName":"Fort Prototype","capabilities":(routineSupport ? ["routines","routineGates"] : []) + (cancellationSupport ? ["requestCancellation"] : []) + (memorySupport ? ["conversationMemory"] : []) + (runtimeSupport ? ["runtimeRecovery"] : [])] }
    func agent(_ id: String) -> [String:Any] { ["agentId":id,"name":id.capitalized,"homeConversationId":id+"-home","readiness":"requiresLiveReview","instructions":"Original instructions.","binding":["bindingRevisionId":id+"-"+revision,"provider":"codex","runtimeVersion":"0.157.1","selectedModel":"gpt-6-astra","hostId":"local-codex","enabled":true]] }
    var skill: [String:Any] {
        var value: [String:Any] = ["skillId":"marker","revisionId":revision,"name":"Marker","instructions":"End with [CHECKED]."]
        if let reviewState {
            value["bundle"] = ["schemaVersion":1,"name":"Marker","version":revision,"source":"owner-local:Fort-instructions","license":"CC0-1.0","entrypoint":"SKILL.md","supportedRuntimes":["codex"],"requestedTools":[],"scripts":[],"files":[["path":"SKILL.md","kind":"file","executable":false,"contentBase64":Data("End with [CHECKED].".utf8).base64EncodedString(),"bytes":19,"sha256":"synthetic"]],"digest":"synthetic"]
            value["review"] = ["state":reviewState,"digest":"synthetic"]
        }
        return value
    }
    func advanceRevision() { revision = "two" }
    func loseAcceptanceResponse() { loseNextResponse = true }
    func reconnect() { offline = false }
    func edits() -> [String] { savedRevisions }
    func replaySummary() -> (Int,Bool) { (submissions.count, submissions.count == 2 && NSDictionary(dictionary:submissions[0]).isEqual(to:submissions[1])) }
    func snapshot(_ conversation: String, before: Int? = nil) -> [String:Any] {
        if paging {
            let upper = (before ?? 301) - 1, lower = max(1,upper - 99)
            let messages: [[String:Any]] = (lower...upper).map { sequence in
                ["messageId":"message-\(sequence)","requestId":"request-\(sequence)","conversationId":conversation,"role":"user","text":"Message \(sequence)","sequence":sequence,"createdAt":"2026-10-18T00:00:00Z"]
            }
            return ["installationId":"model-test","recoveryEpoch":"epoch","conversationId":conversation,"cursor":300,"messages":messages,"runs":[],"approvals":[],"lastReadMessageSequence":0,"window":["before":before as Any? ?? NSNull(),"olderBefore":lower > 1 ? lower as Any : NSNull()]]
        }
        var current: [String:Any] = ["installationId":"model-test","recoveryEpoch":"epoch","conversationId":conversation,"cursor":memoryCursor,"messages":[],"runs":[],"approvals":[],"lastReadMessageSequence":0]
        if let acceptance, let snapshot = acceptance["snapshot"] as? [String:Any], snapshot["conversationId"] as? String == conversation { current = snapshot }
        current["cursor"] = max(current["cursor"] as? Int ?? 0,memoryCursor)
        if let memory = memories[conversation] { current["memory"] = memory }
        return current
    }
    func response(_ request: URLRequest) async throws -> (Int,Data) {
        if offline { throw URLError(.notConnectedToInternet) }
        func result(_ body: [String:Any], _ status: Int = 200) throws -> (Int,Data) { (status,try JSONSerialization.data(withJSONObject:body)) }
        let path = request.url!.path
        if path == "/v1/devices/enroll" { return try result(["installation":installation,"deviceId":"synthetic","deviceToken":"synthetic-only","expiresAt":"2099-01-01T00:00:00Z"]) }
        if path == "/v1/runtime", runtimeSupport { return try result(runtime) }
        if path == "/v1/installation" { return try result(installation) }
        if path == "/v1/routines", request.httpMethod == "GET", routineSupport { return try result(["routines":[routine]]) }
        if path == "/v1/agents" { return try result(["agents":[agent("writer"),agent("reviewer")]]) }
        if path == "/v1/skills/marker/revisions/one", let priorSkill { return try result(priorSkill) }
        if path == "/v1/skills" { return try result(["skills":[skill]]) }
        if path == "/v1/conversations" { return try result(["conversations":[["conversationId":"shared","name":"Sample planning","kind":"shared","participantAgentIds":["writer","reviewer"]],["conversationId":"writer-home","name":"Writer","kind":"home","participantAgentIds":["writer"]],["conversationId":"reviewer-home","name":"Reviewer","kind":"home","participantAgentIds":["reviewer"]]]]) }
        if path.hasSuffix("/snapshot") {
            let before = URLComponents(url:request.url!,resolvingAgainstBaseURL:false)?.queryItems?.first(where:{$0.name == "before"})?.value.flatMap(Int.init)
            return try result(snapshot(path.split(separator:"/").map(String.init)[2],before:before))
        }
        var data = request.httpBody ?? Data()
        if let stream = request.httpBodyStream { stream.open(); defer { stream.close() }; var bytes=[UInt8](repeating:0,count:1024); while stream.hasBytesAvailable { let n=stream.read(&bytes,maxLength:bytes.count); if n<=0 { break }; data.append(contentsOf:bytes.prefix(n)) } }
        let body = try #require(JSONSerialization.jsonObject(with:data) as? [String:Any])
        if path == "/v1/skills/marker/review", body["revisionId"] as? String == "one", var prior = priorSkill {
            prior["review"] = ["state":body["enable"] as? Bool == true ? "enabled" : "disabled","digest":"synthetic"]; priorSkill = prior; return try result(prior)
        }
        if path == "/v1/runtime/restart", runtimeSupport { runtimeRestarts += 1; runtimeReady = true; return try result(runtime) }
        if path.hasPrefix("/v1/conversations/"), path.hasSuffix("/memory"), memorySupport {
            let conversation = String(path.split(separator:"/")[2]), expected = body["expectedRevisionId"] as? String
            savedRevisions.append(expected ?? "none")
            if expected != memories[conversation]?["revisionId"] as? String { return try result(["error":["code":"memoryChanged","message":"Memory changed in another client."]],409) }
            advanceMemory(conversation,text:try #require(body["text"] as? String))
            return try result(memories[conversation]!)
        }
        if path.hasPrefix("/v1/requests/"), path.hasSuffix("/cancel"), cancellationSupport {
            let requestID = String(path.split(separator:"/")[2]); cancellations.append(requestID)
            var response = body; response["requestId"] = requestID
            if var accepted = acceptance, let receipt = accepted["receipt"] as? [String:Any], receipt["requestId"] as? String == requestID,
               var snapshot = accepted["snapshot"] as? [String:Any], var runs = snapshot["runs"] as? [[String:Any]] {
                if runs[0]["state"] as? String == "queued" { runs[0]["state"] = "canceled" }
                snapshot["runs"] = runs; snapshot["cursor"] = 3; accepted["snapshot"] = snapshot; acceptance = accepted
                response["status"] = "accepted"; response["receipt"] = receipt; response["run"] = runs[0]; response["snapshot"] = snapshot
            } else { response["status"] = "notAccepted" }
            if corruptCancellation { response["bindingRevisionId"] = "wrong-binding" }
            if loseCancelResponse { loseCancelResponse = false; offline = true; throw URLError(.networkConnectionLost) }
            return try result(response)
        }
        if path == "/v1/routines/routine" {
            let expected = body["expectedRevisionId"] as? String ?? "missing"; savedRevisions.append(expected)
            if expected != routineVersion { return try result(["error":["code":"routineChanged","message":"Another client changed the routine."]],409) }
            return try result(routine)
        }
        if path == "/v1/routines/routine/run" {
            submissions.append(body)
            if acceptance == nil {
                let requestID = try #require(body["requestId"] as? String), revision = try #require(body["routineRevisionId"] as? String)
                let inputs = try #require(body["inputs"] as? [String:Any])
                let message: [String:Any] = ["messageId":"user","conversationId":"writer-home","requestId":requestID,"role":"user","text":"Run routine: Summary","sequence":1,"createdAt":"2026-10-17T00:00:00Z"]
                let run: [String:Any] = ["runId":"routine-run","requestId":requestID,"conversationId":"writer-home","bindingRevisionId":"writer-one","state":"queued","selectedModel":"gpt-6-astra","modelEvidence":"unreported","routine":["routineId":"routine","revisionId":revision,"definition":routine["definition"]!,"inputs":inputs,"steps":[["stepId":"draft","stepRevisionId":"synthetic","state":"pending"]]]]
                acceptance = ["receipt":["requestId":requestID,"conversationId":"writer-home","messageId":"user","runId":"routine-run","bindingRevisionId":"writer-one","inputDigest":"synthetic","acceptedAt":"2026-10-17T00:00:00Z"],"snapshot":["installationId":"model-test","recoveryEpoch":"epoch","conversationId":"writer-home","cursor":1,"messages":[message],"runs":[run],"approvals":[],"lastReadMessageSequence":0]]
            }
            await holdResponseIfRequested()
            if loseNextResponse { loseNextResponse = false; offline = true; throw URLError(.networkConnectionLost) }
            return try result(acceptance!,201)
        }
        if path == "/v1/agents/writer" || path == "/v1/skills/marker" {
            let expected = body[path.contains("agents") ? "expectedBindingRevisionId" : "expectedRevisionId"] as? String ?? "missing"
            savedRevisions.append(expected)
            let current = path.contains("agents") ? "writer-"+revision : revision
            if expected != current { return try result(["error":["code":"revisionChanged","message":"Another client changed this item."]],409) }
            return try result(path.contains("agents") ? agent("writer") : skill)
        }
        if path == "/v1/conversations/shared/messages" {
            submissions.append(body)
            if acceptance == nil {
                let requestID = try #require(body["requestId"] as? String)
                let binding = try #require(body["bindingRevisionId"] as? String)
                let text = try #require(body["text"] as? String)
                let messages: [[String:Any]] = [["messageId":"user","conversationId":"shared","requestId":requestID,"role":"user","text":text,"sequence":1,"createdAt":"2026-10-16T00:00:00Z","recipientAgentId":"reviewer","recipientAgentName":"Reviewer"],["messageId":"reply","conversationId":"shared","requestId":requestID,"role":"assistant","text":"Synthetic [CHECKED]","sequence":2,"createdAt":"2026-10-16T00:00:01Z","agentId":"reviewer","agentName":"Reviewer"]]
                var run: [String:Any] = ["runId":"only-run","requestId":requestID,"conversationId":"shared","bindingRevisionId":binding,"state":"completed","selectedModel":"gpt-6-astra","modelEvidence":"unreported"]
                if let memory = memories["shared"] {
                    run["configuration"] = ["agentId":"reviewer","agentName":"Reviewer","bindingRevisionId":binding,"selectedModel":"gpt-6-astra","instructions":"Original instructions.","skills":[],"memory":memory]
                }
                acceptance = ["receipt":["requestId":requestID,"conversationId":"shared","messageId":"user","runId":"only-run","bindingRevisionId":binding,"inputDigest":"synthetic","acceptedAt":"2026-10-16T00:00:00Z"],"snapshot":["installationId":"model-test","recoveryEpoch":"epoch","conversationId":"shared","cursor":2,"messages":messages,"runs":[run],"approvals":[],"lastReadMessageSequence":0]]
            }
            await holdResponseIfRequested()
            if loseNextResponse { loseNextResponse = false; offline = true; throw URLError(.networkConnectionLost) }
            return try result(acceptance!,201)
        }
        throw URLError(.badURL)
    }
}
private final class WorkspaceProtocol: URLProtocol, @unchecked Sendable {
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func startLoading() { Task { do { let (status,data)=try await WorkspaceServer.registry.response(request); client?.urlProtocol(self,didReceive:HTTPURLResponse(url:request.url!,statusCode:status,httpVersion:nil,headerFields:["Content-Type":"application/json"])!,cacheStoragePolicy:.notAllowed); client?.urlProtocol(self,didLoad:data); client?.urlProtocolDidFinishLoading(self) } catch { client?.urlProtocol(self,didFailWithError:error) } } }
    override func stopLoading() {}
}
private final class ModelCredentials: DeviceCredentials {
    func save(_ connection: DeviceConnection, token: String) throws {}
    func token(for connection: DeviceConnection) throws -> String { "synthetic-only" }
}
@MainActor private func setupWorkspace() async throws -> (MessengerModel,DeviceStorage,URLSession,WorkspaceServer) {
    let storage=try DeviceStorage(directory:FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString),credentials:ModelCredentials())
    let config=URLSessionConfiguration.ephemeral; config.protocolClasses=[WorkspaceProtocol.self]
    let session=URLSession(configuration:config), server=WorkspaceServer(), host=UUID().uuidString.lowercased()+".invalid"
    await WorkspaceServer.registry.add(server,host:host)
    let model=MessengerModel(storage:storage,session:session); model.endpoint="https://"+host; model.expectedInstallation="model-test"; model.enrollmentCode="synthetic-only"; await model.enroll()
    #expect(model.connected)
    return (model,storage,session,server)
}

@Test @MainActor func earlierHistoryUsesBoundedPagesAndPreservesTheLatestCache() async throws {
    let (model,storage,session,server) = try await setupWorkspace()
    defer { try? FileManager.default.removeItem(at:storage.directory); session.invalidateAndCancel() }
    await server.enablePaging()
    await model.refresh()
    #expect(model.displaySnapshot?.messages.first?.sequence == 201)
    await model.loadEarlierMessages()
    #expect(model.isViewingEarlierMessages)
    #expect(model.displaySnapshot?.messages.count == 100)
    #expect(model.displaySnapshot?.messages.first?.sequence == 101)
    #expect(model.snapshot?.messages.first?.sequence == 201)
    await model.loadEarlierMessages()
    #expect(model.displaySnapshot?.messages.first?.sequence == 1)
    #expect(model.displaySnapshot?.window?.olderBefore == nil)
    #expect(try storage.snapshot(installationID:"model-test",conversationID:try #require(model.activeConversationID))?.messages.first?.sequence == 201)
    model.showLatestMessages()
    #expect(!model.isViewingEarlierMessages)
    #expect(model.displaySnapshot?.messages.first?.sequence == 201)
}

@Test @MainActor func loadedWorkspaceFormsKeepTheirOriginalRevisionAcrossPolling() async throws {
    let (model,storage,session,server)=try await setupWorkspace()
    defer { try? FileManager.default.removeItem(at:storage.directory); session.invalidateAndCancel() }
    let agentForm=WorkspaceForm(agent:try #require(model.agents.first)), skillForm=WorkspaceForm(skill:try #require(model.skills.first))
    agentForm.name="My stale edit"; skillForm.instructions="My stale instructions"
    await server.advanceRevision(); await model.refresh()
    #expect(model.agents.first?.binding.bindingRevisionId == "writer-two")
    #expect(await agentForm.save(using:model) == false)
    #expect(await skillForm.save(using:model) == false)
    #expect(await server.edits() == ["writer-one","one"])
}

@Test @MainActor func openAIModelReplaysLostAcceptanceOnceWithOriginalRecipientAndSkillsAfterRestart() async throws {
    let (model,storage,session,server)=try await setupWorkspace()
    defer { try? FileManager.default.removeItem(at:storage.directory); session.invalidateAndCancel() }
    await model.selectConversation("shared"); model.recipientAgentID="reviewer"; model.selectedSkillIDs=["marker"]
    await server.loseAcceptanceResponse(); model.draft="  Exact sample\n"; await model.send()
    let original=try #require(model.pending.first)
    #expect(original.recipientAgentID == "reviewer")
    #expect(original.skills == [SkillReference(skillId:"marker",revisionId:"one")])
    await server.advanceRevision(); await server.reconnect()
    let restarted=MessengerModel(storage:storage,session:session); await restarted.restore()
    #expect(restarted.pending.isEmpty)
    #expect(restarted.activeConversationID == "shared")
    #expect(restarted.snapshot?.runs.map(\.runId) == ["only-run"])
    #expect(restarted.snapshot?.messages.first?.text == original.text)
    #expect(restarted.snapshot?.messages.last?.agentId == "reviewer")
    let summary=await server.replaySummary(); #expect(summary.0 == 2 && summary.1)
    await restarted.refresh(); #expect(await server.replaySummary().0 == 2)
}

@Test @MainActor func unreviewedOrDisabledBundleCannotBeSelectedForNativeSend() async throws {
    let (model,storage,session,server)=try await setupWorkspace()
    defer { try? FileManager.default.removeItem(at:storage.directory); session.invalidateAndCancel() }
    await model.selectConversation("writer-home"); model.selectedSkillIDs=["marker"]; model.draft="Synthetic test"
    for state in ["reviewRequired","disabled","future-review-state","enabled"] {
        await server.setReviewState(state); await model.refresh()
        #expect(model.canSend == (state == "enabled"))
    }
}

@Test @MainActor func nativeRoutinePendingReplaysPinnedInputsAfterLostAcknowledgementAndRelaunch() async throws {
    let (model,storage,session,server) = try await setupWorkspace()
    defer { try? FileManager.default.removeItem(at:storage.directory); session.invalidateAndCancel() }
    #expect(!model.supportsRoutines && model.connected)
    await server.enableRoutines(); await model.refresh()
    #expect(model.supportsRoutines)
    let revision = try #require(model.routines.first), inputs: [String:RoutineValue] = ["topic":.text("  Café e\u{301}\r\n")]
    let loaded = RoutineForm(revision:revision)
    await server.loseAcceptanceResponse(); #expect(await model.runRoutine(revision,inputs:inputs))
    let pending = try #require(model.pending.first)
    #expect(pending.routine?.revisionId == "routine-one" && pending.routine?.inputs == inputs)
    #expect(!pending.canRetry(in:"restored-epoch"))
    await server.advanceRoutine(); await server.advanceRevision(); await server.reconnect()
    let restarted = MessengerModel(storage:storage,session:session); await restarted.restore()
    #expect(restarted.pending.isEmpty)
    #expect(restarted.snapshot?.runs.first?.routine?.revisionId == "routine-one")
    #expect(restarted.snapshot?.runs.first?.routine?.inputs == inputs)
    #expect(restarted.activeConversationID == "writer-home")
    let summary = await server.replaySummary(); #expect(summary.0 == 2 && summary.1)
    #expect(await loaded.save(using:restarted) == false)
    #expect(await server.edits() == ["routine-one"])
    let roundTrip = try JSONDecoder().decode(RoutineDefinition.self,from:JSONEncoder().encode(revision.definition))
    #expect(roundTrip == revision.definition)
}

@Test @MainActor func pendingCancellationSurvivesLostAcceptanceAndLostCancelResponseAcrossRelaunches() async throws {
    let (model,storage,session,server) = try await setupWorkspace()
    defer { try? FileManager.default.removeItem(at:storage.directory); session.invalidateAndCancel() }
    await server.enableCancellation(); await server.enableRoutines(); await model.refresh()
    await server.loseAcceptanceResponse()
    #expect(await model.runRoutine(try #require(model.routines.first),inputs:["topic":.text("Synthetic")]))
    let pending = try #require(model.pending.first)
    await model.cancelPending(pending)
    #expect(model.pending.first?.cancelRequested == true)
    #expect(model.pending.first?.canRetry(in:"epoch") == false)
    await server.reconnect(); await server.loseCancellationResponse()
    let second = MessengerModel(storage:storage,session:session); await second.restore()
    #expect(second.pending.first?.cancelRequested == true)
    await server.reconnect()
    let third = MessengerModel(storage:storage,session:session); await third.restore()
    #expect(third.pending.isEmpty)
    #expect(third.snapshot?.runs.first?.state == "canceled")
    #expect(await server.replaySummary().0 == 1)
    #expect(await server.cancellationSummary() == [pending.requestID,pending.requestID])
}

@Test @MainActor func offlinePendingCancellationIsAcknowledgedWithoutSubmittingAfterRelaunch() async throws {
    let (model,storage,session,server) = try await setupWorkspace()
    defer { try? FileManager.default.removeItem(at:storage.directory); session.invalidateAndCancel() }
    await server.enableCancellation(); await model.refresh(); await model.selectConversation("shared"); model.recipientAgentID = "reviewer"
    await server.disconnect(); model.draft = "Never send this"; await model.send()
    let pending = try #require(model.pending.first); await model.cancelPending(pending)
    await server.reconnect()
    let restarted = MessengerModel(storage:storage,session:session); await restarted.restore()
    #expect(restarted.pending.isEmpty && restarted.snapshot?.runs.isEmpty == true)
    #expect(await server.replaySummary().0 == 0)
    #expect(await server.cancellationSummary() == [pending.requestID])
}

@Test @MainActor func cancellationWhileAcceptanceResponseIsInFlightIsNotLost() async throws {
    let (model,storage,session,server) = try await setupWorkspace()
    defer { try? FileManager.default.removeItem(at:storage.directory); session.invalidateAndCancel() }
    await server.enableCancellation(); await server.enableRoutines(); await model.refresh()
    let routine = try #require(model.routines.first)
    await server.pauseAcceptanceResponse()
    let send = Task { await model.runRoutine(routine,inputs:["topic":.text("Synthetic race")]) }
    await server.waitForHeldAcceptance()
    let pending = try #require(model.pending.first); await model.cancelPending(pending)
    #expect(model.pending.first?.cancelRequested == true)
    await server.resumeAcceptanceResponse(); #expect(await send.value)
    #expect(model.pending.isEmpty && model.snapshot?.runs.first?.state == "canceled")
    #expect(await server.replaySummary().0 == 1)
    #expect(await server.cancellationSummary() == [pending.requestID])
}

@Test @MainActor func mismatchedCancellationAcknowledgementCannotErasePendingIntent() async throws {
    let (model,storage,session,server) = try await setupWorkspace()
    defer { try? FileManager.default.removeItem(at:storage.directory); session.invalidateAndCancel() }
    await server.enableCancellation(); await server.enableRoutines(); await model.refresh()
    await server.loseAcceptanceResponse()
    #expect(await model.runRoutine(try #require(model.routines.first),inputs:["topic":.text("Synthetic")]))
    let pending = try #require(model.pending.first); await model.cancelPending(pending)
    await server.reconnect(); await server.setCorruptCancellation(true); await model.refresh()
    #expect(model.pending.first?.cancelRequested == true && !model.pendingErrors.isEmpty)
    #expect(await server.replaySummary().0 == 1)
    await server.setCorruptCancellation(false); await model.refresh()
    #expect(model.pending.isEmpty && model.snapshot?.runs.first?.state == "canceled")
}

@Test @MainActor func nativeMemoryEditorPreservesItsLoadedRevisionAndPendingReplayKeepsTheAcceptedNote() async throws {
    let (model,storage,session,server) = try await setupWorkspace()
    defer { try? FileManager.default.removeItem(at:storage.directory); session.invalidateAndCancel() }
    await server.enableMemory(); await model.refresh(); await model.selectConversation("shared"); model.recipientAgentID = "reviewer"
    let form = ConversationMemoryForm(conversationID:"shared",memory:model.conversationMemory)
    form.text = "  Shared sample note: cedar.\r\n"
    #expect(await form.save(using:model))
    let saved = try #require(model.conversationMemory)
    #expect(saved.text == form.text)
    let stale = ConversationMemoryForm(conversationID:"shared",memory:saved); stale.text = "Stale replacement"
    await server.loseAcceptanceResponse(); model.draft = "Use the saved note"; await model.send()
    let pending = try #require(model.pending.first); #expect(pending.memoryRevisionId == saved.revisionId)
    await server.advanceMemory("shared",text:"New note: birch"); await server.reconnect()
    let restarted = MessengerModel(storage:storage,session:session); await restarted.restore()
    #expect(restarted.pending.isEmpty)
    #expect(restarted.snapshot?.runs.first?.configuration?.memory == saved)
    #expect(restarted.conversationMemory?.text == "New note: birch")
    #expect(await stale.save(using:restarted) == false)
    #expect(stale.text == "Stale replacement")
    #expect(await server.edits() == ["none",saved.revisionId])
    let clear = ConversationMemoryForm(conversationID:"shared",memory:restarted.conversationMemory); clear.text = ""
    #expect(await clear.save(using:restarted))
    #expect(restarted.conversationMemory?.text == "")
    await restarted.selectConversation("writer-home"); #expect(restarted.conversationMemory == nil)
    let summary = await server.replaySummary(); #expect(summary.0 == 2 && summary.1)
}

@Test @MainActor func nativeRuntimeRecoveryIsVisibleAndOnlyAnExplicitAvailableActionRestartsIt() async throws {
    let (model,storage,session,server) = try await setupWorkspace()
    defer { try? FileManager.default.removeItem(at:storage.directory); session.invalidateAndCancel() }
    await server.enableRuntimeRecovery(); await model.refresh()
    #expect(model.runtimeReadiness?.state == "restartRequired" && model.runtimeReadiness?.canRestart == true)
    #expect(await server.runtimeRestartCount() == 0)
    await model.restartRuntime()
    #expect(model.runtimeReadiness?.state == "ready")
    #expect(await server.runtimeRestartCount() == 1)
    await model.restartRuntime(); #expect(await server.runtimeRestartCount() == 1)
}

@Test @MainActor func nativePinnedSkillReviewLoadsAndRevokesTheHistoricalRevisionWithoutDisablingCurrent() async throws {
    let (model,storage,session,server) = try await setupWorkspace()
    defer { try? FileManager.default.removeItem(at:storage.directory); session.invalidateAndCancel() }
    let old = try #require(model.skills.first), bundle = try #require(old.bundle)
    let ref = RoutineSkill(skillId:old.id,revisionId:old.revisionId,digest:bundle.digest)
    await server.editSkillKeepingPrevious(); await model.refresh()
    #expect(model.skills.first?.revisionId == "two")
    let pinned = try await model.skillRevision(ref)
    #expect(pinned.revisionId == old.revisionId && pinned.canInvoke)
    let disabled = try #require(await model.reviewSkill(pinned,enable:false))
    #expect(disabled.revisionId == "one" && !disabled.canInvoke)
    #expect(model.skills.first?.revisionId == "two" && model.skills.first?.canInvoke == true)
    #expect(try await model.skillRevision(ref).canInvoke == false)
}
