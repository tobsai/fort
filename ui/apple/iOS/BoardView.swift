//
//  BoardView.swift
//  Fort (iOS)
//
//  The Board tab. Polls GET /api/summary and GET /api/board every ~2s and
//  renders runs as cards with a status badge. A chat field at the bottom
//  submits a turn via client.chat and surfaces where it landed.
//

import SwiftUI
import FortKit

struct BoardView: View {
    @EnvironmentObject private var client: FortClient

    @State private var summary: Summary?
    @State private var runs: [RunSummary] = []
    @State private var loadError: String?

    @State private var draft: String = ""
    @State private var sending = false
    @State private var lastResult: String?

    /// Drives the 2s poll loop; recreated whenever the base URL changes so the
    /// loop restarts against the new host.
    @State private var pollTask: Task<Void, Never>?

    var body: some View {
        VStack(spacing: 0) {
            runList
            Divider()
            chatBar
        }
        .navigationTitle("Board")
        .task(id: client.baseURL) {
            await runLoop()
        }
    }

    // MARK: - Runs

    private var runList: some View {
        List {
            if let summary {
                Section {
                    SummaryStrip(summary: summary)
                        .listRowInsets(EdgeInsets(top: 8, leading: 16, bottom: 8, trailing: 16))
                }
            }

            Section("Runs") {
                if runs.isEmpty {
                    ContentUnavailableCompat(
                        title: loadError == nil ? "No runs yet" : "Can't reach Fort",
                        message: loadError ?? "Chat below to board a task.",
                        systemImage: loadError == nil ? "tray" : "wifi.slash"
                    )
                } else {
                    ForEach(runs) { run in
                        NavigationLink(value: run.id) {
                            RunRow(run: run)
                        }
                    }
                }
            }
        }
        .listStyle(.insetGrouped)
        .navigationDestination(for: String.self) { runID in
            RunDetailView(runID: runID)
        }
        .refreshable { await refreshOnce() }
    }

    // MARK: - Chat

    private var chatBar: some View {
        VStack(alignment: .leading, spacing: 6) {
            if let lastResult {
                Text(lastResult)
                    .font(.footnote)
                    .foregroundStyle(.secondary)
                    .transition(.opacity)
            }
            HStack(spacing: 8) {
                TextField("Message Fort…", text: $draft, axis: .vertical)
                    .textFieldStyle(.roundedBorder)
                    .lineLimit(1...4)
                    .submitLabel(.send)
                    .onSubmit(send)
                    .disabled(sending)

                Button(action: send) {
                    if sending {
                        ProgressView()
                    } else {
                        Image(systemName: "arrow.up.circle.fill")
                            .font(.title2)
                    }
                }
                .disabled(sending || draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                .accessibilityLabel("Send")
            }
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 10)
        .background(.bar)
    }

    private func send() {
        let text = draft.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty, !sending else { return }
        sending = true
        Task {
            defer { sending = false }
            do {
                let result = try await client.chat(text)
                draft = ""
                withAnimation { lastResult = describe(result) }
                await refreshOnce() // reflect the new run promptly
            } catch {
                withAnimation { lastResult = "Send failed: \(errorText(error))" }
            }
        }
    }

    private func describe(_ r: ChatResult) -> String {
        switch r.kind {
        case "flow":
            if let paused = r.paused {
                return "Flow \(r.flowID ?? r.runID) paused at gate \(paused)."
            }
            return "Started flow \(r.flowID ?? r.runID)."
        default:
            if r.queued == true {
                return "Boarded a queued task (control-only)."
            }
            if let route = r.route {
                return "Routed to \(route) as \(r.runID)."
            }
            return "Boarded task \(r.runID)."
        }
    }

    // MARK: - Polling

    private func runLoop() async {
        await refreshOnce()
        // Poll every ~2s until the surrounding .task is cancelled (tab change,
        // baseURL change, or view teardown).
        while !Task.isCancelled {
            try? await Task.sleep(nanoseconds: 2_000_000_000)
            if Task.isCancelled { break }
            await refreshOnce()
        }
    }

    private func refreshOnce() async {
        do {
            async let s = client.summary()
            async let b = client.board()
            let (summaryValue, boardValue) = try await (s, b)
            summary = summaryValue
            runs = boardValue.runs
            loadError = nil
        } catch {
            loadError = errorText(error)
        }
    }
}

// MARK: - Rows & badges

private struct RunRow: View {
    let run: RunSummary

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text(run.title.isEmpty ? run.id : run.title)
                    .font(.body.weight(.medium))
                    .lineLimit(1)
                Spacer()
                StatusBadge(status: run.status)
            }
            HStack(spacing: 6) {
                Label(run.agent.isEmpty ? "unassigned" : run.agent, systemImage: "person")
                    .labelStyle(.titleAndIcon)
                if let flow = run.flowID, !flow.isEmpty {
                    Label(flow, systemImage: "arrow.triangle.branch")
                }
            }
            .font(.caption)
            .foregroundStyle(.secondary)
        }
        .padding(.vertical, 2)
    }
}

