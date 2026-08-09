//
//  PrimaryChannelsView.swift
//  FortKit
//
//  Shared native Phase 1 surface for private Primary Channels (Spec 044).
//  The same hierarchy and recovery contract ships on macOS and iPhone; only
//  the locally selected presentation theme changes.
//

import SwiftUI

#if canImport(Combine)
import Combine
#endif

// MARK: - Local presentation themes

public enum PrimaryTheme: String, CaseIterable, Identifiable {
    case quietIntelligence = "quiet-intelligence"
    case privateChannels = "private-channels"
    case nativeDaylight = "native-daylight"

    public var id: String { rawValue }

    public var title: String {
        switch self {
        case .quietIntelligence: return "Quiet Intelligence"
        case .privateChannels: return "Private Channels"
        case .nativeDaylight: return "Native Daylight"
        }
    }

    fileprivate var colorScheme: ColorScheme {
        self == .nativeDaylight ? .light : .dark
    }

    fileprivate var canvas: Color {
        switch self {
        case .quietIntelligence: return Color(red: 3 / 255, green: 16 / 255, blue: 29 / 255)
        case .privateChannels: return Color(red: 18 / 255, green: 23 / 255, blue: 21 / 255)
        case .nativeDaylight: return Color(red: 247 / 255, green: 248 / 255, blue: 250 / 255)
        }
    }

    fileprivate var panel: Color {
        switch self {
        case .quietIntelligence: return Color(red: 8 / 255, green: 26 / 255, blue: 42 / 255)
        case .privateChannels: return Color(red: 25 / 255, green: 31 / 255, blue: 28 / 255)
        case .nativeDaylight: return .white
        }
    }

    fileprivate var raised: Color {
        switch self {
        case .quietIntelligence: return Color(red: 11 / 255, green: 32 / 255, blue: 51 / 255)
        case .privateChannels: return Color(red: 32 / 255, green: 40 / 255, blue: 33 / 255)
        case .nativeDaylight: return Color(red: 241 / 255, green: 245 / 255, blue: 250 / 255)
        }
    }

    fileprivate var line: Color {
        switch self {
        case .quietIntelligence: return Color(red: 23 / 255, green: 51 / 255, blue: 75 / 255)
        case .privateChannels: return Color(red: 48 / 255, green: 58 / 255, blue: 49 / 255)
        case .nativeDaylight: return Color(red: 217 / 255, green: 225 / 255, blue: 235 / 255)
        }
    }

    fileprivate var accent: Color {
        switch self {
        case .quietIntelligence: return Color(red: 42 / 255, green: 141 / 255, blue: 255 / 255)
        case .privateChannels: return Color(red: 195 / 255, green: 227 / 255, blue: 92 / 255)
        case .nativeDaylight: return Color(red: 8 / 255, green: 120 / 255, blue: 249 / 255)
        }
    }

    fileprivate var primaryText: Color {
        switch self {
        case .quietIntelligence: return Color(red: 237 / 255, green: 246 / 255, blue: 255 / 255)
        case .privateChannels: return Color(red: 241 / 255, green: 244 / 255, blue: 236 / 255)
        case .nativeDaylight: return Color(red: 23 / 255, green: 33 / 255, blue: 43 / 255)
        }
    }

    fileprivate var secondaryText: Color {
        switch self {
        case .quietIntelligence: return Color(red: 202 / 255, green: 216 / 255, blue: 230 / 255)
        case .privateChannels: return Color(red: 216 / 255, green: 221 / 255, blue: 210 / 255)
        case .nativeDaylight: return Color(red: 52 / 255, green: 66 / 255, blue: 82 / 255)
        }
    }

    fileprivate var mutedText: Color {
        switch self {
        case .quietIntelligence: return Color(red: 141 / 255, green: 161 / 255, blue: 181 / 255)
        case .privateChannels: return Color(red: 157 / 255, green: 167 / 255, blue: 154 / 255)
        case .nativeDaylight: return Color(red: 104 / 255, green: 119 / 255, blue: 137 / 255)
        }
    }

    fileprivate var accentInk: Color {
        self == .privateChannels ? Color(red: 22 / 255, green: 32 / 255, blue: 7 / 255) : .white
    }

    fileprivate var success: Color {
        switch self {
        case .quietIntelligence: return Color(red: 107 / 255, green: 201 / 255, blue: 154 / 255)
        case .privateChannels: return Color(red: 168 / 255, green: 205 / 255, blue: 101 / 255)
        case .nativeDaylight: return Color(red: 37 / 255, green: 131 / 255, blue: 77 / 255)
        }
    }

    fileprivate var warning: Color {
        switch self {
        case .quietIntelligence: return Color(red: 231 / 255, green: 189 / 255, blue: 99 / 255)
        case .privateChannels: return Color(red: 242 / 255, green: 181 / 255, blue: 74 / 255)
        case .nativeDaylight: return Color(red: 155 / 255, green: 107 / 255, blue: 17 / 255)
        }
    }

    fileprivate var failure: Color {
        switch self {
        case .quietIntelligence: return Color(red: 238 / 255, green: 124 / 255, blue: 121 / 255)
        case .privateChannels: return Color(red: 231 / 255, green: 133 / 255, blue: 104 / 255)
        case .nativeDaylight: return Color(red: 189 / 255, green: 56 / 255, blue: 56 / 255)
        }
    }
}

private struct PrimaryThemeKey: EnvironmentKey {
    static let defaultValue = PrimaryTheme.quietIntelligence
}

private extension EnvironmentValues {
    var primaryTheme: PrimaryTheme {
        get { self[PrimaryThemeKey.self] }
        set { self[PrimaryThemeKey.self] = newValue }
    }
}

private struct PrimaryConnectionSettingsKey: EnvironmentKey {
    static let defaultValue: (() -> Void)? = nil
}

private extension EnvironmentValues {
    var primaryConnectionSettings: (() -> Void)? {
        get { self[PrimaryConnectionSettingsKey.self] }
        set { self[PrimaryConnectionSettingsKey.self] = newValue }
    }
}

public extension View {
    /// Keeps iPhone encrypted-machine connection management reachable from
    /// Phase 1 Settings.
    func primaryConnectionSettings(_ action: @escaping () -> Void) -> some View {
        environment(\.primaryConnectionSettings, action)
    }
}

/// Applies only the approved material treatment to the existing Fort orbital
/// core. Geometry and Working-only motion remain owned by FortAgentOrbView.
private struct PrimaryOrb: View {
    @Environment(\.primaryTheme) private var theme
    let name: String
    let state: PrimaryOrbState
    let size: CGFloat

    var body: some View {
        FortAgentOrbView(name: name, state: state, size: size)
            .hueRotation(.degrees(theme == .privateChannels ? 52 : 0))
            .saturation(theme == .privateChannels ? 1.2 : (theme == .nativeDaylight ? 0.82 : 1))
            .brightness(theme == .nativeDaylight ? 0.18 : 0)
    }
}

#if os(macOS)
private struct PrimaryServiceControllerKey: EnvironmentKey {
    static let defaultValue: ServiceController? = nil
}

private extension EnvironmentValues {
    var primaryServiceController: ServiceController? {
        get { self[PrimaryServiceControllerKey.self] }
        set { self[PrimaryServiceControllerKey.self] = newValue }
    }
}

public extension View {
    /// Supplies the existing daemon controls to the shared Settings surface on macOS.
    func primaryServiceController(_ controller: ServiceController) -> some View {
        environment(\.primaryServiceController, controller)
    }
}
#endif

// MARK: - Root and navigation

private enum PrimaryDestination: Hashable {
    case channel(String)
    case scheduled
    case needsYou
    case settings
}

private enum PrimaryPhoneTab: Hashable {
    case channels
    case scheduled
    case needsYou
    case settings
}

/// One shared Phase 1 surface. iPhone uses the approved four system tabs while
/// macOS uses a Channels split view; both containers reuse the exact same
/// transcript, schedule, recovery, settings, and model behavior.
public struct PrimaryChannelsView: View {
    @EnvironmentObject private var client: FortClient
    @Environment(\.primaryConnectionSettings) private var connectionSettings
    @StateObject private var model = PrimaryChannelsModel()
    @AppStorage("fort.primary.theme.v1") private var storedTheme = PrimaryTheme.quietIntelligence.rawValue
    @State private var showNewChannel = false

    public init() {}

    private var theme: PrimaryTheme {
        PrimaryTheme(rawValue: storedTheme) ?? .quietIntelligence
    }

    public var body: some View {
        platformContainer
        .environment(\.primaryTheme, theme)
        .preferredColorScheme(theme.colorScheme)
        .tint(theme.accent)
        .foregroundStyle(theme.primaryText)
        .task(id: "\(client.baseURL.absoluteString)|\(client.transportGeneration)") {
            await model.run(client: client)
        }
        .task(id: "\(client.baseURL.absoluteString)|\(client.transportGeneration)|\(model.selectedChannelID ?? "")") {
            await model.consumeSelectedChannelEvents(client: client)
        }
        .onChange(of: model.destination) { destination in
            Task { await model.open(destination, client: client) }
        }
        .sheet(isPresented: $showNewChannel) {
            PrimaryNewChannelSheet { name in
                await model.createChannel(name: name, client: client)
            }
            .environment(\.primaryTheme, theme)
            .preferredColorScheme(theme.colorScheme)
            .tint(theme.accent)
        }
        .overlay(alignment: .top) {
            if let message = model.errorMessage {
                PrimaryErrorBanner(message: message) { model.errorMessage = nil }
                    .padding(12)
            }
        }
    }

