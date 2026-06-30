// Fort iOS shell (backlog AO-037) — a minimal SwiftUI client pointed at
// fort-core. Shows live runs (GET /api/board) and approves/rejects gates
// (POST /api/gate). Mobile visibility + gate approvals are Fort's edge over
// Multica (which has no mobile surface).
//
// This is a scaffold: drop these types into an Xcode iOS App target. Set
// `FortClient.base` to your fort-core address (default http://127.0.0.1:4087).

import SwiftUI

// MARK: - Contract (mirrors ui/contract.go)

struct RunSummary: Codable, Identifiable {
    let id: String
    let title: String
    let agent: String
    let status: String
    var flow_id: String?
}

struct GateItem: Codable, Identifiable {
    let run_id: String
    let node_id: String
    var input: String?
    var id: String { run_id + ":" + node_id }
}

struct Board: Codable {
    var runs: [RunSummary]
    var gates: [GateItem]
}

struct GateDecision: Codable {
    let run_id: String
    let node_id: String
    let decision: String // "approve" | "reject"
}

// MARK: - Client

final class FortClient: ObservableObject {
    static let base = URL(string: "http://127.0.0.1:4087")!
    @Published var board = Board(runs: [], gates: [])

    func refresh() async {
        guard let data = try? await get("/api/board"),
              let b = try? JSONDecoder().decode(Board.self, from: data) else { return }
        await MainActor.run { self.board = b }
    }

    func decide(_ gate: GateItem, _ decision: String) async {
        let body = GateDecision(run_id: gate.run_id, node_id: gate.node_id, decision: decision)
        _ = try? await post("/api/gate", JSONEncoder().encode(body))
        await refresh()
    }

    private func get(_ path: String) async throws -> Data {
        let (d, _) = try await URLSession.shared.data(from: Self.base.appendingPathComponent(path))
        return d
    }

    private func post(_ path: String, _ body: Data) async throws -> Data {
        var req = URLRequest(url: Self.base.appendingPathComponent(path))
        req.httpMethod = "POST"
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = body
        let (d, _) = try await URLSession.shared.data(for: req)
        return d
    }
}

// MARK: - Views

@main
struct FortApp: App {
    var body: some Scene { WindowGroup { BoardView() } }
}

struct BoardView: View {
    @StateObject private var client = FortClient()
    let timer = Timer.publish(every: 2, on: .main, in: .common).autoconnect()

    var body: some View {
        NavigationStack {
            List {
                if !client.board.gates.isEmpty {
                    Section("Gate inbox") {
                        ForEach(client.board.gates) { g in
                            VStack(alignment: .leading) {
                                Text(g.node_id).font(.headline)
                                Text(g.run_id.prefix(8)).font(.caption).foregroundStyle(.secondary)
                                HStack {
                                    Button("Approve") { Task { await client.decide(g, "approve") } }
                                        .buttonStyle(.borderedProminent)
                                    Button("Reject") { Task { await client.decide(g, "reject") } }
                                        .buttonStyle(.bordered).tint(.red)
                                }
                            }
                        }
                    }
                }
                Section("Runs") {
                    ForEach(client.board.runs) { r in
                        HStack {
                            Text(r.status).font(.caption).padding(4)
                                .background(.quaternary).clipShape(Capsule())
                            VStack(alignment: .leading) {
                                Text(r.title.isEmpty ? r.id : r.title)
                                Text(r.agent).font(.caption).foregroundStyle(.secondary)
                            }
                        }
                    }
                }
            }
            .navigationTitle("Fort")
            .task { await client.refresh() }
            .onReceive(timer) { _ in Task { await client.refresh() } }
        }
    }
}
