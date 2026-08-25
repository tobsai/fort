//
//  AgentChannelsView.swift
//  FortKit
//
//  Shared native Spec 046 chat surface. Agent Channels are the top-level
//  destinations; durable Conversations remain nested beneath their exact agent.
//

import SwiftUI

private struct AgentConnectionSettingsKey: EnvironmentKey {
    static let defaultValue: (() -> Void)? = nil
}

private extension EnvironmentValues {
    var agentConnectionSettings: (() -> Void)? {
        get { self[AgentConnectionSettingsKey.self] }
        set { self[AgentConnectionSettingsKey.self] = newValue }
    }
}

public extension View {
    /// Keeps the existing encrypted-machine connection editor reachable from
    /// the additive Agent Channels shell without changing gateway ownership.
    func agentConnectionSettings(_ action: @escaping () -> Void) -> some View {
        environment(\.agentConnectionSettings, action)
    }
}

/// One explicit product-mode branch. `.off` remains the default and preserves
/// the complete Primary Channels rollback implementation.
public struct FortNativeChatView: View {
    private let mode: AgentChannelsPresentationMode

    public init(mode: AgentChannelsPresentationMode = .off) {
        self.mode = mode
    }

    @ViewBuilder
    public var body: some View {
        switch mode {
        case .off:
            PrimaryChannelsView()
        case .primary:
            AgentChannelsView()
        }
    }
}

public struct AgentChannelsView: View {
    @EnvironmentObject private var client: FortClient
    @Environment(\.agentConnectionSettings) private var connectionSettings
    @StateObject private var model = AgentChannelsModel()
    @State private var showAgentOptions = false
    @State private var inspectedAgentID: String?
    @State private var renameTarget: AgentRenameTarget?

    public init() {}

    public var body: some View {
        NavigationSplitView {
            AgentChannelsSidebar(
                channels: model.channels,
                archivedChannels: model.archivedChannels,
                archivedConversationsByAgent: model.archivedConversationsByAgent,
                selectedChannelID: model.selectedChannelID,
                selectedConversationID: model.selectedConversationID,
                needsYouCount: model.needsYou.count,
                addAgent: { showAgentOptions = true },
                selectAgent: { channelID in
                    Task { await model.selectAgent(channelID: channelID, using: client) }
                },
                selectConversation: { channelID, conversationID in
                    Task {
                        await model.selectConversation(
                            channelID: channelID,
                            conversationID: conversationID,
                            using: client
                        )
                    }
                },
                showNeedsYou: { model.destination = .needsYou },
                showSettings: { model.destination = .settings },
                reopenChannel: { channelID in
                    Task { await model.reopenChannel(channelID: channelID, using: client) }
                },
                reopenConversation: { channelID, conversationID in
                    Task {
                        await model.reopenConversation(
                            channelID: channelID,
                            conversationID: conversationID,
                            using: client
                        )
                    }
                }
            )
            .navigationSplitViewColumnWidth(min: 250, ideal: 300, max: 360)
        } detail: {
            destination
        }
        .tint(FortPalette.brass)
        .task(id: "\(client.baseURL.absoluteString)|\(client.transportGeneration)") {
            await model.run(using: client)
        }
        .task(id: "\(client.baseURL.absoluteString)|\(client.transportGeneration)|\(model.selectedChannelID ?? "")|\(model.selectedConversationID ?? "")") {
            await model.consumeSelectedConversationEvents(using: client)
        }
        .sheet(isPresented: $showAgentOptions) {
            AgentOptionsSheet(
                options: model.options,
                recheck: { await model.recheckOptions(using: client) },
                add: { optionID, name in
                    let created = await model.createAgentChannel(
                        optionID: optionID,
                        name: name,
                        using: client
                    )
                    if created { showAgentOptions = false }
                    return created
                }
            )
        }
        .sheet(item: inspectedAgent) { channel in
            AgentIdentityInspectionView(
                channel: channel,
                recheck: { await model.recheckOptions(using: client) }
            )
        }
        .sheet(item: $renameTarget) { target in
            AgentRenameSheet(target: target) { name in
                switch target.kind {
                case .agent:
                    await model.renameSelectedChannel(name, using: client)
                case .conversation:
                    await model.renameSelectedConversation(name, using: client)
                }
            }
        }
        .overlay(alignment: .top) {
            if let message = model.errorMessage {
                AgentErrorBanner(message: message) { model.errorMessage = nil }
                    .padding(12)
            }
        }
    }