    @ViewBuilder
    private var platformContainer: some View {
        #if os(iOS)
        phoneContainer
        #else
        desktopContainer
        #endif
    }

    private var desktopContainer: some View {
        NavigationSplitView {
            PrimaryChannelSidebar(
                channels: model.channels,
                archivedChannels: model.archivedChannels,
                needsYouCount: model.needsYou.count,
                scheduleCount: model.schedules?.items.count ?? 0,
                selection: $model.destination,
                canCreate: model.agent?.selection != nil,
                newChannel: { showNewChannel = true },
                reopen: { id in Task { await model.reopenChannel(id: id, client: client) } }
            )
            .navigationSplitViewColumnWidth(min: 230, ideal: 270, max: 320)
        } detail: {
            ZStack {
                theme.canvas.ignoresSafeArea()
                desktopDestination
            }
        }
    }

    #if os(iOS)
    private var phoneContainer: some View {
        TabView(selection: phoneTab) {
            NavigationStack(path: phoneChannelPath) {
                PrimaryPhoneChannelList(
                    channels: model.channels,
                    archivedChannels: model.archivedChannels,
                    canCreate: model.agent?.selection != nil,
                    configured: model.agent?.selection != nil,
                    newChannel: { showNewChannel = true },
                    chooseAgent: { model.destination = .settings },
                    reopen: { id in Task { await model.reopenChannel(id: id, client: client) } }
                )
                .navigationDestination(for: String.self) { _ in channelDestination }
            }
            .tabItem { Label("Channels", systemImage: "number") }
            .tag(PrimaryPhoneTab.channels)

            NavigationStack { scheduledDestination }
                .tabItem { Label("Scheduled", systemImage: "calendar") }
                .tag(PrimaryPhoneTab.scheduled)

            NavigationStack { needsYouDestination }
                .tabItem { Label("Needs You", systemImage: "bell") }
                .badge(model.needsYou.count)
                .tag(PrimaryPhoneTab.needsYou)

            NavigationStack { settingsDestination }
                .tabItem { Label("Settings", systemImage: "gearshape") }
                .tag(PrimaryPhoneTab.settings)
        }
        .background(theme.canvas)
    }

    private var phoneTab: Binding<PrimaryPhoneTab> {
        Binding(
            get: {
                switch model.destination {
                case .scheduled: return .scheduled
                case .needsYou: return .needsYou
                case .settings: return .settings
                default: return .channels
                }
            },
            set: { tab in
                switch tab {
                case .channels:
                    if !model.destination.isChannel { model.destination = nil }
                case .scheduled: model.destination = .scheduled
                case .needsYou: model.destination = .needsYou
                case .settings: model.destination = .settings
                }
            }
        )
    }

    private var phoneChannelPath: Binding<[String]> {
        Binding(
            get: { model.selectedChannelID.map { [$0] } ?? [] },
            set: { path in
                if let id = path.last {
                    model.destination = .channel(id)
                } else if model.destination.isChannel {
                    model.destination = nil
                }
            }
        )
    }
    #endif

    @ViewBuilder
    private var desktopDestination: some View {
        switch model.destination {
        case .channel:
            channelDestination
        case .scheduled: scheduledDestination
        case .needsYou: needsYouDestination
        case .settings: settingsDestination
        case nil:
            PrimaryWelcomeView(
                configured: model.agent?.selection != nil,
                chooseAgent: { model.destination = .settings },
                createChannel: { showNewChannel = true }
            )
        }
    }

    @ViewBuilder
    private var channelDestination: some View {
        if let detail = model.channelDetail,
           detail.conversation.id == model.selectedChannelID {
            PrimaryChannelTranscript(
                detail: detail,
                pinned: model.channels.first(where: { $0.id == detail.conversation.id })?.pinned ?? false,
                pendingTurn: model.selectedPendingTurn,
                busy: model.busy,
                openSettings: { model.destination = .settings },
                send: { text in await model.send(text: text, client: client) },
                recover: { target, action in
                    await model.recover(target: target, action: action, client: client)
                },
                rename: { name in await model.renameChannel(name: name, client: client) },
                setPinned: { pinned in await model.setPinned(pinned, client: client) },
                archive: { await model.archiveChannel(client: client) }
            )
        } else {
            PrimaryLoadingView(label: "Loading Channel…")
        }
    }

    private var scheduledDestination: some View {
        PrimaryScheduledView(
            scheduleList: model.schedules,
            loadDetail: { id in try await client.primarySchedule(id: id) },
            openChannel: { id in model.destination = .channel(id) }
        )
    }

    private var needsYouDestination: some View {
        PrimaryNeedsYouList(
            items: model.needsYou,
            openChannel: { id in model.destination = .channel(id) },
            recover: { channelID, target, action in
                await model.recover(
                    channelID: channelID,
                    target: target,
                    action: action,
                    client: client
                )
            }
        )
    }

    private var settingsDestination: some View {
        PrimaryAgentSettings(
            agent: model.agent,
            selectedTheme: Binding(
                get: { theme },
                set: { storedTheme = $0.rawValue }
            ),
            connectionSettings: connectionSettings,
            choose: { optionID in await model.chooseAgent(optionID: optionID, client: client) },
            recheck: { await model.recheckAgent(client: client) }
        )
    }
}

private extension Optional where Wrapped == PrimaryDestination {
    var isChannel: Bool {
        if case .channel = self { return true }
        return false
    }
}

private struct PrimaryChannelSidebar: View {
    @Environment(\.primaryTheme) private var theme
    let channels: [PrimaryChannelSummary]
    let archivedChannels: [PrimaryChannelSummary]
    let needsYouCount: Int
    let scheduleCount: Int
    @Binding var selection: PrimaryDestination?
    let canCreate: Bool
    let newChannel: () -> Void
    let reopen: (String) -> Void

    private var pinned: [PrimaryChannelSummary] { channels.filter(\.pinned) }
    private var recent: [PrimaryChannelSummary] { channels.filter { !$0.pinned } }

    var body: some View {
        List(selection: $selection) {
            Section {
                HStack(spacing: 10) {
                    PrimaryOrb(name: "Fort", state: .idle, size: 34)
                    Text("FORT")
                        .font(.system(.callout, design: .monospaced).weight(.bold))
                        .tracking(4)
                }
                .accessibilityElement(children: .combine)

                Button(action: newChannel) {
                    Label("New Channel", systemImage: "plus")
                        .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
                .foregroundStyle(theme.accentInk)
                .disabled(!canCreate)
            }

            channelSection("PINNED", channels: pinned)
            channelSection("RECENT", channels: recent)

            if !archivedChannels.isEmpty {
                Section {
                    DisclosureGroup("ARCHIVED") {
                        ForEach(archivedChannels) { channel in
                            HStack {
                                Text(channel.conversation.title).lineLimit(1)
                                Spacer()
                                Button("Reopen") { reopen(channel.id) }
                                    .buttonStyle(.borderless)
                            }
                        }
                    }
                }
            }

            Section {
                destinationLink(.scheduled, title: "Scheduled", icon: "calendar", count: scheduleCount)
                destinationLink(.needsYou, title: "Needs You", icon: "bell", count: needsYouCount)
                destinationLink(.settings, title: "Settings", icon: "gearshape", count: 0)
            }
        }
        .scrollContentBackground(.hidden)
        .background(theme.canvas)
        .foregroundStyle(theme.primaryText)
    }

    @ViewBuilder
    private func channelSection(_ title: String, channels: [PrimaryChannelSummary]) -> some View {
        if !channels.isEmpty {
            Section(title) {
                ForEach(channels) { channel in
                    NavigationLink(value: PrimaryDestination.channel(channel.id)) {
                        VStack(alignment: .leading, spacing: 3) {
                            Text(channel.conversation.title)
                                .font(.callout.weight(.semibold))
                                .lineLimit(1)
                            Text(channel.participant.displayName)
                                .font(.caption)
                                .foregroundStyle(theme.secondaryText)
                                .lineLimit(1)
                        }
                    }
                    .tag(PrimaryDestination.channel(channel.id))
                }
            }
        }
    }

    private func destinationLink(
        _ destination: PrimaryDestination,
        title: String,
        icon: String,
        count: Int
    ) -> some View {
        NavigationLink(value: destination) {
            HStack {
                Label(title, systemImage: icon)
                Spacer()
                if count > 0 {
                    Text("\(count)")
                        .font(.caption2.monospacedDigit())
                        .padding(.horizontal, 7)
                        .padding(.vertical, 2)
                        .background(theme.raised, in: Capsule())
                }
            }
        }
        .tag(destination)
    }
}

#if os(iOS)
private struct PrimaryPhoneChannelList: View {
    @Environment(\.primaryTheme) private var theme
    let channels: [PrimaryChannelSummary]
    let archivedChannels: [PrimaryChannelSummary]
    let canCreate: Bool
    let configured: Bool
    let newChannel: () -> Void
    let chooseAgent: () -> Void
    let reopen: (String) -> Void

    private var pinned: [PrimaryChannelSummary] { channels.filter(\.pinned) }
    private var recent: [PrimaryChannelSummary] { channels.filter { !$0.pinned } }

