//
//  MessagingView.swift
//  FortKit
//
//  Account-global Spec 053 Hermes Messaging Channels directory and exact
//  machine-routed transcript surface.
//

import SwiftUI

private struct MessagingChannelCacheEnvelope: Codable {
    let accountScope: String
    let channels: [MessagingChannel]
}

private struct MessagingHiddenEnvelope: Codable {
    let accountScope: String
    let channelIDs: Set<MessagingChannelIdentity>
}

private struct MessagingDeliveryEnvelope: Codable {
    let accountScope: String
    let notices: [MessagingDeliveryNotice]
}

/// Account-level directory for profile-scoped Hermes Messaging Channels. Each
/// row retains the exact machine transport that discovered it.
@MainActor
public final class MessagingChannelsModel: ObservableObject {
    private static let cacheKey = "fort.messaging-channels.cache.v1"
    private static let hiddenKey = "fort.messaging-channels.hidden.v1"
    private static let deliveryKey = "fort.messaging-channels.delivery.v1"
    private static let unreachableReason = "The owning Fort machine is unreachable."

    @Published public private(set) var visibleChannels: [MessagingChannel] = []
    @Published public private(set) var hiddenChannels: [MessagingChannel] = []
    @Published public private(set) var selectedChannel: MessagingChannel?
    @Published public private(set) var events: [MessagingEvent] = []
    @Published public private(set) var deliveryNotices: [MessagingDeliveryNotice] = []
    @Published public private(set) var busy = false
    @Published public var errorMessage: String?

    private let defaults: UserDefaults
    private var cacheScope: String?
    private var cachedChannels: [MessagingChannel]
    private var projectedChannels: [MessagingChannel]
    private var hiddenIDs: Set<MessagingChannelIdentity>
    private var cachedDeliveryNotices: [MessagingDeliveryNotice]
    private var acceptedEventsByChannel: [MessagingChannelIdentity: [MessagingEvent]] = [:]
    private var clients: [MessagingChannelIdentity: FortClient] = [:]
    private var after: Int64 = 0
    private var selectionGeneration: UInt64 = 0
    private var refreshGeneration: UInt64 = 0

    public convenience init() {
        self.init(defaults: .standard)
    }

    package init(defaults: UserDefaults) {
        self.defaults = defaults
        let decoder = JSONDecoder()
        if let data = defaults.data(forKey: Self.cacheKey),
           let cached = try? decoder.decode(MessagingChannelCacheEnvelope.self, from: data) {
            cacheScope = cached.accountScope
            cachedChannels = cached.channels
        } else {
            cacheScope = nil
            cachedChannels = []
            defaults.removeObject(forKey: Self.cacheKey)
        }
        if let data = defaults.data(forKey: Self.hiddenKey),
           let hidden = try? decoder.decode(MessagingHiddenEnvelope.self, from: data),
           hidden.accountScope == cacheScope {
            hiddenIDs = hidden.channelIDs
        } else {
            hiddenIDs = []
            defaults.removeObject(forKey: Self.hiddenKey)
        }
        if let data = defaults.data(forKey: Self.deliveryKey),
           let delivery = try? decoder.decode(MessagingDeliveryEnvelope.self, from: data),
           delivery.accountScope == cacheScope {
            cachedDeliveryNotices = delivery.notices
        } else {
            cachedDeliveryNotices = []
            defaults.removeObject(forKey: Self.deliveryKey)
        }
        // A cache is never projected until a current trusted source proves the
        // same opaque gateway-plus-account scope. Raw email and token values
        // are neither persisted nor exposed by this presentation layer.
        projectedChannels = []
        renderDirectory()
    }

    public var messages: [MessagingMessage] {
        var result = events.map(\.message)
        guard let selectedChannel else { return result }
        var observedIDs = Set(result.map(\.id))
        for event in acceptedEventsByChannel[selectedChannel.id, default: []]
            .sorted(by: { $0.sequence < $1.sequence })
            where observedIDs.insert(event.message.id).inserted {
            result.append(event.message)
        }
        result.append(contentsOf: deliveryNotices
            .filter { $0.channelID == selectedChannel.id && !observedIDs.contains($0.messageID) }
            .map { $0.markerMessage(conversationID: selectedChannel.conversationID) })
        return result
    }

    /// Returns transcript rows only when the destination view matches the
    /// model's exact selected Messaging Channel.
    public func conversationMessages(
        channelID: MessagingChannelIdentity
    ) -> [MessagingMessage] {
        guard selectedChannel?.id == channelID else { return [] }
        return messages
    }

    /// Prevents a destination view from presenting another channel's error
    /// while its asynchronous selection task is starting.
    public func conversationError(
        channelID: MessagingChannelIdentity
    ) -> String? {
        guard selectedChannel?.id == channelID else { return nil }
        return errorMessage
    }

