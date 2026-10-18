#if DEBUG
import Foundation
import CryptoKit

public enum FixtureCatalogScenario: String, Codable, CaseIterable, Sendable, Identifiable {
    case ready, missingSelection, failure
    public var id: String { rawValue }
    public var name: String {
        switch self { case .ready: "Available + unavailable models"; case .missingSelection: "Saved model missing"; case .failure: "Discovery failure" }
    }
}
private struct NoFixtureCredentials: DeviceCredentials {
    func save(_ connection: DeviceConnection, token: String) throws { throw ClientError.invalidResponse }
    func token(for connection: DeviceConnection) throws -> String { throw ClientError.invalidResponse }
}
@MainActor public enum InteractiveFixture {
    public static func make(directory: URL) async throws -> MessengerModel {
        let root = directory.standardizedFileURL
        let temporary = FileManager.default.temporaryDirectory.resolvingSymlinksInPath().path + "/"
        guard root.resolvingSymlinksInPath().path.hasPrefix(temporary), (try? root.resourceValues(forKeys:[.isSymbolicLinkKey]).isSymbolicLink) != true else { throw ClientError.invalidResponse }
        let storage = try DeviceStorage(directory:root,credentials:NoFixtureCredentials())
        let isNew = !FileManager.default.fileExists(atPath:root.appendingPathComponent("interactive-fixture.json").path)
        let fixture = try InteractiveFixtureStore(directory:root)
        let installation = await fixture.installation
        let client = FortClient(installationID:installation.installationId,transport:fixture)
        let model = MessengerModel(storage:storage,session:.shared)
        try model.configureInteractiveFixture(client:client,fixture:fixture,installation:installation)
        model.fixtureCatalogScenario = await fixture.scenario
        await model.refresh(); await model.refreshModels()
        if isNew { await model.addFixtureImage() }
        return model
    }
    static var imageURL: URL { Bundle.module.url(forResource:"qa-sample",withExtension:"png")! }
}

