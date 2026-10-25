import Foundation
import Testing
@testable import FortKit

@Test func runMeterFillsAsTheRunMovesAndLightsWhenItNeedsYou() {
    #expect(RunMeter.make(state:"queued") == RunMeter(filled:1,lamp:.waiting))
    #expect(RunMeter.make(state:"running") == RunMeter(filled:2,lamp:.waiting))
    #expect(RunMeter.make(state:"waitingForApproval") == RunMeter(filled:2,lamp:.attention))
    #expect(RunMeter.make(state:"outcomeUnknown").lamp == .attention)
    #expect(RunMeter.make(state:"completed") == RunMeter(filled:3,lamp:.ready))
    #expect(RunMeter.make(state:"failed") == RunMeter(filled:3,lamp:.attention))
    #expect(RunMeter.make(state:"canceled").lamp == .off)
    #expect(RunMeter.make(state:"somethingNew") == RunMeter(filled:0,lamp:.off))
}

@Test func troubleCardsNameEachProblemWithOnePlaceToFixIt() {
    #expect(TroubleCard.make(.init()).isEmpty)
    var facts = TroubleCard.Facts(runtimeState:"stopped",runtimeDetail:"Stopped after a crash.",accountNeedsYou:true,scheduleFault:"A schedule could not be checked.")
    let cards = TroubleCard.make(facts)
    #expect(cards.map(\.kind) == [.hostStopped,.signedOut,.scheduleFault])
    #expect(cards.map(\.tab) == [.devices,.accounts,.playbook])
    #expect(cards[0].line == "Stopped after a crash.")
    facts.runtimeState = "restartRequired"; facts.runtimeDetail = nil
    #expect(TroubleCard.make(facts).first?.kind == .restartRequired)
    facts.runtimeState = "ready"
    #expect(TroubleCard.make(facts).map(\.kind) == [.signedOut,.scheduleFault])
}

@Test func lostContactHidesStaleTroubleUntilFortCanCheckAgain() {
    let facts = TroubleCard.Facts(lostContact:true,status:"The network connection was lost.",runtimeState:"stopped",accountNeedsYou:true)
    let cards = TroubleCard.make(facts)
    #expect(cards.map(\.kind) == [.lostContact])
    #expect(cards[0].line == "The network connection was lost." && cards[0].tab == .devices)
    #expect(TroubleCard.make(.init(lostContact:true)).first?.line.contains("Saved history is kept") == true)
}

@Test func chatHidesFinishedRepliesButKeepsAnythingThatNeedsYou() {
    #expect(!ChatQuiet.showsRun(state:"completed",isRoutine:false))
    #expect(ChatQuiet.showsRun(state:"completed",isRoutine:true))
    for state in ["queued","running","waitingForApproval","outcomeUnknown","failed","canceled","closedUnknown","cancelRequested","somethingNew"] {
        #expect(ChatQuiet.showsRun(state:state,isRoutine:false))
    }
}

@Test func chatHeaderShowsOneLightAndOneWord() {
    #expect(ChatQuiet.header(connected:true,preview:false,runtimeState:"ready",status:"Connected · local prototype") == (.ready,"Ready"))
    #expect(ChatQuiet.header(connected:true,preview:false,runtimeState:"stopped",status:"").lamp == .attention)
    #expect(ChatQuiet.header(connected:false,preview:false,runtimeState:nil,status:"The network connection was lost.") == (.attention,"The network connection was lost."))
    #expect(ChatQuiet.header(connected:false,preview:true,runtimeState:nil,status:"Offline preview").lamp == .off)
}
