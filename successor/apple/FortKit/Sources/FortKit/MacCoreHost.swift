#if os(macOS)
import Foundation
import Darwin

public enum MacCoreHostError: Error, LocalizedError {
    case unavailable, recoveryInProgress, recoveryUnconfirmed
    public var errorDescription: String? {
        switch self {
        case .unavailable: "The core host is unavailable."
        case .recoveryInProgress: "A different pairing recovery is in progress."
        case .recoveryUnconfirmed: "Core shutdown and pairing removal could not be confirmed."
        }
    }
}

public actor MacCoreHost: FortTransport, MacOwnerControl, OwnerSetupControl, OwnerSetupCancellationControl {
    private enum RecoveryPhase {
        case revocationRequired
        case cleanupRequired(MacCorePairingRecovery)
    }
    private enum State {
        case serving
        case recovering(MacOwnerRecoveryRequest, Task<MacOwnerRecoveryReceipt,any Error>)
        case recoveryRequired(RecoveryPhase)
        case recovered(MacOwnerRecoveryReceipt)
        case shuttingDown(Task<MacCoreStop,any Error>)
        case shutDown(Result<MacCoreStop,any Error>)
    }
    private let core: MacCoreProcessTransport
    private let directory: URL
    private var state = State.serving

    init(core: MacCoreProcessTransport, directory: URL) {
        self.core = core; self.directory = directory
    }

    public static func launch(executable: URL, arguments: [String], directory: URL, environment: [String:String], guardian: URL) async throws -> MacCoreHost {
        let core = try await MacCoreProcessTransport.launch(executable:executable,arguments:arguments,directory:directory,environment:environment,lifecycle:.parentExit(guardian:guardian))
        return MacCoreHost(core:core,directory:directory)
    }

    private func requireServing() throws {
        guard case .serving = state else { throw MacCoreHostError.unavailable }
    }
    public func send(_ request: FortRequest) async throws -> FortResponse {
        try requireServing(); return try await core.send(request)
    }
    public func beginOwnerSetup(timeout: TimeInterval) async throws -> OwnerSetupState {
        try requireServing(); return try await core.beginOwnerSetup(timeout:timeout)
    }
    public func completeOwnerSetup(_ request: OwnerSetupRequest, timeout: TimeInterval) async throws -> OwnerSetupReceipt {
        try requireServing(); return try await core.completeOwnerSetup(request,timeout:timeout)
    }
    public func cancelOwnerSetup(_ identity: OwnerSetupIdentity, timeout: TimeInterval) async throws -> OwnerSetupCancellation {
        try requireServing(); return try await core.cancelOwnerSetup(identity,timeout:timeout)
    }
    public func recoverPairing(_ request: MacOwnerRecoveryRequest) async throws -> MacOwnerRecoveryReceipt {
        try request.validate(); try Task.checkCancellation()
        let work: Task<MacOwnerRecoveryReceipt,any Error>
        switch state {
        case .serving:
            work = startRecovery(request,phase:.revocationRequired,resumeOnRejection:true)
        case .recoveryRequired(let phase):
            if case .cleanupRequired(let proof) = phase {
                guard proof.installationID == request.installationID, proof.deviceID == request.deviceID else { throw MacCoreHostError.recoveryUnconfirmed }
            }
            work = startRecovery(request,phase:phase,resumeOnRejection:false)
        case .recovering(let active,let pending):
            guard active.installationID == request.installationID, active.deviceID == request.deviceID else { throw MacCoreHostError.recoveryInProgress }
            work = pending
        case .recovered(let receipt):
            try receipt.validate(for:request); return receipt
        case .shuttingDown, .shutDown: throw MacCoreHostError.unavailable
        }
        let receipt = try await work.value
        try Task.checkCancellation()
        return receipt
    }
    private func startRecovery(_ request: MacOwnerRecoveryRequest, phase: RecoveryPhase, resumeOnRejection: Bool) -> Task<MacOwnerRecoveryReceipt,any Error> {
        let work = Task { try await self.performRecovery(request,phase:phase,resumeOnRejection:resumeOnRejection) }
        state = .recovering(request,work)
        return work
    }
    private func performRecovery(_ request: MacOwnerRecoveryRequest, phase: RecoveryPhase, resumeOnRejection: Bool) async throws -> MacOwnerRecoveryReceipt {
        var retained = phase
        do {
            let proof: MacCorePairingRecovery
            switch phase {
            case .revocationRequired:
                proof = try await core.recoverOwnerPairing(installationID:request.installationID,deviceID:request.deviceID,timeout:request.timeout)
            case .cleanupRequired(let prior): proof = prior
            }
            retained = .cleanupRequired(proof)
            let stopped = try await core.close()
            guard stopped.exitedCleanly && !stopped.forced else { throw MacCoreHostError.recoveryUnconfirmed }
            var info = stat()
            guard lstat(directory.appendingPathComponent("core.sock").path,&info) == -1, errno == ENOENT else { throw MacCoreHostError.recoveryUnconfirmed }
            let receipt = MacOwnerRecoveryReceipt(installationID:proof.installationID,deviceID:proof.deviceID,runtimeStopped:proof.runtimeStopped,pairingRevoked:proof.pairingRevoked,coreStopped:true)
            try receipt.validate(for:request)
            state = .recovered(receipt)
            return receipt
        } catch MacOwnerRecoveryError.identityRejected {
            if case .revocationRequired = retained, resumeOnRejection {
                state = .serving
            } else { state = .recoveryRequired(retained) }
            throw MacCoreHostError.recoveryUnconfirmed
        } catch {
            state = .recoveryRequired(retained)
            throw MacCoreHostError.recoveryUnconfirmed
        }
    }
    public func shutdown() async throws -> MacCoreStop {
        switch state {
        case .recovering(_,let pending):
            _ = await pending.result
            return try await shutdown()
        case .recovered, .recoveryRequired(.cleanupRequired):
            return try await core.close()
        case .shuttingDown(let pending):
            return try await pending.value
        case .shutDown(let result):
            return try result.get()
        case .serving, .recoveryRequired(.revocationRequired):
            let work = Task { () throws -> MacCoreStop in
                let result = await Task { try await self.core.close() }.result
                self.state = .shutDown(result)
                return try result.get()
            }
            state = .shuttingDown(work)
            return try await work.value
        }
    }
}
#endif
