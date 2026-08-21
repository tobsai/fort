//
//  MenuContent.swift
//  FortMac
//
//  A bounded mode-matched chat glance: open destination count, recoverable
//  Needs You rows, and a handoff to the full window. All I/O goes through FortKit.
//

import AppKit
import Combine
import FortKit
import SwiftUI

struct MenuContent: View {
    let mode: AgentChannelsPresentationMode
    @EnvironmentObject private var client: FortClient
    @EnvironmentObject private var model: MenuModel
    @Environment(\.openWindow) private var openWindow

    /// Keeps the badge and recoverable rows current while the menu is open.
    private let refresh = Timer.publish(every: 3, on: .main, in: .common).autoconnect()

    init(mode: AgentChannelsPresentationMode = .off) {
        self.mode = mode
    }

    var body: some View {
        FortMarkSurface {
            VStack(alignment: .leading, spacing: 12) {
                header
                Divider()
                needsYouSection
                Divider()
                counts
                Divider()
                footer
            }
            .padding(14)
            .frame(width: 340)
        }
        .task { await reload() }
        .onReceive(refresh) { _ in Task { await reload() } }
    }

    private var header: some View {
        HStack(spacing: 8) {
            FortProductMarkView(activity: .ambient, size: 25, decorative: true)
            Text("FORT")
                .font(.system(.callout, design: .monospaced).weight(.bold))
                .tracking(3)
                .foregroundStyle(FortPalette.brassBright)
            Text(mode == .primary ? "Agent Channels" : "Primary Channels")
                .font(.caption)
                .foregroundStyle(.secondary)
            Spacer()
        }
    }