    public func deliveryNotice(
        channelID: MessagingChannelIdentity,
        messageID: String
    ) -> MessagingDeliveryNotice? {
        deliveryNotices.first {
            $0.channelID == channelID && $0.messageID == messageID
        }
    }

    public func channel(channelID: MessagingChannelIdentity) -> MessagingChannel? {
        projectedChannels.first { $0.id == channelID }
    }

    public func run(sources: [MessagingChannelSource]) async {
        while !Task.isCancelled {
            await refresh(sources: sources)
            await pollSelected()
            do { try await Task.sleep(nanoseconds: 3_000_000_000) }
            catch { return }
        }
    }

    public func refresh(sources: [MessagingChannelSource]) async {
        refreshGeneration &+= 1
        let generation = refreshGeneration
        let scopes = Set(sources.map(\.accountScope))
        guard scopes.count == 1, let accountScope = scopes.first else {
            closeDirectoryProjection(
                error: scopes.isEmpty ? nil : "Fort received Messaging Channel sources from different accounts."
            )
            return
        }
        if cacheScope != accountScope {
            cacheScope = accountScope
            cachedChannels = []
            projectedChannels = []
            hiddenIDs = []
            cachedDeliveryNotices = []
            deliveryNotices = []
            acceptedEventsByChannel = [:]
            defaults.removeObject(forKey: Self.cacheKey)
            defaults.removeObject(forKey: Self.hiddenKey)
            defaults.removeObject(forKey: Self.deliveryKey)
        }

        var cachedByID: [MessagingChannelIdentity: MessagingChannel] = [:]
        var projectedByID: [MessagingChannelIdentity: MessagingChannel] = [:]
        for channel in cachedChannels {
            cachedByID[channel.id] = channel
            projectedByID[channel.id] = channel.projectedOffline(reason: Self.unreachableReason)
        }
        var nextClients: [MessagingChannelIdentity: FortClient] = [:]
        var successfulSources = 0

        for source in sources {
            let cachedForSource = projectedByID.values.filter {
                $0.identity.machineID == source.machineID
            }
            for channel in cachedForSource {
                projectedByID[channel.id] = channel.projectedOffline(
                    reason: Self.unreachableReason,
                    trustedMachineName: source.machineName
                )
                nextClients[channel.id] = source.client
            }
            do {
                let peers = try await source.client.messagingChannels()
                guard refreshGeneration == generation,
                      cacheScope == accountScope
                else { return }
                var identities = Set<MessagingChannelIdentity>()
                var sourceChannels: [MessagingChannel] = []
                for peer in peers {
                    guard !peer.id.isEmpty,
                          !peer.sourceID.isEmpty,
                          !peer.canonicalProfileID.isEmpty,
                          !peer.displayName.isEmpty,
                          !peer.machineName.isEmpty,
                          peer.machineName == source.machineName,
                          !peer.conversationID.isEmpty
                    else { throw MessagingDirectoryError.invalidProjection }
                    let channel = MessagingChannel(source: source, peer: peer)
                    guard identities.insert(channel.identity).inserted else {
                        throw MessagingDirectoryError.invalidProjection
                    }
                    sourceChannels.append(channel)
                }
                if sourceChannels.isEmpty, !cachedForSource.isEmpty {
                    resetProcessLocalTranscript(forMachineID: source.machineID)
                }
                for channel in sourceChannels {
                    cachedByID[channel.identity] = channel
                    projectedByID[channel.identity] = channel
                    nextClients[channel.identity] = source.client
                }
                successfulSources += 1
            } catch is CancellationError {
                return
            } catch {
                guard refreshGeneration == generation,
                      cacheScope == accountScope
                else { return }
                errorMessage = Self.describe(error)
            }
        }

        guard refreshGeneration == generation,
              cacheScope == accountScope
        else { return }
        cachedChannels = Array(cachedByID.values).sorted(by: Self.channelOrder)
        projectedChannels = Array(projectedByID.values).sorted(by: Self.channelOrder)
        deliveryNotices = cachedDeliveryNotices
        persistCache()
        renderDirectory()
        clients = nextClients
        if let selectedChannel {
            let replacement = projectedChannels.first(where: { $0.id == selectedChannel.id })
            self.selectedChannel = replacement
            if replacement == nil {
                events = []
                after = 0
                selectionGeneration &+= 1
            }
        }
        if successfulSources > 0 { errorMessage = nil }
    }

    /// Changes only this device's directory presentation. It does not send an
    /// HTTP command or mutate Hermes registration or authorization.
    public func setHidden(channelID: MessagingChannelIdentity, hidden: Bool) {
        guard projectedChannels.contains(where: { $0.id == channelID }) else { return }
        if hidden {
            hiddenIDs.insert(channelID)
        } else {
            hiddenIDs.remove(channelID)
        }
        persistHiddenIDs()
        renderDirectory()
    }

