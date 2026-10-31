import Foundation
import Testing
@testable import FortKit

private actor HubTransport: FortTransport {
    var requests: [FortRequest] = []
    let replies: [Data]
    init(_ replies: [String]) { self.replies = replies.map { Data($0.utf8) } }
    func send(_ request: FortRequest) async throws -> FortResponse {
        requests.append(request)
        return FortResponse(status:200,body:replies[min(requests.count - 1,replies.count - 1)])
    }
}

@Test func hubRunnerCallsUseTheOwnerRoutesAndDecodeTheirStatus() async throws {
    let transport = HubTransport([#"{"state":"notEnrolled"}"#, #"{"state":"connecting","hubURL":"https://hub.example.com","runnerId":"toby-mac"}"#, #"{"state":"notEnrolled"}"#])
    let client = FortClient(installationID:"installation",token:"token",transport:transport)
    #expect(try await client.hubRunnerStatus() == HubRunnerStatus(state:"notEnrolled"))
    let enrolled = try await client.enrollHubRunner(invitation:"fort-runner:abc")
    #expect(enrolled.enrolled && enrolled.runnerId == "toby-mac" && enrolled.hubURL == "https://hub.example.com")
    #expect(try await client.removeHubRunner().enrolled == false)
    let requests = await transport.requests
    #expect(requests.map(\.path) == ["/v1/hub/runner","/v1/hub/runner/enroll","/v1/hub/runner/remove"])
    #expect(requests.map(\.method) == [.get,.post,.post])
    let body = try JSONSerialization.jsonObject(with:requests[1].body ?? Data()) as? [String:String]
    #expect(body == ["invitation":"fort-runner:abc"])
}

@Test func cloudHubStatusReadsPlainly() {
    #expect(HubRunnerStatus(state:"connected").summary == "Connected")
    #expect(HubRunnerStatus(state:"waiting",retryMs:4000).summary == "Offline, retrying in 4s")
    #expect(HubRunnerStatus(state:"refused").summary == "The hub removed this Mac")
}

// Spec 063, revision 2026-10-31: the Mac hands its Agents to the hub and opens the hub's workspace.
@Test func hubHandoffAndClientGrantDecode() async throws {
    let status = try JSONDecoder().decode(HubRunnerStatus.self,from:Data(#"{"state":"connected","hubURL":"https://hub.example.com","runnerId":"toby-mac","handoff":{"state":"synced","at":"2026-10-31T12:00:00.000Z","agentIds":["a1","a2"],"staying":[{"agentId":"a3","name":"Loner","reason":"runtime"}]}}"#.utf8))
    #expect(status.handoff?.agentIds == ["a1","a2"])
    #expect(status.handoff?.summary == "2 Agents run on the hub, 1 stays on this Mac")
    #expect(HubHandoffStatus(state:"unsupported",detail:"Older hub.").summary == "Older hub.")
    let agent = try JSONDecoder().decode(Agent.self,from:Data(#"{"agentId":"a1","name":"Scout","homeConversationId":"c1","readiness":"ready","movedToHub":true,"binding":{"bindingRevisionId":"b1","provider":"openai","runtimeVersion":"1","selectedModel":"m","hostId":"mac","enabled":true}}"#.utf8))
    #expect(agent.isOnHub)
    let transport = HubTransport([#"{"state":"ready","hubURL":"https://hub.example.com","installationId":"hub-installation","deviceId":"4b7c0f0e-2f4e-4a8e-9a43-1d2f1f9c7a11","token":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}"#])
    let client = FortClient(installationID:"installation",token:"token",transport:transport)
    let grant = try await client.hubClient()
    #expect(grant.state == "ready" && grant.installationId == "hub-installation")
    #expect(await transport.requests.map(\.path) == ["/v1/hub/client"])
    #if os(macOS)
    let issued = await MessengerModel.issuedHubConnection(grant)
    #expect(issued?.connection.role == .companion && issued?.connection.installationID == "hub-installation")
    #expect(issued?.connection.endpoint == .network(baseURL:URL(string:"https://hub.example.com")!,fixtureLoopback:false))
    var plain = grant; plain.hubURL = "http://hub.example.com"
    #expect(await MessengerModel.issuedHubConnection(plain) == nil)
    var short = grant; short.token = "short"
    #expect(await MessengerModel.issuedHubConnection(short) == nil)
    #expect(await MessengerModel.issuedHubConnection(HubClientGrant(state:"none")) == nil)
    #endif
}