    private var inspectedAgent: Binding<AgentChannelSummary?> {
        Binding(
            get: {
                inspectedAgentID.flatMap { id in model.channels.first { $0.id == id } }
            },
            set: { inspectedAgentID = $0?.id }
        )
    }

    private var destination: some View {
        VStack(spacing: 0) {
            AgentChannelsPersistentChrome()
            Divider()
            destinationContent
        }
    }

    @ViewBuilder
    private var destinationContent: some View {
        switch model.destination {
        case .agents:
            AgentChannelsWelcome(addAgent: { showAgentOptions = true })
        case .agent(let channelID):
            if let channel = model.channels.first(where: { $0.id == channelID }) {
                AgentConversationWorkspace(
                    channel: channel,
                    detail: nil,
                    activity: .ambient,
                    pendingTurn: model.selectedPendingTurn,
                    busy: model.busy,
                    inspect: { inspectedAgentID = channel.id },
                    newConversation: { model.startNewConversation(channelID: channel.id) },
                    renameAgent: {
                        renameTarget = AgentRenameTarget(
                            id: "agent:\(channel.id)",
                            kind: .agent,
                            currentName: channel.channel.name
                        )
                    },
                    renameConversation: {},
                    setPinned: { _ in },
                    archiveConversation: {},
                    send: { text in await model.send(text: text, using: client) },
                    recover: { _, _ in }
                )
            } else {
                AgentLoadingView(label: "Loading Agent Channel…")
            }
        case .conversation(let channelID, let conversationID):
            if let channel = model.channels.first(where: { $0.id == channelID }),
               let detail = model.conversationDetail,
               detail.channelID == channelID,
               detail.conversation.id == conversationID {
                AgentConversationWorkspace(
                    channel: channel,
                    detail: detail,
                    activity: model.selectedActivity,
                    pendingTurn: model.selectedPendingTurn,
                    busy: model.busy,
                    inspect: { inspectedAgentID = channel.id },
                    newConversation: { model.startNewConversation(channelID: channel.id) },
                    renameAgent: {
                        renameTarget = AgentRenameTarget(
                            id: "agent:\(channel.id)",
                            kind: .agent,
                            currentName: channel.channel.name
                        )
                    },
                    renameConversation: {
                        renameTarget = AgentRenameTarget(
                            id: "conversation:\(detail.conversation.id)",
                            kind: .conversation,
                            currentName: detail.conversation.title
                        )
                    },
                    setPinned: { pinned in
                        await model.setSelectedConversationPinned(pinned, using: client)
                    },
                    archiveConversation: {
                        await model.archiveSelectedConversation(using: client)
                    },
                    send: { text in await model.send(text: text, using: client) },
                    recover: { target, action in
                        await model.recover(target: target, action: action, using: client)
                    }
                )
            } else {
                AgentLoadingView(label: "Loading Conversation…")
            }
        case .needsYou:
            AgentNeedsYouView(
                items: model.needsYou,
                open: { item in
                    await model.selectConversation(
                        channelID: item.agentChannel.id,
                        conversationID: item.conversation.id,
                        using: client
                    )
                },
                recover: { item, action in
                    await model.recoverNeedsYou(item, action: action, using: client)
                }
            )
        case .settings:
            AgentSettingsView(
                channels: model.channels,
                options: model.options,
                connectionSettings: connectionSettings,
                inspect: { inspectedAgentID = $0 },
                showOptions: { showAgentOptions = true },
                recheck: { await model.recheckOptions(using: client) }
            )
        }
    }
}

// MARK: - Agent-first navigation

private struct AgentChannelsSidebar: View {
    let channels: [AgentChannelSummary]
    let archivedChannels: [AgentChannelSummary]
    let archivedConversationsByAgent: [String: [AgentConversationSummary]]
    let selectedChannelID: String?
    let selectedConversationID: String?
    let needsYouCount: Int
    let addAgent: () -> Void
    let selectAgent: (String) -> Void
    let selectConversation: (String, String) -> Void
    let showNeedsYou: () -> Void
    let showSettings: () -> Void
    let reopenChannel: (String) -> Void
    let reopenConversation: (String, String) -> Void

