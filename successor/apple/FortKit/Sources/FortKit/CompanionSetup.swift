import Foundation
import CryptoKit

func validCompanionToken(_ value: String) -> Bool {
    guard value.utf8.count == 43, let bytes = Data(base64Encoded:value.replacingOccurrences(of:"-",with:"+").replacingOccurrences(of:"_",with:"/") + "="), bytes.count == 32 else { return false }
    return bytes.base64EncodedString().replacingOccurrences(of:"+",with:"-").replacingOccurrences(of:"/",with:"_").replacingOccurrences(of:"=",with:"") == value
}
func validCompanionIdentifier(_ value: String) -> Bool {
    !value.isEmpty && value.utf8.count <= 128 && value.rangeOfCharacter(from:CharacterSet(charactersIn:"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-").inverted) == nil
}
private func companionDigest(_ value: String) -> String { SHA256.hash(data:Data(value.utf8)).map { String(format:"%02x",$0) }.joined() }

public struct CompanionJoinReceipt: Decodable, Sendable {
    public let installationId: String
    public let deviceId: String
    public let expiresAt: String?
    public let role: PairingStatus.Role
}
public struct PendingCompanionSetup: Equatable, Sendable {
    public let connection: DeviceConnection
    public let invitationID: String
    public let deviceName: String
    public let credentialPrepared: Bool
    public let removalState: PairingRemovalIntent.State?
}
private struct CompanionPlanKey: CodingKey {
    let stringValue: String
    let intValue: Int? = nil
    init?(stringValue: String) { self.stringValue = stringValue }
    init?(intValue: Int) { return nil }
}
private struct CompanionPlan: Codable, Equatable {
    private enum CodingKeys: String, CodingKey, CaseIterable { case version, connection, invitationID, codeSHA256, deviceName, credentialSHA256, removalState }
    let version: Int
    let connection: DeviceConnection
    let invitationID: String
    let codeSHA256: String
    let deviceName: String
    var credentialSHA256: String?
    var removalState: PairingRemovalIntent.State?
    func validate() throws {
        try connection.validate()
        let digestValid: (String) -> Bool = { $0.utf8.count == 64 && $0.rangeOfCharacter(from:CharacterSet(charactersIn:"0123456789abcdef").inverted) == nil }
        guard version == 1, connection.role == .companion,
              [connection.installationID,connection.deviceID,invitationID].allSatisfy(validCompanionIdentifier),
              digestValid(codeSHA256), credentialSHA256.map(digestValid) ?? true,
              credentialSHA256 != codeSHA256, !deviceName.trimmingCharacters(in:.whitespacesAndNewlines).isEmpty,
              deviceName.utf8.count <= 128 else { throw OwnerSetupPreparationError.changedPreparation }
    }
}

extension CompanionPlan {
    init(from decoder: Decoder) throws {
        let keys = try decoder.container(keyedBy:CompanionPlanKey.self)
        guard Set(keys.allKeys.map(\.stringValue)).isSubset(of:Set(CodingKeys.allCases.map(\.rawValue))) else { throw OwnerSetupPreparationError.changedPreparation }
        let values = try decoder.container(keyedBy:CodingKeys.self)
        version = try values.decode(Int.self,forKey:.version)
        connection = try values.decode(DeviceConnection.self,forKey:.connection)
        invitationID = try values.decode(String.self,forKey:.invitationID)
        codeSHA256 = try values.decode(String.self,forKey:.codeSHA256)
        deviceName = try values.decode(String.self,forKey:.deviceName)
        credentialSHA256 = try values.decodeIfPresent(String.self,forKey:.credentialSHA256)
        removalState = try values.decodeIfPresent(PairingRemovalIntent.State.self,forKey:.removalState)
        try validate()
    }
}

