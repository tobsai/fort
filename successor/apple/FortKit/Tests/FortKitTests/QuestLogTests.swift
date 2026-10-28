import Foundation
import Testing
@testable import FortKit

// The same chats as the web client's Quest log tests (successor/web/transcript.test.mjs), so both surfaces agree.
private func agent(_ id: String) -> Agent {
    Agent(agentId:id,name:id.capitalized,homeConversationId:"home-"+id,binding:Binding(bindingRevisionId:id+"-1",provider:"codex",runtimeVersion:"1",selectedModel:"gpt",hostId:"local",enabled:true),readiness:"requiresLiveReview")
}
private let now = ISO8601DateFormatter().date(from:"2026-10-28T12:00:00Z")!
private func snapshot(_ id: String, messages: String, runs: String, approvals: String = "[]") throws -> Snapshot {
    try JSONDecoder().decode(Snapshot.self,from:Data(#"{"installationId":"i","recoveryEpoch":"e","conversationId":"\#(id)","cursor":1,"lastReadMessageSequence":0,"messages":\#(messages),"runs":\#(runs),"approvals":\#(approvals)}"#.utf8))
}
private func message(_ id: String, _ request: String, to recipient: String? = nil) -> String {
    let extra = recipient.map { ",\"recipientAgentId\":\"\($0)\"" } ?? ""
    return #"{"messageId":"\#(id)","conversationId":"c","requestId":"\#(request)","role":"user","text":"hi","sequence":1,"createdAt":""\#(extra)}"#
}
private func run(_ id: String, _ request: String, _ state: String, detail: String? = nil) -> String {
    let extra = detail.map { ",\"statusDetail\":\"\($0)\"" } ?? ""
    return #"{"runId":"\#(id)","requestId":"\#(request)","conversationId":"c","bindingRevisionId":"b","state":"\#(state)","selectedModel":"m","effectiveModel":null,"modelEvidence":"unreported","attemptId":"a"\#(extra)}"#
}
private func gate(_ id: String, run: String, expires: String = "2026-10-28T13:00:00.000Z") -> String {
    #"{"approvalId":"\#(id)","installationId":"i","recoveryEpoch":"e","runId":"\#(run)","attemptId":"a","bindingRevisionId":"b","actionDigest":"d","resourceScope":["routine:continue"],"actionDescription":"Publish the post","expiresAt":"\#(expires)","state":"pending","decidedByDeviceId":null,"decidedAt":null,"kind":"routineGate","routineRevisionId":"r","stepId":"s"}"#
}
private let conversations = [
    Conversation(conversationId:"home-lewis",name:"Lewis",kind:"home",participantAgentIds:["lewis"]),
    Conversation(conversationId:"room",name:"Fort build",kind:"shared",participantAgentIds:["lewis","marty"]),
    Conversation(conversationId:"home-marty",name:"Marty",kind:"home",participantAgentIds:["marty"])
]
private func chats(approval: String? = nil) throws -> [String: Snapshot] {
    [
        "home-lewis": try snapshot("home-lewis",messages:"[\(message("l1","q1"))]",runs:"[\(run("lr1","q1","completed")),\(run("lr2","q2","failed",detail:"The Mac mini did not answer."))]"),
        "room": try snapshot("room",messages:"[\(message("f1","q3",to:"marty")),\(message("f2","q4",to:"lewis"))]",runs:"[\(run("fr1","q3","waitingForApproval",detail:"Publish the post?")),\(run("fr2","q4","running"))]",approvals:"[\(approval ?? gate("g1",run:"fr1"))]"),
        "home-marty": try snapshot("home-marty",messages:"[]",runs:"[\(run("mr1","q5","failed")),\(run("mr2","q6","queued"))]")
    ]
}

@Test func theQuestLogPutsDecisionsFirstThenChatsWhoseLatestTurnFailed() throws {
    let log = QuestLog.make(conversations:conversations,agents:[agent("lewis"),agent("marty")],snapshots:try chats(),now:now)
    #expect(log.quests.map(\.kind) == [.approval,.failed])
    #expect(log.quests.map(\.conversationID) == ["room","home-lewis"])
    #expect(log.quests.map { $0.agent?.name } == ["Marty","Lewis"])
    #expect(log.quests[0].title == "Publish the post" && log.quests[0].detail == "Publish the post?")
    #expect(log.quests[1].title == "Could not complete this turn" && log.quests[1].detail == "The Mac mini did not answer.")
    #expect(log.summary == "2 quests · 2 running")
}

@Test func aRunWaitingOnAnApprovalThisDeviceCannotDecideStillShows() throws {
    let log = QuestLog.make(conversations:conversations,agents:[agent("lewis"),agent("marty")],snapshots:try chats(approval:gate("g1",run:"fr1",expires:"2026-10-28T11:00:00.000Z")),now:now)
    #expect(log.quests.map(\.kind) == [.failed,.waiting])
    #expect(log.quests[1].title == "Publish the post?")
}

@Test func eachChatIsAPanelWithItsCrewLiveRunsAndState() throws {
    let log = QuestLog.make(conversations:conversations,agents:[agent("lewis"),agent("marty")],snapshots:try chats(),now:now)
    #expect(log.panels.map(\.name) == ["Lewis","Fort build","Marty"])
    #expect(log.panels.map(\.tone) == [.failed,.needsYou,.working])
    #expect(log.panels.map(\.quests) == [1,1,0])
    #expect(log.panels[1].crew.map(\.name) == ["Lewis","Marty"])
    #expect(log.panels[1].live.map(\.agentName) == ["Marty","Lewis"])
    #expect(log.panels[1].live.map(\.filled) == [2,2] && log.panels[1].live.map(\.tone) == [.needsYou,.working])
    #expect(log.panels[2].live.map(\.filled) == [1])
    #expect(log.panels.map(\.line) == ["1 needs you","1 needs you · 1 running","1 running"])
    #expect(QuestLog.make(conversations:[],agents:[],snapshots:[:]).summary == "No quests")
}
