import SwiftUI

/// The game screen: a city for the live week with HUD panels around it. Every element reads real records; nothing here is decorative state.
struct CityHUDView: View {
    @Bindable var model: MessengerModel
    let showMessages: () -> Void
    let openMenu: () -> Void
    @State private var talkingTo: String?
    @State private var planning = false
    init(model: MessengerModel, showMessages: @escaping () -> Void, openMenu: @escaping () -> Void) { self.model = model; self.showMessages = showMessages; self.openMenu = openMenu }

    private var currentHUD: CityHUD { CityHUD.make(week:model.liveWeek,agents:model.agents,snapshot:model.displaySnapshot,today:Self.today()) }
    static func today(_ date: Date = Date()) -> String {
        let parts = Calendar.current.dateComponents([.year,.month,.day],from:date)
        return String(format:"%04d-%02d-%02d",parts.year ?? 0,parts.month ?? 0,parts.day ?? 0)
    }

    var body: some View {
        let hud = currentHUD
        ZStack {
            CityMap(week:model.liveWeek).ignoresSafeArea()
            VStack(spacing:0) {
                topBar(hud)
                HStack(alignment:.top,spacing:12) {
                    party(hud).frame(width:220)
                    Spacer(minLength:0)
                    VStack(spacing:12) { goals(hud); orders(hud) }.frame(width:300)
                }.padding(12)
                Spacer(minLength:0)
                if let id = talkingTo, let agent = model.agents.first(where:{ $0.agentId == id }) { dialogue(agent).padding(.horizontal,240).padding(.bottom,16) }
            }
            if model.liveWeek == nil { emptyWeek }
        }
        .background(HUDStyle.ground)
        .environment(\.colorScheme,.dark)
        .sheet(isPresented:$planning) { PlannerView(model:model) }
    }

    private func topBar(_ hud: CityHUD) -> some View {
        HStack(spacing:16) {
            Text("FORT").font(.system(size:22,weight:.black,design:.rounded)).foregroundStyle(HUDStyle.gold)
            VStack(alignment:.leading,spacing:2) {
                Text(hud.title).font(.system(.headline,design:.rounded).weight(.heavy))
                if let day = hud.dayLabel, let left = hud.daysLeft { Text("\(day) · \(left == 1 ? "1 day" : "\(left) days") left").font(.caption).foregroundStyle(.secondary) }
            }
            Spacer()
            Label("\(hud.points) pts",systemImage:"diamond.fill").font(.system(.headline,design:.rounded).weight(.bold)).foregroundStyle(HUDStyle.gold)
            if let grade = hud.grade { Text(grade).font(.system(.title3,design:.rounded).weight(.black)).padding(.horizontal,8).background(HUDStyle.gold,in:RoundedRectangle(cornerRadius:6)).foregroundStyle(.black).accessibilityLabel("Grade \(grade)") }
            Button("Plan Week \((model.weeks.first?.number ?? 0) + 1)",systemImage:"map") { planning = true }.buttonStyle(.borderedProminent).tint(HUDStyle.gold).foregroundStyle(.black).disabled(!model.supportsCommands).accessibilityIdentifier("hud-plan")
            Button("Messages",systemImage:"bubble.left.and.bubble.right",action:showMessages).buttonStyle(.bordered).accessibilityIdentifier("hud-messages")
            // Esc pauses, like a game, unless a conversation is open: then Esc closes the conversation first.
            Button("Menu",systemImage:"line.3.horizontal",action:openMenu).buttonStyle(.bordered).tint(HUDStyle.gold)
                .keyboardShortcut(talkingTo == nil ? .cancelAction : KeyboardShortcut(",",modifiers:.command)).accessibilityIdentifier("pause-menu-open")
        }
        .padding(.horizontal,16).padding(.vertical,10)
        .background(HUDStyle.panel).overlay(alignment:.bottom) { HUDStyle.gold.frame(height:2) }
    }

