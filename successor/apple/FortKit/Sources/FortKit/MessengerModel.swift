import Foundation
import Observation

@Observable @MainActor public final class MessengerModel {
    public let isOfflinePreview: Bool
    public private(set) var isInteractiveFixture = false
    public var hasWorkspace: Bool { isInteractiveFixture || ["openai", "local"].contains(installation?.mode ?? "") }
    public var runtimeProvider: String? { installation?.runtime?.provider ?? recipient?.binding.provider ?? selectedAgent?.binding.provider ?? agents.first?.binding.provider }
    public var runtimeName: String { installation?.runtime?.name ?? runtimeProvider ?? "No runtime selected" }
    /// Every runtime the host serves, primary first (Spec 064).
    public var runtimes: [RuntimeDescriptor] { installation?.runtimes ?? installation?.runtime.map { [$0] } ?? [] }
    /// The runtime whose sign-in and models are shown; nil is the host's primary runtime.
    public private(set) var accountProvider: String?
    public var focusedRuntime: RuntimeDescriptor? { accountProvider.flatMap { name in runtimes.first { $0.provider == name } } ?? installation?.runtime }
    public var focusedRuntimeName: String { focusedRuntime?.name ?? runtimeName }
    public var canChangeAccountProvider: Bool { runtimes.count > 1 && !accountSignInBusy && !hasPendingAccountSignIn && !loadingModels }
    /// Shows the sign-in and models of one runtime. A pending sign-in keeps its own runtime until it settles.
    public func focusRuntime(_ provider: String?) async {
        let selected = provider == installation?.runtime?.provider ? nil : provider
        guard selected != accountProvider, canChangeAccountProvider, selected == nil || runtimes.contains(where:{ $0.provider == selected }) else { return }
        accountProvider = selected; accountStatusGeneration = UUID(); accountSignInStatus = nil; modelCatalog = .unchecked
        await refreshAccountSignIn()
        if canRefreshModels { await refreshModels() }
    }
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
    public var weeks: [WeekView] = []
    public var supportsWeeks: Bool { installation?.supports("weeks") == true }
    public var liveWeek: WeekView? { weeks.first(where:\.isLive) }
    public var selectedConversationID: String?
    public var recipientAgentID: String?
    public var selectedSkillIDs: Set<String> = []
    public var workspaceSaving = false
    public var snapshot: Snapshot?
    private var historyPage: Snapshot?
    private var historyRevision = 0
    public private(set) var loadingEarlierMessages = false
    public var displaySnapshot: Snapshot? {
        let page = isViewingEarlierMessages ? historyPage : snapshot
        return page?.conversationId == activeConversationID && page?.installationId == installation?.installationId ? page : nil
    }
    public var isViewingEarlierMessages: Bool { historyPage != nil && historyPage?.conversationId == activeConversationID && historyPage?.installationId == installation?.installationId && historyPage?.recoveryEpoch == installation?.recoveryEpoch }
    public func showLatestMessages() { historyRevision += 1; historyPage = nil }
    public func loadEarlierMessages() async {
        guard !loadingEarlierMessages, let client, let current = displaySnapshot,
              let before = current.window?.olderBefore, current.conversationId == activeConversationID else { return }
        loadingEarlierMessages = true
        let revision = historyRevision
        defer { loadingEarlierMessages = false }
        do {
            let incoming = try await client.snapshot(conversationID:current.conversationId,before:before)
            guard incoming.recoveryEpoch == current.recoveryEpoch, incoming.window?.before == before else { throw ClientError.invalidResponse }
            var validation = Transcript(installationID:current.installationId,conversationID:current.conversationId)
            try validation.apply(incoming)
            guard historyRevision == revision, activeConversationID == current.conversationId, installation?.installationId == current.installationId,
                  installation?.recoveryEpoch == current.recoveryEpoch, displaySnapshot?.window == current.window else { return }
            historyPage = validation.snapshot
        } catch { status = error.localizedDescription }
    }
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
    public var accountSignInStatus: AccountSignInStatus? {
        didSet {
            if oldValue != accountSignInStatus && accountSignInStatus?.accountSelected != true { modelCatalog = .unchecked }
        }
    }
    public private(set) var accountSignInBusy = false
    public private(set) var accountSignInMessage = "Sign in to connect your account. The first verified account becomes your default."
    private var accountSignInIntent: AccountSignInIntent?
    private var accountStatusGeneration = UUID()
    public var hasPendingAccountSignIn: Bool { accountSignInIntent != nil }
    public var canManageProviderAccount: Bool {
        #if os(macOS)
        canControlHost && installation?.supports("accountSignIn") == true
        #else
        false
        #endif
    }
    public var modelAccountRequirement: String? {
        guard canManageProviderAccount, accountSignInStatus?.accountSelected != true else { return nil }
        return accountSignInStatus?.reportedAccount == nil ? "Sign in to check models. If the provider asks you to choose a workspace, complete that choice there." : "The reported account needs review in Advanced account management before checking models."
    }
    public var canRefreshModels: Bool { !loadingModels && !accountSignInBusy && modelAccountRequirement == nil }
    private func accountScope() throws -> (FortClient, DeviceConnection, Installation) {
        guard canManageProviderAccount, let client, let installation, let connection = try storage.connection(),
              connection.installationID == installation.installationId, pairingStatus?.deviceId == connection.deviceID else { throw ClientError.invalidResponse }
        return (client,connection,installation)
    }
    private func accountScopeMatches(_ connection: DeviceConnection, _ selected: Installation) throws -> Bool {
        let current = try storage.connection()
        return canManageProviderAccount && installation?.installationId == selected.installationId && installation?.recoveryEpoch == selected.recoveryEpoch
            && current == connection && pairingStatus?.deviceId == connection.deviceID
    }
    private func acceptAccountStatus(_ value: AccountSignInStatus, connection: DeviceConnection, installation selected: Installation, intent: AccountSignInIntent?, generation: UUID) throws {
        guard generation == accountStatusGeneration, try accountScopeMatches(connection,selected), try storage.accountSignInIntent(for:connection) == intent else { throw ClientError.invalidResponse }
        if let intent {
            guard intent.request.recoveryEpoch == selected.recoveryEpoch else { throw ClientError.invalidResponse }
            if let operation = value.operation, intent.request.matches(operation) {
                if value.finishedResult != nil { try storage.clearAccountSignIn(intent); accountSignInIntent = nil }
            } else {
                guard value.revision == intent.request.expectedRevision, !value.isActive else { throw ClientError.invalidResponse }
                accountSignInStatus = nil; accountSignInMessage = "The saved request has no confirmed receipt. Retry that request or discard it after it expires."; return
            }
        }
        if let operation = value.operation, value.challenge != nil, operation.expiresAt <= Int64(Date().timeIntervalSince1970 * 1000) {
            accountSignInStatus = .canceling(operation); accountSignInMessage = "The code has expired. Check status for the confirmed outcome."; return
        }
        accountSignInStatus = value
        switch value {
        case .idle: accountSignInMessage = "No sign-in attempt is active."
        case .unavailable: accountSignInMessage = "This runtime does not offer sign-in here."
        case .starting: accountSignInMessage = "Waiting for the runtime’s sign-in instructions."
        case .challenge(_,let challenge): accountSignInMessage = challenge.userCode == nil ? "Open the sign-in page and approve Fort before it expires. You will return here automatically." : "Open the verification page and enter this code before it expires."
        case .canceling: accountSignInMessage = "Cancellation is awaiting confirmation."
        case .identified(_,_,let selected):
            accountSignInMessage = selected ? "Connected. Models are checked automatically for this account." : "This account could not be matched to your saved choice. Review Advanced account management before using it."
        case .finished(_,let result):
            switch result {
            case .signedIn: accountSignInMessage = "The provider has not reported one verified account. Complete any required workspace choice with the provider, then check sign-in status."
            case .canceled: accountSignInMessage = "This sign-in attempt was canceled. Existing account access was not revoked."
            case .failed: accountSignInMessage = "This sign-in attempt failed. You can start a new attempt."
            case .unconfirmed: accountSignInMessage = "This attempt’s outcome is unconfirmed. It will not be retried automatically."
            }
        }
    }
    public func refreshAccountSignIn() async {
        guard !accountSignInBusy, !lifecycleBusy else { return }
        guard canManageProviderAccount else { accountSignInStatus = nil; return }
        let previous = accountSignInStatus
        accountSignInBusy = true
        let generation = accountStatusGeneration
        do {
            let (client,connection,selected) = try accountScope()
            let intent = try storage.accountSignInIntent(for:connection); accountSignInIntent = intent
            if let intent, intent.provider != accountProvider { accountProvider = intent.provider }
            else if intent == nil, let provider = accountProvider, !runtimes.contains(where:{ $0.provider == provider }) { accountProvider = nil }
            let result = try await client.accountSignInStatus(provider:accountProvider)
            try acceptAccountStatus(result,connection:connection,installation:selected,intent:intent,generation:generation)
        } catch { accountSignInStatus = nil; modelCatalog = .unchecked; accountSignInMessage = "Sign-in status is unconfirmed. The saved request is kept for recovery." }
        accountSignInBusy = false
        if accountSignInStatus?.accountSelected == true && (previous?.accountSelected != true || previous?.operation != accountSignInStatus?.operation) { await refreshModels() }
    }
    public func selectReportedAccount() async {
        guard !accountSignInBusy, !lifecycleBusy, !isBusy, !hostStopped, !loadingModels,
              let status = accountSignInStatus, let operation = status.operation, let account = status.reportedAccount, !status.accountSelected else { return }
        accountSignInBusy = true
        let generation = accountStatusGeneration
        do {
            let (client,connection,selected) = try accountScope()
            guard try storage.accountSignInIntent(for:connection) == nil else { throw ClientError.invalidResponse }
            let result = try await client.selectAccount(operation,account:account,recoveryEpoch:selected.recoveryEpoch,provider:accountProvider)
            guard result.operation == operation, result.reportedAccount == account, result.accountSelected else { throw ClientError.invalidResponse }
            try acceptAccountStatus(result,connection:connection,installation:selected,intent:nil,generation:generation)
        } catch { accountSignInStatus = nil; accountSignInMessage = "Account selection is unconfirmed. Check status before selecting again." }
        accountSignInBusy = false
        if accountSignInStatus?.accountSelected == true { await refreshModels() }
    }
    public func startAccountSignIn() async {
        guard !accountSignInBusy, !lifecycleBusy, !isBusy, !hostStopped, !loadingModels else { return }
        let previous = accountSignInStatus
        accountSignInBusy = true
        let generation = accountStatusGeneration
        do {
            let (client,connection,selected) = try accountScope()
            guard try storage.accountSignInIntent(for:connection) == nil else { throw ClientError.invalidResponse }
            let current = try await client.accountSignInStatus(provider:accountProvider)
            guard generation == accountStatusGeneration, try accountScopeMatches(connection,selected), !current.isActive else { throw ClientError.invalidResponse }
            if case .unavailable = current { throw ClientError.invalidResponse }
            let request = AccountSignInRequest(operationId:UUID().uuidString,expectedRevision:current.revision,recoveryEpoch:selected.recoveryEpoch,expiresAt:Int64(Date().timeIntervalSince1970 * 1000) + 110_000)
            let intent = AccountSignInIntent(format:1,connection:connection,request:request,provider:accountProvider)
            try storage.prepareAccountSignIn(intent); accountSignInIntent = intent
            let result = try await client.startAccountSignIn(request,provider:accountProvider)
            try acceptAccountStatus(result,connection:connection,installation:selected,intent:intent,generation:generation)
        } catch { accountSignInStatus = nil; accountSignInMessage = "Sign-in was not confirmed. Check status before retrying the saved request." }
        accountSignInBusy = false
        if accountSignInStatus?.accountSelected == true && (previous?.accountSelected != true || previous?.operation != accountSignInStatus?.operation) { await refreshModels() }
    }
    public func retryAccountSignIn() async {
        guard !accountSignInBusy, !lifecycleBusy, !isBusy, !hostStopped, !loadingModels else { return }
        let previous = accountSignInStatus
        accountSignInBusy = true
        let generation = accountStatusGeneration
        do {
            let (client,connection,selected) = try accountScope()
            guard let intent = try storage.accountSignInIntent(for:connection), intent.request.recoveryEpoch == selected.recoveryEpoch,
                  intent.request.expiresAt > Int64(Date().timeIntervalSince1970 * 1000) else { throw ClientError.invalidResponse }
            accountSignInIntent = intent; accountProvider = intent.provider
            let result = try await client.startAccountSignIn(intent.request,provider:intent.provider)
            try acceptAccountStatus(result,connection:connection,installation:selected,intent:intent,generation:generation)
        } catch { accountSignInStatus = nil; accountSignInMessage = "The exact saved sign-in request could not be confirmed. Its deadline has not been extended." }
        accountSignInBusy = false
        if accountSignInStatus?.accountSelected == true && (previous?.accountSelected != true || previous?.operation != accountSignInStatus?.operation) { await refreshModels() }
    }
    public func cancelAccountSignIn() async {
        guard !accountSignInBusy, !lifecycleBusy else { return }
        accountSignInBusy = true; defer { accountSignInBusy = false }
        let generation = accountStatusGeneration
        do {
            let (client,connection,selected) = try accountScope()
            var intent = try storage.accountSignInIntent(for:connection)
            let current = try await client.accountSignInStatus(provider:accountProvider)
            guard generation == accountStatusGeneration, try accountScopeMatches(connection,selected), let operation = current.operation else { throw ClientError.invalidResponse }
            if let intent { guard intent.request.recoveryEpoch == selected.recoveryEpoch, intent.request.matches(operation) else { throw ClientError.invalidResponse } }
            else {
                let adopted = AccountSignInIntent(format:1,connection:connection,request:AccountSignInRequest(operationId:operation.operationID,expectedRevision:operation.revision - 1,recoveryEpoch:selected.recoveryEpoch,expiresAt:operation.expiresAt),provider:accountProvider)
                try storage.prepareAccountSignIn(adopted); intent = adopted
            }
            accountSignInIntent = intent; accountSignInStatus = nil
            let result = try await client.cancelAccountSignIn(operation,recoveryEpoch:selected.recoveryEpoch,provider:accountProvider)
            try acceptAccountStatus(result,connection:connection,installation:selected,intent:intent,generation:generation)
        } catch { accountSignInStatus = nil; accountSignInMessage = "Cancellation is unconfirmed. Check status or retry cancellation; account access has not been revoked." }
    }
    public func discardExpiredAccountSignIn() async {
        guard !accountSignInBusy, !lifecycleBusy else { return }
        accountSignInBusy = true; defer { accountSignInBusy = false }
        do {
            let (client,connection,selected) = try accountScope()
            guard let intent = try storage.accountSignInIntent(for:connection), intent.request.recoveryEpoch == selected.recoveryEpoch,
                  intent.request.expiresAt + 5000 <= Int64(Date().timeIntervalSince1970 * 1000) else { throw ClientError.invalidResponse }
            let current = try await client.accountSignInStatus(provider:accountProvider)
            guard try accountScopeMatches(connection,selected), !current.isActive, try storage.accountSignInIntent(for:connection) == intent else { throw ClientError.invalidResponse }
            try storage.clearAccountSignIn(intent); accountSignInIntent = nil; accountSignInStatus = current
            accountSignInMessage = "The expired local request was discarded. No login or account revocation was performed."
        } catch { accountSignInStatus = nil; accountSignInMessage = "The request is kept until its deadline and cleanup period pass and the host confirms no active sign-in." }
    }
    public var companionAddress = ""
    public var companionInstallationID = ""
    public var companionInvitationID = ""
    public var companionInvitationCode = ""
    public var companionDeviceName = "iPhone"
    public private(set) var pendingCompanionSetup: PendingCompanionSetup?
    private var companionSetup: CompanionSetupCoordinator?
    private var companionSetupReady = false
    public var canSubmitCompanionInvitation: Bool {
        guard canPrepareCompanion, companionSetupReady, !companionSetupBusy, !lifecycleBusy, !isBusy, client == nil,
              !companionInstallationID.isEmpty, !companionInvitationID.isEmpty,
              !companionDeviceName.trimmingCharacters(in:.whitespacesAndNewlines).isEmpty,
              validCompanionToken(companionInvitationCode), let url = URL(string:companionAddress), url.scheme == "https" else { return false }
        return (try? DeviceEndpoint.network(baseURL:url,fixtureLoopback:false).validate()) != nil
    }
    public private(set) var companionSetupBusy = false
    public private(set) var companionSetupMessage = "Use an invitation from your exact Fort Mac host."
    public var canPrepareCompanion: Bool { companionSetup != nil && !isOfflinePreview && !isInteractiveFixture }
    private func loadCompanionSetup() throws {
        companionSetupReady = false
        guard let companionSetup else { throw ClientError.invalidResponse }
        pendingCompanionSetup = try companionSetup.pendingSetup()
        if let pending = pendingCompanionSetup {
            guard case .network(let address,_) = pending.connection.endpoint else { throw ClientError.invalidResponse }
            companionAddress = address.absoluteString; companionInstallationID = pending.connection.installationID
            companionInvitationID = pending.invitationID; companionDeviceName = pending.deviceName
            companionInvitationCode = ""
            companionSetupReady = pending.removalState == nil
            companionSetupMessage = pending.removalState == nil ? "A prepared phone request is unconfirmed. Recover it or retry the exact invitation." : "Phone removal is pending. Retry its exact removal or explicitly forget only its local credential."
        } else {
            let removal = try storage.pairingRemoval()
            companionSetupReady = try storage.connection() == nil && (removal == nil || removal?.state == .confirmed)
            if removal != nil && removal?.state != .confirmed {
                companionSetupMessage = "Phone revocation remains unconfirmed. Resolve that phone on its owning Mac before another pairing."
            }
        }
    }
    public func inspectCompanionSetup() {
        guard canPrepareCompanion, !companionSetupBusy else { return }
        do { try loadCompanionSetup() }
        catch { companionSetupMessage = "The saved phone setup could not be read safely. Its records have been kept." }
    }
    private func adoptPreparedCompanion(_ connection: DeviceConnection) async throws {
        guard connection.role == .companion, try storage.connection() == connection else { throw OwnerLifecycleError.wrongPairing }
        try loadCompanionSetup(); guard pendingCompanionSetup == nil else { throw ClientError.invalidResponse }
        clearConversationPresentation(); pairingStatus = nil; runtimeReadiness = nil; client = nil
        await restore()
        guard try storage.connection() == connection else {
            client = nil; connected = false; clearConversationPresentation(); throw OwnerLifecycleError.wrongPairing
        }
        companionSetupMessage = connected ? "Phone pairing is confirmed." : "Phone pairing was confirmed. The host is currently unavailable; its saved connection is kept."
    }
    /// Fills the phone setup fields from a Mac invitation; sends nothing.
    @discardableResult public func applyCompanionInvitation(_ text: String) -> Bool {
        guard pendingCompanionSetup == nil, let link = try? CompanionInvitationLink(text:text) else {
            companionSetupMessage = "That isn't a Fort invitation. Copy it again from Phone access on your Mac."; return false
        }
        companionAddress = link.address.absoluteString; companionInstallationID = link.installationID
        companionInvitationID = link.invitationID; companionInvitationCode = link.code
        return true
    }
    public func connectCompanion() async {
        guard canPrepareCompanion, !companionSetupBusy, !lifecycleBusy, !isBusy, client == nil, let companionSetup else { return }
        companionSetupBusy = true; defer { companionSetupBusy = false }
        let address = companionAddress, installation = companionInstallationID, invitation = companionInvitationID,
            code = companionInvitationCode, name = companionDeviceName
        companionInvitationCode = ""
        do {
            guard let url = URL(string:address) else { throw ClientError.invalidResponse }
            let connection = try await companionSetup.connect(address:url,installationID:installation,invitationID:invitation,code:code,deviceName:name)
            try await adoptPreparedCompanion(connection)
        } catch {
            try? loadCompanionSetup()
            companionSetupMessage = "Phone pairing is unconfirmed. Check the exact invitation or recover the saved request; no replacement credential was generated."
        }
    }
    public func disconnectPendingCompanion(_ expected: PendingCompanionSetup) async {
        guard canPrepareCompanion, !companionSetupBusy, !lifecycleBusy, !isBusy, client == nil, let companionSetup else { return }
        companionSetupBusy = true; defer { companionSetupBusy = false }; companionInvitationCode = ""
        do {
            guard try companionSetup.pendingSetup() == expected else { throw OwnerSetupPreparationError.changedPreparation }
            try await companionSetup.disconnectPending(); try loadCompanionSetup()
            companionSetupMessage = "Pending phone setup was canceled and its local credential removed. History is kept."
        } catch {
            try? loadCompanionSetup()
            companionSetupMessage = "Phone removal is unconfirmed. The prepared request is kept for exact retry or explicit local forgetting."
        }
    }
    public func forgetPendingCompanion(_ expected: PendingCompanionSetup) {
        guard canPrepareCompanion, !companionSetupBusy, !lifecycleBusy, !isBusy, client == nil, let companionSetup else { return }
        companionSetupBusy = true; defer { companionSetupBusy = false }; companionInvitationCode = ""
        do {
            guard try companionSetup.pendingSetup() == expected else { throw OwnerSetupPreparationError.changedPreparation }
            try companionSetup.forgetPending(); try loadCompanionSetup()
            companionSetupMessage = expected.removalState == .confirmed ? "Confirmed phone removal is complete locally. History is kept." : "Only the local phone credential was forgotten. Host revocation remains unconfirmed; resolve that phone on its Mac before another pairing."
        } catch {
            try? loadCompanionSetup()
            companionSetupMessage = "The selected phone preparation changed or cleanup failed. Its remaining records are kept."
        }
    }
    public func recoverCompanionSetup() async {
        guard canPrepareCompanion, !companionSetupBusy, !lifecycleBusy, !isBusy, client == nil, let companionSetup else { return }
        companionSetupBusy = true; defer { companionSetupBusy = false }
        companionInvitationCode = ""
        do { try await adoptPreparedCompanion(companionSetup.recover()) }
        catch {
            try? loadCompanionSetup()
            companionSetupMessage = "Phone pairing is still unconfirmed. The prepared credential and request are kept; no new join was sent."
        }
    }
    public var pairingStatus: PairingStatus?
    #if os(macOS)
    public internal(set) var phoneAccessBusy = false
    public internal(set) var phoneAccessMessage = "Import this Mac's Tailscale HTTPS certificate to let your phone connect."
    public internal(set) var phoneServerName: String?
    public internal(set) var phoneListener: MacNetworkListener?
    public internal(set) var phonePairing: CompanionPairingState?
    public internal(set) var phoneInvitation: CompanionInvitationLink?
    #endif
    public var pairingMessage = "Owner setup has not been activated."
    public var lifecycleBusy = false
    private var pairingRemoved = false
    private var hostStopped = false
    private var companionRemoval = false
    public var isCompanionPairing: Bool { pairingStatus?.role == .companion || companionRemoval }
    public var pairingCleanupPending = false
    public var pairingRemovalPending = false
    public var localPairingForgotten = false
    private var ownedInstallation: (any OwnedMacInstallation)?
    public var removableAppURL: URL? { ownedInstallation?.appURL }
    public var canDeleteLocalHistory: Bool { ownedInstallation?.supportsHistoryDeletion == true }
    public var canDeleteSavedHistory: Bool {
        #if os(macOS)
        guard !ownerSetupBusy else { return false }
        #endif
        return !isOfflinePreview && !isInteractiveFixture && !isBusy && !lifecycleBusy && !companionSetupBusy && pendingCompanionSetup == nil &&
        !connected && client == nil && pairingRemoved && !pairingCleanupPending && !pairingRemovalPending && !expectedInstallation.isEmpty
    }
    public var savedHistoryDeletionTarget: DeviceConnection? {
        guard canDeleteSavedHistory else { return nil }
        return try? storage.disconnectedConnection()
    }
    public func deleteSavedHistory(_ expected: DeviceConnection) {
        guard canDeleteSavedHistory else { return }
        lifecycleBusy = true
        defer { lifecycleBusy = false }
        do {
            guard let connection = try storage.disconnectedConnection(), connection == expected, connection.installationID == expectedInstallation else { throw OwnerLifecycleError.wrongPairing }
            try storage.deleteSavedHistory(for:connection)
            clearConversationPresentation()
            status = "Saved history deleted on this device"
            pairingMessage = "Saved conversations, pending submissions and image previews for this installation were deleted on this device. The host ledger and other devices are unchanged. Recovery records are kept."
        } catch { pairingMessage = "Saved-history deletion is incomplete. The displayed history is kept; check this installation and retry." }
    }
    private func clearConversationPresentation() {
        showLatestMessages(); snapshot = nil; installation = nil
        pendingStore = nil; pending = []; pendingErrors = [:]; localImages = []; loadedImageScope = nil; imageError = nil
        agents = []; conversations = []; skills = []; routines = []; schedules = []; weeks = []
        selectedAgentID = nil; selectedConversationID = nil; recipientAgentID = nil; selectedSkillIDs = []
        lastSavedSkill = nil; draft = ""; eventCursor = nil
    }
    public private(set) var preparedAppRemoval: PreparedAppRemoval?
    public var canManagePairing: Bool { !isOfflinePreview && !pairingRemoved && client != nil && installation?.supports("ownerPairing") == true }
    public var canEditPairingExpiry: Bool { canManagePairing && connected && pairingStatus != nil && !lifecycleBusy && !isBusy }
    public var canDisconnectPairing: Bool {
        !isOfflinePreview && !pairingRemoved && client != nil &&
        (isCompanionPairing || canManagePairing && (installation?.supports("companionPairing") != true || pairingStatus != nil))
    }
    public var canControlHost: Bool { canDisconnectPairing && !isCompanionPairing }
    public var canResumeLocalWork: Bool { canControlHost && runtimeReadiness?.state == "stopped" && runtimeReadiness?.canRestart == true && !isBusy && !lifecycleBusy }
    public var runtimeReadiness: RuntimeReadiness?
    public var schedulerStatus: SchedulerStatus?
    private var eventCursor: Int?
    public var status = "Connect to your fixture installation."
    public var isBusy = false
    public var connected = false
    public var showConnection = true
    private let storage: DeviceStorage
    private let session: URLSession
    private var connectionTransport: ((DeviceEndpoint) throws -> any FortTransport)?
    #if os(macOS)
    public var retainedHostHistoryDeletionTarget: MacHistoryDeletionRequest? {
        guard canDeleteSavedHistory, let removal = ownedInstallation as? MacHostRemoval,
              removal.connection.installationID == expectedInstallation else { return nil }
        return try? removal.historyDeletionRequest(recoveryEpoch:installation?.recoveryEpoch)
    }
    public var canDeleteRetainedHostHistory: Bool { retainedHostHistoryDeletionTarget != nil }
    public func deleteRetainedHostHistory(_ expected: MacHistoryDeletionRequest) async {
        guard let target = retainedHostHistoryDeletionTarget, let removal = ownedInstallation as? MacHostRemoval,
              expected.installationID == target.installationID, expected.deviceID == target.deviceID,
              expected.recoveryEpoch == target.recoveryEpoch else { return }
        lifecycleBusy = true; isBusy = true
        defer { lifecycleBusy = false; isBusy = false }
        do {
            guard try storage.disconnectedConnection() == removal.connection else { throw OwnerLifecycleError.wrongPairing }
            _ = try await removal.deleteRetainedHistory(expected)
            clearConversationPresentation()
            status = "Host history and this Mac's saved copy deleted"
            pairingMessage = "Host conversations and this Mac's saved copy were deleted. Owner notes, settings, other devices and provider history are kept."
        } catch {
            pairingMessage = "History deletion is unconfirmed. Retry this saved operation to finish. Any deletion that already completed cannot be undone."
        }
    }
    private var ownerSetupCoordinator: OwnerSetupCoordinator?
    private var ownerSetupEndpoint: DeviceEndpoint?
    private var ownerSetupInspected = false
    public private(set) var localHost: MacHostControl?
    private var localHostSetupReady = false
    private var reinstallConnection: DeviceConnection?
    public private(set) var pendingOwnerSetup: PendingOwnerSetup?
    public private(set) var ownerSetupBusy = false
    public private(set) var ownerSetupMessage = "Checking saved setup…"
    public var hasOwnerSetup: Bool { ownerSetupCoordinator != nil && !isOfflinePreview && !isInteractiveFixture }
    public var canCompleteOwnerSetup: Bool {
        hasOwnerSetup && ownerSetupInspected && !ownerSetupBusy && !isBusy && !lifecycleBusy && client == nil && !connected &&
        pendingOwnerSetup?.phase != .canceling && pendingOwnerSetup?.phase != .canceled &&
        (localHost == nil || (localHostSetupReady && localHost?.canPair == true))
    }
    public var canCancelOwnerSetup: Bool {
        hasOwnerSetup && ownerSetupInspected && pendingOwnerSetup != nil && !ownerSetupBusy && !isBusy && !lifecycleBusy && client == nil && !connected &&
        (localHost == nil || (localHostSetupReady && (localHost?.canPair == true || pendingOwnerSetup?.phase == .canceled)))
    }
    public var canInspectLocalHost: Bool { localHost != nil && localHost?.isBusy == false && !ownerSetupBusy && !isBusy && !lifecycleBusy }
    public var canEnableLocalHost: Bool { canInspectLocalHost && localHostSetupReady && localHost?.canEnable == true }
    public var canDisableUnpairedLocalHost: Bool { canInspectLocalHost && localHostSetupReady && localHost?.canDisableUnpaired == true && client == nil && !connected && pendingOwnerSetup == nil && reinstallConnection == nil }
    public var hasCompletedHostRemoval: Bool { reinstallConnection != nil }
    public var canPrepareLocalReinstall: Bool { canInspectLocalHost && reinstallConnection != nil && localHost?.canEnable == true && client == nil && !connected }
    public func prepareLocalReinstall() async {
        guard canPrepareLocalReinstall, let localHost, let connection = reinstallConnection else { return }
        lifecycleBusy = true; defer { lifecycleBusy = false }
        do {
            let removal = try localHost.removal(connection:connection,storage:storage)
            try await removal.prepareForSetup {
                guard try self.storage.ownerSetupCredential(for:connection) == nil else { throw MacHostRemovalError.setupBlocked }
            }
            preparedAppRemoval = nil
            configureLocalHostSetup()
            if localHostSetupReady { ownerSetupMessage = "Ready to set up again with any retained conversations. Completed history deletion cannot be undone. Enable the local host and connect this Mac separately." }
        } catch {
            ownerSetupMessage = "Setup remains blocked. Confirm that removal and local credential cleanup finished, then check this app's host identity. Completed history deletion cannot be undone."
        }
    }
    public convenience init(directory: URL, localHost: MacHostControl) throws {
        try self.init(directory:directory)
        self.localHost = localHost
    }
    private func verifyLocalHostRecords(_ files: OwnerSetupFiles, endpoint: MacAgentEndpoint) throws {
        guard try files.read(.hostRemoval) == nil else { throw MacHostRemovalError.setupBlocked }
        for record in [OwnerSetupFiles.Record.connection,.disconnected] {
            if let bytes = try files.read(record) {
                guard try JSONDecoder().decode(DeviceConnection.self,from:bytes).endpoint == .macAgent(endpoint) else { throw OwnerSetupPreparationError.wrongHost }
            }
        }
    }
    private func configureLocalHostSetup() {
        localHostSetupReady = false; ownerSetupInspected = false; ownedInstallation = nil; reinstallConnection = nil
        guard let localHost, let endpoint = localHost.endpoint else {
            ownerSetupMessage = localHost?.message ?? "No local host is configured."; return
        }
        do {
            do {
                let files = try OwnerSetupFiles(directory:storage.directory); defer { files.close() }
                if let bytes = try files.read(.connection) ?? files.read(.disconnected) {
                    ownedInstallation = try localHost.removal(connection:JSONDecoder().decode(DeviceConnection.self,from:bytes),storage:storage)
                }
                reinstallConnection = try MacHostRemoval.completedConnection(files,endpoint:endpoint)
                try verifyLocalHostRecords(files,endpoint:endpoint)
            }
            if let ownerSetupEndpoint, ownerSetupEndpoint != .macAgent(endpoint) { throw OwnerSetupPreparationError.wrongHost }
            if ownerSetupCoordinator == nil {
                ownerSetupCoordinator = try OwnerSetupCoordinator(storage:storage,agent:endpoint)
                ownerSetupEndpoint = .macAgent(endpoint)
            }
            localHostSetupReady = true
            inspectOwnerSetup()
        } catch {
            ownerSetupMessage = reinstallConnection != nil
                ? "Removal was recorded as complete. Check host status to set up again with any retained conversations."
                : "Saved host state prevents setup or startup changes. Finish any saved removal and check this installation's identity. Records and credentials have been kept."
        }
    }
    public func inspectLocalHost() async {
        guard canInspectLocalHost, let localHost else { return }
        await localHost.inspect()
        configureLocalHostSetup()
    }
    public func disableUnpairedLocalHost() async {
        guard canDisableUnpairedLocalHost, let localHost else { return }
        lifecycleBusy = true; defer { lifecycleBusy = false }
        do {
            try await localHost.disableUnpaired(storageDirectory:storage.directory)
            configureLocalHostSetup()
            ownerSetupMessage = "Startup registration removed. No pairing or host data existed."
        } catch {
            ownerSetupMessage = "Startup removal could not be confirmed. Existing setup, credentials and host data have been kept."
        }
        status = ownerSetupMessage
    }
    public func enableLocalHost() async {
        guard canEnableLocalHost, let localHost, let endpoint = localHost.endpoint else { return }
        lifecycleBusy = true; defer { lifecycleBusy = false }
        do {
            do {
                let files = try OwnerSetupFiles(directory:storage.directory); defer { files.close() }
                try verifyLocalHostRecords(files,endpoint:endpoint)
                await localHost.enable()
                try verifyLocalHostRecords(files,endpoint:endpoint)
            }
            configureLocalHostSetup()
        } catch {
            localHostSetupReady = false; ownerSetupInspected = false
            ownerSetupMessage = "Startup could not be confirmed while saved host state is unresolved. Existing records and credentials have been kept."
        }
    }
    public convenience init(directory: URL, ownerServiceIdentifier: String, signingTeam: String) throws {
        try self.init(directory:directory)
        ownerSetupCoordinator = try OwnerSetupCoordinator(storage:storage,serviceIdentifier:ownerServiceIdentifier,signingTeam:signingTeam)
        ownerSetupEndpoint = .macHost(serviceIdentifier:ownerServiceIdentifier,signingTeam:signingTeam)
    }
    public convenience init(directory: URL, ownerAgent: MacAgentEndpoint) throws {
        try self.init(directory:directory)
        ownerSetupCoordinator = try OwnerSetupCoordinator(storage:storage,agent:ownerAgent)
        ownerSetupEndpoint = .macAgent(ownerAgent)
    }
    convenience init(storage: DeviceStorage, session: URLSession, ownerEndpoint: DeviceEndpoint, ownerSetupControl: any OwnerSetupControl, connectionTransport: @escaping (DeviceEndpoint) throws -> any FortTransport, localHost: MacHostControl? = nil) throws {
        self.init(storage:storage,session:session)
        self.connectionTransport = connectionTransport
        ownerSetupCoordinator = try OwnerSetupCoordinator(storage:storage,endpoint:ownerEndpoint,control:ownerSetupControl,tokenGenerator:DeviceStorage.makeOwnerDeviceToken)
        ownerSetupEndpoint = ownerEndpoint
        self.localHost = localHost
    }
    public func inspectOwnerSetup() {
        guard hasOwnerSetup, !ownerSetupBusy, let ownerSetupCoordinator else { return }
        guard localHost == nil || localHostSetupReady else { return }
        do {
            pendingOwnerSetup = try ownerSetupCoordinator.pendingSetup()
            ownerSetupInspected = true
            switch pendingOwnerSetup?.phase {
            case .preparing: ownerSetupMessage = "Setup was interrupted before its credential was ready. Continue or cancel this setup."
            case .ready: ownerSetupMessage = "Setup has not been confirmed. Retry the saved setup or cancel it."
            case .canceling: ownerSetupMessage = "Cancellation is saved. The host has not confirmed it yet."
            case .canceled: ownerSetupMessage = "Cancellation was confirmed. Finish removing the prepared credential from this device."
            case nil: ownerSetupMessage = "Connect this Mac to its configured Fort host. Pairing has no default expiry."
            }
        } catch {
            ownerSetupInspected = false
            ownerSetupMessage = "Saved setup could not be read safely. Its records and credential have been kept."
        }
    }
    public func completeOwnerSetup() async {
        guard canCompleteOwnerSetup else { return }
        if localHost != nil { await inspectLocalHost() }
        guard canCompleteOwnerSetup, let ownerSetupCoordinator else { return }
        ownerSetupBusy = true
        var completed = false
        var failure = "Setup could not be confirmed. Review the saved setup before retrying or canceling."
        do { _ = try await ownerSetupCoordinator.connect(); completed = true }
        catch let error as OwnerSetupPreparationError { failure = error.localizedDescription }
        catch {}
        ownerSetupBusy = false
        inspectOwnerSetup()
        if completed {
            preparedAppRemoval = nil
            await restore()
        } else {
            ownerSetupMessage = failure
            status = ownerSetupMessage
        }
    }
    public func cancelOwnerSetup(_ expected: OwnerSetupIdentity) async {
        guard canCancelOwnerSetup else { return }
        if localHost != nil { await inspectLocalHost() }
        guard canCancelOwnerSetup, let ownerSetupCoordinator else { return }
        guard pendingOwnerSetup?.identity == expected else {
            inspectOwnerSetup(); ownerSetupMessage = "Saved setup changed. Review it before canceling."; return
        }
        ownerSetupBusy = true
        var finished = false
        do { _ = try await ownerSetupCoordinator.cancelPendingSetup(expected); finished = true }
        catch {}
        ownerSetupBusy = false
        inspectOwnerSetup()
        if finished {
            await restore()
            ownerSetupMessage = "No unfinished setup remains. Saved conversations have been kept."
            if installation == nil { status = ownerSetupMessage }
        } else {
            ownerSetupMessage += " Recovery could not finish. Its saved state has been kept."
            status = ownerSetupMessage
        }
    }
    #endif
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
    #if DEBUG
    public convenience init(fixtureDirectory: URL) throws {
        guard fixtureDirectory.deletingLastPathComponent().lastPathComponent == "UITests",
              UUID(uuidString:fixtureDirectory.lastPathComponent) != nil else { throw ClientError.invalidResponse }
        self.init(storage:try DeviceStorage(directory:fixtureDirectory),session:.shared)
    }
    #endif
    public init(directory: URL) throws {
        storage = try DeviceStorage(directory:directory); session = .shared; isOfflinePreview = false
        #if os(iOS)
        companionSetup = CompanionSetupCoordinator(storage:storage)
        #endif
    }
    init(storage: DeviceStorage, session: URLSession, isOfflinePreview: Bool = false, ownedInstallation: (any OwnedMacInstallation)? = nil,
         companionSetup: CompanionSetupCoordinator? = nil, connectionTransport: ((DeviceEndpoint) throws -> any FortTransport)? = nil) {
        self.storage = storage; self.session = session; self.isOfflinePreview = isOfflinePreview; self.ownedInstallation = ownedInstallation
        self.companionSetup = companionSetup; self.connectionTransport = connectionTransport
    }
    private func restoredClient(_ connection: DeviceConnection) throws -> FortClient {
        let transport = try connectionTransport?(connection.endpoint) ?? connection.endpoint.transport(session:session)
        return FortClient(installationID:connection.installationID,token:try storage.token(for:connection),transport:transport)
    }
    public func restore() async {
        guard !isOfflinePreview, !isInteractiveFixture, client == nil else { return }
        if canPrepareCompanion {
            do {
                try loadCompanionSetup()
                if pendingCompanionSetup != nil { showConnection = true; return }
            } catch {
                companionSetupMessage = "The saved phone setup could not be read safely. Its records have been kept."
                showConnection = true; return
            }
        }
        #if os(macOS)
        if localHost != nil {
            await inspectLocalHost()
            guard localHostSetupReady, localHost?.canPair == true else {
                showConnection = true; status = ownerSetupMessage; return
            }
        }
        if hasOwnerSetup {
            guard !ownerSetupBusy else { return }
            inspectOwnerSetup()
            guard ownerSetupInspected, pendingOwnerSetup == nil else {
                showConnection = true; status = ownerSetupMessage; return
            }
        }
        #endif
        do {
            let active = try storage.connection()
            guard let connection = try active ?? storage.disconnectedConnection() else { return }
            #if os(macOS)
            if hasOwnerSetup, connection.endpoint != ownerSetupEndpoint {
                status = "The saved connection belongs to a different host. Its records and credential have been kept."
                return
            }
            #endif
            let disconnected = try storage.disconnectedConnection()
            let intent = try storage.pairingRemoval()
            let removal = intent?.connection == connection ? intent : nil
            companionRemoval = connection.role == .companion || removal?.scope == .companion
            pairingRemoved = active == nil || active == disconnected || removal != nil
            pairingRemovalPending = removal?.state == .requested || removal?.state == .locallyForgotten
            localPairingForgotten = removal?.state == .locallyForgotten && active == nil
            pairingCleanupPending = active != nil && (removal?.state == .confirmed || (removal == nil && active == disconnected))
            endpoint = connection.endpoint.displayName; expectedInstallation = connection.installationID
            pendingStore = try PendingStore(directory:storage.directory,installationID:connection.installationID)
            pending = try pendingStore?.pending() ?? []
            let savedNavigation = try storage.navigation(installationID:connection.installationID)
            if let saved = savedNavigation {
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
                loadLocalImages(); status = savedNavigation == nil ? "Disconnected" : "Disconnected · saved history"
                pairingMessage = pairingRemovalPending ? unconfirmedRemovalMessage : pairingCleanupPending ? "Pairing was revoked; local credential removal still needs to finish." : savedNavigation == nil ? "Pairing removed. No saved conversation list is available on this device." : "Pairing removed. History is kept on this device."
                return
            }
            hostStopped = false
            client = try restoredClient(connection)
            loadLocalImages();await refresh()
        } catch ClientError.localHostUnavailable { status = ClientError.localHostUnavailable.localizedDescription }
        catch { status = "Could not restore this device connection. Saved pairing has been kept." }
    }
    public func enroll() async {
        #if os(macOS)
        guard !hasOwnerSetup else { return }
        #endif
        guard !isOfflinePreview, !isInteractiveFixture, !isBusy, let url = URL(string:endpoint) else { return }
        isBusy = true; defer { isBusy = false }
        do {
            let initial = try FortClient(baseURL:url,installationID:expectedInstallation,fixtureLoopback:true,session:session)
            let enrolled = try await initial.enroll(code:enrollmentCode,deviceName:deviceName)
            let connection = DeviceConnection(baseURL:url,installationID:expectedInstallation,deviceID:enrolled.deviceId,fixtureLoopback:true)
            try storage.save(connection,token:enrolled.deviceToken)
            pairingRemoved = false; hostStopped = false; companionRemoval = false; pairingCleanupPending = false; pairingRemovalPending = false; localPairingForgotten = false; pairingStatus = nil
            preparedAppRemoval = nil
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
            guard let connection = try storage.connection(), pairing.deviceId == connection.deviceID, pairing.mode == .ownerLocal,
                  connection.role == nil || pairing.role == connection.role else { throw OwnerLifecycleError.wrongPairing }
            pairingStatus = pairing; if isCompanionPairing { accountSignInStatus = nil }
            pairingMessage = "Pairing lasts until you disconnect unless you choose an expiry."
        } catch {
            pairingStatus = nil; accountSignInStatus = nil
            pairingMessage = isCompanionPairing ? "Phone access could not be verified. You can still disconnect this exact phone." : error.localizedDescription
        }
    }
    public func setPairingExpiry(days: Int?) async {
        guard canEditPairingExpiry, let client else { return }
        lifecycleBusy = true; defer { lifecycleBusy = false }
        do {
            let pairing = try await client.setPairingExpiry(days:days)
            guard let connection = try storage.connection(), pairing.deviceId == connection.deviceID, pairing.mode == .ownerLocal else { throw OwnerLifecycleError.wrongPairing }
            pairingStatus = pairing; pairingMessage = "Pairing expiry saved."
        } catch { pairingMessage = error.localizedDescription }
    }
    private var unconfirmedRemovalMessage: String {
        isCompanionPairing ? "Phone disconnect is unconfirmed. Retry with the saved credential, or revoke this phone on the owning Mac. History is kept." : "Disconnect intent saved. Service revocation and shutdown are unconfirmed. Retry recovery or explicitly forget only this device's credential."
    }
    private func completePairingRemoval(_ connection: DeviceConnection, companion: Bool = false) throws {
        let active = try storage.connection()
        guard try active == connection || active == nil && storage.disconnectedConnection() == connection else { throw OwnerLifecycleError.wrongPairing }
        if companion {
            let intent = try storage.pairingRemoval()
            guard intent?.connection == connection, intent?.scope == .companion else { throw OwnerLifecycleError.wrongPairing }
        }
        if !companion { try storage.retireAccountSignInAfterConfirmedDisconnect(connection); accountSignInIntent = nil }
        accountSignInStatus = nil; client = nil; connected = false; hostStopped = !companion; companionRemoval = companion; pairingRemoved = true; pairingStatus = nil
        pairingCleanupPending = true; pairingRemovalPending = false
        try storage.removePairing(connection)
        pairingCleanupPending = false; localPairingForgotten = false
        pairingMessage = companion ? "This phone’s access was revoked and its local credential removed. The host and saved history are kept." : "Disconnected. Runtime shutdown and revocation were confirmed; the local credential was removed. History is kept."
        status = pairingMessage
    }
    public func stopLocalHost(disconnect: Bool) async {
        guard canDisconnectPairing, disconnect || canControlHost, !isBusy, !lifecycleBusy, let client else { return }
        let companion = isCompanionPairing
        accountStatusGeneration = UUID(); accountSignInStatus = nil
        lifecycleBusy = true; isBusy = true
        defer { lifecycleBusy = false; isBusy = false }
        do {
            guard let connection = try storage.connection() else { throw OwnerLifecycleError.wrongPairing }
            if disconnect {
                try storage.beginPairingRemoval(connection,scope:companion ? .companion : nil)
                companionRemoval = companion
                pairingRemovalPending = true; pairingRemoved = true
            }
            let removal = disconnect ? try storage.pairingRemoval() : nil
            if companion { _ = try await client.leaveCompanion(deviceID:connection.deviceID) }
            else { _ = try await client.stopHost(disconnect:disconnect) }
            if disconnect {
                guard try storage.pairingRemoval() == removal else { throw OwnerLifecycleError.wrongPairing }
                try completePairingRemoval(connection,companion:companion)
            }
            else {
                hostStopped = true
                pairingMessage = "Local work stopped. Pairing, history and owner controls remain available. Resume explicitly when ready."
                status = pairingMessage
                if installation?.supports("runtimeRecovery") == true { runtimeReadiness = try await client.runtimeReadiness() }
            }
        } catch {
            if disconnect && pairingRemovalPending {
                self.client = nil; connected = false
                pairingMessage = pairingCleanupPending ? "Revocation confirmed; local credential removal is incomplete." : unconfirmedRemovalMessage
            } else { pairingMessage = error.localizedDescription }
        }
    }
    public func retryPairingRemoval() async {
        guard pairingRemovalPending, !isBusy, !lifecycleBusy, !isInteractiveFixture else { return }
        lifecycleBusy = true; isBusy = true
        defer { lifecycleBusy = false; isBusy = false }
        do {
            guard let intent = try storage.pairingRemoval() else { throw OwnerLifecycleError.wrongPairing }
            if intent.scope == .companion {
                companionRemoval = true
                guard intent.state != .locallyForgotten, try storage.connection() == intent.connection else { throw OwnerLifecycleError.unavailable }
                _ = try await restoredClient(intent.connection).leaveCompanion(deviceID:intent.connection.deviceID)
            } else if let ownedInstallation {
                let proof = try await ownedInstallation.recoverPairingRemoval(intent.connection)
                guard proof.connection == intent.connection, proof.hostStopped, proof.pairingRevoked else { throw OwnerLifecycleError.stopUnconfirmed }
            } else {
                guard intent.state != .locallyForgotten, try storage.connection() == intent.connection else { throw OwnerLifecycleError.unavailable }
                let recovery = try restoredClient(intent.connection)
                _ = try await recovery.stopHost(disconnect:true)
            }
            guard try storage.pairingRemoval() == intent else { throw OwnerLifecycleError.wrongPairing }
            try completePairingRemoval(intent.connection,companion:intent.scope == .companion)
        } catch { pairingMessage = pairingCleanupPending ? "Revocation confirmed; local credential removal is incomplete." : unconfirmedRemovalMessage }
    }
    public func forgetLocalPairing() {
        guard pairingRemovalPending, !lifecycleBusy else { return }
        do {
            guard let intent = try storage.pairingRemoval() else { throw OwnerLifecycleError.wrongPairing }
            try storage.removePairing(intent.connection,revocationConfirmed:false)
            accountSignInStatus = nil; client = nil; connected = false; pairingRemoved = true; localPairingForgotten = true
            pairingMessage = isCompanionPairing ? "Local credential removed only. Phone revocation is unconfirmed; revoke this phone on the owning Mac. History is kept." : "Local credential removed only. Service revocation and shutdown are unconfirmed. History is kept."
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
    public func prepareForRemoval(history: HistoryRemoval = .keep) async {
        guard !isOfflinePreview, !isBusy, !lifecycleBusy, let ownedInstallation else { return }
        accountStatusGeneration = UUID(); accountSignInStatus = nil
        lifecycleBusy = true; isBusy = true
        preparedAppRemoval = nil
        defer { lifecycleBusy = false; isBusy = false }
        do {
            guard let connection = try storage.connection() ?? storage.disconnectedConnection() else { throw OwnerLifecycleError.wrongPairing }
            let removal = OwnerLifecycle(installation:ownedInstallation,removeCredential:{ self.client = nil; self.connected = false; self.pairingRemoved = true; self.pairingCleanupPending = true; try self.storage.removePairing(connection); self.pairingCleanupPending = false })
            preparedAppRemoval = try await removal.prepareForRemoval(history:history)
            try storage.retireAccountSignInAfterConfirmedDisconnect(connection); accountSignInIntent = nil
            accountSignInStatus = nil; client = nil; connected = false; pairingRemoved = true
            let historyMessage = history == .keep ? "History is kept." : "The confirmed local history was deleted."
            pairingMessage = "Host shutdown, revocation and startup unregistration are confirmed. Local pairing and Fort’s saved provider sign-in were removed. \(historyMessage) Provider-side grants and history are unchanged. Remove the Fort app using TestFlight or macOS."
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
    public func waitForChanges() async {
        guard !Task.isCancelled else { return }
        guard !isBusy else { try? await Task.sleep(for:.milliseconds(250)); return }
        guard let client, let current = installation else { try? await Task.sleep(for:.seconds(1)); return }
        if !current.supports("eventReplay") {
            do { try await Task.sleep(for:.seconds(30)); await refresh() } catch {}
            return
        }
        guard connected, let cursor = eventCursor else {
            await refresh()
            if !connected { try? await Task.sleep(for:.seconds(5)) }
            return
        }
        do {
            let page = try await client.events(after:cursor,recoveryEpoch:current.recoveryEpoch,waitMilliseconds:25_000)
            try Task.checkCancellation()
            guard installation?.installationId == current.installationId, installation?.recoveryEpoch == current.recoveryEpoch else { return }
            if !page.events.isEmpty { await refresh() }
        } catch is CancellationError {
        } catch {
            guard !Task.isCancelled else { return }
            if case ClientError.service(let code,_) = error, ["cursorExpired","recoveryEpochChanged"].contains(code) {
                eventCursor = nil; await refresh()
            } else {
                connected = false; status = error.localizedDescription
                try? await Task.sleep(for:.seconds(5))
            }
        }
    }
    private func synchronize() async {
        guard let client else { return }
        do {
            let current = try await client.installation()
            installation = current
            if current.supports("companionPairing") { await refreshPairing() }
            await refreshAccountSignIn()
            if current.supports("eventReplay"), current.eventCursor == nil || current.eventCursor! < 0 { throw ClientError.invalidResponse }
            runtimeReadiness = current.supports("runtimeRecovery") ? try await client.runtimeReadiness() : nil
            if current.supports("runtimeRecovery") { hostStopped = runtimeReadiness?.state == "stopped" }
            schedulerStatus = current.supports("schedulerStatus") ? try await client.schedulerStatus() : nil
            agents = try await client.agents()
            if !agents.contains(where:{$0.agentId == selectedAgentID}) { selectedAgentID = agents.first?.agentId }
            if hasWorkspace {
                conversations = try await client.conversations(); skills = try await client.skills()
                routines = current.supports("routines") ? try await client.routines() : []
                schedules = current.supports("schedules") ? try await client.schedules() : []
                weeks = current.supports("weeks") ? try await client.weeks() : []
                if !conversations.contains(where:{$0.id == selectedConversationID}) { selectedConversationID = selectedAgent?.homeConversationId }
            } else { conversations = []; skills = []; weeks = []; selectedConversationID = nil }
            try saveNavigation()
            pending = try pendingStore?.pending() ?? []
            pendingErrors = [:]
            if supportsCommands {
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
            eventCursor = current.eventCursor
            loadLocalImages();connected = true
            status = !current.supportsCommands ? "This Fort version is read-only. Update the app to send or change work." : pending.contains(where:{$0.cancelRequested == true}) ? "Cancellation saved on this device · confirmation pending" : pending.contains(where:{!$0.canRetry(in:current.recoveryEpoch)}) ? "Pending work needs review and will not resend." : pendingErrors.isEmpty ? (current.mode == "fixture" ? "Connected · fixture only" : "Connected · local prototype") : "Pending work needs review · history is up to date"
            if isInteractiveFixture && pendingErrors.isEmpty { status = "Interactive QA · simulated replies · no network or credentials" }
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
            guard cancellation.receipt == nil, cancellation.run == nil, cancellation.snapshot == nil, cancellation.message == nil else { throw ClientError.invalidResponse }
        case .accepted:
            guard let receipt = cancellation.receipt, let run = cancellation.run, let snapshot = cancellation.snapshot,
                  run.runId == receipt.runId, run.requestId == item.requestID, run.conversationId == item.conversationID,
                  run.bindingRevisionId == item.bindingRevision, ["cancelRequested","canceled","completed","failed","outcomeUnknown","closedUnknown"].contains(run.state),
                  item.matches(Acceptance(receipt:receipt,snapshot:snapshot,message:cancellation.message,run:run)) else { throw ClientError.invalidResponse }
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
        if try storage.snapshot(installationID:expectedInstallation,conversationID:conversationID) != accepted { try storage.saveSnapshot(accepted) }
        if activeConversationID == conversationID { snapshot = accepted }
    }
    public func send() async {
        guard canSend, let agent = recipient, let conversationID = activeConversationID, let installation, let pendingStore else { return }
        showLatestMessages()
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
        showLatestMessages()
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
        guard !loadingModels, !accountSignInBusy else { return }
        guard installation?.supports("modelCatalog") == true, let client else { modelCatalog = .unchecked; return }
        loadingModels = true; defer { loadingModels = false }
        do {
            let catalog = try await client.models(provider:accountProvider)
            guard catalog.provider == (focusedRuntime?.provider ?? runtimeProvider), ["ready","unavailable","error"].contains(catalog.state), catalog.models.count <= 800, Set(catalog.models.map(\.id)).count == catalog.models.count else { throw ClientError.invalidResponse }
            modelCatalog = catalog
        } catch { modelCatalog = ModelCatalog(provider:focusedRuntime?.provider ?? runtimeProvider ?? "",state:"error",models:[],detail:"Could not check models. Your saved selection has not changed.") }
    }
    public func saveAgent(_ agent: Agent?, name: String, model: String, instructions: String, avatar: AgentAvatar? = nil, provider: String? = nil) async -> Bool {
        guard let client = await beginWorkspaceSave() else { return false }
        do {
            let saved = try await client.saveAgent(agent,name:name,model:model,instructions:instructions,avatar:avatar,provider:provider)
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
    /// Closes the live week, if any, and founds the planned one. A failure after the close leaves no live week; the draft stays on screen to retry.
    public func foundWeek(_ draft: PlanDraft) async -> Bool {
        guard supportsWeeks, draft.canFound, let client = await beginWorkspaceSave() else { return false }
        defer { workspaceSaving = false }
        do {
            if let live = liveWeek { _ = try await client.closeWeek(live) }
            _ = try await client.foundWeek(draft.request)
            isBusy = false; await refresh(); return true
        } catch { status = error.localizedDescription; isBusy = false; await refresh(); return false }
    }
    /// Your own quests build as planned the moment you tick them; there is no agent to confirm.
    public func completeQuest(_ item: WeekPlanItem) async -> Bool {
        guard supportsWeeks, item.owner == .you, item.doneBuildId == nil, let week = liveWeek, week.plan.contains(where:{ $0.itemId == item.itemId }), let client = await beginWorkspaceSave() else { return false }
        defer { workspaceSaving = false }
        do { _ = try await client.placeBuild(item,in:week,size:item.size); isBusy = false; await refresh(); return true }
        catch { status = error.localizedDescription; isBusy = false; await refresh(); return false }
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
        if installation?.supports("ownerPairing") == true && !canControlHost { return }
        let resumingOwner = canResumeLocalWork
        guard installation?.supports("runtimeRecovery") == true, runtimeReadiness?.state == "restartRequired" || resumingOwner, runtimeReadiness?.canRestart == true, !isBusy, let client, let installation else { return }
        isBusy = true
        var failure: String?
        do { runtimeReadiness = try await client.restartRuntime(recoveryEpoch:installation.recoveryEpoch) }
        catch { failure = error.localizedDescription; status = error.localizedDescription }
        isBusy = false; await refresh()
        if resumingOwner {
            pairingMessage = !hostStopped && runtimeReadiness?.state == "ready" ? "Local work resumed. Pairing and history are kept." : failure ?? "Local work remains stopped. Review runtime status before retrying."
            status = pairingMessage
        }
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

#if os(macOS)
extension MessengerModel {
    public var canManagePhoneAccess: Bool { canControlHost && !isOfflinePreview && !isInteractiveFixture && client != nil }
    public var phoneListenerActive: Bool { if case .active = phoneListener { true } else { false } }
    private var phoneAccess: MacPhoneAccess { MacPhoneAccess(directory:storage.directory) }
    private func phoneScope() throws -> (FortClient, DeviceConnection, any MacNetworkControl) {
        guard canManagePhoneAccess, let client, let connection = try storage.connection(), pairingStatus?.deviceId == connection.deviceID,
              let control = try (connectionTransport?(connection.endpoint) ?? connection.endpoint.transport(session:session)) as? any MacNetworkControl else { throw OwnerLifecycleError.wrongPairing }
        return (client,connection,control)
    }
    private func phoneOwner(_ connection: DeviceConnection) -> MacNetworkOwner {
        MacNetworkOwner(installationId:connection.installationID,deviceId:connection.deviceID)
    }
    public func importPhoneIdentity(_ urls: [URL]) async {
        let keys = urls.filter { $0.pathExtension.lowercased() == "key" || $0.lastPathComponent.lowercased().contains("key") }
        let certificates = urls.filter { !keys.contains($0) }
        guard keys.count == 1, certificates.count == 1 else { phoneAccessMessage = "Choose both files from `tailscale cert`: the .crt certificate and the .key private key."; return }
        await importPhoneIdentity(certificate:certificates[0],privateKey:keys[0])
    }
    public func importPhoneIdentity(certificate: URL, privateKey: URL) async {
        guard canManagePhoneAccess, !phoneAccessBusy else { return }
        phoneAccessBusy = true; defer { phoneAccessBusy = false }
        func read(_ url: URL) throws -> Data {
            let scoped = url.startAccessingSecurityScopedResource(); defer { if scoped { url.stopAccessingSecurityScopedResource() } }
            return try Data(contentsOf:url)
        }
        do {
            try phoneAccess.save(certificate:try read(certificate),privateKey:try read(privateKey))
            phoneAccessMessage = "Certificate imported."
            await refreshPhoneAccess()
        } catch { phoneAccessMessage = "That certificate and key could not be used. Choose the .crt and .key files from `tailscale cert`." }
    }
    public func refreshPhoneAccess() async {
        guard canManagePhoneAccess else { return }
        do {
            phoneServerName = try phoneAccess.identity()?.serverName
            let (client,connection,control) = try phoneScope()
            phoneListener = try await control.controlNetwork(try MacNetworkRequest(command:.inspect(owner:phoneOwner(connection)))).listener
            phonePairing = try await client.companionOverview(hostDeviceID:connection.deviceID).pairing
        } catch { phoneAccessMessage = "Phone access status could not be confirmed." }
    }
    public func startPhoneListener() async {
        guard canManagePhoneAccess, !phoneAccessBusy else { return }
        phoneAccessBusy = true; defer { phoneAccessBusy = false }
        do {
            guard let identity = try phoneAccess.identity() else { phoneAccessMessage = "Import the Tailscale certificate first."; return }
            guard let address = MacPhoneAccess.currentTailscaleAddress() else { phoneAccessMessage = "Tailscale isn't connected on this Mac. Open Tailscale, then try again."; return }
            let (_,connection,control) = try phoneScope(), owner = phoneOwner(connection)
            let current = try await control.controlNetwork(try MacNetworkRequest(command:.inspect(owner:owner)))
            let configuration = MacNetworkConfiguration(bindAddress:address,prefixLength:MacPhoneAccess.prefixLength,port:MacPhoneAccess.port,
                serverName:identity.serverName,certificate:identity.certificate,privateKey:identity.privateKey)
            let status = try await control.controlNetwork(try MacNetworkRequest(command:.start(owner:owner,sessionId:current.sessionId,generation:current.generation,configuration:configuration)))
            phoneListener = status.listener
            phoneAccessMessage = phoneListenerActive ? "Your phone can reach this Mac at https://\(identity.serverName):\(MacPhoneAccess.port) over Tailscale." : "The phone listener didn't start. Check the certificate hasn't expired and Tailscale is connected."
        } catch { phoneAccessMessage = "The phone listener couldn't be started or confirmed." }
    }
    public func stopPhoneListener() async {
        guard canManagePhoneAccess, !phoneAccessBusy else { return }
        phoneAccessBusy = true; defer { phoneAccessBusy = false }
        do {
            let (_,connection,control) = try phoneScope(), owner = phoneOwner(connection)
            let current = try await control.controlNetwork(try MacNetworkRequest(command:.inspect(owner:owner)))
            phoneListener = try await control.controlNetwork(try MacNetworkRequest(command:.stop(owner:owner,sessionId:current.sessionId,generation:current.generation))).listener
            phoneInvitation = nil; phoneAccessMessage = "Phone listener stopped. A paired phone keeps its access for when you start it again."
        } catch { phoneAccessMessage = "Stopping the phone listener couldn't be confirmed." }
    }
    public func invitePhone() async {
        guard canManagePhoneAccess, !phoneAccessBusy, phoneListenerActive, let serverName = phoneServerName,
              let address = URL(string:"https://\(serverName):\(MacPhoneAccess.port)") else { return }
        phoneAccessBusy = true; defer { phoneAccessBusy = false }
        do {
            let (client,connection,_) = try phoneScope(), fresh = MacPhoneAccess.newInvitation()
            let receipt = try await client.inviteCompanion(invitationID:fresh.id,code:fresh.code)
            phoneInvitation = try CompanionInvitationLink(address:address,installationID:connection.installationID,invitationID:receipt.invitationId,code:fresh.code)
            phonePairing = .invited(invitationID:receipt.invitationId,expiresAt:receipt.expiresAt)
            phoneAccessMessage = "Copy the invitation, then paste it on your phone under Connect this phone."
        } catch { phoneAccessMessage = "The invitation couldn't be created. A phone may already be paired or invited." }
    }
    public func revokePhone(deviceID: String) async {
        guard canManagePhoneAccess, !phoneAccessBusy else { return }
        phoneAccessBusy = true; defer { phoneAccessBusy = false }
        do {
            let (client,_,_) = try phoneScope()
            _ = try await client.revokeCompanion(deviceID:deviceID)
            phoneInvitation = nil; phoneAccessMessage = "The phone's access was revoked."
            await refreshPhoneAccess()
        } catch { phoneAccessMessage = "Revoking the phone couldn't be confirmed." }
    }
}
#endif
