import SwiftUI

/// The home screen: the Quest log of what needs you beside a panel for every chat. The look is cool and
/// quiet (hairline panels, tags); the game is the loop (quest marks, the crew, run meters and a
/// cleared stamp). Every element reads real records through QuestLog.
struct QuestLogView: View {
    @Bindable var model: MessengerModel
    let openChat: (String) -> Void
    let showCity: (() -> Void)?
    let openMenu: () -> Void
    private struct Cleared: Identifiable, Equatable { let id: String; let title: String; let dropped: Bool }
    @State private var cleared: [Cleared] = []
    #if os(iOS)
    @Environment(\.horizontalSizeClass) private var sizeClass
    private var compact: Bool { sizeClass == .compact }
    #else
    private let compact = false
    #endif

    var body: some View {
        let log = model.questLog
        VStack(spacing:0) {
            ComicTopBar(summary:log.summary,showCity:showCity,openSetup:openMenu,compact:compact)
            ScrollView {
                if compact {
                    VStack(alignment:.leading,spacing:24) { questColumn(log); chatColumn(log) }.padding(16)
                } else {
                    HStack(alignment:.top,spacing:28) { questColumn(log).frame(width:400); chatColumn(log) }.padding(22)
                }
            }
            statusBar
        }
        .background(FortTheme.paper.ignoresSafeArea())
        .foregroundStyle(FortTheme.ink)
        .environment(\.fortSurface,.comic)
    }

    private func questColumn(_ log: QuestLog) -> some View {
        VStack(alignment:.leading,spacing:14) {
            HStack(spacing:10) {
                Text("Quest log").font(FortTheme.display(20)).tracking(2).textCase(.uppercase).accessibilityAddTraits(.isHeader)
                if !log.quests.isEmpty { CaptionBox(text:log.quests.count == 1 ? "1 needs you" : "\(log.quests.count) need you",fill:FortTheme.accent) }
            }
            if log.quests.isEmpty {
                Text("Nothing needs you. Your crew is on it.").font(.system(.body).weight(.bold)).foregroundStyle(FortTheme.muted)
                    .padding(14).frame(maxWidth:.infinity,alignment:.leading).comicPanel()
            }
            ForEach(log.quests) { questCard($0) }
            ForEach(cleared.filter { item in !log.quests.contains { $0.id == item.id } }.suffix(3)) { item in
                HStack(spacing:10) {
                    Text(item.dropped ? "Dropped" : "Cleared").font(FortTheme.display(13)).tracking(1.5).textCase(.uppercase).padding(.horizontal,8).padding(.vertical,1)
                        .background(RoundedRectangle(cornerRadius:2).fill(item.dropped ? FortTheme.off : FortTheme.done)).foregroundStyle(FortTheme.captionInk)
                    Text(item.title).font(.caption.weight(.medium)).foregroundStyle(FortTheme.muted).lineLimit(1)
                }.padding(.vertical,6).padding(.horizontal,10)
            }
        }
    }

    private func questCard(_ quest: QuestLog.Quest) -> some View {
        let kind = switch quest.kind { case .approval: "Approval"; case .failed: "Failed"; case .waiting: "Waiting" }
        let meta = [quest.agent?.name,quest.chat == quest.agent?.name ? nil : quest.chat,kind].compactMap { $0 }.joined(separator:" · ")
        return VStack(alignment:.leading,spacing:8) {
            HStack(alignment:.top,spacing:12) {
                QuestMark(kind:quest.kind)
                VStack(alignment:.leading,spacing:3) {
                    Text(quest.title).font(.body.weight(.semibold))
                    HStack(spacing:6) {
                        if let agent = quest.agent { CrewBadge(model:model,agent:agent,size:20) }
                        Text(meta).font(.caption.weight(.medium)).foregroundStyle(FortTheme.muted)
                    }
                }
            }
            if let detail = quest.detail { Text(detail).font(.callout).foregroundStyle(FortTheme.muted).padding(.leading,40) }
            HStack(spacing:8) {
                if let approval = quest.approval {
                    Button(approval.kind == "routineGate" ? "Continue routine" : "Approve") { decide(quest,approval:approval,approve:true) }
                        .buttonStyle(ComicButtonStyle(fill:FortTheme.accent)).disabled(model.isBusy || !model.supportsCommands)
                    Button("Reject") { decide(quest,approval:approval,approve:false) }.buttonStyle(ComicButtonStyle()).disabled(model.isBusy || !model.supportsCommands)
                    Button("Open chat") { openChat(quest.conversationID) }.buttonStyle(.plain).font(.callout.weight(.semibold)).foregroundStyle(FortTheme.link)
                } else {
                    Button("Open chat") { openChat(quest.conversationID) }.buttonStyle(ComicButtonStyle(fill:FortTheme.accent))
                }
            }.padding(.leading,40)
        }
        .padding(12).frame(maxWidth:.infinity,alignment:.leading)
        .comicPanel(live:quest.kind == .approval)
        .accessibilityElement(children:.contain)
    }

