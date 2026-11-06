import Foundation

/// First run (revision 2026-11-06): the steps between connecting and using Fort, the same as setupSteps in web/transcript.js.
/// Two are needed (a signed-in account, then a first Agent); devices are optional and never block.
struct SetupSteps: Equatable, Sendable {
    struct Step: Equatable, Identifiable, Sendable {
        let id: String
        let title: String
        let line: String
        let action: String
        let tab: PauseTab
        let done: Bool
        let current: Bool
    }
    let steps: [Step]
    var done: Bool { steps.allSatisfy(\.done) }

    static func make(account: Bool, agents: Int) -> SetupSteps {
        let plan: [(String, String, String, String, PauseTab, Bool)] = [
            ("account","Sign in to ChatGPT or Claude","Fort uses your own plan. Nothing is billed to an API key.","Sign in",.accounts,account),
            ("agent","Create your first agent","Give it a name, a model and a few lines on how to answer.","Create agent",.crew,agents > 0),
        ]
        let next = plan.first { !$0.5 }?.0
        return SetupSteps(steps:plan.map { Step(id:$0.0,title:$0.1,line:$0.2,action:$0.3,tab:$0.4,done:$0.5,current:$0.0 == next) })
    }
}

/// The Mac's own first step, before the first run: start Fort's helper with this Mac, then connect to it.
/// Built from the Devices facts, so the guided sheet and the Devices tab never disagree.
struct MacConnectSteps: Equatable, Sendable {
    enum Action: Equatable, Sendable { case enableHost, checkAgain, connect }
    struct Step: Equatable, Identifiable, Sendable {
        let id: String
        let title: String
        let line: String
        let action: Action?
        let done: Bool
        let current: Bool
    }
    let steps: [Step]

    static func make(_ facts: DevicesBoard.Facts) -> MacConnectSteps {
        let started = facts.hostSetup == .enabled || facts.connected
        let startLine: String
        let startAction: MacConnectSteps.Action?
        switch facts.hostSetup {
        case .needsApproval?: startLine = "macOS needs your OK: allow Fort in System Settings, General, Login Items. Then check again."; startAction = .checkAgain
        case .unsupported?: startLine = "macOS reported a state Fort doesn't know. More options has the details."; startAction = .checkAgain
        case .unknown?: startLine = "Checking this Mac…"; startAction = nil
        default: startLine = started ? "Fort's helper starts when you log in." : "Fort's helper runs in the background and starts when you log in."; startAction = started ? nil : .enableHost
        }
        let first = Step(id:"start",title:"Let Fort run on this Mac",line:startLine,action:startAction,done:started,current:!started)
        let second = Step(id:"connect",title:"Connect this Mac",line:facts.connected ? "Connected." : "Pairs this window with the helper. The key stays in your Keychain.",
                          action:started && !facts.connected ? .connect : nil,done:facts.connected,current:started && !facts.connected)
        return MacConnectSteps(steps:[first,second])
    }
}