    public func select(channelID: MessagingChannelIdentity) async {
        selectionGeneration &+= 1
        let generation = selectionGeneration
        guard let channel = (visibleChannels + hiddenChannels).first(where: { $0.id == channelID }) else {
            selectedChannel = nil
            events = []
            after = 0
            errorMessage = "This Hermes Messaging Channel is unavailable."
            return
        }

        selectedChannel = channel
        events = []
        after = 0
        errorMessage = nil
        guard clients[channelID] != nil else {
            errorMessage = "This Hermes Messaging Channel is unavailable."
            return
        }
        guard channel.state == .connected else {
            errorMessage = channel.reason ?? "This Hermes Messaging Channel is offline."
            return
        }
        await pollSelected(channelID: channelID, generation: generation)
    }

    public func pollSelected() async {
        guard let selectedChannel else { return }
        await pollSelected(channelID: selectedChannel.id, generation: selectionGeneration)
    }

    public func send(text: String) async -> Bool {
        let normalized = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !normalized.isEmpty, !busy,
              let channel = selectedChannel,
              let client = clients[channel.id]
        else { return false }
        guard channel.state == .connected else {
            errorMessage = channel.reason ?? "This Hermes Messaging Channel is offline."
            return false
        }

        let generation = selectionGeneration
        let sendAccountScope = cacheScope
        busy = true
        defer { busy = false }
        do {
            let receipt = try await client.postMessagingMessage(
                conversationID: channel.conversationID,
                clientMessageID: UUID().uuidString.lowercased(),
                text: normalized
            )
            guard receipt.message.conversationID == channel.conversationID else {
                throw MessagingPresentationError.invalidProjection
            }
            switch receipt.deliveryState {
            case .pending:
                guard receipt.deliveryCode == nil else {
                    throw MessagingPresentationError.invalidProjection
                }
            case .unknown:
                guard let deliveryCode = receipt.deliveryCode,
                      deliveryCode == "hermes_relay_delivery_failed"
                else {
                    throw MessagingPresentationError.invalidProjection
                }
            }
            guard cacheScope == sendAccountScope else {
                // The old account's authenticated acceptance cannot be
                // projected or persisted into the newly selected account.
                return true
            }
            let isCurrentSelection = selectionGeneration == generation
                && selectedChannel?.id == channel.id
            if isCurrentSelection {
                try accept(receipt: receipt, for: channel)
            }
            try recordAccepted(receipt: receipt, for: channel)
            if receipt.deliveryState == .unknown {
                let notice = MessagingDeliveryNotice(
                    channelID: channel.id,
                    messageID: receipt.message.id,
                    acceptedSequence: receipt.acceptedSequence,
                    deliveryCode: receipt.deliveryCode!
                )
                if let index = cachedDeliveryNotices.firstIndex(where: { $0.id == notice.id }) {
                    guard cachedDeliveryNotices[index] == notice else {
                        throw MessagingPresentationError.invalidProjection
                    }
                } else {
                    cachedDeliveryNotices.append(notice)
                }
                deliveryNotices = cachedDeliveryNotices
                persistDeliveryNotices()
            }
            guard isCurrentSelection else { return true }
            errorMessage = nil
            return true
        } catch is CancellationError {
            return false
        } catch {
            guard selectionGeneration == generation,
                  selectedChannel?.id == channel.id
            else { return false }
            errorMessage = Self.describe(error)
            return false
        }
    }

    private func pollSelected(
        channelID: MessagingChannelIdentity,
        generation: UInt64
    ) async {
        guard let channel = selectedChannel,
              channel.id == channelID,
              channel.state == .connected,
              let client = clients[channelID]
        else { return }
        let requestedAfter = after
        do {
            let page = try await client.messagingEvents(
                conversationID: channel.conversationID,
                after: requestedAfter
            )
            guard selectionGeneration == generation,
                  selectedChannel?.id == channelID
            else { return }
            try accept(page: page, for: channel, requestedAfter: requestedAfter)
            errorMessage = nil
        } catch is CancellationError {
            return
        } catch {
            guard selectionGeneration == generation,
                  selectedChannel?.id == channelID
            else { return }
            errorMessage = Self.describe(error)
        }
    }