    var body: some View {
        List {
            if channels.isEmpty {
                Section {
                    VStack(spacing: 13) {
                        PrimaryOrb(name: "Fort", state: .idle, size: 58)
                        Text(configured ? "Create your first private Channel" : "Choose a Primary Agent")
                            .font(.headline)
                            .multilineTextAlignment(.center)
                        Button(configured ? "New Channel" : "Choose Primary Agent") {
                            configured ? newChannel() : chooseAgent()
                        }
                        .buttonStyle(.borderedProminent)
                        .foregroundStyle(theme.accentInk)
                    }
                    .frame(maxWidth: .infinity)
                    .padding(.vertical, 24)
                }
            }

            phoneSection("PINNED", channels: pinned)
            phoneSection("RECENT", channels: recent)

            if !archivedChannels.isEmpty {
                Section {
                    DisclosureGroup("ARCHIVED") {
                        ForEach(archivedChannels) { channel in
                            HStack {
                                Text(channel.conversation.title).lineLimit(1)
                                Spacer()
                                Button("Reopen") { reopen(channel.id) }
                                    .buttonStyle(.borderless)
                            }
                        }
                    }
                }
            }
        }
        .scrollContentBackground(.hidden)
        .background(theme.canvas)
        .navigationTitle("Channels")
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Button(action: newChannel) { Image(systemName: "plus") }
                    .disabled(!canCreate)
            }
        }
    }

    @ViewBuilder
    private func phoneSection(_ title: String, channels: [PrimaryChannelSummary]) -> some View {
        if !channels.isEmpty {
            Section(title) {
                ForEach(channels) { channel in
                    NavigationLink(value: channel.id) {
                        VStack(alignment: .leading, spacing: 3) {
                            Text(channel.conversation.title).font(.callout.weight(.semibold))
                            Text("\(channel.participant.displayName) · \(channel.participant.machine)")
                                .font(.caption)
                                .foregroundStyle(theme.secondaryText)
                        }
                    }
                }
            }
        }
    }
}
#endif

// MARK: - Channel transcript and recovery

struct PrimaryChannelTranscript: View {
    @Environment(\.primaryTheme) private var theme
    let detail: PrimaryChannelDetail
    let pinned: Bool
    let pendingTurn: PrimaryPendingTurn?
    let busy: Bool
    let openSettings: () -> Void
    let send: (String) async -> Bool
    let recover: (PrimaryTarget, PrimaryTargetAction) async -> Void
    let rename: (String) async -> Void
    let setPinned: (Bool) async -> Void
    let archive: () async -> Void

    @State private var draft = ""
    @State private var showIdentity = false
    @State private var showRename = false

    private var participant: PrimaryParticipant? { detail.participants.first }
    private var latestTargets: [String: PrimaryTarget] {
        PrimaryTargetStatusReducer.latestAttemptsByTurn(detail.targets)
    }
    private var turnsByPrompt: [Int64: PrimaryTurn] {
        Dictionary(uniqueKeysWithValues: detail.turns.map { ($0.promptMessageID, $0) })
    }
    private var working: Bool {
        latestTargets.values.contains { $0.state == "working" }
    }

    var body: some View {
        VStack(spacing: 0) {
            header
            Divider().overlay(theme.line)
            ScrollViewReader { proxy in
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 20) {
                        ForEach(detail.messages.sorted { $0.id < $1.id }) { message in
                            PrimaryMessageRow(
                                message: message,
                                participant: participant,
                                working: messageIsWorking(message)
                            )
                            .id(message.id)

                            if let turn = turnsByPrompt[message.id],
                               let target = latestTargets[turn.id],
                               let presentation = PrimaryTargetStatusReducer.presentation(
                                   for: target,
                                   machine: participant?.machine
                               ) {
                                PrimaryTurnStatusCard(
                                    presentation: presentation,
                                    target: target,
                                    turn: turn,
                                    participant: participant,
                                    recover: { action in await recover(target, action) }
                                )
                            }
                        }

                        if detail.messages.isEmpty {
                            PrimaryEmptyState(
                                icon: "bubble.left.and.bubble.right",
                                title: "A private Channel",
                                detail: "This conversation has one immutable Primary Agent. Start with a text message."
                            )
                        }
                    }
                    .frame(maxWidth: 820, alignment: .leading)
                    .padding(.horizontal, 22)
                    .padding(.vertical, 24)
                    .frame(maxWidth: .infinity)
                }
                .onChange(of: detail.messages.last?.id) { id in
                    guard let id else { return }
                    withAnimation { proxy.scrollTo(id, anchor: .bottom) }
                }
            }
            Divider().overlay(theme.line)
            composer
        }
        .background(theme.canvas)
        .navigationTitle(detail.conversation.title)
        .sheet(isPresented: $showIdentity) {
            PrimaryIdentityView(detail: detail)
                .environment(\.primaryTheme, theme)
                .preferredColorScheme(theme.colorScheme)
        }
        .sheet(isPresented: $showRename) {
            PrimaryRenameSheet(currentName: detail.conversation.title, rename: rename)
                .environment(\.primaryTheme, theme)
                .preferredColorScheme(theme.colorScheme)
        }
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(alignment: .center, spacing: 12) {
                PrimaryOrb(
                    name: participant?.displayName ?? "Primary Agent",
                    state: working ? .working : .idle,
                    size: 42
                )
                VStack(alignment: .leading, spacing: 3) {
                    Text("Primary Agent")
                        .font(.headline)
                    Text(identityLine)
                        .font(.caption)
                        .foregroundStyle(theme.secondaryText)
                        .lineLimit(2)
                }
                Spacer()
                Menu {
                    Button("Identity", systemImage: "info.circle") { showIdentity = true }
                    Button("Rename", systemImage: "pencil") { showRename = true }
                    Button(pinned ? "Unpin" : "Pin", systemImage: "pin") {
                        Task { await setPinned(!pinned) }
                    }
                    Divider()
                    Button("Archive", systemImage: "archivebox", role: .destructive) {
                        Task { await archive() }
                    }
                } label: {
                    Image(systemName: "ellipsis.circle")
                        .font(.title3)
                        .frame(width: 44, height: 44)
                }
                .accessibilityLabel("Channel actions")
            }

            HStack(spacing: 8) {
                Label("Text-only chat", systemImage: "text.bubble")
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(theme.accent)
                Text("Fort supplies only this Channel context; tools, MCP, commands, and file access fail the answer.")
                    .font(.caption)
                    .foregroundStyle(theme.secondaryText)
                    .lineLimit(2)
            }
        }
        .padding(.horizontal, 22)
        .padding(.vertical, 16)
        .background(theme.panel)
    }

    private var composer: some View {
        VStack(alignment: .leading, spacing: 7) {
            if !isReady {
                HStack(spacing: 8) {
                    Label(
                        "The saved Primary Agent changed or is unavailable. Recheck it in Settings, or create a new Channel after choosing a Ready agent.",
                        systemImage: "lock.trianglebadge.exclamationmark"
                    )
                    .font(.caption)
                    .foregroundStyle(theme.warning)
                    Spacer()
                    Button("Settings", action: openSettings)
                        .buttonStyle(.bordered)
                }
            }
            if let pendingTurn {
                Label(
                    "Send result is unknown. Retry reuses the same client turn ID.",
                    systemImage: "arrow.triangle.2.circlepath"
                )
                .font(.caption)
                .foregroundStyle(theme.warning)
                .onAppear { if draft.isEmpty { draft = pendingTurn.text } }
            }
            HStack(alignment: .bottom, spacing: 10) {
                TextField("Message this private Channel", text: $draft, axis: .vertical)
                    .lineLimit(1...5)
                    .textFieldStyle(.plain)
                    .padding(.horizontal, 14)
                    .padding(.vertical, 12)
                    .background(theme.panel, in: RoundedRectangle(cornerRadius: 11))
                    .overlay(RoundedRectangle(cornerRadius: 11).stroke(theme.line))
                    .disabled(busy || !isReady || pendingTurn != nil)
                    .onSubmit { submit() }
                Button(pendingTurn == nil ? "Send" : "Retry send") { submit() }
                    .buttonStyle(.borderedProminent)
                    .foregroundStyle(theme.accentInk)
                    .controlSize(.large)
                    .disabled(busy || !isReady || draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
            }
        }
        .padding(.horizontal, 18)
        .padding(.vertical, 14)
        .background(theme.panel)
    }

    private func submit() {
        let text = pendingTurn?.text ?? draft.trimmingCharacters(in: .whitespacesAndNewlines)
        guard isReady, !text.isEmpty else { return }
        Task {
            if await send(text) { draft = "" }
        }
    }

    private var isReady: Bool { detail.readiness.state == "ready" }

    private func messageIsWorking(_ message: PrimaryMessage) -> Bool {
        guard message.authorKind == "agent", let targetID = message.targetID else { return false }
        return latestTargets.values.contains { $0.id == targetID && $0.state == "working" }
    }

    private var identityLine: String {
        guard let participant else { return "Identity unavailable" }
        let model = participant.model ?? participant.profile
        let plan = detail.primaryIdentity?.policy.accountPlan.capitalized ?? "Subscription"
        return "\(model) · ChatGPT \(plan) · \(participant.machine) · \(detail.readiness.state.capitalized)"
    }

}

private struct PrimaryMessageRow: View {
    @Environment(\.primaryTheme) private var theme
    let message: PrimaryMessage
    let participant: PrimaryParticipant?
    let working: Bool

