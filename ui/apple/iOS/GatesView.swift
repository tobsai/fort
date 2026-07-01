//
//  GatesView.swift
//  Fort (iOS)
//
//  The Gates tab — the gate inbox. Polls GET /api/gates every ~2s and lets the
//  operator Approve or Reject each waiting gate via client.decideGate.
//
//  decideGate returns `false` on HTTP 409 (no execution plane — control-only
//  mode). That is NOT an error: we show a non-fatal notice and leave the gate
//  in place, since there is nothing to decide against.
//

import SwiftUI
import FortKit

struct GatesView: View {
    @EnvironmentObject private var client: FortClient

    @State private var gates: [GateItem] = []
    @State private var loadError: String?

    /// Gate ids currently mid-decision, to disable their buttons.
    @State private var inFlight: Set<String> = []

    /// Non-fatal notice (e.g. control-only 409), shown as an alert.
    @State private var notice: String?

    var body: some View {
        List {
            if gates.isEmpty {
                ContentUnavailableCompat(
                    title: loadError == nil ? "Inbox zero" : "Can't reach Fort",
                    message: loadError ?? "No gates are waiting on a decision.",
                    systemImage: loadError == nil ? "checkmark.seal" : "wifi.slash"
                )
            } else {
                ForEach(gates) { gate in
                    GateRow(
                        gate: gate,
                        busy: inFlight.contains(gate.id),
                        onApprove: { decide(gate, "approve") },
                        onReject: { decide(gate, "reject") }
                    )
                }
            }
        }
        .listStyle(.insetGrouped)
        .navigationTitle("Gates")
        .task(id: client.baseURL) { await runLoop() }
        .refreshable { await refreshOnce() }
        .alert("Heads up", isPresented: noticeBinding) {
            Button("OK", role: .cancel) { notice = nil }
        } message: {
            Text(notice ?? "")
        }
    }

    private var noticeBinding: Binding<Bool> {
        Binding(get: { notice != nil }, set: { if !$0 { notice = nil } })
    }

    // MARK: - Decisions

    private func decide(_ gate: GateItem, _ decision: String) {
        guard !inFlight.contains(gate.id) else { return }
        inFlight.insert(gate.id)
        Task {
            defer { inFlight.remove(gate.id) }
            do {
                let applied = try await client.decideGate(
                    run: gate.runID,
                    node: gate.nodeID,
                    decision: decision
                )
                if applied {
                    await refreshOnce() // gate should drop off the inbox
                } else {
                    // HTTP 409 — control-only: nothing to decide against.
                    notice = "No execution plane attached (control-only mode). Gate decisions can't be applied until a deterministic engine is running."
                }
            } catch {
                notice = "Couldn't \(decision) this gate: \(errorText(error))"
            }
        }
    }

    // MARK: - Polling

    private func runLoop() async {
        await refreshOnce()
        while !Task.isCancelled {
            try? await Task.sleep(nanoseconds: 2_000_000_000)
            if Task.isCancelled { break }
            await refreshOnce()
        }
    }

    private func refreshOnce() async {
        do {
            gates = try await client.gates()
            loadError = nil
        } catch {
            loadError = errorText(error)
        }
    }
}

private struct GateRow: View {
    let gate: GateItem
    let busy: Bool
    let onApprove: () -> Void
    let onReject: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 6) {
                Image(systemName: "hand.raised.fill")
                    .foregroundStyle(.purple)
                Text(gate.nodeID)
                    .font(.body.weight(.medium).monospaced())
                Spacer()
                Text(gate.runID)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
            }

            if let input = gate.input, !input.isEmpty {
                Text(input)
                    .font(.callout)
                    .foregroundStyle(.secondary)
                    .lineLimit(4)
                    .padding(8)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(Color.secondary.opacity(0.08), in: RoundedRectangle(cornerRadius: 8))
            }

            HStack(spacing: 12) {
                Button(role: .destructive, action: onReject) {
                    Label("Reject", systemImage: "xmark")
                        .frame(maxWidth: .infinity)
                }
                .buttonStyle(.bordered)
                .disabled(busy)

                Button(action: onApprove) {
                    Label("Approve", systemImage: "checkmark")
                        .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
                .disabled(busy)
            }
            .overlay(alignment: .center) {
                if busy { ProgressView() }
            }
        }
        .padding(.vertical, 4)
    }
}