    private func accept(
        page: MessagingEventsPage,
        for channel: MessagingChannel,
        requestedAfter: Int64
    ) throws {
        guard page.conversationID == channel.conversationID else {
            throw MessagingPresentationError.invalidProjection
        }
        let retainedAcceptances = acceptedEventsByChannel[channel.id, default: []]
        var previous = requestedAfter
        for event in page.events {
            guard previous < Int64.max,
                  event.sequence == previous + 1,
                  event.message.conversationID == channel.conversationID
            else { throw MessagingPresentationError.invalidProjection }
            if let retained = retainedAcceptances.first(where: {
                $0.sequence == event.sequence || $0.message.id == event.message.id
            }), retained != event {
                throw MessagingPresentationError.invalidProjection
            }
            previous = event.sequence
        }
        guard page.nextAfter == previous else {
            throw MessagingPresentationError.invalidProjection
        }
        try validateMessagingEventIdentities(
            existing: events + retainedAcceptances,
            incoming: page.events
        )
        for event in page.events {
            if let observed = events.first(where: { $0.sequence == event.sequence }) {
                guard observed == event else {
                    throw MessagingPresentationError.invalidProjection
                }
                continue
            }
            guard event.sequence > after else {
                throw MessagingPresentationError.invalidProjection
            }
            events.append(event)
            after = event.sequence
        }
        after = max(after, page.nextAfter)
    }

    private func accept(
        receipt: MessagingMessageReceipt,
        for channel: MessagingChannel
    ) throws {
        let acceptedEvent = MessagingEvent(
            sequence: receipt.acceptedSequence,
            message: receipt.message
        )
        try validateMessagingEventIdentities(
            existing: events + acceptedEventsByChannel[channel.id, default: []],
            incoming: [acceptedEvent]
        )
        if let observed = events.first(where: { $0.sequence == receipt.acceptedSequence }) {
            guard observed.message == receipt.message else {
                throw MessagingPresentationError.invalidProjection
            }
            return
        }
        guard receipt.acceptedSequence > after else {
            throw MessagingPresentationError.invalidProjection
        }
        guard after < Int64.max else {
            throw MessagingPresentationError.invalidProjection
        }
        if receipt.acceptedSequence == after + 1 {
            events.append(acceptedEvent)
            after = receipt.acceptedSequence
        }
    }

    private func recordAccepted(
        receipt: MessagingMessageReceipt,
        for channel: MessagingChannel
    ) throws {
        guard receipt.acceptedSequence > 0 else {
            throw MessagingPresentationError.invalidProjection
        }
        let event = MessagingEvent(
            sequence: receipt.acceptedSequence,
            message: receipt.message
        )
        var accepted = acceptedEventsByChannel[channel.id, default: []]
        if let observed = accepted.first(where: { $0.sequence == event.sequence }) {
            guard observed == event else {
                throw MessagingPresentationError.invalidProjection
            }
            return
        }
        guard !accepted.contains(where: { $0.message.id == event.message.id }) else {
            throw MessagingPresentationError.invalidProjection
        }
        accepted.append(event)
        acceptedEventsByChannel[channel.id] = accepted
    }

    private static func channelOrder(_ lhs: MessagingChannel, _ rhs: MessagingChannel) -> Bool {
        if lhs.displayName != rhs.displayName { return lhs.displayName < rhs.displayName }
        if lhs.machineName != rhs.machineName { return lhs.machineName < rhs.machineName }
        if lhs.identity.machineID != rhs.identity.machineID {
            return lhs.identity.machineID < rhs.identity.machineID
        }
        return lhs.identity.channelID < rhs.identity.channelID
    }

    private func resetProcessLocalTranscript(forMachineID machineID: String) {
        acceptedEventsByChannel = acceptedEventsByChannel.filter {
            $0.key.machineID != machineID
        }
        guard selectedChannel?.identity.machineID == machineID else { return }
        events = []
        after = 0
        selectionGeneration &+= 1
    }

    private func renderDirectory() {
        visibleChannels = projectedChannels
            .filter { !hiddenIDs.contains($0.id) }
            .sorted(by: Self.channelOrder)
        hiddenChannels = projectedChannels
            .filter { hiddenIDs.contains($0.id) }
            .sorted(by: Self.channelOrder)
    }

    private func persistCache() {
        guard let cacheScope,
              let data = try? JSONEncoder().encode(MessagingChannelCacheEnvelope(
                accountScope: cacheScope,
                channels: cachedChannels
              ))
        else { return }
        defaults.set(data, forKey: Self.cacheKey)
    }

    private func persistHiddenIDs() {
        guard let cacheScope,
              let data = try? JSONEncoder().encode(MessagingHiddenEnvelope(
                accountScope: cacheScope,
                channelIDs: hiddenIDs
              ))
        else { return }
        defaults.set(data, forKey: Self.hiddenKey)
    }

    private func persistDeliveryNotices() {
        guard let cacheScope,
              let data = try? JSONEncoder().encode(MessagingDeliveryEnvelope(
                accountScope: cacheScope,
                notices: cachedDeliveryNotices
              ))
        else { return }
        defaults.set(data, forKey: Self.deliveryKey)
    }

