import Foundation
import Testing
@testable import FortKit

@Test func pendingSharedMessageRetainsRecipientAndSelectedSkillRevisions() throws {
    let data = Data(#"{"requestID":"request","conversationID":"shared","bindingRevision":"binding","recoveryEpoch":"epoch","text":"  sample\n","createdAt":0,"recipientAgentID":"reviewer","skills":[{"skillId":"marker","revisionId":"r1"}],"reviewRequired":"The skill changed."}"#.utf8)
    let pending = try JSONDecoder().decode(PendingSubmission.self, from:data)
    let saved = try #require(JSONSerialization.jsonObject(with:JSONEncoder().encode(pending)) as? [String:Any])
    #expect(saved["recipientAgentID"] as? String == "reviewer")
    #expect((saved["skills"] as? [[String:String]])?.first?["revisionId"] == "r1")
    #expect(!pending.canRetry(in:"epoch"))
    #expect(pending.text == "  sample\n")
}

@Test @MainActor func openAIWorkspaceRequiresExplicitSharedRecipientAndRestoresSelectionOffline() async throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    let storage = try DeviceStorage(directory:root, credentials:WorkspaceMemoryCredentials())
    let installation = Installation(contractVersion:"1.0", installationId:"candidate-test", recoveryEpoch:"epoch", mode:"openai", serviceName:"Fort Prototype")
    let agent = Agent(agentId:"writer", name:"Writer", homeConversationId:"home", binding:Binding(bindingRevisionId:"binding", provider:"codex", runtimeVersion:"0.157.1", selectedModel:"gpt-6-astra", hostId:"local-codex", enabled:true), readiness:"requiresLiveReview")
    let room = Conversation(conversationId:"shared", name:"Sample planning", kind:"shared", participantAgentIds:["writer","reviewer"])
    try storage.save(DeviceConnection(baseURL:URL(string:"https://offline.invalid")!, installationID:installation.installationId, deviceID:"synthetic", fixtureLoopback:false), token:"synthetic-only")
    try storage.saveNavigation(SavedNavigation(installation:installation, agents:[agent], selectedAgentID:agent.agentId, conversations:[room], skills:[], selectedConversationID:room.id, recipientAgentID:nil))
    let config = URLSessionConfiguration.ephemeral; config.protocolClasses = [OfflineWorkspaceProtocol.self]
    let session = URLSession(configuration:config); defer { session.invalidateAndCancel() }
    let model = MessengerModel(storage:storage, session:session); await model.restore()
    #expect(model.activeConversationID == "shared")
    model.draft = "  Remember cedar.\n"
    #expect(!model.canSend)
    model.recipientAgentID = "writer"
    #expect(model.canSend)
    await model.send()
    #expect(model.pending.first?.conversationID == "shared")
    #expect(model.pending.first?.recipientAgentID == "writer")
    #expect(model.pending.first?.text == "  Remember cedar.\n")
}

private final class WorkspaceMemoryCredentials: DeviceCredentials {
    func save(_ connection: DeviceConnection, token: String) throws {}
    func token(for connection: DeviceConnection) throws -> String { "synthetic-only" }
}
private final class OfflineWorkspaceProtocol: URLProtocol, @unchecked Sendable {
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func startLoading() { client?.urlProtocol(self, didFailWithError:URLError(.notConnectedToInternet)) }
    override func stopLoading() {}
}
