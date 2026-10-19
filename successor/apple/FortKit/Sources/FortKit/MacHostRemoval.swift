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

private struct HostHistoryRemovalRecord: Codable {
    let version: Int
    let connection: DeviceConnection
    let request: MacHistoryDeletionRequest
    var receipt: MacHistoryDeletionReceipt?
    var complete: Bool
    func validate(for expected: DeviceConnection) throws {
        guard version == 1, connection == expected, request.installationID == expected.installationID,
              request.deviceID == expected.deviceID, !complete || receipt != nil else { throw MacHostRemovalError.changedProgress }
        if let receipt { try receipt.validate(for:request) }
    }
}

@MainActor struct MacHistoryRemovalActions {
    let verifyCredentialAbsent: () throws -> Void
    let maintain: (MacHistoryDeletionRequest) async throws -> MacHistoryDeletionReceipt
    let removeSavedHistory: () throws -> Void
}

@MainActor public final class MacHostRemoval: OwnedMacInstallation {
    public let appURL: URL
    private let verifyBundle: @MainActor () async throws -> Void
    public let connection: DeviceConnection
    private let directory: URL
    private let startup: MacHostStartup
    private let recover: @Sendable (MacOwnerRecoveryRequest) async throws -> MacHostRecoveryObservation
    private let exitTimeout: TimeInterval
    private var active = false
    private let historyActions: MacHistoryRemovalActions?
    private let removeProviderCredential: @MainActor () throws -> Void