    private func closeDirectoryProjection(error: String?) {
        projectedChannels = []
        clients = [:]
        selectedChannel = nil
        events = []
        deliveryNotices = []
        after = 0
        selectionGeneration &+= 1
        renderDirectory()
        errorMessage = error
    }

    private static func describe(_ error: Error) -> String {
        if let localized = error as? LocalizedError,
           let description = localized.errorDescription,
           !description.isEmpty {
            return description
        }
        return "Fort could not update Hermes Messaging Channels."
    }
}

private func validateMessagingEventIdentities(
    existing: [MessagingEvent],
    incoming: [MessagingEvent]
) throws {
    var eventsBySequence: [Int64: MessagingEvent] = [:]
    var eventsByMessageID: [String: MessagingEvent] = [:]
    for event in existing + incoming {
        if let observed = eventsBySequence[event.sequence], observed != event {
            throw MessagingPresentationError.invalidProjection
        }
        if let observed = eventsByMessageID[event.message.id], observed != event {
            throw MessagingPresentationError.invalidProjection
        }
        eventsBySequence[event.sequence] = event
        eventsByMessageID[event.message.id] = event
    }
}

private enum MessagingDirectoryError: LocalizedError {
    case invalidProjection

    var errorDescription: String? {
        "Fort returned an invalid Hermes Messaging Channel directory."
    }
}

/// Top-level directory of Hermes profiles registered as Fort Messaging
/// Channels across the account's already trusted machines.
public struct MessagingChannelsView: View {
    private let sources: [MessagingChannelSource]
    private let sourceWarning: String?
    @StateObject private var model = MessagingChannelsModel()

    public init(sources: [MessagingChannelSource]) {
        self.sources = sources
        sourceWarning = nil
    }

    public init(sourceResolution: MessagingChannelSourceResolution) {
        sources = sourceResolution.sources
        sourceWarning = sourceResolution.warning
    }

    public var body: some View {
        NavigationStack {
            List {
                if let sourceWarning {
                    Section {
                        Label(sourceWarning, systemImage: "exclamationmark.shield.fill")
                            .font(.footnote)
                            .foregroundStyle(.orange)
                    }
                }
                Section("Hermes") {
                    if model.visibleChannels.isEmpty {
                        VStack(spacing: 8) {
                            Image(systemName: "bubble.left.and.bubble.right")
                                .foregroundStyle(.secondary)
                            Text("No visible Hermes channels")
                                .font(.headline)
                            Text("Hermes profiles appear here after they connect to Fort.")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                        .frame(maxWidth: .infinity)
                    }
                    ForEach(model.visibleChannels) { channel in
                        NavigationLink(value: channel.id) {
                            channelRow(channel)
                        }
                        .swipeActions(edge: .trailing) {
                            Button("Hide") {
                                model.setHidden(channelID: channel.id, hidden: true)
                            }
                            .tint(.secondary)
                        }
                    }
                }

                if !model.hiddenChannels.isEmpty {
                    Section("Hidden") {
                        ForEach(model.hiddenChannels) { channel in
                            HStack {
                                NavigationLink(value: channel.id) {
                                    channelRow(channel)
                                }
                                Button("Unhide") {
                                    model.setHidden(channelID: channel.id, hidden: false)
                                }
                                .buttonStyle(.borderless)
                            }
                        }
                    }
                }
            }
            .navigationTitle("Channels")
            .navigationDestination(for: MessagingChannelIdentity.self) { identity in
                MessagingChannelConversationView(model: model, channelID: identity)
            }
            .overlay(alignment: .top) {
                if let message = model.errorMessage,
                   model.visibleChannels.isEmpty,
                   model.hiddenChannels.isEmpty {
                    messagingErrorBanner(message)
                        .padding(12)
                }
            }
        }
        .task(id: sourceTaskIdentity) {
            await model.run(sources: sources)
        }
    }

    private func channelRow(_ channel: MessagingChannel) -> some View {
        VStack(alignment: .leading, spacing: 3) {
            Text(channel.displayName)
                .font(.headline)
            Text(channel.subtitle)
                .font(.caption)
                .foregroundStyle(channel.state == .connected ? .green : .secondary)
        }
        .accessibilityElement(children: .combine)
    }

    private var sourceTaskIdentity: String {
        sources
            .sorted { $0.machineID < $1.machineID }
            .map {
                "\($0.machineID)|\($0.machineName)|\($0.isReachable)|\($0.accountScope)|\($0.transportRevision)"
            }
            .joined(separator: "\n")
    }

    private func messagingErrorBanner(_ message: String) -> some View {
        HStack(spacing: 8) {
            Image(systemName: "exclamationmark.triangle.fill")
            Text(message).font(.callout)
            Spacer()
            Button("Dismiss") { model.errorMessage = nil }
        }
        .padding(10)
        .background(.thinMaterial, in: RoundedRectangle(cornerRadius: 12))
        .shadow(radius: 5, y: 2)
    }
}

private struct MessagingChannelConversationView: View {
    @ObservedObject var model: MessagingChannelsModel
    let channelID: MessagingChannelIdentity
    @State private var draft = ""

