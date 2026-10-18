import Foundation
import Security
import CryptoKit

struct SavedNavigation: Codable {
    let installation: Installation
    let agents: [Agent]
    let selectedAgentID: String?
    var conversations: [Conversation]? = nil
    var skills: [InstructionSkill]? = nil
    var selectedConversationID: String? = nil
    var recipientAgentID: String? = nil
}
public enum DeviceStorageError: Error { case keychain(OSStatus), removalUnavailable }
protocol DeviceCredentials {
    func save(_ connection: DeviceConnection, token: String) throws
    func token(for connection: DeviceConnection) throws -> String
    func remove(_ connection: DeviceConnection) throws
}
extension DeviceCredentials {
    func remove(_ connection: DeviceConnection) throws { throw DeviceStorageError.removalUnavailable }
}
private struct KeychainCredentials: DeviceCredentials {
    public func save(_ connection: DeviceConnection, token: String) throws {
        let account = "\(connection.installationID)/\(connection.deviceID)"
        let query: [String: Any] = [kSecClass as String:kSecClassGenericPassword,kSecAttrService as String:"Fort.Spec058.Device",kSecAttrAccount as String:account]
        let data = Data(token.utf8)
        let status = SecItemUpdate(query as CFDictionary, [kSecValueData as String:data] as CFDictionary)
        if status == errSecItemNotFound {
            var add = query; add[kSecValueData as String] = data; add[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
            let inserted = SecItemAdd(add as CFDictionary, nil)
            guard inserted == errSecSuccess else { throw DeviceStorageError.keychain(inserted) }
        } else if status != errSecSuccess { throw DeviceStorageError.keychain(status) }
    }
    public func remove(_ connection: DeviceConnection) throws {
        let query: [String:Any] = [kSecClass as String:kSecClassGenericPassword,kSecAttrService as String:"Fort.Spec058.Device",kSecAttrAccount as String:"\(connection.installationID)/\(connection.deviceID)"]
        let status = SecItemDelete(query as CFDictionary)
        guard status == errSecSuccess || status == errSecItemNotFound else { throw DeviceStorageError.keychain(status) }
    }
    public func token(for connection: DeviceConnection) throws -> String {
        let query: [String: Any] = [kSecClass as String:kSecClassGenericPassword,kSecAttrService as String:"Fort.Spec058.Device",kSecAttrAccount as String:"\(connection.installationID)/\(connection.deviceID)",kSecReturnData as String:true,kSecMatchLimit as String:kSecMatchLimitOne]
        var result: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &result)
        guard status == errSecSuccess, let data = result as? Data, let text = String(data:data, encoding:.utf8) else { throw DeviceStorageError.keychain(status) }
        return text
    }
}

