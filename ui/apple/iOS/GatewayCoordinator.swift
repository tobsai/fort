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
    @Published private(set) var directHostEnabled = false
    @Published private(set) var isLoading = false
    @Published var errorMessage: String?

    private var authenticationSession: ASWebAuthenticationSession?
    private var restored = false

    override init() {
        var saved = GatewayAccount.load()
        if saved.gatewayURL == nil,
           let configured = Bundle.main.object(forInfoDictionaryKey: "FortGatewayURL") as? String,
           !configured.isEmpty,
           !configured.contains("$(") {
            saved.gatewayURL = URL(string: configured)
        }
        account = saved
        super.init()
    }

    func restore(client: FortClient) async {
        guard !restored else { return }
        restored = true
        defer { restoreComplete = true }
        guard account.gatewayURL != nil, account.bearerToken != nil else { return }
        await refreshMachines()
        guard let selected = account.selectedMachineID,
              account.pinnedPublicKeys[selected] != nil,
              let machine = machines.first(where: { $0.machineID == selected })
        else { return }
        connect(machine, client: client, trustIfNeeded: false)
    }

    func signIn(gatewayURL: URL, client: FortClient) {
        var normalized = gatewayURL
        if normalized.path != "/" && !normalized.path.isEmpty {
            normalized.deleteLastPathComponent()
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
                self.account.gatewayURL = normalized
                self.account.bearerToken = token
                self.account.selectedMachineID = nil
                self.account.save()
                self.directHostEnabled = false
                await self.refreshMachines()
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

    func refreshMachines() async {
        guard let gatewayURL = account.gatewayURL, let token = account.bearerToken else { return }
        isLoading = true
        defer { isLoading = false }
        do {
            machines = try await GatewayService.machines(at: gatewayURL, bearerToken: token)
            errorMessage = nil
        } catch GatewayRelayError.httpStatus(401, _) {
            account.bearerToken = nil
            account.save()
            machines = []
            errorMessage = "Your gateway session expired. Sign in again."
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func connect(_ machine: GatewayMachine, client: FortClient, trustIfNeeded: Bool) {
        if account.pinnedPublicKeys[machine.machineID] == nil {
            guard trustIfNeeded else { return }
            account.pinnedPublicKeys[machine.machineID] = machine.publicKey
        }
        account.selectedMachineID = machine.machineID
        do {
            try client.useGateway(account: account, machine: machine)
            connectedMachineID = machine.machineID
            directHostEnabled = false
            account.save()
            errorMessage = nil
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func disconnect(client: FortClient) {
        account = GatewayAccount()
        account.save()
        machines = []
        connectedMachineID = nil
        directHostEnabled = false
        client.useDirectHost(URL(string: "http://127.0.0.1:4087")!)
    }

    func useDirectHost(_ url: URL, client: FortClient) {
        account = GatewayAccount()
        account.save()
        machines = []
        connectedMachineID = nil
        directHostEnabled = true
        client.useDirectHost(url)
    }

    func presentationAnchor(for session: ASWebAuthenticationSession) -> ASPresentationAnchor {
        UIApplication.shared.connectedScenes
            .compactMap { $0 as? UIWindowScene }
            .flatMap(\.windows)
            .first(where: \.isKeyWindow) ?? ASPresentationAnchor()
    }
}
