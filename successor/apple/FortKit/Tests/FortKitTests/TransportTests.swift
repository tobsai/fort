import Foundation
import Testing
@testable import FortKit

private actor RecordedTransport: FortTransport {
    private var requests: [FortRequest] = []
    private let reply: FortResponse
    init(status: Int = 200, body: String) { reply = FortResponse(status:status,body:Data(body.utf8)) }
    func send(_ request: FortRequest) async throws -> FortResponse { requests.append(request); return reply }
    func received() -> [FortRequest] { requests }
}

@Test func localTransportCarriesExactSubmissionAndIdentityWithoutANetworkAddress() async throws {
    let transport = RecordedTransport(status:409,body:#"{"error":{"code":"fixture-rejection","message":"Synthetic refusal"}}"#)
    let client = FortClient(installationID:"local-installation",token:"synthetic-token",transport:transport)
    let text = "  Exact cedar 🌲 Café e\u{301}\r\n"
    let pending = PendingSubmission(requestID:"request-one",conversationID:"home",bindingRevision:"binding",recoveryEpoch:"epoch",text:text,createdAt:Date(),recipientAgentID:"agent",skills:[])
    do { _ = try await client.submit(pending); Issue.record("Expected service refusal") }
    catch ClientError.service(let code,let message) { #expect(code == "fixture-rejection"); #expect(message == "Synthetic refusal") }
    let request = try #require(await transport.received().first)
    #expect(request.path == "/v1/conversations/home/messages"); #expect(request.method == .post)
    #expect(request.installationID == "local-installation"); #expect(request.token == "synthetic-token")
    #expect(request.timeout == 10)
    let bytes = try #require(request.body)
    let body = try #require(JSONSerialization.jsonObject(with:bytes) as? [String:Any])
    #expect(body["text"] as? String == text); #expect(body["requestId"] as? String == "request-one")
    #expect(body["bindingRevisionId"] as? String == "binding"); #expect(body["recipientAgentId"] as? String == "agent")
    let decoded = try JSONDecoder().decode(FortRequest.self,from:JSONEncoder().encode(request))
    #expect(decoded.body == bytes); #expect(decoded.path == request.path); #expect(decoded.token == "synthetic-token")
}

@Test func localTransportKeepsReplayEncodingAndInstallationChecks() async throws {
    let epoch = "epoch & another=1"
    let transport = RecordedTransport(body:#"{"installationId":"one","recoveryEpoch":"epoch & another=1","cursor":2,"events":[],"semantics":"invalidation"}"#)
    let client = FortClient(installationID:"one",transport:transport)
    let page = try await client.events(after:2,recoveryEpoch:epoch,waitMilliseconds:25_000)
    #expect(page.cursor == 2)
    let request = try #require(await transport.received().first)
    #expect(request.method == .get); #expect(request.timeout == 30); #expect(request.token == nil)
    let components = try #require(URLComponents(string:request.path))
    #expect(components.scheme == nil && components.host == nil)
    #expect(components.path == "/v1/events")
    #expect(components.queryItems == [URLQueryItem(name:"after",value:"2"),URLQueryItem(name:"recoveryEpoch",value:epoch),URLQueryItem(name:"wait",value:"25000")])
    let foreign = RecordedTransport(body:#"{"contractVersion":"1.0","installationId":"other","recoveryEpoch":"epoch","mode":"fixture","serviceName":"Fixture"}"#)
    do { _ = try await FortClient(installationID:"one",transport:foreign).installation(); Issue.record("Expected foreign installation refusal") }
    catch ClientError.wrongInstallation {}
}

private struct WaitingTransport: FortTransport {
    func send(_ request: FortRequest) async throws -> FortResponse {
        try await Task.sleep(for:.seconds(30))
        throw ClientError.invalidResponse
    }
}
@Test func localTransportPropagatesCancellation() async throws {
    let client = FortClient(installationID:"one",transport:WaitingTransport())
    let task = Task { try await client.installation() }
    task.cancel()
    await #expect(throws:CancellationError.self) { try await task.value }
}

private final class PrefixHTTP: URLProtocol, @unchecked Sendable {
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func startLoading() {
        guard let url = request.url, url.absoluteString == "https://fixture.invalid/fort/v1/installation",
              request.httpMethod == "GET", request.value(forHTTPHeaderField:"Authorization") == "Bearer synthetic-token",
              request.value(forHTTPHeaderField:"X-Fort-Installation-Id") == "one",
              let response = HTTPURLResponse(url:url,statusCode:200,httpVersion:nil,headerFields:nil) else {
            client?.urlProtocol(self,didFailWithError:URLError(.badURL)); return
        }
        client?.urlProtocol(self,didReceive:response,cacheStoragePolicy:.notAllowed)
        client?.urlProtocol(self,didLoad:Data(#"{"contractVersion":"1.0","installationId":"one","recoveryEpoch":"epoch","mode":"fixture","serviceName":"Fixture"}"#.utf8))
        client?.urlProtocolDidFinishLoading(self)
    }
    override func stopLoading() {}
}
@Test func networkTransportPreservesBasePathAndAuthentication() async throws {
    let configuration = URLSessionConfiguration.ephemeral; configuration.protocolClasses = [PrefixHTTP.self]
    let session = URLSession(configuration:configuration); defer { session.invalidateAndCancel() }
    let client = try FortClient(baseURL:URL(string:"https://fixture.invalid/fort")!,installationID:"one",token:"synthetic-token",session:session)
    #expect(try await client.installation().installationId == "one")
}

#if os(macOS)
@Test func nativeHostForwardsOneBoundedRequestAndRejectsMalformedInput() async throws {
    let transport = RecordedTransport(body:"  Exact cedar 🌲\r\n")
    let channel = MacHostChannel(transport:transport)
    func exchange(_ bytes: Data, channel: MacHostChannel) async throws -> Data {
        try await withCheckedThrowingContinuation { continuation in
            channel.exchange(bytes) { data,error in
                if let error { continuation.resume(throwing:error) }
                else if let data { continuation.resume(returning:data) }
                else { continuation.resume(throwing:ClientError.invalidResponse) }
            }
        }
    }
    let request = FortRequest(path:"/v1/probe",method:.post,installationID:"exact",token:"synthetic",body:Data("input".utf8),timeout:1)
    let bytes = try JSONEncoder().encode(request)
    let reply = try JSONDecoder().decode(FortResponse.self,from:await exchange(bytes,channel:channel))
    #expect(reply.body == Data("  Exact cedar 🌲\r\n".utf8))
    await #expect(throws:(any Error).self) { try await exchange(bytes,channel:channel) }
    for invalid in [Data("invalid-json".utf8),Data(repeating:65,count:2 * 1024 * 1024 + 1)] {
        await #expect(throws:(any Error).self) { try await exchange(invalid,channel:MacHostChannel(transport:transport)) }
    }
    let requests = await transport.received()
    #expect(requests.count == 1 && requests[0].body == request.body && requests[0].token == "synthetic")
    #expect(throws:ClientError.self) { try MacHostListener(clientIdentifier:"invalid\" or true",signingTeam:"AAAAAAAAAA",transport:transport) }
}

@Test func nativeXPCRefusesMalformedPeerIdentityAndNonrelativeRoutes() async throws {
    #expect(throws:ClientError.self) { try MacXPCTransport(serviceIdentifier:"invalid\" or true",signingTeam:"AAAAAAAAAA") }
    #expect(throws:ClientError.self) { try MacXPCTransport(serviceIdentifier:"synthetic.unavailable",signingTeam:"invalid\"") }
    let transport = try MacXPCTransport(serviceIdentifier:"synthetic.unavailable",signingTeam:"AAAAAAAAAA")
    for path in ["https://elsewhere.invalid/v1/installation","//elsewhere.invalid/v1/installation","/v1/../outside","/outside","/v1/installation#fragment"] {
        let request = FortRequest(path:path,method:.get,installationID:"fixture",token:nil,body:nil,timeout:0.1)
        await #expect(throws:ClientError.self) { try await transport.send(request) }
    }
    for timeout in [0.0,31.0,Double.infinity] {
        let request = FortRequest(path:"/v1/installation",method:.get,installationID:"fixture",token:nil,body:nil,timeout:timeout)
        await #expect(throws:ClientError.self) { try await transport.send(request) }
    }
}
#endif
