#if os(macOS)
import Foundation
import Security
import Darwin

public enum MacHostBundleError: Error, LocalizedError {
    case configuration, unsafeFile, signature, entitlements, launchAgent, profiles
    public var errorDescription: String? {
        switch self {
        case .configuration: "This app's local host configuration is missing or inconsistent."
        case .unsafeFile: "The local host bundle layout could not be verified."
        case .signature: "The exact client or host code signature could not be verified."
        case .entitlements: "The client and host must declare their own App Sandbox and exact shared App Group."
        case .launchAgent: "The local host startup definition does not match this app."
        case .profiles: "Both the client and host need their own embedded provisioning profile."
        }
    }
}

struct MacHostCodeSignature: Sendable {
    let appSandbox: Bool
    let inheritsSandbox: Bool
    let applicationGroups: [String]
    let entitlementKeys: [String]
    init(appSandbox: Bool, inheritsSandbox: Bool, applicationGroups: [String], entitlementKeys: [String] = []) {
        self.appSandbox = appSandbox; self.inheritsSandbox = inheritsSandbox
        self.applicationGroups = applicationGroups; self.entitlementKeys = entitlementKeys
    }
}

public struct MacHostBundle: Sendable {
    public enum ProvisioningEvidence: Sendable { case presentUnverified }
    public let applicationURL: URL
    public let helperURL: URL
    public let endpoint: MacAgentEndpoint
    public let clientIdentifier: String
    public let provisioningEvidence = ProvisioningEvidence.presentUnverified
    private static let helperPath = "Contents/Helpers/FortHost.app"
    private static let program = helperPath + "/Contents/MacOS/FortHost"
    private static let manifestPath = "Contents/Resources/fort-host.json"

    public static func inspect(applicationURL: URL) async throws -> Self {
        try Task.checkCancellation()
        let result = try await Task.detached { try inspect(applicationURL:applicationURL,signature:codeSignature) }.value
        try Task.checkCancellation()
        return result
    }

    static func inspect(applicationURL: URL, signature: (URL,String,String) throws -> MacHostCodeSignature) throws -> Self {
        try directory(applicationURL)
        let data = try read(applicationURL.appendingPathComponent(manifestPath),limit:8192)
        let (client,endpoint) = try configuration(data)
        let service = endpoint.serviceIdentifier, team = endpoint.signingTeam, group = endpoint.appGroup
        let helper = applicationURL.appendingPathComponent(helperPath,isDirectory:true)
        try directory(helper)
        guard try read(helper.appendingPathComponent(manifestPath),limit:8192) == data else { throw MacHostBundleError.configuration }
        try withFile(applicationURL.appendingPathComponent(program),limit:Int.max) { _, info in
            guard info.st_mode & 0o111 != 0 else { throw MacHostBundleError.unsafeFile }
        }
        let definition = try read(applicationURL.appendingPathComponent("Contents/Library/LaunchAgents/" + service + ".plist"),limit:16_384)
        guard let plist = try PropertyListSerialization.propertyList(from:definition,format:nil) as? [String:Any],
              Set(plist.keys) == ["Label","BundleProgram","MachServices"], plist["Label"] as? String == service,
              plist["BundleProgram"] as? String == program,
              let services = plist["MachServices"] as? [String:Any], Set(services.keys) == [endpoint.machServiceName],
              (try? boolean(services[endpoint.machServiceName])) == true else { throw MacHostBundleError.launchAgent }
        for (url, identifier) in [(applicationURL,client),(helper,service)] {
            let signed = try signature(url,identifier,team)
            guard signed.appSandbox, !signed.inheritsSandbox, signed.applicationGroups == [group] else { throw MacHostBundleError.entitlements }
            do { try withFile(url.appendingPathComponent("Contents/embedded.provisionprofile"),limit:1_048_576) { _, _ in } }
            catch { throw MacHostBundleError.profiles }
        }
        return Self(applicationURL:applicationURL,helperURL:helper,endpoint:endpoint,clientIdentifier:client)
    }

    private static func configuration(_ data: Data) throws -> (String,MacAgentEndpoint) {
        let fields = try JSONDecoder().decode([String:String].self,from:data)
        guard Set(fields.keys) == ["format","clientIdentifier","serviceIdentifier","signingTeam","appGroup"], fields["format"] == "1",
              let client = fields["clientIdentifier"], let service = fields["serviceIdentifier"],
              let team = fields["signingTeam"], let group = fields["appGroup"], client != service else { throw MacHostBundleError.configuration }
        _ = try macPeerRequirement(identifier:client,signingTeam:team)
        return (client,try MacAgentEndpoint(serviceIdentifier:service,signingTeam:team,appGroup:group))
    }

