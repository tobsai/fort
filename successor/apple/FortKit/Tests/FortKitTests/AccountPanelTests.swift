import Foundation
import Testing
@testable import FortKit

private let operation = AccountOperation(revision:2,operationID:"op-1",expiresAt:10_000)
private func identity() throws -> AccountIdentity { try JSONDecoder().decode(AccountIdentity.self,from:Data(#"{"accountId":"acct-1","userLabel":"toby@example.com"}"#.utf8)) }
private func code() throws -> AccountChallenge { try JSONDecoder().decode(AccountChallenge.self,from:Data(#"{"kind":"deviceCode","verificationURL":"https://example.com/device","userCode":"WDJB-MJHT"}"#.utf8)) }
private func browser() throws -> AccountChallenge { try JSONDecoder().decode(AccountChallenge.self,from:Data(#"{"kind":"browserAuthorization","authorizationURL":"https://example.com/authorize?x=1"}"#.utf8)) }

@Test func accountPanelShowsOneLightOneLineAndOneAction() throws {
    let ready = AccountPanel.make(status:.identified(operation,try identity(),true),hasPendingRequest:false,now:1)
    #expect(ready.lamp == .ready && ready.action == nil && ready.line == "Signed in as toby@example.com.")
    let unselected = AccountPanel.make(status:.identified(operation,try identity(),false),hasPendingRequest:false,now:1)
    #expect(unselected.lamp == .attention && unselected.action == .useAccount)
    let idle = AccountPanel.make(status:.idle(0),hasPendingRequest:false,now:1)
    #expect(idle.lamp == .attention && idle.action == .signIn && !idle.keepsChecking)
    #expect(AccountPanel.make(status:.unavailable(0),hasPendingRequest:false,now:1).action == nil)
}

@Test func accountPanelKeepsCheckingOnlyWhileSignInIsInProgress() throws {
    let waiting = AccountPanel.make(status:.challenge(operation,try code()),hasPendingRequest:false,now:1)
    #expect(waiting.lamp == .waiting && waiting.action == .cancel && waiting.keepsChecking)
    #expect(waiting.line.hasPrefix("Enter this code"))
    #expect(AccountPanel.make(status:.challenge(operation,try browser()),hasPendingRequest:false,now:1).line.hasPrefix("Finish signing in"))
    let expired = AccountPanel.make(status:.challenge(operation,try code()),hasPendingRequest:false,now:10_000)
    #expect(expired.lamp == .attention && expired.keepsChecking && expired.line.hasPrefix("That code expired"))
    #expect(AccountPanel.make(status:.starting(operation),hasPendingRequest:false,now:1).keepsChecking)
    #expect(AccountPanel.make(status:.canceling(operation),hasPendingRequest:false,now:1).keepsChecking)
    for result in [AccountSignInResult.failed,.canceled,.unconfirmed,.signedIn] {
        let finished = AccountPanel.make(status:.finished(operation,result),hasPendingRequest:false,now:1)
        #expect(finished.action == .signIn && !finished.keepsChecking && finished.lamp == .attention)
    }
}

@Test func accountPanelSendsASavedRequestToTheWorkshopInsteadOfGuessing() {
    let saved = AccountPanel.make(status:nil,hasPendingRequest:true,now:1)
    #expect(saved.lamp == .attention && saved.action == nil && saved.line.contains("Workshop"))
    let unknown = AccountPanel.make(status:nil,hasPendingRequest:false,now:1)
    #expect(unknown.lamp == .off && unknown.action == nil && !unknown.keepsChecking)
}