    var body: some View {
        HStack(alignment: .top, spacing: 12) {
            if message.authorKind == "agent" {
                PrimaryOrb(
                    name: participant?.displayName ?? "Primary Agent",
                    state: working ? .working : .idle,
                    size: 34
                )
            } else {
                Circle()
                    .fill(theme.accent.opacity(0.2))
                    .frame(width: 34, height: 34)
                    .overlay(Text("T").font(.caption.weight(.bold)).foregroundStyle(theme.accent))
                    .accessibilityHidden(true)
            }
            VStack(alignment: .leading, spacing: 6) {
                HStack(spacing: 7) {
                    Text(message.authorKind == "agent" ? (participant?.displayName ?? "Primary Agent") : "You")
                        .font(.callout.weight(.semibold))
                    Text(PrimaryDateText.compact(message.createdAt))
                        .font(.caption2.monospacedDigit())
                        .foregroundStyle(theme.secondaryText)
                }
                Text(message.body)
                    .font(.body)
                    .foregroundStyle(theme.primaryText)
                    .textSelection(.enabled)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

private struct PrimaryTurnStatusCard: View {
    @Environment(\.primaryTheme) private var theme
    let presentation: PrimaryTargetPresentation
    let target: PrimaryTarget
    let turn: PrimaryTurn
    let participant: PrimaryParticipant?
    let recover: (PrimaryTargetAction) async -> Void
    @State private var detailsExpanded = false

    private var color: Color {
        switch presentation.kind {
        case .queued, .working: return theme.accent
        case .canceled: return theme.secondaryText
        case .failed, .interrupted: return theme.failure
        }
    }

    @ViewBuilder
    var body: some View {
        if presentation.kind == .canceled {
            HStack(spacing: 8) {
                Image(systemName: "xmark.circle").foregroundStyle(theme.secondaryText)
                Text("Canceled by you").font(.callout.weight(.semibold))
                Text(PrimaryDateText.compact(target.updatedAt))
                    .font(.caption2.monospacedDigit())
                    .foregroundStyle(theme.secondaryText)
            }
            .padding(.leading, 46)
            .frame(maxWidth: .infinity, alignment: .leading)
            .accessibilityElement(children: .combine)
        } else {
            VStack(alignment: .leading, spacing: 13) {
            HStack(alignment: .top, spacing: 11) {
                Image(systemName: icon)
                    .font(.title3)
                    .foregroundStyle(color)
                    .frame(width: 26)
                VStack(alignment: .leading, spacing: 5) {
                    Text(presentation.title)
                        .font(.headline)
                    if !presentation.body.isEmpty {
                        Text(presentation.body)
                            .font(.callout)
                            .foregroundStyle(theme.secondaryText)
                    }
                }
            }

            if let action = presentation.action {
                if action == .cancel {
                    Button(actionTitle(action)) { Task { await recover(action) } }
                        .buttonStyle(.bordered)
                        .controlSize(.large)
                } else {
                    Button(actionTitle(action)) { Task { await recover(action) } }
                        .buttonStyle(.borderedProminent)
                        .foregroundStyle(theme.accentInk)
                        .controlSize(.large)
                }
            }

            if presentation.showsDetails {
                DisclosureGroup("Details", isExpanded: $detailsExpanded) {
                    VStack(spacing: 0) {
                        detailRow("Reason", target.error ?? presentation.body)
                        detailRow("Attempt", "\(target.attempt)")
                        detailRow("Target", participant?.displayName ?? "Primary Agent")
                        detailRow("Client turn ID", turn.clientTurnID, monospaced: true)
                        detailRow("Computer", participant?.machine ?? "Unknown")
                        detailRow("Error code", target.errorCode ?? target.state, monospaced: true)
                    }
                    .padding(.top, 10)

                    if presentation.kind == .failed || presentation.kind == .interrupted {
                        Text("Retry keeps this client turn ID and creates Attempt \(target.attempt + 1).")
                            .font(.caption)
                            .foregroundStyle(theme.secondaryText)
                            .padding(.top, 12)
                    }
                }
                .font(.callout.weight(.semibold))
            }
        }
            .padding(16)
            .background(theme.panel, in: RoundedRectangle(cornerRadius: 13))
            .overlay(RoundedRectangle(cornerRadius: 13).stroke(color.opacity(0.8)))
            .padding(.leading, 46)
            .accessibilityElement(children: .contain)
        }
    }

    private var icon: String {
        switch presentation.kind {
        case .queued: return "clock"
        case .working: return "sparkles"
        case .canceled: return "xmark.circle"
        case .failed, .interrupted: return "exclamationmark.circle"
        }
    }

    private func actionTitle(_ action: PrimaryTargetAction) -> String {
        switch action {
        case .cancel: return "Cancel"
        case .retry: return "Retry"
        case .recheckAndRetry: return "Recheck and retry"
        }
    }

    private func detailRow(_ title: String, _ value: String, monospaced: Bool = false) -> some View {
        HStack(alignment: .top, spacing: 12) {
            Text(title)
                .foregroundStyle(theme.secondaryText)
                .frame(width: 112, alignment: .leading)
            Text(value.isEmpty ? "Unavailable" : value)
                .font(monospaced ? .caption.monospaced() : .caption)
                .textSelection(.enabled)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
        .padding(.vertical, 8)
        .overlay(alignment: .bottom) { Rectangle().fill(theme.line).frame(height: 1) }
    }
}

// MARK: - Scheduled

package enum PrimaryScheduleOccurrenceAction: Equatable {
    case viewSchedule
    case openRun
    case viewResult
    case reviewFailure

    package var title: String {
        switch self {
        case .viewSchedule: return "View schedule"
        case .openRun: return "Open run"
        case .viewResult: return "View result"
        case .reviewFailure: return "Review failure"
        }
    }
}

package enum PrimarySchedulePresentation {
    package static func occurrenceLabel(for wireState: String?) -> String {
        switch wireState?.lowercased() {
        case nil, "", "scheduled", "upcoming": return "Upcoming"
        case "fired": return "Fired"
        case "running": return "Running"
        case "succeeded", "completed": return "Completed"
        case "canceled", "cancelled": return "Canceled"
        case "failed": return "Failed"
        default: return "Failed"
        }
    }

    package static func definitionLabel(enabled: Bool) -> String {
        enabled ? "Active" : "Paused"
    }

    package static func occurrenceAction(for wireState: String?) -> PrimaryScheduleOccurrenceAction? {
        switch wireState?.lowercased() {
        case nil, "", "scheduled", "upcoming": return .viewSchedule
        case "fired", "running": return .openRun
        case "succeeded", "completed": return .viewResult
        case "failed": return .reviewFailure
        case "canceled", "cancelled": return nil
        default: return nil
        }
    }
}

package enum PrimaryScheduleDetailPresentation {
    package static func item(
        listItem: PrimaryScheduleItem,
        detail: PrimaryScheduleDetail?
    ) -> PrimaryScheduleItem {
        detail?.item ?? listItem
    }
}

/// `PrimaryScheduleList` is the canonical wire snapshot consumed by this
/// read-only destination; Phase 1 intentionally exposes no schedule mutations.
private struct PrimaryScheduledView: View {
    @Environment(\.primaryTheme) private var theme
    let scheduleList: PrimaryScheduleList?
    let loadDetail: (String) async throws -> PrimaryScheduleDetail
    let openChannel: (String) -> Void
    @State private var selected: PrimaryScheduleItem?

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            PrimaryPageHeader(
                title: "Scheduled",
                subtitle: scheduleList.map {
                    "Schedule data · \($0.items.count) definition\($0.items.count == 1 ? "" : "s") · observed \(PrimaryDateText.compact($0.observedAt))"
                } ?? "Loading durable schedule data…"
            )
            Divider().overlay(theme.line)
            if let items = scheduleList?.items {
                if items.isEmpty {
                    PrimaryEmptyState(
                        icon: "calendar",
                        title: "No schedules",
                        detail: "Every durable schedule definition will appear here."
                    )
                } else {
                    ScrollView {
                        LazyVStack(spacing: 10) {
                            ForEach(items) { item in
                                VStack(alignment: .trailing, spacing: 6) {
                                    Button { selected = item } label: {
                                        PrimaryScheduleRow(item: item)
                                    }
                                    .buttonStyle(.plain)
                                    if let action = PrimarySchedulePresentation.occurrenceAction(for: item.latestOccurrence?.state) {
                                        Button(action.title) { perform(action, for: item) }
                                            .buttonStyle(.bordered)
                                    }
                                }
                            }
                        }
                        .frame(maxWidth: 900)
                        .padding(20)
                        .frame(maxWidth: .infinity)
                    }
                }
            } else {
                PrimaryLoadingView(label: "Loading schedules…")
            }
        }
        .background(theme.canvas)
        .navigationTitle("Scheduled")
        .sheet(item: $selected) { item in
            PrimaryScheduleDetailSheet(
                item: item,
                load: loadDetail,
                openChannel: openChannel
            )
                .environment(\.primaryTheme, theme)
                .preferredColorScheme(theme.colorScheme)
        }
    }

    private func perform(_: PrimaryScheduleOccurrenceAction, for item: PrimaryScheduleItem) {
        selected = item
    }
}

private struct PrimaryScheduleRow: View {
    @Environment(\.primaryTheme) private var theme
    let item: PrimaryScheduleItem

    var body: some View {
        HStack(alignment: .top, spacing: 13) {
            Image(systemName: scheduleIcon)
                .foregroundStyle(statusColor)
                .frame(width: 26, height: 26)
            VStack(alignment: .leading, spacing: 5) {
                HStack {
                    Text(item.title).font(.callout.weight(.semibold))
                    Spacer()
                    Text(PrimarySchedulePresentation.definitionLabel(enabled: item.enabled))
                        .font(.caption.weight(.semibold))
                        .foregroundStyle(item.enabled ? theme.success : theme.secondaryText)
                }
                Text(item.id)
                    .font(.caption2.monospaced())
                    .foregroundStyle(theme.mutedText)
                Text("\(item.recurrence) · \(item.timezone)")
                    .font(.caption)
                    .foregroundStyle(theme.secondaryText)
                Text("Next \(item.nextFireAt.map { PrimaryDateText.full($0, timeZoneID: item.timezone) } ?? "none") · Last \(item.lastFireAt.map { PrimaryDateText.full($0, timeZoneID: item.timezone) } ?? "none")")
                    .font(.caption2)
                    .foregroundStyle(theme.secondaryText)
                HStack {
                    Text("\(item.targetKind.capitalized) · \(item.targetID)")
                    Spacer()
                    Text(PrimarySchedulePresentation.occurrenceLabel(for: item.latestOccurrence?.state))
                        .foregroundStyle(statusColor)
                }
                .font(.caption2)
                Text("Scheduler \(item.schedulerOwnership.capitalized) · observed \(PrimaryDateText.full(item.observedAt, timeZoneID: item.timezone))")
                    .font(.caption2)
                    .foregroundStyle(theme.mutedText)
                if let error = item.latestOccurrence?.error, !error.isEmpty {
                    Text(error)
                        .font(.caption2)
                        .foregroundStyle(theme.failure)
                        .lineLimit(2)
                }
                if let related = item.relatedChannel {
                    Label("Related Channel: \(related.name)", systemImage: "number")
                        .font(.caption2)
                        .foregroundStyle(theme.accent)
                } else {
                    Text("System schedule")
                        .font(.caption2)
                        .foregroundStyle(theme.secondaryText)
                }
            }
            Image(systemName: "chevron.right")
                .font(.caption)
                .foregroundStyle(theme.secondaryText)
                .padding(.top, 7)
        }
        .padding(14)
        .background(theme.panel, in: RoundedRectangle(cornerRadius: 12))
        .overlay(RoundedRectangle(cornerRadius: 12).stroke(theme.line))
        .contentShape(Rectangle())
    }

    private var scheduleIcon: String {
        PrimarySchedulePresentation.occurrenceLabel(for: item.latestOccurrence?.state) == "Failed"
            ? "exclamationmark.circle"
            : "clock"
    }

    private var statusColor: Color {
        switch PrimarySchedulePresentation.occurrenceLabel(for: item.latestOccurrence?.state) {
        case "Completed": return theme.success
        case "Failed": return theme.failure
        case "Canceled": return theme.secondaryText
        default: return theme.accent
        }
    }
}

private struct PrimaryScheduleDetailSheet: View {
    @Environment(\.primaryTheme) private var theme
    @Environment(\.dismiss) private var dismiss
    let item: PrimaryScheduleItem
    let load: (String) async throws -> PrimaryScheduleDetail
    let openChannel: (String) -> Void
    @State private var detail: PrimaryScheduleDetail?
    @State private var error: String?

    private var displayedItem: PrimaryScheduleItem {
        PrimaryScheduleDetailPresentation.item(listItem: item, detail: detail)
    }

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 18) {
                    Text(displayedItem.title).font(.title2.bold())
                    Text(PrimarySchedulePresentation.definitionLabel(enabled: displayedItem.enabled))
                        .foregroundStyle(displayedItem.enabled ? theme.success : theme.secondaryText)
                    detailLine("Schedule ID", displayedItem.id)
                    detailLine("Cadence", displayedItem.recurrence)
                    detailLine("Timezone", displayedItem.timezone)
                    detailLine(
                        "Next fire",
                        displayedItem.nextFireAt.map {
                            PrimaryDateText.full($0, timeZoneID: displayedItem.timezone)
                        } ?? "None"
                    )
                    detailLine(
                        "Last fire",
                        displayedItem.lastFireAt.map {
                            PrimaryDateText.full($0, timeZoneID: displayedItem.timezone)
                        } ?? "None"
                    )
                    detailLine("Target", "\(displayedItem.targetKind) · \(displayedItem.targetID)")
                    detailLine("Scheduler", displayedItem.schedulerOwnership.capitalized)
                    if let related = displayedItem.relatedChannel {
                        Button("Open Channel: \(related.name)") {
                            dismiss()
                            openChannel(related.id)
                        }
                        .buttonStyle(.bordered)
                    }
                    if let detail {
                        occurrenceSection("Upcoming", items: detail.upcoming)
                        occurrenceSection("Recent", items: detail.recent)
                    } else if let error {
                        Text(error).foregroundStyle(theme.failure)
                    } else {
                        ProgressView("Loading occurrence evidence…")
                    }
                    Text("Scheduled is read-only in Phase 1.")
                        .font(.caption)
                        .foregroundStyle(theme.secondaryText)
                }
                .padding(22)
                .frame(maxWidth: 620, alignment: .leading)
            }
            .background(theme.canvas)
            .toolbar { Button("Done") { dismiss() } }
            .task {
                do { detail = try await load(item.id) }
                catch { self.error = PrimaryErrorText.describe(error) }
            }
        }
    }

    private func detailLine(_ label: String, _ value: String) -> some View {
        HStack(alignment: .top) {
            Text(label).foregroundStyle(theme.secondaryText).frame(width: 110, alignment: .leading)
            Text(value).textSelection(.enabled)
        }
        .font(.callout)
    }

    @ViewBuilder
    private func occurrenceSection(_ title: String, items: [PrimaryScheduleOccurrence]) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title).font(.headline)
            if items.isEmpty {
                Text("None").font(.caption).foregroundStyle(theme.secondaryText)
            }
            ForEach(items) { occurrence in
                VStack(alignment: .leading, spacing: 3) {
                    Text("\(PrimarySchedulePresentation.occurrenceLabel(for: occurrence.state)) · \(PrimaryDateText.full(occurrence.scheduledFor, timeZoneID: displayedItem.timezone))")
                        .font(.callout.weight(.medium))
                    if let error = occurrence.error { Text(error).font(.caption).foregroundStyle(theme.failure) }
                    if let runID = occurrence.runID { Text("Run \(runID)").font(.caption2.monospaced()) }
                }
                .padding(11)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(theme.panel, in: RoundedRectangle(cornerRadius: 9))
            }
        }
    }
}

