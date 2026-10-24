import Foundation
import Testing
@testable import FortKit

private func week(_ json: String) throws -> WeekView { try JSONDecoder().decode(WeekView.self,from:Data(json.utf8)) }
private let lastWeek = """
{"weekId":"w1","revisionId":"r1","number":44,"name":"Glass Bay","startsOn":"2026-10-26","endsOn":"2026-11-01","state":"live","mainGoalId":"beta",
 "goals":[{"goalId":"beta","text":"Fort beta on TestFlight","zone":"code","lead":{"kind":"agent","agentId":"pascal"},"targetPoints":9},
          {"goalId":"done","text":"Refinance call","zone":"money","lead":{"kind":"agent","agentId":"milton"},"targetPoints":3},
          {"goalId":"life","text":"Your own list","zone":"personal","lead":{"kind":"you"},"targetPoints":3}],
 "plan":[{"itemId":"i1","goalId":"beta","task":"Claude runner adapter PR","owner":{"kind":"agent","agentId":"pascal"},"size":"significant","doneBuildId":"b1"},
         {"itemId":"i2","goalId":"beta","task":"Create the App Store Connect API key","owner":{"kind":"you"},"size":"moderate","unlocks":"Ship to TestFlight","doneBuildId":null},
         {"itemId":"i3","goalId":"done","task":"Lock the rate","owner":{"kind":"agent","agentId":"milton"},"size":"moderate","doneBuildId":"b2"},
         {"itemId":"i4","goalId":"life","task":"Book the dentist","owner":{"kind":"you"},"size":"small","doneBuildId":null}],
 "builds":[],
 "progress":{"points":9,"goals":[{"goalId":"beta","points":6,"targetPoints":9,"met":false},{"goalId":"done","points":3,"targetPoints":3,"met":true},{"goalId":"life","points":0,"targetPoints":3,"met":false}],"goalsMet":1,"goalsCounted":2,"grade":"C"}}
"""
private func agent(_ id: String) -> Agent {
    Agent(agentId:id,name:id.capitalized,homeConversationId:id+"-home",binding:Binding(bindingRevisionId:id+"-1",provider:"codex",runtimeVersion:"1",selectedModel:"gpt",hostId:"local",enabled:true),readiness:"requiresLiveReview")
}

@Test func planningCarriesOnlyUnmetGoalsWithOpenWorkAndKeepsTheMainQuest() throws {
    let draft = PlanDraft(previous:try week(lastWeek),today:"2026-10-31",name:"Harbor Point")
    #expect(draft.goals.map(\.text) == ["Fort beta on TestFlight","Your own list"])
    #expect(draft.goals.map(\.carriedFrom) == ["beta","life"])
    #expect(draft.mainGoalID == draft.goals[0].id)
    #expect(draft.tasks.map(\.text) == ["Create the App Store Connect API key","Book the dentist"])
    #expect(draft.tasks.allSatisfy(\.carried) && draft.tasks[0].unlocks == "Ship to TestFlight")
    #expect(draft.startsOn == "2026-11-02")
    #expect(draft.target(for:draft.goals[0].id) == 3)
}

@Test func aFirstWeekStartsThisMondayOrTomorrowOnASunday() {
    #expect(PlanDraft.startDate(previous:nil,today:"2026-10-28") == "2026-10-26")
    #expect(PlanDraft.startDate(previous:nil,today:"2026-10-26") == "2026-10-26")
    #expect(PlanDraft.startDate(previous:nil,today:"2026-11-01") == "2026-11-02")
    #expect(PlanDraft.startDate(previous:nil,today:"2027-01-03") == "2027-01-04")
}

@Test func capacityCountsPlannedPointsPerPersonAndFlagsOverbooking() {
    var draft = PlanDraft(previous:nil,today:"2026-10-26",name:"Glass Bay")
    let goal = try! #require(draft.addGoal(text:"Ship the beta",zone:.code,lead:.agent("pascal")))
    draft.addTask(goalID:goal,text:"Adapter",owner:.agent("pascal"),size:.milestone)
    draft.addTask(goalID:goal,text:"Upload",owner:.agent("pascal"),size:.small)
    draft.addTask(goalID:goal,text:"Key",owner:.you,size:.moderate)
    let loads = draft.loads(agents:[agent("pascal"),agent("lewis")])
    #expect(loads.map(\.owner) == [.you,.agent("pascal"),.agent("lewis")])
    #expect(loads.map(\.points) == [3,13,0])
    #expect(loads.map(\.overbooked) == [false,true,false])
}

@Test func foundingNeedsANameAGoalAndTasksUnderEveryGoal() throws {
    var draft = PlanDraft(previous:nil,today:"2026-10-26",name:" ")
    #expect(draft.problems == ["Name the city.","Pick at least one goal."])
    draft.name = "Glass Bay"
    let goal = try #require(draft.addGoal(text:"Ship the beta",zone:.code,lead:.you))
    #expect(draft.problems == ["Add a task to “Ship the beta”."] && !draft.canFound)
    let task = try #require(draft.addTask(goalID:goal,text:"Upload",owner:.you,size:.moderate))
    #expect(draft.canFound)
    draft.removeTask(task); draft.removeGoal(goal)
    #expect(draft.goals.isEmpty && draft.tasks.isEmpty && draft.mainGoalID == nil)
    #expect(draft.addTask(goalID:"missing",text:"x",owner:.you,size:.small) == nil)
    for index in 0..<PlanDraft.maximumGoals { #expect(draft.addGoal(text:"Goal \(index)",zone:.writing,lead:.you) != nil) }
    #expect(draft.addGoal(text:"One too many",zone:.writing,lead:.you) == nil)
}

@Test func theFoundingRequestMatchesTheCoreContract() throws {
    var draft = PlanDraft(previous:try week(lastWeek),today:"2026-10-31",name:"  Harbor Point ")
    let goal = try #require(draft.addGoal(text:"Revise chapter 4",zone:.writing,lead:.agent("lewis")))
    draft.addTask(goalID:goal,text:" Line edit ",owner:.agent("lewis"),size:.significant)
    let json = try JSONSerialization.jsonObject(with:JSONEncoder().encode(draft.request)) as! [String:Any]
    #expect(Set(json.keys) == ["name","startsOn","goals","plan","mainGoalKey"])
    #expect(json["name"] as? String == "Harbor Point" && json["startsOn"] as? String == "2026-11-02")
    let goals = try #require(json["goals"] as? [[String:Any]])
    #expect(goals.count == 3 && Set(goals[2].keys) == ["key","text","zone","lead","targetPoints"])
    #expect(goals[2]["zone"] as? String == "writing" && goals[2]["targetPoints"] as? Int == 6)
    #expect((goals[2]["lead"] as? [String:String]) == ["kind":"agent","agentId":"lewis"])
    let plan = try #require(json["plan"] as? [[String:Any]])
    #expect(plan.last?["task"] as? String == "Line edit" && plan.last?["goalKey"] as? String == goal)
    #expect(plan[0]["unlocks"] as? String == "Ship to TestFlight" && plan[1]["unlocks"] == nil)
    #expect((plan[1]["owner"] as? [String:String]) == ["kind":"you"])
    #expect(json["mainGoalKey"] as? String == draft.goals[0].id)
}
