import Foundation
import Security
import CryptoKit

// Google sign-in for a cloud hub that puts everything behind Google (Spec 066). The app signs in
// through Google's iOS OAuth client (no secret, PKCE), hands the ID token to the hub, and keeps the
// hub's session value in the Keychain. Every request to that hub then carries it as X-Fort-Session.
public enum HubSignInError: Error, LocalizedError, Equatable {
    case notOffered, canceled, refused(String), unreadable
    public var errorDescription: String? {
        switch self {
        case .notOffered: "This hub doesn’t offer Google sign-in for the app."
        case .canceled: "Google sign-in was canceled."
        case .refused(let message): message
        case .unreadable: "Google sign-in returned something Fort couldn’t read. Try again."
        }
    }
}

// One Google authorization request: the URL to open, and what's needed to finish it.
struct GoogleAuthorization: Equatable {
    let url: URL
    let callbackScheme: String
    let redirectURI: String
    let clientID: String
    let verifier: String
    let state: String

    static let endpoint = URL(string:"https://accounts.google.com/o/oauth2/v2/auth")!
    static let tokenEndpoint = URL(string:"https://oauth2.googleapis.com/token")!

    // Google's iOS clients redirect to the client ID reversed as a URL scheme.
    static func callbackScheme(clientID: String) throws -> String {
        let suffix = ".apps.googleusercontent.com"
        guard clientID.hasSuffix(suffix), clientID.count > suffix.count,
              clientID.allSatisfy({ $0.isASCII && ($0.isLetter || $0.isNumber || "-._".contains($0)) }) else { throw HubSignInError.unreadable }
        return "com.googleusercontent.apps." + clientID.dropLast(suffix.count)
    }

    init(clientID: String, nonce: String, verifier: String = GoogleAuthorization.random(), state: String = GoogleAuthorization.random()) throws {
        let scheme = try Self.callbackScheme(clientID:clientID)
        let redirect = scheme + ":/oauth2redirect"
        let challenge = Data(SHA256.hash(data:Data(verifier.utf8))).base64URL
        var components = URLComponents(url:Self.endpoint,resolvingAgainstBaseURL:false)!
        components.queryItems = [
            .init(name:"client_id",value:clientID), .init(name:"redirect_uri",value:redirect), .init(name:"response_type",value:"code"),
            .init(name:"scope",value:"openid email"), .init(name:"code_challenge",value:challenge), .init(name:"code_challenge_method",value:"S256"),
            .init(name:"nonce",value:nonce), .init(name:"state",value:state), .init(name:"prompt",value:"select_account"),
        ]
        guard let url = components.url else { throw HubSignInError.unreadable }
        self.url = url; callbackScheme = scheme; redirectURI = redirect; self.clientID = clientID; self.verifier = verifier; self.state = state
    }

    // The authorization code from Google's redirect, if it answers this request.
    func code(from callback: URL) throws -> String {
        guard let components = URLComponents(url:callback,resolvingAgainstBaseURL:false), components.scheme == callbackScheme else { throw HubSignInError.unreadable }
        let items = components.queryItems ?? []
        guard items.first(where:{ $0.name == "state" })?.value == state else { throw HubSignInError.unreadable }
        if items.contains(where:{ $0.name == "error" }) { throw HubSignInError.canceled }
        guard let code = items.first(where:{ $0.name == "code" })?.value, !code.isEmpty else { throw HubSignInError.unreadable }
        return code
    }

    // The token request that trades the code for Google's ID token.
    func tokenRequest(code: String) -> URLRequest {
        var request = URLRequest(url:Self.tokenEndpoint)
        request.httpMethod = "POST"
        request.setValue("application/x-www-form-urlencoded",forHTTPHeaderField:"Content-Type")
        var form = URLComponents()
        form.queryItems = [.init(name:"code",value:code), .init(name:"client_id",value:clientID), .init(name:"redirect_uri",value:redirectURI),
                           .init(name:"grant_type",value:"authorization_code"), .init(name:"code_verifier",value:verifier)]
        request.httpBody = Data((form.percentEncodedQuery ?? "").replacingOccurrences(of:"+",with:"%2B").utf8)
        return request
    }

    static func random() -> String {
        var bytes = [UInt8](repeating:0,count:32)
        _ = SecRandomCopyBytes(kSecRandomDefault,bytes.count,&bytes)
        return Data(bytes).base64URL
    }
}

extension Data {
    var base64URL: String { base64EncodedString().replacingOccurrences(of:"+",with:"-").replacingOccurrences(of:"/",with:"_").replacingOccurrences(of:"=",with:"") }
}

public enum HubGoogleSignIn {
    private struct Offer: Decodable { let google: Bool; let iosClientId: String?; let nonce: String? }
    private struct Tokens: Decodable { let id_token: String }
    private struct Granted: Decodable { let session: String }
    private struct Problem: Decodable { struct Detail: Decodable { let message: String }; let error: Detail }

    /// Signs this device in to the hub at `baseURL` with Google. `authenticate` opens Google's page and
    /// returns the redirect, for example SwiftUI's WebAuthenticationSession.
    @MainActor public static func signIn(baseURL: URL, session: URLSession = .shared,
                              authenticate: (_ url: URL, _ callbackScheme: String) async throws -> URL) async throws {
        guard baseURL.scheme == "https", let host = baseURL.host else { throw ClientError.insecureEndpoint }
        let (offerData,_) = try await session.data(from:baseURL.appendingPathComponent("auth/session"))
        guard let offer = try? JSONDecoder().decode(Offer.self,from:offerData), offer.google, let clientID = offer.iosClientId, let nonce = offer.nonce else { throw HubSignInError.notOffered }
        let authorization = try GoogleAuthorization(clientID:clientID,nonce:nonce)
        let callback: URL
        do { callback = try await authenticate(authorization.url,authorization.callbackScheme) } catch { throw HubSignInError.canceled }
        let (tokenData,tokenResponse) = try await session.data(for:authorization.tokenRequest(code:try authorization.code(from:callback)))
        guard (tokenResponse as? HTTPURLResponse)?.statusCode == 200, let tokens = try? JSONDecoder().decode(Tokens.self,from:tokenData) else { throw HubSignInError.unreadable }
        var request = URLRequest(url:baseURL.appendingPathComponent("auth/google/native"))
        request.httpMethod = "POST"; request.setValue("application/json",forHTTPHeaderField:"Content-Type")
        request.httpBody = try JSONEncoder().encode(["credential":tokens.id_token])
        let (grantData,grantResponse) = try await session.data(for:request)
        guard (grantResponse as? HTTPURLResponse)?.statusCode == 200, let granted = try? JSONDecoder().decode(Granted.self,from:grantData) else {
            throw HubSignInError.refused((try? JSONDecoder().decode(Problem.self,from:grantData))?.error.message ?? "The hub refused this Google account.")
        }
        try HubSessions.save(granted.session,for:host)
    }
}
