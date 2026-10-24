#if os(macOS)
import Foundation

public enum MacAgentServiceError: Error, LocalizedError {
    case alreadyStarted, cleanupUnconfirmed
    public var errorDescription: String? {
        switch self {
        case .alreadyStarted: "This host lifecycle has already started or stopped. No replacement core was launched."
        case .cleanupUnconfirmed: "The host's core shutdown could not be confirmed. Do not start another host until its owned processes are reconciled."
        }
    }
}

@MainActor protocol MacAgentServiceListener: AnyObject {
    func resume() throws
    func invalidate()
}
@MainActor private final class SystemAgentListener: MacAgentServiceListener {
    private let listener: NSXPCListener
    private let delegate: MacHostListener
    init(bundle: MacHostBundle, core: MacCoreHost) throws {
        delegate = try MacHostListener(clientIdentifier:bundle.clientIdentifier,signingTeam:bundle.endpoint.signingTeam,
            transport:core,ownerControl:core,ownerSetup:core,networkControl:core)
        listener = NSXPCListener(machServiceName:bundle.endpoint.machServiceName)
        listener.delegate = delegate
    }
    func resume() { listener.resume() }
    func invalidate() { listener.invalidate() }
}

@MainActor public final class MacAgentService {
    private let prepareStorage: @MainActor () throws -> MacAgentStorage
    private let launch: @MainActor (URL) async throws -> MacCoreHost
    private let makeListener: @MainActor (MacCoreHost) throws -> any MacAgentServiceListener
    private var started = false
    private var closing = false
    private var launchTask: Task<MacCoreHost,any Error>?
    private var shutdownTask: Task<MacCoreStop,any Error>?
    private var listener: (any MacAgentServiceListener)?

    public convenience init(program: MacHostProgram) {
        self.init(program:program,resolveContainer:{ FileManager.default.containerURL(forSecurityApplicationGroupIdentifier:$0) },
            makeListener:{ try SystemAgentListener(bundle:program.bundle,core:$0) })
    }
    convenience init(program: MacHostProgram, resolveContainer: @escaping @MainActor (String) throws -> URL?,
                     makeListener: @escaping @MainActor (MacCoreHost) throws -> any MacAgentServiceListener) {
        self.init(prepareStorage:{
            let storage = try MacAgentStorage.prepare(for:program.bundle.endpoint,resolveContainer:resolveContainer)
            _ = try storage.prepareRuntimeDirectory()
            return storage
        },launch:{ try await MacCoreHost.launch(executable:program.executable,
            arguments:["--jitless","--no-experimental-strip-types","--no-warnings","--openssl-config=" + program.opensslConfiguration.path,program.bootstrap.path],directory:$0,environment:Self.hostEnvironment(),guardian:program.guardian) },
            makeListener:makeListener)
    }
    // Only the sandbox container's temporary directory, and the shorter of its two names. Claude Code's local socket must
    // fit in 103 bytes, so its private directory under TMPDIR stays within 72 (Spec 062). The system temporary directory
    // can be too long; the container's own Data/tmp is usually shorter.
    static func hostEnvironment(temporary: URL = FileManager.default.temporaryDirectory, home: String = NSHomeDirectory()) -> [String:String] {
        let candidates = [home + "/tmp",temporary.path].compactMap(canonicalDirectory)
        return ["TMPDIR":candidates.min(by:{ $0.utf8.count < $1.utf8.count }) ?? temporary.standardizedFileURL.path]
    }
    private static func canonicalDirectory(_ path: String) -> String? {
        guard let resolved = realpath(path,nil) else { return nil }
        defer { free(resolved) }
        let value = String(cString:resolved)
        var info = stat()
        guard lstat(value,&info) == 0, info.st_mode & S_IFMT == S_IFDIR, info.st_uid == geteuid() else { return nil }
        return value
    }
    init(prepareStorage: @escaping @MainActor () throws -> MacAgentStorage,
         launch: @escaping @MainActor (URL) async throws -> MacCoreHost,
         makeListener: @escaping @MainActor (MacCoreHost) throws -> any MacAgentServiceListener) {
        self.prepareStorage = prepareStorage; self.launch = launch; self.makeListener = makeListener
    }

    public func start() async throws {
        guard !started, !closing else { throw MacAgentServiceError.alreadyStarted }
        try Task.checkCancellation()
        started = true
        do {
            let storage = try prepareStorage(), directory = try storage.validatedDirectory()
            let task = Task { try await self.launch(directory) }
            launchTask = task
            let core = try await withTaskCancellationHandler { try await task.value } onCancel: { task.cancel() }
            try Task.checkCancellation()
            guard !closing else { throw CancellationError() }
            guard try storage.validatedDirectory() == directory else { throw MacAgentStorageError.changedDirectory }
            let channel = try makeListener(core)
            listener = channel
            try channel.resume()
            try Task.checkCancellation()
        } catch {
            _ = try await shutdown()
            throw error
        }
    }

    /// Returns once the started core has ended, by itself or through shutdown, or at once if none was launched. The core
    /// ends itself when the owner turns Claude on or off (Spec 064), so the helper can exit and macOS can start a new one.
    public func waitUntilCoreEnds() async {
        guard let launchTask, let core = try? await launchTask.value else { return }
        await core.waitUntilEnded()
    }

    public func shutdown() async throws -> MacCoreStop {
        closing = true
        if let shutdownTask { return try await shutdownTask.value }
        listener?.invalidate(); listener = nil
        let pending = launchTask
        pending?.cancel()
        let task = Task { () throws -> MacCoreStop in
            guard let pending else { return MacCoreStop(exitedCleanly:true,forced:false) }
            let core: MacCoreHost
            do { core = try await pending.value }
            catch { throw MacAgentServiceError.cleanupUnconfirmed }
            let stopped = try await core.shutdown()
            guard stopped.exitedCleanly, !stopped.forced else { throw MacAgentServiceError.cleanupUnconfirmed }
            return stopped
        }
        shutdownTask = task
        return try await task.value
    }
}
#endif
