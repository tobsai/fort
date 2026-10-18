import Foundation
import Darwin

private final class Channel: NSObject, FortHostChannel {
    private var pending: [(Data?, NSError?) -> Void] = []
    func setupOwner(_ data: Data, reply: @escaping (Data?, NSError?) -> Void) {
        reply(nil,ownerSetupUnavailable())
    }
    func recoverPairing(_ data: Data, reply: @escaping (Data?, NSError?) -> Void) {
        reply(nil,ownerControlUnavailable())
    }
    func exchange(_ data: Data, reply: @escaping (Data?, NSError?) -> Void) {
        do {
            guard data.count <= 8192 else { throw ClientError.invalidResponse }
            let request = try JSONDecoder().decode(FortRequest.self,from:data)
            if request.path == "/v1/hold" { pending.append(reply); return }
            if request.path == "/v1/invalid" { reply(Data("invalid-json".utf8),nil); return }
            if request.path == "/v1/status" { reply(try JSONEncoder().encode(FortResponse(status:999,body:Data())),nil); return }
            if request.path == "/v1/large-body" { reply(try JSONEncoder().encode(FortResponse(status:200,body:Data(repeating:65,count:8 * 1024 * 1024 + 1))),nil); return }
            if request.path == "/v1/large-frame" { reply(Data(repeating:65,count:12 * 1024 * 1024 + 1),nil); return }
            guard request.path == "/v1/probe?fixture=exact", request.method == .post, request.token == nil,
                  let body = request.body, body.count <= 4096, let text = String(data:body,encoding:.utf8) else { throw ClientError.invalidResponse }
            let response = try JSONSerialization.data(withJSONObject:["text":text,"installation":request.installationID,"pid":getpid()])
            reply(try JSONEncoder().encode(FortResponse(status:200,body:response)),nil)
        } catch { reply(nil,error as NSError) }
    }
}

private final class Listener: NSObject, NSXPCListenerDelegate {
    let requirement: String
    init(requirement: String) { self.requirement = requirement }
    func listener(_ listener: NSXPCListener, shouldAcceptNewConnection connection: NSXPCConnection) -> Bool {
        guard connection.effectiveUserIdentifier == geteuid() else { return false }
        connection.setCodeSigningRequirement(requirement)
        connection.exportedInterface = NSXPCInterface(with: FortHostChannel.self)
        connection.exportedObject = Channel()
        connection.resume()
        return true
    }
}

private struct RejectingTransport: FortTransport {
    func send(_ request: FortRequest) async throws -> FortResponse { throw ClientError.invalidResponse }
}

@main private enum ServiceMain {
    static func main() {
        guard let requirement = Bundle.main.object(forInfoDictionaryKey:"FortClientRequirement") as? String else { exit(2) }
        DispatchQueue.global().asyncAfter(deadline:.now() + 8) { exit(0) }
        let delegate: any NSXPCListenerDelegate
        if let client = Bundle.main.object(forInfoDictionaryKey:"FortClientIdentifier") as? String, client.hasSuffix(".wrong-client"),
           let team = Bundle.main.object(forInfoDictionaryKey:"FortSigningTeam") as? String {
            guard let pinned = try? MacHostListener(clientIdentifier:client,signingTeam:team,transport:RejectingTransport()) else { exit(2) }
            delegate = pinned
        } else { delegate = Listener(requirement:requirement) }
        let listener = NSXPCListener.service()
        listener.delegate = delegate
        withExtendedLifetime(delegate) { listener.resume() }
    }
}
