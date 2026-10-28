import Foundation

/// The Quest log home, built only from real records: what needs you across every chat, most urgent
/// first, and a panel for each chat with its crew and live runs. The same rules as the web client's
/// questLog and chatPanels (successor/web/transcript.js).
public struct QuestLog: Equatable, Sendable {
    public enum Kind: String, Equatable, Sendable { case approval, failed, waiting }
    /// The status grammar every surface shares, plus quiet for a chat with nothing going on.
    public enum Tone: Equatable, Sendable { case working, needsYou, done, failed, off, quiet }
    public struct Quest: Identifiable, Equatable, Sendable {
        public let id: String; public let kind: Kind; public let conversationID: String; public let chat: String
        public let agent: Agent?; public let title: String; public let detail: String?; public let approval: Approval?
    }
    public struct LiveRun: Identifiable, Equatable, Sendable {
        public let id: String; public let agentName: String?; public let text: String; public let filled: Int; public let tone: Tone
    }
    public struct Panel: Identifiable, Equatable, Sendable {
        public let id: String; public let name: String; public let tone: Tone; public let quests: Int; public let crew: [Agent]; public let live: [LiveRun]
        public var line: String {
            let working = live.filter { $0.tone == .working }.count
            let parts = [quests == 0 ? nil : quests == 1 ? "1 needs you" : "\(quests) need you",working == 0 ? nil : "\(working) running"].compactMap { $0 }
            return parts.isEmpty ? "Quiet" : parts.joined(separator:" · ")
        }
    }
    public let quests: [Quest]
    public let panels: [Panel]
    /// The one line under the header: "2 quests · 3 running".
    public var summary: String {
        let running = panels.reduce(0) { $0 + $1.live.filter { $0.tone == .working }.count }
        let head = quests.isEmpty ? "No quests" : quests.count == 1 ? "1 quest" : "\(quests.count) quests"
        return running == 0 ? head : "\(head) · \(running) running"
    }

    public static func tone(_ state: String) -> Tone {
        switch state {
        case "queued", "running", "cancelRequested": .working
        case "waitingForApproval": .needsYou
        case "completed": .done
        case "canceled", "closedUnknown": .off
        default: .failed
        }
    }
    /// Pips out of three, the same steps as RunMeter.
    public static func filled(_ state: String) -> Int {
        switch state {
        case "queued": 1
        case "running", "cancelRequested", "waitingForApproval", "outcomeUnknown": 2
        case "completed", "failed", "canceled", "closedUnknown": 3
        default: 0
        }
    }

    public static func make(conversations: [Conversation], agents: [Agent], snapshots: [String: Snapshot], now: Date = Date()) -> QuestLog {
        let byID = Dictionary(agents.map { ($0.agentId,$0) },uniquingKeysWith:{ first,_ in first })
        func agent(of run: Run, in snapshot: Snapshot, conversation: Conversation) -> Agent? {
            let asked = snapshot.messages.first { $0.requestId == run.requestId && ($0.recipientAgentId != nil || $0.agentId != nil) }
            if let id = asked?.recipientAgentId ?? asked?.agentId { return byID[id] }
            return conversation.participantAgentIds.count == 1 ? byID[conversation.participantAgentIds[0]] : nil
        }
        var decide: [Quest] = [], failed: [Quest] = [], waiting: [Quest] = []
        for conversation in conversations {
            guard let snapshot = snapshots[conversation.conversationId] else { continue }
            var decidable = Set<String>()
            for approval in snapshot.approvals where approval.canDecide(at:now) {
                let run = snapshot.runs.first { $0.runId == approval.runId }
                decidable.insert(approval.runId)
                let detail = run?.statusDetail == approval.actionDescription ? nil : run?.statusDetail
                decide.append(Quest(id:"approval:"+approval.approvalId,kind:.approval,conversationID:conversation.conversationId,chat:conversation.name,agent:run.flatMap { agent(of:$0,in:snapshot,conversation:conversation) },title:approval.actionDescription,detail:detail,approval:approval))
            }
            for run in snapshot.runs where run.state == "waitingForApproval" && !decidable.contains(run.runId) {
                waiting.append(Quest(id:"run:"+run.runId,kind:.waiting,conversationID:conversation.conversationId,chat:conversation.name,agent:agent(of:run,in:snapshot,conversation:conversation),title:run.statusDetail ?? run.displayState,detail:"Open the chat to answer.",approval:nil))
            }
            // A failed turn stops being a quest as soon as a later turn in the same chat starts.
            if let latest = snapshot.runs.last, tone(latest.state) == .failed {
                failed.append(Quest(id:"run:"+latest.runId,kind:.failed,conversationID:conversation.conversationId,chat:conversation.name,agent:agent(of:latest,in:snapshot,conversation:conversation),title:latest.displayState,detail:latest.statusDetail,approval:nil))
            }
        }
        let quests = decide + failed + waiting
        let panels = conversations.map { conversation -> Panel in
            let snapshot = snapshots[conversation.conversationId]
            let mine = quests.filter { $0.conversationID == conversation.conversationId }
            let live = (snapshot?.runs ?? []).filter { tone($0.state) == .working || tone($0.state) == .needsYou }.map { run in
                LiveRun(id:run.runId,agentName:snapshot.flatMap { agent(of:run,in:$0,conversation:conversation) }?.name,text:run.displayState,filled:filled(run.state),tone:tone(run.state))
            }
            let state: Tone = mine.contains { $0.kind == .failed } ? .failed : !mine.isEmpty ? .needsYou : !live.isEmpty ? .working : .quiet
            return Panel(id:conversation.conversationId,name:conversation.name,tone:state,quests:mine.count,crew:conversation.participantAgentIds.compactMap { byID[$0] },live:live)
        }
        return QuestLog(quests:quests,panels:panels)
    }
}
