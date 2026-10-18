#if os(macOS)
import Foundation

public struct MacHostProgram: Sendable {
    public let bundle: MacHostBundle
    public let executable: URL
    public let guardian: URL
    public let bootstrap: URL

    public static func inspect() async throws -> Self {
        try Task.checkCancellation()
        let helper = Bundle.main.bundleURL
        let result = try await Task.detached { try inspect(helperURL:helper,signature:MacHostBundle.codeSignature) }.value
        try Task.checkCancellation()
        return result
    }

    static func inspect(helperURL: URL, signature: (URL,String,String) throws -> MacHostCodeSignature) throws -> Self {
        let app = helperURL.deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        let bundle = try MacHostBundle.inspect(applicationURL:app,signature:signature)
        guard bundle.helperURL == helperURL else { throw MacHostBundleError.configuration }
        let node = helperURL.appendingPathComponent("Contents/MacOS/node")
        let guardian = helperURL.appendingPathComponent("Contents/MacOS/FortProcessGuardian")
        let bootstrap = helperURL.appendingPathComponent("Contents/Resources/host/main.mjs")
        for (url,suffix,limit) in [(node,"node",512 * 1024 * 1024),(guardian,"guardian",8 * 1024 * 1024)] {
            try MacHostBundle.withFile(url,limit:limit) { _, info in
                guard info.st_mode & 0o111 != 0 else { throw MacHostBundleError.unsafeFile }
            }
            let code = try signature(url,bundle.endpoint.serviceIdentifier + "." + suffix,bundle.endpoint.signingTeam)
            guard code.appSandbox, code.inheritsSandbox, code.applicationGroups.isEmpty,
                  Set(code.entitlementKeys) == ["com.apple.security.app-sandbox","com.apple.security.inherit"] else { throw MacHostBundleError.entitlements }
        }
        try MacHostBundle.withFile(bootstrap,limit:256 * 1024) { _, _ in }
        return Self(bundle:bundle,executable:node,guardian:guardian,bootstrap:bootstrap)
    }

    @MainActor public func makeService() -> MacAgentService {
        MacAgentService(bundle:bundle,executable:executable,
            arguments:["--jitless","--no-experimental-strip-types","--no-warnings",bootstrap.path],environment:[:],guardian:guardian)
    }
}
#endif