// MARK: - Needs you

struct PrimaryNeedsYouList: View {
    @Environment(\.primaryTheme) private var theme
    let items: [PrimaryNeedsYouItem]
    let openChannel: (String) -> Void
    let recover: (String, PrimaryTarget, PrimaryTargetAction) async -> Void

    var body: some View {
        VStack(spacing: 0) {
            PrimaryPageHeader(
                title: "Needs You",
                subtitle: "Only current, recoverable failed Primary Channel attempts"
            )
            Divider().overlay(theme.line)
            if items.isEmpty {
                PrimaryEmptyState(
                    icon: "checkmark.circle",
                    title: "Nothing needs you",
                    detail: "Resolved, canceled, and historical failures stay out of this projection."
                )
            } else {
                ScrollView {
                    LazyVStack(spacing: 12) {
                        ForEach(items) { item in
                            VStack(alignment: .leading, spacing: 12) {
                                HStack {
                                    VStack(alignment: .leading, spacing: 3) {
                                        Text(item.channel.conversation.title).font(.headline)
                                        Text(item.target.error ?? "Primary Agent answer failed")
                                            .font(.caption)
                                            .foregroundStyle(theme.secondaryText)
                                    }
                                    Spacer()
                                    Button("Open Channel") { openChannel(item.channel.id) }
                                }
                                HStack {
                                    ForEach(closedActions(item), id: \.rawValue) { action in
                                        Button(action == .retry ? "Retry" : "Recheck and retry") {
                                            Task { await recover(item.channel.id, item.target, action) }
                                        }
                                        .buttonStyle(.borderedProminent)
                                        .foregroundStyle(theme.accentInk)
                                    }
                                }
                            }
                            .padding(16)
                            .background(theme.panel, in: RoundedRectangle(cornerRadius: 12))
                            .overlay(RoundedRectangle(cornerRadius: 12).stroke(theme.failure.opacity(0.75)))
                        }
                    }
                    .frame(maxWidth: 760)
                    .padding(20)
                    .frame(maxWidth: .infinity)
                }
            }
        }
        .background(theme.canvas)
        .navigationTitle("Needs You")
    }

    private func closedActions(_ item: PrimaryNeedsYouItem) -> [PrimaryTargetAction] {
        let server = Set(item.recoveryActions)
        return PrimaryTargetStatusReducer.recoveryActions(for: item.target.errorCode).filter { action in
            switch action {
            case .retry: return server.contains("retry")
            case .recheckAndRetry: return server.contains("recheck_and_retry") || server.contains("recheck-and-retry")
            case .cancel: return false
            }
        }
    }
}

// MARK: - Settings

package enum PrimarySendOutcome: Equatable {
    case accepted
    case deterministicFailure
    case ambiguous
}

package enum PrimarySendOutcomeReducer {
    package static func failure(for error: Error) -> PrimarySendOutcome {
        if let clientError = error as? FortClientError,
           case .httpStatus(let status, _, _) = clientError,
           (400..<500).contains(status),
           status != 408,
           status != 429 {
            return .deterministicFailure
        }
        return .ambiguous
    }

    package static func pendingTurn(
        for outcome: PrimarySendOutcome,
        submission: PrimaryPendingTurn
    ) -> PrimaryPendingTurn? {
        outcome == .ambiguous ? submission : nil
    }

    package static func reconcile(
        _ outcome: PrimarySendOutcome,
        submission: PrimaryPendingTurn,
        detail: PrimaryChannelDetail?
    ) -> PrimarySendOutcome {
        guard detail?.conversation.id == submission.channelID,
              detail?.turns.contains(where: { $0.clientTurnID == submission.clientTurnID }) == true else {
            return outcome
        }
        return .accepted
    }
}

