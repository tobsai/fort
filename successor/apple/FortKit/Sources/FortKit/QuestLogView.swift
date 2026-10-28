import SwiftUI

/// The home screen: the Quest log of what needs you beside a comic panel for every chat. The comic is
/// the look (ink panels, caption boxes); the game is the loop (quest marks, the crew, run meters and a
/// Quest clear stamp). Every element reads real records through QuestLog.
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
            topBar(log)
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
    }

    private func topBar(_ log: QuestLog) -> some View {
        HStack(spacing:12) {
            Text("FORT").font(FortTheme.display(28)).tracking(1.5).foregroundStyle(Color(hex:0xE4FDFF))
                .shadow(color:Color(hex:0x1B1A24),radius:0,x:2,y:2).shadow(color:FortTheme.glow,radius:8)
                .accessibilityLabel("Fort")
            if !compact { CaptionBox(text:log.summary,fill:Color(hex:0xFFF8EA)) }
            Spacer(minLength:8)
            if let showCity, !compact { Button("City",systemImage:"building.2",action:showCity).buttonStyle(ComicButtonStyle()).accessibilityIdentifier("show-city") }
            Button("Menu",systemImage:"line.3.horizontal",action:openMenu).buttonStyle(ComicButtonStyle())
                .keyboardShortcut(",",modifiers:.command).accessibilityIdentifier("pause-menu-open")
        }
        .padding(.horizontal,16).padding(.vertical,10)
        .background(LinearGradient(colors:FortTheme.sky,startPoint:.leading,endPoint:.trailing).ignoresSafeArea(edges:.top))
        .overlay(alignment:.bottom) { FortTheme.line.frame(height:3) }
        .environment(\.colorScheme,.light)
    }

    private func questColumn(_ log: QuestLog) -> some View {
        VStack(alignment:.leading,spacing:14) {
            HStack(spacing:10) {
                Text("Quest log").font(FortTheme.display(26)).accessibilityAddTraits(.isHeader)
                if !log.quests.isEmpty { CaptionBox(text:log.quests.count == 1 ? "1 needs you" : "\(log.quests.count) need you",fill:FortTheme.accent) }
            }
            if log.quests.isEmpty {
                Text("Nothing needs you. Your crew is on it.").font(.system(.body,design:.rounded).weight(.bold)).foregroundStyle(FortTheme.muted)
                    .padding(14).frame(maxWidth:.infinity,alignment:.leading).comicPanel()
            }
            ForEach(log.quests) { questCard($0) }
            ForEach(cleared.filter { item in !log.quests.contains { $0.id == item.id } }.suffix(3)) { item in
                HStack(spacing:10) {
                    Text(item.dropped ? "Dropped" : "Quest clear!").font(FortTheme.display(17)).padding(.horizontal,10)
                        .background(item.dropped ? FortTheme.off : FortTheme.done).foregroundStyle(FortTheme.captionInk)
                        .overlay(Rectangle().stroke(FortTheme.line,lineWidth:FortTheme.stroke)).rotationEffect(.degrees(-3))
                    Text(item.title).font(.caption.weight(.bold)).foregroundStyle(FortTheme.muted).lineLimit(1)
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
                    Text(quest.title).font(.system(.body,design:.rounded).weight(.heavy))
                    HStack(spacing:6) {
                        if let agent = quest.agent { CrewBadge(model:model,agent:agent,size:20) }
                        Text(meta).font(.caption.weight(.bold)).foregroundStyle(FortTheme.muted)
                    }
                }
            }
            if let detail = quest.detail { Text(detail).font(.callout).foregroundStyle(FortTheme.muted).padding(.leading,40) }
            HStack(spacing:8) {
                if let approval = quest.approval {
                    Button(approval.kind == "routineGate" ? "Continue routine" : "Approve") { decide(quest,approval:approval,approve:true) }
                        .buttonStyle(ComicButtonStyle(fill:FortTheme.accent)).disabled(model.isBusy || !model.supportsCommands)
                    Button("Reject") { decide(quest,approval:approval,approve:false) }.buttonStyle(ComicButtonStyle()).disabled(model.isBusy || !model.supportsCommands)
                    Button("Open chat") { openChat(quest.conversationID) }.buttonStyle(.plain).font(.callout.weight(.bold)).underline().foregroundStyle(FortTheme.link)
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
            Text("Chats").font(FortTheme.display(26)).accessibilityAddTraits(.isHeader)
            if log.panels.isEmpty {
                Text("No agents yet. Open the menu to add one.").font(.system(.body,design:.rounded).weight(.bold)).foregroundStyle(FortTheme.muted)
                    .padding(14).frame(maxWidth:.infinity,alignment:.leading).comicPanel()
            }
            LazyVGrid(columns:[GridItem(.adaptive(minimum:240),spacing:18)],alignment:.leading,spacing:18) {
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
                    Text(panel.line).font(.caption.weight(.heavy)).foregroundStyle(FortTheme.captionInk)
                }
                .padding(.horizontal,12).padding(.vertical,8)
                .background(stripColor(panel.tone))
                .overlay(alignment:.bottom) { FortTheme.line.frame(height:FortTheme.stroke) }
                HStack(spacing:4) { ForEach(panel.crew) { CrewBadge(model:model,agent:$0,size:28) } }
                    .padding(.horizontal,12).padding(.top,10).frame(minHeight:40)
                VStack(alignment:.leading,spacing:8) {
                    if panel.live.isEmpty { Text("Nothing running").font(.caption.weight(.bold)).foregroundStyle(FortTheme.muted) }
                    ForEach(panel.live) { run in
                        HStack(spacing:8) {
                            QuestMeter(filled:run.filled,tone:run.tone)
                            Text([run.agentName,run.text].compactMap { $0 }.joined(separator:": ")).font(.caption.weight(.bold)).lineLimit(2)
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
        switch tone { case .failed: Color(hex:0xF3A49C); case .needsYou: FortTheme.needsYou; case .working: Color(hex:0xBDF2F7); default: FortTheme.bar }
    }

    private var statusBar: some View {
        let header = ChatQuiet.header(connected:model.connected,preview:model.isOfflinePreview || model.isInteractiveFixture,runtimeState:model.runtimeReadiness?.state,status:model.status)
        return HStack(spacing:8) {
            LampView(lamp:header.lamp)
            Text(header.line).font(.caption.weight(.bold)).foregroundStyle(FortTheme.muted).lineLimit(1)
            Spacer()
        }
        .padding(.horizontal,16).padding(.vertical,8)
        .background(FortTheme.bar.ignoresSafeArea(edges:.bottom))
        .overlay(alignment:.top) { FortTheme.line.frame(height:3) }
        .accessibilityElement(children:.combine)
    }
}

/// A quest mark: a gold diamond with a bang, red when the turn failed, hollow while it waits on the chat.
struct QuestMark: View {
    let kind: QuestLog.Kind
    var body: some View {
        let fill = switch kind { case .approval: FortTheme.needsYou; case .failed: FortTheme.failed; case .waiting: FortTheme.panel }
        ZStack {
            Rectangle().fill(fill).overlay(Rectangle().stroke(FortTheme.line,lineWidth:FortTheme.stroke)).rotationEffect(.degrees(45)).frame(width:20,height:20)
            Text("!").font(.system(size:12,weight:.black,design:.rounded)).foregroundStyle(kind == .failed ? Color.white : FortTheme.captionInk)
        }.frame(width:28,height:28).accessibilityHidden(true)
    }
}

/// Three pips, the RunMeter steps: cyan with a holographic glow while working, gold when it needs you.
struct QuestMeter: View {
    let filled: Int
    let tone: QuestLog.Tone
    var body: some View {
        let fill = switch tone { case .working: FortTheme.working; case .needsYou: FortTheme.needsYou; case .done: FortTheme.done; case .failed: FortTheme.failed; default: FortTheme.off }
        HStack(spacing:3) {
            ForEach(0..<3,id:\.self) { index in
                RoundedRectangle(cornerRadius:2).fill(index < filled ? fill : FortTheme.panel)
                    .overlay(RoundedRectangle(cornerRadius:2).stroke(FortTheme.line,lineWidth:2))
                    .frame(width:15,height:9)
                    .shadow(color:tone == .working && index < filled ? FortTheme.glow : .clear,radius:3)
            }
        }.accessibilityHidden(true)
    }
}

/// An Agent's portrait in an inked circle, or its initial on its crew colour until the portrait loads.
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
                Text(agent.name.trimmingCharacters(in:.whitespaces).prefix(1).uppercased()).font(.system(size:size * 0.45,weight:.black,design:.rounded)).foregroundStyle(Color(hex:0x1B1A24))
            }
        }
        .frame(width:size,height:size)
        .overlay(Circle().stroke(FortTheme.line,lineWidth:size < 24 ? 2 : FortTheme.stroke))
        .task(id:agent.portrait?.revision) { await model.loadPortrait(agent) }
        .help(agent.name)
        .accessibilityLabel(agent.name)
    }
}

/// A comic button: ink outline, hard offset shadow, and it presses down into the page.
struct ComicButtonStyle: ButtonStyle {
    var fill: Color = FortTheme.panel
    @Environment(\.isEnabled) private var isEnabled
    func makeBody(configuration: Configuration) -> some View {
        let shape = RoundedRectangle(cornerRadius:FortTheme.radius)
        configuration.label
            .font(.system(.callout,design:.rounded).weight(.heavy))
            .foregroundStyle(FortTheme.captionInk)
            .padding(.horizontal,12)
            #if os(iOS)
            .frame(minHeight:FortTheme.touch)
            #else
            .frame(minHeight:30)
            #endif
            .background(shape.fill(fill == FortTheme.panel ? Color(hex:0xFFFDF6) : fill))
            .overlay(shape.stroke(FortTheme.line,lineWidth:FortTheme.stroke))
            .background(shape.fill(FortTheme.line).offset(x:configuration.isPressed ? 0 : 2,y:configuration.isPressed ? 0 : 2))
            .offset(x:configuration.isPressed ? 2 : 0,y:configuration.isPressed ? 2 : 0)
            .opacity(isEnabled ? 1 : 0.5)
            .contentShape(shape)
    }
}