    var body: some View {
        List {
            Section {
                HStack(spacing: 10) {
                    FortProductMarkView(activity: .ambient, size: 34, decorative: true)
                    VStack(alignment: .leading, spacing: 2) {
                        Text("FORT")
                            .font(.system(.callout, design: .monospaced).weight(.bold))
                            .tracking(4)
                        Text("Agent chat")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
                .accessibilityElement(children: .combine)

                Button(action: addAgent) {
                    Label("Add Agent Channel", systemImage: "plus")
                        .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
            }

            if channels.isEmpty {
                Section {
                    Text("No Agent Channels are enrolled on this Fort yet.")
                        .font(.callout)
                        .foregroundStyle(.secondary)
                }
            }

            ForEach(channels) { channel in
                Section {
                    Button { selectAgent(channel.id) } label: {
                        AgentChannelRow(
                            channel: channel,
                            selected: selectedChannelID == channel.id
                        )
                    }
                    .buttonStyle(.plain)

                    conversationSection(
                        "PINNED CONVERSATIONS",
                        channel: channel,
                        conversations: channel.conversations.filter {
                            $0.pinned && $0.conversation.state == AgentConversationState.open.rawValue
                        }
                    )
                    conversationSection(
                        "RECENT CONVERSATIONS",
                        channel: channel,
                        conversations: channel.conversations.filter {
                            !$0.pinned && $0.conversation.state == AgentConversationState.open.rawValue
                        }
                    )
                    archivedConversationSection(
                        channel: channel,
                        conversations: archivedConversationsByAgent[channel.id] ?? []
                    )
                }
            }

            if !archivedChannels.isEmpty {
                Section {
                    DisclosureGroup("ARCHIVED AGENTS") {
                        ForEach(archivedChannels) { channel in
                            HStack {
                                AgentIdentityView(
                                    name: channel.channel.name,
                                    identityKey: channel.id,
                                    size: 26
                                )
                                Text(channel.channel.name).lineLimit(1)
                                Spacer()
                                Button("Reopen") { reopenChannel(channel.id) }
                                    .buttonStyle(.borderless)
                            }
                        }
                    }
                }
            }

            Section {
                Button(action: showNeedsYou) {
                    HStack {
                        Label("Needs You", systemImage: "bell")
                        Spacer()
                        if needsYouCount > 0 {
                            Text("\(needsYouCount)")
                                .font(.caption.monospacedDigit())
                                .padding(.horizontal, 7)
                                .padding(.vertical, 2)
                                .background(FortPalette.needsYou.opacity(0.18), in: Capsule())
                        }
                    }
                }
                Button(action: showSettings) {
                    Label("Settings", systemImage: "gearshape")
                }
            }
            .buttonStyle(.plain)
        }
        .navigationTitle("Channels")
    }

    @ViewBuilder
    private func archivedConversationSection(
        channel: AgentChannelSummary,
        conversations: [AgentConversationSummary]
    ) -> some View {
        if !conversations.isEmpty {
            DisclosureGroup("ARCHIVED CONVERSATIONS") {
                ForEach(conversations) { conversation in
                    HStack(spacing: 7) {
                        Image(systemName: "archivebox")
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                        Text(conversation.conversation.title)
                            .font(.callout)
                            .lineLimit(1)
                        Spacer()
                        Button("Reopen") {
                            reopenConversation(channel.id, conversation.id)
                        }
                        .buttonStyle(.borderless)
                    }
                    .padding(.leading, 35)
                }
            }
            .font(.caption2.weight(.semibold))
            .foregroundStyle(.secondary)
        }
    }

    @ViewBuilder
    private func conversationSection(
        _ title: String,
        channel: AgentChannelSummary,
        conversations: [AgentConversationSummary]
    ) -> some View {
        if !conversations.isEmpty {
            Text(title)
                .font(.caption2.weight(.semibold))
                .foregroundStyle(.secondary)
                .padding(.leading, 42)
            ForEach(conversations) { conversation in
                Button {
                    selectConversation(channel.id, conversation.id)
                } label: {
                    HStack(spacing: 7) {
                        Image(systemName: conversation.pinned ? "pin.fill" : "bubble.left")
                            .font(.caption2)
                            .foregroundStyle(conversation.pinned ? FortPalette.brass : .secondary)
                        Text(conversation.conversation.title)
                            .font(.callout)
                            .lineLimit(1)
                        Spacer()
                    }
                    .padding(.leading, 35)
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .listRowBackground(
                    selectedConversationID == conversation.id
                        ? FortPalette.brass.opacity(0.12)
                        : Color.clear
                )
                .accessibilityLabel("\(channel.channel.name), \(conversation.conversation.title)")
            }
        }
    }
}

private struct AgentChannelRow: View {
    let channel: AgentChannelSummary
    let selected: Bool

    var body: some View {
        HStack(spacing: 10) {
            AgentIdentityView(
                name: channel.channel.name,
                identityKey: channel.id,
                size: 34
            )
            VStack(alignment: .leading, spacing: 3) {
                Text(channel.channel.name)
                    .font(.callout.weight(.semibold))
                    .lineLimit(1)
                Text("\(channel.channel.binding.seat.agent) · \(readinessText)")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
            }
            Spacer()
            Circle()
                .fill(channel.readiness.state == "ready" ? Color.green : FortPalette.needsYou)
                .frame(width: 8, height: 8)
        }
        .padding(.vertical, 3)
        .contentShape(Rectangle())
        .background(selected ? FortPalette.brass.opacity(0.08) : Color.clear)
        .accessibilityElement(children: .combine)
    }

    private var readinessText: String {
        channel.readiness.state.isEmpty ? "unknown" : channel.readiness.state.capitalized
    }
}

// MARK: - Conversation workspace

private struct AgentConversationWorkspace: View {
    let channel: AgentChannelSummary
    let detail: AgentConversationDetail?
    let activity: FortMarkActivity
    let pendingTurn: AgentPendingTurn?
    let busy: Bool
    let inspect: () -> Void
    let newConversation: () -> Void
    let renameAgent: () -> Void
    let renameConversation: () -> Void
    let setPinned: (Bool) async -> Void
    let archiveConversation: () async -> Void
    let send: (String) async -> Bool
    let recover: (AgentTarget, AgentTargetAction) async -> Void

    var body: some View {
        VStack(spacing: 0) {
            header
            Divider()
            if let detail {
                AgentTranscript(
                    channel: channel,
                    detail: detail,
                    recover: recover
                )
            } else {
                AgentNewConversationState(channel: channel)
            }
            Divider()
            AgentComposer(
                agentName: channel.channel.name,
                readiness: channel.readiness,
                pendingTurn: pendingTurn,
                busy: busy,
                send: send
            )
        }
        .background(Color.primary.opacity(0.018))
        .navigationTitle(detail?.conversation.title ?? channel.channel.name)
    }

    private var header: some View {
        HStack(spacing: 12) {
            AgentIdentityView(
                name: channel.channel.name,
                identityKey: channel.id,
                size: 42,
                decorative: false
            )
            VStack(alignment: .leading, spacing: 3) {
                Text(channel.channel.name)
                    .font(.headline)
                Text(headerSubtitle)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(2)
            }
            Spacer()
            HStack(spacing: 7) {
                FortProductMarkView(activity: activity, size: 25, decorative: true)
                Text(activity == .working ? "Working" : "FORT")
                    .font(.caption.weight(.semibold))
            }
            .accessibilityElement(children: .combine)
            Button(action: inspect) {
                Image(systemName: "info.circle")
            }
            .buttonStyle(.borderless)
            .accessibilityLabel("Inspect exact agent identity")
            Menu {
                Button("New conversation", systemImage: "square.and.pencil", action: newConversation)
                Button("Rename agent", systemImage: "pencil", action: renameAgent)
                if let detail {
                    Button(detail.pinned ? "Unpin conversation" : "Pin conversation", systemImage: "pin") {
                        Task { await setPinned(!detail.pinned) }
                    }
                    Button("Rename conversation", systemImage: "text.cursor", action: renameConversation)
                    Button("Archive conversation", systemImage: "archivebox", role: .destructive) {
                        Task { await archiveConversation() }
                    }
                }
            } label: {
                Image(systemName: "ellipsis.circle")
            }
            .menuStyle(.borderlessButton)
        }
        .padding(.horizontal, 18)
        .padding(.vertical, 12)
    }

    private var headerSubtitle: String {
        let authority = channel.channel.binding.authority
        let model = authority.resolvedModel.isEmpty
            ? (authority.requestedModel.isEmpty ? "unknown model" : authority.requestedModel)
            : authority.resolvedModel
        let readiness = channel.readiness.state.isEmpty ? "unknown" : channel.readiness.state
        return "\(model) · \(channel.channel.binding.seat.machine) · \(readiness)"
    }
}

private struct AgentNewConversationState: View {
    let channel: AgentChannelSummary

    var body: some View {
        VStack(spacing: 15) {
            FortProductMarkView(activity: .ambient, size: 70, decorative: true)
            Text("New conversation")
                .font(.title2.bold())
            Text("Your first Send atomically creates a separate durable transcript with \(channel.channel.name). No other Conversation context is included.")
                .multilineTextAlignment(.center)
                .foregroundStyle(.secondary)
                .frame(maxWidth: 520)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .padding(28)
    }
}

private struct AgentTranscript: View {
    let channel: AgentChannelSummary
    let detail: AgentConversationDetail
    let recover: (AgentTarget, AgentTargetAction) async -> Void

    private var turnsByPrompt: [Int64: AgentTurn] {
        Dictionary(uniqueKeysWithValues: detail.turns.map { ($0.promptMessageID, $0) })
    }

    private var latestTargets: [String: AgentTarget] {
        detail.targets.reduce(into: [:]) { latest, target in
            guard let current = latest[target.turnID] else {
                latest[target.turnID] = target
                return
            }
            if target.attempt > current.attempt
                || (target.attempt == current.attempt && target.id > current.id) {
                latest[target.turnID] = target
            }
        }
    }

    var body: some View {
        ScrollViewReader { proxy in
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 18) {
                    if detail.messages.isEmpty {
                        VStack(spacing: 10) {
                            Image(systemName: "bubble.left.and.bubble.right")
                                .font(.system(size: 36))
                                .foregroundStyle(FortPalette.brass)
                            Text("Conversation ready")
                                .font(.headline)
                            Text("Send a message to this exact agent.")
                                .foregroundStyle(.secondary)
                        }
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 50)
                    }
                    ForEach(detail.messages.sorted { $0.id < $1.id }) { message in
                        AgentMessageRow(channel: channel, message: message)
                            .id(message.id)
                        if let turn = turnsByPrompt[message.id],
                           let target = latestTargets[turn.id],
                           let action = AgentTargetRecovery.action(for: target) {
                            AgentTargetCard(target: target, action: action) {
                                await recover(target, action)
                            }
                        } else if let turn = turnsByPrompt[message.id],
                                  let target = latestTargets[turn.id],
                                  target.state == "canceled" || target.state == "failed" {
                            AgentTargetCard(target: target, action: nil, recover: {})
                        }
                    }
                }
                .frame(maxWidth: 820, alignment: .leading)
                .padding(22)
                .frame(maxWidth: .infinity)
            }
            .onChange(of: detail.messages.last?.id) { id in
                guard let id else { return }
                withAnimation { proxy.scrollTo(id, anchor: .bottom) }
            }
        }
    }
}

private struct AgentMessageRow: View {
    let channel: AgentChannelSummary
    let message: AgentMessage

    var body: some View {
        HStack(alignment: .top, spacing: 11) {
            if message.authorKind == "human" {
                Color.clear.frame(width: 34, height: 1)
            } else {
                AgentIdentityView(
                    name: channel.channel.name,
                    identityKey: channel.id,
                    size: 34
                )
            }
            VStack(alignment: .leading, spacing: 6) {
                HStack {
                    Text(message.authorKind == "human" ? "You" : channel.channel.name)
                        .font(.caption.weight(.semibold))
                    Spacer()
                    Text(message.createdAt)
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                }
                Text(message.body)
                    .textSelection(.enabled)
                    .frame(maxWidth: .infinity, alignment: .leading)
                if message.authorKind != "human" {
                    let authority = channel.channel.binding.authority
                    Text("\(authority.resolvedModel.isEmpty ? authority.requestedModel : authority.resolvedModel) · \(authority.adapterID)")
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                }
            }
        }
        .accessibilityElement(children: .contain)
    }
}

private struct AgentTargetCard: View {
    let target: AgentTarget
    let action: AgentTargetAction?
    let recover: () async -> Void

    var body: some View {
        HStack(spacing: 12) {
            Image(systemName: icon)
                .foregroundStyle(color)
            VStack(alignment: .leading, spacing: 3) {
                Text(title).font(.callout.weight(.semibold))
                Text(target.error ?? target.state.capitalized)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                if let code = target.errorCode {
                    Text(code).font(.caption2.monospaced()).foregroundStyle(.secondary)
                }
            }
            Spacer()
            if let action {
                if action == .cancel {
                    Button(actionTitle(action)) { Task { await recover() } }
                        .buttonStyle(.bordered)
                } else {
                    Button(actionTitle(action)) { Task { await recover() } }
                        .buttonStyle(.borderedProminent)
                }
            }
        }
        .padding(13)
        .background(color.opacity(0.08), in: RoundedRectangle(cornerRadius: 12))
        .overlay(RoundedRectangle(cornerRadius: 12).stroke(color.opacity(0.28)))
        .padding(.leading, 45)
    }

    private var title: String {
        switch target.state {
        case "queued": return "Starting agent…"
        case "working": return "Agent is working"
        case "canceled": return "Canceled by you"
        default: return "Answer failed"
        }
    }

    private var icon: String {
        switch target.state {
        case "queued": return "clock"
        case "working": return "sparkles"
        case "canceled": return "xmark.circle"
        default: return "exclamationmark.circle"
        }
    }

    private var color: Color {
        switch target.state {
        case "queued", "working": return FortPalette.working
        case "canceled": return .secondary
        default: return .red
        }
    }

    private func actionTitle(_ action: AgentTargetAction) -> String {
        switch action {
        case .cancel: return "Cancel"
        case .retry: return "Retry"
        case .recheckAndRetry: return "Recheck and retry"
        }
    }
}

private struct AgentComposer: View {
    let agentName: String
    let readiness: PrimaryChannelReadiness
    let pendingTurn: AgentPendingTurn?
    let busy: Bool
    let send: (String) async -> Bool
    @State private var draft = ""

    var body: some View {
        VStack(alignment: .leading, spacing: 7) {
            if readiness.state != "ready" {
                Label(
                    readiness.reason ?? "This agent is not Ready.",
                    systemImage: "exclamationmark.triangle"
                )
                .font(.caption)
                .foregroundStyle(FortPalette.needsYou)
            }
            if let pendingTurn {
                Label(
                    "Send result is unknown. Retry send reuses the same client turn ID.",
                    systemImage: "arrow.triangle.2.circlepath"
                )
                .font(.caption)
                .foregroundStyle(FortPalette.needsYou)
                .onAppear { if draft.isEmpty { draft = pendingTurn.text } }
            }
            HStack(alignment: .bottom, spacing: 10) {
                TextEditor(text: $draft)
                    .frame(minHeight: 42, maxHeight: 110)
                    .padding(5)
                    .background(Color.secondary.opacity(0.08), in: RoundedRectangle(cornerRadius: 10))
                    .accessibilityLabel("Message \(agentName)")
                    .disabled(pendingTurn != nil)
                Button {
                    let submitted = pendingTurn?.text ?? draft
                    Task {
                        if await send(submitted) { draft = "" }
                    }
                } label: {
                    if busy {
                        ProgressView().controlSize(.small)
                    } else {
                        Label(pendingTurn == nil ? "Send" : "Retry send", systemImage: "arrow.up.circle.fill")
                    }
                }
                .buttonStyle(.borderedProminent)
                .disabled(
                    busy
                        || readiness.state != "ready"
                        || draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
                )
            }
        }
        .padding(12)
    }
}

// MARK: - Identity, options, Needs You, and settings

private struct AgentIdentityInspectionView: View {
    let channel: AgentChannelSummary
    let recheck: () async -> Void
    @Environment(\.dismiss) private var dismiss

    private var facts: AgentIdentityInspection { AgentIdentityInspection(channel: channel) }

    var body: some View {
        NavigationStack {
            Form {
                Section("Agent Seat") {
                    fact("Seat ID", facts.seatID)
                    fact("Agent", facts.agent)
                    fact("Profile", facts.profile)
                    fact("Seat model", facts.seatModel)
                    fact("Requested model", facts.requestedModel)
                    fact("Resolved model", facts.resolvedModel)
                    fact("Computer", facts.machine)
                }
                Section("Authority") {
                    fact("Authority", facts.authority)
                    fact("Policy ID", facts.policyID)
                    fact("Policy revision", facts.policyRevision)
                    fact("Adapter ID", facts.adapterID)
                    fact("Adapter revision", facts.adapterRevision)
                    fact("Runtime contract", facts.runtimeContract)
                    fact("Session mode", facts.sessionMode)
                    fact("Memory mode", facts.memoryMode)
                }
                Section("Current readiness") {
                    fact("State", facts.readiness)
                    if let reason = facts.readinessReason { fact("Reason", reason) }
                    Button("Recheck readiness") { Task { await recheck() } }
                }
            }
            .navigationTitle(facts.name)
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") { dismiss() }
                }
            }
        }
        .frame(minWidth: 420, minHeight: 480)
    }

