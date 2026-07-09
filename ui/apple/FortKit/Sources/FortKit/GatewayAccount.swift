//
//  GatewayAccount.swift
//  FortKit
//
//  Where the client points itself: a gateway base URL (the 028 tunnel proxy or
//  a local/mesh host) plus the machine the user has selected in the sidebar.
//
//  v1 boundary (honest): in-app Google sign-in for the 028 gateway is a DEFERRED
//  follow-on — it needs the deployed gateway and an ASWebAuthenticationSession
//  OAuth flow. For now this just lets the app aim `FortClient` at a base URL;
//  local + mesh machines work fully without any auth.
//

import Foundation

/// Persisted pointer for the client: an optional gateway base URL and the
/// selected machine id. Stored as a single JSON blob in `UserDefaults`.
public struct GatewayAccount: Codable, Sendable, Hashable {

    /// The gateway/base URL to point `FortClient` at, when set (else local default).
    public var gatewayURL: URL?
    /// The machine id chosen in the sidebar, when set.
    public var selectedMachineID: String?

    public init(gatewayURL: URL? = nil, selectedMachineID: String? = nil) {
        self.gatewayURL = gatewayURL
        self.selectedMachineID = selectedMachineID
    }

    enum CodingKeys: String, CodingKey {
        case gatewayURL = "gateway_url"
        case selectedMachineID = "selected_machine_id"
    }

    /// The `UserDefaults` key the account is stored under.
    public static let defaultsKey = "fort.gatewayAccount"

    /// Loads the saved account, or an empty one when nothing is stored / it fails to decode.
    public static func load(from defaults: UserDefaults = .standard) -> GatewayAccount {
        guard let data = defaults.data(forKey: defaultsKey),
              let account = try? JSONDecoder().decode(GatewayAccount.self, from: data)
        else {
            return GatewayAccount()
        }
        return account
    }

    /// Persists this account as JSON in `UserDefaults`.
    public func save(to defaults: UserDefaults = .standard) {
        guard let data = try? JSONEncoder().encode(self) else { return }
        defaults.set(data, forKey: Self.defaultsKey)
    }
}