    private func decide(_ quest: QuestLog.Quest, approval: Approval, approve: Bool) {
        Task {
            await model.decide(approval,approve:approve)
            if !model.questLog.quests.contains(where:{ $0.id == quest.id }) { cleared.append(Cleared(id:quest.id,title:quest.title,dropped:!approve)) }
        }
    }

    private func chatColumn(_ log: QuestLog) -> some View {
        VStack(alignment:.leading,spacing:14) {
            Text("Chats").font(FortTheme.display(20)).tracking(2).textCase(.uppercase).accessibilityAddTraits(.isHeader)
            if log.panels.isEmpty {
                Text("No agents yet. Open Setup to add one.").font(.system(.body).weight(.bold)).foregroundStyle(FortTheme.muted)
                    .padding(14).frame(maxWidth:.infinity,alignment:.leading).comicPanel()
            }
            LazyVGrid(columns:[GridItem(.adaptive(minimum:240),spacing:14)],alignment:.leading,spacing:14) {
                ForEach(log.panels) { chatPanel($0) }
            }
        }
    }

    private func chatPanel(_ panel: QuestLog.Panel) -> some View {
        Button { openChat(panel.id) } label: {
            VStack(alignment:.leading,spacing:0) {
                HStack(spacing:8) {
                    CaptionBox(text:panel.name)
                    Spacer(minLength:4)
                    Text(panel.line.uppercased()).font(.system(size:11,weight:.bold)).tracking(0.9).foregroundStyle(panel.tone == .needsYou || panel.tone == .working || panel.tone == .failed ? FortTheme.captionInk : FortTheme.ink)
                }
                .padding(.horizontal,12).padding(.vertical,8)
                .background(stripColor(panel.tone))
                .overlay(alignment:.bottom) { FortTheme.line.frame(height:FortTheme.stroke) }
                HStack(spacing:4) { ForEach(panel.crew) { CrewBadge(model:model,agent:$0,size:28) } }
                    .padding(.horizontal,12).padding(.top,10).frame(minHeight:40)
                VStack(alignment:.leading,spacing:8) {
                    if panel.live.isEmpty { Text("Nothing running").font(.caption.weight(.medium)).foregroundStyle(FortTheme.muted) }
                    ForEach(panel.live) { run in
                        HStack(spacing:8) {
                            QuestMeter(filled:run.filled,tone:run.tone)
                            Text([run.agentName,run.text].compactMap { $0 }.joined(separator:": ")).font(.caption.weight(.medium)).lineLimit(2)
                        }
                    }
                }.padding(12)
            }
            .frame(maxWidth:.infinity,alignment:.leading)
            .comicPanel()
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel("\(panel.name): \(panel.line)")
    }

    private func stripColor(_ tone: QuestLog.Tone) -> Color {
        switch tone { case .failed: Color(hex:0xF2B8B2); case .needsYou: FortTheme.needsYou; case .working: FortTheme.working; default: FortTheme.bar }
    }

    private var statusBar: some View {
        let header = ChatQuiet.header(connected:model.connected,preview:model.isOfflinePreview || model.isInteractiveFixture,runtimeState:model.runtimeReadiness?.state,status:model.status)
        return HStack(spacing:8) {
            LampView(lamp:header.lamp)
            Text(header.line).font(.caption.weight(.medium)).foregroundStyle(FortTheme.muted).lineLimit(1)
            Spacer()
        }
        .padding(.horizontal,16).padding(.vertical,8)
        .background(FortTheme.bar.ignoresSafeArea(edges:.bottom))
        .overlay(alignment:.top) { FortTheme.line.frame(height:FortTheme.stroke) }
        .accessibilityElement(children:.combine)
    }
}

/// A quest mark: a gold diamond with a bang, red when the turn failed, hollow while it waits on the chat.
struct QuestMark: View {
    let kind: QuestLog.Kind
    var body: some View {
        let fill = switch kind { case .approval: FortTheme.needsYou; case .failed: FortTheme.failed; case .waiting: FortTheme.caption }
        ZStack {
            Rectangle().fill(fill).overlay(Rectangle().stroke(kind == .waiting ? FortTheme.line : Color.clear,lineWidth:FortTheme.stroke)).rotationEffect(.degrees(45)).frame(width:17,height:17)
            Text("!").font(.system(size:11,weight:.heavy)).foregroundStyle(kind == .failed ? Color.white : FortTheme.captionInk)
        }.frame(width:28,height:28).accessibilityHidden(true)
    }
}

/// Three bars, the RunMeter steps: cyan with a holographic glow while working, gold when it needs you.
struct QuestMeter: View {
    let filled: Int
    let tone: QuestLog.Tone
    var body: some View {
        let fill = switch tone { case .working: FortTheme.working; case .needsYou: FortTheme.needsYou; case .done: FortTheme.done; case .failed: FortTheme.failed; default: FortTheme.off }
        HStack(spacing:3) {
            ForEach(0..<3,id:\.self) { index in
                RoundedRectangle(cornerRadius:1).fill(index < filled ? fill : FortTheme.line)
                    .frame(width:14,height:4)
                    .shadow(color:tone == .working && index < filled ? FortTheme.glow : .clear,radius:3)
            }
        }.accessibilityHidden(true)
    }
}

/// An Agent's portrait in a circle, or its initial on its crew colour until the portrait loads.
struct CrewBadge: View {
    let model: MessengerModel
    let agent: Agent
    var size: CGFloat = 28
    var body: some View {
        ZStack {
            Circle().fill(FortTheme.crewColor(for:agent.agentId))
            if let image = model.portraitImage(for:agent) {
                Image(decorative:image,scale:1).resizable().interpolation(.high).scaledToFill().frame(width:size,height:size).clipShape(Circle())
            } else {
                Text(agent.name.trimmingCharacters(in:.whitespaces).prefix(1).uppercased()).font(.system(size:size * 0.42,weight:.bold)).foregroundStyle(Color(hex:0x0B0E13))
            }
        }
        .frame(width:size,height:size)
        .overlay(Circle().stroke(FortTheme.line,lineWidth:FortTheme.stroke))
        .task(id:agent.portrait?.revision) { await model.loadPortrait(agent) }
        .help(agent.name)
        .accessibilityLabel(agent.name)
    }
}

/// A button: a hairline edge on the panel, or a flat fill for gold and the other status colours.
struct ComicButtonStyle: ButtonStyle {
    var fill: Color = FortTheme.panel
    @Environment(\.isEnabled) private var isEnabled
    func makeBody(configuration: Configuration) -> some View {
        let shape = RoundedRectangle(cornerRadius:FortTheme.radius)
        configuration.label
            .font(.callout.weight(.semibold))
            .foregroundStyle(fill == FortTheme.panel || fill == FortTheme.caption ? FortTheme.ink : FortTheme.captionInk)
            .padding(.horizontal,12)
            #if os(iOS)
            .frame(minHeight:FortTheme.touch)
            #else
            .frame(minHeight:30)
            #endif
            .background(shape.fill(fill))
            .overlay(shape.stroke(fill == FortTheme.panel || fill == FortTheme.caption ? FortTheme.line : Color.clear,lineWidth:FortTheme.stroke))
            .opacity(isEnabled ? (configuration.isPressed ? 0.75 : 1) : 0.45)
            .contentShape(shape)
    }
}