    var body: some View {
        VStack(spacing: 0) {
            conversationHeader
            Divider()
            transcript
            Divider()
            composer
        }
        .navigationTitle(destinationChannel?.displayName ?? "Hermes Channel")
        .task(id: channelID) {
            await model.select(channelID: channelID)
        }
        .overlay(alignment: .top) {
            if let message = model.conversationError(channelID: channelID) {
                HStack(spacing: 8) {
                    Image(systemName: "exclamationmark.triangle.fill")
                    Text(message).font(.callout)
                    Spacer()
                    Button("Dismiss") { model.errorMessage = nil }
                }
                .padding(10)
                .background(.thinMaterial, in: RoundedRectangle(cornerRadius: 12))
                .shadow(radius: 5, y: 2)
                .padding(12)
            }
        }
    }

    private var destinationChannel: MessagingChannel? {
        model.channel(channelID: channelID)
    }

    private var presentedMessages: [MessagingMessage] {
        model.conversationMessages(channelID: channelID)
    }

    @ViewBuilder
    private var conversationHeader: some View {
        if let channel = destinationChannel {
            HStack {
                VStack(alignment: .leading, spacing: 2) {
                    Text(channel.displayName)
                        .font(.headline)
                    Text(channel.subtitle)
                        .font(.caption)
                        .foregroundStyle(channel.state == .connected ? .green : .secondary)
                }
                Spacer()
                Button("Hide") {
                    model.setHidden(channelID: channel.id, hidden: true)
                }
                .buttonStyle(.borderless)
            }
            .padding()
            .accessibilityElement(children: .combine)
        } else {
            HStack {
                ProgressView()
                Text("Opening Hermes channel…")
                Spacer()
            }
            .padding()
        }
    }

    private var transcript: some View {
        ScrollViewReader { proxy in
            ScrollView {
                LazyVStack(spacing: 14) {
                    if presentedMessages.isEmpty {
                        VStack(spacing: 10) {
                            Image(systemName: "bubble.left")
                                .font(.system(size: 34))
                                .foregroundStyle(.secondary)
                            Text("No messages yet")
                                .font(.headline)
                        }
                        .padding(.top, 48)
                    }
                    ForEach(presentedMessages) { message in
                        messageRow(message)
                            .id(message.id)
                    }
                }
                .padding()
            }
            .onChange(of: presentedMessages.last?.id) { messageID in
                guard let messageID else { return }
                withAnimation { proxy.scrollTo(messageID, anchor: .bottom) }
            }
        }
    }

    private func messageRow(_ message: MessagingMessage) -> some View {
        let notice = model.deliveryNotice(
            channelID: channelID,
            messageID: message.id
        )
        let isOutcomeMarker = notice?.isMarkerMessage(message) == true
        return HStack {
            if message.authorKind == .human { Spacer(minLength: 44) }
            VStack(alignment: .leading, spacing: 4) {
                Text(
                    isOutcomeMarker
                        ? "Fort"
                        : (message.authorKind == .human ? "You" : (destinationChannel?.displayName ?? "Hermes"))
                )
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(.secondary)
                if isOutcomeMarker, let notice {
                    Label(notice.marker, systemImage: "exclamationmark.triangle.fill")
                        .font(.caption.weight(.semibold))
                        .foregroundStyle(.orange)
                } else {
                    Text(message.body)
                        .textSelection(.enabled)
                    if let notice {
                        Label(notice.marker, systemImage: "exclamationmark.triangle.fill")
                            .font(.caption.weight(.semibold))
                            .foregroundStyle(.orange)
                    }
                }
            }
            .padding(11)
            .background(
                message.authorKind == .human
                    ? Color.accentColor.opacity(0.14)
                    : Color.secondary.opacity(0.10),
                in: RoundedRectangle(cornerRadius: 13)
            )
            if message.authorKind == .peer { Spacer(minLength: 44) }
        }
        .frame(maxWidth: .infinity)
        .accessibilityElement(children: .combine)
    }