    private func party(_ hud: CityHUD) -> some View {
        VStack(spacing:10) {
            ForEach(Array(hud.members.enumerated()),id:\.element.id) { index, member in
                Button { talk(member.id) } label: {
                    HStack(spacing:10) {
                        AgentAvatarView(avatar:member.avatar,size:44)
                        VStack(alignment:.leading,spacing:3) {
                            HStack { Text(member.name).font(.system(.headline,design:.rounded).weight(.heavy)); Spacer(); if index < 9 { Text("⌘\(index + 1)").font(.caption2.monospaced()).padding(.horizontal,4).overlay(RoundedRectangle(cornerRadius:3).stroke(.secondary)) } }
                            Text(stateLabel(member.state)).font(.caption2.weight(.bold)).foregroundStyle(stateColor(member.state))
                            if let next = member.nextTask { Text(next).font(.caption).foregroundStyle(.secondary).lineLimit(1) }
                        }
                    }.padding(10).frame(maxWidth:.infinity,alignment:.leading)
                }
                .buttonStyle(.plain).hudFrame(highlight:talkingTo == member.id)
                .accessibilityIdentifier("hud-member-\(member.id)")
                .keyboardShortcut(KeyEquivalent(Character(String(min(index + 1,9)))),modifiers:.command)
            }
        }
    }
    private func goals(_ hud: CityHUD) -> some View {
        VStack(alignment:.leading,spacing:10) {
            Text("THIS WEEK'S GOALS").font(.caption2.weight(.heavy)).foregroundStyle(HUDStyle.gold)
            if hud.goals.isEmpty { Text("No goals yet.").font(.callout).foregroundStyle(.secondary) }
            ForEach(hud.goals) { goal in
                VStack(alignment:.leading,spacing:4) {
                    HStack {
                        RoundedRectangle(cornerRadius:2).fill(HUDStyle.zone(goal.zone)).frame(width:10,height:10)
                        Text(goal.text).font(.callout.weight(.semibold)).lineLimit(1)
                        if goal.isMain { Image(systemName:"star.fill").font(.caption2).foregroundStyle(HUDStyle.gold).accessibilityLabel("Main quest") }
                        Spacer(); Text("\(goal.points)/\(goal.targetPoints)").font(.caption.monospacedDigit()).foregroundStyle(.secondary)
                    }
                    ProgressView(value:goal.fraction).tint(HUDStyle.zone(goal.zone))
                }
            }
        }.padding(12).hudFrame()
    }
    private func orders(_ hud: CityHUD) -> some View {
        VStack(alignment:.leading,spacing:10) {
            Text("ORDERS").font(.caption2.weight(.heavy)).foregroundStyle(HUDStyle.gold)
            if hud.orders.isEmpty { Text("Nothing is waiting on you.").font(.callout).foregroundStyle(.secondary) }
            ForEach(hud.orders) { order in
                HStack(alignment:.top,spacing:10) {
                    switch order.kind {
                    case .decision: Image(systemName:"diamond").foregroundStyle(.purple)
                    case .quest(let item): Button { Task { _ = await model.completeQuest(item) } } label: { Image(systemName:"square") }.buttonStyle(.plain).foregroundStyle(HUDStyle.gold).disabled(model.isBusy).accessibilityLabel("Mark done: \(item.task)").accessibilityIdentifier("hud-quest-\(item.itemId)")
                    }
                    VStack(alignment:.leading,spacing:2) {
                        Text(order.title).font(.callout.weight(.semibold))
                        Text(order.detail).font(.caption).foregroundStyle(.secondary)
                    }
                    Spacer(minLength:0)
                    if case .decision(let approval) = order.kind, approval.canDecide {
                        Button("Approve") { Task { await model.decide(approval,approve:true) } }.buttonStyle(.borderedProminent).controlSize(.small)
                    }
                }
            }
        }.padding(12).hudFrame()
    }
    private func dialogue(_ agent: Agent) -> some View {
        let messages = model.activeConversationID == agent.homeConversationId ? Array((model.displaySnapshot?.messages ?? []).suffix(4)) : []
        return HStack(alignment:.top,spacing:14) {
            AgentAvatarView(avatar:agent.avatar ?? .orbit,size:88)
            VStack(alignment:.leading,spacing:8) {
                HStack { Text(agent.name.uppercased()).font(.system(.title3,design:.rounded).weight(.black)).foregroundStyle(HUDStyle.gold); Spacer(); Button("Close",systemImage:"xmark") { talkingTo = nil }.labelStyle(.iconOnly).keyboardShortcut(.cancelAction) }
                if messages.isEmpty { Text("Say something to \(agent.name).").foregroundStyle(.secondary) }
                ForEach(messages) { message in
                    Text(message.text).font(.callout).foregroundStyle(message.role == "user" ? .secondary : .primary).lineLimit(4).frame(maxWidth:.infinity,alignment:message.role == "user" ? .trailing : .leading)
                }
                HStack {
                    TextField("Message \(agent.name)",text:$model.draft).textFieldStyle(.roundedBorder).onSubmit(send).accessibilityIdentifier("hud-composer")
                    Button("Send",action:send).buttonStyle(.borderedProminent).disabled(!model.canSend)
                }
            }
        }.padding(14).hudFrame(highlight:true)
    }
    private var emptyWeek: some View {
        VStack(spacing:8) {
            Text("No week planned yet").font(.system(.title2,design:.rounded).weight(.black))
            Text("Pick this week's goals and lay out the blueprint. The city builds itself from what gets done.").font(.callout).foregroundStyle(.secondary).multilineTextAlignment(.center)
            Button("Plan a week") { planning = true }.buttonStyle(.borderedProminent).tint(HUDStyle.gold).foregroundStyle(.black).disabled(!model.supportsCommands)
        }.padding(20).frame(maxWidth:360).hudFrame()
    }
    private func talk(_ agentID: String) {
        talkingTo = agentID
        guard let agent = model.agents.first(where:{ $0.agentId == agentID }) else { return }
        if model.hasWorkspace { Task { await model.selectConversation(agent.homeConversationId) } } else { model.selectedAgentID = agentID }
    }
    private func send() { guard model.canSend else { return }; Task { await model.send() } }
    private func stateLabel(_ state: CityHUD.MemberState) -> String { switch state { case .ready: "READY"; case .busy: "WORKING"; case .needsYou: "NEEDS YOU" } }
    private func stateColor(_ state: CityHUD.MemberState) -> Color { switch state { case .ready: .green; case .busy: .yellow; case .needsYou: .red } }
}

