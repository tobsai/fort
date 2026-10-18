#if os(macOS)
import Foundation
import Darwin
import Testing
@testable import FortKit

private final class ObservedChild {
    let processID: pid_t
    private var input: Int32
    private var reaped = false
    var isRunning: Bool { !reaped && kill(processID,0) == 0 }
    init() throws {
        var descriptors: [Int32] = [-1,-1]
        guard pipe(&descriptors) == 0 else { throw POSIXError(.EIO) }
        var accepted = false
        defer { Darwin.close(descriptors[0]); if !accepted { Darwin.close(descriptors[1]) } }
        var actions: posix_spawn_file_actions_t?
        var attributes: posix_spawnattr_t?
        guard posix_spawn_file_actions_init(&actions) == 0 else { throw POSIXError(.EIO) }
        defer { posix_spawn_file_actions_destroy(&actions) }
        guard posix_spawnattr_init(&attributes) == 0 else { throw POSIXError(.EIO) }
        defer { posix_spawnattr_destroy(&attributes) }
        guard posix_spawn_file_actions_adddup2(&actions,descriptors[0],STDIN_FILENO) == 0,
              posix_spawn_file_actions_addopen(&actions,STDOUT_FILENO,"/dev/null",O_WRONLY,0) == 0,
              posix_spawn_file_actions_addopen(&actions,STDERR_FILENO,"/dev/null",O_WRONLY,0) == 0,
              posix_spawnattr_setflags(&attributes,Int16(POSIX_SPAWN_CLOEXEC_DEFAULT)) == 0 else { throw POSIXError(.EIO) }
        let executable = strdup("/bin/cat")
        defer { free(executable) }
        var arguments = [executable,nil], environment: [UnsafeMutablePointer<CChar>?] = [nil], pid: pid_t = 0
        let error = posix_spawn(&pid,"/bin/cat",&actions,&attributes,&arguments,&environment)
        guard error == 0 else { throw NSError(domain:NSPOSIXErrorDomain,code:Int(error)) }
        processID = pid; input = descriptors[1]; accepted = true
    }
    func finish() throws {
        if input >= 0 { Darwin.close(input); input = -1 }
        let deadline = ContinuousClock.now.advanced(by:.seconds(1))
        while !reaped {
            var status: Int32 = 0
            let result = waitpid(processID,&status,WNOHANG)
            if result == processID { reaped = true; guard status == 0 else { throw POSIXError(.ECHILD) }; return }
            if result < 0 {
                if errno == EINTR { continue }
                if errno == ECHILD { reaped = true }
                throw NSError(domain:NSPOSIXErrorDomain,code:Int(errno))
            }
            guard ContinuousClock.now < deadline else { throw URLError(.timedOut) }
            usleep(1_000)
        }
    }
    deinit {
        do { try finish() }
        catch {
            if !reaped {
                _ = kill(processID,SIGKILL)
                var status: Int32 = 0
                while waitpid(processID,&status,0) < 0 && errno == EINTR { }
            }
        }
    }
}

@Test func hostProcessWaitRequiresExitAndNeverStopsTheObservedProcess() async throws {
    let child = try ObservedChild()
    let observation = try MacHostProcess.observe(processID:child.processID,effectiveUserID:geteuid())
    #expect(observation.processID == child.processID)
    #expect(try !observation.hasExited())
    await #expect(throws:URLError(.timedOut)) { try await observation.waitForExit(timeout:0.03) }
    #expect(child.isRunning)
    #expect(try !observation.hasExited())
    try child.finish()
    try await observation.waitForExit(timeout:1)
    try await observation.waitForExit(timeout:1)
    #expect(try observation.hasExited())
}

@Test func hostProcessWaitCancellationIsCallerLocal() async throws {
    let child = try ObservedChild()
    let observation = try MacHostProcess.observe(processID:child.processID,effectiveUserID:geteuid())
    let canceled = Task { try await observation.waitForExit(timeout:2) }
    let other = Task { try await observation.waitForExit(timeout:2) }
    canceled.cancel()
    await #expect(throws:CancellationError.self) { try await canceled.value }
    #expect(child.isRunning)
    #expect(try !observation.hasExited())
    try child.finish()
    try await other.value
    let preCanceled = Task { withUnsafeCurrentTask { $0?.cancel() }; try await observation.waitForExit(timeout:1) }
    await #expect(throws:CancellationError.self) { try await preCanceled.value }
}