package struct PrimaryPendingTurnStore {
    package static let defaultsKey = "fort.primary.pending-turns.v1"

    private struct Record: Codable {
        let channelID: String
        let text: String
        let clientTurnID: String

        init(_ pending: PrimaryPendingTurn) {
            channelID = pending.channelID
            text = pending.text
            clientTurnID = pending.clientTurnID
        }

        var pending: PrimaryPendingTurn {
            PrimaryPendingTurn(channelID: channelID, text: text, clientTurnID: clientTurnID)
        }
    }

    private let defaults: UserDefaults
    private let key: String

    package init(
        defaults: UserDefaults = .standard,
        key: String = PrimaryPendingTurnStore.defaultsKey
    ) {
        self.defaults = defaults
        self.key = key
    }

    package func load() -> [String: PrimaryPendingTurn] {
        guard let data = defaults.data(forKey: key),
              let records = try? JSONDecoder().decode([Record].self, from: data) else {
            return [:]
        }
        return records.reduce(into: [:]) { turns, record in
            turns[record.channelID] = record.pending
        }
    }

    package func save(_ pendingTurns: [String: PrimaryPendingTurn]) {
        guard !pendingTurns.isEmpty else {
            defaults.removeObject(forKey: key)
            return
        }
        let records = pendingTurns.values
            .sorted { $0.channelID < $1.channelID }
            .map(Record.init)
        guard let data = try? JSONEncoder().encode(records) else { return }
        defaults.set(data, forKey: key)
    }

    package func reconciled(
        _ pendingTurns: [String: PrimaryPendingTurn],
        with detail: PrimaryChannelDetail
    ) -> [String: PrimaryPendingTurn] {
        var result = pendingTurns
        guard let pending = result[detail.conversation.id] else { return result }
        if detail.turns.contains(where: { $0.clientTurnID == pending.clientTurnID }) {
            result[detail.conversation.id] = nil
        }
        return result
    }
}

package struct PrimaryAgentOptionGroup {
    package let machine: String
    package var options: [PrimaryAgentOption]
}

package enum PrimaryAgentOptionGrouping {
    package static func groups(for options: [PrimaryAgentOption]) -> [PrimaryAgentOptionGroup] {
        var groups: [PrimaryAgentOptionGroup] = []
        var indexByMachine: [String: Int] = [:]
        for option in options {
            let machine = option.authority.machineID.isEmpty
                ? option.seat.machine
                : option.authority.machineID
            if let index = indexByMachine[machine] {
                groups[index].options.append(option)
            } else {
                indexByMachine[machine] = groups.count
                groups.append(PrimaryAgentOptionGroup(machine: machine, options: [option]))
            }
        }
        return groups
    }
}

package struct PrimaryModelDisclosure: Equatable {
    package let requested: String
    package let resolved: String

    package init(requested: String?, resolved: String?) {
        self.requested = Self.normalized(requested)
        self.resolved = Self.normalized(resolved)
    }

    private static func normalized(_ value: String?) -> String {
        guard let value = value?.trimmingCharacters(in: .whitespacesAndNewlines),
              !value.isEmpty else {
            return "unknown"
        }
        return value
    }
}

struct PrimaryAgentSettings: View {
    @Environment(\.primaryTheme) private var theme
    let agent: PrimaryAgentView?
    @Binding var selectedTheme: PrimaryTheme
    let connectionSettings: (() -> Void)?
    let choose: (String) async -> Void
    let recheck: () async -> Void
    @State private var busy = false

    #if os(macOS)
    @Environment(\.primaryServiceController) private var service
    #endif

    var body: some View {
        Form {
            Section("Primary Agent") {
                if let selection = agent?.selection {
                    PrimaryAgentOptionView(
                        name: selection.seat.displayName,
                        requestedModel: selectedOption?.authority.requestedModel ?? selection.seat.model,
                        resolvedModel: selectedOption?.authority.resolvedModel,
                        machine: selection.seat.machine,
                        plan: selection.policy.accountPlan,
                        state: agent?.state ?? selection.seat.state,
                        reason: agent?.reason,
                        selected: true
                    )
                } else {
                    Text("Choose one Ready subscription-backed Codex option. New Channels snapshot that exact identity; existing Channels never change.")
                        .foregroundStyle(theme.secondaryText)
                }

                Button("Recheck") {
                    busy = true
                    Task {
                        await recheck()
                        busy = false
                    }
                }
                .disabled(busy)
            }

            ForEach(optionGroups, id: \.machine) { group in
                Section("Computer · \(group.machine)") {
                    ForEach(group.options) { option in
                        Button {
                            busy = true
                            Task {
                                await choose(option.optionID)
                                busy = false
                            }
                        } label: {
                            PrimaryAgentOptionView(
                                name: option.displayName,
                                requestedModel: option.authority.requestedModel,
                                resolvedModel: option.authority.resolvedModel,
                                machine: option.seat.machine,
                                plan: option.authority.accountPlan,
                                state: option.state,
                                reason: option.reason,
                                selected: option.optionID == agent?.selection?.optionID
                            )
                        }
                        .buttonStyle(.plain)
                        .disabled(busy || option.state != "ready")
                    }
                }
            }

            Section("Text-only chat") {
                Label("Fresh ephemeral thread for every attempt", systemImage: "arrow.clockwise")
                Label("Empty per-target work directory", systemImage: "folder.badge.minus")
                Label("No dynamic tools, MCP, commands, or external actions", systemImage: "lock.shield")
                Text("Read-only sandboxing is not described as making file inspection impossible. Fort fails any command, tool, or file-access attempt.")
                    .font(.caption)
                    .foregroundStyle(theme.secondaryText)
            }

            Section("Appearance on this device") {
                Picker("Theme", selection: $selectedTheme) {
                    ForEach(PrimaryTheme.allCases) { theme in
                        Text(theme.title).tag(theme)
                    }
                }
                Text("Stored only on this device under fort.primary.theme.v1. It never changes Channel or provider state.")
                    .font(.caption)
                    .foregroundStyle(theme.secondaryText)
            }

            if let connectionSettings {
                Section("Connection") {
                    Button("Open connection settings", action: connectionSettings)
                    Text("Switch or reconnect the pinned encrypted Fort machine.")
                        .font(.caption)
                        .foregroundStyle(theme.secondaryText)
                }
            }

            if let inventory = agent?.scheduleInventory {
                Section("Schedule inventory") {
                    LabeledContent("State", value: inventory.state.capitalized)
                    LabeledContent("Definitions", value: "\(inventory.items.count)")
                    Text("Digest details remain deployment evidence; this screen does not accept or promote an inventory.")
                        .font(.caption)
                        .foregroundStyle(theme.secondaryText)
                }
            }

            #if os(macOS)
            if let service {
                Section("Fort service") {
                    HStack {
                        Circle()
                            .fill(service.status.running ? theme.success : theme.secondaryText)
                            .frame(width: 8, height: 8)
                        Text(service.status.running ? "Running" : "Stopped")
                    }
                    if service.status.running {
                        Button("Restart") { Task { await service.restart() } }
                    } else {
                        HStack {
                            Button("Install") { Task { await service.install() } }
                            Button("Start") { Task { await service.start() } }
                        }
                        Text("Install a missing service or start an existing stopped service.")
                            .font(.caption)
                            .foregroundStyle(theme.secondaryText)
                    }
                }
                .task { await service.refresh() }
            }
            #endif
        }
        .scrollContentBackground(.hidden)
        .background(theme.canvas)
        .navigationTitle("Settings")
    }

    private var selectedOption: PrimaryAgentOption? {
        guard let optionID = agent?.selection?.optionID else { return nil }
        return agent?.options.first { $0.optionID == optionID }
    }

    private var optionGroups: [PrimaryAgentOptionGroup] {
        PrimaryAgentOptionGrouping.groups(for: agent?.options ?? [])
    }
}

private struct PrimaryAgentOptionView: View {
    @Environment(\.primaryTheme) private var theme
    let name: String
    let requestedModel: String?
    let resolvedModel: String?
    let machine: String
    let plan: String
    let state: String
    let reason: String?
    let selected: Bool

    private var modelDisclosure: PrimaryModelDisclosure {
        PrimaryModelDisclosure(requested: requestedModel, resolved: resolvedModel)
    }

    var body: some View {
        HStack(spacing: 12) {
            PrimaryOrb(name: name, state: .idle, size: 36)
            VStack(alignment: .leading, spacing: 3) {
                Text(name).font(.callout.weight(.semibold))
                Text("Requested model \(modelDisclosure.requested) · Resolved model \(modelDisclosure.resolved)")
                    .font(.caption)
                    .foregroundStyle(theme.secondaryText)
                Text("ChatGPT \(plan.capitalized) · \(machine)")
                    .font(.caption)
                    .foregroundStyle(theme.secondaryText)
                Text(reason ?? state.capitalized)
                    .font(.caption2)
                    .foregroundStyle(state == "ready" ? theme.success : theme.warning)
            }
            Spacer()
            if selected { Image(systemName: "checkmark.circle.fill").foregroundStyle(theme.accent) }
        }
        .contentShape(Rectangle())
    }
}

// MARK: - Model

