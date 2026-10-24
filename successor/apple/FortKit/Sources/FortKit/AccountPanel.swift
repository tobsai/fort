import Foundation

/// The state light in the pause menu: ready, working on it, needs you, or nothing to show.
enum StatusLamp: Equatable, Sendable { case ready, waiting, attention, off }

/// What the Accounts tab says about the focused plan: one light, one sentence, and at most one main action.
/// Recovery steps (check status, retry, discard, change account) stay available in the Workshop drawer.
struct AccountPanel: Equatable, Sendable {
    enum Action: Equatable, Sendable { case signIn, useAccount, cancel }
    let lamp: StatusLamp
    let line: String
    let action: Action?
    /// Fort keeps checking with the host while sign-in is in progress, so nobody has to press "Check status".
    let keepsChecking: Bool

    static func make(status: AccountSignInStatus?, hasPendingRequest: Bool, now: Int64) -> AccountPanel {
        guard let status else {
            if hasPendingRequest { return AccountPanel(lamp:.attention,line:"A saved sign-in request is waiting. Retry or discard it in the Workshop.",action:nil,keepsChecking:false) }
            return AccountPanel(lamp:.off,line:"Checking with Fort…",action:nil,keepsChecking:false)
        }
        switch status {
        case .identified(_,let account,true):
            return AccountPanel(lamp:.ready,line:"Signed in as \(account.userLabel).",action:nil,keepsChecking:false)
        case .identified(_,let account,false):
            return AccountPanel(lamp:.attention,line:"Signed in as \(account.userLabel). Use this account to start.",action:.useAccount,keepsChecking:false)
        case .starting:
            return AccountPanel(lamp:.waiting,line:"Starting sign-in…",action:.cancel,keepsChecking:true)
        case .canceling:
            return AccountPanel(lamp:.waiting,line:"Cancelling sign-in…",action:nil,keepsChecking:true)
        case .challenge(let operation,let challenge):
            if now >= operation.expiresAt {
                return AccountPanel(lamp:.attention,line:challenge.userCode == nil ? "The sign-in page timed out. Fort is checking before you try again." : "That code expired. Fort is checking before you try again.",action:.cancel,keepsChecking:true)
            }
            return AccountPanel(lamp:.waiting,line:challenge.userCode == nil ? "Finish signing in on the page that opened. Fort notices when you're done." : "Enter this code on the page that opened. Fort notices when you're done.",action:.cancel,keepsChecking:true)
        case .finished(_,.canceled):
            return AccountPanel(lamp:.attention,line:"Sign-in was cancelled.",action:.signIn,keepsChecking:false)
        case .finished(_,.failed):
            return AccountPanel(lamp:.attention,line:"Sign-in didn't finish. Try again.",action:.signIn,keepsChecking:false)
        case .finished(_,.unconfirmed):
            return AccountPanel(lamp:.attention,line:"Fort couldn't confirm who signed in. Sign in again to be sure.",action:.signIn,keepsChecking:false)
        case .finished(_,.signedIn):
            return AccountPanel(lamp:.attention,line:"Signed in, but Fort couldn't tell which account. Sign in again to confirm it.",action:.signIn,keepsChecking:false)
        case .idle:
            return AccountPanel(lamp:.attention,line:"Not signed in. Agents on this plan wait until you sign in.",action:.signIn,keepsChecking:false)
        case .unavailable:
            return AccountPanel(lamp:.off,line:"Sign-in isn't available on this Mac right now.",action:nil,keepsChecking:false)
        }
    }
}
