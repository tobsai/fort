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
            transport:core,ownerControl:core,ownerSetup:core)
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
            arguments:["--jitless","--no-experimental-strip-types","--no-warnings","--openssl-config=" + program.opensslConfiguration.path,program.bootstrap.path],directory:$0,environment:[:],guardian:program.guardian) },
            makeListener:makeListener)
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
