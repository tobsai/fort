#if os(macOS)
import Foundation

public struct MacHostProgram: Sendable {
    public let bundle: MacHostBundle
    public let executable: URL
    public let guardian: URL
    public let bootstrap: URL
    public let codex: URL
    public var historyMaintenance: URL { bootstrap.deletingLastPathComponent().appendingPathComponent("history-maintenance.mjs") }
    public var opensslConfiguration: URL { bootstrap.deletingLastPathComponent().appendingPathComponent("node-openssl.cnf") }

    public static func inspect() async throws -> Self {
        try Task.checkCancellation()
        let helper = Bundle.main.bundleURL
        let result = try await Task.detached { try inspect(helperURL:helper,signature:MacHostBundle.codeSignature) }.value
        try Task.checkCancellation()
        return result
    }

    static func inspect(helperURL: URL, signature: (URL,String,String) throws -> MacHostCodeSignature) throws -> Self {
        let bundle = try MacHostBundle.inspectHelper(helperURL:helperURL,signature:signature)
        guard bundle.helperURL == helperURL else { throw MacHostBundleError.configuration }
        let node = helperURL.appendingPathComponent("Contents/MacOS/node")
        let guardian = helperURL.appendingPathComponent("Contents/MacOS/FortProcessGuardian")
        let codex = helperURL.appendingPathComponent("Contents/MacOS/codex")
        let bootstrap = helperURL.appendingPathComponent("Contents/Resources/host/main.mjs")
        for (url,suffix,limit) in [(node,"node",512 * 1024 * 1024),(guardian,"guardian",8 * 1024 * 1024),(codex,"codex",512 * 1024 * 1024)] {
            try MacHostBundle.withFile(url,limit:limit) { _, info in
                guard info.st_mode & 0o111 != 0 else { throw MacHostBundleError.unsafeFile }
            }
            let code = try signature(url,bundle.endpoint.serviceIdentifier + "." + suffix,bundle.endpoint.signingTeam)
            guard code.appSandbox, code.inheritsSandbox, code.applicationGroups.isEmpty,
                  Set(code.entitlementKeys) == ["com.apple.security.app-sandbox","com.apple.security.inherit"] else { throw MacHostBundleError.entitlements }
        }
        for entry in [bootstrap,bootstrap.deletingLastPathComponent().appendingPathComponent("history-maintenance.mjs")] {
            try MacHostBundle.withFile(entry,limit:256 * 1024) { _, _ in }
        }
        guard try MacHostBundle.read(bootstrap.deletingLastPathComponent().appendingPathComponent("node-openssl.cnf"),limit:1024) == Data("nodejs_conf = fort_node\n[fort_node]\n".utf8) else { throw MacHostBundleError.configuration }
        return Self(bundle:bundle,executable:node,guardian:guardian,bootstrap:bootstrap,codex:codex)
    }

    @MainActor public func makeService() -> MacAgentService {
        MacAgentService(program:self)
    }
}
#endif