@Test func hostProcessRefusesInvalidPeerAndDeadlineWithoutAcceptingAbsence() async throws {
    for pid: Int32 in [-1,0,1,getpid()] {
        #expect(throws:ClientError.self) { try MacHostProcess.observe(processID:pid,effectiveUserID:geteuid(),inspect:{ _ in throw POSIXError(.EIO) }) }
    }
    let child = try ObservedChild(), pid = child.processID
    #expect(throws:ClientError.self) { try MacHostProcess.observe(processID:pid,effectiveUserID:geteuid() + 1) }
    #expect(throws:ClientError.self) { try MacHostProcess.observe(processID:pid,effectiveUserID:geteuid(),inspect:{ _ in nil }) }
    #expect(throws:POSIXError(.EPERM)) { try MacHostProcess.observe(processID:pid,effectiveUserID:geteuid(),inspect:{ _ in throw POSIXError(.EPERM) }) }
    let observation = try MacHostProcess.observe(processID:pid,effectiveUserID:geteuid())
    for timeout in [Double.nan,Double.infinity,-1,0,30.01] {
        await #expect(throws:ClientError.self) { try await observation.waitForExit(timeout:timeout) }
    }
    #expect(child.isRunning)
    try child.finish()
    #expect(throws:ClientError.self) { try MacHostProcess.observe(processID:pid,effectiveUserID:geteuid()) }
}

private final class ProcessInspection: @unchecked Sendable {
    private let lock = NSLock()
    private var value: Result<MacProcessSnapshot?,any Error>
    init(_ snapshot: MacProcessSnapshot) { value = .success(snapshot) }
    func set(_ value: Result<MacProcessSnapshot?,any Error>) { lock.withLock { self.value = value } }
    func read(_ pid: Int32) throws -> MacProcessSnapshot? { try lock.withLock { try value.get() } }
}

@Test func hostProcessReplacementConfirmsOriginalExitWithoutSignalingTheNewProcess() async throws {
    let child = try ObservedChild(), pid = child.processID
    let initial = try #require(try MacProcessSnapshot.read(pid))
    let inspection = ProcessInspection(initial)
    let observation = try MacHostProcess.observe(processID:pid,effectiveUserID:geteuid(),inspect:inspection.read)
    inspection.set(.success(MacProcessSnapshot(effectiveUserID:geteuid() + 1,startedSeconds:initial.startedSeconds,startedMicroseconds:initial.startedMicroseconds)))
    #expect(try !observation.hasExited())
    inspection.set(.success(MacProcessSnapshot(effectiveUserID:geteuid(),startedSeconds:initial.startedSeconds + 1,startedMicroseconds:initial.startedMicroseconds)))
    try await observation.waitForExit(timeout:0.1)
    #expect(try observation.hasExited())
    #expect(child.isRunning)
}

@Test func hostProcessInspectionFailureNeverBecomesExitEvidence() async throws {
    let child = try ObservedChild(), pid = child.processID
    let initial = try #require(try MacProcessSnapshot.read(pid)), inspection = ProcessInspection(initial)
    let observation = try MacHostProcess.observe(processID:pid,effectiveUserID:geteuid(),inspect:inspection.read)
    for error in [POSIXError(.EPERM),POSIXError(.EIO),POSIXError(.EINVAL)] {
        inspection.set(.failure(error))
        #expect(throws:error) { try observation.hasExited() }
        await #expect(throws:error) { try await observation.waitForExit(timeout:0.1) }
        #expect(child.isRunning)
    }
    inspection.set(.success(initial))
    #expect(try !observation.hasExited())
    try child.finish()
    inspection.set(.success(nil))
    try await observation.waitForExit(timeout:0.1)
}

@Test func hostProcessInspectionBudget() throws {
    guard let destination = ProcessInfo.processInfo.environment["FORT_HOST_PROCESS_BENCHMARK"] else { return }
    let child = try ObservedChild()
    let observation = try MacHostProcess.observe(processID:child.processID,effectiveUserID:geteuid())
    var samples: [Double] = []
    for _ in 0..<20 {
        let started = ContinuousClock.now
        let exited = try observation.hasExited()
        let duration = started.duration(to:.now).components
        samples.append(Double(duration.seconds) * 1000 + Double(duration.attoseconds) / 1e15)
        #expect(!exited)
    }
    try JSONEncoder().encode(samples).write(to:URL(fileURLWithPath:destination))
    #expect(samples.sorted()[18] <= 5)
}

@Test func hostProcessRestoresItsRecordedLifetimeBeforeAndAfterExit() async throws {
    let child = try ObservedChild()
    let original = try MacHostProcess.observe(processID:child.processID,effectiveUserID:geteuid())
    let encoded = try JSONEncoder().encode(original.record)
    let restored = try MacHostProcess.restore(JSONDecoder().decode(MacHostProcessRecord.self,from:encoded))
    #expect(restored.processID == original.processID)
    #expect(try !restored.hasExited())
    try child.finish()
    let reopened = try MacHostProcess.restore(JSONDecoder().decode(MacHostProcessRecord.self,from:encoded))
    try await reopened.waitForExit(timeout:1)
    #expect(try restored.hasExited())
}
#endif
