import Foundation

public enum GatewayAddressError: Error, Sendable {
    case invalidURL
    case unsupportedScheme
    case credentialsNotAllowed
    case unsupportedPath
    case daemonRelayAddress
}

/// Canonicalizes the public HTTPS web-gateway origin used by native
/// authentication and relay requests. The Cloudflare worker address belongs
/// to the daemon and must never be entered here.
public enum GatewayAddress {
    public static func normalize(_ raw: String) throws -> URL {
        let trimmed = raw.trimmingCharacters(in: .whitespacesAndNewlines)
        guard let url = URL(string: trimmed) else { throw GatewayAddressError.invalidURL }
        return try normalize(url)
    }

    public static func normalize(_ url: URL) throws -> URL {
        guard var components = URLComponents(url: url, resolvingAgainstBaseURL: false),
              let scheme = components.scheme?.lowercased(),
              let host = components.host?.lowercased(),
              !host.isEmpty
        else { throw GatewayAddressError.invalidURL }
        guard scheme == "https" else {
            throw GatewayAddressError.unsupportedScheme
        }
        guard components.user == nil, components.password == nil else {
            throw GatewayAddressError.credentialsNotAllowed
        }
        guard host != "workers.dev", !host.hasSuffix(".workers.dev") else {
            throw GatewayAddressError.daemonRelayAddress
        }
        let path = components.path
        guard path.isEmpty || path == "/" || path == "/native" || path == "/native/" else {
            throw GatewayAddressError.unsupportedPath
        }
        components.scheme = scheme
        components.path = ""
        components.query = nil
        components.fragment = nil
        guard let normalized = components.url else { throw GatewayAddressError.invalidURL }
        return normalized
    }
}