@MainActor
private final class PrimaryChannelsModel: ObservableObject {
    @Published var agent: PrimaryAgentView?
    @Published var channels: [PrimaryChannelSummary] = []
    @Published var archivedChannels: [PrimaryChannelSummary] = []
    @Published var channelDetail: PrimaryChannelDetail?
    @Published var schedules: PrimaryScheduleList?
    @Published var needsYou: [PrimaryNeedsYouItem] = []
    @Published var destination: PrimaryDestination?
    @Published var errorMessage: String?
    @Published private var pendingTurns: [String: PrimaryPendingTurn]
    @Published var busy = false
    private let pendingTurnStore: PrimaryPendingTurnStore

    init(pendingTurnStore: PrimaryPendingTurnStore = PrimaryPendingTurnStore()) {
        self.pendingTurnStore = pendingTurnStore
        pendingTurns = pendingTurnStore.load()
    }

    var selectedChannelID: String? {
        guard case .channel(let id) = destination else { return nil }
        return id
    }

    var selectedPendingTurn: PrimaryPendingTurn? {
        selectedChannelID.flatMap { pendingTurns[$0] }
    }

    func run(client: FortClient) async {
        await reload(client: client, chooseDestination: true)
        while !Task.isCancelled {
            do { try await Task.sleep(nanoseconds: 8_000_000_000) }
            catch { return }
            await reload(client: client, chooseDestination: false)
        }
    }

    /// Replacement snapshots are the live source of transcript truth. If the
    /// stream breaks, the bounded eight-second reload loop above remains the
    /// fallback while this task reconnects for the same selected Channel.
    func consumeSelectedChannelEvents(client: FortClient) async {
        guard let channelID = selectedChannelID else { return }
        while !Task.isCancelled, selectedChannelID == channelID {
            do {
                for try await detail in client.primaryChannelEvents(channelID: channelID) {
                    guard !Task.isCancelled, selectedChannelID == channelID else { return }
                    accept(detail: detail)
                }
            } catch is CancellationError {
                return
            } catch {
                await loadChannel(id: channelID, client: client)
            }
            do { try await Task.sleep(nanoseconds: 1_500_000_000) }
            catch { return }
        }
    }

    func open(_ destination: PrimaryDestination?, client: FortClient) async {
        guard case .channel(let id) = destination else { return }
        await loadChannel(id: id, client: client)
    }

    func createChannel(name: String, client: FortClient) async -> Bool {
        do {
            let detail = try await client.createPrimaryChannel(name: name)
            accept(detail: detail)
            destination = .channel(detail.conversation.id)
            await reload(client: client, chooseDestination: false)
            return true
        } catch {
            errorMessage = PrimaryErrorText.describe(error)
            return false
        }
    }

    func send(text: String, client: FortClient) async -> Bool {
        guard case .channel(let channelID) = destination,
              channelDetail?.conversation.id == channelID,
              channelDetail?.readiness.state == "ready",
              !busy else {
            errorMessage = "The saved Primary Agent is not Ready. Recheck Settings or create a new Channel with a Ready agent."
            return false
        }
        let normalized = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !normalized.isEmpty else { return false }

        let submission: PrimaryPendingTurn
        if let pendingTurn = pendingTurns[channelID] {
            submission = pendingTurn
        } else {
            submission = PrimaryPendingTurn(channelID: channelID, text: normalized)
            setPendingTurn(submission, for: channelID)
        }

        busy = true
        defer { busy = false }
        var outcome = PrimarySendOutcome.accepted
        var submissionError: String?
        do {
            _ = try await client.postPrimaryTurn(
                channelID: submission.channelID,
                clientTurnID: submission.clientTurnID,
                text: submission.text
            )
            setPendingTurn(nil, for: channelID)
            errorMessage = nil
        } catch {
            outcome = PrimarySendOutcomeReducer.failure(for: error)
            setPendingTurn(
                PrimarySendOutcomeReducer.pendingTurn(for: outcome, submission: submission),
                for: channelID
            )
            submissionError = PrimaryErrorText.describe(error)
            errorMessage = submissionError
        }
        await loadChannel(id: channelID, client: client)
        outcome = PrimarySendOutcomeReducer.reconcile(
            outcome,
            submission: submission,
            detail: channelDetail
        )
        if outcome == .accepted {
            setPendingTurn(nil, for: channelID)
        } else if let submissionError {
            errorMessage = submissionError
        }
        return outcome == .accepted
    }

    func recover(target: PrimaryTarget, action: PrimaryTargetAction, client: FortClient) async {
        guard case .channel(let channelID) = destination else { return }
        await recover(channelID: channelID, target: target, action: action, client: client)
    }

    func recover(
        channelID: String,
        target: PrimaryTarget,
        action: PrimaryTargetAction,
        client: FortClient
    ) async {
        guard !busy else { return }
        let allowed = Set(PrimaryTargetStatusReducer.recoveryActions(for: target.errorCode))
        if action != .cancel && !allowed.contains(action) { return }
        busy = true
        defer { busy = false }
        do {
            switch action {
            case .cancel:
                guard target.state == "queued" || target.state == "working" else { return }
                try await client.cancelPrimaryTarget(channelID: channelID, targetID: target.id)
            case .retry:
                _ = try await client.retryPrimaryTarget(channelID: channelID, targetID: target.id)
            case .recheckAndRetry:
                _ = try await client.recheckAndRetryPrimaryTarget(channelID: channelID, targetID: target.id)
            }
            errorMessage = nil
        } catch {
            errorMessage = PrimaryErrorText.describe(error)
        }
        await reload(client: client, chooseDestination: false)
        if case .channel = destination { await loadChannel(id: channelID, client: client) }
    }

    func renameChannel(name: String, client: FortClient) async {
        guard case .channel(let id) = destination else { return }
        await mutateChannel(id: id, client: client) {
            try await client.updatePrimaryChannel(id: id, name: name)
        }
    }

    func setPinned(_ pinned: Bool, client: FortClient) async {
        guard case .channel(let id) = destination else { return }
        await mutateChannel(id: id, client: client) {
            try await client.updatePrimaryChannel(id: id, pinned: pinned)
        }
    }

    func archiveChannel(client: FortClient) async {
        guard case .channel(let id) = destination else { return }
        await mutateChannel(id: id, client: client) {
            try await client.updatePrimaryChannel(id: id, state: .archived)
        }
        destination = channels.first.map { .channel($0.id) } ?? .scheduled
    }

    func reopenChannel(id: String, client: FortClient) async {
        do {
            try await client.updatePrimaryChannel(id: id, state: .open)
            await reload(client: client, chooseDestination: false)
            destination = .channel(id)
            await loadChannel(id: id, client: client)
            errorMessage = nil
        } catch {
            errorMessage = PrimaryErrorText.describe(error)
        }
    }

    func chooseAgent(optionID: String, client: FortClient) async {
        do {
            agent = try await client.setPrimaryAgent(optionID: optionID)
            errorMessage = nil
        } catch {
            errorMessage = PrimaryErrorText.describe(error)
        }
    }

    func recheckAgent(client: FortClient) async {
        do {
            agent = try await client.recheckPrimaryAgent()
            errorMessage = nil
        } catch {
            errorMessage = PrimaryErrorText.describe(error)
        }
    }

    private func mutateChannel(
        id: String,
        client: FortClient,
        mutation: () async throws -> Void
    ) async {
        do {
            try await mutation()
            errorMessage = nil
            await reload(client: client, chooseDestination: false)
            await loadChannel(id: id, client: client)
        } catch {
            errorMessage = PrimaryErrorText.describe(error)
        }
    }

    private func reload(client: FortClient, chooseDestination: Bool) async {
        do {
            async let nextAgent = client.primaryAgent()
            async let nextChannels = client.primaryChannels(state: .open)
            async let nextArchivedChannels = client.primaryChannels(state: .archived)
            async let nextSchedules = client.primarySchedules(filter: .all)
            async let nextNeedsYou = client.primaryNeedsYou()
            let result = try await (
                nextAgent, nextChannels, nextArchivedChannels, nextSchedules, nextNeedsYou
            )
            agent = result.0
            channels = result.1
            archivedChannels = result.2
            schedules = result.3
            needsYou = result.4
            errorMessage = nil

            if chooseDestination {
                if result.0.selection == nil {
                    destination = .settings
                } else if let selectedID = result.1.first?.id {
                    destination = .channel(selectedID)
                    await loadChannel(id: selectedID, client: client)
                } else {
                    destination = nil
                }
            } else if case .channel(let id) = destination {
                if result.1.contains(where: { $0.id == id }) {
                    await loadChannel(id: id, client: client)
                } else {
                    destination = result.1.first.map { .channel($0.id) } ?? .scheduled
                    channelDetail = nil
                }
            }
        } catch {
            errorMessage = PrimaryErrorText.describeUnavailable(error)
        }
    }

    private func loadChannel(id: String, client: FortClient) async {
        do {
            accept(detail: try await client.primaryChannel(id: id))
            errorMessage = nil
        } catch {
            errorMessage = PrimaryErrorText.describe(error)
        }
    }

    private func setPendingTurn(_ pending: PrimaryPendingTurn?, for channelID: String) {
        pendingTurns[channelID] = pending
        pendingTurnStore.save(pendingTurns)
    }

    private func accept(detail: PrimaryChannelDetail) {
        channelDetail = detail
        let reconciled = pendingTurnStore.reconciled(pendingTurns, with: detail)
        guard reconciled != pendingTurns else { return }
        pendingTurns = reconciled
        pendingTurnStore.save(pendingTurns)
    }
}