actor InteractiveFixtureStore: FortTransport {
    private struct State: Codable {
        var installation: Installation
        var agents: [Agent]
        var conversations: [Conversation]
        var snapshots: [String:Snapshot]
        var receipts: [String:Acceptance] = [:]
        var scenario: FixtureCatalogScenario = .ready
    }
    private let file: URL
    private var state: State
    var installation: Installation { state.installation }
    var scenario: FixtureCatalogScenario { state.scenario }
    init(directory: URL) throws {
        file = directory.appendingPathComponent("interactive-fixture.json")
        if FileManager.default.fileExists(atPath:file.path) {
            let values = try file.resourceValues(forKeys:[.isRegularFileKey,.isSymbolicLinkKey,.fileSizeKey])
            guard values.isRegularFile == true, values.isSymbolicLink != true, (values.fileSize ?? 0) < 4_000_000 else { throw ClientError.invalidResponse }
            state = try JSONDecoder().decode(State.self,from:Data(contentsOf:file))
        } else {
            let installation = Installation(contractVersion:"1.0",installationId:"qa-" + UUID().uuidString.lowercased(),recoveryEpoch:"qa-epoch",mode:"fixture",serviceName:"Interactive QA",capabilities:["agentSettings","modelCatalog"])
            let agents = [Self.agent(id:"qa-writer",name:"QA Writer",model:"fixture-alpha",instructions:"Write clearly and keep my voice.",avatar:.orbit),Self.agent(id:"qa-reviewer",name:"QA Reviewer",model:"fixture-retired",instructions:"Point out the important gaps.",avatar:.sprout)]
            let conversations = agents.map { Conversation(conversationId:$0.homeConversationId,name:$0.name,kind:"home",participantAgentIds:[$0.id]) }
            let snapshots = Dictionary(uniqueKeysWithValues:agents.map { agent in
                (agent.homeConversationId,Snapshot(installationId:installation.installationId,recoveryEpoch:installation.recoveryEpoch,conversationId:agent.homeConversationId,cursor:1,messages:[Message(messageId:"welcome-" + agent.id,conversationId:agent.homeConversationId,requestId:"welcome",role:"assistant",text:"Interactive QA: edit this agent in Settings, choose a simulated model and send a message. The sample image is local. No provider, network or credentials are used.",sequence:1,createdAt:Self.time,agentId:agent.id,agentName:agent.name)],runs:[],lastReadMessageSequence:0))
            })
            state = State(installation:installation,agents:agents,conversations:conversations,snapshots:snapshots)
            try Self.write(state,to:file)
        }
    }
    private static let time = "2026-10-17T00:00:00Z"
    private static func agent(id: String, name: String, model: String, instructions: String, avatar: AgentAvatar, revision: String? = nil) -> Agent {
        Agent(agentId:id,name:name,homeConversationId:id + "-home",binding:Binding(bindingRevisionId:revision ?? id + "-binding",provider:"simulated",runtimeVersion:"qa",selectedModel:model,hostId:"This app · no network",enabled:true),readiness:"fixture",instructions:instructions,avatar:avatar)
    }
    private static func write(_ state: State, to file: URL) throws {
        try JSONEncoder().encode(state).write(to:file,options:.atomic)
        try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:file.path)
    }
    func setScenario(_ scenario: FixtureCatalogScenario) throws { state.scenario = scenario; try Self.write(state,to:file) }
    func reload() throws { state = try JSONDecoder().decode(State.self,from:Data(contentsOf:file)) }
    private func catalog() -> ModelCatalog {
        if state.scenario == .failure { return ModelCatalog(provider:"simulated",state:"error",models:[],detail:"Simulated discovery failure. Saved model choices stay unchanged.") }
        var options = [ModelOption(id:"fixture-alpha",name:"QA Alpha",availability:"available"),ModelOption(id:"fixture-beta",name:"QA Beta",availability:"available"),ModelOption(id:"fixture-retired",name:"QA Retired",availability:"unavailable",detail:"This simulated model is unavailable.")]
        if state.scenario == .missingSelection { let saved = Set(state.agents.map { $0.binding.selectedModel }); options.removeAll { saved.contains($0.id) } }
        return ModelCatalog(provider:"simulated",state:"ready",models:options,checkedAt:Self.time,detail:"Simulated catalog · no provider discovery")
    }
    func send(_ request: FortRequest) async throws -> FortResponse {
        guard let url = URLComponents(string:request.path), url.scheme == nil, url.host == nil, request.token == nil,
              request.installationID == state.installation.installationId else { throw ClientError.invalidResponse }
        func response<T:Encodable>(_ value: T, status: Int = 200) throws -> FortResponse {
            FortResponse(status:status,body:try JSONEncoder().encode(value))
        }
        func error(_ code: String, _ message: String) throws -> FortResponse { try response(["error":["code":code,"message":message]],status:409) }
        let parts = url.path.split(separator:"/").map(String.init)
        if request.method == .get {
            switch url.path {
            case "/v1/installation": return try response(state.installation)
            case "/v1/agents": return try response(AgentList(agents:state.agents))
            case "/v1/conversations": return try response(["conversations":state.conversations])
            case "/v1/skills": return try response(["skills":[InstructionSkill]()])
            case "/v1/models": return try response(catalog())
            default:
                if parts.count == 4, parts[1] == "conversations", parts[3] == "snapshot", let snapshot = state.snapshots[parts[2]] { return try response(snapshot) }
                return try error("fixtureRoute","This action is not part of the interactive fixture.")
            }
        }
        guard request.method == .post, let data = request.body else { throw ClientError.invalidResponse }
        if parts.count >= 2, parts[1] == "agents" {
            struct Body: Decodable { let name: String; let instructions: String; let model: String; let avatar: AgentAvatar?; let expectedBindingRevisionId: String? }
            let body = try JSONDecoder().decode(Body.self,from:data)
            let old = parts.count == 3 ? state.agents.first { $0.id == parts[2] } : nil
            if parts.count == 3 && (old == nil || old?.binding.bindingRevisionId != body.expectedBindingRevisionId) { return try error("bindingChanged","Reload the agent before saving.") }
            guard !body.name.trimmingCharacters(in:.whitespacesAndNewlines).isEmpty, body.name.utf8.count <= 128, body.instructions.utf8.count <= 16_384,
                  catalog().canSelect(body.model) || old?.binding.selectedModel == body.model else { return try error("invalidConfiguration","Choose an available simulated model.") }
            let agent = Self.agent(id:old?.id ?? UUID().uuidString.lowercased(),name:body.name,model:body.model,instructions:body.instructions,avatar:body.avatar ?? old?.avatar ?? .orbit,revision:UUID().uuidString.lowercased())
            if let index = state.agents.firstIndex(where:{$0.id == agent.id}) { state.agents[index] = agent }
            else { state.agents.append(agent); state.snapshots[agent.homeConversationId] = Snapshot(installationId:state.installation.installationId,recoveryEpoch:state.installation.recoveryEpoch,conversationId:agent.homeConversationId,cursor:0,messages:[],runs:[],lastReadMessageSequence:0) }
            state.conversations = state.agents.map { Conversation(conversationId:$0.homeConversationId,name:$0.name,kind:"home",participantAgentIds:[$0.id]) }
            try Self.write(state,to:file); return try response(agent)
        }
        if parts.count == 4, parts[1] == "conversations", let snapshot = state.snapshots[parts[2]] {
            if parts[3] == "read" { return try response(["lastReadMessageSequence":snapshot.messages.last?.sequence ?? 0]) }
            if parts[3] == "messages" {
                struct Body: Decodable { let requestId: String; let bindingRevisionId: String; let recoveryEpoch: String; let recipientAgentId: String; let text: String }
                let body = try JSONDecoder().decode(Body.self,from:data)
                if let prior = state.receipts[body.requestId] { return try response(prior) }
                guard let agent = state.agents.first(where:{$0.id == body.recipientAgentId && $0.homeConversationId == parts[2]}), agent.binding.bindingRevisionId == body.bindingRevisionId,
                      state.installation.recoveryEpoch == body.recoveryEpoch, body.text.utf8.count <= 65_536, catalog().canSelect(agent.binding.selectedModel) else { return try error("invalidConfiguration","This simulated model or configuration is unavailable. Review the agent settings.") }
                let user = Message(messageId:body.requestId + "-user",conversationId:parts[2],requestId:body.requestId,role:"user",text:body.text,sequence:snapshot.cursor + 1,createdAt:Self.time,recipientAgentId:agent.id,recipientAgentName:agent.name)
                let reply = Message(messageId:body.requestId + "-reply",conversationId:parts[2],requestId:body.requestId,role:"assistant",text:"Simulated reply · \(agent.binding.selectedModel)\n\(agent.name)'s saved identity: \(agent.instructions ?? "")\nYou wrote: \(body.text)\nNo provider was contacted.",sequence:snapshot.cursor + 2,createdAt:Self.time,agentId:agent.id,agentName:agent.name)
                let configuration = TurnConfiguration(agentId:agent.id,agentName:agent.name,bindingRevisionId:agent.binding.bindingRevisionId,selectedModel:agent.binding.selectedModel,instructions:agent.instructions ?? "",skills:[])
                let run = Run(runId:body.requestId + "-run",requestId:body.requestId,conversationId:parts[2],bindingRevisionId:agent.binding.bindingRevisionId,state:"completed",selectedModel:agent.binding.selectedModel,effectiveModel:nil,modelEvidence:"unreported",attemptId:nil,configuration:configuration,statusDetail:"Simulated locally · no provider")
                let updated = Snapshot(installationId:snapshot.installationId,recoveryEpoch:snapshot.recoveryEpoch,conversationId:snapshot.conversationId,cursor:reply.sequence,messages:snapshot.messages + [user,reply],runs:snapshot.runs + [run],lastReadMessageSequence:snapshot.lastReadMessageSequence)
                let receipt = Receipt(requestId:body.requestId,conversationId:parts[2],messageId:user.id,runId:run.id,bindingRevisionId:body.bindingRevisionId,inputDigest:SHA256.hash(data:Data(body.text.utf8)).map { String(format:"%02x",$0) }.joined(),acceptedAt:Self.time)
                let acceptance = Acceptance(receipt:receipt,snapshot:updated)
                state.snapshots[parts[2]] = updated; state.receipts[body.requestId] = acceptance
                try Self.write(state,to:file); return try response(acceptance,status:201)
            }
        }
        return try error("fixtureRoute","This action is not part of the interactive fixture.")
    }
}
#endif
