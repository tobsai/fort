import Foundation
import Darwin

private struct Echo: Decodable { let text: String; let installation: String; let pid: Int32 }

func probeXPC() async throws -> [String: Any] {
    let bundle = Bundle.main
    guard let identifier = bundle.bundleIdentifier,
          let team = bundle.object(forInfoDictionaryKey:"FortSigningTeam") as? String else { throw CocoaError(.coderValueNotFound) }
    let service = identifier + ".channel"
    let transport = try MacXPCTransport(serviceIdentifier:service,signingTeam:team)
    let text = "  Local cedar 🌲\r\n"
    func request(_ path: String = "/v1/probe?fixture=exact", timeout: TimeInterval = 3, body: Data? = nil) -> FortRequest {
        FortRequest(path:path,method:.post,installationID:"synthetic",token:nil,body:body ?? Data(text.utf8),timeout:timeout)
    }
    let accepted = try await transport.send(request())
    let echo = try JSONDecoder().decode(Echo.self,from:accepted.body)
    let clientMismatch = try MacXPCTransport(serviceIdentifier:identifier + ".rejected",signingTeam:team)
    let serviceMismatch = try MacXPCTransport(serviceIdentifier:service,signingTeam:team == "AAAAAAAAAA" ? "BBBBBBBBBB" : "AAAAAAAAAA")
    func errorCode(_ peer: MacXPCTransport) async -> Int {
        do { _ = try await peer.send(request()); return -1 }
        catch { let error = error as NSError; return error.domain == NSCocoaErrorDomain ? error.code : -1 }
    }
    let clientError = await errorCode(clientMismatch), serviceError = await errorCode(serviceMismatch)
    let ownerRequest = try MacOwnerRecoveryRequest(installationID:"synthetic",deviceID:"synthetic",timeout:3)
    func ownerError(_ peer: MacXPCTransport) async -> NSError? {
        do { _ = try await peer.recoverPairing(ownerRequest); return nil }
        catch { return error as NSError }
    }
    let ownerClientError = await ownerError(clientMismatch), ownerServiceError = await ownerError(serviceMismatch), ownerDisabled = await ownerError(transport)
    let pending = Task { try await transport.send(request("/v1/hold")) }
    try await Task.sleep(for:.milliseconds(50)); pending.cancel()
    var canceled = false
    do { _ = try await pending.value } catch is CancellationError { canceled = true }
    var timedOut = false
    do { _ = try await transport.send(request("/v1/hold",timeout:0.1)) }
    catch let error as URLError { timedOut = error.code == .timedOut }
    func invalid(_ input: FortRequest) async -> Bool {
        do { _ = try await transport.send(input); return false }
        catch ClientError.invalidResponse { return true }
        catch { return false }
    }
    let oversizedBody = await invalid(request("/v1/large-body"))
    let oversizedFrame = await invalid(request("/v1/large-frame"))
    var proof: [String:Any] = ["xpcRoundTrip":accepted.status == 200 && echo.text == text && echo.installation == "synthetic" && echo.pid > 1 && echo.pid != getpid(),
            "xpcClientRejected":clientError == NSXPCConnectionInterrupted,
            "xpcServiceRejected":serviceError == NSXPCConnectionCodeSigningRequirementFailure,
            "xpcClientError":clientError, "xpcServiceError":serviceError, "xpcServicePid":echo.pid,
            "xpcOwnerClientRejected":ownerClientError?.domain == NSCocoaErrorDomain && ownerClientError?.code == NSXPCConnectionInterrupted,
            "xpcOwnerServiceRejected":ownerServiceError?.domain == NSCocoaErrorDomain && ownerServiceError?.code == NSXPCConnectionCodeSigningRequirementFailure,
            "xpcOwnerDisabled":ownerDisabled?.domain == "Fort.OwnerControl" && ownerDisabled?.code == 1,
            "xpcCancellation":canceled, "xpcTimeout":timedOut,
            "xpcMalformedResponseRejected":await invalid(request("/v1/invalid")),
            "xpcInvalidStatusRejected":await invalid(request("/v1/status")),
            "xpcOversizedResponseRejected":oversizedBody && oversizedFrame,
            "xpcOversizedRequestRejected":await invalid(request(body:Data(repeating:65,count:2 * 1024 * 1024)))]
    for (key,value) in try await probeCoreXPC() { proof[key] = value }
    return proof
}