    private func fact(_ label: String, _ value: String) -> some View {
        LabeledContent(label, value: value)
            .textSelection(.enabled)
    }
}

private struct AgentOptionsSheet: View {
    let options: [AgentOption]
    let recheck: () async -> Void
    let add: (String, String) async -> Bool
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            List {
                if options.isEmpty {
                    Text("No eligible agent options are currently available.")
                        .foregroundStyle(.secondary)
                }
                ForEach(options) { option in
                    AgentOptionEnrollmentRow(option: option, add: add)
                }
            }
            .navigationTitle("Add Agent Channel")
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
                ToolbarItem(placement: .primaryAction) {
                    Button("Recheck") { Task { await recheck() } }
                }
            }
        }
        .frame(minWidth: 440, minHeight: 480)
    }
}

private struct AgentOptionEnrollmentRow: View {
    let option: AgentOption
    let add: (String, String) async -> Bool
    @State private var name = ""
    @State private var busy = false

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(spacing: 10) {
                AgentIdentityView(
                    name: option.displayName,
                    identityKey: option.optionID,
                    size: 36
                )
                VStack(alignment: .leading, spacing: 3) {
                    Text(option.displayName).font(.headline)
                    Text("\(option.binding.seat.agent) · \(option.binding.seat.machine) · \(option.state)")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }
            if let reason = option.reason {
                Text(reason).font(.caption).foregroundStyle(FortPalette.needsYou)
            }
            TextField("Agent Channel name", text: $name)
            Button("Add Agent Channel") {
                busy = true
                Task {
                    _ = await add(option.optionID, name.trimmingCharacters(in: .whitespacesAndNewlines))
                    busy = false
                }
            }
            .buttonStyle(.borderedProminent)
            .disabled(option.state != "ready" || name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty || busy)
        }
        .padding(.vertical, 8)
        .onAppear { if name.isEmpty { name = option.displayName } }
    }
}