    @ViewBuilder
    private var needsYouSection: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("Needs You")
                    .font(.subheadline.weight(.semibold))
                Spacer()
                Text("\(model.pendingNeedsYou)")
                    .font(.caption.monospacedDigit())
                    .foregroundStyle(.secondary)
            }

            switch mode {
            case .off:
                if model.needsYou.isEmpty {
                    emptyNeedsYou
                } else {
                    ForEach(Array(model.needsYou.prefix(4))) { item in
                        primaryNeedsYouRow(item)
                    }
                    if model.needsYou.count > 4 {
                        Text("\(model.needsYou.count - 4) more in Fort")
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                    }
                }
            case .primary:
                if model.agentNeedsYou.isEmpty {
                    emptyNeedsYou
                } else {
                    ForEach(Array(model.agentNeedsYou.prefix(4))) { item in
                        agentNeedsYouRow(item)
                    }
                    if model.agentNeedsYou.count > 4 {
                        Text("\(model.agentNeedsYou.count - 4) more in Fort")
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                    }
                }
            }
        }
    }

    private var emptyNeedsYou: some View {
        Text("Nothing needs recovery.")
            .font(.caption)
            .foregroundStyle(.secondary)
    }

    @ViewBuilder
    private var counts: some View {
        switch mode {
        case .off:
            HStack(spacing: 16) {
                countPill(model.channels.count, "Open Channels", FortPalette.working)
                countPill(model.pendingNeedsYou, "Needs You", FortPalette.needsYou)
            }
            .frame(maxWidth: .infinity)
        case .primary:
            HStack(spacing: 16) {
                countPill(model.agentChannels.count, "Open Agents", FortPalette.working)
                countPill(model.pendingNeedsYou, "Needs You", FortPalette.needsYou)
            }
            .frame(maxWidth: .infinity)
        }
    }

    @ViewBuilder
    private var footer: some View {
        if let error = model.lastError {
            Label(error, systemImage: "exclamationmark.triangle")
                .font(.caption)
                .foregroundStyle(.red)
                .lineLimit(2)
        }

        HStack {
            Button("Open Fort") { showMainWindow() }
                .buttonStyle(.borderedProminent)
                .controlSize(.small)
            Spacer()
            Button("Quit Fort") { NSApplication.shared.terminate(nil) }
                .buttonStyle(.borderless)
                .font(.caption)
                .keyboardShortcut("q")
        }
    }

    private func primaryNeedsYouRow(_ item: PrimaryNeedsYouItem) -> some View {
        let presentation = PrimaryTargetStatusReducer.presentation(
            for: item.target,
            machine: item.channel.participant.machine
        )
        return Button { showMainWindow() } label: {
            HStack(alignment: .top, spacing: 8) {
                Image(systemName: "exclamationmark.circle.fill")
                    .foregroundStyle(FortPalette.needsYou)
                    .padding(.top, 2)
                VStack(alignment: .leading, spacing: 2) {
                    Text(item.channel.conversation.title)
                        .font(.callout.weight(.medium))
                        .lineLimit(1)
                    Text(presentation?.title ?? "Recovery available")
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                    if let body = presentation?.body, !body.isEmpty {
                        Text(body)
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                            .lineLimit(2)
                    }
                }
                Spacer(minLength: 4)
                Image(systemName: "chevron.right")
                    .font(.caption2)
                    .foregroundStyle(.tertiary)
                    .padding(.top, 4)
            }
            .padding(8)
            .background(.quaternary.opacity(0.4), in: RoundedRectangle(cornerRadius: 8))
        }
        .buttonStyle(.plain)
    }

    private func agentNeedsYouRow(_ item: AgentNeedsYouItem) -> some View {
        Button { showMainWindow() } label: {
            HStack(alignment: .top, spacing: 8) {
                Image(systemName: "exclamationmark.circle.fill")
                    .foregroundStyle(FortPalette.needsYou)
                    .padding(.top, 2)
                VStack(alignment: .leading, spacing: 2) {
                    Text(item.agentChannel.name)
                        .font(.callout.weight(.medium))
                        .lineLimit(1)
                    Text(item.conversation.title)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                    Text(item.target.error ?? "Recovery available")
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                        .lineLimit(2)
                }
                Spacer(minLength: 4)
                Image(systemName: "chevron.right")
                    .font(.caption2)
                    .foregroundStyle(.tertiary)
                    .padding(.top, 4)
            }
            .padding(8)
            .background(.quaternary.opacity(0.4), in: RoundedRectangle(cornerRadius: 8))
        }
        .buttonStyle(.plain)
    }

    private func countPill(_ value: Int, _ label: String, _ color: Color) -> some View {
        VStack(spacing: 2) {
            Text(String(value))
                .font(.title3.monospacedDigit().weight(.semibold))
                .foregroundStyle(color)
            Text(label)
                .font(.caption2)
                .foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity)
    }

    private func showMainWindow() {
        openWindow(id: "main")
        NSApplication.shared.activate(ignoringOtherApps: true)
    }

    /// Refreshes one exact product mode; it never probes or falls back across modes.
    private func reload() async {
        do {
            switch mode {
            case .off:
                async let nextNeedsYou = client.primaryNeedsYou()
                async let nextChannels = client.primaryChannels(state: .open)
                let (needsYou, channels) = try await (nextNeedsYou, nextChannels)
                model.acceptPrimary(needsYou: needsYou, channels: channels)
            case .primary:
                async let nextNeedsYou = client.agentNeedsYou()
                async let nextChannels = client.agentChannels(state: .open)
                let (needsYou, channels) = try await (nextNeedsYou, nextChannels)
                model.acceptAgentChannels(needsYou: needsYou, channels: channels)
            }
            model.lastError = nil
        } catch {
            model.lastError = friendly(error)
        }
    }

    private func friendly(_ error: Error) -> String {
        switch error {
        case FortClientError.httpStatus(let status, _, let requestID):
            let correlation = requestID.map { " Request ID \($0)." } ?? ""
            return "Server error (\(status)).\(correlation)"
        case FortClientError.nonHTTPResponse:
            return "Unexpected response."
        case let urlError as URLError where urlError.code == .cannotConnectToHost
            || urlError.code == .cannotFindHost
            || urlError.code == .networkConnectionLost:
            return "Fort not reachable."
        default:
            return error.localizedDescription
        }
    }
}
