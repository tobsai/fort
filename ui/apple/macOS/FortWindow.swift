//
//  FortWindow.swift
//  FortMac
//
//  The windowed macOS surface (spec 032): a `NavigationSplitView` that both
//  RUNS Fort (a "Service" section wired to `ServiceController` — the launchd
//  daemon lifecycle) and DRIVES it (a native mirror of the 031 web dashboard:
//  Define → Ready → In progress, with inline gate approve/reject and live
//  per-run activity from the SSE feed).
//
//  It is a faithful-but-native mirror, not pixel-perfect: it exercises the real
//  `FortClient` calls (summary/board/backlog/machines/chat/addBacklog/breakdown/
//  dispatchBacklog/runDetail/decideGate/events) over the existing HTTP/SSE
//  contract. The menu-bar app is retained; this window is additive.
//

import SwiftUI
import FortKit

/// The main window: sidebar (service controls + machines) beside a dashboard
/// detail (Define / Ready / In progress). Driven by a 3s refresh timer plus the
/// SSE live feed for per-run activity.
struct FortWindow: View {
    @EnvironmentObject private var client: FortClient
    @EnvironmentObject private var service: ServiceController

    // Control-plane snapshots, refreshed on a timer.
    @State private var summary: Summary?
    @State private var board = Board(runs: [], gates: [])
    @State private var backlog: [BacklogItem] = []
    @State private var machines: [MachineSummary] = []

    /// Per-run live activity buffers (tool/subagent/message events from SSE),
    /// keyed by run id and capped so long-lived windows don't grow unbounded.
    @State private var activity: [String: [Event]] = [:]

    // Compose ("Define") state.
    @State private var composeText = ""

    // Transient UI state.
    @State private var busy = false
    @State private var lastError: String?
    @State private var decidingGates: Set<String> = []

    private static let activityCap = 6

    /// Polls the control plane on an interval, like the menu-bar surface.
    private let refresh = Timer.publish(every: 3, on: .main, in: .common).autoconnect()

    var body: some View {
        NavigationSplitView {
            sidebar
                .frame(minWidth: 220)
        } detail: {
            dashboard
        }
        .task { await service.refresh(); await reload() }   // initial load
        .task { await streamEvents() }                       // SSE activity feed
        .onReceive(refresh) { _ in Task { await reload() } }
    }

    // MARK: - Sidebar

    private var sidebar: some View {
        List {
            Section("Service") {
                serviceIndicator
                serviceControls
            }
            Section("Machines") {
                if machines.isEmpty {
                    Text("Single-machine mode")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                } else {
                    ForEach(machines) { machineRow($0) }
                }
            }
        }
        .listStyle(.sidebar)
    }

    private var serviceIndicator: some View {
        HStack(spacing: 8) {
            Circle()
                .fill(service.status.running ? Color.green : Color.secondary)
                .frame(width: 9, height: 9)
            Text(service.status.running ? "Running" : "Stopped")
                .font(.callout.weight(.medium))
            Spacer()
        }
        .help(service.status.detail)
    }

