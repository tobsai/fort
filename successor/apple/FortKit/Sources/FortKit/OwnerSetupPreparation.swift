#if os(macOS)
import Foundation
import CryptoKit

public enum OwnerSetupPreparationError: Error, LocalizedError {
    case existingPairing, wrongHost, changedPreparation, missingCredential, busy, randomUnavailable, cancellationPending
    public var errorDescription: String? {
        switch self {
        case .existingPairing: "This host already has a paired device. Its pairing has been kept."
        case .wrongHost: "The saved setup belongs to a different host. Its connection and credential have been kept."
        case .changedPreparation: "The saved setup changed or could not be confirmed. Its records have been kept."
        case .missingCredential: "The prepared credential is missing. Setup has been kept for recovery; no replacement was generated."
        case .busy: "Owner setup is already in progress."
        case .randomUnavailable: "A secure device credential could not be prepared."
        case .cancellationPending: "Finish canceling the saved setup before starting another connection."
        }
    }
}

public struct PendingOwnerSetup: Equatable, Sendable {
    public enum Phase: String, Sendable { case preparing, ready, canceling, canceled }
    public let identity: OwnerSetupIdentity
    public let endpoint: DeviceEndpoint
    public let phase: Phase
    public let offerExpiresAt: String
}

private func setupCredentialDigest(_ token: String) -> String {
    SHA256.hash(data:Data(token.utf8)).map { String(format:"%02x",$0) }.joined()
}

private struct OwnerSetupPlan: Codable, Equatable {
    enum Phase: Equatable {
        case preparing, ready(String), canceling(String?), canceled(String?,OwnerSetupCancellation)
        var digest: String? {
            switch self { case .preparing: nil; case .ready(let digest): digest; case .canceling(let digest),.canceled(let digest,_): digest }
        }
        var publicPhase: PendingOwnerSetup.Phase {
            switch self { case .preparing: .preparing; case .ready: .ready; case .canceling: .canceling; case .canceled: .canceled }
        }
    }
    let connection: DeviceConnection
    let offer: OwnerSetupOffer
    var phase: Phase
    var identity: OwnerSetupIdentity {
        get throws { try OwnerSetupIdentity(installationID:connection.installationID,setupID:offer.setupID,deviceID:connection.deviceID) }
    }
    enum Keys: String, CodingKey { case version, connection, offer, phase, credentialSHA256, cancellation }
    init(connection: DeviceConnection, offer: OwnerSetupOffer, phase: Phase) throws {
        guard connection.installationID == offer.installationID, connection.endpoint.isLocalHost else { throw OwnerSetupPreparationError.changedPreparation }
        _ = try OwnerSetupReceipt(installationID:connection.installationID,deviceID:connection.deviceID,expiresAt:nil)
        if let digest = phase.digest {
            guard digest.utf8.count == 64, digest.rangeOfCharacter(from:CharacterSet(charactersIn:"0123456789abcdef").inverted) == nil else { throw OwnerSetupPreparationError.changedPreparation }
        }
        if case .canceled(_,let receipt) = phase {
            guard receipt.identity == (try OwnerSetupIdentity(installationID:connection.installationID,setupID:offer.setupID,deviceID:connection.deviceID)) else { throw OwnerSetupPreparationError.changedPreparation }
        }
        self.connection = connection; self.offer = offer; self.phase = phase
    }
    init(from decoder: any Decoder) throws {
        let value = try decoder.container(keyedBy:Keys.self)
        guard try value.decode(Int.self,forKey:.version) == 1 else { throw OwnerSetupPreparationError.changedPreparation }
        let phase: Phase
        switch try value.decode(String.self,forKey:.phase) {
        case "preparing":
            guard !value.contains(.credentialSHA256), !value.contains(.cancellation) else { throw OwnerSetupPreparationError.changedPreparation }
            phase = .preparing
        case "ready":
            guard !value.contains(.cancellation) else { throw OwnerSetupPreparationError.changedPreparation }
            phase = .ready(try value.decode(String.self,forKey:.credentialSHA256))
        case "canceling":
            guard !value.contains(.cancellation) else { throw OwnerSetupPreparationError.changedPreparation }
            phase = .canceling(try value.decodeIfPresent(String.self,forKey:.credentialSHA256))
        case "canceled": phase = .canceled(try value.decodeIfPresent(String.self,forKey:.credentialSHA256),try value.decode(OwnerSetupCancellation.self,forKey:.cancellation))
        default: throw OwnerSetupPreparationError.changedPreparation
        }
        try self.init(connection:value.decode(DeviceConnection.self,forKey:.connection),offer:value.decode(OwnerSetupOffer.self,forKey:.offer),phase:phase)
    }
    func encode(to encoder: any Encoder) throws {
        var value = encoder.container(keyedBy:Keys.self)
        try value.encode(1,forKey:.version); try value.encode(connection,forKey:.connection)
        try value.encode(offer,forKey:.offer)
        switch phase {
        case .preparing: try value.encode("preparing",forKey:.phase)
        case .ready(let digest): try value.encode("ready",forKey:.phase); try value.encode(digest,forKey:.credentialSHA256)
        case .canceling(let digest): try value.encode("canceling",forKey:.phase); try value.encodeIfPresent(digest,forKey:.credentialSHA256)
        case .canceled(let digest,let receipt):
            try value.encode("canceled",forKey:.phase); try value.encodeIfPresent(digest,forKey:.credentialSHA256)
            try value.encode(receipt,forKey:.cancellation)
        }
    }
}