public final class DeviceStorage {
    public let directory: URL
    private let credentials: any DeviceCredentials
    public convenience init(directory: URL) throws { try self.init(directory:directory,credentials:KeychainCredentials()) }
    init(directory: URL, credentials: any DeviceCredentials) throws {
        self.directory = directory
        self.credentials = credentials
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
    }
    public func connection() throws -> DeviceConnection? {
        let file = directory.appendingPathComponent("connection.json")
        guard FileManager.default.fileExists(atPath: file.path) else { return nil }
        return try JSONDecoder().decode(DeviceConnection.self, from: Data(contentsOf: file))
    }
    public func save(_ connection: DeviceConnection, token: String) throws {
        let record = try JSONEncoder().encode(connection)
        try credentials.save(connection,token:token)
        try protectedWrite(record,to:directory.appendingPathComponent("connection.json"))
    }
    public func token(for connection: DeviceConnection) throws -> String { try connection.validate(); return try credentials.token(for:connection) }
    func ownerSetupCredential(for connection: DeviceConnection) throws -> String? {
        do { return try token(for:connection) }
        catch DeviceStorageError.keychain(errSecItemNotFound) { return nil }
    }
    #if os(macOS)
    func removeOwnerSetupCredential(_ connection: DeviceConnection, expectedToken: String?) throws {
        guard try ownerSetupCredential(for:connection) == expectedToken else { throw OwnerSetupPreparationError.changedPreparation }
        if expectedToken != nil { try credentials.remove(connection) }
        guard try ownerSetupCredential(for:connection) == nil else { throw OwnerSetupPreparationError.changedPreparation }
    }
    static func makeOwnerDeviceToken() throws -> String {
        var bytes = [UInt8](repeating:0,count:32)
        guard SecRandomCopyBytes(kSecRandomDefault,bytes.count,&bytes) == errSecSuccess else { throw OwnerSetupPreparationError.randomUnavailable }
        return Data(bytes).base64EncodedString().replacingOccurrences(of:"+",with:"-").replacingOccurrences(of:"/",with:"_").replacingOccurrences(of:"=",with:"")
    }
    func prepareOwnerSetupCredential(connection: DeviceConnection, setupID: String, mayCreate: Bool, tokenGenerator: () throws -> String) throws -> OwnerSetupRequest {
        var created = false
        let token: String
        do { token = try credentials.token(for:connection) }
        catch DeviceStorageError.keychain(errSecItemNotFound) {
            guard mayCreate else { throw OwnerSetupPreparationError.missingCredential }
            token = try tokenGenerator(); created = true
        }
        let request = try OwnerSetupRequest(installationID:connection.installationID,setupID:setupID,deviceID:connection.deviceID,deviceToken:token)
        if created { try credentials.save(connection,token:token) }
        return request
    }
    #endif
    public func disconnectedConnection() throws -> DeviceConnection? {
        let file = directory.appendingPathComponent("disconnected.json")
        guard FileManager.default.fileExists(atPath:file.path) else { return nil }
        return try JSONDecoder().decode(DeviceConnection.self,from:Data(contentsOf:file))
    }
    public func pairingRemoval() throws -> PairingRemovalIntent? {
        let file = directory.appendingPathComponent("pairing-removal.json")
        guard FileManager.default.fileExists(atPath:file.path) else { return nil }
        return try JSONDecoder().decode(PairingRemovalIntent.self,from:Data(contentsOf:file))
    }
    public func beginPairingRemoval(_ expected: DeviceConnection) throws {
        guard try connection() == expected else { throw OwnerLifecycleError.wrongPairing }
        if let prior = try pairingRemoval(), prior.connection == expected { return }
        try protectedWrite(JSONEncoder().encode(PairingRemovalIntent(connection:expected,state:.requested)),to:directory.appendingPathComponent("pairing-removal.json"))
    }
    public func removePairing(_ expected: DeviceConnection, revocationConfirmed: Bool = true) throws {
        let current = try connection()
        let disconnected = try disconnectedConnection()
        guard current == expected || (current == nil && disconnected == expected) else { throw OwnerLifecycleError.wrongPairing }
        let prior = try pairingRemoval()
        let confirmed = revocationConfirmed || (prior?.connection == expected && prior?.state == .confirmed)
        try protectedWrite(JSONEncoder().encode(PairingRemovalIntent(connection:expected,state:confirmed ? .confirmed : .locallyForgotten)),to:directory.appendingPathComponent("pairing-removal.json"))
        try protectedWrite(JSONEncoder().encode(expected),to:directory.appendingPathComponent("disconnected.json"))
        try credentials.remove(expected)
        if current != nil { try FileManager.default.removeItem(at:directory.appendingPathComponent("connection.json")) }
    }
    private func snapshotFile(_ installationID: String, _ conversationID: String) -> URL {
        let key = SHA256.hash(data:Data("\(installationID)/\(conversationID)".utf8)).map { String(format:"%02x",$0) }.joined()
        return directory.appendingPathComponent("snapshot-\(key).json")
    }
    func saveNavigation(_ navigation: SavedNavigation) throws {
        try protectedWrite(JSONEncoder().encode(navigation),to:snapshotFile(navigation.installation.installationId,"navigation"))
    }
    func navigation(installationID: String) throws -> SavedNavigation? {
        let file = snapshotFile(installationID,"navigation")
        guard FileManager.default.fileExists(atPath:file.path) else { return nil }
        let saved = try JSONDecoder().decode(SavedNavigation.self,from:Data(contentsOf:file))
        guard saved.installation.installationId == installationID else { throw ContractError.wrongInstallation }
        return saved
    }
    public func saveSnapshot(_ snapshot: Snapshot) throws { try protectedWrite(JSONEncoder().encode(snapshot), to:snapshotFile(snapshot.installationId,snapshot.conversationId)) }
    public func snapshot(installationID: String, conversationID: String) throws -> Snapshot? {
        let file = snapshotFile(installationID,conversationID)
        guard FileManager.default.fileExists(atPath:file.path) else { return nil }
        return try JSONDecoder().decode(Snapshot.self,from:Data(contentsOf:file))
    }
    private func protectedWrite(_ data: Data, to file: URL) throws {
        try data.write(to:file, options:.atomic)
        try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:file.path)
        #if os(iOS)
        try FileManager.default.setAttributes([.protectionKey:FileProtectionType.completeUntilFirstUserAuthentication],ofItemAtPath:file.path)
        #endif
    }
}