private struct AgentNeedsYouView: View {
    let items: [AgentNeedsYouItem]
    let open: (AgentNeedsYouItem) async -> Void
    let recover: (AgentNeedsYouItem, AgentTargetAction) async -> Void

    var body: some View {
        VStack(spacing: 0) {
            AgentPageHeader(
                title: "Needs You",
                subtitle: "Current durable chat failures with bounded recovery actions."
            )
            Divider()
            if items.isEmpty {
                VStack(spacing: 12) {
                    Image(systemName: "checkmark.circle")
                        .font(.system(size: 42))
                        .foregroundStyle(.green)
                    Text("Nothing needs you")
                        .font(.title3.bold())
                    Text("Resolved, canceled, and historical failures stay out of this list.")
                        .foregroundStyle(.secondary)
                        .multilineTextAlignment(.center)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .padding(30)
            } else {
                List(items) { item in
                    HStack(alignment: .top, spacing: 11) {
                        AgentIdentityView(
                            name: item.agentChannel.name,
                            identityKey: item.agentChannel.id,
                            size: 36
                        )
                        VStack(alignment: .leading, spacing: 5) {
                            Text(item.agentChannel.name).font(.headline)
                            Text(item.conversation.title).font(.callout)
                            Text(item.target.error ?? item.target.state)
                                .font(.caption)
                                .foregroundStyle(.secondary)
                            HStack {
                                Button("Open") { Task { await open(item) } }
                                if let action = AgentTargetRecovery.action(for: item) {
                                    Button(action == .retry ? "Retry" : "Recheck and retry") {
                                        Task { await recover(item, action) }
                                    }
                                    .buttonStyle(.borderedProminent)
                                }
                            }
                        }
                    }
                    .padding(.vertical, 6)
                }
            }
        }
        .navigationTitle("Needs You")
    }
}

private struct AgentSettingsView: View {
    let channels: [AgentChannelSummary]
    let options: [AgentOption]
    let connectionSettings: (() -> Void)?
    let inspect: (String) -> Void
    let showOptions: () -> Void
    let recheck: () async -> Void

