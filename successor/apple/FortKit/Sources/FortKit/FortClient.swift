import Foundation

private struct ErrorEnvelope: Decodable { struct Detail: Decodable { let code: String; let message: String }; let error: Detail }

public struct FortClient: Sendable {
    public let installationID: String
    private let token: String?
    private let transport: any FortTransport
    public init(baseURL: URL, installationID: String, token: String? = nil, fixtureLoopback: Bool = false, session: URLSession = .shared) throws {
        self.init(installationID:installationID,token:token,transport:try HTTPFortTransport(baseURL:baseURL,fixtureLoopback:fixtureLoopback,session:session))
    }
    public init(installationID: String, token: String? = nil, transport: any FortTransport) {
        self.installationID = installationID; self.token = token; self.transport = transport
    }
    public func enroll(code: String, deviceName: String) async throws -> Enrollment {
        struct Body: Encodable { let installationId: String; let code: String; let deviceName: String }
        let response: Enrollment = try await send("v1/devices/enroll", method: .post, body: Body(installationId: installationID, code: code, deviceName: deviceName))
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
    public func accountSignInStatus() async throws -> AccountSignInStatus { try await get("v1/account/sign-in") }
    public func startAccountSignIn(_ intent: AccountSignInRequest) async throws -> AccountSignInStatus {
        try intent.validate(); return try await send("v1/account/sign-in/start",method:.post,body:intent)
    }
    public func cancelAccountSignIn(_ operation: AccountOperation, recoveryEpoch: String) async throws -> AccountSignInStatus {
        struct Body: Encodable { let operationId: String; let revision: Int; let recoveryEpoch: String }
        return try await send("v1/account/sign-in/cancel",method:.post,body:Body(operationId:operation.operationID,revision:operation.revision,recoveryEpoch:recoveryEpoch))
    }
    public func pairing() async throws -> PairingStatus { try await get("v1/pairing") }
    public func setPairingExpiry(days: Int?) async throws -> PairingStatus {
        let body: [String:Any] = ["expiresAfterDays":days.map { $0 as Any } ?? NSNull()]
        return try await request(path:"v1/pairing/expiry",method:.post,data:JSONSerialization.data(withJSONObject:body))
    }
    public func stopHost(disconnect: Bool) async throws -> HostStopReceipt {
        let receipt: HostStopReceipt = try await send(disconnect ? "v1/pairing/disconnect" : "v1/host/stop",method:.post,body:[String:String]())
        guard receipt.runtimeStopped, receipt.revoked == disconnect else { throw OwnerLifecycleError.stopUnconfirmed }
        return receipt
    }
    public func leaveCompanion(deviceID: String) async throws -> CompanionRevocation {
        let receipt: CompanionRevocation = try await send("v1/pairing/leave",method:.post,body:["deviceId":deviceID])
        guard receipt.installationId == installationID, receipt.deviceId == deviceID,
              receipt.revoked, receipt.scope == "companion-only" else { throw OwnerLifecycleError.wrongPairing }
        return receipt
    }
    public func models() async throws -> ModelCatalog { try await get("v1/models") }
    public func saveAgent(_ agent: Agent?, name: String, model: String, instructions: String, avatar: AgentAvatar? = nil) async throws -> Agent {
        struct Body: Encodable { let name: String; let model: String; let instructions: String; let expectedBindingRevisionId: String?; let avatar: AgentAvatar? }
        return try await send(agent.map { "v1/agents/\($0.agentId)" } ?? "v1/agents", method:.post, body:Body(name:name, model:model, instructions:instructions, expectedBindingRevisionId:agent?.binding.bindingRevisionId,avatar:avatar))
    }
    public func saveSkill(_ skill: InstructionSkill?, name: String, instructions: String) async throws -> InstructionSkill {
        struct Body: Encodable { let name: String; let instructions: String; let expectedRevisionId: String? }
        return try await send(skill.map { "v1/skills/\($0.skillId)" } ?? "v1/skills", method:.post, body:Body(name:name, instructions:instructions, expectedRevisionId:skill?.revisionId))
    }
    public func importSkillBundle(_ contents: Data, replacing skill: InstructionSkill? = nil) async throws -> InstructionSkill {
        guard contents.count <= 1_000_000 else { throw ClientError.service("invalidSkillBundle","Select a Fort bundle JSON file within 1 MB.") }
        let bundle = try JSONSerialization.jsonObject(with:contents)
        guard bundle is [String:Any] else { throw ClientError.service("invalidSkillBundle","Select a Fort bundle JSON object.") }
        var body: [String:Any] = ["bundle":bundle]
        if let skill { body["skillId"] = skill.id; body["expectedRevisionId"] = skill.revisionId }
        return try await request(path:"v1/skill-bundles",method:.post,data:JSONSerialization.data(withJSONObject:body))
    }
    public func reviewSkill(_ skill: InstructionSkill, enable: Bool) async throws -> InstructionSkill {
        guard let bundle = skill.bundle else { throw ClientError.service("invalidSkillBundle","Select a complete bundle to review.") }
        struct Body: Encodable { let revisionId: String; let digest: String; let enable: Bool }
        return try await send("v1/skills/\(try segment(skill.id))/review",method:.post,body:Body(revisionId:skill.revisionId,digest:bundle.digest,enable:enable))
    }
    public func skillRevision(skillID: String, revisionID: String) async throws -> InstructionSkill {
        try await get("v1/skills/\(try segment(skillID))/revisions/\(try segment(revisionID))")
    }
    public func createConversation(name: String, participantAgentIds: [String]) async throws -> Conversation {
        struct Body: Encodable { let name: String; let participantAgentIds: [String] }
        return try await send("v1/conversations", method:.post, body:Body(name:name, participantAgentIds:participantAgentIds))
    }
    public func snapshot(conversationID: String, before: Int? = nil) async throws -> Snapshot {
        let path = "v1/conversations/\(try segment(conversationID))/snapshot"
        guard let before else { return try await get(path) }
        guard before > 0 else { throw ClientError.invalidResponse }
        return try await request(path:path,query:[URLQueryItem(name:"before",value:String(before))],method:.get,data:nil)
    }
    public func receipt(requestID: String) async throws -> Acceptance { try await get("v1/requests/\(try segment(requestID))") }
    public func schedules() async throws -> [ScheduleRevision] {
        struct Response: Decodable { let schedules: [ScheduleRevision] }
        let response: Response = try await get("v1/schedules"); return response.schedules
    }
    public func saveSchedule(_ revision: ScheduleRevision?, definition: ScheduleDefinition) async throws -> ScheduleRevision {
        struct Body: Encodable { let expectedRevisionId: String?; let definition: ScheduleDefinition }
        let path = try revision.map { "v1/schedules/\(try segment($0.id))" } ?? "v1/schedules"
        return try await send(path,method:.post,body:Body(expectedRevisionId:revision?.revisionId,definition:definition))
    }
    public func previewSchedule(_ revision: ScheduleRevision) async throws -> SchedulePreview { try await get("v1/schedules/\(try segment(revision.id))/preview") }
    public func scheduleOccurrences(_ revision: ScheduleRevision) async throws -> [ScheduleOccurrence] {
        struct Response: Decodable { let occurrences: [ScheduleOccurrence] }
        let response: Response = try await get("v1/schedules/\(try segment(revision.id))/occurrences"); return response.occurrences
    }
    public func setSchedule(_ revision: ScheduleRevision, enabled: Bool, preview: SchedulePreview?, recoveryEpoch: String) async throws -> ScheduleRevision {
        struct Body: Encodable { let recoveryEpoch: String; let revisionId: String; let enable: Bool; let previewDigest: String? }
        return try await send("v1/schedules/\(try segment(revision.id))/state",method:.post,body:Body(recoveryEpoch:recoveryEpoch,revisionId:revision.revisionId,enable:enabled,previewDigest:preview?.digest))
    }
    public func routines() async throws -> [RoutineRevision] {
        struct Response: Decodable { let routines: [RoutineRevision] }
        let response: Response = try await get("v1/routines"); return response.routines
    }
    public func saveRoutine(_ revision: RoutineRevision?, definition: RoutineDefinition) async throws -> RoutineRevision {
        struct Body: Encodable { let expectedRevisionId: String?; let definition: RoutineDefinition }
        let path = try revision.map { "v1/routines/\(try segment($0.id))" } ?? "v1/routines"
        return try await send(path,method:.post,body:Body(expectedRevisionId:revision?.revisionId,definition:definition))
    }
    public func routineRevision(routineID: String, revisionID: String) async throws -> RoutineRevision {
        try await get("v1/routines/\(try segment(routineID))/revisions/\(try segment(revisionID))")
    }
    public func runtimeReadiness() async throws -> RuntimeReadiness { try await get("v1/runtime") }
    public func schedulerStatus() async throws -> SchedulerStatus { try await get("v1/scheduler") }
    public func restartRuntime(recoveryEpoch: String) async throws -> RuntimeReadiness {
        struct Body: Encodable { let recoveryEpoch: String }
        return try await send("v1/runtime/restart",method:.post,body:Body(recoveryEpoch:recoveryEpoch))
    }
    public func saveMemory(conversationID: String, memory: ConversationMemory?, text: String, recoveryEpoch: String) async throws -> ConversationMemory {
        let body: [String:Any] = ["expectedRevisionId":memory.map { $0.revisionId as Any } ?? NSNull(),"recoveryEpoch":recoveryEpoch,"text":text]
        return try await request(path:"v1/conversations/\(try segment(conversationID))/memory",method:.post,data:JSONSerialization.data(withJSONObject:body))
    }
    public func submit(_ pending: PendingSubmission) async throws -> Acceptance {
        if let routine = pending.routine {
            struct RoutineBody: Encodable { let requestId: String; let routineRevisionId: String; let recoveryEpoch: String; let inputs: [String:RoutineValue] }
            return try await send("v1/routines/\(try segment(routine.routineId))/run",method:.post,body:RoutineBody(requestId:pending.requestID,routineRevisionId:routine.revisionId,recoveryEpoch:pending.recoveryEpoch,inputs:routine.inputs))
        }
        struct Body: Encodable { let requestId: String; let bindingRevisionId: String; let recoveryEpoch: String; let text: String; let recipientAgentId: String?; let skills: [SkillReference]?; let memoryRevisionId: String? }
        return try await send("v1/conversations/\(try segment(pending.conversationID))/messages", method: .post, body: Body(requestId: pending.requestID, bindingRevisionId: pending.bindingRevision, recoveryEpoch: pending.recoveryEpoch, text: pending.text, recipientAgentId:pending.recipientAgentID, skills:pending.skills,memoryRevisionId:pending.memoryRevisionId))
    }
    public func events(after cursor: Int, recoveryEpoch: String, waitMilliseconds: Int = 0) async throws -> EventPage {
        guard cursor >= 0, (0...25_000).contains(waitMilliseconds) else { throw ClientError.invalidResponse }
        let query = [URLQueryItem(name: "after", value: String(cursor)), URLQueryItem(name: "recoveryEpoch", value: recoveryEpoch), URLQueryItem(name:"wait",value:String(waitMilliseconds))]
        let page: EventPage = try await request(path:"v1/events",query:query,method:.get,data:nil,timeout:max(10,Double(waitMilliseconds)/1000 + 5))
        guard page.installationId == installationID, page.recoveryEpoch == recoveryEpoch,
              page.events.count <= 256, page.cursor == (page.events.last?.sequence ?? cursor),
              Set(page.events.map(\.eventId)).count == page.events.count,
              waitMilliseconds == 0 || page.semantics == "invalidation" else { throw ClientError.invalidResponse }
        var previous = cursor
        for event in page.events {
            guard event.schemaVersion == 1, event.installationId == installationID, event.recoveryEpoch == recoveryEpoch,
                  !event.eventId.isEmpty, event.sequence > previous else { throw ClientError.invalidResponse }
            previous = event.sequence
        }
        return page
    }
    public func acknowledgeRead(conversationID: String, messageID: String, cursor: Int, recoveryEpoch: String) async throws {
        struct Body: Encodable { let recoveryEpoch: String; let messageId: String; let cursor: Int }
        struct Response: Decodable { let lastReadMessageSequence: Int }
        let _: Response = try await send("v1/conversations/\(try segment(conversationID))/read", method: .post, body: Body(recoveryEpoch: recoveryEpoch, messageId: messageID, cursor: cursor))
    }
    public func cancelPending(_ pending: PendingSubmission) async throws -> RequestCancellation {
        struct Body: Encodable { let recoveryEpoch: String; let conversationId: String; let bindingRevisionId: String }
        return try await send("v1/requests/\(try segment(pending.requestID))/cancel",method:.post,body:Body(recoveryEpoch:pending.recoveryEpoch,conversationId:pending.conversationID,bindingRevisionId:pending.bindingRevision))
    }
    public func cancel(runID: String, recoveryEpoch: String) async throws -> Run {
        struct Body: Encodable { let recoveryEpoch: String }
        let response: RunResponse = try await send("v1/runs/\(try segment(runID))/cancel",method:.post,body:Body(recoveryEpoch:recoveryEpoch))
        return response.run
    }
    public func closeUnknown(runID: String, recoveryEpoch: String) async throws -> Run {
        struct Body: Encodable { let recoveryEpoch: String; let acknowledgeUncertainty: Bool }
        let response: RunResponse = try await send("v1/runs/\(try segment(runID))/close-unknown",method:.post,body:Body(recoveryEpoch:recoveryEpoch,acknowledgeUncertainty:true))
        return response.run
    }
    public func resolve(_ approval: Approval, approve: Bool) async throws -> ApprovalResolution {
        struct Body: Encodable {
            let recoveryEpoch: String; let runId: String; let attemptId: String; let bindingRevisionId: String
            let actionDigest: String; let resourceScope: [String]; let decision: String
        }
        return try await send("v1/approvals/\(try segment(approval.approvalId))/resolve",method:.post,body:Body(recoveryEpoch:approval.recoveryEpoch,runId:approval.runId,attemptId:approval.attemptId,bindingRevisionId:approval.bindingRevisionId,actionDigest:approval.actionDigest,resourceScope:approval.resourceScope,decision:approve ? "approve" : "reject"))
    }
    private func segment(_ value: String) throws -> String {
        guard !value.isEmpty, value.unicodeScalars.allSatisfy({ CharacterSet.alphanumerics.union(CharacterSet(charactersIn: "-_" )).contains($0) }) else { throw ClientError.invalidResponse }
        return value
    }
    private func get<T: Decodable>(_ path: String) async throws -> T { try await request(path:path,method:.get,data:nil) }
    private func send<T: Decodable, B: Encodable>(_ path: String, method: FortRequest.Method, body: B) async throws -> T { try await request(path:path,method:method,data:JSONEncoder().encode(body)) }
    private func request<T: Decodable>(path: String, query: [URLQueryItem]? = nil, method: FortRequest.Method, data: Data?, timeout: TimeInterval = 10) async throws -> T {
        var components = URLComponents(); components.path = "/" + path; components.queryItems = query
        guard let encodedPath = components.string else { throw ClientError.invalidResponse }
        try Task.checkCancellation()
        let response = try await transport.send(FortRequest(path:encodedPath,method:method,installationID:installationID,token:token,body:data,timeout:timeout))
        if !(200..<300).contains(response.status) {
            let error = try? JSONDecoder().decode(ErrorEnvelope.self, from: response.body)
            throw ClientError.service(error?.error.code ?? "unavailable", error?.error.message ?? "Fort is unavailable. Your pending message is saved on this device.")
        }
        return try JSONDecoder().decode(T.self, from: response.body)
    }
}
