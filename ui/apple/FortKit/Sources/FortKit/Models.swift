//
//  Models.swift
//  FortKit
//
//  Wire models for Fort's control-plane HTTP/SSE contract.
//
//  These mirror the authoritative Go source at ui/contract.go EXACTLY —
//  field names, JSON keys, and optionality. `omitempty` string/int fields on
//  the Go side become Swift Optionals; everything else is non-optional.
//
//  JSON keys are snake_case on the wire; each struct declares explicit
//  CodingKeys so encoding/decoding is independent of any decoder key strategy.
//

import Foundation

/// The wire form of one append-only event-log row — the live-feed unit.
///
/// Mirrors `ui.Event`. `NodeID` (spec 027), `Data`, and `Code` are `omitempty`
/// on the Go side, so they are Optionals here. `type` carries the event kind as
/// a raw string, so newer kinds — including the `"tool"` / `"subagent"` activity
/// events (spec 030) — decode without any contract change.
public struct Event: Codable, Sendable, Identifiable, Hashable {
    public let id: Int
    public let runID: String
    /// The flow step this event is attributed to (spec 027); nil for run-level events.
    public let nodeID: String?
    public let type: String
    public let data: String?
    public let code: Int?
    public let time: String

    public init(
        id: Int,
        runID: String,
        nodeID: String? = nil,
        type: String,
        data: String? = nil,
        code: Int? = nil,
        time: String
    ) {
        self.id = id
        self.runID = runID
        self.nodeID = nodeID
        self.type = type
        self.data = data
        self.code = code
        self.time = time
    }

    enum CodingKeys: String, CodingKey {
        case id
        case runID = "run_id"
        case nodeID = "node_id"
        case type
        case data
        case code
        case time
    }
}

/// A board card. Mirrors `ui.RunSummary`. `body` and `flow_id` are `omitempty`.
public struct RunSummary: Codable, Sendable, Identifiable, Hashable {
    public let id: String
    public let title: String
    /// The task body/details, when present (spec 031 compose keeps title+body).
    public let body: String?
    public let agent: String
    public let status: String
    public let flowID: String?

    public init(
        id: String,
        title: String,
        body: String? = nil,
        agent: String,
        status: String,
        flowID: String? = nil
    ) {
        self.id = id
        self.title = title
        self.body = body
        self.agent = agent
        self.status = status
        self.flowID = flowID
    }

    enum CodingKeys: String, CodingKey {
        case id
        case title
        case body
        case agent
        case status
        case flowID = "flow_id"
    }
}

/// A node's state within a run. Mirrors `ui.NodeSummary`. `attempts` is `omitempty`.
public struct NodeSummary: Codable, Sendable, Identifiable, Hashable {
    public let nodeID: String
    public let type: String
    public let status: String
    public let attempts: Int?

    public var id: String { nodeID }

    public init(
        nodeID: String,
        type: String,
        status: String,
        attempts: Int? = nil
    ) {
        self.nodeID = nodeID
        self.type = type
        self.status = status
        self.attempts = attempts
    }

    enum CodingKeys: String, CodingKey {
        case nodeID = "node_id"
        case type
        case status
        case attempts
    }
}

/// A gate awaiting a human decision — the gate inbox unit.
///
/// Mirrors `ui.GateItem`. `input` is `omitempty`.
public struct GateItem: Codable, Sendable, Identifiable, Hashable {
    public let runID: String
    public let nodeID: String
    public let input: String?

    /// Stable identity for SwiftUI lists — a gate is unique per (run, node).
    public var id: String { "\(runID)/\(nodeID)" }

    public init(
        runID: String,
        nodeID: String,
        input: String? = nil
    ) {
        self.runID = runID
        self.nodeID = nodeID
        self.input = input
    }

    enum CodingKeys: String, CodingKey {
        case runID = "run_id"
        case nodeID = "node_id"
        case input
    }
}

/// The live board payload. Mirrors `ui.Board`.
public struct Board: Codable, Sendable, Hashable {
    public let runs: [RunSummary]
    public let gates: [GateItem]

    public init(runs: [RunSummary], gates: [GateItem]) {
        self.runs = runs
        self.gates = gates
    }

    enum CodingKeys: String, CodingKey {
        case runs
        case gates
    }
}

/// A run made replayable from the event log. Mirrors `ui.RunDetail`.
public struct RunDetail: Codable, Sendable, Hashable {
    public let run: RunSummary
    public let nodes: [NodeSummary]
    public let events: [Event]

    public init(run: RunSummary, nodes: [NodeSummary], events: [Event]) {
        self.run = run
        self.nodes = nodes
        self.events = events
    }

    enum CodingKeys: String, CodingKey {
        case run
        case nodes
        case events
    }
}

/// The command body for `POST /api/gate`. Mirrors `ui.GateDecision`.
///
/// `decision` is `"approve"` or `"reject"`. `edit` is `omitempty`.
public struct GateDecision: Codable, Sendable, Hashable {
    public let runID: String
    public let nodeID: String
    public let decision: String
    public let edit: String?

    public init(
        runID: String,
        nodeID: String,
        decision: String,
        edit: String? = nil
    ) {
        self.runID = runID
        self.nodeID = nodeID
        self.decision = decision
        self.edit = edit
    }

