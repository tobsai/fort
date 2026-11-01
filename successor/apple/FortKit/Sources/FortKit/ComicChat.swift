import SwiftUI

// The chat and Setup pieces the Mac and iPhone apps share with the web client (web/app.js and
// web/app.css), so a chat looks and reads the same everywhere: the night top bar, flat message
// panels, run tags, the approval panel and the composer.

/// Which skin a shared control draws in: the dark game HUD of the city, or the comic page used by
/// the Quest log, the chat and Setup (the same look as the web client).
enum FortSurface: Sendable { case hud, comic }
private struct FortSurfaceKey: EnvironmentKey { static let defaultValue = FortSurface.hud }
extension EnvironmentValues {
    var fortSurface: FortSurface { get { self[FortSurfaceKey.self] } set { self[FortSurfaceKey.self] = newValue } }
}

/// The words on an approval, the same as web/transcript.js approvalCopy.
struct ApprovalCopy: Equatable, Sendable {
    let approve: String
    let reject: String
    /// What a decided approval says instead of its buttons; nil while it can still be decided.
    let decided: String?
    /// The caption above the request: "Needs you" while open, otherwise what happened.
    let label: String

    static func make(kind: String, state: String) -> ApprovalCopy {
        let routine = kind == "routineGate"
        let decided: String? = switch state {
        case "approved": routine ? "Continued" : "Approved"
        case "rejected": "Rejected"
        case "expired": "Expired before a decision"
        case "canceled": "Canceled"
        default: nil
        }
        return ApprovalCopy(approve:routine ? "Continue routine" : "Approve",reject:"Reject",decided:decided,label:state == "pending" ? "Needs you" : decided ?? state)
    }
}

/// A run's line in the chat, the same as web/transcript.js runStatus: a tone for the lamp, the plain
/// sentence, and the service's own detail when it adds something (not while it waits on you).
struct RunCaptionLine: Equatable, Sendable {
    let tone: QuestLog.Tone
    let text: String
    let detail: String?
    static func make(state: String, displayState: String, detail: String?) -> RunCaptionLine {
        let tone = QuestLog.tone(state)
        let shown: String? = if let detail, !detail.isEmpty, tone != .needsYou, detail != displayState { detail } else { nil }
        return RunCaptionLine(tone:tone,text:displayState,detail:shown)
    }
}

/// The night top bar of every Fort screen: the wordmark, one tag of what's going on, and Setup.
struct ComicTopBar: View {
    let summary: String
    var showCity: (() -> Void)? = nil
    let openSetup: () -> Void
    var compact = false
    var body: some View {
        HStack(spacing:12) {
            Text("FORT").font(FortTheme.display(22)).tracking(4.8).foregroundStyle(Color(hex:0xE8ECF1))
                .shadow(color:FortTheme.glow.opacity(0.55),radius:10)
                .accessibilityLabel("Fort")
            if !compact { CaptionBox(text:summary) }
            Spacer(minLength:8)
            if let showCity, !compact { Button("City",action:showCity).buttonStyle(ComicButtonStyle()).accessibilityIdentifier("show-city") }
            Button("Setup",action:openSetup).buttonStyle(ComicButtonStyle())
                .keyboardShortcut(",",modifiers:.command).accessibilityIdentifier("pause-menu-open")
        }
        .padding(.horizontal,16).padding(.vertical,10)
        .background(LinearGradient(colors:FortTheme.sky,startPoint:.leading,endPoint:.trailing).ignoresSafeArea(edges:.top))
        .overlay(alignment:.bottom) { Color(hex:0x283039).frame(height:1) }
        // The bar is night in both modes, so what sits on it draws in dark.
        .environment(\.colorScheme,.dark)
    }
}

/// A message: a flat hairline panel, yours tinted, like .msg on the web.
struct SpeechBubble: ViewModifier {
    let mine: Bool
    func body(content: Content) -> some View {
        let fill = mine ? FortTheme.mine : FortTheme.panel
        let shape = RoundedRectangle(cornerRadius:FortTheme.bubbleRadius)
        content
            .padding(.horizontal,14).padding(.vertical,10)
            .background(shape.fill(fill))
            .overlay(shape.strokeBorder(mine ? FortTheme.glow.opacity(0.3) : FortTheme.line,lineWidth:FortTheme.stroke))
    }
}
extension View { func speechBubble(mine: Bool) -> some View { modifier(SpeechBubble(mine:mine)) } }

/// The small uppercase name over a bubble ("You → Lewis", "LEWIS"), like .msg .who on the web.
struct SpeakerCaption: View {
    let text: String
    var body: some View {
        Text(text.uppercased()).font(.system(size:10.5,weight:.bold)).tracking(1.3).foregroundStyle(FortTheme.muted)
    }
}

/// A run as a tag with its lamp, like .run on the web: cyan glow working, gold needs you, red failed.
struct RunCaption: View {
    let line: RunCaptionLine
    var body: some View {
        let failed = line.tone == .failed
        HStack(spacing:8) {
            Circle().fill(lamp).frame(width:8,height:8)
                .shadow(color:line.tone == .working ? FortTheme.glow : .clear,radius:3)
            Text(line.text).font(.system(size:12,weight:.semibold))
            if let detail = line.detail { Text("· " + detail).font(.system(size:12,weight:.medium)).opacity(0.7) }
        }
        .foregroundStyle(failed ? Color.white : FortTheme.ink)
        .padding(.horizontal,10).padding(.vertical,4)
        .background(RoundedRectangle(cornerRadius:2).fill(failed ? FortTheme.failed : FortTheme.caption))
        .overlay(RoundedRectangle(cornerRadius:2).stroke(failed ? Color.clear : FortTheme.line,lineWidth:FortTheme.stroke))
    }
    private var lamp: Color {
        switch line.tone { case .working: FortTheme.working; case .needsYou: FortTheme.needsYou; case .done: FortTheme.done; case .failed: .white; default: FortTheme.off }
    }
}

/// The approval panel's tag: gold "Needs you" while open, quiet once decided, like .approval .label.
struct ApprovalLabel: View {
    let text: String
    let open: Bool
    var body: some View {
        Text(text.uppercased()).font(.system(size:10.5,weight:.bold)).tracking(1.3)
            .accessibilityLabel(text)
            .foregroundStyle(open ? FortTheme.captionInk : FortTheme.muted)
            .padding(.horizontal,8).padding(.vertical,1)
            .background(RoundedRectangle(cornerRadius:2).fill(open ? FortTheme.needsYou : FortTheme.caption))
    }
}

/// An Agent the way every surface lists it: its badge, its name, and "provider · model" underneath.
struct CrewRow: View {
    let model: MessengerModel
    let agent: Agent
    var badge: CGFloat = 36
    var body: some View {
        HStack(spacing:10) {
            CrewBadge(model:model,agent:agent,size:badge)
            VStack(alignment:.leading,spacing:2) {
                Text(agent.name).font(.body.weight(.semibold)).foregroundStyle(FortTheme.ink).lineLimit(1)
                Text(agent.isOnHub ? "On the hub · read-only" : CrewRow.line(provider:PauseMenu.className(provider:agent.binding.provider,runtimes:model.runtimes),model:agent.binding.selectedModel))
                    .font(.caption.weight(.medium)).foregroundStyle(FortTheme.muted).lineLimit(1)
            }
        }
        .accessibilityElement(children:.combine)
    }
    /// "Claude · claude-opus-5-5", the same line as the web's crew list.
    nonisolated static func line(provider: String, model: String) -> String { [provider,model].filter { !$0.isEmpty }.joined(separator:" · ") }
}
