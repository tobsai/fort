#if os(macOS)
import Foundation
import Darwin

public struct MacCoreStop: Sendable {
    public let exitedCleanly: Bool
    public let forced: Bool
}

public enum MacCoreLifecycle: Sendable {
    case processGroup
    case parentExit(guardian: URL)
}

public struct MacCorePairingRecovery: Decodable, Sendable {
    public let installationID: String
    public let deviceID: String
    public let runtimeStopped: Bool
    public let pairingRevoked: Bool
    enum CodingKeys: String, CodingKey {
        case installationID = "installationId", deviceID = "deviceId", runtimeStopped, pairingRevoked = "revoked"
    }
}

public final class MacCoreProcessTransport: FortTransport, OwnerSetupControl, OwnerSetupCancellationControl, Sendable {
    private let state: CoreProcessState
    public let processID: Int32
    private init(_ state: CoreProcessState) { self.state = state; processID = state.pid }
    public static func launch(executable: URL, arguments: [String], directory: URL, environment: [String:String], lifecycle: MacCoreLifecycle) async throws -> MacCoreProcessTransport {
        try Task.checkCancellation()
        let program: URL
        let command: [String]
        switch lifecycle {
        case .processGroup: program = executable; command = arguments
        case .parentExit(let guardian):
            guard executable.isFileURL, executable.path.hasPrefix("/"), !executable.path.utf8.contains(0) else { throw ClientError.invalidResponse }
            program = guardian; command = [String(getpid()),executable.path] + arguments
        }
        let state = try CoreProcessState(executable:program,arguments:command,directory:directory,environment:environment)
        do {
            try await withTaskCancellationHandler { try await state.ready() } onCancel: { state.stop() }
            try Task.checkCancellation()
            return MacCoreProcessTransport(state)
        } catch { _ = try await state.close(); throw error }
    }
    public func send(_ request: FortRequest) async throws -> FortResponse {
        _ = try request.route()
        return try await exchange(.request(request))
    }
    public func recoverOwnerPairing(installationID: String, deviceID: String, timeout: TimeInterval = 30) async throws -> MacCorePairingRecovery {
        let request = try MacOwnerRecoveryRequest(installationID:installationID,deviceID:deviceID,timeout:timeout)
        let response = try await exchange(.recovery(request))
        if response.status == 409, response.body.count <= 1024,
           let rejection = try? JSONDecoder().decode([String:String].self,from:response.body),
           rejection == ["installationId":installationID,"deviceId":deviceID,"outcome":"identityRejected"] {
            throw MacOwnerRecoveryError.identityRejected
        }
        guard response.status == 200,
              let proof = try? JSONDecoder().decode(MacCorePairingRecovery.self,from:response.body),
              proof.installationID == installationID, proof.deviceID == deviceID,
              proof.runtimeStopped, proof.pairingRevoked else { throw ClientError.invalidResponse }
        return proof
    }
    public func beginOwnerSetup(timeout: TimeInterval = 30) async throws -> OwnerSetupState {
        guard timeout.isFinite, timeout > 0, timeout <= 30 else { throw ClientError.invalidResponse }
        let response = try await exchange(.setup(.begin(timeout)))
        guard response.status == 200, let offer = try? JSONDecoder().decode(OwnerSetupState.self,from:response.body) else { throw ClientError.invalidResponse }
        return offer
    }
    public func completeOwnerSetup(_ request: OwnerSetupRequest, timeout: TimeInterval = 30) async throws -> OwnerSetupReceipt {
        guard timeout.isFinite, timeout > 0, timeout <= 30 else { throw ClientError.invalidResponse }
        let response = try await exchange(.setup(.complete(request,timeout)))
        guard response.status == 200, let receipt = try? JSONDecoder().decode(OwnerSetupReceipt.self,from:response.body),
              receipt.installationID == request.installationID, receipt.deviceID == request.deviceID else { throw ClientError.invalidResponse }
        return receipt
    }
    public func cancelOwnerSetup(_ identity: OwnerSetupIdentity, timeout: TimeInterval = 30) async throws -> OwnerSetupCancellation {
        guard timeout.isFinite, timeout > 0, timeout <= 30 else { throw ClientError.invalidResponse }
        let response = try await exchange(.setup(.cancel(identity,timeout)))
        guard response.status == 200, let receipt = try? JSONDecoder().decode(OwnerSetupCancellation.self,from:response.body),
              receipt.identity == identity else { throw ClientError.invalidResponse }
        return receipt
    }
    private func exchange(_ command: CoreCommand) async throws -> FortResponse {
        try Task.checkCancellation()
        let call = CoreCall()
        return try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { state.send(command,call:call,continuation:$0) }
        } onCancel: { call.cancel(); state.cancel(call) }
    }
    public func close() async throws -> MacCoreStop { try await state.close() }
    deinit { state.stop() }
}

