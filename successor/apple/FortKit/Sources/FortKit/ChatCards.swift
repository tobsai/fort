import Foundation

/// A run's progress as three pips: accepted, working, done. The light says whether it needs you.
struct RunMeter: Equatable, Sendable {
    static let steps = 3
    let filled: Int
    let lamp: StatusLamp

    static func make(state: String) -> RunMeter {
        switch state {
        case "queued": RunMeter(filled:1,lamp:.waiting)
        case "running", "cancelRequested": RunMeter(filled:2,lamp:.waiting)
        case "waitingForApproval", "outcomeUnknown": RunMeter(filled:2,lamp:.attention)
        case "completed": RunMeter(filled:3,lamp:.ready)
        case "failed": RunMeter(filled:3,lamp:.attention)
        case "canceled", "closedUnknown": RunMeter(filled:3,lamp:.off)
        default: RunMeter(filled:0,lamp:.off)
        }
    }
}

/// Problems that stop the crew, shown above the chat with one way to fix each. Nothing here acts on its own.
struct TroubleCard: Equatable, Sendable, Identifiable {
    enum Kind: String, Sendable { case lostContact, hostStopped, restartRequired, signedOut, scheduleFault }
    let kind: Kind
    let title: String
    let line: String
    let button: String
    let tab: PauseTab
    var id: String { kind.rawValue }

    struct Facts: Equatable, Sendable {
        var lostContact = false
        var status = ""
        var runtimeState: String?
        var runtimeDetail: String?
        var accountNeedsYou = false
        var scheduleFault: String?
    }

    static func make(_ facts: Facts) -> [TroubleCard] {
        // While Fort can't reach the host, everything else it knows may be stale, so only say that.
        if facts.lostContact {
            return [TroubleCard(kind:.lostContact,title:"Lost contact",line:facts.status.isEmpty ? "Fort can't reach your Mac right now. Saved history is kept." : facts.status,button:"Open Devices",tab:.devices)]
        }
        var cards: [TroubleCard] = []
        switch facts.runtimeState {
        case "stopped"?: cards.append(TroubleCard(kind:.hostStopped,title:"Crew stopped",line:facts.runtimeDetail ?? "Local work is stopped on this Mac.",button:"Open Devices",tab:.devices))
        case "restartRequired"?: cards.append(TroubleCard(kind:.restartRequired,title:"Restart needed",line:facts.runtimeDetail ?? "The runtime needs a restart before it can take work.",button:"Open Devices",tab:.devices))
        default: break
        }
        if facts.accountNeedsYou { cards.append(TroubleCard(kind:.signedOut,title:"Sign-in needed",line:"Agents on this plan wait until you sign in.",button:"Open Accounts",tab:.accounts)) }
        if let fault = facts.scheduleFault { cards.append(TroubleCard(kind:.scheduleFault,title:"Schedule trouble",line:fault,button:"Open Playbook",tab:.playbook)) }
        return cards
    }
}