@MainActor public final class CompanionSetupCoordinator {
    private let storage: DeviceStorage
    private let tokenGenerator: () throws -> String
    private let transport: (DeviceEndpoint) throws -> any FortTransport
    public convenience init(storage: DeviceStorage) {
        self.init(storage:storage,tokenGenerator:DeviceStorage.makeOwnerDeviceToken,transport:{ try $0.transport(session:.shared) })
    }
    init(storage: DeviceStorage, tokenGenerator: @escaping () throws -> String, transport: @escaping (DeviceEndpoint) throws -> any FortTransport) {
        self.storage = storage; self.tokenGenerator = tokenGenerator; self.transport = transport
    }
    public func connect(address: URL, installationID: String, invitationID: String, code: String, deviceName: String) async throws -> DeviceConnection {
        try Task.checkCancellation()
        let endpoint = DeviceEndpoint.network(baseURL:address,fixtureLoopback:false)
        guard address.scheme == "https", validCompanionToken(code) else { throw ClientError.invalidResponse }
        try endpoint.validate()
        let files = try OwnerSetupFiles(directory:storage.directory); defer { files.close() }
        try isolation(files)
        var plan: CompanionPlan
        if let (prior,_) = try read(files) {
            guard prior.connection.endpoint == endpoint, prior.connection.installationID == installationID,
                  prior.invitationID == invitationID, prior.codeSHA256 == companionDigest(code), prior.deviceName == deviceName else { throw OwnerSetupPreparationError.changedPreparation }
            plan = prior
        } else {
            guard try files.read(.connection) == nil else { throw OwnerSetupPreparationError.existingPairing }
            plan = CompanionPlan(version:1,connection:try DeviceConnection(endpoint:endpoint,installationID:installationID,deviceID:UUID().uuidString,role:.companion),invitationID:invitationID,codeSHA256:companionDigest(code),deviceName:deviceName)
            _ = try save(plan,files)
        }
        guard plan.removalState == nil else { throw OwnerSetupPreparationError.cancellationPending }
        try matchingConnection(plan,files)
        let token = try storage.prepareCompanionCredential(connection:plan.connection,mayCreate:plan.credentialSHA256 == nil,tokenGenerator:{
            let token = try self.tokenGenerator()
            guard token != code else { throw ClientError.invalidResponse }
            return token
        })
        let digest = companionDigest(token)
        guard digest != plan.codeSHA256, plan.credentialSHA256 == nil || plan.credentialSHA256 == digest else { throw OwnerSetupPreparationError.changedPreparation }
        let bytes: Data
        if plan.credentialSHA256 == nil { plan.credentialSHA256 = digest; bytes = try save(plan,files) }
        else {
            guard let saved = try files.read(.companionSetup) else { throw OwnerSetupPreparationError.changedPreparation }
            bytes = saved
        }
        try Task.checkCancellation()
        let client = FortClient(installationID:installationID,transport:try transport(endpoint))
        _ = try await client.joinCompanion(invitationID:invitationID,code:code,deviceID:plan.connection.deviceID,deviceToken:token,deviceName:deviceName)
        try finish(plan,bytes:bytes,token:token,files:files)
        return plan.connection
    }
    public func recover() async throws -> DeviceConnection {
        try Task.checkCancellation()
        let files = try OwnerSetupFiles(directory:storage.directory); defer { files.close() }
        try isolation(files)
        guard let (plan,bytes) = try read(files), let digest = plan.credentialSHA256,
              let token = try storage.ownerSetupCredential(for:plan.connection) else { throw OwnerSetupPreparationError.missingCredential }
        guard plan.removalState == nil else { throw OwnerSetupPreparationError.cancellationPending }
        guard validCompanionToken(token), companionDigest(token) == digest else { throw OwnerSetupPreparationError.changedPreparation }
        try matchingConnection(plan,files)
        let client = FortClient(installationID:plan.connection.installationID,token:token,transport:try transport(plan.connection.endpoint))
        let installation = try await client.installation()
        try verify(plan,bytes:bytes,token:token,files:files)
        guard installation.supports("companionPairing") else { throw ClientError.invalidResponse }
        let pairing = try await client.pairing()
        guard pairing.deviceId == plan.connection.deviceID, pairing.mode == .ownerLocal, pairing.role == .companion else { throw OwnerLifecycleError.wrongPairing }
        try finish(plan,bytes:bytes,token:token,files:files)
        return plan.connection
    }
    public func pendingSetup() throws -> PendingCompanionSetup? {
        let files = try OwnerSetupFiles(directory:storage.directory); defer { files.close() }
        guard let (plan,_) = try read(files) else { return nil }
        try isolation(files); try matchingConnection(plan,files)
        return PendingCompanionSetup(connection:plan.connection,invitationID:plan.invitationID,deviceName:plan.deviceName,credentialPrepared:plan.credentialSHA256 != nil,removalState:plan.removalState)
    }
    public func disconnectPending() async throws {
        try Task.checkCancellation()
        let files = try OwnerSetupFiles(directory:storage.directory); defer { files.close() }
        try isolation(files)
        guard var (plan,bytes) = try read(files) else { return }
        guard plan.removalState != .locallyForgotten else { throw OwnerSetupPreparationError.cancellationPending }
        try matchingConnection(plan,files)
        let token = try retainedToken(plan)
        if plan.removalState != .confirmed {
            if plan.credentialSHA256 != nil {
                guard let token else { throw OwnerSetupPreparationError.missingCredential }
                plan.removalState = .requested; bytes = try save(plan,files)
                let client = FortClient(installationID:plan.connection.installationID,token:token,transport:try transport(plan.connection.endpoint))
                _ = try await client.leaveCompanion(deviceID:plan.connection.deviceID)
                try verify(plan,bytes:bytes,token:token,files:files)
            }
            if let token { plan.credentialSHA256 = companionDigest(token) }
            plan.removalState = .confirmed; bytes = try save(plan,files)
        }
        try finishRemoval(plan,bytes:bytes,token:token,files:files)
    }
    public func forgetPending() throws {
        try Task.checkCancellation()
        let files = try OwnerSetupFiles(directory:storage.directory); defer { files.close() }
        try isolation(files)
        guard var (plan,bytes) = try read(files) else { return }
        try matchingConnection(plan,files)
        let token = try retainedToken(plan)
        if let token { plan.credentialSHA256 = companionDigest(token) }
        if plan.removalState != .confirmed { plan.removalState = .locallyForgotten }
        bytes = try save(plan,files)
        try finishRemoval(plan,bytes:bytes,token:token,files:files)
    }
    private func isolation(_ files: OwnerSetupFiles) throws {
        guard try files.read(.pending) == nil, try files.read(.hostRemoval) == nil, try files.read(.historyRemoval) == nil, try files.read(.accountSignIn) == nil else { throw OwnerSetupPreparationError.busy }
        let prior = try files.read(.removal).map { try JSONDecoder().decode(PairingRemovalIntent.self,from:$0) }
        let disconnected = try files.read(.disconnected).map { try JSONDecoder().decode(DeviceConnection.self,from:$0) }
        if let prior {
            guard disconnected == prior.connection else { throw OwnerSetupPreparationError.changedPreparation }
            if let (plan,_) = try read(files), plan.connection == prior.connection {
                guard prior.scope == .companion, plan.removalState == prior.state else { throw OwnerSetupPreparationError.cancellationPending }
            } else { guard prior.state == .confirmed else { throw OwnerSetupPreparationError.cancellationPending } }
        } else if disconnected != nil { throw OwnerSetupPreparationError.changedPreparation }
    }
    private func read(_ files: OwnerSetupFiles) throws -> (CompanionPlan,Data)? {
        guard let bytes = try files.read(.companionSetup) else { return nil }
        let plan = try JSONDecoder().decode(CompanionPlan.self,from:bytes); try plan.validate()
        return (plan,bytes)
    }
    private func save(_ plan: CompanionPlan,_ files: OwnerSetupFiles) throws -> Data {
        try plan.validate(); let bytes = try JSONEncoder().encode(plan)
        try files.write(bytes,to:.companionSetup); return bytes
    }
    private func matchingConnection(_ plan: CompanionPlan,_ files: OwnerSetupFiles) throws {
        let current = try files.read(.connection).map { try JSONDecoder().decode(DeviceConnection.self,from:$0) }
        guard current == nil || current == plan.connection else { throw OwnerSetupPreparationError.changedPreparation }
    }
    private func verify(_ plan: CompanionPlan,bytes: Data,token: String?,files: OwnerSetupFiles) throws {
        try Task.checkCancellation(); try isolation(files); try matchingConnection(plan,files)
        guard try files.read(.companionSetup) == bytes, try storage.ownerSetupCredential(for:plan.connection) == token else { throw OwnerSetupPreparationError.changedPreparation }
    }
    private func finish(_ plan: CompanionPlan,bytes: Data,token: String,files: OwnerSetupFiles) throws {
        try verify(plan,bytes:bytes,token:token,files:files)
        try files.write(JSONEncoder().encode(plan.connection),to:.connection)
        try files.removeCompanionSetup()
    }
    private func retainedToken(_ plan: CompanionPlan) throws -> String? {
        guard let token = try storage.ownerSetupCredential(for:plan.connection) else { return nil }
        let digest = companionDigest(token)
        guard validCompanionToken(token), digest != plan.codeSHA256,
              plan.credentialSHA256 == nil || plan.credentialSHA256 == digest else { throw OwnerSetupPreparationError.changedPreparation }
        return token
    }
    private func finishRemoval(_ plan: CompanionPlan,bytes: Data,token: String?,files: OwnerSetupFiles) throws {
        guard let state = plan.removalState, state == .confirmed || state == .locallyForgotten else { throw ClientError.invalidResponse }
        try verify(plan,bytes:bytes,token:token,files:files)
        try files.write(JSONEncoder().encode(PairingRemovalIntent(connection:plan.connection,state:state,scope:.companion)),to:.removal)
        try files.write(JSONEncoder().encode(plan.connection),to:.disconnected)
        try storage.removeOwnerSetupCredential(plan.connection,expectedToken:token)
        try verify(plan,bytes:bytes,token:nil,files:files)
        if try files.read(.connection) != nil { try files.removeConnection() }
        try files.removeCompanionSetup()
    }
}