/// A colored pill for a run status. Unknown statuses fall back to gray.
struct StatusBadge: View {
    let status: String

    var body: some View {
        Text(status.isEmpty ? "—" : status)
            .font(.caption2.weight(.semibold))
            .textCase(.uppercase)
            .padding(.horizontal, 8)
            .padding(.vertical, 3)
            .background(color.opacity(0.18), in: Capsule())
            .foregroundStyle(color)
    }

    private var color: Color {
        switch status.lowercased() {
        case "running":            return .blue
        case "queued", "pending":  return .orange
        case "blocked", "paused":  return .purple
        case "succeeded", "done":  return .green
        case "failed", "error":    return .red
        default:                   return .gray
        }
    }
}

/// The counts row derived from Summary, with a control-only indicator.
private struct SummaryStrip: View {
    let summary: Summary

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 14) {
                stat("Running", summary.running, .blue)
                stat("Queued", summary.queued, .orange)
                stat("Blocked", summary.blocked, .purple)
                stat("Done", summary.succeeded, .green)
                stat("Failed", summary.failed, .red)
            }
            if !summary.execution {
                Label("Control-only — no execution plane. Chat boards queued tasks; gate actions are unavailable.",
                      systemImage: "exclamationmark.triangle")
                    .font(.caption2)
                    .foregroundStyle(.orange)
            }
        }
    }

    private func stat(_ label: String, _ value: Int, _ color: Color) -> some View {
        VStack(spacing: 2) {
            Text("\(value)")
                .font(.headline.monospacedDigit())
                .foregroundStyle(color)
            Text(label)
                .font(.caption2)
                .foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity)
    }
}

// MARK: - Run detail

/// A read-only replay of a single run: its nodes and its event log.
struct RunDetailView: View {
    @EnvironmentObject private var client: FortClient
    let runID: String

    @State private var detail: RunDetail?
    @State private var loadError: String?

    var body: some View {
        List {
            if let detail {
                Section("Run") {
                    RunRow(run: detail.run)
                }
                Section("Nodes") {
                    if detail.nodes.isEmpty {
                        Text("No nodes.").foregroundStyle(.secondary)
                    } else {
                        ForEach(detail.nodes) { node in
                            HStack {
                                VStack(alignment: .leading, spacing: 2) {
                                    Text(node.nodeID).font(.body.monospaced())
                                    Text(node.type).font(.caption).foregroundStyle(.secondary)
                                }
                                Spacer()
                                if let attempts = node.attempts, attempts > 1 {
                                    Text("×\(attempts)").font(.caption2).foregroundStyle(.secondary)
                                }
                                StatusBadge(status: node.status)
                            }
                        }
                    }
                }
                Section("Events") {
                    if detail.events.isEmpty {
                        Text("No events.").foregroundStyle(.secondary)
                    } else {
                        ForEach(detail.events) { event in
                            EventRow(event: event)
                        }
                    }
                }
            } else if let loadError {
                ContentUnavailableCompat(
                    title: "Can't load run",
                    message: loadError,
                    systemImage: "wifi.slash"
                )
            } else {
                ProgressView()
            }
        }
        .listStyle(.insetGrouped)
        .navigationTitle(runID)
        .navigationBarTitleDisplayMode(.inline)
        .task(id: runID) { await load() }
        .refreshable { await load() }
    }

    private func load() async {
        do {
            detail = try await client.runDetail(runID)
            loadError = nil
        } catch {
            loadError = errorText(error)
        }
    }
}
