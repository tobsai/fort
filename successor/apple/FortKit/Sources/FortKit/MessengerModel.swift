import Foundation
import Observation

@Observable @MainActor public final class MessengerModel {
    public var endpoint = "http://127.0.0.1:8788"
    public var expectedInstallation = ""
    public var enrollmentCode = ""
    public var deviceName = "Fort native client"
    public var draft = ""
    public var agents: [Agent] = []
    public var selectedAgentID: String?
    public var conversations: [Conversation] = []
    public var skills: [InstructionSkill] = []
    public var selectedConversationID: String?
    public var recipientAgentID: String?
    public var selectedSkillIDs: Set<String> = []
    public var workspaceSaving = false
    public var snapshot: Snapshot?
    public var pending: [PendingSubmission] = []
    public var pendingErrors: [String:String] = [:]
    public var installation: Installation?
    public var status = "Connect to your fixture installation."
    public var isBusy = false
    public var connected = false
    public var showConnection = true
    private let storage: DeviceStorage
    private let session: URLSession
    private var client: FortClient?
    private var pendingStore: PendingStore?
    private var refreshing = false
    private var refreshWaiters: [CheckedContinuation<Void,Never>] = []
    public var selectedAgent: Agent? { agents.first { $0.agentId == selectedAgentID } }
    public var activeConversationID: String? { selectedConversationID ?? selectedAgent?.homeConversationId }
    public var selectedConversation: Conversation? { conversations.first { $0.id == activeConversationID } }
    public var recipient: Agent? {
        if let conversation = selectedConversation {
            let recipientID = conversation.kind == "home" ? conversation.participantAgentIds.first : recipientAgentID
            return agents.first { $0.id == recipientID && conversation.participantAgentIds.contains($0.id) }
        }
        return installation?.mode == "fixture" ? selectedAgent : nil
    }
    public var supportsCommands: Bool { installation?.supportsCommands == true }
    public var canSend: Bool { !draft.isEmpty && draft.utf8.count <= 65_536 && recipient?.binding.enabled == true && supportsCommands && selectedSkillIDs.count <= 8 && selectedSkillIDs.isSubset(of:Set(skills.map(\.id))) }
    public init(directory: URL) throws { storage = try DeviceStorage(directory:directory); session = .shared }
    init(storage: DeviceStorage, session: URLSession) { self.storage = storage; self.session = session }
    public func restore() async {
        guard client == nil else { return }
        do {
            guard let connection = try storage.connection() else { return }
            endpoint = connection.baseURL.absoluteString; expectedInstallation = connection.installationID
            pendingStore = try PendingStore(directory:storage.directory,installationID:connection.installationID)
            pending = try pendingStore?.pending() ?? []
            if let saved = try storage.navigation(installationID:connection.installationID) {
                installation = saved.installation; agents = saved.agents
                selectedAgentID = agents.contains(where:{$0.agentId == saved.selectedAgentID}) ? saved.selectedAgentID : agents.first?.agentId
                conversations = saved.conversations ?? []; skills = saved.skills ?? []; selectedConversationID = saved.selectedConversationID; recipientAgentID = saved.recipientAgentID
                if let conversationID = activeConversationID, let cached = try storage.snapshot(installationID:connection.installationID,conversationID:conversationID) {
                    var transcript = Transcript(installationID:connection.installationID,conversationID:conversationID)
                    try transcript.apply(cached); snapshot = transcript.snapshot
                }
            }
            showConnection = false
            client = try FortClient(baseURL:connection.baseURL,installationID:connection.installationID,token:storage.token(for:connection),fixtureLoopback:connection.fixtureLoopback,session:session)
            await refresh()
        } catch { status = "Could not restore this device connection. Enroll again to continue." }
    }
    public func enroll() async {
        guard !isBusy, let url = URL(string:endpoint) else { return }
        isBusy = true; defer { isBusy = false }
        do {
            let initial = try FortClient(baseURL:url,installationID:expectedInstallation,fixtureLoopback:true,session:session)
            let enrolled = try await initial.enroll(code:enrollmentCode,deviceName:deviceName)
            let connection = DeviceConnection(baseURL:url,installationID:expectedInstallation,deviceID:enrolled.deviceId,fixtureLoopback:true)
            try storage.save(connection,token:enrolled.deviceToken)
            client = try FortClient(baseURL:url,installationID:expectedInstallation,token:enrolled.deviceToken,fixtureLoopback:true,session:session)
            pendingStore = try PendingStore(directory:storage.directory,installationID:expectedInstallation)
            pending = try pendingStore?.pending() ?? []; installation = enrolled.installation
            snapshot = nil; agents = []; conversations = []; skills = []; selectedAgentID = nil; selectedConversationID = nil; recipientAgentID = nil; selectedSkillIDs = []; enrollmentCode = ""; showConnection = false
            await synchronize()
        } catch { status = error.localizedDescription }
    }
    public func refresh() async {
        guard !isBusy, client != nil else { return }
        isBusy = true; refreshing = true
        defer {
            isBusy = false; refreshing = false
            let waiters = refreshWaiters; refreshWaiters.removeAll()
            waiters.forEach { $0.resume() }
        }
        await synchronize()
    }
    private func synchronize() async {
        guard let client else { return }
        do {
            let current = try await client.installation()
            installation = current
            agents = try await client.agents()
            if !agents.contains(where:{$0.agentId == selectedAgentID}) { selectedAgentID = agents.first?.agentId }
            if current.mode == "openai" {
                conversations = try await client.conversations(); skills = try await client.skills()
                if !conversations.contains(where:{$0.id == selectedConversationID}) { selectedConversationID = selectedAgent?.homeConversationId }
            } else { conversations = []; skills = []; selectedConversationID = nil }
            try saveNavigation()
            pending = try pendingStore?.pending() ?? []
            pendingErrors = [:]
            if current.supportsCommands {
                for item in pending where item.canRetry(in:current.recoveryEpoch) {
                    do {
                        let acceptance = try await client.submit(item)
                        guard acceptance.receipt.requestId == item.requestID,
                              acceptance.receipt.conversationId == item.conversationID,
                              acceptance.receipt.bindingRevisionId == item.bindingRevision,
                              acceptance.snapshot.messages.contains(where:{$0.messageId == acceptance.receipt.messageId && $0.requestId == item.requestID && $0.role == "user" && $0.text == item.text}) else { throw ClientError.invalidResponse }
                        try persist(acceptance.snapshot,conversationID:item.conversationID,epoch:current.recoveryEpoch)
                        try pendingStore?.remove(requestID:item.requestID)
                    } catch {
                        pendingErrors[item.requestID] = error.localizedDescription
                        if case ClientError.service(let code, let reason) = error, ["bindingChanged","skillChanged","recipientMismatch","requestConflict","bindingConflict"].contains(code) { try pendingStore?.requireReview(requestID:item.requestID,reason:reason) }
                    }

                }
            }
            pending = try pendingStore?.pending() ?? []
            if let conversationID = activeConversationID {
                let incoming = try await client.snapshot(conversationID:conversationID)
                try persist(incoming,conversationID:conversationID,epoch:current.recoveryEpoch)
            }
            connected = true
            status = !current.supportsCommands ? "This Fort version is read-only. Update the app to send or change work." : pending.contains(where:{!$0.canRetry(in:current.recoveryEpoch)}) ? "Pending work needs review and will not resend." : pendingErrors.isEmpty ? (current.mode == "fixture" ? "Connected · fixture only" : "Connected · local prototype") : "Pending work needs review · history is up to date"
        } catch {
            connected = false; status = error.localizedDescription
            if snapshot == nil, let conversationID = activeConversationID { snapshot = try? storage.snapshot(installationID:expectedInstallation,conversationID:conversationID) }
        }
    }
    private func persist(_ incoming: Snapshot, conversationID: String, epoch: String) throws {
        guard incoming.recoveryEpoch == epoch else { throw ClientError.invalidResponse }
        var transcript = Transcript(installationID:expectedInstallation,conversationID:conversationID)
        if let cached = try storage.snapshot(installationID:expectedInstallation,conversationID:conversationID) { try transcript.apply(cached) }
        try transcript.apply(incoming)
        guard let accepted = transcript.snapshot else { throw ClientError.invalidResponse }
        // Crash before pending removal leaves the same immutable request available for safe replay.
        try storage.saveSnapshot(accepted)
        if activeConversationID == conversationID { snapshot = accepted }
    }
    public func send() async {
        guard canSend, let agent = recipient, let conversationID = activeConversationID, let installation, let pendingStore else { return }
        do {
            try pendingStore.enqueue(conversationID:conversationID,bindingRevision:agent.binding.bindingRevisionId,recoveryEpoch:installation.recoveryEpoch,text:draft,recipientAgentID:installation.mode == "openai" ? agent.id : nil,skills:installation.mode == "openai" ? skills.filter { selectedSkillIDs.contains($0.id) }.map(\.reference) : nil)
            draft = ""; selectedSkillIDs = []; pending = try pendingStore.pending(); try saveNavigation()
            status = "Saved on this device · not yet accepted by Fort"
            await refresh()
        } catch { status = "Your message could not be saved on this device. Keep the draft and try again." }
    }
    private func saveNavigation() throws {
        guard let installation else { return }
        try storage.saveNavigation(SavedNavigation(installation:installation, agents:agents, selectedAgentID:selectedAgentID, conversations:conversations, skills:skills, selectedConversationID:selectedConversationID, recipientAgentID:recipientAgentID))
    }
    public func selectConversation(_ id: String?) async {
        selectedConversationID = id; recipientAgentID = nil; selectedSkillIDs = []
        if let room = selectedConversation, room.kind == "home" { selectedAgentID = room.participantAgentIds.first }
        snapshot = id.flatMap { try? storage.snapshot(installationID:expectedInstallation,conversationID:$0) }
        do { try saveNavigation() } catch { status = error.localizedDescription }
        if refreshing { await withCheckedContinuation { refreshWaiters.append($0) } }
        await refresh()
    }
    private func beginWorkspaceSave() async -> FortClient? {
        guard !workspaceSaving else { return nil }
        workspaceSaving = true
        if refreshing { await withCheckedContinuation { refreshWaiters.append($0) } }
        guard supportsCommands, installation?.mode == "openai", !isBusy, let client else { workspaceSaving = false; return nil }
        isBusy = true; return client
    }
    public func saveAgent(_ agent: Agent?, name: String, model: String, instructions: String) async -> Bool {
        guard let client = await beginWorkspaceSave() else { return false }
        do {
            let saved = try await client.saveAgent(agent,name:name,model:model,instructions:instructions)
            selectedAgentID = saved.id; selectedConversationID = saved.homeConversationId; recipientAgentID = nil
            isBusy = false; await refresh(); workspaceSaving = false; return true
        } catch { status = error.localizedDescription; isBusy = false; workspaceSaving = false; return false }
    }
    public func saveSkill(_ skill: InstructionSkill?, name: String, instructions: String) async -> Bool {
        guard let client = await beginWorkspaceSave() else { return false }
        do {
            _ = try await client.saveSkill(skill,name:name,instructions:instructions)
            isBusy = false; await refresh(); workspaceSaving = false; return true
        } catch { status = error.localizedDescription; isBusy = false; workspaceSaving = false; return false }
    }
    public func createConversation(name: String, participantAgentIDs: [String]) async -> Bool {
        guard let client = await beginWorkspaceSave() else { return false }
        do {
            let room = try await client.createConversation(name:name,participantAgentIds:participantAgentIDs)
            selectedConversationID = room.id; recipientAgentID = nil
            isBusy = false; await refresh(); workspaceSaving = false; return true
        } catch { status = error.localizedDescription; isBusy = false; workspaceSaving = false; return false }
    }
    public func decide(_ approval: Approval, approve: Bool) async {
        guard supportsCommands, !isBusy, approval.canDecide, let client, approval.installationId == installation?.installationId, approval.recoveryEpoch == installation?.recoveryEpoch else { return }
        isBusy = true
        do {
            let result = try await client.resolve(approval,approve:approve)
            status = "Approval decision: \(result.approval.state)"
        } catch { status = error.localizedDescription }
        isBusy = false
        await refresh()
    }
    public func cancel(_ run: Run) async {
        if refreshing { await withCheckedContinuation { refreshWaiters.append($0) } }
        guard supportsCommands, !isBusy, run.canRequestCancellation, let client, let installation else { return }
        isBusy = true
        do { let result = try await client.cancel(runID:run.runId,recoveryEpoch:installation.recoveryEpoch); status = result.displayState }
        catch { status = error.localizedDescription }
        isBusy = false
        await refresh()
    }
    public func closeUnknown(_ run: Run) async {
        if refreshing { await withCheckedContinuation { refreshWaiters.append($0) } }
        guard supportsCommands, !isBusy, run.canCloseUnknown, let client, let installation else { return }
        isBusy = true
        do { let result = try await client.closeUnknown(runID:run.runId,recoveryEpoch:installation.recoveryEpoch); status = result.displayState }
        catch { status = error.localizedDescription }
        isBusy = false
        await refresh()
    }
    public func markRead(_ message: Message) async {
        guard supportsCommands, let client, let snapshot, message.conversationId == snapshot.conversationId else { return }
        do { try await client.acknowledgeRead(conversationID:message.conversationId,messageID:message.messageId,cursor:snapshot.cursor,recoveryEpoch:snapshot.recoveryEpoch) }
        catch { /* Read state will be retried when the message is presented again. */ }
    }
}
