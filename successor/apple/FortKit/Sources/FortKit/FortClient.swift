import Foundation

public enum ClientError: Error, LocalizedError {
    case insecureEndpoint, invalidResponse, wrongInstallation, service(String, String)
    public var errorDescription: String? {
        switch self {
        case .insecureEndpoint: "Use an HTTPS Fort address. Fixture HTTP is limited to this computer's loopback address."
        case .invalidResponse: "Fort returned an unreadable response."
        case .wrongInstallation: "This address belongs to a different Fort installation."
        case .service(_, let message): message
        }
    }
}
private final class RefuseRedirects: NSObject, URLSessionTaskDelegate {
    static let shared = RefuseRedirects()
    func urlSession(_ session: URLSession, task: URLSessionTask, willPerformHTTPRedirection response: HTTPURLResponse, newRequest request: URLRequest, completionHandler: @escaping @Sendable (URLRequest?) -> Void) {
        completionHandler(nil)
    }
}
private struct ErrorEnvelope: Decodable { struct Detail: Decodable { let code: String; let message: String }; let error: Detail }

public struct FortClient: Sendable {
    public let baseURL: URL
    public let installationID: String
    private let token: String?
    private let session: URLSession
    public init(baseURL: URL, installationID: String, token: String? = nil, fixtureLoopback: Bool = false, session: URLSession = .shared) throws {
        guard baseURL.user == nil, baseURL.password == nil, baseURL.query == nil, baseURL.fragment == nil,
              baseURL.scheme == "https" || (fixtureLoopback && baseURL.scheme == "http" && ["127.0.0.1", "localhost", "::1"].contains(baseURL.host ?? "")) else { throw ClientError.insecureEndpoint }
        self.baseURL = baseURL; self.installationID = installationID; self.token = token; self.session = session
    }
    public func enroll(code: String, deviceName: String) async throws -> Enrollment {
        struct Body: Encodable { let installationId: String; let code: String; let deviceName: String }
        let response: Enrollment = try await send("v1/devices/enroll", method: "POST", body: Body(installationId: installationID, code: code, deviceName: deviceName))
        guard response.installation.installationId == installationID else { throw ClientError.wrongInstallation }
        return response
    }
    public func installation() async throws -> Installation {
        let result: Installation = try await get("v1/installation")
        guard result.installationId == installationID else { throw ClientError.wrongInstallation }
        return result
    }
    public func agents() async throws -> [Agent] { let result: AgentList = try await get("v1/agents"); return result.agents }
    public func snapshot(conversationID: String) async throws -> Snapshot { try await get("v1/conversations/\(try segment(conversationID))/snapshot") }
    public func receipt(requestID: String) async throws -> Acceptance { try await get("v1/requests/\(try segment(requestID))") }
    public func submit(_ pending: PendingSubmission) async throws -> Acceptance {
        struct Body: Encodable { let requestId: String; let bindingRevisionId: String; let recoveryEpoch: String; let text: String }
        return try await send("v1/conversations/\(try segment(pending.conversationID))/messages", method: "POST", body: Body(requestId: pending.requestID, bindingRevisionId: pending.bindingRevision, recoveryEpoch: pending.recoveryEpoch, text: pending.text))
    }
    public func events(after cursor: Int, recoveryEpoch: String) async throws -> EventPage {
        var components = URLComponents(url: baseURL.appendingPathComponent("v1/events"), resolvingAgainstBaseURL: false)!
        components.queryItems = [URLQueryItem(name: "after", value: String(cursor)), URLQueryItem(name: "recoveryEpoch", value: recoveryEpoch)]
        return try await request(url: components.url!, method: "GET", data: nil)
    }
    public func acknowledgeRead(conversationID: String, messageID: String, cursor: Int, recoveryEpoch: String) async throws {
        struct Body: Encodable { let recoveryEpoch: String; let messageId: String; let cursor: Int }
        struct Response: Decodable { let lastReadMessageSequence: Int }
        let _: Response = try await send("v1/conversations/\(try segment(conversationID))/read", method: "POST", body: Body(recoveryEpoch: recoveryEpoch, messageId: messageID, cursor: cursor))
    }
    public func cancel(runID: String, recoveryEpoch: String) async throws -> Run {
        struct Body: Encodable { let recoveryEpoch: String }
        let response: RunResponse = try await send("v1/runs/\(try segment(runID))/cancel",method:"POST",body:Body(recoveryEpoch:recoveryEpoch))
        return response.run
    }
    public func closeUnknown(runID: String, recoveryEpoch: String) async throws -> Run {
        struct Body: Encodable { let recoveryEpoch: String; let acknowledgeUncertainty: Bool }
        let response: RunResponse = try await send("v1/runs/\(try segment(runID))/close-unknown",method:"POST",body:Body(recoveryEpoch:recoveryEpoch,acknowledgeUncertainty:true))
        return response.run
    }
    public func resolve(_ approval: Approval, approve: Bool) async throws -> ApprovalResolution {
        struct Body: Encodable {
            let recoveryEpoch: String; let runId: String; let attemptId: String; let bindingRevisionId: String
            let actionDigest: String; let resourceScope: [String]; let decision: String
        }
        return try await send("v1/approvals/\(try segment(approval.approvalId))/resolve",method:"POST",body:Body(recoveryEpoch:approval.recoveryEpoch,runId:approval.runId,attemptId:approval.attemptId,bindingRevisionId:approval.bindingRevisionId,actionDigest:approval.actionDigest,resourceScope:approval.resourceScope,decision:approve ? "approve" : "reject"))
    }
    private func segment(_ value: String) throws -> String {
        guard !value.isEmpty, value.unicodeScalars.allSatisfy({ CharacterSet.alphanumerics.union(CharacterSet(charactersIn: "-_" )).contains($0) }) else { throw ClientError.invalidResponse }
        return value
    }
    private func get<T: Decodable>(_ path: String) async throws -> T { try await request(url: baseURL.appendingPathComponent(path), method: "GET", data: nil) }
    private func send<T: Decodable, B: Encodable>(_ path: String, method: String, body: B) async throws -> T { try await request(url: baseURL.appendingPathComponent(path), method: method, data: JSONEncoder().encode(body)) }
    private func request<T: Decodable>(url: URL, method: String, data: Data?) async throws -> T {
        var request = URLRequest(url: url); request.httpMethod = method; request.httpBody = data; request.timeoutInterval = 10
        request.setValue(installationID, forHTTPHeaderField: "X-Fort-Installation-Id")
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        if let token { request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization") }
        let (data, response) = try await session.data(for: request,delegate:RefuseRedirects.shared)
        guard let response = response as? HTTPURLResponse else { throw ClientError.invalidResponse }
        if !(200..<300).contains(response.statusCode) {
            let error = try? JSONDecoder().decode(ErrorEnvelope.self, from: data)
            throw ClientError.service(error?.error.code ?? "unavailable", error?.error.message ?? "Fort is unavailable. Your pending message is saved on this device.")
        }
        return try JSONDecoder().decode(T.self, from: data)
    }
}
