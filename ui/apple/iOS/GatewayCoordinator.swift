import AuthenticationServices
import FortKit
import SwiftUI
import UIKit

@MainActor
final class GatewayCoordinator: NSObject, ObservableObject, ASWebAuthenticationPresentationContextProviding {
    @Published private(set) var account: GatewayAccount
    @Published private(set) var machines: [GatewayMachine] = []
    @Published private(set) var connectedMachineID: String?
    @Published private(set) var restoreComplete = false
    @Published private(set) var isLoading = false
    @Published var errorMessage: String?

    #if DEBUG && targetEnvironment(simulator)
    @Published private(set) var directHostEnabled = false
    #endif

    private var authenticationSession: ASWebAuthenticationSession?
    private var restored = false

    var hasPrimaryTransport: Bool {
        if connectedMachineID != nil { return true }
        #if DEBUG && targetEnvironment(simulator)
        if directHostEnabled { return true }
        #endif
        return false
    }

    override init() {
        var saved = GatewayAccount.load()
        let legacyToken = saved.bearerToken
        if let securedToken = GatewaySessionTokenStore.load() {
            saved.bearerToken = securedToken
        }
        if let savedURL = saved.gatewayURL {
            saved.gatewayURL = try? GatewayAddress.normalize(savedURL)
        }
        if saved.gatewayURL == nil,
           let configured = Bundle.main.object(forInfoDictionaryKey: "FortGatewayURL") as? String,
           !configured.isEmpty,
           !configured.contains("$(") {
            saved.gatewayURL = try? GatewayAddress.normalize(configured)
        }
        account = saved
        super.init()
        if legacyToken != nil {
            persistAccount()
        }
    }

    func restore(client: FortClient) async {
        guard !restored else { return }
        restored = true
        defer { restoreComplete = true }
#if DEBUG && targetEnvironment(simulator)
        if account.bearerToken == nil {
            let configured = ProcessInfo.processInfo.environment["FORT_DIRECT_HOST_URL"]
                ?? "http://127.0.0.1:4087"
            if let directURL = URL(string: configured) {
                client.useDirectHost(directURL)
                directHostEnabled = true
                return
            }
        }
#endif
        guard account.gatewayURL != nil, account.bearerToken != nil else { return }
        await refreshMachines(client: client)
        connectPreferredMachine(client: client)
    }

    func signIn(gatewayURL: URL, client: FortClient) {
        let normalized: URL
        do {
            normalized = try GatewayAddress.normalize(gatewayURL)
        } catch {
            errorMessage = "Enter the public Fort gateway address, such as https://fort-gateway.vercel.app."
            return
        }
        let signInURL = normalized.appendingPathComponent("native")
        let session = ASWebAuthenticationSession(url: signInURL, callbackURLScheme: "fort") { [weak self] callback, error in
            Task { @MainActor in
                guard let self else { return }
                self.authenticationSession = nil
                if let error {
                    if (error as? ASWebAuthenticationSessionError)?.code != .canceledLogin {
                        self.errorMessage = error.localizedDescription
                    }
                    return
                }
                guard let callback,
                      let components = URLComponents(url: callback, resolvingAgainstBaseURL: false),
                      let token = components.queryItems?.first(where: { $0.name == "token" })?.value,
                      !token.isEmpty
                else {
                    self.errorMessage = "The gateway did not return a native session."
                    return
                }
                self.disconnectPrimaryTransport(client: client)
                if self.account.gatewayURL != normalized {
                    self.account.selectedMachineID = nil
                    self.account.pinnedPublicKeys = [:]
                }
                self.account.gatewayURL = normalized
                self.account.bearerToken = token
                self.persistAccount()
                #if DEBUG && targetEnvironment(simulator)
                self.directHostEnabled = false
                #endif
                await self.refreshMachines(client: client)
                self.connectPreferredMachine(client: client)
            }
        }
        session.presentationContextProvider = self
        session.prefersEphemeralWebBrowserSession = false
        authenticationSession = session
        if !session.start() { errorMessage = "Could not open Google sign-in." }
    }

    func refreshMachines(client: FortClient) async {
        guard let gatewayURL = account.gatewayURL, let token = account.bearerToken else { return }
        disconnectPrimaryTransport(client: client)
        isLoading = true
        defer { isLoading = false }
        do {
            machines = try await GatewayService.machines(at: gatewayURL, bearerToken: token)
            if let renewedToken = try? await GatewayService.renewSession(
                at: gatewayURL,
                bearerToken: token
            ) {
                account.bearerToken = renewedToken
                persistAccount()
            }
            errorMessage = nil
        } catch let error as GatewayRelayError where error.statusCode == 401 {
            account.bearerToken = nil
            persistAccount()
            machines = []
            disconnectPrimaryTransport(client: client)
            errorMessage = error.localizedDescription
        } catch {
            machines = []
            disconnectPrimaryTransport(client: client)
            errorMessage = error.localizedDescription
        }
    }

    func connect(_ machine: GatewayMachine, client: FortClient, trustIfNeeded: Bool) {
        if account.pinnedPublicKeys[machine.machineID] == nil {
            guard trustIfNeeded else { return }
            account.pinnedPublicKeys[machine.machineID] = machine.publicKey
        }
        disconnectPrimaryTransport(client: client)
        do {
            try client.useGateway(account: account, machine: machine)
            account.selectedMachineID = machine.machineID
            connectedMachineID = machine.machineID
            #if DEBUG && targetEnvironment(simulator)
            directHostEnabled = false
            #endif
            persistAccount()
            errorMessage = nil
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func disconnectPrimaryTransport(client: FortClient) {
        connectedMachineID = nil
        #if DEBUG && targetEnvironment(simulator)
        directHostEnabled = false
        #endif
        client.disconnectGateway()
    }

    func disconnect(client: FortClient) {
        account = GatewayAccount()
        persistAccount()
        machines = []
        disconnectPrimaryTransport(client: client)
    }

    #if DEBUG && targetEnvironment(simulator)
    func useDirectHost(_ url: URL, client: FortClient) {
        account = GatewayAccount()
        persistAccount()
        machines = []
        connectedMachineID = nil
        directHostEnabled = true
        client.useDirectHost(url)
    }
    #endif

    private func connectPreferredMachine(client: FortClient) {
        if let selected = account.selectedMachineID,
           account.pinnedPublicKeys[selected] != nil,
           let machine = machines.first(where: { $0.machineID == selected }) {
            connect(machine, client: client, trustIfNeeded: false)
            return
        }
        if machines.count == 1,
           let machine = machines.first,
           account.pinnedPublicKeys[machine.machineID] != nil {
            connect(machine, client: client, trustIfNeeded: false)
        }
    }

    private func persistAccount() {
        GatewaySessionTokenStore.save(account.bearerToken)
        var metadata = account
        metadata.bearerToken = nil
        metadata.save()
    }

    func presentationAnchor(for session: ASWebAuthenticationSession) -> ASPresentationAnchor {
        UIApplication.shared.connectedScenes
            .compactMap { $0 as? UIWindowScene }
            .flatMap(\.windows)
            .first(where: \.isKeyWindow) ?? ASPresentationAnchor()
    }
}