    init(connection: DeviceConnection, directory: URL, applicationURL: URL, verifyBundle: @escaping @MainActor () async throws -> Void, startup: MacHostStartup, recover: @escaping @Sendable (MacOwnerRecoveryRequest) async throws -> MacHostRecoveryObservation, exitTimeout: TimeInterval, removeProviderCredential: @escaping @MainActor () throws -> Void, historyActions: MacHistoryRemovalActions? = nil) throws {
        guard case .macAgent(let endpoint) = connection.endpoint, endpoint == startup.endpoint,
              exitTimeout.isFinite, exitTimeout > 0, exitTimeout <= 30 else { throw OwnerLifecycleError.wrongPairing }
        _ = try MacOwnerRecoveryRequest(installationID:connection.installationID,deviceID:connection.deviceID)
        guard applicationURL.isFileURL, applicationURL.path.hasPrefix("/"), applicationURL.host == nil || applicationURL.host == "" else { throw MacHostBundleError.configuration }
        appURL = applicationURL; self.verifyBundle = verifyBundle
        self.connection = connection; self.directory = directory; self.startup = startup
        self.recover = recover; self.exitTimeout = exitTimeout; self.historyActions = historyActions
        self.removeProviderCredential = removeProviderCredential
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
    public func recoverPairingRemoval(_ expected: DeviceConnection) async throws -> PairingRecoveryProof {
        guard expected == connection else { throw OwnerLifecycleError.wrongPairing }
        guard !active else { throw MacHostRemovalError.busy }
        try Task.checkCancellation()
        active = true; defer { active = false }
        try await verifyBundle()
        try Task.checkCancellation()
        let files = try OwnerSetupFiles(directory:directory); defer { files.close() }
        try verifyConnection(files)
        guard try files.read(.hostRemoval) == nil else { throw MacHostRemovalError.setupBlocked }
        let request = try MacOwnerRecoveryRequest(installationID:connection.installationID,deviceID:connection.deviceID)
        let observation = try await recover(request)
        try observation.receipt.validate(for:request)
        try verifyConnection(files)
        return PairingRecoveryProof(connection:connection,hostStopped:observation.receipt.coreStopped,pairingRevoked:observation.receipt.pairingRevoked)
    }
    public func removeHistory() throws { throw OwnerLifecycleError.historyDeletionUnavailable }
    public var supportsRetainedHistoryDeletion: Bool { historyActions != nil }
    public func historyDeletionRequest(recoveryEpoch: String?) throws -> MacHistoryDeletionRequest {
        guard !active else { throw MacHostRemovalError.busy }
        guard supportsRetainedHistoryDeletion else { throw OwnerLifecycleError.historyDeletionUnavailable }
        let files = try OwnerSetupFiles(directory:directory); defer { files.close() }
        try Self.verifyLocalCleanup(files,connection:connection)
        guard let host = try files.read(.hostRemoval) else { throw OwnerLifecycleError.stopUnconfirmed }
        let removal = try JSONDecoder().decode(HostRemovalRecord.self,from:host)
        guard removal.connection == connection, removal.isConfirmed else { throw OwnerLifecycleError.stopUnconfirmed }
        if let data = try files.read(.historyRemoval) {
            let saved = try JSONDecoder().decode(HostHistoryRemovalRecord.self,from:data)
            try saved.validate(for:connection)
            guard !saved.complete else { throw OwnerLifecycleError.historyDeletionUnavailable }
            guard recoveryEpoch == nil || recoveryEpoch == saved.request.recoveryEpoch || recoveryEpoch == saved.receipt?.recoveryEpoch else { throw MacHostRemovalError.changedProgress }
            return saved.request
        }
        guard let recoveryEpoch else { throw OwnerLifecycleError.historyDeletionUnavailable }
        return try MacHistoryDeletionRequest(installationID:connection.installationID,deviceID:connection.deviceID,recoveryEpoch:recoveryEpoch,deletionID:UUID().uuidString)
    }
    public func deleteRetainedHistory(_ request: MacHistoryDeletionRequest) async throws -> MacHistoryDeletionReceipt {
        guard !active else { throw MacHostRemovalError.busy }
        guard let historyActions else { throw OwnerLifecycleError.historyDeletionUnavailable }
        guard request.installationID == connection.installationID, request.deviceID == connection.deviceID else { throw OwnerLifecycleError.wrongPairing }
        try Task.checkCancellation()
        active = true; defer { active = false }
        let files = try OwnerSetupFiles(directory:directory); defer { files.close() }
        try Self.verifyLocalCleanup(files,connection:connection)
        guard let hostBytes = try files.read(.hostRemoval) else { throw OwnerLifecycleError.stopUnconfirmed }
        let host = try JSONDecoder().decode(HostRemovalRecord.self,from:hostBytes)
        guard host.connection == connection, host.isConfirmed, let observation = try host.observation else { throw OwnerLifecycleError.stopUnconfirmed }
        var saved = try files.read(.historyRemoval)
        var record = try saved.map { try JSONDecoder().decode(HostHistoryRemovalRecord.self,from:$0) }
            ?? HostHistoryRemovalRecord(version:1,connection:connection,request:request,receipt:nil,complete:false)
        try record.validate(for:connection)
        guard record.request == request else { throw MacHostRemovalError.changedProgress }
        func unchanged(_ files: OwnerSetupFiles) throws {
            try Self.verifyLocalCleanup(files,connection:connection)
            guard try files.read(.hostRemoval) == hostBytes, try files.read(.historyRemoval) == saved else { throw MacHostRemovalError.changedProgress }
            try historyActions.verifyCredentialAbsent()
            try Self.verifyLocalCleanup(files,connection:connection)
            guard try files.read(.hostRemoval) == hostBytes, try files.read(.historyRemoval) == saved else { throw MacHostRemovalError.changedProgress }
            try Task.checkCancellation()
        }
        func verify(_ files: OwnerSetupFiles) async throws {
            try unchanged(files)
            try await verifyBundle(); try unchanged(files)
            guard try await startup.inspect() == .notRegistered else { throw OwnerLifecycleError.startupUnconfirmed }
            try unchanged(files)
            try await observation.host.waitForExit(timeout:exitTimeout); try unchanged(files)
            guard try await startup.inspect() == .notRegistered else { throw OwnerLifecycleError.startupUnconfirmed }
            try unchanged(files)
        }
        try await verify(files)
        if saved == nil { let data = try JSONEncoder().encode(record); try files.write(data,to:.historyRemoval); saved = data }
        try unchanged(files)
        let receipt = try await historyActions.maintain(request)
        try receipt.validate(for:request)
        try await verify(files)
        if let previous = record.receipt, previous != receipt { throw MacHostRemovalError.changedProgress }
        record.receipt = receipt; record.complete = false
        let data = try JSONEncoder().encode(record); try files.write(data,to:.historyRemoval); saved = data
        files.close()
        try historyActions.removeSavedHistory()
        let finalFiles = try OwnerSetupFiles(directory:directory); defer { finalFiles.close() }
        try await verify(finalFiles)
        record.complete = true
        try finalFiles.write(JSONEncoder().encode(record),to:.historyRemoval)
        return receipt
    }

    private static func verifyLocalCleanup(_ files: OwnerSetupFiles, connection: DeviceConnection) throws {
        guard try files.read(.pending) == nil, try files.read(.connection) == nil,
              let disconnected = try files.read(.disconnected), let removal = try files.read(.removal),
              try JSONDecoder().decode(DeviceConnection.self,from:disconnected) == connection else { throw MacHostRemovalError.setupBlocked }
        let intent = try JSONDecoder().decode(PairingRemovalIntent.self,from:removal)
        guard intent.connection == connection, intent.state == .confirmed else { throw MacHostRemovalError.setupBlocked }
    }
    static func completedConnection(_ files: OwnerSetupFiles, endpoint: MacAgentEndpoint) throws -> DeviceConnection? {
        guard let bytes = try files.read(.hostRemoval) else { return nil }
        let record = try JSONDecoder().decode(HostRemovalRecord.self,from:bytes)
        guard record.isConfirmed, record.connection.endpoint == .macAgent(endpoint) else { throw MacHostRemovalError.setupBlocked }
        try verifyLocalCleanup(files,connection:record.connection)
        return record.connection
    }
    public func prepareForSetup(verifyCredentialAbsent: @MainActor () throws -> Void) async throws {
        guard !active else { throw MacHostRemovalError.busy }
        try Task.checkCancellation()
        active = true; defer { active = false }
        try await verifyBundle()
        try Task.checkCancellation()
        let files = try OwnerSetupFiles(directory:directory); defer { files.close() }
        try Self.verifyLocalCleanup(files,connection:connection)
        guard let bytes = try files.read(.hostRemoval) else { return }
        let record = try JSONDecoder().decode(HostRemovalRecord.self,from:bytes)
        guard record.connection == connection, record.isConfirmed, let observation = try record.observation else { throw MacHostRemovalError.setupBlocked }
        func unchanged() throws {
            try Self.verifyLocalCleanup(files,connection:connection)
            guard try files.read(.hostRemoval) == bytes else { throw MacHostRemovalError.changedProgress }
        }
        guard try await startup.inspect() == .notRegistered else { throw OwnerLifecycleError.startupUnconfirmed }
        try unchanged()
        try await observation.host.waitForExit(timeout:exitTimeout)
        guard try await startup.inspect() == .notRegistered else { throw OwnerLifecycleError.startupUnconfirmed }
        try unchanged()
        try Task.checkCancellation()
        try verifyCredentialAbsent()
        try unchanged()
        try Task.checkCancellation()
        if let history = try files.read(.historyRemoval) {
            try JSONDecoder().decode(HostHistoryRemovalRecord.self,from:history).validate(for:connection)
            try files.removeHistoryRemoval()
        }
        try files.removeHostRemoval()
    }

    public func stopAndRevoke() async throws -> OwnedHostStop {
        guard !active else { throw MacHostRemovalError.busy }
        try Task.checkCancellation()
        active = true
        defer { active = false }
        try await verifyBundle()
        try Task.checkCancellation()
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
        try removeProviderCredential()
        try verifyProgress(files,bytes:saved)
        if !record.isConfirmed {
            let confirmed = try HostRemovalRecord(connection:connection,progress:.confirmed(observation.receipt,observation.host.record))
            try files.write(JSONEncoder().encode(confirmed),to:.hostRemoval)
        }
        return OwnedHostStop(hostStopped:true,pairingRevoked:true,startupUnregistered:true,providerCredentialRemoved:true)
    }
}
#endif
