import Foundation
import Testing
@testable import FortKit

private final class MemoryCredentials: DeviceCredentials {
    private var values: [String:String] = [:]
    func save(_ connection: DeviceConnection, token: String) throws { values[connection.deviceID] = token }
    func token(for connection: DeviceConnection) throws -> String { values[connection.deviceID]! }
}
private actor FixtureHTTP {
    static let registry = Registry()
    actor Registry {
        private var fixtures: [String:FixtureHTTP] = [:]
        func add(_ fixture: FixtureHTTP, host: String) { fixtures[host] = fixture }
        func response(_ request: URLRequest) async throws -> (Int,Data) { try await fixtures[request.url!.host!]!.response(request) }
    }
    let installation = Installation(contractVersion:"1.0",installationId:"fixture-installation",recoveryEpoch:"epoch-one",mode:"fixture",serviceName:"Fort Fixture")
    let agent = Agent(agentId:"agent",name:"Fixture Agent",homeConversationId:"home",binding:Binding(bindingRevisionId:"binding",provider:"fixture",runtimeVersion:"1",selectedModel:"fixture-echo-v1",hostId:"fixture-local",enabled:true),readiness:"fixtureReady")
    var version = "1.0"
    var ownerControlRequests: [String] = []
    func advertiseIncompatibleVersion() { version = "2.0" }
    func controlRequests() -> [String] { ownerControlRequests }
    var pauseInstallation = false
    var installationWaiter: CheckedContinuation<Void,Never>?
    func pauseInstallationReads() { pauseInstallation = true }
    func releaseReads() { pauseInstallation = false; installationWaiter?.resume(); installationWaiter = nil }
    var offline = false
    var disconnectAfterAcceptance = false
    var rejectSubmission = false
    var messages: [Message] = [Message(messageId:"earlier",conversationId:"home",requestId:"earlier-request",role:"assistant",text:"Earlier saved history",sequence:1,createdAt:"2026-10-12T00:00:00.000Z")]
    func setOffline() { offline = true }
    func disconnectAfterNextAcceptance() { disconnectAfterAcceptance = true }
    func rejectNextSubmission() { rejectSubmission = true }
    func addExternalMessage() { messages.append(Message(messageId:"external",conversationId:"home",requestId:"external-request",role:"assistant",text:"New shared history",sequence:2,createdAt:"2026-10-12T00:00:01.000Z")) }
    func snapshot() -> Snapshot { Snapshot(installationId:installation.installationId,recoveryEpoch:installation.recoveryEpoch,conversationId:"home",cursor:messages.count,messages:messages,runs:[],lastReadMessageSequence:0) }
    func response(_ request: URLRequest) async throws -> (Int,Data) {
        if request.url!.path == "/v1/installation" && pauseInstallation { await withCheckedContinuation { installationWaiter = $0 } }
        if offline { throw URLError(.notConnectedToInternet) }
        let encoder = JSONEncoder()
        if request.httpMethod == "POST", request.url!.path.contains("/approvals/") || request.url!.path.contains("/runs/") || request.url!.path.hasSuffix("/read") {
            ownerControlRequests.append(request.url!.path)
            return (409,Data(#"{"error":{"code":"unsupportedContract","message":"Read-only fixture"}}"#.utf8))
        }
        switch request.url!.path {
        case "/v1/devices/enroll": return (200,try encoder.encode(Enrollment(installation:installation,deviceId:"device",deviceToken:"synthetic-fixture-token",expiresAt:"2027-01-01T00:00:00Z")))
        case "/v1/installation": return (200,try encoder.encode(Installation(contractVersion:version,installationId:installation.installationId,recoveryEpoch:installation.recoveryEpoch,mode:installation.mode,serviceName:installation.serviceName)))
        case "/v1/agents": return (200,try encoder.encode(AgentList(agents:[agent])))
        case "/v1/conversations/home/snapshot": return (200,try encoder.encode(snapshot()))
        case "/v1/conversations/home/messages":
            if rejectSubmission { return (409,Data(#"{"error":{"code":"bindingConflict","message":"Binding conflict requires review"}}"#.utf8)) }
            var body = request.httpBody
            if body == nil, let stream = request.httpBodyStream {
                stream.open(); defer { stream.close() }
                var bytes = Data(); var buffer = [UInt8](repeating:0,count:1024)
                while stream.hasBytesAvailable { let count = stream.read(&buffer,maxLength:buffer.count); if count <= 0 { break }; bytes.append(contentsOf:buffer.prefix(count)) }
                body = bytes
            }
            let values = try JSONSerialization.jsonObject(with:body!) as! [String:String]
            let message = Message(messageId:"accepted",conversationId:"home",requestId:values["requestId"]!,role:"user",text:values["text"]!,sequence:messages.count+1,createdAt:"2026-10-12T00:00:01.000Z")
            messages.append(message)
            let receipt = Receipt(requestId:message.requestId,conversationId:"home",messageId:message.messageId,runId:"run",bindingRevisionId:"binding",inputDigest:"fixture-digest",acceptedAt:message.createdAt)
            if disconnectAfterAcceptance { offline = true }
            return (201,try encoder.encode(Acceptance(receipt:receipt,snapshot:snapshot())))
        default: throw URLError(.badURL)
        }
    }
}
private final class FixtureURLProtocol: URLProtocol, @unchecked Sendable {
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func startLoading() {
        Task {
            do {
                let (status,data) = try await FixtureHTTP.registry.response(request)
                client?.urlProtocol(self,didReceive:HTTPURLResponse(url:request.url!,statusCode:status,httpVersion:nil,headerFields:["Content-Type":"application/json"])!,cacheStoragePolicy:.notAllowed)
                client?.urlProtocol(self,didLoad:data)
                client?.urlProtocolDidFinishLoading(self)
            } catch { client?.urlProtocol(self,didFailWithError:error) }
        }
    }
    override func stopLoading() {}
}
@MainActor private func setup() async throws -> (MessengerModel, DeviceStorage, URLSession, FixtureHTTP) {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    let storage = try DeviceStorage(directory:root,credentials:MemoryCredentials())
    let configuration = URLSessionConfiguration.ephemeral; configuration.protocolClasses = [FixtureURLProtocol.self]
    let session = URLSession(configuration:configuration)
    let fixture = FixtureHTTP(); let host = UUID().uuidString.lowercased()+".invalid"
    await FixtureHTTP.registry.add(fixture,host:host)
    let model = MessengerModel(storage:storage,session:session)
    model.endpoint = "https://"+host; model.expectedInstallation = "fixture-installation"; model.enrollmentCode = "synthetic-code"
    await model.enroll()
    #expect(model.connected)
    return (model,storage,session,fixture)
}
@Test @MainActor func coldOfflineLaunchRestoresAgentHistoryAndPendingBeforeNetwork() async throws {
    let (_,storage,session,fixture) = try await setup()
    defer { try? FileManager.default.removeItem(at:storage.directory); session.invalidateAndCancel() }
    let pending = try PendingStore(directory:storage.directory,installationID:"fixture-installation")
    let saved = try pending.enqueue(conversationID:"home",bindingRevision:"binding",recoveryEpoch:"epoch-one",text:"Waiting on this device")
    await fixture.setOffline()
    let restored = MessengerModel(storage:storage,session:session)
    await restored.restore()
    #expect(!restored.connected)
    #expect(restored.selectedAgent?.homeConversationId == "home")
    #expect(restored.snapshot?.messages.first?.text == "Earlier saved history")
    #expect(restored.pending.map(\.requestID) == [saved.requestID])
    restored.draft = "Another message saved offline"
    #expect(restored.canSend)
    await restored.send()
    #expect(restored.pending.count == 2)
}
@Test @MainActor func acceptedSnapshotSurvivesImmediateDisconnectAndColdRestart() async throws {
    let (model,storage,session,fixture) = try await setup()
    defer { try? FileManager.default.removeItem(at:storage.directory); session.invalidateAndCancel() }
    await fixture.disconnectAfterNextAcceptance()
    model.draft = "Accepted before connection dropped"
    await model.send()
    #expect(model.pending.isEmpty)
    #expect(model.snapshot?.messages.last?.text == "Accepted before connection dropped")
    #expect(try storage.snapshot(installationID:"fixture-installation",conversationID:"home")?.messages.last?.text == "Accepted before connection dropped")
    let restored = MessengerModel(storage:storage,session:session); await restored.restore()
    #expect(restored.snapshot?.messages.last?.text == "Accepted before connection dropped")
    #expect(restored.pending.isEmpty)
}
@Test @MainActor func permanentPendingConflictDoesNotSuppressTranscriptRefresh() async throws {
    let (model,storage,session,fixture) = try await setup()
    defer { try? FileManager.default.removeItem(at:storage.directory); session.invalidateAndCancel() }
    await fixture.rejectNextSubmission(); await fixture.addExternalMessage()
    model.draft = "Immutable pending conflict"; await model.send()
    #expect(model.pending.count == 1)
    #expect(model.pending.first?.text == "Immutable pending conflict")
    #expect(model.snapshot?.messages.last?.text == "New shared history")
    #expect(model.status.contains("review"))
}

@Test @MainActor func sendPersistsImmediatelyWhileBackgroundRefreshWaits() async throws {
    let (model,storage,session,fixture) = try await setup()
    defer { try? FileManager.default.removeItem(at:storage.directory); session.invalidateAndCancel() }
    await fixture.pauseInstallationReads()
    let refresh = Task { await model.refresh() }
    while !model.isBusy { await Task.yield() }
    model.draft = "Save locally during refresh"
    await model.send()
    #expect(model.draft.isEmpty)
    #expect(model.pending.first?.text == "Save locally during refresh")
    await fixture.releaseReads()
    await refresh.value
}

@Test @MainActor func incompatibleContractKeepsHistoryReadableAndSendsNoOwnerMutations() async throws {
    let (model,storage,session,fixture) = try await setup()
    defer { try? FileManager.default.removeItem(at:storage.directory); session.invalidateAndCancel() }
    await fixture.advertiseIncompatibleVersion(); await model.refresh()
    model.draft = "This must remain a draft"
    #expect(!model.canSend)
    await model.send()
    let approval = Approval(approvalId:"approval",installationId:"fixture-installation",recoveryEpoch:"epoch-one",runId:"run",attemptId:"attempt",bindingRevisionId:"binding",actionDigest:"digest",resourceScope:["fixture:reply"],actionDescription:"Produce the fixture reply",expiresAt:"2099-01-01T00:00:00.000Z",state:"pending",decidedByDeviceId:nil,decidedAt:nil)
    let run = Run(runId:"run",requestId:"request",conversationId:"home",bindingRevisionId:"binding",state:"outcomeUnknown",selectedModel:"fixture-echo-v1",effectiveModel:nil,modelEvidence:"unreported",attemptId:"attempt")
    await model.decide(approval,approve:true)
    await model.cancel(run)
    await model.closeUnknown(run)
    await model.markRead(try #require(model.snapshot?.messages.first))
    let requests = await fixture.controlRequests()
    #expect(requests == [])
    #expect(model.snapshot?.messages.first?.text == "Earlier saved history")
    #expect(model.draft == "This must remain a draft")
    #expect(model.status.contains("read-only"))
}
