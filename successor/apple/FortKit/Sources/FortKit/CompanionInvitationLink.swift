import Foundation

/// One copyable text carrying the four exact phone setup fields a Mac invitation produces.
public struct CompanionInvitationLink: Equatable, Sendable {
    private static let prefix = "fort-invite:"
    public let address: URL
    public let installationID: String
    public let invitationID: String
    public let code: String
    private struct Payload: Codable { let v: Int; let address: String; let installationId: String; let invitationId: String; let code: String }

    public init(address: URL, installationID: String, invitationID: String, code: String) throws {
        guard address.scheme == "https", let host = address.host, !host.isEmpty, address.user == nil, address.password == nil,
              address.query == nil, address.fragment == nil, validCompanionIdentifier(installationID),
              validCompanionIdentifier(invitationID), validCompanionToken(code) else { throw ClientError.invalidResponse }
        self.address = address; self.installationID = installationID; self.invitationID = invitationID; self.code = code
    }
    public init(text: String) throws {
        let value = text.trimmingCharacters(in:.whitespacesAndNewlines)
        guard value.hasPrefix(Self.prefix), value.utf8.count <= 4096 else { throw ClientError.invalidResponse }
        var encoded = String(value.dropFirst(Self.prefix.count)).replacingOccurrences(of:"-",with:"+").replacingOccurrences(of:"_",with:"/")
        encoded += String(repeating:"=",count:(4 - encoded.count % 4) % 4)
        guard let data = Data(base64Encoded:encoded), let payload = try? JSONDecoder().decode(Payload.self,from:data),
              payload.v == 1, let address = URL(string:payload.address) else { throw ClientError.invalidResponse }
        try self.init(address:address,installationID:payload.installationId,invitationID:payload.invitationId,code:payload.code)
    }
    public var text: String {
        let payload = Payload(v:1,address:address.absoluteString,installationId:installationID,invitationId:invitationID,code:code)
        let data = (try? JSONEncoder().encode(payload)) ?? Data()
        return Self.prefix + data.base64EncodedString().replacingOccurrences(of:"+",with:"-").replacingOccurrences(of:"/",with:"_").replacingOccurrences(of:"=",with:"")
    }
}