    static func inspectHelper(helperURL: URL, signature: (URL,String,String) throws -> MacHostCodeSignature) throws -> Self {
        try directory(helperURL)
        let app = helperURL.deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        guard app.appendingPathComponent(helperPath,isDirectory:true) == helperURL else { throw MacHostBundleError.configuration }
        let (client,endpoint) = try configuration(read(helperURL.appendingPathComponent(manifestPath),limit:8192))
        let signed = try signature(helperURL,endpoint.serviceIdentifier,endpoint.signingTeam)
        guard signed.appSandbox, !signed.inheritsSandbox, signed.applicationGroups == [endpoint.appGroup] else { throw MacHostBundleError.entitlements }
        do { try withFile(helperURL.appendingPathComponent("Contents/embedded.provisionprofile"),limit:1_048_576) { _, _ in } }
        catch { throw MacHostBundleError.profiles }
        try withFile(helperURL.appendingPathComponent("Contents/MacOS/FortHost"),limit:Int.max) { _, info in
            guard info.st_mode & 0o111 != 0 else { throw MacHostBundleError.unsafeFile }
        }
        return Self(applicationURL:app,helperURL:helperURL,endpoint:endpoint,clientIdentifier:client)
    }

    static func codeSignature(at url: URL, identifier: String, team: String) throws -> MacHostCodeSignature {
        let expression = try macPeerRequirement(identifier:identifier,signingTeam:team)
        var code: SecStaticCode?, requirement: SecRequirement?
        guard SecStaticCodeCreateWithPath(url as CFURL,SecCSFlags(),&code) == errSecSuccess, let code,
              SecRequirementCreateWithString(expression as CFString,SecCSFlags(),&requirement) == errSecSuccess, let requirement else { throw MacHostBundleError.signature }
        let flags = SecCSFlags(rawValue:kSecCSStrictValidate | kSecCSCheckAllArchitectures | kSecCSCheckNestedCode).union(.noNetworkAccess)
        guard SecStaticCodeCheckValidity(code,flags,requirement) == errSecSuccess else { throw MacHostBundleError.signature }
        var information: CFDictionary?
        guard SecCodeCopySigningInformation(code,SecCSFlags(rawValue:kSecCSSigningInformation),&information) == errSecSuccess,
              let values = information as? [String:Any], values[kSecCodeInfoIdentifier as String] as? String == identifier,
              values[kSecCodeInfoTeamIdentifier as String] as? String == team else { throw MacHostBundleError.signature }
        let entitlements: [String:Any]
        if let raw = values[kSecCodeInfoEntitlementsDict as String] {
            guard let parsed = raw as? [String:Any] else { throw MacHostBundleError.entitlements }
            entitlements = parsed
        } else { entitlements = [:] }
        let groups: [String]
        if let raw = entitlements["com.apple.security.application-groups"] {
            guard let parsed = raw as? [String] else { throw MacHostBundleError.entitlements }
            groups = parsed
        } else { groups = [] }
        return try MacHostCodeSignature(appSandbox:boolean(entitlements["com.apple.security.app-sandbox"]),
            inheritsSandbox:boolean(entitlements["com.apple.security.inherit"]),applicationGroups:groups,entitlementKeys:Array(entitlements.keys))
    }

    private static func boolean(_ value: Any?) throws -> Bool {
        guard let value else { return false }
        guard let number = value as? NSNumber, CFGetTypeID(number) == CFBooleanGetTypeID() else { throw MacHostBundleError.entitlements }
        return number.boolValue
    }
    private static func path(_ url: URL) throws {
        guard url.isFileURL, url.host == nil || url.host == "", url.path.hasPrefix("/"), url.path != "/", !url.path.contains("\0"),
              url.standardizedFileURL.path == url.path, url.resolvingSymlinksInPath().path == url.path else { throw MacHostBundleError.unsafeFile }
    }
    private static func directory(_ url: URL) throws {
        try path(url)
        var info = stat()
        guard lstat(url.path,&info) == 0, info.st_mode & S_IFMT == S_IFDIR, info.st_mode & 0o022 == 0 else { throw MacHostBundleError.unsafeFile }
    }
    static func withFile<T>(_ url: URL, limit: Int, body: (Int32,stat) throws -> T) throws -> T {
        try path(url)
        let descriptor = Darwin.open(url.path,O_RDONLY | O_NOFOLLOW | O_NONBLOCK | O_CLOEXEC)
        guard descriptor >= 0 else { throw MacHostBundleError.unsafeFile }
        defer { Darwin.close(descriptor) }
        var info = stat()
        guard fstat(descriptor,&info) == 0, info.st_mode & S_IFMT == S_IFREG, info.st_nlink == 1,
              info.st_size > 0, info.st_size <= limit, info.st_mode & 0o022 == 0 else { throw MacHostBundleError.unsafeFile }
        return try body(descriptor,info)
    }
    static func read(_ url: URL, limit: Int) throws -> Data {
        try withFile(url,limit:limit) { descriptor, info in
            let data = try FileHandle(fileDescriptor:descriptor,closeOnDealloc:false).read(upToCount:limit + 1) ?? Data()
            var after = stat()
            guard data.count == info.st_size, fstat(descriptor,&after) == 0,
                  after.st_size == info.st_size, after.st_mtimespec.tv_sec == info.st_mtimespec.tv_sec,
                  after.st_mtimespec.tv_nsec == info.st_mtimespec.tv_nsec,
                  after.st_ctimespec.tv_sec == info.st_ctimespec.tv_sec,
                  after.st_ctimespec.tv_nsec == info.st_ctimespec.tv_nsec else { throw MacHostBundleError.unsafeFile }
            return data
        }
    }
}
#endif
