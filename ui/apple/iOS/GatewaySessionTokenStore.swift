import Foundation
import Security

/// Device-only persistence for the renewable native gateway credential.
/// Non-secret gateway metadata remains in GatewayAccount/UserDefaults.
enum GatewaySessionTokenStore {
    private static let service = Bundle.main.bundleIdentifier ?? "io.mtree.fort"
    private static let account = "fort.gateway.native-session"

    static func load() -> String? {
        var query = baseQuery
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        var item: CFTypeRef?
        guard SecItemCopyMatching(query as CFDictionary, &item) == errSecSuccess,
              let data = item as? Data,
              let token = String(data: data, encoding: .utf8),
              !token.isEmpty
        else { return nil }
        return token
    }

    static func save(_ token: String?) {
        guard let token, !token.isEmpty, let data = token.data(using: .utf8) else {
            SecItemDelete(baseQuery as CFDictionary)
            return
        }
        let attributes: [String: Any] = [
            kSecValueData as String: data,
            kSecAttrAccessible as String: kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly,
        ]
        if SecItemUpdate(baseQuery as CFDictionary, attributes as CFDictionary) == errSecItemNotFound {
            var item = baseQuery
            attributes.forEach { item[$0.key] = $0.value }
            SecItemAdd(item as CFDictionary, nil)
        }
    }

    private static var baseQuery: [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
        ]
    }
}
