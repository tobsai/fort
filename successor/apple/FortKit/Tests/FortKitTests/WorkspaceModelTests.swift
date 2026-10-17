import Foundation
import Testing
@testable import FortKit

private actor WorkspaceServer {
    static let registry = Registry()
    actor Registry {
        var servers: [String:WorkspaceServer] = [:]
        func add(_ server: WorkspaceServer, host: String) { servers[host] = server }
        func response(_ request: URLRequest) async throws -> (Int,Data) { try await servers[request.url!.host!]!.response(request) }
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
    var installation: [String:Any] { ["contractVersion":"1.0","installationId":"model-test","recoveryEpoch":"epoch","mode":"openai","serviceName":"Fort Prototype","capabilities":routineSupport ? ["routines","routineGates"] : []] }
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
    func snapshot(_ conversation: String) -> [String:Any] {
        if let acceptance, let snapshot = acceptance["snapshot"] as? [String:Any], snapshot["conversationId"] as? String == conversation { return snapshot }
        return ["installationId":"model-test","recoveryEpoch":"epoch","conversationId":conversation,"cursor":0,"messages":[],"runs":[],"approvals":[],"lastReadMessageSequence":0]
    }
    func response(_ request: URLRequest) throws -> (Int,Data) {
        if offline { throw URLError(.notConnectedToInternet) }
        func result(_ body: [String:Any], _ status: Int = 200) throws -> (Int,Data) { (status,try JSONSerialization.data(withJSONObject:body)) }
        let path = request.url!.path
        if path == "/v1/devices/enroll" { return try result(["installation":installation,"deviceId":"synthetic","deviceToken":"synthetic-only","expiresAt":"2099-01-01T00:00:00Z"]) }
        if path == "/v1/installation" { return try result(installation) }
        if path == "/v1/routines", request.httpMethod == "GET", routineSupport { return try result(["routines":[routine]]) }
        if path == "/v1/agents" { return try result(["agents":[agent("writer"),agent("reviewer")]]) }
        if path == "/v1/skills" { return try result(["skills":[skill]]) }
        if path == "/v1/conversations" { return try result(["conversations":[["conversationId":"shared","name":"Sample planning","kind":"shared","participantAgentIds":["writer","reviewer"]],["conversationId":"writer-home","name":"Writer","kind":"home","participantAgentIds":["writer"]],["conversationId":"reviewer-home","name":"Reviewer","kind":"home","participantAgentIds":["reviewer"]]]]) }
        if path.hasSuffix("/snapshot") { return try result(snapshot(path.split(separator:"/").map(String.init)[2])) }
        var data = request.httpBody ?? Data()
        if let stream = request.httpBodyStream { stream.open(); defer { stream.close() }; var bytes=[UInt8](repeating:0,count:1024); while stream.hasBytesAvailable { let n=stream.read(&bytes,maxLength:bytes.count); if n<=0 { break }; data.append(contentsOf:bytes.prefix(n)) } }
        let body = try #require(JSONSerialization.jsonObject(with:data) as? [String:Any])
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
                let run: [String:Any] = ["runId":"only-run","requestId":requestID,"conversationId":"shared","bindingRevisionId":binding,"state":"completed","selectedModel":"gpt-6-astra","modelEvidence":"unreported"]
                acceptance = ["receipt":["requestId":requestID,"conversationId":"shared","messageId":"user","runId":"only-run","bindingRevisionId":binding,"inputDigest":"synthetic","acceptedAt":"2026-10-16T00:00:00Z"],"snapshot":["installationId":"model-test","recoveryEpoch":"epoch","conversationId":"shared","cursor":2,"messages":messages,"runs":[run],"approvals":[],"lastReadMessageSequence":0]]
            }
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
