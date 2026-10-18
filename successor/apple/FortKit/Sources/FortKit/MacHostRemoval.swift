#if os(macOS)
import Foundation

public enum MacHostRemovalError: Error, LocalizedError {
    case busy, changedProgress, setupBlocked
    public var errorDescription: String? {
        switch self {
        case .busy: "Host removal is already in progress."
        case .changedProgress: "The saved host removal changed. Its pairing and history have been kept."
        case .setupBlocked: "Finish the saved host removal before preparing another connection."
        }
    }
}

private struct HostRemovalRecord: Codable {
    enum Progress {
        case requested
        case recovered(MacOwnerRecoveryReceipt,MacHostProcessRecord)
        case confirmed(MacOwnerRecoveryReceipt,MacHostProcessRecord)
    }
    let connection: DeviceConnection
    let progress: Progress
    enum Keys: String, CodingKey { case version, connection, phase, receipt, host }
    init(connection: DeviceConnection, progress: Progress) throws {
        guard case .macAgent = connection.endpoint else { throw OwnerLifecycleError.wrongPairing }
        let request = try MacOwnerRecoveryRequest(installationID:connection.installationID,deviceID:connection.deviceID)
        switch progress {
        case .requested: break
        case .recovered(let receipt,let host), .confirmed(let receipt,let host):
            try receipt.validate(for:request); try host.validate()
        }
        self.connection = connection; self.progress = progress
    }
    init(from decoder: any Decoder) throws {
        let values = try decoder.container(keyedBy:Keys.self)
        guard try values.decode(Int.self,forKey:.version) == 1 else { throw MacHostRemovalError.changedProgress }
        let phase: Progress
        switch try values.decode(String.self,forKey:.phase) {
        case "requested":
            guard !values.contains(.receipt), !values.contains(.host) else { throw MacHostRemovalError.changedProgress }
            phase = .requested
        case "recovered": phase = .recovered(try values.decode(MacOwnerRecoveryReceipt.self,forKey:.receipt),try values.decode(MacHostProcessRecord.self,forKey:.host))
        case "confirmed": phase = .confirmed(try values.decode(MacOwnerRecoveryReceipt.self,forKey:.receipt),try values.decode(MacHostProcessRecord.self,forKey:.host))
        default: throw MacHostRemovalError.changedProgress
        }
        try self.init(connection:values.decode(DeviceConnection.self,forKey:.connection),progress:phase)
    }
    func encode(to encoder: any Encoder) throws {
        var values = encoder.container(keyedBy:Keys.self)
        try values.encode(1,forKey:.version); try values.encode(connection,forKey:.connection)
        switch progress {
        case .requested: try values.encode("requested",forKey:.phase)
        case .recovered(let receipt,let host), .confirmed(let receipt,let host):
            try values.encode(isConfirmed ? "confirmed" : "recovered",forKey:.phase)
            try values.encode(receipt,forKey:.receipt); try values.encode(host,forKey:.host)
        }
    }
    var isConfirmed: Bool { if case .confirmed = progress { true } else { false } }
    var observation: MacHostRecoveryObservation? {
        get throws {
            switch progress {
            case .requested: nil
            case .recovered(let receipt,let record), .confirmed(let receipt,let record):
                MacHostRecoveryObservation(receipt:receipt,host:try MacHostProcess.restore(record))
            }
        }
    }
}

@MainActor public final class MacHostRemoval {
    public let connection: DeviceConnection
    private let directory: URL
    private let startup: MacHostStartup
    private let recover: @Sendable (MacOwnerRecoveryRequest) async throws -> MacHostRecoveryObservation
    private let exitTimeout: TimeInterval
    private var active = false

