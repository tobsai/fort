import Foundation
import Observation

@Observable @MainActor public final class MessengerModel {
    public let isOfflinePreview: Bool
    public private(set) var isInteractiveFixture = false
    public var hasWorkspace: Bool { isInteractiveFixture || installation?.mode == "openai" }
    #if DEBUG
    private var interactiveFixture: InteractiveFixtureStore?
    public var fixtureCatalogScenario: FixtureCatalogScenario = .ready
    func configureInteractiveFixture(client: FortClient, fixture: InteractiveFixtureStore, installation: Installation) throws {
        isInteractiveFixture = true; self.client = client; interactiveFixture = fixture; self.installation = installation
        expectedInstallation = installation.installationId; showConnection = false
        pendingStore = try PendingStore(directory:storage.directory,installationID:installation.installationId)
        if let saved = try storage.navigation(installationID:installation.installationId) { selectedAgentID = saved.selectedAgentID; selectedConversationID = saved.selectedConversationID }
    }
    public func setFixtureCatalog(_ scenario: FixtureCatalogScenario) async {
        guard let interactiveFixture else { return }
        do { try await interactiveFixture.setScenario(scenario); fixtureCatalogScenario = scenario; await refreshModels() }
        catch { status = "Could not save the fixture scenario." }
    }
    public func reloadInteractiveFixture() async {
        guard let interactiveFixture else { return }
        do { try await interactiveFixture.reload(); fixtureCatalogScenario = await interactiveFixture.scenario; await refresh(); loadLocalImages(force:true); await refreshModels(); status = "Reloaded saved QA data · no network" }
        catch { status = "Saved QA data could not be reloaded." }
    }
    public func addFixtureImage() async {
        guard isInteractiveFixture else { return }
        await importLocalImage(InteractiveFixture.imageURL)
    }
    #endif
    public var endpoint = "http://127.0.0.1:8788"
    public var expectedInstallation = ""
    public var enrollmentCode = ""
    public var deviceName = "Fort native client"
    public var draft = ""
    public var agents: [Agent] = []
    public var selectedAgentID: String?
    public var conversations: [Conversation] = []
    public var skills: [InstructionSkill] = []
    public var lastSavedSkill: InstructionSkill?
    public var routines: [RoutineRevision] = []
    public var schedules: [ScheduleRevision] = []
    public var selectedConversationID: String?
    public var recipientAgentID: String?
    public var selectedSkillIDs: Set<String> = []
    public var workspaceSaving = false
    public var snapshot: Snapshot?
    public var pending: [PendingSubmission] = []
    public var pendingErrors: [String:String] = [:]
    public var installation: Installation?
    public var localImages: [LocalChatImage] = []
    private var loadedImageScope: String?
    public var imageError: String?
    public var importingImage = false
    private var imageStore: LocalChatImageStore { LocalChatImageStore(directory:storage.directory.appendingPathComponent("local-images",isDirectory:true)) }
    public var modelCatalog: ModelCatalog = .unchecked
    public var loadingModels = false
    public var pairingStatus: PairingStatus?
    public var pairingMessage = "Owner setup has not been activated."
    public var lifecycleBusy = false
    private var pairingRemoved = false
    private var hostStopped = false
    public var pairingCleanupPending = false
    public var pairingRemovalPending = false
    public var localPairingForgotten = false
    private var ownedInstallation: (any OwnedMacInstallation)?
    public var uninstallAppURL: URL? { ownedInstallation?.appURL }
    public var canManagePairing: Bool { !isOfflinePreview && !pairingRemoved && client != nil && installation?.supports("ownerPairing") == true }
    public var runtimeReadiness: RuntimeReadiness?
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
    public var supportsRoutines: Bool { installation?.supports("routines") == true }
    public var supportsCommands: Bool { !isOfflinePreview && !pairingRemoved && !hostStopped && installation?.supportsCommands == true }
    public var supportsMemory: Bool { installation?.supports("conversationMemory") == true }
    public var conversationMemory: ConversationMemory? { snapshot?.conversationId == activeConversationID ? snapshot?.memory : nil }
    public var canEditMemory: Bool { supportsMemory && snapshot?.conversationId == activeConversationID && snapshot?.recoveryEpoch == installation?.recoveryEpoch }
    public var canSend: Bool { (!supportsMemory || canEditMemory) && !draft.isEmpty && draft.utf8.count <= 65_536 && recipient?.binding.enabled == true && supportsCommands && selectedSkillIDs.count <= 8 && selectedSkillIDs.isSubset(of:Set(skills.filter(\.canInvoke).map(\.id))) }
    public init(directory: URL) throws { storage = try DeviceStorage(directory:directory); session = .shared; isOfflinePreview = false }
    init(storage: DeviceStorage, session: URLSession, isOfflinePreview: Bool = false, ownedInstallation: (any OwnedMacInstallation)? = nil) { self.storage = storage; self.session = session; self.isOfflinePreview = isOfflinePreview; self.ownedInstallation = ownedInstallation }
    public func restore() async {
        guard !isOfflinePreview, !isInteractiveFixture, client == nil else { return }
        do {
            let active = try storage.connection()
            guard let connection = try active ?? storage.disconnectedConnection() else { return }
            let disconnected = try storage.disconnectedConnection()
            let intent = try storage.pairingRemoval()
            let removal = intent?.connection == connection ? intent : nil
            pairingRemoved = active == nil || active == disconnected || removal != nil
            pairingRemovalPending = removal?.state == .requested || removal?.state == .locallyForgotten
            localPairingForgotten = removal?.state == .locallyForgotten && active == nil
            pairingCleanupPending = active != nil && (removal?.state == .confirmed || (removal == nil && active == disconnected))
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
            if pairingRemoved {
                loadLocalImages(); status = "Disconnected · saved history"
                pairingMessage = pairingRemovalPending ? "Disconnect intent saved. Service revocation and shutdown are unconfirmed." : pairingCleanupPending ? "Pairing was revoked; local credential removal still needs to finish." : "Pairing removed. History is kept on this device."
                return
            }
            hostStopped = false
            client = try FortClient(baseURL:connection.baseURL,installationID:connection.installationID,token:storage.token(for:connection),fixtureLoopback:connection.fixtureLoopback,session:session)
            loadLocalImages();await refresh()
        } catch { status = "Could not restore this device connection. Enroll again to continue." }
    }
    public func enroll() async {
        guard !isOfflinePreview, !isInteractiveFixture, !isBusy, let url = URL(string:endpoint) else { return }
        isBusy = true; defer { isBusy = false }
        do {
            let initial = try FortClient(baseURL:url,installationID:expectedInstallation,fixtureLoopback:true,session:session)
            let enrolled = try await initial.enroll(code:enrollmentCode,deviceName:deviceName)
            let connection = DeviceConnection(baseURL:url,installationID:expectedInstallation,deviceID:enrolled.deviceId,fixtureLoopback:true)
            try storage.save(connection,token:enrolled.deviceToken)
            pairingRemoved = false; hostStopped = false; pairingCleanupPending = false; pairingRemovalPending = false; localPairingForgotten = false; pairingStatus = nil
            client = try FortClient(baseURL:url,installationID:expectedInstallation,token:enrolled.deviceToken,fixtureLoopback:true,session:session)
            pendingStore = try PendingStore(directory:storage.directory,installationID:expectedInstallation)
            pending = try pendingStore?.pending() ?? []; installation = enrolled.installation
            snapshot = nil; agents = []; conversations = []; skills = []; selectedAgentID = nil; selectedConversationID = nil; recipientAgentID = nil; selectedSkillIDs = []; enrollmentCode = ""; showConnection = false
            await synchronize()
        } catch { status = error.localizedDescription }
    }
    public func refreshPairing() async {
        guard canManagePairing, let client else { return }
        do {
            let pairing = try await client.pairing()
            guard let connection = try storage.connection(), pairing.deviceId == connection.deviceID, pairing.mode == .ownerLocal else { throw OwnerLifecycleError.wrongPairing }
            pairingStatus = pairing; pairingMessage = "Pairing lasts until you disconnect unless you choose an expiry."
        } catch { pairingMessage = error.localizedDescription }
    }
    public func setPairingExpiry(days: Int?) async {
        guard canManagePairing, !lifecycleBusy, let client else { return }
        lifecycleBusy = true; defer { lifecycleBusy = false }
        do {
            let pairing = try await client.setPairingExpiry(days:days)
            guard let connection = try storage.connection(), pairing.deviceId == connection.deviceID, pairing.mode == .ownerLocal else { throw OwnerLifecycleError.wrongPairing }
            pairingStatus = pairing; pairingMessage = "Pairing expiry saved."
        } catch { pairingMessage = error.localizedDescription }
    }
    private func completePairingRemoval(_ connection: DeviceConnection) throws {
        client = nil; connected = false; hostStopped = true; pairingRemoved = true; pairingStatus = nil
        pairingCleanupPending = true; pairingRemovalPending = false
        try storage.removePairing(connection)
        pairingCleanupPending = false; localPairingForgotten = false
        pairingMessage = "Disconnected. Runtime shutdown and revocation were confirmed; the local credential was removed. History is kept."
        status = pairingMessage
    }
    public func stopLocalHost(disconnect: Bool) async {
        guard canManagePairing, !isBusy, !lifecycleBusy, let client else { return }
        lifecycleBusy = true; isBusy = true
        defer { lifecycleBusy = false; isBusy = false }
        do {
            guard let connection = try storage.connection() else { throw OwnerLifecycleError.wrongPairing }
            if disconnect {
                try storage.beginPairingRemoval(connection)
                pairingRemovalPending = true; pairingRemoved = true
            }
            _ = try await client.stopHost(disconnect:disconnect)
            if disconnect { try completePairingRemoval(connection) }
            else { self.client = nil; connected = false; hostStopped = true; pairingMessage = "Runtime stopped. Pairing and history are kept. Start the owned local host to reconnect."; status = pairingMessage }
        } catch {
            if disconnect && pairingRemovalPending {
                self.client = nil; connected = false
                pairingMessage = "Disconnect intent saved. Service revocation and shutdown are unconfirmed. Retry recovery or explicitly forget only this device's credential."
            } else { pairingMessage = error.localizedDescription }
        }
    }
    public func retryPairingRemoval() async {
        guard pairingRemovalPending, !isBusy, !lifecycleBusy, !isInteractiveFixture else { return }
        lifecycleBusy = true; isBusy = true
        defer { lifecycleBusy = false; isBusy = false }
        do {
            guard let intent = try storage.pairingRemoval() else { throw OwnerLifecycleError.wrongPairing }
            if let ownedInstallation {
                let proof = try await ownedInstallation.recoverPairingRemoval(intent.connection)
                guard proof.connection == intent.connection, proof.hostStopped, proof.pairingRevoked else { throw OwnerLifecycleError.stopUnconfirmed }
            } else {
                guard intent.state != .locallyForgotten, try storage.connection() == intent.connection else { throw OwnerLifecycleError.unavailable }
                let recovery = try FortClient(baseURL:intent.connection.baseURL,installationID:intent.connection.installationID,token:storage.token(for:intent.connection),fixtureLoopback:intent.connection.fixtureLoopback,session:session)
                _ = try await recovery.stopHost(disconnect:true)
            }
            try completePairingRemoval(intent.connection)
        } catch { pairingMessage = pairingCleanupPending ? "Revocation confirmed; local credential removal is incomplete." : "Service revocation and shutdown remain unconfirmed. Verified local host recovery is required if the service credential expired or its response was lost." }
    }
    public func forgetLocalPairing() {
        guard pairingRemovalPending, !lifecycleBusy else { return }
        do {
            guard let intent = try storage.pairingRemoval() else { throw OwnerLifecycleError.wrongPairing }
            try storage.removePairing(intent.connection,revocationConfirmed:false)
            client = nil; connected = false; pairingRemoved = true; localPairingForgotten = true
            pairingMessage = "Local credential removed only. Service revocation and shutdown are unconfirmed. History is kept."
        } catch { pairingMessage = "Local credential removal could not finish. Service revocation remains unconfirmed." }
    }
    public func finishPairingRemoval() {
        guard pairingCleanupPending, !lifecycleBusy else { return }
        do {
            guard let connection = try storage.connection() ?? storage.disconnectedConnection() else { throw OwnerLifecycleError.wrongPairing }
            try storage.removePairing(connection); pairingCleanupPending = false; pairingRemovalPending = false
            pairingMessage = "Pairing removed. History is kept on this device."
        } catch { pairingMessage = "Local credential removal is incomplete. Try again after Keychain access is available." }
    }
    public func uninstall(history: HistoryRemoval = .keep) async {
        guard !isOfflinePreview, !isBusy, !lifecycleBusy, let ownedInstallation else { return }
        lifecycleBusy = true; isBusy = true
        defer { lifecycleBusy = false; isBusy = false }
        do {
            guard let connection = try storage.connection() ?? storage.disconnectedConnection() else { throw OwnerLifecycleError.wrongPairing }
            let removal = OwnerLifecycle(installation:ownedInstallation,removeCredential:{ self.client = nil; self.connected = false; self.pairingRemoved = true; self.pairingCleanupPending = true; try self.storage.removePairing(connection); self.pairingCleanupPending = false })
            try await removal.uninstall(history:history)
            client = nil; connected = false; pairingRemoved = true
            pairingMessage = history == .keep ? "Fort was removed. History is kept." : "Fort and the confirmed local history were removed."
        } catch { pairingMessage = error.localizedDescription }
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
            runtimeReadiness = current.supports("runtimeRecovery") ? try await client.runtimeReadiness() : nil
            agents = try await client.agents()
            if !agents.contains(where:{$0.agentId == selectedAgentID}) { selectedAgentID = agents.first?.agentId }
            if hasWorkspace {
                conversations = try await client.conversations(); skills = try await client.skills()
                routines = current.supports("routines") ? try await client.routines() : []
                schedules = current.supports("schedules") ? try await client.schedules() : []
                if !conversations.contains(where:{$0.id == selectedConversationID}) { selectedConversationID = selectedAgent?.homeConversationId }
            } else { conversations = []; skills = []; selectedConversationID = nil }
            try saveNavigation()
            pending = try pendingStore?.pending() ?? []
            pendingErrors = [:]
            if current.supportsCommands {
                for queued in pending where queued.recoveryEpoch == current.recoveryEpoch {
                    guard let item = try pendingStore?.pending().first(where: { $0.requestID == queued.requestID }) else { continue }
                    do {
                        if item.cancelRequested == true {
                            if current.supports("requestCancellation") { try await reconcileCancellation(item,client:client) }
                        } else if item.canRetry(in:current.recoveryEpoch) && (item.routine == nil || current.supports("routines")) {
                            let acceptance = try await client.submit(item)
                            guard item.matches(acceptance) else { throw ClientError.invalidResponse }
                            try persist(acceptance.snapshot,conversationID:item.conversationID,epoch:current.recoveryEpoch)
                            if let retained = try pendingStore?.pending().first(where: { $0.requestID == item.requestID }), retained.cancelRequested == true {
                                if current.supports("requestCancellation") { try await reconcileCancellation(retained,client:client) }
                            } else { try pendingStore?.remove(requestID:item.requestID) }
                        }
                    } catch {
                        pendingErrors[item.requestID] = error.localizedDescription
                        if case ClientError.service(let code, let reason) = error, ["invalidConfiguration","memoryChanged","bindingChanged","skillChanged","skillReviewRequired","recipientMismatch","requestConflict","requestCanceled","bindingConflict","bindingDisabled","routineChanged","invalidRoutine"].contains(code) { try pendingStore?.requireReview(requestID:item.requestID,reason:reason) }
                    }
                }
            }
            pending = try pendingStore?.pending() ?? []
            if let conversationID = activeConversationID {
                let incoming = try await client.snapshot(conversationID:conversationID)
                try persist(incoming,conversationID:conversationID,epoch:current.recoveryEpoch)
            }
            loadLocalImages();connected = true
            status = !current.supportsCommands ? "This Fort version is read-only. Update the app to send or change work." : pending.contains(where:{$0.cancelRequested == true}) ? "Cancellation saved on this device · confirmation pending" : pending.contains(where:{!$0.canRetry(in:current.recoveryEpoch)}) ? "Pending work needs review and will not resend." : pendingErrors.isEmpty ? (current.mode == "fixture" ? "Connected · fixture only" : "Connected · local prototype") : "Pending work needs review · history is up to date"
            if isInteractiveFixture { status = "Interactive QA · simulated replies · no network or credentials" }
        } catch {
            connected = false; status = error.localizedDescription
            if snapshot == nil, let conversationID = activeConversationID { snapshot = try? storage.snapshot(installationID:expectedInstallation,conversationID:conversationID) }
        }
    }
    public func cancelPending(_ item: PendingSubmission) async {
        guard installation?.supports("requestCancellation") == true, item.recoveryEpoch == installation?.recoveryEpoch, let pendingStore else { return }
        do {
            try pendingStore.requestCancellation(item)
            pending = try pendingStore.pending()
            status = "Cancellation saved on this device · confirmation pending"
            if !isBusy { await refresh() }
        } catch { status = error.localizedDescription }
    }
    private func reconcileCancellation(_ item: PendingSubmission, client: FortClient) async throws {
        let cancellation = try await client.cancelPending(item)
        guard cancellation.requestId == item.requestID, cancellation.conversationId == item.conversationID,
              cancellation.bindingRevisionId == item.bindingRevision, cancellation.recoveryEpoch == item.recoveryEpoch else { throw ClientError.invalidResponse }
        switch cancellation.status {
        case .notAccepted:
            guard cancellation.receipt == nil, cancellation.run == nil, cancellation.snapshot == nil else { throw ClientError.invalidResponse }
        case .accepted:
            guard let receipt = cancellation.receipt, let run = cancellation.run, let snapshot = cancellation.snapshot,
                  run.runId == receipt.runId, run.requestId == item.requestID, run.conversationId == item.conversationID,
                  run.bindingRevisionId == item.bindingRevision, ["cancelRequested","canceled","completed","failed","outcomeUnknown","closedUnknown"].contains(run.state), snapshot.runs.contains(run),
                  item.matches(Acceptance(receipt:receipt,snapshot:snapshot)) else { throw ClientError.invalidResponse }
            try persist(snapshot,conversationID:item.conversationID,epoch:item.recoveryEpoch)
        }
        try pendingStore?.remove(requestID:item.requestID)
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
            try pendingStore.enqueue(conversationID:conversationID,bindingRevision:agent.binding.bindingRevisionId,recoveryEpoch:installation.recoveryEpoch,text:draft,recipientAgentID:hasWorkspace ? agent.id : nil,skills:hasWorkspace ? skills.filter { selectedSkillIDs.contains($0.id) }.map(\.reference) : nil,memoryRevisionId:supportsMemory ? conversationMemory?.revisionId : nil)
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
        loadLocalImages()
        do { try saveNavigation() } catch { status = error.localizedDescription }
        if refreshing { await withCheckedContinuation { refreshWaiters.append($0) } }
        await refresh()
    }
    private func beginWorkspaceSave() async -> FortClient? {
        guard !workspaceSaving else { return nil }
        workspaceSaving = true
        if refreshing { await withCheckedContinuation { refreshWaiters.append($0) } }
        guard supportsCommands, hasWorkspace, !isBusy, let client else { workspaceSaving = false; return nil }
        isBusy = true; return client
    }
    private func loadLocalImages(force: Bool = false) {
        guard let installation, let conversationID = activeConversationID else { localImages = []; loadedImageScope = nil; return }
        let scope = installation.installationId + "\0" + conversationID
        guard force || loadedImageScope != scope else { return }
        loadedImageScope = scope; imageError = nil
        do { localImages = try imageStore.images(installationID:installation.installationId,conversationID:conversationID) }
        catch { localImages = []; imageError = "A local image preview could not be loaded. Your conversation history is intact." }
    }
    public func importLocalImage(_ url: URL) async {
        guard !importingImage, !isOfflinePreview, let installation, let conversationID = activeConversationID else { return }
        importingImage = true; defer { importingImage = false }
        let scoped = url.startAccessingSecurityScopedResource();defer { if scoped { url.stopAccessingSecurityScopedResource() } }
        let store = imageStore, sequence = snapshot?.messages.last?.sequence ?? 0, installationID = installation.installationId
        do {
            _ = try await Task.detached { try store.importImage(from:url,installationID:installationID,conversationID:conversationID,afterSequence:sequence) }.value
            loadLocalImages(force:true);status = "Image saved on this device. It was not sent to the agent."
        } catch { status = (error as? LocalImageError)?.errorDescription ?? "The local image could not be displayed. Choose a still PNG or JPEG within the size limit." }
    }
    public func removeLocalImage(_ image: LocalChatImage) {
        guard image.installationID == installation?.installationId, image.conversationID == activeConversationID else { return }
        do { try imageStore.remove(image);loadLocalImages(force:true) } catch { status = "The preview could not be removed. The original image file has not changed." }
    }
    public func refreshModels() async {
        guard !loadingModels else { return }
        guard installation?.supports("modelCatalog") == true, let client else { modelCatalog = .unchecked; return }
        loadingModels = true; defer { loadingModels = false }
        do {
            let catalog = try await client.models()
            guard catalog.provider == "codex", ["ready","unavailable","error"].contains(catalog.state), catalog.models.count <= 800, Set(catalog.models.map(\.id)).count == catalog.models.count else { throw ClientError.invalidResponse }
            modelCatalog = catalog
        } catch { modelCatalog = ModelCatalog(provider:"codex",state:"error",models:[],detail:"Could not check models. Your saved selection has not changed.") }
    }
    public func saveAgent(_ agent: Agent?, name: String, model: String, instructions: String, avatar: AgentAvatar? = nil) async -> Bool {
        guard let client = await beginWorkspaceSave() else { return false }
        do {
            let saved = try await client.saveAgent(agent,name:name,model:model,instructions:instructions,avatar:avatar)
            selectedAgentID = saved.id; selectedConversationID = saved.homeConversationId; recipientAgentID = nil
            isBusy = false; await refresh(); workspaceSaving = false; return true
        } catch { status = error.localizedDescription; isBusy = false; workspaceSaving = false; return false }
    }
    public func saveMemory(conversationID: String, memory: ConversationMemory?, text: String) async -> Bool {
        guard supportsMemory, text.utf8.count <= 8192, let client = await beginWorkspaceSave(), let installation else { return false }
        do {
            let saved = try await client.saveMemory(conversationID:conversationID,memory:memory,text:text,recoveryEpoch:installation.recoveryEpoch)
            guard saved.conversationId == conversationID, saved.text == text, !saved.revisionId.isEmpty else { throw ClientError.invalidResponse }
            isBusy = false; await refresh(); workspaceSaving = false; return true
        } catch { status = error.localizedDescription; isBusy = false; workspaceSaving = false; return false }
    }
    public func saveSkill(_ skill: InstructionSkill?, name: String, instructions: String) async -> Bool {
        guard let client = await beginWorkspaceSave() else { return false }
        do {
            lastSavedSkill = try await client.saveSkill(skill,name:name,instructions:instructions)
            isBusy = false; await refresh(); workspaceSaving = false; return true
        } catch { status = error.localizedDescription; isBusy = false; workspaceSaving = false; return false }
    }
    public func importSkillBundle(_ data: Data, replacing skill: InstructionSkill? = nil) async -> InstructionSkill? {
        guard let client = await beginWorkspaceSave() else { return nil }
        do {
            let saved = try await client.importSkillBundle(data,replacing:skill)
            isBusy = false; await refresh(); workspaceSaving = false; return saved
        } catch { status = error.localizedDescription; isBusy = false; workspaceSaving = false; return nil }
    }
    public func reviewSkill(_ skill: InstructionSkill, enable: Bool) async -> InstructionSkill? {
        guard let client = await beginWorkspaceSave() else { return nil }
        do {
            let saved = try await client.reviewSkill(skill,enable:enable)
            isBusy = false; await refresh(); workspaceSaving = false; return saved
        } catch { status = error.localizedDescription; isBusy = false; workspaceSaving = false; return nil }
    }
    public func skillRevision(_ reference: RoutineSkill) async throws -> InstructionSkill {
        guard let client else { throw ClientError.service("offline","Connect to review this pinned skill.") }
        let skill = try await client.skillRevision(skillID:reference.skillId,revisionID:reference.revisionId)
        guard skill.id == reference.skillId, skill.revisionId == reference.revisionId, skill.bundle?.digest == reference.digest else { throw ClientError.invalidResponse }
        return skill
    }
    public func previousRevision(for skill: InstructionSkill) async throws -> InstructionSkill? {
        guard let revision = skill.previousRevisionId else { return nil }
        guard let client else { throw ClientError.service("offline","Connect to Fort to compare this revision.") }
        return try await client.skillRevision(skillID:skill.id,revisionID:revision)
    }
    public func saveSchedule(_ revision: ScheduleRevision?, definition: ScheduleDefinition) async -> Bool {
        guard installation?.supports("schedules") == true, let client = await beginWorkspaceSave() else { return false }
        do { _ = try await client.saveSchedule(revision,definition:definition); isBusy = false; await refresh(); workspaceSaving = false; return true }
        catch { status = error.localizedDescription; isBusy = false; workspaceSaving = false; return false }
    }
    public func routineRevision(routineID: String, revisionID: String) async throws -> RoutineRevision {
        guard let client else { throw ClientError.service("offline","Connect to review the pinned routine.") }; return try await client.routineRevision(routineID:routineID,revisionID:revisionID)
    }
    public func schedulePreview(_ revision: ScheduleRevision) async throws -> SchedulePreview {
        guard let client else { throw ClientError.service("offline","Connect to preview the schedule.") }; return try await client.previewSchedule(revision)
    }
    public func scheduleOccurrences(_ revision: ScheduleRevision) async throws -> [ScheduleOccurrence] {
        guard let client else { throw ClientError.service("offline","Connect to inspect occurrences.") }; return try await client.scheduleOccurrences(revision)
    }
    public func setSchedule(_ revision: ScheduleRevision, enabled: Bool, preview: SchedulePreview?) async -> Bool {
        guard installation?.supports("schedules") == true, let client = await beginWorkspaceSave(), let installation else { return false }
        do { _ = try await client.setSchedule(revision,enabled:enabled,preview:preview,recoveryEpoch:installation.recoveryEpoch); isBusy = false; await refresh(); workspaceSaving = false; return true }
        catch { status = error.localizedDescription; isBusy = false; workspaceSaving = false; return false }
    }
    public func saveRoutine(_ revision: RoutineRevision?, definition: RoutineDefinition) async -> Bool {
        guard supportsRoutines, let client = await beginWorkspaceSave() else { return false }
        do {
            _ = try await client.saveRoutine(revision,definition:definition)
            isBusy = false; await refresh(); workspaceSaving = false; return true
        } catch { status = error.localizedDescription; isBusy = false; workspaceSaving = false; return false }
    }
    public func runRoutine(_ revision: RoutineRevision, inputs: [String:RoutineValue]) async -> Bool {
        if refreshing { await withCheckedContinuation { refreshWaiters.append($0) } }
        guard supportsRoutines, !isBusy, let installation, let pendingStore else { return false }
        do {
            try pendingStore.enqueueRoutine(revision,inputs:inputs,recoveryEpoch:installation.recoveryEpoch)
            selectedConversationID = revision.definition.conversationId; selectedAgentID = revision.definition.agentId
            pending = try pendingStore.pending(); try saveNavigation(); await refresh(); return true
        } catch { status = "The routine could not be saved on this device. Try again."; return false }
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
        guard supportsCommands, (approval.kind != "routineGate" || installation?.supports("routineGates") == true), !isBusy, approval.canDecide, let client, approval.installationId == installation?.installationId, approval.recoveryEpoch == installation?.recoveryEpoch else { return }
        isBusy = true
        do {
            let result = try await client.resolve(approval,approve:approve)
            status = "Approval decision: \(result.approval.state)"
        } catch { status = error.localizedDescription }
        isBusy = false
        await refresh()
    }
    public func restartRuntime() async {
        if refreshing { await withCheckedContinuation { refreshWaiters.append($0) } }
        guard installation?.supports("runtimeRecovery") == true, runtimeReadiness?.state == "restartRequired", runtimeReadiness?.canRestart == true, !isBusy, let client, let installation else { return }
        isBusy = true
        do { runtimeReadiness = try await client.restartRuntime(recoveryEpoch:installation.recoveryEpoch) }
        catch { status = error.localizedDescription }
        isBusy = false; await refresh()
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