private final class CoreCall: @unchecked Sendable {
    private let lock = NSLock()
    private var canceled = false
    func cancel() { lock.withLock { canceled = true } }
    var isCanceled: Bool { lock.withLock { canceled } }
}

private struct CoreFrame: Decodable {
    let protocolVersion: Int
    let kind: String
    let id: Int?
    let response: FortResponse?
    let error: String?
}
private struct CoreRequestFrame: Encodable {
    let protocolVersion = 1
    let kind: String
    let id: Int
    let request: FortRequest?
    let recovery: MacOwnerRecoveryRequest?
    let setup: OwnerSetupCommand?
    init(id: Int, command: CoreCommand) {
        self.id = id
        switch command {
        case .request(let value): kind = "request"; request = value; recovery = nil; setup = nil
        case .recovery(let value): kind = "ownerRecovery"; recovery = value; request = nil; setup = nil
        case .setup(let value): kind = "ownerSetup"; setup = value; recovery = nil; request = nil
        }
    }
}
private enum CoreCommand: Sendable {
    case request(FortRequest)
    case recovery(MacOwnerRecoveryRequest)
    case setup(OwnerSetupCommand)
    var timeout: TimeInterval {
        switch self { case .request(let value): value.timeout; case .recovery(let value): value.timeout; case .setup(let value): value.timeout }
    }
}
private struct CoreCancelFrame: Encodable {
    let protocolVersion = 1
    let kind = "cancel"
    let id: Int
}

private final class CoreProcessState: @unchecked Sendable {
    let pid: pid_t
    private let queue = DispatchQueue(label:"Fort.CoreProcess")
    private let input: Int32
    private let output: Int32
    private var reader: DispatchSourceRead?
    private var writer: DispatchSourceWrite?
    private var monitor: DispatchSourceProcess?
    private var pipesInstalled = false
    private var writing = false
    private var received = Data()
    private var searchedBytes = 0
    private var writes: [(data:Data, offset:Int, queuedAt:DispatchTime)] = []
    private var writeDeadline: DispatchWorkItem?
    private var startupDeadline: DispatchWorkItem?
    private var startup: CheckedContinuation<Void, any Error>?
    private var isReady = false
    private var closing = false
    private var failed = false
    private var forced = false
    private var status: Int32?
    private var nextID = 0
    private var stopped: Result<MacCoreStop, any Error>?
    private var closers: [CheckedContinuation<MacCoreStop, any Error>] = []
    private struct Pending {
        let call: CoreCall
        var continuation: CheckedContinuation<FortResponse, any Error>?
        var deadline: DispatchWorkItem
    }
    private var pending: [Int:Pending] = [:]

