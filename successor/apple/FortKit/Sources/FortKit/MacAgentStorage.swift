#if os(macOS)
import Foundation
import Darwin

public enum MacAgentStorageError: Error, LocalizedError {
    case groupUnavailable, unsafeDirectory, changedDirectory, unsafeCredential
    public var errorDescription: String? {
        switch self {
        case .groupUnavailable: "The exact Fort App Group is unavailable. No alternate storage was selected."
        case .unsafeDirectory: "Fort's host directory could not be verified. Existing files and permissions have been kept."
        case .changedDirectory: "Fort's host directory changed. Reopen the exact installation before starting its host."
        case .unsafeCredential: "Fort’s local provider credential could not be safely removed. Its state is unconfirmed; keep the app for recovery."
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

    static func requireUnprepared(for endpoint: MacAgentEndpoint) throws {
        try requireUnprepared(for:endpoint,resolveContainer:FileManager.default.containerURL(forSecurityApplicationGroupIdentifier:))
    }
    static func requireUnprepared(for endpoint: MacAgentEndpoint, resolveContainer: (String) throws -> URL?) throws {
        guard let url = try resolveContainer(endpoint.appGroup), url.isFileURL,
              url.host == nil || url.host == "", url.path.hasPrefix("/"), !url.path.contains("\0") else { throw MacAgentStorageError.groupUnavailable }
        let parent = Darwin.open(url.path,O_RDONLY | O_DIRECTORY | O_NOFOLLOW | O_CLOEXEC)
        guard parent >= 0 else { throw ioError() }
        defer { Darwin.close(parent) }
        let expected = try identity(parent,privateDirectory:false)
        let canonical = try path(parent)
        try verifyLink(parent:AT_FDCWD,name:canonical,expected:expected)
        var info = stat()
        guard fstatat(parent,child,&info,AT_SYMLINK_NOFOLLOW) == -1, errno == ENOENT else { throw MacHostRemovalError.setupBlocked }
        try verifyLink(parent:AT_FDCWD,name:canonical,expected:expected)
    }

    public static func prepare(for endpoint: MacAgentEndpoint) throws -> Self {
        try prepare(for:endpoint,resolveContainer:FileManager.default.containerURL(forSecurityApplicationGroupIdentifier:))
    }

    public static func inspectExisting(for endpoint: MacAgentEndpoint) throws -> Self {
        try inspectExisting(for:endpoint,resolveContainer:FileManager.default.containerURL(forSecurityApplicationGroupIdentifier:))
    }

    static func inspectExisting(for endpoint: MacAgentEndpoint, resolveContainer: (String) throws -> URL?) throws -> Self {
        try resolve(for:endpoint,resolveContainer:resolveContainer,create:false)
    }

    static func prepare(for endpoint: MacAgentEndpoint, resolveContainer: (String) throws -> URL?) throws -> Self {
        try resolve(for:endpoint,resolveContainer:resolveContainer,create:true)
    }

    private static func resolve(for endpoint: MacAgentEndpoint, resolveContainer: (String) throws -> URL?, create: Bool) throws -> Self {
        guard let url = try resolveContainer(endpoint.appGroup) else { throw MacAgentStorageError.groupUnavailable }
        guard url.isFileURL, url.host == nil || url.host == "", url.path.hasPrefix("/"), !url.path.contains("\0") else { throw MacAgentStorageError.unsafeDirectory }
        let parent = Darwin.open(url.path,O_RDONLY | O_DIRECTORY | O_NOFOLLOW | O_CLOEXEC)
        guard parent >= 0 else { throw ioError() }
        defer { Darwin.close(parent) }
        let parentIdentity = try identity(parent,privateDirectory:false)
        let canonical = URL(fileURLWithPath:try path(parent),isDirectory:true)
        try verifyLink(parent:AT_FDCWD,name:canonical.path,expected:parentIdentity)
        if create && mkdirat(parent,child,0o700) != 0 && errno != EEXIST { throw ioError() }
        let directory = openat(parent,child,O_RDONLY | O_DIRECTORY | O_NOFOLLOW | O_CLOEXEC)
        guard directory >= 0 else { throw ioError() }
        defer { Darwin.close(directory) }
        let childIdentity = try identity(directory,privateDirectory:true)
        try verifyLink(parent:parent,name:child,expected:childIdentity)
        try verifyLink(parent:AT_FDCWD,name:canonical.path,expected:parentIdentity)
        guard try path(directory) == canonical.appendingPathComponent(child).path else { throw MacAgentStorageError.changedDirectory }
        if create { guard fsync(directory) == 0, fsync(parent) == 0 else { throw ioError() } }
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

    func prepareRuntimeDirectory() throws -> URL {
        let url = try validatedDirectory()
        let root = Darwin.open(url.path,O_RDONLY | O_DIRECTORY | O_NOFOLLOW | O_CLOEXEC)
        guard root >= 0 else { throw Self.ioError() }
        defer { Darwin.close(root) }
        guard try Self.identity(root,privateDirectory:true) == directoryIdentity else { throw MacAgentStorageError.changedDirectory }
        if mkdirat(root,"Runtime",0o700) != 0 && errno != EEXIST { throw Self.ioError() }
        let runtime = openat(root,"Runtime",O_RDONLY | O_DIRECTORY | O_NOFOLLOW | O_CLOEXEC)
        guard runtime >= 0 else { throw Self.ioError() }
        defer { Darwin.close(runtime) }
        let runtimeIdentity = try Self.identity(runtime,privateDirectory:true)
        try Self.verifyLink(parent:root,name:"Runtime",expected:runtimeIdentity)
        for name in ["home","state","workspace","tmp"] {
            if mkdirat(runtime,name,0o700) != 0 && errno != EEXIST { throw Self.ioError() }
            let child = openat(runtime,name,O_RDONLY | O_DIRECTORY | O_NOFOLLOW | O_CLOEXEC)
            guard child >= 0 else { throw Self.ioError() }
            defer { Darwin.close(child) }
            let identity = try Self.identity(child,privateDirectory:true)
            try Self.verifyLink(parent:runtime,name:name,expected:identity)
            guard fsync(child) == 0 else { throw Self.ioError() }
        }
        let result = url.appendingPathComponent("Runtime",isDirectory:true)
        guard try Self.path(runtime) == result.path else { throw MacAgentStorageError.changedDirectory }
        try Self.verifyLink(parent:root,name:"Runtime",expected:runtimeIdentity)
        guard fsync(runtime) == 0, fsync(root) == 0 else { throw Self.ioError() }
        guard try validatedDirectory() == url else { throw MacAgentStorageError.changedDirectory }
        return result
    }

    // Fort-held provider credentials in the runtime state directory: Codex auth and the ChatGPT plan sign-in.
    static let providerCredentialNames = ["auth.json","openai-plan.json"]

    // Caller must confirm owned process absence and startup unregistration first.
    func removeProviderCredentials() throws {
        let url = try validatedDirectory()
        let root = Darwin.open(url.path,O_RDONLY | O_DIRECTORY | O_NOFOLLOW | O_CLOEXEC)
        guard root >= 0 else { throw Self.ioError() }
        defer { Darwin.close(root) }
        guard try Self.identity(root,privateDirectory:true) == directoryIdentity else { throw MacAgentStorageError.changedDirectory }
        let runtime = openat(root,"Runtime",O_RDONLY | O_DIRECTORY | O_NOFOLLOW | O_CLOEXEC)
        guard runtime >= 0 else { throw Self.ioError() }
        defer { Darwin.close(runtime) }
        let runtimeIdentity = try Self.identity(runtime,privateDirectory:true)
        let state = openat(runtime,"state",O_RDONLY | O_DIRECTORY | O_NOFOLLOW | O_CLOEXEC)
        guard state >= 0 else { throw Self.ioError() }
        defer { Darwin.close(state) }
        let stateIdentity = try Self.identity(state,privateDirectory:true)
        func verifyDirectories() throws {
            guard try validatedDirectory() == url,
                  try Self.identity(root,privateDirectory:true) == directoryIdentity,
                  try Self.identity(runtime,privateDirectory:true) == runtimeIdentity,
                  try Self.identity(state,privateDirectory:true) == stateIdentity else { throw MacAgentStorageError.changedDirectory }
            try Self.verifyLink(parent:root,name:"Runtime",expected:runtimeIdentity)
            try Self.verifyLink(parent:runtime,name:"state",expected:stateIdentity)
        }
        try verifyDirectories()
        for name in Self.providerCredentialNames {
            let credential = openat(state,name,O_EVTONLY | O_NOFOLLOW | O_NONBLOCK | O_CLOEXEC)
            if credential < 0 && errno == ENOENT {
                guard fsync(state) == 0 else { throw MacAgentStorageError.unsafeCredential }
                try verifyDirectories(); continue
            }
            guard credential >= 0 else { throw MacAgentStorageError.unsafeCredential }
            defer { Darwin.close(credential) }
            var info = stat()
            guard fstat(credential,&info) == 0, info.st_mode & S_IFMT == S_IFREG,
                  info.st_uid == geteuid(), info.st_nlink == 1, info.st_mode & 0o7177 == 0 else { throw MacAgentStorageError.unsafeCredential }
            try Self.verifyNoACL(credential)
            try verifyDirectories()
            var linked = stat()
            guard fstatat(state,name,&linked,AT_SYMLINK_NOFOLLOW) == 0,
                  linked.st_dev == info.st_dev, linked.st_ino == info.st_ino, linked.st_mode == info.st_mode,
                  linked.st_uid == info.st_uid, linked.st_nlink == 1,
                  linked.st_ctimespec.tv_sec == info.st_ctimespec.tv_sec,
                  linked.st_ctimespec.tv_nsec == info.st_ctimespec.tv_nsec else { throw MacAgentStorageError.unsafeCredential }
            guard unlinkat(state,name,0) == 0, fsync(state) == 0 else { throw MacAgentStorageError.unsafeCredential }
            guard fstatat(state,name,&linked,AT_SYMLINK_NOFOLLOW) == -1, errno == ENOENT else { throw MacAgentStorageError.unsafeCredential }
            try verifyDirectories()
        }
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
        try verifyNoACL(descriptor)
        return snapshot(info)
    }
    private static func verifyNoACL(_ descriptor: Int32) throws {
        guard let acl = acl_get_fd_np(descriptor,ACL_TYPE_EXTENDED) else {
            if errno == ENOENT { return }
            throw ioError()
        }
        defer { acl_free(UnsafeMutableRawPointer(acl)) }
        var entry: acl_entry_t?
        guard acl_get_entry(acl,Int32(ACL_FIRST_ENTRY.rawValue),&entry) == -1, errno == EINVAL else { throw MacAgentStorageError.unsafeDirectory }
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
