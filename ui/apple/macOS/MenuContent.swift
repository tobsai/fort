//
//  MenuContent.swift
//  FortMac
//
//  The body of the menu-bar popover: glanceable summary counts, the pending
//  gate inbox with inline Approve/Reject, a quick-chat field, and a small
//  status footer. All Fort I/O goes through the shared `FortClient` (../FortKit).
//

import SwiftUI
import Combine
import FortKit

struct MenuContent: View {
    @EnvironmentObject private var client: FortClient
    @EnvironmentObject private var model: MenuModel

    /// Draft text for the quick-chat field.
    @State private var chatText: String = ""
    /// True while a chat submission is in flight (disables the field/button).
    @State private var sendingChat = false
    /// Gate ids ("run/node") currently being decided, to disable their buttons.
    @State private var decidingGates: Set<String> = []
    @State private var redirectGate: GateItem?

    /// Polls the summary on an interval; refreshed on appear and after actions.
    private let refresh = Timer.publish(every: 3, on: .main, in: .common).autoconnect()

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            header
            Divider()
            gatesSection
            Divider()
            counts
            Divider()
            chatField
            footer
        }
        .padding(14)
        .frame(width: 320)
        .task { await reload() }              // initial load
        .onReceive(refresh) { _ in Task { await reload() } }
        .sheet(item: $redirectGate) { gate in
            MenuRedirectSheet { note in Task { await decide(gate, "reject", note: note) } }
        }
    }

    // MARK: - Sections

    private var header: some View {
        HStack {
            Text("FORT")
                .font(.system(.callout, design: .monospaced).weight(.bold))
                .tracking(3)
                .foregroundStyle(FortPalette.brassBright)
            Spacer()
            if model.isControlOnly {
                Label("Control-only", systemImage: "bolt.slash")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                    .help("No execution plane attached — chat boards a queued task and gate actions are unavailable.")
            }
        }
    }

    private var counts: some View {
        HStack(spacing: 16) {
            countPill(model.summary?.running, "Working", FortPalette.working)
            countPill(model.summary?.queued, "Up next", FortPalette.muted)
            countPill(model.summary?.blocked, "Needs you", FortPalette.needsYou)
        }
        .frame(maxWidth: .infinity)
    }

    @ViewBuilder
    private var gatesSection: some View {
        let gates = model.summary?.gates ?? []
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("Needs you")
                    .font(.subheadline.weight(.semibold))
                Spacer()
                Text("\(gates.count)")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }

            if gates.isEmpty {
                Text("Everything is moving — no sign-offs waiting.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            } else {
                ForEach(gates) { gate in
                    gateRow(gate)
                }
            }
        }
    }

    private var chatField: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("Give direction")
                .font(.subheadline.weight(.semibold))
            HStack(spacing: 6) {
                TextField("What outcome do you want?", text: $chatText)
                    .textFieldStyle(.roundedBorder)
                    .disabled(sendingChat)
                    .onSubmit { Task { await sendChat() } }
                Button {
                    Task { await sendChat() }
                } label: {
                    if sendingChat {
                        ProgressView().controlSize(.small)
                    } else {
                        Image(systemName: "paperplane.fill")
                    }
                }
                .disabled(sendingChat || chatText.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                .keyboardShortcut(.return, modifiers: [])
            }
        }
    }

    @ViewBuilder
    private var footer: some View {
        if let notice = model.notice {
            Label(notice, systemImage: "info.circle")
                .font(.caption)
                .foregroundStyle(.secondary)
                .transition(.opacity)
        } else if let error = model.lastError {
            Label(error, systemImage: "exclamationmark.triangle")
                .font(.caption)
                .foregroundStyle(.red)
                .lineLimit(2)
        }

        HStack {
            if let total = model.summary?.total {
                Text("\(total) assignment\(total == 1 ? "" : "s")")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            }
            Spacer()
            Button("Quit Fort") { NSApplication.shared.terminate(nil) }
                .buttonStyle(.borderless)
                .font(.caption)
                .keyboardShortcut("q")
        }
    }

    // MARK: - Row builders

    private func countPill(_ value: Int?, _ label: String, _ color: Color) -> some View {
        VStack(spacing: 2) {
            Text(value.map(String.init) ?? "–")
                .font(.title3.monospacedDigit().weight(.semibold))
                .foregroundStyle(color)
            Text(label)
                .font(.caption2)
                .foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity)
    }

    private func gateRow(_ gate: GateItem) -> some View {
        let busy = decidingGates.contains(gate.id)
        return VStack(alignment: .leading, spacing: 4) {
            Text(gate.nodeID)
                .font(.callout.weight(.medium))
                .lineLimit(1)
            Text("assignment \(gate.runID)")
                .font(.caption2)
                .foregroundStyle(.secondary)
                .lineLimit(1)
            if let input = gate.input, !input.isEmpty {
                Text(input)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(2)
            }
            HStack(spacing: 8) {
                Button("Accept") { Task { await decide(gate, "approve") } }
                    .buttonStyle(.borderedProminent)
                    .controlSize(.small)
                    .tint(.green)
                Button("Request changes") { redirectGate = gate }
                    .buttonStyle(.bordered)
                    .controlSize(.small)
                    .tint(.red)
                if busy {
                    ProgressView().controlSize(.small)
                }
            }
            .disabled(busy)
        }
        .padding(8)
        .background(.quaternary.opacity(0.4), in: RoundedRectangle(cornerRadius: 8))
    }

    // MARK: - Actions

    /// Refreshes the summary snapshot. Non-fatal: on failure we keep the last
    /// good data and surface the error in the footer.
    private func reload() async {
        do {
            let summary = try await client.summary()
            model.summary = summary
            model.lastError = nil
        } catch {
            model.lastError = friendly(error)
        }
    }

    /// Submits the quick-chat text as a task, then refreshes.
    private func sendChat() async {
        let text = chatText.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty, !sendingChat else { return }
        sendingChat = true
        defer { sendingChat = false }
        do {
            let result = try await client.chat(text)
            chatText = ""
            // Control-only mode boards a queued task rather than routing it.
            if result.queued == true {
                model.flash("Task queued (control-only).")
            } else {
                model.flash("Filed \(result.kind) \(result.runID).")
            }
            await reload()
        } catch {
            model.lastError = friendly(error)
        }
    }

    /// Decides a gate. `decideGate` returns `false` on HTTP 409 (no execution
    /// plane) — a non-fatal condition we surface as a notice rather than an error.
    private func decide(_ gate: GateItem, _ decision: String, note: String? = nil) async {
        decidingGates.insert(gate.id)
        defer { decidingGates.remove(gate.id) }
        do {
            let applied = try await client.decideGate(
                run: gate.runID,
                node: gate.nodeID,
                decision: decision,
                note: note
            )
            if applied {
                model.flash("\(decision.capitalized)d \(gate.nodeID).")
            } else {
                model.flash("No execution plane — gate action unavailable.")
            }
            await reload()
        } catch {
            model.lastError = friendly(error)
        }
    }

    /// Turns a `FortClient` error into a short, human line for the footer.
    private func friendly(_ error: Error) -> String {
        switch error {
        case FortClientError.httpStatus(let status, _):
            return "Server error (\(status))."
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

private struct MenuRedirectSheet: View {
    @Environment(\.dismiss) private var dismiss
    let submit: (String) -> Void
    @State private var note = ""

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Request changes").font(.headline)
            TextField("What should change?", text: $note, axis: .vertical).lineLimit(3...6)
            HStack {
                Spacer()
                Button("Cancel") { dismiss() }
                Button("Send") { submit(note.trimmingCharacters(in: .whitespacesAndNewlines)); dismiss() }
                    .buttonStyle(.borderedProminent)
                    .disabled(note.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
            }
        }
        .padding(16)
        .frame(width: 360)
    }
}
