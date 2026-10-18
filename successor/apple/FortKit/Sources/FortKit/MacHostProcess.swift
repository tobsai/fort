#if os(macOS)
import Foundation
import Darwin

public struct MacHostRecoveryObservation: Sendable {
    public let receipt: MacOwnerRecoveryReceipt
    public let host: MacHostProcess
}

public struct MacHostProcess: Sendable {
    public let processID: Int32
    let record: MacHostProcessRecord
    private let inspect: @Sendable (Int32) throws -> MacProcessSnapshot?

    private init(record: MacHostProcessRecord, inspect: @escaping @Sendable (Int32) throws -> MacProcessSnapshot?) {
        self.processID = record.processID; self.record = record; self.inspect = inspect
    }
    static func restore(_ record: MacHostProcessRecord, inspect: @escaping @Sendable (Int32) throws -> MacProcessSnapshot? = MacProcessSnapshot.read) throws -> MacHostProcess {
        try record.validate()
        return MacHostProcess(record:record,inspect:inspect)
    }
    static func observe(processID: Int32, effectiveUserID: UInt32, inspect: @escaping @Sendable (Int32) throws -> MacProcessSnapshot? = MacProcessSnapshot.read) throws -> MacHostProcess {
        guard processID > 1, processID != getpid(), effectiveUserID == geteuid(),
              let snapshot = try inspect(processID), snapshot.effectiveUserID == effectiveUserID,
              snapshot.startedSeconds > 0, snapshot.startedMicroseconds < 1_000_000 else { throw ClientError.invalidResponse }
        return try restore(MacHostProcessRecord(processID:processID,effectiveUserID:effectiveUserID,startedSeconds:snapshot.startedSeconds,startedMicroseconds:snapshot.startedMicroseconds),inspect:inspect)
    }
    public func hasExited() throws -> Bool {
        guard let current = try inspect(processID) else { return true }
        return current.startedSeconds != record.startedSeconds || current.startedMicroseconds != record.startedMicroseconds
    }
    public func waitForExit(timeout: TimeInterval = 3) async throws {
        guard timeout.isFinite, timeout > 0, timeout <= 30 else { throw ClientError.invalidResponse }
        let clock = ContinuousClock(), deadline = ContinuousClock.now.advanced(by:.seconds(timeout))
        while true {
            try Task.checkCancellation()
            if try hasExited() { return }
            let remaining = clock.now.duration(to:deadline)
            guard remaining > .zero else { throw URLError(.timedOut) }
            try await clock.sleep(for:min(remaining,.milliseconds(25)))
        }
    }
}

struct MacHostProcessRecord: Codable, Sendable {
    let processID: Int32
    let effectiveUserID: UInt32
    let startedSeconds: UInt64
    let startedMicroseconds: UInt64
    func validate() throws {
        guard processID > 1, effectiveUserID == geteuid(), startedSeconds > 0, startedMicroseconds < 1_000_000 else { throw ClientError.invalidResponse }
    }
}

struct MacProcessSnapshot: Sendable {
    let effectiveUserID: UInt32
    let startedSeconds: UInt64
    let startedMicroseconds: UInt64
    static func read(_ pid: Int32) throws -> MacProcessSnapshot? {
        var info = proc_bsdinfo()
        let size = Int32(MemoryLayout<proc_bsdinfo>.stride)
        errno = 0
        let count = proc_pidinfo(pid,PROC_PIDTBSDINFO,0,&info,size)
        let failure = errno
        if count == 0 && failure == ESRCH { return nil }
        guard count == size else { throw POSIXError(POSIXErrorCode(rawValue:failure) ?? .EIO) }
        guard info.pbi_pid == UInt32(pid), info.pbi_start_tvsec > 0, info.pbi_start_tvusec < 1_000_000 else { throw ClientError.invalidResponse }
        return MacProcessSnapshot(effectiveUserID:info.pbi_uid,startedSeconds:info.pbi_start_tvsec,startedMicroseconds:info.pbi_start_tvusec)
    }
}
#endif
