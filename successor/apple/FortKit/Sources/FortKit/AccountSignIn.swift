import Foundation

private struct AccountKey: CodingKey {
    let stringValue: String
    let intValue: Int? = nil
    init?(stringValue: String) { self.stringValue = stringValue }
    init?(intValue: Int) { return nil }
}
private func accountKeys(_ decoder: Decoder, _ expected: Set<String>) throws {
    let values = try decoder.container(keyedBy:AccountKey.self)
    guard Set(values.allKeys.map(\.stringValue)) == expected else { throw ClientError.invalidResponse }
}
private func accountID(_ value: String) -> Bool {
    !value.isEmpty && value.utf8.count <= 128 && value.utf8.allSatisfy { (65...90).contains($0) || (97...122).contains($0) || (48...57).contains($0) || $0 == 45 || $0 == 95 }
}
// A device code to enter on the provider's page, or (without a code) a provider sign-in page that returns to this Mac.
public struct AccountChallenge: Decodable, Equatable, Sendable {
    public let verificationURL: URL
    public let userCode: String?
    private enum CodingKeys: String, CodingKey { case kind, verificationURL, userCode, authorizationURL }
    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy:CodingKeys.self), kind = try values.decode(String.self,forKey:.kind)
        if kind == "browserAuthorization" {
            try accountKeys(decoder,["kind","authorizationURL"])
            verificationURL = try Self.link(values.decode(String.self,forKey:.authorizationURL),query:true); userCode = nil; return
        }
        try accountKeys(decoder,["kind","verificationURL","userCode"])
        let code = try values.decode(String.self,forKey:.userCode)
        guard kind == "deviceCode", (4...32).contains(code.utf8.count),
              code.split(separator:"-",omittingEmptySubsequences:false).allSatisfy({ !$0.isEmpty && $0.utf8.allSatisfy { (65...90).contains($0) || (97...122).contains($0) || (48...57).contains($0) } })
        else { throw ClientError.invalidResponse }
        verificationURL = try Self.link(values.decode(String.self,forKey:.verificationURL),query:false); userCode = code
    }
    private static func link(_ raw: String, query: Bool) throws -> URL {
        guard raw.utf8.count <= 2048, !raw.unicodeScalars.contains(where:{ $0.value <= 32 || $0.value == 127 }),
              let url = URL(string:raw), let parts = URLComponents(string:raw), parts.scheme == "https", parts.host?.isEmpty == false,
              parts.user == nil, parts.password == nil, parts.port == nil, query || parts.query == nil, parts.fragment == nil,
              url.absoluteString == raw else { throw ClientError.invalidResponse }
        return url
    }
}
public struct AccountIdentity: Codable, Equatable, Sendable {
    public let accountId: String, userLabel: String
    private enum CodingKeys: String, CodingKey { case accountId, userLabel }
    public init(from decoder: Decoder) throws {
        try accountKeys(decoder,["accountId","userLabel"])
        let values = try decoder.container(keyedBy:CodingKeys.self)
        accountId = try values.decode(String.self,forKey:.accountId); userLabel = try values.decode(String.self,forKey:.userLabel)
        guard !accountId.isEmpty, accountId.utf8.count <= 256,
              accountId.utf8.allSatisfy({ (65...90).contains($0) || (97...122).contains($0) || (48...57).contains($0) || [45,95,46,58].contains($0) }),
              !userLabel.isEmpty, userLabel.utf8.count <= 320, userLabel.trimmingCharacters(in:.whitespacesAndNewlines) == userLabel,
              !userLabel.unicodeScalars.contains(where:{ $0.properties.generalCategory == .control || $0.properties.generalCategory == .format })
        else { throw ClientError.invalidResponse }
    }
}
public struct AccountOperation: Equatable, Sendable {
    public let revision: Int, operationID: String, expiresAt: Int64
}
public enum AccountSignInResult: String, Decodable, Sendable { case signedIn, canceled, failed, unconfirmed }
public enum AccountSignInStatus: Decodable, Equatable, Sendable {
    case idle(Int), unavailable(Int), starting(AccountOperation), challenge(AccountOperation,AccountChallenge), canceling(AccountOperation), finished(AccountOperation,AccountSignInResult), identified(AccountOperation,AccountIdentity,Bool)
    public var revision: Int { switch self { case .idle(let value), .unavailable(let value): value; case .starting(let value), .challenge(let value,_), .canceling(let value), .finished(let value,_), .identified(let value,_,_): value.revision } }
    public var operation: AccountOperation? { switch self { case .starting(let value), .challenge(let value,_), .canceling(let value), .finished(let value,_), .identified(let value,_,_): value; default: nil } }
    public var challenge: AccountChallenge? { if case .challenge(_,let value) = self { value } else { nil } }
    public var finishedResult: AccountSignInResult? { switch self { case .finished(_,let value): value; case .identified: .signedIn; default: nil } }
    public var reportedAccount: AccountIdentity? { if case .identified(_,let value,_) = self { value } else { nil } }
    public var accountSelected: Bool { if case .identified(_,_,let selected) = self { selected } else { false } }
    public var isActive: Bool { operation != nil && finishedResult == nil }
    private enum CodingKeys: String, CodingKey { case state, revision, operationId, expiresAt, challenge, result, selected }
    private struct Result: Decodable {
        let kind: AccountSignInResult, account: AccountIdentity?
        private enum CodingKeys: String, CodingKey { case kind, account }
        init(from decoder: Decoder) throws {
            let values = try decoder.container(keyedBy:CodingKeys.self)
            kind = try values.decode(AccountSignInResult.self,forKey:.kind)
            if values.contains(.account) {
                guard kind == .signedIn else { throw ClientError.invalidResponse }
                try accountKeys(decoder,["kind","account"]); account = try values.decode(AccountIdentity.self,forKey:.account)
            } else { try accountKeys(decoder,["kind"]); account = nil }
        }
    }
    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy:CodingKeys.self), state = try values.decode(String.self,forKey:.state), revision = try values.decode(Int.self,forKey:.revision)
        guard revision >= 0, revision <= 9_007_199_254_740_991 else { throw ClientError.invalidResponse }
        if state == "idle" || state == "unavailable" { try accountKeys(decoder,["state","revision"]); self = state == "idle" ? .idle(revision) : .unavailable(revision); return }
        let id = try values.decode(String.self,forKey:.operationId), expiry = try values.decode(Int64.self,forKey:.expiresAt)
        guard revision > 0, accountID(id), expiry > 0, expiry <= 9_007_199_254_740_991 else { throw ClientError.invalidResponse }
        let operation = AccountOperation(revision:revision,operationID:id,expiresAt:expiry), keys: Set<String> = ["state","revision","operationId","expiresAt"]
        switch state {
        case "starting": try accountKeys(decoder,keys); self = .starting(operation)
        case "canceling": try accountKeys(decoder,keys); self = .canceling(operation)
        case "challenge": try accountKeys(decoder,keys.union(["challenge"])); self = .challenge(operation,try values.decode(AccountChallenge.self,forKey:.challenge))
        case "finished":
            let result = try values.decode(Result.self,forKey:.result)
            if let account = result.account {
                try accountKeys(decoder,keys.union(["result","selected"]))
                self = .identified(operation,account,try values.decode(Bool.self,forKey:.selected))
            } else { try accountKeys(decoder,keys.union(["result"])); self = .finished(operation,result.kind) }
        default: throw ClientError.invalidResponse
        }
    }
}
public struct AccountSignInRequest: Codable, Equatable, Sendable {
    public let operationId: String, expectedRevision: Int, recoveryEpoch: String, expiresAt: Int64
    private enum CodingKeys: String, CodingKey { case operationId, expectedRevision, recoveryEpoch, expiresAt }
    init(operationId: String, expectedRevision: Int, recoveryEpoch: String, expiresAt: Int64) {
        self.operationId = operationId; self.expectedRevision = expectedRevision; self.recoveryEpoch = recoveryEpoch; self.expiresAt = expiresAt
    }
    public init(from decoder: Decoder) throws {
        try accountKeys(decoder,["operationId","expectedRevision","recoveryEpoch","expiresAt"])
        let values = try decoder.container(keyedBy:CodingKeys.self)
        operationId = try values.decode(String.self,forKey:.operationId); expectedRevision = try values.decode(Int.self,forKey:.expectedRevision)
        recoveryEpoch = try values.decode(String.self,forKey:.recoveryEpoch); expiresAt = try values.decode(Int64.self,forKey:.expiresAt)
        try validate()
    }
    func validate() throws {
        guard accountID(operationId), accountID(recoveryEpoch), expectedRevision >= 0, expectedRevision < 9_007_199_254_740_991,
              expiresAt > 0, expiresAt <= 9_007_199_254_740_991 else { throw ClientError.invalidResponse }
    }
    func matches(_ operation: AccountOperation) -> Bool { operation.operationID == operationId && operation.revision == expectedRevision + 1 && operation.expiresAt == expiresAt }
}
struct AccountSignInIntent: Codable, Equatable {
    let format: Int, connection: DeviceConnection, request: AccountSignInRequest
    private enum CodingKeys: String, CodingKey { case format, connection, request }
    init(format: Int, connection: DeviceConnection, request: AccountSignInRequest) { self.format = format; self.connection = connection; self.request = request }
    init(from decoder: Decoder) throws {
        try accountKeys(decoder,["format","connection","request"])
        let values = try decoder.container(keyedBy:CodingKeys.self)
        format = try values.decode(Int.self,forKey:.format); connection = try values.decode(DeviceConnection.self,forKey:.connection); request = try values.decode(AccountSignInRequest.self,forKey:.request)
        guard format == 1 else { throw ClientError.invalidResponse }; try connection.validate()
    }
}
extension DeviceStorage {
    private func accountFiles(_ connection: DeviceConnection) throws -> OwnerSetupFiles {
        try connection.validate()
        let files = try OwnerSetupFiles(directory:directory)
        do {
            guard let current = try files.read(.connection), try JSONDecoder().decode(DeviceConnection.self,from:current) == connection,
                  try files.read(.pending) == nil, try files.read(.companionSetup) == nil, try files.read(.hostRemoval) == nil, try files.read(.historyRemoval) == nil else { throw ClientError.invalidResponse }
            let prior = try files.read(.removal).map { try JSONDecoder().decode(PairingRemovalIntent.self,from:$0) }
            let disconnected = try files.read(.disconnected).map { try JSONDecoder().decode(DeviceConnection.self,from:$0) }
            if let prior {
                guard prior.state == .confirmed, prior.connection != connection, disconnected == prior.connection else { throw ClientError.invalidResponse }
            } else if disconnected != nil { throw ClientError.invalidResponse }
            return files
        } catch { files.close(); throw error }
    }
    private func accountIntent(_ files: OwnerSetupFiles, connection: DeviceConnection) throws -> AccountSignInIntent? {
        guard let data = try files.read(.accountSignIn) else { return nil }
        let intent = try JSONDecoder().decode(AccountSignInIntent.self,from:data)
        guard intent.format == 1, intent.connection == connection else { throw ClientError.invalidResponse }
        try intent.request.validate(); return intent
    }
    func accountSignInIntent(for connection: DeviceConnection) throws -> AccountSignInIntent? {
        let files = try accountFiles(connection); defer { files.close() }; return try accountIntent(files,connection:connection)
    }
    func prepareAccountSignIn(_ intent: AccountSignInIntent) throws {
        try intent.request.validate()
        let files = try accountFiles(intent.connection); defer { files.close() }
        guard intent.format == 1, try accountIntent(files,connection:intent.connection) == nil else { throw ClientError.invalidResponse }
        try files.write(JSONEncoder().encode(intent),to:.accountSignIn)
    }
    func retireAccountSignInAfterConfirmedDisconnect(_ connection: DeviceConnection) throws {
        let files = try OwnerSetupFiles(directory:directory); defer { files.close() }
        guard let data = try files.read(.accountSignIn) else { return }
        let intent = try JSONDecoder().decode(AccountSignInIntent.self,from:data)
        guard intent.connection == connection else { throw ClientError.invalidResponse }
        try files.removeAccountSignIn()
    }
    func clearAccountSignIn(_ intent: AccountSignInIntent) throws {
        let files = try accountFiles(intent.connection); defer { files.close() }
        guard try accountIntent(files,connection:intent.connection) == intent else { throw ClientError.invalidResponse }
        try files.removeAccountSignIn()
    }
}
