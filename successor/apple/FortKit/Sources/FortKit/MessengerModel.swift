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
    public var selectedAgent: Agent? { agents.first { $0.agentId == selectedAgentID } }
    public var canSend: Bool { !draft.isEmpty && draft.utf8.count <= 65_536 && selectedAgent?.binding.enabled == true && installation?.supportsCommands == true }
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
                if let agent = selectedAgent, let cached = try storage.snapshot(installationID:connection.installationID,conversationID:agent.homeConversationId) {
                    var transcript = Transcript(installationID:connection.installationID,conversationID:agent.homeConversationId)
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
            snapshot = nil; agents = []; selectedAgentID = nil; enrollmentCode = ""; showConnection = false
            await synchronize()
        } catch { status = error.localizedDescription }
    }
    public func refresh() async {
        guard !isBusy, client != nil else { return }
        isBusy = true; defer { isBusy = false }
        await synchronize()
    }
    private func synchronize() async {
        guard let client else { return }
        do {
            let current = try await client.installation()
            installation = current
            agents = try await client.agents()
            if !agents.contains(where:{$0.agentId == selectedAgentID}) { selectedAgentID = agents.first?.agentId }
            try storage.saveNavigation(SavedNavigation(installation:current,agents:agents,selectedAgentID:selectedAgentID))
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
                    } catch { pendingErrors[item.requestID] = error.localizedDescription }

                }
            }
            pending = try pendingStore?.pending() ?? []
            if let agent = selectedAgent {
                let incoming = try await client.snapshot(conversationID:agent.homeConversationId)
                try persist(incoming,conversationID:agent.homeConversationId,epoch:current.recoveryEpoch)
            }
            connected = true
            status = pending.contains(where:{!$0.canRetry(in:current.recoveryEpoch)}) ? "Installation was restored. Pending work needs review and will not resend." : pendingErrors.isEmpty ? "Connected · fixture only" : "Pending work needs review · history is up to date"
        } catch {
            connected = false; status = error.localizedDescription
            if snapshot == nil, let agent = selectedAgent { snapshot = try? storage.snapshot(installationID:expectedInstallation,conversationID:agent.homeConversationId) }
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
        if selectedAgent?.homeConversationId == conversationID { snapshot = accepted }
    }
    public func send() async {
        guard canSend, let agent = selectedAgent, let installation, let pendingStore else { return }
        do {
            try pendingStore.enqueue(conversationID:agent.homeConversationId,bindingRevision:agent.binding.bindingRevisionId,recoveryEpoch:installation.recoveryEpoch,text:draft)
            draft = ""; pending = try pendingStore.pending()
            status = "Saved on this device · not yet accepted by Fort"
            await refresh()
        } catch { status = "Your message could not be saved on this device. Keep the draft and try again." }
    }
    public func decide(_ approval: Approval, approve: Bool) async {
        guard !isBusy, approval.canDecide, let client, approval.installationId == installation?.installationId, approval.recoveryEpoch == installation?.recoveryEpoch else { return }
        isBusy = true
        do {
            let result = try await client.resolve(approval,approve:approve)
            status = "Approval decision: \(result.approval.state)"
        } catch { status = error.localizedDescription }
        isBusy = false
        await refresh()
    }
    public func cancel(_ run: Run) async {
        guard !isBusy, let client, let installation else { return }
        isBusy = true
        do { let result = try await client.cancel(runID:run.runId,recoveryEpoch:installation.recoveryEpoch); status = result.displayState }
        catch { status = error.localizedDescription }
        isBusy = false
        await refresh()
    }
    public func closeUnknown(_ run: Run) async {
        guard !isBusy, run.state == "outcomeUnknown", let client, let installation else { return }
        isBusy = true
        do { let result = try await client.closeUnknown(runID:run.runId,recoveryEpoch:installation.recoveryEpoch); status = result.displayState }
        catch { status = error.localizedDescription }
        isBusy = false
        await refresh()
    }
    public func markRead(_ message: Message) async {
        guard let client, let snapshot, message.conversationId == snapshot.conversationId else { return }
        do { try await client.acknowledgeRead(conversationID:message.conversationId,messageID:message.messageId,cursor:snapshot.cursor,recoveryEpoch:snapshot.recoveryEpoch) }
        catch { /* Read state will be retried when the message is presented again. */ }
    }
}
