import Foundation

/// The top of the Devices tab: one row per device with a light, a sentence and the next step.
/// Every other control (cancel setup, revoke, certificates, removal) stays in the Workshop below it.
struct DevicesBoard: Equatable, Sendable {
    /// Where this Mac's helper stands with macOS startup, from `MacHostControl`.
    enum HostSetup: Equatable, Sendable { case unknown, needsEnable, needsApproval, enabled, unsupported }
    enum Action: Equatable, Sendable { case enableHost, connect, resume, restart, startListener, invite, openWorkshop }
    struct Row: Equatable, Identifiable, Sendable {
        let id: String
        let title: String
        let lamp: StatusLamp
        let line: String
        let action: Action?
    }
    struct Facts: Equatable, Sendable {
        var isCompanion = false
        var hostSetup: HostSetup?
        var connected = false
        var runtimeState: String?
        var claudeServing = false
        var managesPhone = false
        var phoneAddress: String?
        var listenerActive = false
        var phone: CompanionPairingState?
    }
    let rows: [Row]

    static func make(_ facts: Facts) -> DevicesBoard {
        if facts.isCompanion {
            return DevicesBoard(rows:[facts.connected
                ? Row(id:"phone-self",title:"This iPhone",lamp:.ready,line:"Connected to your Mac.",action:nil)
                : Row(id:"phone-self",title:"This iPhone",lamp:.attention,line:"Not connected. On your Mac open Menu, Devices, Invite iPhone, then paste the invitation here.",action:.openWorkshop)])
        }
        var rows = [macRow(facts)]
        if facts.connected && facts.hostSetup != nil {
            rows.append(facts.claudeServing
                ? Row(id:"claude",title:"Claude on this Mac",lamp:.ready,line:"Claude agents can work here.",action:nil)
                : Row(id:"claude",title:"Claude on this Mac",lamp:.off,line:"Off. Turn it on in the Workshop to add Claude agents.",action:.openWorkshop))
        }
        if facts.managesPhone { rows.append(phoneRow(facts)) }
        return DevicesBoard(rows:rows)
    }

    private static func macRow(_ facts: Facts) -> Row {
        let title = "This Mac"
        switch facts.hostSetup {
        case .needsEnable?: return Row(id:"mac",title:title,lamp:.attention,line:"Fort's helper isn't set to start with this Mac yet.",action:.enableHost)
        case .needsApproval?: return Row(id:"mac",title:title,lamp:.attention,line:"macOS needs your OK. Allow Fort in System Settings, Login Items, then check again in the Workshop.",action:.openWorkshop)
        case .unsupported?: return Row(id:"mac",title:title,lamp:.attention,line:"macOS reported a startup state Fort doesn't know. Details are in the Workshop.",action:.openWorkshop)
        case .unknown?: return Row(id:"mac",title:title,lamp:.off,line:"Checking this Mac…",action:nil)
        case .enabled?, nil:
            if !facts.connected {
                return facts.hostSetup == nil
                    ? Row(id:"mac",title:"This device",lamp:.attention,line:"Not connected to Fort. Connection details are in the Workshop.",action:.openWorkshop)
                    : Row(id:"mac",title:title,lamp:.attention,line:"The helper is ready. Connect this Mac to start your crew.",action:.connect)
            }
            switch facts.runtimeState {
            case "stopped"?: return Row(id:"mac",title:title,lamp:.attention,line:"Your crew is stopped on this Mac. Saved conversations are safe.",action:.resume)
            case "restartRequired"?: return Row(id:"mac",title:title,lamp:.attention,line:"Your crew needs a restart to pick up a change.",action:.restart)
            default: return Row(id:"mac",title:title,lamp:.ready,line:facts.hostSetup == nil ? "Connected to Fort." : "Running your crew. It starts when you log in.",action:nil)
            }
        }
    }

    private static func phoneRow(_ facts: Facts) -> Row {
        let title = "iPhone"
        switch facts.phone {
        case .paired(_,let name,_,true)?: return Row(id:"phone",title:title,lamp:facts.listenerActive ? .ready : .attention,line:facts.listenerActive ? "\(name) is connected." : "\(name) is paired, but the phone listener is off.",action:facts.listenerActive ? nil : .startListener)
        case .paired(_,let name,_,false)?: return Row(id:"phone",title:title,lamp:.attention,line:"\(name)'s access expired. Invite it again.",action:facts.listenerActive ? .invite : .startListener)
        case .invited?: return Row(id:"phone",title:title,lamp:.waiting,line:"An invitation is waiting to be pasted on your iPhone.",action:facts.listenerActive ? .invite : .startListener)
        case .available?, nil:
            guard facts.phoneAddress != nil else { return Row(id:"phone",title:title,lamp:.off,line:"First give this Mac its phone address with a Tailscale certificate. Steps are in the Workshop.",action:.openWorkshop) }
            guard facts.listenerActive else { return Row(id:"phone",title:title,lamp:.off,line:"The phone listener is off.",action:.startListener) }
            return Row(id:"phone",title:title,lamp:.off,line:"Ready to invite your iPhone.",action:.invite)
        }
    }
}
