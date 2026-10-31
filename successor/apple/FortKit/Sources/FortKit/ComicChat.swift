import SwiftUI

// The chat and Setup pieces the Mac and iPhone apps share with the web client (web/app.js and
// web/app.css), so a chat looks and reads the same everywhere: the comic top bar, speech bubbles
// with tails, run captions, the approval panel and the composer pill.

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

/// The comic top bar of every Fort screen: the wordmark, one caption of what's going on, and Setup.
struct ComicTopBar: View {
    let summary: String
    var showCity: (() -> Void)? = nil
    let openSetup: () -> Void
    var compact = false
    var body: some View {
        HStack(spacing:12) {
            Text("FORT").font(FortTheme.display(28)).tracking(1.5).foregroundStyle(Color(hex:0xE4FDFF))
                .shadow(color:Color(hex:0x1B1A24),radius:0,x:2,y:2).shadow(color:FortTheme.glow,radius:8)
                .accessibilityLabel("Fort")
            if !compact { CaptionBox(text:summary,fill:Color(hex:0xFFF8EA)) }
            Spacer(minLength:8)
            if let showCity, !compact { Button("City",action:showCity).buttonStyle(ComicButtonStyle()).accessibilityIdentifier("show-city") }
            Button("Setup",action:openSetup).buttonStyle(ComicButtonStyle())
                .keyboardShortcut(",",modifiers:.command).accessibilityIdentifier("pause-menu-open")
        }
        .padding(.horizontal,16).padding(.vertical,10)
        .background(LinearGradient(colors:FortTheme.sky,startPoint:.leading,endPoint:.trailing).ignoresSafeArea(edges:.top))
        .overlay(alignment:.bottom) { FortTheme.line.frame(height:3) }
        // The sky is the same in light and noir, so what sits on it keeps light ink.
        .environment(\.colorScheme,.light)
    }
}

/// A speech bubble: ink outline, round corners and a tail at the speaker's side, like .msg on the web.
struct SpeechBubble: ViewModifier {
    let mine: Bool
    func body(content: Content) -> some View {
        let fill = mine ? FortTheme.mine : FortTheme.panel
        let shape = RoundedRectangle(cornerRadius:FortTheme.bubbleRadius)
        content
            .padding(.horizontal,14).padding(.vertical,10)
            .background(shape.fill(fill))
            .overlay(shape.strokeBorder(FortTheme.line,lineWidth:FortTheme.stroke))
            .overlay(alignment:mine ? .bottomTrailing : .bottomLeading) {
                ZStack(alignment:mine ? .topTrailing : .topLeading) {
                    BubbleTail(mirrored:mine).fill(FortTheme.line).frame(width:14,height:15)
                    BubbleTail(mirrored:mine).fill(fill).frame(width:8,height:9).offset(x:mine ? -3.5 : 3.5,y:-FortTheme.stroke)
                }
                .frame(width:14,height:15,alignment:mine ? .topTrailing : .topLeading)
                .offset(x:mine ? -22 : 22,y:15)
                .accessibilityHidden(true)
            }
            // Room for the tail, so the speaker's badge lines up with its tip.
            .padding(.bottom,15)
    }
}
/// The right-angled tail under a bubble, its straight edge on the speaker's side.
struct BubbleTail: Shape {
    let mirrored: Bool
    func path(in rect: CGRect) -> Path {
        var path = Path()
        if mirrored {
            path.move(to:CGPoint(x:rect.maxX,y:rect.minY)); path.addLine(to:CGPoint(x:rect.maxX,y:rect.maxY)); path.addLine(to:CGPoint(x:rect.minX,y:rect.minY))
        } else {
            path.move(to:CGPoint(x:rect.minX,y:rect.minY)); path.addLine(to:CGPoint(x:rect.minX,y:rect.maxY)); path.addLine(to:CGPoint(x:rect.maxX,y:rect.minY))
        }
        path.closeSubpath()
        return path
    }
}
extension View { func speechBubble(mine: Bool) -> some View { modifier(SpeechBubble(mine:mine)) } }

/// The small uppercase name over a bubble ("You → Lewis", "LEWIS"), like .msg .who on the web.
struct SpeakerCaption: View {
    let text: String
    var body: some View {
        Text(text.uppercased()).font(.system(size:11,weight:.black,design:.rounded)).tracking(0.66).foregroundStyle(FortTheme.muted)
    }
}

/// A run as a caption box with its lamp, like .run on the web: cyan glow working, gold needs you, red failed.
struct RunCaption: View {
    let line: RunCaptionLine
    var body: some View {
        let failed = line.tone == .failed
        HStack(spacing:8) {
            Circle().fill(lamp).frame(width:10,height:10)
                .overlay(Circle().stroke(FortTheme.line,lineWidth:2))
                .shadow(color:line.tone == .working ? FortTheme.glow : .clear,radius:3)
            Text(line.text).font(.system(size:12,weight:.heavy,design:.rounded))
            if let detail = line.detail { Text("· " + detail).font(.system(size:12,weight:.bold,design:.rounded)).opacity(0.75) }
        }
        .foregroundStyle(failed ? Color.white : FortTheme.captionInk)
        .padding(.horizontal,10).padding(.vertical,4)
        .background(failed ? FortTheme.failed : FortTheme.caption)
        .overlay(Rectangle().stroke(FortTheme.line,lineWidth:2))
    }
    private var lamp: Color {
        switch line.tone { case .working: FortTheme.working; case .needsYou: FortTheme.needsYou; case .done: FortTheme.done; case .failed: .white; default: FortTheme.off }
    }
}

/// The approval panel's caption: gold "Needs you" while open, grey once decided, like .approval .label.
struct ApprovalLabel: View {
    let text: String
    let open: Bool
    var body: some View {
        Text(text.uppercased()).font(.system(size:11,weight:.black,design:.rounded)).tracking(0.66)
            .accessibilityLabel(text)
            .foregroundStyle(FortTheme.captionInk)
            .padding(.horizontal,8).padding(.vertical,1)
            .background(open ? FortTheme.needsYou : FortTheme.off)
            .overlay(Rectangle().stroke(FortTheme.line,lineWidth:2))
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
                Text(agent.name).font(.system(.body,design:.rounded).weight(.heavy)).foregroundStyle(FortTheme.ink).lineLimit(1)
                Text(CrewRow.line(provider:PauseMenu.className(provider:agent.binding.provider,runtimes:model.runtimes),model:agent.binding.selectedModel))
                    .font(.caption.weight(.bold)).foregroundStyle(FortTheme.muted).lineLimit(1)
            }
        }
        .accessibilityElement(children:.combine)
    }
    /// "Claude · claude-opus-5-5", the same line as the web's crew list.
    nonisolated static func line(provider: String, model: String) -> String { [provider,model].filter { !$0.isEmpty }.joined(separator:" · ") }
}
