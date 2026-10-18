import Foundation
import Darwin

final class OwnerSetupFiles {
    enum Record: String { case hostRemoval = "host-removal.json", pending = "owner-setup.json", connection = "connection.json", removal = "pairing-removal.json", disconnected = "disconnected.json" }
    private let path: String
    private let identity: stat
    private var directory: Int32
    private var lock: Int32
    private let limit = 16 * 1024
    private static func ioError() -> NSError { NSError(domain:NSPOSIXErrorDomain,code:Int(errno)) }
    private static func noACL(_ descriptor: Int32) throws {
        errno = 0
        guard let acl = acl_get_fd_np(descriptor,ACL_TYPE_EXTENDED) else {
            if errno == ENOENT { return }
            throw ioError()
        }
        defer { acl_free(UnsafeMutableRawPointer(acl)) }
        var entry: acl_entry_t?
        guard acl_get_entry(acl,Int32(ACL_FIRST_ENTRY.rawValue),&entry) == -1, errno == EINVAL else { throw ClientError.invalidResponse }
    }
    private static func regular(_ descriptor: Int32) throws -> stat {
        var info = stat()
        guard fstat(descriptor,&info) == 0 else { throw ioError() }
        guard info.st_mode & S_IFMT == S_IFREG, info.st_uid == geteuid(), info.st_mode & 0o077 == 0, info.st_nlink == 1 else { throw ClientError.invalidResponse }
        try noACL(descriptor)
        return info
    }
    init(directory url: URL) throws {
        guard url.isFileURL, url.path.hasPrefix("/") else { throw ClientError.invalidResponse }
        let directory = Darwin.open(url.path,O_RDONLY | O_DIRECTORY | O_NOFOLLOW | O_CLOEXEC)
        guard directory >= 0 else { throw Self.ioError() }
        var lock: Int32 = -1, accepted = false
        defer { if !accepted { if lock >= 0 { Darwin.close(lock) }; Darwin.close(directory) } }
        var identity = stat()
        guard fstat(directory,&identity) == 0 else { throw Self.ioError() }
        guard identity.st_mode & S_IFMT == S_IFDIR, identity.st_uid == geteuid(), identity.st_mode & 0o077 == 0 else { throw ClientError.invalidResponse }
        try Self.noACL(directory)
        lock = openat(directory,"owner-setup.lock",O_RDWR | O_CREAT | O_NOFOLLOW | O_CLOEXEC | O_NONBLOCK,0o600)
        guard lock >= 0 else { throw Self.ioError() }
        _ = try Self.regular(lock)
        if flock(lock,LOCK_EX | LOCK_NB) != 0 {
            if errno == EWOULDBLOCK { throw OwnerSetupPreparationError.busy }
            throw Self.ioError()
        }
        self.directory = directory; self.lock = lock; self.path = url.path; self.identity = identity
        accepted = true
    }
    private func verifyDirectory() throws {
        guard directory >= 0 else { throw ClientError.invalidResponse }
        var current = stat()
        guard lstat(path,&current) == 0 else { throw Self.ioError() }
        guard current.st_mode & S_IFMT == S_IFDIR, current.st_dev == identity.st_dev, current.st_ino == identity.st_ino,
              current.st_uid == geteuid(), current.st_mode & 0o077 == 0 else { throw OwnerSetupPreparationError.changedPreparation }
        try Self.noACL(directory)
        _ = try Self.regular(lock)
    }
    func read(_ record: Record) throws -> Data? {
        try verifyDirectory()
        let file = openat(directory,record.rawValue,O_RDONLY | O_NOFOLLOW | O_CLOEXEC | O_NONBLOCK)
        if file < 0 { if errno == ENOENT { return nil }; throw Self.ioError() }
        defer { Darwin.close(file) }
        guard try Self.regular(file).st_size <= limit else { throw ClientError.invalidResponse }
        var result = Data(), bytes = [UInt8](repeating:0,count:4096)
        while true {
            let count = Darwin.read(file,&bytes,bytes.count)
            if count < 0 { if errno == EINTR { continue }; throw Self.ioError() }
            if count == 0 { return result }
            guard result.count + count <= limit else { throw ClientError.invalidResponse }
            result.append(contentsOf:bytes.prefix(count))
        }
    }
    func write(_ data: Data, to record: Record) throws {
        guard !data.isEmpty, data.count <= limit else { throw ClientError.invalidResponse }
        _ = try read(record)
        let temporary = ".owner-setup-" + UUID().uuidString + ".tmp"
        let file = openat(directory,temporary,O_WRONLY | O_CREAT | O_EXCL | O_NOFOLLOW | O_CLOEXEC,0o600)
        guard file >= 0 else { throw Self.ioError() }
        defer { Darwin.close(file); _ = unlinkat(directory,temporary,0) }
        try data.withUnsafeBytes { bytes in
            guard let base = bytes.baseAddress else { throw ClientError.invalidResponse }
            var offset = 0
            while offset < bytes.count {
                let count = Darwin.write(file,base.advanced(by:offset),bytes.count - offset)
                if count < 0 { if errno == EINTR { continue }; throw Self.ioError() }
                guard count > 0 else { throw ClientError.invalidResponse }
                offset += count
            }
        }
        guard fcntl(file,F_FULLFSYNC) == 0 else { throw Self.ioError() }
        try verifyDirectory()
        guard renameat(directory,temporary,directory,record.rawValue) == 0, fsync(directory) == 0 else { throw Self.ioError() }
    }
    private func remove(_ record: Record) throws {
        guard try read(record) != nil else { throw OwnerSetupPreparationError.changedPreparation }
        guard unlinkat(directory,record.rawValue,0) == 0, fsync(directory) == 0 else { throw Self.ioError() }
    }
    func removePending() throws { try remove(.pending) }
    func removeConnection() throws { try remove(.connection) }
    func removeHostRemoval() throws { try remove(.hostRemoval) }
    func close() {
        if lock >= 0 {
            while flock(lock,LOCK_UN) != 0 && errno == EINTR {}
            Darwin.close(lock); lock = -1
        }
        if directory >= 0 { Darwin.close(directory); directory = -1 }
    }
    deinit { close() }
}