    public convenience init(connection: DeviceConnection, directory: URL, exitTimeout: TimeInterval = 3) throws {
        guard case .macAgent(let endpoint) = connection.endpoint else { throw OwnerLifecycleError.wrongPairing }
        try self.init(connection:connection,directory:directory,startup:MacHostStartup(endpoint:endpoint),recover:MacXPCTransport(agent:endpoint).recoverPairingAndObserveHost,exitTimeout:exitTimeout)
    }
    init(connection: DeviceConnection, directory: URL, startup: MacHostStartup, recover: @escaping @Sendable (MacOwnerRecoveryRequest) async throws -> MacHostRecoveryObservation, exitTimeout: TimeInterval) throws {
        guard case .macAgent(let endpoint) = connection.endpoint, endpoint == startup.endpoint,
              exitTimeout.isFinite, exitTimeout > 0, exitTimeout <= 30 else { throw OwnerLifecycleError.wrongPairing }
        _ = try MacOwnerRecoveryRequest(installationID:connection.installationID,deviceID:connection.deviceID)
        self.connection = connection; self.directory = directory; self.startup = startup
        self.recover = recover; self.exitTimeout = exitTimeout
    }
    private func verifyConnection(_ files: OwnerSetupFiles) throws {
        guard try files.read(.pending) == nil else { throw MacHostRemovalError.setupBlocked }
        let decoder = JSONDecoder()
        let current = try files.read(.connection).map { try decoder.decode(DeviceConnection.self,from:$0) }
        let disconnected = try files.read(.disconnected).map { try decoder.decode(DeviceConnection.self,from:$0) }
        guard current == connection || (current == nil && disconnected == connection) else { throw OwnerLifecycleError.wrongPairing }
        if let intent = try files.read(.removal), try decoder.decode(PairingRemovalIntent.self,from:intent).connection != connection { throw OwnerLifecycleError.wrongPairing }
    }
    private func verifyProgress(_ files: OwnerSetupFiles, bytes: Data) throws {
        try verifyConnection(files)
        guard try files.read(.hostRemoval) == bytes else { throw MacHostRemovalError.changedProgress }
    }
    public func stopAndRevoke() async throws -> OwnedHostStop {
        guard !active else { throw MacHostRemovalError.busy }
        try Task.checkCancellation()
        active = true
        defer { active = false }
        let files = try OwnerSetupFiles(directory:directory)
        defer { files.close() }
        try verifyConnection(files)
        var record: HostRemovalRecord
        var saved: Data
        if let prior = try files.read(.hostRemoval) {
            record = try JSONDecoder().decode(HostRemovalRecord.self,from:prior)
            guard record.connection == connection else { throw OwnerLifecycleError.wrongPairing }
            saved = prior
        } else {
            record = try HostRemovalRecord(connection:connection,progress:.requested)
            saved = try JSONEncoder().encode(record)
            try files.write(saved,to:.hostRemoval)
        }
        let status = try await startup.inspect()
        try verifyProgress(files,bytes:saved)
        var observation = try record.observation
        if status == .notRegistered {
            guard observation != nil else { throw OwnerLifecycleError.stopUnconfirmed }
        } else {
            guard !record.isConfirmed, status == .enabled || status == .requiresApproval else { throw OwnerLifecycleError.startupUnconfirmed }
            let request = try MacOwnerRecoveryRequest(installationID:connection.installationID,deviceID:connection.deviceID)
            let recovered = try await recover(request)
            try recovered.receipt.validate(for:request)
            try verifyProgress(files,bytes:saved)
            record = try HostRemovalRecord(connection:connection,progress:.recovered(recovered.receipt,recovered.host.record))
            saved = try JSONEncoder().encode(record)
            try files.write(saved,to:.hostRemoval)
            observation = recovered
        }
        guard let observation else { throw OwnerLifecycleError.stopUnconfirmed }
        try Task.checkCancellation()
        if status != .notRegistered {
            guard try await startup.disable() == .notRegistered else { throw OwnerLifecycleError.startupUnconfirmed }
        }
        try verifyProgress(files,bytes:saved)
        try await observation.host.waitForExit(timeout:exitTimeout)
        guard try await startup.inspect() == .notRegistered else { throw OwnerLifecycleError.startupUnconfirmed }
        try verifyProgress(files,bytes:saved)
        if !record.isConfirmed {
            let confirmed = try HostRemovalRecord(connection:connection,progress:.confirmed(observation.receipt,observation.host.record))
            try files.write(JSONEncoder().encode(confirmed),to:.hostRemoval)
        }
        return OwnedHostStop(hostStopped:true,pairingRevoked:true,startupUnregistered:true)
    }
}
#endif