    init(executable: URL, arguments: [String], directory: URL, environment: [String:String]) throws {
        guard executable.isFileURL, directory.isFileURL,
              [executable.path,directory.path].allSatisfy({ $0.hasPrefix("/") && !$0.utf8.contains(0) }),
              arguments.allSatisfy({ !$0.utf8.contains(0) }),
              environment.allSatisfy({ !$0.key.isEmpty && !$0.key.contains("=") && !$0.key.utf8.contains(0) && !$0.value.utf8.contains(0) }) else { throw ClientError.invalidResponse }
        var permissions = stat()
        guard lstat(directory.path,&permissions) == 0, permissions.st_mode & S_IFMT == S_IFDIR,
              permissions.st_uid == geteuid(), permissions.st_mode & 0o077 == 0 else { throw CocoaError(.fileWriteNoPermission) }
        var incoming = [Int32](repeating:-1,count:2), outgoing = [Int32](repeating:-1,count:2)
        guard pipe(&incoming) == 0 else { throw Self.posixError() }
        guard pipe(&outgoing) == 0 else { incoming.forEach { Darwin.close($0) }; throw Self.posixError() }
        var keep = false
        defer {
            Darwin.close(incoming[0]); Darwin.close(outgoing[1])
            if !keep { Darwin.close(incoming[1]); Darwin.close(outgoing[0]) }
        }
        for descriptor in incoming + outgoing {
            guard fcntl(descriptor,F_SETFD,FD_CLOEXEC) == 0 else { throw Self.posixError() }
        }
        guard fcntl(incoming[1],F_SETFL,O_NONBLOCK) == 0, fcntl(outgoing[0],F_SETFL,O_NONBLOCK) == 0,
              fcntl(incoming[1],F_SETNOSIGPIPE,1) == 0 else { throw Self.posixError() }
        var attributes: posix_spawnattr_t?
        var actions: posix_spawn_file_actions_t?
        try Self.check(posix_spawnattr_init(&attributes))
        defer { posix_spawnattr_destroy(&attributes) }
        try Self.check(posix_spawn_file_actions_init(&actions))
        defer { posix_spawn_file_actions_destroy(&actions) }
        try Self.check(posix_spawnattr_setflags(&attributes,Int16(POSIX_SPAWN_SETPGROUP | POSIX_SPAWN_CLOEXEC_DEFAULT)))
        try Self.check(posix_spawnattr_setpgroup(&attributes,0))
        if #available(macOS 26.0, *) { try Self.check(posix_spawn_file_actions_addchdir(&actions,directory.path)) }
        else { try Self.check(posix_spawn_file_actions_addchdir_np(&actions,directory.path)) }
        try Self.check(posix_spawn_file_actions_adddup2(&actions,incoming[0],STDIN_FILENO))
        try Self.check(posix_spawn_file_actions_adddup2(&actions,outgoing[1],STDOUT_FILENO))
        try Self.check(posix_spawn_file_actions_addopen(&actions,STDERR_FILENO,"/dev/null",O_WRONLY,0))
        var argv = ([executable.path] + arguments).map { strdup($0) } + [nil]
        var env = environment.sorted(by:{ $0.key < $1.key }).map { strdup($0.key + "=" + $0.value) } + [nil]
        defer { argv.forEach { free($0) }; env.forEach { free($0) } }
        var child: pid_t = 0
        try Self.check(posix_spawn(&child,executable.path,&actions,&attributes,&argv,&env))
        pid = child; input = incoming[1]; output = outgoing[0]; keep = true
    }
    private static func posixError() -> NSError { NSError(domain:NSPOSIXErrorDomain,code:Int(errno)) }
    private static func check(_ code: Int32) throws { if code != 0 { throw NSError(domain:NSPOSIXErrorDomain,code:Int(code)) } }

