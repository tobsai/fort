#if os(macOS)
import Foundation
import Darwin
import Testing
@testable import FortKit

private final class ObservedChild {
    let process = Process()
    private let input = Pipe()
    init() throws {
        process.executableURL = URL(fileURLWithPath:"/bin/cat")
        process.arguments = []; process.environment = [:]
        process.standardInput = input
        process.standardOutput = FileHandle.nullDevice
        process.standardError = FileHandle.nullDevice
        try process.run()
    }
    func finish() throws {
        try input.fileHandleForWriting.close()
        process.waitUntilExit()
    }
    deinit { try? input.fileHandleForWriting.close(); process.waitUntilExit() }
}

@Test func hostProcessWaitRequiresExitAndNeverStopsTheObservedProcess() async throws {
    let child = try ObservedChild()
    let observation = try MacHostProcess.observe(processID:child.process.processIdentifier,effectiveUserID:geteuid())
    #expect(observation.processID == child.process.processIdentifier)
    #expect(try !observation.hasExited())
    await #expect(throws:URLError(.timedOut)) { try await observation.waitForExit(timeout:0.03) }
    #expect(child.process.isRunning)
    #expect(try !observation.hasExited())
    try child.finish()
    try await observation.waitForExit(timeout:1)
    try await observation.waitForExit(timeout:1)
    #expect(try observation.hasExited())
}

@Test func hostProcessWaitCancellationIsCallerLocal() async throws {
    let child = try ObservedChild()
    let observation = try MacHostProcess.observe(processID:child.process.processIdentifier,effectiveUserID:geteuid())
    let canceled = Task { try await observation.waitForExit(timeout:2) }
    let other = Task { try await observation.waitForExit(timeout:2) }
    canceled.cancel()
    await #expect(throws:CancellationError.self) { try await canceled.value }
    #expect(child.process.isRunning)
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
    let child = try ObservedChild(), pid = child.process.processIdentifier
    #expect(throws:ClientError.self) { try MacHostProcess.observe(processID:pid,effectiveUserID:geteuid() + 1) }
    #expect(throws:ClientError.self) { try MacHostProcess.observe(processID:pid,effectiveUserID:geteuid(),inspect:{ _ in nil }) }
    #expect(throws:POSIXError(.EPERM)) { try MacHostProcess.observe(processID:pid,effectiveUserID:geteuid(),inspect:{ _ in throw POSIXError(.EPERM) }) }
    let observation = try MacHostProcess.observe(processID:pid,effectiveUserID:geteuid())
    for timeout in [Double.nan,Double.infinity,-1,0,30.01] {
        await #expect(throws:ClientError.self) { try await observation.waitForExit(timeout:timeout) }
    }
    #expect(child.process.isRunning)
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
    let child = try ObservedChild(), pid = child.process.processIdentifier
    let initial = try #require(try MacProcessSnapshot.read(pid))
    let inspection = ProcessInspection(initial)
    let observation = try MacHostProcess.observe(processID:pid,effectiveUserID:geteuid(),inspect:inspection.read)
    inspection.set(.success(MacProcessSnapshot(effectiveUserID:geteuid() + 1,startedSeconds:initial.startedSeconds,startedMicroseconds:initial.startedMicroseconds)))
    #expect(try !observation.hasExited())
    inspection.set(.success(MacProcessSnapshot(effectiveUserID:geteuid(),startedSeconds:initial.startedSeconds + 1,startedMicroseconds:initial.startedMicroseconds)))
    try await observation.waitForExit(timeout:0.1)
    #expect(try observation.hasExited())
    #expect(child.process.isRunning)
}

@Test func hostProcessInspectionFailureNeverBecomesExitEvidence() async throws {
    let child = try ObservedChild(), pid = child.process.processIdentifier
    let initial = try #require(try MacProcessSnapshot.read(pid)), inspection = ProcessInspection(initial)
    let observation = try MacHostProcess.observe(processID:pid,effectiveUserID:geteuid(),inspect:inspection.read)
    for error in [POSIXError(.EPERM),POSIXError(.EIO),POSIXError(.EINVAL)] {
        inspection.set(.failure(error))
        #expect(throws:error) { try observation.hasExited() }
        await #expect(throws:error) { try await observation.waitForExit(timeout:0.1) }
        #expect(child.process.isRunning)
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
    let observation = try MacHostProcess.observe(processID:child.process.processIdentifier,effectiveUserID:geteuid())
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
    let original = try MacHostProcess.observe(processID:child.process.processIdentifier,effectiveUserID:geteuid())
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