enum HUDStyle {
    static let gold = Color(red:0.96,green:0.78,blue:0.26)
    static let panel = Color(red:0.11,green:0.12,blue:0.16).opacity(0.94)
    static let ground = Color(red:0.55,green:0.70,blue:0.45)
    static func zone(_ zone: WeekZone) -> Color {
        switch zone { case .code: Color(red:0.93,green:0.75,blue:0.30); case .money: Color(red:0.36,green:0.60,blue:0.92); case .writing: Color(red:0.42,green:0.75,blue:0.38); case .personal: Color(red:0.25,green:0.72,blue:0.70) }
    }
}
struct HUDFrame: ViewModifier {
    let highlight: Bool
    func body(content: Content) -> some View {
        content.background(HUDStyle.panel,in:RoundedRectangle(cornerRadius:10))
            .overlay(RoundedRectangle(cornerRadius:10).stroke(highlight ? HUDStyle.gold : HUDStyle.gold.opacity(0.45),lineWidth:highlight ? 2 : 1))
    }
}
extension View { func hudFrame(highlight: Bool = false) -> some View { modifier(HUDFrame(highlight:highlight)) } }

/// A flat isometric stand-in for the city: one block per build, faded blocks for planned work. Real sprites replace this in phase 4.
struct CityMap: View {
    let week: WeekView?
    var body: some View {
        Canvas { context, size in
            let tile = CGSize(width:64,height:32), grid = 10
            let origin = CGPoint(x:size.width/2,y:size.height/2 - CGFloat(grid) * tile.height/2)
            func point(_ x: Int, _ y: Int) -> CGPoint { CGPoint(x:origin.x + CGFloat(x - y) * tile.width/2,y:origin.y + CGFloat(x + y) * tile.height/2) }
            func diamond(_ x: Int, _ y: Int, lift: CGFloat = 0) -> Path {
                let p = point(x,y)
                return Path { path in
                    path.move(to:CGPoint(x:p.x,y:p.y - lift)); path.addLine(to:CGPoint(x:p.x + tile.width/2,y:p.y + tile.height/2 - lift))
                    path.addLine(to:CGPoint(x:p.x,y:p.y + tile.height - lift)); path.addLine(to:CGPoint(x:p.x - tile.width/2,y:p.y + tile.height/2 - lift)); path.closeSubpath()
                }
            }
            for x in 0..<grid { for y in 0..<grid {
                context.fill(diamond(x,y),with:.color(Color(red:0.47,green:0.66,blue:0.40)))
                context.stroke(diamond(x,y),with:.color(.black.opacity(0.08)),lineWidth:1)
            } }
            guard let week else { return }
            let blocks = CityMap.layout(week)
            for block in blocks.sorted(by:{ $0.x + $0.y < $1.x + $1.y }) {
                let color = HUDStyle.zone(block.zone), height = CGFloat(block.size.points) * 4 + 6, p = point(block.x,block.y)
                var layer = context; layer.opacity = block.planned ? 0.32 : 1
                let left = Path { path in path.move(to:CGPoint(x:p.x - tile.width/2,y:p.y + tile.height/2)); path.addLine(to:CGPoint(x:p.x,y:p.y + tile.height)); path.addLine(to:CGPoint(x:p.x,y:p.y + tile.height - height)); path.addLine(to:CGPoint(x:p.x - tile.width/2,y:p.y + tile.height/2 - height)); path.closeSubpath() }
                let right = Path { path in path.move(to:CGPoint(x:p.x + tile.width/2,y:p.y + tile.height/2)); path.addLine(to:CGPoint(x:p.x,y:p.y + tile.height)); path.addLine(to:CGPoint(x:p.x,y:p.y + tile.height - height)); path.addLine(to:CGPoint(x:p.x + tile.width/2,y:p.y + tile.height/2 - height)); path.closeSubpath() }
                layer.fill(left,with:.color(color.opacity(0.75))); layer.fill(right,with:.color(color.opacity(0.55)))
                layer.fill(diamond(block.x,block.y,lift:height),with:.color(color))
            }
        }.accessibilityLabel(week.map { "City for week \($0.number), \($0.builds.count) buildings, \($0.plan.filter { $0.doneBuildId == nil }.count) planned" } ?? "Empty city")
    }
    struct Block: Equatable, Sendable { let x: Int; let y: Int; let zone: WeekZone; let size: WeekSize; let planned: Bool }
    /// Each goal gets two rows of lots; builds fill them first, then planned work as ghosts.
    nonisolated static func layout(_ week: WeekView) -> [Block] {
        week.goals.prefix(5).enumerated().flatMap { index, goal -> [Block] in
            let built = week.builds.filter { $0.goalId == goal.goalId }.map { ($0.size,false) }
            let planned = week.plan.filter { $0.goalId == goal.goalId && $0.doneBuildId == nil }.map { ($0.size,true) }
            return (built + planned).prefix(20).enumerated().map { lot, entry in Block(x:lot % 10,y:index * 2 + lot / 10,zone:goal.zone,size:entry.0,planned:entry.1) }
        }
    }
}
