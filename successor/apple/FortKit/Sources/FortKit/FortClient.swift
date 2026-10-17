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
    public func conversations() async throws -> [Conversation] { struct Response: Decodable { let conversations: [Conversation] }; let result: Response = try await get("v1/conversations"); return result.conversations }
    public func skills() async throws -> [InstructionSkill] { struct Response: Decodable { let skills: [InstructionSkill] }; let result: Response = try await get("v1/skills"); return result.skills }
    public func models() async throws -> ModelCatalog { try await get("v1/models") }
    public func saveAgent(_ agent: Agent?, name: String, model: String, instructions: String, avatar: AgentAvatar? = nil) async throws -> Agent {
        struct Body: Encodable { let name: String; let model: String; let instructions: String; let expectedBindingRevisionId: String?; let avatar: AgentAvatar? }
        return try await send(agent.map { "v1/agents/\($0.agentId)" } ?? "v1/agents", method:"POST", body:Body(name:name, model:model, instructions:instructions, expectedBindingRevisionId:agent?.binding.bindingRevisionId,avatar:avatar))
    }
    public func saveSkill(_ skill: InstructionSkill?, name: String, instructions: String) async throws -> InstructionSkill {
        struct Body: Encodable { let name: String; let instructions: String; let expectedRevisionId: String? }
        return try await send(skill.map { "v1/skills/\($0.skillId)" } ?? "v1/skills", method:"POST", body:Body(name:name, instructions:instructions, expectedRevisionId:skill?.revisionId))
    }
    public func importSkillBundle(_ contents: Data, replacing skill: InstructionSkill? = nil) async throws -> InstructionSkill {
        guard contents.count <= 1_000_000 else { throw ClientError.service("invalidSkillBundle","Select a Fort bundle JSON file within 1 MB.") }
        let bundle = try JSONSerialization.jsonObject(with:contents)
        guard bundle is [String:Any] else { throw ClientError.service("invalidSkillBundle","Select a Fort bundle JSON object.") }
        var body: [String:Any] = ["bundle":bundle]
        if let skill { body["skillId"] = skill.id; body["expectedRevisionId"] = skill.revisionId }
        return try await request(url:baseURL.appendingPathComponent("v1/skill-bundles"),method:"POST",data:JSONSerialization.data(withJSONObject:body))
    }
    public func reviewSkill(_ skill: InstructionSkill, enable: Bool) async throws -> InstructionSkill {
        guard let bundle = skill.bundle else { throw ClientError.service("invalidSkillBundle","Select a complete bundle to review.") }
        struct Body: Encodable { let revisionId: String; let digest: String; let enable: Bool }
        return try await send("v1/skills/\(try segment(skill.id))/review",method:"POST",body:Body(revisionId:skill.revisionId,digest:bundle.digest,enable:enable))
    }
    public func skillRevision(skillID: String, revisionID: String) async throws -> InstructionSkill {
        try await get("v1/skills/\(try segment(skillID))/revisions/\(try segment(revisionID))")
    }
    public func createConversation(name: String, participantAgentIds: [String]) async throws -> Conversation {
        struct Body: Encodable { let name: String; let participantAgentIds: [String] }
        return try await send("v1/conversations", method:"POST", body:Body(name:name, participantAgentIds:participantAgentIds))
    }
    public func snapshot(conversationID: String) async throws -> Snapshot { try await get("v1/conversations/\(try segment(conversationID))/snapshot") }
    public func receipt(requestID: String) async throws -> Acceptance { try await get("v1/requests/\(try segment(requestID))") }
    public func schedules() async throws -> [ScheduleRevision] {
        struct Response: Decodable { let schedules: [ScheduleRevision] }
        let response: Response = try await get("v1/schedules"); return response.schedules
    }
    public func saveSchedule(_ revision: ScheduleRevision?, definition: ScheduleDefinition) async throws -> ScheduleRevision {
        struct Body: Encodable { let expectedRevisionId: String?; let definition: ScheduleDefinition }
        let path = try revision.map { "v1/schedules/\(try segment($0.id))" } ?? "v1/schedules"
        return try await send(path,method:"POST",body:Body(expectedRevisionId:revision?.revisionId,definition:definition))
    }
    public func previewSchedule(_ revision: ScheduleRevision) async throws -> SchedulePreview { try await get("v1/schedules/\(try segment(revision.id))/preview") }
    public func scheduleOccurrences(_ revision: ScheduleRevision) async throws -> [ScheduleOccurrence] {
        struct Response: Decodable { let occurrences: [ScheduleOccurrence] }
        let response: Response = try await get("v1/schedules/\(try segment(revision.id))/occurrences"); return response.occurrences
    }
    public func setSchedule(_ revision: ScheduleRevision, enabled: Bool, preview: SchedulePreview?, recoveryEpoch: String) async throws -> ScheduleRevision {
        struct Body: Encodable { let recoveryEpoch: String; let revisionId: String; let enable: Bool; let previewDigest: String? }
        return try await send("v1/schedules/\(try segment(revision.id))/state",method:"POST",body:Body(recoveryEpoch:recoveryEpoch,revisionId:revision.revisionId,enable:enabled,previewDigest:preview?.digest))
    }
    public func routines() async throws -> [RoutineRevision] {
        struct Response: Decodable { let routines: [RoutineRevision] }
        let response: Response = try await get("v1/routines"); return response.routines
    }
    public func saveRoutine(_ revision: RoutineRevision?, definition: RoutineDefinition) async throws -> RoutineRevision {
        struct Body: Encodable { let expectedRevisionId: String?; let definition: RoutineDefinition }
        let path = try revision.map { "v1/routines/\(try segment($0.id))" } ?? "v1/routines"
        return try await send(path,method:"POST",body:Body(expectedRevisionId:revision?.revisionId,definition:definition))
    }
    public func routineRevision(routineID: String, revisionID: String) async throws -> RoutineRevision {
        try await get("v1/routines/\(try segment(routineID))/revisions/\(try segment(revisionID))")
    }
    public func runtimeReadiness() async throws -> RuntimeReadiness { try await get("v1/runtime") }
    public func restartRuntime(recoveryEpoch: String) async throws -> RuntimeReadiness {
        struct Body: Encodable { let recoveryEpoch: String }
        return try await send("v1/runtime/restart",method:"POST",body:Body(recoveryEpoch:recoveryEpoch))
    }
    public func saveMemory(conversationID: String, memory: ConversationMemory?, text: String, recoveryEpoch: String) async throws -> ConversationMemory {
        let body: [String:Any] = ["expectedRevisionId":memory.map { $0.revisionId as Any } ?? NSNull(),"recoveryEpoch":recoveryEpoch,"text":text]
        return try await request(url:baseURL.appendingPathComponent("v1/conversations/\(try segment(conversationID))/memory"),method:"POST",data:JSONSerialization.data(withJSONObject:body))
    }
    public func submit(_ pending: PendingSubmission) async throws -> Acceptance {
        if let routine = pending.routine {
            struct RoutineBody: Encodable { let requestId: String; let routineRevisionId: String; let recoveryEpoch: String; let inputs: [String:RoutineValue] }
            return try await send("v1/routines/\(try segment(routine.routineId))/run",method:"POST",body:RoutineBody(requestId:pending.requestID,routineRevisionId:routine.revisionId,recoveryEpoch:pending.recoveryEpoch,inputs:routine.inputs))
        }
        struct Body: Encodable { let requestId: String; let bindingRevisionId: String; let recoveryEpoch: String; let text: String; let recipientAgentId: String?; let skills: [SkillReference]?; let memoryRevisionId: String? }
        return try await send("v1/conversations/\(try segment(pending.conversationID))/messages", method: "POST", body: Body(requestId: pending.requestID, bindingRevisionId: pending.bindingRevision, recoveryEpoch: pending.recoveryEpoch, text: pending.text, recipientAgentId:pending.recipientAgentID, skills:pending.skills,memoryRevisionId:pending.memoryRevisionId))
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
    public func cancelPending(_ pending: PendingSubmission) async throws -> RequestCancellation {
        struct Body: Encodable { let recoveryEpoch: String; let conversationId: String; let bindingRevisionId: String }
        return try await send("v1/requests/\(try segment(pending.requestID))/cancel",method:"POST",body:Body(recoveryEpoch:pending.recoveryEpoch,conversationId:pending.conversationID,bindingRevisionId:pending.bindingRevision))
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