    private var serviceControls: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(spacing: 6) {
                Button("Install") { Task { await service.install() } }
                Button("Start") { Task { await service.start() } }
            }
            HStack(spacing: 6) {
                Button("Stop") { Task { await service.stop() } }
                Button("Restart") { Task { await service.restart() } }
            }
        }
        .controlSize(.small)
    }

    private func machineRow(_ m: MachineSummary) -> some View {
        HStack(spacing: 8) {
            Image(systemName: m.local ? "desktopcomputer" : "network")
                .foregroundStyle(m.reachable ? Color.green : Color.secondary)
            VStack(alignment: .leading, spacing: 1) {
                Text(m.name).font(.callout)
                let agents = m.agents ?? []
                if !agents.isEmpty {
                    Text(agents.joined(separator: ", "))
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                }
            }
            Spacer()
        }
    }

    // MARK: - Dashboard detail

    private var dashboard: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                if let summary { countsRow(summary) }
                defineSection
                Divider()
                readySection
                Divider()
                progressSection
                if let lastError {
                    Label(lastError, systemImage: "exclamationmark.triangle")
                        .font(.caption)
                        .foregroundStyle(.red)
                }
            }
            .padding(20)
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .navigationTitle("Fort")
    }

    private func countsRow(_ s: Summary) -> some View {
        HStack(spacing: 20) {
            countPill(s.running, "Running", .green)
            countPill(s.queued, "Queued", .orange)
            countPill(s.blocked, "Blocked", .red)
            countPill(s.succeeded, "Done", .secondary)
            Spacer()
            if !s.execution {
                Label("Control-only", systemImage: "bolt.slash")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .help("No execution plane — chat boards a queued task; gate/breakdown need `fort serve`.")
            }
        }
    }

    private func countPill(_ value: Int, _ label: String, _ color: Color) -> some View {
        VStack(spacing: 2) {
            Text("\(value)")
                .font(.title2.monospacedDigit().weight(.semibold))
                .foregroundStyle(color)
            Text(label).font(.caption2).foregroundStyle(.secondary)
        }
    }

    // MARK: Define

    private var defineSection: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("Define").font(.headline)
            Text("First line is the title; the rest is the body.")
                .font(.caption)
                .foregroundStyle(.secondary)
            TextEditor(text: $composeText)
                .font(.body.monospaced())
                .frame(minHeight: 90)
                .overlay(RoundedRectangle(cornerRadius: 8).stroke(.quaternary))
            HStack(spacing: 10) {
                Button("Add to Ready") { Task { await addToReady() } }
                Button("Break down") { Task { await breakDown() } }
                Spacer()
                Button("Run ▸") { Task { await runNow() } }
                    .buttonStyle(.borderedProminent)
            }
            .disabled(busy || composeText.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
        }
    }

    // MARK: Ready

    private var readySection: some View {
        let queued = board.runs.filter { $0.status == "queued" }
        return VStack(alignment: .leading, spacing: 8) {
            sectionHeader("Ready", backlog.count + queued.count)
            if backlog.isEmpty && queued.isEmpty {
                emptyRow
            } else {
                ForEach(backlog) { readyItem($0) }
                ForEach(queued) { queuedItem($0) }
            }
        }
    }

    private func readyItem(_ b: BacklogItem) -> some View {
        card {
            VStack(alignment: .leading, spacing: 4) {
                Text(b.title).font(.callout.weight(.medium))
                if let body = b.body, !body.isEmpty {
                    Text(body).font(.caption).foregroundStyle(.secondary).lineLimit(3)
                }
                HStack {
                    if let agent = b.agent, !agent.isEmpty { tag(agent) }
                    if let machine = b.machine, !machine.isEmpty { tag(machine) }
                    Spacer()
                    Button("Start ▸") { Task { await dispatch(b) } }
                        .controlSize(.small)
                }
            }
        }
    }

    private func queuedItem(_ r: RunSummary) -> some View {
        card {
            HStack {
                VStack(alignment: .leading, spacing: 2) {
                    Text(r.title.isEmpty ? r.id : r.title).font(.callout.weight(.medium))
                    tag(r.agent)
                }
                Spacer()
                Text("queued").font(.caption2).foregroundStyle(.secondary)
            }
        }
    }

    // MARK: In progress

    private var progressSection: some View {
        let live = board.runs.filter { $0.status == "running" || $0.status == "blocked" }
        return VStack(alignment: .leading, spacing: 8) {
            sectionHeader("In progress", live.count)
            if live.isEmpty {
                emptyRow
            } else {
                ForEach(live) { progressItem($0) }
            }
        }
    }

    private func progressItem(_ r: RunSummary) -> some View {
        let gates = board.gates.filter { $0.runID == r.id }
        let acts = activity[r.id] ?? []
        return card {
            VStack(alignment: .leading, spacing: 6) {
                HStack {
                    Text(r.title.isEmpty ? r.id : r.title).font(.callout.weight(.medium))
                    Spacer()
                    tag(r.status)
                }
                if let body = r.body, !body.isEmpty {
                    Text(body).font(.caption).foregroundStyle(.secondary).lineLimit(2)
                }
                HStack(spacing: 6) { tag(r.agent) }
                ForEach(acts) { activityRow($0) }
                ForEach(gates) { gateRow($0) }
            }
        }
    }

    private func activityRow(_ e: Event) -> some View {
        let a = Self.activityText(e)
        return Label(a.text, systemImage: a.icon)
            .font(.caption)
            .foregroundStyle(.secondary)
            .lineLimit(1)
    }

    private func gateRow(_ gate: GateItem) -> some View {
        let busy = decidingGates.contains(gate.id)
        return HStack(spacing: 8) {
            Label("gate · \(gate.nodeID)", systemImage: "hand.raised")
                .font(.caption)
                .foregroundStyle(.orange)
            Spacer()
            Button("Approve") { Task { await decide(gate, "approve") } }
                .controlSize(.small).tint(.green)
            Button("Reject") { Task { await decide(gate, "reject") } }
                .controlSize(.small).tint(.red)
            if busy { ProgressView().controlSize(.small) }
        }
        .disabled(busy)
    }

    // MARK: - Small view helpers

    private func sectionHeader(_ title: String, _ count: Int) -> some View {
        HStack {
            Text(title).font(.headline)
            Text("\(count)").font(.caption).foregroundStyle(.secondary)
            Spacer()
        }
    }

    private var emptyRow: some View {
        Text("—").font(.callout).foregroundStyle(.secondary)
    }

    private func card<Content: View>(@ViewBuilder _ content: () -> Content) -> some View {
        content()
            .padding(10)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(.quaternary.opacity(0.35), in: RoundedRectangle(cornerRadius: 10))
    }

    private func tag(_ text: String) -> some View {
        Text(text)
            .font(.caption2)
            .padding(.horizontal, 6)
            .padding(.vertical, 2)
            .background(.quaternary.opacity(0.5), in: Capsule())
    }

    // MARK: - Activity formatting (mirrors the web dashboard's activityLine)

    /// Renders a tool/subagent/message event into an icon + one-line label.
    private static func activityText(_ e: Event) -> (icon: String, text: String) {
        switch e.type {
        case "tool":
            let d = parseData(e.data)
            let name = d["name"] ?? "tool"
            if let summary = d["summary"], !summary.isEmpty {
                return ("wrench.and.screwdriver", "\(name) · \(summary)")
            }
            return ("wrench.and.screwdriver", name)
        case "subagent":
            let d = parseData(e.data)
            var s = "subagent"
            if let agent = d["agent"], !agent.isEmpty { s += " (\(agent))" }
            if let desc = d["description"], !desc.isEmpty { s += " · \(desc)" }
            return ("cpu", s)
        default: // "message"
            let first = (e.data ?? "").split(separator: "\n", maxSplits: 1).first.map(String.init) ?? ""
            let clipped = first.count > 120 ? String(first.prefix(119)) + "…" : first
            return ("bubble.left", clipped)
        }
    }

    /// Best-effort parse of an event's JSON `data` into a flat string map.
    private static func parseData(_ s: String?) -> [String: String] {
        guard let s, let data = s.data(using: .utf8),
              let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any]
        else { return [:] }
        var out: [String: String] = [:]
        for (k, v) in obj { if let sv = v as? String { out[k] = sv } }
        return out
    }

    // MARK: - Data loading

    /// Refreshes summary, board, backlog and machines. Non-fatal: keeps the last
    /// good data and surfaces the error inline.
    private func reload() async {
        do {
            async let s = client.summary()
            async let b = client.board()
            async let bl = client.backlog()
            async let ms = client.machines()
            summary = try await s
            board = try await b
            backlog = try await bl
            machines = try await ms
            // Prune activity buffers for runs no longer in progress.
            let liveIDs = Set(board.runs.filter { $0.status == "running" || $0.status == "blocked" }.map(\.id))
            activity = activity.filter { liveIDs.contains($0.key) }
            lastError = nil
        } catch {
            lastError = friendly(error)
        }
    }

    /// Consumes the SSE feed, appending tool/subagent/message events to the
    /// per-run activity buffers. Ends quietly on error/cancel; the timer still
    /// refreshes the board.
    private func streamEvents() async {
        do {
            for try await e in client.events() {
                guard e.type == "tool" || e.type == "subagent" || e.type == "message" else { continue }
                var buf = activity[e.runID] ?? []
                buf.append(e)
                if buf.count > Self.activityCap { buf.removeFirst(buf.count - Self.activityCap) }
                activity[e.runID] = buf
            }
        } catch {
            // Transport/parse errors just end the stream; non-fatal.
        }
    }

    // MARK: - Compose actions

    private func split(_ text: String) -> (title: String, body: String) {
        if let nl = text.firstIndex(of: "\n") {
            let title = text[..<nl].trimmingCharacters(in: .whitespacesAndNewlines)
            let body = text[text.index(after: nl)...].trimmingCharacters(in: .whitespacesAndNewlines)
            return (title, body)
        }
        return (text.trimmingCharacters(in: .whitespacesAndNewlines), "")
    }

    private func runNow() async {
        let text = composeText.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { return }
        busy = true; defer { busy = false }
        do {
            _ = try await client.chat(text)
            composeText = ""
            await reload()
        } catch { lastError = friendly(error) }
    }

    private func addToReady() async {
        let (title, body) = split(composeText)
        guard !title.isEmpty else { return }
        busy = true; defer { busy = false }
        do {
            _ = try await client.addBacklog(title: title, body: body.isEmpty ? nil : body)
            composeText = ""
            await reload()
        } catch { lastError = friendly(error) }
    }

    private func breakDown() async {
        let text = composeText.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { return }
        busy = true; defer { busy = false }
        do {
            _ = try await client.breakdown(text)
            composeText = ""
            await reload()
        } catch { lastError = friendly(error) }
    }

    private func dispatch(_ b: BacklogItem) async {
        do {
            _ = try await client.dispatchBacklog(b.id)
            await reload()
        } catch { lastError = friendly(error) }
    }

    private func decide(_ gate: GateItem, _ decision: String) async {
        decidingGates.insert(gate.id)
        defer { decidingGates.remove(gate.id) }
        do {
            let applied = try await client.decideGate(run: gate.runID, node: gate.nodeID, decision: decision)
            if !applied { lastError = "No execution plane — gate action unavailable." }
            await reload()
        } catch { lastError = friendly(error) }
    }

    /// Turns a `FortClient` error into a short, human line.
    private func friendly(_ error: Error) -> String {
        switch error {
        case FortClientError.httpStatus(let status, _):
            return "Server error (\(status))."
        case FortClientError.nonHTTPResponse:
            return "Unexpected response."
        case let urlError as URLError where urlError.code == .cannotConnectToHost
            || urlError.code == .cannotFindHost
            || urlError.code == .networkConnectionLost:
            return "Fort not reachable — is the service running?"
        default:
            return error.localizedDescription
        }
    }
}
