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
        guard let selected = account.selectedMachineID,
              account.pinnedPublicKeys[selected] != nil,
              let machine = machines.first(where: { $0.machineID == selected })
        else { return }
        connect(machine, client: client, trustIfNeeded: false)
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
                self.account.gatewayURL = normalized
                self.account.bearerToken = token
                self.account.selectedMachineID = nil
                self.account.save()
                #if DEBUG && targetEnvironment(simulator)
                self.directHostEnabled = false
                #endif
                await self.refreshMachines(client: client)
                if self.machines.count == 1, let machine = self.machines.first {
                    self.connect(machine, client: client, trustIfNeeded: false)
                }
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
            errorMessage = nil
        } catch let error as GatewayRelayError where error.statusCode == 401 {
            account.bearerToken = nil
            account.selectedMachineID = nil
            account.save()
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
            account.save()
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
        account.save()
        machines = []
        disconnectPrimaryTransport(client: client)
    }

    #if DEBUG && targetEnvironment(simulator)
    func useDirectHost(_ url: URL, client: FortClient) {
        account = GatewayAccount()
        account.save()
        machines = []
        connectedMachineID = nil
        directHostEnabled = true
        client.useDirectHost(url)
    }
    #endif

    func presentationAnchor(for session: ASWebAuthenticationSession) -> ASPresentationAnchor {
        UIApplication.shared.connectedScenes
            .compactMap { $0 as? UIWindowScene }
            .flatMap(\.windows)
            .first(where: \.isKeyWindow) ?? ASPresentationAnchor()
    }
}
