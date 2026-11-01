import SwiftUI

/// Setup, the one place for everything off the map: Crew, Routines, Accounts and Devices, the same
/// sections in the same order as the web's Setup sheet, drawn on the same comic page.
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
                FortTheme.line.frame(width:3)
                NavigationStack { content(tab).background(FortTheme.paper) }.frame(maxWidth:.infinity,maxHeight:.infinity)
            }.frame(minWidth:760,minHeight:540)
            #else
            TabView(selection:$tab) {
                ForEach(menu.entries) { entry in
                    NavigationStack {
                        content(entry.tab).background(FortTheme.paper).toolbar { ToolbarItem(placement:.cancellationAction) { Button("Done") { dismiss() }.accessibilityIdentifier("pause-resume") } }
                    }
                    .tabItem { Label(entry.tab.title,systemImage:entry.tab.symbol) }.badge(entry.badge).tag(entry.tab)
                }
            }
            #endif
        }
        // The comic page in light and noir, following the system like the web does.
        .tint(FortTheme.accent)
        .foregroundStyle(FortTheme.ink)
        .environment(\.fortSurface,.comic)
        .sheet(isPresented:$showRoutines) { RoutineLibrary(model:model) }
        .sheet(isPresented:$showLibrary) { WorkspaceEditor(model:model) }
        #if os(macOS)
        .background(NonBlockingSheetWindow().allowsHitTesting(false).accessibilityHidden(true))
        #endif
    }

    private func nav(_ menu: PauseMenu) -> some View {
        VStack(alignment:.leading,spacing:6) {
            Text("Setup").font(FortTheme.display(22)).tracking(2.2).textCase(.uppercase).foregroundStyle(FortTheme.ink).padding(.bottom,10).accessibilityAddTraits(.isHeader)
            ForEach(menu.entries) { entry in
                let selected = tab == entry.tab
                let shape = RoundedRectangle(cornerRadius:FortTheme.radius)
                Button { tab = entry.tab } label: {
                    HStack(spacing:10) {
                        Image(systemName:entry.tab.symbol).frame(width:20)
                        Text(entry.tab.title).font(.callout.weight(.semibold))
                        Spacer(minLength:0)
                        if entry.badge > 0 { CaptionBox(text:"\(entry.badge)",fill:FortTheme.needsYou) }
                    }
                    .padding(.horizontal,10).padding(.vertical,8).frame(maxWidth:.infinity,alignment:.leading)
                    .background(shape.fill(selected ? FortTheme.caption : Color.clear))
                    .overlay(alignment:.leading) { FortTheme.accent.frame(width:selected ? 2 : 0).padding(.vertical,6) }
                    .foregroundStyle(selected ? FortTheme.ink : FortTheme.muted)
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .accessibilityIdentifier("pause-tab-\(entry.tab.rawValue)")
                .accessibilityAddTraits(selected ? .isSelected : [])
                .accessibilityValue(entry.badge > 0 ? "\(entry.badge) need you" : "")
            }
            Spacer(minLength:0)
            Button { dismiss() } label: { Text("Done").frame(maxWidth:.infinity) }
                .buttonStyle(ComicButtonStyle(fill:FortTheme.accent)).keyboardShortcut(.cancelAction).accessibilityIdentifier("pause-resume")
        }
        .padding(14).frame(maxHeight:.infinity,alignment:.top).background(FortTheme.bar)
    }

    @ViewBuilder private func content(_ tab: PauseTab) -> some View {
        switch tab {
        case .crew: crew
        case .playbook: playbook
        case .accounts: accounts
        case .devices: PairingSettingsView(model:model)
        }
    }

    /// The crew, listed like the web's Setup: each Agent's badge, name and "provider · model", then New agent.
    private var crew: some View {
        ScrollView {
            #if DEBUG
            if model.isInteractiveFixture { Form { InteractiveFixtureControls(model:model) }.formStyle(.grouped).frame(minHeight:150) }
            #endif
            VStack(alignment:.leading,spacing:12) {
                ForEach(model.agents) { agent in
                    NavigationLink { AgentSettingsEditor(model:model,agent:agent) } label: {
                        HStack { CrewRow(model:model,agent:agent,badge:44); Spacer(minLength:8); Image(systemName:"chevron.right").foregroundStyle(FortTheme.muted) }
                            .padding(12).frame(maxWidth:.infinity,alignment:.leading).comicPanel().contentShape(Rectangle())
                    }.buttonStyle(.plain).accessibilityIdentifier("crew-\(agent.agentId)")
                }
                NavigationLink { AgentSettingsEditor(model:model,agent:nil) } label: {
                    HStack(spacing:10) {
                        Text("+").font(.system(size:22,weight:.light)).frame(width:44,height:44)
                            .overlay(Circle().strokeBorder(FortTheme.line,style:StrokeStyle(lineWidth:1,dash:[4,3])))
                        VStack(alignment:.leading,spacing:2) {
                            Text("New agent").font(.body.weight(.semibold))
                            Text("Pick art, a name and a model").font(.caption.weight(.medium)).foregroundStyle(FortTheme.muted)
                        }
                    }
                    .padding(12).frame(maxWidth:.infinity,alignment:.leading)
                    .overlay(RoundedRectangle(cornerRadius:FortTheme.radius).strokeBorder(FortTheme.line,style:StrokeStyle(lineWidth:1,dash:[6,4])))
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
                    PauseRow(title:"Your routines",detail:"Plays your crew runs start to finish, and when they repeat.") {
                        Button("Open") { showRoutines = true }.modifier(GoldButton(on:true)).accessibilityIdentifier("playbook-routines")
                    }
                }
                if model.hasWorkspace && !model.isInteractiveFixture {
                    PauseRow(title:"Skills and conversations",detail:"Teach skills and set up shared conversations.") {
                        Button("Open") { showLibrary = true }.modifier(GoldButton(on:false)).accessibilityIdentifier("playbook-library")
                    }
                }
                if !model.supportsRoutines && (!model.hasWorkspace || model.isInteractiveFixture) {
                    Text("Nothing to set up here yet. Routines appear once this Fort supports them.").font(.callout).foregroundStyle(.secondary).frame(maxWidth:.infinity,alignment:.leading)
                }
            }.padding(18)
        }
        .navigationTitle("Routines")
    }

    private var accounts: some View {
        ScrollView {
            VStack(alignment:.leading,spacing:12) {
                Text("Fort only uses your own Claude and ChatGPT plans. It never falls back to API billing.").font(.callout).foregroundStyle(FortTheme.muted)
                #if os(macOS)
                AccountSignInPanel(model:model)
                #endif
            }.padding(18)
        }
        .navigationTitle("Accounts")
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
                Text(title).font(.headline.weight(.semibold))
                Text(detail).font(.callout).foregroundStyle(FortTheme.muted)
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
        Circle().fill(color).frame(width:8,height:8)
            .shadow(color:lamp == .off ? .clear : lamp == .waiting ? FortTheme.glow : color,radius:lamp == .waiting ? 4 : 3)
            .accessibilityLabel(label)
    }
    private var color: Color { lamp.color }
    private var label: String { switch lamp { case .ready: "Ready"; case .waiting: "In progress"; case .attention: "Needs you"; case .failed: "Failed"; case .off: "Not available" } }
}

/// One colour per state on every surface, the web's tokens: cyan working, gold needs you, green done, red failed.
extension StatusLamp {
    var color: Color { switch self { case .ready: FortTheme.done; case .waiting: FortTheme.working; case .attention: FortTheme.needsYou; case .failed: FortTheme.failed; case .off: FortTheme.off } }
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
