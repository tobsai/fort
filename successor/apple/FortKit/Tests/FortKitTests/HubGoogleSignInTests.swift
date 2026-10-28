import Foundation
import Testing
import CryptoKit
@testable import FortKit

// Spec 066: the app's Google request for a hub behind Google sign-in.
private let clientID = "1234-ios.apps.googleusercontent.com"

@Test func googleAuthorizationAsksForAnIDTokenWithPKCEAndTheHubsNonce() throws {
    let request = try GoogleAuthorization(clientID:clientID,nonce:"hub-nonce",verifier:"verifier",state:"state-1")
    #expect(request.callbackScheme == "com.googleusercontent.apps.1234-ios")
    #expect(request.redirectURI == "com.googleusercontent.apps.1234-ios:/oauth2redirect")
    let items = Dictionary(uniqueKeysWithValues:(URLComponents(url:request.url,resolvingAgainstBaseURL:false)?.queryItems ?? []).map { ($0.name,$0.value ?? "") })
    #expect(request.url.host == "accounts.google.com")
    #expect(items["client_id"] == clientID)
    #expect(items["response_type"] == "code")
    #expect(items["scope"] == "openid email")
    #expect(items["nonce"] == "hub-nonce")
    #expect(items["state"] == "state-1")
    #expect(items["code_challenge_method"] == "S256")
    #expect(items["code_challenge"] == Data(SHA256.hash(data:Data("verifier".utf8))).base64URL)
}

@Test func googleAuthorizationTakesOnlyItsOwnRedirect() throws {
    let request = try GoogleAuthorization(clientID:clientID,nonce:"n",verifier:"v",state:"s")
    #expect(try request.code(from:URL(string:"com.googleusercontent.apps.1234-ios:/oauth2redirect?state=s&code=4/abc+def")!) == "4/abc+def")
    #expect(throws:HubSignInError.unreadable) { try request.code(from:URL(string:"com.googleusercontent.apps.1234-ios:/oauth2redirect?state=other&code=x")!) }
    #expect(throws:HubSignInError.unreadable) { try request.code(from:URL(string:"com.evil.app:/oauth2redirect?state=s&code=x")!) }
    #expect(throws:HubSignInError.canceled) { try request.code(from:URL(string:"com.googleusercontent.apps.1234-ios:/oauth2redirect?state=s&error=access_denied")!) }
}

@Test func googleTokenRequestSendsTheVerifierAndNoSecret() throws {
    let request = try GoogleAuthorization(clientID:clientID,nonce:"n",verifier:"v",state:"s").tokenRequest(code:"4/a+b")
    let body = String(decoding:request.httpBody ?? Data(),as:UTF8.self)
    #expect(request.url?.absoluteString == "https://oauth2.googleapis.com/token")
    #expect(body.contains("code=4/a%2Bb"))
    #expect(body.contains("code_verifier=v"))
    #expect(body.contains("grant_type=authorization_code"))
    #expect(!body.contains("client_secret"))
}

@Test func onlyGoogleIOSClientIDsAreAccepted() {
    #expect(throws:HubSignInError.unreadable) { try GoogleAuthorization.callbackScheme(clientID:"not-a-client") }
    #expect(throws:HubSignInError.unreadable) { try GoogleAuthorization.callbackScheme(clientID:"x/y.apps.googleusercontent.com") }
}
