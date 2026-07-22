//
//  GatewayAccount.swift
//  FortKit
//
//  Where the client points itself: a gateway base URL (the 028 tunnel proxy or
//  a local/mesh host) plus the machine the user has selected in the sidebar.
//
//  The iOS app obtains a short-lived bearer credential through Google sign-in.
//  The selected daemon key is pinned here before FortKit opens its Noise tunnel.
//

import Foundation

/// Persisted pointer for the client: an optional gateway base URL and the
/// selected machine id. Stored as a single JSON blob in `UserDefaults`.
public struct GatewayAccount: Codable, Sendable, Hashable {

    /// The gateway/base URL to point `FortClient` at, when set (else local default).
    public var gatewayURL: URL?
    /// The machine id chosen in the sidebar, when set.
    public var selectedMachineID: String?
    /// Short-lived native credential issued by the gateway after Google sign-in.
    public var bearerToken: String?
    /// TOFU pins, keyed by gateway machine id; values are base64 X25519 keys.
    public var pinnedPublicKeys: [String: String]

    public init(
        gatewayURL: URL? = nil,
        selectedMachineID: String? = nil,
        bearerToken: String? = nil,
        pinnedPublicKeys: [String: String] = [:]
    ) {
        self.gatewayURL = gatewayURL
        self.selectedMachineID = selectedMachineID
        self.bearerToken = bearerToken
        self.pinnedPublicKeys = pinnedPublicKeys
    }

    enum CodingKeys: String, CodingKey {
        case gatewayURL = "gateway_url"
        case selectedMachineID = "selected_machine_id"
        case bearerToken = "bearer_token"
        case pinnedPublicKeys = "pinned_public_keys"
    }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        gatewayURL = try values.decodeIfPresent(URL.self, forKey: .gatewayURL)
        selectedMachineID = try values.decodeIfPresent(String.self, forKey: .selectedMachineID)
        bearerToken = try values.decodeIfPresent(String.self, forKey: .bearerToken)
        pinnedPublicKeys = try values.decodeIfPresent([String: String].self, forKey: .pinnedPublicKeys) ?? [:]
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
