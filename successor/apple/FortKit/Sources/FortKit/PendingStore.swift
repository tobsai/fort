import Foundation
import CryptoKit

public enum PendingStoreError: Error { case textTooLarge, invalidIdentity }

public struct PendingSubmission: Codable, Equatable, Sendable, Identifiable {
    public var id: String { requestID }
    public let requestID: String
    public let conversationID: String
    public let bindingRevision: String
    public let recoveryEpoch: String
    public let text: String
    public let createdAt: Date
    public func canRetry(in epoch: String) -> Bool { recoveryEpoch == epoch }
}

/// One device-owned store; callers serialize mutations on their UI actor.
public final class PendingStore {
    private let file: URL
    public init(directory: URL, installationID: String) throws {
        guard !installationID.isEmpty else { throw PendingStoreError.invalidIdentity }
        let key = SHA256.hash(data: Data(installationID.utf8)).map { String(format: "%02x", $0) }.joined()
        let scoped = directory.appendingPathComponent(key, isDirectory: true)
        try FileManager.default.createDirectory(at: scoped, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
        file = scoped.appendingPathComponent("pending.json")
    }
    public func pending() throws -> [PendingSubmission] {
        guard FileManager.default.fileExists(atPath: file.path) else { return [] }
        return try JSONDecoder().decode([PendingSubmission].self, from: Data(contentsOf: file))
    }
    @discardableResult public func enqueue(conversationID: String, bindingRevision: String, recoveryEpoch: String, text: String) throws -> PendingSubmission {
        guard text.utf8.count <= 65_536 else { throw PendingStoreError.textTooLarge }
        let item = PendingSubmission(requestID: UUID().uuidString.lowercased(), conversationID: conversationID, bindingRevision: bindingRevision, recoveryEpoch: recoveryEpoch, text: text, createdAt: Date())
        var items = try pending()
        items.append(item)
        try write(items)
        return item
    }
    public func remove(requestID: String) throws { try write(pending().filter { $0.requestID != requestID }) }
    private func write(_ items: [PendingSubmission]) throws {
        try JSONEncoder().encode(items).write(to: file, options: .atomic)
        try FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: file.path)
        #if os(iOS)
        try FileManager.default.setAttributes([.protectionKey: FileProtectionType.completeUntilFirstUserAuthentication], ofItemAtPath: file.path)
        #endif
    }
}
