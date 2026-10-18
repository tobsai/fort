import Foundation
import Darwin

private final class Channel: NSObject, FortSandboxChannel {
    func echo(_ text: String, reply: @escaping (String, Int32) -> Void) {
        guard text.utf8.count <= 4096 else { reply("", getpid()); return }
        reply(text, getpid())
    }
}

private final class Listener: NSObject, NSXPCListenerDelegate {
    let requirement: String
    init(requirement: String) { self.requirement = requirement }
    func listener(_ listener: NSXPCListener, shouldAcceptNewConnection connection: NSXPCConnection) -> Bool {
        guard connection.effectiveUserIdentifier == geteuid() else { return false }
        connection.setCodeSigningRequirement(requirement)
        connection.exportedInterface = NSXPCInterface(with: FortSandboxChannel.self)
        connection.exportedObject = Channel()
        connection.resume()
        return true
    }
}

@main private enum ServiceMain {
    static func main() {
        guard let requirement = Bundle.main.object(forInfoDictionaryKey:"FortClientRequirement") as? String else { exit(2) }
        DispatchQueue.global().asyncAfter(deadline:.now() + 8) { exit(0) }
        let delegate = Listener(requirement:requirement)
        let listener = NSXPCListener.service()
        listener.delegate = delegate
        withExtendedLifetime(delegate) { listener.resume() }
    }
}
