#if os(macOS)
import Foundation
import Observation

@Observable @MainActor public final class MacHostControl {
    public let applicationURL: URL
    public private(set) var status: MacHostStartupStatus = .unknown
    public private(set) var isBusy = false
    public private(set) var endpoint: MacAgentEndpoint?
    private var identity: MacHostBundle?
    private var failure: String?
    private let inspectBundle: @MainActor (URL) async throws -> MacHostBundle
    private let makeStartup: @MainActor (MacAgentEndpoint) throws -> MacHostStartup
    public var canEnable: Bool { !isBusy && endpoint != nil && (status == .notRegistered || status == .notFound) }
    public var canPair: Bool { !isBusy && endpoint != nil && status == .enabled }
    public var message: String {
        if let failure { return failure }
        switch status {
        case .unknown: return "Check this app's local host before connecting."
        case .notRegistered: return "The local host is not enabled. Enable it to let macOS start it for this app."
        case .enabled: return "Local host startup is enabled. Connect this Mac separately to verify pairing and runtime access."
        case .requiresApproval: return "macOS requires approval. Allow Fort in System Settings, then check its status again."
        case .notFound: return "macOS has no startup record for this verified local host. Enable it to register this app’s host."
        case .unsupported: return "This macOS startup state is not supported. No startup change was confirmed."
        }
    }
    public convenience init() {
        self.init(applicationURL:Bundle.main.bundleURL,inspectBundle:MacHostBundle.inspect,
            makeStartup:{ try MacHostStartup(endpoint:$0) })
    }
    init(applicationURL: URL, inspectBundle: @escaping @MainActor (URL) async throws -> MacHostBundle,
         makeStartup: @escaping @MainActor (MacAgentEndpoint) throws -> MacHostStartup) {
        self.applicationURL = applicationURL; self.inspectBundle = inspectBundle; self.makeStartup = makeStartup
    }
    func removal(connection: DeviceConnection, storage: DeviceStorage) throws -> MacHostRemoval {
        guard let identity, endpoint == identity.endpoint, connection.endpoint == .macAgent(identity.endpoint) else { throw OwnerLifecycleError.wrongPairing }
        let startup = try makeStartup(identity.endpoint)
        let history = MacHistoryRemovalActions(verifyCredentialAbsent:{
            guard try storage.ownerSetupCredential(for:connection) == nil else { throw MacHostRemovalError.setupBlocked }
        },maintain:{ request in
            let program = try await Task.detached {
                try MacHostProgram.inspect(helperURL:identity.helperURL,signature:MacHostBundle.codeSignature)
            }.value
            guard program.bundle.applicationURL == identity.applicationURL, program.bundle.helperURL == identity.helperURL,
                  program.bundle.endpoint == identity.endpoint, program.bundle.clientIdentifier == identity.clientIdentifier else { throw MacHostBundleError.configuration }
            return try await program.deleteHistory(request,storage:MacAgentStorage.inspectExisting(for:identity.endpoint))
        },removeSavedHistory:{ try storage.deleteSavedHistory(for:connection) })
        return try MacHostRemoval(connection:connection,directory:storage.directory,applicationURL:identity.applicationURL,verifyBundle:{
            let observed = try await self.inspectBundle(identity.applicationURL)
            guard observed.applicationURL == identity.applicationURL, observed.helperURL == identity.helperURL,
                  observed.endpoint == identity.endpoint, observed.clientIdentifier == identity.clientIdentifier else { throw MacHostBundleError.configuration }
        },startup:startup,recover:MacXPCTransport(agent:identity.endpoint).recoverPairingAndObserveHost,exitTimeout:3,removeProviderCredential:{
            try MacAgentStorage.inspectExisting(for:identity.endpoint).removeCodexCredential()
        },historyActions:history)
    }
    public func inspect() async { await perform(enable:false) }
    public func enable() async {
        guard canEnable else { return }
        await perform(enable:true)
    }
    private func perform(enable: Bool) async {
        guard !isBusy else { return }
        isBusy = true; status = .unknown; endpoint = nil; failure = nil
        defer { isBusy = false }
        do {
            try Task.checkCancellation()
            let bundle = try await inspectBundle(applicationURL)
            try Task.checkCancellation()
            guard bundle.applicationURL == applicationURL else { throw MacHostBundleError.configuration }
            if let identity {
                guard bundle.endpoint == identity.endpoint, bundle.clientIdentifier == identity.clientIdentifier,
                      bundle.helperURL == identity.helperURL else { throw MacHostBundleError.configuration }
            }
            identity = bundle
            let startup = try makeStartup(bundle.endpoint)
            guard startup.endpoint == bundle.endpoint else { throw MacHostBundleError.configuration }
            let observed = try await (enable ? startup.enable() : startup.inspect())
            try Task.checkCancellation()
            endpoint = bundle.endpoint; status = observed
        } catch let error as MacHostBundleError {
            failure = error.localizedDescription
        } catch is CancellationError {
            failure = "The host check was canceled. Check its status before another action."
        } catch {
            failure = "Local host status could not be confirmed. Check the installed app and macOS startup permissions before retrying."
        }
    }
}
#endif
