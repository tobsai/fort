import Foundation
import Testing
@testable import FortKit

private let weekJSON = """
{"weekId":"w1","revisionId":"r1","number":44,"name":"Glass Bay","startsOn":"2026-10-26","endsOn":"2026-11-01","state":"live","mainGoalId":"beta",
 "goals":[{"goalId":"beta","text":"Fort beta on TestFlight","zone":"code","lead":{"kind":"agent","agentId":"pascal"},"targetPoints":9},
          {"goalId":"life","text":"Your own list","zone":"personal","lead":{"kind":"you"},"targetPoints":3}],
 "plan":[{"itemId":"i1","goalId":"beta","task":"Claude runner adapter PR","owner":{"kind":"agent","agentId":"pascal"},"size":"significant","doneBuildId":"b1"},
         {"itemId":"i2","goalId":"beta","task":"Upload the beta","owner":{"kind":"agent","agentId":"pascal"},"size":"moderate","doneBuildId":null},
         {"itemId":"i3","goalId":"beta","task":"Create the App Store Connect API key","owner":{"kind":"you"},"size":"moderate","unlocks":"Ship to TestFlight","doneBuildId":null},
         {"itemId":"i4","goalId":"life","task":"Book the dentist","owner":{"kind":"you"},"size":"small","doneBuildId":null}],
 "builds":[{"buildId":"b1","goalId":"beta","itemId":"i1","task":"Claude runner adapter PR","size":"significant","by":{"kind":"agent","agentId":"pascal"},"placedAt":"2026-10-27T10:00:00.000Z","evidence":"PR #27"}],
 "progress":{"points":6,"goals":[{"goalId":"beta","points":6,"targetPoints":9,"met":false},{"goalId":"life","points":0,"targetPoints":3,"met":false}],"goalsMet":0,"goalsCounted":1,"grade":"D"}}
"""
private func agent(_ id: String) -> Agent {
    Agent(agentId:id,name:id.capitalized,homeConversationId:id+"-home",binding:Binding(bindingRevisionId:id+"-1",provider:"codex",runtimeVersion:"1",selectedModel:"gpt",hostId:"local",enabled:true),readiness:"requiresLiveReview")
}

@Test func weekDecodesFromTheCoreContractAndRoundTrips() throws {
    let week = try JSONDecoder().decode(WeekView.self,from:Data(weekJSON.utf8))
    #expect(week.isLive && week.number == 44)
    #expect(week.plan[2].owner == .you && week.plan[0].owner == .agent("pascal"))
    #expect(week.plan[2].unlocks == "Ship to TestFlight" && week.builds[0].evidence == "PR #27")
    #expect(WeekSize.allCases.map(\.points) == [1,3,6,12])
    #expect(try JSONDecoder().decode(WeekView.self,from:JSONEncoder().encode(week)) == week)
    #expect(throws:DecodingError.self) { try JSONDecoder().decode(WeekOwner.self,from:Data(#"{"kind":"robot"}"#.utf8)) }
}

@Test func cityHUDShowsTheWeekCrewAndOnlyYourOpenQuestsAsOrders() throws {
    let week = try JSONDecoder().decode(WeekView.self,from:Data(weekJSON.utf8))
    let hud = CityHUD.make(week:week,agents:[agent("pascal"),agent("lewis")],snapshot:nil,today:"2026-10-30")
    #expect(hud.title == "Week 44 · Glass Bay")
    #expect(hud.dayLabel == "Fri" && hud.daysLeft == 2)
    #expect(hud.points == 6 && hud.grade == "D")
    #expect(hud.goals.map(\.isMain) == [true,false])
    #expect(hud.goals[0].fraction == 6.0/9.0)
    #expect(hud.members.map(\.state) == [.ready,.ready])
    #expect(hud.members[0].nextTask == "Upload the beta" && hud.members[0].openTasks == 1 && hud.members[1].nextTask == nil)
    #expect(hud.orders.map(\.title) == ["Create the App Store Connect API key","Book the dentist"])
    #expect(hud.orders[0].detail == "Fort beta on TestFlight · unlocks Ship to TestFlight")
}

@Test func cityHUDWithoutAWeekStaysEmptyAndClampsDaysOutsideTheWeek() throws {
    let empty = CityHUD.make(week:nil,agents:[agent("pascal")],snapshot:nil,today:"2026-10-30")
    #expect(empty.title == "No week planned" && empty.goals.isEmpty && empty.orders.isEmpty && empty.dayLabel == nil)
    let week = try JSONDecoder().decode(WeekView.self,from:Data(weekJSON.utf8))
    #expect(CityHUD.make(week:week,agents:[],snapshot:nil,today:"2026-11-09").daysLeft == 0)
    #expect(CityHUD.make(week:week,agents:[],snapshot:nil,today:"2026-10-26").dayLabel == "Mon")
}

@Test func cityMapPlacesBuildsSolidAndOpenPlanAsGhosts() throws {
    let week = try JSONDecoder().decode(WeekView.self,from:Data(weekJSON.utf8))
    let blocks = CityMap.layout(week)
    #expect(blocks.filter { !$0.planned }.count == week.builds.count)
    #expect(blocks.filter(\.planned).count == week.plan.filter { $0.doneBuildId == nil }.count)
    #expect(Set(blocks.map { "\($0.x),\($0.y)" }).count == blocks.count)
}