    enum CodingKeys: String, CodingKey {
        case runID = "run_id"
        case nodeID = "node_id"
        case decision
        case edit
    }
}

/// The command body for `POST /api/chat`. Mirrors `ui.ChatRequest`.
///
/// `agent` forces a specific agent; it is `omitempty`.
public struct ChatRequest: Codable, Sendable, Hashable {
    public let text: String
    public let agent: String?

    public init(text: String, agent: String? = nil) {
        self.text = text
        self.agent = agent
    }

    enum CodingKeys: String, CodingKey {
        case text
        case agent
    }
}

/// The response for chat/openclaw. Mirrors `ui.ChatResult`.
///
/// `kind` is `"task"` or `"flow"`. `route`, `queued`, `flow_id`, and `paused`
/// are all `omitempty`.
public struct ChatResult: Codable, Sendable, Hashable {
    public let kind: String
    public let runID: String
    public let route: String?
    public let queued: Bool?
    public let flowID: String?
    public let paused: String?

    public init(
        kind: String,
        runID: String,
        route: String? = nil,
        queued: Bool? = nil,
        flowID: String? = nil,
        paused: String? = nil
    ) {
        self.kind = kind
        self.runID = runID
        self.route = route
        self.queued = queued
        self.flowID = flowID
        self.paused = paused
    }

    enum CodingKeys: String, CodingKey {
        case kind
        case runID = "run_id"
        case route
        case queued
        case flowID = "flow_id"
        case paused
    }
}

/// The glanceable control-plane snapshot for constrained surfaces
/// (watch complication, CarPlay). Served at `GET /api/summary`.
///
/// Mirrors `ui.Summary`. `execution: false` means CONTROL-ONLY: there is no
/// deterministic engine attached, so chat only boards a queued task and gate
/// actions return HTTP 409.
public struct Summary: Codable, Sendable, Hashable {
    public let total: Int
    public let running: Int
    public let queued: Int
    public let blocked: Int
    public let succeeded: Int
    public let failed: Int
    public let execution: Bool
    public let gates: [GateItem]

    public init(
        total: Int,
        running: Int,
        queued: Int,
        blocked: Int,
        succeeded: Int,
        failed: Int,
        execution: Bool,
        gates: [GateItem]
    ) {
        self.total = total
        self.running = running
        self.queued = queued
        self.blocked = blocked
        self.succeeded = succeeded
        self.failed = failed
        self.execution = execution
        self.gates = gates
    }

    enum CodingKeys: String, CodingKey {
        case total
        case running
        case queued
        case blocked
        case succeeded
        case failed
        case execution
        case gates
    }
}

/// An inbound OpenClaw message. Mirrors `ui.OpenClawMessage`.
public struct OpenClawMessage: Codable, Sendable, Hashable {
    public let from: String
    public let text: String

    public init(from: String, text: String) {
        self.from = from
        self.text = text
    }

    enum CodingKeys: String, CodingKey {
        case from
        case text
    }
}

/// A generic command result (gate decisions). Mirrors `ui.ActionResult`.
///
/// `paused_node` is `omitempty`.
public struct ActionResult: Codable, Sendable, Hashable {
    public let state: String
    public let pausedNode: String?

    public init(state: String, pausedNode: String? = nil) {
        self.state = state
        self.pausedNode = pausedNode
    }

    enum CodingKeys: String, CodingKey {
        case state
        case pausedNode = "paused_node"
    }
}

/// A pending task queued on the board (spec 025). Mirrors `ui.BacklogItem`.
///
/// `body`, `agent`, `machine`, and `labels` are `omitempty` on the Go side.
/// `source` is `"user"` or `"agent"`.
public struct BacklogItem: Codable, Sendable, Identifiable, Hashable {
    public let id: String
    public let title: String
    public let body: String?
    public let agent: String?
    public let machine: String?
    public let labels: [String]?
    public let source: String

    public init(
        id: String,
        title: String,
        body: String? = nil,
        agent: String? = nil,
        machine: String? = nil,
        labels: [String]? = nil,
        source: String
    ) {
        self.id = id
        self.title = title
        self.body = body
        self.agent = agent
        self.machine = machine
        self.labels = labels
        self.source = source
    }

    enum CodingKeys: String, CodingKey {
        case id
        case title
        case body
        case agent
        case machine
        case labels
        case source
    }
}

/// The command body for `POST /api/breakdown` (spec 026). Mirrors
/// `ui.BreakdownRequest`. `agent` and `machine` are `omitempty`.
public struct BreakdownRequest: Codable, Sendable, Hashable {
    public let text: String
    public let agent: String?
    public let machine: String?

    public init(text: String, agent: String? = nil, machine: String? = nil) {
        self.text = text
        self.agent = agent
        self.machine = machine
    }

    enum CodingKeys: String, CodingKey {
        case text
        case agent
        case machine
    }
}

/// The response for `POST /api/breakdown` (spec 026): the visible planner run's
/// id. Sub-tasks appear in the backlog when that run completes. Mirrors
/// `ui.BreakdownResult`.
public struct BreakdownResult: Codable, Sendable, Hashable {
    public let runID: String

    public init(runID: String) {
        self.runID = runID
    }

    enum CodingKeys: String, CodingKey {
        case runID = "run_id"
    }
}
