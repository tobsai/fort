import Foundation
import Security
import CryptoKit

public struct DeviceConnection: Codable, Sendable {
    public let baseURL: URL
    public let installationID: String
    public let deviceID: String
    public let fixtureLoopback: Bool
}
public enum DeviceStorageError: Error { case keychain(OSStatus) }
public final class DeviceStorage {
    public let directory: URL
    public init(directory: URL) throws {
        self.directory = directory
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
    }
    public func connection() throws -> DeviceConnection? {
        let file = directory.appendingPathComponent("connection.json")
        guard FileManager.default.fileExists(atPath: file.path) else { return nil }
        return try JSONDecoder().decode(DeviceConnection.self, from: Data(contentsOf: file))
    }
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
        try protectedWrite(JSONEncoder().encode(connection), to: directory.appendingPathComponent("connection.json"))
    }
    public func token(for connection: DeviceConnection) throws -> String {
        let query: [String: Any] = [kSecClass as String:kSecClassGenericPassword,kSecAttrService as String:"Fort.Spec058.Device",kSecAttrAccount as String:"\(connection.installationID)/\(connection.deviceID)",kSecReturnData as String:true,kSecMatchLimit as String:kSecMatchLimitOne]
        var result: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &result)
        guard status == errSecSuccess, let data = result as? Data, let text = String(data:data, encoding:.utf8) else { throw DeviceStorageError.keychain(status) }
        return text
    }
    private func snapshotFile(_ installationID: String, _ conversationID: String) -> URL {
        let key = SHA256.hash(data:Data("\(installationID)/\(conversationID)".utf8)).map { String(format:"%02x",$0) }.joined()
        return directory.appendingPathComponent("snapshot-\(key).json")
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