@MainActor public final class OwnerSetupCoordinator {
    private let storage: DeviceStorage
    private let endpoint: DeviceEndpoint
    private let control: any OwnerSetupControl
    private let tokenGenerator: () throws -> String
    public convenience init(storage: DeviceStorage, serviceIdentifier: String, signingTeam: String) throws {
        try self.init(storage:storage,endpoint:.macHost(serviceIdentifier:serviceIdentifier,signingTeam:signingTeam),control:MacXPCTransport(serviceIdentifier:serviceIdentifier,signingTeam:signingTeam),tokenGenerator:DeviceStorage.makeOwnerDeviceToken)
    }
    public convenience init(storage: DeviceStorage, agent: MacAgentEndpoint) throws {
        try self.init(storage:storage,endpoint:.macAgent(agent),control:MacXPCTransport(agent:agent),tokenGenerator:DeviceStorage.makeOwnerDeviceToken)
    }
    init(storage: DeviceStorage, endpoint: DeviceEndpoint, control: any OwnerSetupControl, tokenGenerator: @escaping () throws -> String) throws {
        guard endpoint.isLocalHost else { throw OwnerSetupPreparationError.wrongHost }
        try endpoint.validate()
        self.storage = storage; self.endpoint = endpoint; self.control = control; self.tokenGenerator = tokenGenerator
    }
    private func matchingConnection(_ files: OwnerSetupFiles, for plan: OwnerSetupPlan) throws -> DeviceConnection? {
        let current = try files.read(.connection).map { try JSONDecoder().decode(DeviceConnection.self,from:$0) }
        guard plan.connection.endpoint == endpoint, current == nil || current == plan.connection else { throw OwnerSetupPreparationError.wrongHost }
        return current
    }
    private func verifyPlan(_ plan: OwnerSetupPlan, files: OwnerSetupFiles) throws {
        guard let saved = try files.read(.pending), try JSONDecoder().decode(OwnerSetupPlan.self,from:saved) == plan else { throw OwnerSetupPreparationError.changedPreparation }
        _ = try matchingConnection(files,for:plan)
    }
    public func pendingSetup() throws -> PendingOwnerSetup? {
        let files = try OwnerSetupFiles(directory:storage.directory)
        defer { files.close() }
        guard let saved = try files.read(.pending) else { return nil }
        let plan = try JSONDecoder().decode(OwnerSetupPlan.self,from:saved)
        _ = try matchingConnection(files,for:plan)
        return try PendingOwnerSetup(identity:plan.identity,endpoint:plan.connection.endpoint,phase:plan.phase.publicPhase,offerExpiresAt:plan.offer.expiresAt)
    }
    public func cancelPendingSetup(_ expected: OwnerSetupIdentity) async throws -> OwnerSetupCancellation? {
        try Task.checkCancellation()
        let files = try OwnerSetupFiles(directory:storage.directory)
        defer { files.close() }
        let decoder = JSONDecoder(), encoder = JSONEncoder()
        guard let saved = try files.read(.pending) else { return nil }
        var plan = try decoder.decode(OwnerSetupPlan.self,from:saved)
        guard try plan.identity == expected else { throw OwnerSetupPreparationError.changedPreparation }
        _ = try matchingConnection(files,for:plan)
        let receipt: OwnerSetupCancellation
        if case .canceled(_,let confirmed) = plan.phase { receipt = confirmed }
        else {
            guard let cancellation = control as? any OwnerSetupCancellationControl else { throw ownerSetupUnavailable() }
            if case .canceling = plan.phase {} else {
                plan.phase = .canceling(plan.phase.digest)
                try files.write(encoder.encode(plan),to:.pending)
            }
            try Task.checkCancellation()
            receipt = try await cancellation.cancelOwnerSetup(expected,timeout:30)
            try Task.checkCancellation()
            guard receipt.identity == expected else { throw OwnerSetupPreparationError.changedPreparation }
            try verifyPlan(plan,files:files)
            plan.phase = .canceled(plan.phase.digest,receipt)
            try files.write(encoder.encode(plan),to:.pending)
        }
        try Task.checkCancellation()
        try verifyPlan(plan,files:files)
        let token = try storage.ownerSetupCredential(for:plan.connection)
        if let token {
            _ = try OwnerSetupRequest(installationID:expected.installationID,setupID:expected.setupID,deviceID:expected.deviceID,deviceToken:token)
            let digest = setupCredentialDigest(token)
            guard plan.phase.digest == nil || plan.phase.digest == digest else { throw OwnerSetupPreparationError.changedPreparation }
            if plan.phase.digest == nil {
                plan.phase = .canceled(digest,receipt)
                try files.write(encoder.encode(plan),to:.pending)
            }
        }
        let current = try matchingConnection(files,for:plan)
        if current != nil {
            if let prior = try files.read(.removal), try decoder.decode(PairingRemovalIntent.self,from:prior).connection != plan.connection { throw OwnerSetupPreparationError.changedPreparation }
            if let prior = try files.read(.disconnected), try decoder.decode(DeviceConnection.self,from:prior) != plan.connection { throw OwnerSetupPreparationError.changedPreparation }
            try files.write(encoder.encode(PairingRemovalIntent(connection:plan.connection,state:.confirmed)),to:.removal)
            try files.write(encoder.encode(plan.connection),to:.disconnected)
        }
        try storage.removeOwnerSetupCredential(plan.connection,expectedToken:token)
        try verifyPlan(plan,files:files)
        if try matchingConnection(files,for:plan) != nil { try files.removeConnection() }
        try files.removePending()
        return receipt
    }
    public func connect() async throws -> DeviceConnection {
        try Task.checkCancellation()
        let files = try OwnerSetupFiles(directory:storage.directory)
        defer { files.close() }
        let decoder = JSONDecoder(), encoder = JSONEncoder()
        let existing = try files.read(.connection).map { try decoder.decode(DeviceConnection.self,from:$0) }
        var plan: OwnerSetupPlan
        if let saved = try files.read(.pending) {
            plan = try decoder.decode(OwnerSetupPlan.self,from:saved)
            guard plan.connection.endpoint == endpoint, existing == nil || existing == plan.connection else { throw OwnerSetupPreparationError.wrongHost }
        } else {
            if let existing {
                guard existing.endpoint == endpoint else { throw OwnerSetupPreparationError.wrongHost }
                _ = try storage.token(for:existing)
                return existing
            }
            guard case .available(let offer) = try await control.beginOwnerSetup(timeout:30) else { throw OwnerSetupPreparationError.existingPairing }
            try Task.checkCancellation()
            guard try files.read(.connection) == nil, try files.read(.pending) == nil else { throw OwnerSetupPreparationError.changedPreparation }
            plan = try OwnerSetupPlan(connection:DeviceConnection(endpoint:endpoint,installationID:offer.installationID,deviceID:UUID().uuidString),offer:offer,phase:.preparing)
            try files.write(encoder.encode(plan),to:.pending)
        }
        switch plan.phase {
        case .preparing,.ready: break
        case .canceling,.canceled: throw OwnerSetupPreparationError.cancellationPending
        }
        let request = try storage.prepareOwnerSetupCredential(connection:plan.connection,setupID:plan.offer.setupID,mayCreate:plan.phase == .preparing,tokenGenerator:tokenGenerator)
        let ready = OwnerSetupPlan.Phase.ready(setupCredentialDigest(request.deviceToken))
        if plan.phase == .preparing { plan.phase = ready; try files.write(encoder.encode(plan),to:.pending) }
        else if plan.phase != ready { throw OwnerSetupPreparationError.changedPreparation }
        try Task.checkCancellation()
        let receipt = try await control.completeOwnerSetup(request,timeout:30)
        try Task.checkCancellation()
        guard receipt.installationID == request.installationID, receipt.deviceID == request.deviceID,
              let saved = try files.read(.pending), try decoder.decode(OwnerSetupPlan.self,from:saved) == plan,
              try storage.token(for:plan.connection) == request.deviceToken else { throw OwnerSetupPreparationError.changedPreparation }
        let current = try files.read(.connection).map { try decoder.decode(DeviceConnection.self,from:$0) }
        guard current == nil || current == plan.connection else { throw OwnerSetupPreparationError.changedPreparation }
        try files.write(encoder.encode(plan.connection),to:.connection)
        try files.removePending()
        return plan.connection
    }
}
#endif