    private var composer: some View {
        HStack(alignment: .bottom, spacing: 10) {
            TextField(
                destinationChannel.map { "Message \($0.displayName)" } ?? "Message Hermes",
                text: $draft,
                axis: .vertical
            )
            .lineLimit(1...5)
            .textFieldStyle(.roundedBorder)
            Button {
                let submitted = draft
                Task {
                    if await model.send(text: submitted) {
                        draft = ""
                    }
                }
            } label: {
                if model.busy {
                    ProgressView()
                } else {
                    Image(systemName: "arrow.up.circle.fill")
                        .font(.title2)
                }
            }
            .buttonStyle(.plain)
            .accessibilityLabel("Send message")
            .disabled(
                model.busy
                    || model.selectedChannel?.id != channelID
                    || model.selectedChannel?.state != .connected
                    || draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
            )
        }
        .padding()
    }
}

@MainActor
package final class MessagingModel: ObservableObject {
    @Published package private(set) var peer: MessagingPeer?
    @Published package private(set) var events: [MessagingEvent] = []
    @Published package private(set) var busy = false
    @Published package var errorMessage: String?

    private var after: Int64 = 0

    package init() {}

    package var messages: [MessagingMessage] { events.map(\.message) }

    package func run(using client: FortClient) async {
        await load(using: client)
        while !Task.isCancelled {
            do { try await Task.sleep(nanoseconds: 1_000_000_000) }
            catch { return }
            await poll(using: client)
        }
    }

    package func load(using client: FortClient) async {
        do {
            let peers = try await client.messagingPeers()
            guard peers.count == 1, let exactPeer = peers.first else {
                throw MessagingPresentationError.exactlyOnePeerRequired
            }
            if peer?.id != exactPeer.id
                || peer?.conversationID != exactPeer.conversationID
                || peer?.machineName != exactPeer.machineName
            {
                events = []
                after = 0
            }
            peer = exactPeer
            errorMessage = nil
            await poll(using: client)
        } catch is CancellationError {
            return
        } catch {
            errorMessage = Self.describe(error)
        }
    }

    package func poll(using client: FortClient) async {
        guard let peer else { return }
        let requestedAfter = after
        do {
            let page = try await client.messagingEvents(
                conversationID: peer.conversationID,
                after: requestedAfter
            )
            try accept(page, for: peer, requestedAfter: requestedAfter)
            errorMessage = nil
        } catch is CancellationError {
            return
        } catch {
            errorMessage = Self.describe(error)
        }
    }

    package func send(text: String, using client: FortClient) async -> Bool {
        let normalized = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !normalized.isEmpty, !busy, let peer else { return false }
        guard peer.state == .connected else {
            errorMessage = peer.reason ?? "Hermes is offline."
            return false
        }

        busy = true
        defer { busy = false }
        do {
            let receipt = try await client.postMessagingMessage(
                conversationID: peer.conversationID,
                clientMessageID: UUID().uuidString.lowercased(),
                text: normalized
            )
            guard receipt.message.conversationID == peer.conversationID else {
                throw MessagingPresentationError.invalidProjection
            }
            let acceptedEvent = MessagingEvent(
                sequence: receipt.acceptedSequence,
                message: receipt.message
            )
            try validateMessagingEventIdentities(
                existing: events,
                incoming: [acceptedEvent]
            )
            if let observed = events.first(where: { $0.sequence == receipt.acceptedSequence }) {
                guard observed.message == receipt.message else {
                    throw MessagingPresentationError.invalidProjection
                }
                errorMessage = nil
                return true
            }
            guard receipt.acceptedSequence > after else {
                throw MessagingPresentationError.invalidProjection
            }
            events.append(acceptedEvent)
            if receipt.acceptedSequence == after + 1 {
                after = receipt.acceptedSequence
            }
            events.sort { $0.sequence < $1.sequence }
            errorMessage = nil
            return true
        } catch is CancellationError {
            return false
        } catch {
            errorMessage = Self.describe(error)
            return false
        }
    }

    private func accept(
        _ page: MessagingEventsPage,
        for peer: MessagingPeer,
        requestedAfter: Int64
    ) throws {
        guard page.conversationID == peer.conversationID else {
            throw MessagingPresentationError.invalidProjection
        }
        var previous = requestedAfter
        for event in page.events {
            guard previous < Int64.max,
                  event.sequence == previous + 1,
                  event.message.conversationID == peer.conversationID
            else { throw MessagingPresentationError.invalidProjection }
            previous = event.sequence
        }
        guard page.nextAfter == previous else {
            throw MessagingPresentationError.invalidProjection
        }
        try validateMessagingEventIdentities(existing: events, incoming: page.events)
        for event in page.events {
            if let observed = events.first(where: { $0.sequence == event.sequence }) {
                guard observed == event else {
                    throw MessagingPresentationError.invalidProjection
                }
                continue
            }
            guard event.sequence > after else {
                throw MessagingPresentationError.invalidProjection
            }
            events.append(event)
            after = event.sequence
        }
        events.sort { $0.sequence < $1.sequence }
        after = max(after, page.nextAfter)
    }

    private static func describe(_ error: Error) -> String {
        if let localized = error as? LocalizedError,
           let description = localized.errorDescription,
           !description.isEmpty {
            return description
        }
        return "Fort could not update this Hermes conversation."
    }
}

package enum MessagingPresentationError: Error {
    case exactlyOnePeerRequired
    case invalidProjection
}

extension MessagingPresentationError: LocalizedError {
    package var errorDescription: String? {
        switch self {
        case .exactlyOnePeerRequired:
            return "This proof requires exactly one configured Hermes peer."
        case .invalidProjection:
            return "Fort returned messages for a different Hermes conversation."
        }
    }
}

public struct MessagingView: View {
    @EnvironmentObject private var client: FortClient
    @StateObject private var model = MessagingModel()
    @State private var draft = ""

