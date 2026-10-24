import Foundation
import Testing
@testable import FortKit

// Spec 064: one host serves several runtimes; account and model calls name the runtime they mean.
private actor RecordingTransport: FortTransport {
    var paths: [String] = []
    var bodies: [Data] = []
    func send(_ request: FortRequest) async throws -> FortResponse {
        paths.append(request.path); if let body = request.body { bodies.append(body) }
        let body: String
        switch request.path.split(separator:"?").first.map(String.init) ?? "" {
        case "/v1/models": body = #"{"provider":"claude-code","state":"unavailable","models":[]}"#
        case "/v1/agents": body = #"{"agentId":"a","name":"Claude agent","instructions":"Synthetic","homeConversationId":"h","readiness":"requiresLiveReview","binding":{"bindingRevisionId":"b","provider":"claude-code","runtimeVersion":"2.1.310","selectedModel":"m","hostId":"local-claude-code","enabled":true}}"#
        default: body = #"{"state":"idle","revision":0}"#
        }
        return FortResponse(status:200,body:Data(body.utf8))
    }
}

@Test func clientNamesTheRuntimeForAccountModelAndNewAgentCalls() async throws {
    let transport = RecordingTransport()
    let client = FortClient(installationID:"one",token:"token",transport:transport)
    _ = try await client.accountSignInStatus()
    _ = try await client.accountSignInStatus(provider:"claude-code")
    _ = try await client.models(provider:"claude-code")
    _ = try await client.saveAgent(nil,name:"Claude agent",model:"m",instructions:"Synthetic",provider:"claude-code")
    #expect(await transport.paths == ["/v1/account/sign-in","/v1/account/sign-in?provider=claude-code","/v1/models?provider=claude-code","/v1/agents"])
    let created = try #require(await transport.bodies.last.flatMap { try JSONSerialization.jsonObject(with:$0) as? [String:Any] })
    #expect(created["provider"] as? String == "claude-code")
    await #expect(throws:(any Error).self) { _ = try await client.models(provider:"../other") }
}

@Test func installationListsEveryRuntimeAndOlderHostsStillDecode() throws {
    let runtime = #"{"provider":"codex","name":"ChatGPT","runtimeVersion":"1","hostId":"h","session":"stateless","capabilities":["plainText"]}"#
    let both = try JSONDecoder().decode(Installation.self,from:Data(#"{"contractVersion":"1.0","installationId":"one","recoveryEpoch":"e","mode":"local","serviceName":"Fort","runtime":\#(runtime),"runtimes":[\#(runtime)]}"#.utf8))
    #expect(both.runtimes?.map(\.provider) == ["codex"])
    let older = try JSONDecoder().decode(Installation.self,from:Data(#"{"contractVersion":"1.0","installationId":"one","recoveryEpoch":"e","mode":"local","serviceName":"Fort","runtime":\#(runtime)}"#.utf8))
    #expect(older.runtimes == nil)
}
