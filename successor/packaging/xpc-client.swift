import Foundation
import Darwin

private enum ChannelReply {
    case value(String, Int32)
    case rejected(Int)
}
private final class ReplyBox: @unchecked Sendable {
    let signal = DispatchSemaphore(value:0)
    private let lock = NSLock()
    private var value: ChannelReply?
    func finish(_ result: ChannelReply) {
        lock.lock(); defer { lock.unlock() }
        guard value == nil else { return }
        value = result; signal.signal()
    }
    func result() -> ChannelReply? { lock.lock(); defer { lock.unlock() }; return value }
}

private func exchange(service: String, requirement: String, text: String) -> ChannelReply? {
    let box = ReplyBox()
    let connection = NSXPCConnection(serviceName:service)
    connection.remoteObjectInterface = NSXPCInterface(with:FortSandboxChannel.self)
    connection.setCodeSigningRequirement(requirement)
    connection.resume()
    defer { connection.invalidate() }
    let proxy = connection.remoteObjectProxyWithErrorHandler { error in
        let error = error as NSError
        box.finish(.rejected(error.domain == NSCocoaErrorDomain ? error.code : -1))
    }
    guard let channel = proxy as? FortSandboxChannel else { return nil }
    channel.echo(text) { value, pid in box.finish(.value(value,pid)) }
    guard box.signal.wait(timeout:.now() + 3) == .success else { return nil }
    return box.result()
}

func probeXPC() throws -> [String: Any] {
    let bundle = Bundle.main
    guard let identifier = bundle.bundleIdentifier,
          let team = bundle.object(forInfoDictionaryKey:"FortSigningTeam") as? String else { throw CocoaError(.coderValueNotFound) }
    let allowedService = identifier + ".channel"
    let deniedService = identifier + ".rejected"
    let pin: (String) -> String = { "anchor apple generic and identifier \"\($0)\" and certificate leaf[subject.OU] = \"\(team)\"" }
    let text = "  Local cedar 🌲\r\n"
    let accepted = exchange(service:allowedService,requirement:pin(allowedService),text:text)
    let deniedClient = exchange(service:deniedService,requirement:pin(deniedService),text:text)
    let deniedServiceReply = exchange(service:allowedService,requirement:pin(identifier + ".wrong-service"),text:text)
    let errorCode: (ChannelReply?) -> Int? = { if case .rejected(let code) = $0 { return code }; return nil }
    var exact = false, pid: Int32 = 0
    if case .value(let response,let process) = accepted { exact = response == text && process > 1 && process != getpid(); pid = process }
    return ["xpcRoundTrip":exact, "xpcClientRejected":errorCode(deniedClient) == NSXPCConnectionInterrupted,
            "xpcServiceRejected":errorCode(deniedServiceReply) == NSXPCConnectionCodeSigningRequirementFailure,
            "xpcClientError":errorCode(deniedClient) ?? -1, "xpcServiceError":errorCode(deniedServiceReply) ?? -1, "xpcServicePid":pid]
}
