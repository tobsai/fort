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