// MARK: - Small shared views

private struct PrimaryWelcomeView: View {
    @Environment(\.primaryTheme) private var theme
    let configured: Bool
    let chooseAgent: () -> Void
    let createChannel: () -> Void

    var body: some View {
        VStack(spacing: 18) {
            PrimaryOrb(name: "Fort", state: .idle, size: 76)
            Text(configured ? "Create your first private Channel" : "Choose a Primary Agent")
                .font(.title2.bold())
            Text(configured
                 ? "Each Channel keeps one immutable Primary Agent and its own durable context."
                 : "Fort will show only subscription-backed options that satisfy the text-only authority contract.")
                .multilineTextAlignment(.center)
                .foregroundStyle(theme.secondaryText)
                .frame(maxWidth: 460)
            Button(configured ? "New Channel" : "Choose Primary Agent") {
                configured ? createChannel() : chooseAgent()
            }
            .buttonStyle(.borderedProminent)
            .foregroundStyle(theme.accentInk)
            .controlSize(.large)
        }
        .padding(30)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

private struct PrimaryNewChannelSheet: View {
    @Environment(\.primaryTheme) private var theme
    @Environment(\.dismiss) private var dismiss
    let create: (String) async -> Bool
    @State private var name = ""
    @State private var busy = false

    var body: some View {
        NavigationStack {
            Form {
                Section("Channel name") {
                    TextField("e.g. Weekly Review", text: $name)
                    Text("The selected Primary Agent is snapshotted when this Channel is created.")
                        .font(.caption)
                        .foregroundStyle(theme.secondaryText)
                }
            }
            .scrollContentBackground(.hidden)
            .background(theme.canvas)
            .navigationTitle("New Channel")
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Create") {
                        let normalized = name.trimmingCharacters(in: .whitespacesAndNewlines)
                        busy = true
                        Task {
                            if await create(normalized) { dismiss() }
                            busy = false
                        }
                    }
                    .disabled(busy || name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                }
            }
        }
        .frame(minWidth: 340, minHeight: 250)
    }
}

private struct PrimaryRenameSheet: View {
    @Environment(\.primaryTheme) private var theme
    @Environment(\.dismiss) private var dismiss
    let currentName: String
    let rename: (String) async -> Void
    @State private var name: String

    init(currentName: String, rename: @escaping (String) async -> Void) {
        self.currentName = currentName
        self.rename = rename
        _name = State(initialValue: currentName)
    }

    var body: some View {
        NavigationStack {
            Form { TextField("Channel name", text: $name) }
                .scrollContentBackground(.hidden)
                .background(theme.canvas)
                .navigationTitle("Rename Channel")
                .toolbar {
                    ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
                    ToolbarItem(placement: .confirmationAction) {
                        Button("Save") {
                            let normalized = name.trimmingCharacters(in: .whitespacesAndNewlines)
                            Task { await rename(normalized); dismiss() }
                        }
                        .disabled(name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                    }
                }
        }
        .frame(minWidth: 340, minHeight: 220)
    }
}

private struct PrimaryIdentityView: View {
    @Environment(\.primaryTheme) private var theme
    @Environment(\.dismiss) private var dismiss
    let detail: PrimaryChannelDetail

    var body: some View {
        NavigationStack {
            ScrollView {
                if let participant = detail.participants.first, let identity = detail.primaryIdentity {
                    VStack(alignment: .leading, spacing: 13) {
                        Text(participant.displayName).font(.title2.bold())
                        identityRow("Profile", participant.profile)
                        identityRow("Requested model", modelDisclosure.requested)
                        identityRow("Resolved model", modelDisclosure.resolved)
                        identityRow("Computer", participant.machine)
                        identityRow("ChatGPT plan", identity.policy.accountPlan.capitalized)
                        identityRow("Account type", identity.policy.accountType)
                        identityRow("Readiness", detail.readiness.state.capitalized)
                        identityRow("Adapter", identity.policy.adapterID)
                        identityRow("Adapter revision", identity.policy.adapterRevision)
                        identityRow("Codex", identity.policy.codexVersion)
                        identityRow("Executable revision", identity.policy.codexExecutableRevision)
                        identityRow("Schema revision", identity.policy.codexSchemaRevision)
                        identityRow("Policy", identity.policy.policyID)
                        identityRow("Policy revision", identity.policy.policyRevision)
                        identityRow("Reasoning", "\(identity.policy.reasoningEffort) · \(identity.policy.reasoningContext)")
                        identityRow("Timeout", "\(identity.policy.requestTimeoutMillis) ms")
                        identityRow("Instruction revision", identity.policy.developerInstructionRevision)
                        identityRow("Thread", identity.policy.threadMode)
                        identityRow("Work directory", identity.policy.workdirMode)
                        identityRow("Sandbox", identity.policy.sandboxMode)
                        identityRow("Approvals", identity.policy.approvalPolicy)
                        identityRow("Dynamic tools", identity.policy.dynamicToolsMode)
                        identityRow("MCP", identity.policy.mcpMode)
                        identityRow("Command attempts", identity.policy.commandPolicy)
                        identityRow("File-read attempts", identity.policy.fileReadPolicy)
                        identityRow("Isolation revision", identity.policy.isolationRevision)
                        Text("This is immutable for this Channel. Changing Settings affects only Channels created afterward.")
                            .font(.caption)
                            .foregroundStyle(theme.secondaryText)
                            .padding(.top, 8)
                    }
                    .padding(22)
                    .frame(maxWidth: 640, alignment: .leading)
                }
            }
            .background(theme.canvas)
            .navigationTitle("Primary Agent identity")
            .toolbar { Button("Done") { dismiss() } }
        }
    }

    private var modelDisclosure: PrimaryModelDisclosure {
        let latestTarget = detail.targets.max { lhs, rhs in
            lhs.updatedAt == rhs.updatedAt ? lhs.id < rhs.id : lhs.updatedAt < rhs.updatedAt
        }
        return PrimaryModelDisclosure(
            requested: latestTarget?.authority?.requestedModel ?? detail.participants.first?.model,
            resolved: latestTarget?.receipt?.resolvedModel
        )
    }

    private func identityRow(_ label: String, _ value: String) -> some View {
        HStack(alignment: .top, spacing: 12) {
            Text(label).foregroundStyle(theme.secondaryText).frame(width: 125, alignment: .leading)
            Text(value).textSelection(.enabled).frame(maxWidth: .infinity, alignment: .leading)
        }
        .font(.callout)
        .padding(.vertical, 5)
        .overlay(alignment: .bottom) { Rectangle().fill(theme.line).frame(height: 1) }
    }
}

private struct PrimaryPageHeader: View {
    @Environment(\.primaryTheme) private var theme
    let title: String
    let subtitle: String

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(title).font(.title2.bold())
            Text(subtitle).font(.caption).foregroundStyle(theme.secondaryText)
        }
        .padding(.horizontal, 22)
        .padding(.vertical, 17)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(theme.panel)
    }
}

private struct PrimaryEmptyState: View {
    @Environment(\.primaryTheme) private var theme
    let icon: String
    let title: String
    let detail: String

    var body: some View {
        VStack(spacing: 12) {
            Image(systemName: icon).font(.system(size: 34)).foregroundStyle(theme.accent)
            Text(title).font(.headline)
            Text(detail).multilineTextAlignment(.center).foregroundStyle(theme.secondaryText)
        }
        .padding(30)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

private struct PrimaryLoadingView: View {
    let label: String
    var body: some View {
        ProgressView(label).frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

private struct PrimaryErrorBanner: View {
    @Environment(\.primaryTheme) private var theme
    let message: String
    let dismiss: () -> Void

    var body: some View {
        HStack(spacing: 10) {
            Image(systemName: "exclamationmark.triangle.fill")
            Text(message).font(.callout).lineLimit(3)
            Spacer()
            Button("Dismiss", action: dismiss).buttonStyle(.plain)
        }
        .padding(12)
        .foregroundStyle(theme.primaryText)
        .background(theme.failure.opacity(0.92), in: RoundedRectangle(cornerRadius: 11))
        .shadow(radius: 8)
        .frame(maxWidth: 720)
    }
}

package enum PrimaryDateText {
    static func compact(_ value: String) -> String {
        guard let date = parse(value) else { return value }
        let formatter = DateFormatter()
        formatter.dateStyle = .none
        formatter.timeStyle = .short
        return formatter.string(from: date)
    }

    package static func full(_ value: String, timeZoneID: String? = nil) -> String {
        guard let date = parse(value) else { return value }
        let formatter = DateFormatter()
        formatter.dateStyle = .medium
        formatter.timeStyle = .short
        if let timeZoneID, let timeZone = TimeZone(identifier: timeZoneID) {
            formatter.timeZone = timeZone
        }
        return formatter.string(from: date)
    }

    private static func parse(_ value: String) -> Date? {
        let formatter = ISO8601DateFormatter()
        if let date = formatter.date(from: value) { return date }
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return formatter.date(from: value)
    }
}

private enum PrimaryErrorText {
    static func describeUnavailable(_ error: Error) -> String {
        if case FortClientError.httpStatus(let status, _, _) = error,
           [403, 404, 503].contains(status) {
            return "Primary Channels are unavailable on this Fort. Update or enable the Phase 1 service; legacy chat is not used as a fallback."
        }
        return describe(error)
    }

    static func describe(_ error: Error) -> String {
        if let fort = error as? FortClientError {
            if let coded = fort.codedError { return coded.message }
            return fort.localizedDescription
        }
        return error.localizedDescription
    }
}
