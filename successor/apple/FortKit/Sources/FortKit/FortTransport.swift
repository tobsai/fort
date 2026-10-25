import Foundation
import Security

public enum ClientError: Error, LocalizedError {
    case insecureEndpoint, invalidResponse, wrongInstallation, localHostUnavailable, service(String, String)
    public var errorDescription: String? {
        switch self {
        case .insecureEndpoint: "Use an HTTPS Fort address. Fixture HTTP is limited to this computer's loopback address."
        case .invalidResponse: "Fort returned an unreadable response."
        case .wrongInstallation: "This address belongs to a different Fort installation."
        case .localHostUnavailable: "This connection is available only on its Mac. Connect this device using the host’s secure network address."
        case .service(_, let message): message
        }
    }
}

public struct FortRequest: Codable, Sendable {
    public enum Method: String, Codable, Sendable { case get = "GET", post = "POST" }
    public let path: String
    public let method: Method
    public let installationID: String
    public let token: String?
    public let body: Data?
    public let timeout: TimeInterval
    func route() throws -> URLComponents {
        guard let route = URLComponents(string:path), route.scheme == nil, route.host == nil, route.fragment == nil,
              route.path.hasPrefix("/v1/"), !route.path.split(separator:"/").contains(where:{ $0 == "." || $0 == ".." }),
              timeout.isFinite, timeout > 0, timeout <= 30 else { throw ClientError.invalidResponse }
        return route
    }
}
public struct FortResponse: Codable, Sendable {
    public let status: Int
    public let body: Data
    public init(status: Int, body: Data) { self.status = status; self.body = body }
}
public protocol FortTransport: Sendable {
    func send(_ request: FortRequest) async throws -> FortResponse
}

private final class RefuseRedirects: NSObject, URLSessionTaskDelegate {
    static let shared = RefuseRedirects()
    func urlSession(_ session: URLSession, task: URLSessionTask, willPerformHTTPRedirection response: HTTPURLResponse, newRequest request: URLRequest, completionHandler: @escaping @Sendable (URLRequest?) -> Void) {
        completionHandler(nil)
    }
}

struct HTTPFortTransport: FortTransport {
    private let baseURL: URL
    private let session: URLSession
    init(baseURL: URL, fixtureLoopback: Bool, session: URLSession) throws {
        guard let host = baseURL.host, !host.isEmpty,
              baseURL.user == nil, baseURL.password == nil, baseURL.query == nil, baseURL.fragment == nil,
              baseURL.scheme == "https" || (fixtureLoopback && baseURL.scheme == "http" && ["127.0.0.1", "localhost", "::1"].contains(host)) else { throw ClientError.insecureEndpoint }
        self.baseURL = baseURL; self.session = session
    }
    func send(_ input: FortRequest) async throws -> FortResponse {
        let path = try input.route()
        guard var target = URLComponents(url:baseURL.appendingPathComponent(path.path),resolvingAgainstBaseURL:false) else { throw ClientError.invalidResponse }
        target.queryItems = path.queryItems
        guard let url = target.url else { throw ClientError.invalidResponse }
        var request = URLRequest(url:url)
        request.httpMethod = input.method.rawValue; request.httpBody = input.body; request.timeoutInterval = input.timeout
        request.setValue(input.installationID,forHTTPHeaderField:"X-Fort-Installation-Id")
        request.setValue("application/json",forHTTPHeaderField:"Content-Type")
        if let token = input.token { request.setValue("Bearer \(token)",forHTTPHeaderField:"Authorization") }
        // A hub behind Google sign-in (Spec 066) also wants this device's Google session.
        if baseURL.scheme == "https", let host = baseURL.host, let session = HubSessions.session(for:host) { request.setValue(session,forHTTPHeaderField:"X-Fort-Session") }
        let (data,response) = try await session.data(for:request,delegate:RefuseRedirects.shared)
        guard let response = response as? HTTPURLResponse else { throw ClientError.invalidResponse }
        return FortResponse(status:response.statusCode,body:data)
    }
}

// Google sessions for hubs behind Google sign-in (Spec 066), one per hub host, kept in the Keychain on this device only.
public enum HubSessions {
    private static let service = "Fort.HubSession"
    private static func query(_ host: String) -> [String:Any] {
        [kSecClass as String:kSecClassGenericPassword,kSecAttrService as String:service,kSecAttrAccount as String:host.lowercased()]
    }
    public static func session(for host: String) -> String? {
        var lookup = query(host); lookup[kSecReturnData as String] = true; lookup[kSecMatchLimit as String] = kSecMatchLimitOne
        var result: CFTypeRef?
        guard SecItemCopyMatching(lookup as CFDictionary,&result) == errSecSuccess, let data = result as? Data else { return nil }
        return String(data:data,encoding:.utf8)
    }
    public static func save(_ session: String, for host: String) throws {
        let data = Data(session.utf8)
        let status = SecItemUpdate(query(host) as CFDictionary,[kSecValueData as String:data] as CFDictionary)
        if status == errSecItemNotFound {
            var add = query(host); add[kSecValueData as String] = data; add[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
            let inserted = SecItemAdd(add as CFDictionary,nil)
            guard inserted == errSecSuccess else { throw ClientError.service("keychain","Fort couldn’t save the hub sign-in on this device.") }
        } else if status != errSecSuccess { throw ClientError.service("keychain","Fort couldn’t save the hub sign-in on this device.") }
    }
    public static func remove(for host: String) { _ = SecItemDelete(query(host) as CFDictionary) }
}