    public init() {}

    public var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                peerHeader
                Divider()
                transcript
                Divider()
                composer
            }
            .navigationTitle(model.peer?.displayName ?? "Connecting…")
            .overlay(alignment: .top) {
                if let message = model.errorMessage {
                    errorBanner(message)
                        .padding(12)
                }
            }
        }
        .task(id: "\(client.baseURL.absoluteString)|\(client.transportGeneration)") {
            await model.run(using: client)
        }
    }

    @ViewBuilder
    private var peerHeader: some View {
        if let peer = model.peer {
            HStack(spacing: 10) {
                Image(systemName: "bubble.left.and.bubble.right.fill")
                    .foregroundStyle(.tint)
                    .accessibilityHidden(true)
                VStack(alignment: .leading, spacing: 2) {
                    Text(peer.headerSubtitle)
                        .font(.caption)
                        .foregroundStyle(peer.state == .connected ? .green : .secondary)
                }
                Spacer()
            }
            .padding()
            .accessibilityElement(children: .combine)
        } else {
            HStack(spacing: 10) {
                ProgressView()
                Text("Connecting to Hermes…")
                Spacer()
            }
            .padding()
        }
    }

    private var transcript: some View {
        ScrollViewReader { proxy in
            ScrollView {
                LazyVStack(spacing: 14) {
                    if model.messages.isEmpty {
                        VStack(spacing: 10) {
                            Image(systemName: "bubble.left")
                                .font(.system(size: 34))
                                .foregroundStyle(.secondary)
                            Text("No messages yet")
                                .font(.headline)
                            Text("Send a message to the configured Hermes bot.")
                                .font(.callout)
                                .foregroundStyle(.secondary)
                                .multilineTextAlignment(.center)
                        }
                        .padding(.top, 48)
                    }
                    ForEach(model.messages) { message in
                        messageRow(message)
                            .id(message.id)
                    }
                }
                .padding()
            }
            .onChange(of: model.messages.last?.id) { messageID in
                guard let messageID else { return }
                withAnimation { proxy.scrollTo(messageID, anchor: .bottom) }
            }
        }
    }

    private func messageRow(_ message: MessagingMessage) -> some View {
        HStack {
            if message.authorKind == .human { Spacer(minLength: 44) }
            VStack(alignment: .leading, spacing: 4) {
                Text(message.authorKind == .human ? "You" : (model.peer?.displayName ?? "Hermes"))
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(.secondary)
                Text(message.body)
                    .textSelection(.enabled)
            }
            .padding(11)
            .background(
                message.authorKind == .human
                    ? Color.accentColor.opacity(0.14)
                    : Color.secondary.opacity(0.10),
                in: RoundedRectangle(cornerRadius: 13)
            )
            if message.authorKind == .peer { Spacer(minLength: 44) }
        }
        .frame(maxWidth: .infinity)
        .accessibilityElement(children: .combine)
    }

    private var composer: some View {
        HStack(alignment: .bottom, spacing: 10) {
            TextField("Message Hermes", text: $draft, axis: .vertical)
                .lineLimit(1...5)
                .textFieldStyle(.roundedBorder)
            Button {
                let submitted = draft
                Task {
                    if await model.send(text: submitted, using: client) {
                        draft = ""
                    }
                }
            } label: {
                if model.busy {
                    ProgressView()
                } else {
                    Image(systemName: "arrow.up.circle.fill")
                        .font(.title2)
                }
            }
            .buttonStyle(.plain)
            .accessibilityLabel("Send message")
            .disabled(
                model.busy
                    || model.peer?.state != .connected
                    || draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
            )
        }
        .padding()
    }

    private func errorBanner(_ message: String) -> some View {
        HStack(spacing: 8) {
            Image(systemName: "exclamationmark.triangle.fill")
            Text(message).font(.callout)
            Spacer()
            Button("Dismiss") { model.errorMessage = nil }
        }
        .padding(10)
        .background(.thinMaterial, in: RoundedRectangle(cornerRadius: 12))
        .shadow(radius: 5, y: 2)
    }
}
