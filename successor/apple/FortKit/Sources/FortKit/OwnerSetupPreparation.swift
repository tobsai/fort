#if os(macOS)
import Foundation
import CryptoKit

public enum OwnerSetupPreparationError: Error, LocalizedError {
    case existingPairing, wrongHost, changedPreparation, missingCredential, busy, randomUnavailable
    public var errorDescription: String? {
        switch self {
        case .existingPairing: "This host already has a paired device. Its pairing has been kept."
        case .wrongHost: "The saved setup belongs to a different host. Its connection and credential have been kept."
        case .changedPreparation: "The saved setup changed or could not be confirmed. Its records have been kept."
        case .missingCredential: "The prepared credential is missing. Setup has been kept for recovery; no replacement was generated."
        case .busy: "Owner setup is already in progress."
        case .randomUnavailable: "A secure device credential could not be prepared."
        }
    }
}

private struct OwnerSetupPlan: Codable, Equatable {
    enum Phase: Equatable { case preparing, ready(String) }
    let connection: DeviceConnection
    let offer: OwnerSetupOffer
    var phase: Phase
    enum Keys: String, CodingKey { case version, connection, offer, phase, credentialSHA256 }
    init(connection: DeviceConnection, offer: OwnerSetupOffer, phase: Phase) throws {
        guard connection.installationID == offer.installationID, case .macHost = connection.endpoint else { throw OwnerSetupPreparationError.changedPreparation }
        _ = try OwnerSetupReceipt(installationID:connection.installationID,deviceID:connection.deviceID,expiresAt:nil)
        if case .ready(let digest) = phase {
            guard digest.utf8.count == 64, digest.rangeOfCharacter(from:CharacterSet(charactersIn:"0123456789abcdef").inverted) == nil else { throw OwnerSetupPreparationError.changedPreparation }
        }
        self.connection = connection; self.offer = offer; self.phase = phase
    }
    init(from decoder: any Decoder) throws {
        let value = try decoder.container(keyedBy:Keys.self)
        guard try value.decode(Int.self,forKey:.version) == 1 else { throw OwnerSetupPreparationError.changedPreparation }
        let phase: Phase
        switch try value.decode(String.self,forKey:.phase) {
        case "preparing":
            guard !value.contains(.credentialSHA256) else { throw OwnerSetupPreparationError.changedPreparation }
            phase = .preparing
        case "ready": phase = .ready(try value.decode(String.self,forKey:.credentialSHA256))
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
    init(storage: DeviceStorage, endpoint: DeviceEndpoint, control: any OwnerSetupControl, tokenGenerator: @escaping () throws -> String) throws {
        guard case .macHost = endpoint else { throw OwnerSetupPreparationError.wrongHost }
        try endpoint.validate()
        self.storage = storage; self.endpoint = endpoint; self.control = control; self.tokenGenerator = tokenGenerator
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
        let request = try storage.prepareOwnerSetupCredential(connection:plan.connection,setupID:plan.offer.setupID,mayCreate:plan.phase == .preparing,tokenGenerator:tokenGenerator)
        let ready = OwnerSetupPlan.Phase.ready(SHA256.hash(data:Data(request.deviceToken.utf8)).map { String(format:"%02x",$0) }.joined())
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
