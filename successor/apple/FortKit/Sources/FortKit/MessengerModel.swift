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
    public var installation: Installation?
    public var status = "Connect to your fixture installation."
    public var isBusy = false
    public var connected = false
    public var showConnection = true
    private let storage: DeviceStorage
    private var client: FortClient?
    private var pendingStore: PendingStore?
    public var selectedAgent: Agent? { agents.first { $0.agentId == selectedAgentID } }
    public var canSend: Bool { !draft.isEmpty && draft.utf8.count <= 65_536 && selectedAgent?.binding.enabled == true && installation?.supportsCommands == true && !isBusy }
    public init(directory: URL) throws { storage = try DeviceStorage(directory:directory) }
    public func restore() async {
        guard client == nil else { return }
        do {
            guard let connection = try storage.connection() else { return }
            endpoint = connection.baseURL.absoluteString; expectedInstallation = connection.installationID
            client = try FortClient(baseURL:connection.baseURL,installationID:connection.installationID,token:storage.token(for:connection),fixtureLoopback:connection.fixtureLoopback)
            pendingStore = try PendingStore(directory:storage.directory,installationID:connection.installationID)
            pending = try pendingStore?.pending() ?? []
            showConnection = false
            await refresh()
        } catch { status = "Could not restore this device connection. Enroll again to continue." }
    }
    public func enroll() async {
        guard !isBusy, let url = URL(string:endpoint) else { return }
        isBusy = true; defer { isBusy = false }
        do {
            let initial = try FortClient(baseURL:url,installationID:expectedInstallation,fixtureLoopback:true)
            let enrolled = try await initial.enroll(code:enrollmentCode,deviceName:deviceName)
            let connection = DeviceConnection(baseURL:url,installationID:expectedInstallation,deviceID:enrolled.deviceId,fixtureLoopback:true)
            try storage.save(connection,token:enrolled.deviceToken)
            client = try FortClient(baseURL:url,installationID:expectedInstallation,token:enrolled.deviceToken,fixtureLoopback:true)
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
            pending = try pendingStore?.pending() ?? []
            if current.supportsCommands {
                for item in pending where item.canRetry(in:current.recoveryEpoch) {
                    // Same immutable request is safe after a lost acceptance acknowledgement.
                    let acceptance = try await client.submit(item)
                    guard acceptance.receipt.requestId == item.requestID,
                          acceptance.receipt.conversationId == item.conversationID,
                          acceptance.receipt.bindingRevisionId == item.bindingRevision else { throw ClientError.invalidResponse }
                    try pendingStore?.remove(requestID:item.requestID)
                }
            }
            pending = try pendingStore?.pending() ?? []
            if let agent = selectedAgent {
                let incoming = try await client.snapshot(conversationID:agent.homeConversationId)
                var transcript = Transcript(installationID:current.installationId,conversationID:agent.homeConversationId)
                if let snapshot, snapshot.conversationId == agent.homeConversationId { try transcript.apply(snapshot) }
                try transcript.apply(incoming)
                snapshot = transcript.snapshot
                if let snapshot { try storage.saveSnapshot(snapshot) }
            }
            connected = true
            status = pending.contains(where:{!$0.canRetry(in:current.recoveryEpoch)}) ? "Installation was restored. Pending work needs review and will not resend." : "Connected · fixture only"
        } catch {
            connected = false; status = error.localizedDescription
            if snapshot == nil, let agent = selectedAgent { snapshot = try? storage.snapshot(installationID:expectedInstallation,conversationID:agent.homeConversationId) }
        }
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
    public func markRead(_ message: Message) async {
        guard let client, let snapshot, message.conversationId == snapshot.conversationId else { return }
        do { try await client.acknowledgeRead(conversationID:message.conversationId,messageID:message.messageId,cursor:snapshot.cursor,recoveryEpoch:snapshot.recoveryEpoch) }
        catch { /* Read state will be retried when the message is presented again. */ }
    }
}