    func ready() async throws {
        try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, any Error>) in
            queue.async {
                if self.closing { continuation.resume(throwing:CancellationError()); return }
                self.startup = continuation
                self.pipesInstalled = true
                let reader = DispatchSource.makeReadSource(fileDescriptor:self.output,queue:self.queue)
                reader.setEventHandler { self.read() }
                let output = self.output; reader.setCancelHandler { _ = Darwin.close(output) }
                self.reader = reader; reader.resume()
                let writer = DispatchSource.makeWriteSource(fileDescriptor:self.input,queue:self.queue)
                writer.setEventHandler { self.flush() }
                let input = self.input; writer.setCancelHandler { _ = Darwin.close(input) }
                self.writer = writer
                let monitor = DispatchSource.makeProcessSource(identifier:self.pid,eventMask:.exit,queue:self.queue)
                monitor.setEventHandler { self.reap() }; self.monitor = monitor; monitor.resume()
                let deadline = DispatchWorkItem { self.fail() }
                self.startupDeadline = deadline; self.queue.asyncAfter(deadline:.now() + 5,execute:deadline)
                self.reap()
            }
        }
    }
    func send(_ command: CoreCommand, call: CoreCall, continuation: CheckedContinuation<FortResponse, any Error>) {
        queue.async {
            if call.isCanceled { continuation.resume(throwing:CancellationError()); return }
            guard self.isReady && !self.closing, self.pending.count < 8, self.nextID < 9_007_199_254_740_991 else { continuation.resume(throwing:URLError(.resourceUnavailable)); return }
            let id = self.nextID + 1
            guard let encoded = try? JSONEncoder().encode(CoreRequestFrame(id:id,command:command)), encoded.count <= 2 * 1024 * 1024 else { continuation.resume(throwing:ClientError.invalidResponse); return }
            self.nextID = id
            let deadline = DispatchWorkItem { self.cancel(id,error:URLError(.timedOut)) }
            self.pending[id] = Pending(call:call,continuation:continuation,deadline:deadline)
            self.queue.asyncAfter(deadline:.now() + command.timeout,execute:deadline)
            self.enqueue(encoded)
        }
    }
    func cancel(_ call: CoreCall) { queue.async { if let id = self.pending.first(where:{ $0.value.call === call })?.key { self.cancel(id,error:CancellationError()) } } }
    private func cancel(_ id: Int, error: any Error) {
        guard var call = pending[id], let continuation = call.continuation else { return }
        call.continuation = nil; call.deadline.cancel()
        let deadline = DispatchWorkItem { self.fail() }; call.deadline = deadline; pending[id] = call
        continuation.resume(throwing:error)
        queue.asyncAfter(deadline:.now() + 5,execute:deadline)
        guard let bytes = try? JSONEncoder().encode(CoreCancelFrame(id:id)) else { fail(); return }
        enqueue(bytes)
    }
    private func enqueue(_ bytes: Data) {
        guard !closing else { return }
        writes.append((bytes + Data([10]),0,.now()))
        if !writing { writing = true; writer?.resume() }
        flush()
    }
    private func flush() {
        guard !closing else { return }
        while !writes.isEmpty {
            let next = writes[0]
            let count = next.data.withUnsafeBytes { Darwin.write(input,$0.baseAddress!.advanced(by:next.offset),next.data.count - next.offset) }
            if count < 0 {
                if errno == EINTR { continue }
                if errno == EAGAIN { break }
                fail(); return
            }
            guard count > 0 else { fail(); return }
            writes[0].offset += count
            if writes[0].offset == writes[0].data.count { writes.removeFirst() }
        }
        writeDeadline?.cancel(); writeDeadline = nil
        if let next = writes.first {
            let deadline = DispatchWorkItem { self.fail() }; writeDeadline = deadline
            queue.asyncAfter(deadline:next.queuedAt + 5,execute:deadline)
        } else if writing { writer?.suspend(); writing = false }
    }
    private func read() {
        var bytes = [UInt8](repeating:0,count:65_536)
        while !closing {
            let count = Darwin.read(output,&bytes,bytes.count)
            if count < 0 {
                if errno == EINTR { continue }
                if errno == EAGAIN { return }
                fail(); return
            }
            guard count > 0 else { fail(); return }
            received.append(contentsOf:bytes.prefix(count))
            while let newline = received.range(of:Data([10]),in:received.index(received.startIndex,offsetBy:searchedBytes)..<received.endIndex)?.lowerBound {
                let length = received.distance(from:received.startIndex,to:newline)
                guard length <= 12 * 1024 * 1024,
                      let frame = try? JSONDecoder().decode(CoreFrame.self,from:received.prefix(length)) else { fail(); return }
                received.removeFirst(length + 1)
                searchedBytes = 0
                accept(frame)
                if closing { return }
            }
            searchedBytes = received.count
            if received.count > 12 * 1024 * 1024 { fail(); return }
        }
    }
    private func accept(_ frame: CoreFrame) {
        guard frame.protocolVersion == 1 else { fail(); return }
        if frame.kind == "ready" {
            guard !isReady, frame.id == nil, frame.response == nil, frame.error == nil else { fail(); return }
            isReady = true; startupDeadline?.cancel(); startupDeadline = nil
            startup?.resume(); startup = nil; return
        }
        guard isReady, let id = frame.id, let call = pending[id] else { fail(); return }
        let result: Result<FortResponse, any Error>
        if frame.kind == "response", frame.error == nil, let response = frame.response,
           (100..<600).contains(response.status), response.body.count <= 8 * 1024 * 1024 { result = .success(response) }
        else if frame.kind == "error", frame.response == nil {
            switch frame.error {
            case "timeout": result = .failure(URLError(.timedOut))
            case "canceled": result = .failure(CancellationError())
            case "unavailable": result = .failure(URLError(.cannotConnectToHost))
            default: fail(); return
            }
        } else { fail(); return }
        pending.removeValue(forKey:id); call.deadline.cancel(); call.continuation?.resume(with:result)
    }
    func stop() { queue.async { self.beginClose() } }
    func close() async throws -> MacCoreStop {
        try await withCheckedThrowingContinuation { continuation in
            queue.async {
                if let stopped = self.stopped { continuation.resume(with:stopped); return }
                self.closers.append(continuation); self.beginClose()
            }
        }
    }
    private func fail() { failed = true; beginClose() }
    private func beginClose() {
        guard !closing else { return }
        closing = true
        startupDeadline?.cancel(); startupDeadline = nil
        startup?.resume(throwing:URLError(.cannotConnectToHost)); startup = nil
        writeDeadline?.cancel(); writeDeadline = nil
        for call in pending.values { call.deadline.cancel(); call.continuation?.resume(throwing:URLError(.networkConnectionLost)) }
        pending.removeAll(); writes.removeAll(); received.removeAll()
        if pipesInstalled {
            if let writer { if !writing { writer.resume() }; writer.cancel(); self.writer = nil }
            reader?.cancel(); reader = nil
        } else { Darwin.close(input); Darwin.close(output) }
        if monitor == nil && status == nil {
            let monitor = DispatchSource.makeProcessSource(identifier:pid,eventMask:.exit,queue:queue)
            monitor.setEventHandler { self.reap() }; self.monitor = monitor; monitor.resume()
        }
        checkStopped(started:ProcessInfo.processInfo.systemUptime)
    }
    private func reap() {
        guard status == nil else { return }
        var value: Int32 = 0
        var result: pid_t
        repeat { result = waitpid(pid,&value,WNOHANG) } while result == -1 && errno == EINTR
        if result == pid { status = value; monitor?.cancel(); monitor = nil; if !closing { fail() } }
    }
    private func checkStopped(started: TimeInterval) {
        reap()
        let absent = kill(-pid,0) == -1 && errno == ESRCH
        if let status, absent { finish(.success(MacCoreStop(exitedCleanly:status == 0 && !failed && !forced,forced:forced))); return }
        let elapsed = ProcessInfo.processInfo.systemUptime - started
        if elapsed >= 6 { finish(.failure(URLError(.cannotCloseFile))); return }
        if elapsed >= 4 { forced = true; _ = kill(-pid,SIGKILL) }
        else if elapsed >= 2 { forced = true; _ = kill(-pid,SIGTERM) }
        queue.asyncAfter(deadline:.now() + 0.05) { self.checkStopped(started:started) }
    }
    private func finish(_ result: Result<MacCoreStop, any Error>) {
        guard stopped == nil else { return }
        stopped = result; monitor?.cancel(); monitor = nil
        for continuation in closers { continuation.resume(with:result) }; closers.removeAll()
    }
}
#endif
