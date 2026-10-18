import Foundation
import CryptoKit
import Darwin

public enum SavedHistoryRemovalError: Error, LocalizedError {
    case unsafeStore, changedStore, limitExceeded
    public var errorDescription: String? {
        switch self {
        case .unsafeStore: "Saved history could not be safely identified. Deletion is incomplete."
        case .changedStore: "Saved history or pairing changed. Deletion is incomplete; retry after checking this installation."
        case .limitExceeded: "This saved history exceeds the supported deletion limit. Deletion is incomplete."
        }
    }
}

private final class HistoryDirectory {
    let descriptor: Int32
    let path: String
    private let identity: stat
    init(path: String, parent: Int32 = AT_FDCWD, name: String? = nil) throws {
        descriptor = openat(parent,name ?? path,O_RDONLY | O_DIRECTORY | O_NOFOLLOW | O_CLOEXEC)
        guard descriptor >= 0 else { throw SavedHistoryRemovalError.unsafeStore }
        var info = stat()
        guard fstat(descriptor,&info) == 0, info.st_mode & S_IFMT == S_IFDIR,
              info.st_uid == geteuid(), info.st_mode & 0o777 == 0o700 else {
            Darwin.close(descriptor); throw SavedHistoryRemovalError.unsafeStore
        }
        do {
            try noHistoryACL(descriptor)
            var current = stat()
            guard lstat(path,&current) == 0, current.st_dev == info.st_dev, current.st_ino == info.st_ino,
                  current.st_mode == info.st_mode else { throw SavedHistoryRemovalError.changedStore }
        } catch { Darwin.close(descriptor); throw error }
        self.path = path; identity = info
    }
    deinit { Darwin.close(descriptor) }
    func verify() throws {
        var current = stat()
        guard lstat(path,&current) == 0, current.st_mode == identity.st_mode,
              current.st_dev == identity.st_dev, current.st_ino == identity.st_ino,
              current.st_uid == geteuid() else { throw SavedHistoryRemovalError.changedStore }
        try noHistoryACL(descriptor)
    }
    func names() throws -> [String] {
        try verify()
        let copy = dup(descriptor)
        guard copy >= 0 else { throw SavedHistoryRemovalError.unsafeStore }
        guard let stream = fdopendir(copy) else { Darwin.close(copy); throw SavedHistoryRemovalError.unsafeStore }
        defer { closedir(stream) }
        rewinddir(stream)
        var names: [String] = []
        while true {
            errno = 0
            guard let entry = readdir(stream) else {
                guard errno == 0 else { throw SavedHistoryRemovalError.unsafeStore }
                return names.sorted()
            }
            let name = withUnsafePointer(to:&entry.pointee.d_name) {
                $0.withMemoryRebound(to:CChar.self,capacity:Int(entry.pointee.d_namlen)+1) { String(validatingCString:$0) }
            }
            guard let name else { throw SavedHistoryRemovalError.unsafeStore }
            if name != "." && name != ".." { names.append(name) }
            guard names.count <= 4096 else { throw SavedHistoryRemovalError.limitExceeded }
        }
    }
    func child(_ name: String) throws -> HistoryDirectory {
        try verify()
        return try HistoryDirectory(path:path + "/" + name,parent:descriptor,name:name)
    }
}
private func noHistoryACL(_ descriptor: Int32) throws {
    guard let acl = acl_get_fd_np(descriptor,ACL_TYPE_EXTENDED) else {
        if errno == ENOENT { return }
        throw SavedHistoryRemovalError.unsafeStore
    }
    defer { acl_free(UnsafeMutableRawPointer(acl)) }
    var entry: acl_entry_t?
    guard acl_get_entry(acl,Int32(ACL_FIRST_ENTRY.rawValue),&entry) == -1, errno == EINVAL else { throw SavedHistoryRemovalError.unsafeStore }
}
private func historyDigest(_ data: Data) -> String { SHA256.hash(data:data).map { String(format:"%02x",$0) }.joined() }
private func historyKey(_ value: String) -> String { historyDigest(Data(value.utf8)) }
private func isHistoryKey(_ value: String) -> Bool { value.count == 64 && value.utf8.allSatisfy { (48...57).contains($0) || (97...102).contains($0) } }
private struct HistoryFile {
    let directory: HistoryDirectory
    let name: String
    let identity: stat
    let digest: String
    static func read(directory: HistoryDirectory, name: String) throws -> (Self, Data) {
        try directory.verify()
        let fd = openat(directory.descriptor,name,O_RDONLY | O_NOFOLLOW | O_CLOEXEC | O_NONBLOCK)
        guard fd >= 0 else { throw SavedHistoryRemovalError.unsafeStore }
        defer { Darwin.close(fd) }
        var info = stat()
        guard fstat(fd,&info) == 0, info.st_mode & S_IFMT == S_IFREG, info.st_uid == geteuid(),
              info.st_mode & 0o077 == 0, info.st_nlink == 1, info.st_size >= 0, info.st_size <= 16 * 1024 * 1024 else { throw SavedHistoryRemovalError.unsafeStore }
        try noHistoryACL(fd)
        var data = Data(), buffer = [UInt8](repeating:0,count:8192)
        while true {
            let count = Darwin.read(fd,&buffer,buffer.count)
            if count < 0 { if errno == EINTR { continue }; throw SavedHistoryRemovalError.unsafeStore }
            if count == 0 { break }
            guard data.count + count <= 16 * 1024 * 1024 else { throw SavedHistoryRemovalError.limitExceeded }
            data.append(contentsOf:buffer.prefix(count))
        }
        guard data.count == info.st_size else { throw SavedHistoryRemovalError.changedStore }
        return (Self(directory:directory,name:name,identity:info,digest:historyDigest(data)),data)
    }
    func verify() throws {
        let (current,_) = try Self.read(directory:directory,name:name)
        guard current.identity.st_dev == identity.st_dev, current.identity.st_ino == identity.st_ino,
              current.identity.st_mode == identity.st_mode, current.digest == digest else { throw SavedHistoryRemovalError.changedStore }
    }
}
private func savedHistory(in root: HistoryDirectory, installationID: String) throws -> [HistoryFile] {
    var files: [HistoryFile] = [], entries = 0, bytes = 0
    let decoder = JSONDecoder()
    func names(_ directory: HistoryDirectory) throws -> [String] {
        let result = try directory.names(); entries += result.count
        guard entries <= 4096 else { throw SavedHistoryRemovalError.limitExceeded }
        return result
    }
    func read(_ directory: HistoryDirectory, _ name: String) throws -> (HistoryFile,Data) {
        let value = try HistoryFile.read(directory:directory,name:name); bytes += value.1.count
        guard bytes <= 256 * 1024 * 1024 else { throw SavedHistoryRemovalError.limitExceeded }
        return value
    }
    let rootNames = try names(root)
    for name in rootNames where name.hasPrefix("snapshot-") {
        guard name.hasSuffix(".json"), isHistoryKey(String(name.dropFirst(9).dropLast(5))) else { throw SavedHistoryRemovalError.unsafeStore }
        let (file,data) = try read(root,name)
        let owner: String, conversation: String
        if let navigation = try? decoder.decode(SavedNavigation.self,from:data) {
            owner = navigation.installation.installationId; conversation = "navigation"
        } else {
            let snapshot = try decoder.decode(Snapshot.self,from:data)
            owner = snapshot.installationId; conversation = snapshot.conversationId
        }
        guard name == "snapshot-" + historyKey(owner + "/" + conversation) + ".json" else { throw SavedHistoryRemovalError.unsafeStore }
        if owner == installationID { files.append(file) }
    }
    let pendingName = historyKey(installationID)
    if rootNames.contains(pendingName) {
        let directory = try root.child(pendingName)
        if try names(directory).contains("pending.json") {
            let (file,data) = try read(directory,"pending.json")
            _ = try decoder.decode([PendingSubmission].self,from:data)
            files.append(file)
        }
    }
    if rootNames.contains("local-images") {
        let images = try root.child("local-images")
        for folder in try names(images) {
            guard isHistoryKey(folder) else { throw SavedHistoryRemovalError.unsafeStore }
            let directory = try images.child(folder)
            for name in try names(directory) {
                let (file,data) = try read(directory,name)
                let image = try decoder.decode(LocalChatImage.self,from:data)
                guard UUID(uuidString:image.id) != nil, name == image.id + ".json",
                      folder == historyKey(image.installationID + "\0" + image.conversationID) else { throw SavedHistoryRemovalError.unsafeStore }
                if image.installationID == installationID { files.append(file) }
            }
        }
    }
    return files
}