    var body: some View {
        Form {
            Section("Agent Channels") {
                ForEach(channels) { channel in
                    Button { inspect(channel.id) } label: {
                        HStack(spacing: 10) {
                            AgentIdentityView(
                                name: channel.channel.name,
                                identityKey: channel.id,
                                size: 32
                            )
                            VStack(alignment: .leading) {
                                Text(channel.channel.name)
                                Text("\(channel.channel.binding.seat.agent) · \(channel.readiness.state)")
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                        }
                    }
                    .buttonStyle(.plain)
                }
                Button("Add Agent Channel", action: showOptions)
                Button("Recheck all options") { Task { await recheck() } }
            }
            Section("Eligible options") {
                Text("\(options.filter { $0.state == "ready" }.count) Ready of \(options.count)")
                    .foregroundStyle(.secondary)
            }
            if let connectionSettings {
                Section("Connection") {
                    Button("Connection settings", action: connectionSettings)
                }
            }
            Section("About") {
                FortReleaseIdentityView()
            }
        }
        .navigationTitle("Settings")
    }
}

private struct AgentChannelsWelcome: View {
    let addAgent: () -> Void

    var body: some View {
        VStack(spacing: 17) {
            FortProductMarkView(activity: .ambient, size: 78, decorative: true)
            Text("Your agents, one place")
                .font(.title2.bold())
            Text("Channels are agents. Choose an Agent Channel to continue one of its Conversations or start a separate context.")
                .multilineTextAlignment(.center)
                .foregroundStyle(.secondary)
                .frame(maxWidth: 520)
            Button("Add Agent Channel", action: addAgent)
                .buttonStyle(.borderedProminent)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .padding(30)
    }
}

private struct AgentPageHeader: View {
    let title: String
    let subtitle: String

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(title).font(.title2.bold())
            Text(subtitle).font(.callout).foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(20)
    }
}

private struct AgentChannelsPersistentChrome: View {
    var body: some View {
        HStack(spacing: 8) {
            FortProductMarkView(activity: .ambient, size: 24, decorative: true)
            Text("FORT")
                .font(.system(.caption, design: .monospaced).weight(.bold))
                .tracking(3)
            Spacer()
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 8)
        .accessibilityElement(children: .combine)
    }
}

private struct AgentLoadingView: View {
    let label: String

