import SwiftUI

/// The one menu for everything off the map: Crew, Playbook, Accounts and Devices.
/// Each tab hosts the existing screens, so their checks, confirmations and recovery steps are unchanged.
struct PauseMenuView: View {
    @Bindable var model: MessengerModel
    @State private var tab: PauseTab
    @State private var showRoutines = false
    @State private var showLibrary = false
    @Environment(\.dismiss) private var dismiss
    init(model: MessengerModel, tab: PauseTab? = nil) {
        self.model = model
        _tab = State(initialValue:PauseMenu.make(model.pauseFacts).opening(tab) ?? .devices)
    }

    var body: some View {
        let menu = PauseMenu.make(model.pauseFacts)
        Group {
            #if os(macOS)
            HStack(spacing:0) {
                nav(menu).frame(width:190)
                HUDStyle.gold.frame(width:2)
                NavigationStack { content(tab) }.frame(maxWidth:.infinity,maxHeight:.infinity)
            }.frame(minWidth:760,minHeight:540)
            #else
            TabView(selection:$tab) {
                ForEach(menu.entries) { entry in
                    NavigationStack {
                        content(entry.tab).toolbar { ToolbarItem(placement:.cancellationAction) { Button("Resume") { dismiss() }.accessibilityIdentifier("pause-resume") } }
                    }
                    .tabItem { Label(entry.tab.title,systemImage:entry.tab.symbol) }.badge(entry.badge).tag(entry.tab)
                }
            }
            #endif
        }
        .tint(HUDStyle.gold)
        .environment(\.colorScheme,.dark)
        .sheet(isPresented:$showRoutines) { RoutineLibrary(model:model) }
        .sheet(isPresented:$showLibrary) { WorkspaceEditor(model:model) }
        #if os(macOS)
        .background(NonBlockingSheetWindow().allowsHitTesting(false).accessibilityHidden(true))
        #endif
    }