extension DeviceStorage {
    public func deleteSavedHistory(for connection: DeviceConnection) throws {
        try connection.validate()
        let records = try OwnerSetupFiles(directory:directory); defer { records.close() }
        func verifyPairing() throws {
            guard try records.read(.pending) == nil, try records.read(.connection) == nil,
                  let disconnected = try records.read(.disconnected),
                  try JSONDecoder().decode(DeviceConnection.self,from:disconnected) == connection,
                  let removal = try records.read(.removal),
                  try ownerSetupCredential(for:connection) == nil else { throw SavedHistoryRemovalError.changedStore }
            let intent = try JSONDecoder().decode(PairingRemovalIntent.self,from:removal)
            guard intent.connection == connection, intent.state == .confirmed else { throw SavedHistoryRemovalError.changedStore }
        }
        try verifyPairing()
        let root = try HistoryDirectory(path:directory.path)
        let files = try savedHistory(in:root,installationID:connection.installationID)
        try verifyPairing()
        for file in files { try file.verify() }
        for file in files {
            try verifyPairing(); try root.verify(); try file.verify()
            guard unlinkat(file.directory.descriptor,file.name,0) == 0, fsync(file.directory.descriptor) == 0 else { throw SavedHistoryRemovalError.changedStore }
        }
        try verifyPairing()
        guard try savedHistory(in:root,installationID:connection.installationID).isEmpty else { throw SavedHistoryRemovalError.changedStore }
    }
}
