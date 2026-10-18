#if os(macOS)
import Foundation
import Darwin

public enum MacAgentStorageError: Error, LocalizedError {
    case groupUnavailable, unsafeDirectory, changedDirectory
    public var errorDescription: String? {
        switch self {
        case .groupUnavailable: "The exact Fort App Group is unavailable. No alternate storage was selected."
        case .unsafeDirectory: "Fort's host directory could not be verified. Existing files and permissions have been kept."
        case .changedDirectory: "Fort's host directory changed. Reopen the exact installation before starting its host."
        }
    }
}

public struct MacAgentStorage: Sendable {
    private struct Identity: Equatable, Sendable {
        let device: dev_t
        let inode: ino_t
        let permissions: mode_t
    }
    private let container: URL
    private let containerIdentity: Identity
    private let directoryIdentity: Identity
    private static let child = "FortHost"

    public static func prepare(for endpoint: MacAgentEndpoint) throws -> Self {
        try prepare(for:endpoint,resolveContainer:FileManager.default.containerURL(forSecurityApplicationGroupIdentifier:))
    }

    static func prepare(for endpoint: MacAgentEndpoint, resolveContainer: (String) throws -> URL?) throws -> Self {
        guard let url = try resolveContainer(endpoint.appGroup) else { throw MacAgentStorageError.groupUnavailable }
        guard url.isFileURL, url.host == nil || url.host == "", url.path.hasPrefix("/"), !url.path.contains("\0") else { throw MacAgentStorageError.unsafeDirectory }
        let parent = Darwin.open(url.path,O_RDONLY | O_DIRECTORY | O_NOFOLLOW | O_CLOEXEC)
        guard parent >= 0 else { throw ioError() }
        defer { Darwin.close(parent) }
        let parentIdentity = try identity(parent,privateDirectory:false)
        let canonical = URL(fileURLWithPath:try path(parent),isDirectory:true)
        try verifyLink(parent:AT_FDCWD,name:canonical.path,expected:parentIdentity)
        if mkdirat(parent,child,0o700) != 0 && errno != EEXIST { throw ioError() }
        let directory = openat(parent,child,O_RDONLY | O_DIRECTORY | O_NOFOLLOW | O_CLOEXEC)
        guard directory >= 0 else { throw ioError() }
        defer { Darwin.close(directory) }
        let childIdentity = try identity(directory,privateDirectory:true)
        try verifyLink(parent:parent,name:child,expected:childIdentity)
        try verifyLink(parent:AT_FDCWD,name:canonical.path,expected:parentIdentity)
        guard try path(directory) == canonical.appendingPathComponent(child).path else { throw MacAgentStorageError.changedDirectory }
        guard fsync(directory) == 0, fsync(parent) == 0 else { throw ioError() }
        let result = Self(container:canonical,containerIdentity:parentIdentity,directoryIdentity:childIdentity)
        _ = try result.validatedDirectory()
        return result
    }

    public func validatedDirectory() throws -> URL {
        let parent = Darwin.open(container.path,O_RDONLY | O_DIRECTORY | O_NOFOLLOW | O_CLOEXEC)
        guard parent >= 0 else { throw Self.ioError() }
        defer { Darwin.close(parent) }
        guard try Self.identity(parent,privateDirectory:false) == containerIdentity,
              try Self.path(parent) == container.path else { throw MacAgentStorageError.changedDirectory }
        let directory = openat(parent,Self.child,O_RDONLY | O_DIRECTORY | O_NOFOLLOW | O_CLOEXEC)
        guard directory >= 0 else { throw Self.ioError() }
        defer { Darwin.close(directory) }
        let url = container.appendingPathComponent(Self.child,isDirectory:true)
        guard try Self.identity(directory,privateDirectory:true) == directoryIdentity,
              try Self.path(directory) == url.path else { throw MacAgentStorageError.changedDirectory }
        try Self.verifyLink(parent:parent,name:Self.child,expected:directoryIdentity)
        try Self.verifyLink(parent:AT_FDCWD,name:container.path,expected:containerIdentity)
        return url
    }

    private static func ioError() -> NSError { NSError(domain:NSPOSIXErrorDomain,code:Int(errno)) }
    private static func snapshot(_ info: stat) -> Identity {
        Identity(device:info.st_dev,inode:info.st_ino,permissions:info.st_mode & 0o7777)
    }
    private static func identity(_ descriptor: Int32, privateDirectory: Bool) throws -> Identity {
        var info = stat()
        guard fstat(descriptor,&info) == 0 else { throw ioError() }
        let permissions = info.st_mode & 0o7777
        guard info.st_mode & S_IFMT == S_IFDIR, info.st_uid == geteuid(), permissions & 0o700 == 0o700,
              privateDirectory ? permissions == 0o700 : permissions & 0o7022 == 0 else { throw MacAgentStorageError.unsafeDirectory }
        guard let acl = acl_get_fd_np(descriptor,ACL_TYPE_EXTENDED) else {
            if errno == ENOENT { return snapshot(info) }
            throw ioError()
        }
        defer { acl_free(UnsafeMutableRawPointer(acl)) }
        var entry: acl_entry_t?
        guard acl_get_entry(acl,Int32(ACL_FIRST_ENTRY.rawValue),&entry) == -1, errno == EINVAL else { throw MacAgentStorageError.unsafeDirectory }
        return snapshot(info)
    }
    private static func verifyLink(parent: Int32, name: String, expected: Identity) throws {
        var info = stat()
        guard fstatat(parent,name,&info,AT_SYMLINK_NOFOLLOW) == 0 else { throw ioError() }
        guard info.st_mode & S_IFMT == S_IFDIR, info.st_uid == geteuid(), snapshot(info) == expected else { throw MacAgentStorageError.changedDirectory }
    }
    private static func path(_ descriptor: Int32) throws -> String {
        var buffer = [CChar](repeating:0,count:Int(MAXPATHLEN))
        guard fcntl(descriptor,F_GETPATH,&buffer) == 0 else { throw ioError() }
        let bytes = buffer.prefix { $0 != 0 }.map { UInt8(bitPattern:$0) }
        guard let path = String(bytes:bytes,encoding:.utf8), path.hasPrefix("/"), !bytes.isEmpty else { throw MacAgentStorageError.unsafeDirectory }
        return path
    }
}
#endif
