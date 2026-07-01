//
//  GlanceView.swift
//  FortWatch
//
//  The watch app's single screen: a glanceable summary (counts) at the top and
//  the pending-gate inbox below, with a one-tap Approve on the first gate.
//
//  Control-only handling: when `decideGate` returns false (HTTP 409, no
//  execution plane), we surface a non-fatal notice rather than treating it as a
//  failure. The same signal is visible at a glance via `Summary.execution`.
//

import SwiftUI
import FortKit

struct GlanceView: View {
    @EnvironmentObject private var client: FortClient

    @State private var summary: Summary?
    @State private var loadError: String?
    @State private var notice: String?
    @State private var isDeciding = false

    var body: some View {
        NavigationStack {
            List {
                summarySection
                gatesSection
                if let notice {
                    Section {
                        Label(notice, systemImage: "info.circle")
                            .font(.footnote)
                            .foregroundStyle(.secondary)
                    }
                }
                if let loadError {
                    Section {
                        Label(loadError, systemImage: "exclamationmark.triangle")
                            .font(.footnote)
                            .foregroundStyle(.orange)
                    }
                }
            }
            .navigationTitle("Fort")
            .task { await refresh() }
            .refreshable { await refresh() }
        }
    }

    // MARK: - Sections

    @ViewBuilder
    private var summarySection: some View {
        Section("Runs") {
            if let summary {
                CountRow(label: "Running", value: summary.running, tint: .green)
                CountRow(label: "Queued", value: summary.queued, tint: .blue)
                CountRow(label: "Blocked", value: summary.blocked, tint: .orange)
                CountRow(label: "Failed", value: summary.failed, tint: .red)
                if !summary.execution {
                    Label("Control-only — no execution plane", systemImage: "bolt.slash")
                        .font(.footnote)
                        .foregroundStyle(.secondary)
                }
            } else if loadError == nil {
                ProgressView()
                    .frame(maxWidth: .infinity, alignment: .center)
            }
        }
    }

    @ViewBuilder
    private var gatesSection: some View {
        let gates = summary?.gates ?? []
        Section("Gates") {
            if gates.isEmpty {
                Text("No gates waiting")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            } else {
                ForEach(Array(gates.enumerated()), id: \.element.id) { index, gate in
                    GateRow(
                        gate: gate,
                        canApprove: index == 0 && !isDeciding,
                        approve: { await approve(gate) }
                    )
                }
            }
        }
    }

    // MARK: - Actions

    private func refresh() async {
        do {
            summary = try await client.summary()
            loadError = nil
        } catch {
            loadError = "Can't reach Fort"
        }
    }

    private func approve(_ gate: GateItem) async {
        guard !isDeciding else { return }
        isDeciding = true
        notice = nil
        defer { isDeciding = false }
        do {
            let applied = try await client.decideGate(
                run: gate.runID,
                node: gate.nodeID,
                decision: "approve"
            )
            if applied {
                await refresh()
            } else {
                // HTTP 409 — control-only mode, no execution plane to act on.
                notice = "No execution plane — can't approve here"
            }
        } catch {
            notice = "Approve failed"
        }
    }
}

// MARK: - Rows

private struct CountRow: View {
    let label: String
    let value: Int
    let tint: Color

    var body: some View {
        HStack {
            Text(label)
            Spacer()
            Text("\(value)")
                .font(.headline)
                .foregroundStyle(tint)
        }
    }
}

private struct GateRow: View {
    let gate: GateItem
    let canApprove: Bool
    let approve: () async -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(gate.nodeID)
                .font(.headline)
                .lineLimit(1)
            Text(gate.runID)
                .font(.caption2)
                .foregroundStyle(.secondary)
                .lineLimit(1)
            if let input = gate.input, !input.isEmpty {
                Text(input)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(2)
            }
            if canApprove {
                Button {
                    Task { await approve() }
                } label: {
                    Label("Approve", systemImage: "checkmark.circle.fill")
                        .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
                .tint(.green)
            }
        }
        .padding(.vertical, 2)
    }
}

#Preview {
    GlanceView()
        .environmentObject(FortClient())
}