    private func nav(_ menu: PauseMenu) -> some View {
        VStack(alignment:.leading,spacing:6) {
            Text("PAUSED").font(.system(size:30,weight:.black,design:.rounded)).foregroundStyle(HUDStyle.gold).padding(.bottom,10)
            ForEach(menu.entries) { entry in
                let selected = tab == entry.tab
                Button { tab = entry.tab } label: {
                    HStack(spacing:10) {
                        Image(systemName:entry.tab.symbol).frame(width:20)
                        Text(entry.tab.title.uppercased()).font(.system(.callout,design:.rounded).weight(.heavy))
                        Spacer(minLength:0)
                        if entry.badge > 0 { Text("\(entry.badge)").font(.caption.weight(.black)).padding(.horizontal,5).background(.red,in:RoundedRectangle(cornerRadius:3)).foregroundStyle(.black) }
                    }
                    .padding(.horizontal,10).padding(.vertical,8).frame(maxWidth:.infinity,alignment:.leading)
                    .background(selected ? HUDStyle.gold : .clear,in:RoundedRectangle(cornerRadius:5))
                    .foregroundStyle(selected ? Color.black : Color.primary)
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .accessibilityIdentifier("pause-tab-\(entry.tab.rawValue)")
                .accessibilityAddTraits(selected ? .isSelected : [])
                .accessibilityValue(entry.badge > 0 ? "\(entry.badge) need you" : "")
            }
            Spacer(minLength:0)
            Button { dismiss() } label: { Text("RESUME").font(.system(.callout,design:.rounded).weight(.heavy)).frame(maxWidth:.infinity) }
                .buttonStyle(.bordered).keyboardShortcut(.cancelAction).accessibilityIdentifier("pause-resume")
        }
        .padding(14).frame(maxHeight:.infinity,alignment:.top).background(HUDStyle.panel)
    }

    @ViewBuilder private func content(_ tab: PauseTab) -> some View {
        switch tab {
        case .crew: crew
        case .playbook: playbook
        case .accounts: accounts
        case .devices: PairingSettingsView(model:model)
        }
    }

    private var crew: some View {
        ScrollView {
            #if DEBUG
            if model.isInteractiveFixture { Form { InteractiveFixtureControls(model:model) }.formStyle(.grouped).frame(minHeight:150) }
            #endif
            LazyVGrid(columns:[GridItem(.adaptive(minimum:160),spacing:12)],spacing:12) {
                ForEach(model.agents) { agent in
                    NavigationLink { AgentSettingsEditor(model:model,agent:agent) } label: {
                        CrewCard(model:model,agent:agent,className:PauseMenu.className(provider:agent.binding.provider,runtimes:model.runtimes))
                    }.buttonStyle(.plain).accessibilityIdentifier("crew-\(agent.agentId)")
                }
                NavigationLink { AgentSettingsEditor(model:model,agent:nil) } label: {
                    VStack(spacing:6) {
                        Text("+").font(.system(size:44,weight:.black,design:.rounded)).foregroundStyle(HUDStyle.gold)
                        Text("RECRUIT").font(.system(.title3,design:.rounded).weight(.black))
                        Text("Pick art, a name and a class").font(.caption).foregroundStyle(.secondary)
                    }
                    .frame(maxWidth:.infinity,minHeight:170).padding(12)
                    .overlay(RoundedRectangle(cornerRadius:10).strokeBorder(style:StrokeStyle(lineWidth:1.5,dash:[6,4])).foregroundStyle(.secondary))
                    .contentShape(Rectangle())
                }.buttonStyle(.plain).accessibilityIdentifier("crew-recruit")
            }.padding(18)
        }
        .navigationTitle("Crew")
    }

    private var playbook: some View {
        ScrollView {
            VStack(spacing:10) {
                if model.supportsRoutines {
                    PauseRow(title:"Routines",detail:"Plays your crew runs start to finish, and when they repeat.") {
                        Button("Open") { showRoutines = true }.buttonStyle(.borderedProminent).foregroundStyle(.black).accessibilityIdentifier("playbook-routines")
                    }
                }
                if model.hasWorkspace && !model.isInteractiveFixture {
                    PauseRow(title:"Skills and conversations",detail:"Teach skills and set up shared conversations.") {
                        Button("Open") { showLibrary = true }.buttonStyle(.bordered).accessibilityIdentifier("playbook-library")
                    }
                }
                if !model.supportsRoutines && (!model.hasWorkspace || model.isInteractiveFixture) {
                    Text("Nothing to set up here yet. Plays appear once this Fort supports routines.").font(.callout).foregroundStyle(.secondary).frame(maxWidth:.infinity,alignment:.leading)
                }
            }.padding(18)
        }
        .navigationTitle("Playbook")
    }

    private var accounts: some View {
        ScrollView {
            VStack(alignment:.leading,spacing:12) {
                Text("Fort only uses your own Claude and ChatGPT plans. It never falls back to API billing.").font(.callout).foregroundStyle(.secondary)
                #if os(macOS)
                AccountSignInPanel(model:model)
                #endif
            }.padding(18)
        }
        .navigationTitle("Accounts")
    }
}

private struct CrewCard: View {
    let model: MessengerModel
    let agent: Agent
    let className: String
    var body: some View {
        VStack(alignment:.leading,spacing:8) {
            AgentPortraitView(model:model,agent:agent,size:72)
            HStack(spacing:6) {
                Text(agent.name.uppercased()).font(.system(.title3,design:.rounded).weight(.black)).lineLimit(1)
                Spacer(minLength:4)
                Text(className.uppercased()).font(.caption2.weight(.heavy)).lineLimit(1)
                    .padding(.horizontal,5).padding(.vertical,2).background(HUDStyle.gold,in:RoundedRectangle(cornerRadius:3)).foregroundStyle(.black)
            }
            Text(agent.binding.selectedModel).font(.caption).foregroundStyle(.secondary).lineLimit(1)
        }
        .padding(12).frame(maxWidth:.infinity,alignment:.leading).hudFrame().contentShape(Rectangle())
        .accessibilityElement(children:.combine)
    }
}

/// A status row in the pause menu: what it is, one plain sentence, and its action.
struct PauseRow<Action: View>: View {
    let title: String
    let detail: String
    @ViewBuilder let action: () -> Action
    var body: some View {
        HStack(spacing:12) {
            VStack(alignment:.leading,spacing:3) {
                Text(title.uppercased()).font(.system(.headline,design:.rounded).weight(.black))
                Text(detail).font(.callout).foregroundStyle(.secondary)
            }
            Spacer(minLength:8)
            action()
        }
        .padding(12).hudFrame()
    }
}

struct LampView: View {
    let lamp: StatusLamp
    var body: some View {
        Circle().fill(color).frame(width:11,height:11).shadow(color:color.opacity(lamp == .off ? 0 : 0.8),radius:4)
            .accessibilityLabel(label)
    }
    private var color: Color { switch lamp { case .ready: .green; case .waiting: .cyan; case .attention: .red; case .off: .gray } }
    private var label: String { switch lamp { case .ready: "Ready"; case .waiting: "In progress"; case .attention: "Needs you"; case .off: "Not available" } }
}

/// Recovery and rarely used controls, tucked under a dashed drawer at the bottom of a tab. Nothing in it is removed, only moved.
struct WorkshopDrawer<Content: View>: View {
    @SwiftUI.Binding var isExpanded: Bool
    var forceOpen = false
    let identifier: String
    @ViewBuilder let content: () -> Content
    var body: some View {
        let open = isExpanded || forceOpen
        VStack(alignment:.leading,spacing:8) {
            Button { isExpanded.toggle() } label: {
                Label("WORKSHOP",systemImage:open ? "chevron.down" : "chevron.right").font(.caption.weight(.heavy)).foregroundStyle(.secondary)
            }
            .buttonStyle(.plain).disabled(forceOpen)
            .accessibilityValue(open ? "Expanded" : "Collapsed").accessibilityIdentifier(identifier)
            if open { content() }
        }
        .padding(10).frame(maxWidth:.infinity,alignment:.leading)
        .overlay(RoundedRectangle(cornerRadius:6).strokeBorder(style:StrokeStyle(lineWidth:1,dash:[5,4])).foregroundStyle(Color.secondary.opacity(0.6)))
    }
}

struct OptionalHUDFrame: ViewModifier {
    let on: Bool
    func body(content: Content) -> some View { if on { content.hudFrame() } else { content } }
}

extension MessengerModel {
    var pauseFacts: PauseMenu.Facts {
        PauseMenu.Facts(
            hasWorkspace:hasWorkspace,
            supportsRoutines:supportsRoutines,
            canManageAccount:canManageProviderAccount,
            accountSelected:accountSignInStatus?.accountSelected == true,
            showsDevices:!isOfflinePreview,
            runtimeState:runtimeReadiness?.state,
            pendingDecisions:(displaySnapshot?.approvals ?? []).filter { $0.state == "pending" }.count
        )
    }
}