    var body: some View {
        VStack(spacing: 12) {
            FortProductMarkView(activity: .ambient, size: 52, decorative: true)
            ProgressView(label)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

private struct AgentErrorBanner: View {
    let message: String
    let dismiss: () -> Void

    var body: some View {
        HStack(spacing: 10) {
            Image(systemName: "exclamationmark.triangle.fill")
            Text(message).font(.callout)
            Spacer()
            Button("Dismiss", action: dismiss)
        }
        .padding(12)
        .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 12))
        .shadow(radius: 8)
        .accessibilityElement(children: .combine)
    }
}

private enum AgentRenameKind: Sendable {
    case agent
    case conversation
}

private struct AgentRenameTarget: Identifiable, Sendable {
    let id: String
    let kind: AgentRenameKind
    let currentName: String
}

private struct AgentRenameSheet: View {
    let target: AgentRenameTarget
    let save: (String) async -> Void
    @Environment(\.dismiss) private var dismiss
    @State private var name = ""

    var body: some View {
        NavigationStack {
            Form {
                TextField(target.kind == .agent ? "Agent Channel name" : "Conversation name", text: $name)
            }
            .navigationTitle(target.kind == .agent ? "Rename agent" : "Rename conversation")
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") {
                        Task {
                            await save(name.trimmingCharacters(in: .whitespacesAndNewlines))
                            dismiss()
                        }
                    }
                    .disabled(name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                }
            }
        }
        .frame(minWidth: 360, minHeight: 180)
        .onAppear { name = target.currentName }
    }
}
