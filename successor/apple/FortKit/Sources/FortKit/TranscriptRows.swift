public enum TranscriptRow: Identifiable {
    case message(Message)
    case run(Run)
    case approval(Approval)

    public var id: String {
        switch self {
        case .message(let value): "message:" + value.id
        case .run(let value): "run:" + value.id
        case .approval(let value): "approval:" + value.id
        }
    }
}

public extension Snapshot {
    var transcriptRows: [TranscriptRow] {
        let lastMessage = Dictionary(messages.map { ($0.requestId, $0.id) }, uniquingKeysWith: { _, latest in latest })
        let runsByRequest = Dictionary(grouping:runs, by:\.requestId)
        let approvalsByRun = Dictionary(grouping:approvals, by:\.runId)
        var rows: [TranscriptRow] = []
        func append(_ run: Run) {
            rows.append(.run(run))
            rows.append(contentsOf:(approvalsByRun[run.id] ?? []).map(TranscriptRow.approval))
        }
        for message in messages {
            rows.append(.message(message))
            if lastMessage[message.requestId] == message.id {
                for run in runsByRequest[message.requestId] ?? [] { append(run) }
            }
        }
        for run in runs where lastMessage[run.requestId] == nil { append(run) }
        let runIDs = Set(runs.map(\.id))
        rows.append(contentsOf:approvals.filter { !runIDs.contains($0.runId) }.map(TranscriptRow.approval))
        return rows
    }
}
